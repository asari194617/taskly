package main

import (
	"database/sql"
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

	groups, err := computeLiveDigestGroups(db)
	if err != nil {
		log.Fatal(err)
	}
	if len(groups) == 0 {
		log.Println("no eligible email digests to send")
		return
	}

	totalSent := 0
	for uid, group := range groups {
		if err := deliverGroupEmail(db, mailer, uid, group, deliverOpts); err != nil {
			log.Printf("deliver group failed uid=%d: %v\n", uid, err)
			continue
		}
		totalSent++
	}
	log.Printf("email notifications sent (groups): %d\n", totalSent)
}
