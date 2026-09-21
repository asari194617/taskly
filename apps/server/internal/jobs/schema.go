package jobs

import (
	"fmt"

	"gorm.io/gorm"
)

// EnsureSchema bridges deployments that do not run the SQL migrations. Each
// operation is checked separately so a restart can finish an interrupted run.
func EnsureSchema(db *gorm.DB) error {
	if !db.Migrator().HasTable(&Job{}) {
		if err := db.Exec(`CREATE TABLE IF NOT EXISTS integration_jobs (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
			kind VARCHAR(32) NOT NULL, payload JSON NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'pending', attempts INT NOT NULL DEFAULT 0,
			available_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
			lease_until DATETIME(6) NULL, dedup_key VARCHAR(255) NULL,
			last_error TEXT NULL,
			created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
			updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
			UNIQUE KEY uniq_integration_dedup (dedup_key),
			INDEX idx_integration_claim (status, available_at, lease_until)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`).Error; err != nil {
			return fmt.Errorf("create integration_jobs: %w", err)
		}
	}
	if !db.Migrator().HasColumn("tasks_teams_logs", "inbound_job_id") {
		if err := db.Exec(`ALTER TABLE tasks_teams_logs ADD COLUMN inbound_job_id BIGINT UNSIGNED NULL`).Error; err != nil {
			return fmt.Errorf("add inbound_job_id: %w", err)
		}
	}
	if !db.Migrator().HasIndex("tasks_teams_logs", "uniq_teams_inbound_job") {
		if err := db.Exec(`ALTER TABLE tasks_teams_logs ADD UNIQUE KEY uniq_teams_inbound_job (inbound_job_id)`).Error; err != nil {
			return fmt.Errorf("index inbound_job_id: %w", err)
		}
	}
	return nil
}
