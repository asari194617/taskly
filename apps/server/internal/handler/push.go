package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	mw "github.com/asari194617/taskly/apps/server/internal/middleware"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func (h *Handler) RegisterPushRoutes(router fiber.Router, mw *mw.Middleware) {
	// Web Push subscription endpoints (no push send yet)
	router.Post("/subscribe", mw.AuthRequired, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var body struct{ Endpoint, P256dh, Auth string }
		if err := utils.ParseJSONStrict(c, &body); err != nil || strings.TrimSpace(body.Endpoint) == "" || strings.TrimSpace(body.P256dh) == "" || strings.TrimSpace(body.Auth) == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		payloadJSON, err := json.Marshal(body.Endpoint)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid endpoint"})
		}
		ua := strings.ToLower(strings.TrimSpace(c.Get("User-Agent")))
		name := detectDeviceNameFromUA(ua)
		if err := h.db.Transaction(func(tx *gorm.DB) error {
			var rec domain.WebPushSubscription
			if err := tx.Where("user_id = ? AND endpoint = ?", u.ID, body.Endpoint).First(&rec).Error; err == nil {
				rec.P256dh, rec.Auth = body.P256dh, body.Auth
				if err := tx.Save(&rec).Error; err != nil {
					return err
				}
			} else if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := tx.Create(&domain.WebPushSubscription{UserID: u.ID, Endpoint: body.Endpoint, P256dh: body.P256dh, Auth: body.Auth}).Error; err != nil {
					return err
				}
			} else {
				return err
			}
			var dev domain.NotificationDevice
			if err := tx.Where("user_id = ? AND platform = 'WEBPUSH' AND JSON_UNQUOTE(payload) = ?", u.ID, body.Endpoint).First(&dev).Error; err == nil {
				return tx.Model(&dev).Updates(map[string]any{"is_enabled": true, "name": name}).Error
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			pl := string(payloadJSON)
			return tx.Create(&domain.NotificationDevice{UserID: u.ID, Name: &name, Platform: "WEBPUSH", Payload: &pl, IsEnabled: true}).Error
		}); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})
	router.Post("/unsubscribe", mw.AuthRequired, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var body struct{ Endpoint string }
		if err := utils.ParseJSONStrict(c, &body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if strings.TrimSpace(body.Endpoint) != "" {
			if err := h.db.Transaction(func(tx *gorm.DB) error {
				if err := tx.Where("user_id = ? AND endpoint = ?", u.ID, body.Endpoint).Delete(&domain.WebPushSubscription{}).Error; err != nil {
					return err
				}
				return tx.Model(&domain.NotificationDevice{}).Where("user_id = ? AND platform = 'WEBPUSH' AND JSON_UNQUOTE(payload) = ?", u.ID, body.Endpoint).Update("is_enabled", false).Error
			}); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "db"})
			}
		}
		return c.SendStatus(204)
	})

	// Devices listing (WEBPUSH only for UI)
	router.Get("/devices", mw.AuthRequired, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var devs []domain.NotificationDevice
		if err := h.db.Where("user_id = ? AND platform = 'WEBPUSH'", u.ID).Order("id DESC").Find(&devs).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		out := make([]map[string]any, 0, len(devs))
		for _, d := range devs {
			nm := ""
			if d.Name != nil {
				nm = *d.Name
			}
			out = append(out, map[string]any{"id": d.ID, "name": nm, "is_enabled": d.IsEnabled})
		}
		return c.JSON(out)
	})
	router.Post("/devices/:id", mw.AuthRequired, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var body struct {
			IsEnabled *bool `json:"is_enabled"`
		}
		if err := utils.ParseJSONStrict(c, &body); err != nil || body.IsEnabled == nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid json"})
		}
		res := h.db.Model(&domain.NotificationDevice{}).Where("id = ? AND user_id = ? AND platform = 'WEBPUSH'", c.Params("id"), u.ID).Update("is_enabled", *body.IsEnabled)
		if res.Error != nil || res.RowsAffected == 0 {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		return c.SendStatus(204)
	})

	router.Get("/vapidPublicKey", func(c *fiber.Ctx) error {
		pub, _, err := h.getOrInitVAPID()
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "vapid"})
		}
		return c.JSON(fiber.Map{"publicKey": pub})
	})
	router.Post("/test", mw.AuthRequired, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		sent, err := h.sendPushToUser(u.ID, map[string]any{
			"title": "Taskly",
			"body":  "プッシュ通知のテスト",
			"data":  map[string]any{"url": "/home"},
		})
		if err != nil {
			log.Printf("push test error: %v", err)
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "send"})
		}
		return c.JSON(fiber.Map{"sent": sent})
	})

	// Debug: list current user's subscriptions (masked endpoints)
	router.Get("/subscriptions", mw.AuthRequired, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var subs []domain.WebPushSubscription
		if err := h.db.Where("user_id = ?", u.ID).Find(&subs).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		out := make([]fiber.Map, 0, len(subs))
		for _, s := range subs {
			ep := s.Endpoint
			if len(ep) > 16 {
				ep = ep[:8] + "…" + ep[len(ep)-5:]
			}
			out = append(out, fiber.Map{"endpoint": ep, "created_at": s.CreatedAt})
		}
		return c.JSON(fiber.Map{"count": len(subs), "items": out})
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
