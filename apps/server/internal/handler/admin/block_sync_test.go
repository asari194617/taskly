package adminHandler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAdminBlockRoutesSyncAuthState(t *testing.T) {
	var mu sync.Mutex
	states := make([]bool, 0, 2)
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/internal/users/1/block" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-internal-token" {
			t.Errorf("unexpected authorization header")
		}
		var payload struct {
			Active bool `json:"active"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		mu.Lock()
		states = append(states, payload.Active)
		mu.Unlock()
		w.Write([]byte(`{"active":true}`))
	}))
	defer auth.Close()
	t.Setenv("AUTH_BLOCKS_EXTERNAL", "1")
	t.Setenv("AUTH_BLOCK_SYNC_URL", auth.URL)
	t.Setenv("AUTH_INTERNAL_TOKEN", "test-internal-token")
	t.Setenv("NODE_ENV", "development")

	db, err := gorm.Open(sqlite.Open("file:admin-block-sync?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.User{}, &domain.Block{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&domain.User{ID: 1, Email: "blocked@example.test"}).Error; err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	NewAdminHandler(db, nil, nil).RegisterAdminUsersRoutes(app.Group("/admin/users"))

	request := httptest.NewRequest(http.MethodPost, "/admin/users/1/block", strings.NewReader(`{"reason":"abuse"}`))
	request.Header.Set("Content-Type", "application/json")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("block status = %d", response.StatusCode)
	}
	request = httptest.NewRequest(http.MethodPost, "/admin/users/1/unblock", nil)
	response, err = app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("unblock status = %d", response.StatusCode)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(states) != 2 || !states[0] || states[1] {
		t.Fatalf("synced states = %v", states)
	}
}

func TestAdminBlockFailsClosedWhenAuthSyncFails(t *testing.T) {
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer auth.Close()
	t.Setenv("AUTH_BLOCKS_EXTERNAL", "1")
	t.Setenv("AUTH_BLOCK_SYNC_URL", auth.URL)
	t.Setenv("AUTH_INTERNAL_TOKEN", "test-internal-token")
	t.Setenv("NODE_ENV", "development")

	db, err := gorm.Open(sqlite.Open("file:admin-block-sync-failure?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.User{}, &domain.Block{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&domain.User{ID: 1, Email: "blocked-failure@example.test"}).Error; err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	NewAdminHandler(db, nil, nil).RegisterAdminUsersRoutes(app.Group("/admin/users"))
	request := httptest.NewRequest(http.MethodPost, "/admin/users/1/block", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusBadGateway)
	}
	var count int64
	if err := db.Model(&domain.Block{}).Where("user_id = ? AND expires_at IS NULL", 1).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("active app blocks = %d, want 1", count)
	}
}
