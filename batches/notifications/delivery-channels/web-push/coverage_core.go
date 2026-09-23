package main

import (
	"database/sql"
	"fmt"

	dcCommon "github.com/asari194617/taskly/batches/notifications/delivery-channels/common"
)

// This file contains deterministic delivery policy and idempotency state.
// External web-push transport stays in main.go and outside unit coverage.
func notificationKey(it dcCommon.QueueItem) string {
	if it.EventKey.Valid && it.EventKey.String != "" {
		return it.EventKey.String
	}
	return fmt.Sprintf("queue:%d", it.ID)
}

func webPushEligible(db *sql.DB, it dcCommon.QueueItem) (bool, error) {
	if !it.UserID.Valid || it.UserID.Int64 <= 0 {
		return false, nil
	}
	device, prefs, eligible, err := dcCommon.LoadEligibleUserAndPrefs(db, uint(it.UserID.Int64),
		dcCommon.UserDevicePrefs{Email: true, WebPush: false},
		map[string]bool{"TASK": true, "TASK_MORNING": true, "TASK_EVENING": true, "DAYSHIFT": true})
	if err != nil || !eligible || !device.WebPush {
		return false, err
	}
	typeKey := "TASK"
	slotKey := slotType(it.Slot)
	if it.Kind == "dayshift" {
		typeKey = "DAYSHIFT"
		slotKey = "DAYSHIFT"
	}
	return prefs[typeKey] && prefs[slotKey], nil
}

func slotType(slot sql.NullString) string {
	switch slot.String {
	case "morning":
		return "TASK_MORNING"
	case "evening":
		return "TASK_EVENING"
	default:
		return "TASK"
	}
}

func claimWebPushDelivery(db *sql.DB, key string, userID int64) (bool, error) {
	result, err := db.Exec(`INSERT INTO notification_deliveries (delivery_key, channel, user_id, status, attempts)
		VALUES (?, 'webpush', ?, 'sending', 1)
		ON DUPLICATE KEY UPDATE
		attempts = IF(status='error', attempts+1, attempts),
		error_message = IF(status='error', NULL, error_message),
		status = IF(status='error', 'sending', status)`, key, userID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func finishWebPushDelivery(db *sql.DB, key string, sendErr error) error {
	status := "sent"
	var message any
	if sendErr != nil {
		status = "error"
		message = fmt.Sprintf("%.2000s", sendErr.Error())
	}
	_, err := db.Exec(`UPDATE notification_deliveries SET status=?, error_message=?, sent_at=IF(?='sent', NOW(), sent_at)
		WHERE delivery_key=? AND status='sending'`, status, message, status, key)
	return err
}
