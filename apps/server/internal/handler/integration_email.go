package handler

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/getsentry/sentry-go"
	"github.com/gofiber/fiber/v2"
	"io"
	"os"
	"strings"
	"time"
)

func (h *Handler) registerEmailIntegrationRoutes(router fiber.Router) {
	// Cloudflare Email Workers inbound (DMARC, bounces, etc.)
	router.Post("/email/inbound", func(c *fiber.Ctx) error {
		// Auth via header
		sec := c.Get("X-Email-Integration-Secret")
		if strings.TrimSpace(sec) == "" || sec != os.Getenv("EMAIL_INTEGRATION_SECRET") {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		// Expected JSON body
		var body struct {
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
		if err := utils.ParseJSONStrict(c, &body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		// Capture minimal event to Sentry
		sentry.CaptureMessage(fmt.Sprintf("[EmailInbound] from=%s to=%s subject=%s", body.From, body.To, body.Subject))

		// Detect DMARC aggregate reports and/or DSN bounces
		// 1) Try DMARC XML from attachments (possibly zipped/gzipped)
		var dmarc *dmarcSummary
		for _, att := range body.Attachments {
			name := strings.ToLower(att.Filename)
			ctype := strings.ToLower(att.ContentType)
			if !(strings.HasSuffix(name, ".xml") || strings.HasSuffix(name, ".zip") || strings.HasSuffix(name, ".gz") || strings.Contains(ctype, "xml") || strings.Contains(ctype, "zip") || strings.Contains(ctype, "gzip")) {
				continue
			}
			raw, _ := base64.StdEncoding.DecodeString(att.ContentB64)
			// if zip
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

		// 2) Try DSN bounce parsing from headers/text
		suppressed := []string{}
		if dmarc == nil { // unrelated to DMARC aggregate
			candidates := extractBouncedRecipients(body.Text + "\n" + body.Html)
			for _, addr := range candidates {
				if h.suppressEmail(addr, "bounced", 7*24*time.Hour) {
					suppressed = append(suppressed, addr)
				}
			}
		}

		// Build Discord embeds
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
			// generic info embed
			embeds = append(embeds, map[string]any{
				"title":       "Inbound Email Received",
				"description": fmt.Sprintf("from=%s to=%s subject=%s", body.From, body.To, body.Subject),
				"color":       0x95A5A6,
				"timestamp":   time.Now().UTC().Format(time.RFC3339),
			})
		}
		go utils.SendDiscordEmbeds(embeds)
		return c.SendStatus(202)
	})
}

// --- Email inbound helpers ---
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
		// Final-Recipient: rfc822; user@example.com
		if strings.HasPrefix(strings.ToLower(ln), "final-recipient:") || strings.HasPrefix(strings.ToLower(ln), "original-recipient:") {
			parts := strings.Split(ln, ";")
			if len(parts) >= 2 {
				addr := strings.TrimSpace(parts[len(parts)-1])
				if utils.IsEmail(addr) {
					out = append(out, addr)
				}
			}
		}
		// Common phrasing
		if strings.Contains(strings.ToLower(ln), "delivery to the following recipient failed") {
			continue
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

func (h *Handler) suppressEmail(addr, reason string, dur time.Duration) bool {
	if !utils.IsEmail(addr) {
		return false
	}
	until := time.Now().Add(dur)
	rec := utils.EmailSuppression{Email: addr, SuspendedUntil: until}
	if strings.TrimSpace(reason) != "" {
		rec.Reason = &reason
	}
	if err := h.db.Create(&rec).Error; err != nil {
		// on duplicate, ignore
		return false
	}
	return true
}
