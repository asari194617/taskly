package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	graphql "github.com/graphql-go/graphql"
)

// queryFieldsCalendar returns GraphQL fields for business calendar queries.
func queryFieldsCalendar(t *gqlTypes) graphql.Fields {
	return graphql.Fields{
		// Admin: business calendar paginated list with filters
		"adminCalendarList": &graphql.Field{
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "BusinessCalendarList",
				Fields: graphql.Fields{
					"items":     &graphql.Field{Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{Name: "BusinessCalendarEntry2", Fields: graphql.Fields{"date": &graphql.Field{Type: graphql.String}, "is_business_day": &graphql.Field{Type: graphql.Boolean}, "notes": &graphql.Field{Type: graphql.String}}}))},
					"page":      &graphql.Field{Type: graphql.Int},
					"page_size": &graphql.Field{Type: graphql.Int},
					"total":     &graphql.Field{Type: graphql.Int},
					"has_next":  &graphql.Field{Type: graphql.Boolean},
					"has_prev":  &graphql.Field{Type: graphql.Boolean},
				},
			}),
			Args: graphql.FieldConfigArgument{
				"page":         &graphql.ArgumentConfig{Type: graphql.Int},
				"page_size":    &graphql.ArgumentConfig{Type: graphql.Int},
				"include_past": &graphql.ArgumentConfig{Type: graphql.Boolean},
				"order":        &graphql.ArgumentConfig{Type: graphql.String}, // "asc" | "desc"
				"q":            &graphql.ArgumentConfig{Type: graphql.String},
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
					pageSize := 30
					if v, ok := p.Args["page_size"].(int); ok && v > 0 && v <= 200 {
						pageSize = v
					}
					includePast, _ := p.Args["include_past"].(bool)
					order := strings.ToLower(strings.TrimSpace(fmt.Sprint(p.Args["order"])))
					if order != "desc" {
						order = "asc"
					}
					qstr, _ := p.Args["q"].(string)
					qstr = strings.TrimSpace(qstr)

					dbq := DB.Model(&BusinessCalendar{})
					today := time.Now()
					todayDate := dateOnly(today)
					if !includePast {
						dbq = dbq.Where("date >= ?", todayDate)
					}
					if qstr != "" {
						// Optimize date-only search: if q looks like YYYY-MM-DD, filter by date equality to use index
						if regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`).MatchString(qstr) {
							dbq = dbq.Where("date = ?", qstr)
						} else {
							like := "%" + qstr + "%"
							dbq = dbq.Where("notes LIKE ? OR DATE_FORMAT(date,'%Y-%m-%d') LIKE ?", like, like)
						}
					}
					var total int64
					if err := dbq.Count(&total).Error; err != nil {
						return nil, err
					}
					if order == "asc" {
						dbq = dbq.Order("date ASC")
					} else {
						dbq = dbq.Order("date DESC")
					}
					var rows []BusinessCalendar
					if err := dbq.Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
						return nil, err
					}
					items := make([]map[string]any, 0, len(rows))
					for _, it := range rows {
						var notes string
						if it.Notes != nil {
							notes = *it.Notes
						}
						items = append(items, map[string]any{
							"date":            it.Date.Format("2006-01-02"),
							"is_business_day": it.IsBusinessDay,
							"notes":           notes,
						})
					}
					hasPrev := page > 1
					hasNext := int64(page*pageSize) < total
					return map[string]any{
						"items":     items,
						"page":      page,
						"page_size": pageSize,
						"total":     int(total),
						"has_prev":  hasPrev,
						"has_next":  hasNext,
					}, nil
				}
				return nil, fmt.Errorf("context missing")
			},
		},
		// Admin: business calendar entries by date range
		"adminCalendarRange": &graphql.Field{
			Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
				Name:   "BusinessCalendarEntry",
				Fields: graphql.Fields{"date": &graphql.Field{Type: graphql.String}, "is_business_day": &graphql.Field{Type: graphql.Boolean}, "notes": &graphql.Field{Type: graphql.String}},
			})),
			Args: graphql.FieldConfigArgument{
				"start": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"end":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
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
					s := strings.TrimSpace(p.Args["start"].(string))
					e := strings.TrimSpace(p.Args["end"].(string))
					sd, err1 := time.Parse("2006-01-02", s)
					ed, err2 := time.Parse("2006-01-02", e)
					if err1 != nil || err2 != nil || sd.After(ed) {
						return nil, fmt.Errorf("invalid dates")
					}
					var items []BusinessCalendar
					if err := DB.Where("date >= ? AND date <= ?", sd, ed).Order("date ASC").Find(&items).Error; err != nil {
						return nil, err
					}
					out := make([]map[string]any, 0, len(items))
					for _, it := range items {
						var notes string
						if it.Notes != nil {
							notes = *it.Notes
						}
						out = append(out, map[string]any{
							"date":            it.Date.Format("2006-01-02"),
							"is_business_day": it.IsBusinessDay,
							"notes":           notes,
						})
					}
					return out, nil
				}
				return nil, fmt.Errorf("context missing")
			},
		},
	}
}
