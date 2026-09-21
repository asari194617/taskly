package handler

import (
	"fmt"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
)

func (h *Handler) RegisterIcalTokensRoutes(router fiber.Router) {
	// iCal token management under settings
	router.Get("/", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var items []domain.IcalKey
		if err := h.db.Where("user_id = ?", u.ID).Order("id DESC").Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		base := strings.TrimRight(utils.Env("ICAL_BASE_URL", utils.Env("API_PUBLIC_URL", "http://localhost:8080")+"/ical"), "/")
		out := make([]map[string]any, 0, len(items))
		for _, it := range items {
			out = append(out, map[string]any{
				"id":          it.ID,
				"description": it.Description,
				"url":         fmt.Sprintf("%s/%s.ics", base, it.Token),
				"created_at":  it.CreatedAt,
				"expires_at":  it.ExpiresAt,
			})
		}
		return c.JSON(out)
	})

	router.Post("/", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var body struct {
			Description *string    `json:"description"`
			ExpiresAt   *time.Time `json:"expires_at"`
		}
		_ = utils.ParseJSONStrict(c, &body)
		if body.Description != nil {
			d := strings.TrimSpace(*body.Description)
			if !utils.MaxLen(d, 255) {
				return utils.ValidationError(c, map[string]string{"description": "too long"})
			}
		}
		tok, err := utils.GenerateIcalToken()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "server"})
		}
		rec := domain.IcalKey{UserID: u.ID, Token: tok, Description: body.Description, ExpiresAt: body.ExpiresAt}
		if err := h.db.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		base := strings.TrimRight(utils.Env("ICAL_BASE_URL", utils.Env("API_PUBLIC_URL", "http://localhost:8080")+"/ical"), "/")
		return c.Status(201).JSON(fiber.Map{"id": rec.ID, "url": fmt.Sprintf("%s/%s.ics", base, rec.Token)})
	})

	router.Delete("/:id", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		if err := h.db.Where("id = ? AND user_id = ?", c.Params("id"), u.ID).Delete(&domain.IcalKey{}).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})
}
