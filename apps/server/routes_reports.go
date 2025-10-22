package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

func registerReportRoutes(app *fiber.App) {
	app.Post("/reports", authRequired, audit, func(c *fiber.Ctx) error {
		u := c.Locals("user").(*User)
		var body struct {
			TaskID  uint    `json:"task_id"`
			Reason  string  `json:"reason"`
			Comment *string `json:"comment"`
		}
		if err := parseJSONStrict(c, &body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid_json"})
		}
		body.Reason = strings.TrimSpace(body.Reason)
		if body.TaskID == 0 || body.Reason == "" || len(body.Reason) > 255 {
			return validationError(c, map[string]string{"task_id": "required", "reason": "required,max=255"})
		}
		rec := UserReport{ReporterUserID: &u.ID, TaskID: body.TaskID, Reason: body.Reason, Comment: body.Comment, CreatedAt: time.Now()}
		if err := DB.Create(&rec).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "db"})
		}
		// Fire-and-forget Discord notification if configured
		go func(taskID uint, reason string) {
			hook := env("DISCORD_WEBHOOK_URL", "")
			if strings.TrimSpace(hook) == "" {
				return
			}
			payload := map[string]any{
				"content": "[Report] TaskID: " + fmt.Sprintf("%d", taskID) + " Reason: " + reason,
			}
			b, _ := json.Marshal(payload)
			req, _ := http.NewRequest("POST", hook, strings.NewReader(string(b)))
			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{Timeout: 5 * time.Second}
			if resp, err := client.Do(req); err == nil && resp != nil {
				_ = resp.Body.Close()
			}
		}(body.TaskID, body.Reason)
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": rec.ID})
	})
}
