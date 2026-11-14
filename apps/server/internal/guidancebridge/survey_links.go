package guidancebridge

import "regexp"

var assignmentScopePattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (b *bridge) automaticOwnership(row completionRecord, present bool, sourceID uint, scope string) (bool, bool) {
	owned := present && row.AutoEventID != nil && *row.AutoEventID == b.cfg.eventID && row.AutoSourceTaskID != nil && *row.AutoSourceTaskID == sourceID
	return owned, owned && row.AutoAssignmentScope != nil && *row.AutoAssignmentScope == scope
}

func (b *bridge) linkVisit(uid, sourceID, targetID uint, scope string, completed, sourceAlreadyComplete bool) error {
	row, present, err := b.record(uid, targetID)
	if err != nil {
		return err
	}
	owned, sameScope := b.automaticOwnership(row, present, sourceID, scope)
	if completed {
		// Only a new survey completion may create a visit completion. A duplicate
		// true after a later manual visit undo must not restore the removed check.
		if !sourceAlreadyComplete && !present {
			return b.db.Table("tasks_users_completions").Create(map[string]any{"user_id": uid, "task_id": targetID, "auto_event_id": b.cfg.eventID, "auto_source_task_id": sourceID, "auto_assignment_scope": scope}).Error
		}
	} else if sameScope {
		return b.db.Exec("DELETE FROM tasks_users_completions WHERE user_id = ? AND task_id = ?", uid, targetID).Error
	}
	if owned && !sameScope {
		// The roster/company changed. Preserve the existing canonical mark and
		// retire its old automatic ownership instead of removing another visit.
		return b.db.Table("tasks_users_completions").Where("user_id = ? AND task_id = ?", uid, targetID).Updates(map[string]any{"auto_event_id": nil, "auto_source_task_id": nil, "auto_assignment_scope": nil}).Error
	}
	return nil
}
