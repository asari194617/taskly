package main

import (
	"crypto/sha256"
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

	items, err := fetchPendingWebPushQueue(db, 0)
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
		log.Fatal("missing VAPID keys")
	}

	opts := &webpush.Options{Subscriber: vapidSubject, VAPIDPublicKey: vapidPublic, VAPIDPrivateKey: vapidPrivate}
	sent := 0
	failed := 0
	lastID := int64(0)
	for len(items) > 0 {
		for _, it := range items {
			lastID = it.ID
			eligible, err := webPushEligible(db, it)
			if err != nil {
				log.Printf("eligibility error id=%d: %v", it.ID, err)
				failed++
				continue
			}
			if !eligible {
				if err := markQueueSkipped(db, it.ID, "notification setting disabled or user blocked"); err != nil {
					failed++
				}
				continue
			}
			subs, err := fetchUserSubscriptions(db, it.UserID)
			if err != nil {
				log.Printf("subs error id=%d: %v\n", it.ID, err)
				_ = markQueueError(db, it.ID, err)
				failed++
				continue
			}
			if len(subs) == 0 {
				log.Printf("no subs for id=%d\n", it.ID)
				_ = markQueueSkipped(db, it.ID, "no subscription")
				continue
			}

			payload, err := json.Marshal(map[string]string{
				"title": fmt.Sprintf("[%s] %s", it.WhenLabel, it.Subject),
				"body":  it.Title + "\n" + it.Body,
				"tag":   fmt.Sprintf("taskly-%d", it.ID),
				"url":   dcCommon.ValueOr(os.Getenv("WEB_URL"), "https://app.taskly.local"),
			})
			if err != nil {
				_ = markQueueError(db, it.ID, err)
				failed++
				continue
			}

			itemFailed := false
			for _, s := range subs {
				key := fmt.Sprintf("%s:sub:%x", notificationKey(it), sha256.Sum256([]byte(s.Endpoint)))
				claimed, err := claimWebPushDelivery(db, key, it.UserID.Int64)
				if err != nil {
					log.Printf("claim error id=%d: %v", it.ID, err)
					itemFailed = true
					continue
				}
				if !claimed {
					var status string
					if err := db.QueryRow(`SELECT status FROM notification_deliveries WHERE delivery_key=?`, key).Scan(&status); err != nil || status != "sent" {
						itemFailed = true
					}
					continue
				}
				resp, err := webpush.SendNotification(payload, &webpush.Subscription{Endpoint: s.Endpoint, Keys: webpush.Keys{P256dh: s.P256dh, Auth: s.Auth}}, opts)
				if err == nil {
					_ = resp.Body.Close()
					if resp.StatusCode < 200 || resp.StatusCode >= 300 {
						err = fmt.Errorf("webpush HTTP %d", resp.StatusCode)
					}
				}
				if recordErr := finishWebPushDelivery(db, key, err); recordErr != nil {
					log.Printf("record delivery id=%d: %v", it.ID, recordErr)
					itemFailed = true
				}
				if err != nil {
					log.Printf("webpush send error id=%d: %v\n", it.ID, err)
					itemFailed = true
				}
			}
			if itemFailed {
				failed++
				_ = markQueueError(db, it.ID, fmt.Errorf("one or more subscriptions failed"))
				continue
			}
			sent++
			if err := markQueueProcessed(db, it.ID); err != nil {
				failed++
			}
		}
		items, err = fetchPendingWebPushQueue(db, lastID)
		if err != nil {
			log.Fatal(err)
		}
	}
	log.Printf("webpush notifications sent: %d\n", sent)
	if failed > 0 {
		log.Fatalf("webpush notifications failed: %d", failed)
	}
}

func fetchPendingWebPushQueue(db *sql.DB, afterID int64) ([]dcCommon.QueueItem, error) {
	rows, err := db.Query(`
		SELECT q.id, q.kind, q.event_key, q.slot, q.when_label, q.subject, q.title, q.body, q.user_id
		FROM notifications_queue q
		WHERE q.processed_at IS NULL
		  AND q.channel = 'webpush'
		  AND q.id > ?
		ORDER BY q.id ASC
		LIMIT 200`, afterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []dcCommon.QueueItem
	for rows.Next() {
		var it dcCommon.QueueItem
		if err := rows.Scan(&it.ID, &it.Kind, &it.EventKey, &it.Slot, &it.WhenLabel, &it.Subject, &it.Title, &it.Body, &it.UserID); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func fetchUserSubscriptions(db *sql.DB, userID sql.NullInt64) ([]WebPushSubscription, error) {
	if !userID.Valid || userID.Int64 == 0 {
		return nil, nil
	}
	// Unsubscribe deletes the subscription row, so there is no revocation flag
	// in the canonical schema to filter on here.
	rows, err := db.Query(`SELECT DISTINCT wps.endpoint, wps.p256dh, wps.auth
		FROM web_push_subscriptions wps
		JOIN notification_devices d ON d.user_id=wps.user_id
		  AND d.platform='WEBPUSH' AND d.is_enabled=1
		  AND JSON_UNQUOTE(d.payload)=wps.endpoint
		WHERE wps.user_id = ?`, userID.Int64)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WebPushSubscription
	for rows.Next() {
		var s WebPushSubscription
		if err := rows.Scan(&s.Endpoint, &s.P256dh, &s.Auth); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
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
	_, err := db.Exec(`UPDATE notifications_queue SET processed_at = NOW(), status='sent', error_message=NULL WHERE id = ?`, id)
	return err
}

func markQueueError(db *sql.DB, id int64, e error) error {
	msg := e.Error()
	if len(msg) > 2000 {
		msg = msg[:2000]
	}
	_, err := db.Exec(`UPDATE notifications_queue SET status='error', error_message=? WHERE id = ?`, msg, id)
	return err
}

func markQueueSkipped(db *sql.DB, id int64, reason string) error {
	if len(reason) > 2000 {
		reason = reason[:2000]
	}
	_, err := db.Exec(`UPDATE notifications_queue SET processed_at = NOW(), status='skipped', error_message=? WHERE id = ?`, reason, id)
	return err
}
