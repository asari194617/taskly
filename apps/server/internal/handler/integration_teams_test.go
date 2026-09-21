package handler

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestParseTeamsStatusPayload(t *testing.T) {
	tests := []struct {
		name string
		raw  map[string]any
		want teamsStatusPayload
	}{
		{
			name: "string task id",
			raw: map[string]any{
				"task_id": " 42 ", "is_success": true,
				"message_id": "message", "conversation_id": "conversation",
			},
			want: teamsStatusPayload{true, 42, "message", "conversation"},
		},
		{
			name: "numeric task id",
			raw:  map[string]any{"task_id": float64(7)},
			want: teamsStatusPayload{TaskID: 7},
		},
		{
			name: "invalid task id",
			raw:  map[string]any{"task_id": "not-a-number"},
			want: teamsStatusPayload{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseTeamsStatusPayload(tt.raw); got != tt.want {
				t.Fatalf("parseTeamsStatusPayload() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

type sequenceHTTPClient struct {
	responses []int
	calls     int
}

func (c *sequenceHTTPClient) Do(*http.Request) (*http.Response, error) {
	c.calls++
	if c.calls > len(c.responses) {
		return nil, errors.New("unexpected call")
	}
	return &http.Response{
		StatusCode: c.responses[c.calls-1],
		Body:       io.NopCloser(strings.NewReader("response")),
	}, nil
}

func TestTeamsProcessorCallbackRetriesWithInjectedBackoff(t *testing.T) {
	t.Setenv("TEAMS_CALLBACK_ALLOWED_ORIGINS", "https://example.test")
	db, err := gorm.Open(sqlite.Open("file:teams-callback?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE tasks_teams_logs (
		task_id integer PRIMARY KEY,
		attempts integer,
		last_status text,
		last_error text,
		last_response_status integer,
		last_response_body text
	)`).Error; err != nil {
		t.Fatalf("create callback log table: %v", err)
	}
	if err := db.Exec("INSERT INTO tasks_teams_logs (task_id) VALUES (9)").Error; err != nil {
		t.Fatalf("seed callback log: %v", err)
	}

	client := &sequenceHTTPClient{responses: []int{500, 204}}
	delays := []time.Duration{}
	processor := newTeamsProcessor(db)
	processor.client = client
	processor.secret = func() string { return "test-secret" }
	processor.sleep = func(delay time.Duration) { delays = append(delays, delay) }

	processor.sendCallback(9, true, "https://example.test/callback")

	if client.calls != 2 {
		t.Fatalf("callback calls = %d, want 2", client.calls)
	}
	if len(delays) != 1 || delays[0] != time.Second {
		t.Fatalf("backoff delays = %v, want [1s]", delays)
	}
	var row struct {
		Attempts           int
		LastStatus         string
		ResponseStatusCode int `gorm:"column:last_response_status"`
	}
	if err := db.Table("tasks_teams_logs").Where("task_id = ?", 9).Take(&row).Error; err != nil {
		t.Fatalf("read callback log: %v", err)
	}
	if row.Attempts != 2 || row.LastStatus != "callback_sent" || row.ResponseStatusCode != 204 {
		t.Fatalf("callback log = %#v", row)
	}
}

func TestTeamsProcessorStopBeforeStartRejectsNewJobs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:teams-stop?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	processor := newTeamsProcessor(db)
	processor.Stop()
	if processor.Enqueue(teamsJob{Title: "not accepted"}) {
		t.Fatal("Enqueue() accepted work after Stop()")
	}
}
