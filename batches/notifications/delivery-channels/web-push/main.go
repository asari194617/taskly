package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	webpush "github.com/SherClockHolmes/webpush-go"
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

type WebPushSubscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

func main() {
	dsn := os.Getenv("MYSQL_USER") + ":" + os.Getenv("MYSQL_PASSWORD") + "@tcp(" + hostPort(os.Getenv("DB_HOST"), os.Getenv("DB_PORT")) + ")/" + os.Getenv("MYSQL_DATABASE") + "?parseTime=true&charset=utf8mb4&loc=Local"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	items, err := fetchPendingWebPushQueue(db)
	if err != nil {
		log.Fatal(err)
	}
	if len(items) == 0 {
		log.Println("no pending notifications for webpush")
		return
	}

	vapidPrivate := os.Getenv("VAPID_PRIVATE_KEY")
	vapidPublic := os.Getenv("VAPID_PUBLIC_KEY")
	vapidSubject := valueOr(os.Getenv("VAPID_SUBJECT"), "mailto:noreply@taskly.local")
	if strings.TrimSpace(vapidPrivate) == "" || strings.TrimSpace(vapidPublic) == "" {
		log.Println("missing VAPID keys; skipping web push")
		return
	}

	opts := &webpush.Options{Subscriber: vapidSubject, VAPIDPublicKey: vapidPublic, VAPIDPrivateKey: vapidPrivate}
	sent := 0
	for _, it := range items {
		subs, err := fetchUserSubscriptions(db, it.UserID)
		if err != nil {
			log.Printf("subs error id=%d: %v\n", it.ID, err)
			_ = markQueueError(db, it.ID, err)
			continue
		}
		if len(subs) == 0 {
			log.Printf("no subs for id=%d\n", it.ID)
			_ = markQueueSkipped(db, it.ID, "no subscription")
			continue
		}

		payload, _ := json.Marshal(map[string]string{
			"title": fmt.Sprintf("[%s] %s", it.WhenLabel, it.Subject),
			"body":  it.Title + "\n" + it.Body,
			"tag":   "taskly",
			"url":   valueOr(os.Getenv("WEB_URL"), "https://app.taskly.local"),
		})

		for _, s := range subs {
			resp, err := webpush.SendNotification(payload, &webpush.Subscription{Endpoint: s.Endpoint, Keys: webpush.Keys{P256dh: s.P256dh, Auth: s.Auth}}, opts)
			if err != nil {
				log.Printf("webpush send error id=%d: %v\n", it.ID, err)
				continue
			}
			_ = resp.Body.Close()
		}
		sent++
		_ = markQueueProcessed(db, it.ID)
	}
	log.Printf("webpush notifications sent: %d\n", sent)
	// Cleanup: remove processed rows to keep queue temporary
	if _, err := db.Exec(`DELETE FROM notifications_queue WHERE processed_at IS NOT NULL`); err != nil {
		log.Println("cleanup processed queue:", err)
	}
}

func fetchPendingWebPushQueue(db *sql.DB) ([]QueueItem, error) {
	rows, err := db.Query(`SELECT id, when_label, subject, title, body, user_id FROM notifications_queue WHERE processed_at IS NULL AND (channel = 'webpush') ORDER BY id ASC LIMIT 200`)
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

func fetchUserSubscriptions(db *sql.DB, userID sql.NullInt64) ([]WebPushSubscription, error) {
	if !userID.Valid || userID.Int64 == 0 {
		return nil, nil
	}
	rows, err := db.Query(`SELECT endpoint, p256dh, auth FROM web_push_subscriptions WHERE user_id = ? AND (is_revoked IS NULL OR is_revoked = 0)`, userID.Int64)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WebPushSubscription
	for rows.Next() {
		var s WebPushSubscription
		if err := rows.Scan(&s.Endpoint, &s.P256dh, &s.Auth); err != nil {
			continue
		}
		out = append(out, s)
	}
	return out, nil
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

func markQueueSkipped(db *sql.DB, id int64, reason string) error {
	if len(reason) > 2000 {
		reason = reason[:2000]
	}
	_, err := db.Exec(`UPDATE notifications_queue SET processed_at = NOW(), status='skipped', error_message=? WHERE id = ?`, reason, id)
	return err
}

func hostPort(h, p string) string {
	if strings.TrimSpace(p) == "" {
		p = "3306"
	}
	return fmt.Sprintf("%s:%s", valueOr(h, "localhost"), p)
}

func valueOr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
