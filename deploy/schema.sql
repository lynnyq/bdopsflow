-- BDopsFlow 数据库初始化脚本
-- rqlite 分布式数据库
-- 版本：v2.2
-- 日期：2026-05-17
-- 描述：执行器全面重构，统一使用数据库ID，移除executor_id字段

-- 启用外键约束
PRAGMA foreign_keys = ON;

-- ============================================================================
-- 第一部分：基础功能表
-- ============================================================================

-- 1. 领域表
CREATE TABLE IF NOT EXISTS bdopsflow_domains (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_domains_name ON bdopsflow_domains(name);

-- 2. 用户表
CREATE TABLE IF NOT EXISTS bdopsflow_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,
    email TEXT,
    domain_id INTEGER,
    role TEXT NOT NULL,
    is_active BOOLEAN DEFAULT 1,
    last_login_at DATETIME,
    created_by INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (domain_id) REFERENCES bdopsflow_domains(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_users_username ON bdopsflow_users(username);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_users_domain_id ON bdopsflow_users(domain_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_users_role ON bdopsflow_users(role);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_users_is_active ON bdopsflow_users(is_active);

-- 3. 工作流表
CREATE TABLE IF NOT EXISTS bdopsflow_workflows (
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
    FOREIGN KEY (domain_id) REFERENCES bdopsflow_domains(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_workflows_name ON bdopsflow_workflows(name);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_workflows_domain_id ON bdopsflow_workflows(domain_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_workflows_is_enabled ON bdopsflow_workflows(is_enabled);

-- 4. 任务表
CREATE TABLE IF NOT EXISTS bdopsflow_tasks (
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
    assigned_executor_id INTEGER,
    created_by INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (workflow_id) REFERENCES bdopsflow_workflows(id) ON DELETE CASCADE,
    FOREIGN KEY (domain_id) REFERENCES bdopsflow_domains(id) ON DELETE CASCADE,
    FOREIGN KEY (assigned_executor_id) REFERENCES bdopsflow_executors(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_tasks_workflow_id ON bdopsflow_tasks(workflow_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_tasks_domain_id ON bdopsflow_tasks(domain_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_tasks_is_enabled ON bdopsflow_tasks(is_enabled);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_tasks_type ON bdopsflow_tasks(type);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_tasks_status ON bdopsflow_tasks(status);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_tasks_cron_enabled ON bdopsflow_tasks(is_enabled, cron_expression) WHERE cron_expression != '';
CREATE INDEX IF NOT EXISTS idx_bdopsflow_tasks_assigned_executor ON bdopsflow_tasks(assigned_executor_id);

-- 5. 任务执行记录表
CREATE TABLE IF NOT EXISTS bdopsflow_task_executions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    execution_id TEXT NOT NULL UNIQUE,
    executor_id INTEGER,
    status TEXT NOT NULL,
    start_time DATETIME,
    end_time DATETIME,
    output TEXT,
    error TEXT,
    retry_times INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    progress INTEGER DEFAULT 0,
    progress_msg TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES bdopsflow_tasks(id) ON DELETE CASCADE,
    FOREIGN KEY (executor_id) REFERENCES bdopsflow_executors(id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bdopsflow_task_executions_execution_id ON bdopsflow_task_executions(execution_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_task_executions_task_id ON bdopsflow_task_executions(task_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_task_executions_executor_id ON bdopsflow_task_executions(executor_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_task_executions_status ON bdopsflow_task_executions(status);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_task_executions_created_at ON bdopsflow_task_executions(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_task_executions_status_time ON bdopsflow_task_executions(status, created_at);

-- 6. 执行器节点表 (使用 name 作为唯一标识)
CREATE TABLE IF NOT EXISTS bdopsflow_executors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    address TEXT NOT NULL,
    status TEXT DEFAULT 'online',
    last_heartbeat DATETIME,
    capacity INTEGER DEFAULT 10,
    current_load INTEGER DEFAULT 0,
    is_global BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bdopsflow_executors_name ON bdopsflow_executors(name);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_executors_status ON bdopsflow_executors(status);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_executors_last_heartbeat ON bdopsflow_executors(last_heartbeat);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_executors_status_heartbeat ON bdopsflow_executors(status, last_heartbeat);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_executors_is_global ON bdopsflow_executors(is_global);

-- 7. 工作流执行记录表
CREATE TABLE IF NOT EXISTS bdopsflow_workflow_executions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_id INTEGER NOT NULL,
    execution_id TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL,
    start_time DATETIME,
    end_time DATETIME,
    node_states TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (workflow_id) REFERENCES bdopsflow_workflows(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bdopsflow_workflow_executions_execution_id ON bdopsflow_workflow_executions(execution_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_workflow_executions_workflow_id ON bdopsflow_workflow_executions(workflow_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_workflow_executions_status ON bdopsflow_workflow_executions(status);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_workflow_executions_created_at ON bdopsflow_workflow_executions(created_at DESC);

-- 8. 任务依赖表（血缘关系）
CREATE TABLE IF NOT EXISTS bdopsflow_task_dependencies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    parent_task_id INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES bdopsflow_tasks(id) ON DELETE CASCADE,
    FOREIGN KEY (parent_task_id) REFERENCES bdopsflow_tasks(id) ON DELETE CASCADE,
    UNIQUE(task_id, parent_task_id)
);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_task_dependencies_task_id ON bdopsflow_task_dependencies(task_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_task_dependencies_parent_id ON bdopsflow_task_dependencies(parent_task_id);

-- 9. 任务执行日志表
CREATE TABLE IF NOT EXISTS bdopsflow_task_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    execution_id TEXT NOT NULL,
    task_id INTEGER NOT NULL,
    executor_id INTEGER,
    node_id TEXT,
    log_level TEXT DEFAULT 'info',
    message TEXT NOT NULL,
    log_time DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES bdopsflow_tasks(id) ON DELETE CASCADE,
    FOREIGN KEY (executor_id) REFERENCES bdopsflow_executors(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_task_logs_execution_id ON bdopsflow_task_logs(execution_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_task_logs_task_id ON bdopsflow_task_logs(task_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_task_logs_executor_id ON bdopsflow_task_logs(executor_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_task_logs_log_time ON bdopsflow_task_logs(log_time DESC);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_task_logs_level ON bdopsflow_task_logs(log_level);

-- ============================================================================
-- 第二部分：权限管理系统表
-- ============================================================================

-- 10. 角色表
CREATE TABLE IF NOT EXISTS bdopsflow_roles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    code TEXT NOT NULL,
    description TEXT,
    is_system BOOLEAN DEFAULT 0,
    domain_id INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (domain_id) REFERENCES bdopsflow_domains(id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bdopsflow_roles_code ON bdopsflow_roles(code);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_roles_domain_id ON bdopsflow_roles(domain_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_roles_is_system ON bdopsflow_roles(is_system);

-- 11. 权限表
CREATE TABLE IF NOT EXISTS bdopsflow_permissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    resource TEXT NOT NULL,
    action TEXT NOT NULL,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(resource, action)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bdopsflow_permissions_resource_action ON bdopsflow_permissions(resource, action);

-- 12. 角色权限映射表
CREATE TABLE IF NOT EXISTS bdopsflow_role_permissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    role_id INTEGER NOT NULL,
    permission_id INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (role_id) REFERENCES bdopsflow_roles(id) ON DELETE CASCADE,
    FOREIGN KEY (permission_id) REFERENCES bdopsflow_permissions(id) ON DELETE CASCADE,
    UNIQUE(role_id, permission_id)
);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_role_permissions_role_id ON bdopsflow_role_permissions(role_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_role_permissions_permission_id ON bdopsflow_role_permissions(permission_id);

-- 13. 用户角色映射表
CREATE TABLE IF NOT EXISTS bdopsflow_user_roles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    role_id INTEGER NOT NULL,
    domain_id INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES bdopsflow_users(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES bdopsflow_roles(id) ON DELETE CASCADE,
    FOREIGN KEY (domain_id) REFERENCES bdopsflow_domains(id) ON DELETE SET NULL,
    UNIQUE(user_id, role_id, domain_id)
);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_user_roles_user_id ON bdopsflow_user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_user_roles_role_id ON bdopsflow_user_roles(role_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_user_roles_domain_id ON bdopsflow_user_roles(domain_id);

-- 14. 执行器领域分配表
CREATE TABLE IF NOT EXISTS bdopsflow_domain_executors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain_id INTEGER NOT NULL,
    executor_id INTEGER NOT NULL,
    assigned_by INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (domain_id) REFERENCES bdopsflow_domains(id) ON DELETE CASCADE,
    FOREIGN KEY (executor_id) REFERENCES bdopsflow_executors(id) ON DELETE CASCADE,
    FOREIGN KEY (assigned_by) REFERENCES bdopsflow_users(id) ON DELETE SET NULL,
    UNIQUE(domain_id, executor_id)
);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_domain_executors_domain_id ON bdopsflow_domain_executors(domain_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_domain_executors_executor_id ON bdopsflow_domain_executors(executor_id);

-- ============================================================================
-- 第三部分：初始化数据
-- ============================================================================

-- 插入默认领域
INSERT OR IGNORE INTO bdopsflow_domains (name, description) VALUES ('default', '默认领域');

-- 插入预设角色
INSERT OR IGNORE INTO bdopsflow_roles (name, code, description, is_system, domain_id) VALUES
('系统管理员', 'system_admin', '系统最高权限，可管理所有资源', 1, NULL),
('领域管理员', 'domain_admin', '领域级管理权限', 1, NULL),
('普通用户', 'user', '基础查看和操作权限', 1, NULL);

-- 插入所有权限定义
INSERT OR IGNORE INTO bdopsflow_permissions (resource, action, description) VALUES
-- 用户管理权限
('user', 'create', '创建用户'),
('user', 'read', '查看用户'),
('user', 'update', '更新用户'),
('user', 'delete', '删除用户'),
('user', 'manage', '完整管理用户'),

-- 角色管理权限
('role', 'create', '创建角色'),
('role', 'read', '查看角色'),
('role', 'update', '更新角色'),
('role', 'delete', '删除角色'),
('role', 'manage', '完整管理角色'),

-- 权限查看权限
('permission', 'read', '查看权限列表'),

-- 领域管理权限
('domain', 'create', '创建领域'),
('domain', 'read', '查看领域'),
('domain', 'update', '更新领域'),
('domain', 'delete', '删除领域'),
('domain', 'manage', '完整管理领域'),

-- 执行器管理权限
('executor', 'read', '查看执行器'),
('executor', 'assign', '分配执行器'),
('executor', 'manage', '完整管理执行器'),

-- 任务管理权限
('task', 'create', '创建任务'),
('task', 'read', '查看任务'),
('task', 'update', '更新任务'),
('task', 'delete', '删除任务'),
('task', 'trigger', '手动触发任务'),
('task', 'manage', '完整管理任务'),

-- 日志管理权限
('log', 'read', '查看日志'),
('log', 'delete', '删除日志'),
('log', 'manage', '完整管理日志'),

-- 工作流管理权限
('workflow', 'create', '创建工作流'),
('workflow', 'read', '查看工作流'),
('workflow', 'update', '更新工作流'),
('workflow', 'delete', '删除工作流'),
('workflow', 'manage', '完整管理工作流');

-- 为系统管理员分配所有权限
INSERT OR IGNORE INTO bdopsflow_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM bdopsflow_roles r, bdopsflow_permissions p
WHERE r.code = 'system_admin';

-- 为领域管理员分配任务、执行器、日志、工作流的全部权限
INSERT OR IGNORE INTO bdopsflow_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM bdopsflow_roles r, bdopsflow_permissions p
WHERE r.code = 'domain_admin'
AND p.resource IN ('task', 'executor', 'log', 'workflow', 'permission');

-- 为普通用户分配查看和手动触发权限
INSERT OR IGNORE INTO bdopsflow_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM bdopsflow_roles r, bdopsflow_permissions p
WHERE r.code = 'user'
AND p.action IN ('read', 'trigger');

-- 默认管理员用户 (密码: admin123, bcrypt hash)
INSERT OR IGNORE INTO bdopsflow_users (username, password, email, domain_id, role, is_active) 
VALUES ('admin', '$2a$10$V4DeC68lOaLwF6N1pAVR8ux7WzY9NOeuPgwrAkyF9XcpWOL9muEaG', 'admin@example.com', 1, 'system_admin', 1);

-- 将 admin 用户设置为系统管理员
INSERT OR IGNORE INTO bdopsflow_user_roles (user_id, role_id, domain_id)
SELECT u.id, r.id, NULL FROM bdopsflow_users u, bdopsflow_roles r
WHERE u.username = 'admin' AND r.code = 'system_admin';

-- ============================================================================
-- 初始化完成
-- ============================================================================
