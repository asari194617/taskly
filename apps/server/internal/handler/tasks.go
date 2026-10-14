package handler

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/jobs"
	"github.com/asari194617/taskly/apps/server/internal/taskimage"
	"github.com/asari194617/taskly/apps/server/internal/tasklist"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type sqlCountingLogger struct {
	gormlogger.Interface
	count *int
}

func (l sqlCountingLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	return sqlCountingLogger{Interface: l.Interface.LogMode(level), count: l.count}
}

func (l sqlCountingLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.count != nil {
		*l.count++
	}
	l.Interface.Trace(ctx, begin, fc, err)
}

func (h *Handler) RegisterTasksRoutes(router fiber.Router) {
	// Tasks CRUD with permissions
	router.Get("/", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		sqlCount := 0
		requestDB := h.db
		if strings.ToLower(strings.TrimSpace(utils.Env("TASKS_SQL_DEBUG", ""))) == "true" {
			requestDB = h.db.Session(&gorm.Session{Logger: sqlCountingLogger{Interface: h.db.Config.Logger, count: &sqlCount}.LogMode(gormlogger.Info)})
			defer func() {
				log.Printf("[sql-count] %s %s total=%d", c.Method(), c.Path(), sqlCount)
			}()
		}
		// Permission: read tasks (default allow)
		if ok, err := utils.CheckPermission(u.ID, "tasks", "read", nil, requestDB); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "perm"})
		} else if !ok {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}

		limit, _ := strconv.Atoi(c.Query("limit", "20"))
		var ownerID *uint
		mcpRequest := c.Locals("mcpToken") == true
		if mcpRequest {
			ownerID = &u.ID
		}
		offset, _ := strconv.Atoi(c.Query("offset", "0"))
		items, nextCursor, err := tasklist.List(requestDB, u.ID, tasklist.Options{
			Limit: limit, Overdue: c.Query("overdue") == "true", Search: c.Query("q"),
			CursorDeadline: c.Query("cursor_deadline"), CursorID: c.Query("cursor_id"), OwnerID: ownerID,
			MCP: mcpRequest, Offset: offset,
		})
		if err != nil {
			if strings.HasPrefix(err.Error(), "invalid cursor") || err.Error() == "cursor_id required" {
				return c.Status(400).JSON(fiber.Map{"error": err.Error()})
			}
			return c.Status(500).JSON(fiber.Map{"error": "db"})
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
			Title              string  `json:"title"`
			SubjectID          uint    `json:"subject_id"`
			Description        *string `json:"description"`
			ImageKey           *string `json:"image_key"`
			Type               *string `json:"type"`
			IsRequireHandIn    *bool   `json:"is_require_hand_in"`
			Deadline           string  `json:"deadline"`       // YYYY-MM-DD, required
			DeadlineSince      *string `json:"deadline_since"` // ENUM: NONE,P1..P6
			DeadlineUntil      *string `json:"deadline_until"` // ENUM: NONE,MORNING,P1..P6
			SelfCheck          *bool   `json:"is_self_check_required"`
			CompletionDisabled *bool   `json:"is_completion_disabled"`
		}
		if err := c.BodyParser(&body); err != nil || strings.TrimSpace(body.Title) == "" || body.SubjectID == 0 || strings.TrimSpace(body.Deadline) == "" {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		// Parse deadline date
		dead, err := time.Parse("2006-01-02", body.Deadline)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid deadline"})
		}
		rec := domain.Task{AddedUserID: u.ID, Title: body.Title, SubjectID: body.SubjectID, Description: body.Description, Deadline: utils.DateOnly(dead), DeadlineSince: "NONE", DeadlineUntil: "NONE"}
		if body.ImageKey != nil && strings.TrimSpace(*body.ImageKey) != "" {
			key := strings.TrimSpace(*body.ImageKey)
			images, imageErr := taskimage.NewFromEnv()
			if imageErr != nil {
				return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "image upload unavailable"})
			}
			if imageErr := images.ValidateUploaded(c.UserContext(), u.ID, key); imageErr != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid image upload"})
			}
			rec.ImageKey = &key
		}
		if rec.Type == "" {
			rec.Type = "OTHER"
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
		if body.DeadlineSince != nil && strings.TrimSpace(*body.DeadlineSince) != "" {
			rec.DeadlineSince = strings.ToUpper(strings.TrimSpace(*body.DeadlineSince))
		}
		if body.DeadlineUntil != nil && strings.TrimSpace(*body.DeadlineUntil) != "" {
			rec.DeadlineUntil = *body.DeadlineUntil
		}
		since, until, err := tasklist.NormalizeDeadlineWindow(rec.DeadlineSince, rec.DeadlineUntil)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		rec.DeadlineSince, rec.DeadlineUntil = since, until
		var completionDisabled []bool
		if body.CompletionDisabled != nil {
			completionDisabled = []bool{*body.CompletionDisabled}
		}
		recCreated, err := jobs.CreateTask(h.db, u.ID, body.Title, body.SubjectID, body.Description, rec.Type, rec.IsRequireHandIn, rec.IsSelfCheckRequired, rec, u.Email, completionDisabled...)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		return c.Status(201).JSON(recCreated)
	})

	router.Patch("/:id", func(c *fiber.Ctx) error {
		u := c.Locals("user").(*domain.User)
		var rec domain.Task
		if err := h.db.First(&rec, c.Params("id")).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		if c.Locals("mcpToken") == true && (rec.AddedUserID != u.ID || rec.IsHidden) {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		// Keep a copy for change diff
		var body struct {
			Title              *string `json:"title"`
			SubjectID          *uint   `json:"subject_id"`
			Description        *string `json:"description"`
			Type               *string `json:"type"`
			IsRequireHandIn    *bool   `json:"is_require_hand_in"`
			SelfCheck          *bool   `json:"is_self_check_required"`
			CompletionDisabled *bool   `json:"is_completion_disabled"`
			Deadline           *string `json:"deadline"`       // YYYY-MM-DD
			DeadlineSince      *string `json:"deadline_since"` // ENUM
			DeadlineUntil      *string `json:"deadline_until"` // ENUM
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if (body.Title != nil && strings.TrimSpace(*body.Title) == "") || (body.SubjectID != nil && *body.SubjectID == 0) {
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
			rec.DeadlineUntil = *body.DeadlineUntil
		}
		since, until, err := tasklist.NormalizeDeadlineWindow(rec.DeadlineSince, rec.DeadlineUntil)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		rec.DeadlineSince, rec.DeadlineUntil = since, until
		// enforce permission: update own or explicit
		if ok, err := tasklist.CanAccessTask(h.db, u.ID, "update", rec); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "perm"})
		} else if !ok {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		// perform DB update via domain service
		_, updated, err := jobs.UpdateTask(h.db, rec.ID, func(r *domain.Task) error {
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
			if body.CompletionDisabled != nil {
				r.IsCompletionDisabled = *body.CompletionDisabled
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
			r.DeadlineSince = rec.DeadlineSince
			r.DeadlineUntil = rec.DeadlineUntil
			return nil
		}, u.Email)
		if err != nil {
			if err.Error() == "deadline required" || err.Error() == "invalid deadline" {
				return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
			}
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
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
		if c.Locals("mcpToken") == true && (rec.AddedUserID != u.ID || rec.IsHidden) {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		allowed, err := tasklist.CanAccessTask(h.db, u.ID, "delete", rec)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "perm"})
		}
		if !allowed {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		imageKeys, imageKeysErr := taskimage.KeysForTask(h.db, rec.ID)
		if imageKeysErr != nil || len(imageKeys) == 0 {
			if rec.ImageKey != nil {
				imageKeys = []string{*rec.ImageKey}
			}
		}
		_, err = jobs.DeleteTask(h.db, rec.ID, u.Email)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "db"})
		}
		for _, key := range imageKeys {
			key := key
			deleteTaskImage(c.UserContext(), &key)
		}
		return c.SendStatus(204)
	})
}
