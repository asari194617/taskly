package handler

import (
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/asari194617/taskly/apps/server/internal/jobs"
	"github.com/gofiber/fiber/v2"
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

func TestParseTeamsStatusRejectsInvalidNumericIDs(t *testing.T) {
	for _, value := range []any{"18446744073709551616", float64(-1), 1.5, math.NaN(), math.Inf(1), math.Exp2(strconv.IntSize)} {
		if got := parseTeamsStatusPayload(map[string]any{"task_id": value}); got.TaskID != 0 {
			t.Errorf("task_id %v became %d, want 0", value, got.TaskID)
		}
	}
	if strconv.IntSize == 32 {
		if got := parseTeamsStatusPayload(map[string]any{"task_id": "4294967296"}); got.TaskID != 0 {
			t.Fatal("32-bit overflow must not wrap")
		}
	}
}

func TestTeamsStatusReturnsRetryableErrorWhenLogWriteFails(t *testing.T) {
	t.Setenv("TEAMS_INTEGRATION_SECRET", "test-secret")
	db, err := gorm.Open(sqlite.Open("file:teams-callback-db-error?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	app := fiber.New()
	NewHandler(db, nil, nil).registerTeamsIntegrationRoutes(app.Group("/integrations"))
	for _, path := range []string{"/integrations/teams/callback", "/integrations/teams/status"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path,
				strings.NewReader(`{"is_success":true,"task_id":42,"message_id":"posted"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Teams-Integration-Secret", "test-secret")
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503 so Power Automate can retry", resp.StatusCode)
			}
		})
	}
}

func TestTeamsInboundAcceptsOnlyCommittedJob(t *testing.T) {
	t.Setenv("TEAMS_INTEGRATION_SECRET", "test-secret")
	db, err := gorm.Open(sqlite.Open("file:teams-inbound-route?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE users (id integer primary key, email text)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO users(id, email) VALUES (1, 'user@example.test')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&jobs.Job{}); err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	NewHandler(db, nil, nil).registerTeamsIntegrationRoutes(app.Group("/integrations"))
	request := func() int {
		req := httptest.NewRequest(http.MethodPost, "/integrations/teams/inbound", strings.NewReader(`{"email":"user@example.test","title":"task","subject_id":1}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Teams-Integration-Secret", "test-secret")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}
	if status := request(); status != http.StatusAccepted {
		t.Fatalf("status = %d", status)
	}
	var count int64
	if err := db.Model(&jobs.Job{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("jobs = %d, %v", count, err)
	}
	if err := db.Migrator().DropTable(&jobs.Job{}); err != nil {
		t.Fatal(err)
	}
	if status := request(); status != http.StatusServiceUnavailable {
		t.Fatalf("DB failure status = %d", status)
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

func TestIntegrationWorkersPerKind(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  int
	}{
		{"", 1}, {"0", 1}, {"9", 1}, {"abc", 1}, {"4", 4},
	} {
		t.Setenv("INTEGRATION_JOB_WORKERS_PER_KIND", tc.value)
		if got := integrationWorkersPerKind(); got != tc.want {
			t.Fatalf("%q: got %d, want %d", tc.value, got, tc.want)
		}
	}
}
