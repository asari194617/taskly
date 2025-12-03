package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Cycle struct {
	ID          int64
	BeginDate   time.Time
	FinishDate  time.Time
	Description string
}

type Pair struct {
	ID       int64
	CycleID  int64
	UserIDs  []int64
	Position int
}

func main() {
	dsn := os.Getenv("MYSQL_USER") + ":" + os.Getenv("MYSQL_PASSWORD") + "@tcp(" + hostPort(os.Getenv("DB_HOST"), os.Getenv("DB_PORT")) + ")/" + os.Getenv("MYSQL_DATABASE") + "?parseTime=true&charset=utf8mb4&loc=Local"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	cycles, err := fetchActiveCycles(db)
	if err != nil {
		log.Fatal(err)
	}
	if len(cycles) == 0 {
		log.Println("no active cycles")
		return
	}

	holidays, err := fetchHolidays(db)
	if err != nil {
		log.Fatal(err)
	}
	isHoliday := func(d time.Time) bool { _, ok := holidays[dateOnly(d)]; return ok }

	for _, c := range cycles {
		pairs, err := fetchGroupsWithMembers(db, c.ID)
		if err != nil {
			log.Println("pairs:", err)
			continue
		}
		if len(pairs) == 0 {
			log.Println("no pairs for cycle", c.ID)
			continue
		}

		// Find the last assigned date in DB for this cycle, continue from next business day
		start := maxDate(dateOnly(c.BeginDate), nextBusinessDayAfter(lastAssignedDateForCycle(db, c.ID), isHoliday))
		for d := start; !d.After(dateOnly(c.FinishDate)); d = d.Add(24 * time.Hour) {
			if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday || isHoliday(d) {
				continue
			}
			p := pairs[(weekdayIndex(d))%len(pairs)]
			asgID, err := upsertAssignment(db, d, c.ID)
			if err != nil {
				log.Println("assign:", err)
				continue
			}
			// also reflect members for the group on that date into day_shift_assignments_users
			if err := upsertAssignmentMembers(db, asgID, p.UserIDs); err != nil {
				log.Println("assign members:", err)
			}
		}
	}

	// If rotations end within 5 business days or there are no assignments, generate for the next cycle
	today := dateOnly(time.Now())
	lastAll := lastAssignedDateAll(db)
	if lastAll.IsZero() || businessDaysBetween(today, lastAll, isHoliday) <= 5 {
		// find next cycle whose begin_date > lastAll (or today if none)
		base := lastAll
		if base.IsZero() || base.Before(today) {
			base = today
		}
		next, ok := findNextCycle(db, base)
		if ok {
			pairs, err := fetchGroupsWithMembers(db, next.ID)
			if err == nil && len(pairs) > 0 {
				for d := dateOnly(next.BeginDate); !d.After(dateOnly(next.FinishDate)); d = d.Add(24 * time.Hour) {
					if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday || isHoliday(d) {
						continue
					}
					p := pairs[(weekdayIndex(d))%len(pairs)]
					asgID, err := upsertAssignment(db, d, next.ID)
					if err != nil {
						log.Println("next assign:", err)
						continue
					}
					if err := upsertAssignmentMembers(db, asgID, p.UserIDs); err != nil {
						log.Println("next members:", err)
					}
				}
			}
		}
	}
}

func hostPort(h, p string) string {
	if strings.TrimSpace(p) == "" {
		p = "3306"
	}
	return fmt.Sprintf("%s:%s", valueOr(h, "localhost"), p)
}

func valueOr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func nextBusinessDayAfter(d time.Time, isHoliday func(time.Time) bool) time.Time {
	x := dateOnly(d).Add(24 * time.Hour)
	for i := 0; i < 30; i++ {
		if x.Weekday() != time.Saturday && x.Weekday() != time.Sunday && !isHoliday(x) {
			return x
		}
		x = x.Add(24 * time.Hour)
	}
	return x
}

func maxDate(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func weekdayIndex(d time.Time) int { // Mon..Fri -> 0..4 (roll through pairs)
	i := int(d.Weekday()) - 1
	if i < 0 {
		i = 0
	}
	return i
}

func fetchActiveCycles(db *sql.DB) ([]Cycle, error) {
	// Server schema: table name is day_shift_cycles (no is_active column)
	// 直近以降（終了日が今日以降）のサイクルを対象
	rows, err := db.Query(`SELECT id, begin_date, finish_date, description FROM day_shift_cycles WHERE finish_date >= CURDATE() ORDER BY begin_date ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Cycle
	for rows.Next() {
		var c Cycle
		if err := rows.Scan(&c.ID, &c.BeginDate, &c.FinishDate, &c.Description); err != nil {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

func fetchGroupsWithMembers(db *sql.DB, cycleID int64) ([]Pair, error) {
	// groups
	gRows, err := db.Query(`SELECT id, cycle_id, position FROM day_shift_cycle_groups WHERE cycle_id = ? ORDER BY position ASC, id ASC`, cycleID)
	if err != nil {
		return nil, err
	}
	defer gRows.Close()
	var out []Pair
	for gRows.Next() {
		var id, cid int64
		var pos int
		if err := gRows.Scan(&id, &cid, &pos); err != nil {
			continue
		}
		// members for this group
		mRows, err := db.Query(`SELECT user_id FROM day_shift_cycle_group_members WHERE group_id = ? ORDER BY position ASC, id ASC`, id)
		if err != nil {
			return nil, err
		}
		ids := make([]int64, 0, 4)
		for mRows.Next() {
			var uid int64
			if err := mRows.Scan(&uid); err == nil {
				ids = append(ids, uid)
			}
		}
		_ = mRows.Close()
		out = append(out, Pair{ID: id, CycleID: cid, UserIDs: ids, Position: pos})
	}
	return out, nil
}

func fetchHolidays(db *sql.DB) (map[time.Time]struct{}, error) {
	// Server schema: business_calendar(date, is_business_day)
	// 祝日（非営業日）を取得
	rows, err := db.Query(`SELECT date FROM business_calendar WHERE is_business_day = 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[time.Time]struct{})
	for rows.Next() {
		var d time.Time
		if err := rows.Scan(&d); err != nil {
			continue
		}
		m[dateOnly(d)] = struct{}{}
	}
	return m, nil
}

func lastAssignedDateForCycle(db *sql.DB, cycleID int64) time.Time {
	row := db.QueryRow(`SELECT MAX(date) FROM day_shift_assignments WHERE cycle_id = ?`, cycleID)
	var t sql.NullTime
	_ = row.Scan(&t)
	if t.Valid {
		return dateOnly(t.Time)
	}
	return time.Time{}
}

// upsertAssignment creates or ensures an assignment row and returns its id
func upsertAssignment(db *sql.DB, date time.Time, cycleID int64) (int64, error) {
	// Use ON DUPLICATE KEY UPDATE to set LAST_INSERT_ID for retrieving id in both insert/update cases
	res, err := db.Exec(`INSERT INTO day_shift_assignments(date, cycle_id, status) VALUES(?,?, 'assigned') 
		ON DUPLICATE KEY UPDATE id=LAST_INSERT_ID(id), cycle_id=VALUES(cycle_id)`, dateOnly(date), cycleID)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func upsertAssignmentMembers(db *sql.DB, assignmentID int64, userIDs []int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		} else {
			_ = tx.Commit()
		}
	}()
	// clear existing members for the assignment id
	if _, err = tx.Exec(`DELETE FROM day_shift_assignments_users WHERE day_shift_assignments_id = ?`, assignmentID); err != nil {
		return err
	}
	// insert all
	for _, uid := range userIDs {
		if _, err = tx.Exec(`INSERT INTO day_shift_assignments_users(day_shift_assignments_id, user_id) VALUES(?,?)`, assignmentID, uid); err != nil {
			return err
		}
	}
	return nil
}

// isColumnUnsigned checks if a given column is defined as unsigned.
func isColumnUnsigned(db *sql.DB, table, column string) (bool, error) {
	row := db.QueryRow(`SELECT COLUMN_TYPE FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?`, table, column)
	var colType string
	if err := row.Scan(&colType); err != nil {
		return false, err
	}
	return strings.Contains(strings.ToLower(colType), "unsigned"), nil
}

func lastAssignedDateAll(db *sql.DB) time.Time {
	row := db.QueryRow(`SELECT MAX(date) FROM day_shift_assignments`)
	var t sql.NullTime
	_ = row.Scan(&t)
	if t.Valid {
		return dateOnly(t.Time)
	}
	return time.Time{}
}

func businessDaysBetween(start, end time.Time, isHoliday func(time.Time) bool) int {
	s := dateOnly(start)
	e := dateOnly(end)
	if e.Before(s) {
		return 0
	}
	cnt := 0
	for d := s; !d.After(e); d = d.Add(24 * time.Hour) {
		wd := d.Weekday()
		if wd == time.Saturday || wd == time.Sunday || isHoliday(d) {
			continue
		}
		cnt++
	}
	if cnt > 0 {
		return cnt - 1
	}
	return 0
}

func findNextCycle(db *sql.DB, after time.Time) (Cycle, bool) {
	row := db.QueryRow(`SELECT id, begin_date, finish_date, description FROM day_shift_cycles WHERE begin_date > ? ORDER BY begin_date ASC LIMIT 1`, after)
	var c Cycle
	if err := row.Scan(&c.ID, &c.BeginDate, &c.FinishDate, &c.Description); err != nil {
		return Cycle{}, false
	}
	return c, true
}
