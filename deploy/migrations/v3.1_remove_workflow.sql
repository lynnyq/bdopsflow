-- BDopsFlow 迁移脚本
-- 版本：v3.1
-- 日期：2026-05-31
-- 描述：移除工作流(Workflow)模块所有相关功能

PRAGMA foreign_keys = OFF;

-- ============================================================================
-- 1. 删除工作流执行记录表
-- ============================================================================
DROP TABLE IF EXISTS bdopsflow_workflow_executions;

-- ============================================================================
-- 2. 删除工作流表
-- ============================================================================
DROP TABLE IF EXISTS bdopsflow_workflows;

-- ============================================================================
-- 3. 从任务表移除 workflow_id 字段
-- ============================================================================
-- rqlite 不支持 ALTER TABLE DROP COLUMN，需要重建表

CREATE TABLE IF NOT EXISTS bdopsflow_tasks_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    config TEXT NOT NULL,
    cron_expression TEXT,
    timeout_seconds INTEGER DEFAULT 300,
    retry_count INTEGER DEFAULT 3,
    retry_interval INTEGER DEFAULT 5,
    is_enabled BOOLEAN DEFAULT 1,
    status TEXT DEFAULT 'pending',
    domain_id INTEGER NOT NULL,
    webhook_config TEXT,
    webhook_id INTEGER,
    webhook_events TEXT DEFAULT '[]',
    assigned_executor_id INTEGER,
    created_by INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (domain_id) REFERENCES bdopsflow_domains(id) ON DELETE CASCADE,
    FOREIGN KEY (assigned_executor_id) REFERENCES bdopsflow_executors(id) ON DELETE SET NULL
);

INSERT INTO bdopsflow_tasks_new (
    id, name, type, config, cron_expression, timeout_seconds,
    retry_count, retry_interval, is_enabled, status, domain_id,
    webhook_config, webhook_id, webhook_events, assigned_executor_id,
    created_by, created_at, updated_at
)
SELECT
    id, name, type, config, cron_expression, timeout_seconds,
    retry_count, retry_interval, is_enabled, status, domain_id,
    webhook_config, webhook_id, webhook_events, assigned_executor_id,
    created_by, created_at, updated_at
FROM bdopsflow_tasks;

DROP TABLE bdopsflow_tasks;

ALTER TABLE bdopsflow_tasks_new RENAME TO bdopsflow_tasks;

-- ============================================================================
-- 4. 重建任务表索引（不含 workflow_id 索引）
-- ============================================================================
CREATE INDEX IF NOT EXISTS idx_bdopsflow_tasks_domain_id ON bdopsflow_tasks(domain_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_tasks_is_enabled ON bdopsflow_tasks(is_enabled);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_tasks_type ON bdopsflow_tasks(type);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_tasks_status ON bdopsflow_tasks(status);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_tasks_cron_enabled ON bdopsflow_tasks(is_enabled, cron_expression) WHERE cron_expression != '';
CREATE INDEX IF NOT EXISTS idx_bdopsflow_tasks_assigned_executor ON bdopsflow_tasks(assigned_executor_id);

-- ============================================================================
-- 5. 删除工作流相关权限
-- ============================================================================
DELETE FROM bdopsflow_role_permissions
WHERE permission_id IN (
    SELECT id FROM bdopsflow_permissions WHERE resource = 'workflow'
);

DELETE FROM bdopsflow_permissions WHERE resource = 'workflow';

PRAGMA foreign_keys = ON;
