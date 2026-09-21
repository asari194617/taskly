package handler

import (
	"strconv"

	"github.com/asari194617/taskly/apps/server/internal/jobs"
	"github.com/gofiber/fiber/v2"
)

// RegisterIntegrationJobRoutes is mounted behind the normal admin middleware.
func (h *Handler) RegisterIntegrationJobRoutes(router fiber.Router) {
	router.Get("/", func(c *fiber.Ctx) error {
		status := c.Query("status", "failed")
		switch status {
		case "pending", "processing", "succeeded", "failed":
		default:
			return c.Status(400).JSON(fiber.Map{"error": "invalid status"})
		}
		var rows []jobs.Job
		if err := h.db.Where("status = ?", status).Order("id DESC").Limit(100).Find(&rows).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		items := make([]fiber.Map, 0, len(rows))
		for _, row := range rows {
			items = append(items, fiber.Map{
				"id": row.ID, "kind": row.Kind, "status": row.Status,
				"attempts": row.Attempts, "available_at": row.AvailableAt,
				"last_error": row.LastError, "created_at": row.CreatedAt,
			})
		}
		return c.JSON(fiber.Map{"items": items})
	})
	router.Post("/:id/retry", func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 64)
		if err != nil || id == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
		}
		if err := jobs.Retry(h.db, id); err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "failed job not found"})
		}
		return c.SendStatus(202)
	})
}
