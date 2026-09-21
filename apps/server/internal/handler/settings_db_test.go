package handler

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/middleware"
	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNotificationMethodDefaultMatchesDelivery(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.NotificationDevice{}, &domain.NotificationTypePref{}); err != nil {
		t.Fatal(err)
	}
	u := domain.User{ID: 7}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("user", &u); return c.Next() })
	NewHandler(db, nil, nil).RegisterSettingsRoutes(app.Group("/settings"))
	check := func(wantEmail bool) {
		t.Helper()
		resp, err := app.Test(httptest.NewRequest("GET", "/settings/notification-methods", nil))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("status=%d", resp.StatusCode)
		}
		var methods struct {
			Email   bool `json:"email"`
			WebPush bool `json:"webpush"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&methods); err != nil {
			t.Fatal(err)
		}
		if methods.Email != wantEmail || methods.WebPush {
			t.Fatalf("methods=%+v", methods)
		}
	}
	check(true)
	post := func(path, body string) {
		t.Helper()
		req := httptest.NewRequest("POST", path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 204 {
			t.Fatalf("POST %s status=%d", path, resp.StatusCode)
		}
	}
	post("/settings/notification-methods", `{"email":false}`)
	check(false)
	post("/settings/notification-types", `{"TASK_MORNING":false,"DAYSHIFT":false}`)
	resp, err := app.Test(httptest.NewRequest("GET", "/settings/notification-types", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var types map[string]bool
	if err := json.NewDecoder(resp.Body).Decode(&types); err != nil {
		t.Fatal(err)
	}
	if types["TASK_MORNING"] || types["DAYSHIFT"] || !types["TASK"] {
		t.Fatalf("notification types=%v", types)
	}
}

func TestSettingsDoNotReportDatabaseFailureAsSuccess(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.User{}, &domain.NotificationDevice{}, &domain.NotificationTypePref{}); err != nil {
		t.Fatal(err)
	}
	u := domain.User{Email: "settings@example.com", DisplayName: "Settings"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("user", &u); return c.Next() })
	NewHandler(db, nil, nil).RegisterSettingsRoutes(app.Group("/settings"))
	request := func(method, path, body string, want int) {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != want {
			t.Fatalf("%s %s: got %d, want %d", method, path, resp.StatusCode, want)
		}
	}

	request("POST", "/settings/lang", `{"notification_lang":"en"}`, 204)
	if err := db.Exec("DROP TABLE users").Error; err != nil {
		t.Fatal(err)
	}
	request("POST", "/settings/auth", `{"email_login_enabled":true}`, 500)
	request("GET", "/settings/lang", "", 500)
	request("GET", "/settings/auth", "", 500)
}

func TestRecoveryCodeGenerationRollsBackOnInsertFailure(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.RecoveryCode{}); err != nil {
		t.Fatal(err)
	}
	u := domain.User{ID: 9}
	if err := db.Create(&domain.RecoveryCode{UserID: u.ID, HashedCode: "old"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TRIGGER fail_new_code BEFORE INSERT ON recovery_codes WHEN NEW.hashed_code != 'old' BEGIN SELECT RAISE(ABORT, 'injected failure'); END").Error; err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("user", &u); return c.Next() })
	NewHandler(db, nil, nil).Register2FARoutes(app.Group("/2fa"), middleware.NewMiddleware(db, nil, nil))
	resp, err := app.Test(httptest.NewRequest("POST", "/2fa/recovery/generate", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 500 {
		t.Fatalf("got %d, want 500", resp.StatusCode)
	}
	var codes []domain.RecoveryCode
	if err := db.Where("user_id = ?", u.ID).Find(&codes).Error; err != nil {
		t.Fatal(err)
	}
	if len(codes) != 1 || codes[0].HashedCode != "old" {
		t.Fatalf("codes changed after failed transaction: %+v", codes)
	}
}

func TestNotificationMethodsRollBackOnSecondWriteFailure(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.NotificationDevice{}); err != nil {
		t.Fatal(err)
	}
	u := domain.User{ID: 7}
	if err := db.Create(&domain.NotificationDevice{UserID: u.ID, Platform: "WEBPUSH", IsEnabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TRIGGER fail_webpush BEFORE UPDATE ON notification_devices WHEN OLD.platform = 'WEBPUSH' BEGIN SELECT RAISE(ABORT, 'injected failure'); END").Error; err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("user", &u); return c.Next() })
	NewHandler(db, nil, nil).RegisterSettingsRoutes(app.Group("/settings"))
	req := httptest.NewRequest("POST", "/settings/notification-methods", strings.NewReader(`{"email":true,"webpush":false}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 500 {
		t.Fatalf("got %d, want 500", resp.StatusCode)
	}
	var count int64
	if err := db.Model(&domain.NotificationDevice{}).Where("platform = 'EMAIL'").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("partial EMAIL write survived rollback")
	}
}
