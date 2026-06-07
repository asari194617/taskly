package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/asari194617/taskly/apps/server/internal/domain"
)

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
