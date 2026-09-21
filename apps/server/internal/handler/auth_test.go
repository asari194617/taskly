package handler

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
)

func TestFormatLoginCodeSubject(t *testing.T) {
	got := formatLoginCodeSubject("ログインコード", "123456")
	want := "ログインコード: 123456"
	if got != want {
		t.Fatalf("formatLoginCodeSubject() = %q, want %q", got, want)
	}
}

func TestGenerateLoginCode(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input []byte
		want  string
	}{
		{"leading zeroes", []byte{0, 0, 1}, "000001"},
		{"upper bound", []byte{0x0f, 0x42, 0x3f}, "999999"},
		{"rejection sampling", []byte{0x0f, 0x42, 0x40, 0, 0, 2}, "000002"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := generateLoginCode(bytes.NewReader(tc.input))
			if err != nil || got != tc.want {
				t.Fatalf("generateLoginCode() = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	if code, err := generateLoginCode(bytes.NewReader(nil)); err == nil || code != "" {
		t.Fatalf("random source failure = %q, %v", code, err)
	}
}

func TestRequestCodeStopsWhenRandomSourceFails(t *testing.T) {
	app, mr, db, _ := newAuthTransactionTestApp(t)
	// Set the reader on the registered handler through a fresh route fixture.
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	h := NewHandler(db, rdb, nil)
	h.loginCodeRandom = bytes.NewReader(nil)
	app = fiber.New()
	h.RegisterAuthRoutes(app.Group("/auth"))
	rw, _, err := doJSONRequest(app, fiber.MethodPost, "/auth/request-code", map[string]string{"email": "mfa@example.com"}, nil)
	if err != nil || rw.Code != fiber.StatusInternalServerError {
		t.Fatalf("request status=%d err=%v", rw.Code, err)
	}
	var count int64
	if err := db.Model(&domain.LoginCode{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("issued codes=%d err=%v", count, err)
	}
}

func TestIssuedLoginCodeAuthenticates(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	mail := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.WriteString(conn, "220 test smtp\r\n")
		reader := bufio.NewReader(conn)
		var message strings.Builder
		inData := false
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if inData {
				if strings.TrimSpace(line) == "." {
					mail <- message.String()
					inData = false
					_, _ = io.WriteString(conn, "250 queued\r\n")
				} else {
					message.WriteString(line)
				}
				continue
			}
			switch {
			case strings.HasPrefix(line, "DATA"):
				inData = true
				_, _ = io.WriteString(conn, "354 send data\r\n")
			case strings.HasPrefix(line, "QUIT"):
				_, _ = io.WriteString(conn, "221 bye\r\n")
				return
			default:
				_, _ = io.WriteString(conn, "250 ok\r\n")
			}
		}
	}()
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SMTP_HOST", host)
	t.Setenv("SMTP_PORT", port)
	t.Setenv("EMAIL_COMPOSE_GRPC_ADDR", "127.0.0.1:1")
	t.Setenv("NODE_ENV", "production")
	t.Setenv("API_CORS_ORIGINS", "https://taskly.example")
	app, mr, db, user := newAuthTransactionTestApp(t)
	if err := db.Model(&user).Update("hashed_password", nil).Error; err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	h := NewHandler(db, rdb, nil)
	h.loginCodeRandom = bytes.NewReader([]byte{0, 0, 7})
	app = fiber.New()
	h.RegisterAuthRoutes(app.Group("/auth"))
	rw, data, err := doJSONRequest(app, fiber.MethodPost, "/auth/request-code", map[string]string{"email": user.Email}, nil)
	if err != nil || rw.Code != fiber.StatusOK {
		t.Fatalf("issue status=%d err=%v body=%s", rw.Code, err, data)
	}
	select {
	case delivered := <-mail:
		if !strings.Contains(delivered, "000007") {
			t.Fatalf("mail did not contain issued code")
		}
	case <-time.After(time.Second):
		t.Fatal("mail was not delivered")
	}
	var issued domain.LoginCode
	if err := db.Where("email = ?", user.Email).First(&issued).Error; err != nil {
		t.Fatal(err)
	}
	if issued.HashedCode != utils.Sha256Hex([]byte("000007")) || issued.ExpiresAt.Sub(issued.CreatedAt) != 10*time.Minute {
		t.Fatalf("issued code state: %+v", issued)
	}
	rw, data, err = doJSONRequest(app, fiber.MethodPost, "/auth/login", map[string]string{"email": user.Email, "code": "000007"}, map[string]string{"Origin": "https://taskly.example", "X-Client-Platform": "web"})
	if err != nil || rw.Code != fiber.StatusOK {
		t.Fatalf("login status=%d err=%v body=%s", rw.Code, err, data)
	}
}

func TestLoginCodeDevelopmentLoggingGate(t *testing.T) {
	for _, tc := range []struct {
		env  string
		want bool
	}{
		{"development", true}, {"DEVELOPMENT", true}, {"production", false}, {"", false}, {"staging", false},
	} {
		if got := shouldLogLoginCode(tc.env); got != tc.want {
			t.Errorf("shouldLogLoginCode(%q) = %v, want %v", tc.env, got, tc.want)
		}
	}
}

func TestLoginCodeExpiryAttemptsAndConsumption(t *testing.T) {
	for _, scenario := range []string{"expired", "attempt limit", "single use"} {
		t.Run(scenario, func(t *testing.T) {
			app, _, db, user := newAuthTransactionTestApp(t)
			if err := db.Model(&user).Update("hashed_password", nil).Error; err != nil {
				t.Fatal(err)
			}
			code := "123456"
			now := time.Now()
			expiry := now.Add(10 * time.Minute)
			if scenario == "expired" {
				expiry = now.Add(-time.Second)
			}
			loginCode := domain.LoginCode{Email: user.Email, HashedCode: utils.Sha256Hex([]byte(code)), CreatedAt: now, ExpiresAt: expiry}
			if err := db.Create(&loginCode).Error; err != nil {
				t.Fatal(err)
			}
			login := func(value string) int {
				rw, data, err := doJSONRequest(app, fiber.MethodPost, "/auth/login", map[string]string{"email": user.Email, "code": value}, nil)
				if err != nil {
					t.Fatal(err)
				}
				if rw.Code != fiber.StatusOK && rw.Code != fiber.StatusUnauthorized && rw.Code != fiber.StatusTooManyRequests {
					t.Fatalf("unexpected login status=%d body=%s", rw.Code, data)
				}
				return rw.Code
			}
			switch scenario {
			case "expired":
				if got := login(code); got != fiber.StatusUnauthorized {
					t.Fatalf("expired code status=%d", got)
				}
			case "attempt limit":
				for i := 0; i < 5; i++ {
					if got := login("000000"); got != fiber.StatusUnauthorized {
						t.Fatalf("attempt %d status=%d", i+1, got)
					}
				}
				if got := login(code); got != fiber.StatusTooManyRequests {
					t.Fatalf("locked code status=%d", got)
				}
			case "single use":
				if got := login(code); got != fiber.StatusOK {
					t.Fatalf("valid code status=%d", got)
				}
				if got := login(code); got != fiber.StatusUnauthorized {
					t.Fatalf("reused code status=%d", got)
				}
			}
			if err := db.First(&loginCode, loginCode.ID).Error; err != nil {
				t.Fatal(err)
			}
			if scenario == "attempt limit" && (loginCode.FailedAttempts != 5 || loginCode.LockedUntil == nil) {
				t.Fatalf("attempt limit state: %+v", loginCode)
			}
			if scenario == "single use" && loginCode.ConsumedAt == nil {
				t.Fatal("successful code was not consumed")
			}
		})
	}
}

func TestWebLoginUsesHttpOnlyCookieWithoutReturningToken(t *testing.T) {
	app, _, db, user := newAuthTransactionTestApp(t)
	// This fixture normally requires MFA; disable email-code login so the
	// password completes the session in a single request.
	if err := db.Model(&user).Update("email_code_allowed", false).Error; err != nil {
		t.Fatal(err)
	}
	rw, data, err := doJSONRequest(app, fiber.MethodPost, "/auth/login", map[string]string{
		"email": "mfa@example.com", "password": "correct-password",
	}, map[string]string{"X-Client-Platform": "web"})
	if err != nil {
		t.Fatal(err)
	}
	if rw.Code != fiber.StatusOK {
		t.Fatalf("status=%d body=%s", rw.Code, data)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["authenticated"] != true || payload["token"] != nil || payload["jwt"] != nil {
		t.Fatalf("unexpected web login payload: %s", data)
	}
	cookies := rw.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies=%d want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != "taskly_session" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Value == "" {
		t.Fatalf("unexpected session cookie: %#v", cookie)
	}
}

func TestNativeLoginKeepsBearerTokenAndRejectsBrowserSpoofing(t *testing.T) {
	app, _, db, user := newAuthTransactionTestApp(t)
	if err := db.Model(&user).Update("email_code_allowed", false).Error; err != nil {
		t.Fatal(err)
	}
	body := map[string]string{"email": "mfa@example.com", "password": "correct-password"}
	rw, data, err := doJSONRequest(app, fiber.MethodPost, "/auth/login", body, map[string]string{"X-Client-Platform": "ios"})
	if err != nil || rw.Code != fiber.StatusOK {
		t.Fatalf("native login status=%d err=%v body=%s", rw.Code, err, data)
	}
	var payload map[string]any
	_ = json.Unmarshal(data, &payload)
	if payload["token"] == nil || len(rw.Result().Cookies()) != 0 {
		t.Fatalf("unexpected native login response: body=%s cookies=%v", data, rw.Result().Cookies())
	}

	rw, _, err = doJSONRequest(app, fiber.MethodPost, "/auth/login", body, map[string]string{
		"X-Client-Platform": "ios",
		"Origin":            "https://taskly.example",
	})
	if err != nil || rw.Code != fiber.StatusForbidden {
		t.Fatalf("browser spoof status=%d err=%v", rw.Code, err)
	}
}

func TestCookieSessionCSRFAndLogoutRevokesDatabaseFallback(t *testing.T) {
	t.Setenv("API_CORS_ORIGINS", "https://taskly.example")
	app, mr, db, user := newAuthTransactionTestApp(t)
	if err := db.Model(&user).Update("email_code_allowed", false).Error; err != nil {
		t.Fatal(err)
	}
	rw, data, err := doJSONRequest(app, fiber.MethodPost, "/auth/login", map[string]string{
		"email": "mfa@example.com", "password": "correct-password",
	}, map[string]string{"X-Client-Platform": "web", "Origin": "https://taskly.example"})
	if err != nil || rw.Code != fiber.StatusOK {
		t.Fatalf("login status=%d err=%v body=%s", rw.Code, err, data)
	}
	cookie := rw.Result().Cookies()[0]

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	app.Get("/me", func(c *fiber.Ctx) error {
		if !utils.UserFromSession(c, db, rdb) {
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
	request := func(method, path, origin, csrf string) *http.Response {
		req := httptest.NewRequest(method, path, nil)
		req.AddCookie(cookie)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if csrf != "" {
			req.Header.Set(utils.CSRFHeaderName, csrf)
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	checkStatus := func(resp *http.Response, status int) {
		defer resp.Body.Close()
		if resp.StatusCode != status {
			t.Fatalf("status=%d want=%d", resp.StatusCode, status)
		}
	}
	checkStatus(request(fiber.MethodGet, "/me", "", ""), fiber.StatusNoContent)
	checkStatus(request(fiber.MethodPost, "/auth/logout", "https://evil.example", utils.CSRFHeaderValue), fiber.StatusForbidden)
	checkStatus(request(fiber.MethodPost, "/auth/logout", "https://taskly.example", ""), fiber.StatusForbidden)
	logout := request(fiber.MethodPost, "/auth/logout", "https://taskly.example", utils.CSRFHeaderValue)
	checkStatus(logout, fiber.StatusNoContent)
	cleared := logout.Cookies()[0]
	if cleared.Name != utils.SessionCookieName || cleared.MaxAge != -1 {
		t.Fatalf("logout did not clear cookie: %#v", cleared)
	}
	if mr.Exists("session:" + cookie.Value) {
		t.Fatal("Redis session survived logout")
	}
	if members, err := mr.SMembers(fmt.Sprintf("user_sessions:%d", user.ID)); err == nil && len(members) != 0 {
		t.Fatalf("session index survived logout: %v", members)
	}
	checkStatus(request(fiber.MethodGet, "/me", "", ""), fiber.StatusUnauthorized)
	var count int64
	if err := db.Model(&domain.Session{}).Where("hashed_token = ?", utils.Sha256Hex([]byte(cookie.Value))).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("DB fallback survived logout: count=%d err=%v", count, err)
	}
}

func doJSONRequest(app *fiber.App, method, path string, body any, headers map[string]string) (*httptest.ResponseRecorder, []byte, error) {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, nil, err
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := app.Test(req, -1)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return &httptest.ResponseRecorder{Code: resp.StatusCode, Body: bytes.NewBuffer(data), HeaderMap: resp.Header.Clone()}, data, nil
}

// TestAuthOptions verifies /auth/options responds with available methods for a user.
func TestAuthOptions_PasswordOnly(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&domain.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// create user with password and disable email-code MFA
	pw := "secret"
	hpw, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	hpwStr := string(hpw)
	u := domain.User{Email: "bob@example.com", DisplayName: "Bob", HashedPassword: &hpwStr}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Model(&u).Update("email_code_allowed", false).Error; err != nil {
		t.Fatalf("update flag: %v", err)
	}

	h := NewHandler(db, rdb, nil)
	app := fiber.New()
	h.RegisterAuthRoutes(app.Group("/auth"))

	body := map[string]string{"email": "bob@example.com"}
	rw, data, err := doJSONRequest(app, "POST", "/auth/options", body, nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if rw.Code != 200 {
		t.Fatalf("expected 200 got %d body=%s", rw.Code, string(data))
	}
	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if hasPw, _ := resp["has_password"].(bool); !hasPw {
		t.Fatalf("expected has_password true")
	}
	if emailAllowed, _ := resp["email_login_enabled"].(bool); emailAllowed {
		t.Fatalf("expected email_login_enabled false")
	}
}

func newAuthTransactionTestApp(t *testing.T) (*fiber.App, *miniredis.Miniredis, *gorm.DB, domain.User) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&domain.User{},
		&domain.LoginCode{},
		&domain.TOTPSecret{},
		&domain.RecoveryCode{},
		&domain.Block{},
		&domain.Session{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	hash := string(passwordHash)
	u := domain.User{
		Email:            "mfa@example.com",
		DisplayName:      "MFA User",
		HashedPassword:   &hash,
		EmailCodeAllowed: true,
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	h := NewHandler(db, rdb, nil)
	app := fiber.New()
	app.Use(utils.RequireCookieCSRF)
	h.RegisterAuthRoutes(app.Group("/auth"))
	return app, mr, db, u
}

func beginPasswordTransaction(t *testing.T, app *fiber.App, headers map[string]string) string {
	t.Helper()
	rw, data, err := doJSONRequest(app, "POST", "/auth/login", map[string]string{
		"email":    "mfa@example.com",
		"password": "correct-password",
	}, headers)
	if err != nil {
		t.Fatalf("begin request: %v", err)
	}
	if rw.Code != fiber.StatusAccepted {
		t.Fatalf("begin status = %d, body=%s", rw.Code, data)
	}
	var response struct {
		MFARequired   bool     `json:"mfa_required"`
		TransactionID string   `json:"auth_transaction"`
		Methods       []string `json:"methods"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("decode begin response: %v", err)
	}
	if !response.MFARequired || response.TransactionID == "" {
		t.Fatalf("unexpected begin response: %s", data)
	}
	return response.TransactionID
}

func TestPasswordMFAUsesHashedSingleUseTransaction(t *testing.T) {
	app, mr, db, _ := newAuthTransactionTestApp(t)
	headers := map[string]string{"X-Client-Platform": "web", "User-Agent": "transaction-test"}
	transactionID := beginPasswordTransaction(t, app, headers)

	if mr.Exists("auth_transaction:" + transactionID) {
		t.Fatal("raw transaction ID must not be used as the Redis key")
	}
	storedKey := authTransactionKey(transactionID)
	if !mr.Exists(storedKey) {
		t.Fatal("hashed transaction key was not stored")
	}
	payload, err := mr.Get(storedKey)
	if err != nil {
		t.Fatalf("read transaction: %v", err)
	}
	if strings.Contains(payload, "correct-password") || strings.Contains(payload, transactionID) {
		t.Fatal("transaction state contains a password or raw transaction ID")
	}

	now := time.Now()
	if err := db.Create(&domain.LoginCode{
		Email:      "mfa@example.com",
		HashedCode: "8d969eef6ecad3c29a3a629280e686cf0c3f5d5a86aff3ca12020c923adc6c92",
		// Keep this valid across runners whose local timezone differs from the
		// application's Asia/Tokyo login clock.
		ExpiresAt: now.Add(24 * time.Hour),
		CreatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create login code: %v", err)
	}
	rw, data, err := doJSONRequest(app, "POST", "/auth/login", map[string]string{
		"auth_transaction": transactionID,
		"code":             "123456",
	}, headers)
	if err != nil {
		t.Fatalf("complete request: %v", err)
	}
	if rw.Code != fiber.StatusOK {
		t.Fatalf("complete status = %d, body=%s", rw.Code, data)
	}
	if mr.Exists(storedKey) {
		t.Fatal("successful transaction was not consumed")
	}

	rw, _, err = doJSONRequest(app, "POST", "/auth/login", map[string]string{
		"auth_transaction": transactionID,
		"code":             "123456",
	}, headers)
	if err != nil {
		t.Fatalf("reuse request: %v", err)
	}
	if rw.Code != fiber.StatusUnauthorized {
		t.Fatalf("reused transaction status = %d, want 401", rw.Code)
	}
}

func TestAuthTransactionRejectsTamperingAndExpires(t *testing.T) {
	t.Run("different user", func(t *testing.T) {
		app, _, _, _ := newAuthTransactionTestApp(t)
		headers := map[string]string{"X-Client-Platform": "web", "User-Agent": "transaction-test"}
		transactionID := beginPasswordTransaction(t, app, headers)
		rw, _, err := doJSONRequest(app, "POST", "/auth/login", map[string]string{
			"auth_transaction": transactionID,
			"email":            "attacker@example.com",
			"code":             "123456",
		}, headers)
		if err != nil || rw.Code != fiber.StatusUnauthorized {
			t.Fatalf("user substitution status=%d err=%v", rw.Code, err)
		}
	})

	t.Run("different client", func(t *testing.T) {
		app, _, _, _ := newAuthTransactionTestApp(t)
		transactionID := beginPasswordTransaction(t, app, map[string]string{
			"X-Client-Platform": "web", "User-Agent": "original-client",
		})
		rw, _, err := doJSONRequest(app, "POST", "/auth/login", map[string]string{
			"auth_transaction": transactionID,
			"code":             "123456",
		}, map[string]string{"X-Client-Platform": "web", "User-Agent": "other-client"})
		if err != nil || rw.Code != fiber.StatusUnauthorized {
			t.Fatalf("client substitution status=%d err=%v", rw.Code, err)
		}
	})

	t.Run("method substitution", func(t *testing.T) {
		app, _, _, _ := newAuthTransactionTestApp(t)
		headers := map[string]string{"X-Client-Platform": "web", "User-Agent": "transaction-test"}
		transactionID := beginPasswordTransaction(t, app, headers)
		rw, _, err := doJSONRequest(app, "POST", "/auth/login", map[string]string{
			"auth_transaction": transactionID,
			"recovery_code":    "NOT-ALLOWED",
		}, headers)
		if err != nil || rw.Code != fiber.StatusUnauthorized {
			t.Fatalf("method substitution status=%d err=%v", rw.Code, err)
		}
	})

	t.Run("expired", func(t *testing.T) {
		app, mr, _, _ := newAuthTransactionTestApp(t)
		headers := map[string]string{"X-Client-Platform": "web", "User-Agent": "transaction-test"}
		transactionID := beginPasswordTransaction(t, app, headers)
		mr.FastForward(authTransactionTTL + time.Second)
		rw, _, err := doJSONRequest(app, "POST", "/auth/login", map[string]string{
			"auth_transaction": transactionID,
			"code":             "123456",
		}, headers)
		if err != nil || rw.Code != fiber.StatusUnauthorized {
			t.Fatalf("expired transaction status=%d err=%v", rw.Code, err)
		}
	})
}

func TestAuthTransactionDeletedAfterAttemptLimit(t *testing.T) {
	app, mr, _, _ := newAuthTransactionTestApp(t)
	headers := map[string]string{"X-Client-Platform": "web", "User-Agent": "transaction-test"}
	transactionID := beginPasswordTransaction(t, app, headers)
	for attempt := 1; attempt <= authTransactionMaxAttempts; attempt++ {
		rw, _, err := doJSONRequest(app, "POST", "/auth/login", map[string]string{
			"auth_transaction": transactionID,
			"code":             "000000",
		}, headers)
		if err != nil || rw.Code != fiber.StatusUnauthorized {
			t.Fatalf("attempt %d status=%d err=%v", attempt, rw.Code, err)
		}
	}
	if mr.Exists(authTransactionKey(transactionID)) {
		t.Fatal("transaction remains after the failed-attempt limit")
	}
}
