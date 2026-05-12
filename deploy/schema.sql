-- BDopsFlow 数据库初始化脚本
-- rqlite 分布式数据库

-- 1. 用户表
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,
    email TEXT,
    domain_id INTEGER,
    role TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 2. 领域表
CREATE TABLE IF NOT EXISTS domains (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 3. 工作流表
CREATE TABLE IF NOT EXISTS workflows (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT,
    domain_id INTEGER NOT NULL,
    dag_config TEXT,
    cron_expression TEXT,
    is_enabled BOOLEAN DEFAULT 1,
    created_by INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (domain_id) REFERENCES domains(id)
);

-- 4. 任务表
CREATE TABLE IF NOT EXISTS tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_id INTEGER,
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
    created_by INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (workflow_id) REFERENCES workflows(id),
    FOREIGN KEY (domain_id) REFERENCES domains(id)
);

-- 5. 任务执行记录表
CREATE TABLE IF NOT EXISTS task_executions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    execution_id TEXT NOT NULL UNIQUE,
    executor_id TEXT,
    status TEXT NOT NULL,
    start_time DATETIME,
    end_time DATETIME,
    output TEXT,
    error TEXT,
    retry_times INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES tasks(id)
);

-- 6. 执行器节点表
CREATE TABLE IF NOT EXISTS executors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    executor_id TEXT NOT NULL UNIQUE,
    name TEXT,
    address TEXT NOT NULL,
    status TEXT DEFAULT 'online',
    last_heartbeat DATETIME,
    capacity INTEGER DEFAULT 10,
    current_load INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 7. 工作流执行记录表
CREATE TABLE IF NOT EXISTS workflow_executions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_id INTEGER NOT NULL,
    execution_id TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL,
    start_time DATETIME,
    end_time DATETIME,
    node_states TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (workflow_id) REFERENCES workflows(id)
);

-- 8. 任务依赖表（血缘关系）
CREATE TABLE IF NOT EXISTS task_dependencies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    parent_task_id INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES tasks(id),
    FOREIGN KEY (parent_task_id) REFERENCES tasks(id),
    UNIQUE(task_id, parent_task_id)
);

-- 9. 任务执行日志表（用于存储详细日志）
CREATE TABLE IF NOT EXISTS task_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    execution_id TEXT NOT NULL,
    task_id INTEGER NOT NULL,
    executor_id TEXT,
    node_id TEXT,
    log_level TEXT DEFAULT 'info',
    message TEXT NOT NULL,
    log_time DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES tasks(id)
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_workflow_executions_workflow_id ON workflow_executions(workflow_id);
CREATE INDEX IF NOT EXISTS idx_workflow_executions_execution_id ON workflow_executions(execution_id);
CREATE INDEX IF NOT EXISTS idx_task_logs_execution_id ON task_logs(execution_id);
CREATE INDEX IF NOT EXISTS idx_task_logs_task_id ON task_logs(task_id);
CREATE INDEX IF NOT EXISTS idx_task_logs_executor_id ON task_logs(executor_id);

-- 插入默认数据
-- 默认领域
INSERT OR IGNORE INTO domains (name, description) VALUES ('default', '默认领域');

-- 默认管理员用户 (密码: admin123, bcrypt hash)
INSERT OR IGNORE INTO users (username, password, email, domain_id, role) 
VALUES ('admin', '$2a$10$V4DeC68lOaLwF6N1pAVR8ux7WzY9NOeuPgwrAkyF9XcpWOL9muEaG', 'admin@example.com', 1, 'admin');
