package jobs

import (
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
	for _, kind := range kinds {
		query := db.Model(&Job{}).Where("kind = ? AND status = ? AND available_at <= ?", kind, "pending", now)
		var count int64
		if err := query.Count(&count).Error; err != nil {
			return err
		}
		dueJobs.WithLabelValues(metricKind(kind)).Set(float64(count))
		age := 0.0
		if count > 0 {
			var oldest Job
			if err := query.Order("available_at").First(&oldest).Error; err != nil {
				return err
			}
			age = now.Sub(oldest.AvailableAt).Seconds()
		}
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
	}
	if err := c.db.Table("integration_jobs").Select("status, COUNT(*) AS count").
		Where("status IN ?", []string{"pending", "processing", "failed"}).Group("status").Scan(&rows).Error; err != nil {
		ch <- prometheus.NewInvalidMetric(c.count, err)
		return
	}
	byStatus := make(map[string]int64, len(rows))
	for _, row := range rows {
		byStatus[row.Status] = row.Count
	}
	for _, status := range []string{"pending", "processing", "failed"} {
		age := 0.0
		if byStatus[status] > 0 {
			var oldest Job
			if err := c.db.Select("created_at").Where("status = ?", status).Order("created_at ASC").First(&oldest).Error; err != nil {
				ch <- prometheus.NewInvalidMetric(c.oldest, err)
				return
			}
			age = time.Since(oldest.CreatedAt).Seconds()
			if age < 0 {
				age = 0
			}
		}
		ch <- prometheus.MustNewConstMetric(c.count, prometheus.GaugeValue, float64(byStatus[status]), status)
		ch <- prometheus.MustNewConstMetric(c.oldest, prometheus.GaugeValue, age, status)
	}
}
