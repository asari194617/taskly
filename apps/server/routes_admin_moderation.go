package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
)

type ModerationPolicy struct {
	// Default auto-hide threshold: low|medium|high
	DefaultAutoHide string `json:"default_auto_hide"`
	// Per-category override: category -> threshold
	AutoHideByCat map[string]string `json:"auto_hide_by_category"`
}

func defaultModerationPolicy() ModerationPolicy {
	return ModerationPolicy{DefaultAutoHide: "high", AutoHideByCat: map[string]string{"threat": "medium"}}
}

func loadModerationPolicy() ModerationPolicy {
	var row domain.SystemSetting
	if err := DB.Where("`key` = ?", "moderation_policy").First(&row).Error; err != nil || strings.TrimSpace(row.Value) == "" {
		return defaultModerationPolicy()
	}
	var p ModerationPolicy
	if err := json.Unmarshal([]byte(row.Value), &p); err != nil {
		return defaultModerationPolicy()
	}
	if p.AutoHideByCat == nil {
		p.AutoHideByCat = map[string]string{}
	}
	if p.DefaultAutoHide == "" {
		p.DefaultAutoHide = "high"
	}
	return p
}

func saveModerationPolicy(p ModerationPolicy) error {
	b, _ := json.Marshal(p)
	now := time.Now()
	var existing domain.SystemSetting
	if err := DB.Where("`key` = ?", "moderation_policy").First(&existing).Error; err == nil {
		existing.Value = string(b)
		existing.UpdatedAt = now
		return DB.Save(&existing).Error
	}
	rec := domain.SystemSetting{Key: "moderation_policy", Value: string(b), CreatedAt: now, UpdatedAt: now}
	return DB.Create(&rec).Error
}

func severityRank(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "low":
		return 1
	case "medium":
		return 2
	case "high":
		return 3
	default:
		return 0
	}
}

func registerAdminModerationRoutes(app *fiber.App) {
	// List flagged items
	app.Get("/admin/moderation/flagged", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		limit, _ := strconv.Atoi(c.Query("limit", "50"))
		if limit <= 0 || limit > 200 {
			limit = 50
		}
		since := c.Query("since")
		dbq := DB.Table("flagged_contents f").Select("f.*, t.title AS task_title, t.is_hidden AS task_hidden").Joins("JOIN tasks t ON t.id = f.task_id")
		if strings.TrimSpace(since) != "" {
			if ts, err := time.Parse(time.RFC3339, since); err == nil {
				dbq = dbq.Where("f.created_at >= ?", ts)
			}
		}
		var rows []map[string]any
		if err := dbq.Order("f.id DESC").Limit(limit).Find(&rows).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.JSON(rows)
	})

	// Hide/unhide task
	app.Post("/admin/moderation/tasks/:id/hide", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		if err := DB.Model(&domain.Task{}).Where("id = ?", c.Params("id")).Update("is_hidden", true).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})
	app.Post("/admin/moderation/tasks/:id/unhide", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		if err := DB.Model(&domain.Task{}).Where("id = ?", c.Params("id")).Update("is_hidden", false).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// Policy get/set
	app.Get("/admin/moderation/settings", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		p := loadModerationPolicy()
		return c.JSON(p)
	})
	app.Put("/admin/moderation/settings", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		var body ModerationPolicy
		if err := utils.ParseJSONStrict(c, &body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid json"})
		}
		// sanitize
		body.DefaultAutoHide = strings.ToLower(strings.TrimSpace(body.DefaultAutoHide))
		if body.DefaultAutoHide == "" {
			body.DefaultAutoHide = "high"
		}
		for k, v := range body.AutoHideByCat {
			body.AutoHideByCat[strings.ToLower(strings.TrimSpace(k))] = strings.ToLower(strings.TrimSpace(v))
		}
		if err := saveModerationPolicy(body); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})
}
