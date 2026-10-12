package gqlQuery

import (
	"fmt"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	gqlH "github.com/asari194617/taskly/apps/server/internal/handler/graphql/shared"
	"github.com/asari194617/taskly/apps/server/internal/legal"
	"github.com/asari194617/taskly/apps/server/internal/taskimage"
	"github.com/asari194617/taskly/apps/server/internal/tasklist"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	graphql "github.com/graphql-go/graphql"
)

// queryFieldsTasks returns GraphQL fields related to tasks listing with cursor.
func (h *Handler) QueryFieldsTasks(t *gqlH.GqlTypes) graphql.Fields {
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
				h := h.withContext(p.Context)
				if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
					if !utils.UserFromSession(c, h.db, h.rdb) {
						return nil, fmt.Errorf("unauthorized")
					}
					u := c.Locals("user").(*domain.User)
					if okp, err := utils.CheckPermission(u.ID, "tasks", "read", nil, h.db); err != nil {
						return nil, err
					} else if !okp {
						return nil, fmt.Errorf("forbidden")
					}
					overdue, _ := p.Args["overdue"].(bool)
					search, _ := p.Args["q"].(string)
					limit, _ := p.Args["limit"].(int)
					cursorDeadline, _ := p.Args["cursor_deadline"].(string)
					cursorID, _ := p.Args["cursor_id"].(string)
					items, nextCursor, err := tasklist.List(h.db, u.ID, tasklist.Options{
						Limit: limit, Overdue: overdue, Search: search,
						CursorDeadline: cursorDeadline, CursorID: cursorID,
					})
					if err != nil {
						return nil, err
					}
					// serialize
					// Preload user's completion map for fetched items
					completed := map[uint]bool{}
					if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
						u := c.Locals("user").(*domain.User)
						ids := make([]uint, 0, len(items))
						for _, it := range items {
							ids = append(ids, it.ID)
						}
						if len(ids) > 0 {
							// Build IN clause safely
							// Use GORM's Where with slice
							rows := []struct{ TaskID uint }{}
							if err := h.db.Raw("SELECT task_id AS task_id FROM tasks_users_completions WHERE user_id = ? AND task_id IN ?", u.ID, ids).Scan(&rows).Error; err == nil {
								for _, r := range rows {
									completed[r.TaskID] = true
								}
							}
						}
					}
					outItems := make([]map[string]any, 0, len(items))
					images, _ := taskimage.NewFromEnv()
					taskIDs := make([]uint, 0, len(items))
					for _, item := range items {
						taskIDs = append(taskIDs, item.ID)
					}
					imageKeysByTask, _ := taskimage.KeysForTasks(h.db, taskIDs)
					for _, it := range items {
						imageKeys := imageKeysByTask[it.ID]
						if len(imageKeys) == 0 && it.ImageKey != nil {
							imageKeys = []string{*it.ImageKey}
						}
						out := map[string]any{
							"id":                     it.ID,
							"title":                  it.Title,
							"subject_id":             it.SubjectID,
							"description":            it.Description,
							"image_url":              nil,
							"image_keys":             imageKeys,
							"image_urls":             []string{},
							"type":                   it.Type,
							"is_require_hand_in":     it.IsRequireHandIn,
							"is_self_check_required": it.IsSelfCheckRequired,
							"is_hidden":              it.IsHidden,
						}
						out["deadline"] = it.Deadline.Format("2006-01-02")
						out["deadline_since"] = it.DeadlineSince
						out["deadline_until"] = it.DeadlineUntil
						out["is_completed"] = completed[it.ID]
						out["created_at"] = it.CreatedAt.Format(time.RFC3339)
						out["updated_at"] = it.UpdatedAt.Format(time.RFC3339)
						if images != nil {
							imageURLs := make([]string, 0, len(imageKeys))
							allSigned := true
							for _, key := range imageKeys {
								if url, err := images.PresignView(p.Context, key); err == nil {
									imageURLs = append(imageURLs, url)
								} else {
									allSigned = false
									break
								}
							}
							if !allSigned {
								imageURLs = []string{}
							}
							out["image_urls"] = imageURLs
							if len(imageURLs) > 0 {
								out["image_url"] = imageURLs[0]
							}
						}
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
func (h *Handler) QueryFieldsMisc(t *gqlH.GqlTypes) graphql.Fields {
	subjectType := t.SubjectType
	meType := t.MeType
	authSettingsType := t.AuthSettingsType
	passkeyType := t.PasskeyType

	fields := graphql.Fields{
		"subjects": &graphql.Field{
			Type: graphql.NewList(subjectType),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				h := h.withContext(p.Context)
				if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
					if !utils.UserFromSession(c, h.db, h.rdb) { // session only
						return nil, fmt.Errorf("unauthorized")
					}
					var subs []domain.Subject
					now := time.Now()
					if err := h.db.Where("expires_at IS NULL OR expires_at > ?", now).Order("id ASC").Find(&subs).Error; err != nil {
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
				h := h.withContext(p.Context)
				if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
					if !utils.UserFromSession(c, h.db, h.rdb) {
						return nil, fmt.Errorf("unauthorized")
					}
					u := c.Locals("user").(*domain.User)
					var roles []domain.RoleUser
					_ = h.db.Where("user_id = ?", u.ID).Find(&roles).Error
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
					agreed, err := legal.Current(h.db, u.ID)
					if err != nil {
						return nil, err
					}
					notice, err := legal.Notice(h.db, u.ID)
					if err != nil {
						return nil, err
					}
					return map[string]any{
						"id":                         u.ID,
						"email":                      u.Email,
						"display_name":               u.DisplayName,
						"is_admin":                   isAdmin,
						"is_manager":                 isManager,
						"is_registered":              u.IsRegistered,
						"is_agreed_policies":         agreed,
						"policy_notice_effective_at": notice,
					}, nil
				}
				return nil, fmt.Errorf("context missing")
			},
		},
		"authSettings": &graphql.Field{
			Type: authSettingsType,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				h := h.withContext(p.Context)
				if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
					if !utils.UserFromSession(c, h.db, h.rdb) {
						return nil, fmt.Errorf("unauthorized")
					}
					u := c.Locals("user").(*domain.User)
					var fresh domain.User
					if err := h.db.First(&fresh, u.ID).Error; err != nil {
						return nil, err
					}
					hasPassword := fresh.HashedPassword != nil && *fresh.HashedPassword != ""
					totpEnabled, _, _ := utils.GetActiveTOTP(fresh.ID, h.db)
					// counts for UI logic
					var pkCount int64
					_ = h.db.Model(&domain.Passkey{}).Where("user_id = ?", fresh.ID).Count(&pkCount).Error
					var rcCount int64
					_ = h.db.Model(&domain.RecoveryCode{}).Where("user_id = ? AND is_used = 0", fresh.ID).Count(&rcCount).Error
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
				h := h.withContext(p.Context)
				if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
					if !utils.UserFromSession(c, h.db, h.rdb) {
						return nil, fmt.Errorf("unauthorized")
					}
					u := c.Locals("user").(*domain.User)
					var items []domain.Passkey
					if err := h.db.Where("user_id = ?", u.ID).Order("id DESC").Find(&items).Error; err != nil {
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
	for k, v := range h.queryFieldsAdminSubjects(t) {
		fields[k] = v
	}
	return fields
}
