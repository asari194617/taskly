package handler

import (
	"errors"
	"strings"

	"github.com/asari194617/taskly/apps/server/internal/authsync"
	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
)

func (h *Handler) RegisterUsersRoutes(router fiber.Router) {
	// User profile: update display name and notification language
	router.Patch("/me", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var body struct {
			DisplayName      *string `json:"display_name"`
			NotificationLang *string `json:"notification_lang"`
		}
		if err := utils.ParseJSONStrict(c, &body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		updates := map[string]any{}
		if body.DisplayName != nil {
			dn := strings.TrimSpace(*body.DisplayName)
			if len(dn) > 255 {
				dn = dn[:255]
			}
			updates["display_name"] = dn
		}
		if body.NotificationLang != nil {
			lang := strings.ToLower(strings.TrimSpace(*body.NotificationLang))
			if lang != "ja" && lang != "en" {
				return c.Status(400).JSON(fiber.Map{"error": "invalid notification_lang"})
			}
			updates["notification_lang"] = lang
		}
		if len(updates) == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "no changes"})
		}
		if authsync.UsersEnabled() {
			input := authsync.UserUpdate{}
			if value, ok := updates["display_name"].(string); ok {
				input.DisplayName = &value
			}
			if value, ok := updates["notification_lang"].(string); ok {
				input.NotificationLang = &value
			}
			if err := authsync.UpdateUser(c.Context(), u.ID, input); err != nil {
				return c.Status(503).JSON(fiber.Map{"error": "auth synchronization pending"})
			}
		}
		if err := h.db.Model(&domain.User{}).Where("id = ?", u.ID).Updates(updates).Error; err != nil {
			if authsync.UsersEnabled() {
				return c.Status(503).JSON(fiber.Map{"error": "auth synchronization pending"})
			}
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// User account delete (soft delete)
	router.Delete("/me", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		if err := utils.SoftDeleteUser(c.Context(), u.ID, h.db, h.rdb); err != nil {
			if errors.Is(err, authsync.ErrLifecycleSync) {
				utils.ClearSessionCookie(c)
				return c.Status(503).JSON(fiber.Map{"error": "auth synchronization pending"})
			}
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		utils.ClearSessionCookie(c)
		return c.SendStatus(204)
	})

	// Users/me (auth required)
	router.Get("/me", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		// roles
		var rls []domain.RoleUser
		_ = h.db.Where("user_id = ?", u.ID).Find(&rls).Error
		roles := make([]string, 0, len(rls))
		isAdmin := false
		isManager := false
		for _, r := range rls {
			roles = append(roles, r.Role)
			if r.Role == "admin" {
				isAdmin = true
			}
			if r.Role == "manager" {
				isManager = true
			}
		}
		return c.JSON(fiber.Map{"id": u.ID, "email": u.Email, "display_name": u.DisplayName, "roles": roles, "is_admin": isAdmin, "is_manager": isManager})
	})
}
