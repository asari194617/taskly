package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	utils "github.com/asari194617/taskly/apps/server/utils"
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

func registerWebAuthnRoutes(app *fiber.App) {
	// Registration
	app.Post("/webauthn/register/begin", authRequired, audit, func(c *fiber.Ctx) error {
		if WAuth == nil {
			return c.Status(501).JSON(fiber.Map{"error": "webauthn not configured"})
		}
		u := c.Locals("user").(*utils.User)
		creation, sd, err := WAuth.BeginRegistration(&webauthnUserWrapper{User: u, db: DB})
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "begin failed"})
		}
		b, _ := json.Marshal(sd)
		_ = RDB.Set(c.Context(), fmt.Sprintf("webauthn:reg:%d", u.ID), string(b), 10*time.Minute).Err()
		return c.JSON(creation)
	})
	app.Post("/webauthn/register/finish", authRequired, audit, func(c *fiber.Ctx) error {
		if WAuth == nil {
			return c.Status(501).JSON(fiber.Map{"error": "webauthn not configured"})
		}
		u := c.Locals("user").(*utils.User)
		raw, _ := RDB.Get(c.Context(), fmt.Sprintf("webauthn:reg:%d", u.ID)).Result()
		if raw == "" {
			return c.Status(400).JSON(fiber.Map{"error": "session expired"})
		}
		var sd webauthn.SessionData
		_ = json.Unmarshal([]byte(raw), &sd)
		req := fiberToHTTPRequest(c)
		cred, err := WAuth.FinishRegistration(&webauthnUserWrapper{User: u, db: DB}, sd, req)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "finish failed"})
		}
		// store credential
		pub := base64.RawStdEncoding.EncodeToString(cred.PublicKey)
		cid := base64.RawStdEncoding.EncodeToString(cred.ID)
		devName := "Passkey"
		rec := utils.Passkey{
			UserID:       u.ID,
			CredentialID: cid,
			PublicKey:    pub,
			SignCount:    int64(cred.Authenticator.SignCount),
			DeviceName:   &devName,
		}
		if err := DB.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		_ = RDB.Del(c.Context(), fmt.Sprintf("webauthn:reg:%d", u.ID)).Err()
		return c.SendStatus(204)
	})

	// Login
	app.Post("/webauthn/login/begin", func(c *fiber.Ctx) error {
		if WAuth == nil {
			return c.Status(501).JSON(fiber.Map{"error": "webauthn not configured"})
		}
		var body struct {
			Email string `json:"email"`
		}
		if err := c.BodyParser(&body); err != nil || strings.TrimSpace(body.Email) == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var u utils.User
		if err := DB.Where("email = ?", body.Email).First(&u).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		assertion, sd, err := WAuth.BeginLogin(&webauthnUserWrapper{User: &u, db: DB})
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "begin failed"})
		}
		b, _ := json.Marshal(sd)
		_ = RDB.Set(c.Context(), fmt.Sprintf("webauthn:login:%d", u.ID), string(b), 10*time.Minute).Err()
		return c.JSON(assertion)
	})
	app.Post("/webauthn/login/finish", func(c *fiber.Ctx) error {
		if WAuth == nil {
			return c.Status(501).JSON(fiber.Map{"error": "webauthn not configured"})
		}
		var body struct {
			Email string `json:"email"`
		}
		_ = c.BodyParser(&body)
		if strings.TrimSpace(body.Email) == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var u utils.User
		if err := DB.Where("email = ?", body.Email).First(&u).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		raw, _ := RDB.Get(c.Context(), fmt.Sprintf("webauthn:login:%d", u.ID)).Result()
		if raw == "" {
			return c.Status(400).JSON(fiber.Map{"error": "session expired"})
		}
		var sd webauthn.SessionData
		_ = json.Unmarshal([]byte(raw), &sd)
		req := fiberToHTTPRequest(c)
		cred, err := WAuth.FinishLogin(&webauthnUserWrapper{User: &u, db: DB}, sd, req)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "invalid passkey"})
		}
		// Blocklist enforcement before issuing session
		var blkCnt int64
		_ = DB.Model(&utils.Block{}).
			Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", u.ID, time.Now()).
			Count(&blkCnt).Error
		if blkCnt > 0 {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "blocked"})
		}
		_ = DB.Model(&utils.Passkey{}).
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
		if err := RDB.Set(c.Context(), "session:"+token, fmt.Sprintf("%d", u.ID), ttl).Err(); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "session"})
		}
		_ = RDB.SAdd(c.Context(), fmt.Sprintf("user_sessions:%d", u.ID), token).Err()
		_ = RDB.Expire(c.Context(), fmt.Sprintf("user_sessions:%d", u.ID), ttl).Err()
		DB.AutoMigrate(&utils.Session{})
		DB.Create(&utils.Session{UserID: u.ID, ClientPlatform: plat, HashedToken: utils.Sha256Hex([]byte(token)), ExpiresAt: utils.PtrTime(time.Now().Add(ttl))})
		return c.JSON(fiber.Map{"token": token})
	})
}
