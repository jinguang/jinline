-- ============================================================
-- jinline MySQL 建库建表脚本（MySQL 8.0+）
--
-- 说明：
--   后端启动时会通过 GORM AutoMigrate 自动建表，本脚本用于
--   DBA 预先建表、审计或迁移，内容与程序自动建表的结果一致。
--
-- 用法：
--   mysql -uroot -p < schema.sql
--   或指定库执行：mysql -ujinline -p jinline < schema.sql
-- ============================================================

CREATE DATABASE IF NOT EXISTS `jinline`
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_unicode_ci;

USE `jinline`;

-- 流水线运行记录：每次触发运行写入一行
CREATE TABLE IF NOT EXISTS `pipeline_runs` (
  `id` varchar(40) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '运行 ID（UUID）',
  `pipeline_name` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '流水线名称',
  `branch` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '运行分支',
  `commit` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'commit SHA',
  `message` varchar(512) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '提交信息',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'pending/running/success/failed/canceled',
  `trigger` varchar(16) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '触发方式',
  `error` varchar(1024) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '失败原因',
  `started_at` datetime(3) DEFAULT NULL COMMENT '开始时间',
  `finished_at` datetime(3) DEFAULT NULL COMMENT '结束时间',
  `created_at` datetime(3) DEFAULT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_pipeline_runs_pipeline_name` (`pipeline_name`),
  KEY `idx_pipeline_runs_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='流水线运行记录';

-- 阶段记录：一次运行中的 checkout / 命令阶段 / 镜像阶段
CREATE TABLE IF NOT EXISTS `run_stages` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `run_id` varchar(40) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '所属运行 ID',
  `name` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '阶段名称',
  `type` varchar(16) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'git/command/docker',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'pending/running/success/failed/canceled',
  `exit_code` bigint DEFAULT NULL COMMENT '命令退出码',
  `duration_ms` bigint DEFAULT NULL COMMENT '耗时（毫秒）',
  `started_at` datetime(3) DEFAULT NULL COMMENT '开始时间',
  `finished_at` datetime(3) DEFAULT NULL COMMENT '结束时间',
  PRIMARY KEY (`id`),
  KEY `idx_run_stages_run_id` (`run_id`),
  CONSTRAINT `fk_pipeline_runs_stages` FOREIGN KEY (`run_id`) REFERENCES `pipeline_runs` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='流水线阶段记录';
