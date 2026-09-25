package server

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

var authExternalFlags = []string{
	"AUTH_PASSKEYS_EXTERNAL",
	"AUTH_PASSWORD_EXTERNAL",
	"AUTH_EMAIL_SETTINGS_EXTERNAL",
	"AUTH_RECOVERY_EXTERNAL",
	"AUTH_TOTP_EXTERNAL",
	"AUTH_BLOCKS_EXTERNAL",
	"AUTH_ACCOUNT_LIFECYCLE_EXTERNAL",
	"AUTH_USERS_EXTERNAL",
}

func validateAuthMigrationConfig() error {
	enabled := 0
	for _, key := range authExternalFlags {
		value := strings.TrimSpace(os.Getenv(key))
		if value != "" && value != "0" && value != "1" {
			return fmt.Errorf("%s must be 0 or 1", key)
		}
		if value == "1" {
			enabled++
		}
	}
	if paused := strings.TrimSpace(os.Getenv("AUTH_WRITES_PAUSED")); paused != "" && paused != "0" && paused != "1" {
		return errors.New("AUTH_WRITES_PAUSED must be 0 or 1")
	}

	oauthKeys := []string{"AUTH_ISSUER", "AUTH_CLIENT_ID", "AUTH_CLIENT_SECRET", "AUTH_REDIRECT_URI"}
	oauthConfigured := 0
	for _, key := range oauthKeys {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			oauthConfigured++
		}
	}
	if oauthConfigured != 0 && oauthConfigured != len(oauthKeys) {
		return errors.New("AUTH_ISSUER, AUTH_CLIENT_ID, AUTH_CLIENT_SECRET, and AUTH_REDIRECT_URI must be configured together")
	}
	if oauthConfigured == len(oauthKeys) && strings.EqualFold(os.Getenv("NODE_ENV"), "production") {
		for _, key := range []string{"AUTH_ISSUER", "AUTH_REDIRECT_URI", "WEB_PUBLIC_URL"} {
			if !validHTTPSURL(os.Getenv(key)) {
				return fmt.Errorf("%s must be an HTTPS URL in production", key)
			}
		}
		if len(os.Getenv("AUTH_CLIENT_SECRET")) < 32 {
			return errors.New("AUTH_CLIENT_SECRET must contain at least 32 characters in production")
		}
	}

	if enabled == 0 {
		return nil
	}
	if enabled != len(authExternalFlags) {
		return errors.New("all AUTH_*_EXTERNAL ownership flags must be enabled together")
	}
	if oauthConfigured != len(oauthKeys) {
		return errors.New("OAuth client configuration is required when auth ownership is external")
	}
	if len(os.Getenv("AUTH_INTERNAL_TOKEN")) < 32 {
		return errors.New("AUTH_INTERNAL_TOKEN must contain at least 32 characters")
	}
	for _, key := range []string{"AUTH_BLOCK_SYNC_URL", "AUTH_LIFECYCLE_SYNC_URL", "AUTH_USER_SYNC_URL"} {
		if !validAuthSyncURL(os.Getenv(key)) {
			return fmt.Errorf("%s must use HTTPS or the private auth service", key)
		}
	}
	return nil
}

func validHTTPSURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

func validAuthSyncURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	return parsed.Scheme == "http" && parsed.Hostname() == "auth"
}
