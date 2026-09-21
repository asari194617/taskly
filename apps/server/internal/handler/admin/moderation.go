package adminHandler

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
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

func (h *Handler) loadModerationPolicy() (ModerationPolicy, error) {
	var row domain.SystemSetting
	if err := h.db.Where("`key` = ?", "moderation_policy").First(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return defaultModerationPolicy(), nil
	} else if err != nil {
		return ModerationPolicy{}, err
	}
	if strings.TrimSpace(row.Value) == "" {
		return defaultModerationPolicy(), nil
	}
	var p ModerationPolicy
	if err := json.Unmarshal([]byte(row.Value), &p); err != nil {
		return defaultModerationPolicy(), nil
	}
	if p.AutoHideByCat == nil {
		p.AutoHideByCat = map[string]string{}
	}
	if p.DefaultAutoHide == "" {
		p.DefaultAutoHide = "high"
	}
	return p, nil
}

func (h *Handler) saveModerationPolicy(p ModerationPolicy) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	now := time.Now()
	var existing domain.SystemSetting
	if err := h.db.Where("`key` = ?", "moderation_policy").First(&existing).Error; err == nil {
		existing.Value = string(b)
		existing.UpdatedAt = now
		return h.db.Save(&existing).Error
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	rec := domain.SystemSetting{Key: "moderation_policy", Value: string(b), CreatedAt: now, UpdatedAt: now}
	return h.db.Create(&rec).Error
}

func (h *Handler) RegisterAdminModerationRoutes(router fiber.Router) {
	// List flagged items
	router.Get("/flagged", func(c *fiber.Ctx) error {
		limit, _ := strconv.Atoi(c.Query("limit", "50"))
		if limit <= 0 || limit > 200 {
			limit = 50
		}
		since := c.Query("since")
		dbq := h.db.Table("flagged_contents f").Select("f.*, t.title AS task_title, t.is_hidden AS task_hidden").Joins("JOIN tasks t ON t.id = f.task_id")
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
	router.Post("/tasks/:id/hide", func(c *fiber.Ctx) error {
		if err := h.db.Model(&domain.Task{}).Where("id = ?", c.Params("id")).Update("is_hidden", true).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})
	router.Post("/tasks/:id/unhide", func(c *fiber.Ctx) error {
		if err := h.db.Model(&domain.Task{}).Where("id = ?", c.Params("id")).Update("is_hidden", false).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// Policy get/set
	router.Get("/settings", func(c *fiber.Ctx) error {
		p, err := h.loadModerationPolicy()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.JSON(p)
	})
	router.Put("/settings", func(c *fiber.Ctx) error {
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
		if err := h.saveModerationPolicy(body); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})
}
