package gqlH

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/legal"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUpdateDayShiftCycle(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(db.AutoMigrate(&domain.RoleUser{}, &legal.Consent{}, &domain.DayShiftCycle{}, &domain.DayShiftCycleGroup{}, &domain.DayShiftCycleGroupMember{}, &domain.Log{}))
	must(db.Create(&legal.Consent{UserID: 1, TermsVersion: legal.Version, PrivacyVersion: legal.Version, SecurityVersion: legal.Version, DisplayLanguage: "ja"}).Error)
	cycle := domain.DayShiftCycle{ID: 7, BeginDate: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), FinishDate: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), Description: "2学期"}
	must(db.Create(&cycle).Error)
	must(db.Create(&domain.DayShiftCycleGroup{ID: 3, CycleID: 7, Position: 2}).Error)
	must(db.Create(&domain.DayShiftCycleGroupMember{ID: 4, GroupID: 3, UserID: 1, Position: 1}).Error)
	rdb := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	var actor *domain.User
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if actor != nil {
			c.Locals("user", actor)
		}
		return c.Next()
	})
	NewHandler(db, rdb, nil).RegisterGraphQL(app.Group("/graphql"))
	for _, tt := range []struct {
		name, role, begin, finish, description, wantError string
		id, status                                        int
	}{
		{"anonymous", "", "2026-09-02", "2026-12-25", "更新", "unauthorized", 7, 401},
		{"regular user", "user", "2026-09-02", "2026-12-25", "更新", "forbidden", 7, 400},
		{"invalid date", "admin", "invalid", "2026-12-25", "更新", "invalid dates", 7, 400},
		{"same date", "admin", "2026-09-02", "2026-09-02", "更新", "invalid dates", 7, 400},
		{"reversed dates", "admin", "2026-12-25", "2026-09-02", "更新", "invalid dates", 7, 400},
		{"blank name", "admin", "2026-09-02", "2026-12-25", "  ", "description required", 7, 400},
		{"missing cycle", "admin", "2026-09-02", "2026-12-25", "更新", "cycle not found", 999, 400},
		{"manager", "manager", "2026-09-02", "2026-12-25", "更新", "", 7, 200},
		{"admin unchanged", "admin", "2026-09-02", "2026-12-25", "更新", "", 7, 200},
		{"admin changed", "admin", "2026-09-03", "2026-12-26", " 再更新 ", "", 7, 200},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var before domain.DayShiftCycle
			must(db.First(&before, 7).Error)
			actor = nil
			must(db.Where("user_id = ?", 1).Delete(&domain.RoleUser{}).Error)
			if tt.role != "" {
				actor = &domain.User{ID: 1}
				if tt.role != "user" {
					must(db.Create(&domain.RoleUser{UserID: 1, Role: tt.role}).Error)
				}
			}
			op := fmt.Sprintf("mutation { adminDayshiftUpdateCycle(id:%d, begin_date:%q, finish_date:%q, description:%q) }", tt.id, tt.begin, tt.finish, tt.description)
			assertAdminGraphQLResult(t, app, op, tt.status, tt.wantError)
			var after domain.DayShiftCycle
			must(db.First(&after, 7).Error)
			if tt.wantError != "" {
				if after != before {
					t.Fatalf("failed update changed cycle: %+v", after)
				}
			} else if after.BeginDate.Format("2006-01-02") != tt.begin || after.FinishDate.Format("2006-01-02") != tt.finish || after.Description != strings.TrimSpace(tt.description) {
				t.Fatalf("cycle not saved: %+v", after)
			}
		})
	}
	var savedGroup domain.DayShiftCycleGroup
	must(db.First(&savedGroup, 3).Error)
	if savedGroup.CycleID != 7 || savedGroup.Position != 2 {
		t.Fatalf("group changed: %+v", savedGroup)
	}
	var savedMember domain.DayShiftCycleGroupMember
	must(db.First(&savedMember, 4).Error)
	if savedMember.GroupID != 3 || savedMember.UserID != 1 || savedMember.Position != 1 {
		t.Fatalf("member changed: %+v", savedMember)
	}
	var count int64
	must(db.Model(&domain.DayShiftCycle{}).Count(&count).Error)
	if count != 1 {
		t.Fatalf("cycle count = %d", count)
	}
}
