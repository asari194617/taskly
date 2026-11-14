package domain

import (
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CreateTask creates a new task record and triggers async notifications.
func CreateTask(db *gorm.DB, addedUserID uint, title string, subjectID uint, description *string, typ string, isRequireHandIn bool, isSelfCheckRequired bool, deadline time.Time, deadlineSince string, deadlineUntil string, userEmail string, completionDisabled ...bool) (Task, error) {
	rec := Task{AddedUserID: addedUserID, Title: title, SubjectID: subjectID, Description: description, Type: typ, IsRequireHandIn: isRequireHandIn, IsSelfCheckRequired: isSelfCheckRequired, Deadline: deadline, DeadlineSince: deadlineSince, DeadlineUntil: deadlineUntil}
	if len(completionDisabled) > 0 {
		rec.IsCompletionDisabled = completionDisabled[0]
	} else {
		defaults, err := TaskCompletionDefaults(db)
		if err != nil {
			return Task{}, err
		}
		rec.IsCompletionDisabled = defaults[typ]
	}
	if err := db.Create(&rec).Error; err != nil {
		return Task{}, err
	}
	return rec, nil
}

// UpdateTask loads a task, applies modifier, saves it and triggers notifications.
// Returns before and after copies of the task.
func UpdateTask(db *gorm.DB, id uint, modifier func(*Task) error, userEmail string) (Task, Task, error) {
	var rec Task
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&rec, id).Error; err != nil {
		return Task{}, Task{}, err
	}
	before := rec
	if err := modifier(&rec); err != nil {
		return Task{}, Task{}, err
	}
	if err := db.Save(&rec).Error; err != nil {
		return Task{}, Task{}, err
	}
	return before, rec, nil
}

// DeleteTask deletes the task and triggers notifications.
func DeleteTask(db *gorm.DB, id uint, userEmail string) (Task, error) {
	var rec Task
	if err := db.First(&rec, id).Error; err != nil {
		return Task{}, err
	}
	if err := db.Delete(&Task{}, rec.ID).Error; err != nil {
		return Task{}, err
	}
	return rec, nil
}

// LockCompletionUser serializes one user's completion and ownership changes.
// Always acquire this lock before any Task locks in a completion transaction.
func LockCompletionUser(tx *gorm.DB, userID uint) error {
	var user User
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ? AND (is_deleted = ? OR is_deleted IS NULL)", userID, false).First(&user).Error
}

// CompleteTask records an explicit manual check, retiring automatic ownership
// on this endpoint and on visits previously derived from this source task.
func CompleteTask(db *gorm.DB, userID uint, taskID uint) error {
	return changeTaskCompletion(db, userID, taskID, true, true)
}

// UncompleteTask records an explicit manual undo. Other completion flags stay
// unchanged, while metadata linking them to this endpoint is retired.
func UncompleteTask(db *gorm.DB, userID uint, taskID uint) error {
	return changeTaskCompletion(db, userID, taskID, false, true)
}

// SetLinkedTaskCompletion is for the validated atomic guidance pair only. Its
// caller holds the user lock and all pair Task locks in ascending order. It
// must not retire ownership before the pair's scoped undo has examined it.
func SetLinkedTaskCompletion(db *gorm.DB, userID uint, taskID uint, completed bool) error {
	return changeTaskCompletion(db, userID, taskID, completed, false)
}

func changeTaskCompletion(db *gorm.DB, userID uint, taskID uint, completed, manual bool) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := LockCompletionUser(tx, userID); err != nil {
			return err
		}
		var task Task
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&task, taskID).Error; err != nil {
			return err
		}
		if completed && task.IsCompletionDisabled {
			return fmt.Errorf("task completion disabled")
		}
		var err error
		if completed {
			err = tx.Exec("INSERT INTO tasks_users_completions(user_id, task_id) VALUES(?, ?) ON DUPLICATE KEY UPDATE completed_at = CURRENT_TIMESTAMP, auto_event_id = NULL, auto_source_task_id = NULL, auto_assignment_scope = NULL", userID, taskID).Error
		} else {
			err = tx.Exec("DELETE FROM tasks_users_completions WHERE user_id = ? AND task_id = ?", userID, taskID).Error
		}
		if err != nil {
			return err
		}
		if manual {
			return tx.Table("tasks_users_completions").Where("user_id = ? AND auto_source_task_id = ?", userID, taskID).Updates(map[string]any{"auto_event_id": nil, "auto_source_task_id": nil, "auto_assignment_scope": nil}).Error
		}
		return nil
	})
}
