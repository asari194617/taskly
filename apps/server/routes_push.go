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
		if err := parseJSONStrict(c, &body); err != nil || strings.TrimSpace(body.Endpoint) == "" || strings.TrimSpace(body.P256dh) == "" || strings.TrimSpace(body.Auth) == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var rec WebPushSubscription
		if err := DB.Where("user_id = ? AND endpoint = ?", u.ID, body.Endpoint).First(&rec).Error; err == nil {
			rec.P256dh, rec.Auth = body.P256dh, body.Auth
			_ = DB.Save(&rec).Error
		} else {
			_ = DB.Create(&WebPushSubscription{UserID: u.ID, Endpoint: body.Endpoint, P256dh: body.P256dh, Auth: body.Auth}).Error
		}
		// Ensure a NotificationDevice row per subscription (Platform=WEBPUSH)
		ua := strings.ToLower(strings.TrimSpace(c.Get("User-Agent")))
		name := detectDeviceNameFromUA(ua)
		payload := body.Endpoint
		var dev NotificationDevice
		if err := DB.Where("user_id = ? AND platform = 'WEBPUSH' AND payload = ?", u.ID, payload).First(&dev).Error; err == nil {
			_ = DB.Model(&NotificationDevice{}).Where("id = ?", dev.ID).Updates(map[string]any{"is_enabled": true, "name": &name}).Error
		} else {
			nm := name
			pl := payload
			_ = DB.Create(&NotificationDevice{UserID: u.ID, Name: &nm, Platform: "WEBPUSH", Payload: &pl, IsEnabled: true}).Error
		}
		return c.SendStatus(204)
	})
	app.Post("/push/unsubscribe", authRequired, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var body struct{ Endpoint string }
		_ = parseJSONStrict(c, &body)
		if strings.TrimSpace(body.Endpoint) != "" {
			_ = DB.Where("user_id = ? AND endpoint = ?", u.ID, body.Endpoint).Delete(&WebPushSubscription{}).Error
			// Soft-disable corresponding device if exists
			_ = DB.Model(&NotificationDevice{}).Where("user_id = ? AND platform = 'WEBPUSH' AND payload = ?", u.ID, body.Endpoint).Update("is_enabled", false).Error
		}
		return c.SendStatus(204)
	})
}

// detectDeviceNameFromUA returns a friendly device name from user-agent.
func detectDeviceNameFromUA(ua string) string {
	if ua == "" {
		return "Web Push"
	}
	if strings.Contains(ua, "iphone") {
		return "iPhone"
	}
	if strings.Contains(ua, "ipad") {
		return "iPad"
	}
	if strings.Contains(ua, "mac os") || strings.Contains(ua, "macintosh") {
		return "Mac"
	}
	if strings.Contains(ua, "android") {
		return "Android"
	}
	if strings.Contains(ua, "windows") {
		return "Windows"
	}
	return "Web Push"
}
