package authsync

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLifecycleSyncDisabled(t *testing.T) {
	t.Setenv("AUTH_ACCOUNT_LIFECYCLE_EXTERNAL", "0")
	if err := SyncDeleted(context.Background(), 42, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := Erase(context.Background(), 42); err != nil {
		t.Fatal(err)
	}
}

func TestLifecycleSyncRequests(t *testing.T) {
	deletedAt := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		if r.URL.Path != "/internal/users/42" && r.URL.Path != "/internal/users/42/lifecycle" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer internal-test-token" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		if r.Method == http.MethodPut {
			var body lifecycleRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if !body.Deleted || body.DeletedAt != deletedAt.Format(time.RFC3339) {
				t.Fatalf("body = %+v", body)
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	t.Setenv("AUTH_ACCOUNT_LIFECYCLE_EXTERNAL", "1")
	t.Setenv("AUTH_LIFECYCLE_SYNC_URL", server.URL)
	t.Setenv("AUTH_INTERNAL_TOKEN", "internal-test-token")
	if err := SyncDeleted(context.Background(), 42, deletedAt); err != nil {
		t.Fatal(err)
	}
	if err := Erase(context.Background(), 42); err != nil {
		t.Fatal(err)
	}
	if len(methods) != 2 || methods[0] != http.MethodPut || methods[1] != http.MethodDelete {
		t.Fatalf("methods = %v", methods)
	}
}

func TestLifecycleSyncRejectsUnsafeProductionConfig(t *testing.T) {
	t.Setenv("AUTH_ACCOUNT_LIFECYCLE_EXTERNAL", "1")
	t.Setenv("AUTH_LIFECYCLE_SYNC_URL", "http://auth.example.test")
	t.Setenv("AUTH_INTERNAL_TOKEN", "short")
	t.Setenv("NODE_ENV", "production")
	if err := SyncDeleted(context.Background(), 42, time.Now()); !errors.Is(err, ErrLifecycleSync) {
		t.Fatalf("error = %v, want lifecycle sync error", err)
	}
}

func TestLifecycleSyncReportsHTTPFailureWithoutBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "private upstream detail", http.StatusBadGateway)
	}))
	defer server.Close()
	t.Setenv("AUTH_ACCOUNT_LIFECYCLE_EXTERNAL", "1")
	t.Setenv("AUTH_LIFECYCLE_SYNC_URL", server.URL)
	t.Setenv("AUTH_INTERNAL_TOKEN", "internal-test-token")
	err := Erase(context.Background(), 42)
	if !errors.Is(err, ErrLifecycleSync) || err.Error() != "auth lifecycle synchronization: status 502" {
		t.Fatalf("error = %v", err)
	}
}

func TestEraseReportsUserNotDeleted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "active", http.StatusConflict)
	}))
	defer server.Close()
	t.Setenv("AUTH_ACCOUNT_LIFECYCLE_EXTERNAL", "1")
	t.Setenv("AUTH_LIFECYCLE_SYNC_URL", server.URL)
	t.Setenv("AUTH_INTERNAL_TOKEN", "internal-test-token")
	if err := Erase(context.Background(), 42); !errors.Is(err, ErrLifecycleNotDeleted) || !errors.Is(err, ErrLifecycleSync) {
		t.Fatalf("error = %v", err)
	}
}
