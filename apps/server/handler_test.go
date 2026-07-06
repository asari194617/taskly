package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestIcalFeedRoute(t *testing.T) {
	db := newTestIcalDB(t)
	app := fiber.New()
	registerIcalFeedRoute(app, db)

	token := "feed-token"
	user := domain.User{Email: "user@example.com", DisplayName: "User"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	subject := domain.Subject{NameJa: "数学"}
	if err := db.Create(&subject).Error; err != nil {
		t.Fatalf("create subject: %v", err)
	}

	deadline := time.Date(2026, time.June, 8, 0, 0, 0, 0, time.UTC)
	task := domain.Task{
		AddedUserID: user.ID,
		Title:       "宿題",
		SubjectID:   subject.ID,
		Deadline:    deadline,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	key := domain.IcalKey{UserID: user.ID, Token: token}
	if err := db.Create(&key).Error; err != nil {
		t.Fatalf("create iCal key: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/ical/"+token+".ics", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Content-Type"); got != "text/calendar; charset=utf-8" {
		t.Fatalf("content-type = %q, want %q", got, "text/calendar; charset=utf-8")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	bodyText := string(body)
	if !strings.Contains(bodyText, "BEGIN:VCALENDAR") {
		t.Fatalf("response body missing calendar header: %s", bodyText)
	}
	if !strings.Contains(bodyText, "SUMMARY:[課題] 数学: 宿題") {
		t.Fatalf("response body missing task summary: %s", bodyText)
	}
}

func TestIcalFeedRoute_NotFound(t *testing.T) {
	db := newTestIcalDB(t)
	app := fiber.New()
	registerIcalFeedRoute(app, db)

	req := httptest.NewRequest(http.MethodGet, "/ical/missing.ics", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func newTestIcalDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := "file:ical-route-" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()) + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	sqlDB.SetMaxOpenConns(1)

	stmts := []string{
		`CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL DEFAULT '',
			hashed_password TEXT,
			email_code_allowed INTEGER NOT NULL DEFAULT 1,
			is_deleted INTEGER NOT NULL DEFAULT 0,
			notification_lang TEXT NOT NULL DEFAULT 'ja',
			is_registered INTEGER NOT NULL DEFAULT 0,
			is_agreed_policies INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE subjects (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name_ja TEXT NOT NULL DEFAULT '',
			name_en TEXT,
			expires_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			added_user_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			subject_id INTEGER NOT NULL,
			description TEXT,
			type TEXT NOT NULL DEFAULT 'OTHER',
			is_require_hand_in INTEGER NOT NULL DEFAULT 0,
			is_self_check_required INTEGER NOT NULL DEFAULT 0,
			is_hidden INTEGER NOT NULL DEFAULT 0,
			deadline DATETIME NOT NULL,
			deadline_since TEXT NOT NULL DEFAULT 'NONE',
			deadline_until TEXT NOT NULL DEFAULT 'NONE',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE ical_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			token TEXT NOT NULL UNIQUE,
			description TEXT,
			expires_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create test tables: %v", err)
		}
	}

	return db
}
