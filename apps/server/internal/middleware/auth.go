package middleware

import (
	"strconv"
	"strings"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/legal"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Middleware struct {
	db    *gorm.DB
	rdb   *redis.Client
	wAuth *webauthn.WebAuthn
}

func NewMiddleware(db *gorm.DB, rdb *redis.Client, wAuth *webauthn.WebAuthn) *Middleware {
	return &Middleware{
		db:    db,
		rdb:   rdb,
		wAuth: wAuth,
	}
}

// AuthRequired ensures the request is authenticated via session or API key
func (mw *Middleware) AuthRequired(c *fiber.Ctx) error {
	if utils.UserFromSession(c, mw.db, mw.rdb) || utils.UserFromAPIKey(c, mw.db) || utils.UserFromMCPToken(c, mw.db) {
		u := c.Locals("user").(*domain.User)
		current, err := legal.Current(mw.db, u.ID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "consent status unavailable"})
		}
		if !current {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "policy consent required"})
		}
		return c.Next()
	}
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
}

// Audit logs API key authenticated actions for REST endpoints
func (mw *Middleware) Audit(c *fiber.Ctx) error {
	err := c.Next()
	// Per policy: record ONLY REST requests authenticated with API Key
	if apiKey, ok := c.Locals("apiKey").(string); ok && apiKey != "" {
		hashed := utils.Sha256Hex([]byte(apiKey))
		var uid *uint
		if u, ok := c.Locals("user").(*domain.User); ok {
			uid = &u.ID
		}
		// The requested action has already completed; audit logging is best effort.
		// Report failures to Sentry without changing the action's response.
		if logErr := mw.db.Create(&domain.ApiActionLog{UserID: uid, HashedToken: hashed, Action: c.Method() + " " + c.Path(), IP: c.IP()}).Error; logErr != nil {
			utils.CaptureError(logErr)
		}
	}
	if err != nil && utils.Env("SENTRY_DSN", "") != "" {
		// best-effort capture
		// Note: importing sentry here would create circular imports; call via global function
		utils.CaptureError(err)
	}
	return err
}

// AdminOnly allows only admin or manager roles
func (mw *Middleware) AdminOnly(c *fiber.Ctx) error {
	u := c.Locals("user").(*domain.User)
	isAdmin, err1 := utils.UserHasRole(u.ID, "admin", mw.db)
	isManager, err2 := utils.UserHasRole(u.ID, "manager", mw.db)
	if err1 != nil || err2 != nil {
		return c.Status(500).JSON(fiber.Map{"error": "perm"})
	}
	if !(isAdmin || isManager) {
		return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
	}
	return c.Next()
}

// ApiKeyRouteScope restricts what endpoints can be accessed with API and MCP tokens.
// Allowed when using API Key:
// - GET /tasks
// - POST /tasks
// - PATCH /tasks/:id (numeric)
// - DELETE /tasks/:id (numeric)
// - GET /users/me
// - PATCH /users/me
func (mw *Middleware) ApiKeyRouteScope(c *fiber.Ctx) error {
	if c.Get("X-MCP-Token") != "" {
		if c.Get("X-API-Key") != "" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "ambiguous credentials"})
		}
		method, path := c.Method(), strings.TrimRight(c.Path(), "/")
		if ((method == fiber.MethodGet || method == fiber.MethodPost) && path == "/tasks") ||
			((method == fiber.MethodPatch || method == fiber.MethodDelete) && strings.HasPrefix(path, "/tasks/") && validTaskID(strings.TrimPrefix(path, "/tasks/"))) ||
			(method == fiber.MethodPatch && path == "/users/me") {
			return c.Next()
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "mcp token not allowed for this endpoint"})
	}
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

func validTaskID(id string) bool {
	parsed, err := strconv.ParseUint(id, 10, 64)
	return err == nil && parsed > 0
}
