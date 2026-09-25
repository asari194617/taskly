package server

import (
	"strings"
	"testing"
)

func TestValidateAuthMigrationConfig(t *testing.T) {
	for _, key := range append(append([]string{}, authExternalFlags...),
		"AUTH_WRITES_PAUSED", "AUTH_ISSUER", "AUTH_CLIENT_ID", "AUTH_CLIENT_SECRET",
		"AUTH_REDIRECT_URI", "WEB_PUBLIC_URL", "AUTH_INTERNAL_TOKEN",
		"AUTH_BLOCK_SYNC_URL", "AUTH_LIFECYCLE_SYNC_URL", "AUTH_USER_SYNC_URL") {
		t.Setenv(key, "")
	}
	t.Setenv("NODE_ENV", "production")

	t.Run("legacy configuration remains valid", func(t *testing.T) {
		if err := validateAuthMigrationConfig(); err != nil {
			t.Fatalf("validateAuthMigrationConfig() error = %v", err)
		}
	})

	t.Run("dormant complete configuration remains valid", func(t *testing.T) {
		setOAuthClientConfig(t)
		for _, key := range authExternalFlags {
			t.Setenv(key, "0")
		}
		if err := validateAuthMigrationConfig(); err != nil {
			t.Fatalf("validateAuthMigrationConfig() error = %v", err)
		}
	})

	t.Run("partial ownership transfer is rejected", func(t *testing.T) {
		setOAuthClientConfig(t)
		t.Setenv(authExternalFlags[0], "1")
		if err := validateAuthMigrationConfig(); err == nil || !strings.Contains(err.Error(), "enabled together") {
			t.Fatalf("validateAuthMigrationConfig() error = %v", err)
		}
	})

	t.Run("complete ownership transfer requires internal routes", func(t *testing.T) {
		setOAuthClientConfig(t)
		for _, key := range authExternalFlags {
			t.Setenv(key, "1")
		}
		t.Setenv("AUTH_INTERNAL_TOKEN", strings.Repeat("a", 32))
		if err := validateAuthMigrationConfig(); err == nil || !strings.Contains(err.Error(), "AUTH_BLOCK_SYNC_URL") {
			t.Fatalf("validateAuthMigrationConfig() error = %v", err)
		}
	})

	t.Run("complete ownership transfer is valid", func(t *testing.T) {
		setOAuthClientConfig(t)
		for _, key := range authExternalFlags {
			t.Setenv(key, "1")
		}
		t.Setenv("AUTH_INTERNAL_TOKEN", strings.Repeat("a", 32))
		t.Setenv("AUTH_BLOCK_SYNC_URL", "http://auth:3067")
		t.Setenv("AUTH_LIFECYCLE_SYNC_URL", "http://auth:3067")
		t.Setenv("AUTH_USER_SYNC_URL", "http://auth:3067")
		if err := validateAuthMigrationConfig(); err != nil {
			t.Fatalf("validateAuthMigrationConfig() error = %v", err)
		}
	})
}

func setOAuthClientConfig(t *testing.T) {
	t.Helper()
	t.Setenv("AUTH_ISSUER", "https://taskly-auth-api.iy0306.com")
	t.Setenv("AUTH_CLIENT_ID", "taskly-web")
	t.Setenv("AUTH_CLIENT_SECRET", strings.Repeat("b", 32))
	t.Setenv("AUTH_REDIRECT_URI", "https://api.taskly.iy0306.com/auth/oauth/callback")
	t.Setenv("WEB_PUBLIC_URL", "https://taskly.iy0306.com")
}
