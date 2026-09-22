package handler

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/legal"
	"github.com/asari194617/taskly/apps/server/internal/middleware"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMCPUserProfileDoesNotChangeAccountState(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:mcp-user-profile?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.User{}, &domain.MCP{}, &domain.Block{}, &legal.Consent{}); err != nil {
		t.Fatal(err)
	}
	user := domain.User{Email: "mcp-profile@example.test"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&legal.Consent{UserID: user.ID, TermsVersion: legal.Version, PrivacyVersion: legal.Version, SecurityVersion: legal.Version, DisplayLanguage: "ja"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&domain.MCP{UserID: user.ID, HashedToken: utils.Sha256Hex([]byte("profile-token"))}).Error; err != nil {
		t.Fatal(err)
	}
	mw := middleware.NewMiddleware(db, nil, nil)
	app := fiber.New()
	app.Use(mw.ApiKeyRouteScope)
	NewHandler(db, nil, nil).RegisterUsersRoutes(app.Group("/users", mw.AuthRequired))
	patch := func(path, body string) int {
		req := httptest.NewRequest(fiber.MethodPatch, path, strings.NewReader(body))
		req.Header.Set("X-MCP-Token", "profile-token")
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	for _, field := range []string{"is_deleted", "is_registered", "is_agreed_policies"} {
		body := `{"display_name":"Should not apply","` + field + `":true}`
		if got := patch("/users/me", body); got != fiber.StatusBadRequest {
			t.Fatalf("%s: status = %d", field, got)
		}
	}
	if got := patch("/users/me", `{"notification_lang":"invalid"}`); got != fiber.StatusBadRequest {
		t.Fatalf("invalid language: status = %d", got)
	}
	var fresh domain.User
	if err := db.First(&fresh, user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if fresh.DisplayName != "" || fresh.IsDeleted || fresh.IsRegistered || fresh.IsAgreedPolicies {
		t.Fatalf("rejected patch changed user state: %+v", fresh)
	}
	if got := patch("/users/me", `{}`); got != fiber.StatusBadRequest {
		t.Fatalf("empty profile: status = %d", got)
	}
	deleteRequest := httptest.NewRequest(fiber.MethodDelete, "/users/me", nil)
	deleteRequest.Header.Set("X-MCP-Token", "profile-token")
	deleteResponse, err := app.Test(deleteRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer deleteResponse.Body.Close()
	if deleteResponse.StatusCode != fiber.StatusForbidden {
		t.Fatalf("MCP account deletion: status = %d", deleteResponse.StatusCode)
	}
	if got := patch("/users/me", `{"display_name":"  Updated  ","notification_lang":" EN "}`); got != fiber.StatusNoContent {
		t.Fatalf("valid profile: status = %d", got)
	}
	if err := db.First(&fresh, user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if fresh.DisplayName != "Updated" || fresh.NotificationLang != "en" || fresh.IsDeleted || fresh.IsRegistered || fresh.IsAgreedPolicies {
		t.Fatalf("unexpected user state: %+v", fresh)
	}
}
