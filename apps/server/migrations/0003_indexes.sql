-- Indexes for performance
ALTER TABLE tasks
  ADD INDEX idx_tasks_deadline_id (deadline, id);

-- FULLTEXT index for simple search (requires MySQL 5.6+; InnoDB supports FULLTEXT in 5.6+ / 8.0+ recommended)
ALTER TABLE tasks
  ADD FULLTEXT INDEX ft_tasks_title_desc (title, description);
