package domain

import "time"

// Day shift models
type DayShiftCycle struct {
	ID          uint `gorm:"primaryKey"`
	BeginDate   time.Time
	FinishDate  time.Time
	Description string `gorm:"size:255"`
}

type DayShiftCycleGroup struct {
	ID        uint      `gorm:"primaryKey"`
	CycleID   uint      `gorm:"index;not null"`
	Position  int       `gorm:"default:0"`
	CreatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
}

type DayShiftCycleGroupMember struct {
	ID        uint      `gorm:"primaryKey"`
	GroupID   uint      `gorm:"index;not null"`
	UserID    uint      `gorm:"not null"`
	Position  int       `gorm:"default:0"`
	CreatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
}