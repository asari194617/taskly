package main

import (
	"database/sql"
	"log"
	"strings"
	"time"

	dcCommon "github.com/asari194617/taskly/batches/notifications/delivery-channels/common"
)

// computeLiveDigestGroups collects due tasks and expands them to per-user email digests
func computeLiveDigestGroups(db *sql.DB) (map[int64][]QueueItem, error) {
	now := time.Now()
	if loc, err := time.LoadLocation("Asia/Tokyo"); err == nil {
		now = now.In(loc)
	}
	tasks, err := fetchDueTasks(db, now)
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return map[int64][]QueueItem{}, nil
	}
	ids, devicePrefs, typePrefs := loadEligibleUsersAndPrefs(db)
	groups := make(map[int64][]QueueItem, len(ids))
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
		items := make([]QueueItem, 0, len(tasks))
		for _, t := range tasks {
			items = append(items, QueueItem{
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

// fetchDueTasks returns tasks whose effective deadline is within [now, end of the day after tomorrow]
func fetchDueTasks(db *sql.DB, now time.Time) ([]dcCommon.Task, error) {
	startDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endDate := time.Date(now.Year(), now.Month(), now.Day()+2, 0, 0, 0, 0, now.Location())
	rows, err := db.Query(
		"SELECT t.id, t.added_user_id, t.subject_id, COALESCE(s.name_ja, '') AS subject_name, t.type, t.title, t.description, t.deadline, t.deadline_since, t.deadline_until, t.is_require_hand_in, t.is_self_check_required "+
			"FROM tasks t LEFT JOIN subjects s ON s.id = t.subject_id WHERE t.deadline BETWEEN ? AND ? ORDER BY t.deadline ASC",
		startDate, endDate,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]dcCommon.Task, 0, 256)
	for rows.Next() {
		var t dcCommon.Task
		var desc sql.NullString
		if err := rows.Scan(&t.ID, &t.AddedUserID, &t.SubjectID, &t.SubjectName, &t.Type, &t.Title, &desc, &t.Deadline, &t.DeadlineSince, &t.DeadlineUntil, &t.IsRequireHandIn, &t.IsSelfCheckRequired); err != nil {
			continue
		}
		if desc.Valid {
			t.Description = &desc.String
		}
		out = append(out, t)
	}
	return out, nil
}

// loadEligibleUsersAndPrefs returns non-deleted/non-blocked users and their notification prefs. Defaults applied.
func loadEligibleUsersAndPrefs(db *sql.DB) ([]uint, map[uint]dcCommon.UserDevicePrefs, map[uint]map[string]bool) {
	ids := make([]uint, 0, 1024)
	device := make(map[uint]dcCommon.UserDevicePrefs, 1024)
	typePrefs := make(map[uint]map[string]bool, 1024)
	rows, err := db.Query(`SELECT id, is_deleted FROM users`)
	if err != nil {
		log.Println("load users:", err)
		return ids, device, typePrefs
	}
	defer rows.Close()
	for rows.Next() {
		var id uint
		var del bool
		if err := rows.Scan(&id, &del); err != nil {
			continue
		}
		if del {
			continue
		}
		if dcCommon.IsUserBlocked(db, id) {
			continue
		}
		ids = append(ids, id)
		device[id] = dcCommon.UserDevicePrefs{Email: true, WebPush: true}
		typePrefs[id] = make(map[string]bool)
	}
	rows2, err := db.Query(`SELECT user_id, platform, is_enabled FROM notification_devices`)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var uid uint
			var platform string
			var enabled bool
			if err := rows2.Scan(&uid, &platform, &enabled); err != nil {
				continue
			}
			p := device[uid]
			switch strings.ToUpper(strings.TrimSpace(platform)) {
			case "EMAIL":
				p.Email = enabled
			case "WEBPUSH":
				p.WebPush = enabled
			}
			device[uid] = p
		}
	}
	rows3, err := db.Query(`SELECT user_id, notification_type, is_enabled FROM notification_type_prefs`)
	if err == nil {
		defer rows3.Close()
		for rows3.Next() {
			var uid uint
			var t string
			var en bool
			if err := rows3.Scan(&uid, &t, &en); err != nil {
				continue
			}
			m := typePrefs[uid]
			m[strings.ToUpper(strings.TrimSpace(t))] = en
			typePrefs[uid] = m
		}
	}
	return ids, device, typePrefs
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
