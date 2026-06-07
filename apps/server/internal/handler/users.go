package handler

import (
	"strings"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/gofiber/fiber/v2"
)

func (h *Handler) RegisterUsersRoutes(router fiber.Router) {
	// User profile: update display name
	router.Patch("/me", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var body struct {
			DisplayName *string `json:"display_name"`
		}
		if err := c.BodyParser(&body); err != nil {
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
		if len(updates) == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "no changes"})
		}
		if err := h.db.Model(&domain.User{}).Where("id = ?", u.ID).Updates(updates).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// User account delete (soft delete)
	router.Delete("/me", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		if err := h.db.Model(&domain.User{}).Where("id = ?", u.ID).Update("is_deleted", true).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
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
