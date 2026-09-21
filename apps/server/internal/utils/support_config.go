package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"log"
	"net/mail"
	"os"
	"strings"
	"time"
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
