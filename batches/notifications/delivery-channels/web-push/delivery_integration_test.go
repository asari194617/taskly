package main

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dcCommon "github.com/asari194617/taskly/batches/notifications/delivery-channels/common"
)

func TestWebPushEligibilitySettingsAndBlocks(t *testing.T) {
	for _, tc := range []struct {
		name    string
		kind    string
		slot    string
		typeKey string
		slotKey string
		count   int
		want    bool
	}{
		{"enabled", "task", "morning", "TASK", "TASK_MORNING", 1, true},
		{"type_off_or_blocked", "task", "morning", "TASK", "TASK_MORNING", 0, false},
		{"day_shift_ignores_task_setting", "dayshift", "", "DAYSHIFT", "DAYSHIFT", 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectQuery("SELECT COUNT\\(1\\) FROM users").WithArgs(int64(7), tc.typeKey, tc.slotKey).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tc.count))
			got, err := webPushEligible(db, dcCommon.QueueItem{
				Kind: tc.kind, UserID: sql.NullInt64{Int64: 7, Valid: true},
				Slot: sql.NullString{String: tc.slot, Valid: tc.slot != ""},
			})
			if err != nil || got != tc.want {
				t.Fatalf("eligible=%v err=%v", got, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSubscriptionFailureRetriesWithoutResendingSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	key := "task:2026-09-21:morning:1:webpush:7:sub:abc"
	mock.ExpectExec("INSERT INTO notification_deliveries").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE notification_deliveries").WithArgs("error", "webpush HTTP 503", "error", key).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO notification_deliveries").WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("UPDATE notification_deliveries").WithArgs("sent", nil, "sent", key).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO notification_deliveries").WillReturnResult(sqlmock.NewResult(0, 0))
	claimed, err := claimWebPushDelivery(db, key, 7)
	if err != nil || !claimed {
		t.Fatalf("first claim=%v err=%v", claimed, err)
	}
	if err := finishWebPushDelivery(db, key, errors.New("webpush HTTP 503")); err != nil {
		t.Fatal(err)
	}
	claimed, err = claimWebPushDelivery(db, key, 7)
	if err != nil || !claimed {
		t.Fatalf("retry claim=%v err=%v", claimed, err)
	}
	if err := finishWebPushDelivery(db, key, nil); err != nil {
		t.Fatal(err)
	}
	claimed, err = claimWebPushDelivery(db, key, 7)
	if err != nil || claimed {
		t.Fatalf("sent claim=%v err=%v", claimed, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
