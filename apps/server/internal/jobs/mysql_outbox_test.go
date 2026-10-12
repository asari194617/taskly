package jobs

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	mysqlgorm "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// CI runs this only against its disposable, Atlas-initialized MySQL database.
func TestMySQLTransactionalOutbox(t *testing.T) {
	if os.Getenv("TEST_SCHEMA_DB") != "1" {
		t.Skip("requires disposable MySQL database")
	}
	config := mysql.NewConfig()
	config.User = os.Getenv("MYSQL_USER")
	config.Passwd = os.Getenv("MYSQL_PASSWORD")
	config.Net = "tcp"
	config.Addr = os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT")
	config.DBName = os.Getenv("MYSQL_DATABASE")
	config.ParseTime = true
	config.Loc = time.Local
	db, err := gorm.Open(mysqlgorm.Open(config.FormatDSN()), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	w := NewKindWorker(db, "task_discord", func(Job) error { return nil })
	defer w.subscribe()()
	rollback := errors.New("abort task mutation")
	err = Transaction(db, func(tx *gorm.DB) error {
		if _, err := Enqueue(tx, "task_discord", map[string]string{"source": "mysql-outbox-test"}, nil); err != nil {
			return err
		}
		expectWake(t, w, false)
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	expectWake(t, w, false)
	var job Job
	if err := Transaction(db, func(tx *gorm.DB) error {
		var err error
		job, err = Enqueue(tx, "task_discord", map[string]string{"source": "mysql-outbox-test"}, nil)
		expectWake(t, w, false)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	defer db.Delete(&Job{}, job.ID)
	expectWake(t, w, true)
	if err := ObserveBacklog(db, []string{"task_discord"}); err != nil {
		t.Fatal(err)
	}
	collector := &queueCollector{db: db,
		count:  prometheus.NewDesc("taskly_integration_jobs", "jobs", []string{"status"}, nil),
		oldest: prometheus.NewDesc("taskly_integration_job_oldest_age_seconds", "age", []string{"status"}, nil),
	}
	registry := prometheus.NewRegistry()
	if err := registry.Register(collector); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Gather(); err != nil {
		t.Fatal(err)
	}
	if processed, err := w.RunOne(); err != nil || !processed {
		t.Fatalf("delivery processed=%v err=%v", processed, err)
	}
	var stored Job
	if err := db.First(&stored, job.ID).Error; err != nil || stored.Status != "succeeded" {
		t.Fatalf("job=%+v err=%v", stored, err)
	}
}
