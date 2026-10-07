package handler

import (
	"crypto/subtle"
	"errors"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
		err = h.db.Transaction(func(tx *gorm.DB) error {
			var user domain.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "display_name").Where("id = ? AND is_deleted = 0", id).First(&user).Error; err != nil {
				return err
			}
			if user.DisplayName == name {
				return nil
			}
			return tx.Model(&user).Update("display_name", name).Error
		})
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.SendStatus(fiber.StatusNotFound)
		}
		if err != nil {
			return c.SendStatus(fiber.StatusServiceUnavailable)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
}
