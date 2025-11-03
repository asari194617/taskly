package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Task struct {
	ID                uint
	AddedUserID       uint
	SubjectID         uint
	SubjectName       string
	Type              string
	Title             string
	Description       *string
	Deadline          time.Time
	DeadlineSince     string
	DeadlineUntil     string
	IsRequireHandIn   bool
	SelfCheckRequired bool
}

type userDevicePrefs struct{ Email, WebPush bool }

func main() {
	webhook := strings.TrimSpace(os.Getenv("DISCORD_WEBHOOK_URL"))
	if webhook == "" {
		log.Println("DISCORD_WEBHOOK_URL not set; skipping discord delivery")
		return
	}

	dsn := os.Getenv("MYSQL_USER") + ":" + os.Getenv("MYSQL_PASSWORD") + "@tcp(" + hostPort(os.Getenv("DB_HOST"), os.Getenv("DB_PORT")) + ")/" + os.Getenv("MYSQL_DATABASE") + "?parseTime=true&charset=utf8mb4&loc=Local"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	now := time.Now()
	if tz := strings.TrimSpace(os.Getenv("TIMEZONE")); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			now = now.In(loc)
		}
	}

	tasks, err := fetchDueTasks(db, now)
	if err != nil {
		log.Fatal(err)
	}
	if len(tasks) == 0 {
		log.Println("no due tasks; skip discord")
		return
	}

	// Prefs
	uids, devicePrefs, typePrefs := loadEligibleUsersAndPrefs(db)

	// Build embed items from tasks
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
			// prefix badges in square brackets
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

	// For Discord, we send one message with multiple embeds per batch to avoid flooding.
	// But to respect per-user prefs, only send if there is at least one eligible recipient (TASK+Email/Webpush). Here we check any email or webpush enabled.
	anyEligible := false
	for _, uid := range uids {
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
		return
	}

	payload := discordEmbedsPayload(items)
	if err := postDiscord(webhook, payload); err != nil {
		log.Fatal(err)
	}
	log.Printf("discord delivered %d embeds", len(items))
}

func discordEmbedsPayload(items []map[string]any) map[string]any {
	embeds := make([]map[string]any, 0, len(items))
	color := 0x3B82F6 // blue
	for _, it := range items {
		embeds = append(embeds, map[string]any{
			"title":       it["title"],
			"description": fmt.Sprintf("%s\n%s", it["when"], it["meta"]),
			"color":       color,
		})
	}
	return map[string]any{
		"content": "期限が近い課題のお知らせ",
		"embeds":  embeds,
	}
}

func postDiscord(webhook string, body map[string]any) error {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", webhook, strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	cli := &http.Client{Timeout: 10 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord webhook http %d", resp.StatusCode)
	}
	return nil
}

func fetchDueTasks(db *sql.DB, now time.Time) ([]Task, error) {
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
	end := time.Date(now.Year(), now.Month(), now.Day()+2, 23, 59, 59, 0, now.Location())
	rows, err := db.Query(
		"SELECT t.id, t.added_user_id, t.subject_id, COALESCE(s.name_ja, '') AS subject_name, t.type, t.title, t.description, t.deadline, t.deadline_since, t.deadline_until, t.is_require_hand_in, t.self_check_required "+
			"FROM tasks t LEFT JOIN subjects s ON s.id = t.subject_id WHERE "+eff+" BETWEEN ? AND ?",
		start, end,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Task, 0, 256)
	for rows.Next() {
		var t Task
		var desc sql.NullString
		if err := rows.Scan(&t.ID, &t.AddedUserID, &t.SubjectID, &t.SubjectName, &t.Type, &t.Title, &desc, &t.Deadline, &t.DeadlineSince, &t.DeadlineUntil, &t.IsRequireHandIn, &t.SelfCheckRequired); err != nil {
			continue
		}
		if desc.Valid {
			t.Description = &desc.String
		}
		out = append(out, t)
	}
	return out, nil
}

func loadEligibleUsersAndPrefs(db *sql.DB) ([]uint, map[uint]userDevicePrefs, map[uint]map[string]bool) {
	ids := make([]uint, 0, 1024)
	device := make(map[uint]userDevicePrefs, 1024)
	typePrefs := make(map[uint]map[string]bool, 1024)
	rows, err := db.Query(`SELECT id, is_deleted FROM users`)
	if err != nil {
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
		typePrefs[id] = map[string]bool{"TASK": true, "DAYSHIFT": true}
	}
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

func isUserBlocked(db *sql.DB, uid uint) bool {
	row := db.QueryRow(`SELECT COUNT(1) FROM blocks WHERE user_id = ? AND (expires_at IS NULL OR expires_at > NOW())`, uid)
	var cnt int
	if err := row.Scan(&cnt); err != nil {
		return false
	}
	return cnt > 0
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
func isNextMonth(a, b time.Time) bool { return a.Month() != b.Month() && a.Year() == b.Year() }
func isNextYear(a, b time.Time) bool  { return a.Year() != b.Year() }

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
	default:
		return 0
	}
}

func buildBadges(t Task) []string {
	out := make([]string, 0, 3)
	if lbl := displayTaskTypeJaLocal(t.Type); strings.TrimSpace(lbl) != "" && strings.ToUpper(strings.TrimSpace(t.Type)) != "OTHER" {
		out = append(out, lbl)
	}
	if t.IsRequireHandIn {
		out = append(out, "提出必須")
	}
	if t.SelfCheckRequired {
		out = append(out, "自己採点")
	}
	return out
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
