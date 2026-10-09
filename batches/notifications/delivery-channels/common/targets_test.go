package common

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestFetchDueTasksWindowAndOrder(t *testing.T) {
	jst := time.FixedZone("JST", 9*60*60)
	for _, tc := range []struct {
		name, today, morningEnd, eveningEnd string
	}{
		{"ordinary", "2026-09-22", "2026-09-24", "2026-09-25"},
		{"month boundary", "2026-09-30", "2026-10-02", "2026-10-03"},
		{"year boundary", "2026-12-30", "2027-01-01", "2027-01-02"},
		{"leap year", "2028-02-27", "2028-02-29", "2028-03-01"},
	} {
		for _, hour := range []int{7, 17} {
			t.Run(fmt.Sprintf("%s/%02d", tc.name, hour), func(t *testing.T) {
				db, mock, err := sqlmock.New()
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				start, err := time.ParseInLocation(time.DateOnly, tc.today, jst)
				if err != nil {
					t.Fatal(err)
				}
				endDate := tc.morningEnd
				query := `FROM tasks t LEFT JOIN subjects s ON s.id = t.subject_id WHERE t.deadline >= \? AND t.deadline < \? ORDER BY t.deadline ASC`
				args := []driver.Value{tc.today, endDate}
				todayUntil := "P5"
				if hour == 17 {
					endDate = tc.eveningEnd
					query = `FROM tasks t LEFT JOIN subjects s ON s.id = t.subject_id WHERE t.deadline >= \? AND t.deadline < \? AND \(t.deadline > \? OR t.deadline_until IN \('AFTER_SCHOOL', 'NONE'\)\) ORDER BY t.deadline ASC`
					args = []driver.Value{tc.today, endDate, tc.today}
					todayUntil = "AFTER_SCHOOL"
				}
				end, err := time.ParseInLocation(time.DateOnly, endDate, jst)
				if err != nil {
					t.Fatal(err)
				}
				now := start.Add(time.Duration(hour) * time.Hour)
				// The exclusive end includes tomorrow in the morning and the day
				// after tomorrow in the evening. Today evening keeps only deadlines
				// marked after school or with no time specified. Date strings preserve
				// JST calendar boundaries even if the DB connection uses UTC.
				mock.ExpectQuery(query).
					WithArgs(args...).WillReturnRows(sqlmock.NewRows([]string{
					"id", "added_user_id", "subject_id", "subject_name", "type", "title", "description", "deadline", "deadline_since", "deadline_until", "is_require_hand_in", "is_self_check_required",
				}).AddRow(1, 2, 3, "Math", "OTHER", "Today", nil, start, "", todayUntil, false, true).
					AddRow(2, 2, 3, "Math", "OTHER", "Final included day", nil, end.AddDate(0, 0, -1), "", "P5", false, true))
				tasks, err := FetchDueTasks(db, now)
				if err != nil {
					t.Fatal(err)
				}
				if len(tasks) != 2 || tasks[0].ID != 1 || tasks[1].ID != 2 || tasks[0].Description != nil || !tasks[0].IsSelfCheckRequired {
					t.Fatalf("tasks=%+v", tasks)
				}
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestCoreFormattingHelpers(t *testing.T) {
	if got := BuildDSN(MySQLConfig{User: "taskly", Password: "secret", Database: "app"}); got != "taskly:secret@tcp(localhost:3306)/app?parseTime=true&charset=utf8mb4&loc=Local" {
		t.Fatalf("BuildDSN() = %q", got)
	}
	if got := HostPort("db", "3307"); got != "db:3307" {
		t.Fatalf("HostPort() = %q", got)
	}
	if ValueOr("  ", "fallback") != "fallback" || ValueOr("value", "fallback") != "value" {
		t.Fatal("ValueOr did not apply blank-value defaults")
	}

	for input, want := range map[string]int{"P1": 1, " p12 ": 12, "P0": 0, "lunch": 0} {
		if got := PeriodIndex(input); got != want {
			t.Errorf("PeriodIndex(%q) = %d, want %d", input, got, want)
		}
	}
	for input, want := range map[string]string{
		"ONLINE": "オンライン", "face_to_face": "対面", "QUIZ": "小テスト",
		"PRESENTATION": "発表", "SELF_STUDY": "自主学習", "OTHER": "",
	} {
		if got := DisplayTaskTypeJaLocal(input); got != want {
			t.Errorf("DisplayTaskTypeJaLocal(%q) = %q, want %q", input, got, want)
		}
	}
	badges := BuildBadges("QUIZ", true, true)
	if got := BadgeLabels(badges); len(got) != 3 || got[0] != "小テスト" || got[2] != "自己採点" {
		t.Fatalf("BadgeLabels() = %v", got)
	}
	if got := BadgesToJSON(badges); got != `[{"label":"小テスト","variant":"gray"},{"label":"提出必須","variant":"red"},{"label":"自己採点","variant":"blue"}]` {
		t.Fatalf("BadgesToJSON() = %s", got)
	}
}

func TestDeadlineLabels(t *testing.T) {
	deadline := time.Date(2026, time.September, 24, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct{ since, until, want string }{
		{"", "MORNING", "9/24 朝まで"},
		{"", "LUNCH", "9/24 昼休みまで"},
		{"", "AFTER_SCHOOL", "9/24 放課後まで"},
		{"", "P3", "9/24 3限まで"},
		{"P2", "P4", "9/24 2〜4限まで"},
		{"P4", "P2", "9/24 まで"},
	} {
		if got := DeadlineLabel(tc.since, tc.until, deadline); got != tc.want {
			t.Errorf("DeadlineLabel(%q, %q) = %q, want %q", tc.since, tc.until, got, tc.want)
		}
	}
}

func TestLoadEligibleUserAndPrefs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, _, eligible, err := LoadEligibleUserAndPrefs(db, 0, UserDevicePrefs{}, nil); err != nil || eligible {
		t.Fatalf("zero user eligible=%v err=%v", eligible, err)
	}
	mock.ExpectQuery("SELECT id, is_deleted FROM users WHERE id = \\?").WithArgs(uint(7)).WillReturnRows(sqlmock.NewRows([]string{"id", "is_deleted"}).AddRow(7, false))
	mock.ExpectQuery("SELECT COUNT\\(1\\) FROM blocks").WithArgs(uint(7)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT user_id, platform, MAX\\(is_enabled\\) FROM notification_devices WHERE user_id = \\?").WithArgs(uint(7)).WillReturnRows(sqlmock.NewRows([]string{"user_id", "platform", "enabled"}).AddRow(7, "email", false))
	mock.ExpectQuery("SELECT user_id, notification_type, is_enabled FROM notification_type_prefs WHERE user_id = \\?").WithArgs(uint(7)).WillReturnRows(sqlmock.NewRows([]string{"user_id", "type", "enabled"}).AddRow(7, " task ", false))
	device, prefs, eligible, err := LoadEligibleUserAndPrefs(db, 7, UserDevicePrefs{Email: true, WebPush: true}, map[string]bool{"TASK": true})
	if err != nil || !eligible || device.Email || !device.WebPush || prefs["TASK"] {
		t.Fatalf("device=%+v prefs=%v eligible=%v err=%v", device, prefs, eligible, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDatabaseErrorsAreReturned(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	want := errors.New("database unavailable")
	mock.ExpectQuery("FROM tasks").WillReturnError(want)
	if _, err := FetchDueTasks(db, time.Now()); !errors.Is(err, want) {
		t.Fatalf("FetchDueTasks error = %v", err)
	}
	mock.ExpectQuery("SELECT COUNT\\(1\\) FROM blocks").WithArgs(uint(9)).WillReturnError(want)
	if _, err := IsUserBlocked(db, 9); !errors.Is(err, want) {
		t.Fatalf("IsUserBlocked error = %v", err)
	}
	mock.ExpectQuery("SELECT id, is_deleted FROM users").WillReturnError(want)
	if _, _, _, err := LoadEligibleUsersAndPrefs(db, UserDevicePrefs{}, nil); !errors.Is(err, want) {
		t.Fatalf("LoadEligibleUsersAndPrefs error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadEligibleUsersAndPrefsDefaultsAndOverrides(t *testing.T) {
	for _, tc := range []struct {
		name                                          string
		defaults                                      UserDevicePrefs
		types                                         map[string]bool
		wantEmail, wantWebPush, wantTask, wantMorning bool
	}{
		{"queue", UserDevicePrefs{Email: true}, map[string]bool{"TASK": true, "TASK_MORNING": true}, true, true, false, true},
		{"email", UserDevicePrefs{Email: true, WebPush: true}, map[string]bool{}, true, true, false, false},
		{"discord", UserDevicePrefs{Email: true}, map[string]bool{"TASK": true}, true, true, false, false},
		{"webpush", UserDevicePrefs{}, map[string]bool{"TASK": true, "TASK_MORNING": true}, false, true, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectQuery("SELECT id, is_deleted FROM users").WillReturnRows(sqlmock.NewRows([]string{"id", "is_deleted"}).AddRow(1, false).AddRow(2, true))
			mock.ExpectQuery("SELECT COUNT\\(1\\) FROM blocks").WithArgs(uint(1)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			mock.ExpectQuery("SELECT user_id, platform, MAX\\(is_enabled\\) FROM notification_devices").WillReturnRows(sqlmock.NewRows([]string{"user_id", "platform", "enabled"}).AddRow(1, "WEBPUSH", true))
			mock.ExpectQuery("SELECT user_id, notification_type, is_enabled FROM notification_type_prefs").WillReturnRows(sqlmock.NewRows([]string{"user_id", "notification_type", "enabled"}).AddRow(1, "TASK", false))
			ids, device, types, err := LoadEligibleUsersAndPrefs(db, tc.defaults, tc.types)
			if err != nil {
				t.Fatal(err)
			}
			if len(ids) != 1 || ids[0] != 1 || device[1].Email != tc.wantEmail || device[1].WebPush != tc.wantWebPush || types[1]["TASK"] != tc.wantTask || types[1]["TASK_MORNING"] != tc.wantMorning {
				t.Fatalf("ids=%v device=%v types=%v", ids, device, types)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
