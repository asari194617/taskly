package jobs

import (
	"strings"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/taskimage"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	"gorm.io/gorm"
)

func CreateTask(db *gorm.DB, addedUserID uint, title string, subjectID uint, description *string, typ string, handIn, selfCheck bool, deadline domain.Task, actor string) (domain.Task, error) {
	keys := deadline.ImageKeys
	if len(keys) == 0 && deadline.ImageKey != nil {
		keys = []string{*deadline.ImageKey}
	}
	return CreateTaskWithImages(db, addedUserID, title, subjectID, description, typ, handIn, selfCheck, deadline, keys, actor)
}

func CreateTaskWithImages(db *gorm.DB, addedUserID uint, title string, subjectID uint, description *string, typ string, handIn, selfCheck bool, deadline domain.Task, imageKeys []string, actor string) (domain.Task, error) {
	var created domain.Task
	err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		created, err = domain.CreateTask(tx, addedUserID, title, subjectID, description, typ, handIn, selfCheck,
			deadline.Deadline, deadline.DeadlineSince, deadline.DeadlineUntil, actor)
		if err != nil {
			return err
		}
		if len(imageKeys) > 0 {
			created.ImageKey = &imageKeys[0]
			created.ImageKeys = append([]string(nil), imageKeys...)
			if err := taskimage.Replace(tx, created.ID, imageKeys); err != nil {
				return err
			}
		}
		return EnqueueTaskNotices(tx, "created", created, actor, "", "")
	})
	return created, err
}

func UpdateTask(db *gorm.DB, id uint, modifier func(*domain.Task) error, actor string) (domain.Task, domain.Task, error) {
	return UpdateTaskWithImages(db, id, modifier, nil, actor)
}

func UpdateTaskWithImages(db *gorm.DB, id uint, modifier func(*domain.Task) error, imageKeys *[]string, actor string) (domain.Task, domain.Task, error) {
	var before, after domain.Task
	err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		before, after, err = domain.UpdateTask(tx, id, modifier, actor)
		if err != nil {
			return err
		}
		if imageKeys != nil {
			if err := taskimage.Replace(tx, after.ID, *imageKeys); err != nil {
				return err
			}
			after.ImageKeys = append([]string(nil), (*imageKeys)...)
			after.ImageKey = nil
			if len(*imageKeys) > 0 {
				after.ImageKey = &(*imageKeys)[0]
			}
		}
		mid, err := teamsMessageID(tx, after.ID)
		if err != nil {
			return err
		}
		message := ""
		notify := mid != ""
		if !notify && strings.TrimSpace(utils.Env("TEAMS_NOTIFY_EMAIL", "")) != "" {
			notify, err = hasOutboundTeamsPost(tx, after.ID)
			if err != nil {
				return err
			}
		}
		if notify {
			message = utils.BuildTaskUpdateMessageJa(before, after, tx)
		}
		return EnqueueTaskNotices(tx, "updated", after, actor, mid, message)
	})
	return before, after, err
}

func DeleteTask(db *gorm.DB, id uint, actor string) (domain.Task, error) {
	var deleted domain.Task
	err := db.Transaction(func(tx *gorm.DB) error {
		mid, err := teamsMessageID(tx, id)
		if err != nil {
			return err
		}
		notify := mid != ""
		if !notify && strings.TrimSpace(utils.Env("TEAMS_NOTIFY_EMAIL", "")) != "" {
			notify, err = hasOutboundTeamsPost(tx, id)
			if err != nil {
				return err
			}
		}
		deleted, err = domain.DeleteTask(tx, id, actor)
		if err != nil {
			return err
		}
		message := ""
		if notify {
			message = utils.BuildTaskDeleteMessageJa(deleted)
		}
		return EnqueueTaskNotices(tx, "deleted", deleted, actor, mid, message)
	})
	return deleted, err
}

func teamsMessageID(tx *gorm.DB, taskID uint) (string, error) {
	var row struct{ TeamsMessageID string }
	err := tx.Table("tasks_teams_logs").Select("teams_message_id").
		Where("task_id = ? AND is_success = ? AND teams_message_id IS NOT NULL AND teams_message_id <> ''", taskID, true).
		Order("id DESC").Limit(1).Scan(&row).Error
	return row.TeamsMessageID, err
}

// The durable create job identifies tasks that have an outbound Teams post.
// Teams-originated tasks do not have one and must not wait for a message ID.
func hasOutboundTeamsPost(tx *gorm.DB, taskID uint) (bool, error) {
	var job Job
	result := tx.Model(&Job{}).Select("id").
		Where("kind = ? AND JSON_EXTRACT(payload, '$.task.ID') = ? AND JSON_EXTRACT(payload, '$.action') LIKE ?", "task_email", taskID, "%created%").
		Limit(1).Find(&job)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected != 0, nil
}
