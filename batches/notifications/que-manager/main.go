package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	dcCommon "github.com/asari194617/taskly/batches/notifications/delivery-channels/common"
	_ "github.com/go-sql-driver/mysql"
)

type Task = dcCommon.Task

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
	TaskID    uint
	Kind      string
	WhenLabel string
	Subject   string
	Title     string
	Body      string
	UserID    uint // optional; when zero and Kind=task we expand to all users by prefs
	Meta      string
	Badges    string // JSON array string
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

	queueSchema, err := loadNotificationsQueueSchema(db)
	if err != nil {
		log.Fatal(err)
	}

	// Always use Asia/Tokyo (JST)
	now := time.Now()
	if loc, err := time.LoadLocation("Asia/Tokyo"); err == nil {
		now = now.In(loc)
	}
	items := make([]NotifyItem, 0, 256)

	// 期限が近い課題（今日〜2日後の終日）を抽出
	tasks, err := dcCommon.FetchDueTasks(db, now)
	if err != nil {
		log.Fatal("query tasks (effective window): ", err)
	}
	for _, t := range tasks {
		// ラベルは日付ベース（今日/あす）
		label := whenLabel(now, t.Deadline)
		subj := strings.TrimSpace(t.SubjectName)
		if subj == "" {
			subj = "課題"
		}
		// メタ（科目 + 期限ラベル）
		deadline := deadlineLabel(t, t.Deadline)
		meta := fmt.Sprintf("%s %s", subj, deadline)
		// バッジJSON
		badges := buildBadgesJSON(t)
		items = append(items, NotifyItem{
			TaskID:    t.ID,
			Kind:      "task",
			WhenLabel: label,
			Subject:   subj,
			Title:     t.Title,
			Body:      deadline,
			UserID:    0, // expand to all users by preferences below
			Meta:      meta,
			Badges:    badges,
		})
	}

	// Resolve eligible users and preferences
	userIDs, devicePrefs, typePrefs, err := dcCommon.LoadEligibleUsersAndPrefs(db,
		dcCommon.UserDevicePrefs{Email: true, WebPush: false},
		map[string]bool{"TASK": true, "TASK_MORNING": true, "TASK_EVENING": true, "DAYSHIFT": true})
	if err != nil {
		log.Fatal("load notification preferences: ", err)
	}
	// Determine delivery slot (morning vs evening) to respect per-time preferences
	isMorning := now.Hour() < 12
	// キューに投入（共通テーブル）: 各チャネルごとに投入（email, webpush）
	for _, it := range items {
		// tasks: expand to all users
		if it.Kind == "task" && it.UserID == 0 {
			for _, uid := range userIDs {
				// type preference: TASK default ON + per-time gating
				if tp, ok := typePrefs[uid]; ok {
					if !tp["TASK"] {
						continue
					}
					// Morning/evening sub-flags are optional; default true when absent
					if isMorning {
						if v, ok := tp["TASK_MORNING"]; ok && !v {
							continue
						}
					} else {
						if v, ok := tp["TASK_EVENING"]; ok && !v {
							continue
						}
					}
				}
				pf := devicePrefs[uid]
				// Do not enqueue email digests; they will be built live at delivery time
				if pf.WebPush {
					if err := enqueueNotification(db, queueSchema, it, uid, now); err != nil {
						log.Fatal("enqueue webpush: ", err)
					}
				}
			}
			continue
		}
		// fallback: enqueue for specific user (e.g., dayshift future expansion)
		if it.UserID != 0 {
			// respect per-type preference
			typeKey := "TASK"
			if strings.EqualFold(it.Kind, "dayshift") {
				typeKey = "DAYSHIFT"
			}
			if tp, ok := typePrefs[it.UserID]; ok {
				if !tp[typeKey] {
					continue
				}
			}
			pf := devicePrefs[it.UserID]
			// Do not enqueue email digests; they will be built live at delivery time
			if pf.WebPush {
				if err := enqueueNotification(db, queueSchema, it, it.UserID, now); err != nil {
					log.Fatal("enqueue webpush: ", err)
				}
			}
		}
	}
}

type notificationsQueueSchema struct {
	hasMeta       bool
	hasBadgesJSON bool
}

func loadNotificationsQueueSchema(db *sql.DB) (notificationsQueueSchema, error) {
	meta, err := tableColumnExists(db, "notifications_queue", "meta")
	if err != nil {
		return notificationsQueueSchema{}, err
	}
	badges, err := tableColumnExists(db, "notifications_queue", "badges_json")
	if err != nil {
		return notificationsQueueSchema{}, err
	}
	return notificationsQueueSchema{hasMeta: meta, hasBadgesJSON: badges}, nil
}

func tableColumnExists(db *sql.DB, tableName, columnName string) (bool, error) {
	row := db.QueryRow(
		`SELECT COUNT(1) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
		tableName,
		columnName,
	)
	var count int
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func enqueueNotification(db *sql.DB, schema notificationsQueueSchema, item NotifyItem, userID uint, now time.Time) error {
	columns := []string{"kind", "when_label", "subject", "title", "body"}
	values := []any{item.Kind, item.WhenLabel, item.Subject, item.Title, item.Body}

	if schema.hasMeta {
		columns = append(columns, "meta")
		values = append(values, item.Meta)
	}
	if schema.hasBadgesJSON {
		columns = append(columns, "badges_json")
		values = append(values, item.Badges)
	}

	slot := "evening"
	if now.Hour() < 12 {
		slot = "morning"
	}
	key := fmt.Sprintf("task:%s:%s:%d:webpush:%d", now.Format("2006-01-02"), slot, item.TaskID, userID)
	columns = append(columns, "user_id", "channel", "event_key", "slot", "created_at")
	values = append(values, userID, "webpush", key, slot, now)

	placeholders := strings.Repeat("?,", len(columns))
	placeholders = strings.TrimSuffix(placeholders, ",")
	query := fmt.Sprintf("INSERT INTO notifications_queue (%s) VALUES (%s) ON DUPLICATE KEY UPDATE id=id", strings.Join(columns, ", "), placeholders)
	_, err := db.Exec(query, values...)
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
	date := d.Format("1/2")
	label := deriveDeadlineLabelEnum(t.DeadlineSince, t.DeadlineUntil)
	if label == "" {
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
	if strings.EqualFold(until, "LUNCH") {
		return "昼休みまで"
	}
	if strings.EqualFold(until, "AFTER_SCHOOL") {
		return "放課後まで"
	}
	if uIdx > 0 && (sIdx == 0 || sIdx == uIdx) {
		return fmt.Sprintf("%d限まで", uIdx)
	}
	if sIdx > 0 && uIdx > 0 && sIdx < uIdx {
		return fmt.Sprintf("%d〜%d限まで", sIdx, uIdx)
	}
	return ""
}

func periodIndex(s string) int {
	s = strings.ToUpper(strings.TrimSpace(s))
	if strings.HasPrefix(s, "P") {
		if num, err := strconv.Atoi(s[1:]); err == nil && num >= 1 {
			return num
		}
	}
	return 0
}

// buildBadgesJSON builds badges JSON based on task attributes
func buildBadgesJSON(t Task) string {
	type badge struct {
		Label   string `json:"label"`
		Variant string `json:"variant,omitempty"`
	}
	out := make([]badge, 0, 3)
	if lbl := displayTaskTypeJaLocal(t.Type); strings.TrimSpace(lbl) != "" && strings.ToUpper(strings.TrimSpace(t.Type)) != "OTHER" {
		out = append(out, badge{Label: lbl, Variant: "gray"})
	}
	if t.IsRequireHandIn {
		out = append(out, badge{Label: "提出必須", Variant: "red"})
	}
	if t.IsSelfCheckRequired {
		out = append(out, badge{Label: "自己採点", Variant: "blue"})
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func displayTaskTypeJaLocal(e string) string {
	switch strings.ToUpper(strings.TrimSpace(e)) {
	case "ONLINE":
		return "オンライン"
	case "FACE_TO_FACE":
		return "対面"
	case "QUIZ":
		return "小テスト"
	case "PRESENTATION":
		return "発表"
	case "SELF_STUDY":
		return "自主学習"
	default:
		return ""
	}
}
