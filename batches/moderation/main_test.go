package main

import (
	"testing"
)

func TestModerationUtilities(t *testing.T) {
	if got := hostPort("", ""); got != "localhost:3306" {
		t.Fatalf("hostPort = %q", got)
	}
	if got := hostPort("db", "3307"); got != "db:3307" {
		t.Fatalf("hostPort custom = %q", got)
	}
	if got := placeholders(3); got != "?,?,?" {
		t.Fatalf("placeholders = %q", got)
	}
	if got := placeholders(0); got != "" {
		t.Fatalf("empty placeholders = %q", got)
	}
	if min(2, 3) != 2 || min(4, 3) != 3 {
		t.Fatal("min returned wrong value")
	}
	if safeStr(nil) != "" {
		t.Fatal("nil string was not empty")
	}
	value := "details"
	if safeStr(&value) != value {
		t.Fatal("string pointer was not read")
	}
	if nullIfEmpty("  ") != nil {
		t.Fatal("blank string was not null")
	}
	if nullIfEmpty("value") != "value" {
		t.Fatal("value was changed")
	}
	if valueOr("", "fallback") != "fallback" || valueOr("value", "fallback") != "value" {
		t.Fatal("valueOr failed")
	}
}

func TestModerateTaskSkipsRemoteCallWithoutAPIKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	result, raw, model := moderateTask(Task{Title: "ordinary task"})
	if result != nil || raw != "" || model != "" {
		t.Fatalf("moderateTask = %#v, %q, %q", result, raw, model)
	}
}

func TestShouldAutoHide(t *testing.T) {
	policy := ModerationPolicy{DefaultAutoHide: "high", AutoHideByCat: map[string]string{"harassment": "medium"}}
	tests := []struct {
		name   string
		result *ModerationResult
		want   bool
	}{
		{"nil", nil, false},
		{"threat overrides severity", &ModerationResult{Threat: true, Severity: "low"}, true},
		{"blank severity", &ModerationResult{}, false},
		{"below default", &ModerationResult{Severity: "medium", Category: "other"}, false},
		{"at default", &ModerationResult{Severity: "high", Category: "other"}, true},
		{"category override", &ModerationResult{Severity: "medium", Category: " Harassment "}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldAutoHide(tc.result, policy); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSeverityRank(t *testing.T) {
	for input, want := range map[string]int{"low": 1, "medium": 2, "high": 3, "unknown": 0} {
		if got := rank(input); got != want {
			t.Errorf("rank(%q) = %d, want %d", input, got, want)
		}
	}
}
