package authsync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	ErrLifecycleSync       = errors.New("auth lifecycle synchronization")
	ErrLifecycleNotDeleted = fmt.Errorf("%w: user not deleted", ErrLifecycleSync)
)

var lifecycleHTTPClient = &http.Client{
	Timeout: 5 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

type lifecycleRequest struct {
	Deleted   bool   `json:"deleted"`
	DeletedAt string `json:"deleted_at"`
}

func LifecycleEnabled() bool {
	return os.Getenv("AUTH_ACCOUNT_LIFECYCLE_EXTERNAL") == "1"
}

func SyncDeleted(ctx context.Context, userID uint, deletedAt time.Time) error {
	if !LifecycleEnabled() {
		return nil
	}
	base, token, err := lifecycleConfig()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrLifecycleSync, err)
	}
	payload, err := json.Marshal(lifecycleRequest{Deleted: true, DeletedAt: deletedAt.UTC().Format(time.RFC3339)})
	if err != nil {
		return fmt.Errorf("%w: encode state", ErrLifecycleSync)
	}
	endpoint := lifecycleUserURL(base, userID) + "/lifecycle"
	return sendLifecycleRequest(ctx, http.MethodPut, endpoint, token, bytes.NewReader(payload), http.StatusOK)
}

// SyncRestored reopens the account without reactivating revoked credentials.
func SyncRestored(ctx context.Context, userID uint) error {
	if !LifecycleEnabled() {
		return nil
	}
	base, token, err := lifecycleConfig()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrLifecycleSync, err)
	}
	return sendLifecycleRequest(ctx, http.MethodPut, lifecycleUserURL(base, userID)+"/lifecycle", token, strings.NewReader(`{"deleted":false}`), http.StatusOK)
}

func Erase(ctx context.Context, userID uint) error {
	if !LifecycleEnabled() {
		return nil
	}
	base, token, err := lifecycleConfig()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrLifecycleSync, err)
	}
	return sendLifecycleRequest(ctx, http.MethodDelete, lifecycleUserURL(base, userID), token, nil, http.StatusNoContent)
}

func sendLifecycleRequest(ctx context.Context, method, endpoint, token string, body io.Reader, expectedStatus int) error {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return fmt.Errorf("%w: create request", ErrLifecycleSync)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := lifecycleHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: send request", ErrLifecycleSync)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 16<<10))
	if resp.StatusCode != expectedStatus {
		if method == http.MethodDelete && resp.StatusCode == http.StatusConflict {
			return ErrLifecycleNotDeleted
		}
		return fmt.Errorf("%w: status %d", ErrLifecycleSync, resp.StatusCode)
	}
	return nil
}

func lifecycleUserURL(base *url.URL, userID uint) string {
	return strings.TrimRight(base.String(), "/") + "/internal/users/" + strconv.FormatUint(uint64(userID), 10)
}

func lifecycleConfig() (*url.URL, string, error) {
	rawURL := strings.TrimSpace(os.Getenv("AUTH_LIFECYCLE_SYNC_URL"))
	token := os.Getenv("AUTH_INTERNAL_TOKEN")
	if rawURL == "" || token == "" {
		return nil, "", errors.New("lifecycle synchronization is not configured")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, "", errors.New("lifecycle synchronization URL is invalid")
	}
	if strings.EqualFold(os.Getenv("NODE_ENV"), "production") {
		host := parsed.Hostname()
		privateHTTP := parsed.Scheme == "http" && (host == "auth" || host == "localhost" || host == "127.0.0.1")
		if parsed.Scheme != "https" && !privateHTTP {
			return nil, "", errors.New("lifecycle synchronization URL must use HTTPS or the private auth service")
		}
		if len(token) < 32 {
			return nil, "", errors.New("lifecycle synchronization token is too short")
		}
	}
	return parsed, token, nil
}
