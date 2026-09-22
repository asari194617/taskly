package server

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestIcalFeedRoute_AllTasksWithConstantQueries(t *testing.T) {
	for _, count := range []int{127, 501} {
		t.Run(fmt.Sprintf("%d_tasks", count), func(t *testing.T) {
			db := newTestIcalDB(t)
			user := domain.User{Email: "feed@example.test", NotificationLang: "en"}
			if err := db.Create(&user).Error; err != nil {
				t.Fatal(err)
			}
			en := "Math"
			subject := domain.Subject{NameJa: "数学", NameEn: &en}
			if err := db.Create(&subject).Error; err != nil {
				t.Fatal(err)
			}
			deadline := time.Date(2026, time.June, 8, 0, 0, 0, 0, time.UTC)
			tasks := make([]domain.Task, count)
			for i := range tasks {
				tasks[i] = domain.Task{AddedUserID: user.ID, SubjectID: subject.ID, Title: fmt.Sprintf("task-%03d", i), Deadline: deadline}
			}
			if err := db.CreateInBatches(tasks, 100).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&domain.IcalKey{UserID: user.ID, Token: "many-tasks"}).Error; err != nil {
				t.Fatal(err)
			}

			var queries atomic.Int64
			if err := db.Callback().Query().Before("gorm:query").Register("count_ical_queries", func(*gorm.DB) {
				queries.Add(1)
			}); err != nil {
				t.Fatal(err)
			}
			app := fiber.New()
			registerIcalFeedRoute(app, db)
			resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/ical/many-tasks.ics", nil), -1)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d: %s", resp.StatusCode, body)
			}
			if got := strings.Count(string(body), "BEGIN:VEVENT"); got != count {
				t.Fatalf("events = %d, want %d", got, count)
			}
			if !strings.Contains(string(body), fmt.Sprintf("SUMMARY:[課題] Math: task-%03d", count-1)) {
				t.Fatal("last task or English subject name missing")
			}
			if got := queries.Load(); got != 3 {
				t.Fatalf("queries = %d, want 3", got)
			}
		})
	}
}

func TestIcalFeedRoute_DatabaseError(t *testing.T) {
	db := newTestIcalDB(t)
	user := domain.User{Email: "feed-error@example.test"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&domain.IcalKey{UserID: user.ID, Token: "broken-feed"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("DROP TABLE tasks").Error; err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	registerIcalFeedRoute(app, db)
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/ical/broken-feed.ics", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); strings.HasPrefix(got, "text/calendar") {
		t.Fatalf("error response has calendar content type: %q", got)
	}
}

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

func TestIcalFeedRoute_DeletedUser(t *testing.T) {
	db := newTestIcalDB(t)
	user := domain.User{Email: "deleted@example.test", IsDeleted: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&domain.IcalKey{UserID: user.ID, Token: "old-feed-token"}).Error; err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	registerIcalFeedRoute(app, db)
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/ical/old-feed-token.ics", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("deleted user's feed status = %d", resp.StatusCode)
	}
}

func BenchmarkBuildICSForUser(b *testing.B) {
	for _, count := range []int{127, 501} {
		b.Run(fmt.Sprintf("%d_tasks", count), func(b *testing.B) {
			db := newTestIcalDB(b)
			user := domain.User{Email: "benchmark@example.test"}
			if err := db.Create(&user).Error; err != nil {
				b.Fatal(err)
			}
			subject := domain.Subject{NameJa: "数学"}
			if err := db.Create(&subject).Error; err != nil {
				b.Fatal(err)
			}
			tasks := make([]domain.Task, count)
			for i := range tasks {
				tasks[i] = domain.Task{
					AddedUserID: user.ID, SubjectID: subject.ID,
					Title:    fmt.Sprintf("task-%03d", i),
					Deadline: time.Date(2026, time.June, 8, 0, 0, 0, 0, time.UTC),
				}
			}
			if err := db.CreateInBatches(tasks, 100).Error; err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := utils.BuildICSForUser(user.ID, db); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func newTestIcalDB(t testing.TB) *gorm.DB {
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
			deleted_at DATETIME,
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
