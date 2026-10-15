package gqlMutation

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/authsync"
	"github.com/asari194617/taskly/apps/server/internal/domain"
	gqlH "github.com/asari194617/taskly/apps/server/internal/handler/graphql/shared"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	graphql "github.com/graphql-go/graphql"
	"gorm.io/gorm"
)

// mutationFieldsAdminUsers groups admin user management mutations.
func (h *Handler) mutationFieldsAdminUsers() graphql.Fields {
	fields := graphql.Fields{}

	fields["adminUserUpdate"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"user_id":             &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
			"email":               &graphql.ArgumentConfig{Type: graphql.String},
			"display_name":        &graphql.ArgumentConfig{Type: graphql.String},
			"notification_lang":   &graphql.ArgumentConfig{Type: graphql.String},
			"email_login_enabled": &graphql.ArgumentConfig{Type: graphql.Boolean},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				u, err := h.requireAdminActor(c, true)
				if err != nil {
					return false, err
				}
				id := p.Args["user_id"].(int)
				updates := map[string]any{}
				if v, ok := p.Args["email"].(string); ok {
					updates["email"] = strings.TrimSpace(v)
				}
				if v, ok := p.Args["display_name"].(string); ok {
					updates["display_name"] = v
				}
				if v, ok := p.Args["notification_lang"].(string); ok {
					updates["notification_lang"] = strings.ToLower(strings.TrimSpace(v))
				}
				if v, ok := p.Args["email_login_enabled"].(bool); ok {
					updates["email_code_allowed"] = v
				}
				if len(updates) == 0 {
					return true, nil
				}
				if err := h.updateAdminUser(p.Context, uint(id), updates); err != nil {
					if errors.Is(err, authsync.ErrUserSync) || errors.Is(err, errUserProjectionConflict) {
						return false, fmt.Errorf("auth user synchronization pending")
					}
					return false, fmt.Errorf("db")
				}
				h.recordAuditLog(&domain.Log{UserID: &u.ID, Action: fmt.Sprintf("admin.user.update:%d", id)})
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["adminUserCreate"] = &graphql.Field{
		Type: graphql.NewObject(graphql.ObjectConfig{Name: "AdminUserCreateResult", Fields: graphql.Fields{"id": &graphql.Field{Type: graphql.Int}}}),
		Args: graphql.FieldConfigArgument{
			"email":             &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			"display_name":      &graphql.ArgumentConfig{Type: graphql.String},
			"notification_lang": &graphql.ArgumentConfig{Type: graphql.String},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				u, err := h.requireAdminActor(c, true)
				if err != nil {
					return nil, err
				}
				email := strings.TrimSpace(p.Args["email"].(string))
				if email == "" {
					return nil, fmt.Errorf("invalid email")
				}
				dn, _ := p.Args["display_name"].(string)
				lang, _ := p.Args["notification_lang"].(string)
				lang = strings.ToLower(strings.TrimSpace(lang))
				if lang == "" {
					lang = "ja"
				}
				rec, _, err := h.createAdminUser(p.Context, email, dn, lang)
				if err != nil {
					if errors.Is(err, authsync.ErrUserSync) || errors.Is(err, errUserProjectionConflict) {
						return nil, fmt.Errorf("auth user synchronization pending")
					}
					// 重複メールなどの一般的なh.dbエラーを判別して、人間が読める文言を返す
					msg := err.Error()
					if strings.Contains(strings.ToLower(msg), "duplicate") && strings.Contains(strings.ToLower(msg), "email") {
						return nil, fmt.Errorf("メールアドレスは既に存在します")
					}
					return nil, fmt.Errorf("db: %s", msg)
				}
				h.recordAuditLog(&domain.Log{UserID: &u.ID, Action: fmt.Sprintf("admin.user.create:%d", rec.ID)})
				return map[string]any{"id": rec.ID}, nil
			}
			return nil, fmt.Errorf("context missing")
		},
	}

	fields["adminUserBulkCreate"] = &graphql.Field{
		Type: graphql.NewObject(graphql.ObjectConfig{Name: "AdminUserBulkCreateResult", Fields: graphql.Fields{"created": &graphql.Field{Type: graphql.Int}, "errors": &graphql.Field{Type: graphql.NewList(graphql.String)}}}),
		Args: graphql.FieldConfigArgument{
			"csv": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				u, err := h.requireAdminActor(c, true)
				if err != nil {
					return nil, err
				}
				raw := p.Args["csv"].(string)
				lines := strings.Split(raw, "\n")
				created := 0
				var errs []string
				for i, ln := range lines {
					s := strings.TrimSpace(ln)
					if s == "" {
						continue
					}
					parts := strings.Split(s, ",")
					if len(parts) < 1 {
						errs = append(errs, fmt.Sprintf("line %d: empty", i+1))
						continue
					}
					email := strings.TrimSpace(parts[0])
					if email == "" {
						errs = append(errs, fmt.Sprintf("line %d: invalid email", i+1))
						continue
					}
					dn := ""
					if len(parts) > 1 {
						dn = strings.TrimSpace(parts[1])
					}
					lang := "ja"
					if len(parts) > 2 {
						lang = strings.ToLower(strings.TrimSpace(parts[2]))
					}
					_, wasCreated, err := h.createAdminUser(p.Context, email, dn, lang)
					if err != nil {
						if errors.Is(err, authsync.ErrUserSync) || errors.Is(err, errUserProjectionConflict) {
							errs = append(errs, fmt.Sprintf("line %d: auth user synchronization pending", i+1))
							continue
						}
						msg := err.Error()
						if strings.Contains(strings.ToLower(msg), "duplicate") && strings.Contains(strings.ToLower(msg), "email") {
							errs = append(errs, fmt.Sprintf("line %d: duplicate email", i+1))
						} else {
							errs = append(errs, fmt.Sprintf("line %d: db: %s", i+1, msg))
						}
						continue
					}
					if wasCreated {
						created++
					} else {
						errs = append(errs, fmt.Sprintf("line %d: duplicate email", i+1))
					}
				}
				h.recordAuditLog(&domain.Log{UserID: &u.ID, Action: fmt.Sprintf("admin.user.bulk_create:%d", created)})
				return map[string]any{"created": created, "errors": errs}, nil
			}
			return nil, fmt.Errorf("context missing")
		},
	}

	fields["adminUserSetCustomRole"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"user_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
			"role":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			"enabled": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Boolean)},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				u, err := h.requireAdminActor(c, true)
				if err != nil {
					return false, err
				}
				uid := uint(p.Args["user_id"].(int))
				role := strings.ToLower(strings.TrimSpace(p.Args["role"].(string)))
				enabled := p.Args["enabled"].(bool)
				if role == "" {
					return false, fmt.Errorf("invalid role")
				}
				if enabled {
					var cnt int64
					if err := h.db.Model(&domain.RoleUser{}).Where("user_id = ? AND role = ?", uid, role).Count(&cnt).Error; err != nil {
						return false, fmt.Errorf("db: %w", err)
					}
					if cnt == 0 {
						if err := h.db.Create(&domain.RoleUser{UserID: uid, Role: role}).Error; err != nil {
							return false, fmt.Errorf("db: %w", err)
						}
					}
				} else {
					if err := h.db.Where("user_id = ? AND role = ?", uid, role).Delete(&domain.RoleUser{}).Error; err != nil {
						return false, fmt.Errorf("db: %w", err)
					}
				}
				h.recordAuditLog(&domain.Log{UserID: &u.ID, Action: fmt.Sprintf("admin.role.setcustom:%d:%s:%t", uid, role, enabled)})
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["adminUserRestore"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"user_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				actor, err := h.requireAdminActor(c, true)
				if err != nil {
					return false, err
				}
				id := p.Args["user_id"].(int)
				if id <= 0 {
					return false, fmt.Errorf("invalid user")
				}
				if err := utils.RestoreUser(p.Context, uint(id), h.db, h.rdb); err != nil {
					if errors.Is(err, authsync.ErrLifecycleSync) {
						return false, fmt.Errorf("auth service synchronization pending")
					}
					if errors.Is(err, gorm.ErrRecordNotFound) {
						return false, fmt.Errorf("user not found")
					}
					return false, fmt.Errorf("db")
				}
				h.recordAuditLog(&domain.Log{UserID: &actor.ID, Action: fmt.Sprintf("admin.user.restore:%d", id)})
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["adminUserDelete"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"user_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
			"hard":    &graphql.ArgumentConfig{Type: graphql.Boolean}, // true: hard delete; false: set is_deleted
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				u, err := h.requireAdminActor(c, true)
				if err != nil {
					return false, err
				}
				id := p.Args["user_id"].(int)
				hard, _ := p.Args["hard"].(bool)
				if hard {
					if authsync.LifecycleEnabled() {
						var target domain.User
						if err := h.db.Select("id", "is_deleted", "deleted_at").First(&target, id).Error; err != nil {
							return false, fmt.Errorf("db")
						}
						if !target.IsDeleted {
							if err := utils.SoftDeleteUser(p.Context, uint(id), h.db, h.rdb); err != nil {
								return false, fmt.Errorf("auth service synchronization pending")
							}
						}
						err := authsync.Erase(p.Context, uint(id))
						if errors.Is(err, authsync.ErrLifecycleNotDeleted) {
							deletedAt := time.Now().UTC().Truncate(time.Second)
							if target.DeletedAt != nil {
								deletedAt = target.DeletedAt.UTC()
							}
							if err = authsync.SyncDeleted(p.Context, uint(id), deletedAt); err == nil {
								err = authsync.Erase(p.Context, uint(id))
							}
						}
						if err != nil {
							return false, fmt.Errorf("auth service synchronization pending")
						}
					}
					if err := h.db.Delete(&domain.User{}, id).Error; err != nil {
						return false, fmt.Errorf("db")
					}
				} else {
					if err := utils.SoftDeleteUser(p.Context, uint(id), h.db, h.rdb); err != nil {
						if errors.Is(err, authsync.ErrLifecycleSync) {
							return false, fmt.Errorf("auth service synchronization pending")
						}
						return false, fmt.Errorf("db")
					}
				}
				h.recordAuditLog(&domain.Log{UserID: &u.ID, Action: fmt.Sprintf("admin.user.delete:%d:%t", id, hard)})
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["adminUserAddRole"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{"user_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)}, "role": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				u, err := h.requireAdminActor(c, true)
				if err != nil {
					return false, err
				}
				uid := uint(p.Args["user_id"].(int))
				role := strings.ToLower(strings.TrimSpace(p.Args["role"].(string)))
				if role != "admin" && role != "manager" {
					return false, fmt.Errorf("invalid role")
				}
				var cnt int64
				if err := h.db.Model(&domain.RoleUser{}).Where("user_id = ? AND role = ?", uid, role).Count(&cnt).Error; err != nil {
					return false, fmt.Errorf("db: %w", err)
				}
				if cnt == 0 {
					if err := h.db.Create(&domain.RoleUser{UserID: uid, Role: role}).Error; err != nil {
						return false, fmt.Errorf("db: %w", err)
					}
				}
				h.recordAuditLog(&domain.Log{UserID: &u.ID, Action: fmt.Sprintf("admin.role.add:%d:%s", uid, role)})
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["adminUserRemoveRole"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{"user_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)}, "role": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				u, err := h.requireAdminActor(c, true)
				if err != nil {
					return false, err
				}
				uid := uint(p.Args["user_id"].(int))
				role := strings.ToLower(strings.TrimSpace(p.Args["role"].(string)))
				if err := h.db.Where("user_id = ? AND role = ?", uid, role).Delete(&domain.RoleUser{}).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				h.recordAuditLog(&domain.Log{UserID: &u.ID, Action: fmt.Sprintf("admin.role.remove:%d:%s", uid, role)})
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["adminUserBlock"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{"user_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)}, "reason": &graphql.ArgumentConfig{Type: graphql.String}, "expires_at": &graphql.ArgumentConfig{Type: graphql.String}},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				u, err := h.requireAdminActor(c, true)
				if err != nil {
					return false, err
				}
				uid := uint(p.Args["user_id"].(int))
				var expPtr *time.Time
				if s, ok := p.Args["expires_at"].(string); ok && strings.TrimSpace(s) != "" {
					t, err := time.Parse(time.RFC3339, s)
					if err != nil {
						return false, fmt.Errorf("invalid expires_at")
					}
					expPtr = &t
				}
				var reasonPtr *string
				if s, ok := p.Args["reason"].(string); ok && strings.TrimSpace(s) != "" {
					v := s
					reasonPtr = &v
				}
				rec := domain.Block{UserID: uid, Reason: reasonPtr, ExpiresAt: expPtr}
				if err := h.db.Create(&rec).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				h.recordAuditLog(&domain.Log{UserID: &u.ID, Action: fmt.Sprintf("admin.block:%d", uid)})
				if err := authsync.SyncBlock(p.Context, uid, true, reasonPtr, expPtr); err != nil {
					return false, fmt.Errorf("authentication block synchronization pending")
				}
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["adminUserUnblock"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{"user_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)}},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				u, err := h.requireAdminActor(c, true)
				if err != nil {
					return false, err
				}
				uid := uint(p.Args["user_id"].(int))
				now := time.Now()
				if err := h.db.Model(&domain.Block{}).Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", uid, now).Updates(map[string]any{"expires_at": now}).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				h.recordAuditLog(&domain.Log{UserID: &u.ID, Action: fmt.Sprintf("admin.unblock:%d", uid)})
				if err := authsync.SyncBlock(p.Context, uid, false, nil, nil); err != nil {
					return false, fmt.Errorf("authentication block synchronization pending")
				}
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	// Set per-type preference (TASK or DAYSHIFT)
	fields["adminUserSetNotificationTypePref"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"user_id":           &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
			"notification_type": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			"enabled":           &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Boolean)},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				u, err := h.requireAdminActor(c, true)
				if err != nil {
					return false, err
				}
				uid := uint(p.Args["user_id"].(int))
				tp := strings.ToUpper(strings.TrimSpace(p.Args["notification_type"].(string)))
				if tp != "TASK" && tp != "DAYSHIFT" {
					return false, fmt.Errorf("invalid type")
				}
				en := p.Args["enabled"].(bool)
				var rec domain.NotificationTypePref
				if err := h.db.Where("user_id = ? AND notification_type = ?", uid, tp).First(&rec).Error; errors.Is(err, gorm.ErrRecordNotFound) {
					rec = domain.NotificationTypePref{UserID: uid, NotificationType: tp, IsEnabled: en}
					if err := h.db.Create(&rec).Error; err != nil {
						return false, fmt.Errorf("db")
					}
				} else if err != nil {
					return false, fmt.Errorf("db: %w", err)
				} else {
					if err := h.db.Model(&domain.NotificationTypePref{}).Where("id = ?", rec.ID).Update("is_enabled", en).Error; err != nil {
						return false, fmt.Errorf("db")
					}
				}
				h.recordAuditLog(&domain.Log{UserID: &u.ID, Action: fmt.Sprintf("admin.user.set_type_pref:%d:%s:%t", uid, tp, en)})
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	// Create a device (for IOS/ANDROID or named WEBPUSH entries if needed)
	fields["adminUserCreateDevice"] = &graphql.Field{
		Type: graphql.NewObject(graphql.ObjectConfig{Name: "AdminUserCreateDeviceResult", Fields: graphql.Fields{"id": &graphql.Field{Type: graphql.Int}}}),
		Args: graphql.FieldConfigArgument{
			"user_id":  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
			"platform": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			"name":     &graphql.ArgumentConfig{Type: graphql.String},
			"payload":  &graphql.ArgumentConfig{Type: graphql.String},
			"enabled":  &graphql.ArgumentConfig{Type: graphql.Boolean},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				u, err := h.requireAdminActor(c, true)
				if err != nil {
					return nil, err
				}
				uid := uint(p.Args["user_id"].(int))
				platform := strings.ToUpper(strings.TrimSpace(p.Args["platform"].(string)))
				if platform == "" {
					return nil, fmt.Errorf("invalid platform")
				}
				name, _ := p.Args["name"].(string)
				var namePtr *string
				if strings.TrimSpace(name) != "" {
					v := name
					namePtr = &v
				}
				payload, _ := p.Args["payload"].(string)
				var payloadPtr *string
				if strings.TrimSpace(payload) != "" {
					v := payload
					payloadPtr = &v
				}
				enabled := true
				if v, ok := p.Args["enabled"].(bool); ok {
					enabled = v
				}
				rec := domain.NotificationDevice{UserID: uid, Platform: platform, Name: namePtr, Payload: payloadPtr, IsEnabled: enabled}
				if err := h.db.Create(&rec).Error; err != nil {
					return nil, fmt.Errorf("db")
				}
				h.recordAuditLog(&domain.Log{UserID: &u.ID, Action: fmt.Sprintf("admin.device.create:%d:%s", uid, platform)})
				return map[string]any{"id": int(rec.ID)}, nil
			}
			return nil, fmt.Errorf("context missing")
		},
	}

	// Update a device (name/payload/is_enabled)
	fields["adminUserUpdateDevice"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"device_id":  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
			"name":       &graphql.ArgumentConfig{Type: graphql.String},
			"payload":    &graphql.ArgumentConfig{Type: graphql.String},
			"is_enabled": &graphql.ArgumentConfig{Type: graphql.Boolean},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				u, err := h.requireAdminActor(c, true)
				if err != nil {
					return false, err
				}
				id := uint(p.Args["device_id"].(int))
				updates := map[string]any{}
				if v, ok := p.Args["name"].(string); ok {
					if strings.TrimSpace(v) == "" {
						updates["name"] = nil
					} else {
						updates["name"] = v
					}
				}
				if v, ok := p.Args["payload"].(string); ok {
					if strings.TrimSpace(v) == "" {
						updates["payload"] = nil
					} else {
						updates["payload"] = v
					}
				}
				if v, ok := p.Args["is_enabled"].(bool); ok {
					updates["is_enabled"] = v
				}
				if len(updates) == 0 {
					return true, nil
				}
				if err := h.db.Model(&domain.NotificationDevice{}).Where("id = ?", id).Updates(updates).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				h.recordAuditLog(&domain.Log{UserID: &u.ID, Action: fmt.Sprintf("admin.device.update:%d", id)})
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	// Delete a device
	fields["adminUserDeleteDevice"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"device_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				u, err := h.requireAdminActor(c, true)
				if err != nil {
					return false, err
				}
				id := uint(p.Args["device_id"].(int))
				if err := h.db.Delete(&domain.NotificationDevice{}, id).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				h.recordAuditLog(&domain.Log{UserID: &u.ID, Action: fmt.Sprintf("admin.device.delete:%d", id)})
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["adminUserSetNotificationSettings"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"user_id":         &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
			"email_enabled":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Boolean)},
			"webpush_enabled": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Boolean)},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				u, err := h.requireAdminActor(c, true)
				if err != nil {
					return false, err
				}
				uid := uint(p.Args["user_id"].(int))
				em := p.Args["email_enabled"].(bool)
				wp := p.Args["webpush_enabled"].(bool)
				if err := h.db.Transaction(func(tx *gorm.DB) error {
					// Upsert EMAIL device
					var emDev domain.NotificationDevice
					if err := tx.Where("user_id = ? AND platform = ?", uid, "EMAIL").First(&emDev).Error; err == nil {
						if err := tx.Model(&domain.NotificationDevice{}).Where("id = ?", emDev.ID).Update("is_enabled", em).Error; err != nil {
							return err
						}
					} else if errors.Is(err, gorm.ErrRecordNotFound) && em {
						// create record only when enabling
						rec := domain.NotificationDevice{UserID: uid, Platform: "EMAIL", IsEnabled: true}
						if err := tx.Create(&rec).Error; err != nil {
							return err
						}
					} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
						return err
					}
					// Upsert WEBPUSH device
					var wpDev domain.NotificationDevice
					if err := tx.Where("user_id = ? AND platform = ?", uid, "WEBPUSH").First(&wpDev).Error; err == nil {
						if err := tx.Model(&domain.NotificationDevice{}).Where("id = ?", wpDev.ID).Update("is_enabled", wp).Error; err != nil {
							return err
						}
					} else if errors.Is(err, gorm.ErrRecordNotFound) && wp {
						// create record only when enabling
						rec := domain.NotificationDevice{UserID: uid, Platform: "WEBPUSH", IsEnabled: true}
						if err := tx.Create(&rec).Error; err != nil {
							return err
						}
					} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
						return err
					}
					return nil
				}); err != nil {
					return false, fmt.Errorf("db: %w", err)
				}
				h.recordAuditLog(&domain.Log{UserID: &u.ID, Action: fmt.Sprintf("admin.user.set_notifications:%d:%t:%t", uid, em, wp)})
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	return fields
}
