package taskimage

import "gorm.io/gorm"

const MaxTaskImages = 5

type record struct {
	TaskID   uint
	ImageKey string
	Position uint
}

func KeysForTask(db *gorm.DB, taskID uint) ([]string, error) {
	byTask, err := KeysForTasks(db, []uint{taskID})
	if err != nil {
		return nil, err
	}
	return byTask[taskID], nil
}

func KeysForTasks(db *gorm.DB, taskIDs []uint) (map[uint][]string, error) {
	result := make(map[uint][]string, len(taskIDs))
	if len(taskIDs) == 0 {
		return result, nil
	}
	var rows []record
	if err := db.Table("task_images").Select("task_id, image_key, position").
		Where("task_id IN ?", taskIDs).Order("task_id ASC, position ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.TaskID] = append(result[row.TaskID], row.ImageKey)
	}
	return result, nil
}

// Replace stores the complete ordered image set and keeps tasks.image_key as a
// backwards-compatible pointer to the first image.
func Replace(db *gorm.DB, taskID uint, keys []string) error {
	if len(keys) > MaxTaskImages {
		return gorm.ErrInvalidData
	}
	if err := db.Exec("DELETE FROM task_images WHERE task_id = ?", taskID).Error; err != nil {
		return err
	}
	for position, key := range keys {
		if err := db.Exec("INSERT INTO task_images (task_id, image_key, position) VALUES (?, ?, ?)", taskID, key, position).Error; err != nil {
			return err
		}
	}
	var first any
	if len(keys) > 0 {
		first = keys[0]
	}
	return db.Exec("UPDATE tasks SET image_key = ? WHERE id = ?", first, taskID).Error
}
