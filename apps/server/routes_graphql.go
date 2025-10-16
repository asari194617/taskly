package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v2"
	graphql "github.com/graphql-go/graphql"
	"golang.org/x/crypto/bcrypt"
)

// registerGraphQL builds the schema and mounts the /graphql endpoint.
func registerGraphQL(app *fiber.App) {
	// Types
	subjectType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Subject",
		Fields: graphql.Fields{
			"id":      &graphql.Field{Type: graphql.Int},
			"name_ja": &graphql.Field{Type: graphql.String},
			"name_en": &graphql.Field{Type: graphql.String},
			"name": &graphql.Field{Type: graphql.String, Resolve: func(p graphql.ResolveParams) (any, error) {
				// Resolve localized name based on current user's notification language
				var subj Subject
				if m, ok := p.Source.(map[string]any); ok {
					if idv, ok2 := m["id"]; ok2 {
						var idUint uint
						switch vv := idv.(type) {
						case int:
							idUint = uint(vv)
						case uint:
							idUint = vv
						case int64:
							idUint = uint(vv)
						case float64:
							idUint = uint(vv)
						}
						_ = DB.First(&subj, idUint).Error
					}
				}
				lang := "ja"
				if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
					if u, ok2 := c.Locals("user").(*User); ok2 && strings.TrimSpace(u.NotificationLang) != "" {
						lang = strings.ToLower(u.NotificationLang)
					}
				}
				if lang == "en" {
					if subj.NameEn != nil && *subj.NameEn != "" {
						return *subj.NameEn, nil
					}
				}
				if strings.TrimSpace(subj.NameJa) != "" {
					return subj.NameJa, nil
				}
				return "", nil
			}},
		},
	})
	taskType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Task",
		Fields: graphql.Fields{
			"id":                  &graphql.Field{Type: graphql.Int},
			"title":               &graphql.Field{Type: graphql.String},
			"subject_id":          &graphql.Field{Type: graphql.Int},
			"description":         &graphql.Field{Type: graphql.String},
			"type":                &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"is_require_hand_in":  &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"self_check_required": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"deadline":            &graphql.Field{Type: graphql.NewNonNull(graphql.String)}, // YYYY-MM-DD
			"deadline_since":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)}, // ENUM
			"deadline_until":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)}, // ENUM
			"created_at":          &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"updated_at":          &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	meType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Me",
		Fields: graphql.Fields{
			"id":                 &graphql.Field{Type: graphql.Int},
			"email":              &graphql.Field{Type: graphql.String},
			"display_name":       &graphql.Field{Type: graphql.String},
			"is_admin":           &graphql.Field{Type: graphql.Boolean},
			"is_manager":         &graphql.Field{Type: graphql.Boolean},
			"is_registered":      &graphql.Field{Type: graphql.Boolean},
			"is_agreed_policies": &graphql.Field{Type: graphql.Boolean},
		},
	})
	authSettingsType := graphql.NewObject(graphql.ObjectConfig{
		Name: "AuthSettings",
		Fields: graphql.Fields{
			"email_login_enabled": &graphql.Field{Type: graphql.Boolean},
			"has_password":        &graphql.Field{Type: graphql.Boolean},
			"totp_enabled":        &graphql.Field{Type: graphql.Boolean},
		},
	})

	// Settings and integrations types
	apiKeyType := graphql.NewObject(graphql.ObjectConfig{
		Name: "ApiKey",
		Fields: graphql.Fields{
			"id":          &graphql.Field{Type: graphql.Int},
			"description": &graphql.Field{Type: graphql.String},
			"is_valid":    &graphql.Field{Type: graphql.Boolean},
			"created_at":  &graphql.Field{Type: graphql.String},
		},
	})
	icalTokenType := graphql.NewObject(graphql.ObjectConfig{
		Name: "IcalToken",
		Fields: graphql.Fields{
			"id":          &graphql.Field{Type: graphql.Int},
			"description": &graphql.Field{Type: graphql.String},
			"url":         &graphql.Field{Type: graphql.String},
			"created_at":  &graphql.Field{Type: graphql.String},
		},
	})
	mcpInfoType := graphql.NewObject(graphql.ObjectConfig{
		Name: "McpInfo",
		Fields: graphql.Fields{
			"enabled":   &graphql.Field{Type: graphql.Boolean},
			"has_token": &graphql.Field{Type: graphql.Boolean},
		},
	})
	logItemType := graphql.NewObject(graphql.ObjectConfig{
		Name: "LogItem",
		Fields: graphql.Fields{
			"action":     &graphql.Field{Type: graphql.String},
			"created_at": &graphql.Field{Type: graphql.String},
		},
	})

	// Root Query (subset: we keep existing behavior)
	rootQuery := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
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
			"subjects": &graphql.Field{
				Type: graphql.NewList(subjectType),
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
						if !userFromSession(c) { // session only
							return nil, fmt.Errorf("unauthorized")
						}
						var subs []Subject
						now := time.Now()
						if err := DB.Where("expires_at IS NULL OR expires_at > ?", now).Order("id ASC").Find(&subs).Error; err != nil {
							return nil, err
						}
						out := make([]map[string]any, 0, len(subs))
						for _, s := range subs {
							out = append(out, map[string]any{"id": s.ID, "name_ja": s.NameJa, "name_en": s.NameEn})
						}
						return out, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			"me": &graphql.Field{
				Type: meType,
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						var roles []RoleUser
						_ = DB.Where("user_id = ?", u.ID).Find(&roles).Error
						isAdmin := false
						isManager := false
						for _, r := range roles {
							if r.Role == "admin" {
								isAdmin = true
							}
							if r.Role == "manager" {
								isManager = true
							}
						}
						return map[string]any{
							"id":                 u.ID,
							"email":              u.Email,
							"display_name":       u.DisplayName,
							"is_admin":           isAdmin,
							"is_manager":         isManager,
							"is_registered":      u.IsRegistered,
							"is_agreed_policies": u.IsAgreedPolicies,
						}, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			"authSettings": &graphql.Field{
				Type: authSettingsType,
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						var fresh User
						if err := DB.First(&fresh, u.ID).Error; err != nil {
							return nil, err
						}
						hasPassword := fresh.HashedPassword != nil && *fresh.HashedPassword != ""
						totpEnabled, _, _ := getActiveTOTP(fresh.ID)
						return map[string]any{
							"email_login_enabled": fresh.EmailCodeAllowed,
							"has_password":        hasPassword,
							"totp_enabled":        totpEnabled,
						}, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			"tasks": &graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: "TasksResult",
					Fields: graphql.Fields{
						"items": &graphql.Field{Type: graphql.NewList(taskType)},
						"next_cursor": &graphql.Field{Type: graphql.NewObject(graphql.ObjectConfig{
							Name: "TasksCursor",
							Fields: graphql.Fields{
								"cursor_deadline": &graphql.Field{Type: graphql.String},
								"cursor_id":       &graphql.Field{Type: graphql.String},
							},
						})},
					},
				}),
				Args: graphql.FieldConfigArgument{
					"limit":           &graphql.ArgumentConfig{Type: graphql.Int},
					"overdue":         &graphql.ArgumentConfig{Type: graphql.Boolean},
					"q":               &graphql.ArgumentConfig{Type: graphql.String},
					"cursor_deadline": &graphql.ArgumentConfig{Type: graphql.String},
					"cursor_id":       &graphql.ArgumentConfig{Type: graphql.String},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						if okp, err := checkPermission(u.ID, "tasks", "read", nil); err != nil {
							return nil, err
						} else if !okp {
							return nil, fmt.Errorf("forbidden")
						}
						overdue, _ := p.Args["overdue"].(bool)
						qstr, _ := p.Args["q"].(string)
						limit := 20
						if v, ok := p.Args["limit"].(int); ok && v > 0 && v <= 200 {
							limit = v
						}
						var cursorDeadline *time.Time
						if s, _ := p.Args["cursor_deadline"].(string); s != "" {
							if t, err := time.Parse(time.RFC3339, s); err == nil {
								cursorDeadline = &t
							}
						}
						var cursorID uint
						if s, _ := p.Args["cursor_id"].(string); s != "" {
							if v, err := strconv.Atoi(s); err == nil && v > 0 {
								cursorID = uint(v)
							}
						}
						dbq := DB.Model(&Task{})
						now := time.Now()
						eff := effectiveDeadlineExpr("")
						if overdue {
							dbq = dbq.Where(eff+" < ?", now)
						} else {
							dbq = dbq.Where(eff+" >= ?", now)
						}
						if strings.TrimSpace(qstr) != "" {
							like := "%" + strings.TrimSpace(qstr) + "%"
							dbq = dbq.Where("title LIKE ? OR description LIKE ?", like, like)
						}
						if cursorDeadline != nil {
							if overdue {
								dbq = dbq.Where("("+eff+" < ?) OR ("+eff+" = ? AND id < ?)", cursorDeadline, cursorDeadline, cursorID)
							} else {
								dbq = dbq.Where("("+eff+" > ?) OR ("+eff+" = ? AND id > ?)", cursorDeadline, cursorDeadline, cursorID)
							}
						} else if cursorID != 0 {
							if overdue {
								dbq = dbq.Where("id < ?", cursorID)
							} else {
								dbq = dbq.Where("id > ?", cursorID)
							}
						}
						if overdue {
							dbq = dbq.Order(eff + " DESC").Order("id DESC")
						} else {
							dbq = dbq.Order(eff + " ASC").Order("id ASC")
						}
						var items []Task
						if err := dbq.Limit(limit + 1).Find(&items).Error; err != nil {
							return nil, err
						}
						var nextCursor map[string]string
						if len(items) > limit {
							last := items[limit-1]
							items = items[:limit]
							ed := effectiveDeadlineTime(last)
							nextCursor = map[string]string{"cursor_deadline": ed.Format(time.RFC3339), "cursor_id": fmt.Sprintf("%d", last.ID)}
						}
						// serialize
						outItems := make([]map[string]any, 0, len(items))
						for _, it := range items {
							out := map[string]any{
								"id":                  it.ID,
								"title":               it.Title,
								"subject_id":          it.SubjectID,
								"description":         it.Description,
								"type":                it.Type,
								"is_require_hand_in":  it.IsRequireHandIn,
								"self_check_required": it.SelfCheckRequired,
							}
							out["deadline"] = it.Deadline.Format("2006-01-02")
							out["deadline_since"] = it.DeadlineSince
							out["deadline_until"] = it.DeadlineUntil
							out["created_at"] = it.CreatedAt.Format(time.RFC3339)
							out["updated_at"] = it.UpdatedAt.Format(time.RFC3339)
							outItems = append(outItems, out)
						}
						return map[string]any{"items": outItems, "next_cursor": nextCursor}, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
		},
	})

	// Mutations (moved from main.go)
	rootMutation := graphql.NewObject(graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{
			"agreePolicies": &graphql.Field{
				Type: graphql.Boolean,
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return false, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						if err := DB.Model(&User{}).Where("id = ?", u.ID).Update("is_agreed_policies", true).Error; err != nil {
							return false, err
						}
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			"mcpCreate": &graphql.Field{
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
			},
			"mcpRevoke": &graphql.Field{
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
			},
			`adminUserAddRole`: &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{"user_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)}, "role": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}},
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
						uid := uint(p.Args["user_id"].(int))
						role := strings.ToLower(strings.TrimSpace(p.Args["role"].(string)))
						if role != "admin" && role != "manager" {
							return false, fmt.Errorf("invalid role")
						}
						var cnt int64
						_ = DB.Model(&RoleUser{}).Where("user_id = ? AND role = ?", uid, role).Count(&cnt).Error
						if cnt == 0 {
							_ = DB.Create(&RoleUser{UserID: uid, Role: role}).Error
						}
						_ = DB.Create(&Log{UserID: &u.ID, Action: fmt.Sprintf("admin.role.add:%d:%s", uid, role)}).Error
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			`adminUserRemoveRole`: &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{"user_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)}, "role": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}},
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
						uid := uint(p.Args["user_id"].(int))
						role := strings.ToLower(strings.TrimSpace(p.Args["role"].(string)))
						if err := DB.Where("user_id = ? AND role = ?", uid, role).Delete(&RoleUser{}).Error; err != nil {
							return false, fmt.Errorf("db")
						}
						_ = DB.Create(&Log{UserID: &u.ID, Action: fmt.Sprintf("admin.role.remove:%d:%s", uid, role)}).Error
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			`adminUserBlock`: &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{"user_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)}, "reason": &graphql.ArgumentConfig{Type: graphql.String}, "expires_at": &graphql.ArgumentConfig{Type: graphql.String}},
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
						uid := uint(p.Args["user_id"].(int))
						var expPtr *time.Time
						if s, ok := p.Args["expires_at"].(string); ok && strings.TrimSpace(s) != "" {
							if t, err := time.Parse(time.RFC3339, s); err == nil {
								expPtr = &t
							}
						}
						var reasonPtr *string
						if s, ok := p.Args["reason"].(string); ok && strings.TrimSpace(s) != "" {
							v := s
							reasonPtr = &v
						}
						rec := Block{UserID: uid, Reason: reasonPtr, ExpiresAt: expPtr}
						if err := DB.Create(&rec).Error; err != nil {
							return false, fmt.Errorf("db")
						}
						_ = DB.Create(&Log{UserID: &u.ID, Action: fmt.Sprintf("admin.block:%d", uid)}).Error
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			`adminUserUnblock`: &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{"user_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)}},
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
						uid := uint(p.Args["user_id"].(int))
						now := time.Now()
						if err := DB.Model(&Block{}).Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", uid, now).Updates(map[string]any{"expires_at": now}).Error; err != nil {
							return false, fmt.Errorf("db")
						}
						_ = DB.Create(&Log{UserID: &u.ID, Action: fmt.Sprintf("admin.unblock:%d", uid)}).Error
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			`adminCalendarUpsertRange`: &graphql.Field{
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
			},
			`adminDayshiftCycles`: &graphql.Field{
				Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{Name: "DayShiftCycle", Fields: graphql.Fields{"id": &graphql.Field{Type: graphql.Int}, "begin_date": &graphql.Field{Type: graphql.String}, "finish_date": &graphql.Field{Type: graphql.String}, "description": &graphql.Field{Type: graphql.String}}})),
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
			`adminDayshiftCreateCycle`: &graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{Name: "DayShiftCycleCreateResult", Fields: graphql.Fields{"id": &graphql.Field{Type: graphql.Int}}}),
				Args: graphql.FieldConfigArgument{"begin_date": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}, "finish_date": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}, "description": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}},
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
						bd, err1 := time.Parse("2006-01-02", p.Args["begin_date"].(string))
						fd, err2 := time.Parse("2006-01-02", p.Args["finish_date"].(string))
						if err1 != nil || err2 != nil || !bd.Before(fd) {
							return nil, fmt.Errorf("invalid dates")
						}
						desc := p.Args["description"].(string)
						rec := DayShiftCycle{BeginDate: bd, FinishDate: fd, Description: desc}
						if err := DB.Create(&rec).Error; err != nil {
							return nil, fmt.Errorf("db")
						}
						_ = DB.Create(&Log{UserID: &u.ID, Action: "admin.dayshift.cycle.create"}).Error
						return map[string]any{"id": rec.ID}, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			`adminDayshiftDeleteCycle`: &graphql.Field{
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
						isAdmin, _ := userHasRole(u.ID, "admin")
						isManager, _ := userHasRole(u.ID, "manager")
						if !(isAdmin || isManager) {
							return false, fmt.Errorf("forbidden")
						}
						if err := DB.Delete(&DayShiftCycle{}, p.Args["id"].(int)).Error; err != nil {
							return false, fmt.Errorf("db")
						}
						_ = DB.Create(&Log{UserID: &u.ID, Action: "admin.dayshift.cycle.delete"}).Error
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			`adminDayshiftPairs`: &graphql.Field{
				Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
					Name: "DayShiftPair",
					Fields: graphql.Fields{
						"id":       &graphql.Field{Type: graphql.Int},
						"cycle_id": &graphql.Field{Type: graphql.Int},
						"user1_id": &graphql.Field{Type: graphql.Int},
						"user2_id": &graphql.Field{Type: graphql.Int},
						"position": &graphql.Field{Type: graphql.Int},
					},
				})),
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
						var pairs []DayShiftCyclePair
						if err := DB.Where("cycle_id = ?", cid).Order("position ASC, id ASC").Find(&pairs).Error; err != nil {
							return nil, fmt.Errorf("db")
						}
						out := make([]map[string]any, 0, len(pairs))
						for _, it := range pairs {
							out = append(out, map[string]any{
								"id":       it.ID,
								"cycle_id": it.CycleID,
								"user1_id": it.User1ID,
								"user2_id": it.User2ID,
								"position": it.Position,
							})
						}
						return out, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			`adminDayshiftCreatePair`: &graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: "DayShiftPairCreateResult",
					Fields: graphql.Fields{
						"id": &graphql.Field{Type: graphql.Int},
					},
				}),
				Args: graphql.FieldConfigArgument{
					"cycle_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
					"user1_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
					"user2_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
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
						u1 := p.Args["user1_id"].(int)
						u2 := p.Args["user2_id"].(int)
						var max int64
						_ = DB.Model(&DayShiftCyclePair{}).Where("cycle_id = ?", cid).Select("COALESCE(MAX(position),0)").Scan(&max).Error
						pos := int(max) + 1
						rec := DayShiftCyclePair{CycleID: uint(cid), User1ID: uint(u1), User2ID: uint(u2), Position: pos}
						if err := DB.Create(&rec).Error; err != nil {
							return nil, fmt.Errorf("db")
						}
						_ = DB.Create(&Log{UserID: &u.ID, Action: "admin.dayshift.pair.create"}).Error
						return map[string]any{"id": rec.ID}, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			`adminDayshiftDeletePair`: &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{
					"cycle_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
					"pair_id":  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
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
						if err := DB.Delete(
							&DayShiftCyclePair{},
							"id = ? AND cycle_id = ?",
							p.Args["pair_id"].(int),
							p.Args["cycle_id"].(int),
						).Error; err != nil {
							return false, fmt.Errorf("db")
						}
						_ = DB.Create(&Log{UserID: &u.ID, Action: "admin.dayshift.pair.delete"}).Error
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			"apiKeyCreate": &graphql.Field{
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
			},
			"apiKeyUpdate": &graphql.Field{
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
			},
			"icalCreate": &graphql.Field{
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
			},
			"icalDelete": &graphql.Field{
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
			},
			"createTask": &graphql.Field{
				Type: taskType,
				Args: graphql.FieldConfigArgument{
					"title":               &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"subject_id":          &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
					"description":         &graphql.ArgumentConfig{Type: graphql.String},
					"type":                &graphql.ArgumentConfig{Type: graphql.String},
					"is_require_hand_in":  &graphql.ArgumentConfig{Type: graphql.Boolean},
					"self_check_required": &graphql.ArgumentConfig{Type: graphql.Boolean},
					"deadline":            &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}, // YYYY-MM-DD
					"deadline_since":      &graphql.ArgumentConfig{Type: graphql.String},                     // ENUM
					"deadline_until":      &graphql.ArgumentConfig{Type: graphql.String},                     // ENUM
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						if okp, err := checkPermission(u.ID, "tasks", "create", nil); err != nil {
							return nil, err
						} else if !okp {
							return nil, fmt.Errorf("forbidden")
						}
						title, _ := p.Args["title"].(string)
						subjectID, _ := p.Args["subject_id"].(int)
						desc, _ := p.Args["description"].(string)
						var descPtr *string
						if strings.TrimSpace(desc) != "" {
							descPtr = &desc
						}
						typ, _ := p.Args["type"].(string)
						typ = strings.TrimSpace(typ)
						irh, _ := p.Args["is_require_hand_in"].(bool)
						scr, _ := p.Args["self_check_required"].(bool)
						deadlineStr, _ := p.Args["deadline"].(string)
						deadlineStr = strings.TrimSpace(deadlineStr)
						if deadlineStr == "" {
							return nil, fmt.Errorf("deadline required")
						}
						dt, err := time.Parse("2006-01-02", deadlineStr)
						if err != nil {
							return nil, fmt.Errorf("invalid deadline")
						}
						dlSinceStr, _ := p.Args["deadline_since"].(string)
						dlUntilStr, _ := p.Args["deadline_until"].(string)
						since := strings.ToUpper(strings.TrimSpace(dlSinceStr))
						until := strings.ToUpper(strings.TrimSpace(dlUntilStr))
						// defaults
						if since == "" {
							since = "NONE"
						}
						if until == "" {
							until = "NONE"
						}
						// validate enums
						validSince := map[string]bool{"NONE": true, "P1": true, "P2": true, "P3": true, "P4": true, "P5": true, "P6": true, "P7": true, "P8": true}
						validUntil := map[string]bool{"NONE": true, "MORNING": true, "P1": true, "P2": true, "P3": true, "P4": true, "P5": true, "P6": true, "P7": true, "P8": true}
						if !validSince[since] {
							return nil, fmt.Errorf("invalid deadline_since")
						}
						if !validUntil[until] {
							return nil, fmt.Errorf("invalid deadline_until")
						}
						rec := Task{AddedUserID: u.ID, Title: title, SubjectID: uint(subjectID), Description: descPtr, Type: typ, IsRequireHandIn: irh, SelfCheckRequired: scr, Deadline: dateOnly(dt), DeadlineSince: since, DeadlineUntil: until}
						if err := DB.Create(&rec).Error; err != nil {
							return nil, fmt.Errorf("db")
						}
						// Fire async Teams notification email for Power Automate
						go sendTeamsNotifyEmailForTask(rec)
						out := map[string]any{
							"id":                  rec.ID,
							"title":               rec.Title,
							"subject_id":          rec.SubjectID,
							"description":         rec.Description,
							"type":                rec.Type,
							"is_require_hand_in":  rec.IsRequireHandIn,
							"self_check_required": rec.SelfCheckRequired,
							"deadline":            rec.Deadline.Format("2006-01-02"),
							"deadline_since":      rec.DeadlineSince,
							"deadline_until":      rec.DeadlineUntil,
							"created_at":          rec.CreatedAt.Format(time.RFC3339),
							"updated_at":          rec.UpdatedAt.Format(time.RFC3339),
						}
						return out, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			"updateTask": &graphql.Field{
				Type: taskType,
				Args: graphql.FieldConfigArgument{
					"id":                  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
					"title":               &graphql.ArgumentConfig{Type: graphql.String},
					"subject_id":          &graphql.ArgumentConfig{Type: graphql.Int},
					"description":         &graphql.ArgumentConfig{Type: graphql.String},
					"type":                &graphql.ArgumentConfig{Type: graphql.String},
					"is_require_hand_in":  &graphql.ArgumentConfig{Type: graphql.Boolean},
					"self_check_required": &graphql.ArgumentConfig{Type: graphql.Boolean},
					"deadline":            &graphql.ArgumentConfig{Type: graphql.String}, // YYYY-MM-DD
					"deadline_since":      &graphql.ArgumentConfig{Type: graphql.String},
					"deadline_until":      &graphql.ArgumentConfig{Type: graphql.String},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						id, _ := p.Args["id"].(int)
						var rec Task
						if err := DB.First(&rec, id).Error; err != nil {
							return nil, fmt.Errorf("not found")
						}
						if okp, err := checkPermission(u.ID, "tasks", "update", &rec.AddedUserID); err != nil {
							return nil, err
						} else if !okp {
							return nil, fmt.Errorf("forbidden")
						}
						if v, ok := p.Args["title"].(string); ok {
							rec.Title = v
						}
						if v, ok := p.Args["subject_id"].(int); ok {
							rec.SubjectID = uint(v)
						}
						if v, ok := p.Args["description"].(string); ok {
							if strings.TrimSpace(v) == "" {
								rec.Description = nil
							} else {
								rec.Description = &v
							}
						}
						if v, ok := p.Args["type"].(string); ok {
							rec.Type = strings.TrimSpace(v)
						}
						if v, ok := p.Args["is_require_hand_in"].(bool); ok {
							rec.IsRequireHandIn = v
						}
						if v, ok := p.Args["self_check_required"].(bool); ok {
							rec.SelfCheckRequired = v
						}
						if v, ok := p.Args["deadline"].(string); ok {
							vv := strings.TrimSpace(v)
							if vv != "" {
								if t, err := time.Parse("2006-01-02", vv); err == nil {
									rec.Deadline = dateOnly(t)
								} else {
									return nil, fmt.Errorf("invalid deadline")
								}
							}
						}
						if v, ok := p.Args["deadline_since"].(string); ok {
							vv := strings.ToUpper(strings.TrimSpace(v))
							if vv == "" {
								vv = "NONE"
							}
							if vv != "NONE" && vv != "P1" && vv != "P2" && vv != "P3" && vv != "P4" && vv != "P5" && vv != "P6" && vv != "P7" && vv != "P8" {
								return nil, fmt.Errorf("invalid deadline_since")
							}
							rec.DeadlineSince = vv
						}
						if v, ok := p.Args["deadline_until"].(string); ok {
							vv := strings.ToUpper(strings.TrimSpace(v))
							if vv == "" {
								vv = "NONE"
							}
							if vv != "NONE" && vv != "MORNING" && vv != "P1" && vv != "P2" && vv != "P3" && vv != "P4" && vv != "P5" && vv != "P6" && vv != "P7" && vv != "P8" {
								return nil, fmt.Errorf("invalid deadline_until")
							}
							rec.DeadlineUntil = vv
						}
						if err := DB.Save(&rec).Error; err != nil {
							return nil, fmt.Errorf("db")
						}
						out := map[string]any{
							"id":                  rec.ID,
							"title":               rec.Title,
							"subject_id":          rec.SubjectID,
							"description":         rec.Description,
							"type":                rec.Type,
							"is_require_hand_in":  rec.IsRequireHandIn,
							"self_check_required": rec.SelfCheckRequired,
							"deadline":            rec.Deadline.Format("2006-01-02"),
							"deadline_since":      rec.DeadlineSince,
							"deadline_until":      rec.DeadlineUntil,
							"created_at":          rec.CreatedAt.Format(time.RFC3339),
							"updated_at":          rec.UpdatedAt.Format(time.RFC3339),
						}
						return out, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			"deleteTask": &graphql.Field{
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
						var rec Task
						if err := DB.First(&rec, id).Error; err != nil {
							return false, fmt.Errorf("not found")
						}
						isAdmin, _ := userHasRole(u.ID, "admin")
						isManager, _ := userHasRole(u.ID, "manager")
						allowed := isAdmin || isManager
						if !allowed {
							if okp, err := checkPermission(u.ID, "tasks", "delete", &rec.AddedUserID); err != nil {
								return false, err
							} else {
								allowed = okp
							}
						}
						if !allowed {
							return false, fmt.Errorf("forbidden")
						}
						if err := DB.Delete(&Task{}, rec.ID).Error; err != nil {
							return false, fmt.Errorf("db")
						}
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			"updateMe": &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{
					"display_name": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					displayName, _ := p.Args["display_name"].(string)
					if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return false, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						dn := strings.TrimSpace(displayName)
						if len(dn) > 255 {
							dn = dn[:255]
						}
						if err := DB.Model(&User{}).Where("id = ?", u.ID).Update("display_name", dn).Error; err != nil {
							return false, err
						}
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			"setAuthSettings": &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{
					"email_login_enabled": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Boolean)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return false, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						val, _ := p.Args["email_login_enabled"].(bool)
						var fresh User
						if err := DB.First(&fresh, u.ID).Error; err != nil {
							return false, err
						}
						hasPassword := fresh.HashedPassword != nil && *fresh.HashedPassword != ""
						totpEnabled, _, _ := getActiveTOTP(fresh.ID)
						var pkCount int64
						_ = DB.Model(&Passkey{}).Where("user_id = ?", fresh.ID).Count(&pkCount).Error
						if !val && !(hasPassword || totpEnabled || pkCount > 0) {
							return false, fmt.Errorf("cannot disable without another auth factor")
						}
						if err := DB.Model(&User{}).Where("id = ?", fresh.ID).Update("email_code_allowed", val).Error; err != nil {
							return false, err
						}
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			"logout": &graphql.Field{
				Type: graphql.Boolean,
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return false, fmt.Errorf("unauthorized")
						}
						if tok, ok := c.Locals("token").(string); ok && tok != "" {
							_ = RDB.Del(c.Context(), "session:"+tok).Err()
							_ = RDB.SRem(c.Context(), fmt.Sprintf("user_sessions:%d", c.Locals("user").(*User).ID), tok).Err()
						}
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			"deleteMe": &graphql.Field{
				Type: graphql.Boolean,
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return false, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						if err := DB.Model(&User{}).Where("id = ?", u.ID).Update("is_deleted", true).Error; err != nil {
							return false, err
						}
						if tok, ok := c.Locals("token").(string); ok && tok != "" {
							_ = RDB.Del(c.Context(), "session:"+tok).Err()
							_ = RDB.SRem(c.Context(), fmt.Sprintf("user_sessions:%d", u.ID), tok).Err()
						}
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			"totpSetup": &graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: "TOTPSetup",
					Fields: graphql.Fields{
						"secret":  &graphql.Field{Type: graphql.String},
						"otpauth": &graphql.Field{Type: graphql.String},
					},
				}),
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						secret := generateBase32Secret()
						key := fmt.Sprintf("totp-setup:%d", u.ID)
						if err := RDB.Set(c.Context(), key, secret, 10*time.Minute).Err(); err != nil {
							return nil, fmt.Errorf("server")
						}
						issuer := "Taskly"
						label := fmt.Sprintf("%s:%s", issuer, u.Email)
						otpauth := fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s", urlEncode(label), secret, urlEncode(issuer))
						return map[string]any{"secret": secret, "otpauth": otpauth}, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			"totpActivate": &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{
					"code": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return false, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						code, _ := p.Args["code"].(string)
						key := fmt.Sprintf("totp-setup:%d", u.ID)
						secret, _ := RDB.Get(c.Context(), key).Result()
						if secret == "" {
							return false, fmt.Errorf("setup expired")
						}
						if !validateTOTP(secret, code) {
							return false, fmt.Errorf("invalid code")
						}
						enc, err := encryptString(secret)
						if err != nil {
							return false, fmt.Errorf("server")
						}
						var ts TOTPSecret
						if err := DB.Where("user_id = ?", u.ID).First(&ts).Error; err == nil {
							ts.Secret = enc
							ts.IsActive = true
							_ = DB.Save(&ts).Error
						} else {
							_ = DB.Create(&TOTPSecret{UserID: u.ID, Secret: enc, IsActive: true}).Error
						}
						_ = RDB.Del(c.Context(), key).Err()
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			"totpDisable": &graphql.Field{
				Type: graphql.Boolean,
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return false, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						var ts TOTPSecret
						if err := DB.Where("user_id = ?", u.ID).First(&ts).Error; err != nil {
							return false, fmt.Errorf("not enabled")
						}
						ts.IsActive = false
						if err := DB.Save(&ts).Error; err != nil {
							return false, fmt.Errorf("db")
						}
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			"recoveryGenerate": &graphql.Field{
				Type: graphql.NewList(graphql.String),
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return nil, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						_ = DB.Where("user_id = ? AND is_used = 0", u.ID).Delete(&RecoveryCode{}).Error
						codes := make([]string, 0, 10)
						for i := 0; i < 10; i++ {
							plain := generateRecoveryCode()
							hashed := sha256Hex([]byte(plain))
							_ = DB.Create(&RecoveryCode{UserID: u.ID, HashedCode: hashed}).Error
							codes = append(codes, plain)
						}
						return codes, nil
					}
					return nil, fmt.Errorf("context missing")
				},
			},
			"webauthnRegisterBegin": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return "", fmt.Errorf("unauthorized")
						}
						if WAuth == nil {
							return "", fmt.Errorf("webauthn not configured")
						}
						u := c.Locals("user").(*User)
						creation, sd, err := WAuth.BeginRegistration(u)
						if err != nil {
							return "", fmt.Errorf("begin failed")
						}
						bsd, _ := json.Marshal(sd)
						_ = RDB.Set(c.Context(), fmt.Sprintf("webauthn:reg:%d", u.ID), string(bsd), 10*time.Minute).Err()
						b, _ := json.Marshal(creation)
						return string(b), nil
					}
					return "", fmt.Errorf("context missing")
				},
			},
			"webauthnRegisterFinish": &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{
					"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return false, fmt.Errorf("unauthorized")
						}
						if WAuth == nil {
							return false, fmt.Errorf("webauthn not configured")
						}
						u := c.Locals("user").(*User)
						raw, _ := RDB.Get(c.Context(), fmt.Sprintf("webauthn:reg:%d", u.ID)).Result()
						if raw == "" {
							return false, fmt.Errorf("session expired")
						}
						var sd webauthn.SessionData
						_ = json.Unmarshal([]byte(raw), &sd)
						payload, _ := p.Args["input"].(string)
						req, _ := http.NewRequestWithContext(c.Context(), "POST", "", strings.NewReader(payload))
						req.Header.Set("Content-Type", "application/json")
						cred, err := WAuth.FinishRegistration(u, sd, req)
						if err != nil {
							return false, fmt.Errorf("finish failed")
						}
						pub := base64.RawStdEncoding.EncodeToString(cred.PublicKey)
						cid := base64.RawStdEncoding.EncodeToString(cred.ID)
						devName := "Passkey"
						rec := Passkey{UserID: u.ID, CredentialID: cid, PublicKey: pub, SignCount: int64(cred.Authenticator.SignCount), DeviceName: &devName}
						if err := DB.Create(&rec).Error; err != nil {
							return false, fmt.Errorf("db")
						}
						_ = RDB.Del(c.Context(), fmt.Sprintf("webauthn:reg:%d", u.ID)).Err()
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
			"setPassword": &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{
					"current": &graphql.ArgumentConfig{Type: graphql.String},
					"next":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
						if !userFromSession(c) {
							return false, fmt.Errorf("unauthorized")
						}
						u := c.Locals("user").(*User)
						curr, _ := p.Args["current"].(string)
						next, _ := p.Args["next"].(string)
						if len(strings.TrimSpace(next)) < 8 {
							return false, fmt.Errorf("password too short")
						}
						var fresh User
						if err := DB.First(&fresh, u.ID).Error; err != nil {
							return false, fmt.Errorf("db")
						}
						if fresh.HashedPassword != nil && *fresh.HashedPassword != "" {
							if err := bcrypt.CompareHashAndPassword([]byte(*fresh.HashedPassword), []byte(curr)); err != nil {
								return false, fmt.Errorf("current password invalid")
							}
						}
						hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
						if err != nil {
							return false, fmt.Errorf("hash")
						}
						hs := string(hash)
						if err := DB.Model(&User{}).Where("id = ?", u.ID).Update("hashed_password", &hs).Error; err != nil {
							return false, fmt.Errorf("db")
						}
						curTok, _ := c.Locals("token").(string)
						revokeOtherSessions(c, u.ID, curTok)
						return true, nil
					}
					return false, fmt.Errorf("context missing")
				},
			},
		},
	})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: rootQuery, Mutation: rootMutation})
	if err != nil {
		return
	}
	app.Post("/graphql", func(c *fiber.Ctx) error {
		var body struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = c.BodyParser(&body)
		params := graphql.Params{
			Schema:         schema,
			RequestString:  body.Query,
			VariableValues: body.Variables,
			Context:        context.WithValue(context.Background(), ctxKeyFiberCtx, c),
		}
		result := graphql.Do(params)
		if len(result.Errors) > 0 {
			return c.Status(400).JSON(result)
		}
		return c.JSON(result)
	})
}
