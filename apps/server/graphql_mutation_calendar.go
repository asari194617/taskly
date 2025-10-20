package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	graphql "github.com/graphql-go/graphql"
)

// mutationFieldsCalendar groups business calendar mutations.
func mutationFieldsCalendar(t *gqlTypes) graphql.Fields {
	fields := graphql.Fields{}

	fields["adminCalendarUpsertRange"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{"start": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}, "end": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}, "is_business_day": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Boolean)}, "notes": &graphql.ArgumentConfig{Type: graphql.String}},
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
				s := p.Args["start"].(string)
				e := p.Args["end"].(string)
				isBiz := p.Args["is_business_day"].(bool)
				sd, err1 := time.Parse("2006-01-02", s)
				ed, err2 := time.Parse("2006-01-02", e)
				if err1 != nil || err2 != nil || sd.After(ed) {
					return false, fmt.Errorf("invalid dates")
				}
				var notes *string
				if v, ok := p.Args["notes"].(string); ok && strings.TrimSpace(v) != "" {
					nv := v
					notes = &nv
				}
				for d := sd; !d.After(ed); d = d.Add(24 * time.Hour) {
					var bc BusinessCalendar
					if err := DB.Where("date = ?", d.Format("2006-01-02")).First(&bc).Error; err != nil {
						bc = BusinessCalendar{Date: dateOnly(d), IsBusinessDay: isBiz, Notes: notes}
						_ = DB.Create(&bc).Error
					} else {
						bc.IsBusinessDay = isBiz
						bc.Notes = notes
						_ = DB.Save(&bc).Error
					}
				}
				_ = DB.Create(&Log{UserID: &u.ID, Action: "admin.calendar.upsert"}).Error
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["adminCalendarDelete"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"date": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
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
				if _, err := time.Parse("2006-01-02", ds); err != nil {
					return false, fmt.Errorf("invalid date")
				}
				if err := DB.Where("date = ?", ds).Delete(&BusinessCalendar{}).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				_ = DB.Create(&Log{UserID: &u.ID, Action: "admin.calendar.delete"}).Error
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	return fields
}
