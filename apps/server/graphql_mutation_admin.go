package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	graphql "github.com/graphql-go/graphql"
)

// mutationFieldsAdminUsers groups admin user management mutations.
func mutationFieldsAdminUsers(t *gqlTypes) graphql.Fields {
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
				if err := DB.Model(&User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				_ = DB.Create(&Log{UserID: &u.ID, Action: fmt.Sprintf("admin.user.update:%d", id)}).Error
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
				email := strings.TrimSpace(p.Args["email"].(string))
				if email == "" {
					return nil, fmt.Errorf("invalid email")
				}
				dn, _ := p.Args["display_name"].(string)
				lang, _ := p.Args["notification_lang"].(string)
				rec := User{Email: email, DisplayName: dn, NotificationLang: strings.ToLower(strings.TrimSpace(lang))}
				if rec.NotificationLang == "" {
					rec.NotificationLang = "ja"
				}
				if err := DB.Create(&rec).Error; err != nil {
					return nil, fmt.Errorf("db")
				}
				_ = DB.Create(&Log{UserID: &u.ID, Action: fmt.Sprintf("admin.user.create:%d", rec.ID)}).Error
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
					rec := User{Email: email, DisplayName: dn, NotificationLang: lang}
					if err := DB.Create(&rec).Error; err != nil {
						errs = append(errs, fmt.Sprintf("line %d: db", i+1))
						continue
					}
					created++
				}
				_ = DB.Create(&Log{UserID: &u.ID, Action: fmt.Sprintf("admin.user.bulk_create:%d", created)}).Error
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
				enabled := p.Args["enabled"].(bool)
				if role == "" {
					return false, fmt.Errorf("invalid role")
				}
				if enabled {
					var cnt int64
					_ = DB.Model(&RoleUser{}).Where("user_id = ? AND role = ?", uid, role).Count(&cnt).Error
					if cnt == 0 {
						_ = DB.Create(&RoleUser{UserID: uid, Role: role}).Error
					}
				} else {
					_ = DB.Where("user_id = ? AND role = ?", uid, role).Delete(&RoleUser{}).Error
				}
				_ = DB.Create(&Log{UserID: &u.ID, Action: fmt.Sprintf("admin.role.setcustom:%d:%s:%t", uid, role, enabled)}).Error
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
				id := p.Args["user_id"].(int)
				hard, _ := p.Args["hard"].(bool)
				if hard {
					if err := DB.Delete(&User{}, id).Error; err != nil {
						return false, fmt.Errorf("db")
					}
				} else {
					if err := DB.Model(&User{}).Where("id = ?", id).Update("is_deleted", true).Error; err != nil {
						return false, fmt.Errorf("db")
					}
				}
				_ = DB.Create(&Log{UserID: &u.ID, Action: fmt.Sprintf("admin.user.delete:%d:%t", id, hard)}).Error
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	fields["adminUserAddRole"] = &graphql.Field{
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
	}

	fields["adminUserRemoveRole"] = &graphql.Field{
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
	}

	fields["adminUserBlock"] = &graphql.Field{
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
	}

	fields["adminUserUnblock"] = &graphql.Field{
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
				tp := strings.ToUpper(strings.TrimSpace(p.Args["notification_type"].(string)))
				if tp != "TASK" && tp != "DAYSHIFT" {
					return false, fmt.Errorf("invalid type")
				}
				en := p.Args["enabled"].(bool)
				var rec NotificationTypePref
				if err := DB.Where("user_id = ? AND notification_type = ?", uid, tp).First(&rec).Error; err != nil {
					rec = NotificationTypePref{UserID: uid, NotificationType: tp, IsEnabled: en}
					if err := DB.Create(&rec).Error; err != nil {
						return false, fmt.Errorf("db")
					}
				} else {
					if err := DB.Model(&NotificationTypePref{}).Where("id = ?", rec.ID).Update("is_enabled", en).Error; err != nil {
						return false, fmt.Errorf("db")
					}
				}
				_ = DB.Create(&Log{UserID: &u.ID, Action: fmt.Sprintf("admin.user.set_type_pref:%d:%s:%t", uid, tp, en)}).Error
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
				rec := NotificationDevice{UserID: uid, Platform: platform, Name: namePtr, Payload: payloadPtr, IsEnabled: enabled}
				if err := DB.Create(&rec).Error; err != nil {
					return nil, fmt.Errorf("db")
				}
				_ = DB.Create(&Log{UserID: &u.ID, Action: fmt.Sprintf("admin.device.create:%d:%s", uid, platform)}).Error
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
				if err := DB.Model(&NotificationDevice{}).Where("id = ?", id).Updates(updates).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				_ = DB.Create(&Log{UserID: &u.ID, Action: fmt.Sprintf("admin.device.update:%d", id)}).Error
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
				id := uint(p.Args["device_id"].(int))
				if err := DB.Delete(&NotificationDevice{}, id).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				_ = DB.Create(&Log{UserID: &u.ID, Action: fmt.Sprintf("admin.device.delete:%d", id)}).Error
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
				em := p.Args["email_enabled"].(bool)
				wp := p.Args["webpush_enabled"].(bool)
				// Upsert EMAIL device
				var emDev NotificationDevice
				if err := DB.Where("user_id = ? AND platform = ?", uid, "EMAIL").First(&emDev).Error; err == nil {
					if err := DB.Model(&NotificationDevice{}).Where("id = ?", emDev.ID).Update("is_enabled", em).Error; err != nil {
						return false, fmt.Errorf("db")
					}
				} else if em {
					// create record only when enabling
					rec := NotificationDevice{UserID: uid, Platform: "EMAIL", IsEnabled: true}
					if err := DB.Create(&rec).Error; err != nil {
						return false, fmt.Errorf("db")
					}
				}
				// Upsert WEBPUSH device
				var wpDev NotificationDevice
				if err := DB.Where("user_id = ? AND platform = ?", uid, "WEBPUSH").First(&wpDev).Error; err == nil {
					if err := DB.Model(&NotificationDevice{}).Where("id = ?", wpDev.ID).Update("is_enabled", wp).Error; err != nil {
						return false, fmt.Errorf("db")
					}
				} else if wp {
					// create record only when enabling
					rec := NotificationDevice{UserID: uid, Platform: "WEBPUSH", IsEnabled: true}
					if err := DB.Create(&rec).Error; err != nil {
						return false, fmt.Errorf("db")
					}
				}
				_ = DB.Create(&Log{UserID: &u.ID, Action: fmt.Sprintf("admin.user.set_notifications:%d:%t:%t", uid, em, wp)}).Error
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	return fields
}
