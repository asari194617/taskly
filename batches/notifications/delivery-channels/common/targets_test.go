package common

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestFetchDueTasksWindowAndOrder(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 9, 22, 17, 0, 0, 0, time.UTC)
	start := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery("FROM tasks t LEFT JOIN subjects s.*ORDER BY t.deadline ASC").WithArgs(start, end).WillReturnRows(sqlmock.NewRows([]string{
		"id", "added_user_id", "subject_id", "subject_name", "type", "title", "description", "deadline", "deadline_since", "deadline_until", "is_require_hand_in", "is_self_check_required",
	}).AddRow(1, 2, 3, "Math", "OTHER", "Homework", nil, end, "", "P5", false, true))
	tasks, err := FetchDueTasks(db, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].ID != 1 || tasks[0].Description != nil || !tasks[0].IsSelfCheckRequired {
		t.Fatalf("tasks=%+v", tasks)
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
