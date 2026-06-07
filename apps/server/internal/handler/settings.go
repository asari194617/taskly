package handler

import (
	"strings"
	"net/http"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
)

func (h *Handler) RegisterSettingsRoutes(router fiber.Router) {
	// Auth settings endpoints
	router.Get("/auth", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		// refresh user
		var fresh domain.User
		_ = h.db.First(&fresh, u.ID).Error
		hasPassword := fresh.HashedPassword != nil && *fresh.HashedPassword != ""
		totpEnabled, _, _ := utils.GetActiveTOTP(fresh.ID, h.db)
		return c.JSON(fiber.Map{
			"email_login_enabled": fresh.EmailCodeAllowed,
			"has_password":        hasPassword,
			"totp_enabled":        totpEnabled,
		})
	})

	router.Post("/auth", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var body struct {
			EmailLoginEnabled *bool `json:"email_login_enabled"`
		}
		if err := utils.ParseJSONStrict(c, &body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid json"})
		}
		if body.EmailLoginEnabled != nil {
			_ = h.db.Model(&domain.User{}).Where("id = ?", u.ID).Update("email_code_allowed", *body.EmailLoginEnabled).Error
		}
		return c.SendStatus(204)
	})

	// Push settings (deprecated): map to WEBPUSH device enabled state
	router.Get("/push", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var cnt int64
		_ = h.db.Model(&domain.NotificationDevice{}).Where("user_id = ? AND platform = ? AND is_enabled = 1", u.ID, "WEBPUSH").Count(&cnt).Error
		return c.JSON(fiber.Map{"daily_enabled": cnt > 0})
	})

	router.Post("/push", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var body struct {
			DailyEnabled *bool `json:"daily_enabled"`
		}
		if err := utils.ParseJSONStrict(c, &body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid json"})
		}
		if body.DailyEnabled == nil {
			return c.SendStatus(204)
		}
		// Ensure a default WEBPUSH device row exists and set its enabled state
		var dev domain.NotificationDevice
		if err := h.db.Where("user_id = ? AND platform = ?", u.ID, "WEBPUSH").First(&dev).Error; err != nil {
			// create
			var name *string
			nm := "default"
			name = &nm
			dev = domain.NotificationDevice{UserID: u.ID, Name: name, Platform: "WEBPUSH", IsEnabled: *body.DailyEnabled}
			_ = h.db.Create(&dev).Error
		} else {
			_ = h.db.Model(&domain.NotificationDevice{}).Where("id = ?", dev.ID).Update("is_enabled", *body.DailyEnabled).Error
		}
		return c.SendStatus(204)
	})

	// Language settings (notification only)
	router.Get("/lang", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var fresh domain.User
		_ = h.db.First(&fresh, u.ID).Error
		return c.JSON(fiber.Map{"notification_lang": fresh.NotificationLang})
	})

	router.Post("/lang", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var body struct {
			NotificationLang *string `json:"notification_lang"`
		}
		if err := utils.ParseJSONStrict(c, &body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid json"})
		}
		if body.NotificationLang != nil {
			v := strings.ToLower(strings.TrimSpace(*body.NotificationLang))
			if v != "ja" && v != "en" {
				return c.Status(400).JSON(fiber.Map{"error": "invalid notification_lang"})
			}
			_ = h.db.Model(&domain.User{}).Where("id = ?", u.ID).Update("notification_lang", v).Error
		}
		return c.SendStatus(204)
	})

	// Methods (per channel enable): EMAIL, WEBPUSH
	router.Get("/notification-methods", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var emailCnt int64
		var webCnt int64
		_ = h.db.Model(&domain.NotificationDevice{}).Where("user_id = ? AND platform = 'EMAIL' AND is_enabled = 1", u.ID).Count(&emailCnt).Error
		_ = h.db.Model(&domain.NotificationDevice{}).Where("user_id = ? AND platform = 'WEBPUSH' AND is_enabled = 1", u.ID).Count(&webCnt).Error
		return c.JSON(fiber.Map{"email": emailCnt > 0, "webpush": webCnt > 0})
	})
	router.Post("/notification-methods", func(c *fiber.Ctx) error {
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
			if err := h.db.Where("user_id = ? AND platform = 'EMAIL'", u.ID).First(&dev).Error; err != nil {
				nm := "email"
				_ = h.db.Create(&domain.NotificationDevice{UserID: u.ID, Name: &nm, Platform: "EMAIL", IsEnabled: *body.Email}).Error
			} else {
				_ = h.db.Model(&domain.NotificationDevice{}).Where("user_id = ? AND platform = 'EMAIL'", u.ID).Update("is_enabled", *body.Email).Error
			}
		}
		if body.WebPush != nil {
			_ = h.db.Model(&domain.NotificationDevice{}).Where("user_id = ? AND platform = 'WEBPUSH'", u.ID).Update("is_enabled", *body.WebPush).Error
		}
		return c.SendStatus(204)
	})

	// Notification types: TASK, TASK_MORNING, TASK_EVENING, DAYSHIFT
	router.Get("/notification-types", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		types := []string{"TASK", "TASK_MORNING", "TASK_EVENING", "DAYSHIFT"}
		out := make(map[string]bool, len(types))
		for _, t := range types {
			var rec domain.NotificationTypePref
			if err := h.db.Where("user_id = ? AND notification_type = ?", u.ID, t).First(&rec).Error; err == nil {
				out[t] = rec.IsEnabled
			} else {
				// defaults: TASK=true, DAYSHIFT=true, MORNING/EVENTING=true
				out[t] = true
			}
		}
		return c.JSON(out)
	})
	router.Post("/notification-types", func(c *fiber.Ctx) error {
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
			if err := h.db.Where("user_id = ? AND notification_type = ?", u.ID, t).First(&rec).Error; err == nil {
				_ = h.db.Model(&domain.NotificationTypePref{}).Where("id = ?", rec.ID).Update("is_enabled", v).Error
			} else {
				rec = domain.NotificationTypePref{UserID: u.ID, NotificationType: t, IsEnabled: v}
				_ = h.db.Create(&rec).Error
			}
		}
		return c.SendStatus(204)
	})
}
