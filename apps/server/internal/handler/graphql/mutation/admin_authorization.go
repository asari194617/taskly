package gqlMutation

import (
	"fmt"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// requireAdminActor authenticates an admin operation and optionally permits managers.
func (h *Handler) requireAdminActor(c *fiber.Ctx, allowManager bool) (*domain.User, error) {
	if !utils.UserFromSession(c, h.db, h.rdb) {
		return nil, fmt.Errorf("unauthorized")
	}
	actor := c.Locals("user").(*domain.User)
	isAdmin, err := utils.UserHasRole(actor.ID, "admin", h.db)
	if err != nil {
		return nil, fmt.Errorf("authorization: %w", err)
	}
	if isAdmin {
		return actor, nil
	}
	if allowManager {
		isManager, err := utils.UserHasRole(actor.ID, "manager", h.db)
		if err != nil {
			return nil, fmt.Errorf("authorization: %w", err)
		}
		if isManager {
			return actor, nil
		}
	}
	return nil, fmt.Errorf("forbidden")
}
