package main

import (
	"database/sql"
	"strings"
	"time"

	dcCommon "github.com/asari194617/taskly/batches/notifications/delivery-channels/common"
)

// computeLiveDigestGroupsAt collects due tasks for a single delivery slot.
func computeLiveDigestGroupsAt(db *sql.DB, now time.Time) (map[int64][]dcCommon.QueueItem, error) {
	tasks, err := dcCommon.FetchDueTasks(db, now)
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return map[int64][]dcCommon.QueueItem{}, nil
	}
	ids, devicePrefs, typePrefs, err := dcCommon.LoadEligibleUsersAndPrefs(db, dcCommon.UserDevicePrefs{Email: true, WebPush: true}, map[string]bool{})
	if err != nil {
		return nil, err
	}
	slot := "TASK_EVENING"
	if now.Hour() < 12 {
		slot = "TASK_MORNING"
	}
	groups := make(map[int64][]dcCommon.QueueItem, len(ids))
	for _, uid := range ids {
		blocked, err := dcCommon.IsUserBlocked(db, uid)
		if err != nil {
			return nil, err
		}
		if blocked {
			continue
		}
		dp := devicePrefs[uid]
		if !dp.Email {
			continue
		}
		tp := typePrefs[uid]
		allowTask := true
		if v, ok := tp["TASK"]; ok {
			allowTask = v
		}
		if !allowTask {
			continue
		}
		if enabled, ok := tp[slot]; ok && !enabled {
			continue
		}
		items := make([]dcCommon.QueueItem, 0, len(tasks))
		for _, t := range tasks {
			items = append(items, dcCommon.QueueItem{
				ID:        int64(t.ID),
				WhenLabel: dcCommon.DeadlineLabel(t.DeadlineSince, t.DeadlineUntil, t.Deadline),
				Subject:   dcCommon.ValueOr(t.SubjectName, ""),
				Title:     t.Title,
				Body:      buildBodyFromTask(t),
				Meta:      sql.NullString{Valid: t.Description != nil && strings.TrimSpace(*t.Description) != "", String: dcCommon.ValueOr(ptrStr(t.Description), "")},
				Badges:    sql.NullString{Valid: true, String: buildBadgesJSON(t)},
				UserID:    sql.NullInt64{Valid: true, Int64: int64(uid)},
			})
		}
		if len(items) > 0 {
			groups[int64(uid)] = items
		}
	}
	return groups, nil
}

func resolveRecipientEmail(db *sql.DB, userID sql.NullInt64) (string, string, error) {
	if !userID.Valid || userID.Int64 == 0 {
		return "", "", nil
	}
	row := db.QueryRow(`SELECT email, notification_lang FROM users WHERE id = ? AND is_deleted = 0`, userID.Int64)
	var email sql.NullString
	var lang sql.NullString
	if err := row.Scan(&email, &lang); err != nil {
		return "", "", err
	}
	if !email.Valid {
		return "", "", nil
	}
	return email.String, lang.String, nil
}

func markQueueProcessed(db *sql.DB, id int64) error {
	_, err := db.Exec(`UPDATE notifications_queue SET processed_at = NOW(), status='sent' WHERE id = ?`, id)
	return err
}

func markQueueError(db *sql.DB, id int64, e error) error {
	msg := e.Error()
	if len(msg) > 2000 {
		msg = msg[:2000]
	}
	_, err := db.Exec(`UPDATE notifications_queue SET processed_at = NOW(), status='error', error_message=? WHERE id = ?`, msg, id)
	return err
}
