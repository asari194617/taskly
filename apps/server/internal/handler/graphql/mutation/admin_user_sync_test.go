package gqlMutation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/asari194617/taskly/apps/server/internal/authsync"
	"github.com/asari194617/taskly/apps/server/internal/domain"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAdminUserProjectionCreateRetryAndUpdate(t *testing.T) {
	postCount := 0
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			postCount++
			w.WriteHeader(map[bool]int{true: http.StatusCreated, false: http.StatusOK}[postCount == 1])
			json.NewEncoder(w).Encode(authsync.UserProvisionResult{ID: 42, Created: postCount == 1})
		case http.MethodPatch:
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]uint{"id": 42})
		default:
			t.Fatalf("method = %s", r.Method)
		}
	}))
	defer auth.Close()
	t.Setenv("AUTH_USERS_EXTERNAL", "1")
	t.Setenv("AUTH_USER_SYNC_URL", auth.URL)
	t.Setenv("AUTH_INTERNAL_TOKEN", "internal-test-token")
	t.Setenv("NODE_ENV", "development")
	db := userProjectionTestDB(t, "success")
	h := &Handler{db: db}
	record, created, err := h.createAdminUser(context.Background(), " USER@Example.Test ", " User ", "ja")
	if err != nil || !created || record.ID != 42 || record.Email != "user@example.test" {
		t.Fatalf("record=%+v created=%t error=%v", record, created, err)
	}
	record, created, err = h.createAdminUser(context.Background(), "user@example.test", "ignored", "en")
	if err != nil || created || record.ID != 42 {
		t.Fatalf("retry record=%+v created=%t error=%v", record, created, err)
	}
	if err := db.Create(&domain.Session{UserID: 42, ClientPlatform: "web", HashedToken: "old"}).Error; err != nil {
		t.Fatal(err)
	}
	email := "updated@example.test"
	if err := h.updateAdminUser(context.Background(), 42, map[string]any{"email": email}); err != nil {
		t.Fatal(err)
	}
	var updated domain.User
	if err := db.First(&updated, 42).Error; err != nil {
		t.Fatal(err)
	}
	var sessions int64
	if err := db.Model(&domain.Session{}).Where("user_id = ?", 42).Count(&sessions).Error; err != nil {
		t.Fatal(err)
	}
	if updated.Email != email || sessions != 0 {
		t.Fatalf("email=%q sessions=%d", updated.Email, sessions)
	}
}

func TestAdminUserProjectionFailureLeavesAppUnchanged(t *testing.T) {
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer auth.Close()
	t.Setenv("AUTH_USERS_EXTERNAL", "1")
	t.Setenv("AUTH_USER_SYNC_URL", auth.URL)
	t.Setenv("AUTH_INTERNAL_TOKEN", "internal-test-token")
	t.Setenv("NODE_ENV", "development")
	db := userProjectionTestDB(t, "failure")
	if err := db.Create(&domain.User{ID: 42, Email: "old@example.test"}).Error; err != nil {
		t.Fatal(err)
	}
	err := (&Handler{db: db}).updateAdminUser(context.Background(), 42, map[string]any{"email": "new@example.test"})
	if !errors.Is(err, authsync.ErrUserSync) {
		t.Fatalf("error = %v", err)
	}
	var user domain.User
	if err := db.First(&user, 42).Error; err != nil {
		t.Fatal(err)
	}
	if user.Email != "old@example.test" {
		t.Fatalf("email = %q", user.Email)
	}
}

func userProjectionTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:user-projection-"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.User{}, &domain.Session{}); err != nil {
		t.Fatal(err)
	}
	return db
}
