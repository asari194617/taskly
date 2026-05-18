package main

import (
	"strconv"
	"strings"

	utils "github.com/asari194617/taskly/apps/server/utils"
	"github.com/gofiber/fiber/v2"
)

// authRequired ensures the request is authenticated via session or API key
func authRequired(c *fiber.Ctx) error {
	if utils.UserFromSession(c, DB, RDB) || utils.UserFromAPIKey(c, DB) {
		return c.Next()
	}
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
}

// audit logs API key authenticated actions for REST endpoints
func audit(c *fiber.Ctx) error {
	err := c.Next()
	// Per policy: record ONLY REST requests authenticated with API Key
	if apiKey, ok := c.Locals("apiKey").(string); ok && apiKey != "" {
		hashed := utils.Sha256Hex([]byte(apiKey))
		var uid *uint
		if u, ok := c.Locals("user").(*utils.User); ok {
			uid = &u.ID
		}
		_ = DB.Create(&utils.ApiActionLog{UserID: uid, HashedToken: hashed, Action: c.Method() + " " + c.Path(), IP: c.IP()}).Error
	}
	if err != nil && utils.Env("SENTRY_DSN", "") != "" {
		// best-effort capture
		// Note: importing sentry here would create circular imports; call via global function
		utils.CaptureError(err)
	}
	return err
}

// adminOnly allows only admin or manager roles
func adminOnly(c *fiber.Ctx) error {
	u := c.Locals("user").(*utils.User)
	isAdmin, err1 := utils.UserHasRole(u.ID, "admin", DB)
	isManager, err2 := utils.UserHasRole(u.ID, "manager", DB)
	if err1 != nil || err2 != nil {
		return c.Status(500).JSON(fiber.Map{"error": "perm"})
	}
	if !(isAdmin || isManager) {
		return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
	}
	return c.Next()
}

// apiKeyRouteScope restricts what endpoints can be accessed when using X-API-Key
// Allowed when using API Key:
// - GET /tasks
// - POST /tasks
// - PATCH /tasks/:id (numeric)
// - DELETE /tasks/:id (numeric)
// - GET /users/me
// - PATCH /users/me
func apiKeyRouteScope(c *fiber.Ctx) error {
	if c.Get("X-API-Key") == "" {
		return c.Next()
	}
	method := c.Method()
	path := c.Path()
	// Normalize path (strip trailing slash)
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = strings.TrimRight(path, "/")
	}
	// Exact matches first
	if (method == fiber.MethodGet && path == "/tasks") ||
		(method == fiber.MethodPost && path == "/tasks") ||
		(method == fiber.MethodGet && path == "/users/me") ||
		(method == fiber.MethodPatch && path == "/users/me") {
		return c.Next()
	}
	// /tasks/:id numeric for PATCH/DELETE
	if strings.HasPrefix(path, "/tasks/") && (method == fiber.MethodPatch || method == fiber.MethodDelete) {
		seg := strings.TrimPrefix(path, "/tasks/")
		if seg != "" {
			if _, err := strconv.ParseUint(seg, 10, 64); err == nil {
				return c.Next()
			}
		}
	}
	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "api key not allowed for this endpoint"})
}
