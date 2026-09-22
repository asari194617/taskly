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
		dueJobs.WithLabelValues(kind).Set(float64(count))
		age := 0.0
		if count > 0 {
			var oldest Job
			if err := query.Order("available_at").First(&oldest).Error; err != nil {
				return err
			}
			age = now.Sub(oldest.AvailableAt).Seconds()
		}
		oldestWait.WithLabelValues(kind).Set(age)
	}
	return nil
}
