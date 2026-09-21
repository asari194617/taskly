package server

import (
	"github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// pauseForAuthMigration closes every API route while the identity tables are
// copied. A GET callback can create a session, so HTTP method checks are not
// sufficient. Keep only the health endpoint available to deployment probes.
func pauseForAuthMigration(c *fiber.Ctx) error {
	if utils.Env("AUTH_WRITES_PAUSED", "") != "1" || (c.Method() == fiber.MethodGet && c.Path() == "/healthz") {
		return c.Next()
	}
	c.Set("Retry-After", "60")
	c.Set("Cache-Control", "no-store")
	return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "maintenance"})
}
