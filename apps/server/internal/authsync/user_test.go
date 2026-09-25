package authsync

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUserSyncRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer internal-test-token" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		switch r.Method {
		case http.MethodPost:
			json.NewEncoder(w).Encode(UserProvisionResult{ID: 42, Created: true})
		case http.MethodPatch:
			if r.URL.Path != "/internal/users/42" {
				t.Fatalf("path = %q", r.URL.Path)
			}
			json.NewEncoder(w).Encode(map[string]uint{"id": 42})
		default:
			t.Fatalf("method = %s", r.Method)
		}
	}))
	defer server.Close()
	t.Setenv("AUTH_USERS_EXTERNAL", "1")
	t.Setenv("AUTH_USER_SYNC_URL", server.URL)
	t.Setenv("AUTH_INTERNAL_TOKEN", "internal-test-token")
	result, err := ProvisionUser(context.Background(), UserProvision{Email: "user@example.test", NotificationLang: "ja"})
	if err != nil || result.ID != 42 || !result.Created {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	email := "updated@example.test"
	if err := UpdateUser(context.Background(), 42, UserUpdate{Email: &email}); err != nil {
		t.Fatal(err)
	}
}

func TestUserSyncRejectsUnsafeProductionConfig(t *testing.T) {
	t.Setenv("AUTH_USERS_EXTERNAL", "1")
	t.Setenv("AUTH_USER_SYNC_URL", "http://auth.example.test")
	t.Setenv("AUTH_INTERNAL_TOKEN", "short")
	t.Setenv("NODE_ENV", "production")
	_, err := ProvisionUser(context.Background(), UserProvision{})
	if !errors.Is(err, ErrUserSync) {
		t.Fatalf("error = %v", err)
	}
}
