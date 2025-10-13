package main

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

func registerPushRoutes(app *fiber.App) {
	// Web Push subscription endpoints (no push send yet)
	app.Post("/push/subscribe", authRequired, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var body struct{ Endpoint, P256dh, Auth string }
		if err := c.BodyParser(&body); err != nil || body.Endpoint == "" || body.P256dh == "" || body.Auth == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var rec WebPushSubscription
		if err := DB.Where("user_id = ? AND endpoint = ?", u.ID, body.Endpoint).First(&rec).Error; err == nil {
			rec.P256dh, rec.Auth = body.P256dh, body.Auth
			_ = DB.Save(&rec).Error
		} else {
			_ = DB.Create(&WebPushSubscription{UserID: u.ID, Endpoint: body.Endpoint, P256dh: body.P256dh, Auth: body.Auth}).Error
		}
		return c.SendStatus(204)
	})
	app.Post("/push/unsubscribe", authRequired, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var body struct{ Endpoint string }
		_ = c.BodyParser(&body)
		if strings.TrimSpace(body.Endpoint) != "" {
			_ = DB.Where("user_id = ? AND endpoint = ?", u.ID, body.Endpoint).Delete(&WebPushSubscription{}).Error
		}
		return c.SendStatus(204)
	})
}
