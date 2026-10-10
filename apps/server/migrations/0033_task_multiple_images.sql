CREATE TABLE task_images (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  task_id BIGINT UNSIGNED NOT NULL,
  image_key VARCHAR(128) NOT NULL,
  position SMALLINT UNSIGNED NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE INDEX idx_task_images_image_key (image_key),
  UNIQUE INDEX idx_task_images_task_position (task_id, position),
  INDEX idx_task_images_task_id (task_id),
  CONSTRAINT fk_task_images_task FOREIGN KEY (task_id) REFERENCES tasks (id) ON DELETE CASCADE
);

INSERT INTO task_images (task_id, image_key, position)
SELECT id, image_key, 0
FROM tasks
WHERE image_key IS NOT NULL AND image_key <> '';
