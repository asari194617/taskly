package main

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func registerSettingsRoutes(app *fiber.App) {
	// Auth settings endpoints
	app.Get("/settings/auth", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		// refresh user
		var fresh User
		_ = DB.First(&fresh, u.ID).Error
		hasPassword := fresh.HashedPassword != nil && *fresh.HashedPassword != ""
		totpEnabled, _, _ := getActiveTOTP(fresh.ID)
		return c.JSON(fiber.Map{
			"email_login_enabled": fresh.EmailCodeAllowed,
			"has_password":        hasPassword,
			"totp_enabled":        totpEnabled,
		})
	})
	app.Post("/settings/auth", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var body struct {
			EmailLoginEnabled *bool `json:"email_login_enabled"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if body.EmailLoginEnabled != nil {
			_ = DB.Model(&User{}).Where("id = ?", u.ID).Update("email_code_allowed", *body.EmailLoginEnabled).Error
		}
		return c.SendStatus(204)
	})

	// iCal token management under settings
	app.Get("/ical/tokens", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var items []IcalKey
		if err := DB.Where("user_id = ?", u.ID).Order("id DESC").Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		base := strings.TrimRight(env("ICAL_BASE_URL", env("API_PUBLIC_URL", "http://localhost:8080")+"/ical"), "/")
		out := make([]map[string]any, 0, len(items))
		for _, it := range items {
			out = append(out, map[string]any{
				"id":          it.ID,
				"description": it.Description,
				"url":         fmt.Sprintf("%s/%s.ics", base, it.Token),
				"created_at":  it.CreatedAt,
			})
		}
		return c.JSON(out)
	})
	app.Post("/ical/tokens", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var body struct {
			Description *string `json:"description"`
		}
		_ = c.BodyParser(&body)
		tok := generateIcalToken()
		rec := IcalKey{UserID: u.ID, Token: tok, Description: body.Description}
		if err := DB.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		base := strings.TrimRight(env("ICAL_BASE_URL", env("API_PUBLIC_URL", "http://localhost:8080")+"/ical"), "/")
		return c.Status(201).JSON(fiber.Map{"id": rec.ID, "url": fmt.Sprintf("%s/%s.ics", base, rec.Token)})
	})
	app.Delete("/ical/tokens/:id", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		if err := DB.Where("id = ? AND user_id = ?", c.Params("id"), u.ID).Delete(&IcalKey{}).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// Push settings (per-user)
	app.Get("/settings/push", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var fresh User
		_ = DB.First(&fresh, u.ID).Error
		return c.JSON(fiber.Map{
			"daily_enabled": fresh.PushDailyEnabled,
		})
	})
	app.Post("/settings/push", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var body struct {
			DailyEnabled *bool `json:"daily_enabled"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if body.DailyEnabled != nil {
			_ = DB.Model(&User{}).Where("id = ?", u.ID).Update("push_daily_enabled", *body.DailyEnabled).Error
		}
		return c.SendStatus(204)
	})
}
