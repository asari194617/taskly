package main

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	graphql "github.com/graphql-go/graphql"
)

// mutationFieldsKeys groups API key and iCal token mutations, and MCP token mutations.
func mutationFieldsKeys(t *gqlTypes) graphql.Fields {
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
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !userFromSession(c) {
					return nil, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*User)
				limit, err := apiKeyLimitForUser(u.ID)
				if err != nil {
					return nil, err
				}
				if limit >= 0 {
					var cur int64
					if err := DB.Model(&APIKey{}).Where("user_id = ? AND is_valid = 1", u.ID).Count(&cur).Error; err != nil {
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
				plain, hashed, genErr := generateAPIKey()
				if genErr != nil {
					return nil, fmt.Errorf("gen")
				}
				rec := APIKey{UserID: u.ID, HashedToken: hashed, Description: descPtr}
				if err := DB.Create(&rec).Error; err != nil {
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
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !userFromSession(c) {
					return nil, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*User)
				id, _ := p.Args["id"].(int)
				var rec APIKey
				if err := DB.Where("id = ? AND user_id = ?", id, u.ID).First(&rec).Error; err != nil {
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
				if err := DB.Save(&rec).Error; err != nil {
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
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !userFromSession(c) {
					return nil, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*User)
				desc, _ := p.Args["description"].(string)
				var descPtr *string
				if strings.TrimSpace(desc) != "" {
					descPtr = &desc
				}
				tok := generateIcalToken()
				rec := IcalKey{UserID: u.ID, Token: tok, Description: descPtr}
				if err := DB.Create(&rec).Error; err != nil {
					return nil, fmt.Errorf("db")
				}
				base := strings.TrimRight(env("ICAL_BASE_URL", env("API_PUBLIC_URL", "http://localhost:8080")+"/ical"), "/")
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
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !userFromSession(c) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*User)
				id, _ := p.Args["id"].(int)
				if err := DB.Where("id = ? AND user_id = ?", id, u.ID).Delete(&IcalKey{}).Error; err != nil {
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
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !userFromSession(c) {
					return nil, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*User)
				if strings.ToLower(env("MCP_ENABLED", "true")) != "true" {
					return nil, fmt.Errorf("disabled")
				}
				plain, hashed, err := generateAPIKey()
				if err != nil {
					return nil, fmt.Errorf("gen")
				}
				rec := MCP{UserID: u.ID, HashedToken: hashed}
				_ = DB.Where("user_id = ?", u.ID).Delete(&MCP{}).Error
				if err := DB.Create(&rec).Error; err != nil {
					return nil, fmt.Errorf("db")
				}
				_ = DB.Create(&Log{UserID: &u.ID, Action: "mcp.create"}).Error
				return map[string]any{"ok": true, "token": plain}, nil
			}
			return nil, fmt.Errorf("context missing")
		},
	}

	fields["mcpRevoke"] = &graphql.Field{
		Type: graphql.Boolean,
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !userFromSession(c) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*User)
				if err := DB.Where("user_id = ?", u.ID).Delete(&MCP{}).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				_ = DB.Create(&Log{UserID: &u.ID, Action: "mcp.revoke"}).Error
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	return fields
}
