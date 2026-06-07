package adminHandler

import (
	"strings"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/gofiber/fiber/v2"
)

func (h *Handler) RegisterAdminPermissionsRoutes(router fiber.Router) {
	// Admin: manage permissions
	router.Get("/", func(c *fiber.Ctx) error {
		var items []domain.Permission
		if err := h.db.Order("id DESC").Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.JSON(items)
	})
	router.Post("/", func(c *fiber.Ctx) error {
		var body struct {
			UserID uint    `json:"user_id"`
			Target string  `json:"target"`
			Method string  `json:"method"`
			Value  *string `json:"value"`
		}
		if err := c.BodyParser(&body); err != nil || body.UserID == 0 || body.Target == "" || body.Method == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		rec := domain.Permission{UserID: body.UserID, Target: body.Target, Method: strings.ToLower(body.Method), Value: body.Value}
		if err := h.db.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.Status(201).JSON(rec)
	})
	router.Delete("/:id", func(c *fiber.Ctx) error {
		if err := h.db.Delete(&domain.Permission{}, c.Params("id")).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})
}
