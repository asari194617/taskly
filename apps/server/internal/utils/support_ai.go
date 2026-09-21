package utils

import (
	"context"
	"github.com/getsentry/sentry-go"
	"google.golang.org/genai"
	"strings"
	"time"
)

func DateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// Decide whether to rotate day shift based on Gemini API analysis of subject/body.
// Returns true when rotation/skip should occur. This version preserves the
// legacy signature and logic; higher-level code is responsible for deciding
// whether the rotation is "today" or "tomorrow" based on additional cues.
func ShouldRotateByGemini(subject, body string) bool {
	// 1) Strong heuristic first (Japanese keywords)
	s := strings.ToLower(subject + "\n" + body)
	if shouldRotateHeuristic(s) {
		return true
	}
	// 2) If Gemini API key is configured, ask the model; parse JSON result safely
	if Env("GEMINI_API_KEY", "") == "" {
		// No API key and heuristic didn't match → do not rotate
		return false
	}
	// Allow overriding model via env; default to gemini-2.5-flash to match current API examples
	modelName := strings.TrimSpace(Env("GEMINI_MODEL", "gemini-2.5-flash"))
	ctx := context.Background()
	cli, err := genai.NewClient(ctx, nil)
	if err == nil {
		prompt := "以下のメール内容が『当日日直の延期・振替・順延・スライド（＝日直をずらす）』を求めているか判定してください。回答は YES または NO のみ。\nSubject: " + subject + "\nBody: " + body
		content := genai.NewContentFromText(prompt, "")
		resp, err := cli.Models.GenerateContent(ctx, modelName, []*genai.Content{content}, nil)
		if err == nil && resp != nil {
			ans := strings.ToUpper(resp.Text())
			if strings.Contains(ans, "YES") || strings.Contains(ans, "ＹＥＳ") || strings.Contains(ans, "はい") {
				return true
			}
			if strings.Contains(ans, "NO") || strings.Contains(ans, "ＮＯ") || strings.Contains(ans, "いいえ") {
				return false
			}
		}
	}
	// 3) Fallback: if model response ambiguous, rely on heuristic again
	return shouldRotateHeuristic(s)
}

// shouldRotateHeuristic detects Japanese phrases that imply postponing/rescheduling day shift
func shouldRotateHeuristic(s string) bool {
	// Normalize (remove spaces)
	s = strings.ReplaceAll(s, " ", "")
	// Positive signals
	positives := []string{
		"延期", "順延", "日程変更", "変更します", "変更したい", "変更を希望", "スライド", "ずらし", "ずらせ", "ずらしたい",
		"振替", "振り替え", "別日に", "後ろ倒し", "先送り", "移動したい", "日直を変", "交代をお願い", "交代したい",
		"日直変更", "日直ずらし", "日直延期", "都合が悪いので", "本日の日直を延期", "今日の", "日直を延期",
	}
	for _, kw := range positives {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// Shift all assignments from the next business day after 'start' one business day forward,
// and ensure today's users are also assigned to that next business day.

// Legacy backward-shift helper also depended on day_shift_assignments; keep signature but no-op.

// Reports error to Sentry if DSN is configured
func CaptureError(err error) {
	if err == nil {
		return
	}
	if Env("SENTRY_DSN", "") == "" {
		return
	}
	sentry.CaptureException(err)
}
