package main

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

type User struct {
	ID               uint    `gorm:"primaryKey"`
	Email            string  `gorm:"uniqueIndex;size:255;not null"`
	DisplayName      string  `gorm:"size:255"`
	HashedPassword   *string `gorm:"size:255"`
	EmailCodeAllowed bool    `gorm:"default:true"`
	IsDeleted        bool    `gorm:"default:false"`
	IsDayShiftActive bool    `gorm:"default:false"`
	CreatedAt        time.Time
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
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Web Push subscription for PWA notifications
type WebPushSubscription struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"index;not null"`
	Endpoint  string    `gorm:"size:1024;not null"`
	P256dh    string    `gorm:"size:1024;not null"`
	Auth      string    `gorm:"size:255;not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// RBAC and permissions
type RoleUser struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    uint   `gorm:"index;not null"`
	Role      string `gorm:"size:16;not null"` // admin | manager
	CreatedAt time.Time
}

type Permission struct {
	ID        uint    `gorm:"primaryKey"`
	UserID    uint    `gorm:"index;not null"`
	Target    string  `gorm:"size:64;not null"`
	Method    string  `gorm:"size:16;not null"`
	Value     *string `gorm:"size:255"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Block struct {
	ID        uint `gorm:"primaryKey"`
	UserID    uint `gorm:"index;not null"`
	Reason    *string
	ExpiresAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type APIKey struct {
	ID          uint    `gorm:"primaryKey"`
	UserID      uint    `gorm:"index;not null"`
	HashedToken string  `gorm:"size:255;not null"`
	Description *string `gorm:"size:255"`
	IsValid     bool    `gorm:"default:true"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Subject struct {
	ID        uint   `gorm:"primaryKey"`
	Title     string `gorm:"size:255;not null"`
	ExpiresAt *time.Time
	CreatedAt time.Time
}

type Task struct {
	ID                uint    `gorm:"primaryKey"`
	AddedUserID       uint    `gorm:"index;not null"`
	Title             string  `gorm:"size:255;not null"`
	SubjectID         uint    `gorm:"index;not null"`
	Description       *string `gorm:"type:text"`
	Type              *string `gorm:"size:32"`
	IsRequireHandIn   bool    `gorm:"default:false"`
	SelfCheckRequired bool    `gorm:"default:false"`
	Deadline          *time.Time
	DeadlineMeta      *string `gorm:"type:text"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ApiActionLog struct {
	ID          uint   `gorm:"primaryKey"`
	UserID      *uint  `gorm:"index"`
	HashedToken string `gorm:"size:255;not null"`
	Action      string `gorm:"size:64;not null"`
	IP          string `gorm:"size:64;not null"`
	CreatedAt   time.Time
}

// Generic system/user logs per specification
type Log struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    *uint     `gorm:"index"`
	Action    string    `gorm:"size:64;not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

type TOTPSecret struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    uint   `gorm:"uniqueIndex;not null"`
	Secret    string `gorm:"size:255;not null"` // encrypted base32
	IsActive  bool   `gorm:"default:false"`
	CreatedAt time.Time
}

type RecoveryCode struct {
	ID         uint   `gorm:"primaryKey"`
	UserID     uint   `gorm:"index;not null"`
	HashedCode string `gorm:"size:255;not null"`
	IsUsed     bool   `gorm:"default:false"`
	CreatedAt  time.Time
}

// Login codes (email magic code)
type LoginCode struct {
	ID         uint      `gorm:"primaryKey"`
	Email      string    `gorm:"index;size:255;not null"`
	HashedCode string    `gorm:"size:255;not null"`
	ExpiresAt  time.Time `gorm:"index;not null"`
	ConsumedAt *time.Time
	CreatedAt  time.Time
}

// Passkeys (WebAuthn) per specification
type Passkey struct {
	ID           uint      `gorm:"primaryKey"`
	UserID       uint      `gorm:"index;not null"`
	CredentialID string    `gorm:"size:255;uniqueIndex;not null"`
	PublicKey    string    `gorm:"type:text;not null"`
	SignCount    int64     `gorm:"not null"`
	DeviceName   *string   `gorm:"size:255"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`
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
	CreatedAt time.Time
	UpdatedAt time.Time
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
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type DayShiftCyclePair struct {
	ID        uint `gorm:"primaryKey"`
	CycleID   uint `gorm:"index;not null"`
	User1ID   uint `gorm:"not null"`
	User2ID   uint `gorm:"not null"`
	Position  int  `gorm:"default:0"`
	CreatedAt time.Time
}

type IcalKey struct {
	ID          uint    `gorm:"primaryKey"`
	UserID      uint    `gorm:"index;not null"`
	Token       string  `gorm:"uniqueIndex;size:255;not null"`
	Description *string `gorm:"size:255"`
	CreatedAt   time.Time
}

// MCP integration tokens per user (for MCP server access via user session)
type MCP struct {
	ID          uint   `gorm:"primaryKey"`
	UserID      uint   `gorm:"index"`
	HashedToken string `gorm:"size:255"`
	Description *string
	CreatedAt   time.Time
}
