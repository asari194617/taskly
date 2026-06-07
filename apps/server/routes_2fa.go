package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
)

func register2FARoutes(app *fiber.App) {
	// 2FA: TOTP setup, activate, disable
	app.Post("/2fa/totp/setup", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		// generate base32 secret
		secret := utils.GenerateBase32Secret()
		// store in Redis temporary
		key := fmt.Sprintf("totp-setup:%d", u.ID)
		if err := RDB.Set(c.Context(), key, secret, 10*time.Minute).Err(); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "server"})
		}
		issuer := "Taskly"
		label := fmt.Sprintf("%s:%s", issuer, u.Email)
		otpauth := fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s", utils.UrlEncode(label), secret, utils.UrlEncode(issuer))
		return c.JSON(fiber.Map{"secret": secret, "otpauth": otpauth})
	})
	app.Post("/2fa/totp/activate", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var body struct {
			Code string `json:"code"`
		}
		if err := utils.ParseJSONStrict(c, &body); err != nil || strings.TrimSpace(body.Code) == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		key := fmt.Sprintf("totp-setup:%d", u.ID)
		secret, _ := RDB.Get(c.Context(), key).Result()
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
		if err := DB.Where("user_id = ?", u.ID).First(&ts).Error; err == nil {
			ts.Secret = enc
			ts.IsActive = true
			_ = DB.Save(&ts).Error
		} else {
			_ = DB.Create(&domain.TOTPSecret{UserID: u.ID, Secret: enc, IsActive: true}).Error
		}
		_ = RDB.Del(c.Context(), key).Err()
		return c.SendStatus(204)
	})
	app.Post("/2fa/totp/disable", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var ts domain.TOTPSecret
		if err := DB.Where("user_id = ?", u.ID).First(&ts).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not enabled"})
		}
		ts.IsActive = false
		if err := DB.Save(&ts).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// Recovery codes
	app.Post("/2fa/recovery/generate", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		// delete existing unused codes
		_ = DB.Where("user_id = ? AND is_used = 0", u.ID).Delete(&domain.RecoveryCode{}).Error
		// generate 10 codes
		codes := make([]string, 0, 10)
		for i := 0; i < 10; i++ {
			plain := utils.GenerateRecoveryCode()
			hashed := utils.Sha256Hex([]byte(plain))
			_ = DB.Create(&domain.RecoveryCode{UserID: u.ID, HashedCode: hashed}).Error
			codes = append(codes, plain)
		}
		return c.JSON(fiber.Map{"codes": codes})
	})
	app.Post("/2fa/recovery/consume", func(c *fiber.Ctx) error {
		var body struct {
			Email string `json:"email"`
			Code  string `json:"code"`
		}
		if err := utils.ParseJSONStrict(c, &body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid json"})
		}
		if !utils.IsEmail(body.Email) || strings.TrimSpace(body.Code) == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var u domain.User
		if err := DB.Where("email = ?", body.Email).First(&u).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		if utils.ConsumeRecoveryCode(u.ID, body.Code, DB) {
			return c.SendStatus(204)
		}
		return c.Status(401).JSON(fiber.Map{"error": "invalid code"})
	})
}
