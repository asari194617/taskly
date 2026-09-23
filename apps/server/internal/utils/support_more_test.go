package utils

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/gofiber/fiber/v2"
)

func TestValidationHelpers(t *testing.T) {
	t.Setenv("TASKLY_TEST_ENV", "configured")
	if got := Env("TASKLY_TEST_ENV", "fallback"); got != "configured" {
		t.Fatalf("Env configured = %q", got)
	}
	t.Setenv("TASKLY_TEST_ENV", "")
	if got := Env("TASKLY_TEST_ENV", "fallback"); got != "fallback" {
		t.Fatalf("Env fallback = %q", got)
	}

	valid := []string{"person@example.test", "Name <person@example.test>"}
	for _, value := range valid {
		if !IsEmail(value) {
			t.Errorf("IsEmail(%q) = false", value)
		}
	}
	invalid := []string{"", "not-an-email", strings.Repeat("a", 250) + "@x.test"}
	for _, value := range invalid {
		if IsEmail(value) {
			t.Errorf("IsEmail(%q) = true", value)
		}
	}
	if !MaxLen("abc", 3) || MaxLen("abcd", 3) {
		t.Fatal("MaxLen boundary failed")
	}
}

func TestParseJSONStrictAndValidationError(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}
	app := fiber.New()
	app.Post("/", func(c *fiber.Ctx) error {
		var body payload
		if err := ParseJSONStrict(c, &body); err != nil {
			return ValidationError(c, map[string]string{"body": err.Error()})
		}
		return c.JSON(body)
	})
	for _, tc := range []struct {
		body string
		want int
	}{
		{`{"name":"task"}`, fiber.StatusOK},
		{`{"name":"task","unknown":true}`, fiber.StatusBadRequest},
		{`not-json`, fiber.StatusBadRequest},
	} {
		request := httptest.NewRequest(fiber.MethodPost, "/", strings.NewReader(tc.body))
		request.Header.Set("Content-Type", "application/json")
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != tc.want {
			t.Errorf("body %q status = %d, want %d", tc.body, response.StatusCode, tc.want)
		}
	}
}

func TestSanitizeURLAndQuery(t *testing.T) {
	if got := SanitizeURLPath("/ical/secret-token.ics"); got != "/ical/[REDACTED].ics" {
		t.Fatalf("path = %q", got)
	}
	if got := SanitizeURLPath("/ical/token/extra.ics"); got != "/ical/[REDACTED]/extra.ics" {
		t.Fatalf("nested path = %q", got)
	}
	if got := SanitizeURLPath("/tasks/1"); got != "/tasks/1" {
		t.Fatalf("ordinary path = %q", got)
	}
	if got := sanitizeURLPath("/ical/abc.ics"); got != "/ical/[REDACTED].ics" {
		t.Fatalf("private wrapper = %q", got)
	}
	if got := SanitizeQueryString(""); got != "" {
		t.Fatalf("empty query = %q", got)
	}
	if got := SanitizeQueryString("page=2&token=secret&note=bearer%20safe"); got != "page=2&token=[REDACTED]&note=bearer%20safe" {
		t.Fatalf("query = %q", got)
	}
	if got := SanitizeQueryString("value=Bearer secret"); got != "value=[REDACTED]" {
		t.Fatalf("sensitive value = %q", got)
	}
}

func TestSensitiveClassifiersAndRecursiveScrubbing(t *testing.T) {
	for _, key := range []string{"Authorization", " api_key ", "password", "refresh_token"} {
		if !IsSensitiveKey(key) {
			t.Errorf("key %q was not sensitive", key)
		}
	}
	if IsSensitiveKey("page") {
		t.Fatal("ordinary key was sensitive")
	}
	for _, value := range []string{"Bearer abc", "tsk_key", "sess_value", "aaa.bbbbbbbbbbbbbbbbbbbbbbbbbbbbb.ccc", "abcdefabcdefabcdefabcdef"} {
		if !IsSensitiveValue(value) {
			t.Errorf("value %q was not sensitive", value)
		}
	}
	for _, value := range []string{"", "ordinary", "a.b"} {
		if IsSensitiveValue(value) {
			t.Errorf("value %q was sensitive", value)
		}
	}

	data := map[string]any{
		"token": "plain", "note": "Bearer abc",
		"nested": map[string]any{"password": "visible", "safe": "ok"},
		"labels": map[string]string{"api_key": "visible", "safe": "sess_hidden"},
	}
	ScrubPIIMap(data)
	if data["token"] != "[REDACTED]" || data["note"] != "[REDACTED]" {
		t.Fatalf("top-level scrub = %#v", data)
	}
	nested := data["nested"].(map[string]any)
	if nested["password"] != "[REDACTED]" || nested["safe"] != "ok" {
		t.Fatalf("nested scrub = %#v", nested)
	}
	labels := data["labels"].(map[string]string)
	if labels["api_key"] != "[REDACTED]" || labels["safe"] != "[REDACTED]" {
		t.Fatalf("map scrub = %#v", labels)
	}
}

func TestDeadlineAndDisplayHelpers(t *testing.T) {
	labels := map[[2]string]string{
		{"", "MORNING"}: "朝まで", {"", "LUNCH"}: "昼休みまで", {"", "AFTER_SCHOOL"}: "放課後まで",
		{"P2", "P2"}: "2限", {"P2", "P4"}: "2〜4限まで", {"", "P3"}: "3限まで", {"P3", ""}: "3限以降",
		{"P4", "P2"}: "2限まで", {"bad", "bad"}: "",
	}
	for input, want := range labels {
		if got := deriveDeadlineLabelEnum(input[0], input[1]); got != want {
			t.Errorf("derive%v = %q, want %q", input, got, want)
		}
	}
	for input, want := range map[string]int{"P1": 1, " p10 ": 10, "P0": 0, "bad": 0} {
		if got := periodIndex(input); got != want {
			t.Errorf("periodIndex(%q) = %d", input, got)
		}
	}
	for input, want := range map[string]string{"remote": "ONLINE", "対面授業": "FACE_TO_FACE", "exam": "QUIZ", "発表": "PRESENTATION", "復習": "SELF_STUDY", "misc": "OTHER", "unknown": "OTHER"} {
		if got := NormalizeTaskType(input); got != want {
			t.Errorf("NormalizeTaskType(%q) = %q", input, got)
		}
	}
	if displayBoolJa(true) != "はい" || displayBoolJa(false) != "いいえ" {
		t.Fatal("boolean labels failed")
	}
	if displaySinceJa("P3") != "3限" || displaySinceJa("") != "指定なし" || displaySinceJa("custom") != "CUSTOM" {
		t.Fatal("since labels failed")
	}
	if displayUntilJa("P3") != "3限まで" || displayUntilJa("morning") != "朝まで" || displayUntilJa("") != "指定なし" {
		t.Fatal("until labels failed")
	}
	if EffectiveDeadlineExpr("") != "deadline" || EffectiveDeadlineExpr("t") != "t.deadline" {
		t.Fatal("deadline expression failed")
	}
}

func TestTaskNotificationMessages(t *testing.T) {
	beforeDescription, afterDescription := "old", "new"
	before := domain.Task{ID: 7, Title: "old", SubjectID: 1, Description: &beforeDescription, Type: "ONLINE", Deadline: time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC), DeadlineSince: "P1", DeadlineUntil: "P2"}
	after := domain.Task{ID: 7, Title: "new", SubjectID: 2, Description: &afterDescription, Type: "QUIZ", IsRequireHandIn: true, IsSelfCheckRequired: true, Deadline: before.Deadline.AddDate(0, 0, 1), DeadlineSince: "P2", DeadlineUntil: "LUNCH"}
	db := openPasskeyTestDB(t)
	if err := db.AutoMigrate(&domain.Subject{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&domain.Subject{ID: 1, NameJa: "国語"}).Error; err != nil {
		t.Fatal(err)
	}
	english := "English"
	if err := db.Create(&domain.Subject{ID: 2, NameEn: &english}).Error; err != nil {
		t.Fatal(err)
	}
	message := BuildTaskUpdateMessageJa(before, after, db)
	for _, want := range []string{"ID:7", "'old' → 'new'", "'国語' → 'English'", "'オンライン' → '小テスト'", "2026-09-23 → 2026-09-24", "1限 → 2限", "2限まで → 昼休みまで"} {
		if !strings.Contains(message, want) {
			t.Errorf("message missing %q:\n%s", want, message)
		}
	}
	if BuildTaskDeleteMessageJa(after) != "この課題は削除されました。" {
		t.Fatal("delete message changed")
	}
	if strOrUnsetJa(nil) != "（未設定）" {
		t.Fatal("nil description label failed")
	}
}
