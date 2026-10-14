package utils

import (
	"strings"
	"testing"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
)

func TestTeamsUpdateComparesDeadlineAsCalendarDate(t *testing.T) {
	before := domain.Task{ID: 7, Title: "before", Deadline: time.Date(2026, 10, 15, 0, 0, 0, 0, time.FixedZone("JST", 9*3600))}
	after := before
	after.Deadline = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	if got := BuildTaskUpdateMessageJa(before, after, nil); got != "" {
		t.Fatalf("same date generated update: %q", got)
	}
	after.Title = "after"
	got := BuildTaskUpdateMessageJa(before, after, nil)
	if !strings.Contains(got, "タイトル") || strings.Contains(got, "期限日") {
		t.Fatalf("title-only update: %q", got)
	}
	after.Deadline = after.Deadline.AddDate(0, 0, 1)
	if got := BuildTaskUpdateMessageJa(before, after, nil); !strings.Contains(got, "2026-10-15 → 2026-10-16") {
		t.Fatalf("changed date missing: %q", got)
	}
}

func TestTeamsUpdateSkipsFieldsAbsentFromOriginalPost(t *testing.T) {
	before := domain.Task{ID: 7, Title: "task", Type: "ONLINE", Deadline: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)}
	after := before
	after.Type = "QUIZ"
	after.IsRequireHandIn = true
	after.IsSelfCheckRequired = true
	empty := ""
	after.Description = &empty
	if got := BuildTaskUpdateMessageJa(before, after, nil); got != "" {
		t.Fatalf("invisible changes generated update: %q", got)
	}
	after.Title = "updated"
	got := BuildTaskUpdateMessageJa(before, after, nil)
	for _, field := range []string{"種別", "要提出", "自己チェック"} {
		if strings.Contains(got, field) {
			t.Fatalf("invisible field in mixed update: %q", got)
		}
	}
	if !strings.Contains(got, "タイトル") {
		t.Fatalf("visible field missing: %q", got)
	}
}

func TestTeamsHTMLPreservesBreaksAndLiteralMarkup(t *testing.T) {
	input := "1行目\r\n2行目\n\n<重要> & <br>\r最後"
	want := "1行目<br>2行目<br><br>&lt;重要&gt; &amp; &lt;br&gt;<br>最後"
	if got := teamsHTML(input); got != want {
		t.Fatalf("teamsHTML = %q, want %q", got, want)
	}
}
