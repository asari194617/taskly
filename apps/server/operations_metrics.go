package server

import (
	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
)

type operationsCollector struct {
	db            *gorm.DB
	notifications *prometheus.Desc
}

func registerOperationsMetrics(db *gorm.DB) {
	prometheus.MustRegister(&operationsCollector{
		db:            db,
		notifications: prometheus.NewDesc("taskly_notification_deliveries", "Day shift notification delivery records by fixed outcome.", []string{"outcome"}, nil),
	})
}

func (c *operationsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.notifications
}

func (c *operationsCollector) Collect(ch chan<- prometheus.Metric) {
	var deliveries []struct {
		Status string
		Count  int64
	}
	if err := c.db.Table("notification_deliveries").Select("status, COUNT(*) AS count").
		Where("status IN ?", []string{"sent", "error"}).Group("status").Scan(&deliveries).Error; err != nil {
		ch <- prometheus.NewInvalidMetric(c.notifications, err)
		return
	}
	counts := map[string]int64{"sent": 0, "error": 0}
	for _, delivery := range deliveries {
		counts[delivery.Status] = delivery.Count
	}
	for _, outcome := range []string{"sent", "error"} {
		ch <- prometheus.MustNewConstMetric(c.notifications, prometheus.GaugeValue, float64(counts[outcome]), outcome)
	}
}
