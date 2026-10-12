package jobs

import (
	dto "github.com/prometheus/client_model/go"
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

func gaugeValue(t *testing.T, gauge prometheus.Gauge) float64 {
	t.Helper()
	metric := &dto.Metric{}
	if err := gauge.Write(metric); err != nil {
		t.Fatal(err)
	}
	return metric.GetGauge().GetValue()
}

func TestGroupedBacklogFiltersDueJobsAndResetsEmptyKinds(t *testing.T) {
	db := testDB(t)
	now := time.Now()
	for _, job := range []Job{
		{Kind: "task_discord", Status: "pending", AvailableAt: now.Add(-time.Minute)},
		{Kind: "task_discord", Status: "pending", AvailableAt: now.Add(-2 * time.Minute)},
		{Kind: "task_discord", Status: "pending", AvailableAt: now.Add(time.Minute)},
		{Kind: "task_discord", Status: "processing", AvailableAt: now.Add(-3 * time.Minute)},
		{Kind: "task_email", Status: "pending", AvailableAt: now.Add(-time.Minute)},
	} {
		if err := db.Create(&job).Error; err != nil {
			t.Fatal(err)
		}
	}
	kinds := []string{"task_discord", "task_email", "teams_inbound"}
	if err := ObserveBacklog(db, kinds); err != nil {
		t.Fatal(err)
	}
	if got := gaugeValue(t, dueJobs.WithLabelValues("task_discord")); got != 2 {
		t.Fatalf("due Discord=%v", got)
	}
	if got := gaugeValue(t, oldestWait.WithLabelValues("task_discord")); got < 119 || got > 125 {
		t.Fatalf("oldest Discord=%v", got)
	}
	if got := gaugeValue(t, dueJobs.WithLabelValues("task_email")); got != 1 {
		t.Fatalf("due email=%v", got)
	}
	if got := gaugeValue(t, dueJobs.WithLabelValues("teams_inbound")); got != 0 {
		t.Fatalf("empty inbound=%v", got)
	}
	if err := db.Model(&Job{}).Where("kind = ?", "task_discord").Update("status", "succeeded").Error; err != nil {
		t.Fatal(err)
	}
	if err := ObserveBacklog(db, kinds); err != nil {
		t.Fatal(err)
	}
	if gaugeValue(t, dueJobs.WithLabelValues("task_discord")) != 0 || gaugeValue(t, oldestWait.WithLabelValues("task_discord")) != 0 {
		t.Fatal("stale backlog metrics after drain")
	}
}
