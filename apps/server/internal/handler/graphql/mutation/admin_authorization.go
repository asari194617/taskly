package gqlMutation

import (
	"fmt"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// requireAdminActor keeps transport authentication and role checks out of
// individual admin resolvers. Managers intentionally retain the same access
// they had before this extraction.
func (h *Handler) requireAdminActor(c *fiber.Ctx) (*domain.User, error) {
	if !utils.UserFromSession(c, h.db, h.rdb) {
		return nil, fmt.Errorf("unauthorized")
	}
	actor := c.Locals("user").(*domain.User)
	isAdmin, err := utils.UserHasRole(actor.ID, "admin", h.db)
	if err != nil {
		return nil, fmt.Errorf("authorization: %w", err)
	}
	isManager, err := utils.UserHasRole(actor.ID, "manager", h.db)
	if err != nil {
		return nil, fmt.Errorf("authorization: %w", err)
	}
	if !isAdmin && !isManager {
		return nil, fmt.Errorf("forbidden")
	}
	return actor, nil
}
