package handler

import (
	"context"
	"strings"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/taskimage"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
)

func (h *Handler) RegisterTaskImageRoutes(router fiber.Router) {
	router.Post("/presign", func(c *fiber.Ctx) error {
		user := c.Locals("user").(*domain.User)
		var body struct {
			ContentType string `json:"content_type"`
			Size        int64  `json:"size"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
		}
		service, err := taskimage.NewFromEnv()
		if err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "image upload unavailable"})
		}
		upload, err := service.PresignUpload(c.UserContext(), user.ID, strings.TrimSpace(body.ContentType), body.Size)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(upload)
	})
}

func deleteTaskImage(ctx context.Context, key *string) {
	if key == nil || strings.TrimSpace(*key) == "" {
		return
	}
	service, err := taskimage.NewFromEnv()
	if err != nil {
		utils.CaptureError(err)
		return
	}
	if err := service.Delete(ctx, *key); err != nil {
		utils.CaptureError(err)
	}
}
