package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	graphql "github.com/graphql-go/graphql"
)

// queryFieldsAdminUsers returns GraphQL fields for admin users queries.
func queryFieldsAdminUsers(t *gqlTypes) graphql.Fields {
	adminUserType := t.AdminUserType
	nsType := t.NotificationSettingsType
	devType := t.NotificationDeviceType
	ntpType := t.NotificationTypePrefsType

	return graphql.Fields{
		"adminUserNotificationSettings": &graphql.Field{
			Type: nsType,
			Args: graphql.FieldConfigArgument{
				"user_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
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
					// Defaults
					emailEnabled := true
					webpushEnabled := false
					var latest *time.Time
					var devs []NotificationDevice
					_ = DB.Where("user_id = ? AND platform IN ?", uid, []string{"EMAIL", "WEBPUSH"}).Find(&devs).Error
					for _, d := range devs {
						switch strings.ToUpper(strings.TrimSpace(d.Platform)) {
						case "EMAIL":
							emailEnabled = d.IsEnabled
						case "WEBPUSH":
							webpushEnabled = d.IsEnabled
						}
						if latest == nil || d.UpdatedAt.After(*latest) {
							t := d.UpdatedAt
							latest = &t
						}
					}
					var updated any = nil
					if latest != nil {
						updated = latest.Format(time.RFC3339)
					}
					return map[string]any{"email_enabled": emailEnabled, "webpush_enabled": webpushEnabled, "updated_at": updated}, nil
				}
				return nil, fmt.Errorf("context missing")
			},
		},
		// Notification type preferences
		"adminUserNotificationTypePrefs": &graphql.Field{
			Type: ntpType,
			Args: graphql.FieldConfigArgument{
				"user_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
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
					// defaults
					prefs := map[string]bool{"TASK": true, "DAYSHIFT": true}
					var list []NotificationTypePref
					_ = DB.Where("user_id = ?", uid).Find(&list).Error
					for _, it := range list {
						k := strings.ToUpper(strings.TrimSpace(it.NotificationType))
						prefs[k] = it.IsEnabled
					}
					return map[string]any{"TASK": prefs["TASK"], "DAYSHIFT": prefs["DAYSHIFT"]}, nil
				}
				return nil, fmt.Errorf("context missing")
			},
		},

		// Devices list by user
		"adminUserDevices": &graphql.Field{
			Type: graphql.NewList(devType),
			Args: graphql.FieldConfigArgument{
				"user_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
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
					var devs []NotificationDevice
					if err := DB.Where("user_id = ?", uid).Order("id ASC").Find(&devs).Error; err != nil {
						return nil, err
					}
					out := make([]map[string]any, 0, len(devs))
					for _, d := range devs {
						out = append(out, map[string]any{
							"id":         int(d.ID),
							"user_id":    int(d.UserID),
							"platform":   d.Platform,
							"name":       d.Name,
							"payload":    d.Payload,
							"is_enabled": d.IsEnabled,
							"created_at": d.CreatedAt.Format(time.RFC3339),
							"updated_at": d.UpdatedAt.Format(time.RFC3339),
						})
					}
					return out, nil
				}
				return nil, fmt.Errorf("context missing")
			},
		},

		// Helper: fetch specific users by IDs for display mapping
		"adminUsersByIds": &graphql.Field{
			Type: graphql.NewList(adminUserType),
			Args: graphql.FieldConfigArgument{
				"ids": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.Int)))},
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
					raw, _ := p.Args["ids"].([]any)
					if len(raw) == 0 {
						return []any{}, nil
					}
					ids := make([]uint, 0, len(raw))
					for _, v := range raw {
						switch t := v.(type) {
						case int:
							ids = append(ids, uint(t))
						case int64:
							ids = append(ids, uint(t))
						case float64:
							ids = append(ids, uint(t))
						}
					}
					var users []User
					if err := DB.Where("id IN ?", ids).Order("id ASC").Find(&users).Error; err != nil {
						return nil, err
					}
					ridx := map[uint][]string{}
					if len(users) > 0 {
						var rls []RoleUser
						_ = DB.Where("user_id IN ?", ids).Find(&rls).Error
						for _, r := range rls {
							ridx[r.UserID] = append(ridx[r.UserID], r.Role)
						}
					}
					blocked := map[uint]bool{}
					if len(users) > 0 {
						var blks []Block
						_ = DB.Where("user_id IN ? AND (expires_at IS NULL OR expires_at > ?)", ids, time.Now()).Find(&blks).Error
						for _, b := range blks {
							blocked[b.UserID] = true
						}
					}
					out := make([]map[string]any, 0, len(users))
					for _, it := range users {
						out = append(out, map[string]any{
							"id":                  it.ID,
							"email":               it.Email,
							"display_name":        it.DisplayName,
							"notification_lang":   it.NotificationLang,
							"email_login_enabled": it.EmailCodeAllowed,
							"roles":               ridx[it.ID],
							"is_blocked":          blocked[it.ID],
						})
					}
					return out, nil
				}
				return nil, fmt.Errorf("context missing")
			},
		},
		// Admin: users listing with roles and block status
		"adminUsers": &graphql.Field{
			Type: graphql.NewList(adminUserType),
			Args: graphql.FieldConfigArgument{
				"q": &graphql.ArgumentConfig{Type: graphql.String},
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
					qstr, _ := p.Args["q"].(string)
					qstr = strings.TrimSpace(qstr)
					dbq := DB.Model(&User{})
					if qstr != "" {
						like := "%" + qstr + "%"
						dbq = dbq.Where("email LIKE ? OR display_name LIKE ?", like, like)
					}
					var users []User
					if err := dbq.Order("id DESC").Limit(100).Find(&users).Error; err != nil {
						return nil, err
					}
					ids := make([]uint, 0, len(users))
					for _, uu := range users {
						ids = append(ids, uu.ID)
					}
					roleMap := map[uint][]string{}
					if len(ids) > 0 {
						var roles []RoleUser
						_ = DB.Where("user_id IN ?", ids).Find(&roles).Error
						for _, r := range roles {
							roleMap[r.UserID] = append(roleMap[r.UserID], r.Role)
						}
					}
					blocked := map[uint]bool{}
					if len(ids) > 0 {
						var blks []Block
						_ = DB.Where("user_id IN ? AND (expires_at IS NULL OR expires_at > ?)", ids, time.Now()).Find(&blks).Error
						for _, b := range blks {
							blocked[b.UserID] = true
						}
					}
					out := make([]map[string]any, 0, len(users))
					for _, it := range users {
						out = append(out, map[string]any{
							"id":                  it.ID,
							"email":               it.Email,
							"display_name":        it.DisplayName,
							"notification_lang":   it.NotificationLang,
							"email_login_enabled": it.EmailCodeAllowed,
							"roles":               roleMap[it.ID],
							"is_blocked":          blocked[it.ID],
						})
					}
					return out, nil
				}
				return nil, fmt.Errorf("context missing")
			},
		},
		// Admin: users listing with pagination and sorting
		"adminUsersPaginated": &graphql.Field{
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "AdminUsersPaginatedResult",
				Fields: graphql.Fields{
					"items": &graphql.Field{Type: graphql.NewList(adminUserType)},
					"total": &graphql.Field{Type: graphql.Int},
				},
			}),
			Args: graphql.FieldConfigArgument{
				"q":         &graphql.ArgumentConfig{Type: graphql.String},
				"sort_by":   &graphql.ArgumentConfig{Type: graphql.String}, // id | email | display_name | created_at
				"sort_dir":  &graphql.ArgumentConfig{Type: graphql.String}, // asc | desc
				"page":      &graphql.ArgumentConfig{Type: graphql.Int},
				"page_size": &graphql.ArgumentConfig{Type: graphql.Int},
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
					qstr, _ := p.Args["q"].(string)
					qstr = strings.TrimSpace(qstr)
					sortBy, _ := p.Args["sort_by"].(string)
					sortDir, _ := p.Args["sort_dir"].(string)
					page := 1
					if v, ok := p.Args["page"].(int); ok && v > 0 {
						page = v
					}
					size := 20
					if v, ok := p.Args["page_size"].(int); ok && v > 0 && v <= 200 {
						size = v
					}
					dbq := DB.Model(&User{})
					if qstr != "" {
						like := "%" + qstr + "%"
						dbq = dbq.Where("email LIKE ? OR display_name LIKE ?", like, like)
					}
					var total int64
					_ = dbq.Count(&total).Error
					// Sorting mapping
					sb := strings.ToLower(strings.TrimSpace(sortBy))
					sd := strings.ToLower(strings.TrimSpace(sortDir))
					col := "id"
					switch sb {
					case "email":
						col = "email"
					case "display_name":
						col = "display_name"
					case "created_at":
						col = "created_at"
					default:
						col = "id"
					}
					ord := "DESC"
					if sd == "asc" {
						ord = "ASC"
					} else if sd == "desc" {
						ord = "DESC"
					} else if sb == "" { // no sort requested
						col = "id"
						ord = "DESC"
					}
					offset := (page - 1) * size
					var users []User
					if err := dbq.Order(col + " " + ord).Offset(offset).Limit(size).Find(&users).Error; err != nil {
						return nil, err
					}
					ids := make([]uint, 0, len(users))
					for _, uu := range users {
						ids = append(ids, uu.ID)
					}
					roleMap := map[uint][]string{}
					if len(ids) > 0 {
						var roles []RoleUser
						_ = DB.Where("user_id IN ?", ids).Find(&roles).Error
						for _, r := range roles {
							roleMap[r.UserID] = append(roleMap[r.UserID], r.Role)
						}
					}
					blocked := map[uint]bool{}
					if len(ids) > 0 {
						var blks []Block
						_ = DB.Where("user_id IN ? AND (expires_at IS NULL OR expires_at > ?)", ids, time.Now()).Find(&blks).Error
						for _, b := range blks {
							blocked[b.UserID] = true
						}
					}
					out := make([]map[string]any, 0, len(users))
					for _, it := range users {
						out = append(out, map[string]any{
							"id":                  it.ID,
							"email":               it.Email,
							"display_name":        it.DisplayName,
							"notification_lang":   it.NotificationLang,
							"email_login_enabled": it.EmailCodeAllowed,
							"roles":               roleMap[it.ID],
							"is_blocked":          blocked[it.ID],
						})
					}
					return map[string]any{"items": out, "total": int(total)}, nil
				}
				return nil, fmt.Errorf("context missing")
			},
		},
	}
}
