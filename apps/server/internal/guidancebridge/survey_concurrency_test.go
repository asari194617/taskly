package guidancebridge

import (
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/legal"
	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// A committed fixture and a separate connection pool are essential here: two
// goroutines sharing the fixture's outer transaction cannot exercise MySQL's
// inter-transaction locking or expose a deadlock.
func committedSurveyFixture(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t, true)
	if err := f.db.Commit().Error; err != nil {
		t.Fatal(err)
	}
	cfg := mysqldriver.NewConfig()
	cfg.User, cfg.Passwd = os.Getenv("MYSQL_USER"), os.Getenv("MYSQL_PASSWORD")
	cfg.Net = "tcp"
	cfg.Addr = os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT")
	cfg.DBName, cfg.ParseTime = os.Getenv("MYSQL_DATABASE"), true
	db, err := gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(8)
	t.Cleanup(func() { sqlDB.Close() })
	f.db = db
	f.app = fiber.New()
	Register(f.app.Group("/integrations/nnct-guidance"), f.db)
	t.Cleanup(func() {
		ids := make([]uint, 0, len(f.bindings))
		for _, id := range f.bindings {
			ids = append(ids, id)
		}
		// Delete only this committed synthetic fixture, never reset shared tables.
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("DELETE FROM tasks_users_completions WHERE task_id IN ?", ids).Error; err != nil {
				return err
			}
			if err := tx.Delete(&domain.Task{}, ids).Error; err != nil {
				return err
			}
			if err := tx.Where("user_id = ?", f.user.ID).Delete(&legal.Consent{}).Error; err != nil {
				return err
			}
			if err := tx.Delete(&domain.User{}, f.user.ID).Error; err != nil {
				return err
			}
			return tx.Delete(&domain.Subject{}, f.task.SubjectID).Error
		}); err != nil {
			t.Errorf("clean committed synthetic fixture: %v", err)
		}
	})
	return f
}

func concurrentPairRequest(f *fixture, key string, completed bool) error {
	req := httptest.NewRequest("PUT", "/integrations/nnct-guidance/tasks/"+key, strings.NewReader(pairBody(completed, scopeA)))
	req.Header.Set("Authorization", "Bearer synthetic-access")
	req.Header.Set("X-Guidance-Event-ID", "synthetic-event")
	req.Header.Set("Content-Type", "application/json")
	res, err := f.app.Test(req, 10000)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("concurrent pair status %d: %s", res.StatusCode, body)
	}
	return nil
}

func raceCompletionOperations(t *testing.T, operations ...func(context.Context) error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	start := make(chan struct{})
	results := make(chan error, len(operations))
	var workers sync.WaitGroup
	for _, operation := range operations {
		workers.Add(1)
		go func(operation func(context.Context) error) {
			defer workers.Done()
			<-start
			results <- operation(ctx)
		}(operation)
	}
	close(start)
	workers.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Errorf("concurrent operation failed: %v", err)
		}
	}
	if t.Failed() {
		t.FailNow()
	}
}

func TestMySQLGuidanceSurveyConcurrentManualOwnership(t *testing.T) {
	requireMySQL(t)
	f := committedSurveyFixture(t)
	surveyID, visitID := f.bindings["visit-survey-2"], f.bindings["assigned-visit-2"]
	resetAndLink := func(t *testing.T) {
		t.Helper()
		for _, id := range []uint{surveyID, visitID} {
			if err := domain.UncompleteTask(f.db, f.user.ID, id); err != nil {
				t.Fatal(err)
			}
		}
		pairRequest(t, f, true, scopeA, 200)
		assertRow(t, f, "assigned-visit-2", true, scopeA)
	}
	for round := 0; round < 8; round++ {
		t.Run(fmt.Sprintf("manual_visit_recheck_%d", round), func(t *testing.T) {
			resetAndLink(t)
			// Both legal serializations retain the explicit manual visit check.
			raceCompletionOperations(t,
				func(context.Context) error { return concurrentPairRequest(f, "visit-survey-2", false) },
				func(ctx context.Context) error { return domain.CompleteTask(f.db.WithContext(ctx), f.user.ID, visitID) },
			)
			assertRow(t, f, "visit-survey-2", false, "")
			assertRow(t, f, "assigned-visit-2", true, "")
		})
		t.Run(fmt.Sprintf("manual_visit_undo_duplicate_survey_%d", round), func(t *testing.T) {
			resetAndLink(t)
			// A duplicate true is not a new source transition. It cannot restore
			// a visit that was manually removed in either serialization.
			raceCompletionOperations(t,
				func(context.Context) error { return concurrentPairRequest(f, "visit-survey-2", true) },
				func(ctx context.Context) error {
					return domain.UncompleteTask(f.db.WithContext(ctx), f.user.ID, visitID)
				},
			)
			assertRow(t, f, "visit-survey-2", true, "")
			assertRow(t, f, "assigned-visit-2", false, "")
		})
		t.Run(fmt.Sprintf("manual_survey_recheck_reverse_undo_%d", round), func(t *testing.T) {
			resetAndLink(t)
			// If the explicit source check wins first it retires the automatic
			// relation; if it wins last it recreates the manually owned source.
			// Both cases must preserve that source and remove only the visit.
			raceCompletionOperations(t,
				func(context.Context) error { return concurrentPairRequest(f, "assigned-visit-2", false) },
				func(ctx context.Context) error {
					return domain.CompleteTask(f.db.WithContext(ctx), f.user.ID, surveyID)
				},
			)
			assertRow(t, f, "visit-survey-2", true, "")
			assertRow(t, f, "assigned-visit-2", false, "")
		})
	}
}
