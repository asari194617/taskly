package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

func registerIntegrationRoutes(app *fiber.App) {
	// Callback endpoint for Power Automate to report posting result
	app.Post("/integrations/teams/callback", func(c *fiber.Ctx) error {
		// verify HMAC signature (optional)
		body := c.Body()
		sig := c.Get("X-Signature")
		secret := env("TEAMS_STATUS_SIGNING_SECRET", "")
		if secret != "" {
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(body)
			expected := fmt.Sprintf("sha256=%x", mac.Sum(nil))
			if sig == "" || !hmac.Equal([]byte(sig), []byte(expected)) {
				return c.Status(401).JSON(fiber.Map{"error": "invalid signature"})
			}
		}
		// Accept task_id as string or number
		var raw map[string]any
		if err := c.BodyParser(&raw); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var p struct {
			IsSuccess      bool
			TaskID         uint
			MessageID      string
			ConversationID string
		}
		if v, ok := raw["is_success"].(bool); ok {
			p.IsSuccess = v
		}
		if tid, ok := raw["task_id"]; ok {
			switch tv := tid.(type) {
			case string:
				if n, err := strconv.ParseUint(strings.TrimSpace(tv), 10, 64); err == nil {
					p.TaskID = uint(n)
				}
			case float64:
				if tv >= 0 {
					p.TaskID = uint(tv)
				}
			default:
				// ignore
			}
		}
		if v, ok := raw["message_id"].(string); ok {
			p.MessageID = v
		}
		if v, ok := raw["conversation_id"].(string); ok {
			p.ConversationID = v
		}
		if p.TaskID != 0 {
			_ = DB.Table("tasks_teams_logs").Create(map[string]any{
				"task_id":    p.TaskID,
				"is_success": p.IsSuccess,
				"teams_message_id": func() *string {
					if p.MessageID == "" {
						return nil
					}
					v := p.MessageID
					return &v
				}(),
				"teams_conversation_id": func() *string {
					if p.ConversationID == "" {
						return nil
					}
					v := p.ConversationID
					return &v
				}(),
				"last_status": "callback_received",
			}).Error
		}
		return c.SendStatus(fiber.StatusOK)
	})
	app.Post("/integrations/teams/status", func(c *fiber.Ctx) error {
		// verify HMAC signature
		body := c.Body()
		sig := c.Get("X-Signature")
		secret := env("TEAMS_STATUS_SIGNING_SECRET", "")
		if secret != "" {
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(body)
			expected := fmt.Sprintf("sha256=%x", mac.Sum(nil))
			if sig == "" || !hmac.Equal([]byte(sig), []byte(expected)) {
				return c.Status(401).JSON(fiber.Map{"error": "invalid signature"})
			}
		}
		// store minimal log; accept task_id string or number
		var raw map[string]any
		_ = c.BodyParser(&raw)
		var p struct {
			IsSuccess      bool
			TaskID         uint
			MessageID      string
			ConversationID string
		}
		if v, ok := raw["is_success"].(bool); ok {
			p.IsSuccess = v
		}
		if tid, ok := raw["task_id"]; ok {
			switch tv := tid.(type) {
			case string:
				if n, err := strconv.ParseUint(strings.TrimSpace(tv), 10, 64); err == nil {
					p.TaskID = uint(n)
				}
			case float64:
				if tv >= 0 {
					p.TaskID = uint(tv)
				}
			}
		}
		if v, ok := raw["message_id"].(string); ok {
			p.MessageID = v
		}
		if v, ok := raw["conversation_id"].(string); ok {
			p.ConversationID = v
		}
		if p.TaskID != 0 {
			_ = DB.Table("tasks_teams_logs").Create(map[string]any{
				"task_id":    p.TaskID,
				"is_success": p.IsSuccess,
				"teams_message_id": func() *string {
					if p.MessageID == "" {
						return nil
					}
					v := p.MessageID
					return &v
				}(),
				"teams_conversation_id": func() *string {
					if p.ConversationID == "" {
						return nil
					}
					v := p.ConversationID
					return &v
				}(),
				"last_status": "callback_received",
			}).Error
		}
		return c.SendStatus(fiber.StatusOK)
	})

	// Teams inbound + async processing
	// Simple buffered channel acts as a queue for demo purposes
	var teamsQueue = make(chan map[string]any, 100)
	// Worker goroutine
	go func() {
		for job := range teamsQueue {
			// Extract data
			title, _ := job["title"].(string)
			desc, _ := job["description"].(*string)
			addedUserID, _ := job["added_user_id"].(uint)
			subjectID, _ := job["subject_id"].(uint)
			callback, _ := job["status_callback"].(*string)
			// Create task
			rec := Task{AddedUserID: addedUserID, Title: title, SubjectID: subjectID, Description: desc}
			var isSuccess bool
			if err := DB.Create(&rec).Error; err != nil {
				isSuccess = false
			} else {
				isSuccess = true
			}
			// Log into tasks_teams_logs (initial)
			_ = DB.Table("tasks_teams_logs").Create(map[string]any{
				"task_id":    rec.ID,
				"is_success": isSuccess,
				"callback_url": func() *string {
					if callback == nil {
						return nil
					}
					return callback
				}(),
				"attempts": 0,
				"last_status": func() string {
					if isSuccess {
						return "created"
					} else {
						return "create_failed"
					}
				}(),
			}).Error
			// Callback with retries if provided
			if callback != nil && *callback != "" {
				cbURL := *callback
				payload := fmt.Sprintf(`{"is_success":%t,"task_id":%d}`, isSuccess, rec.ID)
				secret := env("TEAMS_STATUS_SIGNING_SECRET", "")
				// HMAC signature
				var sig string
				if secret != "" {
					mac := hmac.New(sha256.New, []byte(secret))
					mac.Write([]byte(payload))
					sig = fmt.Sprintf("sha256=%x", mac.Sum(nil))
				}
				client := &http.Client{Timeout: 10 * time.Second}
				attempts := 0
				for attempts < 3 {
					attempts++
					req, _ := http.NewRequest("POST", cbURL, strings.NewReader(payload))
					req.Header.Set("Content-Type", "application/json")
					if sig != "" {
						req.Header.Set("X-Signature", sig)
					}
					resp, err := client.Do(req)
					if err != nil {
						_ = DB.Table("tasks_teams_logs").Where("task_id = ?", rec.ID).Updates(map[string]any{
							"attempts":    attempts,
							"last_status": "callback_error",
							"last_error":  err.Error(),
						}).Error
						time.Sleep(time.Duration(attempts) * time.Second)
						continue
					}
					bodyBytes := make([]byte, 0)
					if resp.Body != nil {
						bodyBytes, _ = io.ReadAll(resp.Body)
						_ = resp.Body.Close()
					}
					_ = DB.Table("tasks_teams_logs").Where("task_id = ?", rec.ID).Updates(map[string]any{
						"attempts":             attempts,
						"last_status":          "callback_sent",
						"last_response_status": resp.StatusCode,
						"last_response_body":   string(bodyBytes),
					}).Error
					if resp.StatusCode >= 200 && resp.StatusCode < 300 {
						break
					}
					time.Sleep(time.Duration(attempts) * time.Second)
				}
			}
		}
	}()

	app.Post("/integrations/teams/inbound", func(c *fiber.Ctx) error {
		// Verify shared secret header
		secret := c.Get("X-Teams-Secret")
		if secret == "" || secret != env("TEAMS_CALLBACK_ENDPOINT_SECRET", "change_me") {
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
		// Resolve user by email
		var u User
		if err := DB.Where("email = ?", body.Email).First(&u).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "user not found"})
		}
		// Enqueue job
		job := map[string]any{
			"title":         body.Title,
			"description":   body.Description,
			"added_user_id": u.ID,
			"subject_id":    body.SubjectID,
		}
		if body.StatusCallback != nil {
			job["status_callback"] = body.StatusCallback
		}
		select {
		case teamsQueue <- job:
			return c.SendStatus(202)
		default:
			return c.Status(503).JSON(fiber.Map{"error": "queue full"})
		}
	})

	// Inbound endpoint to mark day shift unfulfilled when an email about 日直 is received via Power Automate
	app.Post("/integrations/dayshift/inbound", func(c *fiber.Ctx) error {
		secret := c.Get("X-Taskly-Secret")
		if secret == "" || secret != env("DAYSHIFT_INBOUND_SECRET", "change_me") {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		var body struct {
			Title   string  `json:"title"`
			Content string  `json:"content"`
			Date    *string `json:"date"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if !strings.Contains(body.Title+" "+body.Content, "日直") {
			return c.Status(400).JSON(fiber.Map{"error": "not a dayshift notice"})
		}
		var target time.Time
		if body.Date != nil && *body.Date != "" {
			if t, err := time.Parse("2006-01-02", *body.Date); err == nil {
				target = t
			} else {
				target = dateOnly(time.Now())
			}
		} else {
			target = dateOnly(time.Now())
		}
		var asg DayShiftAssignment
		if err := DB.Where("date = ?", target.Format("2006-01-02")).First(&asg).Error; err != nil {
			// create if missing within current cycle if any
			cyc, _ := getCurrentCycle(target)
			asg = DayShiftAssignment{Date: dateOnly(target)}
			if cyc != nil {
				asg.CycleID = cyc.ID
			}
			asg.Status = "unfulfilled"
			if err := DB.Create(&asg).Error; err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "db"})
			}
		} else {
			asg.Status = "unfulfilled"
			_ = DB.Save(&asg).Error
		}
		return c.SendStatus(202)
	})
}
