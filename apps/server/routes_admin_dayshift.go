package main

import (
	"time"

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
		if err := c.BodyParser(&body); err != nil || body.BeginDate == "" || body.FinishDate == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		bd, err1 := time.Parse("2006-01-02", body.BeginDate)
		fd, err2 := time.Parse("2006-01-02", body.FinishDate)
		if err1 != nil || err2 != nil || !bd.Before(fd) {
			return c.Status(400).JSON(fiber.Map{"error": "invalid dates"})
		}
		rec := DayShiftCycle{BeginDate: bd, FinishDate: fd}
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
		if err := DB.Delete(&DayShiftCycle{}, c.Params("id")).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})

	// business calendar management (range updates)
	app.Post("/admin/calendar/upsert-range", authRequired, adminOnly, audit, func(c *fiber.Ctx) error {
		var body struct {
			Start      string  `json:"start"`
			End        string  `json:"end"`
			IsBusiness bool    `json:"is_business_day"`
			Notes      *string `json:"notes"`
		}
		if err := c.BodyParser(&body); err != nil || body.Start == "" || body.End == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		s, e := body.Start, body.End
		sd, err1 := time.Parse("2006-01-02", s)
		ed, err2 := time.Parse("2006-01-02", e)
		if err1 != nil || err2 != nil || sd.After(ed) {
			return c.Status(400).JSON(fiber.Map{"error": "invalid dates"})
		}
		for d := sd; !d.After(ed); d = d.Add(24 * time.Hour) {
			var bc BusinessCalendar
			if err := DB.Where("date = ?", d.Format("2006-01-02")).First(&bc).Error; err != nil {
				bc = BusinessCalendar{Date: dateOnly(d), IsBusinessDay: body.IsBusiness, Notes: body.Notes}
				_ = DB.Create(&bc).Error
			} else {
				bc.IsBusinessDay = body.IsBusiness
				bc.Notes = body.Notes
				_ = DB.Save(&bc).Error
			}
		}
		return c.SendStatus(204)
	})
}
