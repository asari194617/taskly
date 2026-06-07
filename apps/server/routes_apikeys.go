package main

import (
	"strconv"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
)

func registerAPIKeyRoutes(app *fiber.App) {
	// List API keys for current user
	app.Get("/apikeys", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var items []domain.APIKey
		if err := DB.Where("user_id = ?", u.ID).Order("id DESC").Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		out := make([]map[string]any, 0, len(items))
		for _, it := range items {
			out = append(out, map[string]any{
				"id":          it.ID,
				"description": it.Description,
				"is_valid":    it.IsValid,
				"created_at":  it.CreatedAt,
			})
		}
		return c.JSON(out)
	})

	// Create new API key and return the plaintext token once
	app.Post("/apikeys", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		// limit from env, -1 for unlimited
		limit := -1
		if s := utils.Env("API_KEYS_LIMIT", "-1"); s != "" {
			if v, err := strconv.Atoi(s); err == nil {
				limit = v
			}
		}
		if limit >= 0 { // -1 means unlimited
			var cur int64
			if err := DB.Model(&domain.APIKey{}).Where("user_id = ? AND is_valid = 1", u.ID).Count(&cur).Error; err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "db"})
			}
			if int(cur) >= limit {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "api key limit reached"})
			}
		}
		var body struct {
			Description *string `json:"description"`
		}
		_ = c.BodyParser(&body)
		plain, hashed, genErr := utils.GenerateAPIKey()
		if genErr != nil {
			return c.Status(500).JSON(fiber.Map{"error": "gen"})
		}
		rec := domain.APIKey{UserID: u.ID, HashedToken: hashed, Description: body.Description}
		if err := DB.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": rec.ID, "token": plain})
	})

	// Patch API key
	app.Patch("/apikeys/:id", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var rec domain.APIKey
		if err := DB.Where("id = ? AND user_id = ?", c.Params("id"), u.ID).First(&rec).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
		}
		var body struct {
			IsValid     *bool   `json:"is_valid"`
			Description *string `json:"description"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
		}
		if body.IsValid != nil {
			rec.IsValid = *body.IsValid
		}
		if body.Description != nil {
			rec.Description = body.Description
		}
		if err := DB.Save(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.JSON(fiber.Map{"id": rec.ID, "is_valid": rec.IsValid})
	})
}
