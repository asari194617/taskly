package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

func registerTasksRoutes(app *fiber.App) {
	// Tasks CRUD with permissions
	app.Get("/tasks", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		// Permission: read tasks (default allow)
		if ok, err := checkPermission(u.ID, "tasks", "read", nil); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "perm"})
		} else if !ok {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}

		overdue := c.Query("overdue") == "true"
		qstr := strings.TrimSpace(c.Query("q"))
		limit, _ := strconv.Atoi(c.Query("limit", "20"))
		if limit <= 0 || limit > 100 {
			limit = 20
		}
		// Cursor based on deadline and id to ensure stable ordering
		cursorDeadlineStr := c.Query("cursor_deadline")
		cursorIDStr := c.Query("cursor_id")
		var cursorDeadline *time.Time
		var cursorID uint
		if cursorDeadlineStr != "" {
			if t, err := time.Parse(time.RFC3339, cursorDeadlineStr); err == nil {
				cursorDeadline = &t
			}
		}
		if cursorIDStr != "" {
			if v, err := strconv.Atoi(cursorIDStr); err == nil && v > 0 {
				cursorID = uint(v)
			}
		}

		// Build query
		dbq := DB.Model(&Task{})
		now := time.Now()
		if overdue {
			dbq = dbq.Where("deadline IS NOT NULL AND deadline < ?", now)
		} else {
			dbq = dbq.Where("deadline IS NULL OR deadline >= ?", now)
		}
		if qstr != "" {
			like := "%" + qstr + "%"
			dbq = dbq.Where("title LIKE ? OR description LIKE ?", like, like)
		}
		// Apply cursor for pagination
		// For non-overdue list (ascending by deadline NULLS LAST then id), we page after (deadline, id)
		// For overdue (descending), we page before (deadline, id)
		if cursorDeadline != nil {
			if overdue {
				dbq = dbq.Where("(deadline < ?) OR (deadline = ? AND id < ?)", cursorDeadline, cursorDeadline, cursorID)
			} else {
				dbq = dbq.Where("(deadline > ?) OR (deadline = ? AND id > ?)", cursorDeadline, cursorDeadline, cursorID)
			}
		} else if cursorID != 0 {
			// Fallback if only id is present
			if overdue {
				dbq = dbq.Where("id < ?", cursorID)
			} else {
				dbq = dbq.Where("id > ?", cursorID)
			}
		}
		// Ordering
		if overdue {
			dbq = dbq.Order("deadline DESC").Order("id DESC")
		} else {
			// deadline ASC with NULLS LAST: emulate by ordering by (deadline IS NULL) then deadline ASC
			dbq = dbq.Order("CASE WHEN deadline IS NULL THEN 1 ELSE 0 END ASC").Order("deadline ASC").Order("id ASC")
		}
		var items []Task
		if err := dbq.Limit(limit + 1).Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		var nextCursor map[string]string
		if len(items) > limit {
			last := items[limit-1]
			items = items[:limit]
			if last.Deadline != nil {
				nextCursor = map[string]string{
					"cursor_deadline": last.Deadline.Format(time.RFC3339),
					"cursor_id":       fmt.Sprintf("%d", last.ID),
				}
			} else {
				nextCursor = map[string]string{"cursor_id": fmt.Sprintf("%d", last.ID)}
			}
		}
		return c.JSON(fiber.Map{"items": items, "next_cursor": nextCursor})
	})

	app.Post("/tasks", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		if ok, err := checkPermission(u.ID, "tasks", "create", nil); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "perm"})
		} else if !ok {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		var body struct {
			Title           string  `json:"title"`
			SubjectID       uint    `json:"subject_id"`
			Description     *string `json:"description"`
			IsRequireHandIn *bool   `json:"is_require_hand_in"`
			Deadline        *string `json:"deadline"`
		}
		if err := c.BodyParser(&body); err != nil || body.Title == "" || body.SubjectID == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var deadlinePtr *time.Time
		if body.Deadline != nil && *body.Deadline != "" {
			if t, err := time.Parse(time.RFC3339, *body.Deadline); err == nil {
				deadlinePtr = &t
			} else {
				return c.Status(400).JSON(fiber.Map{"error": "invalid deadline"})
			}
		}
		rec := Task{AddedUserID: u.ID, Title: body.Title, SubjectID: body.SubjectID, Description: body.Description, Deadline: deadlinePtr}
		if body.IsRequireHandIn != nil {
			rec.IsRequireHandIn = *body.IsRequireHandIn
		}
		if err := DB.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.Status(201).JSON(rec)
	})

	app.Patch("/tasks/:id", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var rec Task
		if err := DB.First(&rec, c.Params("id")).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		var body struct {
			Title           *string `json:"title"`
			SubjectID       *uint   `json:"subject_id"`
			Description     *string `json:"description"`
			IsRequireHandIn *bool   `json:"is_require_hand_in"`
			Deadline        *string `json:"deadline"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if body.Title != nil {
			rec.Title = *body.Title
		}
		if body.SubjectID != nil {
			rec.SubjectID = *body.SubjectID
		}
		if body.Description != nil {
			rec.Description = body.Description
		}
		if body.IsRequireHandIn != nil {
			rec.IsRequireHandIn = *body.IsRequireHandIn
		}
		if body.Deadline != nil {
			if *body.Deadline == "" {
				rec.Deadline = nil
			} else if t, err := time.Parse(time.RFC3339, *body.Deadline); err == nil {
				rec.Deadline = &t
			} else {
				return c.Status(400).JSON(fiber.Map{"error": "invalid deadline"})
			}
		}
		// enforce permission: update own or explicit
		if ok, err := checkPermission(u.ID, "tasks", "update", &rec.AddedUserID); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "perm"})
		} else if !ok {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		if err := DB.Save(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.JSON(rec)
	})

	app.Delete("/tasks/:id", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		if _, err := strconv.Atoi(c.Params("id")); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
		}
		var rec Task
		if err := DB.First(&rec, c.Params("id")).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		// Admin/manager fallback, otherwise require delete permission
		isAdmin, _ := userHasRole(u.ID, "admin")
		isManager, _ := userHasRole(u.ID, "manager")
		allowed := isAdmin || isManager
		if !allowed {
			if ok, err := checkPermission(u.ID, "tasks", "delete", &rec.AddedUserID); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "perm"})
			} else {
				allowed = ok
			}
		}
		if !allowed {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		if err := DB.Delete(&Task{}, rec.ID).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.SendStatus(204)
	})
}
