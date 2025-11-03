package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	graphql "github.com/graphql-go/graphql"
	"gorm.io/gorm"
)

// mutationFieldsDayShift groups DayShift-related mutations.
func mutationFieldsDayShift(t *gqlTypes) graphql.Fields {
	fields := graphql.Fields{}

	fields["adminDayshiftUpdateAssignmentMembers"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"date":     &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			"user_ids": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.Int)))},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !userFromSession(c) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*User)
				isAdmin, _ := userHasRole(u.ID, "admin")
				isManager, _ := userHasRole(u.ID, "manager")
				if !(isAdmin || isManager) {
					return false, fmt.Errorf("forbidden")
				}
				ds := strings.TrimSpace(p.Args["date"].(string))
				dd, err := time.Parse("2006-01-02", ds)
				if err != nil {
					return false, fmt.Errorf("invalid date")
				}
				var asg DayShiftAssignment
				if err := DB.Where("date = ?", ds).First(&asg).Error; err != nil {
					// create if not exists within current cycle
					cyc, _ := getCurrentCycle(dd)
					asg = DayShiftAssignment{Date: dateOnly(dd), Status: "assigned"}
					if cyc != nil {
						asg.CycleID = cyc.ID
					}
					_ = DB.Create(&asg).Error
				}
				_ = DB.Where("day_shift_assignments_id = ?", asg.ID).Delete(&DayShiftAssignmentUser{}).Error
				raw, _ := p.Args["user_ids"].([]any)
				for _, v := range raw {
					var id int
					switch t := v.(type) {
					case int:
						id = t
					case int64:
						id = int(t)
					case float64:
						id = int(t)
					}
					_ = DB.Create(&DayShiftAssignmentUser{DayShiftAssignmentsID: asg.ID, UserID: uint(id)}).Error
				}
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["adminDayshiftDeleteAssignment"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{"date": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !userFromSession(c) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*User)
				isAdmin, _ := userHasRole(u.ID, "admin")
				isManager, _ := userHasRole(u.ID, "manager")
				if !(isAdmin || isManager) {
					return false, fmt.Errorf("forbidden")
				}
				ds := strings.TrimSpace(p.Args["date"].(string))
				var asg DayShiftAssignment
				if err := DB.Where("date = ?", ds).First(&asg).Error; err != nil {
					return false, fmt.Errorf("not found")
				}
				_ = DB.Where("day_shift_assignments_id = ?", asg.ID).Delete(&DayShiftAssignmentUser{}).Error
				if err := DB.Delete(&DayShiftAssignment{}, asg.ID).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["adminDayshiftShift"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"start":     &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			"direction": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}, // forward | backward
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !userFromSession(c) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*User)
				isAdmin, _ := userHasRole(u.ID, "admin")
				isManager, _ := userHasRole(u.ID, "manager")
				if !(isAdmin || isManager) {
					return false, fmt.Errorf("forbidden")
				}
				ds := strings.TrimSpace(p.Args["start"].(string))
				dt, err := time.Parse("2006-01-02", ds)
				if err != nil {
					return false, fmt.Errorf("invalid date")
				}
				dir := strings.ToLower(strings.TrimSpace(fmt.Sprint(p.Args["direction"])))
				if dir == "forward" {
					if err := pushAssignmentsOneDayForward(dt); err != nil {
						return false, err
					}
					return true, nil
				} else if dir == "backward" {
					if err := pushAssignmentsOneDayBackward(dt); err != nil {
						return false, err
					}
					return true, nil
				}
				return false, fmt.Errorf("invalid direction")
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["adminDayshiftUpdateGroupMembers"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"group_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
			"user_ids": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.Int)))},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !userFromSession(c) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*User)
				isAdmin, _ := userHasRole(u.ID, "admin")
				isManager, _ := userHasRole(u.ID, "manager")
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
					if err := tx.Where("group_id = ?", gid).Delete(&DayShiftCycleGroupMember{}).Error; err != nil {
						return err
					}
					for i, id := range ids {
						if err := tx.Create(&DayShiftCycleGroupMember{GroupID: uint(gid), UserID: uint(id), Position: i + 1}).Error; err != nil {
							return err
						}
					}
					return nil
				}); err != nil {
					return false, fmt.Errorf("db")
				}
				_ = DB.Create(&Log{UserID: &u.ID, Action: "admin.dayshift.group.update_members"}).Error
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
				if !userFromSession(c) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*User)
				isAdmin, _ := userHasRole(u.ID, "admin")
				isManager, _ := userHasRole(u.ID, "manager")
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
						if err := tx.Model(&DayShiftCycleGroup{}).Where("id = ? AND cycle_id = ?", id, cid).Update("position", i+1).Error; err != nil {
							return err
						}
					}
					return nil
				})
				if err != nil {
					return false, fmt.Errorf("db")
				}
				_ = DB.Create(&Log{UserID: &u.ID, Action: "admin.dayshift.groups.reorder"}).Error
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
				if !userFromSession(c) {
					return nil, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*User)
				isAdmin, _ := userHasRole(u.ID, "admin")
				isManager, _ := userHasRole(u.ID, "manager")
				if !(isAdmin || isManager) {
					return nil, fmt.Errorf("forbidden")
				}
				bd, err1 := time.Parse("2006-01-02", p.Args["begin_date"].(string))
				fd, err2 := time.Parse("2006-01-02", p.Args["finish_date"].(string))
				if err1 != nil || err2 != nil || !bd.Before(fd) {
					return nil, fmt.Errorf("invalid dates")
				}
				desc := p.Args["description"].(string)
				rec := DayShiftCycle{BeginDate: bd, FinishDate: fd, Description: desc}
				if err := DB.Create(&rec).Error; err != nil {
					return nil, fmt.Errorf("db")
				}
				_ = DB.Create(&Log{UserID: &u.ID, Action: "admin.dayshift.cycle.create"}).Error
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
				if !userFromSession(c) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*User)
				isAdmin, _ := userHasRole(u.ID, "admin")
				isManager, _ := userHasRole(u.ID, "manager")
				if !(isAdmin || isManager) {
					return false, fmt.Errorf("forbidden")
				}
				if err := DB.Delete(&DayShiftCycle{}, p.Args["id"].(int)).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				_ = DB.Create(&Log{UserID: &u.ID, Action: "admin.dayshift.cycle.delete"}).Error
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
				if !userFromSession(c) {
					return nil, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*User)
				isAdmin, _ := userHasRole(u.ID, "admin")
				isManager, _ := userHasRole(u.ID, "manager")
				if !(isAdmin || isManager) {
					return nil, fmt.Errorf("forbidden")
				}
				cid := p.Args["cycle_id"].(int)
				uidVals, _ := p.Args["user_ids"].([]any)
				if len(uidVals) == 0 {
					return nil, fmt.Errorf("user_ids required")
				}
				var max int64
				_ = DB.Model(&DayShiftCycleGroup{}).Where("cycle_id = ?", cid).Select("COALESCE(MAX(position),0)").Scan(&max).Error
				pos := int(max) + 1
				g := DayShiftCycleGroup{CycleID: uint(cid), Position: pos}
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
					_ = DB.Create(&DayShiftCycleGroupMember{GroupID: g.ID, UserID: uint(idInt), Position: i + 1}).Error
				}
				_ = DB.Create(&Log{UserID: &u.ID, Action: "admin.dayshift.group.create"}).Error
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
				if !userFromSession(c) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*User)
				isAdmin, _ := userHasRole(u.ID, "admin")
				isManager, _ := userHasRole(u.ID, "manager")
				if !(isAdmin || isManager) {
					return false, fmt.Errorf("forbidden")
				}
				gid := p.Args["group_id"].(int)
				cid := p.Args["cycle_id"].(int)
				if err := DB.Where("group_id IN (SELECT id FROM day_shift_cycle_groups WHERE id = ? AND cycle_id = ?)", gid, cid).Delete(&DayShiftCycleGroupMember{}).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				if err := DB.Delete(&DayShiftCycleGroup{}, "id = ? AND cycle_id = ?", gid, cid).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				_ = DB.Create(&Log{UserID: &u.ID, Action: "admin.dayshift.group.delete"}).Error
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	// Send day shift notification for a specific date and notify Discord asynchronously
	fields["adminDayshiftNotify"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"date": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}, // YYYY-MM-DD
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !userFromSession(c) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*User)
				isAdmin, _ := userHasRole(u.ID, "admin")
				isManager, _ := userHasRole(u.ID, "manager")
				if !(isAdmin || isManager) {
					return false, fmt.Errorf("forbidden")
				}
				ds := strings.TrimSpace(p.Args["date"].(string))
				dt, err := time.Parse("2006-01-02", ds)
				if err != nil {
					return false, fmt.Errorf("invalid date")
				}
				// Resolve assigned users for the date
				var asg DayShiftAssignment
				if err := DB.Where("date = ?", ds).First(&asg).Error; err != nil {
					return false, fmt.Errorf("assignment not found")
				}
				var members []DayShiftAssignmentUser
				_ = DB.Where("day_shift_assignments_id = ?", asg.ID).Find(&members).Error
				names := make([]string, 0, len(members))
				for _, m := range members {
					var usr User
					if err := DB.Select("email", "display_name").First(&usr, m.UserID).Error; err == nil {
						if strings.TrimSpace(usr.DisplayName) != "" {
							names = append(names, usr.DisplayName)
						} else {
							names = append(names, usr.Email)
						}
					}
				}
				// Async Discord embed
				go sendDiscordDayShiftNotifyEmbed(dt, names, u.Email)
				// Future: trigger actual notification to assigned users (email/push) here
				_ = DB.Create(&Log{UserID: &u.ID, Action: "admin.dayshift.notify"}).Error
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	return fields
}
