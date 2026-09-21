package main

import (
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestDigestRecipientsRespectSettingsAndBlocks(t *testing.T) {
	for _, tc := range []struct {
		name     string
		hour     int
		disabled string
	}{
		{"morning", 7, "TASK_MORNING"},
		{"evening", 17, "TASK_EVENING"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			now := time.Date(2026, 9, 21, tc.hour, 0, 0, 0, time.UTC)
			mock.ExpectQuery("SELECT t.id").WillReturnRows(sqlmock.NewRows([]string{
				"id", "added_user_id", "subject_id", "subject_name", "type", "title", "description", "deadline", "deadline_since", "deadline_until", "is_require_hand_in", "is_self_check_required",
			}).AddRow(1, 1, 1, "Math", "OTHER", "Homework", nil, now.Add(time.Hour), "", "", false, false))
			mock.ExpectQuery("SELECT id, is_deleted FROM users").WillReturnRows(sqlmock.NewRows([]string{"id", "is_deleted"}).
				AddRow(1, false).AddRow(2, false).AddRow(3, false).AddRow(4, false))
			for _, blocked := range []int{0, 0, 0, 1} {
				mock.ExpectQuery("SELECT COUNT\\(1\\) FROM blocks").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(blocked))
			}
			mock.ExpectQuery("SELECT user_id, platform, MAX\\(is_enabled\\) FROM notification_devices").WillReturnRows(
				sqlmock.NewRows([]string{"user_id", "platform", "is_enabled"}).AddRow(3, "EMAIL", false))
			mock.ExpectQuery("SELECT user_id, notification_type, is_enabled FROM notification_type_prefs").WillReturnRows(
				sqlmock.NewRows([]string{"user_id", "notification_type", "is_enabled"}).AddRow(2, tc.disabled, false))
			for range 3 {
				mock.ExpectQuery("SELECT COUNT\\(1\\) FROM blocks").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			}
			groups, err := computeLiveDigestGroupsAt(db, now)
			if err != nil {
				t.Fatal(err)
			}
			if len(groups) != 1 || len(groups[1]) != 1 {
				t.Fatalf("wanted only eligible user 1, got %v", groups)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDeliveryFailureCanRetryButSentCannot(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	key := "task:2026-09-21:morning:email:1"
	mock.ExpectExec("INSERT INTO notification_deliveries").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE notification_deliveries").WithArgs("error", "smtp unavailable", "error", key).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO notification_deliveries").WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("UPDATE notification_deliveries").WithArgs("sent", nil, "sent", key).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO notification_deliveries").WillReturnResult(sqlmock.NewResult(0, 0))
	claimed, err := claimDelivery(db, key, "email", 1)
	if err != nil || !claimed {
		t.Fatalf("initial claim=%v err=%v", claimed, err)
	}
	if err := finishDelivery(db, key, errors.New("smtp unavailable")); err != nil {
		t.Fatal(err)
	}
	claimed, err = claimDelivery(db, key, "email", 1)
	if err != nil || !claimed {
		t.Fatalf("retry claim=%v err=%v", claimed, err)
	}
	if err := finishDelivery(db, key, nil); err != nil {
		t.Fatal(err)
	}
	claimed, err = claimDelivery(db, key, "email", 1)
	if err != nil || claimed {
		t.Fatalf("sent claim=%v err=%v", claimed, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
