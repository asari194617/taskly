package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func settingsDBError(c *fiber.Ctx) error {
	return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "db"})
}

func (h *Handler) RegisterSettingsRoutes(router fiber.Router) {
	// Auth settings endpoints
	router.Get("/auth", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		// refresh user
		var fresh domain.User
		if err := h.db.First(&fresh, u.ID).Error; err != nil {
			return settingsDBError(c)
		}
		hasPassword := fresh.HashedPassword != nil && *fresh.HashedPassword != ""
		totpEnabled, _, err := utils.GetActiveTOTP(fresh.ID, h.db)
		if err != nil {
			return settingsDBError(c)
		}
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
			if err := h.db.Model(&domain.User{}).Where("id = ?", u.ID).Update("email_code_allowed", *body.EmailLoginEnabled).Error; err != nil {
				return settingsDBError(c)
			}
		}
		return c.SendStatus(204)
	})

	// Push settings (deprecated): map to WEBPUSH device enabled state
	router.Get("/push", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var cnt int64
		if err := h.db.Model(&domain.NotificationDevice{}).Where("user_id = ? AND platform = ? AND is_enabled = 1", u.ID, "WEBPUSH").Count(&cnt).Error; err != nil {
			return settingsDBError(c)
		}
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
		if err := h.db.Where("user_id = ? AND platform = ?", u.ID, "WEBPUSH").First(&dev).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			// create
			var name *string
			nm := "default"
			name = &nm
			dev = domain.NotificationDevice{UserID: u.ID, Name: name, Platform: "WEBPUSH", IsEnabled: *body.DailyEnabled}
			if err := h.db.Create(&dev).Error; err != nil {
				return settingsDBError(c)
			}
		} else if err != nil {
			return settingsDBError(c)
		} else {
			if err := h.db.Model(&domain.NotificationDevice{}).Where("id = ?", dev.ID).Update("is_enabled", *body.DailyEnabled).Error; err != nil {
				return settingsDBError(c)
			}
		}
		return c.SendStatus(204)
	})

	// Language settings (notification only)
	router.Get("/lang", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var fresh domain.User
		if err := h.db.First(&fresh, u.ID).Error; err != nil {
			return settingsDBError(c)
		}
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
			if err := h.db.Model(&domain.User{}).Where("id = ?", u.ID).Update("notification_lang", v).Error; err != nil {
				return settingsDBError(c)
			}
		}
		return c.SendStatus(204)
	})

	// Methods (per channel enable): EMAIL, WEBPUSH
	router.Get("/notification-methods", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var emailCnt int64
		var webCnt int64
		if err := h.db.Model(&domain.NotificationDevice{}).Where("user_id = ? AND platform = 'EMAIL' AND is_enabled = 1", u.ID).Count(&emailCnt).Error; err != nil {
			return settingsDBError(c)
		}
		if err := h.db.Model(&domain.NotificationDevice{}).Where("user_id = ? AND platform = 'WEBPUSH' AND is_enabled = 1", u.ID).Count(&webCnt).Error; err != nil {
			return settingsDBError(c)
		}
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
		if err := h.db.Transaction(func(tx *gorm.DB) error {
			if body.Email != nil {
				// Ensure at least one EMAIL device row exists
				var dev domain.NotificationDevice
				if err := tx.Where("user_id = ? AND platform = 'EMAIL'", u.ID).First(&dev).Error; errors.Is(err, gorm.ErrRecordNotFound) {
					nm := "email"
					if err := tx.Create(&domain.NotificationDevice{UserID: u.ID, Name: &nm, Platform: "EMAIL", IsEnabled: *body.Email}).Error; err != nil {
						return err
					}
				} else if err != nil {
					return err
				} else {
					if err := tx.Model(&domain.NotificationDevice{}).Where("user_id = ? AND platform = 'EMAIL'", u.ID).Update("is_enabled", *body.Email).Error; err != nil {
						return err
					}
				}
			}
			if body.WebPush != nil {
				if err := tx.Model(&domain.NotificationDevice{}).Where("user_id = ? AND platform = 'WEBPUSH'", u.ID).Update("is_enabled", *body.WebPush).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return settingsDBError(c)
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
			} else if errors.Is(err, gorm.ErrRecordNotFound) {
				// defaults: TASK=true, DAYSHIFT=true, MORNING/EVENTING=true
				out[t] = true
			} else {
				return settingsDBError(c)
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
		if err := h.db.Transaction(func(tx *gorm.DB) error {
			for k, v := range body {
				t := strings.ToUpper(strings.TrimSpace(k))
				if t != "TASK" && t != "TASK_MORNING" && t != "TASK_EVENING" && t != "DAYSHIFT" {
					continue
				}
				var rec domain.NotificationTypePref
				if err := tx.Where("user_id = ? AND notification_type = ?", u.ID, t).First(&rec).Error; err == nil {
					if err := tx.Model(&domain.NotificationTypePref{}).Where("id = ?", rec.ID).Update("is_enabled", v).Error; err != nil {
						return err
					}
				} else if errors.Is(err, gorm.ErrRecordNotFound) {
					rec = domain.NotificationTypePref{UserID: u.ID, NotificationType: t, IsEnabled: v}
					if err := tx.Create(&rec).Error; err != nil {
						return err
					}
				} else {
					return err
				}
			}
			return nil
		}); err != nil {
			return settingsDBError(c)
		}
		return c.SendStatus(204)
	})
}
