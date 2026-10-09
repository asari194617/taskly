-- Declarative Taskly application schema for MySQL 8.4.
-- This file is the source of truth applied by Atlas. It contains no row data.

SET FOREIGN_KEY_CHECKS=0;

CREATE TABLE `api_action_logs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned DEFAULT NULL,
  `hashed_token` varchar(255) NOT NULL,
  `action` varchar(64) NOT NULL,
  `ip` varchar(64) NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_api_action_logs_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `api_keys` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `hashed_token` varchar(255) NOT NULL,
  `description` varchar(255) DEFAULT NULL,
  `is_valid` tinyint(1) DEFAULT '1',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `expires_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_api_keys_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `blocks` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `reason` longtext,
  `expires_at` datetime(3) DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_blocks_user_id` (`user_id`),
  KEY `idx_blocks_user_id_expires_at` (`user_id`,`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `configs` (
  `key` varchar(128) NOT NULL,
  `value` text,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `day_shift_assignment_users` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `day_shift_assignments_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_asg_id` (`day_shift_assignments_id`),
  KEY `idx_user_id` (`user_id`),
  CONSTRAINT `fk_asg_users_asg` FOREIGN KEY (`day_shift_assignments_id`) REFERENCES `day_shift_assignments` (`id`),
  CONSTRAINT `fk_asg_users_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `day_shift_assignments` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `date` datetime(3) DEFAULT NULL,
  `cycle_id` bigint unsigned DEFAULT NULL,
  `status` varchar(16) DEFAULT NULL,
  `notes` longtext,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `day_shift_cycle_group_members` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `group_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `position` bigint DEFAULT '0',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_day_shift_cycle_group_members_group_id` (`group_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `day_shift_cycle_groups` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `cycle_id` bigint unsigned NOT NULL,
  `position` bigint DEFAULT '0',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_day_shift_cycle_groups_cycle_id` (`cycle_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `day_shift_cycles` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `begin_date` datetime(3) DEFAULT NULL,
  `finish_date` datetime(3) DEFAULT NULL,
  `description` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `email_suppressions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `email` varchar(255) NOT NULL,
  `reason` varchar(255) DEFAULT NULL,
  `suspended_until` datetime(3) NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_email_suppressions_email` (`email`),
  KEY `idx_email_suppressions_until` (`suspended_until`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `flagged_contents` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `task_id` bigint unsigned NOT NULL,
  `category` varchar(64) NOT NULL,
  `severity` varchar(16) NOT NULL,
  `score` double DEFAULT NULL,
  `threat` tinyint(1) NOT NULL DEFAULT '0',
  `reason` text,
  `model` varchar(64) DEFAULT NULL,
  `metadata` json DEFAULT NULL,
  `snapshot` json DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_flagged_contents_task_id` (`task_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `ical_keys` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `token` varchar(255) NOT NULL,
  `description` varchar(255) DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `expires_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_ical_keys_token` (`token`),
  KEY `idx_ical_keys_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `integration_jobs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `kind` varchar(32) NOT NULL,
  `payload` json NOT NULL,
  `status` varchar(16) NOT NULL DEFAULT 'pending',
  `attempts` int NOT NULL DEFAULT '0',
  `available_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `lease_until` datetime(6) DEFAULT NULL,
  `dedup_key` varchar(255) DEFAULT NULL,
  `last_error` text,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_integration_dedup` (`dedup_key`),
  KEY `idx_integration_claim` (`status`,`available_at`,`lease_until`),
  KEY `idx_integration_kind_claim` (`kind`,`status`,`available_at`,`lease_until`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `login_codes` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `email` varchar(255) NOT NULL,
  `hashed_code` varchar(255) NOT NULL,
  `expires_at` datetime(3) NOT NULL,
  `consumed_at` datetime(3) DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `failed_attempts` bigint NOT NULL DEFAULT '0',
  `locked_until` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_login_codes_email` (`email`),
  KEY `idx_login_codes_expires_at` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `logs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned DEFAULT NULL,
  `action` varchar(64) NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_logs_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `mcps` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned DEFAULT NULL,
  `hashed_token` varchar(255) DEFAULT NULL,
  `description` longtext,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `expires_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_mcps_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `migrations` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL,
  `applied_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_migrations_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `notification_deliveries` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `delivery_key` varchar(191) NOT NULL,
  `channel` varchar(32) NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `status` enum('sending','sent','error') NOT NULL,
  `attempts` int unsigned NOT NULL DEFAULT '0',
  `error_message` text,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `sent_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_delivery_key` (`delivery_key`),
  KEY `idx_delivery_status` (`status`,`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `notification_devices` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `name` varchar(255) DEFAULT NULL,
  `platform` enum('EMAIL','WEBPUSH','IOS','ANDROID') CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL,
  `payload` json DEFAULT NULL,
  `is_enabled` tinyint(1) NOT NULL DEFAULT '1',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_notification_devices_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `notification_type_prefs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `notification_type` enum('TASK','TASK_MORNING','TASK_EVENING','DAYSHIFT') NOT NULL,
  `is_enabled` tinyint(1) NOT NULL DEFAULT '1',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_notification_type_prefs_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `notifications_queue` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `kind` varchar(32) NOT NULL,
  `when_label` varchar(32) NOT NULL,
  `subject` varchar(255) NOT NULL,
  `title` text,
  `body` text,
  `user_id` bigint unsigned DEFAULT NULL,
  `channel` varchar(32) DEFAULT NULL,
  `status` varchar(32) DEFAULT NULL,
  `error_message` text,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `processed_at` datetime DEFAULT NULL,
  `event_key` varchar(191) DEFAULT NULL,
  `slot` varchar(16) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_notification_event` (`event_key`),
  KEY `idx_processed_at` (`processed_at`),
  KEY `idx_notification_pending` (`channel`,`status`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `passkeys` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `credential_id` varchar(255) NOT NULL,
  `public_key` text NOT NULL,
  `sign_count` bigint NOT NULL,
  `device_name` varchar(255) DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_passkeys_credential_id` (`credential_id`),
  KEY `idx_passkeys_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `permissions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `target` varchar(64) NOT NULL,
  `method` varchar(16) NOT NULL,
  `value` varchar(255) DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_permissions_user_id` (`user_id`),
  KEY `idx_permissions_user_target_method` (`user_id`,`target`,`method`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `recovery_codes` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `hashed_code` varchar(255) NOT NULL,
  `is_used` tinyint(1) DEFAULT '0',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_recovery_codes_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `role_users` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `role` enum('admin','manager') CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_role_users_user_id` (`user_id`),
  KEY `idx_role_users_user_id_role` (`user_id`,`role`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `sessions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `client_platform` varchar(64) NOT NULL,
  `hashed_token` varchar(255) NOT NULL,
  `expires_at` datetime(3) DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_sessions_user_id` (`user_id`),
  KEY `idx_sessions_user` (`user_id`),
  KEY `idx_sessions_hashed_token` (`hashed_token`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `subjects` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name_ja` varchar(255) NOT NULL,
  `name_en` varchar(255) DEFAULT NULL,
  `expires_at` datetime(3) DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_subjects_expires_at_id` (`expires_at`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `tasks` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `added_user_id` bigint unsigned NOT NULL,
  `title` varchar(255) NOT NULL,
  `subject_id` bigint unsigned NOT NULL,
  `description` text,
  `image_key` varchar(128) DEFAULT NULL,
  `type` enum('ONLINE','FACE_TO_FACE','QUIZ','PRESENTATION','SELF_STUDY','OTHER') NOT NULL DEFAULT 'OTHER',
  `is_require_hand_in` tinyint(1) NOT NULL DEFAULT '0',
  `is_self_check_required` tinyint(1) NOT NULL DEFAULT '0',
  `deadline` date NOT NULL,
  `deadline_since` enum('NONE','P1','P2','P3','P4','P5','P6','P7','P8','P9','P10') NOT NULL DEFAULT 'NONE',
  `deadline_until` enum('NONE','MORNING','LUNCH','AFTER_SCHOOL','P1','P2','P3','P4','P5','P6','P7','P8','P9','P10') NOT NULL DEFAULT 'NONE',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `is_hidden` tinyint(1) NOT NULL DEFAULT '0',
  `effective_deadline` date GENERATED ALWAYS AS (`deadline`) STORED,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_tasks_image_key` (`image_key`),
  KEY `idx_tasks_subject_id` (`subject_id`),
  KEY `idx_tasks_added_user_id` (`added_user_id`),
  KEY `idx_tasks_deadline_id` (`deadline`,`id`),
  KEY `idx_tasks_effective_deadline_id` (`effective_deadline`,`id`),
  FULLTEXT KEY `ft_tasks_title_desc` (`title`,`description`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `tasks_teams_logs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `task_id` bigint unsigned NOT NULL,
  `teams_message_id` varchar(255) DEFAULT NULL,
  `teams_conversation_id` varchar(255) DEFAULT NULL,
  `is_success` tinyint(1) NOT NULL DEFAULT '0',
  `callback_url` text,
  `attempts` int NOT NULL DEFAULT '0',
  `last_status` varchar(32) DEFAULT NULL,
  `last_error` text,
  `last_response_status` int DEFAULT NULL,
  `last_response_body` text,
  `inbound_job_id` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_teams_inbound_job` (`inbound_job_id`),
  KEY `idx_tasks_teams_logs_task_id` (`task_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `tasks_users_completions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `task_id` bigint NOT NULL,
  `completed_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_user_task` (`user_id`,`task_id`),
  KEY `idx_task_user` (`task_id`,`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE `totp_secrets` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `secret` varchar(255) NOT NULL,
  `is_active` tinyint(1) DEFAULT '0',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_totp_secrets_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `user_reports` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `reporter_user_id` bigint unsigned DEFAULT NULL,
  `task_id` bigint unsigned NOT NULL,
  `reason` varchar(255) NOT NULL,
  `comment` text,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_user_reports_reporter_user_id` (`reporter_user_id`),
  KEY `idx_user_reports_task_id` (`task_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `users` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `email` varchar(255) NOT NULL,
  `display_name` varchar(255) DEFAULT NULL,
  `hashed_password` varchar(255) DEFAULT NULL,
  `email_code_allowed` tinyint(1) DEFAULT '1',
  `is_deleted` tinyint(1) DEFAULT '0',
  `notification_lang` varchar(5) NOT NULL DEFAULT 'ja',
  `is_registered` tinyint(1) DEFAULT '0',
  `is_agreed_policies` tinyint(1) DEFAULT '0',
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `display_lang` varchar(5) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT 'ja',
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_users_email` (`email`),
  KEY `idx_users_erasure_due` (`is_deleted`,`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `web_push_subscriptions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `endpoint` varchar(1024) NOT NULL,
  `p256dh` varchar(1024) NOT NULL,
  `auth` varchar(255) NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_web_push_subscriptions_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `web_push_vapid` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_key` varchar(255) NOT NULL,
  `private_key` varchar(255) NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `account_erasure_holds` (
  `user_id` bigint unsigned NOT NULL,
  `reason` varchar(255) NOT NULL,
  `review_at` datetime NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`user_id`),
  CONSTRAINT `account_erasure_holds_ibfk_1` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE `policy_consents` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `terms_version` varchar(32) NOT NULL,
  `privacy_version` varchar(32) NOT NULL,
  `security_version` varchar(32) NOT NULL,
  `display_language` varchar(5) NOT NULL,
  `accepted_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_policy_consents_user_time` (`user_id`,`accepted_at`),
  CONSTRAINT `fk_policy_consents_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

SET FOREIGN_KEY_CHECKS=1;
