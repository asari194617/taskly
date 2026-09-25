package handler

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

func TestOAuthCallbackBindsStateAndPreservesUserID(t *testing.T) {
	_, mr, db, user := newAuthTransactionTestApp(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	var expectedChallenge string
	var revokeCalls atomic.Int32
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			id, secret, ok := r.BasicAuth()
			if !ok || id != "taskly-web" || secret != "test-secret" || r.FormValue("code") != "sample-code" {
				t.Errorf("invalid token exchange request")
				w.WriteHeader(401)
				return
			}
			h := sha256.Sum256([]byte(r.FormValue("code_verifier")))
			if base64.RawURLEncoding.EncodeToString(h[:]) != expectedChallenge {
				t.Errorf("PKCE verifier mismatch")
				w.WriteHeader(400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "test-access", "refresh_token": "test-refresh", "token_type": "Bearer"})
		case "/oauth/userinfo":
			if r.Header.Get("Authorization") != "Bearer test-access" {
				t.Errorf("missing bearer token")
				w.WriteHeader(401)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"sub": "1", "email": "mfa@example.com"})
		case "/oauth/revoke":
			id, secret, ok := r.BasicAuth()
			if !ok || id != "taskly-web" || secret != "test-secret" || r.FormValue("token") != "test-refresh" {
				t.Errorf("invalid token revocation request")
				w.WriteHeader(401)
				return
			}
			revokeCalls.Add(1)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(404)
		}
	}))
	defer auth.Close()
	t.Setenv("AUTH_ISSUER", auth.URL)
	t.Setenv("AUTH_CLIENT_ID", "taskly-web")
	t.Setenv("AUTH_CLIENT_SECRET", "test-secret")
	t.Setenv("AUTH_REDIRECT_URI", "http://localhost:3063/auth/oauth/callback")
	t.Setenv("WEB_PUBLIC_URL", "http://localhost:4321")
	app := fiber.New()
	NewHandler(db, rdb, nil).RegisterOAuthRoutes(app.Group("/auth/oauth"))
	start, err := app.Test(httptest.NewRequest(http.MethodGet, "/auth/oauth/start?next=/home", nil))
	if err != nil || start.StatusCode != 302 {
		t.Fatalf("start: status=%v err=%v", start.StatusCode, err)
	}
	authorizeURL, err := url.Parse(start.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	state := authorizeURL.Query().Get("state")
	expectedChallenge = authorizeURL.Query().Get("code_challenge")
	if state == "" || expectedChallenge == "" {
		t.Fatal("state or challenge missing")
	}
	var stateCookie *http.Cookie
	for _, cookie := range start.Cookies() {
		if cookie.Name == oauthStateCookie {
			stateCookie = cookie
		}
	}
	if stateCookie == nil {
		t.Fatal("state cookie missing")
	}
	callbackURL := "/auth/oauth/callback?state=" + url.QueryEscape(state) + "&iss=" + url.QueryEscape(auth.URL) + "&code=sample-code"
	bad, err := app.Test(httptest.NewRequest(http.MethodGet, callbackURL, nil))
	if err != nil || bad.StatusCode != 400 {
		t.Fatalf("unbound state accepted: %v %v", bad.StatusCode, err)
	}
	req := httptest.NewRequest(http.MethodGet, callbackURL, nil)
	req.AddCookie(stateCookie)
	resp, err := app.Test(req)
	if err != nil || resp.StatusCode != 302 {
		t.Fatalf("callback: status=%v err=%v", resp.StatusCode, err)
	}
	if resp.Header.Get("Location") != "http://localhost:4321/home" {
		t.Fatalf("redirect=%q", resp.Header.Get("Location"))
	}
	var sessionCookie *http.Cookie
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "taskly_session" {
			sessionCookie = cookie
		}
	}
	if sessionCookie == nil || !sessionCookie.HttpOnly {
		t.Fatal("session cookie missing")
	}
	if revokeCalls.Load() != 1 {
		t.Fatalf("refresh token not revoked before session issue: %d calls", revokeCalls.Load())
	}
	var session domain.Session
	if err := db.Where("user_id=?", user.ID).First(&session).Error; err != nil {
		t.Fatal(err)
	}
	replay := httptest.NewRequest(http.MethodGet, callbackURL, nil)
	replay.AddCookie(stateCookie)
	replayResponse, err := app.Test(replay)
	if err != nil || replayResponse.StatusCode != 400 {
		t.Fatalf("callback replay accepted: %v %v", replayResponse.StatusCode, err)
	}
}

func TestSafeOAuthNext(t *testing.T) {
	for _, raw := range []string{"//example.com", "/\\example.com", "https://example.com", "/\r\nLocation: https://example.com"} {
		if got := safeOAuthNext(raw); got != "/home" {
			t.Fatalf("unsafe next %q accepted as %q", raw, got)
		}
	}
	if got := safeOAuthNext("/settings/basic"); got != "/settings/basic" {
		t.Fatalf("safe path changed: %q", got)
	}
}
