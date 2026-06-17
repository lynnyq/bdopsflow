-- BDopsFlow 迁移脚本
-- 版本：v4
-- 日期：2026-06-17
-- 描述：新增接口测试模块（HTTP/gRPC接口测试、Proto文件管理、证书管理）

PRAGMA foreign_keys = OFF;

-- ============================================================================
-- 1. 创建接口测试用例表
-- ============================================================================
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

-- ============================================================================
-- 2. 创建Proto文件表
-- ============================================================================
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

-- ============================================================================
-- 3. 创建证书文件表
-- ============================================================================
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

-- ============================================================================
-- 4. 创建接口测试结果表
-- ============================================================================
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
-- 5. 新增接口测试权限
-- ============================================================================
INSERT OR IGNORE INTO bdopsflow_permissions (resource, action, description) VALUES
('api_test', 'create', '创建接口测试'),
('api_test', 'read', '查看接口测试'),
('api_test', 'update', '更新接口测试'),
('api_test', 'delete', '删除接口测试'),
('api_test', 'execute', '执行接口测试'),
('api_test', 'manage', '完整管理接口测试');

-- ============================================================================
-- 6. 为领域管理员角色分配接口测试权限
-- ============================================================================
INSERT OR IGNORE INTO bdopsflow_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM bdopsflow_roles r, bdopsflow_permissions p
WHERE r.code = 'domain_admin'
AND p.resource = 'api_test';

PRAGMA foreign_keys = ON;
