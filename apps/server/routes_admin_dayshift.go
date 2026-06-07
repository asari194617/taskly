package main

import (
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
)

func registerAdminDayShiftRoutes(app *fiber.App) {
	// Create cycle
	app.Post("/admin/dayshift/cycles", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		var body struct {
			BeginDate   string  `json:"begin_date"`
			FinishDate  string  `json:"finish_date"`
			Description *string `json:"description"`
		}
		if err := utils.ParseJSONStrict(c, &body); err != nil || body.BeginDate == "" || body.FinishDate == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		bd, err1 := time.Parse("2006-01-02", body.BeginDate)
		fd, err2 := time.Parse("2006-01-02", body.FinishDate)
		if err1 != nil || err2 != nil || !bd.Before(fd) {
			return c.Status(400).JSON(fiber.Map{"error": "invalid dates"})
		}
		rec := domain.DayShiftCycle{BeginDate: bd, FinishDate: fd}
		if body.Description != nil {
			rec.Description = *body.Description
		}
		if err := DB.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.Status(201).JSON(rec)
	})

	// Delete cycle
	app.Delete("/admin/dayshift/cycles/:id", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		if err := DB.Delete(&domain.DayShiftCycle{}, c.Params("id")).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// Business calendar feature removed
}
