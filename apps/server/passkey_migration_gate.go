package server

import (
	"github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// blockLegacyPasskeys is enabled only after the auth service owns passkey
// writes. It also stops legacy assertion counter updates during cutover.
func blockLegacyPasskeys(c *fiber.Ctx) error {
	if utils.Env("AUTH_PASSKEYS_EXTERNAL", "") != "1" {
		return c.Next()
	}
	c.Set("Cache-Control", "no-store")
	return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "passkeys_managed_by_auth_service"})
}
