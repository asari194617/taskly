package guidancebridge

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/tasklist"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type bridge struct {
	db        *gorm.DB
	cfg       config
	configErr error
}

type taskState struct {
	Key                 string      `json:"key"`
	TaskID              uint        `json:"task_id"`
	Completed           bool        `json:"completed"`
	CompletionDisabled  bool        `json:"completion_disabled"`
	TasklyURL           string      `json:"taskly_url"`
	AutoCompletedBy     string      `json:"auto_completed_by,omitempty"`
	AutoAssignmentScope string      `json:"auto_assignment_scope,omitempty"`
	RelatedTasks        []taskState `json:"related_tasks,omitempty"`
}

type completionRecord struct {
	UserID              uint
	TaskID              uint
	AutoEventID         *string
	AutoSourceTaskID    *uint
	AutoAssignmentScope *string
}

// Register accepts dedicated guidance web tokens or narrowly exchanged MCP
// bridge tokens. Both policies use live introspection and canonical task permissions.
// Generic API keys and Taskly browser sessions cannot authorize this bridge.
func Register(router fiber.Router, db *gorm.DB) {
	cfg, err := loadConfig()
	b := &bridge{db: db, cfg: cfg, configErr: err}
	router.Get("/tasks", b.authenticate, b.list)
	router.Put("/tasks/:key", b.authenticate, b.put)
}

func fail(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": message})
}

func (b *bridge) record(uid, id uint) (completionRecord, bool, error) {
	var row completionRecord
	err := b.db.Table("tasks_users_completions").Where("user_id = ? AND task_id = ?", uid, id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, false, nil
	}
	return row, err == nil, err
}

func (b *bridge) state(c *fiber.Ctx, key string) (taskState, error) {
	uid := c.Locals("guidance-user").(uint)
	id, found := b.cfg.bindings[key]
	if !found {
		return taskState{}, fiber.NewError(404, "task not found")
	}
	var task domain.Task
	if err := b.db.First(&task, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return taskState{}, fiber.NewError(404, "task not found")
	} else if err != nil {
		return taskState{}, fiber.NewError(503, "task unavailable")
	}
	allowed, err := tasklist.CanAccessTask(b.db, uid, "read", task)
	if err != nil {
		return taskState{}, fiber.NewError(503, "task access unavailable")
	}
	if !allowed {
		return taskState{}, fiber.NewError(403, "task access denied")
	}
	row, completed, err := b.record(uid, id)
	if err != nil {
		return taskState{}, fiber.NewError(503, "completion unavailable")
	}
	state := taskState{Key: key, TaskID: id, Completed: !task.IsCompletionDisabled && completed, CompletionDisabled: task.IsCompletionDisabled, TasklyURL: b.cfg.webURL + "/"}
	if state.Completed && strings.HasPrefix(key, "assigned-visit-") && row.AutoEventID != nil && *row.AutoEventID == b.cfg.eventID && row.AutoSourceTaskID != nil && row.AutoAssignmentScope != nil && assignmentScopePattern.MatchString(*row.AutoAssignmentScope) {
		sourceKey := "visit-survey-" + strings.TrimPrefix(key, "assigned-visit-")
		if sourceID := b.cfg.bindings[sourceKey]; sourceID > 0 && sourceID == *row.AutoSourceTaskID {
			state.AutoCompletedBy = sourceKey
			state.AutoAssignmentScope = *row.AutoAssignmentScope
		}
	}
	return state, nil
}

func (b *bridge) list(c *fiber.Ctx) error {
	states := make([]taskState, 0, len(b.cfg.bindings))
	// One snapshot prevents a concurrent atomic pair update from appearing as
	// mismatched survey/visit states in a multi-query response.
	err := b.db.Transaction(func(tx *gorm.DB) error {
		scoped := *b
		scoped.db = tx
		for _, key := range b.cfg.keys() {
			state, err := scoped.state(c, key)
			if err != nil {
				return err
			}
			states = append(states, state)
		}
		return nil
	}, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return bridgeError(c, err)
	}
	return c.JSON(fiber.Map{"event_id": b.cfg.eventID, "tasks": states})
}

func bridgeError(c *fiber.Ctx, err error) error {
	var fe *fiber.Error
	if errors.As(err, &fe) {
		return fail(c, fe.Code, fe.Message)
	}
	return fail(c, 503, "completion update unavailable")
}

func (b *bridge) put(c *fiber.Ctx) error {
	var input struct {
		Completed         *bool  `json:"completed"`
		LinkAssignedVisit bool   `json:"link_assigned_visit"`
		AssignmentScope   string `json:"assignment_scope"`
	}
	decoder := json.NewDecoder(bytes.NewReader(c.Body()))
	decoder.DisallowUnknownFields()
	if len(c.Body()) > 1024 || decoder.Decode(&input) != nil || input.Completed == nil || decoder.Decode(new(any)) != io.EOF {
		return fail(c, 400, "completed boolean required")
	}
	key := c.Params("key")
	targetKey := ""
	reverse := false
	if input.LinkAssignedVisit {
		if !taskKeyPattern.MatchString(key) || !assignmentScopePattern.MatchString(input.AssignmentScope) {
			return fail(c, 400, "invalid survey link")
		}
		if strings.HasPrefix(key, "visit-survey-") {
			targetKey = "assigned-visit-" + strings.TrimPrefix(key, "visit-survey-")
		} else if strings.HasPrefix(key, "assigned-visit-") && !*input.Completed {
			reverse = true
			targetKey = "visit-survey-" + strings.TrimPrefix(key, "assigned-visit-")
		} else {
			return fail(c, 400, "invalid survey link")
		}
		if b.cfg.bindings[targetKey] == 0 {
			return fail(c, 404, "assigned task not found")
		}
	} else if input.AssignmentScope != "" {
		return fail(c, 400, "invalid survey link")
	}
	id := b.cfg.bindings[key]
	if id == 0 {
		return fail(c, 404, "task not found")
	}
	var updated taskState
	err := b.db.Transaction(func(tx *gorm.DB) error {
		scoped := *b
		scoped.db = tx
		uid := c.Locals("guidance-user").(uint)
		if err := domain.LockCompletionUser(tx, uid); err != nil {
			return err
		}
		ids := []uint{id}
		if targetKey != "" {
			ids = append(ids, b.cfg.bindings[targetKey])
		}
		// All mutation paths use Task rows as their serialization boundary. Sort
		// pair locks to agree with single-task manual complete/undo operations.
		var locked []domain.Task
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ?", ids).Order("id ASC").Find(&locked).Error; err != nil {
			return err
		}
		if len(locked) != len(ids) {
			return fiber.NewError(404, "task not found")
		}
		state, err := scoped.state(c, key)
		if err != nil {
			return err
		}
		if *input.Completed && state.CompletionDisabled {
			return fiber.NewError(409, "task completion disabled")
		}
		var target taskState
		var sourceAlreadyComplete bool
		var reverseOwned bool
		if targetKey != "" {
			target, err = scoped.state(c, targetKey)
			if err != nil {
				return err
			}
			if *input.Completed && target.CompletionDisabled {
				return fiber.NewError(409, "task completion disabled")
			}
			row, present, readErr := scoped.record(uid, id)
			err = readErr
			if err != nil {
				return err
			}
			sourceAlreadyComplete = present
			if reverse {
				_, reverseOwned = scoped.automaticOwnership(row, present, target.TaskID, input.AssignmentScope)
			}
		}
		if targetKey != "" {
			err = domain.SetLinkedTaskCompletion(tx, uid, id, *input.Completed)
		} else if *input.Completed {
			err = domain.CompleteTask(tx, uid, id)
		} else {
			err = domain.UncompleteTask(tx, uid, id)
		}
		if err != nil {
			return err
		}
		if targetKey != "" {
			if reverse {
				if reverseOwned {
					err = domain.SetLinkedTaskCompletion(tx, uid, target.TaskID, false)
				}
			} else {
				err = scoped.linkVisit(uid, id, target.TaskID, input.AssignmentScope, *input.Completed, sourceAlreadyComplete)
			}
			if err != nil {
				return err
			}
		}
		updated, err = scoped.state(c, key)
		if err != nil {
			return err
		}
		if targetKey != "" {
			related, err := scoped.state(c, targetKey)
			if err != nil {
				return err
			}
			updated.RelatedTasks = []taskState{related}
		}
		return nil
	})
	if err != nil {
		return bridgeError(c, err)
	}
	return c.JSON(updated)
}
