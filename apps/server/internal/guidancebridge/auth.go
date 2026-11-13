package guidancebridge

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/legal"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type introspection struct {
	Active    bool   `json:"active"`
	ClientID  string `json:"client_id"`
	Subject   string `json:"sub"`
	Scope     string `json:"scope"`
	Audience  string `json:"aud"`
	Issuer    string `json:"iss"`
	Expires   int64  `json:"exp"`
	TokenType string `json:"token_type"`
}

var client = &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func (b *bridge) authenticate(c *fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	if b.configErr != nil {
		return fail(c, 503, "guidance integration unavailable")
	}
	if c.Get("X-Guidance-Event-ID") != b.cfg.eventID {
		return fail(c, 409, "guidance event mismatch")
	}
	auth := strings.Fields(c.Get("Authorization"))
	if len(auth) != 2 || !strings.EqualFold(auth[0], "Bearer") || len(auth[1]) > 4096 || c.Get("Cookie") != "" || c.Get("X-API-Key") != "" || c.Get("X-MCP-Token") != "" {
		return fail(c, 401, "unauthorized")
	}
	form := url.Values{"token": {auth[1]}, "token_type_hint": {"access_token"}}
	req, err := http.NewRequestWithContext(c.UserContext(), http.MethodPost, b.cfg.issuer+"/oauth/introspect", strings.NewReader(form.Encode()))
	if err != nil {
		return fail(c, 503, "identity service unavailable")
	}
	req.SetBasicAuth(b.cfg.clientID, b.cfg.clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := client.Do(req)
	if err != nil {
		return fail(c, 503, "identity service unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fail(c, 503, "identity service unavailable")
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 16385))
	var identity introspection
	if err != nil || len(body) > 16384 || json.Unmarshal(body, &identity) != nil {
		return fail(c, 503, "identity service unavailable")
	}
	uid, err := strconv.ParseUint(identity.Subject, 10, strconv.IntSize)
	hasScope := false
	for _, scope := range strings.Fields(identity.Scope) {
		if scope == "taskly:login" {
			hasScope = true
		}
	}
	if err != nil || uid == 0 || !identity.Active || identity.ClientID != guidanceApplication || identity.Audience != "taskly-api" || identity.Issuer != b.cfg.issuer || identity.TokenType != "Bearer" || identity.Expires <= time.Now().Unix() || !hasScope {
		return fail(c, 401, "unauthorized")
	}
	var user domain.User
	if err := b.db.Where("id = ? AND is_deleted = ?", uid, false).First(&user).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return fail(c, 401, "unauthorized")
	} else if err != nil {
		return fail(c, 503, "account status unavailable")
	}
	var blocks int64
	if err := b.db.Model(&domain.Block{}).Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", user.ID, time.Now()).Count(&blocks).Error; err != nil {
		return fail(c, 503, "account status unavailable")
	}
	if blocks > 0 {
		return fail(c, 403, "account blocked")
	}
	current, err := legal.Current(b.db, user.ID)
	if err != nil {
		return fail(c, 503, "consent status unavailable")
	}
	if !current {
		return fail(c, 403, "policy consent required")
	}
	c.Locals("guidance-user", user.ID)
	return c.Next()
}
