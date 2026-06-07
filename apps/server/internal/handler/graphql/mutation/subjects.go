package gqlMutation

import (
	"fmt"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	gqlH "github.com/asari194617/taskly/apps/server/internal/handler/graphql/shared"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	graphql "github.com/graphql-go/graphql"
)

// mutationFieldsSubjects groups subject CRUD mutations for admin/managers.
func (h *Handler) mutationFieldsSubjects(t *gqlH.GqlTypes) graphql.Fields {
	fields := graphql.Fields{}

	subjectType := t.SubjectType

	fields["adminSubjectCreate"] = &graphql.Field{
		Type: subjectType,
		Args: graphql.FieldConfigArgument{
			"name_ja":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			"name_en":    &graphql.ArgumentConfig{Type: graphql.String},
			"expires_at": &graphql.ArgumentConfig{Type: graphql.String},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return nil, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				isAdmin, _ := utils.UserHasRole(u.ID, "admin", h.db)
				isManager, _ := utils.UserHasRole(u.ID, "manager", h.db)
				if !(isAdmin || isManager) {
					return nil, fmt.Errorf("forbidden")
				}
				nameJa := strings.TrimSpace(p.Args["name_ja"].(string))
				if nameJa == "" {
					return nil, fmt.Errorf("invalid name_ja")
				}
				var nameEnPtr *string
				if v, ok := p.Args["name_en"].(string); ok {
					vv := strings.TrimSpace(v)
					if vv != "" {
						nameEnPtr = &vv
					}
				}
				var expPtr *time.Time
				if v, ok := p.Args["expires_at"].(string); ok {
					vv := strings.TrimSpace(v)
					if vv != "" {
						if t, err := time.Parse(time.RFC3339, vv); err == nil {
							expPtr = &t
						} else {
							return nil, fmt.Errorf("invalid expires_at")
						}
					}
				}
				rec := domain.Subject{NameJa: nameJa, NameEn: nameEnPtr, ExpiresAt: expPtr}
				if err := h.db.Create(&rec).Error; err != nil {
					return nil, fmt.Errorf("db")
				}
				out := map[string]any{
					"id":         rec.ID,
					"name_ja":    rec.NameJa,
					"name_en":    rec.NameEn,
					"created_at": rec.CreatedAt.Format(time.RFC3339),
				}
				if rec.ExpiresAt != nil {
					out["expires_at"] = rec.ExpiresAt.Format(time.RFC3339)
				}
				return out, nil
			}
			return nil, fmt.Errorf("context missing")
		},
	}

	fields["adminSubjectUpdate"] = &graphql.Field{
		Type: subjectType,
		Args: graphql.FieldConfigArgument{
			"id":         &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
			"name_ja":    &graphql.ArgumentConfig{Type: graphql.String},
			"name_en":    &graphql.ArgumentConfig{Type: graphql.String},
			"expires_at": &graphql.ArgumentConfig{Type: graphql.String},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return nil, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				isAdmin, _ := utils.UserHasRole(u.ID, "admin", h.db)
				isManager, _ := utils.UserHasRole(u.ID, "manager", h.db)
				if !(isAdmin || isManager) {
					return nil, fmt.Errorf("forbidden")
				}
				id := p.Args["id"].(int)
				var rec domain.Subject
				if err := h.db.First(&rec, id).Error; err != nil {
					return nil, fmt.Errorf("not found")
				}
				if v, ok := p.Args["name_ja"].(string); ok {
					vv := strings.TrimSpace(v)
					if vv == "" {
						return nil, fmt.Errorf("invalid name_ja")
					}
					rec.NameJa = vv
				}
				if v, ok := p.Args["name_en"].(string); ok {
					vv := strings.TrimSpace(v)
					if vv == "" {
						rec.NameEn = nil
					} else {
						rec.NameEn = &vv
					}
				}
				if v, ok := p.Args["expires_at"].(string); ok {
					vv := strings.TrimSpace(v)
					if vv == "" {
						rec.ExpiresAt = nil
					} else {
						if t, err := time.Parse(time.RFC3339, vv); err == nil {
							rec.ExpiresAt = &t
						} else {
							return nil, fmt.Errorf("invalid expires_at")
						}
					}
				}
				if err := h.db.Save(&rec).Error; err != nil {
					return nil, fmt.Errorf("db")
				}
				out := map[string]any{
					"id":         rec.ID,
					"name_ja":    rec.NameJa,
					"name_en":    rec.NameEn,
					"created_at": rec.CreatedAt.Format(time.RFC3339),
				}
				if rec.ExpiresAt != nil {
					out["expires_at"] = rec.ExpiresAt.Format(time.RFC3339)
				}
				return out, nil
			}
			return nil, fmt.Errorf("context missing")
		},
	}

	fields["adminSubjectDelete"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"id":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
			"hard": &graphql.ArgumentConfig{Type: graphql.Boolean},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				isAdmin, _ := utils.UserHasRole(u.ID, "admin", h.db)
				isManager, _ := utils.UserHasRole(u.ID, "manager", h.db)
				if !(isAdmin || isManager) {
					return false, fmt.Errorf("forbidden")
				}
				id := p.Args["id"].(int)
				hard, _ := p.Args["hard"].(bool)
				if hard {
					if err := h.db.Delete(&domain.Subject{}, id).Error; err != nil {
						return false, fmt.Errorf("db")
					}
					return true, nil
				}
				// Soft delete by setting expires_at to now
				now := time.Now()
				if err := h.db.Model(&domain.Subject{}).Where("id = ?", id).Update("expires_at", now).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	return fields
}
