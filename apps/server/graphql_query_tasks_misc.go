package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	graphql "github.com/graphql-go/graphql"
)

// queryFieldsTasks returns GraphQL fields related to tasks listing with cursor.
func queryFieldsTasks(t *gqlTypes) graphql.Fields {
	taskType := t.TaskType
	return graphql.Fields{
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
	}
}

// queryFieldsMisc returns GraphQL fields for subjects, me, authSettings.
func queryFieldsMisc(t *gqlTypes) graphql.Fields {
	subjectType := t.SubjectType
	meType := t.MeType
	authSettingsType := t.AuthSettingsType
	passkeyType := t.PasskeyType

	fields := graphql.Fields{
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
						row := map[string]any{"id": s.ID, "name_ja": s.NameJa, "name_en": s.NameEn}
						if s.ExpiresAt != nil {
							row["expires_at"] = s.ExpiresAt.Format(time.RFC3339)
						}
						row["created_at"] = s.CreatedAt.Format(time.RFC3339)
						out = append(out, row)
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
					// counts for UI logic
					var pkCount int64
					_ = DB.Model(&Passkey{}).Where("user_id = ?", fresh.ID).Count(&pkCount).Error
					var rcCount int64
					_ = DB.Model(&RecoveryCode{}).Where("user_id = ? AND is_used = 0", fresh.ID).Count(&rcCount).Error
					return map[string]any{
						"email_login_enabled":  fresh.EmailCodeAllowed,
						"has_password":         hasPassword,
						"totp_enabled":         totpEnabled,
						"passkey_count":        int(pkCount),
						"recovery_codes_count": int(rcCount),
					}, nil
				}
				return nil, fmt.Errorf("context missing")
			},
		},
		// List my passkeys for management UI
		"myPasskeys": &graphql.Field{
			Type: graphql.NewList(passkeyType),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
					if !userFromSession(c) {
						return nil, fmt.Errorf("unauthorized")
					}
					u := c.Locals("user").(*User)
					var items []Passkey
					if err := DB.Where("user_id = ?", u.ID).Order("id DESC").Find(&items).Error; err != nil {
						return nil, err
					}
					out := make([]map[string]any, 0, len(items))
					for _, it := range items {
						out = append(out, map[string]any{
							"id": it.ID,
							"device_name": func() any {
								if it.DeviceName != nil {
									return *it.DeviceName
								}
								return "Passkey"
							}(),
							"created_at": it.CreatedAt.Format(time.RFC3339),
						})
					}
					return out, nil
				}
				return nil, fmt.Errorf("context missing")
			},
		},
	}

	// Merge in admin subjects query
	for k, v := range queryFieldsAdminSubjects(t) {
		fields[k] = v
	}
	return fields
}
