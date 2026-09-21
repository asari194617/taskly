package handler

import (
	"fmt"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	"github.com/getsentry/sentry-go"
	"github.com/gofiber/fiber/v2"
	"strings"
	"time"
)

func (h *Handler) registerDayShiftIntegrationRoutes(router fiber.Router) {
	// Inbound day shift skip/rotation endpoint (Teams → Taskly)
	// This endpoint is called via Power Automate when a teacher sends a
	// "日直スキップ・延期" に関するメール。subject/body を Gemini で解析し、
	// 実際にスキップ・連続させるかを決定する。
	router.Post("/dayshift/inbound", func(c *fiber.Ctx) error {
		if !hasTeamsIntegrationSecret(c) {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		var body struct {
			Subject string `json:"subject"`
			Body    string `json:"body"`
		}
		if err := utils.ParseJSONStrict(c, &body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		joined := strings.ToLower(body.Subject + "\n" + body.Body)
		if !strings.Contains(joined, "日直") {
			return c.Status(400).JSON(fiber.Map{"error": "missing 日直 keyword"})
		}

		// Decide whether "skip/rotation" is requested using Gemini+heuristic.
		if !utils.ShouldRotateByGemini(body.Subject, body.Body) {
			return c.Status(202).JSON(fiber.Map{"ok": true, "rotated": false})
		}

		// At this point, we accepted a valid day-shift skip request; log to
		// Discord for traceability regardless of today/tomorrow mode.
		go func() {
			fields := []map[string]any{
				{"name": "ReceivedAt", "value": time.Now().UTC().Format(time.RFC3339), "inline": true},
				{"name": "Subject", "value": body.Subject, "inline": false},
			}
			utils.SendDiscordEmbeds([]map[string]any{{
				"title":       "Day Shift Continue Request Accepted",
				"description": "日直継続リクエストを受理しました。",
				"color":       0x1ABC9C,
				"fields":      fields,
				"timestamp":   time.Now().UTC().Format(time.RFC3339),
			}})
		}()

		// Simple heuristic to distinguish "today" vs "tomorrow". This keeps
		// ShouldRotateByGemini focused on yes/no while we infer timing here.
		mode := "today"
		if strings.Contains(joined, "明日") || strings.Contains(joined, "あした") || strings.Contains(joined, "翌日") {
			mode = "tomorrow"
		}

		now := time.Now()
		today := now
		tomorrow := now.Add(24 * time.Hour)
		if mode == "tomorrow" {
			// Tomorrow-only skip: set flag so that the next business day's index
			// increment is skipped by the Airflow DAG.
			if err := utils.SetConfigString("day_shift_index_increment_skip", "1", h.db); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "failed to set skip flag"})
			}
			return c.Status(202).JSON(fiber.Map{"ok": true, "rotated": true, "mode": "tomorrow"})
		}

		// Today+tomorrow continuous duty:
		// 1) Decrement index by 1.
		idx, err := utils.GetConfigInt("day_shift_current_index", h.db)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to read index"})
		}
		before := idx
		newIdx := idx - 1
		if newIdx < 0 {
			newIdx = 0
		}
		if err := utils.SetConfigString("day_shift_current_index", fmt.Sprintf("%d", newIdx), h.db); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to update index"})
		}

		// 2) Resolve today and next-day members based on index and cycle.
		todayMembers, nextMembers, err := utils.ResolveDayShiftTodayAndNextMembers(today, before, h.db)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to resolve members"})
		}

		// 3) Compose and send email via email-compose gRPC.
		if err := utils.SendDayShiftSkipEmail(c.Context(), today, tomorrow, todayMembers, nextMembers); err != nil {
			// Non-fatal: log to Sentry and still return 202 to avoid retries storm
			sentry.CaptureException(err)
		}

		// 4) Notify Discord for observability.
		go func() {
			fields := []map[string]any{
				{"name": "Today", "value": today.Format("2006-01-02"), "inline": true},
				{"name": "Index(before)", "value": fmt.Sprintf("%d", before), "inline": true},
				{"name": "Index(after)", "value": fmt.Sprintf("%d", newIdx), "inline": true},
			}
			if len(todayMembers) > 0 {
				fields = append(fields, map[string]any{"name": "Today Members", "value": utils.JoinUserNames(todayMembers), "inline": false})
			}
			if len(nextMembers) > 0 {
				fields = append(fields, map[string]any{"name": "Next Members", "value": utils.JoinUserNames(nextMembers), "inline": false})
			}
			utils.SendDiscordEmbeds([]map[string]any{{
				"title":       "Day Shift Continuous Duty (Today+Next)",
				"description": body.Subject,
				"color":       0x3498DB,
				"fields":      fields,
				"timestamp":   time.Now().UTC().Format(time.RFC3339),
			}})
		}()

		return c.Status(202).JSON(fiber.Map{"ok": true, "rotated": true, "mode": "today"})
	})
}
