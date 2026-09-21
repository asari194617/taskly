package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/asari194617/taskly/apps/server/internal/domain"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Posts one or more embeds to the Discord webhook. Logs basic diagnostics.
func SendDiscordEmbeds(embeds []map[string]any) {
	hook := Env("DISCORD_WEBHOOK_URL", "")
	if strings.TrimSpace(hook) == "" {
		log.Printf("discord: skipped (no DISCORD_WEBHOOK_URL)")
		return
	}
	if len(embeds) == 0 {
		log.Printf("discord: skipped (no embeds)")
		return
	}
	payloadObj := map[string]any{"embeds": embeds}
	b, err := json.Marshal(payloadObj)
	if err != nil {
		log.Printf("discord: marshal error: %v", err)
		return
	}
	req, err := http.NewRequest("POST", hook, bytes.NewReader(b))
	if err != nil {
		log.Printf("discord: build request error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "taskly-server/1.0")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("discord: post error: %v", err)
		return
	}
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("discord: non-2xx status: %d", resp.StatusCode)
		return
	}
	log.Printf("discord: embeds sent ok")
}

// SendDiscordTaskEmbed builds and sends an embed for task events (created/updated/deleted)
func SendDiscordTaskEmbed(action string, t domain.Task, actor string) {
	// Prepare fields
	webURL := strings.TrimSpace(Env("WEB_URL", ""))
	title := "Task"
	color := 5814783 // default bluish
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "created", "create":
		title = "Task Created"
		color = 0x2ecc71 // green
	case "updated", "update":
		title = "Task Updated"
		color = 0xf1c40f // yellow
	case "deleted", "delete":
		title = "Task Deleted"
		color = 0xe74c3c // red
	}
	// Human-readable label for since/until to avoid NONE/NONE display
	suLabel := deriveDeadlineLabelEnum(t.DeadlineSince, t.DeadlineUntil)
	if strings.TrimSpace(suLabel) == "" {
		suLabel = "指定なし"
	}
	fields := []map[string]any{
		{"name": "ID", "value": fmt.Sprintf("%d", t.ID), "inline": true},
		{"name": "Deadline", "value": t.Deadline.Format("2006-01-02"), "inline": true},
		{"name": "Window", "value": suLabel, "inline": true},
		{"name": "By", "value": actor, "inline": true},
	}
	// Optional link to web app (no specific task page, so link to home if provided)
	embed := map[string]any{
		"title":       title,
		"description": t.Title,
		"color":       color,
		"fields":      fields,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	}
	if webURL != "" {
		embed["url"] = strings.TrimRight(webURL, "/") + "/home"
	}
	SendDiscordEmbeds([]map[string]any{embed})
}
