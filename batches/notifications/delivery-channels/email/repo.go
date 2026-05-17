package main

import (
	"database/sql"
	"strings"
	"time"

	dcCommon "github.com/asari194617/taskly/batches/notifications/delivery-channels/common"
)

// computeLiveDigestGroups collects due tasks and expands them to per-user email digests
func computeLiveDigestGroups(db *sql.DB) (map[int64][]dcCommon.QueueItem, error) {
	now := time.Now()
	if loc, err := time.LoadLocation("Asia/Tokyo"); err == nil {
		now = now.In(loc)
	}
	tasks, err := dcCommon.FetchDueTasks(db, now)
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return map[int64][]dcCommon.QueueItem{}, nil
	}
	ids, devicePrefs, typePrefs := dcCommon.LoadEligibleUsersAndPrefs(db, dcCommon.UserDevicePrefs{Email: true, WebPush: true}, map[string]bool{})
	groups := make(map[int64][]dcCommon.QueueItem, len(ids))
	for _, uid := range ids {
		if dcCommon.IsUserBlocked(db, uid) {
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

func resolveRecipientEmail(db *sql.DB, userID sql.NullInt64) (string, error) {
	if !userID.Valid || userID.Int64 == 0 {
		return "", nil
	}
	row := db.QueryRow(`SELECT email FROM users WHERE id = ? AND is_deleted = 0`, userID.Int64)
	var email sql.NullString
	if err := row.Scan(&email); err != nil {
		return "", err
	}
	if !email.Valid {
		return "", nil
	}
	return email.String, nil
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
