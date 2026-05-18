package main

import (
	"fmt"
	"strings"

	utils "github.com/asari194617/taskly/apps/server/utils"
	"github.com/gofiber/fiber/v2"
	graphql "github.com/graphql-go/graphql"
)

// queryFieldsDayShift returns GraphQL fields related to day shift (assignments, groups, cycles).
func queryFieldsDayShift(t *gqlTypes) graphql.Fields {
	dayShiftGroupType := t.DayShiftGroupType
	dayShiftCycleType := t.DayShiftCycleType
	dayShiftCycleListType := t.DayShiftCycleListType
	dayShiftGroupListType := t.DayShiftGroupListType

	return graphql.Fields{
		// IDs only for groups (to help reordering with server-side pagination)
		"adminDayshiftGroupIds": &graphql.Field{
			Type: graphql.NewList(graphql.Int),
			Args: graphql.FieldConfigArgument{
				"cycle_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
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
					var groups []utils.DayShiftCycleGroup
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
					if !utils.UserFromSession(c, DB, RDB) {
						return nil, fmt.Errorf("unauthorized")
					}
					u := c.Locals("user").(*utils.User)
					isAdmin, _ := utils.UserHasRole(u.ID, "admin", DB)
					isManager, _ := utils.UserHasRole(u.ID, "manager", DB)
					if !(isAdmin || isManager) {
						return nil, fmt.Errorf("forbidden")
					}
					var items []utils.DayShiftCycle
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
					if !utils.UserFromSession(c, DB, RDB) {
						return nil, fmt.Errorf("unauthorized")
					}
					u := c.Locals("user").(*utils.User)
					isAdmin, _ := utils.UserHasRole(u.ID, "admin", DB)
					isManager, _ := utils.UserHasRole(u.ID, "manager", DB)
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
					dbq := DB.Model(&utils.DayShiftCycle{})
					if qstr != "" {
						dbq = dbq.Where("description LIKE ?", "%"+qstr+"%")
					}
					var total int64
					if err := dbq.Count(&total).Error; err != nil {
						return nil, err
					}
					var items []utils.DayShiftCycle
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
					var groups []utils.DayShiftCycleGroup
					if err := DB.Where("cycle_id = ?", cid).Order("position ASC, id ASC").Find(&groups).Error; err != nil {
						return nil, fmt.Errorf("db")
					}
					out := make([]map[string]any, 0, len(groups))
					for _, g := range groups {
						var mems []utils.DayShiftCycleGroupMember
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
					page := 1
					if v, ok := p.Args["page"].(int); ok && v > 0 {
						page = v
					}
					size := 30
					if v, ok := p.Args["page_size"].(int); ok && v > 0 && v <= 200 {
						size = v
					}
					dbq := DB.Model(&utils.DayShiftCycleGroup{}).Where("cycle_id = ?", cid)
					var total int64
					if err := dbq.Count(&total).Error; err != nil {
						return nil, err
					}
					var groups []utils.DayShiftCycleGroup
					if err := dbq.Order("position ASC, id ASC").Offset((page - 1) * size).Limit(size).Find(&groups).Error; err != nil {
						return nil, fmt.Errorf("db")
					}
					out := make([]map[string]any, 0, len(groups))
					for _, g := range groups {
						var mems []utils.DayShiftCycleGroupMember
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
