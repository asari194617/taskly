package common

import (
	"database/sql"
	"strings"
)

// LoadEligibleUsersAndPrefs returns non-deleted/non-blocked users and their notification prefs.
// Caller provides per-channel defaults for device prefs and type prefs.
func LoadEligibleUsersAndPrefs(db *sql.DB, defaultDevice UserDevicePrefs, defaultTypePrefs map[string]bool) ([]uint, map[uint]UserDevicePrefs, map[uint]map[string]bool, error) {
	ids := make([]uint, 0, 1024)
	device := make(map[uint]UserDevicePrefs, 1024)
	typePrefs := make(map[uint]map[string]bool, 1024)
	rows, err := db.Query(`SELECT id, is_deleted FROM users`)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uint
		var del bool
		if err := rows.Scan(&id, &del); err != nil {
			return nil, nil, nil, err
		}
		if del {
			continue
		}
		blocked, err := IsUserBlocked(db, id)
		if err != nil {
			return nil, nil, nil, err
		}
		if blocked {
			continue
		}
		ids = append(ids, id)
		// copy defaults
		device[id] = UserDevicePrefs{Email: defaultDevice.Email, WebPush: defaultDevice.WebPush}
		// shallow copy defaultTypePrefs into a new map instance
		m := make(map[string]bool, len(defaultTypePrefs))
		for k, v := range defaultTypePrefs {
			m[k] = v
		}
		typePrefs[id] = m
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, err
	}
	rows2, err := db.Query(`SELECT user_id, platform, is_enabled FROM notification_devices`)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var uid uint
		var platform string
		var enabled bool
		if err := rows2.Scan(&uid, &platform, &enabled); err != nil {
			return nil, nil, nil, err
		}
		if _, ok := device[uid]; !ok {
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
	if err := rows2.Err(); err != nil {
		return nil, nil, nil, err
	}
	rows3, err := db.Query(`SELECT user_id, notification_type, is_enabled FROM notification_type_prefs`)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows3.Close()
	for rows3.Next() {
		var uid uint
		var t string
		var en bool
		if err := rows3.Scan(&uid, &t, &en); err != nil {
			return nil, nil, nil, err
		}
		m := typePrefs[uid]
		if m == nil {
			continue
		}
		m[strings.ToUpper(strings.TrimSpace(t))] = en
		typePrefs[uid] = m
	}
	if err := rows3.Err(); err != nil {
		return nil, nil, nil, err
	}
	return ids, device, typePrefs, nil
}
