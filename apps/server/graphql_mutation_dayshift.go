package main

import (
	"fmt"
	"time"

	utils "github.com/asari194617/taskly/apps/server/utils"
	"github.com/gofiber/fiber/v2"
	graphql "github.com/graphql-go/graphql"
	"gorm.io/gorm"
)

// mutationFieldsDayShift groups DayShift-related mutations.
func mutationFieldsDayShift(t *gqlTypes) graphql.Fields {
	fields := graphql.Fields{}

	fields["adminDayshiftUpdateGroupMembers"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"group_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
			"user_ids": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.Int)))},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !utils.UserFromSession(c, DB, RDB) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*utils.User)
				isAdmin, _ := utils.UserHasRole(u.ID, "admin", DB)
				isManager, _ := utils.UserHasRole(u.ID, "manager", DB)
				if !(isAdmin || isManager) {
					return false, fmt.Errorf("forbidden")
				}
				gid := p.Args["group_id"].(int)
				raw, _ := p.Args["user_ids"].([]any)
				ids := make([]int, 0, len(raw))
				for _, v := range raw {
					switch t := v.(type) {
					case int:
						ids = append(ids, t)
					case int64:
						ids = append(ids, int(t))
					case float64:
						ids = append(ids, int(t))
					}
				}
				if err := DB.Transaction(func(tx *gorm.DB) error {
					if err := tx.Where("group_id = ?", gid).Delete(&utils.DayShiftCycleGroupMember{}).Error; err != nil {
						return err
					}
					for i, id := range ids {
						if err := tx.Create(&utils.DayShiftCycleGroupMember{GroupID: uint(gid), UserID: uint(id), Position: i + 1}).Error; err != nil {
							return err
						}
					}
					return nil
				}); err != nil {
					return false, fmt.Errorf("db")
				}
				_ = DB.Create(&utils.Log{UserID: &u.ID, Action: "admin.dayshift.group.update_members"}).Error
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["adminDayshiftReorderGroups"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"cycle_id":  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
			"group_ids": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.Int)))},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !utils.UserFromSession(c, DB, RDB) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*utils.User)
				isAdmin, _ := utils.UserHasRole(u.ID, "admin", DB)
				isManager, _ := utils.UserHasRole(u.ID, "manager", DB)
				if !(isAdmin || isManager) {
					return false, fmt.Errorf("forbidden")
				}
				cid := p.Args["cycle_id"].(int)
				raw, _ := p.Args["group_ids"].([]any)
				if len(raw) == 0 {
					return false, fmt.Errorf("group_ids required")
				}
				ids := make([]int, 0, len(raw))
				for _, v := range raw {
					switch t := v.(type) {
					case int:
						ids = append(ids, t)
					case int64:
						ids = append(ids, int(t))
					case float64:
						ids = append(ids, int(t))
					}
				}
				// update positions transactionally
				err := DB.Transaction(func(tx *gorm.DB) error {
					for i, id := range ids {
						if err := tx.Model(&utils.DayShiftCycleGroup{}).Where("id = ? AND cycle_id = ?", id, cid).Update("position", i+1).Error; err != nil {
							return err
						}
					}
					return nil
				})
				if err != nil {
					return false, fmt.Errorf("db")
				}
				_ = DB.Create(&utils.Log{UserID: &u.ID, Action: "admin.dayshift.groups.reorder"}).Error
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["adminDayshiftCreateCycle"] = &graphql.Field{
		Type: graphql.NewObject(graphql.ObjectConfig{Name: "DayShiftCycleCreateResult", Fields: graphql.Fields{"id": &graphql.Field{Type: graphql.Int}}}),
		Args: graphql.FieldConfigArgument{"begin_date": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}, "finish_date": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}, "description": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !utils.UserFromSession(c, DB, RDB) {
					return nil, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*utils.User)
				isAdmin, _ := utils.UserHasRole(u.ID, "admin", DB)
				isManager, _ := utils.UserHasRole(u.ID, "manager", DB)
				if !(isAdmin || isManager) {
					return nil, fmt.Errorf("forbidden")
				}
				bd, err1 := time.Parse("2006-01-02", p.Args["begin_date"].(string))
				fd, err2 := time.Parse("2006-01-02", p.Args["finish_date"].(string))
				if err1 != nil || err2 != nil || !bd.Before(fd) {
					return nil, fmt.Errorf("invalid dates")
				}
				desc := p.Args["description"].(string)
				rec := utils.DayShiftCycle{BeginDate: bd, FinishDate: fd, Description: desc}
				if err := DB.Create(&rec).Error; err != nil {
					return nil, fmt.Errorf("db")
				}
				_ = DB.Create(&utils.Log{UserID: &u.ID, Action: "admin.dayshift.cycle.create"}).Error
				return map[string]any{"id": rec.ID}, nil
			}
			return nil, fmt.Errorf("context missing")
		},
	}

	fields["adminDayshiftDeleteCycle"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !utils.UserFromSession(c, DB, RDB) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*utils.User)
				isAdmin, _ := utils.UserHasRole(u.ID, "admin", DB)
				isManager, _ := utils.UserHasRole(u.ID, "manager", DB)
				if !(isAdmin || isManager) {
					return false, fmt.Errorf("forbidden")
				}
				if err := DB.Delete(&utils.DayShiftCycle{}, p.Args["id"].(int)).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				_ = DB.Create(&utils.Log{UserID: &u.ID, Action: "admin.dayshift.cycle.delete"}).Error
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["adminDayshiftCreateGroup"] = &graphql.Field{
		Type: graphql.NewObject(graphql.ObjectConfig{
			Name:   "DayShiftGroupCreateResult",
			Fields: graphql.Fields{"id": &graphql.Field{Type: graphql.Int}},
		}),
		Args: graphql.FieldConfigArgument{
			"cycle_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
			"user_ids": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.Int)))},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !utils.UserFromSession(c, DB, RDB) {
					return nil, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*utils.User)
				isAdmin, _ := utils.UserHasRole(u.ID, "admin", DB)
				isManager, _ := utils.UserHasRole(u.ID, "manager", DB)
				if !(isAdmin || isManager) {
					return nil, fmt.Errorf("forbidden")
				}
				cid := p.Args["cycle_id"].(int)
				uidVals, _ := p.Args["user_ids"].([]any)
				if len(uidVals) == 0 {
					return nil, fmt.Errorf("user_ids required")
				}
				var max int64
				_ = DB.Model(&utils.DayShiftCycleGroup{}).Where("cycle_id = ?", cid).Select("COALESCE(MAX(position),0)").Scan(&max).Error
				pos := int(max) + 1
				g := utils.DayShiftCycleGroup{CycleID: uint(cid), Position: pos}
				if err := DB.Create(&g).Error; err != nil {
					return nil, fmt.Errorf("db")
				}
				// insert members with relative positions
				for i, v := range uidVals {
					var idInt int
					switch t := v.(type) {
					case int:
						idInt = t
					case int64:
						idInt = int(t)
					case float64:
						idInt = int(t)
					default:
						continue
					}
					_ = DB.Create(&utils.DayShiftCycleGroupMember{GroupID: g.ID, UserID: uint(idInt), Position: i + 1}).Error
				}
				_ = DB.Create(&utils.Log{UserID: &u.ID, Action: "admin.dayshift.group.create"}).Error
				return map[string]any{"id": g.ID}, nil
			}
			return nil, fmt.Errorf("context missing")
		},
	}

	fields["adminDayshiftDeleteGroup"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"cycle_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
			"group_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !utils.UserFromSession(c, DB, RDB) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*utils.User)
				isAdmin, _ := utils.UserHasRole(u.ID, "admin", DB)
				isManager, _ := utils.UserHasRole(u.ID, "manager", DB)
				if !(isAdmin || isManager) {
					return false, fmt.Errorf("forbidden")
				}
				gid := p.Args["group_id"].(int)
				cid := p.Args["cycle_id"].(int)
				if err := DB.Where("group_id IN (SELECT id FROM day_shift_cycle_groups WHERE id = ? AND cycle_id = ?)", gid, cid).Delete(&utils.DayShiftCycleGroupMember{}).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				if err := DB.Delete(&utils.DayShiftCycleGroup{}, "id = ? AND cycle_id = ?", gid, cid).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				_ = DB.Create(&utils.Log{UserID: &u.ID, Action: "admin.dayshift.group.delete"}).Error
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	// Send day shift notification for a specific date and notify Discord asynchronously
	// Old adminDayshiftNotify mutation relied on day_shift_assignments; removed in favor of index-based pipeline.

	return fields
}
