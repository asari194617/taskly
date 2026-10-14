package utils

import (
	"crypto/tls"
	"fmt"
	"github.com/asari194617/taskly/apps/server/internal/domain"
	"gorm.io/gorm"
	"html"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// SMTP helper to send plain text email (no HTML/multipart)
func smtpSendPlain(to, subject, text string) error {
	smtpHost := Env("SMTP_HOST", "localhost")
	smtpPort := Env("SMTP_PORT", "25")
	from := Env("SMTP_FROM", "noreply@taskly.local")
	smtpUser := Env("SMTP_USER", from)
	smtpPass := Env("SMTP_PASSWORD", "")
	nowRFC := time.Now().Format(time.RFC1123Z)
	msg := []byte(
		"From: " + from + "\r\n" +
			"To: " + to + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"Date: " + nowRFC + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/plain; charset=utf-8\r\n\r\n" + text + "\r\n",
	)
	var auth smtp.Auth
	if smtpPass != "" {
		auth = smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	}
	toSec, _ := strconv.Atoi(Env("SMTP_TIMEOUT", "60"))
	if toSec <= 0 {
		toSec = 60
	}
	// Keep the complete SMTP attempt inside the two-minute job lease. The
	// deadline covers both connection setup and the SMTP conversation.
	if toSec > 90 {
		toSec = 90
	}
	deadline := time.Now().Add(time.Duration(toSec) * time.Second)
	dialer := &net.Dialer{Deadline: deadline}
	mode := strings.ToLower(strings.TrimSpace(Env("SMTP_TLS", "")))
	var conn net.Conn
	var err error
	if mode == "smtps" || smtpPort == "465" {
		// SMTPS (implicit TLS) - enforce certificate verification
		conn, err = tls.DialWithDialer(dialer, "tcp", smtpHost+":"+smtpPort, &tls.Config{ServerName: smtpHost, InsecureSkipVerify: false})
	} else {
		conn, err = dialer.Dial("tcp", smtpHost+":"+smtpPort)
	}
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(deadline)
	cl, err := smtp.NewClient(conn, smtpHost)
	if err != nil {
		return err
	}
	defer func() { _ = cl.Quit() }()
	if mode == "starttls" {
		if ok, _ := cl.Extension("STARTTLS"); ok {
			// Enforce certificate verification
			if err := cl.StartTLS(&tls.Config{ServerName: smtpHost, InsecureSkipVerify: false}); err != nil {
				return err
			}
		}
	}
	if auth != nil {
		if ok, _ := cl.Extension("AUTH"); ok {
			if err := cl.Auth(auth); err != nil {
				return err
			}
		}
	}
	if err := cl.Mail(from); err != nil {
		return err
	}
	if err := cl.Rcpt(to); err != nil {
		return err
	}
	w, err := cl.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return nil
}

// Build formatted title for Teams email like: "教科名: タイトル(朝まで|X限まで|X〜Y限まで|M/D HH:mmまで)"
func buildTeamsTitleForTask(t domain.Task, DB *gorm.DB) string {
	var subj domain.Subject
	_ = DB.First(&subj, t.SubjectID).Error
	subjName := ""
	if strings.TrimSpace(subj.NameJa) != "" {
		subjName = subj.NameJa
	}
	dateLabel := t.Deadline.Format("1/2")
	label := deriveDeadlineLabelEnum(t.DeadlineSince, t.DeadlineUntil)
	if label == "" {
		label = dateLabel
	} else {
		label = dateLabel + " " + label
	}
	return fmt.Sprintf("%s: %s(%s)", subjName, t.Title, label)
}

// deriveDeadlineLabelEnum maps enums into symbolic labels like 朝まで or N限/N〜M限まで
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
	if sIdx > 0 && uIdx > 0 {
		if sIdx == uIdx {
			// Single-period window: show like "n限" (not "n限まで")
			return fmt.Sprintf("%d限", uIdx)
		}
		if sIdx < uIdx {
			return fmt.Sprintf("%d〜%d限まで", sIdx, uIdx)
		}
	}
	if uIdx > 0 {
		return fmt.Sprintf("%d限まで", uIdx)
	}
	if sIdx > 0 {
		return fmt.Sprintf("%d限以降", sIdx)
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

// NormalizeTaskType converts user-provided or localized labels into canonical enum values.
// Allowed enums: ONLINE, FACE_TO_FACE, QUIZ, PRESENTATION, SELF_STUDY, OTHER
func NormalizeTaskType(s string) string {
	v := strings.TrimSpace(strings.ToUpper(s))
	switch v {
	case "ONLINE", "ONL", "WEB", "REMOTE", "リモート", "ｵﾝﾗｲﾝ", "オンライン":
		return "ONLINE"
	case "FACE_TO_FACE", "F2F", "INPERSON", "IN_PERSON", "対面", "対面授業", "授業":
		return "FACE_TO_FACE"
	case "QUIZ", "TEST", "EXAM", "小テスト", "小ﾃｽﾄ", "テスト", "ﾃｽﾄ":
		return "QUIZ"
	case "PRESENTATION", "PRESEN", "LT", "発表", "ﾌﾟﾚｾﾞﾝ", "ﾌﾟﾚｾﾞﾝﾃｰｼｮﾝ", "プレゼン", "プレゼンテーション":
		return "PRESENTATION"
	case "SELF_STUDY", "SELFSTUDY", "STUDY", "PREVIEW", "予習", "復習", "自習", "自主学習", "予習・復習":
		return "SELF_STUDY"
	case "OTHER", "MISC", "その他", "ｿﾉ他":
		return "OTHER"
	default:
		return "OTHER"
	}
}

// ---- Japanese display helpers for change notifications ----
func displayTaskTypeJa(e string) string {
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
		return "その他"
	}
}

func displayBoolJa(b bool) string {
	if b {
		return "はい"
	}
	return "いいえ"
}

func displaySinceJa(since string) string {
	v := strings.ToUpper(strings.TrimSpace(since))
	switch v {
	case "P1", "P2", "P3", "P4", "P5", "P6", "P7", "P8", "P9", "P10":
		return strings.TrimPrefix(v, "P") + "限"
	case "NONE", "":
		return "指定なし"
	default:
		return v
	}
}

func displayUntilJa(until string) string {
	v := strings.ToUpper(strings.TrimSpace(until))
	switch v {
	case "MORNING":
		return "朝まで"
	case "LUNCH":
		return "昼休みまで"
	case "AFTER_SCHOOL":
		return "放課後まで"
	case "P1", "P2", "P3", "P4", "P5", "P6", "P7", "P8", "P9", "P10":
		return strings.TrimPrefix(v, "P") + "限まで"
	case "NONE", "":
		return "指定なし"
	default:
		return v
	}
}

func subjectNameJa(id uint, DB *gorm.DB) string {
	var s domain.Subject
	if err := DB.First(&s, id).Error; err == nil {
		if strings.TrimSpace(s.NameJa) != "" {
			return s.NameJa
		}
		if s.NameEn != nil && *s.NameEn != "" {
			return *s.NameEn
		}
	}
	return fmt.Sprintf("科目ID:%d", id)
}

func strOrUnsetJa(p *string) string {
	if p == nil || strings.TrimSpace(*p) == "" {
		return "（未設定）"
	}
	return *p
}

// Describe changes to fields included in the original Teams post.
// An empty message means there is no Teams-visible change.
func BuildTaskUpdateMessageJa(before domain.Task, after domain.Task, DB *gorm.DB) string {
	b := &strings.Builder{}
	// Title
	if before.Title != after.Title {
		fmt.Fprintf(b, "・タイトル: '%s' → '%s'\n", before.Title, after.Title)
	}
	// Subject
	if before.SubjectID != after.SubjectID {
		fmt.Fprintf(b, "・教科: '%s' → '%s'\n", subjectNameJa(before.SubjectID, DB), subjectNameJa(after.SubjectID, DB))
	}
	// Description
	if strOrUnsetJa(before.Description) != strOrUnsetJa(after.Description) {
		fmt.Fprintf(b, "・説明: '%s' → '%s'\n", strOrUnsetJa(before.Description), strOrUnsetJa(after.Description))
	}
	// Deadline date
	if before.Deadline.Format("2006-01-02") != after.Deadline.Format("2006-01-02") {
		fmt.Fprintf(b, "・期限日: %s → %s\n", before.Deadline.Format("2006-01-02"), after.Deadline.Format("2006-01-02"))
	}
	// Since / Until
	if !strings.EqualFold(before.DeadlineSince, after.DeadlineSince) {
		fmt.Fprintf(b, "・開始限: %s → %s\n", displaySinceJa(before.DeadlineSince), displaySinceJa(after.DeadlineSince))
	}
	if !strings.EqualFold(before.DeadlineUntil, after.DeadlineUntil) {
		fmt.Fprintf(b, "・終了限: %s → %s\n", displayUntilJa(before.DeadlineUntil), displayUntilJa(after.DeadlineUntil))
	}
	if b.Len() == 0 {
		return ""
	}
	return fmt.Sprintf("課題が更新されました (ID:%d)\n%s", after.ID, b.String())
}

func BuildTaskDeleteMessageJa(t domain.Task) string {
	// Teams 返信用のメッセージはシンプルに固定文言で通知
	// もし詳細が必要になった場合は、タイトル等を後段に追記する運用に変更可能
	return "この課題は削除されました。"
}

// Build SQL expression string for effective deadline timestamp
func EffectiveDeadlineExpr(alias string) string {
	// Date-only ordering: use the DATE column directly
	if alias != "" {
		return alias + ".deadline"
	}
	return "deadline"
}

// SendTeamsNotifyEmailForTaskSync sends a newly created task to Power Automate.
func SendTeamsNotifyEmailForTaskSync(t domain.Task, DB *gorm.DB) error {
	to := Env("TEAMS_NOTIFY_EMAIL", "")
	if strings.TrimSpace(to) == "" {
		return fmt.Errorf("TEAMS_NOTIFY_EMAIL is not configured")
	}
	subject := "[Taskly]tasks:add"
	formatted := buildTeamsTitleForTask(t, DB)
	desc := ""
	if t.Description != nil {
		desc = *t.Description
	}
	// JSON body consumed by Power Automate
	body := fmt.Sprintf(`{"task_id":%q,"title":%q,"description":%q,"sent_at":%q}`, fmt.Sprintf("%d", t.ID), formatted, teamsHTML(desc), time.Now().Format(time.RFC3339))
	return smtpSendPlain(to, subject, body)
}

// Look up the latest Teams message id for a task from tasks_teams_logs
func LatestTeamsMessageIDForTask(taskID uint, DB *gorm.DB) (string, bool) {
	var row struct {
		TeamsMessageID *string
	}
	err := DB.Table("tasks_teams_logs").
		Select("teams_message_id").
		Where("task_id = ? AND teams_message_id IS NOT NULL AND teams_message_id <> ''", taskID).
		Order("id DESC").
		Limit(1).
		Scan(&row).Error
	if err == nil && row.TeamsMessageID != nil && *row.TeamsMessageID != "" {
		return *row.TeamsMessageID, true
	}
	return "", false
}

// SendTeamsReplyEmailSync sends an update/delete reply for an existing message.
func SendTeamsReplyEmailSync(taskID uint, messageID string, message string) error {
	to := Env("TEAMS_NOTIFY_EMAIL", "")
	if strings.TrimSpace(to) == "" {
		return fmt.Errorf("TEAMS_NOTIFY_EMAIL is not configured")
	}
	subject := "[Taskly]tasks:reply"
	body := fmt.Sprintf(`{"message_id":%q,"task_id":%q,"message":%q}`, messageID, fmt.Sprintf("%d", taskID), teamsHTML(message))
	return smtpSendPlain(to, subject, body)
}

// Power Automate inserts these values directly into an HTML message body.
// Escape user text before adding breaks so literal markup stays literal.
func teamsHTML(text string) string {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	return strings.ReplaceAll(html.EscapeString(text), "\n", "<br>")
}
