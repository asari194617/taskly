package domain

import "time"

// SystemSetting provides a generic key/value store for system-wide configs
type SystemSetting struct {
	ID        uint      `gorm:"primaryKey"`
	Key       string    `gorm:"uniqueIndex;size:128;not null"`
	Value     string    `gorm:"type:text;not null"` // JSON string
	CreatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP;autoUpdateTime"`
}