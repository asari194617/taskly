package gqlQuery

import (
	"fmt"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	gqlShared "github.com/asari194617/taskly/apps/server/internal/handler/graphql/shared"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	graphql "github.com/graphql-go/graphql"
)

// queryFieldsAdminSubjects returns admin-scoped subject listing including expired.
func (h *Handler) queryFieldsAdminSubjects(t *gqlShared.GqlTypes) graphql.Fields {
	subjectType := t.SubjectType
	return graphql.Fields{
		"adminSubjects": &graphql.Field{
			Type: graphql.NewList(subjectType),
			Args: graphql.FieldConfigArgument{
				"include_expired": &graphql.ArgumentConfig{Type: graphql.Boolean},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				if c, ok := gqlShared.FiberCtxFromContext(p.Context); ok {
					if !utils.UserFromSession(c, h.db, h.rdb) {
						return nil, fmt.Errorf("unauthorized")
					}
					u := c.Locals("user").(*domain.User)
					isAdmin, _ := utils.UserHasRole(u.ID, "admin", h.db)
					isManager, _ := utils.UserHasRole(u.ID, "manager", h.db)
					if !(isAdmin || isManager) {
						return nil, fmt.Errorf("forbidden")
					}
					includeExpired, _ := p.Args["include_expired"].(bool)
					dbq := h.db.Model(&domain.Subject{})
					if !includeExpired {
						dbq = dbq.Where("expires_at IS NULL OR expires_at > ?", time.Now())
					}
					var subs []domain.Subject
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
