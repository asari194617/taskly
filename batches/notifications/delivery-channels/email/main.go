package main

import (
	"crypto/tls"
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
)

type QueueItem struct {
	ID        int64
	WhenLabel string
	Subject   string
	Title     string
	Body      string
	UserID    sql.NullInt64
}

func main() {
	dsn := os.Getenv("MYSQL_USER") + ":" + os.Getenv("MYSQL_PASSWORD") + "@tcp(" + hostPort(os.Getenv("DB_HOST"), os.Getenv("DB_PORT")) + ")/" + os.Getenv("MYSQL_DATABASE") + "?parseTime=true&charset=utf8mb4&loc=Local"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	items, err := fetchPendingEmailQueue(db)
	if err != nil {
		log.Fatal(err)
	}
	if len(items) == 0 {
		log.Println("no pending notifications for email")
		return
	}

	// Group by user and send digest per user
	groups := map[int64][]QueueItem{}
	for _, it := range items {
		if !it.UserID.Valid || it.UserID.Int64 == 0 {
			// skip items without a direct recipient for now
			continue
		}
		groups[it.UserID.Int64] = append(groups[it.UserID.Int64], it)
	}

	totalSent := 0
	for uid, group := range groups {
		if err := deliverGroupEmail(db, uid, group); err != nil {
			log.Printf("deliver group failed uid=%d: %v\n", uid, err)
			// mark errors on each item
			for _, it := range group {
				_ = markQueueError(db, it.ID, err)
			}
			continue
		}
		totalSent++
		for _, it := range group {
			_ = markQueueProcessed(db, it.ID)
		}
	}
	log.Printf("email notifications sent (groups): %d\n", totalSent)
	// Cleanup: remove processed rows to keep queue temporary
	if _, err := db.Exec(`DELETE FROM notifications_queue WHERE processed_at IS NOT NULL`); err != nil {
		log.Println("cleanup processed queue:", err)
	}
}

func fetchPendingEmailQueue(db *sql.DB) ([]QueueItem, error) {
	rows, err := db.Query(`SELECT id, when_label, subject, title, body, user_id FROM notifications_queue WHERE processed_at IS NULL AND (channel IS NULL OR channel='email') ORDER BY id ASC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []QueueItem
	for rows.Next() {
		var it QueueItem
		if err := rows.Scan(&it.ID, &it.WhenLabel, &it.Subject, &it.Title, &it.Body, &it.UserID); err != nil {
			continue
		}
		out = append(out, it)
	}
	return out, nil
}

func deliverGroupEmail(db *sql.DB, uid int64, group []QueueItem) error {
	// Resolve recipient
	to, err := resolveRecipientEmail(db, sql.NullInt64{Int64: uid, Valid: true})
	if err != nil {
		return fmt.Errorf("resolve: %w", err)
	}
	if to == "" {
		return fmt.Errorf("no recipient")
	}

	host := valueOr(os.Getenv("SMTP_HOST"), "localhost")
	port := valueOr(os.Getenv("SMTP_PORT"), "25")
	from := valueOr(os.Getenv("SMTP_FROM"), "noreply@taskly.local")
	user := valueOr(os.Getenv("SMTP_USER"), from)
	pass := os.Getenv("SMTP_PASSWORD")
	addr := fmt.Sprintf("%s:%s", host, port)

	// Compose via email-compose HTTP
	composeAddr := valueOr(os.Getenv("EMAIL_COMPOSE_HTTP_ADDR"), "http://email-compose:50052")
	locale := valueOr(os.Getenv("LOCALE"), "ja")
	appName := valueOr(os.Getenv("APP_NAME"), "Taskly")
	payload := buildComposePayload(locale, appName, group)
	subj, text, html, err := callComposeTaskDigest(composeAddr, payload)
	if err != nil {
		return fmt.Errorf("compose: %w", err)
	}

	header := make(map[string]string)
	header["From"] = from
	header["To"] = to
	header["Subject"] = subj
	header["Date"] = time.Now().Format(time.RFC1123Z)
	header["MIME-Version"] = "1.0"
	boundary := fmt.Sprintf("==TASKLY_%d==", time.Now().UnixNano())
	header["Content-Type"] = fmt.Sprintf("multipart/alternative; boundary=%s", boundary)

	var msg strings.Builder
	for k, v := range header {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n")
	// text part
	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	msg.WriteString(text)
	msg.WriteString("\r\n")
	// html part
	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	msg.WriteString(html)
	msg.WriteString("\r\n")
	msg.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	// AUTH if password is provided, otherwise attempt unauthenticated
	var auth smtp.Auth
	if strings.TrimSpace(pass) != "" {
		auth = smtp.PlainAuth("", user, pass, host)
	}

	// optional TLS (skip verify for internal relay like postfix)
	c, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer c.Close()
	_ = c.Hello("localhost")
	if ok, _ := c.Extension("STARTTLS"); ok {
		config := &tls.Config{InsecureSkipVerify: true, ServerName: host}
		if err := c.StartTLS(config); err != nil {
			log.Println("starttls:", err)
		}
	}
	if auth != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err := c.Auth(auth); err != nil {
				log.Println("auth:", err)
			}
		}
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(msg.String())); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

type composePayload struct {
	Locale  string              `json:"locale"`
	AppName string              `json:"app_name"`
	Items   []map[string]string `json:"items"`
}

func buildComposePayload(locale, appName string, items []QueueItem) composePayload {
	out := composePayload{Locale: locale, AppName: appName}
	for _, it := range items {
		out.Items = append(out.Items, map[string]string{
			"when_label": it.WhenLabel,
			"subject":    it.Subject,
			"title":      it.Title,
			"body":       it.Body,
		})
	}
	return out
}

func callComposeTaskDigest(base string, payload composePayload) (subject, text, html string, err error) {
	url := strings.TrimRight(base, "/") + "/compose/task-digest"
	body, _ := jsonMarshal(payload)
	req, _ := http.NewRequest("POST", url, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	cli := &http.Client{Timeout: 10 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", "", "", fmt.Errorf("compose http %d: %s", resp.StatusCode, string(b))
	}
	var out struct {
		Subject string `json:"subject"`
		Text    string `json:"text"`
		Html    string `json:"html"`
	}
	dec := jsonNewDecoder(resp.Body)
	if err := dec.Decode(&out); err != nil {
		return "", "", "", err
	}
	return out.Subject, out.Text, out.Html, nil
}

// Small wrappers to avoid importing full encoding/json symbol names everywhere
func jsonMarshal(v any) ([]byte, error)        { return json.Marshal(v) }
func jsonNewDecoder(r io.Reader) *json.Decoder { return json.NewDecoder(r) }

func resolveRecipientEmail(db *sql.DB, userID sql.NullInt64) (string, error) {
	// If userID is null or 0, skip (no direct recipient). In future: expand to assignment owners.
	if !userID.Valid || userID.Int64 == 0 {
		return "", nil
	}
	row := db.QueryRow(`SELECT email FROM users WHERE id = ? AND is_deleted = 0`, userID.Int64)
	var email sql.NullString
	if err := row.Scan(&email); err != nil {
		return "", err
	}
	if !email.Valid {
		return "", nil
	}
	return email.String, nil
}

func markQueueProcessed(db *sql.DB, id int64) error {
	_, err := db.Exec(`UPDATE notifications_queue SET processed_at = NOW(), status='sent' WHERE id = ?`, id)
	return err
}

func markQueueError(db *sql.DB, id int64, e error) error {
	msg := e.Error()
	if len(msg) > 2000 {
		msg = msg[:2000]
	}
	_, err := db.Exec(`UPDATE notifications_queue SET processed_at = NOW(), status='error', error_message=? WHERE id = ?`, msg, id)
	return err
}

func hostPort(h, p string) string {
	if strings.TrimSpace(p) == "" {
		p = "25"
	}
	return fmt.Sprintf("%s:%s", valueOr(h, "localhost"), p)
}

func valueOr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
