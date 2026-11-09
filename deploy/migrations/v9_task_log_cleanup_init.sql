-- v9: 新增任务日志清理相关系统配置项
--
-- 说明：
--   1. 新增 task_execution.retention_days（执行记录保留天数，默认 30 天）
--   2. 新增 task_log.retention_days（过程日志保留天数，默认 30 天）
--   3. 自动清理开关：设为 0 禁用自动清理
--
-- 使用 INSERT OR IGNORE，仅对未存在的配置项进行初始化，已存在的配置值保持不变

INSERT OR IGNORE INTO bdopsflow_system_config (config_key, config_value, description, updated_at) VALUES
('task_execution.retention_days', '30', '执行记录保留天数，超过此天数的记录将被自动清理（同时级联删除关联的过程日志）。设为 0 禁用自动清理。', datetime('now')),
('task_log.retention_days', '30', '过程日志保留天数，超过此天数的日志将被自动清理。设为 0 禁用自动清理。', datetime('now'));
