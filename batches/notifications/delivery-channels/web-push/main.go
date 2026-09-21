package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	webpush "github.com/SherClockHolmes/webpush-go"
	dcCommon "github.com/asari194617/taskly/batches/notifications/delivery-channels/common"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	dsn := dcCommon.BuildDSN(dcCommon.MySQLConfig{
		User:     os.Getenv("MYSQL_USER"),
		Password: os.Getenv("MYSQL_PASSWORD"),
		Host:     os.Getenv("DB_HOST"),
		Port:     os.Getenv("DB_PORT"),
		Database: os.Getenv("MYSQL_DATABASE"),
	})
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

	// Prefer DB-stored VAPID keys when available, falling back to env.
	vapidPublic, vapidPrivate, vapidSubject := loadVAPIDFromDB(db)
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
			"url":   dcCommon.ValueOr(os.Getenv("WEB_URL"), "https://app.taskly.local"),
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

func fetchPendingWebPushQueue(db *sql.DB) ([]dcCommon.QueueItem, error) {
	// notifications_queue.user_id を起点に、以下の条件でフィルタ:
	// - blocks によるブロック中ユーザーを除外
	// - notification_type_prefs で該当 type が is_enabled=0 のユーザーを除外
	//   (レコードがない場合は配信OK)
	// - notification_devices で WEBPUSH デバイスが is_enabled=1 のユーザーに限定
	//   (レコードがないユーザーには配信しない)
	// - web_push_subscriptions に有効な購読があるユーザーに限定
	// DISTINCT prevents multiple devices/subscriptions from duplicating a queue item;
	// fetchUserSubscriptions fans that item out to each subscription exactly once.
	rows, err := db.Query(`
		SELECT DISTINCT q.id, q.when_label, q.subject, q.title, q.body, q.user_id
		FROM notifications_queue q
		JOIN users u ON u.id = q.user_id
		LEFT JOIN blocks b
		  ON b.user_id = u.id
		 AND (b.expires_at IS NULL OR b.expires_at > NOW())
		LEFT JOIN notification_type_prefs ntp
		  ON ntp.user_id = u.id
		 AND ntp.notification_type IN ('TASK','DAYSHIFT')
		LEFT JOIN notification_devices nd
		  ON nd.user_id = u.id
		 AND nd.platform = 'WEBPUSH'
		 AND nd.is_enabled = 1
		JOIN web_push_subscriptions wps
		  ON wps.user_id = u.id
		WHERE q.processed_at IS NULL
		  AND q.channel = 'webpush'
		  AND COALESCE(u.is_deleted, 0) = 0
		  AND b.id IS NULL
		  AND (
		    ntp.id IS NULL
		    OR (ntp.notification_type = 'TASK' AND ntp.is_enabled = 1 AND q.kind = 'task')
		    OR (ntp.notification_type = 'DAYSHIFT' AND ntp.is_enabled = 1 AND q.kind = 'dayshift')
		  )
		  AND nd.id IS NOT NULL
		ORDER BY q.id ASC
		LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []dcCommon.QueueItem
	for rows.Next() {
		var it dcCommon.QueueItem
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
	// Unsubscribe deletes the subscription row, so there is no revocation flag
	// in the canonical schema to filter on here.
	rows, err := db.Query(`SELECT endpoint, p256dh, auth FROM web_push_subscriptions WHERE user_id = ?`, userID.Int64)
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

// loadVAPIDFromDB loads keys from the canonical table, then the legacy GORM
// table during migration, and finally the environment.
func loadVAPIDFromDB(db *sql.DB) (pub, priv, subj string) {
	for _, table := range []string{"web_push_vapid", "web_push_v_api_ds"} {
		row := db.QueryRow(`SELECT public_key, private_key FROM ` + table + ` ORDER BY id DESC LIMIT 1`)
		var p, s string
		if err := row.Scan(&p, &s); err == nil && strings.TrimSpace(p) != "" && strings.TrimSpace(s) != "" {
			pub = p
			priv = s
			subj = dcCommon.ValueOr(os.Getenv("VAPID_SUBJECT"), "mailto:noreply@taskly.local")
			return
		}
	}
	priv = os.Getenv("VAPID_PRIVATE_KEY")
	pub = os.Getenv("VAPID_PUBLIC_KEY")
	subj = dcCommon.ValueOr(os.Getenv("VAPID_SUBJECT"), "mailto:noreply@taskly.local")
	return
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
