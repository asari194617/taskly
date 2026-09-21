package tasklist

import (
	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	"gorm.io/gorm"
)

// CanAccessTask combines moderation visibility with the task permission policy.
func CanAccessTask(db *gorm.DB, userID uint, action string, task domain.Task) (bool, error) {
	admin, err := utils.UserHasRole(userID, "admin", db)
	if err != nil {
		return false, err
	}
	manager, err := utils.UserHasRole(userID, "manager", db)
	if err != nil {
		return false, err
	}
	if task.IsHidden && !admin && !manager {
		return false, nil
	}
	return utils.CheckPermission(userID, "tasks", action, &task.AddedUserID, db)
}
