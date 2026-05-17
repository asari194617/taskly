package common

import (
	"database/sql"
	"time"
)

// FetchDueTasks returns tasks whose effective deadline is within [now, end of the day after tomorrow].
func FetchDueTasks(db *sql.DB, now time.Time) ([]Task, error) {
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
	out := make([]Task, 0, 256)
	for rows.Next() {
		var t Task
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
