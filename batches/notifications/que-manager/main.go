package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Task struct {
	ID                uint
	AddedUserID       uint
	Title             string
	Description       *string
	Deadline          time.Time
	DeadlineSince     string
	DeadlineUntil     string
	IsRequireHandIn   bool
	SelfCheckRequired bool
}

type DayShift struct {
	Date time.Time
}

// Notification item for downstream delivery channels
// Channel-agnostic payload
// kind: "task" | "dayshift"
// whenLabel: 今日/あす/あさって/日付表記
// subject: 表示名（教科や日直）
// title: 件名相当
// body: 本文相当
// userID: 宛先ユーザー

type NotifyItem struct {
	Kind      string
	WhenLabel string
	Subject   string
	Title     string
	Body      string
	UserID    uint
}

func main() {
	// Ensure queue table exists
	// Note: This creates a minimal table used by batch notifications.
	// If your server migrations already create it, this is a no-op.
	// Safe to run repeatedly.
	dsn := os.Getenv("MYSQL_USER") + ":" + os.Getenv("MYSQL_PASSWORD") + "@tcp(" + hostPort(os.Getenv("DB_HOST"), os.Getenv("DB_PORT")) + ")/" + os.Getenv("MYSQL_DATABASE") + "?parseTime=true&charset=utf8mb4&loc=Local"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := ensureQueueTable(db); err != nil {
		log.Fatalf("ensure notifications_queue: %v", err)
	}

	// Prefer TZ from env (e.g., Asia/Tokyo)
	now := time.Now()
	if tz := strings.TrimSpace(os.Getenv("TIMEZONE")); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			now = now.In(loc)
		}
	}
	items := make([]NotifyItem, 0, 256)

	// 期限が近い課題（今日〜明日までの「実際の締切時刻」）を抽出
	// サーバーと同じロジック（deadline + deadline_until の合成）で絞り込む
	eff := "TIMESTAMP(deadline, CASE " +
		"WHEN deadline_until='MORNING' THEN '07:59:59' " +
		"WHEN deadline_until='P1' THEN '10:30:00' " +
		"WHEN deadline_until='P2' THEN '12:10:00' " +
		"WHEN deadline_until='P3' THEN '14:30:00' " +
		"WHEN deadline_until='P4' THEN '16:10:00' " +
		"WHEN deadline_until='P5' THEN '17:50:00' " +
		"WHEN deadline_until='P6' THEN '19:30:00' " +
		"WHEN deadline_until='P7' THEN '21:10:00' " +
		"WHEN deadline_until='P8' THEN '22:50:00' " +
		"ELSE '23:59:59' END)"
	start := now
	end := time.Date(now.Year(), now.Month(), now.Day()+1, 23, 59, 59, 0, now.Location())
	rows, err := db.Query(
		"SELECT id, added_user_id, title, description, deadline, deadline_since, deadline_until, is_require_hand_in, self_check_required "+
			"FROM tasks WHERE "+eff+" BETWEEN ? AND ?",
		start, end,
	)
	if err != nil {
		log.Println("query tasks (effective window):", err)
	} else {
		for rows.Next() {
			var t Task
			var desc sql.NullString
			if err := rows.Scan(&t.ID, &t.AddedUserID, &t.Title, &desc, &t.Deadline, &t.DeadlineSince, &t.DeadlineUntil, &t.IsRequireHandIn, &t.SelfCheckRequired); err != nil {
				continue
			}
			if desc.Valid {
				t.Description = &desc.String
			}
			// ラベルは日付ベース（今日/あす）
			label := whenLabel(now, t.Deadline)
			if isUserDeleted(db, t.AddedUserID) || isUserBlocked(db, t.AddedUserID) {
				continue
			}
			items = append(items, NotifyItem{
				Kind:      "task",
				WhenLabel: label,
				Subject:   "課題",
				Title:     t.Title,
				Body:      fmt.Sprintf("%s まで", deadlineLabel(t, t.Deadline)),
				UserID:    t.AddedUserID,
			})
		}
		rows.Close()
	}

	// 日直（前日/当日）
	for _, d := range []time.Time{businessPrev(now), now} {
		row := db.QueryRow(`SELECT date FROM day_shift_assignments WHERE date = ?`, dateOnly(d).Format("2006-01-02"))
		var date time.Time
		if err := row.Scan(&date); err == nil {
			items = append(items, NotifyItem{
				Kind:      "dayshift",
				WhenLabel: whenLabel(now, date),
				Subject:   "日直",
				Title:     "日直の担当日です",
				Body:      "本日は日直の担当日です。よろしくお願いします。",
				UserID:    0, // 配信チャネル側で担当者に展開する
			})
		}
	}

	// キューに投入（共通テーブル）: 各チャネルごとに投入（email, webpush）
	for _, it := range items {
		// email
		if _, err := db.Exec(`INSERT INTO notifications_queue (kind, when_label, subject, title, body, user_id, channel, created_at) VALUES (?,?,?,?,?,?, 'email', NOW())`, it.Kind, it.WhenLabel, it.Subject, it.Title, it.Body, it.UserID); err != nil {
			log.Println("enqueue email:", err)
		}
		// webpush
		if _, err := db.Exec(`INSERT INTO notifications_queue (kind, when_label, subject, title, body, user_id, channel, created_at) VALUES (?,?,?,?,?,?, 'webpush', NOW())`, it.Kind, it.WhenLabel, it.Subject, it.Title, it.Body, it.UserID); err != nil {
			log.Println("enqueue webpush:", err)
		}
	}
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

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func businessPrev(d time.Time) time.Time {
	x := d.Add(-24 * time.Hour)
	for i := 0; i < 7; i++ {
		if x.Weekday() != time.Saturday && x.Weekday() != time.Sunday {
			return dateOnly(x)
		}
		x = x.Add(-24 * time.Hour)
	}
	return dateOnly(d.Add(-24 * time.Hour))
}

func isNextMonth(a, b time.Time) bool {
	return a.Month() != b.Month() && a.Year() == b.Year()
}
func isNextYear(a, b time.Time) bool { return a.Year() != b.Year() }

func whenLabel(base, target time.Time) string {
	b := dateOnly(base)
	t := dateOnly(target)
	if t.Equal(b) {
		return "今日"
	}
	if t.Equal(b.Add(24 * time.Hour)) {
		return "あす"
	}
	if t.Equal(b.Add(48 * time.Hour)) {
		return "あさって"
	}
	if isNextYear(b, t) {
		return t.Format("2006年01月02日")
	}
	if isNextMonth(b, t) {
		return t.Format("01月02日")
	}
	return t.Format("02日")
}

func deadlineLabel(t Task, d time.Time) string {
	// 例: 10/16 5限まで, 朝まで, など
	date := d.Format("1/2")
	label := deriveDeadlineLabelEnum(t.DeadlineSince, t.DeadlineUntil)
	if strings.TrimSpace(label) == "" {
		return date + " まで"
	}
	return date + " " + label
}

func deriveDeadlineLabelEnum(since, until string) string {
	sIdx := periodIndex(since)
	uIdx := periodIndex(until)
	if strings.EqualFold(until, "MORNING") {
		return "朝まで"
	}
	if uIdx > 0 && (sIdx == 0 || sIdx == uIdx) {
		return fmt.Sprintf("%d限まで", uIdx)
	}
	if sIdx > 0 && uIdx > 0 {
		if sIdx == uIdx {
			return fmt.Sprintf("%d限まで", uIdx)
		}
		if sIdx < uIdx {
			return fmt.Sprintf("%d〜%d限まで", sIdx, uIdx)
		}
	}
	return ""
}

func periodIndex(s string) int {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "P1":
		return 1
	case "P2":
		return 2
	case "P3":
		return 3
	case "P4":
		return 4
	case "P5":
		return 5
	case "P6":
		return 6
	case "P7":
		return 7
	case "P8":
		return 8
	default:
		return 0
	}
}

func isUserDeleted(db *sql.DB, uid uint) bool {
	row := db.QueryRow(`SELECT is_deleted FROM users WHERE id = ?`, uid)
	var del sql.NullBool
	if err := row.Scan(&del); err != nil {
		return false
	}
	return del.Valid && del.Bool
}

func isUserBlocked(db *sql.DB, uid uint) bool {
	row := db.QueryRow(`SELECT COUNT(1) FROM blocks WHERE user_id = ? AND (expires_at IS NULL OR expires_at > NOW())`, uid)
	var cnt int
	if err := row.Scan(&cnt); err != nil {
		return false
	}
	return cnt > 0
}

// targetDates returns business dates for notifications: today, next biz day, and the following biz day
// Note: business-day helpers removed for tasks; weekendsも含めて締切を判定

func ensureQueueTable(db *sql.DB) error {
	ddl := `CREATE TABLE IF NOT EXISTS notifications_queue (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		kind VARCHAR(32) NOT NULL,
		when_label VARCHAR(32) NOT NULL,
		subject VARCHAR(255) NOT NULL,
		title TEXT,
		body TEXT,
		user_id BIGINT NULL,
		channel VARCHAR(32) NULL,
		status VARCHAR(32) NULL,
		error_message TEXT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		processed_at DATETIME NULL,
		INDEX idx_processed_at (processed_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`
	_, err := db.Exec(ddl)
	return err
}
