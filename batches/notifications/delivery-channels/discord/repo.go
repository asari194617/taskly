package main

import (
	"database/sql"
	"strings"
	"time"

	dcCommon "github.com/asari194617/taskly/batches/notifications/delivery-channels/common"
)

func fetchDueTasks(db *sql.DB, now time.Time) ([]dcCommon.Task, error) {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := time.Date(now.Year(), now.Month(), now.Day()+2, 0, 0, 0, 0, now.Location())
	rows, err := db.Query(
		"SELECT t.id, t.added_user_id, t.subject_id, COALESCE(s.name_ja, '') AS subject_name, t.type, t.title, t.description, t.deadline, t.deadline_since, t.deadline_until, t.is_require_hand_in, t.is_self_check_required "+
			"FROM tasks t LEFT JOIN subjects s ON s.id = t.subject_id WHERE t.deadline BETWEEN ? AND ? ORDER BY t.deadline ASC",
		start, end,
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

func loadEligibleUsersAndPrefs(db *sql.DB) ([]uint, map[uint]dcCommon.UserDevicePrefs, map[uint]map[string]bool) {
	ids := make([]uint, 0, 1024)
	device := make(map[uint]dcCommon.UserDevicePrefs, 1024)
	typePrefs := make(map[uint]map[string]bool, 1024)
	rows, err := db.Query(`SELECT id, is_deleted FROM users`)
	if err != nil {
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
		device[id] = dcCommon.UserDevicePrefs{Email: true, WebPush: false}
		typePrefs[id] = map[string]bool{"TASK": true, "DAYSHIFT": true}
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
