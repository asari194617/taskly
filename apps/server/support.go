package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/pquerna/otp/totp"
	"gorm.io/gorm"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ensureDeadlineDateColumn alters tasks.deadline to DATE (from DATETIME) if necessary.
// This keeps the legacy field date-only, matching current requirements.
func ensureDeadlineDateColumn() {
	// Best effort: MySQL syntax (GORM uses MySQL driver in this project).
	// If tasks table/column doesn't exist yet (fresh DB), SELECT returns 0 rows and we skip.
	var dt string
	type res struct{ Cnt int }
	var r res
	_ = DB.Raw("SELECT COUNT(1) AS cnt FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'tasks' AND COLUMN_NAME = 'deadline'").Scan(&r).Error
	if r.Cnt == 0 {
		return
	}
	if err := DB.Raw("SELECT DATA_TYPE FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'tasks' AND COLUMN_NAME = 'deadline'").Scan(&dt).Error; err == nil {
		if strings.ToLower(strings.TrimSpace(dt)) == "date" {
			return
		}
	}
	// Alter to DATE NULL
	if err := DB.Exec("ALTER TABLE tasks MODIFY COLUMN deadline DATE NULL").Error; err != nil {
		log.Printf("migration warning: alter tasks.deadline to DATE failed: %v", err)
	}
}

// ensureTasksSchema verifies the tasks table has expected column data types for the new model.
// If the table has no rows and types mismatch, it will drop and recreate the table.
func ensureTasksSchema() {
	// Check table exists
	type res struct{ Cnt int }
	var r res
	_ = DB.Raw("SELECT COUNT(1) AS cnt FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'tasks'").Scan(&r).Error
	if r.Cnt == 0 {
		return
	}
	// Check row count
	var rowCnt int64
	_ = DB.Table("tasks").Count(&rowCnt).Error
	// Read column data types
	type col struct {
		ColumnName string
		DataType   string
	}
	var cols []col
	_ = DB.Raw("SELECT COLUMN_NAME, DATA_TYPE FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'tasks' AND COLUMN_NAME IN ('deadline','deadline_since','deadline_until')").Scan(&cols).Error
	exp := map[string]string{"deadline": "date", "deadline_since": "enum", "deadline_until": "enum"}
	mismatch := false
	for _, c := range cols {
		if exp[strings.ToLower(c.ColumnName)] != strings.ToLower(strings.TrimSpace(c.DataType)) {
			mismatch = true
		}
	}
	if mismatch && rowCnt == 0 {
		// Drop and recreate tasks table only (safe since no rows)
		if err := DB.Migrator().DropTable("tasks"); err != nil {
			log.Printf("migration warning: drop tasks failed: %v", err)
			return
		}
		if err := DB.AutoMigrate(&Task{}); err != nil {
			log.Printf("migration warning: recreate tasks failed: %v", err)
			return
		}
	}
}

// runMigrations executes SQL migrations from migrations folder if not already applied
func runMigrations() {
	type migrationFile struct {
		name       string
		statements []string
	}

	migrations := []migrationFile{
		{
			name: "0006_task_type_enum_subject_i18n",
			statements: []string{
				"UPDATE tasks SET type = 'ONLINE' WHERE type IN ('Online', 'online', 'ONLINE')",
				"UPDATE tasks SET type = 'FACE_TO_FACE' WHERE type IN ('Face-to-face', 'face-to-face', 'FACE_TO_FACE', 'Face to Face')",
				"UPDATE tasks SET type = 'QUIZ' WHERE type IN ('Quiz', 'quiz', 'QUIZ', 'Test', 'test')",
				"UPDATE tasks SET type = 'PRESENTATION' WHERE type IN ('Presentation', 'presentation', 'PRESENTATION')",
				"UPDATE tasks SET type = 'SELF_STUDY' WHERE type IN ('Self-study', 'self-study', 'SELF_STUDY', 'Self-study/Preview', 'Preview')",
				"UPDATE tasks SET type = 'OTHER' WHERE type IN ('Other', 'other', 'OTHER')",
				"UPDATE tasks SET type = 'ONLINE' WHERE type IN ('オンライン', 'ｵﾝﾗｲﾝ')",
				"UPDATE tasks SET type = 'FACE_TO_FACE' WHERE type IN ('対面', '対面授業', '授業')",
				"UPDATE tasks SET type = 'QUIZ' WHERE type IN ('小テスト', '小ﾃｽﾄ', 'テスト', 'ﾃｽﾄ')",
				"UPDATE tasks SET type = 'PRESENTATION' WHERE type IN ('プレゼン', 'ﾌﾟﾚｾﾞﾝ', 'プレゼンテーション', 'ﾌﾟﾚｾﾞﾝﾃｰｼｮﾝ', '発表')",
				"UPDATE tasks SET type = 'SELF_STUDY' WHERE type IN ('予習', '自習', '自主学習', '予習・復習', '復習')",
				"UPDATE tasks SET type = 'OTHER' WHERE type IN ('その他', 'ｿﾉ他')",
				"DELETE FROM tasks WHERE type NOT IN ('ONLINE','FACE_TO_FACE','QUIZ','PRESENTATION','SELF_STUDY','OTHER')",
				"ALTER TABLE tasks MODIFY COLUMN type ENUM('ONLINE','FACE_TO_FACE','QUIZ','PRESENTATION','SELF_STUDY','OTHER') NOT NULL DEFAULT 'OTHER'",
				"ALTER TABLE subjects ADD COLUMN name_en VARCHAR(255) NULL",
			},
		},
		{
			name: "0007_subject_name_ja_backfill",
			statements: []string{
				// Add name_ja nullable first (ignore if exists)
				"ALTER TABLE subjects ADD COLUMN name_ja VARCHAR(255) NULL",
				// Backfill from legacy title where empty
				"UPDATE subjects SET name_ja = title WHERE (name_ja IS NULL OR name_ja = '') AND title IS NOT NULL AND title <> ''",
				// Enforce NOT NULL after backfill
				"ALTER TABLE subjects MODIFY COLUMN name_ja VARCHAR(255) NOT NULL",
			},
		},
		{
			name: "0008_subject_drop_title",
			statements: []string{
				// Ensure no NULL/empty name_ja before dropping title
				"UPDATE subjects SET name_ja = title WHERE (name_ja IS NULL OR name_ja = '') AND title IS NOT NULL AND title <> ''",
				// Drop legacy title column (ignore if already dropped)
				"ALTER TABLE subjects DROP COLUMN title",
			},
		},
	}

	for _, m := range migrations {
		// Check if already applied
		var existing Migration
		if err := DB.Where("name = ?", m.name).First(&existing).Error; err == nil {
			continue // Already applied
		}

		// Execute migration statements one by one
		log.Printf("Running migration: %s", m.name)
		failed := false
		for _, stmt := range m.statements {
			if err := DB.Exec(stmt).Error; err != nil {
				// Check if error is about column already existing (safe to ignore)
				errMsg := err.Error()
				if strings.Contains(errMsg, "Duplicate column") ||
					strings.Contains(errMsg, "already exists") ||
					strings.Contains(errMsg, "1060") { // MySQL error code for duplicate column
					continue // Safe to ignore
				}
				log.Printf("Migration %s statement failed: %v\nStatement: %s", m.name, err, stmt)
				failed = true
				break
			}
		}

		if !failed {
			// Record as applied
			DB.Create(&Migration{Name: m.name})
			log.Printf("Migration %s applied successfully", m.name)
		}
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

// helpers moved from main.go (auth, crypto, email, ical, dayshift)
func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:])
}

func generateAPIKey() (plain string, hashed string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	plain = fmt.Sprintf("tsk_%x", buf)
	hashed = sha256Hex([]byte(plain))
	return
}

func generateIcalToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("ict_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("ict_%x", buf)
}

func parseAuthToken(authHeader string) string {
	s := strings.TrimSpace(authHeader)
	if s == "" {
		return ""
	}
	parts := strings.SplitN(s, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return s
}

func userFromSession(c *fiber.Ctx) bool {
	raw := c.Get("Authorization")
	token := parseAuthToken(raw)
	if token == "" {
		return false
	}
	headerPlat := strings.ToLower(strings.TrimSpace(c.Get("X-Client-Platform")))
	uidStr, err := RDB.Get(c.Context(), "session:"+token).Result()
	if err != nil || strings.TrimSpace(uidStr) == "" {
		var sess Session
		if dberr := DB.Where("(hashed_token = ? OR hashed_token = ?) AND (expires_at IS NULL OR expires_at > ?)", sha256Hex([]byte(token)), token, time.Now()).Order("id DESC").First(&sess).Error; dberr != nil {
			return false
		}
		if headerPlat != "" && sess.ClientPlatform != "" && !strings.EqualFold(headerPlat, sess.ClientPlatform) {
			return false
		}
		ttl := 30 * 24 * time.Hour
		if sess.ExpiresAt != nil {
			rem := time.Until(*sess.ExpiresAt)
			if rem > 0 && rem < ttl {
				ttl = rem
			}
		}
		_ = RDB.Set(c.Context(), "session:"+token, fmt.Sprintf("%d", sess.UserID), ttl).Err()
		uidStr = fmt.Sprintf("%d", sess.UserID)
	} else {
		if headerPlat != "" {
			var sess Session
			if err := DB.Where("hashed_token = ?", sha256Hex([]byte(token))).Order("id DESC").First(&sess).Error; err == nil {
				if sess.ClientPlatform != "" && !strings.EqualFold(headerPlat, sess.ClientPlatform) {
					return false
				}
			}
		}
	}
	var u User
	if err := DB.First(&u, uidStr).Error; err != nil {
		return false
	}
	var cnt int64
	if err := DB.Model(&Block{}).Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", u.ID, time.Now()).Count(&cnt).Error; err == nil && cnt > 0 {
		return false
	}
	c.Locals("user", &u)
	c.Locals("token", token)
	return true
}

func isAllowedPlatform(p string) bool {
	switch strings.ToLower(p) {
	case "web", "ios", "android", "mac", "windows":
		return true
	default:
		return false
	}
}

func generateSessionToken(uid uint, platform string) string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("sess_%d_%s_%d", uid, platform, time.Now().UnixNano())
	}
	enc := base64.RawURLEncoding.EncodeToString(b)
	return fmt.Sprintf("sess_%d_%s_%s", uid, platform, enc)
}

func revokeOtherSessions(c *fiber.Ctx, userID uint, currentToken string) {
	key := fmt.Sprintf("user_sessions:%d", userID)
	toks, _ := RDB.SMembers(c.Context(), key).Result()
	for _, t := range toks {
		if t == currentToken || strings.TrimSpace(t) == "" {
			continue
		}
		_ = RDB.Del(c.Context(), "session:"+t).Err()
		_ = RDB.SRem(c.Context(), key, t).Err()
	}
	curHash := sha256Hex([]byte(currentToken))
	_ = DB.Where("user_id = ? AND hashed_token <> ?", userID, curHash).Delete(&Session{}).Error
}

func userFromAPIKey(c *fiber.Ctx) bool {
	apiKey := c.Get("X-API-Key")
	if apiKey == "" {
		return false
	}
	hashed := sha256Hex([]byte(apiKey))
	var key APIKey
	if err := DB.Where("hashed_token = ? AND is_valid = 1", hashed).First(&key).Error; err != nil {
		return false
	}
	var u User
	if err := DB.First(&u, key.UserID).Error; err != nil {
		return false
	}
	var cnt int64
	if err := DB.Model(&Block{}).Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", u.ID, time.Now()).Count(&cnt).Error; err == nil && cnt > 0 {
		return false
	}
	c.Locals("user", &u)
	c.Locals("apiKey", apiKey)
	return true
}

// 2FA helpers
func urlEncode(s string) string { return url.QueryEscape(s) }

func generateBase32Secret() string {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	return strings.TrimRight(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), "=")
}

func validateTOTP(secret, code string) bool {
	return totp.Validate(code, secret)
}

func generateRecoveryCode() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	const groups = 4
	const perGroup = 4
	out := make([]byte, 0, groups*perGroup+(groups-1))
	buf := make([]byte, groups*perGroup)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%08x", time.Now().UnixNano())
	}
	for i := 0; i < groups*perGroup; i++ {
		out = append(out, alphabet[int(buf[i])%len(alphabet)])
		if (i+1)%perGroup == 0 && i+1 < groups*perGroup {
			out = append(out, '-')
		}
	}
	return string(out)
}

func consumeRecoveryCode(userID uint, code string) bool {
	hashed := sha256Hex([]byte(code))
	ok := false
	_ = DB.Transaction(func(tx *gorm.DB) error {
		var rc RecoveryCode
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("user_id = ? AND hashed_code = ? AND is_used = 0", userID, hashed).First(&rc).Error; err != nil {
			return nil
		}
		rc.IsUsed = true
		if err := tx.Save(&rc).Error; err != nil {
			return err
		}
		ok = true
		return nil
	})
	return ok
}

func getActiveTOTP(userID uint) (bool, string, error) {
	var ts TOTPSecret
	if err := DB.Where("user_id = ? AND is_active = 1", userID).First(&ts).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, "", nil
		}
		return false, "", err
	}
	secret, err := decryptString(ts.Secret)
	if err != nil {
		return false, "", err
	}
	return true, secret, nil
}

func getEncKey() []byte {
	keyStr := env("TOTP_ENC_KEY", "dev_totp_key")
	sum := sha256.Sum256([]byte(keyStr))
	return sum[:]
}

func encryptString(plain string) (string, error) {
	key := getEncKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nil, nonce, []byte(plain), nil)
	out := append(nonce, ct...)
	return base64.RawStdEncoding.EncodeToString(out), nil
}

func decryptString(enc string) (string, error) {
	key := getEncKey()
	raw, err := base64.RawStdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ct := raw[:ns], raw[ns:]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// iCal builder and helpers
func buildICSForUser(userID uint) string {
	b := &strings.Builder{}
	crlf := "\r\n"
	now := time.Now().UTC().Format("20060102T150405Z")
	b.WriteString("BEGIN:VCALENDAR" + crlf)
	b.WriteString("VERSION:2.0" + crlf)
	b.WriteString("PRODID:-//Taskly//EN" + crlf)
	b.WriteString("CALSCALE:GREGORIAN" + crlf)
	b.WriteString("METHOD:PUBLISH" + crlf)
	var tasks []Task
	// Order by effective deadline timestamp derived from date + enum
	eff := effectiveDeadlineExpr("")
	_ = DB.Where("added_user_id = ?", userID).Order(eff + " ASC").Limit(500).Find(&tasks).Error
	for _, t := range tasks {
		ed := effectiveDeadlineTime(t)
		if ed.IsZero() {
			continue
		}
		var subj Subject
		_ = DB.First(&subj, t.SubjectID).Error
		var u User
		_ = DB.First(&u, t.AddedUserID).Error
		lang := strings.ToLower(strings.TrimSpace(u.NotificationLang))
		if lang == "" {
			lang = "ja"
		}
		sName := subj.NameJa
		if lang == "en" && subj.NameEn != nil && *subj.NameEn != "" {
			sName = *subj.NameEn
		}
		b.WriteString("BEGIN:VEVENT" + crlf)
		b.WriteString(fmt.Sprintf("UID:tsk-%d@taskly\r\n", t.ID))
		b.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", now))
		b.WriteString(fmt.Sprintf("SUMMARY:%s\r\n", icalEscape("[Task] "+sName+": "+t.Title)))
		if t.Description != nil {
			b.WriteString(fmt.Sprintf("DESCRIPTION:%s\r\n", icalEscape(*t.Description)))
		}
		b.WriteString(fmt.Sprintf("DTSTART:%s\r\n", ed.UTC().Format("20060102T150405Z")))
		b.WriteString(fmt.Sprintf("DTEND:%s\r\n", ed.UTC().Format("20060102T150405Z")))
		b.WriteString("END:VEVENT" + crlf)
	}
	var asgs []struct {
		ID   uint
		Date time.Time
	}
	_ = DB.Table("day_shift_assignments a").
		Select("a.id, a.date").
		Joins("JOIN day_shift_assignments_users u ON u.day_shift_assignments_id = a.id").
		Where("u.user_id = ?", userID).Order("a.date ASC").Limit(500).Scan(&asgs).Error
	for _, a := range asgs {
		start := time.Date(a.Date.Year(), a.Date.Month(), a.Date.Day(), 9, 0, 0, 0, a.Date.Location())
		end := start.Add(8 * time.Hour)
		b.WriteString("BEGIN:VEVENT" + crlf)
		b.WriteString(fmt.Sprintf("UID:ds-%d@taskly\r\n", a.ID))
		b.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", now))
		b.WriteString("SUMMARY:Day Shift\r\n")
		b.WriteString(fmt.Sprintf("DTSTART:%s\r\n", start.UTC().Format("20060102T150405Z")))
		b.WriteString(fmt.Sprintf("DTEND:%s\r\n", end.UTC().Format("20060102T150405Z")))
		b.WriteString("END:VEVENT" + crlf)
	}
	b.WriteString("END:VCALENDAR" + crlf)
	return b.String()
}

func icalEscape(s string) string {
	r := strings.ReplaceAll(s, "\\", "\\\\")
	r = strings.ReplaceAll(r, ",", "\\,")
	r = strings.ReplaceAll(r, ";", "\\;")
	r = strings.ReplaceAll(r, "\n", "\\n")
	r = strings.ReplaceAll(r, "\r", "")
	return r
}

// SMTP helper to send multipart email
func smtpSend(to, subject, text, html string) error {
	smtpHost := env("SMTP_HOST", "localhost")
	smtpPort := env("SMTP_PORT", "25")
	from := env("SMTP_FROM", "noreply@taskly.local")
	smtpUser := env("SMTP_USER", from)
	smtpPass := env("SMTP_PASSWORD", "")
	nowRFC := time.Now().Format(time.RFC1123Z)
	boundary := fmt.Sprintf("taskly-%d", time.Now().UnixNano())
	msg := []byte(
		"From: " + from + "\r\n" +
			"To: " + to + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"Date: " + nowRFC + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n\r\n" +
			"--" + boundary + "\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" + text + "\r\n" +
			"--" + boundary + "\r\nContent-Type: text/html; charset=utf-8\r\n\r\n" + html + "\r\n" +
			"--" + boundary + "--\r\n",
	)
	var auth smtp.Auth
	if smtpPass != "" {
		auth = smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	}
	toSec, _ := strconv.Atoi(env("SMTP_TIMEOUT", "60"))
	if toSec <= 0 {
		toSec = 60
	}
	dialer := &net.Dialer{Timeout: time.Duration(toSec) * time.Second}
	mode := strings.ToLower(strings.TrimSpace(env("SMTP_TLS", "")))
	var conn net.Conn
	var err error
	if mode == "smtps" || smtpPort == "465" {
		// SMTPS (implicit TLS)
		conn, err = tls.DialWithDialer(dialer, "tcp", smtpHost+":"+smtpPort, &tls.Config{ServerName: smtpHost})
	} else {
		// Plain TCP first; may upgrade via STARTTLS
		conn, err = dialer.Dial("tcp", smtpHost+":"+smtpPort)
	}
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Duration(toSec) * time.Second))
	cl, err := smtp.NewClient(conn, smtpHost)
	if err != nil {
		return err
	}
	defer func() { _ = cl.Quit() }()
	if mode == "starttls" {
		if ok, _ := cl.Extension("STARTTLS"); ok {
			if err := cl.StartTLS(&tls.Config{ServerName: smtpHost}); err != nil {
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

// SMTP helper to send plain text email (no HTML/multipart)
func smtpSendPlain(to, subject, text string) error {
	smtpHost := env("SMTP_HOST", "localhost")
	smtpPort := env("SMTP_PORT", "25")
	from := env("SMTP_FROM", "noreply@taskly.local")
	smtpUser := env("SMTP_USER", from)
	smtpPass := env("SMTP_PASSWORD", "")
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
	toSec, _ := strconv.Atoi(env("SMTP_TIMEOUT", "60"))
	if toSec <= 0 {
		toSec = 60
	}
	dialer := &net.Dialer{Timeout: time.Duration(toSec) * time.Second}
	mode := strings.ToLower(strings.TrimSpace(env("SMTP_TLS", "")))
	var conn net.Conn
	var err error
	if mode == "smtps" || smtpPort == "465" {
		conn, err = tls.DialWithDialer(dialer, "tcp", smtpHost+":"+smtpPort, &tls.Config{ServerName: smtpHost})
	} else {
		conn, err = dialer.Dial("tcp", smtpHost+":"+smtpPort)
	}
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Duration(toSec) * time.Second))
	cl, err := smtp.NewClient(conn, smtpHost)
	if err != nil {
		return err
	}
	defer func() { _ = cl.Quit() }()
	if mode == "starttls" {
		if ok, _ := cl.Extension("STARTTLS"); ok {
			if err := cl.StartTLS(&tls.Config{ServerName: smtpHost}); err != nil {
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
func buildTeamsTitleForTask(t Task) string {
	var subj Subject
	_ = DB.First(&subj, t.SubjectID).Error
	// Determine preferred language from adding user if available
	var user User
	_ = DB.First(&user, t.AddedUserID).Error
	lang := strings.ToLower(strings.TrimSpace(user.NotificationLang))
	if lang == "" {
		lang = "ja"
	}
	subjName := ""
	if lang == "en" {
		if subj.NameEn != nil && *subj.NameEn != "" {
			subjName = *subj.NameEn
		}
	}
	if subjName == "" {
		if strings.TrimSpace(subj.NameJa) != "" {
			subjName = subj.NameJa
		}
	}
	dateLabel := t.Deadline.Format("1/2")
	label := deriveDeadlineLabelEnum(t.DeadlineSince, t.DeadlineUntil)
	if label == "" {
		label = dateLabel + "まで"
	} else {
		label = dateLabel + " " + label
	}
	return fmt.Sprintf("%s: %s(%s)", subjName, t.Title, label)
}

// (removed obsolete meta parsing helpers)

// deriveDeadlineLabelEnum maps enums into symbolic labels like 朝まで or N限/N〜M限まで
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

// end time of deadline based on enum until
func endTimeForUntil(until string) (hour, min int) {
	switch strings.ToUpper(strings.TrimSpace(until)) {
	case "MORNING":
		return 7, 59
	case "P1":
		return 10, 30
	case "P2":
		return 12, 10
	case "P3":
		return 14, 30
	case "P4":
		return 16, 10
	case "P5":
		return 17, 50
	case "P6":
		return 19, 30
	case "P7":
		return 21, 10
	case "P8":
		return 22, 50
	default:
		return 23, 59
	}
}

// Build SQL expression string for effective deadline timestamp
func effectiveDeadlineExpr(alias string) string {
	col := func(name string) string {
		if alias != "" {
			return alias + "." + name
		}
		return name
	}
	// CONCAT(date, ' ', time) cast to datetime
	return "TIMESTAMP(" + col("deadline") + ", CASE " +
		"WHEN " + col("deadline_until") + "='MORNING' THEN '07:59:59' " +
		"WHEN " + col("deadline_until") + "='P1' THEN '10:30:00' " +
		"WHEN " + col("deadline_until") + "='P2' THEN '12:10:00' " +
		"WHEN " + col("deadline_until") + "='P3' THEN '14:30:00' " +
		"WHEN " + col("deadline_until") + "='P4' THEN '16:10:00' " +
		"WHEN " + col("deadline_until") + "='P5' THEN '17:50:00' " +
		"WHEN " + col("deadline_until") + "='P6' THEN '19:30:00' " +
		"WHEN " + col("deadline_until") + "='P7' THEN '21:10:00' " +
		"WHEN " + col("deadline_until") + "='P8' THEN '22:50:00' " +
		"ELSE '23:59:59' END)"
}

// Compute effective deadline timestamp in Go for the given task
func effectiveDeadlineTime(t Task) time.Time {
	h, m := endTimeForUntil(t.DeadlineUntil)
	return time.Date(t.Deadline.Year(), t.Deadline.Month(), t.Deadline.Day(), h, m, 0, 0, t.Deadline.Location())
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Send Teams notification email for a newly created task
func sendTeamsNotifyEmailForTask(t Task) {
	to := env("TEAMS_NOTIFY_EMAIL", "")
	if strings.TrimSpace(to) == "" {
		return
	}
	subject := "[Taskly]tasks:add"
	formatted := buildTeamsTitleForTask(t)
	desc := ""
	if t.Description != nil {
		desc = *t.Description
	}
	// JSON body consumed by Power Automate
	body := fmt.Sprintf(`{"task_id":%q,"title":%q,"description":%q}`, fmt.Sprintf("%d", t.ID), formatted, desc)
	// Fire and forget
	go func() {
		_ = smtpSendPlain(to, subject, body)
	}()
}

// small helpers
func toUint(s string) uint {
	v, _ := strconv.ParseUint(s, 10, 64)
	return uint(v)
}

// (removed obsolete TLS wrappers; smtpSend uses crypto/tls directly)

// Authorization and permission helpers
func userHasRole(userID uint, role string) (bool, error) {
	var cnt int64
	if err := DB.Model(&RoleUser{}).Where("user_id = ? AND role = ?", userID, role).Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}

func apiKeyLimitForUser(userID uint) (int, error) {
	if ok, err := userHasRole(userID, "admin"); err != nil {
		return 0, err
	} else if ok {
		return -1, nil
	}
	if ok, err := userHasRole(userID, "manager"); err != nil {
		return 0, err
	} else if ok {
		return 100, nil
	}
	var p Permission
	if err := DB.Where("user_id = ? AND target = ?", userID, "api_key_limit").Order("id DESC").First(&p).Error; err == nil && p.Value != nil {
		var v int
		fmt.Sscanf(*p.Value, "%d", &v)
		if v >= 0 {
			return v, nil
		}
	}
	return 10, nil
}

func checkPermission(userID uint, target, action string, ownerID *uint) (bool, error) {
	if ok, err := userHasRole(userID, "admin"); err != nil {
		return false, err
	} else if ok {
		return true, nil
	}
	if ok, err := userHasRole(userID, "manager"); err != nil {
		return false, err
	} else if ok {
		return true, nil
	}
	var p Permission
	if err := DB.Where("user_id = ? AND target = ? AND method = ?", userID, target, action).Order("id DESC").First(&p).Error; err == nil {
		if p.Value == nil || *p.Value == "" || strings.EqualFold(*p.Value, "all") {
			return true, nil
		}
		if strings.EqualFold(*p.Value, "own") {
			if ownerID != nil && *ownerID == userID {
				return true, nil
			}
			return false, nil
		}
		return false, nil
	}
	switch strings.ToLower(action) {
	case "read":
		return true, nil
	case "create":
		return true, nil
	case "update":
		if ownerID != nil && *ownerID == userID {
			return true, nil
		}
		return false, nil
	case "delete":
		return false, nil
	default:
		return false, nil
	}
}

// Day shift helpers and schedulers
func dayShiftScheduler(notifyHHmm string) {
	parts := strings.SplitN(notifyHHmm, ":", 2)
	hh, _ := strconv.Atoi(parts[0])
	mm, _ := strconv.Atoi(parts[1])
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), hh, mm, 0, 0, now.Location())
		if !now.Before(next) {
			next = next.Add(24 * time.Hour)
		}
		time.Sleep(time.Until(next))
		go func(ts time.Time) {
			_ = notifyDayShift(ts)
			_ = ensureNextCycleIfNearEnd(ts)
			// Run rotation to ensure assignments are kept in sync
			_ = performDayShiftRotation(ts)
		}(time.Now())
	}
}

func isBusinessDay(d time.Time) bool {
	var bc BusinessCalendar
	if err := DB.Where("date = ?", d.Format("2006-01-02")).First(&bc).Error; err == nil {
		return bc.IsBusinessDay
	}
	wd := d.Weekday()
	return wd != time.Saturday && wd != time.Sunday
}

func notifyDayShift(ts time.Time) error {
	if !isBusinessDay(ts) {
		return nil
	}
	var asg DayShiftAssignment
	if err := DB.Where("date = ?", ts.Format("2006-01-02")).First(&asg).Error; err != nil {
		return nil
	}
	var users []DayShiftAssignmentUser
	_ = DB.Where("day_shift_assignments_id = ?", asg.ID).Find(&users).Error
	if len(users) == 0 {
		return nil
	}
	hook := env("DISCORD_WEBHOOK_URL", "")
	if hook == "" {
		return nil
	}
	content := fmt.Sprintf("Day shift for %s: %d user(s) assigned.", ts.Format("2006-01-02"), len(users))
	payload := fmt.Sprintf(`{"content": %q}`, content)
	req, _ := http.NewRequest("POST", hook, strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err == nil && resp != nil {
		_ = resp.Body.Close()
	}
	return err
}

func tasksNotifyScheduler(notifyHHmm string) {
	parts := strings.SplitN(notifyHHmm, ":", 2)
	hh, _ := strconv.Atoi(parts[0])
	mm, _ := strconv.Atoi(parts[1])
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), hh, mm, 0, 0, now.Location())
		if !now.Before(next) {
			next = next.Add(24 * time.Hour)
		}
		time.Sleep(time.Until(next))
		go sendDailyTaskDigest()
	}
}

func sendDailyTaskDigest() {
	start := dateOnly(time.Now())
	end := dateOnly(time.Now().Add(48 * time.Hour)).Add(-time.Nanosecond)
	var users []User
	_ = DB.Where("is_deleted = 0").Find(&users).Error
	for _, u := range users {
		var tasks []Task
		eff := effectiveDeadlineExpr("")
		_ = DB.Where("added_user_id = ? AND "+eff+" BETWEEN ? AND ?", u.ID, start, end).Order(eff + " ASC").Find(&tasks).Error
		if len(tasks) == 0 {
			continue
		}
		b := &strings.Builder{}
		b.WriteString("<p>期限が近い課題の一覧です。</p>")
		b.WriteString("<table style=\"width:100%; border-collapse:collapse;\"><thead><tr><th style=\"text-align:left; padding:8px; border-bottom:1px solid #e5e7eb;\">タイトル</th><th style=\"text-align:left; padding:8px; border-bottom:1px solid #e5e7eb;\">期限</th></tr></thead><tbody>")
		for _, t := range tasks {
			dl := effectiveDeadlineTime(t).Format("2006-01-02 15:04")
			b.WriteString(fmt.Sprintf("<tr><td style=\"padding:8px; border-bottom:1px solid #f1f5f9;\">%s</td><td style=\"padding:8px; border-bottom:1px solid #f1f5f9; white-space:nowrap;\">%s</td></tr>", icalEscape(t.Title), icalEscape(dl)))
		}
		b.WriteString("</tbody></table>")
		html := buildEmailHTML("期限が近い課題のお知らせ", b.String())
		text := "期限が近い課題があります。アプリで確認してください。"
		_ = smtpSend(u.Email, "[Taskly] 期限が近い課題のお知らせ", text, html)
	}
}

func buildEmailHTML(title, contentHTML string) string {
	return fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8"/></head>
  <body style="background-color:#0b0f19; padding:24px; font-family:-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, 'Noto Sans', sans-serif;">
    <div style="max-width:560px; margin:0 auto; background-color:#0b0f19;">
      <div style="background:#0f172a; color:#e2e8f0; border:1px solid #1f2937; border-radius:12px; padding:20px;">
        <h1 style="font-size:18px; margin:0 0 12px 0;">%s</h1>
        <div style="font-size:14px; line-height:1.7; color:#cbd5e1;">%s</div>
      </div>
      <div style="text-align:center; color:#64748b; font-size:12px; margin-top:16px;">Taskly</div>
    </div>
  </body></html>`, icalEscape(title), contentHTML)
}

func performDayShiftRotation(now time.Time) error {
	d := now
	for !isBusinessDay(d) {
		d = d.Add(24 * time.Hour)
	}
	cyc, err := getCurrentCycle(d)
	if err != nil {
		return err
	}
	if cyc == nil {
		return nil
	}
	prev := prevBusinessDay(d)
	if !prev.IsZero() {
		var prevAsg DayShiftAssignment
		if err := DB.Where("date = ?", prev.Format("2006-01-02")).First(&prevAsg).Error; err == nil {
			if strings.EqualFold(prevAsg.Status, "unfulfilled") {
				var today DayShiftAssignment
				if err := DB.Where("date = ?", d.Format("2006-01-02")).First(&today).Error; err != nil {
					today = DayShiftAssignment{Date: time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location()), Status: "assigned", CycleID: cyc.ID}
					if err := DB.Create(&today).Error; err != nil {
						return err
					}
				}
				var prevUsers []DayShiftAssignmentUser
				_ = DB.Where("day_shift_assignments_id = ?", prevAsg.ID).Find(&prevUsers).Error
				_ = DB.Where("day_shift_assignments_id = ?", today.ID).Delete(&DayShiftAssignmentUser{}).Error
				for _, pu := range prevUsers {
					_ = DB.Create(&DayShiftAssignmentUser{DayShiftAssignmentsID: today.ID, UserID: pu.UserID}).Error
				}
				return nil
			}
		}
	}
	var today DayShiftAssignment
	if err := DB.Where("date = ?", d.Format("2006-01-02")).First(&today).Error; err != nil {
		today = DayShiftAssignment{Date: time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location()), Status: "assigned", CycleID: cyc.ID}
		if err := DB.Create(&today).Error; err != nil {
			return err
		}
	}
	var cfgPairs []DayShiftCyclePair
	_ = DB.Where("cycle_id = ?", cyc.ID).Order("position ASC, id ASC").Find(&cfgPairs).Error
	var pairs [][]uint
	if len(cfgPairs) > 0 {
		for _, p := range cfgPairs {
			pairs = append(pairs, []uint{p.User1ID, p.User2ID})
		}
	} else {
		actUsers, err := listActiveUsers()
		if err != nil || len(actUsers) < 2 {
			return err
		}
		pairs = computePairs(actUsers)
	}
	if len(pairs) == 0 {
		return nil
	}
	dayIdx := businessDaysBetween(cyc.BeginDate, d)
	pair := pairs[dayIdx%len(pairs)]
	_ = DB.Where("day_shift_assignments_id = ?", today.ID).Delete(&DayShiftAssignmentUser{}).Error
	for _, uid := range pair {
		_ = DB.Create(&DayShiftAssignmentUser{DayShiftAssignmentsID: today.ID, UserID: uid}).Error
	}
	return nil
}

func ensureNextCycleIfNearEnd(now time.Time) error {
	var cyc DayShiftCycle
	if err := DB.Order("finish_date DESC").First(&cyc).Error; err != nil {
		return nil
	}
	if now.After(cyc.FinishDate.AddDate(0, 0, -3)) {
		var next DayShiftCycle
		if err := DB.Where("begin_date > ?", cyc.FinishDate).Order("begin_date ASC").First(&next).Error; err != nil {
			hook := env("DISCORD_WEBHOOK_URL", "")
			if hook != "" {
				content := fmt.Sprintf("[DayShift] No next cycle defined after %s", cyc.FinishDate.Format("2006-01-02"))
				payload := fmt.Sprintf(`{"content": %q}`, content)
				req, _ := http.NewRequest("POST", hook, strings.NewReader(payload))
				req.Header.Set("Content-Type", "application/json")
				client := &http.Client{Timeout: 5 * time.Second}
				resp, err := client.Do(req)
				if err == nil && resp != nil {
					_ = resp.Body.Close()
				}
			}
			return nil
		}
		for dt := dateOnly(next.BeginDate); !dt.After(dateOnly(next.FinishDate)); dt = dt.Add(24 * time.Hour) {
			if !isBusinessDay(dt) {
				continue
			}
			var cnt int64
			_ = DB.Model(&DayShiftAssignment{}).Where("date = ?", dt.Format("2006-01-02")).Count(&cnt).Error
			if cnt == 0 {
				_ = DB.Create(&DayShiftAssignment{Date: time.Date(dt.Year(), dt.Month(), dt.Day(), 0, 0, 0, 0, dt.Location()), CycleID: next.ID, Status: "assigned"}).Error
			}
		}
	}
	return nil
}

func getCurrentCycle(d time.Time) (*DayShiftCycle, error) {
	var cyc DayShiftCycle
	if err := DB.Where("begin_date <= ? AND finish_date >= ?", d.Format("2006-01-02"), d.Format("2006-01-02")).First(&cyc).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &cyc, nil
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

//lint:ignore U1000 kept for future scheduling logic
func prevBusinessDay(d time.Time) time.Time {
	x := d.Add(-24 * time.Hour)
	for i := 0; i < 10; i++ {
		if isBusinessDay(x) {
			return dateOnly(x)
		}
		x = x.Add(-24 * time.Hour)
	}
	return time.Time{}
}

func nextBusinessDay(d time.Time) time.Time {
	x := d.Add(24 * time.Hour)
	for i := 0; i < 30; i++ {
		if isBusinessDay(x) {
			return dateOnly(x)
		}
		x = x.Add(24 * time.Hour)
	}
	return time.Time{}
}

//lint:ignore U1000 used by optional rotation logic
func listActiveUsers() ([]User, error) {
	var users []User
	if err := DB.Where("is_deleted = 0 AND is_day_shift_active = 1").Order("id ASC").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func computePairs(users []User) [][]uint {
	out := [][]uint{}
	n := len(users)
	if n < 2 {
		return out
	}
	if n%2 == 1 {
		log.Printf("[DayShift] odd number of active users (%d), dropping last (id=%d)", n, users[n-1].ID)
		n = n - 1
	}
	for i := 0; i < n; i += 2 {
		out = append(out, []uint{users[i].ID, users[i+1].ID})
	}
	return out
}

// Mark optional helpers as used to satisfy static analysis when rotation is disabled in some builds
//
//lint:ignore U1000 used by optional rotation logic
var (
	_ = prevBusinessDay
	_ = listActiveUsers
	_ = businessDaysBetween
)

func businessDaysBetween(start time.Time, end time.Time) int {
	s := dateOnly(start)
	e := dateOnly(end)
	cnt := 0
	for d := s; !d.After(e); d = d.Add(24 * time.Hour) {
		if isBusinessDay(d) {
			cnt++
		}
	}
	if cnt > 0 {
		return cnt - 1
	}
	return 0
}
