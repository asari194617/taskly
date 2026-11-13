package utils

import (
	"github.com/asari194617/taskly/apps/server/internal/domain"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"testing"
)

func TestCheckPermissionFailsClosedWhenPermissionStoreUnavailable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&domain.RoleUser{}, &domain.Permission{}); err != nil {
		t.Fatal(err)
	}
	// A genuinely absent explicit rule retains the existing default read policy.
	if allowed, err := CheckPermission(1, "tasks", "read", nil, db); err != nil || !allowed {
		t.Fatalf("absent rule: allowed=%v err=%v", allowed, err)
	}
	db.Migrator().DropTable(&domain.Permission{})
	if allowed, err := CheckPermission(1, "tasks", "read", nil, db); err == nil || allowed {
		t.Fatalf("store outage: allowed=%v err=%v", allowed, err)
	}
}
