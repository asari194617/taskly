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

var blockHTTPClient = &http.Client{
	Timeout: 5 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

type blockRequest struct {
	Active    bool    `json:"active"`
	Reason    *string `json:"reason,omitempty"`
	ExpiresAt *string `json:"expires_at,omitempty"`
}

func BlocksEnabled() bool {
	return os.Getenv("AUTH_BLOCKS_EXTERNAL") == "1"
}

func SyncBlock(ctx context.Context, userID uint, active bool, reason *string, expiresAt *time.Time) error {
	if !BlocksEnabled() {
		return nil
	}
	base, token, err := blockConfig()
	if err != nil {
		return err
	}
	payload := blockRequest{Active: active}
	if active {
		payload.Reason = reason
		if expiresAt != nil {
			value := expiresAt.UTC().Format(time.RFC3339)
			payload.ExpiresAt = &value
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return errors.New("encode block state")
	}
	endpoint := strings.TrimRight(base.String(), "/") + "/internal/users/" + strconv.FormatUint(uint64(userID), 10) + "/block"
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return errors.New("create block synchronization request")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := blockHTTPClient.Do(req)
	if err != nil {
		return errors.New("send block synchronization request")
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 16<<10))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("block synchronization status %d", resp.StatusCode)
	}
	return nil
}

func blockConfig() (*url.URL, string, error) {
	rawURL := strings.TrimSpace(os.Getenv("AUTH_BLOCK_SYNC_URL"))
	token := os.Getenv("AUTH_INTERNAL_TOKEN")
	if rawURL == "" || token == "" {
		return nil, "", errors.New("block synchronization is not configured")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, "", errors.New("block synchronization URL is invalid")
	}
	if strings.EqualFold(os.Getenv("NODE_ENV"), "production") {
		host := parsed.Hostname()
		privateHTTP := parsed.Scheme == "http" && (host == "auth" || host == "localhost" || host == "127.0.0.1")
		if parsed.Scheme != "https" && !privateHTTP {
			return nil, "", errors.New("block synchronization URL must use HTTPS or the private auth service")
		}
		if len(token) < 32 {
			return nil, "", errors.New("block synchronization token is too short")
		}
	}
	return parsed, token, nil
}
