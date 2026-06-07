package utils

import (
	"time"
	"github.com/asari194617/taskly/apps/server/internal/domain"
	"gorm.io/gorm"
)

// Email suppression entries for temporary delivery pause
type EmailSuppression struct {
	ID             uint   `gorm:"primaryKey"`
	Email          string `gorm:"size:255;index;not null"`
	Reason         *string
	SuspendedUntil time.Time `gorm:"index;not null"`
	CreatedAt      time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
}

// Migration tracking
type Migration struct {
	ID        uint      `gorm:"primaryKey"`
	Name      string    `gorm:"uniqueIndex;size:255;not null"`
	AppliedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
}

type WebauthnUserWrapper struct {
	*domain.User
	db *gorm.DB
}

func NewWebauthnUserWrapper(u *domain.User, db *gorm.DB) *WebauthnUserWrapper {
  return &WebauthnUserWrapper{User: u, db: db}
}