package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	mw "github.com/asari194617/taskly/apps/server/internal/middleware"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v2"
)

// convert Fiber request to http.Request for go-webauthn library
func fiberToHTTPRequest(c *fiber.Ctx) *http.Request {
	req, _ := http.NewRequestWithContext(c.Context(), "POST", "", io.NopCloser(strings.NewReader(string(c.Body()))))
	for k, v := range c.GetReqHeaders() {
		if len(v) > 0 {
			req.Header.Set(k, v[0])
		}
	}
	return req
}

func (h *Handler) RegisterWebAuthnRoutes(router fiber.Router, mw *mw.Middleware) {
	// Registration
	router.Post("/register/begin", mw.AuthRequired, mw.Audit, func(c *fiber.Ctx) error {
		if h.wAuth == nil {
			return c.Status(501).JSON(fiber.Map{"error": "webauthn not configured"})
		}
		u := c.Locals("user").(*domain.User)
		creation, sd, err := h.wAuth.BeginRegistration(utils.NewWebauthnUserWrapper(u, h.db))
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "begin failed"})
		}
		b, _ := json.Marshal(sd)
		_ = h.rdb.Set(c.Context(), fmt.Sprintf("webauthn:reg:%d", u.ID), string(b), 10*time.Minute).Err()
		return c.JSON(creation)
	})
	router.Post("/register/finish", mw.AuthRequired, mw.Audit, func(c *fiber.Ctx) error {
		if h.wAuth == nil {
			return c.Status(501).JSON(fiber.Map{"error": "webauthn not configured"})
		}
		u := c.Locals("user").(*domain.User)
		raw, _ := h.rdb.Get(c.Context(), fmt.Sprintf("webauthn:reg:%d", u.ID)).Result()
		if raw == "" {
			return c.Status(400).JSON(fiber.Map{"error": "session expired"})
		}
		var sd webauthn.SessionData
		_ = json.Unmarshal([]byte(raw), &sd)
		req := fiberToHTTPRequest(c)
		cred, err := h.wAuth.FinishRegistration(utils.NewWebauthnUserWrapper(u, h.db), sd, req)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "finish failed"})
		}
		// store credential
		pub := base64.RawStdEncoding.EncodeToString(cred.PublicKey)
		cid := base64.RawStdEncoding.EncodeToString(cred.ID)
		devName := "Passkey"
		rec := domain.Passkey{
			UserID:       u.ID,
			CredentialID: cid,
			PublicKey:    pub,
			SignCount:    int64(cred.Authenticator.SignCount),
			DeviceName:   &devName,
		}
		if err := h.db.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		_ = h.rdb.Del(c.Context(), fmt.Sprintf("webauthn:reg:%d", u.ID)).Err()
		return c.SendStatus(204)
	})

	// Login
	router.Post("/login/begin", func(c *fiber.Ctx) error {
		if h.wAuth == nil {
			return c.Status(501).JSON(fiber.Map{"error": "webauthn not configured"})
		}
		var body struct {
			Email string `json:"email"`
		}
		if err := c.BodyParser(&body); err != nil || strings.TrimSpace(body.Email) == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var u domain.User
		if err := h.db.Where("email = ?", body.Email).First(&u).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		assertion, sd, err := h.wAuth.BeginLogin(utils.NewWebauthnUserWrapper(&u, h.db))
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "begin failed"})
		}
		b, _ := json.Marshal(sd)
		_ = h.rdb.Set(c.Context(), fmt.Sprintf("webauthn:login:%d", u.ID), string(b), 10*time.Minute).Err()
		return c.JSON(assertion)
	})
	router.Post("/login/finish", func(c *fiber.Ctx) error {
		if h.wAuth == nil {
			return c.Status(501).JSON(fiber.Map{"error": "webauthn not configured"})
		}
		var body struct {
			Email string `json:"email"`
		}
		_ = c.BodyParser(&body)
		if strings.TrimSpace(body.Email) == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var u domain.User
		if err := h.db.Where("email = ?", body.Email).First(&u).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		raw, _ := h.rdb.Get(c.Context(), fmt.Sprintf("webauthn:login:%d", u.ID)).Result()
		if raw == "" {
			return c.Status(400).JSON(fiber.Map{"error": "session expired"})
		}
		var sd webauthn.SessionData
		_ = json.Unmarshal([]byte(raw), &sd)
		req := fiberToHTTPRequest(c)
		cred, err := h.wAuth.FinishLogin(utils.NewWebauthnUserWrapper(&u, h.db), sd, req)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "invalid passkey"})
		}
		// Blocklist enforcement before issuing session
		var blkCnt int64
		_ = h.db.Model(&domain.Block{}).
			Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", u.ID, time.Now()).
			Count(&blkCnt).Error
		if blkCnt > 0 {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "blocked"})
		}
		_ = h.db.Model(&domain.Passkey{}).
			Where("user_id = ? AND credential_id = ?", u.ID, base64.RawStdEncoding.EncodeToString(cred.ID)).
			Update("sign_count", cred.Authenticator.SignCount).Error
		// Create session
		plat := strings.ToLower(strings.TrimSpace(c.Get("X-Client-Platform")))
		if plat == "" {
			plat = "web"
		}
		if !utils.IsAllowedPlatform(plat) {
			return c.Status(400).JSON(fiber.Map{"error": "invalid platform"})
		}
		token := utils.GenerateSessionToken(u.ID, plat)
		ttl := 30 * 24 * time.Hour
		if err := h.rdb.Set(c.Context(), "session:"+token, fmt.Sprintf("%d", u.ID), ttl).Err(); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "session"})
		}
		_ = h.rdb.SAdd(c.Context(), fmt.Sprintf("user_sessions:%d", u.ID), token).Err()
		_ = h.rdb.Expire(c.Context(), fmt.Sprintf("user_sessions:%d", u.ID), ttl).Err()
		h.db.AutoMigrate(&domain.Session{})
		h.db.Create(&domain.Session{UserID: u.ID, ClientPlatform: plat, HashedToken: utils.Sha256Hex([]byte(token)), ExpiresAt: utils.PtrTime(time.Now().Add(ttl))})
		return c.JSON(fiber.Map{"token": token})
	})
}
