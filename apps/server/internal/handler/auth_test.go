package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
)

func TestFormatLoginCodeSubject(t *testing.T) {
	got := formatLoginCodeSubject("ログインコード", "123456")
	want := "ログインコード: 123456"
	if got != want {
		t.Fatalf("formatLoginCodeSubject() = %q, want %q", got, want)
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
	return &httptest.ResponseRecorder{Code: resp.StatusCode, Body: bytes.NewBuffer(data)}, data, nil
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
