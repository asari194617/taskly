package main

import (
	"database/sql"
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

type composePayload struct {
	Locale  string           `json:"locale"`
	AppName string           `json:"app_name"`
	Items   []map[string]any `json:"items"`
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
