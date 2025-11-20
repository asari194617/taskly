package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	graphql "github.com/graphql-go/graphql"
)

// mutationFieldsTasks groups task CRUD mutations.
func mutationFieldsTasks(t *gqlTypes) graphql.Fields {
	fields := graphql.Fields{}

	taskType := t.TaskType

	fields["createTask"] = &graphql.Field{
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
				typ = normalizeTaskType(typ)
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
				// Async Discord notification (embed) to align with REST create
				go sendDiscordTaskEmbed("created", rec, u.Email)
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
	}

	fields["updateTask"] = &graphql.Field{
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
				before := rec
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
					rec.Type = normalizeTaskType(v)
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
				// Fire Teams reply for update (if original message exists)
				if mid, ok := latestTeamsMessageIDForTask(rec.ID); ok {
					msg := buildTaskUpdateMessageJa(before, rec)
					go sendTeamsReplyEmail(rec.ID, mid, msg)
				}
				// Async Discord notification (embed) to align with REST update
				go sendDiscordTaskEmbed("updated", rec, u.Email)
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
	}

	fields["deleteTask"] = &graphql.Field{
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
				// Fire Teams reply for delete (if original message exists)
				if mid, ok := latestTeamsMessageIDForTask(rec.ID); ok {
					msg := buildTaskDeleteMessageJa(rec)
					go sendTeamsReplyEmail(rec.ID, mid, msg)
				}
				// Async Discord notification (embed) to align with REST delete
				go sendDiscordTaskEmbed("deleted", rec, u.Email)
				return true, nil
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
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !userFromSession(c) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*User)
				id, _ := p.Args["id"].(int)
				// Ensure task exists
				var rec Task
				if err := DB.First(&rec, id).Error; err != nil {
					return false, fmt.Errorf("not found")
				}
				// Upsert completion
				if err := DB.Exec("INSERT INTO tasks_users_completions(user_id, task_id) VALUES(?, ?) ON DUPLICATE KEY UPDATE completed_at = CURRENT_TIMESTAMP", u.ID, rec.ID).Error; err != nil {
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
			if c, ok := p.Context.Value(ctxKeyFiberCtx).(*fiber.Ctx); ok {
				if !userFromSession(c) {
					return false, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*User)
				id, _ := p.Args["id"].(int)
				if err := DB.Exec("DELETE FROM tasks_users_completions WHERE user_id = ? AND task_id = ?", u.ID, id).Error; err != nil {
					return false, fmt.Errorf("db")
				}
				return true, nil
			}
			return false, fmt.Errorf("context missing")
		},
	}

	return fields
}
