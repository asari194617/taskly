package server

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestLegacyPasskeyRESTGate(t *testing.T) {
	for _, test := range []struct {
		name, flag string
		want       int
	}{
		{name: "legacy mode", flag: "0", want: fiber.StatusNoContent},
		{name: "external auth mode", flag: "1", want: fiber.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("AUTH_PASSKEYS_EXTERNAL", test.flag)
			app := fiber.New()
			app.Group("/webauthn", blockLegacyPasskeys).Post("/login/finish", func(c *fiber.Ctx) error {
				return c.SendStatus(fiber.StatusNoContent)
			})
			response, err := app.Test(httptest.NewRequest("POST", "/webauthn/login/finish", nil))
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != test.want {
				t.Fatalf("status=%d, want %d", response.StatusCode, test.want)
			}
		})
	}
}
