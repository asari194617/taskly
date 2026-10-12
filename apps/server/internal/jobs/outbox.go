package jobs

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"gorm.io/gorm"
)

// The database is the durable outbox; wakeups are only a latency optimization.
// Recovery covers a crash between commit and notification, other API instances,
// delayed retries, expired leases, and writers that do not use Transaction.
const recoveryInterval = 30 * time.Second

type outboxContextKey struct{}
type outboxChanges struct{ kinds map[string]struct{} }

// Transaction saves application changes and jobs atomically, then wakes only
// the affected worker lanes after the outermost database commit succeeds.
func Transaction(db *gorm.DB, fn func(*gorm.DB) error) error {
	changes := &outboxChanges{kinds: make(map[string]struct{})}
	ctx := context.WithValue(db.Statement.Context, outboxContextKey{}, changes)
	if err := db.WithContext(ctx).Transaction(fn); err != nil {
		return err
	}
	if _, nested := db.Statement.ConnPool.(gorm.TxCommitter); nested {
		// A savepoint release is not a commit. Merge only successful nested
		// changes into the enclosing outbox transaction; plain GORM outer
		// transactions rely on recovery after their eventual commit.
		if parent, ok := db.Statement.Context.Value(outboxContextKey{}).(*outboxChanges); ok {
			for kind := range changes.kinds {
				parent.kinds[kind] = struct{}{}
			}
		}
		return nil
	}
	for kind := range changes.kinds {
		Notify(db, kind)
	}
	return nil
}

func recordWake(db *gorm.DB, kind string) {
	if changes, ok := db.Statement.Context.Value(outboxContextKey{}).(*outboxChanges); ok {
		changes.kinds[kind] = struct{}{}
		return
	}
	if _, transactional := db.Statement.ConnPool.(gorm.TxCommitter); !transactional {
		Notify(db, kind)
	}
}

var wakeups = struct {
	sync.Mutex
	workers map[*sql.DB]map[*Worker]struct{}
}{workers: make(map[*sql.DB]map[*Worker]struct{})}

func (w *Worker) subscribe() func() {
	pool, err := w.DB.DB()
	if err != nil {
		return func() {}
	}
	wakeups.Lock()
	if wakeups.workers[pool] == nil {
		wakeups.workers[pool] = make(map[*Worker]struct{})
	}
	wakeups.workers[pool][w] = struct{}{}
	wakeups.Unlock()
	return func() {
		wakeups.Lock()
		defer wakeups.Unlock()
		delete(wakeups.workers[pool], w)
		if len(wakeups.workers[pool]) == 0 {
			delete(wakeups.workers, pool)
		}
	}
}

// Notify coalesces in-process hints without blocking an API request. All lanes
// of the affected kind wake so a burst can use the configured concurrency.
// No notification is needed for durability and no Redis dependency is added.
func Notify(db *gorm.DB, kind string) {
	pool, err := db.DB()
	if err != nil {
		return
	}
	wakeups.Lock()
	defer wakeups.Unlock()
	for w := range wakeups.workers[pool] {
		if w.Kind != "" && w.Kind != kind {
			continue
		}
		excluded := false
		for _, known := range w.ExcludeKinds {
			if known == kind {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}
		select {
		case w.wake <- struct{}{}:
		default:
		}
	}
}
