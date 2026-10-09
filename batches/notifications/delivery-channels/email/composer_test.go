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
		Completed: true,
	}}
	payload := buildComposePayload("en", "Taskly", items)
	item := payload.Items[0]
	if item["when_label"] != "9/22 by morning" || item["body"] != "Due: 9/22 by morning\n" {
		t.Fatalf("unexpected English task: %#v", item)
	}
	if item["completed"] != true {
		t.Fatalf("completed state was not preserved: %#v", item)
	}
	badges := item["badges"].([]map[string]any)
	if badges[0]["label"] != "Submission required" {
		t.Fatalf("unexpected English badges: %#v", badges)
	}
}

func TestBuildComposePayloadPreservesJapaneseAndOptionalFields(t *testing.T) {
	items := []dcCommon.QueueItem{
		{
			WhenLabel: "9/22 2限まで", Subject: "数学", Title: "宿題", Body: "締切: 9/22 2限まで\n",
			Meta:   sql.NullString{String: `{"task_id":1}`, Valid: true},
			Badges: sql.NullString{String: `[{"label":"小テスト"},{"label":"unknown"}]`, Valid: true},
		},
		{Badges: sql.NullString{String: "not-json", Valid: true}},
	}
	payload := buildComposePayload("ja", "Taskly", items)
	if len(payload.Items) != 2 || payload.Items[0]["when_label"] != "9/22 2限まで" || payload.Items[0]["meta"] != `{"task_id":1}` {
		t.Fatalf("unexpected Japanese payload: %#v", payload)
	}
	if _, ok := payload.Items[1]["badges"]; ok {
		t.Fatalf("invalid badges should be omitted: %#v", payload.Items[1])
	}
}

func TestBuildComposePayloadTranslatesEveryBadge(t *testing.T) {
	labels := []string{"オンライン", "対面", "小テスト", "発表", "自主学習", "提出必須", "自己採点", "unknown"}
	badges := "["
	for i, label := range labels {
		if i > 0 {
			badges += ","
		}
		badges += `{"label":"` + label + `"}`
	}
	badges += "]"
	payload := buildComposePayload("en", "Taskly", []dcCommon.QueueItem{{Badges: sql.NullString{String: badges, Valid: true}}})
	got := payload.Items[0]["badges"].([]map[string]any)
	want := []string{"Online", "In person", "Quiz", "Presentation", "Self study", "Submission required", "Self check", "unknown"}
	for i := range want {
		if got[i]["label"] != want[i] {
			t.Fatalf("badge %d = %#v, want %q", i, got[i]["label"], want[i])
		}
	}
}
