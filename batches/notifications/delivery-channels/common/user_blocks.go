package common

import (
	"database/sql"
)

// IsUserBlocked checks active blocks for the user.
func IsUserBlocked(db *sql.DB, uid uint) (bool, error) {
	row := db.QueryRow(`SELECT COUNT(1) FROM blocks WHERE user_id = ? AND (expires_at IS NULL OR expires_at > NOW())`, uid)
	var cnt int
	if err := row.Scan(&cnt); err != nil {
		return false, err
	}
	return cnt > 0, nil
}
