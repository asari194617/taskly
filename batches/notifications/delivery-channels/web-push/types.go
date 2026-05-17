package main

import (
	"database/sql"
)

type QueueItem struct {
	ID        int64
	WhenLabel string
	Subject   string
	Title     string
	Body      string
	UserID    sql.NullInt64
}

type WebPushSubscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}