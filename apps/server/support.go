package main

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

	"github.com/getsentry/sentry-go"
	"github.com/gofiber/fiber/v2"
	"github.com/pquerna/otp/totp"
	"google.golang.org/genai"
	"gorm.io/gorm"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// --- JSON parsing & validation helpers ---
func parseJSONStrict(c *fiber.Ctx, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(c.Body()))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func validationError(c *fiber.Ctx, fields map[string]string) error {
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "validation", "fields": fields})
}

func isEmail(s string) bool {
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

func maxLen(s string, n int) bool { return len(s) <= n }

func isEnum(v string, allowed ...string) bool {
	for _, a := range allowed {
		if strings.EqualFold(v, a) {
			return true
		}
	}
	return false
}

func matchPattern(s string, allowed func(r rune) bool) bool {
	for _, r := range s {
		if !allowed(r) {
			return false
		}
	}
	return true
}

// --- Scrubbing helpers ---
// Sanitize URL path by masking token-like segments such as /ical/{token}.ics or Bearer tokens in query
func sanitizeURLPath(u string) string {
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

// Sanitize query string by masking sensitive parameters
func sanitizeQueryString(q string) string {
	if strings.TrimSpace(q) == "" {
		return q
	}
	// try to parse lightly as k=v&k2=v2 (without url decoding)
	parts := strings.Split(q, "&")
	for i, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) == 2 {
			k := strings.ToLower(kv[0])
			if isSensitiveKey(k) || isSensitiveValue(kv[1]) {
				parts[i] = kv[0] + "=[REDACTED]"
			}
		}
	}
	return strings.Join(parts, "&")
}

// Scrub values in a map[string]any recursively when keys or values look sensitive
func scrubPIIMap(m map[string]any) {
	for k, v := range m {
		lk := strings.ToLower(k)
		if isSensitiveKey(lk) {
			m[k] = "[REDACTED]"
			continue
		}
		switch t := v.(type) {
		case string:
			if isSensitiveValue(t) {
				m[k] = "[REDACTED]"
			}
		case map[string]any:
			scrubPIIMap(t)
		case map[string]string:
			for kk, vv := range t {
				if isSensitiveKey(kk) || isSensitiveValue(vv) {
					t[kk] = "[REDACTED]"
				}
			}
		}
	}
}

func isSensitiveKey(k string) bool {
	k = strings.ToLower(strings.TrimSpace(k))
	switch k {
	case "authorization", "x-api-key", "api_key", "apikey", "password", "passwd", "secret", "token", "session", "cookie", "set-cookie", "refresh_token", "access_token":
		return true
	default:
		return false
	}
}

func isSensitiveValue(v string) bool {
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
	hexish := 0
	for i := 0; i < len(v); i++ {
		c := v[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			hexish++
		}
	}
	if len(v) >= 24 && hexish > len(v)/2 {
		return true
	}
	return false
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

// ensureTasksTeamsLogsTable creates the tasks_teams_logs table if it does not exist yet.
// This protects integrations that write to this table even when external SQL migrations are not applied.
func ensureTasksTeamsLogsTable() {
	// Create without FK to avoid cross-environment mismatches; add index on task_id
	const ddl = `CREATE TABLE IF NOT EXISTS tasks_teams_logs (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  task_id BIGINT UNSIGNED NOT NULL,
  teams_message_id VARCHAR(255) NULL,
  teams_conversation_id VARCHAR(255) NULL,
  is_success BOOLEAN NOT NULL DEFAULT FALSE,
  callback_url TEXT NULL,
  attempts INT NOT NULL DEFAULT 0,
  last_status VARCHAR(32) NULL,
  last_error TEXT NULL,
  last_response_status INT NULL,
  last_response_body TEXT NULL,
  INDEX idx_tasks_teams_logs_task_id (task_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`
	if err := DB.Exec(ddl).Error; err != nil {
		log.Printf("migration warning: create tasks_teams_logs failed: %v", err)
	}
}

// runMigrations executes SQL migrations from migrations folder if not already applied
func runMigrations() {
	type migrationFile struct {
		name       string
		statements []string
	}

	// small helper to check column existence
	hasColumn := func(table, column string) bool {
		type res struct{ Cnt int }
		var r res
		_ = DB.Raw("SELECT COUNT(1) AS cnt FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?", table, column).Scan(&r).Error
		return r.Cnt > 0
	}

	migrations := []migrationFile{
		{
			name: "0019_email_suppressions",
			statements: []string{
				`CREATE TABLE IF NOT EXISTS email_suppressions (
					id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
					email VARCHAR(255) NOT NULL,
					reason TEXT NULL,
					suspended_until DATETIME NOT NULL,
					created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					INDEX idx_email_suppressions_email (email),
					INDEX idx_email_suppressions_until (suspended_until)
				) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
			},
		},
		{
			name: "0018_api_mcp_expiry",
			statements: []string{
				// Add expires_at to api_keys and mcps (nullable)
				"ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS expires_at DATETIME NULL",
				"ALTER TABLE mcps ADD COLUMN IF NOT EXISTS expires_at DATETIME NULL",
			},
		},
		{
			name: "0017_system_settings",
			statements: []string{
				`CREATE TABLE IF NOT EXISTS system_settings (
					id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
					` + "`key`" + ` VARCHAR(128) NOT NULL UNIQUE,
					value TEXT NOT NULL,
					created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
				) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
			},
		},
		{
			name: "0016_moderation_flags",
			statements: []string{
				// Add is_hidden to tasks if missing
				"ALTER TABLE tasks ADD COLUMN IF NOT EXISTS is_hidden BOOLEAN NOT NULL DEFAULT 0",
				// Create flagged_contents table
				`CREATE TABLE IF NOT EXISTS flagged_contents (
					id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
					task_id BIGINT UNSIGNED NOT NULL,
					category VARCHAR(64) NOT NULL,
					severity VARCHAR(16) NOT NULL,
					score DECIMAL(5,3) NULL,
					threat BOOLEAN NOT NULL DEFAULT FALSE,
					reason TEXT NULL,
					model VARCHAR(64) NULL,
					metadata JSON NULL,
					snapshot JSON NULL,
					created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					INDEX idx_flagged_contents_task_id (task_id),
					INDEX idx_flagged_contents_created_at (created_at)
				) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
				// Create user_reports table
				`CREATE TABLE IF NOT EXISTS user_reports (
					id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
					reporter_user_id BIGINT UNSIGNED NULL,
					task_id BIGINT UNSIGNED NOT NULL,
					reason VARCHAR(255) NOT NULL,
					comment TEXT NULL,
					created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					INDEX idx_user_reports_task_id (task_id),
					INDEX idx_user_reports_reporter (reporter_user_id)
				) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
			},
		},
		{
			name: "0012_timestamp_sanitize",
			statements: []string{
				// Temporarily relax NO_ZERO_DATE validation so we can fix bad data
				"SET @OLD_SQL_MODE = @@sql_mode",
				"SET SESSION sql_mode=(SELECT REPLACE(@@sql_mode,'NO_ZERO_DATE',''))",
				"SET SESSION sql_mode=(SELECT REPLACE(@@sql_mode,'NO_ZERO_IN_DATE',''))",
				// Normalize existing rows that may have invalid date/time values before altering columns
				"UPDATE users SET created_at = '1970-01-01 00:00:00' WHERE created_at IS NULL OR created_at = '0000-00-00 00:00:00'",
				"UPDATE tasks SET created_at = '1970-01-01 00:00:00' WHERE created_at IS NULL OR created_at = '0000-00-00 00:00:00'",
				"UPDATE tasks SET updated_at = IFNULL(updated_at, created_at) WHERE updated_at IS NULL OR updated_at = '0000-00-00 00:00:00'",
				"UPDATE api_keys SET created_at = '1970-01-01 00:00:00' WHERE created_at IS NULL OR created_at = '0000-00-00 00:00:00'",
				"UPDATE api_keys SET updated_at = IFNULL(updated_at, created_at) WHERE updated_at IS NULL OR updated_at = '0000-00-00 00:00:00'",
				"UPDATE permissions SET created_at = '1970-01-01 00:00:00' WHERE created_at IS NULL OR created_at = '0000-00-00 00:00:00'",
				"UPDATE permissions SET updated_at = IFNULL(updated_at, created_at) WHERE updated_at IS NULL OR updated_at = '0000-00-00 00:00:00'",
				"UPDATE blocks SET created_at = '1970-01-01 00:00:00' WHERE created_at IS NULL OR created_at = '0000-00-00 00:00:00'",
				"UPDATE blocks SET updated_at = IFNULL(updated_at, created_at) WHERE updated_at IS NULL OR updated_at = '0000-00-00 00:00:00'",
				"UPDATE role_users SET created_at = '1970-01-01 00:00:00' WHERE created_at IS NULL OR created_at = '0000-00-00 00:00:00'",
				"UPDATE passkeys SET created_at = '1970-01-01 00:00:00' WHERE created_at IS NULL OR created_at = '0000-00-00 00:00:00'",
				"UPDATE passkeys SET updated_at = IFNULL(updated_at, created_at) WHERE updated_at IS NULL OR updated_at = '0000-00-00 00:00:00'",
				"UPDATE day_shift_assignments SET created_at = '1970-01-01 00:00:00' WHERE created_at IS NULL OR created_at = '0000-00-00 00:00:00'",
				"UPDATE day_shift_assignments SET updated_at = IFNULL(updated_at, created_at) WHERE updated_at IS NULL OR updated_at = '0000-00-00 00:00:00'",
				"UPDATE business_calendar SET created_at = '1970-01-01 00:00:00' WHERE created_at IS NULL OR created_at = '0000-00-00 00:00:00'",
				"UPDATE business_calendar SET updated_at = IFNULL(updated_at, created_at) WHERE updated_at IS NULL OR updated_at = '0000-00-00 00:00:00'",
				"UPDATE web_push_subscriptions SET created_at = '1970-01-01 00:00:00' WHERE created_at IS NULL OR created_at = '0000-00-00 00:00:00'",
				"UPDATE web_push_subscriptions SET updated_at = IFNULL(updated_at, created_at) WHERE updated_at IS NULL OR updated_at = '0000-00-00 00:00:00'",
				// Restore SQL mode
				"SET SESSION sql_mode=@OLD_SQL_MODE",
			},
		},
		{
			name: "0011_timestamp_defaults",
			statements: []string{
				// Ensure DB-level defaults and ON UPDATE for updated_at across core tables
				"ALTER TABLE sessions MODIFY COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, MODIFY COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP",
				"ALTER TABLE api_keys MODIFY COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, MODIFY COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP",
				"ALTER TABLE permissions MODIFY COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, MODIFY COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP",
				"ALTER TABLE blocks MODIFY COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, MODIFY COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP",
				"ALTER TABLE tasks MODIFY COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, MODIFY COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP",
				"ALTER TABLE passkeys MODIFY COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, MODIFY COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP",
				"ALTER TABLE day_shift_assignments MODIFY COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, MODIFY COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP",
				"ALTER TABLE business_calendar MODIFY COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, MODIFY COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP",
				"ALTER TABLE role_users MODIFY COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP",
				// Tables created via AutoMigrate (ensure correct defaults)
				"ALTER TABLE web_push_subscriptions MODIFY COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, MODIFY COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP",
			},
		},
		{
			name: "0013_role_users_timestamp_fix",
			statements: []string{
				"SET @OLD_SQL_MODE = @@sql_mode",
				"SET SESSION sql_mode=(SELECT REPLACE(@@sql_mode,'NO_ZERO_DATE',''))",
				"SET SESSION sql_mode=(SELECT REPLACE(@@sql_mode,'NO_ZERO_IN_DATE',''))",
				"UPDATE role_users SET created_at = '1970-01-01 00:00:00' WHERE created_at IS NULL OR created_at = '0000-00-00 00:00:00'",
				"ALTER TABLE role_users MODIFY COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP",
				"SET SESSION sql_mode=@OLD_SQL_MODE",
			},
		},
		{
			name: "0014_subjects_timestamp_fix",
			statements: []string{
				"SET @OLD_SQL_MODE = @@sql_mode",
				"SET SESSION sql_mode=(SELECT REPLACE(@@sql_mode,'NO_ZERO_DATE',''))",
				"SET SESSION sql_mode=(SELECT REPLACE(@@sql_mode,'NO_ZERO_IN_DATE',''))",
				"UPDATE subjects SET created_at = '1970-01-01 00:00:00' WHERE created_at IS NULL OR created_at = '0000-00-00 00:00:00'",
				"ALTER TABLE subjects MODIFY COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP",
				"SET SESSION sql_mode=@OLD_SQL_MODE",
			},
		},
		{
			name: "0015_migrations_applied_at_fix",
			statements: []string{
				"SET @OLD_SQL_MODE = @@sql_mode",
				"SET SESSION sql_mode=(SELECT REPLACE(@@sql_mode,'NO_ZERO_DATE',''))",
				"SET SESSION sql_mode=(SELECT REPLACE(@@sql_mode,'NO_ZERO_IN_DATE',''))",
				"UPDATE migrations SET applied_at = NOW() WHERE applied_at IS NULL OR applied_at = '0000-00-00 00:00:00'",
				"ALTER TABLE migrations MODIFY COLUMN applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP",
				"SET SESSION sql_mode=@OLD_SQL_MODE",
			},
		},
		{
			name: "0010_business_calendar_date_index",
			statements: []string{
				// Create index if missing. MySQL 8 supports IF NOT EXISTS for indexes from 8.0.13+, but safer to try and ignore duplicate errors.
				"CREATE INDEX idx_business_calendar_date ON business_calendar(date)",
			},
		},
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
				// MySQL 8.0+: IF NOT EXISTS is supported; harmless if already present
				"ALTER TABLE subjects ADD COLUMN IF NOT EXISTS name_en VARCHAR(255) NULL",
			},
		},
		{
			name: "0007_subject_name_ja_backfill",
			statements: []string{
				// Add name_ja nullable first (ignore if exists)
				"ALTER TABLE subjects ADD COLUMN IF NOT EXISTS name_ja VARCHAR(255) NULL",
				// Backfill from legacy title where empty (only if legacy 'title' exists)
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
				// Drop legacy title column if exists
				"ALTER TABLE subjects DROP COLUMN IF EXISTS title",
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
		// Determine presence of legacy column once per migration
		titleExists := hasColumn("subjects", "title")
		for _, stmt := range m.statements {
			// Skip statements that reference legacy 'title' when it doesn't exist
			if !titleExists && (strings.Contains(strings.ToLower(stmt), " name_ja = title ") || strings.Contains(strings.ToLower(stmt), "drop column title")) {
				continue
			}
			if err := DB.Exec(stmt).Error; err != nil {
				// Check if error is about column already existing (safe to ignore)
				errMsg := err.Error()
				if strings.Contains(errMsg, "Duplicate column") ||
					strings.Contains(errMsg, "already exists") ||
					strings.Contains(errMsg, "1060") || // MySQL error code for duplicate column
					strings.Contains(errMsg, "Unknown column") ||
					strings.Contains(errMsg, "1054") || // Unknown column
					strings.Contains(errMsg, "doesn't exist") || // Table doesn't exist
					strings.Contains(errMsg, "1146") || // MySQL error code for table doesn't exist
					strings.Contains(errMsg, "Duplicate key name") || // duplicate index name
					strings.Contains(errMsg, "1061") { // MySQL error code for duplicate key name
					continue // Safe to ignore
				}
				log.Printf("Migration %s statement failed: %v\nStatement: %s", m.name, err, stmt)
				failed = true
				break
			}

			// Ensure new tables for generalized day shift groups exist
			if err := DB.AutoMigrate(&DayShiftCycleGroup{}, &DayShiftCycleGroupMember{}); err != nil {
				log.Printf("migration warning: automigrate day shift groups failed: %v", err)
			}

			// One-time data migration: convert existing pairs to groups if groups are empty
			{
				var existing Migration
				if err := DB.Where("name = ?", "0009_dayshift_groups").First(&existing).Error; err != nil {
					// Not applied yet
					var groupsCnt int64
					_ = DB.Model(&DayShiftCycleGroup{}).Count(&groupsCnt).Error
					if groupsCnt == 0 {
						var pairs []DayShiftCyclePair
						_ = DB.Order("cycle_id ASC, position ASC, id ASC").Find(&pairs).Error
						if len(pairs) > 0 {
							// group by cycle and position similar to pairs
							for _, p := range pairs {
								g := DayShiftCycleGroup{CycleID: p.CycleID, Position: p.Position}
								if err := DB.Create(&g).Error; err != nil {
									log.Printf("migration warning: create group from pair failed: %v", err)
									continue
								}
								_ = DB.Create(&DayShiftCycleGroupMember{GroupID: g.ID, UserID: p.User1ID, Position: 1}).Error
								_ = DB.Create(&DayShiftCycleGroupMember{GroupID: g.ID, UserID: p.User2ID, Position: 2}).Error
							}
						}
					}
					DB.Create(&Migration{Name: "0009_dayshift_groups"})
				}
			}
		}

		if !failed {
			// Record as applied
			DB.Create(&Migration{Name: m.name})
			log.Printf("Migration %s applied successfully", m.name)
		}
	}

	// Always ensure generalized day shift group tables exist and perform one-time backfill if needed.
	ensureDayShiftGroupSchemaAndBackfill()
}

func ptrTime(t time.Time) *time.Time { return &t }

// ensureDayShiftGroupSchemaAndBackfill guarantees the new generalized day shift
// group tables exist and migrates legacy pairs into groups once.
func ensureDayShiftGroupSchemaAndBackfill() {
	// Create/alter tables idempotently
	if err := DB.AutoMigrate(&DayShiftCycleGroup{}, &DayShiftCycleGroupMember{}); err != nil {
		log.Printf("migration warning: automigrate day shift groups failed: %v", err)
		// Even if this fails, return to avoid unexpected behavior
		return
	}

	// One-time data migration: convert existing pairs to groups if not yet marked as applied
	var existing Migration
	if err := DB.Where("name = ?", "0009_dayshift_groups").First(&existing).Error; err == nil {
		// Already migrated
		return
	}

	// Only backfill when groups table is empty to avoid duplications
	var groupsCnt int64
	_ = DB.Model(&DayShiftCycleGroup{}).Count(&groupsCnt).Error
	if groupsCnt > 0 {
		// Still mark as applied to avoid checking on every boot
		_ = DB.Create(&Migration{Name: "0009_dayshift_groups"}).Error
		return
	}

	var pairs []DayShiftCyclePair
	if err := DB.Order("cycle_id ASC, position ASC, id ASC").Find(&pairs).Error; err != nil {
		log.Printf("migration warning: fetch pairs for backfill failed: %v", err)
		return
	}
	for _, p := range pairs {
		g := DayShiftCycleGroup{CycleID: p.CycleID, Position: p.Position}
		if err := DB.Create(&g).Error; err != nil {
			log.Printf("migration warning: create group from pair failed: %v", err)
			continue
		}
		_ = DB.Create(&DayShiftCycleGroupMember{GroupID: g.ID, UserID: p.User1ID, Position: 1}).Error
		_ = DB.Create(&DayShiftCycleGroupMember{GroupID: g.ID, UserID: p.User2ID, Position: 2}).Error
	}
	_ = DB.Create(&Migration{Name: "0009_dayshift_groups"}).Error
}

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
	// Optional debug logging for auth flow (no secrets). Enable by setting DEBUG_AUTH=true
	debugAuth := strings.ToLower(strings.TrimSpace(env("DEBUG_AUTH", ""))) == "true"
	raw := c.Get("Authorization")
	token := parseAuthToken(raw)
	if token == "" {
		if debugAuth {
			log.Printf("[auth] %s %s missing Authorization", c.Method(), sanitizeURLPath(c.Path()))
		}
		return false
	}
	headerPlat := strings.ToLower(strings.TrimSpace(c.Get("X-Client-Platform")))
	uidStr, err := RDB.Get(c.Context(), "session:"+token).Result()
	if err != nil || strings.TrimSpace(uidStr) == "" {
		var sess Session
		if dberr := DB.Where("(hashed_token = ? OR hashed_token = ?) AND (expires_at IS NULL OR expires_at > ?)", sha256Hex([]byte(token)), token, time.Now()).Order("id DESC").First(&sess).Error; dberr != nil {
			if debugAuth {
				log.Printf("[auth] %s %s redis:miss db:miss", c.Method(), sanitizeURLPath(c.Path()))
			}
			return false
		}
		if headerPlat != "" && sess.ClientPlatform != "" && !strings.EqualFold(headerPlat, sess.ClientPlatform) {
			if debugAuth {
				log.Printf("[auth] %s %s platform mismatch header=%q db=%q", c.Method(), sanitizeURLPath(c.Path()), headerPlat, sess.ClientPlatform)
			}
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
		if debugAuth {
			log.Printf("[auth] %s %s redis:miss db:hit user=%s plat=%q", c.Method(), sanitizeURLPath(c.Path()), uidStr, sess.ClientPlatform)
		}
	} else {
		if headerPlat != "" {
			var sess Session
			if err := DB.Where("hashed_token = ?", sha256Hex([]byte(token))).Order("id DESC").First(&sess).Error; err == nil {
				if sess.ClientPlatform != "" && !strings.EqualFold(headerPlat, sess.ClientPlatform) {
					if debugAuth {
						log.Printf("[auth] %s %s redis:hit platform mismatch header=%q db=%q", c.Method(), sanitizeURLPath(c.Path()), headerPlat, sess.ClientPlatform)
					}
					return false
				}
			}
		}
		if debugAuth {
			log.Printf("[auth] %s %s redis:hit user=%s plat_header=%q", c.Method(), sanitizeURLPath(c.Path()), strings.TrimSpace(uidStr), headerPlat)
		}
	}
	var u User
	if err := DB.First(&u, uidStr).Error; err != nil {
		if debugAuth {
			log.Printf("[auth] %s %s user not found id=%s", c.Method(), sanitizeURLPath(c.Path()), strings.TrimSpace(uidStr))
		}
		return false
	}
	var cnt int64
	if err := DB.Model(&Block{}).Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", u.ID, time.Now()).Count(&cnt).Error; err == nil && cnt > 0 {
		if debugAuth {
			log.Printf("[auth] %s %s user blocked id=%d", c.Method(), sanitizeURLPath(c.Path()), u.ID)
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
	if err := DB.Where("hashed_token = ? AND is_valid = 1 AND (expires_at IS NULL OR expires_at > ?)", hashed, time.Now()).First(&key).Error; err != nil {
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
		// SMTPS (implicit TLS) - enforce certificate verification
		conn, err = tls.DialWithDialer(dialer, "tcp", smtpHost+":"+smtpPort, &tls.Config{ServerName: smtpHost, InsecureSkipVerify: false})
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

// normalizeTaskType converts user-provided or localized labels into canonical enum values.
// Allowed enums: ONLINE, FACE_TO_FACE, QUIZ, PRESENTATION, SELF_STUDY, OTHER
func normalizeTaskType(s string) string {
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
	case "P1", "P2", "P3", "P4", "P5", "P6", "P7", "P8":
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
	case "P1", "P2", "P3", "P4", "P5", "P6", "P7", "P8":
		return strings.TrimPrefix(v, "P") + "限まで"
	case "NONE", "":
		return "指定なし"
	default:
		return v
	}
}

func subjectNameJa(id uint) string {
	var s Subject
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
func buildTaskUpdateMessageJa(before Task, after Task) string {
	b := &strings.Builder{}
	fmt.Fprintf(b, "課題が更新されました (ID:%d)\n", after.ID)
	// Title
	if before.Title != after.Title {
		fmt.Fprintf(b, "・タイトル: '%s' → '%s'\n", before.Title, after.Title)
	}
	// Subject
	if before.SubjectID != after.SubjectID {
		fmt.Fprintf(b, "・教科: '%s' → '%s'\n", subjectNameJa(before.SubjectID), subjectNameJa(after.SubjectID))
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
	if before.SelfCheckRequired != after.SelfCheckRequired {
		fmt.Fprintf(b, "・自己チェック: %s → %s\n", displayBoolJa(before.SelfCheckRequired), displayBoolJa(after.SelfCheckRequired))
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

func buildTaskDeleteMessageJa(t Task) string {
	return fmt.Sprintf("課題が削除されました: %s (ID:%d)", t.Title, t.ID)
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

// Look up the latest Teams message id for a task from tasks_teams_logs
func latestTeamsMessageIDForTask(taskID uint) (string, bool) {
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
func sendTeamsReplyEmail(taskID uint, messageID string, message string) {
	to := env("TEAMS_NOTIFY_EMAIL", "")
	if strings.TrimSpace(to) == "" {
		return
	}
	subject := "[Taskly]tasks:reply"
	body := fmt.Sprintf(`{"message_id":%q,"task_id":%q,"message":%q}`, messageID, fmt.Sprintf("%d", taskID), message)
	go func() { _ = smtpSendPlain(to, subject, body) }()
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
// (server no longer runs in-process schedulers; handled by Airflow)

func isBusinessDay(d time.Time) bool {
	var bc BusinessCalendar
	if err := DB.Where("date = ?", d.Format("2006-01-02")).First(&bc).Error; err == nil {
		return bc.IsBusinessDay
	}
	wd := d.Weekday()
	return wd != time.Saturday && wd != time.Sunday
}

// (batch day shift notification moved to Airflow)

// (daily tasks digest moved to Airflow notifications DAG)

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

// sendDiscordContent posts a simple content message to Discord webhook if configured. Fire-and-forget friendly.
func sendDiscordContent(content string) {
	hook := env("DISCORD_WEBHOOK_URL", "")
	if strings.TrimSpace(hook) == "" {
		log.Printf("discord: skipped (no DISCORD_WEBHOOK_URL)")
		return
	}
	if strings.TrimSpace(content) == "" {
		log.Printf("discord: skipped (empty content)")
		return
	}
	payload := fmt.Sprintf(`{"content": %q}`, content)
	req, err := http.NewRequest("POST", hook, strings.NewReader(payload))
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
	// Drain and close body
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("discord: non-2xx status: %d", resp.StatusCode)
		return
	}
	log.Printf("discord: sent ok")
}

// sendDiscordEmbeds posts one or more embeds to the Discord webhook. Logs basic diagnostics.
func sendDiscordEmbeds(embeds []map[string]any) {
	hook := env("DISCORD_WEBHOOK_URL", "")
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

// sendDiscordTaskEmbed builds and sends an embed for task events (created/updated/deleted)
func sendDiscordTaskEmbed(action string, t Task, actor string) {
	// Prepare fields
	eff := effectiveDeadlineTime(t)
	webURL := strings.TrimSpace(env("WEB_URL", ""))
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
	fields := []map[string]any{
		{"name": "ID", "value": fmt.Sprintf("%d", t.ID), "inline": true},
		{"name": "Deadline", "value": eff.Format("2006-01-02 15:04"), "inline": true},
		{"name": "Since/Until", "value": fmt.Sprintf("%s / %s", strings.ToUpper(t.DeadlineSince), strings.ToUpper(t.DeadlineUntil)), "inline": true},
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
	sendDiscordEmbeds([]map[string]any{embed})
}

// sendDiscordDayShiftNotifyEmbed sends an embed about day shift notification sending
func sendDiscordDayShiftNotifyEmbed(date time.Time, userNames []string, actor string) {
	title := "Day Shift Notified"
	color := 0x3498db // blue
	users := ""
	if len(userNames) > 0 {
		users = strings.Join(userNames, ", ")
	} else {
		users = "(none)"
	}
	fields := []map[string]any{
		{"name": "Date", "value": date.Format("2006-01-02"), "inline": true},
		{"name": "Users", "value": users, "inline": false},
		{"name": "By", "value": actor, "inline": true},
	}
	embed := map[string]any{
		"title":     title,
		"color":     color,
		"fields":    fields,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	sendDiscordEmbeds([]map[string]any{embed})
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
	// Prefer new group configuration if present
	var cfgGroups []DayShiftCycleGroup
	_ = DB.Where("cycle_id = ?", cyc.ID).Order("position ASC, id ASC").Find(&cfgGroups).Error
	groups := make([][]uint, 0)
	if len(cfgGroups) > 0 {
		for _, g := range cfgGroups {
			var members []DayShiftCycleGroupMember
			_ = DB.Where("group_id = ?", g.ID).Order("position ASC, id ASC").Find(&members).Error
			if len(members) == 0 {
				continue
			}
			arr := make([]uint, 0, len(members))
			for _, m := range members {
				arr = append(arr, m.UserID)
			}
			groups = append(groups, arr)
		}
	}
	if len(groups) == 0 {
		// Fallback to legacy pairs config
		var cfgPairs []DayShiftCyclePair
		_ = DB.Where("cycle_id = ?", cyc.ID).Order("position ASC, id ASC").Find(&cfgPairs).Error
		if len(cfgPairs) > 0 {
			for _, p := range cfgPairs {
				groups = append(groups, []uint{p.User1ID, p.User2ID})
			}
		}
	}
	if len(groups) == 0 {
		// Auto compute groups from active users using env group size (default 2)
		actUsers, err := listActiveUsers()
		if err != nil || len(actUsers) < 1 {
			return err
		}
		gs := 2
		if v := strings.TrimSpace(env("DAYSHIFT_GROUP_SIZE", "")); v != "" {
			if n, e := strconv.Atoi(v); e == nil && n >= 1 {
				gs = n
			}
		}
		groups = computeGroups(actUsers, gs)
	}
	if len(groups) == 0 {
		return nil
	}
	dayIdx := businessDaysBetween(cyc.BeginDate, d)
	grp := groups[dayIdx%len(groups)]
	_ = DB.Where("day_shift_assignments_id = ?", today.ID).Delete(&DayShiftAssignmentUser{}).Error
	for _, uid := range grp {
		_ = DB.Create(&DayShiftAssignmentUser{DayShiftAssignmentsID: today.ID, UserID: uid}).Error
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

// computeGroups splits users into groups of given size in order.
// If remainder is non-zero, drop the trailing users (and log) to keep group sizes uniform.
func computeGroups(users []User, size int) [][]uint {
	out := [][]uint{}
	n := len(users)
	if size <= 0 {
		size = 2
	}
	if n < size {
		return out
	}
	rem := n % size
	if rem != 0 {
		log.Printf("[DayShift] users not divisible by group size (n=%d, size=%d), dropping last %d user(s)", n, size, rem)
		n = n - rem
	}
	for i := 0; i < n; i += size {
		grp := make([]uint, 0, size)
		for j := 0; j < size; j++ {
			grp = append(grp, users[i+j].ID)
		}
		out = append(out, grp)
	}
	return out
}

// Mark optional helpers as used to satisfy static analysis when rotation is disabled in some builds
var (
	_ = prevBusinessDay
	_ = nextBusinessDay
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

// Decide whether to rotate day shift based on Gemini API analysis of subject/body
func shouldRotateByGemini(subject, body string) bool {
	// 1) Strong heuristic first (Japanese keywords)
	s := strings.ToLower(subject + "\n" + body)
	if shouldRotateHeuristic(s) {
		return true
	}
	// 2) If Gemini API key is configured, ask the model; parse JSON result safely
	if env("GEMINI_API_KEY", "") == "" {
		// No API key and heuristic didn't match → do not rotate
		return false
	}
	// Allow overriding model via env; default to gemini-2.5-flash to match current API examples
	modelName := strings.TrimSpace(env("GEMINI_MODEL", "gemini-2.5-flash"))
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

// Notify Discord with a simple embed indicating day shift rotation occurred
func notifyDayShiftRotationDiscord(target time.Time) {
	hook := env("DISCORD_WEBHOOK_URL", "")
	if hook == "" {
		return
	}
	embed := fmt.Sprintf(`{"embeds":[{"title":"Day Shift Rotation Executed","description":"Rotation processed for %s","color":5814783}]}`, target.Format("2006-01-02"))
	req, _ := http.NewRequest("POST", hook, strings.NewReader(embed))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err == nil && resp != nil {
		_ = resp.Body.Close()
	}
}

// Shift all assignments from the next business day after 'start' one business day forward,
// and ensure today's users are also assigned to that next business day.
func pushAssignmentsOneDayForward(start time.Time) error {
	start = dateOnly(start)
	next := nextBusinessDay(start)
	if next.IsZero() {
		return nil
	}
	// Load all assignments on/after next within current cycle if any (or for next 60 days)
	end := next.AddDate(0, 0, 60)
	if cyc, err := getCurrentCycle(start); err == nil && cyc != nil && cyc.FinishDate.After(next) {
		end = dateOnly(cyc.FinishDate)
	}
	var items []DayShiftAssignment
	if err := DB.Where("date BETWEEN ? AND ?", next.Format("2006-01-02"), end.Format("2006-01-02")).Order("date ASC").Find(&items).Error; err != nil {
		return err
	}
	// Iterate in reverse to move forward safely
	for i := len(items) - 1; i >= 0; i-- {
		cur := items[i]
		dest := nextBusinessDay(cur.Date)
		if dest.IsZero() {
			continue
		}
		var destAsg DayShiftAssignment
		err := DB.Where("date = ?", dest.Format("2006-01-02")).First(&destAsg).Error
		if err == nil {
			var curUsers []DayShiftAssignmentUser
			_ = DB.Where("day_shift_assignments_id = ?", cur.ID).Find(&curUsers).Error
			for _, u := range curUsers {
				var cnt int64
				_ = DB.Model(&DayShiftAssignmentUser{}).Where("day_shift_assignments_id = ? AND user_id = ?", destAsg.ID, u.UserID).Count(&cnt).Error
				if cnt == 0 {
					_ = DB.Create(&DayShiftAssignmentUser{DayShiftAssignmentsID: destAsg.ID, UserID: u.UserID}).Error
				}
			}
			_ = DB.Delete(&DayShiftAssignment{}, cur.ID).Error
		} else {
			_ = DB.Model(&DayShiftAssignment{}).Where("id = ?", cur.ID).Update("date", dateOnly(dest)).Error
		}
	}
	// Copy today's users to next business day as well
	var today DayShiftAssignment
	if err := DB.Where("date = ?", start.Format("2006-01-02")).First(&today).Error; err == nil {
		var todayUsers []DayShiftAssignmentUser
		_ = DB.Where("day_shift_assignments_id = ?", today.ID).Find(&todayUsers).Error
		var dest DayShiftAssignment
		if err := DB.Where("date = ?", next.Format("2006-01-02")).First(&dest).Error; err != nil {
			cyc, _ := getCurrentCycle(next)
			dest = DayShiftAssignment{Date: next, Status: "assigned"}
			if cyc != nil {
				dest.CycleID = cyc.ID
			}
			_ = DB.Create(&dest).Error
		}
		for _, u := range todayUsers {
			var cnt int64
			_ = DB.Model(&DayShiftAssignmentUser{}).Where("day_shift_assignments_id = ? AND user_id = ?", dest.ID, u.UserID).Count(&cnt).Error
			if cnt == 0 {
				_ = DB.Create(&DayShiftAssignmentUser{DayShiftAssignmentsID: dest.ID, UserID: u.UserID}).Error
			}
		}
	}
	return nil
}

// Shift all assignments from the previous business day before 'start' one business day backward,
// and ensure today's users are also assigned to that previous business day.
func pushAssignmentsOneDayBackward(start time.Time) error {
	start = dateOnly(start)
	prev := prevBusinessDay(start)
	if prev.IsZero() {
		return nil
	}
	// Load all assignments on/before prev within current cycle if any (or for previous 60 days)
	begin := prev.AddDate(0, 0, -60)
	if cyc, err := getCurrentCycle(start); err == nil && cyc != nil && cyc.BeginDate.Before(prev) {
		begin = dateOnly(cyc.BeginDate)
	}
	var items []DayShiftAssignment
	if err := DB.Where("date BETWEEN ? AND ?", begin.Format("2006-01-02"), prev.Format("2006-01-02")).Order("date ASC").Find(&items).Error; err != nil {
		return err
	}
	// Iterate in ascending order to move backward safely
	for i := 0; i < len(items); i++ {
		cur := items[i]
		dest := prevBusinessDay(cur.Date)
		if dest.IsZero() {
			continue
		}
		var destAsg DayShiftAssignment
		err := DB.Where("date = ?", dest.Format("2006-01-02")).First(&destAsg).Error
		if err == nil {
			// merge users then delete current assignment
			var curUsers []DayShiftAssignmentUser
			_ = DB.Where("day_shift_assignments_id = ?", cur.ID).Find(&curUsers).Error
			for _, u := range curUsers {
				var cnt int64
				_ = DB.Model(&DayShiftAssignmentUser{}).Where("day_shift_assignments_id = ? AND user_id = ?", destAsg.ID, u.UserID).Count(&cnt).Error
				if cnt == 0 {
					_ = DB.Create(&DayShiftAssignmentUser{DayShiftAssignmentsID: destAsg.ID, UserID: u.UserID}).Error
				}
			}
			_ = DB.Delete(&DayShiftAssignment{}, cur.ID).Error
		} else {
			// simply move the date backward
			_ = DB.Model(&DayShiftAssignment{}).Where("id = ?", cur.ID).Update("date", dateOnly(dest)).Error
		}
	}
	// Copy today's users to previous business day as well
	var today DayShiftAssignment
	if err := DB.Where("date = ?", start.Format("2006-01-02")).First(&today).Error; err == nil {
		var todayUsers []DayShiftAssignmentUser
		_ = DB.Where("day_shift_assignments_id = ?", today.ID).Find(&todayUsers).Error
		var dest DayShiftAssignment
		if err := DB.Where("date = ?", prev.Format("2006-01-02")).First(&dest).Error; err != nil {
			cyc, _ := getCurrentCycle(prev)
			dest = DayShiftAssignment{Date: prev, Status: "assigned"}
			if cyc != nil {
				dest.CycleID = cyc.ID
			}
			_ = DB.Create(&dest).Error
		}
		for _, u := range todayUsers {
			var cnt int64
			_ = DB.Model(&DayShiftAssignmentUser{}).Where("day_shift_assignments_id = ? AND user_id = ?", dest.ID, u.UserID).Count(&cnt).Error
			if cnt == 0 {
				_ = DB.Create(&DayShiftAssignmentUser{DayShiftAssignmentsID: dest.ID, UserID: u.UserID}).Error
			}
		}
	}
	return nil
}

// captureError reports error to Sentry if DSN is configured
func captureError(err error) {
	if err == nil {
		return
	}
	if env("SENTRY_DSN", "") == "" {
		return
	}
	sentry.CaptureException(err)
}
