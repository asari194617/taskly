package handler

import (
	"os"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) RegisterIntegrationRoutes(router fiber.Router) {
	h.registerTeamsIntegrationRoutes(router)
	h.registerDayShiftIntegrationRoutes(router)
	h.registerEmailIntegrationRoutes(router)
}

func hasTeamsIntegrationSecret(c *fiber.Ctx) bool {
	secret := c.Get("x-teams-integration-secret")
	return secret != "" && secret == os.Getenv("TEAMS_INTEGRATION_SECRET")
}
