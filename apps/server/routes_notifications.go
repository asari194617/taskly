package main

import (
	"net/http"
	"strings"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// registerNotificationSettingsRoutes adds endpoints for methods, types, and device management
func registerNotificationSettingsRoutes(app *fiber.App) {
	// Methods (per channel enable): EMAIL, WEBPUSH
	app.Get("/settings/notification-methods", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var emailCnt int64
		var webCnt int64
		_ = DB.Model(&domain.NotificationDevice{}).Where("user_id = ? AND platform = 'EMAIL' AND is_enabled = 1", u.ID).Count(&emailCnt).Error
		_ = DB.Model(&domain.NotificationDevice{}).Where("user_id = ? AND platform = 'WEBPUSH' AND is_enabled = 1", u.ID).Count(&webCnt).Error
		return c.JSON(fiber.Map{"email": emailCnt > 0, "webpush": webCnt > 0})
	})
	app.Post("/settings/notification-methods", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var body struct {
			Email   *bool `json:"email"`
			WebPush *bool `json:"webpush"`
		}
		if err := utils.ParseJSONStrict(c, &body); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid json"})
		}
		if body.Email != nil {
			// Ensure at least one EMAIL device row exists
			var dev domain.NotificationDevice
			if err := DB.Where("user_id = ? AND platform = 'EMAIL'", u.ID).First(&dev).Error; err != nil {
				nm := "email"
				_ = DB.Create(&domain.NotificationDevice{UserID: u.ID, Name: &nm, Platform: "EMAIL", IsEnabled: *body.Email}).Error
			} else {
				_ = DB.Model(&domain.NotificationDevice{}).Where("user_id = ? AND platform = 'EMAIL'", u.ID).Update("is_enabled", *body.Email).Error
			}
		}
		if body.WebPush != nil {
			_ = DB.Model(&domain.NotificationDevice{}).Where("user_id = ? AND platform = 'WEBPUSH'", u.ID).Update("is_enabled", *body.WebPush).Error
		}
		return c.SendStatus(204)
	})

	// Notification types: TASK, TASK_MORNING, TASK_EVENING, DAYSHIFT
	app.Get("/settings/notification-types", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		types := []string{"TASK", "TASK_MORNING", "TASK_EVENING", "DAYSHIFT"}
		out := make(map[string]bool, len(types))
		for _, t := range types {
			var rec domain.NotificationTypePref
			if err := DB.Where("user_id = ? AND notification_type = ?", u.ID, t).First(&rec).Error; err == nil {
				out[t] = rec.IsEnabled
			} else {
				// defaults: TASK=true, DAYSHIFT=true, MORNING/EVENTING=true
				out[t] = true
			}
		}
		return c.JSON(out)
	})
	app.Post("/settings/notification-types", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var body map[string]bool
		if err := utils.ParseJSONStrict(c, &body); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid json"})
		}
		for k, v := range body {
			t := strings.ToUpper(strings.TrimSpace(k))
			if t != "TASK" && t != "TASK_MORNING" && t != "TASK_EVENING" && t != "DAYSHIFT" {
				continue
			}
			var rec domain.NotificationTypePref
			if err := DB.Where("user_id = ? AND notification_type = ?", u.ID, t).First(&rec).Error; err == nil {
				_ = DB.Model(&domain.NotificationTypePref{}).Where("id = ?", rec.ID).Update("is_enabled", v).Error
			} else {
				rec = domain.NotificationTypePref{UserID: u.ID, NotificationType: t, IsEnabled: v}
				_ = DB.Create(&rec).Error
			}
		}
		return c.SendStatus(204)
	})

	// Devices listing (WEBPUSH only for UI)
	app.Get("/push/devices", authRequired, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var devs []domain.NotificationDevice
		if err := DB.Where("user_id = ? AND platform = 'WEBPUSH'", u.ID).Order("id DESC").Find(&devs).Error; err != nil {
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
	app.Post("/push/devices/:id", authRequired, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var body struct {
			IsEnabled *bool `json:"is_enabled"`
		}
		if err := utils.ParseJSONStrict(c, &body); err != nil || body.IsEnabled == nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid json"})
		}
		res := DB.Model(&domain.NotificationDevice{}).Where("id = ? AND user_id = ? AND platform = 'WEBPUSH'", c.Params("id"), u.ID).Update("is_enabled", *body.IsEnabled)
		if res.Error != nil || res.RowsAffected == 0 {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		return c.SendStatus(204)
	})
}
