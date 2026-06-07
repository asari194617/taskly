package handler

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
)

func (h *Handler) RegisterTasksRoutes(router fiber.Router) {
	// Tasks CRUD with permissions
	router.Get("/", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		// Permission: read tasks (default allow)
		if ok, err := utils.CheckPermission(u.ID, "tasks", "read", nil, h.db); err != nil {
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
			// cursor_deadline is date-only (YYYY-MM-DD)
			if t, err := time.Parse("2006-01-02", cursorDeadlineStr); err == nil {
				td := utils.DateOnly(t)
				cursorDeadline = &td
			}
		}
		if cursorIDStr != "" {
			if v, err := strconv.Atoi(cursorIDStr); err == nil && v > 0 {
				cursorID = uint(v)
			}
		}

		// Build query
		dbq := h.db.Model(&domain.Task{})
		// compare using date-only to keep ordering/filters consistent with stored Deadline (date-only)
		now := utils.DateOnly(time.Now())
		eff := utils.EffectiveDeadlineExpr("")
		if overdue {
			dbq = dbq.Where(eff+" < ?", now)
		} else {
			dbq = dbq.Where(eff+" >= ?", now)
		}
		// Hide moderated content from non-admin/manager
		isAdmin, _ := utils.UserHasRole(u.ID, "admin", h.db)
		isManager, _ := utils.UserHasRole(u.ID, "manager", h.db)
		if !(isAdmin || isManager) {
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
		var items []domain.Task
		if err := dbq.Limit(limit + 1).Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		var nextCursor map[string]string
		if len(items) > limit {
			last := items[limit-1]
			items = items[:limit]
			// return date-only cursor (YYYY-MM-DD)
			nextCursor = map[string]string{"cursor_deadline": last.Deadline.Format("2006-01-02"), "cursor_id": fmt.Sprintf("%d", last.ID)}
		}
		return c.JSON(fiber.Map{"items": items, "next_cursor": nextCursor})
	})

	router.Post("/", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		if ok, err := utils.CheckPermission(u.ID, "tasks", "create", nil, h.db); err != nil {
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
			SelfCheck       *bool   `json:"is_self_check_required"`
		}
		if err := c.BodyParser(&body); err != nil || body.Title == "" || body.SubjectID == 0 || strings.TrimSpace(body.Deadline) == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		// Parse deadline date
		dead, err := time.Parse("2006-01-02", body.Deadline)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid deadline"})
		}
		rec := domain.Task{AddedUserID: u.ID, Title: body.Title, SubjectID: body.SubjectID, Description: body.Description, Deadline: utils.DateOnly(dead), DeadlineSince: "NONE", DeadlineUntil: "NONE"}
		if body.Type != nil {
			rec.Type = utils.NormalizeTaskType(*body.Type)
		}
		if body.IsRequireHandIn != nil {
			rec.IsRequireHandIn = *body.IsRequireHandIn
		}
		if body.SelfCheck != nil {
			rec.IsSelfCheckRequired = *body.SelfCheck
		}
		if body.DeadlineSince != nil && strings.TrimSpace(*body.DeadlineSince) != "" {
			rec.DeadlineSince = strings.ToUpper(strings.TrimSpace(*body.DeadlineSince))
		}
		if body.DeadlineUntil != nil && strings.TrimSpace(*body.DeadlineUntil) != "" {
			rec.DeadlineUntil = strings.ToUpper(strings.TrimSpace(*body.DeadlineUntil))
		}
		recCreated, err := domain.CreateTask(h.db, u.ID, body.Title, body.SubjectID, body.Description, rec.Type, rec.IsRequireHandIn, rec.IsSelfCheckRequired, rec.Deadline, rec.DeadlineSince, rec.DeadlineUntil, u.Email)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		// Asynchronous Teams notification via Outlook/Power Automate email
		go utils.SendTeamsNotifyEmailForTask(recCreated, h.db)
		// Async Discord notification (embed)
		go utils.SendDiscordTaskEmbed("created", recCreated, u.Email)
		return c.Status(201).JSON(recCreated)
	})

	router.Patch("/:id", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var rec domain.Task
		if err := h.db.First(&rec, c.Params("id")).Error; err != nil {
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
			SelfCheck       *bool   `json:"is_self_check_required"`
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
			rec.Type = utils.NormalizeTaskType(*body.Type)
		}
		if body.IsRequireHandIn != nil {
			rec.IsRequireHandIn = *body.IsRequireHandIn
		}
		if body.SelfCheck != nil {
			rec.IsSelfCheckRequired = *body.SelfCheck
		}
		if body.Deadline != nil {
			if strings.TrimSpace(*body.Deadline) == "" {
				return c.Status(400).JSON(fiber.Map{"error": "deadline required"})
			}
			if t, err := time.Parse("2006-01-02", *body.Deadline); err == nil {
				rec.Deadline = utils.DateOnly(t)
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
		if ok, err := utils.CheckPermission(u.ID, "tasks", "update", &rec.AddedUserID, h.db); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "perm"})
		} else if !ok {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		// perform DB update via domain service
		_, updated, err := domain.UpdateTask(h.db, rec.ID, func(r *domain.Task) error {
			if body.Title != nil {
				r.Title = *body.Title
			}
			if body.SubjectID != nil {
				r.SubjectID = *body.SubjectID
			}
			if body.Description != nil {
				r.Description = body.Description
			}
			if body.Type != nil {
				r.Type = utils.NormalizeTaskType(*body.Type)
			}
			if body.IsRequireHandIn != nil {
				r.IsRequireHandIn = *body.IsRequireHandIn
			}
			if body.SelfCheck != nil {
				r.IsSelfCheckRequired = *body.SelfCheck
			}
			if body.Deadline != nil {
				if strings.TrimSpace(*body.Deadline) == "" {
					return fmt.Errorf("deadline required")
				}
				if t, err := time.Parse("2006-01-02", *body.Deadline); err == nil {
					r.Deadline = utils.DateOnly(t)
				} else {
					return fmt.Errorf("invalid deadline")
				}
			}
			if body.DeadlineSince != nil {
				r.DeadlineSince = strings.ToUpper(strings.TrimSpace(*body.DeadlineSince))
			}
			if body.DeadlineUntil != nil {
				r.DeadlineUntil = strings.ToUpper(strings.TrimSpace(*body.DeadlineUntil))
			}
			return nil
		}, u.Email)
		if err != nil {
			if err.Error() == "deadline required" || err.Error() == "invalid deadline" {
				return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
			}
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		// Fire Teams reply for update
		if mid, ok := utils.LatestTeamsMessageIDForTask(updated.ID, h.db); ok {
			msg := utils.BuildTaskUpdateMessageJa(before, updated, h.db)
			go utils.SendTeamsReplyEmail(updated.ID, mid, msg)
		}
		// Async Discord notification (embed)
		go utils.SendDiscordTaskEmbed("updated", updated, u.Email)
		return c.JSON(updated)
	})

	router.Delete("/:id", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		if _, err := strconv.Atoi(c.Params("id")); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
		}
		var rec domain.Task
		if err := h.db.First(&rec, c.Params("id")).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		// Admin/manager fallback, otherwise require delete permission
		isAdmin, _ := utils.UserHasRole(u.ID, "admin", h.db)
		isManager, _ := utils.UserHasRole(u.ID, "manager", h.db)
		allowed := isAdmin || isManager
		if !allowed {
			if ok, err := utils.CheckPermission(u.ID, "tasks", "delete", &rec.AddedUserID, h.db); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "perm"})
			} else {
				allowed = ok
			}
		}
		if !allowed {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		deleted, err := domain.DeleteTask(h.db, rec.ID, u.Email)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		// Fire Teams reply for delete
		if mid, ok := utils.LatestTeamsMessageIDForTask(deleted.ID, h.db); ok {
			msg := utils.BuildTaskDeleteMessageJa(deleted)
			go utils.SendTeamsReplyEmail(deleted.ID, mid, msg)
		}
		// Async Discord notification (embed)
		go utils.SendDiscordTaskEmbed("deleted", deleted, u.Email)
		return c.SendStatus(204)
	})
}
