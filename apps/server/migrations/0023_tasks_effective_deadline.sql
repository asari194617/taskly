-- Add generated effective_deadline column for faster filtering/ordering with index support
ALTER TABLE tasks
  ADD COLUMN effective_deadline DATE GENERATED ALWAYS AS (deadline) STORED;

ALTER TABLE tasks
  ADD INDEX idx_tasks_effective_deadline_id (effective_deadline, id);