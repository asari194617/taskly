package jobs

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TeamsInbound struct {
	Title          string    `json:"title"`
	Description    *string   `json:"description"`
	AddedUserID    uint      `json:"added_user_id"`
	SubjectID      uint      `json:"subject_id"`
	Deadline       time.Time `json:"deadline"`
	StatusCallback *string   `json:"status_callback"`
}

type TaskNotice struct {
	Task      domain.Task `json:"task"`
	Action    string      `json:"action"`
	Actor     string      `json:"actor"`
	MessageID string      `json:"message_id,omitempty"`
	Message   string      `json:"message,omitempty"`
}

type Callback struct {
	TaskID uint   `json:"task_id"`
	URL    string `json:"url"`
}

type CallbackPolicy struct {
	Validate func(string) error
	Client   *http.Client
}

var ErrTeamsMessagePending = errors.New("Teams message ID is pending")

func shouldDeferTeamsReply(db *gorm.DB, job Job) (bool, error) {
	if job.Kind != "task_email" {
		return false, nil
	}
	var notice TaskNotice
	if err := json.Unmarshal([]byte(job.Payload), &notice); err != nil {
		return false, nil // Deliver records malformed payloads as ordinary failures.
	}
	if (notice.Action != "updated" && notice.Action != "deleted") || notice.Task.ID == 0 {
		return false, nil
	}
	// Multiple workers can claim different jobs concurrently. A later reply
	// waits until every earlier email for this task has finished.
	var earlier Job
	result := db.Model(&Job{}).Select("id").Where(
		"kind = ? AND id < ? AND status IN ? AND JSON_EXTRACT(payload, '$.task.ID') = ?",
		"task_email", job.ID, []string{"pending", "processing"}, notice.Task.ID,
	).Limit(1).Find(&earlier)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected != 0 {
		return true, nil
	}
	if notice.MessageID != "" {
		return false, nil
	}
	messageID, err := teamsMessageID(db, notice.Task.ID)
	return messageID == "", err
}

func Deliver(db *gorm.DB, job Job, policy CallbackPolicy) error {
	switch job.Kind {
	case "teams_inbound":
		var payload TeamsInbound
		if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
			return err
		}
		return deliverTeamsInbound(db, job.ID, job.CreatedAt, payload)
	case "teams_callback":
		var payload Callback
		if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
			return err
		}
		return deliverCallback(db, job.ID, payload, policy)
	case "task_discord", "task_email":
		var payload TaskNotice
		if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
			return err
		}
		if job.Kind == "task_discord" {
			return utils.SendDiscordTaskEmbedResult(payload.Action, payload.Task, payload.Actor)
		}
		if payload.Action == "created" {
			return utils.SendTeamsNotifyEmailForTaskSync(payload.Task, db)
		}
		messageID := payload.MessageID
		if messageID == "" {
			var err error
			messageID, err = teamsMessageID(db, payload.Task.ID)
			if err != nil {
				return err
			}
			if messageID == "" {
				return ErrTeamsMessagePending
			}
		}
		return utils.SendTeamsReplyEmailSync(payload.Task.ID, messageID, payload.Message)
	default:
		return fmt.Errorf("unknown job kind %q", job.Kind)
	}
}

func deliverTeamsInbound(db *gorm.DB, jobID uint64, acceptedAt time.Time, payload TeamsInbound) error {
	return Transaction(db, func(tx *gorm.DB) error {
		// A retry after a crash must not create a second task. This lookup and
		// the task insert are protected by the job row lock.
		var job Job
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&job, jobID).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Table("tasks_teams_logs").Where("inbound_job_id = ?", jobID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		deadline := payload.Deadline
		if deadline.IsZero() {
			deadline = acceptedAt
		}
		task := domain.Task{AddedUserID: payload.AddedUserID, Title: payload.Title, Description: payload.Description, SubjectID: payload.SubjectID,
			Deadline: deadline, DeadlineSince: "NONE", DeadlineUntil: "NONE", Type: "OTHER"}
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		if err := tx.Table("tasks_teams_logs").Create(map[string]any{
			"task_id": task.ID, "inbound_job_id": jobID, "is_success": true,
			"callback_url": payload.StatusCallback, "attempts": 0, "last_status": "created",
		}).Error; err != nil {
			return err
		}
		if payload.StatusCallback != nil && *payload.StatusCallback != "" {
			_, err := Enqueue(tx, "teams_callback", Callback{TaskID: task.ID, URL: *payload.StatusCallback}, nil)
			return err
		}
		return nil
	})
}

func deliverCallback(db *gorm.DB, jobID uint64, payload Callback, policy CallbackPolicy) error {
	if policy.Validate == nil || policy.Client == nil {
		return fmt.Errorf("callback policy is not configured")
	}
	if err := policy.Validate(payload.URL); err != nil {
		return err
	}
	body := fmt.Sprintf(`{"is_success":true,"task_id":%d}`, payload.TaskID)
	req, err := http.NewRequest(http.MethodPost, payload.URL, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", fmt.Sprintf("taskly-callback-%d", jobID))
	if secret := os.Getenv("TEAMS_INTEGRATION_SECRET"); secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(body))
		req.Header.Set("X-Signature", fmt.Sprintf("sha256=%x", mac.Sum(nil)))
	}
	resp, err := policy.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	response, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	status := "callback_sent"
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		status = "callback_error"
	}
	if err := db.Table("tasks_teams_logs").Where("task_id = ?", payload.TaskID).Updates(map[string]any{
		"last_status": status, "last_response_status": resp.StatusCode, "last_response_body": string(response),
	}).Error; err != nil {
		return err
	}
	if status == "callback_error" {
		return fmt.Errorf("callback http %d", resp.StatusCode)
	}
	return nil
}

// EnqueueTaskNotices must run in the same transaction as the task mutation.
func EnqueueTaskNotices(tx *gorm.DB, action string, task domain.Task, actor, messageID, message string) error {
	if strings.TrimSpace(os.Getenv("DISCORD_WEBHOOK_URL")) != "" {
		if _, err := Enqueue(tx, "task_discord", TaskNotice{Task: task, Action: action, Actor: actor}, nil); err != nil {
			return err
		}
	}
	if strings.TrimSpace(os.Getenv("TEAMS_NOTIFY_EMAIL")) != "" && (action == "created" || messageID != "" || message != "") {
		_, err := Enqueue(tx, "task_email", TaskNotice{Task: task, Action: action, MessageID: messageID, Message: message}, nil)
		return err
	}
	return nil
}
