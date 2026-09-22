package domain

import (
	"fmt"
	"time"
)

type User struct {
	ID               uint       `gorm:"primaryKey"`
	Email            string     `gorm:"uniqueIndex;size:255;not null"`
	DisplayName      string     `gorm:"size:255"`
	HashedPassword   *string    `gorm:"size:255"`
	EmailCodeAllowed bool       `gorm:"default:true"`
	IsDeleted        bool       `gorm:"default:false"`
	DeletedAt        *time.Time `gorm:"index"`
	NotificationLang string     `gorm:"size:5;not null;default:'ja'"`
	IsRegistered     bool       `gorm:"default:false"`
	IsAgreedPolicies bool       `gorm:"default:false"`
	CreatedAt        time.Time  `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
}

type TOTPSecret struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"uniqueIndex;not null"`
	Secret    string    `gorm:"size:255;not null"` // encrypted base32
	IsActive  bool      `gorm:"default:false"`
	CreatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
}

type RecoveryCode struct {
	ID         uint      `gorm:"primaryKey"`
	UserID     uint      `gorm:"index;not null"`
	HashedCode string    `gorm:"size:255;not null"`
	IsUsed     bool      `gorm:"default:false"`
	CreatedAt  time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
}

// Login codes (email magic code)
type LoginCode struct {
	ID             uint      `gorm:"primaryKey"`
	Email          string    `gorm:"index;size:255;not null"`
	HashedCode     string    `gorm:"size:255;not null"`
	ExpiresAt      time.Time `gorm:"index;not null"`
	ConsumedAt     *time.Time
	FailedAttempts int `gorm:"not null;default:0"`
	LockedUntil    *time.Time
	CreatedAt      time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
}

// Passkeys (WebAuthn) per specification
type Passkey struct {
	ID           uint      `gorm:"primaryKey"`
	UserID       uint      `gorm:"index;not null"`
	CredentialID string    `gorm:"size:255;uniqueIndex;not null"`
	PublicKey    string    `gorm:"type:text;not null"`
	SignCount    int64     `gorm:"not null"`
	DeviceName   *string   `gorm:"size:255"`
	CreatedAt    time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt    time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP;autoUpdateTime"`
}

type Block struct {
	ID        uint `gorm:"primaryKey"`
	UserID    uint `gorm:"index;not null"`
	Reason    *string
	ExpiresAt *time.Time
	CreatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP;autoUpdateTime"`
}

// RBAC and permissions
type RoleUser struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"index;not null"`
	Role      string    `gorm:"size:16;not null"` // admin | manager
	CreatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
}

type Permission struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"index;not null"`
	Target    string    `gorm:"size:64;not null"`
	Method    string    `gorm:"size:16;not null"`
	Value     *string   `gorm:"size:255"`
	CreatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP;autoUpdateTime"`
}

type Session struct {
	ID             uint   `gorm:"primaryKey"`
	UserID         uint   `gorm:"index;not null"`
	ClientPlatform string `gorm:"size:64;not null"`
	HashedToken    string `gorm:"size:255;not null"`
	ExpiresAt      *time.Time
	CreatedAt      time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt      time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP;autoUpdateTime"`
}

type APIKey struct {
	ID          uint    `gorm:"primaryKey"`
	UserID      uint    `gorm:"index;not null"`
	HashedToken string  `gorm:"size:255;not null"`
	Description *string `gorm:"size:255"`
	IsValid     bool    `gorm:"default:true"`
	ExpiresAt   *time.Time
	CreatedAt   time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt   time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP;autoUpdateTime"`
}

type ApiActionLog struct {
	ID          uint      `gorm:"primaryKey"`
	UserID      *uint     `gorm:"index"`
	HashedToken string    `gorm:"size:255;not null"`
	Action      string    `gorm:"size:64;not null"`
	IP          string    `gorm:"size:64;not null"`
	CreatedAt   time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
}

// Generic system/user logs per specification
type Log struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    *uint     `gorm:"index"`
	Action    string    `gorm:"size:64;not null"`
	CreatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
}

// WebAuthn user interface implementation
func (u *User) WebAuthnID() []byte {
	return []byte(fmt.Sprintf("%d", u.ID))
}
func (u *User) WebAuthnName() string {
	if u.Email != "" {
		return u.Email
	}
	return fmt.Sprintf("user-%d", u.ID)
}
func (u *User) WebAuthnDisplayName() string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	if u.Email != "" {
		return u.Email
	}
	return fmt.Sprintf("user-%d", u.ID)
}
func (u *User) WebAuthnIcon() string { return "" }
