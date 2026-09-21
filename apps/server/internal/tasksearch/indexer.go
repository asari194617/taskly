package tasksearch

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"gorm.io/gorm"
)

type change struct {
	TaskID  uint `gorm:"column:task_id"`
	Version uint64
}

func (c *Client) run(ctx context.Context, db *gorm.DB) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	bootstrapped := false
	for {
		if !bootstrapped {
			err := c.ensureIndex(ctx)
			if err == nil {
				err = c.backfill(ctx, db)
			}
			if err != nil {
				c.ready.Store(false)
				log.Printf("task search: bootstrap failed: %v", err)
			} else {
				bootstrapped = true
			}
		}
		if bootstrapped {
			_, status, err := c.request(ctx, http.MethodHead, "/"+indexName, nil)
			if err == nil && status != http.StatusOK {
				err = errors.New("task index missing")
				bootstrapped = false
			}
			if err == nil {
				err = c.drain(ctx, db)
			}
			if err == nil && !c.Ready() {
				_, err = c.Search(ctx, SearchOptions{Query: "taskly-healthcheck", Today: time.Now().Format("2006-01-02"), Size: 0})
			}
			if err != nil {
				c.ready.Store(false)
				log.Printf("task search: sync failed: %v", err)
			} else {
				c.ready.Store(true)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Backfill is idempotent and runs before search is enabled after each API start.
// Mutations during the scan remain in the transactional outbox and are replayed.
func (c *Client) backfill(ctx context.Context, db *gorm.DB) error {
	var last uint
	for {
		var tasks []domain.Task
		if err := db.WithContext(ctx).Where("id > ?", last).Order("id ASC").Limit(100).Find(&tasks).Error; err != nil {
			return err
		}
		if len(tasks) == 0 {
			return nil
		}
		for _, task := range tasks {
			if err := c.index(ctx, task); err != nil {
				return err
			}
		}
		last = tasks[len(tasks)-1].ID
	}
}

func (c *Client) drain(ctx context.Context, db *gorm.DB) error {
	for {
		var changes []change
		if err := db.WithContext(ctx).Table("task_search_outbox").Order("task_id ASC").Limit(100).Find(&changes).Error; err != nil {
			return err
		}
		if len(changes) == 0 {
			return nil
		}
		for _, row := range changes {
			var task domain.Task
			err := db.WithContext(ctx).First(&task, row.TaskID).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				err = c.remove(ctx, row.TaskID)
			} else if err == nil {
				err = c.index(ctx, task)
			}
			if err != nil {
				return err
			}
			if err := db.WithContext(ctx).Exec("DELETE FROM task_search_outbox WHERE task_id = ? AND version = ?", row.TaskID, row.Version).Error; err != nil {
				return err
			}
		}
		if len(changes) < 100 {
			return nil
		}
	}
}
