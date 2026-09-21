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
	"github.com/go-webauthn/webauthn/protocol"
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
		devName := "Passkey"
		rec, err := utils.NewPasskeyRecord(u.ID, devName, cred)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "credential storage"})
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
		parsedResponse, err := protocol.ParseCredentialRequestResponse(req)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "invalid passkey"})
		}
		// Legacy rows did not persist credential flags. The value is safe to use
		// as a migration hint because ValidateLogin verifies the authenticator
		// data signature with the stored public key before authentication succeeds.
		backupEligible := parsedResponse.Response.AuthenticatorData.Flags.HasBackupEligible()
		cred, err := h.wAuth.ValidateLogin(utils.NewWebauthnUserWrapperForAssertion(&u, h.db, backupEligible), sd, parsedResponse)
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
		storedCredential, err := utils.EncodeWebauthnCredential(cred)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "credential storage"})
		}
		if err := h.db.Model(&domain.Passkey{}).
			Where("user_id = ? AND credential_id = ?", u.ID, base64.RawStdEncoding.EncodeToString(cred.ID)).
			Updates(map[string]any{
				"public_key": storedCredential,
				"sign_count": cred.Authenticator.SignCount,
			}).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "credential storage"})
		}
		// Create session
		plat := strings.ToLower(strings.TrimSpace(c.Get("X-Client-Platform")))
		if plat == "" {
			plat = "web"
		}
		if !utils.IsAllowedPlatform(plat) {
			return c.Status(400).JSON(fiber.Map{"error": "invalid platform"})
		}
		if err := validateLoginClient(c, plat); err != nil {
			return err
		}
		return h.issueSession(c, u.ID, plat, time.Now())
	})
}
