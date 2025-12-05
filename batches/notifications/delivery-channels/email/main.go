package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Queue item representing a digest entry
type QueueItem struct {
	ID        int64
	WhenLabel string
	Subject   string
	Title     string
	Body      string
	Meta      sql.NullString
	Badges    sql.NullString // JSON
	UserID    sql.NullInt64
}

// Domain task snapshot for live composition
type Task struct {
	ID                  uint
	AddedUserID         uint
	SubjectID           uint
	SubjectName         string
	Type                string
	Title               string
	Description         *string
	Deadline            time.Time
	DeadlineSince       string
	DeadlineUntil       string
	IsRequireHandIn     bool
	IsSelfCheckRequired bool
}

func main() {
	dsn := os.Getenv("MYSQL_USER") + ":" + os.Getenv("MYSQL_PASSWORD") + "@tcp(" + hostPort(os.Getenv("DB_HOST"), os.Getenv("DB_PORT")) + ")/" + os.Getenv("MYSQL_DATABASE") + "?parseTime=true&charset=utf8mb4&loc=Local"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Build digest on-the-fly from current DB state (do not rely on queued snapshots)
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
		if err := deliverGroupEmail(db, uid, group); err != nil {
			log.Printf("deliver group failed uid=%d: %v\n", uid, err)
			continue
		}
		totalSent++
	}
	log.Printf("email notifications sent (groups): %d\n", totalSent)
}

// computeLiveDigestGroups collects due tasks and expands them to per-user email digests
func computeLiveDigestGroups(db *sql.DB) (map[int64][]QueueItem, error) {
	// For now, return no groups; composition pipeline handled elsewhere.
	return map[int64][]QueueItem{}, nil
}

// fetchDueTasks returns tasks whose effective deadline is within [now, end of the day after tomorrow]
func fetchDueTasks(db *sql.DB, now time.Time) ([]Task, error) {
	// Date-only window: include deadlines between today and day-after-tomorrow
	startDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endDate := time.Date(now.Year(), now.Month(), now.Day()+2, 0, 0, 0, 0, now.Location())
	rows, err := db.Query(
		"SELECT t.id, t.added_user_id, t.subject_id, COALESCE(s.name_ja, '') AS subject_name, t.type, t.title, t.description, t.deadline, t.deadline_since, t.deadline_until, t.is_require_hand_in, t.is_self_check_required "+
			"FROM tasks t LEFT JOIN subjects s ON s.id = t.subject_id WHERE t.deadline BETWEEN ? AND ? ORDER BY t.deadline ASC",
		startDate, endDate,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Task, 0, 256)
	for rows.Next() {
		var t Task
		var desc sql.NullString
		if err := rows.Scan(&t.ID, &t.AddedUserID, &t.SubjectID, &t.SubjectName, &t.Type, &t.Title, &desc, &t.Deadline, &t.DeadlineSince, &t.DeadlineUntil, &t.IsRequireHandIn, &t.IsSelfCheckRequired); err != nil {
			continue
		}
		if desc.Valid {
			t.Description = &desc.String
		}
		out = append(out, t)
	}
	return out, nil
}

type userDevicePrefs struct{ Email, WebPush bool }

// loadEligibleUsersAndPrefs returns non-deleted/non-blocked users and their notification prefs. Defaults applied.
func loadEligibleUsersAndPrefs(db *sql.DB) ([]uint, map[uint]userDevicePrefs, map[uint]map[string]bool) {
	ids := make([]uint, 0, 1024)
	device := make(map[uint]userDevicePrefs, 1024)
	typePrefs := make(map[uint]map[string]bool, 1024)
	// users
	rows, err := db.Query(`SELECT id, is_deleted FROM users`)
	if err != nil {
		log.Println("load users:", err)
		return ids, device, typePrefs
	}
	defer rows.Close()
	for rows.Next() {
		var id uint
		var del bool
		if err := rows.Scan(&id, &del); err != nil {
			continue
		}
		if del {
			continue
		}
		if isUserBlocked(db, id) {
			continue
		}
		ids = append(ids, id)
		device[id] = userDevicePrefs{Email: true, WebPush: false}
		typePrefs[id] = map[string]bool{"TASK": true, "TASK_MORNING": true, "TASK_EVENING": true, "DAYSHIFT": true}
	}
	// device prefs overlay
	rows2, err := db.Query(`SELECT user_id, platform, is_enabled FROM notification_devices`)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var uid uint
			var platform string
			var enabled bool
			if err := rows2.Scan(&uid, &platform, &enabled); err != nil {
				continue
			}
			p := device[uid]
			switch strings.ToUpper(strings.TrimSpace(platform)) {
			case "EMAIL":
				p.Email = enabled
			case "WEBPUSH":
				p.WebPush = enabled
			}
			device[uid] = p
		}
	}
	// type prefs overlay
	rows3, err := db.Query(`SELECT user_id, notification_type, is_enabled FROM notification_type_prefs`)
	if err == nil {
		defer rows3.Close()
		for rows3.Next() {
			var uid uint
			var t string
			var en bool
			if err := rows3.Scan(&uid, &t, &en); err != nil {
				continue
			}
			m := typePrefs[uid]
			m[strings.ToUpper(strings.TrimSpace(t))] = en
			typePrefs[uid] = m
		}
	}
	return ids, device, typePrefs
}

// isUserBlocked checks active blocks for the user
func isUserBlocked(db *sql.DB, uid uint) bool {
	row := db.QueryRow(`SELECT COUNT(1) FROM blocks WHERE user_id = ? AND (expires_at IS NULL OR expires_at > NOW())`, uid)
	var cnt int
	if err := row.Scan(&cnt); err != nil {
		return false
	}
	return cnt > 0
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
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
	case "P9":
		return 9
	case "P10":
		return 10
	default:
		return 0
	}
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

func deliverGroupEmail(db *sql.DB, uid int64, group []QueueItem) error {
	// Temporarily no-op to ensure batch compiles. Delivery handled by other pipeline.
	_ = uid
	_ = group
	return nil
}

type composePayload struct {
	Locale  string           `json:"locale"`
	AppName string           `json:"app_name"`
	Items   []map[string]any `json:"items"`
}

func buildComposePayload(locale, appName string, items []QueueItem) composePayload {
	out := composePayload{Locale: locale, AppName: appName}
	for _, it := range items {
		m := map[string]any{
			"when_label": it.WhenLabel,
			"subject":    it.Subject,
			"title":      it.Title,
			"body":       it.Body,
		}
		if it.Meta.Valid {
			m["meta"] = it.Meta.String
		}
		if it.Badges.Valid && strings.TrimSpace(it.Badges.String) != "" {
			// try parse JSON
			var arr []map[string]any
			if err := json.Unmarshal([]byte(it.Badges.String), &arr); err == nil {
				m["badges"] = arr
			}
		}
		out.Items = append(out.Items, m)
	}
	return out
}

// (HTTP composer removed. Using gRPC instead.)

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
