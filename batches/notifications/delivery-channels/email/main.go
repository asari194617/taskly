package main

import (
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

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
	now := time.Now()
	// Use TIMEZONE if provided
	if tz := strings.TrimSpace(os.Getenv("TIMEZONE")); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			now = now.In(loc)
		}
	}
	tasks, err := fetchDueTasks(db, now)
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return map[int64][]QueueItem{}, nil
	}
	// Eligible users and prefs
	uids, devicePrefs, typePrefs := loadEligibleUsersAndPrefs(db)
	// Build common items once
	baseItems := make([]QueueItem, 0, len(tasks))
	for _, tsk := range tasks {
		subj := strings.TrimSpace(tsk.SubjectName)
		if subj == "" {
			subj = "課題"
		}
		wl := whenLabel(now, tsk.Deadline)
		meta := fmt.Sprintf("%s %s", subj, deadlineLabel(tsk, tsk.Deadline))
		badges := buildBadgesJSON(tsk)
		// Body should contain only the task description (if any), not the deadline label
		desc := ""
		if tsk.Description != nil {
			desc = strings.TrimSpace(*tsk.Description)
		}
		baseItems = append(baseItems, QueueItem{
			WhenLabel: wl,
			Subject:   subj,
			Title:     tsk.Title,
			Body:      desc,
			Meta:      sql.NullString{String: meta, Valid: true},
			Badges:    sql.NullString{String: badges, Valid: strings.TrimSpace(badges) != ""},
		})
	}
	// Expand to per-user groups
	groups := make(map[int64][]QueueItem, len(uids))
	for _, uid := range uids {
		// Type preference: TASK must be enabled
		if tp, ok := typePrefs[uid]; ok {
			if !tp["TASK"] {
				continue
			}
		}
		// Device preference: email must be enabled
		if pf := devicePrefs[uid]; !pf.Email {
			continue
		}
		// assign a copy (to keep future extensibility per-user)
		arr := make([]QueueItem, len(baseItems))
		copy(arr, baseItems)
		groups[int64(uid)] = arr
	}
	return groups, nil
}

// fetchDueTasks returns tasks whose effective deadline is within [now, end of the day after tomorrow]
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
			"FROM tasks t LEFT JOIN subjects s ON s.id = t.subject_id WHERE "+eff+" BETWEEN ? AND ? ORDER BY "+eff+" ASC",
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
		typePrefs[id] = map[string]bool{"TASK": true, "DAYSHIFT": true}
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
	if t.SelfCheckRequired {
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
	// Resolve recipient
	to, err := resolveRecipientEmail(db, sql.NullInt64{Int64: uid, Valid: true})
	if err != nil {
		return fmt.Errorf("resolve: %w", err)
	}
	if to == "" {
		return fmt.Errorf("no recipient")
	}

	host := valueOr(os.Getenv("SMTP_HOST"), "localhost")
	port := valueOr(os.Getenv("SMTP_PORT"), "25")
	from := valueOr(os.Getenv("SMTP_FROM"), "noreply@taskly.local")
	user := valueOr(os.Getenv("SMTP_USER"), from)
	pass := os.Getenv("SMTP_PASSWORD")
	addr := fmt.Sprintf("%s:%s", host, port)

	// Compose via email-compose HTTP
	composeAddr := valueOr(os.Getenv("EMAIL_COMPOSE_HTTP_ADDR"), "http://email-compose:50052")
	locale := valueOr(os.Getenv("LOCALE"), "ja")
	appName := valueOr(os.Getenv("APP_NAME"), "Taskly")
	payload := buildComposePayload(locale, appName, group)
	subj, text, html, err := callComposeTaskDigest(composeAddr, payload)
	if err != nil {
		return fmt.Errorf("compose: %w", err)
	}

	header := make(map[string]string)
	header["From"] = from
	header["To"] = to
	header["Subject"] = subj
	header["Date"] = time.Now().Format(time.RFC1123Z)
	header["MIME-Version"] = "1.0"
	boundary := fmt.Sprintf("==TASKLY_%d==", time.Now().UnixNano())
	header["Content-Type"] = fmt.Sprintf("multipart/alternative; boundary=%s", boundary)

	var msg strings.Builder
	for k, v := range header {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n")
	// text part
	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	msg.WriteString(text)
	msg.WriteString("\r\n")
	// html part
	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	msg.WriteString(html)
	msg.WriteString("\r\n")
	msg.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	// AUTH if password is provided, otherwise attempt unauthenticated
	var auth smtp.Auth
	if strings.TrimSpace(pass) != "" {
		auth = smtp.PlainAuth("", user, pass, host)
	}

	// optional TLS (skip verify for internal relay like postfix)
	c, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer c.Close()
	_ = c.Hello("localhost")
	if ok, _ := c.Extension("STARTTLS"); ok {
		config := &tls.Config{InsecureSkipVerify: true, ServerName: host}
		if err := c.StartTLS(config); err != nil {
			log.Println("starttls:", err)
		}
	}
	if auth != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err := c.Auth(auth); err != nil {
				log.Println("auth:", err)
			}
		}
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(msg.String())); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
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

func callComposeTaskDigest(base string, payload composePayload) (subject, text, html string, err error) {
	url := strings.TrimRight(base, "/") + "/compose/task-digest"
	body, _ := jsonMarshal(payload)
	req, _ := http.NewRequest("POST", url, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	cli := &http.Client{Timeout: 10 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", "", "", fmt.Errorf("compose http %d: %s", resp.StatusCode, string(b))
	}
	var out struct {
		Subject string `json:"subject"`
		Text    string `json:"text"`
		Html    string `json:"html"`
	}
	dec := jsonNewDecoder(resp.Body)
	if err := dec.Decode(&out); err != nil {
		return "", "", "", err
	}
	return out.Subject, out.Text, out.Html, nil
}

// Small wrappers to avoid importing full encoding/json symbol names everywhere
func jsonMarshal(v any) ([]byte, error)        { return json.Marshal(v) }
func jsonNewDecoder(r io.Reader) *json.Decoder { return json.NewDecoder(r) }

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
