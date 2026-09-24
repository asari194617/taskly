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

var ErrUserSync = errors.New("auth user synchronization")

var userHTTPClient = &http.Client{
	Timeout:       5 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

type UserProvision struct {
	Email            string `json:"email"`
	DisplayName      string `json:"display_name"`
	NotificationLang string `json:"notification_lang"`
}

type UserProvisionResult struct {
	ID      uint `json:"id"`
	Created bool `json:"created"`
}

type UserUpdate struct {
	Email            *string `json:"email,omitempty"`
	DisplayName      *string `json:"display_name,omitempty"`
	NotificationLang *string `json:"notification_lang,omitempty"`
	EmailCodeAllowed *bool   `json:"email_code_allowed,omitempty"`
}

func UsersEnabled() bool { return os.Getenv("AUTH_USERS_EXTERNAL") == "1" }

func ProvisionUser(ctx context.Context, input UserProvision) (UserProvisionResult, error) {
	if !UsersEnabled() {
		return UserProvisionResult{}, errors.New("auth user synchronization is disabled")
	}
	body, err := json.Marshal(input)
	if err != nil {
		return UserProvisionResult{}, fmt.Errorf("%w: encode request", ErrUserSync)
	}
	base, token, err := userConfig()
	if err != nil {
		return UserProvisionResult{}, fmt.Errorf("%w: %v", ErrUserSync, err)
	}
	response, status, err := sendUserRequest(ctx, http.MethodPost, strings.TrimRight(base.String(), "/")+"/internal/users", token, body)
	if err != nil {
		return UserProvisionResult{}, err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return UserProvisionResult{}, fmt.Errorf("%w: status %d", ErrUserSync, status)
	}
	var result UserProvisionResult
	if err := json.Unmarshal(response, &result); err != nil || result.ID == 0 {
		return UserProvisionResult{}, fmt.Errorf("%w: invalid response", ErrUserSync)
	}
	return result, nil
}

func UpdateUser(ctx context.Context, userID uint, input UserUpdate) error {
	if !UsersEnabled() {
		return nil
	}
	body, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("%w: encode request", ErrUserSync)
	}
	base, token, err := userConfig()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUserSync, err)
	}
	endpoint := strings.TrimRight(base.String(), "/") + "/internal/users/" + strconv.FormatUint(uint64(userID), 10)
	_, status, err := sendUserRequest(ctx, http.MethodPatch, endpoint, token, body)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("%w: status %d", ErrUserSync, status)
	}
	return nil
}

func sendUserRequest(ctx context.Context, method, endpoint, token string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("%w: create request", ErrUserSync)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := userHTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: send request", ErrUserSync)
	}
	defer resp.Body.Close()
	response, err := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
	if err != nil {
		return nil, 0, fmt.Errorf("%w: read response", ErrUserSync)
	}
	return response, resp.StatusCode, nil
}

func userConfig() (*url.URL, string, error) {
	rawURL := strings.TrimSpace(os.Getenv("AUTH_USER_SYNC_URL"))
	token := os.Getenv("AUTH_INTERNAL_TOKEN")
	if rawURL == "" || token == "" {
		return nil, "", errors.New("user synchronization is not configured")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, "", errors.New("user synchronization URL is invalid")
	}
	if strings.EqualFold(os.Getenv("NODE_ENV"), "production") {
		host := parsed.Hostname()
		privateHTTP := parsed.Scheme == "http" && (host == "auth" || host == "localhost" || host == "127.0.0.1")
		if parsed.Scheme != "https" && !privateHTTP {
			return nil, "", errors.New("user synchronization URL must use HTTPS or the private auth service")
		}
		if len(token) < 32 {
			return nil, "", errors.New("user synchronization token is too short")
		}
	}
	return parsed, token, nil
}
