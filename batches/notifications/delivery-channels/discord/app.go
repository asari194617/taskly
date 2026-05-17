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

func run() error {
	webhook := strings.TrimSpace(os.Getenv("DISCORD_WEBHOOK_URL"))
	if webhook == "" {
		log.Println("DISCORD_WEBHOOK_URL not set; skipping discord delivery")
		return nil
	}

	dsn := dcCommon.BuildDSN(dcCommon.MySQLConfig{
		User:     os.Getenv("MYSQL_USER"),
		Password: os.Getenv("MYSQL_PASSWORD"),
		Host:     os.Getenv("DB_HOST"),
		Port:     os.Getenv("DB_PORT"),
		Database: os.Getenv("MYSQL_DATABASE"),
	})
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	now := time.Now()
	if loc, err := time.LoadLocation("Asia/Tokyo"); err == nil {
		now = now.In(loc)
	}

	tasks, err := dcCommon.FetchDueTasks(db, now)
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		log.Println("no due tasks; skip discord")
		return nil
	}

	uIds, devicePrefs, typePrefs := dcCommon.LoadEligibleUsersAndPrefs(db, dcCommon.UserDevicePrefs{Email: true, WebPush: false}, map[string]bool{"TASK": true, "DAYSHIFT": true})

	items := make([]map[string]any, 0, len(tasks))
	for _, tsk := range tasks {
		subj := strings.TrimSpace(tsk.SubjectName)
		if subj == "" {
			subj = "課題"
		}
		wl := whenLabel(now, tsk.Deadline)
		meta := fmt.Sprintf("%s %s", subj, deadlineLabel(tsk, tsk.Deadline))
		badges := buildBadges(tsk)
		title := tsk.Title
		if len(badges) > 0 {
			prefix := ""
			for _, b := range badges {
				prefix += "[" + b + "]"
			}
			title = prefix + " " + title
		}
		items = append(items, map[string]any{
			"when":  wl,
			"title": title,
			"meta":  meta,
		})
	}

	anyEligible := false
	for _, uid := range uIds {
		if tp, ok := typePrefs[uid]; ok {
			if !tp["TASK"] {
				continue
			}
		}
		pf := devicePrefs[uid]
		if pf.Email || pf.WebPush {
			anyEligible = true
			break
		}
	}
	if !anyEligible {
		log.Println("no eligible users for TASK; skipping discord")
		return nil
	}

	payload := discordEmbedsPayload(items)
	if err := postDiscord(webhook, payload); err != nil {
		return err
	}
	log.Printf("discord delivered %d embeds", len(items))
	return nil
}
