package main

import (
	"database/sql"
	"fmt"
)

// A sending row is deliberately not reclaimed automatically: SMTP can accept a
// message before the process loses its connection. Operators can inspect it
// without a rerun sending the same message again.
func claimDelivery(db *sql.DB, key, channel string, userID int64) (bool, error) {
	result, err := db.Exec(`INSERT INTO notification_deliveries
		(delivery_key, channel, user_id, status, attempts)
		VALUES (?, ?, ?, 'sending', 1)
		ON DUPLICATE KEY UPDATE
		attempts = IF(status = 'error', attempts + 1, attempts),
		error_message = IF(status = 'error', NULL, error_message),
		status = IF(status = 'error', 'sending', status)`, key, channel, userID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func finishDelivery(db *sql.DB, key string, sendErr error) error {
	status := "sent"
	var message any
	if sendErr != nil {
		status = "error"
		message = fmt.Sprintf("%.2000s", sendErr.Error())
	}
	_, err := db.Exec(`UPDATE notification_deliveries
		SET status = ?, error_message = ?, sent_at = IF(? = 'sent', NOW(), sent_at)
		WHERE delivery_key = ? AND status = 'sending'`, status, message, status, key)
	return err
}
