ALTER TABLE tasks
  ADD COLUMN image_key VARCHAR(128) NULL AFTER description,
  ADD UNIQUE INDEX idx_tasks_image_key (image_key);
