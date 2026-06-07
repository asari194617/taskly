package gqlMutation

import (
	"fmt"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	gqlH "github.com/asari194617/taskly/apps/server/internal/handler/graphql/shared"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	graphql "github.com/graphql-go/graphql"
)

// mutationFieldsTasks groups task CRUD mutations.
func (h *Handler) mutationFieldsTasks(t *gqlH.GqlTypes) graphql.Fields {
	fields := graphql.Fields{}

	taskType := t.TaskType

	fields["createTask"] = &graphql.Field{
		Type: taskType,
		Args: graphql.FieldConfigArgument{
			"title":                  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			"subject_id":             &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
			"description":            &graphql.ArgumentConfig{Type: graphql.String},
			"type":                   &graphql.ArgumentConfig{Type: graphql.String},
			"is_require_hand_in":     &graphql.ArgumentConfig{Type: graphql.Boolean},
			"is_self_check_required": &graphql.ArgumentConfig{Type: graphql.Boolean},
			"deadline":               &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}, // YYYY-MM-DD
			"deadline_since":         &graphql.ArgumentConfig{Type: graphql.String},                     // ENUM
			"deadline_until":         &graphql.ArgumentConfig{Type: graphql.String},                     // ENUM
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return nil, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				if okp, err := utils.CheckPermission(u.ID, "tasks", "create", nil, h.db); err != nil {
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
				typ = utils.NormalizeTaskType(typ)
				irh, _ := p.Args["is_require_hand_in"].(bool)
				scr, _ := p.Args["is_self_check_required"].(bool)
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
				validPeriods := map[string]bool{"NONE": true, "P1": true, "P2": true, "P3": true, "P4": true, "P5": true, "P6": true, "P7": true, "P8": true, "P9": true, "P10": true}
				validTimeLabels := map[string]bool{"MORNING": true, "LUNCH": true, "AFTER_SCHOOL": true}
				if !validPeriods[since] {
					return nil, fmt.Errorf("invalid deadline_since")
				}
				validUntil := make(map[string]bool)
				for k, v := range validPeriods {
					validUntil[k] = v
				}
				for k, v := range validTimeLabels {
					validUntil[k] = v
				}
				if !validUntil[until] {
					return nil, fmt.Errorf("invalid deadline_until")
				}
				recCreated, err := domain.CreateTask(h.db, u.ID, title, uint(subjectID), descPtr, typ, irh, scr, utils.DateOnly(dt), since, until, u.Email)
				if err != nil {
					return nil, fmt.Errorf("db")
				}
				// Fire async Teams notification email for Power Automate
				go utils.SendTeamsNotifyEmailForTask(recCreated, h.db)
				// Async Discord notification (embed) to align with REST create
				go utils.SendDiscordTaskEmbed("created", recCreated, u.Email)
				out := map[string]any{
					"id":                     recCreated.ID,
					"title":                  recCreated.Title,
					"subject_id":             recCreated.SubjectID,
					"description":            recCreated.Description,
					"type":                   recCreated.Type,
					"is_require_hand_in":     recCreated.IsRequireHandIn,
					"is_self_check_required": recCreated.IsSelfCheckRequired,
					"deadline":               recCreated.Deadline.Format("2006-01-02"),
					"deadline_since":         recCreated.DeadlineSince,
					"deadline_until":         recCreated.DeadlineUntil,
					"created_at":             recCreated.CreatedAt.Format(time.RFC3339),
					"updated_at":             recCreated.UpdatedAt.Format(time.RFC3339),
				}
				return out, nil
			}
			return nil, fmt.Errorf("context missing")
		},
	}

	fields["updateTask"] = &graphql.Field{
		Type: taskType,
		Args: graphql.FieldConfigArgument{
			"id":                     &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
			"title":                  &graphql.ArgumentConfig{Type: graphql.String},
			"subject_id":             &graphql.ArgumentConfig{Type: graphql.Int},
			"description":            &graphql.ArgumentConfig{Type: graphql.String},
			"type":                   &graphql.ArgumentConfig{Type: graphql.String},
			"is_require_hand_in":     &graphql.ArgumentConfig{Type: graphql.Boolean},
			"is_self_check_required": &graphql.ArgumentConfig{Type: graphql.Boolean},
			"deadline":               &graphql.ArgumentConfig{Type: graphql.String}, // YYYY-MM-DD
			"deadline_since":         &graphql.ArgumentConfig{Type: graphql.String},
			"deadline_until":         &graphql.ArgumentConfig{Type: graphql.String},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return nil, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				id, _ := p.Args["id"].(int)
				// perform update via domain service
				before, after, err := domain.UpdateTask(h.db, uint(id), func(r *domain.Task) error {
					if v, ok := p.Args["title"].(string); ok {
						r.Title = v
					}
					if v, ok := p.Args["subject_id"].(int); ok {
						r.SubjectID = uint(v)
					}
					if v, ok := p.Args["description"].(string); ok {
						if strings.TrimSpace(v) == "" {
							r.Description = nil
						} else {
							r.Description = &v
						}
					}
					if v, ok := p.Args["type"].(string); ok {
						r.Type = utils.NormalizeTaskType(v)
					}
					if v, ok := p.Args["is_require_hand_in"].(bool); ok {
						r.IsRequireHandIn = v
					}
					if v, ok := p.Args["is_self_check_required"].(bool); ok {
						r.IsSelfCheckRequired = v
					}
					if v, ok := p.Args["deadline"].(string); ok {
						vv := strings.TrimSpace(v)
						if vv != "" {
							if t, err := time.Parse("2006-01-02", vv); err == nil {
								r.Deadline = utils.DateOnly(t)
							} else {
								return fmt.Errorf("invalid deadline")
							}
						}
					}
					if v, ok := p.Args["deadline_since"].(string); ok {
						vv := strings.ToUpper(strings.TrimSpace(v))
						if vv == "" {
							vv = "NONE"
						}
						if vv != "NONE" && vv != "P1" && vv != "P2" && vv != "P3" && vv != "P4" && vv != "P5" && vv != "P6" && vv != "P7" && vv != "P8" && vv != "P9" && vv != "P10" {
							return fmt.Errorf("invalid deadline_since")
						}
						r.DeadlineSince = vv
					}
					if v, ok := p.Args["deadline_until"].(string); ok {
						vv := strings.ToUpper(strings.TrimSpace(v))
						if vv == "" {
							vv = "NONE"
						}
						// Allow MORNING, LUNCH, AFTER_SCHOOL, or period enums
						if vv != "NONE" && vv != "MORNING" && vv != "LUNCH" && vv != "AFTER_SCHOOL" && vv != "P1" && vv != "P2" && vv != "P3" && vv != "P4" && vv != "P5" && vv != "P6" && vv != "P7" && vv != "P8" && vv != "P9" && vv != "P10" {
							return fmt.Errorf("invalid deadline_until")
						}
						r.DeadlineUntil = vv
					}
					return nil
				}, u.Email)
				if err != nil {
					return nil, err
				}
				// Fire Teams reply for update (if original message exists)
				if mid, ok := utils.LatestTeamsMessageIDForTask(after.ID, h.db); ok {
					msg := utils.BuildTaskUpdateMessageJa(before, after, h.db)
					go utils.SendTeamsReplyEmail(after.ID, mid, msg)
				}
				// Async Discord notification (embed) to align with REST update
				go utils.SendDiscordTaskEmbed("updated", after, u.Email)
				out := map[string]any{
					"id":                     after.ID,
					"title":                  after.Title,
					"subject_id":             after.SubjectID,
					"description":            after.Description,
					"type":                   after.Type,
					"is_require_hand_in":     after.IsRequireHandIn,
					"is_self_check_required": after.IsSelfCheckRequired,
					"deadline":               after.Deadline.Format("2006-01-02"),
					"deadline_since":         after.DeadlineSince,
					"deadline_until":         after.DeadlineUntil,
					"created_at":             after.CreatedAt.Format(time.RFC3339),
					"updated_at":             after.UpdatedAt.Format(time.RFC3339),
				}
				return out, nil
			}
			return nil, fmt.Errorf("context missing")
		},
	}

	fields["deleteTask"] = &graphql.Field{
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
				var rec domain.Task
				if err := h.db.First(&rec, id).Error; err != nil {
					return false, fmt.Errorf("not found")
				}
				isAdmin, _ := utils.UserHasRole(u.ID, "admin", h.db)
				isManager, _ := utils.UserHasRole(u.ID, "manager", h.db)
				allowed := isAdmin || isManager
				if !allowed {
					if okp, err := utils.CheckPermission(u.ID, "tasks", "delete", &rec.AddedUserID, h.db); err != nil {
						return false, err
					} else {
						allowed = okp
					}
				}
				if !allowed {
					return false, fmt.Errorf("forbidden")
				}
				if deleted, err := domain.DeleteTask(h.db, rec.ID, u.Email); err != nil {
					return false, fmt.Errorf("db")
				} else {
					// Fire Teams reply for delete (if original message exists)
					if mid, ok := utils.LatestTeamsMessageIDForTask(deleted.ID, h.db); ok {
						msg := utils.BuildTaskDeleteMessageJa(deleted)
						go utils.SendTeamsReplyEmail(deleted.ID, mid, msg)
					}
					// Async Discord notification (embed) to align with REST delete
					go utils.SendDiscordTaskEmbed("deleted", deleted, u.Email)
					return true, nil
				}
			}
			return false, fmt.Errorf("context missing")
		},
	}

	// Mark task as completed for current user
	fields["completeTask"] = &graphql.Field{
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
				// Ensure task exists
				var rec domain.Task
				if err := h.db.First(&rec, id).Error; err != nil {
					return false, fmt.Errorf("not found")
				}
				// Upsert completion via domain service
				if err := domain.CompleteTask(h.db, u.ID, rec.ID); err != nil {
					return false, fmt.Errorf("db")
				}
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	// Unmark task completion for current user
	fields["uncompleteTask"] = &graphql.Field{
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
				if err := domain.UncompleteTask(h.db, u.ID, uint(id)); err != nil {
					return false, fmt.Errorf("db")
				}
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	return fields
}
