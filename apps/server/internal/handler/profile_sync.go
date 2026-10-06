package handler

import (
	"crypto/subtle"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
)

func (h *Handler) RegisterAuthProfileSyncRoutes(router fiber.Router) {
	router.Patch("/users/:id/profile", func(c *fiber.Ctx) error {
		if os.Getenv("AUTH_USERS_EXTERNAL") != "1" {
			return c.SendStatus(fiber.StatusServiceUnavailable)
		}
		token := os.Getenv("AUTH_INTERNAL_TOKEN")
		provided := strings.TrimPrefix(c.Get("Authorization"), "Bearer ")
		if len(token) < 32 || len(provided) != len(token) || subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		id, err := strconv.ParseUint(c.Params("id"), 10, 64)
		if err != nil || id == 0 {
			return c.SendStatus(fiber.StatusBadRequest)
		}
		var body struct {
			DisplayName string `json:"display_name"`
		}
		if err := utils.ParseJSONStrict(c, &body); err != nil {
			return c.SendStatus(fiber.StatusBadRequest)
		}
		name := strings.TrimSpace(body.DisplayName)
		if name == "" || !utf8.ValidString(name) || len([]byte(name)) > 255 {
			return c.SendStatus(fiber.StatusBadRequest)
		}
		result := h.db.Model(&domain.User{}).Where("id = ? AND is_deleted = 0", id).Update("display_name", name)
		if result.Error != nil {
			return c.SendStatus(fiber.StatusServiceUnavailable)
		}
		if result.RowsAffected != 1 {
			return c.SendStatus(fiber.StatusNotFound)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
}
