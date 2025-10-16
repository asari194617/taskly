package main

import (
	"crypto/tls"
	"database/sql"
	"fmt"
	"log"
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
	dsn := os.Getenv("DB_USER") + ":" + os.Getenv("DB_PASSWORD") + "@tcp(" + hostPort(os.Getenv("DB_HOST"), os.Getenv("DB_PORT")) + ")/" + os.Getenv("DB_NAME") + "?parseTime=true&charset=utf8mb4&loc=Local"
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

	sent := 0
	for _, it := range items {
		if err := deliverEmail(db, it); err != nil {
			log.Printf("deliver failed id=%d: %v\n", it.ID, err)
			_ = markQueueError(db, it.ID, err)
			continue
		}
		sent++
		_ = markQueueProcessed(db, it.ID)
	}
	log.Printf("email notifications sent: %d\n", sent)
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

func deliverEmail(db *sql.DB, it QueueItem) error {
	// Resolve recipient
	to, err := resolveRecipientEmail(db, it.UserID)
	if err != nil {
		return fmt.Errorf("resolve: %w", err)
	}
	if to == "" {
		return fmt.Errorf("no recipient")
	}

	host := valueOr(os.Getenv("SMTP_HOST"), "localhost")
	port := valueOr(os.Getenv("SMTP_PORT"), "25")
	from := valueOr(os.Getenv("SMTP_FROM"), "noreply@taskly.local")
	pass := os.Getenv("SMTP_PASSWORD")
	addr := fmt.Sprintf("%s:%s", host, port)

	header := make(map[string]string)
	header["From"] = from
	header["To"] = to
	header["Subject"] = fmt.Sprintf("[%s] %s - %s", it.WhenLabel, it.Subject, it.Title)
	header["Date"] = time.Now().Format(time.RFC1123Z)
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/plain; charset=UTF-8"

	var msg strings.Builder
	for k, v := range header {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n")
	msg.WriteString(it.Body)

	// AUTH if password is provided, otherwise attempt unauthenticated
	var auth smtp.Auth
	if strings.TrimSpace(pass) != "" {
		auth = smtp.PlainAuth("", from, pass, host)
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
