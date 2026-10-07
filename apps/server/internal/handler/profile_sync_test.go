package handler

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAuthProfileSyncRequiresCutoverAndToken(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:auth-profile-sync?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.User{}); err != nil {
		t.Fatal(err)
	}
	user := domain.User{Email: "sync@example.test", DisplayName: "Before"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	NewHandler(db, nil, nil).RegisterAuthProfileSyncRoutes(app.Group("/internal/auth"))
	token := strings.Repeat("a", 32)
	t.Setenv("AUTH_INTERNAL_TOKEN", token)
	send := func(bearer string) int {
		req := httptest.NewRequest(fiber.MethodPatch, "/internal/auth/users/"+strconv.FormatUint(uint64(user.ID), 10)+"/profile", strings.NewReader(`{"display_name":"After"}`))
		req.Header.Set("Content-Type", "application/json")
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	if got := send(token); got != 503 {
		t.Fatalf("before cutover: %d", got)
	}
	t.Setenv("AUTH_USERS_EXTERNAL", "1")
	if got := send(""); got != 401 {
		t.Fatalf("missing token: %d", got)
	}
	if got := send(strings.Repeat("b", 32)); got != 401 {
		t.Fatalf("wrong token: %d", got)
	}
	if got := send(token); got != 204 {
		t.Fatalf("valid sync: %d", got)
	}
	var updated domain.User
	if err := db.First(&updated, user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.DisplayName != "After" {
		t.Fatalf("name = %q", updated.DisplayName)
	}
	// SQLite may report one affected row for an unchanged value, unlike MySQL's
	// default changed-row count. Require the live-user read that makes a retry
	// independent of that driver difference.
	livenessReads := 0
	if err := db.Callback().Query().After("gorm:query").Register("test:profile_sync_liveness", func(tx *gorm.DB) {
		if tx.Statement.Table == "users" {
			livenessReads++
		}
	}); err != nil {
		t.Fatal(err)
	}
	if got := send(token); got != 204 {
		t.Fatalf("repeat sync: %d", got)
	}
	if livenessReads == 0 {
		t.Fatal("repeat sync did not confirm the user still exists")
	}
	if err := db.Model(&user).Update("is_deleted", true).Error; err != nil {
		t.Fatal(err)
	}
	if got := send(token); got != 404 {
		t.Fatalf("deleted user sync: %d", got)
	}
}
