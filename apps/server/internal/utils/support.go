package utils

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/getsentry/sentry-go"
	"github.com/gofiber/fiber/v2"
	"github.com/pquerna/otp/totp"
	"github.com/redis/go-redis/v9"
	"google.golang.org/genai"
	"gorm.io/gorm"
)

func Env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// MustEnv returns the environment variable value for the given key.
// If the variable is not set (empty), the function will log.Fatal and exit.
func MustEnv(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	msg := fmt.Sprintf("required environment variable %s is not set", key)
	// Send to Sentry if initialized (no-op if not)
	sentry.CaptureMessage(msg)
	// Flush briefly to attempt delivery
	_ = sentry.Flush(2 * time.Second)

	// Send to Discord webhook (use helper)
	SendDiscordEnvMissing(key, msg)

	log.Fatalf("%s", msg)
	return ""
}

// SendDiscordEnvMissing posts a standardized embed to the configured Discord webhook
// for a missing environment variable event. It's a thin wrapper around
// SendDiscordEmbeds so callers don't need to construct embeds themselves.
func SendDiscordEnvMissing(key, msg string) {
	embed := map[string]any{
		"title":       "Missing environment variable",
		"description": msg,
		"color":       15158332, // red-ish
		"timestamp":   time.Now().Format(time.RFC3339),
		"fields": []map[string]any{
			{"name": "variable", "value": key, "inline": true},
			{"name": "host", "value": Env("HOSTNAME", Env("API_HOST", "unknown")), "inline": true},
		},
	}
	SendDiscordEmbeds([]map[string]any{embed})
}

// --- Config helpers (configs table) ---
func GetConfigString(key string, DB *gorm.DB) (string, error) {
	var row struct{ V string }
	if err := DB.Raw("SELECT value AS v FROM configs WHERE `key`=? LIMIT 1", key).Scan(&row).Error; err != nil {
		return "", err
	}
	return row.V, nil
}

func SetConfigString(key, val string, DB *gorm.DB) error {
	res := DB.Exec("UPDATE configs SET value=? WHERE `key`=?", val, key)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return DB.Exec("INSERT INTO configs(`key`,`value`) VALUES(?,?) ON DUPLICATE KEY UPDATE value=VALUES(value)", key, val).Error
	}
	return nil
}

func GetConfigInt(key string, DB *gorm.DB) (int, error) {
	s, err := GetConfigString(key, DB)
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(s) == "" {
		return 0, nil
	}
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n, nil
}

// --- JSON parsing & validation helpers ---
func ParseJSONStrict(c *fiber.Ctx, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(c.Body()))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func ValidationError(c *fiber.Ctx, fields map[string]string) error {
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "validation", "fields": fields})
}

func IsEmail(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 255 || !strings.Contains(s, "@") {
		return false
	}
	// accept simple address forms; mail.ParseAddress is lenient but works for basic format
	if _, err := mail.ParseAddress(s); err != nil {
		return false
	}
	return true
}

func MaxLen(s string, n int) bool { return len(s) <= n }

// --- Scrubbing helpers ---
// Sanitize URL path by masking token-like segments such as /ical/{token}.ics or Bearer tokens in query
func SanitizeURLPath(u string) string {
	// We avoid importing net/url here to keep behavior simple as input may already be sanitized by framework
	s := u
	// mask /ical/<token>.ics
	if strings.Contains(s, "/ical/") && strings.HasSuffix(s, ".ics") {
		// replace the last path segment before .ics
		idx := strings.Index(s, "/ical/")
		tail := s[idx+len("/ical/"):]
		if p := strings.Index(tail, "/"); p >= 0 {
			// extra segments; mask first segment only
			first := tail[:p]
			s = strings.Replace(s, first, "[REDACTED]", 1)
		} else {
			// single segment like token.ics
			if q := strings.Index(tail, ".ics"); q > 0 {
				token := tail[:q]
				s = strings.Replace(s, token, "[REDACTED]", 1)
			}
		}
	}
	return s
}

func sanitizeURLPath(u string) string { return SanitizeURLPath(u) }

// Sanitize query string by masking sensitive parameters
func SanitizeQueryString(q string) string {
	if strings.TrimSpace(q) == "" {
		return q
	}
	// try to parse lightly as k=v&k2=v2 (without url decoding)
	parts := strings.Split(q, "&")
	for i, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) == 2 {
			k := strings.ToLower(kv[0])
			if IsSensitiveKey(k) || IsSensitiveValue(kv[1]) {
				parts[i] = kv[0] + "=[REDACTED]"
			}
		}
	}
	return strings.Join(parts, "&")
}

// Scrub values in a map[string]any recursively when keys or values look sensitive
func ScrubPIIMap(m map[string]any) {
	for k, v := range m {
		lk := strings.ToLower(k)
		if IsSensitiveKey(lk) {
			m[k] = "[REDACTED]"
			continue
		}
		switch t := v.(type) {
		case string:
			if IsSensitiveValue(t) {
				m[k] = "[REDACTED]"
			}
		case map[string]any:
			ScrubPIIMap(t)
		case map[string]string:
			for kk, vv := range t {
				if IsSensitiveKey(kk) || IsSensitiveValue(vv) {
					t[kk] = "[REDACTED]"
				}
			}
		}
	}
}

func IsSensitiveKey(k string) bool {
	k = strings.ToLower(strings.TrimSpace(k))
	switch k {
	case "authorization", "x-api-key", "api_key", "apikey", "password", "passwd", "secret", "token", "session", "cookie", "set-cookie", "refresh_token", "access_token":
		return true
	default:
		return false
	}
}

func IsSensitiveValue(v string) bool {
	s := strings.ToLower(strings.TrimSpace(v))
	if s == "" {
		return false
	}
	// common bearer/api key prefixes
	if strings.HasPrefix(s, "bearer ") || strings.HasPrefix(s, "tsk_") || strings.HasPrefix(s, "sess_") || strings.HasPrefix(s, "ict_") {
		return true
	}
	// looks like a JWT (three base64url parts)
	if strings.Count(v, ".") == 2 && len(v) > 30 {
		return true
	}
	// long hex-ish strings
	hexIsh := 0
	for i := 0; i < len(v); i++ {
		c := v[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			hexIsh++
		}
	}
	if len(v) >= 24 && hexIsh > len(v)/2 {
		return true
	}
	return false
}

// resolveDayShiftTodayAndNextMembers returns the users for today and the
// "next" day based on the provided base index and current cycle. baseIndex is
// interpreted the same way as in the Airflow DAGs: it is 0-based and used with
// modulo over the ordered groups.
func ResolveDayShiftTodayAndNextMembers(today time.Time, baseIndex int, DB *gorm.DB) (todayMembers []*domain.User, nextMembers []*domain.User, err error) {
	// Detect current cycle by date (same as DAG behavior)
	todayDate := today.Format("2006-01-02")
	var cidRow struct{ ID uint }
	if err = DB.Raw("SELECT id AS id FROM day_shift_cycles WHERE begin_date <= ? AND finish_date >= ? ORDER BY finish_date DESC, id DESC LIMIT 1", todayDate, todayDate).Scan(&cidRow).Error; err != nil {
		return nil, nil, err
	}
	if cidRow.ID == 0 {
		return []*domain.User{}, []*domain.User{}, nil
	}
	cycleID := cidRow.ID

	// Load ordered groups for the cycle
	var groups []struct {
		ID       uint
		Position int
	}
	if err = DB.Raw("SELECT id AS id, position AS position FROM day_shift_cycle_groups WHERE cycle_id=? ORDER BY position ASC, id ASC", cycleID).Scan(&groups).Error; err != nil {
		return nil, nil, err
	}
	if len(groups) == 0 {
		return []*domain.User{}, []*domain.User{}, nil
	}

	// configs.day_shift_current_index は 0 始まりのインデックスですが、
	// 実運用上「現在の担当」を 1 始まりのカウンタとして扱っているため、
	// ここでは 1 を引いた値を基準として today/next を解決する。
	// これにより、Airflow 側のインクリメントロジックと突き合わせたときに
	// 「今日」と「次のメンバー」が 1 つ後ろにずれる問題を防ぐ。
	idx := baseIndex - 1
	if idx < 0 {
		idx = 0
	}
	giToday := idx % len(groups)
	giNext := (idx + 1) % len(groups)
	gidToday := groups[giToday].ID
	gidNext := groups[giNext].ID

	// helper to load members for a group
	loadMembers := func(gid uint) ([]*domain.User, error) {
		var memberRows []struct {
			UserID uint
		}
		if err := DB.Raw("SELECT user_id AS user_id FROM day_shift_cycle_group_members WHERE group_id=? ORDER BY position ASC, id ASC", gid).Scan(&memberRows).Error; err != nil {
			return nil, err
		}
		if len(memberRows) == 0 {
			return []*domain.User{}, nil
		}
		ids := make([]uint, 0, len(memberRows))
		for _, mr := range memberRows {
			ids = append(ids, mr.UserID)
		}
		var users []*domain.User
		if err := DB.Where("id IN ? AND is_deleted = 0", ids).Find(&users).Error; err != nil {
			return nil, err
		}
		return users, nil
	}

	if todayMembers, err = loadMembers(gidToday); err != nil {
		return nil, nil, err
	}
	if nextMembers, err = loadMembers(gidNext); err != nil {
		return nil, nil, err
	}
	return todayMembers, nextMembers, nil
}

// Joins display names (falling back to email) for logs/Discord/email.
func JoinUserNames(users []*domain.User) string {
	names := make([]string, 0, len(users))
	for _, u := range users {
		name := strings.TrimSpace(u.DisplayName)
		if name == "" {
			name = u.Email
		}
		if name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

// SendDayShiftSkipEmail is currently a lightweight stub. The actual email
// rendering is handled by the email-compose service; here we simply log the
// intended recipients and dates for observability. Wiring a full mail send
// pipeline can be done later by following the composeLoginCodeGRPC pattern.
func SendDayShiftSkipEmail(ctx context.Context, today, next time.Time, todayMembers, nextMembers []*domain.User) error {
	subject := fmt.Sprintf("[Taskly] 日直スキップ: %s", today.Format("2006-01-02"))
	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category: "dayshift_skip_email",
		Message:  subject,
		Data: map[string]any{
			"today":        today.Format("2006-01-02"),
			"next":         next.Format("2006-01-02"),
			"todayMembers": JoinUserNames(todayMembers),
			"nextMembers":  JoinUserNames(nextMembers),
		},
		Level: sentry.LevelInfo,
	})
	return nil
}

func PtrTime(t time.Time) *time.Time { return &t }

// helpers moved from main.go (auth, crypto, email, ical, dayshift)
func Sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:])
}

func GenerateAPIKey() (plain string, hashed string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	plain = fmt.Sprintf("tsk_%x", buf)
	hashed = Sha256Hex([]byte(plain))
	return
}

func GenerateIcalToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("ict_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("ict_%x", buf)
}

func ParseAuthToken(authHeader string) string {
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

func UserFromSession(c *fiber.Ctx, DB *gorm.DB, RDB *redis.Client) bool {
	if u, ok := c.Locals("user").(*domain.User); ok && u != nil {
		return true
	}

	// Optional debug logging for auth flow (no secrets). Enable by setting DEBUG_AUTH=true
	debugAuth := strings.ToLower(strings.TrimSpace(Env("DEBUG_AUTH", ""))) == "true"
	raw := c.Get("Authorization")
	token := ParseAuthToken(raw)
	if token == "" {
		if debugAuth {
			log.Printf("[auth] %s %s missing Authorization", c.Method(), sanitizeURLPath(c.Path()))
		}
		return false
	}
	headerPlat := strings.ToLower(strings.TrimSpace(c.Get("X-Client-Platform")))
	uidStr, err := RDB.Get(c.Context(), "session:"+token).Result()
	if err != nil || strings.TrimSpace(uidStr) == "" {
		var session domain.Session
		if dbErr := DB.Where("(hashed_token = ? OR hashed_token = ?) AND (expires_at IS NULL OR expires_at > ?)", Sha256Hex([]byte(token)), token, time.Now()).Order("id DESC").First(&session).Error; dbErr != nil {
			if debugAuth {
				log.Printf("[auth] %s %s redis:miss db:miss", c.Method(), sanitizeURLPath(c.Path()))
			}
			return false
		}
		if headerPlat != "" && session.ClientPlatform != "" && !strings.EqualFold(headerPlat, session.ClientPlatform) {
			if debugAuth {
				log.Printf("[auth] %s %s platform mismatch header=%q db=%q", c.Method(), sanitizeURLPath(c.Path()), headerPlat, session.ClientPlatform)
			}
			return false
		}
		ttl := 30 * 24 * time.Hour
		if session.ExpiresAt != nil {
			rem := time.Until(*session.ExpiresAt)
			if rem > 0 && rem < ttl {
				ttl = rem
			}
		}
		_ = RDB.Set(c.Context(), "session:"+token, fmt.Sprintf("%d", session.UserID), ttl).Err()
		uidStr = fmt.Sprintf("%d", session.UserID)
		if debugAuth {
			log.Printf("[auth] %s %s redis:miss db:hit user=%s plat=%q", c.Method(), sanitizeURLPath(c.Path()), uidStr, session.ClientPlatform)
		}
	} else {
		if headerPlat != "" {
			var session domain.Session
			if err := DB.Where("hashed_token = ?", Sha256Hex([]byte(token))).Order("id DESC").First(&session).Error; err == nil {
				if session.ClientPlatform != "" && !strings.EqualFold(headerPlat, session.ClientPlatform) {
					if debugAuth {
						log.Printf("[auth] %s %s redis:hit platform mismatch header=%q db=%q", c.Method(), sanitizeURLPath(c.Path()), headerPlat, session.ClientPlatform)
					}
					return false
				}
			}
		}
		if debugAuth {
			log.Printf("[auth] %s %s redis:hit user=%s plat_header=%q", c.Method(), sanitizeURLPath(c.Path()), strings.TrimSpace(uidStr), headerPlat)
		}
	}
	var u domain.User
	blockSubquery := DB.Model(&domain.Block{}).
		Select("1").
		Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", uidStr, time.Now())
	if err := DB.Where("id = ? AND NOT EXISTS (?)", uidStr, blockSubquery).First(&u).Error; err != nil {
		if debugAuth {
			log.Printf("[auth] %s %s user not found or blocked id=%s", c.Method(), sanitizeURLPath(c.Path()), strings.TrimSpace(uidStr))
		}
		return false
	}
	c.Locals("user", &u)
	c.Locals("token", token)
	if debugAuth {
		log.Printf("[auth] %s %s OK user=%d plat_header=%q", c.Method(), sanitizeURLPath(c.Path()), u.ID, headerPlat)
	}
	return true
}

func IsAllowedPlatform(p string) bool {
	switch strings.ToLower(p) {
	case "web", "ios", "android", "mac", "windows":
		return true
	default:
		return false
	}
}

func GenerateSessionToken(uid uint, platform string) string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("sess_%d_%s_%d", uid, platform, time.Now().UnixNano())
	}
	enc := base64.RawURLEncoding.EncodeToString(b)
	return fmt.Sprintf("sess_%d_%s_%s", uid, platform, enc)
}

func RevokeOtherSessions(c *fiber.Ctx, userID uint, currentToken string, DB *gorm.DB, RDB *redis.Client) {
	key := fmt.Sprintf("user_sessions:%d", userID)
	toks, _ := RDB.SMembers(c.Context(), key).Result()
	for _, t := range toks {
		if t == currentToken || strings.TrimSpace(t) == "" {
			continue
		}
		_ = RDB.Del(c.Context(), "session:"+t).Err()
		_ = RDB.SRem(c.Context(), key, t).Err()
	}
	curHash := Sha256Hex([]byte(currentToken))
	_ = DB.Where("user_id = ? AND hashed_token <> ?", userID, curHash).Delete(&domain.Session{}).Error
}

func UserFromAPIKey(c *fiber.Ctx, DB *gorm.DB) bool {
	apiKey := c.Get("X-API-Key")
	if apiKey == "" {
		return false
	}
	hashed := Sha256Hex([]byte(apiKey))
	var key domain.APIKey
	if err := DB.Where("hashed_token = ? AND is_valid = 1 AND (expires_at IS NULL OR expires_at > ?)", hashed, time.Now()).First(&key).Error; err != nil {
		return false
	}
	var u domain.User
	if err := DB.First(&u, key.UserID).Error; err != nil {
		return false
	}
	var cnt int64
	if err := DB.Model(&domain.Block{}).Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", u.ID, time.Now()).Count(&cnt).Error; err == nil && cnt > 0 {
		return false
	}
	c.Locals("user", &u)
	c.Locals("apiKey", apiKey)
	return true
}

// 2FA helpers
func UrlEncode(s string) string { return url.QueryEscape(s) }

func GenerateBase32Secret() string {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	return strings.TrimRight(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), "=")
}

func ValidateTOTP(secret, code string) bool {
	return totp.Validate(code, secret)
}

func GenerateRecoveryCode() string {
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

func ConsumeRecoveryCode(userID uint, code string, DB *gorm.DB) bool {
	hashed := Sha256Hex([]byte(code))
	ok := false
	_ = DB.Transaction(func(tx *gorm.DB) error {
		var rc domain.RecoveryCode
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

func GetActiveTOTP(userID uint, DB *gorm.DB) (bool, string, error) {
	var ts domain.TOTPSecret
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
	keyStr := Env("TOTP_ENC_KEY", "dev_totp_key")
	sum := sha256.Sum256([]byte(keyStr))
	return sum[:]
}

func EncryptString(plain string) (string, error) {
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
func BuildICSForUser(userID uint, DB *gorm.DB) string {
	b := &strings.Builder{}
	crlf := "\r\n"
	now := time.Now().UTC().Format("20060102T150405Z")
	b.WriteString("BEGIN:VCALENDAR" + crlf)
	b.WriteString("VERSION:2.0" + crlf)
	b.WriteString("PRODID:-//Taskly//EN" + crlf)
	b.WriteString("CALSCALE:GREGORIAN" + crlf)
	b.WriteString("METHOD:PUBLISH" + crlf)
	var tasks []domain.Task
	// Order by effective deadline timestamp derived from date + enum
	eff := EffectiveDeadlineExpr("")
	_ = DB.Where("added_user_id = ?", userID).Order(eff + " ASC").Limit(500).Find(&tasks).Error
	for _, t := range tasks {
		// Treat tasks as all-day events in the calendar (use date-only DTSTART/DTEND)
		// Use the task.Deadline (date only) as the day; DTEND is exclusive and set to next day
		if t.Deadline.IsZero() {
			continue
		}

		var subj domain.Subject
		_ = DB.First(&subj, t.SubjectID).Error
		var u domain.User
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
		b.WriteString(fmt.Sprintf("SUMMARY:%s\r\n", ICalEscape("[課題] "+sName+": "+t.Title)))
		// Build description: include existing description and human-readable period label
		desc := ""
		if t.Description != nil && strings.TrimSpace(*t.Description) != "" {
			// Replace newlines with spaces per UX requirement
			desc = strings.TrimSpace(strings.ReplaceAll(*t.Description, "\n", " "))
		}
		// Build compact period label
		label := ""
		sIdx := periodIndex(t.DeadlineSince)
		uIdx := periodIndex(t.DeadlineUntil)
		if strings.EqualFold(strings.TrimSpace(t.DeadlineUntil), "MORNING") {
			label = "朝まで"
		} else if strings.EqualFold(strings.TrimSpace(t.DeadlineUntil), "LUNCH") {
			label = "昼休みまで"
		} else if strings.EqualFold(strings.TrimSpace(t.DeadlineUntil), "AFTER_SCHOOL") {
			label = "放課後まで"
		} else if sIdx > 0 && uIdx > 0 {
			if sIdx == uIdx {
				label = fmt.Sprintf("%d限まで", sIdx)
			} else if sIdx < uIdx {
				label = fmt.Sprintf("%d〜%d限まで", sIdx, uIdx)
			}
		} else if uIdx > 0 {
			label = fmt.Sprintf("%d限まで", uIdx)
		} else if sIdx > 0 {
			label = fmt.Sprintf("%d限以降", sIdx)
		}
		if strings.TrimSpace(label) != "" {
			if desc != "" {
				desc = desc + " " + label
			} else {
				desc = label
			}
		}
		b.WriteString(fmt.Sprintf("DESCRIPTION:%s\r\n", ICalEscape(desc)))
		// All-day DTSTART/DTEND (DTEND is exclusive — next day)
		dtStart := t.Deadline.Format("20060102")
		dtEnd := t.Deadline.AddDate(0, 0, 1).Format("20060102")
		b.WriteString(fmt.Sprintf("DTSTART;VALUE=DATE:%s\r\n", dtStart))
		b.WriteString(fmt.Sprintf("DTEND;VALUE=DATE:%s\r\n", dtEnd))
		b.WriteString("END:VEVENT" + crlf)
	}
	b.WriteString("END:VCALENDAR" + crlf)
	return b.String()
}

func ICalEscape(s string) string {
	r := strings.ReplaceAll(s, "\\", "\\\\")
	r = strings.ReplaceAll(r, ",", "\\,")
	r = strings.ReplaceAll(r, ";", "\\;")
	r = strings.ReplaceAll(r, "\n", "\\n")
	r = strings.ReplaceAll(r, "\r", "")
	return r
}

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
	dialer := &net.Dialer{Timeout: time.Duration(toSec) * time.Second}
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
	_ = conn.SetDeadline(time.Now().Add(time.Duration(toSec) * time.Second))
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

// Build a Japanese message describing changes between two Task states
func BuildTaskUpdateMessageJa(before domain.Task, after domain.Task, DB *gorm.DB) string {
	b := &strings.Builder{}
	fmt.Fprintf(b, "課題が更新されました (ID:%d)\n", after.ID)
	// Title
	if before.Title != after.Title {
		fmt.Fprintf(b, "・タイトル: '%s' → '%s'\n", before.Title, after.Title)
	}
	// Subject
	if before.SubjectID != after.SubjectID {
		fmt.Fprintf(b, "・教科: '%s' → '%s'\n", subjectNameJa(before.SubjectID, DB), subjectNameJa(after.SubjectID, DB))
	}
	// Description
	if (before.Description == nil) != (after.Description == nil) || (before.Description != nil && after.Description != nil && *before.Description != *after.Description) {
		fmt.Fprintf(b, "・説明: '%s' → '%s'\n", strOrUnsetJa(before.Description), strOrUnsetJa(after.Description))
	}
	// Type
	if !strings.EqualFold(before.Type, after.Type) {
		fmt.Fprintf(b, "・種別: '%s' → '%s'\n", displayTaskTypeJa(before.Type), displayTaskTypeJa(after.Type))
	}
	// Hand-in
	if before.IsRequireHandIn != after.IsRequireHandIn {
		fmt.Fprintf(b, "・要提出: %s → %s\n", displayBoolJa(before.IsRequireHandIn), displayBoolJa(after.IsRequireHandIn))
	}
	// Self check
	if before.IsSelfCheckRequired != after.IsSelfCheckRequired {
		fmt.Fprintf(b, "・自己チェック: %s → %s\n", displayBoolJa(before.IsSelfCheckRequired), displayBoolJa(after.IsSelfCheckRequired))
	}
	// Deadline date
	if !before.Deadline.Equal(after.Deadline) {
		fmt.Fprintf(b, "・期限日: %s → %s\n", before.Deadline.Format("2006-01-02"), after.Deadline.Format("2006-01-02"))
	}
	// Since / Until
	if !strings.EqualFold(before.DeadlineSince, after.DeadlineSince) {
		fmt.Fprintf(b, "・開始限: %s → %s\n", displaySinceJa(before.DeadlineSince), displaySinceJa(after.DeadlineSince))
	}
	if !strings.EqualFold(before.DeadlineUntil, after.DeadlineUntil) {
		fmt.Fprintf(b, "・終了限: %s → %s\n", displayUntilJa(before.DeadlineUntil), displayUntilJa(after.DeadlineUntil))
	}
	return b.String()
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

// Send Teams notification email for a newly created task
func SendTeamsNotifyEmailForTask(t domain.Task, DB *gorm.DB) {
	to := Env("TEAMS_NOTIFY_EMAIL", "")
	if strings.TrimSpace(to) == "" {
		return
	}
	subject := "[Taskly]tasks:add"
	formatted := buildTeamsTitleForTask(t, DB)
	desc := ""
	if t.Description != nil {
		desc = *t.Description
	}
	// JSON body consumed by Power Automate
	body := fmt.Sprintf(`{"task_id":%q,"title":%q,"description":%q,"sent_at":%q}`, fmt.Sprintf("%d", t.ID), formatted, desc, time.Now().Format(time.RFC3339))
	// Fire and forget
	go func() {
		_ = smtpSendPlain(to, subject, body)
	}()
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

// Send Teams reply email for an existing message (update/delete events)
func SendTeamsReplyEmail(taskID uint, messageID string, message string) {
	to := Env("TEAMS_NOTIFY_EMAIL", "")
	if strings.TrimSpace(to) == "" {
		return
	}
	subject := "[Taskly]tasks:reply"
	body := fmt.Sprintf(`{"message_id":%q,"task_id":%q,"message":%q}`, messageID, fmt.Sprintf("%d", taskID), message)
	go func() { _ = smtpSendPlain(to, subject, body) }()
}

// small helpers
func ToUint(s string) uint {
	v, _ := strconv.ParseUint(s, 10, 64)
	return uint(v)
}

// (removed obsolete TLS wrappers; smtpSend uses crypto/tls directly)

// Authorization and permission helpers
func UserHasRole(userID uint, role string, DB *gorm.DB) (bool, error) {
	var cnt int64
	if err := DB.Model(&domain.RoleUser{}).Where("user_id = ? AND role = ?", userID, role).Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}

func ApiKeyLimitForUser(userID uint, DB *gorm.DB) (int, error) {
	if ok, err := UserHasRole(userID, "admin", DB); err != nil {
		return 0, err
	} else if ok {
		return -1, nil
	}
	if ok, err := UserHasRole(userID, "manager", DB); err != nil {
		return 0, err
	} else if ok {
		return 100, nil
	}
	var p domain.Permission
	if err := DB.Where("user_id = ? AND target = ?", userID, "api_key_limit").Order("id DESC").First(&p).Error; err == nil && p.Value != nil {
		var v int
		fmt.Sscanf(*p.Value, "%d", &v)
		if v >= 0 {
			return v, nil
		}
	}
	return 10, nil
}

func CheckPermission(userID uint, target, action string, ownerID *uint, DB *gorm.DB) (bool, error) {
	if ok, err := UserHasRole(userID, "admin", DB); err != nil {
		return false, err
	} else if ok {
		return true, nil
	}
	if ok, err := UserHasRole(userID, "manager", DB); err != nil {
		return false, err
	} else if ok {
		return true, nil
	}
	var p domain.Permission
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

// Posts one or more embeds to the Discord webhook. Logs basic diagnostics.
func SendDiscordEmbeds(embeds []map[string]any) {
	hook := Env("DISCORD_WEBHOOK_URL", "")
	if strings.TrimSpace(hook) == "" {
		log.Printf("discord: skipped (no DISCORD_WEBHOOK_URL)")
		return
	}
	if len(embeds) == 0 {
		log.Printf("discord: skipped (no embeds)")
		return
	}
	payloadObj := map[string]any{"embeds": embeds}
	b, err := json.Marshal(payloadObj)
	if err != nil {
		log.Printf("discord: marshal error: %v", err)
		return
	}
	req, err := http.NewRequest("POST", hook, bytes.NewReader(b))
	if err != nil {
		log.Printf("discord: build request error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "taskly-server/1.0")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("discord: post error: %v", err)
		return
	}
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("discord: non-2xx status: %d", resp.StatusCode)
		return
	}
	log.Printf("discord: embeds sent ok")
}

// SendDiscordTaskEmbed builds and sends an embed for task events (created/updated/deleted)
func SendDiscordTaskEmbed(action string, t domain.Task, actor string) {
	// Prepare fields
	webURL := strings.TrimSpace(Env("WEB_URL", ""))
	title := "Task"
	color := 5814783 // default bluish
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "created", "create":
		title = "Task Created"
		color = 0x2ecc71 // green
	case "updated", "update":
		title = "Task Updated"
		color = 0xf1c40f // yellow
	case "deleted", "delete":
		title = "Task Deleted"
		color = 0xe74c3c // red
	}
	// Human-readable label for since/until to avoid NONE/NONE display
	suLabel := deriveDeadlineLabelEnum(t.DeadlineSince, t.DeadlineUntil)
	if strings.TrimSpace(suLabel) == "" {
		suLabel = "指定なし"
	}
	fields := []map[string]any{
		{"name": "ID", "value": fmt.Sprintf("%d", t.ID), "inline": true},
		{"name": "Deadline", "value": t.Deadline.Format("2006-01-02"), "inline": true},
		{"name": "Window", "value": suLabel, "inline": true},
		{"name": "By", "value": actor, "inline": true},
	}
	// Optional link to web app (no specific task page, so link to home if provided)
	embed := map[string]any{
		"title":       title,
		"description": t.Title,
		"color":       color,
		"fields":      fields,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	}
	if webURL != "" {
		embed["url"] = strings.TrimRight(webURL, "/") + "/home"
	}
	SendDiscordEmbeds([]map[string]any{embed})
}

func DateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// Decide whether to rotate day shift based on Gemini API analysis of subject/body.
// Returns true when rotation/skip should occur. This version preserves the
// legacy signature and logic; higher-level code is responsible for deciding
// whether the rotation is "today" or "tomorrow" based on additional cues.
func ShouldRotateByGemini(subject, body string) bool {
	// 1) Strong heuristic first (Japanese keywords)
	s := strings.ToLower(subject + "\n" + body)
	if shouldRotateHeuristic(s) {
		return true
	}
	// 2) If Gemini API key is configured, ask the model; parse JSON result safely
	if Env("GEMINI_API_KEY", "") == "" {
		// No API key and heuristic didn't match → do not rotate
		return false
	}
	// Allow overriding model via env; default to gemini-2.5-flash to match current API examples
	modelName := strings.TrimSpace(Env("GEMINI_MODEL", "gemini-2.5-flash"))
	ctx := context.Background()
	cli, err := genai.NewClient(ctx, nil)
	if err == nil {
		prompt := "以下のメール内容が『当日日直の延期・振替・順延・スライド（＝日直をずらす）』を求めているか判定してください。回答は YES または NO のみ。\nSubject: " + subject + "\nBody: " + body
		content := genai.NewContentFromText(prompt, "")
		resp, err := cli.Models.GenerateContent(ctx, modelName, []*genai.Content{content}, nil)
		if err == nil && resp != nil {
			ans := strings.ToUpper(resp.Text())
			if strings.Contains(ans, "YES") || strings.Contains(ans, "ＹＥＳ") || strings.Contains(ans, "はい") {
				return true
			}
			if strings.Contains(ans, "NO") || strings.Contains(ans, "ＮＯ") || strings.Contains(ans, "いいえ") {
				return false
			}
		}
	}
	// 3) Fallback: if model response ambiguous, rely on heuristic again
	return shouldRotateHeuristic(s)
}

// shouldRotateHeuristic detects Japanese phrases that imply postponing/rescheduling day shift
func shouldRotateHeuristic(s string) bool {
	// Normalize (remove spaces)
	s = strings.ReplaceAll(s, " ", "")
	// Positive signals
	positives := []string{
		"延期", "順延", "日程変更", "変更します", "変更したい", "変更を希望", "スライド", "ずらし", "ずらせ", "ずらしたい",
		"振替", "振り替え", "別日に", "後ろ倒し", "先送り", "移動したい", "日直を変", "交代をお願い", "交代したい",
		"日直変更", "日直ずらし", "日直延期", "都合が悪いので", "本日の日直を延期", "今日の", "日直を延期",
	}
	for _, kw := range positives {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// Shift all assignments from the next business day after 'start' one business day forward,
// and ensure today's users are also assigned to that next business day.

// Legacy backward-shift helper also depended on day_shift_assignments; keep signature but no-op.

// Reports error to Sentry if DSN is configured
func CaptureError(err error) {
	if err == nil {
		return
	}
	if Env("SENTRY_DSN", "") == "" {
		return
	}
	sentry.CaptureException(err)
}
