package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func discordEmbedsPayload(items []map[string]any) map[string]any {
	embeds := make([]map[string]any, 0, len(items))
	color := 0x3B82F6 // blue
	for _, it := range items {
		embeds = append(embeds, map[string]any{
			"title":       it["title"],
			"description": fmt.Sprintf("%s\n%s", it["when"], it["meta"]),
			"color":       color,
		})
	}
	return map[string]any{
		"content": "期限が近い課題のお知らせ",
		"embeds":  embeds,
	}
}

func postDiscord(webhook string, body map[string]any) error {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", webhook, strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	cli := &http.Client{Timeout: 10 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord webhook http %d", resp.StatusCode)
	}
	return nil
}
