package gqlMutation

import (
	"fmt"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	gqlH "github.com/asari194617/taskly/apps/server/internal/handler/graphql/shared"
	"github.com/asari194617/taskly/apps/server/internal/jobs"
	"github.com/asari194617/taskly/apps/server/internal/tasklist"
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
			"image_key":              &graphql.ArgumentConfig{Type: graphql.String},
			"image_keys":             &graphql.ArgumentConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String))},
			"type":                   &graphql.ArgumentConfig{Type: graphql.String},
			"is_require_hand_in":     &graphql.ArgumentConfig{Type: graphql.Boolean},
			"is_self_check_required": &graphql.ArgumentConfig{Type: graphql.Boolean},
			"deadline":               &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}, // YYYY-MM-DD
			"deadline_since":         &graphql.ArgumentConfig{Type: graphql.String},                     // ENUM
			"deadline_until":         &graphql.ArgumentConfig{Type: graphql.String},                     // ENUM
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
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
				if strings.TrimSpace(title) == "" || subjectID <= 0 {
					return nil, fmt.Errorf("invalid task")
				}
				desc, _ := p.Args["description"].(string)
				var descPtr *string
				if strings.TrimSpace(desc) != "" {
					descPtr = &desc
				}
				var imageKeys []string
				var err error
				if raw, supplied := p.Args["image_keys"]; supplied {
					imageKeys, err = parseTaskImageKeys(raw)
					if err != nil {
						return nil, err
					}
				} else if imageKey, ok := p.Args["image_key"].(string); ok && strings.TrimSpace(imageKey) != "" {
					imageKeys = []string{strings.TrimSpace(imageKey)}
				}
				if err := validateTaskImageKeys(p.Context, u.ID, imageKeys); err != nil {
					return nil, err
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
				since, until, err := tasklist.NormalizeDeadlineWindow(dlSinceStr, dlUntilStr)
				if err != nil {
					return nil, err
				}
				recCreated, err := jobs.CreateTaskWithImages(h.db, u.ID, title, uint(subjectID), descPtr, typ, irh, scr, domain.Task{Deadline: utils.DateOnly(dt), DeadlineSince: since, DeadlineUntil: until}, imageKeys, u.Email)
				if err != nil {
					return nil, fmt.Errorf("db")
				}
				imageURL, imageURLs := taskImageFields(p.Context, imageKeys)
				out := map[string]any{
					"id":                     recCreated.ID,
					"title":                  recCreated.Title,
					"subject_id":             recCreated.SubjectID,
					"description":            recCreated.Description,
					"image_url":              imageURL,
					"image_keys":             imageKeys,
					"image_urls":             imageURLs,
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
			"image_keys":             &graphql.ArgumentConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String))},
			"type":                   &graphql.ArgumentConfig{Type: graphql.String},
			"is_require_hand_in":     &graphql.ArgumentConfig{Type: graphql.Boolean},
			"is_self_check_required": &graphql.ArgumentConfig{Type: graphql.Boolean},
			"deadline":               &graphql.ArgumentConfig{Type: graphql.String}, // YYYY-MM-DD
			"deadline_since":         &graphql.ArgumentConfig{Type: graphql.String},
			"deadline_until":         &graphql.ArgumentConfig{Type: graphql.String},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			h := h.withContext(p.Context)
			if c, ok := gqlH.FiberCtxFromContext(p.Context); ok {
				if !utils.UserFromSession(c, h.db, h.rdb) {
					return nil, fmt.Errorf("unauthorized")
				}
				u := c.Locals("user").(*domain.User)
				id, _ := p.Args["id"].(int)
				var target domain.Task
				if err := h.db.First(&target, id).Error; err != nil {
					return nil, fmt.Errorf("not found")
				}
				allowed, err := tasklist.CanAccessTask(h.db, u.ID, "update", target)
				if err != nil {
					return nil, err
				}
				if !allowed {
					return nil, fmt.Errorf("forbidden")
				}
				previousImageKeys := taskKeys(h.db, target)
				var imageKeys *[]string
				if raw, supplied := p.Args["image_keys"]; supplied {
					keys, err := parseTaskImageKeys(raw)
					if err != nil {
						return nil, err
					}
					if err := validateTaskImageKeys(p.Context, u.ID, keys); err != nil {
						return nil, err
					}
					imageKeys = &keys
				}
				// perform update via domain service
				_, after, err := jobs.UpdateTaskWithImages(h.db, uint(id), func(r *domain.Task) error {
					if v, ok := p.Args["title"].(string); ok {
						if strings.TrimSpace(v) == "" {
							return fmt.Errorf("invalid title")
						}
						r.Title = v
					}
					if v, ok := p.Args["subject_id"].(int); ok {
						if v <= 0 {
							return fmt.Errorf("invalid subject_id")
						}
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
						if vv == "" {
							return fmt.Errorf("deadline required")
						}
						if t, err := time.Parse("2006-01-02", vv); err == nil {
							r.Deadline = utils.DateOnly(t)
						} else {
							return fmt.Errorf("invalid deadline")
						}
					}
					if v, ok := p.Args["deadline_since"].(string); ok {
						r.DeadlineSince = v
					}
					if v, ok := p.Args["deadline_until"].(string); ok {
						r.DeadlineUntil = v
					}
					since, until, err := tasklist.NormalizeDeadlineWindow(r.DeadlineSince, r.DeadlineUntil)
					if err != nil {
						return err
					}
					r.DeadlineSince, r.DeadlineUntil = since, until
					return nil
				}, imageKeys, u.Email)
				if err != nil {
					return nil, err
				}
				finalImageKeys := previousImageKeys
				if imageKeys != nil {
					finalImageKeys = *imageKeys
					deleteTaskImages(p.Context, removedTaskImageKeys(previousImageKeys, finalImageKeys))
				}
				imageURL, imageURLs := taskImageFields(p.Context, finalImageKeys)
				out := map[string]any{
					"id":                     after.ID,
					"title":                  after.Title,
					"subject_id":             after.SubjectID,
					"description":            after.Description,
					"image_url":              imageURL,
					"image_keys":             finalImageKeys,
					"image_urls":             imageURLs,
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
			h := h.withContext(p.Context)
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
				allowed, err := tasklist.CanAccessTask(h.db, u.ID, "delete", rec)
				if err != nil {
					return false, err
				}
				if !allowed {
					return false, fmt.Errorf("forbidden")
				}
				imageKeys := taskKeys(h.db, rec)
				_, err = jobs.DeleteTask(h.db, rec.ID, u.Email)
				if err != nil {
					return false, fmt.Errorf("db")
				}
				deleteTaskImages(p.Context, imageKeys)
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
			h := h.withContext(p.Context)
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
				allowed, err := tasklist.CanAccessTask(h.db, u.ID, "read", rec)
				if err != nil {
					return false, err
				}
				if !allowed {
					return false, fmt.Errorf("forbidden")
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
			h := h.withContext(p.Context)
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
				allowed, err := tasklist.CanAccessTask(h.db, u.ID, "read", rec)
				if err != nil {
					return false, err
				}
				if !allowed {
					return false, fmt.Errorf("forbidden")
				}
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
