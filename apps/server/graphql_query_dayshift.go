package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	graphql "github.com/graphql-go/graphql"
)

// queryFieldsDayShift returns GraphQL fields related to day shift (assignments, groups, cycles).
func queryFieldsDayShift(t *gqlTypes) graphql.Fields {
	dayShiftAssignmentType := t.DayShiftAssignmentType
	dayShiftGroupType := t.DayShiftGroupType
	dayShiftCycleType := t.DayShiftCycleType
	dayShiftCycleListType := t.DayShiftCycleListType
	dayShiftGroupListType := t.DayShiftGroupListType

	return graphql.Fields{
		// Day shift: assignments list (by cycle or date range)
		"adminDayshiftAssignments": &graphql.Field{
			Type: graphql.NewList(dayShiftAssignmentType),
			Args: graphql.FieldConfigArgument{
				"cycle_id": &graphql.ArgumentConfig{Type: graphql.Int},
				"start":    &graphql.ArgumentConfig{Type: graphql.String},
				"end":      &graphql.ArgumentConfig{Type: graphql.String},
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
					dbq := DB.Model(&DayShiftAssignment{})
					if v, ok := p.Args["cycle_id"].(int); ok && v > 0 {
						dbq = dbq.Where("cycle_id = ?", v)
					}
					if s, ok := p.Args["start"].(string); ok && strings.TrimSpace(s) != "" {
						if _, err := time.Parse("2006-01-02", s); err == nil {
							dbq = dbq.Where("date >= ?", s)
						}
					}
					if e, ok := p.Args["end"].(string); ok && strings.TrimSpace(e) != "" {
						if _, err := time.Parse("2006-01-02", e); err == nil {
							dbq = dbq.Where("date <= ?", e)
						}
					}
					var items []DayShiftAssignment
					if err := dbq.Order("date ASC").Find(&items).Error; err != nil {
						return nil, err
					}
					// build user_ids per assignment
					out := make([]map[string]any, 0, len(items))
					for _, it := range items {
						var mems []DayShiftAssignmentUser
						_ = DB.Where("day_shift_assignments_id = ?", it.ID).Order("id ASC").Find(&mems).Error
						ids := make([]int, 0, len(mems))
						for _, m := range mems {
							ids = append(ids, int(m.UserID))
						}
						out = append(out, map[string]any{
							"id":       it.ID,
							"date":     it.Date.Format("2006-01-02"),
							"cycle_id": it.CycleID,
							"status":   it.Status,
							"user_ids": ids,
						})
					}
					return out, nil
				}
				return nil, fmt.Errorf("context missing")
			},
		},
		// IDs only for groups (to help reordering with server-side pagination)
		"adminDayshiftGroupIds": &graphql.Field{
			Type: graphql.NewList(graphql.Int),
			Args: graphql.FieldConfigArgument{
				"cycle_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
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
					var groups []DayShiftCycleGroup
					if err := DB.Where("cycle_id = ?", cid).Order("position ASC, id ASC").Find(&groups).Error; err != nil {
						return nil, fmt.Errorf("db")
					}
					out := make([]int, 0, len(groups))
					for _, g := range groups {
						out = append(out, int(g.ID))
					}
					return out, nil
				}
				return nil, fmt.Errorf("context missing")
			},
		},
		// Day shift: cycles list
		"adminDayshiftCycles": &graphql.Field{
			Type: graphql.NewList(dayShiftCycleType),
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
					var items []DayShiftCycle
					if err := DB.Order("begin_date DESC").Find(&items).Error; err != nil {
						return nil, err
					}
					out := make([]map[string]any, 0, len(items))
					for _, it := range items {
						out = append(out, map[string]any{"id": it.ID, "begin_date": it.BeginDate.Format("2006-01-02"), "finish_date": it.FinishDate.Format("2006-01-02"), "description": it.Description})
					}
					return out, nil
				}
				return nil, fmt.Errorf("context missing")
			},
		},
		// Day shift: cycles paginated with optional search by description
		"adminDayshiftCyclesPaginated": &graphql.Field{
			Type: dayShiftCycleListType,
			Args: graphql.FieldConfigArgument{
				"page":      &graphql.ArgumentConfig{Type: graphql.Int},
				"page_size": &graphql.ArgumentConfig{Type: graphql.Int},
				"q":         &graphql.ArgumentConfig{Type: graphql.String},
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
					page := 1
					if v, ok := p.Args["page"].(int); ok && v > 0 {
						page = v
					}
					size := 30
					if v, ok := p.Args["page_size"].(int); ok && v > 0 && v <= 200 {
						size = v
					}
					qstr, _ := p.Args["q"].(string)
					qstr = strings.TrimSpace(qstr)
					dbq := DB.Model(&DayShiftCycle{})
					if qstr != "" {
						dbq = dbq.Where("description LIKE ?", "%"+qstr+"%")
					}
					var total int64
					if err := dbq.Count(&total).Error; err != nil {
						return nil, err
					}
					var items []DayShiftCycle
					if err := dbq.Order("begin_date DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
						return nil, err
					}
					outItems := make([]map[string]any, 0, len(items))
					for _, it := range items {
						outItems = append(outItems, map[string]any{"id": it.ID, "begin_date": it.BeginDate.Format("2006-01-02"), "finish_date": it.FinishDate.Format("2006-01-02"), "description": it.Description})
					}
					return map[string]any{
						"items":     outItems,
						"page":      page,
						"page_size": size,
						"total":     int(total),
						"has_prev":  page > 1,
						"has_next":  int64(page*size) < total,
					}, nil
				}
				return nil, fmt.Errorf("context missing")
			},
		},
		// Day shift: groups list for a cycle
		"adminDayshiftGroups": &graphql.Field{
			Type: graphql.NewList(dayShiftGroupType),
			Args: graphql.FieldConfigArgument{
				"cycle_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
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
					var groups []DayShiftCycleGroup
					if err := DB.Where("cycle_id = ?", cid).Order("position ASC, id ASC").Find(&groups).Error; err != nil {
						return nil, fmt.Errorf("db")
					}
					out := make([]map[string]any, 0, len(groups))
					for _, g := range groups {
						var mems []DayShiftCycleGroupMember
						_ = DB.Where("group_id = ?", g.ID).Order("position ASC, id ASC").Find(&mems).Error
						ids := make([]int, 0, len(mems))
						for _, m := range mems {
							ids = append(ids, int(m.UserID))
						}
						out = append(out, map[string]any{"id": g.ID, "cycle_id": g.CycleID, "position": g.Position, "user_ids": ids})
					}
					return out, nil
				}
				return nil, fmt.Errorf("context missing")
			},
		},
		// Day shift: groups paginated for a cycle
		"adminDayshiftGroupsPaginated": &graphql.Field{
			Type: dayShiftGroupListType,
			Args: graphql.FieldConfigArgument{
				"cycle_id":  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
				"page":      &graphql.ArgumentConfig{Type: graphql.Int},
				"page_size": &graphql.ArgumentConfig{Type: graphql.Int},
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
					page := 1
					if v, ok := p.Args["page"].(int); ok && v > 0 {
						page = v
					}
					size := 30
					if v, ok := p.Args["page_size"].(int); ok && v > 0 && v <= 200 {
						size = v
					}
					dbq := DB.Model(&DayShiftCycleGroup{}).Where("cycle_id = ?", cid)
					var total int64
					if err := dbq.Count(&total).Error; err != nil {
						return nil, err
					}
					var groups []DayShiftCycleGroup
					if err := dbq.Order("position ASC, id ASC").Offset((page - 1) * size).Limit(size).Find(&groups).Error; err != nil {
						return nil, fmt.Errorf("db")
					}
					out := make([]map[string]any, 0, len(groups))
					for _, g := range groups {
						var mems []DayShiftCycleGroupMember
						_ = DB.Where("group_id = ?", g.ID).Order("position ASC, id ASC").Find(&mems).Error
						ids := make([]int, 0, len(mems))
						for _, m := range mems {
							ids = append(ids, int(m.UserID))
						}
						out = append(out, map[string]any{"id": g.ID, "cycle_id": g.CycleID, "position": g.Position, "user_ids": ids})
					}
					return map[string]any{
						"items":     out,
						"page":      page,
						"page_size": size,
						"total":     int(total),
						"has_prev":  page > 1,
						"has_next":  int64(page*size) < total,
					}, nil
				}
				return nil, fmt.Errorf("context missing")
			},
		},
	}
}
