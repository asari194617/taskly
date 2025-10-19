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

	if err := ensureAssignmentsTable(db); err != nil {
		log.Fatal(err)
	}

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

		// Find the last assigned date in DB, continue from next business day
		start := maxDate(dateOnly(c.BeginDate), nextBusinessDayAfter(lastAssignedDate(db, c.ID)))
		for d := start; !d.After(dateOnly(c.FinishDate)); d = d.Add(24 * time.Hour) {
			if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday || isHoliday(d) {
				continue
			}
			p := pairs[(weekdayIndex(d))%len(pairs)]
			if err := upsertGroupAssignment(db, d, p.ID, c.ID); err != nil {
				log.Println("assign:", err)
				continue
			}
			// also reflect members for the group on that date
			if err := upsertAssignmentMembers(db, d, p.UserIDs); err != nil {
				log.Println("assign members:", err)
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

func nextBusinessDayAfter(d time.Time) time.Time {
	x := dateOnly(d).Add(24 * time.Hour)
	for i := 0; i < 14; i++ {
		if x.Weekday() != time.Saturday && x.Weekday() != time.Sunday {
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
	rows, err := db.Query(`SELECT id, begin_date, finish_date, description FROM dayshift_cycles WHERE is_active = 1`)
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
	rows, err := db.Query(`SELECT holiday_date FROM business_calendar WHERE is_holiday = 1`)
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

func lastAssignedDate(db *sql.DB, cycleID int64) time.Time {
	row := db.QueryRow(`SELECT MAX(date) FROM day_shift_group_assignments WHERE cycle_id = ?`, cycleID)
	var t sql.NullTime
	_ = row.Scan(&t)
	if t.Valid {
		return dateOnly(t.Time)
	}
	return time.Time{}
}

func upsertGroupAssignment(db *sql.DB, date time.Time, groupID, cycleID int64) error {
	_, err := db.Exec(`INSERT INTO day_shift_group_assignments(date, group_id, cycle_id) VALUES(?,?,?) ON DUPLICATE KEY UPDATE group_id=VALUES(group_id), cycle_id=VALUES(cycle_id)`, dateOnly(date), groupID, cycleID)
	return err
}

func upsertAssignmentMembers(db *sql.DB, date time.Time, userIDs []int64) error {
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
	// clear existing
	if _, err = tx.Exec(`DELETE FROM day_shift_group_assignment_members WHERE date = ?`, dateOnly(date)); err != nil {
		return err
	}
	// insert all
	for _, uid := range userIDs {
		if _, err = tx.Exec(`INSERT INTO day_shift_group_assignment_members(date, user_id) VALUES(?,?)`, dateOnly(date), uid); err != nil {
			return err
		}
	}
	return nil
}

func ensureAssignmentsTable(db *sql.DB) error {
	// main assignment table: one group per date
	ddl1 := `CREATE TABLE IF NOT EXISTS day_shift_group_assignments (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		date DATE NOT NULL,
		group_id BIGINT NULL,
		cycle_id BIGINT NULL,
		UNIQUE KEY uniq_date (date)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`
	if _, err := db.Exec(ddl1); err != nil {
		return err
	}
	// members snapshot for a date
	ddl2 := `CREATE TABLE IF NOT EXISTS day_shift_group_assignment_members (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		date DATE NOT NULL,
		user_id BIGINT NOT NULL,
		KEY idx_date (date)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`
	if _, err := db.Exec(ddl2); err != nil {
		return err
	}
	return nil
}
