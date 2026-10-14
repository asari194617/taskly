package domain

import (
	"fmt"
	"gorm.io/gorm"
)

var TaskTypes = []string{"ONLINE", "FACE_TO_FACE", "QUIZ", "PRESENTATION", "SELF_STUDY", "OTHER"}

func TaskCompletionDefaultKey(typ string) string { return "task_completion_disabled_" + typ }

// Defaults affect new tasks only; each task stores its own setting.
func TaskCompletionDefaults(db *gorm.DB) (map[string]bool, error) {
	defaults := map[string]bool{}
	keys := make([]string, 0, len(TaskTypes))
	for _, typ := range TaskTypes {
		defaults[typ] = typ == "QUIZ"
		keys = append(keys, TaskCompletionDefaultKey(typ))
	}
	var rows []struct {
		Key   string
		Value string
	}
	if err := db.Table("configs").Where("`key` IN ?", keys).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		for _, typ := range TaskTypes {
			if row.Key == TaskCompletionDefaultKey(typ) {
				switch row.Value {
				case "true":
					defaults[typ] = true
				case "false":
					defaults[typ] = false
				default:
					return nil, fmt.Errorf("invalid task completion default")
				}
			}
		}
	}
	return defaults, nil
}
