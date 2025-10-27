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
		eff := effectiveDeadlineExpr("")
		if overdue {
			dbq = dbq.Where(eff+" < ?", now)
		} else {
			dbq = dbq.Where(eff+" >= ?", now)
		}
		// Hide moderated content from non-admins
		if isAdmin, _ := userHasRole(u.ID, "admin"); !isAdmin {
			dbq = dbq.Where("is_hidden = 0")
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
				dbq = dbq.Where("("+eff+" < ?) OR ("+eff+" = ? AND id < ?)", cursorDeadline, cursorDeadline, cursorID)
			} else {
				dbq = dbq.Where("("+eff+" > ?) OR ("+eff+" = ? AND id > ?)", cursorDeadline, cursorDeadline, cursorID)
			}
		} else if cursorID != 0 {
			// Fallback if only id is present
			if overdue {
				dbq = dbq.Where("id < ?", cursorID)
			} else {
				dbq = dbq.Where("id > ?", cursorID)
			}
		}
		// Ordering by effective deadline
		if overdue {
			dbq = dbq.Order(eff + " DESC").Order("id DESC")
		} else {
			dbq = dbq.Order(eff + " ASC").Order("id ASC")
		}
		var items []Task
		if err := dbq.Limit(limit + 1).Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		var nextCursor map[string]string
		if len(items) > limit {
			last := items[limit-1]
			items = items[:limit]
			ed := effectiveDeadlineTime(last)
			nextCursor = map[string]string{"cursor_deadline": ed.Format(time.RFC3339), "cursor_id": fmt.Sprintf("%d", last.ID)}
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
			Type            *string `json:"type"`
			IsRequireHandIn *bool   `json:"is_require_hand_in"`
			Deadline        string  `json:"deadline"`       // YYYY-MM-DD, required
			DeadlineSince   *string `json:"deadline_since"` // ENUM: NONE,P1..P6
			DeadlineUntil   *string `json:"deadline_until"` // ENUM: NONE,MORNING,P1..P6
			SelfCheck       *bool   `json:"self_check_required"`
		}
		if err := c.BodyParser(&body); err != nil || body.Title == "" || body.SubjectID == 0 || strings.TrimSpace(body.Deadline) == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		// Parse deadline date
		dead, err := time.Parse("2006-01-02", body.Deadline)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid deadline"})
		}
		rec := Task{AddedUserID: u.ID, Title: body.Title, SubjectID: body.SubjectID, Description: body.Description, Deadline: dateOnly(dead), DeadlineSince: "NONE", DeadlineUntil: "NONE"}
		if body.Type != nil {
			rec.Type = normalizeTaskType(*body.Type)
		}
		if body.IsRequireHandIn != nil {
			rec.IsRequireHandIn = *body.IsRequireHandIn
		}
		if body.SelfCheck != nil {
			rec.SelfCheckRequired = *body.SelfCheck
		}
		if body.DeadlineSince != nil && strings.TrimSpace(*body.DeadlineSince) != "" {
			rec.DeadlineSince = strings.ToUpper(strings.TrimSpace(*body.DeadlineSince))
		}
		if body.DeadlineUntil != nil && strings.TrimSpace(*body.DeadlineUntil) != "" {
			rec.DeadlineUntil = strings.ToUpper(strings.TrimSpace(*body.DeadlineUntil))
		}
		if err := DB.Create(&rec).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		// Asynchronous Teams notification via Outlook/Power Automate email
		go sendTeamsNotifyEmailForTask(rec)
		// Async Discord notification
		go sendDiscordContent(fmt.Sprintf("[Task Created] id=%d title=%q by=%s", rec.ID, rec.Title, u.Email))
		return c.Status(201).JSON(rec)
	})

	app.Patch("/tasks/:id", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var rec Task
		if err := DB.First(&rec, c.Params("id")).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		// Keep a copy for change diff
		before := rec
		var body struct {
			Title           *string `json:"title"`
			SubjectID       *uint   `json:"subject_id"`
			Description     *string `json:"description"`
			Type            *string `json:"type"`
			IsRequireHandIn *bool   `json:"is_require_hand_in"`
			SelfCheck       *bool   `json:"self_check_required"`
			Deadline        *string `json:"deadline"`       // YYYY-MM-DD
			DeadlineSince   *string `json:"deadline_since"` // ENUM
			DeadlineUntil   *string `json:"deadline_until"` // ENUM
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
		if body.Type != nil {
			rec.Type = normalizeTaskType(*body.Type)
		}
		if body.IsRequireHandIn != nil {
			rec.IsRequireHandIn = *body.IsRequireHandIn
		}
		if body.SelfCheck != nil {
			rec.SelfCheckRequired = *body.SelfCheck
		}
		if body.Deadline != nil {
			if strings.TrimSpace(*body.Deadline) == "" {
				return c.Status(400).JSON(fiber.Map{"error": "deadline required"})
			}
			if t, err := time.Parse("2006-01-02", *body.Deadline); err == nil {
				rec.Deadline = dateOnly(t)
			} else {
				return c.Status(400).JSON(fiber.Map{"error": "invalid deadline"})
			}
		}
		if body.DeadlineSince != nil {
			rec.DeadlineSince = strings.ToUpper(strings.TrimSpace(*body.DeadlineSince))
		}
		if body.DeadlineUntil != nil {
			rec.DeadlineUntil = strings.ToUpper(strings.TrimSpace(*body.DeadlineUntil))
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
		// Fire Teams reply for update
		if mid, ok := latestTeamsMessageIDForTask(rec.ID); ok {
			msg := buildTaskUpdateMessageJa(before, rec)
			go sendTeamsReplyEmail(rec.ID, mid, msg)
		}
		// Async Discord notification
		go sendDiscordContent(fmt.Sprintf("[Task Updated] id=%d title=%q by=%s", rec.ID, rec.Title, u.Email))
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
		// Fire Teams reply for delete
		if mid, ok := latestTeamsMessageIDForTask(rec.ID); ok {
			msg := buildTaskDeleteMessageJa(rec)
			go sendTeamsReplyEmail(rec.ID, mid, msg)
		}
		// Async Discord notification
		go sendDiscordContent(fmt.Sprintf("[Task Deleted] id=%d title=%q by=%s", rec.ID, rec.Title, u.Email))
		return c.SendStatus(204)
	})
}
