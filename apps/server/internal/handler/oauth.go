package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

const oauthStateCookie = "taskly_oauth_state"

type oauthLoginState struct {
	Verifier string `json:"verifier"`
	Next     string `json:"next"`
}

type oauthClientConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURI  string
	WebURL       string
}

func oauthConfig() (oauthClientConfig, bool) {
	cfg := oauthClientConfig{
		Issuer:       strings.TrimRight(utils.Env("AUTH_ISSUER", ""), "/"),
		ClientID:     utils.Env("AUTH_CLIENT_ID", ""),
		ClientSecret: utils.Env("AUTH_CLIENT_SECRET", ""),
		RedirectURI:  utils.Env("AUTH_REDIRECT_URI", ""),
		WebURL:       strings.TrimRight(utils.Env("WEB_PUBLIC_URL", "http://localhost:4321"), "/"),
	}
	if cfg.Issuer == "" || cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RedirectURI == "" {
		return cfg, false
	}
	if strings.EqualFold(utils.Env("NODE_ENV", "development"), "production") {
		if !strings.HasPrefix(cfg.Issuer, "https://") || !strings.HasPrefix(cfg.RedirectURI, "https://") || !strings.HasPrefix(cfg.WebURL, "https://") {
			return cfg, false
		}
	}
	return cfg, true
}

func oauthRandom() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (h *Handler) RegisterOAuthRoutes(router fiber.Router) {
	router.Get("/start", h.oauthStart)
	router.Get("/callback", h.oauthCallback)
}

func (h *Handler) oauthStart(c *fiber.Ctx) error {
	cfg, ok := oauthConfig()
	if !ok {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "oauth not configured"})
	}
	state, err := oauthRandom()
	if err != nil {
		return c.SendStatus(500)
	}
	verifier, err := oauthRandom()
	if err != nil {
		return c.SendStatus(500)
	}
	next := safeOAuthNext(c.Query("next"))
	encoded, err := json.Marshal(oauthLoginState{Verifier: verifier, Next: next})
	if err != nil {
		return c.SendStatus(500)
	}
	if err := h.rdb.Set(c.Context(), "oauth_login:"+state, encoded, 5*time.Minute).Err(); err != nil {
		return c.SendStatus(503)
	}
	c.Cookie(&fiber.Cookie{Name: oauthStateCookie, Value: state, Path: "/auth/oauth/callback", MaxAge: 300, HTTPOnly: true, Secure: utils.SessionCookieSecure(), SameSite: fiber.CookieSameSiteLaxMode})
	challenge := sha256.Sum256([]byte(verifier))
	query := url.Values{}
	query.Set("response_type", "code")
	query.Set("client_id", cfg.ClientID)
	query.Set("redirect_uri", cfg.RedirectURI)
	query.Set("scope", "taskly:login")
	query.Set("state", state)
	query.Set("code_challenge", base64.RawURLEncoding.EncodeToString(challenge[:]))
	query.Set("code_challenge_method", "S256")
	return c.Redirect(cfg.Issuer+"/oauth/authorize?"+query.Encode(), fiber.StatusFound)
}

func (h *Handler) oauthCallback(c *fiber.Ctx) error {
	cfg, ok := oauthConfig()
	if !ok {
		return c.SendStatus(503)
	}
	state := c.Query("state")
	cookieState := c.Cookies(oauthStateCookie)
	c.Cookie(&fiber.Cookie{Name: oauthStateCookie, Value: "", Path: "/auth/oauth/callback", MaxAge: -1, HTTPOnly: true, Secure: utils.SessionCookieSecure(), SameSite: fiber.CookieSameSiteLaxMode})
	if state == "" || cookieState == "" || subtle.ConstantTimeCompare([]byte(state), []byte(cookieState)) != 1 || c.Query("iss") != cfg.Issuer {
		return c.SendStatus(400)
	}
	raw, err := h.rdb.GetDel(c.Context(), "oauth_login:"+state).Bytes()
	if errors.Is(err, redis.Nil) {
		return c.SendStatus(400)
	}
	if err != nil {
		return c.SendStatus(503)
	}
	var pending oauthLoginState
	if json.Unmarshal(raw, &pending) != nil || pending.Verifier == "" {
		return c.SendStatus(400)
	}
	if c.Query("error") != "" {
		return c.Redirect(cfg.WebURL+"/login", fiber.StatusFound)
	}
	code := c.Query("code")
	if code == "" {
		return c.SendStatus(400)
	}
	token, err := oauthExchange(cfg, code, pending.Verifier)
	if err != nil {
		return c.SendStatus(502)
	}
	userID, email, err := oauthUserID(cfg, token)
	if err != nil {
		return c.SendStatus(502)
	}
	var user domain.User
	if err := h.db.Where("id = ? AND is_deleted = 0", userID).First(&user).Error; err != nil {
		return c.SendStatus(403)
	}
	if !strings.EqualFold(user.Email, email) {
		return c.SendStatus(403)
	}
	if err := h.issueSession(c, user.ID, "web", time.Now()); err != nil {
		return err
	}
	next := safeOAuthNext(pending.Next)
	return c.Redirect(cfg.WebURL+next, fiber.StatusFound)
}

func safeOAuthNext(raw string) string {
	next := strings.TrimSpace(raw)
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.ContainsAny(next, "\\\r\n") {
		return "/home"
	}
	u, err := url.Parse(next)
	if err != nil || u.IsAbs() || u.Host != "" {
		return "/home"
	}
	return next
}

var oauthHTTPClient = &http.Client{Timeout: 5 * time.Second}

func oauthExchange(cfg oauthClientConfig, code, verifier string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", cfg.RedirectURI)
	form.Set("code_verifier", verifier)
	req, err := http.NewRequest(http.MethodPost, cfg.Issuer+"/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(cfg.ClientID, cfg.ClientSecret)
	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", errors.New("token exchange failed")
	}
	var payload struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<10)).Decode(&payload); err != nil {
		return "", err
	}
	if payload.AccessToken == "" || !strings.EqualFold(payload.TokenType, "Bearer") {
		return "", errors.New("invalid token response")
	}
	return payload.AccessToken, nil
}

func oauthUserID(cfg oauthClientConfig, token string) (uint64, string, error) {
	req, err := http.NewRequest(http.MethodGet, cfg.Issuer+"/oauth/userinfo", nil)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return 0, "", errors.New("userinfo failed")
	}
	var payload struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<10)).Decode(&payload); err != nil {
		return 0, "", err
	}
	id, err := strconv.ParseUint(payload.Sub, 10, 64)
	if err != nil || id == 0 || payload.Email == "" {
		return 0, "", errors.New("invalid subject")
	}
	return id, payload.Email, nil
}
