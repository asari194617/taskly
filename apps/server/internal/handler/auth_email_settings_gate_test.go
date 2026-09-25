package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestLegacyEmailSettingsRestBlocksAfterOwnershipTransfer(t *testing.T) {
	t.Setenv("AUTH_EMAIL_SETTINGS_EXTERNAL", "1")
	app := fiber.New()
	(&Handler{}).RegisterSettingsRoutes(app.Group("/settings"))
	response, err := app.Test(httptest.NewRequest(http.MethodPost, "/settings/auth", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("settings write status=%d, want %d", response.StatusCode, http.StatusConflict)
	}
}
