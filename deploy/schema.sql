-- BDopsFlow 数据库初始化脚本
-- rqlite 分布式数据库
-- 版本：v3.0
-- 日期：2026-05-26
-- 描述：权限体系完全重写 - 纯RBAC模型、多领域支持、角色继承、菜单自动推导

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

-- 2. 用户表（移除 domain_id 和 role 字段，权限统一由 RBAC 管理）
CREATE TABLE IF NOT EXISTS bdopsflow_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    real_name TEXT DEFAULT '',
    phone TEXT DEFAULT '',
    password TEXT NOT NULL,
    email TEXT,
    is_active BOOLEAN DEFAULT 1,
    last_login_at DATETIME,
    created_by INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_users_username ON bdopsflow_users(username);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_users_is_active ON bdopsflow_users(is_active);

-- 3. 用户-领域关联表（多对多，支持一个用户属于多个领域）
CREATE TABLE IF NOT EXISTS bdopsflow_user_domains (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    domain_id INTEGER NOT NULL,
    is_default BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES bdopsflow_users(id) ON DELETE CASCADE,
    FOREIGN KEY (domain_id) REFERENCES bdopsflow_domains(id) ON DELETE CASCADE,
    UNIQUE(user_id, domain_id)
);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_user_domains_user_id ON bdopsflow_user_domains(user_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_user_domains_domain_id ON bdopsflow_user_domains(domain_id);

-- 4. 任务表
CREATE TABLE IF NOT EXISTS bdopsflow_tasks (
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

CREATE INDEX IF NOT EXISTS idx_bdopsflow_tasks_domain_id ON bdopsflow_tasks(domain_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_tasks_is_enabled ON bdopsflow_tasks(is_enabled);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_tasks_type ON bdopsflow_tasks(type);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_tasks_status ON bdopsflow_tasks(status);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_tasks_cron_enabled ON bdopsflow_tasks(is_enabled, cron_expression) WHERE cron_expression != '';
CREATE INDEX IF NOT EXISTS idx_bdopsflow_tasks_assigned_executor ON bdopsflow_tasks(assigned_executor_id);

-- 6. 任务执行记录表
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
CREATE INDEX IF NOT EXISTS idx_bdopsflow_task_executions_task_status ON bdopsflow_task_executions(task_id, status, created_at DESC);

-- 7. 执行器节点表
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

-- 7. 任务依赖表
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

-- 8. 任务执行日志表
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
-- 第二部分：权限管理系统表（v3.0 重写）
-- ============================================================================

-- 9. 角色表（新增 parent_id 支持角色继承）
CREATE TABLE IF NOT EXISTS bdopsflow_roles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    code TEXT NOT NULL UNIQUE,
    description TEXT,
    is_system BOOLEAN DEFAULT 0,
    parent_id INTEGER,
    domain_id INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (parent_id) REFERENCES bdopsflow_roles(id) ON DELETE SET NULL,
    FOREIGN KEY (domain_id) REFERENCES bdopsflow_domains(id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bdopsflow_roles_code ON bdopsflow_roles(code);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_roles_domain_id ON bdopsflow_roles(domain_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_roles_is_system ON bdopsflow_roles(is_system);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_roles_parent_id ON bdopsflow_roles(parent_id);

-- 10. 权限表（移除 menu:* 权限，菜单由资源权限自动推导）
CREATE TABLE IF NOT EXISTS bdopsflow_permissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    resource TEXT NOT NULL,
    action TEXT NOT NULL,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(resource, action)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bdopsflow_permissions_resource_action ON bdopsflow_permissions(resource, action);

-- 11. 角色权限映射表
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

-- 12. 用户角色映射表
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

-- 13. 执行器领域分配表
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

INSERT OR IGNORE INTO bdopsflow_domains (name, description) VALUES ('default', '默认领域');

-- 预设角色（含继承层级：system_admin → domain_admin → user）
INSERT OR IGNORE INTO bdopsflow_roles (name, code, description, is_system, parent_id, domain_id) VALUES
('系统管理员', 'system_admin', '系统最高权限，可管理所有资源', 1, NULL, NULL),
('领域管理员', 'domain_admin', '领域级管理权限，继承普通用户权限', 1, NULL, NULL),
('普通用户', 'user', '基础查看和操作权限', 1, NULL, NULL);

-- 权限定义（无 menu:* 权限，菜单由资源权限自动推导）
INSERT OR IGNORE INTO bdopsflow_permissions (resource, action, description) VALUES
-- 仪表盘
('dashboard', 'read', '查看仪表盘'),

-- 用户管理
('user', 'create', '创建用户'),
('user', 'read', '查看用户'),
('user', 'update', '更新用户'),
('user', 'delete', '删除用户'),
('user', 'reset_password', '重置用户密码'),
('user', 'manage', '完整管理用户'),

-- 角色管理
('role', 'create', '创建角色'),
('role', 'read', '查看角色'),
('role', 'update', '更新角色'),
('role', 'delete', '删除角色'),
('role', 'manage', '完整管理角色'),

-- 权限查看
('permission', 'read', '查看权限列表'),

-- 领域管理
('domain', 'create', '创建领域'),
('domain', 'read', '查看领域'),
('domain', 'update', '更新领域'),
('domain', 'delete', '删除领域'),
('domain', 'manage', '完整管理领域'),

-- 执行器管理
('executor', 'read', '查看执行器'),
('executor', 'assign', '分配执行器到领域'),
('executor', 'online', '上线执行器'),
('executor', 'offline', '下线执行器'),
('executor', 'delete', '删除执行器'),
('executor', 'manage', '完整管理执行器'),

-- 任务管理
('task', 'create', '创建任务'),
('task', 'read', '查看任务'),
('task', 'update', '更新任务'),
('task', 'delete', '删除任务'),
('task', 'trigger', '手动触发任务'),
('task', 'manage', '完整管理任务'),

-- 日志管理
('log', 'read', '查看日志'),
('log', 'delete', '删除日志'),
('log', 'manage', '完整管理日志'),

-- 数据源管理
('datasource', 'create', '创建数据源'),
('datasource', 'read', '查看数据源'),
('datasource', 'update', '更新数据源'),
('datasource', 'delete', '删除数据源'),
('datasource', 'query', '查询数据'),
('datasource', 'download', '下载数据'),
('datasource', 'manage', '完整管理数据源'),

-- Webhook管理
('webhook', 'create', '创建Webhook'),
('webhook', 'read', '查看Webhook'),
('webhook', 'update', '更新Webhook'),
('webhook', 'delete', '删除Webhook'),
('webhook', 'trigger', '手动触发Webhook'),
('webhook', 'manage', '完整管理Webhook'),

-- 审计日志
('audit_log', 'read', '查看审计日志'),
('audit_log', 'delete', '删除审计日志'),
('audit_log', 'manage', '完整管理审计日志'),

-- 系统配置
('config', 'read', '查看系统配置'),
('config', 'update', '更新系统配置'),
('config', 'manage', '完整管理系统配置'),

-- 接口测试
('api_test', 'create', '创建接口测试'),
('api_test', 'read', '查看接口测试'),
('api_test', 'update', '更新接口测试'),
('api_test', 'delete', '删除接口测试'),
('api_test', 'execute', '执行接口测试'),
('api_test', 'manage', '完整管理接口测试');

-- 系统管理员：所有权限
INSERT OR IGNORE INTO bdopsflow_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM bdopsflow_roles r, bdopsflow_permissions p
WHERE r.code = 'system_admin';

-- 领域管理员：任务、执行器、日志、数据源、Webhook、权限查看、仪表盘、用户管理、角色管理、领域管理
INSERT OR IGNORE INTO bdopsflow_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM bdopsflow_roles r, bdopsflow_permissions p
WHERE r.code = 'domain_admin'
AND (
    p.resource = 'dashboard'
    OR (p.resource = 'task' AND p.action IN ('create', 'read', 'update', 'delete', 'trigger', 'manage'))
    OR (p.resource = 'executor' AND p.action IN ('read', 'assign', 'online', 'offline', 'manage'))
    OR (p.resource = 'log' AND p.action IN ('read', 'delete', 'manage'))
    OR (p.resource = 'datasource' AND p.action IN ('create', 'read', 'update', 'delete', 'query', 'download', 'manage'))
    OR (p.resource = 'webhook' AND p.action IN ('create', 'read', 'update', 'delete', 'trigger', 'manage'))
    OR (p.resource = 'user' AND p.action IN ('create', 'read', 'update', 'delete', 'reset_password', 'manage'))
    OR (p.resource = 'role' AND p.action IN ('create', 'read', 'update', 'delete', 'manage'))
    OR (p.resource = 'domain' AND p.action IN ('read', 'update', 'manage'))
    OR (p.resource = 'permission' AND p.action IN ('read'))
    OR (p.resource = 'api_test' AND p.action IN ('create', 'read', 'update', 'delete', 'execute', 'manage'))
);

-- 普通用户：查看、触发、查询
INSERT OR IGNORE INTO bdopsflow_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM bdopsflow_roles r, bdopsflow_permissions p
WHERE r.code = 'user'
AND (
    p.resource = 'dashboard'
    OR (p.resource = 'task' AND p.action IN ('read', 'trigger'))
    OR (p.resource = 'executor' AND p.action IN ('read'))
    OR (p.resource = 'log' AND p.action IN ('read'))
    OR (p.resource = 'datasource' AND p.action IN ('read', 'query'))
    OR (p.resource = 'webhook' AND p.action IN ('read'))
);

-- 默认管理员用户 (密码: admin123)
INSERT OR IGNORE INTO bdopsflow_users (username, real_name, phone, password, email, is_active)
VALUES ('admin', '系统管理员', '', '$2a$10$V4DeC68lOaLwF6N1pAVR8ux7WzY9NOeuPgwrAkyF9XcpWOL9muEaG', 'admin@example.com', 1);

-- admin 用户关联默认领域
INSERT OR IGNORE INTO bdopsflow_user_domains (user_id, domain_id, is_default)
SELECT u.id, d.id, 1 FROM bdopsflow_users u, bdopsflow_domains d
WHERE u.username = 'admin' AND d.name = 'default';

-- admin 用户分配系统管理员角色（全局）
INSERT OR IGNORE INTO bdopsflow_user_roles (user_id, role_id, domain_id)
SELECT u.id, r.id, NULL FROM bdopsflow_users u, bdopsflow_roles r
WHERE u.username = 'admin' AND r.code = 'system_admin';

-- ============================================================================
-- 第四部分：数据查询模块表
-- ============================================================================

-- 14. 数据源表
CREATE TABLE IF NOT EXISTS bdopsflow_datasources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    host TEXT,
    port INTEGER,
    path TEXT,
    database TEXT,
    username TEXT,
    password TEXT,
    auth_type TEXT DEFAULT 'simple',
    connection_mode TEXT DEFAULT 'single',
    zk_hosts TEXT,
    zk_path TEXT,
    rqlite_hosts TEXT,
    config TEXT,
    description TEXT,
    domain_id INTEGER NOT NULL,
    is_enabled BOOLEAN DEFAULT 1,
    allow_write_sql BOOLEAN DEFAULT 0,
    test_status TEXT DEFAULT 'untested',
    last_test_at DATETIME,
    created_by INTEGER,
    updated_by INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (domain_id) REFERENCES bdopsflow_domains(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_datasources_domain_id ON bdopsflow_datasources(domain_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_datasources_type ON bdopsflow_datasources(type);
CREATE UNIQUE INDEX IF NOT EXISTS idx_bdopsflow_datasources_name_domain ON bdopsflow_datasources(name, domain_id);

-- 15. 数据源实例权限表
CREATE TABLE IF NOT EXISTS bdopsflow_datasource_permissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    datasource_id INTEGER NOT NULL,
    role_id INTEGER,
    user_id INTEGER,
    permission_type TEXT NOT NULL CHECK(permission_type IN ('query', 'read', 'update', 'delete', 'download', 'manage')),
    granted_by INTEGER,
    granted_at TEXT NOT NULL,
    FOREIGN KEY (datasource_id) REFERENCES bdopsflow_datasources(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES bdopsflow_roles(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES bdopsflow_users(id) ON DELETE CASCADE,
    CHECK(role_id IS NOT NULL OR user_id IS NOT NULL),
    UNIQUE(datasource_id, role_id, permission_type),
    UNIQUE(datasource_id, user_id, permission_type)
);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_ds_perms_datasource_id ON bdopsflow_datasource_permissions(datasource_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_ds_perms_role_id ON bdopsflow_datasource_permissions(role_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_ds_perms_user_id ON bdopsflow_datasource_permissions(user_id);

-- 16. 保存的SQL表
CREATE TABLE IF NOT EXISTS bdopsflow_saved_sql (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    datasource_id INTEGER NOT NULL,
    sql_text TEXT NOT NULL,
    description TEXT,
    created_by INTEGER,
    updated_by INTEGER,
    domain_id INTEGER NOT NULL,
    is_public BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (datasource_id) REFERENCES bdopsflow_datasources(id) ON DELETE CASCADE,
    FOREIGN KEY (domain_id) REFERENCES bdopsflow_domains(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_saved_sql_datasource_id ON bdopsflow_saved_sql(datasource_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_saved_sql_domain_id ON bdopsflow_saved_sql(domain_id);

-- 17. 查询历史表
CREATE TABLE IF NOT EXISTS bdopsflow_query_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    query_id TEXT,
    datasource_id INTEGER,
    datasource_name TEXT,
    sql_text TEXT NOT NULL,
    database TEXT,
    execution_time REAL,
    row_count INTEGER,
    status TEXT NOT NULL,
    error_message TEXT,
    executed_by INTEGER,
    domain_id INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (datasource_id) REFERENCES bdopsflow_datasources(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_query_history_datasource_id ON bdopsflow_query_history(datasource_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_query_history_domain_id ON bdopsflow_query_history(domain_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_query_history_created_at ON bdopsflow_query_history(created_at);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_query_history_query_id ON bdopsflow_query_history(query_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_query_history_executed_by ON bdopsflow_query_history(executed_by);

-- 18. 系统配置表
CREATE TABLE IF NOT EXISTS bdopsflow_system_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_key TEXT NOT NULL UNIQUE,
    config_value TEXT NOT NULL,
    description TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bdopsflow_system_config_key ON bdopsflow_system_config(config_key);

-- 19. 配置变更历史表
CREATE TABLE IF NOT EXISTS bdopsflow_system_config_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_key TEXT NOT NULL,
    old_value TEXT,
    new_value TEXT NOT NULL,
    changed_by INTEGER,
    changed_at TEXT NOT NULL,
    FOREIGN KEY (changed_by) REFERENCES bdopsflow_users(id)
);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_config_history_key ON bdopsflow_system_config_history(config_key);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_config_history_time ON bdopsflow_system_config_history(changed_at);

-- 20. 审计日志表
CREATE TABLE IF NOT EXISTS bdopsflow_audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    username TEXT NOT NULL,
    real_name TEXT DEFAULT '',
    role TEXT,
    domain_id INTEGER,
    action TEXT NOT NULL,
    resource TEXT NOT NULL,
    resource_id TEXT,
    resource_name TEXT,
    status TEXT NOT NULL,
    ip_address TEXT,
    user_agent TEXT,
    request_method TEXT,
    request_path TEXT,
    detail TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON bdopsflow_audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON bdopsflow_audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource ON bdopsflow_audit_logs(resource);
CREATE INDEX IF NOT EXISTS idx_audit_logs_status ON bdopsflow_audit_logs(status);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON bdopsflow_audit_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_action ON bdopsflow_audit_logs(resource, action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_domain_id ON bdopsflow_audit_logs(domain_id);

-- 21. Webhook配置表
CREATE TABLE IF NOT EXISTS bdopsflow_webhooks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    method TEXT DEFAULT 'POST',
    headers TEXT DEFAULT '{}',
    secret TEXT DEFAULT '',
    domain_id INTEGER NOT NULL,
    is_enabled BOOLEAN DEFAULT 1,
    description TEXT DEFAULT '',
    created_by INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (domain_id) REFERENCES bdopsflow_domains(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_webhooks_domain_id ON bdopsflow_webhooks(domain_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_webhooks_is_enabled ON bdopsflow_webhooks(is_enabled);
CREATE UNIQUE INDEX IF NOT EXISTS idx_bdopsflow_webhooks_name_domain ON bdopsflow_webhooks(name, domain_id);

-- 22. Webhook实例权限表
CREATE TABLE IF NOT EXISTS bdopsflow_webhook_permissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    webhook_id INTEGER NOT NULL,
    role_id INTEGER,
    user_id INTEGER,
    permission_type TEXT NOT NULL CHECK(permission_type IN ('read', 'update', 'delete', 'trigger', 'manage')),
    granted_by INTEGER,
    granted_at TEXT NOT NULL,
    FOREIGN KEY (webhook_id) REFERENCES bdopsflow_webhooks(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES bdopsflow_roles(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES bdopsflow_users(id) ON DELETE CASCADE,
    CHECK(role_id IS NOT NULL OR user_id IS NOT NULL),
    UNIQUE(webhook_id, role_id, permission_type),
    UNIQUE(webhook_id, user_id, permission_type)
);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_webhook_perms_webhook_id ON bdopsflow_webhook_permissions(webhook_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_webhook_perms_role_id ON bdopsflow_webhook_permissions(role_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_webhook_perms_user_id ON bdopsflow_webhook_permissions(user_id);

-- ============================================================================
-- 第六部分：接口测试模块表
-- ============================================================================

-- 23. 接口测试用例表
CREATE TABLE IF NOT EXISTS bdopsflow_api_tests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    config TEXT NOT NULL,
    created_by INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (created_by) REFERENCES bdopsflow_users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_api_tests_type ON bdopsflow_api_tests(type);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_api_tests_created_by ON bdopsflow_api_tests(created_by);
CREATE UNIQUE INDEX IF NOT EXISTS idx_bdopsflow_api_tests_name_user ON bdopsflow_api_tests(name, created_by);

-- 24. Proto文件表
CREATE TABLE IF NOT EXISTS bdopsflow_proto_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    content TEXT NOT NULL,
    file_hash TEXT NOT NULL,
    parsed_result TEXT,
    dependencies TEXT DEFAULT '[]',
    created_by INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (created_by) REFERENCES bdopsflow_users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_proto_files_created_by ON bdopsflow_proto_files(created_by);
CREATE UNIQUE INDEX IF NOT EXISTS idx_bdopsflow_proto_files_name_user ON bdopsflow_proto_files(name, created_by);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_proto_files_file_hash ON bdopsflow_proto_files(file_hash);

-- 25. 证书文件表
CREATE TABLE IF NOT EXISTS bdopsflow_certificates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    ca_cert TEXT,
    client_cert TEXT,
    client_key TEXT,
    created_by INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (created_by) REFERENCES bdopsflow_users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_certificates_created_by ON bdopsflow_certificates(created_by);
CREATE UNIQUE INDEX IF NOT EXISTS idx_bdopsflow_certificates_name_user ON bdopsflow_certificates(name, created_by);

-- 26. 接口测试结果表
CREATE TABLE IF NOT EXISTS bdopsflow_api_test_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    test_id INTEGER,
    type TEXT NOT NULL,
    status_code INTEGER,
    latency_ms INTEGER,
    headers TEXT,
    body TEXT,
    error TEXT,
    assertions_result TEXT,
    executed_by INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (test_id) REFERENCES bdopsflow_api_tests(id) ON DELETE CASCADE,
    FOREIGN KEY (executed_by) REFERENCES bdopsflow_users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_bdopsflow_api_test_results_test_id ON bdopsflow_api_test_results(test_id);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_api_test_results_type ON bdopsflow_api_test_results(type);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_api_test_results_executed_by ON bdopsflow_api_test_results(executed_by);
CREATE INDEX IF NOT EXISTS idx_bdopsflow_api_test_results_created_at ON bdopsflow_api_test_results(created_at DESC);

-- ============================================================================
-- 第五部分：系统配置初始化
-- ============================================================================

INSERT OR IGNORE INTO bdopsflow_system_config (config_key, config_value, description, updated_at) VALUES
('web.enabled', 'false', '是否启用内置Web UI', datetime('now')),
('datasource.default_limit', '1000', 'SQL查询默认限制行数', datetime('now')),
('datasource.max_export_rows', '1000', 'CSV导出最大行数', datetime('now')),
('datasource.cache_ttl', '300', '查询结果缓存TTL(秒)', datetime('now')),
('datasource.cache_max_size', '100', '缓存最大内存占用(MB)', datetime('now')),
('datasource.query_timeout', '60', '查询超时时间(秒)', datetime('now')),
('datasource.max_concurrent_per_user', '5', '单用户并发查询限制', datetime('now')),
('datasource.max_concurrent_global', '50', '全局并发查询限制', datetime('now')),
('datasource.allow_write_sql', 'false', '是否允许写操作SQL', datetime('now')),
('datasource.history_retention_days', '30', '查询历史保留天数', datetime('now')),
('datasource.connection_max_idle', '5', '连接池最大空闲连接数', datetime('now')),
('datasource.connection_max_open', '10', '连接池最大打开连接数', datetime('now')),
('datasource.connection_max_lifetime', '1800', '连接最大生命周期(秒)', datetime('now')),
('datasource.max_sql_length', '65536', 'SQL文本最大长度(字节)', datetime('now')),
('datasource.max_cell_size', '65536', '单个单元格值最大字节数', datetime('now')),
('datasource.health_check_interval', '300', '健康检查间隔(秒),0为禁用', datetime('now')),
('datasource.test_timeout', '10', '连接测试超时时间(秒)', datetime('now')),
('audit_log.retention_days', '90', '审计日志保留天数', datetime('now')),
('wecom.robot_url', 'https://qyapi.weixin.qq.com/cgi-bin/webhook/send', '企业微信群机器人URL', datetime('now'));

-- ============================================================================
-- 初始化完成
-- ============================================================================
