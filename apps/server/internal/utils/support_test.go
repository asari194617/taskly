package utils

import (
	"crypto/rand"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestICalEscape(t *testing.T) {
	in := "Hello, World; Line\\Break\nNext"
	out := ICalEscape(in)
	if out == in {
		t.Fatalf("expected escaped string, got same")
	}
	if out == "" {
		t.Fatalf("expected non-empty")
	}
}

func TestGenerateIcalToken(t *testing.T) {
	a, err := GenerateIcalToken()
	if err != nil {
		t.Fatal(err)
	}
	b, err := GenerateIcalToken()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatalf("expected different tokens")
	}
}

type failingRandomReader struct{}

func (failingRandomReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestSecretGeneratorsFailClosedOnRandomError(t *testing.T) {
	original := rand.Reader
	rand.Reader = failingRandomReader{}
	t.Cleanup(func() { rand.Reader = original })

	for name, generate := range map[string]func() (string, error){
		"ical":     GenerateIcalToken,
		"session":  func() (string, error) { return GenerateSessionToken(1, "web") },
		"totp":     GenerateBase32Secret,
		"recovery": GenerateRecoveryCode,
	} {
		t.Run(name, func(t *testing.T) {
			value, err := generate()
			if value != "" || !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("value = %q, error = %v", value, err)
			}
		})
	}
}

func TestSessionCookieIsAlwaysSecureInProduction(t *testing.T) {
	t.Setenv("NODE_ENV", "production")
	t.Setenv("SESSION_COOKIE_SECURE", "false")
	if !SessionCookieSecure() {
		t.Fatal("production must not issue an insecure session cookie")
	}
}

func TestSessionTokenFromRequestRejectsConflictingCredentials(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString(SessionTokenFromRequest(c))
	})
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer bearer-token")
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "cookie-token"})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body := make([]byte, resp.ContentLength)
	_, _ = resp.Body.Read(body)
	if string(body) != "" {
		t.Fatalf("conflicting credentials returned %q", body)
	}
}

func TestRequireCookieCSRF(t *testing.T) {
	t.Setenv("API_CORS_ORIGINS", "https://taskly.example")
	app := fiber.New()
	app.Use(RequireCookieCSRF)
	app.Post("/change", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })

	request := func(origin, csrf string, withCookie bool) int {
		req := httptest.NewRequest(fiber.MethodPost, "/change", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if csrf != "" {
			req.Header.Set(CSRFHeaderName, csrf)
		}
		if withCookie {
			req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session"})
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}

	if got := request("https://taskly.example", CSRFHeaderValue, true); got != fiber.StatusNoContent {
		t.Fatalf("valid cookie request status = %d", got)
	}
	if got := request("https://evil.example", CSRFHeaderValue, true); got != fiber.StatusForbidden {
		t.Fatalf("invalid origin status = %d", got)
	}
	if got := request("https://taskly.example", "", true); got != fiber.StatusForbidden {
		t.Fatalf("missing csrf header status = %d", got)
	}
	if got := request("", "", false); got != fiber.StatusNoContent {
		t.Fatalf("non-cookie client status = %d", got)
	}
}
