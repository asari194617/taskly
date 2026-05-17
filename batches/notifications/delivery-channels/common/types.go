package common

import "time"

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
