package handler

import (
	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/gofiber/fiber/v2"
	"os"
	"strconv"
	"strings"
)

func (h *Handler) registerTeamsIntegrationRoutes(router fiber.Router) {
	// Callback endpoint for Power Automate to report posting result
	router.Post("/teams/callback", h.handleTeamsStatus(true))
	router.Post("/teams/status", h.handleTeamsStatus(false))

	router.Post("/teams/inbound", func(c *fiber.Ctx) error {
		if !hasTeamsIntegrationSecret(c) {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		var body struct {
			Email          string  `json:"email"`
			Title          string  `json:"title"`
			Description    *string `json:"description"`
			SubjectID      uint    `json:"subject_id"`
			StatusCallback *string `json:"status_callback"`
		}
		if err := c.BodyParser(&body); err != nil || body.Email == "" || body.Title == "" || body.SubjectID == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if body.StatusCallback != nil && *body.StatusCallback != "" {
			if err := validateTeamsCallbackURL(*body.StatusCallback, os.Getenv("TEAMS_CALLBACK_ALLOWED_ORIGINS")); err != nil {
				return c.Status(400).JSON(fiber.Map{"error": "invalid status_callback"})
			}
		}
		// Resolve user by email
		var u domain.User
		if err := h.db.Where("email = ?", body.Email).First(&u).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "user not found"})
		}
		// Enqueue job
		job := teamsJob{
			Title:          body.Title,
			Description:    body.Description,
			AddedUserID:    u.ID,
			SubjectID:      body.SubjectID,
			StatusCallback: body.StatusCallback,
		}
		if h.teams.Enqueue(job) {
			return c.SendStatus(fiber.StatusAccepted)
		}
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "queue unavailable"})
	})
}

type teamsStatusPayload struct {
	IsSuccess      bool
	TaskID         uint
	MessageID      string
	ConversationID string
}

func (h *Handler) handleTeamsStatus(strictBody bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !hasTeamsIntegrationSecret(c) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		var raw map[string]any
		if err := c.BodyParser(&raw); err != nil {
			if strictBody {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
			}
			return c.SendStatus(fiber.StatusOK)
		}
		payload := parseTeamsStatusPayload(raw)
		if payload.TaskID != 0 {
			_ = h.db.Table("tasks_teams_logs").Create(map[string]any{
				"task_id":               payload.TaskID,
				"is_success":            payload.IsSuccess,
				"teams_message_id":      optionalString(payload.MessageID),
				"teams_conversation_id": optionalString(payload.ConversationID),
				"last_status":           "callback_received",
			}).Error
		}
		return c.SendStatus(fiber.StatusOK)
	}
}

func parseTeamsStatusPayload(raw map[string]any) teamsStatusPayload {
	payload := teamsStatusPayload{}
	payload.IsSuccess, _ = raw["is_success"].(bool)
	switch taskID := raw["task_id"].(type) {
	case string:
		if value, err := strconv.ParseUint(strings.TrimSpace(taskID), 10, 64); err == nil {
			payload.TaskID = uint(value)
		}
	case float64:
		if taskID >= 0 {
			payload.TaskID = uint(taskID)
		}
	}
	payload.MessageID, _ = raw["message_id"].(string)
	payload.ConversationID, _ = raw["conversation_id"].(string)
	return payload
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
