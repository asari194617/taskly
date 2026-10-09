package common

import (
	"database/sql"
	"time"
)

// QueueItem represents a digest entry for a recipient.
type QueueItem struct {
	ID        int64
	Kind      string
	EventKey  sql.NullString
	Slot      sql.NullString
	WhenLabel string
	Subject   string
	Title     string
	Body      string
	Meta      sql.NullString
	Badges    sql.NullString // JSON
	UserID    sql.NullInt64
	Completed bool
}

type Task struct {
	ID                  uint
	AddedUserID         uint
	SubjectID           uint
	SubjectName         string
	Type                string
	Title               string
	Description         *string
	Deadline            time.Time
	DeadlineSince       string
	DeadlineUntil       string
	IsRequireHandIn     bool
	IsSelfCheckRequired bool
}

type UserDevicePrefs struct{ Email, WebPush bool }
