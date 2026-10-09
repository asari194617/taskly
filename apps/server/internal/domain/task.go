package domain

import "time"

type Subject struct {
	ID        uint    `gorm:"primaryKey"`
	NameJa    string  `gorm:"size:255;not null;default:''"`
	NameEn    *string `gorm:"size:255"`
	ExpiresAt *time.Time
	CreatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
}

type Task struct {
	ID                  uint    `gorm:"primaryKey"`
	AddedUserID         uint    `gorm:"index;not null"`
	Title               string  `gorm:"size:255;not null"`
	SubjectID           uint    `gorm:"index;not null"`
	Description         *string `gorm:"type:text"`
	ImageKey            *string `gorm:"->;column:image_key" json:"-"`
	Type                string  `gorm:"type:ENUM('ONLINE','FACE_TO_FACE','QUIZ','PRESENTATION','SELF_STUDY','OTHER');not null;default:'OTHER'"`
	IsRequireHandIn     bool    `gorm:"not null;default:0"`
	IsSelfCheckRequired bool    `gorm:"not null;default:0"`
	// Moderation: auto-hide flagged content from non-admin views
	IsHidden bool `gorm:"not null;default:0"`
	// Deadline date (required)
	Deadline time.Time `gorm:"type:date;not null"`
	// Canonical deadline window enums (relative to Deadline date)
	// Allowed values: NONE, MORNING/LUNCH/AFTER_SCHOOL (until only), P1..P10
	DeadlineSince string    `gorm:"type:ENUM('NONE','P1','P2','P3','P4','P5','P6','P7','P8','P9','P10');not null;default:'NONE'"`
	DeadlineUntil string    `gorm:"type:ENUM('NONE','MORNING','LUNCH','AFTER_SCHOOL','P1','P2','P3','P4','P5','P6','P7','P8','P9','P10');not null;default:'NONE'"`
	CreatedAt     time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt     time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP;autoUpdateTime"`
}

// FlaggedContent stores moderation results for content such as tasks
type FlaggedContent struct {
	ID        uint      `gorm:"primaryKey"`
	TaskID    uint      `gorm:"index;not null"`
	Category  string    `gorm:"size:64;not null"`
	Severity  string    `gorm:"size:16;not null"` // low|medium|high
	Score     *float64  `gorm:"null"`
	Threat    bool      `gorm:"not null;default:0"`
	Reason    *string   `gorm:"type:text"`
	Model     *string   `gorm:"size:64"`
	Metadata  *string   `gorm:"type:json"` // raw model response or extra info
	Snapshot  *string   `gorm:"type:json"` // snapshot of content at flag time
	CreatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
}

// UserReport records user-initiated reports on content
type UserReport struct {
	ID             uint      `gorm:"primaryKey"`
	ReporterUserID *uint     `gorm:"index"`
	TaskID         uint      `gorm:"index;not null"`
	Reason         string    `gorm:"size:255;not null"`
	Comment        *string   `gorm:"type:text"`
	CreatedAt      time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
}

type IcalKey struct {
	ID          uint    `gorm:"primaryKey"`
	UserID      uint    `gorm:"index;not null"`
	Token       string  `gorm:"uniqueIndex;size:255;not null"`
	Description *string `gorm:"size:255"`
	ExpiresAt   *time.Time
	CreatedAt   time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
}

// MCP integration tokens per user (for MCP server access via user session)
type MCP struct {
	ID          uint   `gorm:"primaryKey"`
	UserID      uint   `gorm:"index"`
	HashedToken string `gorm:"size:255"`
	Description *string
	ExpiresAt   *time.Time
	CreatedAt   time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
}
