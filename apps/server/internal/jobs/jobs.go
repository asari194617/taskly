package jobs

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxAttempts = 3

var ErrIdempotencyConflict = errors.New("idempotency key used for a different request")

type Job struct {
	ID          uint64 `gorm:"primaryKey"`
	Kind        string
	Payload     string
	Status      string
	Attempts    int
	AvailableAt time.Time
	LeaseUntil  *time.Time
	DedupKey    *string
	LastError   *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Job) TableName() string { return "integration_jobs" }

// Enqueue commits a job with the caller's transaction. A duplicate client key
// returns the original job; callers can safely repeat a timed-out request.
func Enqueue(db *gorm.DB, kind string, payload any, dedupKey *string) (Job, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return Job{}, err
	}
	job := Job{Kind: kind, Payload: string(data), Status: "pending", AvailableAt: time.Now(), DedupKey: dedupKey}
	if dedupKey != nil {
		var existing Job
		result := db.Where("dedup_key = ?", *dedupKey).Limit(1).Find(&existing)
		if result.Error != nil {
			return Job{}, result.Error
		}
		if result.RowsAffected != 0 {
			if existing.Kind != kind || existing.Payload != job.Payload {
				return Job{}, ErrIdempotencyConflict
			}
			return existing, nil
		}
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&job).Error; err != nil {
			return Job{}, err
		}
		if job.ID != 0 {
			enqueued.WithLabelValues(kind).Inc()
		}
		if err := db.Where("dedup_key = ?", *dedupKey).Take(&existing).Error; err != nil {
			return Job{}, err
		}
		if existing.Kind != kind || existing.Payload != job.Payload {
			return Job{}, ErrIdempotencyConflict
		}
		return existing, nil
	}
	if err := db.Create(&job).Error; err != nil {
		return Job{}, err
	}
	enqueued.WithLabelValues(kind).Inc()
	return job, nil
}

type Worker struct {
	DB           *gorm.DB
	Deliver      func(Job) error
	Kind         string
	ExcludeKinds []string
	stop         chan struct{}
	done         chan struct{}
}

func NewWorker(db *gorm.DB, deliver func(Job) error) *Worker {
	return &Worker{DB: db, Deliver: deliver, stop: make(chan struct{}), done: make(chan struct{})}
}

// NewKindWorker reserves a processing lane for one job kind. Multiple workers
// of the same kind still contend through the database lease.
func NewKindWorker(db *gorm.DB, kind string, deliver func(Job) error) *Worker {
	w := NewWorker(db, deliver)
	w.Kind = kind
	return w
}

func (w *Worker) Start() {
	go func() {
		defer close(w.done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-w.stop:
				return
			default:
			}
			processed, err := w.RunOne()
			if err != nil {
				log.Printf("integration jobs: %v", err)
			}
			if processed {
				continue
			}
			select {
			case <-w.stop:
				return
			case <-ticker.C:
			}
		}
	}()
}

func (w *Worker) Stop() {
	close(w.stop)
	select {
	case <-w.done:
	case <-time.After(5 * time.Second):
	}
}

// RunOne atomically leases one due row. The conditional update prevents two
// server replicas from delivering the same job at the same time.
func (w *Worker) RunOne() (bool, error) {
	now := time.Now()
	var candidates []Job
	query := w.DB.Where("(status = ? AND available_at <= ?) OR (status = ? AND lease_until <= ?)", "pending", now, "processing", now)
	if w.Kind != "" {
		query = query.Where("kind = ?", w.Kind)
	}
	if len(w.ExcludeKinds) > 0 {
		query = query.Where("kind NOT IN ?", w.ExcludeKinds)
	}
	if err := query.Order("id").Limit(10).Find(&candidates).Error; err != nil {
		return false, err
	}
	for _, candidate := range candidates {
		if candidate.Attempts >= maxAttempts {
			result := w.DB.Model(&Job{}).Where("id = ? AND status = ? AND lease_until <= ? AND attempts >= ?", candidate.ID, "processing", now, maxAttempts).
				Updates(map[string]any{"status": "failed", "lease_until": nil, "last_error": "lease expired after final attempt; delivery outcome unknown"})
			if result.Error != nil {
				return false, result.Error
			}
			if result.RowsAffected != 0 {
				return true, nil
			}
			continue
		}
		lease := now.Add(2 * time.Minute)
		result := w.DB.Model(&Job{}).Where("id = ? AND kind = ? AND ((status = ? AND available_at <= ?) OR (status = ? AND lease_until <= ?))", candidate.ID, candidate.Kind, "pending", now, "processing", now).
			Updates(map[string]any{"status": "processing", "lease_until": lease, "attempts": gorm.Expr("attempts + 1")})
		if result.Error != nil {
			return false, result.Error
		}
		if result.RowsAffected == 0 {
			continue
		}
		candidate.Attempts++
		waitSeconds.WithLabelValues(candidate.Kind).Observe(max(0, now.Sub(candidate.AvailableAt).Seconds()))
		started := time.Now()
		err := w.Deliver(candidate)
		deliverySeconds.WithLabelValues(candidate.Kind).Observe(time.Since(started).Seconds())
		outcome := "succeeded"
		if err != nil {
			outcome = "failed"
		}
		completed.WithLabelValues(candidate.Kind, outcome).Inc()
		if err == nil {
			err = w.DB.Model(&Job{}).Where("id = ? AND status = ? AND attempts = ?", candidate.ID, "processing", candidate.Attempts).Updates(map[string]any{"status": "succeeded", "lease_until": nil, "last_error": nil}).Error
		} else {
			status := "pending"
			if candidate.Attempts >= maxAttempts {
				status = "failed"
			}
			message := err.Error()
			if len(message) > 2000 {
				message = message[:2000]
			}
			updateErr := w.DB.Model(&Job{}).Where("id = ? AND status = ? AND attempts = ?", candidate.ID, "processing", candidate.Attempts).Updates(map[string]any{
				"status": status, "lease_until": nil, "last_error": message,
				"available_at": time.Now().Add(time.Duration(candidate.Attempts*candidate.Attempts) * time.Minute),
			}).Error
			if updateErr != nil {
				return true, fmt.Errorf("delivery: %v; recording failure: %w", err, updateErr)
			}
		}
		return true, err
	}
	return false, nil
}

// Retry exposes a deliberate operator action; failed rows never retry forever.
func Retry(db *gorm.DB, id uint64) error {
	result := db.Model(&Job{}).Where("id = ? AND status = ?", id, "failed").Updates(map[string]any{
		"status": "pending", "attempts": 0, "available_at": time.Now(), "lease_until": nil, "last_error": nil,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
