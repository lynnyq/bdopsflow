-- BDopsFlow 迁移脚本
-- 版本：v6
-- 日期：2026-07-06
-- 描述：审计日志表增加 response_code 列与 username 索引

-- 1. 新增 response_code 列（IF NOT EXISTS 等效：用 try-catch 风格，rqlite 不支持 IF NOT EXISTS for ADD COLUMN）
-- SQLite 支持 ALTER TABLE ADD COLUMN，重复执行会报错，需通过 PRAGMA table_info 检查
-- 这里使用 INSERT OR IGNORE 模式不可行，直接执行 ALTER TABLE，如已存在会报错可忽略

ALTER TABLE bdopsflow_audit_logs ADD COLUMN response_code INTEGER DEFAULT 0;

-- 2. 新增 username 索引（IF NOT EXISTS 幂等）
CREATE INDEX IF NOT EXISTS idx_audit_logs_username ON bdopsflow_audit_logs(username);
