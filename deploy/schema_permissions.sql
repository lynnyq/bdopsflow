-- BDopsFlow 权限管理系统数据库迁移脚本
-- 版本：v2.0
-- 日期：2026-05-14
-- 描述：添加用户权限与多租户管理系统所需的数据表

-- ============================================================================
-- 第一部分：创建新的权限相关表
-- ============================================================================

-- 1. 角色表
CREATE TABLE IF NOT EXISTS roles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    code TEXT NOT NULL,
    description TEXT,
    is_system BOOLEAN DEFAULT 0,
    domain_id INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (domain_id) REFERENCES domains(id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_roles_name ON roles(name);
CREATE UNIQUE INDEX IF NOT EXISTS idx_roles_code ON roles(code);
CREATE INDEX IF NOT EXISTS idx_roles_domain_id ON roles(domain_id);

-- 2. 权限表
CREATE TABLE IF NOT EXISTS permissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    resource TEXT NOT NULL,
    action TEXT NOT NULL,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(resource, action)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_permissions_resource_action ON permissions(resource, action);

-- 3. 角色权限映射表
CREATE TABLE IF NOT EXISTS role_permissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    role_id INTEGER NOT NULL,
    permission_id INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
    FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE,
    UNIQUE(role_id, permission_id)
);

CREATE INDEX IF NOT EXISTS idx_role_permissions_role_id ON role_permissions(role_id);
CREATE INDEX IF NOT EXISTS idx_role_permissions_permission_id ON role_permissions(permission_id);

-- 4. 用户角色映射表
CREATE TABLE IF NOT EXISTS user_roles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    role_id INTEGER NOT NULL,
    domain_id INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
    FOREIGN KEY (domain_id) REFERENCES domains(id) ON DELETE SET NULL,
    UNIQUE(user_id, role_id, domain_id)
);

CREATE INDEX IF NOT EXISTS idx_user_roles_user_id ON user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_role_id ON user_roles(role_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_domain_id ON user_roles(domain_id);

-- 5. 执行器领域分配表
CREATE TABLE IF NOT EXISTS domain_executors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain_id INTEGER NOT NULL,
    executor_id INTEGER NOT NULL,
    assigned_by INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (domain_id) REFERENCES domains(id) ON DELETE CASCADE,
    FOREIGN KEY (executor_id) REFERENCES executors(id) ON DELETE CASCADE,
    FOREIGN KEY (assigned_by) REFERENCES users(id) ON DELETE SET NULL,
    UNIQUE(domain_id, executor_id)
);

CREATE INDEX IF NOT EXISTS idx_domain_executors_domain_id ON domain_executors(domain_id);
CREATE INDEX IF NOT EXISTS idx_domain_executors_executor_id ON domain_executors(executor_id);

-- ============================================================================
-- 第二部分：修改现有表
-- ============================================================================

-- 6. users 表新增字段
ALTER TABLE users ADD COLUMN is_active BOOLEAN DEFAULT 1;
ALTER TABLE users ADD COLUMN last_login_at DATETIME;
ALTER TABLE users ADD COLUMN created_by INTEGER;

-- 7. executors 表新增字段
ALTER TABLE executors ADD COLUMN is_global BOOLEAN DEFAULT 0;

-- ============================================================================
-- 第三部分：初始化数据
-- ============================================================================

-- 8. 插入预设角色
INSERT OR IGNORE INTO roles (name, code, description, is_system, domain_id) VALUES
('系统管理员', 'system_admin', '系统最高权限，可管理所有资源', 1, NULL),
('领域管理员', 'domain_admin', '领域级管理权限', 1, NULL),
('普通用户', 'user', '基础查看和操作权限', 1, NULL);

-- 9. 插入所有权限定义
INSERT OR IGNORE INTO permissions (resource, action, description) VALUES
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

-- 10. 为系统管理员分配所有权限
INSERT OR IGNORE INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.code = 'system_admin';

-- 11. 为领域管理员分配任务、执行器、日志、工作流的全部权限
INSERT OR IGNORE INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.code = 'domain_admin'
AND p.resource IN ('task', 'executor', 'log', 'workflow', 'permission');

-- 12. 为普通用户分配查看和手动触发权限
INSERT OR IGNORE INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.code = 'user'
AND p.action IN ('read', 'trigger');

-- 13. 将 admin 用户设置为系统管理员
INSERT OR IGNORE INTO user_roles (user_id, role_id, domain_id)
SELECT u.id, r.id, NULL FROM users u, roles r
WHERE u.username = 'admin' AND r.code = 'system_admin';

-- 14. 将 admin 用户标记为激活状态
UPDATE users SET is_active = 1 WHERE username = 'admin';

-- ============================================================================
-- 第四部分：数据验证
-- ============================================================================

-- 验证角色数量
-- SELECT '角色数量:', COUNT(*) FROM roles;

-- 验证权限数量
-- SELECT '权限数量:', COUNT(*) FROM permissions;

-- 验证角色权限关联
-- SELECT '角色权限关联数:', COUNT(*) FROM role_permissions;

-- 验证用户角色关联
-- SELECT '用户角色关联数:', COUNT(*) FROM user_roles;

-- ============================================================================
-- 迁移完成
-- ============================================================================

-- 输出迁移成功信息（仅用于开发调试，生产环境请删除）
-- SELECT 'Migration v2.0 completed successfully!' AS message;
