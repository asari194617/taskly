package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"google.golang.org/genai"
)

type Task struct {
	ID          uint
	AddedUserID uint
	Title       string
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Admin struct {
	Email string
}

type ModerationResult struct {
	Category string   `json:"category"`
	Severity string   `json:"severity"` // low|medium|high
	Score    *float64 `json:"score,omitempty"`
	Threat   bool     `json:"threat"`
	Reason   string   `json:"reason"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func main() {
	dsn := os.Getenv("MYSQL_USER") + ":" + os.Getenv("MYSQL_PASSWORD") + "@tcp(" + hostPort(os.Getenv("DB_HOST"), os.Getenv("DB_PORT")) + ")/" + os.Getenv("MYSQL_DATABASE") + "?parseTime=true&charset=utf8mb4&loc=Local"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Select tasks created in the last day that are not hidden
	rows, err := db.Query(`SELECT id, added_user_id, title, description, created_at, updated_at FROM tasks WHERE (created_at >= NOW() - INTERVAL 1 DAY OR updated_at >= NOW() - INTERVAL 1 DAY) AND (is_hidden = 0 OR is_hidden IS NULL)`)
	if err != nil {
		log.Printf("query tasks failed (continuing): %v", err)
		return
	}
	defer rows.Close()

	policy := loadPolicy(db)
	var toHide []Task
	var flagged int
	for rows.Next() {
		var t Task
		var desc sql.NullString
		if err := rows.Scan(&t.ID, &t.AddedUserID, &t.Title, &desc, &t.CreatedAt, &t.UpdatedAt); err != nil {
			continue
		}
		if desc.Valid {
			t.Description = &desc.String
		}
		res, raw, modelName := moderateTask(t)
		if res == nil {
			continue
		}
		flagged++
		// Store flagged record
		metaJSON, _ := json.Marshal(map[string]any{"raw": raw})
		snapshot := map[string]any{"title": t.Title, "description": t.Description}
		snapJSON, _ := json.Marshal(snapshot)
		_, _ = db.Exec(`INSERT INTO flagged_contents (task_id, category, severity, score, threat, reason, model, metadata, snapshot, created_at) VALUES (?,?,?,?,?,?,?,?,?,NOW())`,
			t.ID, res.Category, res.Severity, res.Score, res.Threat, nullIfEmpty(res.Reason), modelName, string(metaJSON), string(snapJSON))
		// Auto-hide according to policy
		if shouldAutoHide(res, policy) {
			toHide = append(toHide, t)
		}
	}

	if len(toHide) > 0 {
		ids := make([]any, 0, len(toHide))
		for _, t := range toHide {
			ids = append(ids, t.ID)
		}
		q := "UPDATE tasks SET is_hidden = 1 WHERE id IN (" + placeholders(len(ids)) + ")"
		if _, err := db.Exec(q, ids...); err != nil {
			log.Printf("hide tasks update failed: %v", err)
		}
		// Notify Discord
		notifyDiscord(fmt.Sprintf(":rotating_light: Moderation flagged %d item(s), auto-hidden %d.", flagged, len(toHide)))
		// Email admins using email-compose (HTML)
		admins := listAdmins(db)
		// Prepare highlights (up to 5)
		highs := make([]map[string]any, 0, min(5, len(toHide)))
		for i, t := range toHide {
			if i >= 5 {
				break
			}
			highs = append(highs, map[string]any{"id": t.ID, "title": t.Title})
		}
		comp := composeModerationEmail(len(toHide)+flagged-len(toHide), len(toHide), highs)
		for _, a := range admins {
			if comp != nil {
				_ = smtpSendMultipart(a.Email, comp.Subject, comp.Text, comp.HTML)
			} else {
				_ = smtpSendPlain(a.Email, "[Taskly] Moderation Alert", fmt.Sprintf("%d item(s) were auto-hidden due to potential threats.", len(toHide)))
			}
		}
	} else if flagged > 0 {
		notifyDiscord(fmt.Sprintf("Moderation flagged %d item(s) (no auto-hide).", flagged))
	}
}

func hostPort(h, p string) string {
	if strings.TrimSpace(p) == "" {
		p = "3306"
	}
	if strings.TrimSpace(h) == "" {
		h = "localhost"
	}
	return fmt.Sprintf("%s:%s", h, p)
}

func placeholders(n int) string {
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		parts[i] = "?"
	}
	return strings.Join(parts, ",")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func moderateTask(t Task) (*ModerationResult, string, string) {
	if strings.TrimSpace(os.Getenv("GEMINI_API_KEY")) == "" {
		return nil, "", ""
	}
	// Allow overriding model via env; default to latest spec example
	modelName := strings.TrimSpace(os.Getenv("GEMINI_MODEL"))
	if modelName == "" {
		modelName = "gemini-2.5-flash"
	}
	prompt := "You are a content moderation classifier. Analyze the following content (title and description) for policy violations (harassment, hate, sexual, violence, self-harm, illegal, malware, threats). Respond ONLY in compact JSON with keys: category (string), severity (low|medium|high), score (0..1), threat (true|false), reason (short string)."
	content := fmt.Sprintf("Title: %s\nDescription: %s", t.Title, safeStr(t.Description))

	ctx := context.Background()
	client, err := genai.NewClient(ctx, nil)
	if err != nil {
		return nil, "", modelName
	}
	resp, err := client.Models.GenerateContent(ctx, modelName, []*genai.Content{genai.NewContentFromText(prompt+"\n\n"+content, "")}, nil)
	if err != nil || resp == nil {
		return nil, "", modelName
	}
	raw := resp.Text()
	payload := strings.TrimSpace(raw)
	payload = strings.TrimPrefix(payload, "```json")
	payload = strings.TrimSuffix(payload, "```")
	payload = strings.TrimSpace(payload)
	var res ModerationResult
	if err := json.Unmarshal([]byte(payload), &res); err != nil {
		// Fallback heuristic: if suspicious words present
		s := strings.ToLower(t.Title + "\n" + safeStr(t.Description))
		if strings.Contains(s, "kill") || strings.Contains(s, "threat") || strings.Contains(s, "爆破") || strings.Contains(s, "殺す") {
			sev := "high"
			tr := true
			return &ModerationResult{Category: "threat", Severity: sev, Threat: tr, Reason: "keyword heuristic"}, raw, modelName
		}
		return nil, raw, modelName
	}
	return &res, raw, modelName
}

func safeStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func notifyDiscord(message string) {
	url := strings.TrimSpace(os.Getenv("DISCORD_WEBHOOK_URL"))
	if url == "" {
		return
	}
	b, _ := json.Marshal(map[string]string{"content": message})
	req, _ := http.NewRequest("POST", url, strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Second}
	if resp, err := client.Do(req); err == nil && resp != nil {
		_ = resp.Body.Close()
	}
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func listAdmins(db *sql.DB) []Admin {
	out := []Admin{}
	rows, err := db.Query(`SELECT u.email FROM users u JOIN role_users r ON r.user_id = u.id WHERE r.role = 'admin' AND u.is_deleted = 0`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var a Admin
		if err := rows.Scan(&a.Email); err == nil && strings.TrimSpace(a.Email) != "" {
			out = append(out, a)
		}
	}
	return out
}

type ModerationPolicy struct {
	DefaultAutoHide string            `json:"default_auto_hide"`
	AutoHideByCat   map[string]string `json:"auto_hide_by_category"`
}

func loadPolicy(db *sql.DB) ModerationPolicy {
	p := ModerationPolicy{DefaultAutoHide: "high", AutoHideByCat: map[string]string{"threat": "medium"}}
	row := db.QueryRow(`SELECT value FROM system_settings WHERE ` + "`key`" + ` = 'moderation_policy'`)
	var val string
	if err := row.Scan(&val); err != nil || strings.TrimSpace(val) == "" {
		return p
	}
	var got ModerationPolicy
	if err := json.Unmarshal([]byte(val), &got); err == nil {
		if got.DefaultAutoHide != "" {
			p.DefaultAutoHide = strings.ToLower(strings.TrimSpace(got.DefaultAutoHide))
		}
		if got.AutoHideByCat != nil {
			p.AutoHideByCat = got.AutoHideByCat
		}
	}
	return p
}

func shouldAutoHide(res *ModerationResult, policy ModerationPolicy) bool {
	if res == nil {
		return false
	}
	if res.Threat {
		return true
	}
	sev := strings.ToLower(strings.TrimSpace(res.Severity))
	if sev == "" {
		return false
	}
	th := policy.DefaultAutoHide
	if v, ok := policy.AutoHideByCat[strings.ToLower(strings.TrimSpace(res.Category))]; ok && strings.TrimSpace(v) != "" {
		th = strings.ToLower(strings.TrimSpace(v))
	}
	return rank(sev) >= rank(th)
}

func rank(s string) int {
	switch s {
	case "low":
		return 1
	case "medium":
		return 2
	case "high":
		return 3
	default:
		return 0
	}
}

func smtpSendPlain(to, subject, text string) error {
	smtpHost := valueOr(os.Getenv("SMTP_HOST"), "localhost")
	smtpPort := valueOr(os.Getenv("SMTP_PORT"), "25")
	from := valueOr(os.Getenv("SMTP_FROM"), "noreply@taskly.local")
	smtpUser := valueOr(os.Getenv("SMTP_USER"), from)
	smtpPass := os.Getenv("SMTP_PASSWORD")
	msg := []byte("From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n\r\n" + text + "\r\n")
	var auth smtp.Auth
	if strings.TrimSpace(smtpPass) != "" {
		auth = smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	}
	addr := smtpHost + ":" + smtpPort
	return smtp.SendMail(addr, auth, from, []string{to}, msg)
}

type Composed struct{ Subject, HTML, Text string }

func composeModerationEmail(totalFlagged, autoHidden int, highlights []map[string]any) *Composed {
	addr := strings.TrimSpace(os.Getenv("EMAIL_COMPOSE_HTTP_ADDR"))
	if addr == "" {
		addr = "http://email-compose:50052"
	}
	locale := strings.TrimSpace(os.Getenv("ADMIN_EMAIL_LOCALE"))
	if locale == "" {
		// Default to Japanese per app default; override via ADMIN_EMAIL_LOCALE=en
		locale = "ja"
	}
	reviewURL := strings.TrimSpace(os.Getenv("ADMIN_REVIEW_URL"))
	body := map[string]any{
		"locale":        locale,
		"app_name":      "Taskly",
		"total_flagged": totalFlagged,
		"auto_hidden":   autoHidden,
		"review_url":    reviewURL,
		"highlights":    highlights,
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", strings.TrimRight(addr, "/")+"/compose/moderation-alert", strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp == nil {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out struct{ Subject, Html, HTML, Text string }
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	html := out.HTML
	if html == "" {
		html = out.Html
	}
	return &Composed{Subject: out.Subject, HTML: html, Text: out.Text}
}

func smtpSendMultipart(to, subject, text, html string) error {
	smtpHost := valueOr(os.Getenv("SMTP_HOST"), "localhost")
	smtpPort := valueOr(os.Getenv("SMTP_PORT"), "25")
	from := valueOr(os.Getenv("SMTP_FROM"), "noreply@taskly.local")
	smtpUser := valueOr(os.Getenv("SMTP_USER"), from)
	smtpPass := os.Getenv("SMTP_PASSWORD")
	boundary := fmt.Sprintf("mixed_%d", time.Now().UnixNano())
	headers := "From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n\r\n"
	partText := "--" + boundary + "\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n\r\n" + text + "\r\n"
	partHTML := "--" + boundary + "\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n\r\n" + html + "\r\n"
	closing := "--" + boundary + "--\r\n"
	msg := []byte(headers + partText + partHTML + closing)
	var auth smtp.Auth
	if strings.TrimSpace(smtpPass) != "" {
		auth = smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	}
	addr := smtpHost + ":" + smtpPort
	return smtp.SendMail(addr, auth, from, []string{to}, msg)
}

func valueOr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
