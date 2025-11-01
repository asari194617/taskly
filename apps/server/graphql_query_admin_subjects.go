package main

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	graphql "github.com/graphql-go/graphql"
)

// queryFieldsAdminSubjects returns admin-scoped subject listing including expired.
func queryFieldsAdminSubjects(t *gqlTypes) graphql.Fields {
	subjectType := t.SubjectType
	return graphql.Fields{
		"adminSubjects": &graphql.Field{
			Type: graphql.NewList(subjectType),
			Args: graphql.FieldConfigArgument{
				"include_expired": &graphql.ArgumentConfig{Type: graphql.Boolean},
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
					includeExpired, _ := p.Args["include_expired"].(bool)
					dbq := DB.Model(&Subject{})
					if !includeExpired {
						dbq = dbq.Where("expires_at IS NULL OR expires_at > ?", time.Now())
					}
					var subs []Subject
					if err := dbq.Order("id ASC").Find(&subs).Error; err != nil {
						return nil, err
					}
					out := make([]map[string]any, 0, len(subs))
					for _, s := range subs {
						row := map[string]any{
							"id":         s.ID,
							"name_ja":    s.NameJa,
							"name_en":    s.NameEn,
							"created_at": s.CreatedAt.Format(time.RFC3339),
						}
						if s.ExpiresAt != nil {
							row["expires_at"] = s.ExpiresAt.Format(time.RFC3339)
						}
						out = append(out, row)
					}
					return out, nil
				}
				return nil, fmt.Errorf("context missing")
			},
		},
	}
}
