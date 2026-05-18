package main

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	utils "github.com/asari194617/taskly/apps/server/utils"
)

func registerAdminPermissionsRoutes(app *fiber.App) {
	// Admin: manage permissions
	app.Get("/admin/permissions", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		var items []utils.Permission
		if err := DB.Order("id DESC").Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.JSON(items)
	})
	app.Post("/admin/permissions", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		var body struct {
			UserID uint    `json:"user_id"`
			Target string  `json:"target"`
			Method string  `json:"method"`
			Value  *string `json:"value"`
		}
		if err := c.BodyParser(&body); err != nil || body.UserID == 0 || body.Target == "" || body.Method == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		rec := utils.Permission{UserID: body.UserID, Target: body.Target, Method: strings.ToLower(body.Method), Value: body.Value}
		if err := DB.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.Status(201).JSON(rec)
	})
	app.Delete("/admin/permissions/:id", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		if err := DB.Delete(&utils.Permission{}, c.Params("id")).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})
}
