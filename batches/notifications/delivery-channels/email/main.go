package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	dcCommon "github.com/asari194617/taskly/batches/notifications/delivery-channels/common"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	db, err := sql.Open("mysql", dcCommon.BuildDSN(dcCommon.MySQLConfig{
		User:     os.Getenv("MYSQL_USER"),
		Password: os.Getenv("MYSQL_PASSWORD"),
		Host:     os.Getenv("DB_HOST"),
		Port:     os.Getenv("DB_PORT"),
		Database: os.Getenv("MYSQL_DATABASE"),
	}))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	timeoutSec := parseInt(dcCommon.ValueOr(os.Getenv("SMTP_TIMEOUT"), "60"))
	from := os.Getenv("SMTP_FROM")

	mailCfg := &EmailConfig{
		Host:     os.Getenv("SMTP_HOST"),
		Port:     os.Getenv("SMTP_PORT"),
		From:     from,
		User:     dcCommon.ValueOr(os.Getenv("SMTP_USER"), from),
		Password: os.Getenv("SMTP_PASSWORD"),
		Timeout:  time.Duration(timeoutSec) * time.Second,
		UseTLS:   strings.EqualFold(dcCommon.ValueOr(os.Getenv("SMTP_TLS"), "none"), "starttls"),
	}

	mailer := NewSMTPMailer(mailCfg)

	deliverOpts := DeliverOptions{
		AppName: os.Getenv("APP_NAME"),
	}

	now := time.Now()
	if loc, err := time.LoadLocation("Asia/Tokyo"); err == nil {
		now = now.In(loc)
	}
	groups, err := computeLiveDigestGroupsAt(db, now)
	if err != nil {
		log.Fatal(err)
	}
	if len(groups) == 0 {
		log.Println("no eligible email digests to send")
		return
	}

	totalSent := 0
	failed := 0
	slot := "morning"
	if now.Hour() >= 12 {
		slot = "evening"
	}
	for uid, group := range groups {
		key := fmt.Sprintf("task:%s:%s:email:%d", now.Format("2006-01-02"), slot, uid)
		claimed, err := claimDelivery(db, key, "email", uid)
		if err != nil {
			log.Printf("claim delivery failed uid=%d: %v", uid, err)
			failed++
			continue
		}
		if !claimed {
			var status string
			if err := db.QueryRow(`SELECT status FROM notification_deliveries WHERE delivery_key=?`, key).Scan(&status); err != nil || status != "sent" {
				log.Printf("delivery unresolved uid=%d key=%s: %s %v", uid, key, status, err)
				failed++
			}
			continue
		}
		if err := deliverGroupEmail(db, mailer, uid, group, deliverOpts); err != nil {
			log.Printf("deliver group failed uid=%d: %v\n", uid, err)
			if recordErr := finishDelivery(db, key, err); recordErr != nil {
				log.Printf("record delivery failure uid=%d: %v", uid, recordErr)
			}
			failed++
			continue
		}
		if err := finishDelivery(db, key, nil); err != nil {
			log.Printf("record delivery success uid=%d: %v", uid, err)
			failed++
			continue
		}
		totalSent++
	}
	log.Printf("email notifications sent (groups): %d\n", totalSent)
	if failed > 0 {
		log.Fatalf("email notifications failed: %d", failed)
	}
}
