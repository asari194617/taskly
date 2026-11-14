package domain

import (
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"strings"
	"testing"
	"time"
)

func completionDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	for _, sql := range []string{
		"CREATE TABLE users (id integer primary key, is_deleted boolean default 0)",
		"INSERT INTO users(id) VALUES(1),(7)",
		"CREATE TABLE configs (`key` text primary key, value text)",
		`CREATE TABLE tasks (id integer primary key, added_user_id integer, title text, subject_id integer, description text, type text, is_require_hand_in boolean, is_self_check_required boolean, is_completion_disabled boolean default 0, is_hidden boolean, deadline date, deadline_since text, deadline_until text, created_at datetime, updated_at datetime)`,
		"CREATE TABLE tasks_users_completions (user_id integer, task_id integer, completed_at datetime, auto_event_id text, auto_source_task_id integer, auto_assignment_scope text)",
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestTaskCompletionDefaultsAndExplicitOverrides(t *testing.T) {
	db := completionDB(t)
	create := func(typ string, override ...bool) Task {
		t.Helper()
		task, err := CreateTask(db, 1, "task", 1, nil, typ, false, false, time.Now(), "NONE", "NONE", "actor", override...)
		if err != nil {
			t.Fatal(err)
		}
		return task
	}
	defaults, err := TaskCompletionDefaults(db)
	if err != nil || len(defaults) != 6 {
		t.Fatalf("defaults = %v, %v", defaults, err)
	}
	for _, typ := range TaskTypes {
		if defaults[typ] != (typ == "QUIZ") {
			t.Fatalf("unexpected default for %s", typ)
		}
	}
	quiz := create("QUIZ")
	if !quiz.IsCompletionDisabled {
		t.Fatal("quiz should disable completion")
	}
	if create("QUIZ", false).IsCompletionDisabled {
		t.Fatal("explicit false lost")
	}
	if !create("OTHER", true).IsCompletionDisabled {
		t.Fatal("explicit true lost")
	}
	if err := db.Exec("INSERT INTO configs (`key`, value) VALUES (?, ?)", TaskCompletionDefaultKey("QUIZ"), "false").Error; err != nil {
		t.Fatal(err)
	}
	if create("QUIZ").IsCompletionDisabled {
		t.Fatal("configured default ignored")
	}
	var stored Task
	if err := db.First(&stored, quiz.ID).Error; err != nil || !stored.IsCompletionDisabled {
		t.Fatalf("existing task changed: %v, %v", stored, err)
	}
	if err := db.Exec("UPDATE configs SET value = 'invalid'").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := TaskCompletionDefaults(db); err == nil {
		t.Fatal("invalid config accepted")
	}
}

func TestCompleteTaskRejectsDisabledAndMissingTasks(t *testing.T) {
	db := completionDB(t)
	if err := db.Exec("INSERT INTO tasks (id, is_completion_disabled) VALUES (1, true)").Error; err != nil {
		t.Fatal(err)
	}
	if err := CompleteTask(db, 7, 1); err == nil || !strings.Contains(err.Error(), "completion disabled") {
		t.Fatalf("error = %v", err)
	}
	if err := CompleteTask(db, 7, 99); err == nil {
		t.Fatal("missing task accepted")
	}
	var count int64
	if err := db.Table("tasks_users_completions").Count(&count).Error; err != nil || count != 0 {
		t.Fatal(fmt.Sprintf("marks = %d: %v", count, err))
	}
}
