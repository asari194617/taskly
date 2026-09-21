package jobs

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&Job{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func createTasksTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE tasks (
		id integer primary key autoincrement, added_user_id integer not null,
		title text not null, subject_id integer not null, description text,
		type text not null default 'OTHER', is_require_hand_in integer not null default 0,
		is_self_check_required integer not null default 0, is_hidden integer not null default 0,
		deadline date, deadline_since text not null default 'NONE',
		deadline_until text not null default 'NONE', created_at datetime default CURRENT_TIMESTAMP,
		updated_at datetime default CURRENT_TIMESTAMP
	)`).Error; err != nil {
		t.Fatal(err)
	}
}

func TestRestartResumesAcceptedTeamsJobExactlyOnce(t *testing.T) {
	db := testDB(t)
	createTasksTable(t, db)
	if err := db.Exec(`CREATE TABLE tasks_teams_logs (id integer primary key, task_id integer not null, inbound_job_id integer unique, is_success integer, callback_url text, attempts integer, last_status text)`).Error; err != nil {
		t.Fatal(err)
	}
	key := "teams-inbound:request-1"
	deadline := time.Date(2026, 9, 22, 0, 0, 0, 0, time.Local)
	accepted, err := Enqueue(db, "teams_inbound", TeamsInbound{Title: "from Teams", AddedUserID: 1, SubjectID: 1, Deadline: deadline}, &key)
	if err != nil {
		t.Fatal(err)
	}
	db.Model(&Job{}).Where("id = ?", accepted.ID).Update("available_at", time.Now().Add(-time.Second))
	duplicate, err := Enqueue(db, "teams_inbound", TeamsInbound{Title: "from Teams", AddedUserID: 1, SubjectID: 1, Deadline: deadline}, &key)
	if err != nil || duplicate.ID != accepted.ID {
		t.Fatalf("duplicate = %v, %v", duplicate.ID, err)
	}
	if _, err := Enqueue(db, "teams_inbound", TeamsInbound{Title: "different", AddedUserID: 1, SubjectID: 1}, &key); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("changed request with same key: %v", err)
	}
	// No worker ran before the simulated restart.
	worker := NewWorker(db, func(job Job) error { return Deliver(db, job, CallbackPolicy{}) })
	if _, err := worker.RunOne(); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&domain.Task{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("tasks = %d, %v", count, err)
	}
	// A crash after transaction commit but before marking the job complete
	// replays the same row without inserting a second task.
	if err := db.Model(&Job{}).Where("id = ?", accepted.ID).Updates(map[string]any{"status": "processing", "lease_until": time.Now().Add(-time.Minute)}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := worker.RunOne(); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&domain.Task{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("replayed tasks = %d, %v", count, err)
	}
}

func TestDeliveryFailureStopsAndCanBeRetried(t *testing.T) {
	db := testDB(t)
	job, err := Enqueue(db, "task_discord", map[string]any{"test": true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	db.Model(&Job{}).Where("id = ?", job.ID).Update("available_at", time.Now().Add(-time.Second))
	worker := NewWorker(db, func(Job) error { return errors.New("webhook unavailable") })
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			db.Model(&Job{}).Where("id = ?", job.ID).Update("available_at", time.Now().Add(-time.Second))
		}
		if _, err := worker.RunOne(); err == nil {
			t.Fatal("expected delivery error")
		}
	}
	var stored Job
	if err := db.First(&stored, job.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != "failed" || stored.Attempts != maxAttempts || stored.LastError == nil {
		t.Fatalf("job = %#v", stored)
	}
	if err := Retry(db, job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := NewWorker(db, func(Job) error { return nil }).RunOne(); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&stored, job.ID).Error; err != nil || stored.Status != "succeeded" {
		t.Fatalf("retried job = %#v, %v", stored, err)
	}
}

func TestDBFailureRejectsEnqueueAndRollsBackTask(t *testing.T) {
	db := testDB(t)
	createTasksTable(t, db)
	t.Setenv("DISCORD_WEBHOOK_URL", "https://example.test/webhook")
	if err := db.Migrator().DropTable(&Job{}); err != nil {
		t.Fatal(err)
	}
	_, err := CreateTask(db, 1, "task", 1, nil, "OTHER", false, false, domain.Task{Deadline: time.Now(), DeadlineSince: "NONE", DeadlineUntil: "NONE"}, "actor")
	if err == nil {
		t.Fatal("expected database error")
	}
	var count int64
	if err := db.Model(&domain.Task{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("tasks after rollback = %d, %v", count, err)
	}
}

func TestCallbackDeliveryFailureIsTrackedAndRetried(t *testing.T) {
	db := testDB(t)
	if err := db.Exec(`CREATE TABLE tasks_teams_logs (task_id integer primary key, last_status text, last_response_status integer, last_response_body text)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO tasks_teams_logs(task_id) VALUES (9)`).Error; err != nil {
		t.Fatal(err)
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Idempotency-Key") == "" {
			t.Error("missing idempotency key")
		}
		if calls == 1 {
			w.WriteHeader(500)
		} else {
			w.WriteHeader(204)
		}
	}))
	defer server.Close()
	job, err := Enqueue(db, "teams_callback", Callback{TaskID: 9, URL: server.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	policy := CallbackPolicy{Validate: func(string) error { return nil }, Client: server.Client()}
	worker := NewWorker(db, func(job Job) error { return Deliver(db, job, policy) })
	if _, err := worker.RunOne(); err == nil {
		t.Fatal("expected HTTP failure")
	}
	var stored Job
	if err := db.First(&stored, job.ID).Error; err != nil || stored.Status != "pending" || stored.LastError == nil {
		t.Fatalf("failed delivery = %#v, %v", stored, err)
	}
	if err := db.Model(&Job{}).Where("id = ?", job.ID).Update("available_at", time.Now().Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := worker.RunOne(); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&stored, job.ID).Error; err != nil || stored.Status != "succeeded" || calls != 2 {
		t.Fatalf("retry = %#v, calls=%d, err=%v", stored, calls, err)
	}
}
