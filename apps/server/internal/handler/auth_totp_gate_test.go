package handler

import (
	"testing"

	"github.com/asari194617/taskly/apps/server/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func TestLegacyLoginRefusesTOTPAfterOwnershipTransfer(t *testing.T) {
	t.Setenv("AUTH_TOTP_EXTERNAL", "1")
	app := fiber.New()
	(&Handler{}).RegisterAuthRoutes(app.Group("/auth"))
	response, _, err := doJSONRequest(app, fiber.MethodPost, "/auth/login", map[string]string{
		"email": "account@example.test", "totp": "123456",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.Code != fiber.StatusConflict {
		t.Fatalf("legacy TOTP login status=%d, want %d", response.Code, fiber.StatusConflict)
	}
}

func TestLegacyTOTPRoutesRefuseWritesAfterOwnershipTransfer(t *testing.T) {
	t.Setenv("AUTH_TOTP_EXTERNAL", "1")
	app := fiber.New()
	(&Handler{}).Register2FARoutes(app.Group("/2fa"), middleware.NewMiddleware(nil, nil, nil))
	for _, path := range []string{"/2fa/totp/setup", "/2fa/totp/activate", "/2fa/totp/disable"} {
		response, _, err := doJSONRequest(app, fiber.MethodPost, path, map[string]string{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if response.Code != fiber.StatusConflict {
			t.Fatalf("%s status=%d, want %d", path, response.Code, fiber.StatusConflict)
		}
	}
}
