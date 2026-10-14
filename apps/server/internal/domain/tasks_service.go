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

// CompleteTask marks completion for a user.
func CompleteTask(db *gorm.DB, userID uint, taskID uint) error {
	// Lock the task so disabling completion and marking it complete serialize.
	return db.Transaction(func(tx *gorm.DB) error {
		var task Task
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&task, taskID).Error; err != nil {
			return err
		}
		if task.IsCompletionDisabled {
			return fmt.Errorf("task completion disabled")
		}
		return tx.Exec("INSERT INTO tasks_users_completions(user_id, task_id) VALUES(?, ?) ON DUPLICATE KEY UPDATE completed_at = CURRENT_TIMESTAMP", userID, taskID).Error
	})
}

// UncompleteTask removes completion mark for a user.
func UncompleteTask(db *gorm.DB, userID uint, taskID uint) error {
	return db.Exec("DELETE FROM tasks_users_completions WHERE user_id = ? AND task_id = ?", userID, taskID).Error
}
