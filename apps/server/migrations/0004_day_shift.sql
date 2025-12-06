-- Day shift core tables
CREATE TABLE IF NOT EXISTS day_shift_cycles (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  begin_date DATE NOT NULL,
  finish_date DATE NOT NULL,
  description VARCHAR(255) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS day_shift_assignments (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  date DATE NOT NULL,
  cycle_id BIGINT NOT NULL,
  status ENUM('assigned','completed','unfulfilled') NOT NULL DEFAULT 'assigned',
  notes TEXT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  FOREIGN KEY (cycle_id) REFERENCES day_shift_cycles(id),
  UNIQUE KEY uniq_day_cycle (date, cycle_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS day_shift_assignment_users (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  day_shift_assignments_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  FOREIGN KEY (day_shift_assignments_id) REFERENCES day_shift_assignments(id),
  FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
