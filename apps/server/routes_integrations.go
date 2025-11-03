package main

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gofiber/fiber/v2"
)

func registerIntegrationRoutes(app *fiber.App) {
	// Unified Teams integration secret header verification helper
	requireTeamsSecret := func(c *fiber.Ctx) bool {
		sec := c.Get("x-teams-integration-secret")
		return sec != "" && sec == env("TEAMS_INTEGRATION_SECRET", "change_me")
	}

	// Callback endpoint for Power Automate to report posting result
	app.Post("/integrations/teams/callback", func(c *fiber.Ctx) error {
		if !requireTeamsSecret(c) {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		// Accept task_id as string or number
		var raw map[string]any
		if err := c.BodyParser(&raw); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var p struct {
			IsSuccess      bool
			TaskID         uint
			MessageID      string
			ConversationID string
		}
		if v, ok := raw["is_success"].(bool); ok {
			p.IsSuccess = v
		}
		if tid, ok := raw["task_id"]; ok {
			switch tv := tid.(type) {
			case string:
				if n, err := strconv.ParseUint(strings.TrimSpace(tv), 10, 64); err == nil {
					p.TaskID = uint(n)
				}
			case float64:
				if tv >= 0 {
					p.TaskID = uint(tv)
				}
			default:
				// ignore
			}
		}
		if v, ok := raw["message_id"].(string); ok {
			p.MessageID = v
		}
		if v, ok := raw["conversation_id"].(string); ok {
			p.ConversationID = v
		}
		if p.TaskID != 0 {
			_ = DB.Table("tasks_teams_logs").Create(map[string]any{
				"task_id":    p.TaskID,
				"is_success": p.IsSuccess,
				"teams_message_id": func() *string {
					if p.MessageID == "" {
						return nil
					}
					v := p.MessageID
					return &v
				}(),
				"teams_conversation_id": func() *string {
					if p.ConversationID == "" {
						return nil
					}
					v := p.ConversationID
					return &v
				}(),
				"last_status": "callback_received",
			}).Error
		}
		return c.SendStatus(fiber.StatusOK)
	})
	app.Post("/integrations/teams/status", func(c *fiber.Ctx) error {
		if !requireTeamsSecret(c) {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		// store minimal log; accept task_id string or number
		var raw map[string]any
		_ = c.BodyParser(&raw)
		var p struct {
			IsSuccess      bool
			TaskID         uint
			MessageID      string
			ConversationID string
		}
		if v, ok := raw["is_success"].(bool); ok {
			p.IsSuccess = v
		}
		if tid, ok := raw["task_id"]; ok {
			switch tv := tid.(type) {
			case string:
				if n, err := strconv.ParseUint(strings.TrimSpace(tv), 10, 64); err == nil {
					p.TaskID = uint(n)
				}
			case float64:
				if tv >= 0 {
					p.TaskID = uint(tv)
				}
			}
		}
		if v, ok := raw["message_id"].(string); ok {
			p.MessageID = v
		}
		if v, ok := raw["conversation_id"].(string); ok {
			p.ConversationID = v
		}
		if p.TaskID != 0 {
			_ = DB.Table("tasks_teams_logs").Create(map[string]any{
				"task_id":    p.TaskID,
				"is_success": p.IsSuccess,
				"teams_message_id": func() *string {
					if p.MessageID == "" {
						return nil
					}
					v := p.MessageID
					return &v
				}(),
				"teams_conversation_id": func() *string {
					if p.ConversationID == "" {
						return nil
					}
					v := p.ConversationID
					return &v
				}(),
				"last_status": "callback_received",
			}).Error
		}
		return c.SendStatus(fiber.StatusOK)
	})

	// Teams inbound + async processing
	// Simple buffered channel acts as a queue for demo purposes
	var teamsQueue = make(chan map[string]any, 100)
	// Worker goroutine
	go func() {
		for job := range teamsQueue {
			// Extract data
			title, _ := job["title"].(string)
			desc, _ := job["description"].(*string)
			addedUserID, _ := job["added_user_id"].(uint)
			subjectID, _ := job["subject_id"].(uint)
			callback, _ := job["status_callback"].(*string)
			// Create task
			rec := Task{AddedUserID: addedUserID, Title: title, SubjectID: subjectID, Description: desc}
			var isSuccess bool
			if err := DB.Create(&rec).Error; err != nil {
				isSuccess = false
			} else {
				isSuccess = true
			}
			// Log into tasks_teams_logs (initial)
			_ = DB.Table("tasks_teams_logs").Create(map[string]any{
				"task_id":    rec.ID,
				"is_success": isSuccess,
				"callback_url": func() *string {
					if callback == nil {
						return nil
					}
					return callback
				}(),
				"attempts": 0,
				"last_status": func() string {
					if isSuccess {
						return "created"
					} else {
						return "create_failed"
					}
				}(),
			}).Error
			// Callback with retries if provided
			if callback != nil && *callback != "" {
				cbURL := *callback
				payload := fmt.Sprintf(`{"is_success":%t,"task_id":%d}`, isSuccess, rec.ID)
				secret := env("TEAMS_INTEGRATION_SECRET", "")
				// HMAC signature
				var sig string
				if secret != "" {
					mac := hmac.New(sha256.New, []byte(secret))
					mac.Write([]byte(payload))
					sig = fmt.Sprintf("sha256=%x", mac.Sum(nil))
				}
				client := &http.Client{Timeout: 10 * time.Second}
				attempts := 0
				for attempts < 3 {
					attempts++
					req, _ := http.NewRequest("POST", cbURL, strings.NewReader(payload))
					req.Header.Set("Content-Type", "application/json")
					if sig != "" {
						req.Header.Set("X-Signature", sig)
					}
					resp, err := client.Do(req)
					if err != nil {
						_ = DB.Table("tasks_teams_logs").Where("task_id = ?", rec.ID).Updates(map[string]any{
							"attempts":    attempts,
							"last_status": "callback_error",
							"last_error":  err.Error(),
						}).Error
						time.Sleep(time.Duration(attempts) * time.Second)
						continue
					}
					bodyBytes := make([]byte, 0)
					if resp.Body != nil {
						bodyBytes, _ = io.ReadAll(resp.Body)
						_ = resp.Body.Close()
					}
					_ = DB.Table("tasks_teams_logs").Where("task_id = ?", rec.ID).Updates(map[string]any{
						"attempts":             attempts,
						"last_status":          "callback_sent",
						"last_response_status": resp.StatusCode,
						"last_response_body":   string(bodyBytes),
					}).Error
					if resp.StatusCode >= 200 && resp.StatusCode < 300 {
						break
					}
					time.Sleep(time.Duration(attempts) * time.Second)
				}
			}
		}
	}()

	app.Post("/integrations/teams/inbound", func(c *fiber.Ctx) error {
		if !requireTeamsSecret(c) {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		var body struct {
			Email          string  `json:"email"`
			Title          string  `json:"title"`
			Description    *string `json:"description"`
			SubjectID      uint    `json:"subject_id"`
			StatusCallback *string `json:"status_callback"`
		}
		if err := c.BodyParser(&body); err != nil || body.Email == "" || body.Title == "" || body.SubjectID == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		// Resolve user by email
		var u User
		if err := DB.Where("email = ?", body.Email).First(&u).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "user not found"})
		}
		// Enqueue job
		job := map[string]any{
			"title":         body.Title,
			"description":   body.Description,
			"added_user_id": u.ID,
			"subject_id":    body.SubjectID,
		}
		if body.StatusCallback != nil {
			job["status_callback"] = body.StatusCallback
		}
		select {
		case teamsQueue <- job:
			return c.SendStatus(202)
		default:
			return c.Status(503).JSON(fiber.Map{"error": "queue full"})
		}
	})

	// Inbound endpoint to mark day shift unfulfilled when an email about 日直 is received via Power Automate
	app.Post("/integrations/dayshift/inbound", func(c *fiber.Ctx) error {
		if !requireTeamsSecret(c) {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		var body struct {
			Subject string `json:"subject"`
			Body    string `json:"body"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if !strings.Contains(body.Subject+" "+body.Body, "日直") {
			return c.Status(400).JSON(fiber.Map{"error": "not a dayshift notice"})
		}
		// Use Gemini to decide whether to execute rotation/delay
		doRotate := shouldRotateByGemini(body.Subject, body.Body)
		if !doRotate {
			return c.Status(202).JSON(fiber.Map{"ok": true, "rotated": false})
		}
		target := dateOnly(time.Now())
		var asg DayShiftAssignment
		if err := DB.Where("date = ?", target.Format("2006-01-02")).First(&asg).Error; err != nil {
			// create if missing within current cycle if any
			cyc, _ := getCurrentCycle(target)
			asg = DayShiftAssignment{Date: dateOnly(target)}
			if cyc != nil {
				asg.CycleID = cyc.ID
			}
			asg.Status = "unfulfilled"
			if err := DB.Create(&asg).Error; err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "db"})
			}
		} else {
			asg.Status = "unfulfilled"
			_ = DB.Save(&asg).Error
		}
		// Shift today's users to next business day and push rest by one day
		if err := performDayShiftRotation(target); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "rotation"})
		}
		// Notify Discord via embeds (async)
		go notifyDayShiftRotationDiscord(target)
		return c.Status(202).JSON(fiber.Map{"ok": true, "rotated": true})
	})

	// Cloudflare Email Workers inbound (DMARC, bounces, etc.)
	app.Post("/integrations/email/inbound", func(c *fiber.Ctx) error {
		// Auth via header
		sec := c.Get("X-Email-Integration-Secret")
		if strings.TrimSpace(sec) == "" || sec != env("EMAIL_INTEGRATION_SECRET", "change_me_email_inbound_secret") {
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
		if err := parseJSONStrict(c, &body); err != nil {
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
				if suppressEmail(addr, "bounced", 7*24*time.Hour) {
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
		go sendDiscordEmbeds(embeds)
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
				if isEmail(addr) {
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

func suppressEmail(addr, reason string, dur time.Duration) bool {
	if !isEmail(addr) {
		return false
	}
	until := time.Now().Add(dur)
	rec := EmailSuppression{Email: addr, SuspendedUntil: until}
	if strings.TrimSpace(reason) != "" {
		rec.Reason = &reason
	}
	if err := DB.Create(&rec).Error; err != nil {
		// on duplicate, ignore
		return false
	}
	return true
}
