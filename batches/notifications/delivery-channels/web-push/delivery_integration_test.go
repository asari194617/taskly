package main

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dcCommon "github.com/asari194617/taskly/batches/notifications/delivery-channels/common"
)

func TestNotificationKeyAndSlotType(t *testing.T) {
	if got := notificationKey(dcCommon.QueueItem{ID: 42}); got != "queue:42" {
		t.Fatalf("notificationKey fallback = %q", got)
	}
	if got := notificationKey(dcCommon.QueueItem{EventKey: sql.NullString{String: "event:1", Valid: true}}); got != "event:1" {
		t.Fatalf("notificationKey event = %q", got)
	}
	for slot, want := range map[string]string{"morning": "TASK_MORNING", "evening": "TASK_EVENING", "": "TASK"} {
		if got := slotType(sql.NullString{String: slot, Valid: slot != ""}); got != want {
			t.Errorf("slotType(%q) = %q, want %q", slot, got, want)
		}
	}
	if eligible, err := webPushEligible(nil, dcCommon.QueueItem{}); err != nil || eligible {
		t.Fatalf("missing user eligible=%v err=%v", eligible, err)
	}
}

func TestWebPushEligibilitySettingsAndBlocks(t *testing.T) {
	for _, tc := range []struct {
		name, kind, slot, disabled   string
		blocked, deviceEnabled, want bool
	}{
		{"enabled", "task", "morning", "", false, true, true},
		{"type_off", "task", "morning", "TASK", false, true, false},
		{"slot_off", "task", "morning", "TASK_MORNING", false, true, false},
		{"blocked", "task", "morning", "", true, true, false},
		{"device_off", "task", "morning", "", false, false, false},
		{"day_shift_ignores_task_setting", "dayshift", "", "TASK", false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectQuery("SELECT id, is_deleted FROM users WHERE id = ?").WithArgs(uint(7)).WillReturnRows(sqlmock.NewRows([]string{"id", "is_deleted"}).AddRow(7, false))
			block := 0
			if tc.blocked {
				block = 1
			}
			mock.ExpectQuery("SELECT COUNT\\(1\\) FROM blocks").WithArgs(uint(7)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(block))
			if !tc.blocked {
				mock.ExpectQuery("SELECT user_id, platform, MAX\\(is_enabled\\) FROM notification_devices").WillReturnRows(sqlmock.NewRows([]string{"user_id", "platform", "enabled"}).AddRow(7, "WEBPUSH", tc.deviceEnabled))
				types := sqlmock.NewRows([]string{"user_id", "notification_type", "is_enabled"})
				if tc.disabled != "" {
					types.AddRow(7, tc.disabled, false)
				}
				mock.ExpectQuery("SELECT user_id, notification_type, is_enabled FROM notification_type_prefs").WillReturnRows(types)
			}
			got, err := webPushEligible(db, dcCommon.QueueItem{Kind: tc.kind, UserID: sql.NullInt64{Int64: 7, Valid: true}, Slot: sql.NullString{String: tc.slot, Valid: tc.slot != ""}})
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
