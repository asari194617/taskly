package handler

import (
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestLegacyLoginRefusesRecoveryCodeAfterOwnershipTransfer(t *testing.T) {
	t.Setenv("AUTH_RECOVERY_EXTERNAL", "1")
	app := fiber.New()
	(&Handler{}).RegisterAuthRoutes(app.Group("/auth"))
	response, _, err := doJSONRequest(app, fiber.MethodPost, "/auth/login", map[string]string{
		"email": "account@example.test", "recovery_code": "ABCD-EFGH-JKLM-NPQR",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.Code != fiber.StatusConflict {
		t.Fatalf("legacy recovery login status=%d, want %d", response.Code, fiber.StatusConflict)
	}
}
