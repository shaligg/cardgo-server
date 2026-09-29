-- LoginServer 账号域初始化，MySQL 8.0+。
-- 在目标账号库中独立执行；仅用于尚未创建以下两表的环境，不迁移已有表或数据。
-- 字段对应 internal/repo/model/account.go；时间戳由应用写入，精度为毫秒。

CREATE TABLE `accounts` (
  `uid` VARCHAR(64) NOT NULL,
  `status` VARCHAR(16) NOT NULL,
  `token_hash` CHAR(64) NOT NULL,
  `expires_at` DATETIME(3) DEFAULT NULL,
  `device_id` VARBINARY(128) NOT NULL COMMENT '最近一次设备，仅记录，允许重复',
  `guest_device_id` VARBINARY(128) DEFAULT NULL COMMENT '游客恢复键，绑定后清空为 NULL',
  `created_at` DATETIME(3) DEFAULT NULL,
  `updated_at` DATETIME(3) DEFAULT NULL,
  PRIMARY KEY (`uid`),
  UNIQUE KEY `idx_accounts_guest_device_id` (`guest_device_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `account_identities` (
  `provider` VARCHAR(32) NOT NULL,
  `subject` VARCHAR(64) NOT NULL,
  `uid` VARCHAR(64) NOT NULL,
  `password_hash` VARCHAR(128) NOT NULL,
  `created_at` DATETIME(3) DEFAULT NULL,
  PRIMARY KEY (`provider`, `subject`),
  KEY `idx_account_identities_uid` (`uid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
