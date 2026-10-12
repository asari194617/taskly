package jobs

import (
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
)

var (
	enqueued        = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "integration_jobs_enqueued_total", Help: "New integration jobs by kind."}, []string{"kind"})
	completed       = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "integration_jobs_deliveries_total", Help: "Delivery attempts by kind and outcome."}, []string{"kind", "outcome"})
	waitSeconds     = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "integration_jobs_wait_seconds", Help: "Time from availability to claim.", Buckets: prometheus.ExponentialBuckets(0.1, 2, 12)}, []string{"kind"})
	deliverySeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "integration_jobs_delivery_seconds", Help: "Delivery attempt duration.", Buckets: prometheus.ExponentialBuckets(0.01, 2, 14)}, []string{"kind"})
	dueJobs         = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "integration_jobs_due", Help: "Due pending jobs by kind."}, []string{"kind"})
	oldestWait      = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "integration_jobs_oldest_wait_seconds", Help: "Age of the oldest due pending job by kind."}, []string{"kind"})
)

func init() {
	prometheus.MustRegister(enqueued, completed, waitSeconds, deliverySeconds, dueJobs, oldestWait)
}

func metricKind(kind string) string {
	switch kind {
	case "teams_inbound", "teams_callback", "task_discord", "task_email":
		return kind
	default:
		return "other"
	}
}

// ObserveBacklog refreshes per-kind backlog gauges. A failed query keeps the
// previous reading until the next poll and is returned to the caller for logging.
func ObserveBacklog(db *gorm.DB, kinds []string) error {
	now := time.Now()
	var rows []struct {
		Kind   string
		Count  int64
		Oldest aggregateTime
	}
	if err := db.Model(&Job{}).Select("kind, COUNT(*) AS count, MIN(available_at) AS oldest").
		Where("kind IN ? AND status = ? AND available_at <= ?", kinds, "pending", now).
		Group("kind").Scan(&rows).Error; err != nil {
		return err
	}
	byKind := make(map[string]struct {
		count  int64
		oldest time.Time
	}, len(rows))
	for _, row := range rows {
		byKind[row.Kind] = struct {
			count  int64
			oldest time.Time
		}{row.Count, row.Oldest.Time}
	}
	for _, kind := range kinds {
		row := byKind[kind]
		age := 0.0
		if row.count > 0 {
			age = max(0, now.Sub(row.oldest).Seconds())
		}
		dueJobs.WithLabelValues(metricKind(kind)).Set(float64(row.count))
		oldestWait.WithLabelValues(metricKind(kind)).Set(age)
	}
	return nil
}

type queueCollector struct {
	db     *gorm.DB
	count  *prometheus.Desc
	oldest *prometheus.Desc
}

func RegisterQueueMetrics(db *gorm.DB) {
	prometheus.MustRegister(&queueCollector{
		db:     db,
		count:  prometheus.NewDesc("taskly_integration_jobs", "Current integration jobs by status.", []string{"status"}, nil),
		oldest: prometheus.NewDesc("taskly_integration_job_oldest_age_seconds", "Age of oldest integration job by status, zero when empty.", []string{"status"}, nil),
	})
}

func (c *queueCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.count
	ch <- c.oldest
}

func (c *queueCollector) Collect(ch chan<- prometheus.Metric) {
	var rows []struct {
		Status string
		Count  int64
		Oldest aggregateTime
	}
	if err := c.db.Table("integration_jobs").Select("status, COUNT(*) AS count, MIN(created_at) AS oldest").
		Where("status IN ?", []string{"pending", "processing", "failed"}).Group("status").Scan(&rows).Error; err != nil {
		ch <- prometheus.NewInvalidMetric(c.count, err)
		return
	}
	byStatus := make(map[string]struct {
		count  int64
		oldest time.Time
	}, len(rows))
	for _, row := range rows {
		byStatus[row.Status] = struct {
			count  int64
			oldest time.Time
		}{row.Count, row.Oldest.Time}
	}
	for _, status := range []string{"pending", "processing", "failed"} {
		age := 0.0
		if byStatus[status].count > 0 {
			age = max(0, time.Since(byStatus[status].oldest).Seconds())
		}
		ch <- prometheus.MustNewConstMetric(c.count, prometheus.GaugeValue, float64(byStatus[status].count), status)
		ch <- prometheus.MustNewConstMetric(c.oldest, prometheus.GaugeValue, age, status)
	}
}

// MySQL returns aggregate DATETIMEs as time.Time with parseTime enabled;
// SQLite returns text even when the source column is a DATETIME.
type aggregateTime struct{ time.Time }

func (t aggregateTime) Value() (driver.Value, error) { return t.Time, nil }

func (t *aggregateTime) Scan(value any) error {
	if value == nil {
		return nil
	}
	if v, ok := value.(time.Time); ok {
		t.Time = v
		return nil
	}
	var raw string
	switch v := value.(type) {
	case string:
		raw = v
	case []byte:
		raw = string(v)
	default:
		return fmt.Errorf("unexpected aggregate timestamp type %T", value)
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05.999999999-07:00 MST", "2006-01-02 15:04:05.999999999"} {
		parsed, err := time.ParseInLocation(layout, raw, time.Local)
		if err == nil {
			t.Time = parsed
			return nil
		}
	}
	return fmt.Errorf("invalid aggregate timestamp %q", raw)
}
