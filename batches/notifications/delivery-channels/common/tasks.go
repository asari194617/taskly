package common

import (
	"database/sql"
	"time"
)

// FetchDueTasks returns tasks due today or tomorrow in the morning. In the
// evening it returns tasks due tomorrow or the day after, plus today's tasks
// due after school. Tasks without a specified deadline time are excluded.
// Calendar days are evaluated in now's timezone (JST in notification jobs).
func FetchDueTasks(db *sql.DB, now time.Time) ([]Task, error) {
	startDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOffset := 2
	if now.Hour() >= 12 {
		endOffset = 3
	}
	endDate := startDate.AddDate(0, 0, endOffset)
	// Bind calendar dates as strings: the MySQL driver converts time.Time
	// parameters to the connection timezone (UTC on the production host),
	// which would shift JST midnight to the previous day.
	start := startDate.Format(time.DateOnly)
	query := "SELECT t.id, t.added_user_id, t.subject_id, COALESCE(s.name_ja, '') AS subject_name, t.type, t.title, t.description, t.deadline, t.deadline_since, t.deadline_until, t.is_require_hand_in, t.is_self_check_required " +
		"FROM tasks t LEFT JOIN subjects s ON s.id = t.subject_id WHERE t.deadline >= ? AND t.deadline < ? AND t.deadline_until <> 'NONE'"
	args := []any{start, endDate.Format(time.DateOnly)}
	if now.Hour() >= 12 {
		query += " AND (t.deadline > ? OR t.deadline_until = 'AFTER_SCHOOL')"
		args = append(args, start)
	}
	query += " ORDER BY t.deadline ASC"
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Task, 0, 256)
	for rows.Next() {
		var t Task
		var desc sql.NullString
		if err := rows.Scan(&t.ID, &t.AddedUserID, &t.SubjectID, &t.SubjectName, &t.Type, &t.Title, &desc, &t.Deadline, &t.DeadlineSince, &t.DeadlineUntil, &t.IsRequireHandIn, &t.IsSelfCheckRequired); err != nil {
			return nil, err
		}
		if desc.Valid {
			t.Description = &desc.String
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
