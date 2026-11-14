package utils

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/authsync"
	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/gofiber/fiber/v2"
	"github.com/pquerna/otp/totp"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	SessionCookieName = "taskly_session"
	// X-Client-Platform is a non-safelisted header that the production nginx
	// preflight already permits. Requiring it in addition to an exact Origin
	// check prevents cross-origin form submissions from using the cookie.
	CSRFHeaderName  = "X-Client-Platform"
	CSRFHeaderValue = "web"
	SessionTTL      = 30 * 24 * time.Hour
)

// helpers moved from main.go (auth, crypto, email, ical, dayshift)
func Sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:])
}

func GenerateAPIKey() (plain string, hashed string, err error) {
	buf := make([]byte, 32)
	if _, err = io.ReadFull(rand.Reader, buf); err != nil {
		return "", "", err
	}
	plain = fmt.Sprintf("tsk_%x", buf)
	hashed = Sha256Hex([]byte(plain))
	return
}

func GenerateIcalToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("ict_%x", buf), nil
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

// SessionTokenFromRequest accepts the HttpOnly cookie used by the web client
// and the Authorization header used by native clients. If both are present,
// they must identify the same session to avoid ambiguous authentication.
func SessionTokenFromRequest(c *fiber.Ctx) string {
	cookieToken := strings.TrimSpace(c.Cookies(SessionCookieName))
	bearerToken := ParseAuthToken(c.Get("Authorization"))
	if cookieToken != "" && bearerToken != "" && subtle.ConstantTimeCompare([]byte(cookieToken), []byte(bearerToken)) != 1 {
		return ""
	}
	if cookieToken != "" {
		c.Locals("session_auth_method", "cookie")
		return cookieToken
	}
	if bearerToken != "" {
		c.Locals("session_auth_method", "bearer")
	}
	return bearerToken
}

func SessionCookieSecure() bool {
	if strings.ToLower(Env("NODE_ENV", "development")) == "production" {
		return true
	}
	if raw := strings.TrimSpace(os.Getenv("SESSION_COOKIE_SECURE")); raw != "" {
		return strings.EqualFold(raw, "true") || raw == "1"
	}
	return false
}

func SetSessionCookie(c *fiber.Ctx, token string, expires time.Time) {
	c.Cookie(&fiber.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		Secure:   SessionCookieSecure(),
		HTTPOnly: true,
		SameSite: fiber.CookieSameSiteLaxMode,
	})
}

func ClearSessionCookie(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		Secure:   SessionCookieSecure(),
		HTTPOnly: true,
		SameSite: fiber.CookieSameSiteLaxMode,
	})
}

func AllowedWebOrigins() map[string]struct{} {
	raw := Env("API_CORS_ORIGINS", "http://localhost:4321,http://127.0.0.1:4321,http://localhost:3000,http://127.0.0.1:3000,http://localhost:5173,http://127.0.0.1:5173")
	allowed := make(map[string]struct{})
	for _, origin := range strings.Split(raw, ",") {
		origin = strings.TrimSpace(strings.TrimSuffix(origin, "/"))
		if origin != "" && origin != "*" {
			allowed[origin] = struct{}{}
		}
	}
	return allowed
}

func IsAllowedWebOrigin(origin string) bool {
	origin = strings.TrimSpace(strings.TrimSuffix(origin, "/"))
	if origin == "" {
		return false
	}
	_, ok := AllowedWebOrigins()[origin]
	return ok
}

// RequireCookieCSRF protects unsafe requests authenticated by the browser
// cookie. Bearer and API-key clients remain unaffected.
func RequireCookieCSRF(c *fiber.Ctx) error {
	if strings.TrimSpace(c.Cookies(SessionCookieName)) == "" || c.Method() == fiber.MethodGet || c.Method() == fiber.MethodHead || c.Method() == fiber.MethodOptions {
		return c.Next()
	}
	if !IsAllowedWebOrigin(c.Get("Origin")) || subtle.ConstantTimeCompare([]byte(c.Get(CSRFHeaderName)), []byte(CSRFHeaderValue)) != 1 {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "csrf validation failed"})
	}
	return c.Next()
}

func RevokeSession(c *fiber.Ctx, token string, userID uint, DB *gorm.DB, RDB *redis.Client) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	var failures []error
	if err := DB.Where("hashed_token = ? OR hashed_token = ?", Sha256Hex([]byte(token)), token).Delete(&domain.Session{}).Error; err != nil {
		failures = append(failures, fmt.Errorf("delete persisted session: %w", err))
	}
	if err := RDB.Del(c.UserContext(), "session:"+token).Err(); err != nil {
		failures = append(failures, fmt.Errorf("delete cached session: %w", err))
	}
	if userID != 0 {
		if err := RDB.SRem(c.UserContext(), fmt.Sprintf("user_sessions:%d", userID), token).Err(); err != nil {
			failures = append(failures, fmt.Errorf("remove session index: %w", err))
		}
	}
	return errors.Join(failures...)
}

// SoftDeleteUser revokes persisted credentials together with the account update.
// Redis cleanup is best effort: every authentication path also checks is_deleted.
func SoftDeleteUser(ctx context.Context, userID uint, DB *gorm.DB, RDB *redis.Client) error {
	deletedAt := time.Now().UTC().Truncate(time.Second)
	if err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&domain.User{}).Where("id = ? AND is_deleted = 0", userID).Updates(map[string]any{"is_deleted": true, "deleted_at": deletedAt}).Error; err != nil {
			return err
		}
		return revokeUserCredentials(tx, userID)
	}); err != nil {
		return err
	}
	ClearUserSessionCache(ctx, userID, RDB)
	// Serialize lifecycle delivery with restoration. A restore may have completed
	// after the local deletion committed; do not send an obsolete deletion then.
	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user domain.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userID).Error; err != nil {
			return err
		}
		if !user.IsDeleted {
			return nil
		}
		if user.DeletedAt != nil {
			deletedAt = user.DeletedAt.UTC()
		}
		return authsync.SyncDeleted(ctx, userID, deletedAt)
	})
}

// RestoreUser locks the app account against erasure and synchronizes auth first.
// A failed synchronization leaves the app account deleted; retries are safe.
func RestoreUser(ctx context.Context, userID uint, DB *gorm.DB, RDB *redis.Client) error {
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user domain.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userID).Error; err != nil {
			return err
		}
		if !user.IsDeleted {
			return nil
		}
		if err := revokeUserCredentials(tx, userID); err != nil {
			return err
		}
		if err := authsync.SyncRestored(ctx, userID); err != nil {
			return err
		}
		return tx.Model(&user).Updates(map[string]any{"is_deleted": false, "deleted_at": nil}).Error
	})
	if err == nil {
		ClearUserSessionCache(ctx, userID, RDB)
	}
	return err
}

func revokeUserCredentials(tx *gorm.DB, userID uint) error {
	if err := tx.Where("user_id = ?", userID).Delete(&domain.Session{}).Error; err != nil {
		return err
	}
	if err := tx.Model(&domain.APIKey{}).Where("user_id = ?", userID).Update("is_valid", false).Error; err != nil {
		return err
	}
	if err := tx.Where("user_id = ?", userID).Delete(&domain.IcalKey{}).Error; err != nil {
		return err
	}
	return tx.Where("user_id = ?", userID).Delete(&domain.MCP{}).Error
}

func ClearUserSessionCache(ctx context.Context, userID uint, RDB *redis.Client) {
	if RDB == nil {
		return
	}
	key := fmt.Sprintf("user_sessions:%d", userID)
	if tokens, err := RDB.SMembers(ctx, key).Result(); err == nil {
		for _, token := range tokens {
			_ = RDB.Del(ctx, "session:"+token).Err()
		}
	}
	_ = RDB.Del(ctx, key).Err()
}

func UserFromSession(c *fiber.Ctx, DB *gorm.DB, RDB *redis.Client) bool {
	if u, ok := c.Locals("user").(*domain.User); ok && u != nil {
		return true
	}

	// Optional debug logging for auth flow (no secrets). Enable by setting DEBUG_AUTH=true
	debugAuth := strings.ToLower(strings.TrimSpace(Env("DEBUG_AUTH", ""))) == "true"
	token := SessionTokenFromRequest(c)
	if token == "" {
		if debugAuth {
			log.Printf("[auth] %s %s missing or ambiguous session credential", c.Method(), sanitizeURLPath(c.Path()))
		}
		return false
	}
	headerPlat := strings.ToLower(strings.TrimSpace(c.Get("X-Client-Platform")))
	// The persisted row is authoritative. A cached Redis entry may outlive a
	// revocation or be repopulated after cleanup, so it cannot grant access.
	var session domain.Session
	if err := DB.Where("(hashed_token = ? OR hashed_token = ?) AND (expires_at IS NULL OR expires_at > ?)", Sha256Hex([]byte(token)), token, time.Now()).Order("id DESC").First(&session).Error; err != nil {
		_ = RDB.Del(c.UserContext(), "session:"+token).Err()
		if debugAuth {
			log.Printf("[auth] %s %s db:miss", c.Method(), sanitizeURLPath(c.Path()))
		}
		return false
	}
	if headerPlat != "" && session.ClientPlatform != "" && !strings.EqualFold(headerPlat, session.ClientPlatform) {
		if debugAuth {
			log.Printf("[auth] %s %s platform mismatch header=%q db=%q", c.Method(), sanitizeURLPath(c.Path()), headerPlat, session.ClientPlatform)
		}
		return false
	}
	uidStr := fmt.Sprintf("%d", session.UserID)
	ttl := SessionTTL
	if session.ExpiresAt != nil {
		rem := time.Until(*session.ExpiresAt)
		if rem > 0 && rem < ttl {
			ttl = rem
		}
	}
	_ = RDB.Set(c.UserContext(), "session:"+token, uidStr, ttl).Err()
	if debugAuth {
		log.Printf("[auth] %s %s db:hit user=%s plat=%q", c.Method(), sanitizeURLPath(c.Path()), uidStr, session.ClientPlatform)
	}
	var u domain.User
	blockSubquery := DB.Model(&domain.Block{}).
		Select("1").
		Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", uidStr, time.Now())
	if err := DB.Where("id = ? AND is_deleted = 0 AND NOT EXISTS (?)", uidStr, blockSubquery).First(&u).Error; err != nil {
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

func GenerateSessionToken(uid uint, platform string) (string, error) {
	b := make([]byte, 24)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding.EncodeToString(b)
	return fmt.Sprintf("sess_%d_%s_%s", uid, platform, enc), nil
}

func RevokeOtherSessions(c *fiber.Ctx, userID uint, currentToken string, DB *gorm.DB, RDB *redis.Client) error {
	curHash := Sha256Hex([]byte(currentToken))
	if err := DB.Where("user_id = ? AND hashed_token <> ? AND hashed_token <> ?", userID, curHash, currentToken).Delete(&domain.Session{}).Error; err != nil {
		return fmt.Errorf("delete other persisted sessions: %w", err)
	}
	// Cached entries do not grant access without a persisted row. Clean them up
	// best effort after the database operation succeeds.
	key := fmt.Sprintf("user_sessions:%d", userID)
	toks, err := RDB.SMembers(c.UserContext(), key).Result()
	if err != nil {
		CaptureError(fmt.Errorf("list cached sessions for cleanup: %w", err))
		return nil
	}
	for _, t := range toks {
		if t == currentToken || strings.TrimSpace(t) == "" {
			continue
		}
		if err := RDB.Del(c.UserContext(), "session:"+t).Err(); err != nil {
			CaptureError(fmt.Errorf("delete cached session: %w", err))
		}
		if err := RDB.SRem(c.UserContext(), key, t).Err(); err != nil {
			CaptureError(fmt.Errorf("remove cached session index: %w", err))
		}
	}
	return nil
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
	if err := DB.Where("id = ? AND is_deleted = 0", key.UserID).First(&u).Error; err != nil {
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

// UserFromMCPToken authenticates the dedicated MCP task bridge. MCP tokens
// never grant access to the public API key or GraphQL surfaces.
func UserFromMCPToken(c *fiber.Ctx, DB *gorm.DB) bool {
	token := c.Get("X-MCP-Token")
	if token == "" {
		return false
	}
	var credential domain.MCP
	if err := DB.Where("hashed_token = ? AND (expires_at IS NULL OR expires_at > ?)", Sha256Hex([]byte(token)), time.Now()).First(&credential).Error; err != nil {
		return false
	}
	var user domain.User
	if err := DB.Where("id = ? AND is_deleted = 0", credential.UserID).First(&user).Error; err != nil {
		return false
	}
	var blocks int64
	if err := DB.Model(&domain.Block{}).Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", user.ID, time.Now()).Count(&blocks).Error; err != nil || blocks > 0 {
		return false
	}
	c.Locals("user", &user)
	c.Locals("mcpToken", true)
	return true
}

// 2FA helpers
func UrlEncode(s string) string { return url.QueryEscape(s) }

func GenerateBase32Secret() (string, error) {
	b := make([]byte, 20)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return strings.TrimRight(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), "="), nil
}

func ValidateTOTP(secret, code string) bool {
	return totp.Validate(code, secret)
}

func GenerateRecoveryCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	const groups = 4
	const perGroup = 4
	out := make([]byte, 0, groups*perGroup+(groups-1))
	buf := make([]byte, groups*perGroup)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", err
	}
	for i := 0; i < groups*perGroup; i++ {
		out = append(out, alphabet[int(buf[i])%len(alphabet)])
		if (i+1)%perGroup == 0 && i+1 < groups*perGroup {
			out = append(out, '-')
		}
	}
	return string(out), nil
}

func ConsumeRecoveryCode(userID uint, code string, DB *gorm.DB) bool {
	hashed := Sha256Hex([]byte(code))
	result := DB.Model(&domain.RecoveryCode{}).
		Where("user_id = ? AND hashed_code = ? AND is_used = 0", userID, hashed).
		Update("is_used", true)
	return result.Error == nil && result.RowsAffected == 1
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
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
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
