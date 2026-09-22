package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	mw "github.com/asari194617/taskly/apps/server/internal/middleware"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func (h *Handler) Register2FARoutes(router fiber.Router, mw *mw.Middleware) {
	// 2FA: TOTP setup, activate, disable
	router.Post("/totp/setup", mw.AuthRequired, mw.Audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		// generate base32 secret
		secret, err := utils.GenerateBase32Secret()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "server"})
		}
		// store in Redis temporary
		key := fmt.Sprintf("totp-setup:%d", u.ID)
		if err := h.rdb.Set(c.Context(), key, secret, 10*time.Minute).Err(); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "server"})
		}
		issuer := "Taskly"
		label := fmt.Sprintf("%s:%s", issuer, u.Email)
		otpauth := fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s", utils.UrlEncode(label), secret, utils.UrlEncode(issuer))
		return c.JSON(fiber.Map{"secret": secret, "otpauth": otpauth})
	})
	router.Post("/totp/activate", mw.AuthRequired, mw.Audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var body struct {
			Code string `json:"code"`
		}
		if err := utils.ParseJSONStrict(c, &body); err != nil || strings.TrimSpace(body.Code) == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		key := fmt.Sprintf("totp-setup:%d", u.ID)
		secret, err := h.rdb.Get(c.Context(), key).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return c.Status(500).JSON(fiber.Map{"error": "server"})
		}
		if secret == "" {
			return c.Status(400).JSON(fiber.Map{"error": "setup expired"})
		}
		if !utils.ValidateTOTP(secret, body.Code) {
			return c.Status(401).JSON(fiber.Map{"error": "invalid code"})
		}
		enc, err := utils.EncryptString(secret)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "server"})
		}
		// upsert
		var ts domain.TOTPSecret
		if err := h.db.Where("user_id = ?", u.ID).First(&ts).Error; err == nil {
			ts.Secret = enc
			ts.IsActive = true
			if err := h.db.Save(&ts).Error; err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "db"})
			}
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := h.db.Create(&domain.TOTPSecret{UserID: u.ID, Secret: enc, IsActive: true}).Error; err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "db"})
			}
		} else {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		// Temporary setup cleanup is best effort: activation is already persisted.
		// Observe cleanup failures in Sentry when configured.
		if err := h.rdb.Del(c.Context(), key).Err(); err != nil {
			utils.CaptureError(err)
		}
		return c.SendStatus(204)
	})
	router.Post("/totp/disable", mw.AuthRequired, mw.Audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var ts domain.TOTPSecret
		if err := h.db.Where("user_id = ?", u.ID).First(&ts).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "not enabled"})
		} else if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		ts.IsActive = false
		if err := h.db.Save(&ts).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// Recovery codes
	router.Post("/recovery/generate", mw.AuthRequired, mw.Audit, func(c *fiber.Ctx) error {
		if utils.Env("AUTH_RECOVERY_EXTERNAL", "") == "1" {
			return c.Status(http.StatusConflict).JSON(fiber.Map{"error": "recovery codes managed by auth service"})
		}
		u := c.Locals("user").(*domain.User)
		codes := make([]string, 0, 10)
		if err := h.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("user_id = ? AND is_used = 0", u.ID).Delete(&domain.RecoveryCode{}).Error; err != nil {
				return err
			}
			for i := 0; i < 10; i++ {
				plain, err := utils.GenerateRecoveryCode()
				if err != nil {
					return err
				}
				hashed := utils.Sha256Hex([]byte(plain))
				if err := tx.Create(&domain.RecoveryCode{UserID: u.ID, HashedCode: hashed}).Error; err != nil {
					return err
				}
				codes = append(codes, plain)
			}
			return nil
		}); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "server"})
		}
		return c.JSON(fiber.Map{"codes": codes})
	})
}
