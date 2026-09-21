package utils

import (
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
	a := GenerateIcalToken()
	b := GenerateIcalToken()
	if a == b {
		t.Fatalf("expected different tokens")
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
