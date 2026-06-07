package adminHandler

import (
	"strconv"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
)

func (h *Handler) RegisterAdminUsersRoutes(router fiber.Router) {
	// Admin: user management (list/search, roles, block/unblock)
	router.Get("/", func(c *fiber.Ctx) error {
		q := strings.TrimSpace(c.Query("q"))
		limit, _ := strconv.Atoi(c.Query("limit", "100"))
		if limit <= 0 || limit > 500 {
			limit = 100
		}
		dbq := h.db.Model(&domain.User{})
		if q != "" {
			like := "%" + q + "%"
			dbq = dbq.Where("email LIKE ? OR display_name LIKE ?", like, like)
		}
		var users []domain.User
		if err := dbq.Order("id DESC").Limit(limit).Find(&users).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		// fetch roles and block status
		ids := make([]uint, 0, len(users))
		for _, u := range users {
			ids = append(ids, u.ID)
		}
		var roles []domain.RoleUser
		if len(ids) > 0 {
			_ = h.db.Where("user_id IN ?", ids).Find(&roles).Error
		}
		roleMap := map[uint][]string{}
		for _, r := range roles {
			roleMap[r.UserID] = append(roleMap[r.UserID], r.Role)
		}
		// block status
		blocked := map[uint]bool{}
		if len(ids) > 0 {
			var blks []domain.Block
			_ = h.db.Where("user_id IN ? AND (expires_at IS NULL OR expires_at > ?)", ids, time.Now()).Find(&blks).Error
			for _, b := range blks {
				blocked[b.UserID] = true
			}
		}
		out := make([]map[string]any, 0, len(users))
		for _, u := range users {
			out = append(out, map[string]any{"id": u.ID, "email": u.Email, "display_name": u.DisplayName, "is_deleted": u.IsDeleted, "roles": roleMap[u.ID], "is_blocked": blocked[u.ID]})
		}
		return c.JSON(out)
	})

	router.Post("/:id/roles", func(c *fiber.Ctx) error {
		uid := c.Params("id")
		var body struct {
			Role string `json:"role"`
		}
		if err := utils.ParseJSONStrict(c, &body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid json"})
		}
		role := strings.ToLower(strings.TrimSpace(body.Role))
		if role != "admin" && role != "manager" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid role"})
		}
		// check exists
		var cnt int64
		_ = h.db.Model(&domain.RoleUser{}).Where("user_id = ? AND role = ?", uid, role).Count(&cnt).Error
		if cnt == 0 {
			_ = h.db.Create(&domain.RoleUser{UserID: utils.ToUint(uid), Role: role}).Error
		}
		return c.SendStatus(201)
	})

	router.Delete("/:id/roles/:role", func(c *fiber.Ctx) error {
		if err := h.db.Where("user_id = ? AND role = ?", c.Params("id"), c.Params("role")).Delete(&domain.RoleUser{}).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})
	
	router.Post("/:id/block", func(c *fiber.Ctx) error {
		var body struct {
			Reason    *string `json:"reason"`
			ExpiresAt *string `json:"expires_at"`
		}
		if err := utils.ParseJSONStrict(c, &body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid json"})
		}
		var expPtr *time.Time
		if body.ExpiresAt != nil && *body.ExpiresAt != "" {
			if t, err := time.Parse(time.RFC3339, *body.ExpiresAt); err == nil {
				expPtr = &t
			} else {
				return c.Status(400).JSON(fiber.Map{"error": "invalid expires_at"})
			}
		}
		rec := domain.Block{UserID: utils.ToUint(c.Params("id")), Reason: body.Reason, ExpiresAt: expPtr}
		if err := h.db.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(201)
	})

	router.Post("/:id/unblock", func(c *fiber.Ctx) error {
		now := time.Now()
		if err := h.db.Model(&domain.Block{}).Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", c.Params("id"), now).Updates(map[string]any{"expires_at": now}).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})
}
