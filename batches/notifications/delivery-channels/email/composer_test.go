package main

import (
	"database/sql"
	"testing"

	dcCommon "github.com/asari194617/taskly/batches/notifications/delivery-channels/common"
)

func TestEnglishDeadlineLabel(t *testing.T) {
	tests := map[string]string{
		"9/22 朝まで":    "9/22 by morning",
		"9/22 昼休みまで":  "9/22 by lunch",
		"9/22 放課後まで":  "9/22 after school",
		"9/22 2限まで":   "9/22 by period 2",
		"9/22 2〜3限まで": "9/22 by periods 2–3",
		"9/22 まで":     "9/22 due",
	}
	for input, want := range tests {
		if got := englishDeadlineLabel(input); got != want {
			t.Errorf("englishDeadlineLabel(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestBuildComposePayloadEnglish(t *testing.T) {
	items := []dcCommon.QueueItem{{
		WhenLabel: "9/22 朝まで",
		Body:      "締切: 9/22 朝まで\n",
		Badges:    sql.NullString{String: `[{"label":"提出必須","variant":"red"}]`, Valid: true},
	}}
	payload := buildComposePayload("en", "Taskly", items)
	item := payload.Items[0]
	if item["when_label"] != "9/22 by morning" || item["body"] != "Due: 9/22 by morning\n" {
		t.Fatalf("unexpected English task: %#v", item)
	}
	badges := item["badges"].([]map[string]any)
	if badges[0]["label"] != "Submission required" {
		t.Fatalf("unexpected English badges: %#v", badges)
	}
}
