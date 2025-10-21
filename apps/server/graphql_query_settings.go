package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	graphql "github.com/graphql-go/graphql"
)

// queryFieldsSettings returns GraphQL fields for settings: API keys, iCal tokens, MCP, logs.
func queryFieldsSettings(t *gqlTypes) graphql.Fields {
	apiKeyType := t.ApiKeyType
	icalTokenType := t.IcalTokenType
	mcpInfoType := t.McpInfoType
	logItemType := t.LogItemType

	return graphql.Fields{
		// Settings: API Keys
		"apiKeys": &graphql.Field{
			Type: graphql.NewList(apiKeyType),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
					if !userFromSession(c) {
						return nil, fmt.Errorf("unauthorized")
					}
					u := c.Locals("user").(*User)
					var items []APIKey
					if err := DB.Where("user_id = ?", u.ID).Order("id DESC").Find(&items).Error; err != nil {
						return nil, err
					}
					out := make([]map[string]any, 0, len(items))
					for _, it := range items {
						out = append(out, map[string]any{
							"id":          it.ID,
							"description": it.Description,
							"is_valid":    it.IsValid,
							"created_at":  it.CreatedAt.Format(time.RFC3339),
						})
					}
					return out, nil
				}
				return nil, fmt.Errorf("context missing")
			},
		},
		// Settings: iCal tokens
		"icalTokens": &graphql.Field{
			Type: graphql.NewList(icalTokenType),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
					if !userFromSession(c) {
						return nil, fmt.Errorf("unauthorized")
					}
					u := c.Locals("user").(*User)
					var items []IcalKey
					if err := DB.Where("user_id = ?", u.ID).Order("id DESC").Find(&items).Error; err != nil {
						return nil, err
					}
					base := strings.TrimRight(env("ICAL_BASE_URL", env("API_PUBLIC_URL", "http://localhost:8080")+"/ical"), "/")
					out := make([]map[string]any, 0, len(items))
					for _, it := range items {
						out = append(out, map[string]any{
							"id":          it.ID,
							"description": it.Description,
							"url":         fmt.Sprintf("%s/%s.ics", base, it.Token),
							"created_at":  it.CreatedAt.Format(time.RFC3339),
							"expires_at": func() any {
								if it.ExpiresAt != nil {
									return it.ExpiresAt.Format(time.RFC3339)
								}
								return nil
							}(),
						})
					}
					return out, nil
				}
				return nil, fmt.Errorf("context missing")
			},
		},
		// MCP info for current user
		"mcpInfo": &graphql.Field{
			Type: mcpInfoType,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
					if !userFromSession(c) {
						return nil, fmt.Errorf("unauthorized")
					}
					u := c.Locals("user").(*User)
					enabled := strings.ToLower(env("MCP_ENABLED", "true")) == "true"
					var cnt int64
					_ = DB.Model(&MCP{}).Where("user_id = ?", u.ID).Count(&cnt).Error
					return map[string]any{"enabled": enabled, "has_token": cnt > 0}, nil
				}
				return nil, fmt.Errorf("context missing")
			},
		},
		// Logs for current user (prefix/limit)
		"myLogs": &graphql.Field{
			Type: graphql.NewList(logItemType),
			Args: graphql.FieldConfigArgument{
				"prefix": &graphql.ArgumentConfig{Type: graphql.String},
				"limit":  &graphql.ArgumentConfig{Type: graphql.Int},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
					if !userFromSession(c) {
						return nil, fmt.Errorf("unauthorized")
					}
					u := c.Locals("user").(*User)
					pref, _ := p.Args["prefix"].(string)
					limit := 20
					if v, ok := p.Args["limit"].(int); ok && v > 0 && v <= 100 {
						limit = v
					}
					q := DB.Model(&Log{}).Where("user_id = ?", u.ID)
					if strings.TrimSpace(pref) != "" {
						q = q.Where("action LIKE ?", strings.TrimSpace(pref)+"%")
					}
					q = q.Order("id DESC").Limit(limit)
					var items []Log
					if err := q.Find(&items).Error; err != nil {
						return nil, err
					}
					out := make([]map[string]any, 0, len(items))
					for _, it := range items {
						out = append(out, map[string]any{"action": it.Action, "created_at": it.CreatedAt.Format(time.RFC3339)})
					}
					return out, nil
				}
				return nil, fmt.Errorf("context missing")
			},
		},
	}
}
