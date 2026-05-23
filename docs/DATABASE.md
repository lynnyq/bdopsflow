# BDopsFlow 数据库设计文档

本文档描述了 BDopsFlow 分布式工作流调度平台的数据库设计，包括表结构、索引和配置说明。

## 数据库选型

- **开发环境**：rqlite 单节点部署
- **生产环境**：rqlite 3 节点集群部署，支持 Raft 共识协议

### rqlite 简介

rqlite 是一个轻量级的分布式关系型数据库，基于 SQLite 和 Raft 共识协议构建：
- 分布式：多节点通过 Raft 协议保证数据一致性
- 高可用：支持故障自动转移
- 简单：SQLite 兼容，无需额外的数据库服务器
- 可靠：使用 Raft 共识算法保证数据一致性

## 表结构设计

所有表名添加 `bdopsflow_` 前缀以实现数据库隔离。

### 1. 领域表 (bdopsflow_domains)

资源隔离的边界，支持多租户。

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_domains (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    is_active INTEGER DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_domains_name ON bdopsflow_domains(name);
CREATE INDEX IF NOT EXISTS idx_domains_is_active ON bdopsflow_domains(is_active);
```

### 2. 用户表 (bdopsflow_users)

用户认证和基本信息。

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    real_name TEXT DEFAULT '',
    phone TEXT DEFAULT '',
    password TEXT NOT NULL,
    email TEXT,
    domain_id INTEGER,
    role TEXT NOT NULL DEFAULT 'user',
    is_active INTEGER DEFAULT 1,
    last_login_at TEXT,
    created_by INTEGER,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (domain_id) REFERENCES bdopsflow_domains(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_users_username ON bdopsflow_users(username);
CREATE INDEX IF NOT EXISTS idx_users_email ON bdopsflow_users(email);
CREATE INDEX IF NOT EXISTS idx_users_domain_id ON bdopsflow_users(domain_id);
CREATE INDEX IF NOT EXISTS idx_users_role ON bdopsflow_users(role);
CREATE INDEX IF NOT EXISTS idx_users_is_active ON bdopsflow_users(is_active);
```

### 3. 执行器表 (bdopsflow_executors)

执行器注册和管理，使用 `name` 作为唯一标识。

```sql
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

CREATE UNIQUE INDEX IF NOT EXISTS idx_executors_name ON bdopsflow_executors(name);
CREATE INDEX IF NOT EXISTS idx_executors_status ON bdopsflow_executors(status);
CREATE INDEX IF NOT EXISTS idx_executors_last_heartbeat ON bdopsflow_executors(last_heartbeat);
CREATE INDEX IF NOT EXISTS idx_executors_status_heartbeat ON bdopsflow_executors(status, last_heartbeat);
CREATE INDEX IF NOT EXISTS idx_executors_is_global ON bdopsflow_executors(is_global);
```

### 4. 任务表 (bdopsflow_tasks)

任务定义和配置。

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_id INTEGER,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    config TEXT,
    cron_expression TEXT,
    timeout_seconds INTEGER NOT NULL DEFAULT 3600,
    retry_count INTEGER NOT NULL DEFAULT 0,
    retry_interval INTEGER NOT NULL DEFAULT 60,
    is_enabled INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'idle',
    domain_id INTEGER NOT NULL,
    webhook_config TEXT,
    assigned_executor_id INTEGER,
    created_by INTEGER,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (workflow_id) REFERENCES bdopsflow_workflows(id) ON DELETE CASCADE,
    FOREIGN KEY (domain_id) REFERENCES bdopsflow_domains(id) ON DELETE CASCADE,
    FOREIGN KEY (assigned_executor_id) REFERENCES bdopsflow_executors(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_tasks_workflow_id ON bdopsflow_tasks(workflow_id);
CREATE INDEX IF NOT EXISTS idx_tasks_name ON bdopsflow_tasks(name);
CREATE INDEX IF NOT EXISTS idx_tasks_type ON bdopsflow_tasks(type);
CREATE INDEX IF NOT EXISTS idx_tasks_is_enabled ON bdopsflow_tasks(is_enabled);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON bdopsflow_tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_domain_id ON bdopsflow_tasks(domain_id);
CREATE INDEX IF NOT EXISTS idx_tasks_assigned_executor ON bdopsflow_tasks(assigned_executor_id);
CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON bdopsflow_tasks(created_at);
```

### 5. 工作流表 (bdopsflow_workflows)

工作流定义。

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_workflows (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT,
    dag_config TEXT,
    cron_expression TEXT,
    is_enabled INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'idle',
    domain_id INTEGER NOT NULL,
    created_by INTEGER,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (domain_id) REFERENCES bdopsflow_domains(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_workflows_name ON bdopsflow_workflows(name);
CREATE INDEX IF NOT EXISTS idx_workflows_is_enabled ON bdopsflow_workflows(is_enabled);
CREATE INDEX IF NOT EXISTS idx_workflows_status ON bdopsflow_workflows(status);
CREATE INDEX IF NOT EXISTS idx_workflows_domain_id ON bdopsflow_workflows(domain_id);
```

### 6. 任务依赖表 (bdopsflow_task_dependencies)

任务依赖关系定义（DAG），记录任务间的血缘关系。

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_task_dependencies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    parent_task_id INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES bdopsflow_tasks(id) ON DELETE CASCADE,
    FOREIGN KEY (parent_task_id) REFERENCES bdopsflow_tasks(id) ON DELETE CASCADE,
    UNIQUE(task_id, parent_task_id)
);

CREATE INDEX IF NOT EXISTS idx_task_deps_task_id ON bdopsflow_task_dependencies(task_id);
CREATE INDEX IF NOT EXISTS idx_task_deps_parent_id ON bdopsflow_task_dependencies(parent_task_id);
```

### 7. 任务执行记录表 (bdopsflow_task_executions)

任务执行历史记录。

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_task_executions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    execution_id TEXT NOT NULL UNIQUE,
    executor_id INTEGER,
    status TEXT NOT NULL DEFAULT 'pending',
    start_time DATETIME,
    end_time DATETIME,
    output TEXT,
    error TEXT,
    retry_times INTEGER DEFAULT 0,
    progress INTEGER DEFAULT 0,
    progress_msg TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES bdopsflow_tasks(id) ON DELETE CASCADE,
    FOREIGN KEY (executor_id) REFERENCES bdopsflow_executors(id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_executions_execution_id ON bdopsflow_task_executions(execution_id);
CREATE INDEX IF NOT EXISTS idx_executions_task_id ON bdopsflow_task_executions(task_id);
CREATE INDEX IF NOT EXISTS idx_executions_executor_id ON bdopsflow_task_executions(executor_id);
CREATE INDEX IF NOT EXISTS idx_executions_status ON bdopsflow_task_executions(status);
CREATE INDEX IF NOT EXISTS idx_executions_created_at ON bdopsflow_task_executions(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_executions_status_time ON bdopsflow_task_executions(status, created_at);
```

### 8. 工作流执行记录表 (bdopsflow_workflow_executions)

工作流执行历史记录。

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_workflow_executions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_id INTEGER NOT NULL,
    execution_id TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'pending',
    start_time DATETIME,
    end_time DATETIME,
    node_states TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (workflow_id) REFERENCES bdopsflow_workflows(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_wf_executions_execution_id ON bdopsflow_workflow_executions(execution_id);
CREATE INDEX IF NOT EXISTS idx_wf_executions_workflow_id ON bdopsflow_workflow_executions(workflow_id);
CREATE INDEX IF NOT EXISTS idx_wf_executions_status ON bdopsflow_workflow_executions(status);
CREATE INDEX IF NOT EXISTS idx_wf_executions_created_at ON bdopsflow_workflow_executions(created_at DESC);
```

### 9. 任务执行日志表 (bdopsflow_task_logs)

任务执行实时日志。

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_task_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    execution_id TEXT NOT NULL,
    task_id INTEGER NOT NULL,
    executor_id INTEGER,
    node_id TEXT,
    log_level TEXT NOT NULL DEFAULT 'info',
    message TEXT NOT NULL,
    log_time TEXT NOT NULL,
    FOREIGN KEY (task_id) REFERENCES bdopsflow_tasks(id) ON DELETE CASCADE,
    FOREIGN KEY (executor_id) REFERENCES bdopsflow_executors(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_logs_execution_id ON bdopsflow_task_logs(execution_id);
CREATE INDEX IF NOT EXISTS idx_logs_task_id ON bdopsflow_task_logs(task_id);
CREATE INDEX IF NOT EXISTS idx_logs_log_level ON bdopsflow_task_logs(log_level);
CREATE INDEX IF NOT EXISTS idx_logs_log_time ON bdopsflow_task_logs(log_time);
```

### 10. 角色表 (bdopsflow_roles)

系统角色定义。

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_roles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    code TEXT NOT NULL UNIQUE,
    description TEXT,
    is_system INTEGER NOT NULL DEFAULT 0,
    domain_id INTEGER,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (domain_id) REFERENCES bdopsflow_domains(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_roles_code ON bdopsflow_roles(code);
CREATE INDEX IF NOT EXISTS idx_roles_domain_id ON bdopsflow_roles(domain_id);
CREATE INDEX IF NOT EXISTS idx_roles_is_system ON bdopsflow_roles(is_system);
```

### 11. 权限表 (bdopsflow_permissions)

系统权限定义。

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_permissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    resource TEXT NOT NULL,
    action TEXT NOT NULL,
    description TEXT,
    created_at TEXT NOT NULL,
    UNIQUE(resource, action)
);

CREATE INDEX IF NOT EXISTS idx_perms_resource ON bdopsflow_permissions(resource);
CREATE INDEX IF NOT EXISTS idx_perms_action ON bdopsflow_permissions(action);
```

### 12. 角色权限映射表 (bdopsflow_role_permissions)

角色与权限的多对多关系。

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_role_permissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    role_id INTEGER NOT NULL,
    permission_id INTEGER NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (role_id) REFERENCES bdopsflow_roles(id) ON DELETE CASCADE,
    FOREIGN KEY (permission_id) REFERENCES bdopsflow_permissions(id) ON DELETE CASCADE,
    UNIQUE(role_id, permission_id)
);

CREATE INDEX IF NOT EXISTS idx_role_perms_role_id ON bdopsflow_role_permissions(role_id);
CREATE INDEX IF NOT EXISTS idx_role_perms_permission_id ON bdopsflow_role_permissions(permission_id);
```

### 13. 用户角色映射表 (bdopsflow_user_roles)

用户与角色的多对多关系。

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_user_roles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    role_id INTEGER NOT NULL,
    domain_id INTEGER,
    created_at TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES bdopsflow_users(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES bdopsflow_roles(id) ON DELETE CASCADE,
    FOREIGN KEY (domain_id) REFERENCES bdopsflow_domains(id) ON DELETE CASCADE,
    UNIQUE(user_id, role_id, domain_id)
);

CREATE INDEX IF NOT EXISTS idx_user_roles_user_id ON bdopsflow_user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_role_id ON bdopsflow_user_roles(role_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_domain_id ON bdopsflow_user_roles(domain_id);
```

### 14. 执行器领域分配表 (bdopsflow_domain_executors)

执行器与领域的分配关系。

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_domain_executors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain_id INTEGER NOT NULL,
    executor_id INTEGER NOT NULL,
    assigned_by INTEGER,
    created_at TEXT NOT NULL,
    FOREIGN KEY (domain_id) REFERENCES bdopsflow_domains(id) ON DELETE CASCADE,
    FOREIGN KEY (executor_id) REFERENCES bdopsflow_executors(id) ON DELETE CASCADE,
    UNIQUE(domain_id, executor_id)
);

CREATE INDEX IF NOT EXISTS idx_domain_executors_domain_id ON bdopsflow_domain_executors(domain_id);
CREATE INDEX IF NOT EXISTS idx_domain_executors_executor_id ON bdopsflow_domain_executors(executor_id);
```

### 15. 数据源表 (bdopsflow_datasources)

数据源连接配置，支持多种数据库类型。

```sql
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

CREATE INDEX IF NOT EXISTS idx_datasources_domain_id ON bdopsflow_datasources(domain_id);
CREATE INDEX IF NOT EXISTS idx_datasources_type ON bdopsflow_datasources(type);
CREATE UNIQUE INDEX IF NOT EXISTS idx_datasources_name_domain ON bdopsflow_datasources(name, domain_id);
```

### 16. 保存的SQL表 (bdopsflow_saved_sql)

用户保存的常用SQL查询。

```sql
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

CREATE INDEX IF NOT EXISTS idx_saved_sql_datasource_id ON bdopsflow_saved_sql(datasource_id);
CREATE INDEX IF NOT EXISTS idx_saved_sql_domain_id ON bdopsflow_saved_sql(domain_id);
```

### 17. 数据源权限表 (bdopsflow_datasource_permissions)

数据源访问权限控制，支持角色和用户两种授权方式。

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_datasource_permissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    datasource_id INTEGER NOT NULL,
    role_id INTEGER,
    user_id INTEGER,
    permission_type TEXT NOT NULL,
    granted_by INTEGER,
    granted_at TEXT NOT NULL,
    FOREIGN KEY (datasource_id) REFERENCES bdopsflow_datasources(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES bdopsflow_roles(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES bdopsflow_users(id) ON DELETE CASCADE,
    CHECK(role_id IS NOT NULL OR user_id IS NOT NULL),
    UNIQUE(datasource_id, role_id, permission_type),
    UNIQUE(datasource_id, user_id, permission_type)
);

CREATE INDEX IF NOT EXISTS idx_ds_perms_datasource_id ON bdopsflow_datasource_permissions(datasource_id);
CREATE INDEX IF NOT EXISTS idx_ds_perms_role_id ON bdopsflow_datasource_permissions(role_id);
CREATE INDEX IF NOT EXISTS idx_ds_perms_user_id ON bdopsflow_datasource_permissions(user_id);
```

### 18. 查询历史表 (bdopsflow_query_history)

SQL查询执行历史记录。

```sql
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

CREATE INDEX IF NOT EXISTS idx_query_history_datasource_id ON bdopsflow_query_history(datasource_id);
CREATE INDEX IF NOT EXISTS idx_query_history_domain_id ON bdopsflow_query_history(domain_id);
CREATE INDEX IF NOT EXISTS idx_query_history_created_at ON bdopsflow_query_history(created_at);
CREATE INDEX IF NOT EXISTS idx_query_history_query_id ON bdopsflow_query_history(query_id);
```

### 19. 系统配置表 (bdopsflow_system_config)

系统级配置项，支持动态配置。

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_system_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_key TEXT NOT NULL UNIQUE,
    config_value TEXT NOT NULL,
    description TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_system_config_key ON bdopsflow_system_config(config_key);
```

### 20. 配置变更历史表 (bdopsflow_system_config_history)

系统配置变更审计记录。

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_system_config_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_key TEXT NOT NULL,
    old_value TEXT,
    new_value TEXT NOT NULL,
    changed_by INTEGER,
    changed_at TEXT NOT NULL,
    FOREIGN KEY (changed_by) REFERENCES bdopsflow_users(id)
);

CREATE INDEX IF NOT EXISTS idx_config_history_key ON bdopsflow_system_config_history(config_key);
CREATE INDEX IF NOT EXISTS idx_config_history_time ON bdopsflow_system_config_history(changed_at);
```

### 21. 审计日志表 (bdopsflow_audit_logs)

系统操作审计日志，记录所有关键操作。

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    username TEXT NOT NULL,
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
```

## 初始化数据

### 默认领域

```sql
INSERT OR IGNORE INTO bdopsflow_domains (name, description)
VALUES ('default', '默认领域');
```

### 默认管理员用户

密码：`admin123` (bcrypt 加密)

```sql
INSERT OR IGNORE INTO bdopsflow_users (username, real_name, phone, password, email, domain_id, role, is_active) 
VALUES ('admin', '系统管理员', '', '$2a$10$V4DeC68lOaLwF6N1pAVR8ux7WzY9NOeuPgwrAkyF9XcpWOL9muEaG', 'admin@example.com', 1, 'system_admin', 1);
```

### 默认角色

```sql
INSERT OR IGNORE INTO bdopsflow_roles (name, code, description, is_system, domain_id) VALUES
('系统管理员', 'system_admin', '系统最高权限，可管理所有资源', 1, NULL),
('领域管理员', 'domain_admin', '领域级管理权限', 1, NULL),
('普通用户', 'user', '基础查看和操作权限', 1, NULL);
```

### 默认权限

```sql
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
('workflow', 'manage', '完整管理工作流'),

-- 数据源管理权限
('datasource', 'create', '创建数据源'),
('datasource', 'read', '查看数据源'),
('datasource', 'update', '更新数据源'),
('datasource', 'delete', '删除数据源'),
('datasource', 'manage', '完整管理数据源'),
('datasource', 'query', '查询数据'),
('datasource', 'download', '下载数据'),

-- 审计日志权限
('audit_log', 'read', '查看审计日志'),
('audit_log', 'delete', '删除审计日志'),
('audit_log', 'manage', '完整管理审计日志');
```

### 角色权限分配

```sql
-- 为系统管理员分配所有权限
INSERT OR IGNORE INTO bdopsflow_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM bdopsflow_roles r, bdopsflow_permissions p
WHERE r.code = 'system_admin';

-- 为领域管理员分配任务、执行器、日志、工作流、权限、数据源的权限
INSERT OR IGNORE INTO bdopsflow_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM bdopsflow_roles r, bdopsflow_permissions p
WHERE r.code = 'domain_admin'
AND p.resource IN ('task', 'executor', 'log', 'workflow', 'permission', 'datasource');

-- 为普通用户分配查看和手动触发权限
INSERT OR IGNORE INTO bdopsflow_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM bdopsflow_roles r, bdopsflow_permissions p
WHERE r.code = 'user'
AND p.action IN ('read', 'trigger');
```

### 管理员角色绑定

```sql
INSERT OR IGNORE INTO bdopsflow_user_roles (user_id, role_id, domain_id)
SELECT u.id, r.id, NULL FROM bdopsflow_users u, bdopsflow_roles r
WHERE u.username = 'admin' AND r.code = 'system_admin';
```

### 系统配置初始化

```sql
INSERT OR IGNORE INTO bdopsflow_system_config (config_key, config_value, description, updated_at) VALUES
('web.enabled', 'false', '是否启用内置Web UI，启用后可通过调度器端口直接访问', datetime('now')),
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
('audit_log.retention_days', '90', '审计日志保留天数', datetime('now'));
```

## 配置说明

### 开发环境配置 (单节点)

```yaml
database:
  rqlite_addrs:
    - "http://localhost:4001"
  rqlite_user: ""
  rqlite_password: ""
  rqlite_tls: false
```

### 生产环境配置 (集群)

```yaml
database:
  rqlite_addrs:
    - "http://rqlite1:4001"
    - "http://rqlite2:4001"
    - "http://rqlite3:4001"
  rqlite_user: "admin"
  rqlite_password: "your-secure-password"
  rqlite_tls: true  # 建议开启 TLS
```

### rqlite 服务端认证配置

rqlite 支持 HTTP Basic Auth 和 TLS，需要在启动时配置：

```bash
# 启动带认证的 rqlite
rqlited -node-id 1 \
  -http-addr 0.0.0.0:4001 \
  -raft-addr 0.0.0.0:4002 \
  -data-dir /data \
  -auth /path/to/auth.json
```

`auth.json` 格式：

```json
{
  "single": [
    {
      "username": "admin",
      "password": "your-password"
    }
  ]
}
```

## 备份与恢复

### 备份

rqlite 提供多种备份方式：

```bash
# 在线备份
curl http://localhost:4001/db/backup?pretty

# 使用快照（生产环境推荐）
curl http://localhost:4001/snapshot > backup.snapshot
```

### 恢复

```bash
# 从快照恢复
curl -X POST http://localhost:4001/snapshot \
  --data-binary @backup.snapshot
```

## 性能优化

### 建议

1. **索引优化**：根据查询频率添加合适的索引
2. **批量写入**：使用事务批量写入数据
3. **定期清理**：定期清理过期的执行记录和日志
4. **监控**：监控数据库大小和查询延迟

### 日志保留策略

建议配置日志保留策略：

```sql
-- 保留最近 30 天的执行记录
DELETE FROM bdopsflow_task_executions 
WHERE created_at < datetime('now', '-30 days');

-- 保留最近 7 天的日志
DELETE FROM bdopsflow_task_logs 
WHERE log_time < datetime('now', '-7 days');

-- 保留最近 90 天的审计日志（可通过 audit_log.retention_days 配置调整）
DELETE FROM bdopsflow_audit_logs 
WHERE created_at < datetime('now', '-90 days');

-- 保留最近 30 天的查询历史（可通过 datasource.history_retention_days 配置调整）
DELETE FROM bdopsflow_query_history 
WHERE created_at < datetime('now', '-30 days');
```

## 相关文档

- [架构设计](ARCHITECTURE.md) - 系统架构
- [部署指南](DEPLOYMENT.md) - 详细部署步骤
- [开发指南](DEVELOPMENT.md) - 开发环境配置
