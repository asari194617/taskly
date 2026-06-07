package domain

import (
	"time"

	"gorm.io/gorm"
)

// CreateReport creates a new UserReport record.
// Assumes inputs are already validated by the caller.
func CreateReport(db *gorm.DB, reporter *User, taskID uint, reason string, comment *string) (UserReport, error) {
	rec := UserReport{ReporterUserID: nil, TaskID: taskID, Reason: reason, Comment: comment, CreatedAt: time.Now()}
	if reporter != nil {
		rec.ReporterUserID = &reporter.ID
	}
	if err := db.Create(&rec).Error; err != nil {
		return UserReport{}, err
	}
	return rec, nil
}
