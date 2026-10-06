package jobs

import (
	"fmt"

	"gorm.io/gorm"
)

// EnsureSchema verifies that Atlas applied the integration job schema before
// the API starts accepting work.
func EnsureSchema(db *gorm.DB) error {
	if !db.Migrator().HasTable(&Job{}) {
		return fmt.Errorf("integration_jobs is missing; apply apps/server/schema.sql with Atlas")
	}
	if !db.Migrator().HasColumn("tasks_teams_logs", "inbound_job_id") {
		return fmt.Errorf("tasks_teams_logs.inbound_job_id is missing; apply apps/server/schema.sql with Atlas")
	}
	if !db.Migrator().HasIndex("tasks_teams_logs", "uniq_teams_inbound_job") {
		return fmt.Errorf("uniq_teams_inbound_job is missing; apply apps/server/schema.sql with Atlas")
	}
	if !db.Migrator().HasIndex("integration_jobs", "idx_integration_kind_claim") {
		return fmt.Errorf("idx_integration_kind_claim is missing; apply apps/server/schema.sql with Atlas")
	}
	return nil
}
