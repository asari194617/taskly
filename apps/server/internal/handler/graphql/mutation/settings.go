package gqlMutation

import (
	"fmt"
	"strings"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	gqlH "github.com/asari194617/taskly/apps/server/internal/handler/graphql/shared"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	graphql "github.com/graphql-go/graphql"
)

// mutationFieldsSettings groups settings-related mutations (configs etc.).
func (h *Handler) mutationFieldsSettings(t *gqlH.GqlTypes) graphql.Fields {
	fields := graphql.Fields{}

	// Update day shift current index in configs (admin/manager only)
	fields["adminUpdateDayShiftIndex"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"index": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
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
				// Clamp and store integer as string value
				idx := p.Args["index"].(int)
				if idx < 0 {
					idx = 0
				}
				// Upsert into configs
				// Prefer UPDATE first; if no row affected, INSERT
				res := h.db.Exec("UPDATE configs SET value = ? WHERE `key`='day_shift_current_index'", fmt.Sprintf("%d", idx))
				if res.Error != nil {
					return false, fmt.Errorf("db")
				}
				if res.RowsAffected == 0 {
					if err := h.db.Exec("INSERT INTO configs(`key`, `value`) VALUES('day_shift_current_index', ?) ON DUPLICATE KEY UPDATE value=VALUES(value)", fmt.Sprintf("%d", idx)).Error; err != nil {
						return false, fmt.Errorf("db")
					}
				}
				_ = h.db.Create(&domain.Log{UserID: &u.ID, Action: "admin.settings.update_day_shift_index"}).Error
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	// Generic config setter (string value) for future needs; admin-only
	fields["adminSetConfig"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"key":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			"value": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				isAdmin, _ := utils.UserHasRole(u.ID, "admin", h.db)
				if !isAdmin {
					return false, fmt.Errorf("forbidden")
				}
				key := strings.TrimSpace(p.Args["key"].(string))
				val := p.Args["value"].(string)
				if key == "" {
					return false, fmt.Errorf("invalid key")
				}
				res := h.db.Exec("UPDATE configs SET value = ? WHERE `key`=?", val, key)
				if res.Error != nil {
					return false, fmt.Errorf("db")
				}
				if res.RowsAffected == 0 {
					if err := h.db.Exec("INSERT INTO configs(`key`, `value`) VALUES(?, ?) ON DUPLICATE KEY UPDATE value=VALUES(value)", key, val).Error; err != nil {
						return false, fmt.Errorf("db")
					}
				}
				_ = h.db.Create(&domain.Log{UserID: &u.ID, Action: "admin.settings.set_config"}).Error
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	// Set current day shift cycle id (admin/manager)
	fields["adminSetDayShiftCurrentCycle"] = &graphql.Field{
		Type: graphql.Boolean,
		Args: graphql.FieldConfigArgument{
			"cycle_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
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
				cid := p.Args["cycle_id"].(int)
				if cid < 0 {
					cid = 0
				}
				if err := utils.SetConfigString("day_shift_current_cycle", fmt.Sprintf("%d", cid), h.db); err != nil {
					return false, fmt.Errorf("db")
				}
				_ = h.db.Create(&domain.Log{UserID: &u.ID, Action: "admin.settings.set_day_shift_current_cycle"}).Error
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	return fields
}
