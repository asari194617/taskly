package utils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/authsync"
	"github.com/asari194617/taskly/apps/server/internal/domain"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSoftDeleteUserSyncsExactDeletionTime(t *testing.T) {
	var request struct {
		Deleted   bool   `json:"deleted"`
		DeletedAt string `json:"deleted_at"`
	}
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/internal/users/42/lifecycle" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer auth.Close()
	t.Setenv("AUTH_ACCOUNT_LIFECYCLE_EXTERNAL", "1")
	t.Setenv("AUTH_LIFECYCLE_SYNC_URL", auth.URL)
	t.Setenv("AUTH_INTERNAL_TOKEN", "internal-test-token")
	t.Setenv("NODE_ENV", "development")
	db := lifecycleTestDB(t, "success")
	if err := db.Create(&domain.User{ID: 42, Email: "delete@example.test"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := SoftDeleteUser(context.Background(), 42, db, nil); err != nil {
		t.Fatal(err)
	}
	var user domain.User
	if err := db.First(&user, 42).Error; err != nil {
		t.Fatal(err)
	}
	if !user.IsDeleted || user.DeletedAt == nil || !request.Deleted || request.DeletedAt != user.DeletedAt.UTC().Format("2006-01-02T15:04:05Z07:00") {
		t.Fatalf("user=%+v request=%+v", user, request)
	}
}

func TestSoftDeleteUserKeepsAccountClosedWhenSyncFails(t *testing.T) {
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer auth.Close()
	t.Setenv("AUTH_ACCOUNT_LIFECYCLE_EXTERNAL", "1")
	t.Setenv("AUTH_LIFECYCLE_SYNC_URL", auth.URL)
	t.Setenv("AUTH_INTERNAL_TOKEN", "internal-test-token")
	t.Setenv("NODE_ENV", "development")
	db := lifecycleTestDB(t, "failure")
	if err := db.Create(&domain.User{ID: 42, Email: "delete-failure@example.test"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&domain.Session{UserID: 42, ClientPlatform: "web", HashedToken: "session"}).Error; err != nil {
		t.Fatal(err)
	}
	err := SoftDeleteUser(context.Background(), 42, db, nil)
	if !errors.Is(err, authsync.ErrLifecycleSync) {
		t.Fatalf("error = %v", err)
	}
	var user domain.User
	if err := db.First(&user, 42).Error; err != nil {
		t.Fatal(err)
	}
	var sessions int64
	if err := db.Model(&domain.Session{}).Where("user_id = ?", 42).Count(&sessions).Error; err != nil {
		t.Fatal(err)
	}
	if !user.IsDeleted || user.DeletedAt == nil || sessions != 0 {
		t.Fatalf("deleted=%t deletedAt=%v sessions=%d", user.IsDeleted, user.DeletedAt, sessions)
	}
}

func lifecycleTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:lifecycle-"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.User{}, &domain.Session{}, &domain.APIKey{}, &domain.IcalKey{}, &domain.MCP{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestRestoreUserSyncAndFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			calls := 0
			auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var input map[string]any
				if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
					t.Error(err)
				}
				if r.Method != "PUT" || r.URL.Path != "/internal/users/42/lifecycle" || input["deleted"] != false {
					t.Errorf("request: %s %s %+v", r.Method, r.URL.Path, input)
				}
				if fail {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer auth.Close()
			t.Setenv("AUTH_ACCOUNT_LIFECYCLE_EXTERNAL", "1")
			t.Setenv("AUTH_LIFECYCLE_SYNC_URL", auth.URL)
			t.Setenv("AUTH_INTERNAL_TOKEN", "internal-test-token")
			t.Setenv("NODE_ENV", "development")
			db := lifecycleTestDB(t, "restore-"+fmt.Sprint(fail))
			deletedAt := time.Now().UTC().Truncate(time.Second)
			if err := db.Create(&domain.User{ID: 42, Email: "restore@example.test", IsDeleted: true, DeletedAt: &deletedAt}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&domain.Session{UserID: 42, ClientPlatform: "web", HashedToken: "stale"}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&domain.APIKey{UserID: 42, IsValid: true}).Error; err != nil {
				t.Fatal(err)
			}
			err := RestoreUser(context.Background(), 42, db, nil)
			if fail && !errors.Is(err, authsync.ErrLifecycleSync) {
				t.Fatalf("error=%v", err)
			}
			if !fail && err != nil {
				t.Fatal(err)
			}
			var user domain.User
			if err := db.First(&user, 42).Error; err != nil {
				t.Fatal(err)
			}
			var sessions, keys int64
			db.Model(&domain.Session{}).Where("user_id = ?", 42).Count(&sessions)
			db.Model(&domain.APIKey{}).Where("user_id = ? AND is_valid = ?", 42, true).Count(&keys)
			if user.IsDeleted != fail || (user.DeletedAt != nil) != fail || sessions != map[bool]int64{true: 1, false: 0}[fail] || keys != map[bool]int64{true: 1, false: 0}[fail] {
				t.Fatalf("deleted=%v date=%v sessions=%d keys=%d", user.IsDeleted, user.DeletedAt, sessions, keys)
			}
			if !fail {
				if err := RestoreUser(context.Background(), 42, db, nil); err != nil {
					t.Fatal(err)
				}
				if calls != 1 {
					t.Fatalf("repeat called auth %d times", calls)
				}
			}
			if err := RestoreUser(context.Background(), 999, db, nil); !errors.Is(err, gorm.ErrRecordNotFound) {
				t.Fatalf("missing user error=%v", err)
			}
		})
	}
}
