package jobs

import (
	"errors"
	"gorm.io/gorm"
	"sync/atomic"
	"testing"
	"time"
)

func expectWake(t *testing.T, w *Worker, want bool) {
	t.Helper()
	select {
	case <-w.wake:
		if !want {
			t.Fatal("unexpected worker notification")
		}
	default:
		if want {
			t.Fatal("missing worker notification")
		}
	}
}

func TestOutboxWakesOnlyAfterCommit(t *testing.T) {
	db := testDB(t)
	w := NewKindWorker(db, "task_discord", nil)
	defer w.subscribe()()
	other := NewKindWorker(db, "task_email", nil)
	defer other.subscribe()()
	if err := Transaction(db, func(tx *gorm.DB) error {
		if _, err := Enqueue(tx, "task_discord", map[string]int{"event": 1}, nil); err != nil {
			return err
		}
		expectWake(t, w, false)
		if err := Transaction(tx, func(nested *gorm.DB) error {
			_, err := Enqueue(nested, "task_discord", map[string]int{"event": 2}, nil)
			return err
		}); err != nil {
			return err
		}
		expectWake(t, w, false)
		// An inner rollback must not add its lane to the outer commit hints.
		rollback := errors.New("nested rollback")
		err := Transaction(tx, func(nested *gorm.DB) error {
			if _, err := Enqueue(nested, "task_email", map[string]int{"event": 3}, nil); err != nil {
				return err
			}
			return rollback
		})
		if !errors.Is(err, rollback) {
			return err
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	expectWake(t, w, true)
	expectWake(t, w, false)
	expectWake(t, other, false)
	var count int64
	if err := db.Model(&Job{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("rows=%d err=%v", count, err)
	}
}

func TestOutboxRollbackDoesNotWake(t *testing.T) {
	db := testDB(t)
	w := NewKindWorker(db, "task_discord", nil)
	defer w.subscribe()()
	rollback := errors.New("application mutation failed")
	err := Transaction(db, func(tx *gorm.DB) error {
		if _, err := Enqueue(tx, "task_discord", map[string]int{"event": 1}, nil); err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	expectWake(t, w, false)
	var count int64
	if err := db.Model(&Job{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("rows=%d err=%v", count, err)
	}
}

func TestPlainOuterTransactionDoesNotNotifyAtSavepoint(t *testing.T) {
	db := testDB(t)
	w := NewKindWorker(db, "task_discord", nil)
	defer w.subscribe()()
	rollback := errors.New("outer rollback")
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := Transaction(tx, func(nested *gorm.DB) error {
			_, err := Enqueue(nested, "task_discord", map[string]int{"event": 1}, nil)
			return err
		}); err != nil {
			return err
		}
		expectWake(t, w, false)
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	expectWake(t, w, false)
}

func TestStandaloneEnqueueAndRetryWakeOnlyTheirLane(t *testing.T) {
	db := testDB(t)
	w := NewKindWorker(db, "task_discord", nil)
	defer w.subscribe()()
	fallback := NewWorker(db, nil)
	fallback.ExcludeKinds = []string{"task_discord"}
	defer fallback.subscribe()()
	job, err := Enqueue(db, "task_discord", map[string]int{"event": 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	expectWake(t, w, true)
	expectWake(t, fallback, false)
	if err := db.Model(&Job{}).Where("id = ?", job.ID).Update("status", "failed").Error; err != nil {
		t.Fatal(err)
	}
	if err := Retry(db, job.ID); err != nil {
		t.Fatal(err)
	}
	expectWake(t, w, true)
	if _, err := Enqueue(db, "future_kind", map[string]int{"event": 2}, nil); err != nil {
		t.Fatal(err)
	}
	expectWake(t, fallback, true)
	expectWake(t, w, false)
}

func TestWorkerIdleThenEventDrainsBurstWithoutSecondPolling(t *testing.T) {
	db := testDB(t)
	var selects atomic.Int64
	if err := db.Callback().Query().After("gorm:query").Register("test:count_idle_queries", func(tx *gorm.DB) {
		if tx.Statement.Table == "integration_jobs" {
			selects.Add(1)
		}
	}); err != nil {
		t.Fatal(err)
	}
	var delivered atomic.Int64
	w := NewKindWorker(db, "task_discord", func(Job) error { delivered.Add(1); return nil })
	w.Start()
	defer w.Stop()
	deadline := time.Now().Add(time.Second)
	for selects.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if selects.Load() != 1 {
		t.Fatalf("startup selects=%d", selects.Load())
	}
	time.Sleep(1100 * time.Millisecond)
	if selects.Load() != 1 {
		t.Fatalf("idle worker still polls each second: selects=%d", selects.Load())
	}
	const burst = 20
	if err := Transaction(db, func(tx *gorm.DB) error {
		for i := 0; i < burst; i++ {
			if _, err := Enqueue(tx, "task_discord", map[string]int{"event": i}, nil); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(time.Second)
	for delivered.Load() < burst && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if delivered.Load() != burst {
		t.Fatalf("delivered=%d/%d", delivered.Load(), burst)
	}
}

func TestRecoveryFindsCommittedJobWithoutNotification(t *testing.T) {
	db := testDB(t)
	delivered := make(chan struct{}, 1)
	w := NewKindWorker(db, "task_discord", func(Job) error { delivered <- struct{}{}; return nil })
	w.recoveryEvery = 20 * time.Millisecond
	w.Start()
	defer w.Stop()
	// A direct write simulates another API instance or a crash after commit.
	if err := db.Create(&Job{Kind: "task_discord", Payload: "{}", Status: "pending", AvailableAt: time.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	select {
	case <-delivered:
	case <-time.After(time.Second):
		t.Fatal("recovery missed a committed job")
	}
}

func TestNotificationsDoNotCrossDatabasePools(t *testing.T) {
	db := testDB(t)
	t.Run("other_pool", func(t *testing.T) {
		otherDB := testDB(t)
		w := NewKindWorker(otherDB, "task_discord", nil)
		defer w.subscribe()()
		if _, err := Enqueue(db, "task_discord", map[string]int{"event": 1}, nil); err != nil {
			t.Fatal(err)
		}
		expectWake(t, w, false)
	})
}
