package jobs

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestQueueCollectorIncludesEmptyStatusesAndOldestAge(t *testing.T) {
	db := testDB(t)
	job := Job{Kind: "task_email", Status: "pending", AvailableAt: time.Now(), CreatedAt: time.Now().Add(-15 * time.Minute)}
	if err := db.Create(&job).Error; err != nil {
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
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]float64{}
	ages := map[string]float64{}
	for _, family := range families {
		for _, metric := range family.GetMetric() {
			status := metric.GetLabel()[0].GetValue()
			if family.GetName() == "taskly_integration_jobs" {
				counts[status] = metric.GetGauge().GetValue()
			}
			if family.GetName() == "taskly_integration_job_oldest_age_seconds" {
				ages[status] = metric.GetGauge().GetValue()
			}
		}
	}
	if counts["pending"] != 1 || counts["processing"] != 0 || counts["failed"] != 0 || ages["pending"] < 890 || ages["pending"] > 910 {
		t.Fatalf("counts=%v ages=%v", counts, ages)
	}
}
