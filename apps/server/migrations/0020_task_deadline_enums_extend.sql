-- Extend deadline enums: P9/P10 for since, and add MORNING/LUNCH/AFTER_SCHOOL/P9/P10 for until

ALTER TABLE tasks
  MODIFY COLUMN deadline_since ENUM('NONE','P1','P2','P3','P4','P5','P6','P7','P8','P9','P10') NOT NULL DEFAULT 'NONE';

ALTER TABLE tasks
  MODIFY COLUMN deadline_until ENUM('NONE','MORNING','LUNCH','AFTER_SCHOOL','P1','P2','P3','P4','P5','P6','P7','P8','P9','P10') NOT NULL DEFAULT 'NONE';
