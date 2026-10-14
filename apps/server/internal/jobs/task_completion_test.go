package jobs

import (
	"github.com/asari194617/taskly/apps/server/internal/domain"
	"testing"
	"time"
)

func TestDisablingTaskCompletionClearsAllMarks(t *testing.T) {
	db := testDB(t)
	createTasksTable(t, db)
	for _, sql := range []string{
		"CREATE TABLE tasks_users_completions (task_id integer, user_id integer)",
		"CREATE TABLE tasks_teams_logs (id integer primary key, task_id integer, teams_message_id text, is_success boolean)",
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	task := domain.Task{Title: "task", AddedUserID: 1, SubjectID: 1, Type: "OTHER", Deadline: time.Now(), DeadlineSince: "NONE", DeadlineUntil: "NONE"}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO tasks_users_completions VALUES (?, 1), (?, 2), (999, 1)", task.ID, task.ID).Error; err != nil {
		t.Fatal(err)
	}
	_, updated, err := UpdateTask(db, task.ID, func(rec *domain.Task) error { rec.IsCompletionDisabled = true; return nil }, "actor")
	if err != nil || !updated.IsCompletionDisabled {
		t.Fatalf("updated = %v, %v", updated, err)
	}
	var count int64
	db.Table("tasks_users_completions").Where("task_id = ?", task.ID).Count(&count)
	if count != 0 {
		t.Fatal("completion marks remain")
	}
	db.Table("tasks_users_completions").Where("task_id = 999").Count(&count)
	if count != 1 {
		t.Fatal("unrelated marks removed")
	}
	_, updated, err = UpdateTask(db, task.ID, func(rec *domain.Task) error { rec.IsCompletionDisabled = false; return nil }, "actor")
	if err != nil || updated.IsCompletionDisabled {
		t.Fatalf("reenable = %v, %v", updated, err)
	}
	db.Table("tasks_users_completions").Where("task_id = ?", task.ID).Count(&count)
	if count != 0 {
		t.Fatal("old marks restored")
	}
}
