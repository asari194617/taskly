package main

import (
	"github.com/gofiber/fiber/v2"
)

// authRequired ensures the request is authenticated via session or API key
func authRequired(c *fiber.Ctx) error {
	if userFromSession(c) || userFromAPIKey(c) {
		return c.Next()
	}
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
}

// audit logs API key authenticated actions for REST endpoints
func audit(c *fiber.Ctx) error {
	err := c.Next()
	// Per policy: record ONLY REST requests authenticated with API Key
	if apiKey, ok := c.Locals("apiKey").(string); ok && apiKey != "" {
		hashed := sha256Hex([]byte(apiKey))
		var uid *uint
		if u, ok := c.Locals("user").(*User); ok {
			uid = &u.ID
		}
		_ = DB.Create(&ApiActionLog{UserID: uid, HashedToken: hashed, Action: c.Method() + " " + c.Path(), IP: c.IP()}).Error
	}
	return err
}

// adminOnly allows only admin or manager roles
func adminOnly(c *fiber.Ctx) error {
	u := c.Locals("user").(*User)
	isAdmin, err1 := userHasRole(u.ID, "admin")
	isManager, err2 := userHasRole(u.ID, "manager")
	if err1 != nil || err2 != nil {
		return c.Status(500).JSON(fiber.Map{"error": "perm"})
	}
	if !(isAdmin || isManager) {
		return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
	}
	return c.Next()
}
