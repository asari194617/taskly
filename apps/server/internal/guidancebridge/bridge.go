package guidancebridge

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/tasklist"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type bridge struct {
	db        *gorm.DB
	cfg       config
	configErr error
}

type taskState struct {
	Key                string `json:"key"`
	TaskID             uint   `json:"task_id"`
	Completed          bool   `json:"completed"`
	CompletionDisabled bool   `json:"completion_disabled"`
	TasklyURL          string `json:"taskly_url"`
}

// Register accepts only the dedicated guidance client's live OAuth access tokens.
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
	var count int64
	if err := b.db.Table("tasks_users_completions").Where("user_id = ? AND task_id = ?", uid, id).Count(&count).Error; err != nil {
		return taskState{}, fiber.NewError(503, "completion unavailable")
	}
	return taskState{Key: key, TaskID: id, Completed: !task.IsCompletionDisabled && count > 0, CompletionDisabled: task.IsCompletionDisabled, TasklyURL: b.cfg.webURL + "/"}, nil
}

func (b *bridge) list(c *fiber.Ctx) error {
	states := make([]taskState, 0, len(b.cfg.bindings))
	for _, key := range b.cfg.keys() {
		state, err := b.state(c, key)
		if err != nil {
			return err
		}
		states = append(states, state)
	}
	return c.JSON(fiber.Map{"event_id": b.cfg.eventID, "tasks": states})
}

func (b *bridge) put(c *fiber.Ctx) error {
	var input struct {
		Completed *bool `json:"completed"`
	}
	decoder := json.NewDecoder(bytes.NewReader(c.Body()))
	decoder.DisallowUnknownFields()
	if len(c.Body()) > 1024 || decoder.Decode(&input) != nil || input.Completed == nil || decoder.Decode(new(any)) != io.EOF {
		return fail(c, 400, "completed boolean required")
	}
	state, err := b.state(c, c.Params("key"))
	if err != nil {
		return err
	}
	if *input.Completed && state.CompletionDisabled {
		return fail(c, 409, "task completion disabled")
	}
	uid := c.Locals("guidance-user").(uint)
	if *input.Completed {
		err = domain.CompleteTask(b.db, uid, state.TaskID)
	} else {
		err = domain.UncompleteTask(b.db, uid, state.TaskID)
	}
	if err != nil {
		return fail(c, 503, "completion update unavailable")
	}
	state, err = b.state(c, state.Key)
	if err != nil {
		return err
	}
	return c.JSON(state)
}
