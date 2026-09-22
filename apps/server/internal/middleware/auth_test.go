package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/legal"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMCPTokenOnlyAccessesTaskRoutes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:mcp-auth?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.User{}, &domain.MCP{}, &domain.Block{}, &legal.Consent{}); err != nil {
		t.Fatal(err)
	}
	user := domain.User{Email: "mcp-auth@example.test"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&legal.Consent{UserID: user.ID, TermsVersion: legal.Version, PrivacyVersion: legal.Version, SecurityVersion: legal.Version, DisplayLanguage: "ja"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&domain.MCP{UserID: user.ID, HashedToken: utils.Sha256Hex([]byte("valid-token"))}).Error; err != nil {
		t.Fatal(err)
	}
	mw := NewMiddleware(db, nil, nil)
	app := fiber.New()
	app.Use(mw.ApiKeyRouteScope)
	app.Get("/tasks", mw.AuthRequired, func(c *fiber.Ctx) error {
		if c.Locals("mcpToken") != true {
			return c.SendStatus(500)
		}
		return c.SendStatus(200)
	})
	app.Get("/settings", mw.AuthRequired, func(c *fiber.Ctx) error { return c.SendStatus(200) })
	request := func(path, token string) int {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("X-MCP-Token", token)
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	if status := request("/tasks", "valid-token"); status != 200 {
		t.Fatalf("valid task status = %d", status)
	}
	if status := request("/tasks", "invalid-token"); status != 401 {
		t.Fatalf("invalid token status = %d", status)
	}
	if status := request("/settings", "valid-token"); status != 403 {
		t.Fatalf("out-of-scope status = %d", status)
	}
}

func TestLegacyAgreementCannotAccessProtectedRoute(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:legacy-policy-gate?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.User{}, &legal.Consent{}); err != nil {
		t.Fatal(err)
	}
	user := domain.User{Email: "legacy@example.test", IsAgreedPolicies: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	mw := NewMiddleware(db, nil, nil)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("user", &user); return c.Next() })
	app.Get("/protected", mw.AuthRequired, func(c *fiber.Ctx) error { return c.SendStatus(200) })
	request := func() int {
		t.Helper()
		res, err := app.Test(httptest.NewRequest("GET", "/protected", nil))
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	if got := request(); got != 403 {
		t.Fatalf("legacy flag grants access: %d", got)
	}
	if err := legal.Accept(db, user.ID, legal.Version, legal.Version, legal.Version, "ja"); err != nil {
		t.Fatal(err)
	}
	if got := request(); got != 200 {
		t.Fatalf("current consent denied: %d", got)
	}
}
