package main

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

type User struct {
	ID               uint      `gorm:"primaryKey"`
	Email            string    `gorm:"uniqueIndex;size:255;not null"`
	DisplayName      string    `gorm:"size:255"`
	HashedPassword   *string   `gorm:"size:255"`
	EmailCodeAllowed bool      `gorm:"default:true"`
	IsDeleted        bool      `gorm:"default:false"`
	IsDayShiftActive bool      `gorm:"default:false"`
	NotificationLang string    `gorm:"size:5;not null;default:'ja'"`
	IsRegistered     bool      `gorm:"default:false"`
	IsAgreedPolicies bool      `gorm:"default:false"`
	CreatedAt        time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
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
func (u *User) WebAuthnCredentials() []webauthn.Credential {
	var pks []Passkey
	_ = DB.Where("user_id = ?", u.ID).Find(&pks).Error
	res := make([]webauthn.Credential, 0, len(pks))
	for _, p := range pks {
		cid, _ := base64.RawStdEncoding.DecodeString(p.CredentialID)
		pub, _ := base64.RawStdEncoding.DecodeString(p.PublicKey)
		res = append(res, webauthn.Credential{ID: cid, PublicKey: pub, Authenticator: webauthn.Authenticator{SignCount: uint32(p.SignCount)}})
	}
	return res
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

// Web Push subscription for PWA notifications
type WebPushSubscription struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"index;not null"`
	Endpoint  string    `gorm:"size:1024;not null"`
	P256dh    string    `gorm:"size:1024;not null"`
	Auth      string    `gorm:"size:255;not null"`
	CreatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP;autoUpdateTime"`
}

// VAPID keys for Web Push (single row)
type WebPushVAPID struct {
	ID         uint      `gorm:"primaryKey"`
	PublicKey  string    `gorm:"size:255;not null"`
	PrivateKey string    `gorm:"size:255;not null"`
	CreatedAt  time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
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

type Block struct {
	ID        uint `gorm:"primaryKey"`
	UserID    uint `gorm:"index;not null"`
	Reason    *string
	ExpiresAt *time.Time
	CreatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP;autoUpdateTime"`
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

type Subject struct {
	ID        uint    `gorm:"primaryKey"`
	NameJa    string  `gorm:"size:255;not null;default:''"`
	NameEn    *string `gorm:"size:255"`
	ExpiresAt *time.Time
	CreatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
}

type Task struct {
	ID                uint    `gorm:"primaryKey"`
	AddedUserID       uint    `gorm:"index;not null"`
	Title             string  `gorm:"size:255;not null"`
	SubjectID         uint    `gorm:"index;not null"`
	Description       *string `gorm:"type:text"`
	Type              string  `gorm:"type:ENUM('ONLINE','FACE_TO_FACE','QUIZ','PRESENTATION','SELF_STUDY','OTHER');not null;default:'OTHER'"`
	IsRequireHandIn   bool    `gorm:"not null;default:0"`
	SelfCheckRequired bool    `gorm:"not null;default:0"`
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

// Email suppression entries for temporary delivery pause
type EmailSuppression struct {
	ID             uint   `gorm:"primaryKey"`
	Email          string `gorm:"size:255;index;not null"`
	Reason         *string
	SuspendedUntil time.Time `gorm:"index;not null"`
	CreatedAt      time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
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

// Day shift models
type DayShiftCycle struct {
	ID          uint `gorm:"primaryKey"`
	BeginDate   time.Time
	FinishDate  time.Time
	Description string `gorm:"size:255"`
}
type DayShiftAssignment struct {
	ID        uint `gorm:"primaryKey"`
	Date      time.Time
	CycleID   uint
	Status    string `gorm:"size:16"`
	Notes     *string
	CreatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP;autoUpdateTime"`
}
type DayShiftAssignmentUser struct {
	ID                    uint `gorm:"primaryKey"`
	DayShiftAssignmentsID uint
	UserID                uint
}
type BusinessCalendar struct {
	ID            uint `gorm:"primaryKey"`
	Date          time.Time
	IsBusinessDay bool
	Notes         *string
	CreatedAt     time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt     time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP;autoUpdateTime"`
}

// Deprecated: Legacy pair model kept only for migration/backfill.
// Use DayShiftCycleGroup and DayShiftCycleGroupMember instead.
type DayShiftCyclePair struct {
	ID        uint      `gorm:"primaryKey"`
	CycleID   uint      `gorm:"index;not null"`
	User1ID   uint      `gorm:"not null"`
	User2ID   uint      `gorm:"not null"`
	Position  int       `gorm:"default:0"`
	CreatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
}

// Generalized day shift group (supports 2 or more users per position)
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

// Migration tracking
type Migration struct {
	ID        uint      `gorm:"primaryKey"`
	Name      string    `gorm:"uniqueIndex;size:255;not null"`
	AppliedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
}

// SystemSetting provides a generic key/value store for system-wide configs
type SystemSetting struct {
	ID        uint      `gorm:"primaryKey"`
	Key       string    `gorm:"uniqueIndex;size:128;not null"`
	Value     string    `gorm:"type:text;not null"` // JSON string
	CreatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP;autoUpdateTime"`
}

// NotificationSettings stores per-user delivery preferences
// Removed: NotificationSettings (replaced by NotificationDevice + NotificationTypePref)

// NotificationTypePref stores per-user on/off for specific notification kinds
type NotificationTypePref struct {
	ID               uint      `gorm:"primaryKey"`
	UserID           uint      `gorm:"index;not null"`
	NotificationType string    `gorm:"size:16;not null"` // TASK | DAYSHIFT
	IsEnabled        bool      `gorm:"not null;default:true"`
	CreatedAt        time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
}

// NotificationDevice stores per-user devices/channels (email, webpush, mobile)
type NotificationDevice struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"index;not null"`
	Name      *string   `gorm:"size:255"`
	Platform  string    `gorm:"size:16;not null"` // EMAIL | WEBPUSH | IOS | ANDROID
	Payload   *string   `gorm:"type:json"`        // optional JSON payload/metadata
	IsEnabled bool      `gorm:"not null;default:true"`
	CreatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP;autoUpdateTime"`
}
