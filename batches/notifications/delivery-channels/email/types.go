package main

import (
	"database/sql"
	"time"
)

// QueueItem represents a digest entry for a recipient
type QueueItem struct {
	ID        int64
	WhenLabel string
	Subject   string
	Title     string
	Body      string
	Meta      sql.NullString
	Badges    sql.NullString // JSON
	UserID    sql.NullInt64
}

// Task is a snapshot of a task used for live composition
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

type composePayload struct {
	Locale  string           `json:"locale"`
	AppName string           `json:"app_name"`
	Items   []map[string]any `json:"items"`
}

// MySQLConfig holds parameters for building a MySQL DSN
type MySQLConfig struct {
	User     string
	Password string
	Host     string
	Port     string
	Database string
	Params   string // optional extra params
}

// EmailMessage represents the parts needed to build a multipart MIME message
type EmailMessage struct {
	From     string
	To       string
	Subject  string
	Text     string
	HTML     string
	Boundary string // optional
}

type DeliverOptions struct {
	Locale  string
	AppName string
}

type userDevicePrefs struct{ Email, WebPush bool }
