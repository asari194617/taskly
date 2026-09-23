package authsync

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSyncBlockDisabled(t *testing.T) {
	t.Setenv("AUTH_BLOCKS_EXTERNAL", "0")
	if err := SyncBlock(context.Background(), 42, true, nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestSyncBlockRequest(t *testing.T) {
	reason := "abuse"
	expires := time.Date(2026, 9, 24, 1, 2, 3, 0, time.FixedZone("JST", 9*60*60))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/internal/users/42/block" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-internal-token" {
			t.Errorf("authorization = %q", got)
		}
		var payload blockRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if !payload.Active || payload.Reason == nil || *payload.Reason != reason || payload.ExpiresAt == nil || *payload.ExpiresAt != "2026-09-23T16:02:03Z" {
			t.Errorf("payload = %#v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"active":true}`))
	}))
	defer server.Close()
	t.Setenv("AUTH_BLOCKS_EXTERNAL", "1")
	t.Setenv("AUTH_BLOCK_SYNC_URL", server.URL)
	t.Setenv("AUTH_INTERNAL_TOKEN", "test-internal-token")
	t.Setenv("NODE_ENV", "development")
	if err := SyncBlock(context.Background(), 42, true, &reason, &expires); err != nil {
		t.Fatal(err)
	}
}

func TestSyncBlockRejectsUnsafeProductionConfig(t *testing.T) {
	t.Setenv("AUTH_BLOCKS_EXTERNAL", "1")
	t.Setenv("AUTH_BLOCK_SYNC_URL", "http://example.com")
	t.Setenv("AUTH_INTERNAL_TOKEN", "short")
	t.Setenv("NODE_ENV", "production")
	if err := SyncBlock(context.Background(), 42, false, nil, nil); err == nil {
		t.Fatal("expected unsafe configuration to be rejected")
	}
}
