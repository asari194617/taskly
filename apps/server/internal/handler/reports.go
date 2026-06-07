package handler

import (
	"strings"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/notify"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
)

func (h *Handler) RegisterReportRoutes(router fiber.Router) {
	router.Post("/", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var body struct {
			TaskID  uint    `json:"task_id"`
			Reason  string  `json:"reason"`
			Comment *string `json:"comment"`
		}
		if err := utils.ParseJSONStrict(c, &body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid_json"})
		}
		body.Reason = strings.TrimSpace(body.Reason)
		if body.TaskID == 0 || body.Reason == "" || len(body.Reason) > 255 {
			return utils.ValidationError(c, map[string]string{"task_id": "required", "reason": "required,max=255"})
		}
		// Delegate DB creation to domain layer
		rec, err := domain.CreateReport(h.db, u, body.TaskID, body.Reason, body.Comment)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "db"})
		}
		// Fire-and-forget Discord notification if configured (in notifier package)
		go func(taskID uint, reason string) {
			hook := utils.Env("DISCORD_WEBHOOK_URL", "")
			if strings.TrimSpace(hook) == "" {
				return
			}
			_ = notify.SendReportToDiscord(hook, taskID, reason)
		}(body.TaskID, body.Reason)
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": rec.ID})
	})
}
