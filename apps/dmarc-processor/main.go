package main

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type EmailSuppression struct {
	ID             uint      `gorm:"primaryKey"`
	Email          string    `gorm:"uniqueIndex;size:255;not null"`
	Reason         *string   `gorm:"size:255"`
	SuspendedUntil time.Time `gorm:"not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type dmarcSummary struct {
	ReportDomain string
	DateRange    string
	Total        int
	Failed       int
	Pass         int
}

type dmarcXMLReport struct {
	XMLName        xml.Name `xml:"feedback"`
	ReportMetadata struct {
		OrgName   string `xml:"org_name"`
		Email     string `xml:"email"`
		DateRange struct {
			Begin int64 `xml:"begin"`
			End   int64 `xml:"end"`
		} `xml:"date_range"`
	} `xml:"report_metadata"`
	PolicyPublished struct {
		Domain string `xml:"domain"`
	} `xml:"policy_published"`
	Records []struct {
		Row struct {
			Count int `xml:"count"`
		} `xml:"row"`
		PolicyEvaluated struct {
			Disposition string `xml:"disposition"`
			Dkim        string `xml:"dkim"`
			Spf         string `xml:"spf"`
		} `xml:"policy_evaluated"`
	} `xml:"record"`
}

type inboundPayload struct {
	From        string            `json:"from"`
	To          string            `json:"to"`
	Subject     string            `json:"subject"`
	Headers     map[string]string `json:"headers"`
	Text        string            `json:"text"`
	Html        string            `json:"html"`
	Raw         *string           `json:"raw"`
	Attachments []struct {
		Filename    string `json:"filename"`
		ContentType string `json:"content_type"`
		ContentB64  string `json:"content_base64"`
	} `json:"attachments"`
}

func main() {
	initSentry()
	db := mustOpenDB()
	_ = db.AutoMigrate(&EmailSuppression{})

	port := env("DMARC_PORT", "8080")
	app := fiber.New()

	app.Get("/healthz", func(c *fiber.Ctx) error { return c.SendString("ok") })
	app.Post("/inbound", func(c *fiber.Ctx) error {
		sec := c.Get("X-Email-Integration-Secret")
		if strings.TrimSpace(sec) == "" || sec != env("EMAIL_INTEGRATION_SECRET", "change_me_email_inbound_secret") {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		var body inboundPayload
		if err := parseJSONStrict(c, &body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		sentry.CaptureMessage(fmt.Sprintf("[DMARC] inbound from=%s to=%s subject=%s", body.From, body.To, body.Subject))

		var dmarc *dmarcSummary
		for _, att := range body.Attachments {
			name := strings.ToLower(att.Filename)
			ctype := strings.ToLower(att.ContentType)
			if !(strings.HasSuffix(name, ".xml") || strings.HasSuffix(name, ".zip") || strings.HasSuffix(name, ".gz") || strings.Contains(ctype, "xml") || strings.Contains(ctype, "zip") || strings.Contains(ctype, "gzip")) {
				continue
			}
			raw, _ := base64.StdEncoding.DecodeString(att.ContentB64)
			if strings.HasSuffix(name, ".zip") || strings.Contains(ctype, "zip") {
				if sum := parseDMARCFromZip(raw); sum != nil {
					dmarc = sum
					break
				}
			} else if strings.HasSuffix(name, ".gz") || strings.Contains(ctype, "gzip") {
				if sum := parseDMARCFromGzip(raw); sum != nil {
					dmarc = sum
					break
				}
			} else {
				if sum := parseDMARCXML(raw); sum != nil {
					dmarc = sum
					break
				}
			}
		}

		suppressed := []string{}
		if dmarc == nil {
			candidates := extractBouncedRecipients(body.Text + "\n" + body.Html)
			for _, addr := range candidates {
				if suppressEmail(db, addr, "bounced", 7*24*time.Hour) {
					suppressed = append(suppressed, addr)
				}
			}
		}

		embeds := make([]map[string]any, 0, 2)
		if dmarc != nil {
			rate := 0.0
			if dmarc.Total > 0 {
				rate = float64(dmarc.Failed) / float64(dmarc.Total) * 100.0
			}
			fields := []map[string]any{
				{"name": "Domain", "value": dmarc.ReportDomain, "inline": true},
				{"name": "Range", "value": dmarc.DateRange, "inline": true},
				{"name": "Total", "value": fmt.Sprintf("%d", dmarc.Total), "inline": true},
				{"name": "Pass", "value": fmt.Sprintf("%d", dmarc.Pass), "inline": true},
				{"name": "Failed", "value": fmt.Sprintf("%d", dmarc.Failed), "inline": true},
				{"name": "Fail %", "value": fmt.Sprintf("%.2f%%", rate), "inline": true},
			}
			embeds = append(embeds, map[string]any{
				"title":       "DMARC Aggregate Report",
				"description": body.Subject,
				"color":       0x5865F2,
				"fields":      fields,
				"timestamp":   time.Now().UTC().Format(time.RFC3339),
			})
		}
		if len(suppressed) > 0 {
			fields := []map[string]any{
				{"name": "Suppressed", "value": strings.Join(suppressed, ", ")},
				{"name": "Until", "value": time.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339)},
			}
			embeds = append(embeds, map[string]any{
				"title":       "Email Delivery Suppressed",
				"description": "High bounce detected. Temporarily paused delivery.",
				"color":       0xE74C3C,
				"fields":      fields,
				"timestamp":   time.Now().UTC().Format(time.RFC3339),
			})
		}
		if len(embeds) == 0 {
			embeds = append(embeds, map[string]any{
				"title":       "Inbound Email Received",
				"description": fmt.Sprintf("from=%s to=%s subject=%s", body.From, body.To, body.Subject),
				"color":       0x95A5A6,
				"timestamp":   time.Now().UTC().Format(time.RFC3339),
			})
		}
		go sendDiscordEmbeds(embeds)
		return c.SendStatus(202)
	})

	addr := ":" + port
	log.Printf("DMARC processor listening on %s", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatal(err)
	}
}

func parseJSONStrict(c *fiber.Ctx, dst any) error {
	dec := json.NewDecoder(strings.NewReader(string(c.Body())))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func parseDMARCXML(b []byte) *dmarcSummary {
	var r dmarcXMLReport
	if err := xml.Unmarshal(b, &r); err != nil {
		return nil
	}
	sum := dmarcSummary{}
	sum.ReportDomain = r.PolicyPublished.Domain
	if r.ReportMetadata.DateRange.Begin > 0 && r.ReportMetadata.DateRange.End > 0 {
		sum.DateRange = fmt.Sprintf("%s - %s", time.Unix(r.ReportMetadata.DateRange.Begin, 0).Format("2006-01-02"), time.Unix(r.ReportMetadata.DateRange.End, 0).Format("2006-01-02"))
	}
	for _, rec := range r.Records {
		c := rec.Row.Count
		sum.Total += c
		pass := 0
		if strings.EqualFold(strings.TrimSpace(rec.PolicyEvaluated.Dkim), "pass") {
			pass++
		}
		if strings.EqualFold(strings.TrimSpace(rec.PolicyEvaluated.Spf), "pass") {
			pass++
		}
		if pass == 2 && !strings.EqualFold(strings.TrimSpace(rec.PolicyEvaluated.Disposition), "reject") {
			sum.Pass += c
		} else {
			sum.Failed += c
		}
	}
	return &sum
}

func parseDMARCFromZip(b []byte) *dmarcSummary {
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil
	}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, _ := io.ReadAll(rc)
		_ = rc.Close()
		if strings.HasSuffix(strings.ToLower(f.Name), ".xml") {
			if sum := parseDMARCXML(data); sum != nil {
				return sum
			}
		}
		if strings.HasSuffix(strings.ToLower(f.Name), ".gz") {
			if sum := parseDMARCFromGzip(data); sum != nil {
				return sum
			}
		}
	}
	return nil
}

func parseDMARCFromGzip(b []byte) *dmarcSummary {
	gr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil
	}
	defer gr.Close()
	data, _ := io.ReadAll(gr)
	return parseDMARCXML(data)
}

func extractBouncedRecipients(s string) []string {
	out := []string{}
	lines := strings.Split(s, "\n")
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(strings.ToLower(ln), "final-recipient:") || strings.HasPrefix(strings.ToLower(ln), "original-recipient:") {
			parts := strings.Split(ln, ";")
			if len(parts) >= 2 {
				addr := strings.TrimSpace(parts[len(parts)-1])
				if isEmail(addr) {
					out = append(out, addr)
				}
			}
		}
	}
	return uniqueStrings(out)
}

func uniqueStrings(a []string) []string {
	m := map[string]struct{}{}
	out := make([]string, 0, len(a))
	for _, v := range a {
		v = strings.ToLower(strings.TrimSpace(v))
		if _, ok := m[v]; ok || v == "" {
			continue
		}
		m[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func isEmail(s string) bool {
	if s == "" {
		return false
	}
	at := strings.IndexByte(s, '@')
	return at > 0 && at < len(s)-1
}

func suppressEmail(db *gorm.DB, addr, reason string, dur time.Duration) bool {
	if !isEmail(addr) {
		return false
	}
	until := time.Now().Add(dur)
	rec := EmailSuppression{Email: addr, SuspendedUntil: until}
	if strings.TrimSpace(reason) != "" {
		rec.Reason = &reason
	}
	if err := db.Create(&rec).Error; err != nil {
		// try update existing
		var ex EmailSuppression
		if tx := db.Where("email = ?", addr).First(&ex); tx.Error == nil {
			ex.SuspendedUntil = until
			ex.Reason = &reason
			_ = db.Save(&ex).Error
			return true
		}
		return false
	}
	return true
}

func sendDiscordEmbeds(embeds []map[string]any) {
	url := env("DISCORD_WEBHOOK_URL", "")
	if strings.TrimSpace(url) == "" {
		return
	}
	payload := map[string]any{
		"embeds": embeds,
	}
	b, _ := json.Marshal(payload)
	_ = httpPostJSON(url, b)
}

func httpPostJSON(url string, body []byte) error {
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return nil
}

func mustOpenDB() *gorm.DB {
	host := env("DB_HOST", "db")
	port := env("DB_PORT", "3306")
	dbname := env("MYSQL_DATABASE", "taskly")
	user := env("MYSQL_USER", "taskly")
	pass := env("MYSQL_PASSWORD", "taskly_password")
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4,utf8&loc=Local", user, pass, host, port, dbname)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("db connect error: %v", err)
	}
	return db
}

func initSentry() {
	dsn := env("SENTRY_DSN", "")
	if strings.TrimSpace(dsn) == "" {
		return
	}
	_ = sentry.Init(sentry.ClientOptions{Dsn: dsn, Environment: env("SENTRY_ENVIRONMENT", "development")})
	sentry.CaptureMessage("dmarc-processor starting")
	go func() {
		for {
			sentry.Flush(2 * time.Second)
			time.Sleep(10 * time.Second)
		}
	}()
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
