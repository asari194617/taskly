package gqlMutation

import (
	"fmt"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	gqlH "github.com/asari194617/taskly/apps/server/internal/handler/graphql/shared"
	utils "github.com/asari194617/taskly/apps/server/internal/utils"
	graphql "github.com/graphql-go/graphql"
)

// mutationFieldsKeys groups API key and iCal token mutations, and domain.MCP token mutations.
func (h *Handler) mutationFieldsKeys() graphql.Fields {
	fields := graphql.Fields{}

	fields["apiKeyCreate"] = &graphql.Field{
		Type: graphql.NewObject(graphql.ObjectConfig{
			Name: "ApiKeyCreateResult",
			Fields: graphql.Fields{
				"id":    &graphql.Field{Type: graphql.Int},
				"token": &graphql.Field{Type: graphql.String},
			},
		}),
		Args: graphql.FieldConfigArgument{
			"description": &graphql.ArgumentConfig{Type: graphql.String},
			"expires_at":  &graphql.ArgumentConfig{Type: graphql.String},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return nil, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				limit, err := utils.ApiKeyLimitForUser(u.ID, h.db)
				if err != nil {
					return nil, err
				}
				if limit >= 0 {
					var cur int64
					if err := h.db.Model(&domain.APIKey{}).Where("user_id = ? AND is_valid = 1", u.ID).Count(&cur).Error; err != nil {
						return nil, err
					}
					if int(cur) >= limit {
						return nil, fmt.Errorf("api key limit reached")
					}
				}
				desc, _ := p.Args["description"].(string)
				var descPtr *string
				if strings.TrimSpace(desc) != "" {
					descPtr = &desc
				}
				plain, hashed, genErr := utils.GenerateAPIKey()
				if genErr != nil {
					return nil, fmt.Errorf("gen")
				}
				var expPtr *time.Time
				if s, ok := p.Args["expires_at"].(string); ok && strings.TrimSpace(s) != "" {
					if t, err := time.Parse(time.RFC3339, s); err == nil {
						expPtr = &t
					} else {
						return nil, fmt.Errorf("invalid expires_at")
					}
				}
				rec := domain.APIKey{UserID: u.ID, HashedToken: hashed, Description: descPtr, ExpiresAt: expPtr}
				if err := h.db.Create(&rec).Error; err != nil {
					return nil, fmt.Errorf("db")
				}
				return map[string]any{"id": rec.ID, "token": plain}, nil
			}
			return nil, fmt.Errorf("context missing")
		},
	}

	fields["apiKeyUpdate"] = &graphql.Field{
		Type: graphql.NewObject(graphql.ObjectConfig{
			Name: "ApiKeyUpdateResult",
			Fields: graphql.Fields{
				"id":       &graphql.Field{Type: graphql.Int},
				"is_valid": &graphql.Field{Type: graphql.Boolean},
			},
		}),
		Args: graphql.FieldConfigArgument{
			"id":          &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
			"is_valid":    &graphql.ArgumentConfig{Type: graphql.Boolean},
			"description": &graphql.ArgumentConfig{Type: graphql.String},
			"expires_at":  &graphql.ArgumentConfig{Type: graphql.String},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return nil, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				id, _ := p.Args["id"].(int)
				var rec domain.APIKey
				if err := h.db.Where("id = ? AND user_id = ?", id, u.ID).First(&rec).Error; err != nil {
					return nil, fmt.Errorf("not found")
				}
				if v, ok := p.Args["is_valid"].(bool); ok {
					rec.IsValid = v
				}
				if v, ok := p.Args["description"].(string); ok {
					if strings.TrimSpace(v) == "" {
						rec.Description = nil
					} else {
						rec.Description = &v
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
				return map[string]any{"id": rec.ID, "is_valid": rec.IsValid}, nil
			}
			return nil, fmt.Errorf("context missing")
		},
	}

	fields["icalCreate"] = &graphql.Field{
		Type: graphql.NewObject(graphql.ObjectConfig{
			Name: "IcalCreateResult",
			Fields: graphql.Fields{
				"id":  &graphql.Field{Type: graphql.Int},
				"url": &graphql.Field{Type: graphql.String},
			},
		}),
		Args: graphql.FieldConfigArgument{
			"description": &graphql.ArgumentConfig{Type: graphql.String},
			"expires_at":  &graphql.ArgumentConfig{Type: graphql.String},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return nil, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				desc, _ := p.Args["description"].(string)
				var descPtr *string
				if strings.TrimSpace(desc) != "" {
					descPtr = &desc
				}
				tok, err := utils.GenerateIcalToken()
				if err != nil {
					return nil, fmt.Errorf("server")
				}
				var expPtr *time.Time
				if s, ok := p.Args["expires_at"].(string); ok && strings.TrimSpace(s) != "" {
					if t, err := time.Parse(time.RFC3339, s); err == nil {
						expPtr = &t
					} else {
						return nil, fmt.Errorf("invalid expires_at")
					}
				}
				rec := domain.IcalKey{UserID: u.ID, Token: tok, Description: descPtr, ExpiresAt: expPtr}
				if err := h.db.Create(&rec).Error; err != nil {
					return nil, fmt.Errorf("db")
				}
				base := strings.TrimRight(utils.Env("ICAL_BASE_URL", utils.Env("API_PUBLIC_URL", "http://localhost:8080")+"/ical"), "/")
				return map[string]any{"id": rec.ID, "url": fmt.Sprintf("%s/%s.ics", base, rec.Token)}, nil
			}
			return nil, fmt.Errorf("context missing")
		},
	}

	fields["icalDelete"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				id, _ := p.Args["id"].(int)
				if err := h.db.Where("id = ? AND user_id = ?", id, u.ID).Delete(&domain.IcalKey{}).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	// Update iCal token expiration
	fields["icalUpdate"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"id":         &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
			"expires_at": &graphql.ArgumentConfig{Type: graphql.String},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				id, _ := p.Args["id"].(int)
				var rec domain.IcalKey
				if err := h.db.Where("id = ? AND user_id = ?", id, u.ID).First(&rec).Error; err != nil {
					return false, fmt.Errorf("not found")
				}
				if v, ok := p.Args["expires_at"].(string); ok {
					vv := strings.TrimSpace(v)
					if vv == "" {
						rec.ExpiresAt = nil
					} else {
						if t, err := time.Parse(time.RFC3339, vv); err == nil {
							rec.ExpiresAt = &t
						} else {
							return false, fmt.Errorf("invalid expires_at")
						}
					}
				}
				if err := h.db.Save(&rec).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["mcpCreate"] = &graphql.Field{
		Type: graphql.NewObject(graphql.ObjectConfig{
			Name: "McpCreateResult",
			Fields: graphql.Fields{
				"ok":    &graphql.Field{Type: graphql.Boolean},
				"token": &graphql.Field{Type: graphql.String},
			},
		}),
		Args: graphql.FieldConfigArgument{
			"expires_at": &graphql.ArgumentConfig{Type: graphql.String},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return nil, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				if strings.ToLower(utils.Env("MCP_ENABLED", "true")) != "true" {
					return nil, fmt.Errorf("disabled")
				}
				plain, hashed, err := utils.GenerateAPIKey()
				if err != nil {
					return nil, fmt.Errorf("gen")
				}
				var expPtr *time.Time
				if s, ok := p.Args["expires_at"].(string); ok && strings.TrimSpace(s) != "" {
					if t, e := time.Parse(time.RFC3339, s); e == nil {
						expPtr = &t
					} else {
						return nil, fmt.Errorf("invalid expires_at")
					}
				}
				rec := domain.MCP{UserID: u.ID, HashedToken: hashed, ExpiresAt: expPtr}
				_ = h.db.Where("user_id = ?", u.ID).Delete(&domain.MCP{}).Error
				if err := h.db.Create(&rec).Error; err != nil {
					return nil, fmt.Errorf("db")
				}
				h.recordAuditLog(&domain.Log{UserID: &u.ID, Action: "mcp.create"})
				return map[string]any{"ok": true, "token": plain}, nil
			}
			return nil, fmt.Errorf("context missing")
		},
	}

	fields["mcpRevoke"] = &graphql.Field{
		Type: graphql.Boolean,
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				if err := h.db.Where("user_id = ?", u.ID).Delete(&domain.MCP{}).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				h.recordAuditLog(&domain.Log{UserID: &u.ID, Action: "mcp.revoke"})
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	// Update MCP token expiration
	fields["mcpUpdate"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"expires_at": &graphql.ArgumentConfig{Type: graphql.String},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				var rec domain.MCP
				if err := h.db.Where("user_id = ?", u.ID).First(&rec).Error; err != nil {
					return false, fmt.Errorf("not found")
				}
				if v, ok := p.Args["expires_at"].(string); ok {
					vv := strings.TrimSpace(v)
					if vv == "" {
						rec.ExpiresAt = nil
					} else {
						if t, err := time.Parse(time.RFC3339, vv); err == nil {
							rec.ExpiresAt = &t
						} else {
							return false, fmt.Errorf("invalid expires_at")
						}
					}
				}
				if err := h.db.Save(&rec).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				h.recordAuditLog(&domain.Log{UserID: &u.ID, Action: "mcp.update"})
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	return fields
}
