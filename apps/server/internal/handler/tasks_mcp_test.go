package handler

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/legal"
	"github.com/asari194617/taskly/apps/server/internal/middleware"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMCPTaskRoutesAreOwnerScoped(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:mcp-tasks?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.User{}, &domain.MCP{}, &domain.Block{}, &domain.RoleUser{}, &domain.Permission{}, &legal.Consent{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE tasks (
		id integer primary key, added_user_id integer, title text, subject_id integer,
		description text, type text, is_require_hand_in boolean, is_self_check_required boolean, is_completion_disabled boolean default 0,
		is_hidden boolean, deadline date, effective_deadline date, deadline_since text,
		deadline_until text, created_at datetime, updated_at datetime
	)`).Error; err != nil {
		t.Fatal(err)
	}
	user := domain.User{Email: "mcp-owner@example.test"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&legal.Consent{UserID: user.ID, TermsVersion: legal.Version, PrivacyVersion: legal.Version, SecurityVersion: legal.Version, DisplayLanguage: "ja"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&domain.MCP{UserID: user.ID, HashedToken: utils.Sha256Hex([]byte("owner-token"))}).Error; err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	for _, row := range []struct {
		id     int
		owner  uint
		hidden bool
	}{{1, user.ID, false}, {2, user.ID, true}, {3, user.ID + 1, false}} {
		if err := db.Exec(`INSERT INTO tasks (id, added_user_id, title, subject_id, is_hidden, deadline, effective_deadline, deadline_since, deadline_until)
			VALUES (?, ?, 'task', 1, ?, ?, ?, 'NONE', 'NONE')`, row.id, row.owner, row.hidden, deadline, deadline).Error; err != nil {
			t.Fatal(err)
		}
	}
	mw := middleware.NewMiddleware(db, nil, nil)
	app := fiber.New()
	app.Use(mw.ApiKeyRouteScope)
	NewHandler(db, nil, nil).RegisterTasksRoutes(app.Group("/tasks", mw.AuthRequired))
	get := httptest.NewRequest("GET", "/tasks?limit=10", nil)
	get.Header.Set("X-MCP-Token", "owner-token")
	res, err := app.Test(get)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("list status = %d", res.StatusCode)
	}
	var list struct {
		Items []struct{ ID int }
	}
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].ID != 1 {
		t.Fatalf("owner list = %+v", list.Items)
	}
	patch := httptest.NewRequest("PATCH", "/tasks/3", strings.NewReader(`{"title":"changed"}`))
	patch.Header.Set("X-MCP-Token", "owner-token")
	patch.Header.Set("Content-Type", "application/json")
	res, err = app.Test(patch)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatalf("cross-owner patch status = %d", res.StatusCode)
	}
}
