# BDopsFlow 数据查询功能设计文档

**版本**: 2.1
**日期**: 2026-05-20
**状态**: 设计完成

***

## 修订历史

| 版本 | 日期 | 修订内容 |
|-----|------|---------|
| v1.0 | 2026-05-18 | 初始设计 |
| v1.1 | 2026-05-18 | 补充SQL执行安全控制、连接池、缓存设计等 |
| v1.3 | 2026-05-18 | 补充密码更新、查询取消、编辑器、错误码等 |
| v1.4 | 2026-05-19 | 修正认证类型、补充请求体参数等 |
| v1.5 | 2026-05-19 | 同步表定义、补充路由和菜单项等 |
| v1.6 | 2026-05-19 | 补充密钥管理、Redis策略、多调度器一致性等 |
| v2.0 | 2026-05-19 | 合并所有附录到正文，统一文档结构 |
| v2.1 | 2026-05-20 | 全面审查优化：对齐项目代码风格（Model双tag、SQL IF NOT EXISTS、响应格式、Redis Key命名、导航菜单、API路径、权限初始化SQL等） |

***

## 1. 项目概述

### 1.1 背景

BDopsFlow 是一个分布式工作流调度平台，现已具备任务调度、权限管理、领域隔离等核心功能。为了满足用户的数据查询需求，需要新增一个独立的数据查询模块，支持多种类型的数据源连接和 SQL 查询。

### 1.2 目标

- 支持多种类型的数据源（MySQL, Hive, Trino, Spark, StarRocks, Doris, SQLite, Rqlite, Kyuubi）
- 提供数据源管理功能（新增、修改、删除、查看）
- 提供 SQL 查询编辑器，支持格式化、保存、执行
- 支持查询结果缓存和 CSV 导出
- 完善的权限控制和领域隔离
- 支持 LDAP/Kerberos 认证连接数据源

### 1.3 范围

本文档涵盖数据查询模块的完整设计，包括：

- 数据库表结构设计
- 权限设计
- Driver 接口设计
- API 设计
- 前端设计
- 安全设计
- 多调度器一致性设计
- 监控与运维设计
- 实施步骤

***

## 2. 数据库设计

### 2.1 数据源表 (bdopsflow_datasources)

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
    config TEXT,
    description TEXT,
    domain_id INTEGER NOT NULL,
    is_enabled BOOLEAN DEFAULT 1,
    test_status TEXT DEFAULT 'untested',
    last_test_at DATETIME,
    created_by INTEGER,
    updated_by INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (domain_id) REFERENCES bdopsflow_domains(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_datasources_domain ON bdopsflow_datasources(domain_id);
CREATE INDEX IF NOT EXISTS idx_datasources_type ON bdopsflow_datasources(type);
CREATE UNIQUE INDEX IF NOT EXISTS idx_datasources_name_domain ON bdopsflow_datasources(name, domain_id);
```

**字段说明**:

| 字段 | 类型 | 说明 |
|-----|------|------|
| `type` | TEXT | 数据源类型 (mysql, hive, trino, spark, kyuubi, starrocks, doris, sqlite, rqlite)，创建后不可修改 |
| `host/port` | TEXT/INTEGER | TCP 连接时使用 |
| `path` | TEXT | 文件路径（仅 SQLite 使用） |
| `password` | TEXT | AES-256-GCM 加密后的密码 |
| `auth_type` | TEXT | 认证类型 (simple, ldap, basic, none, kerberos) |
| `config` | TEXT | JSON 格式的扩展配置（LDAP 参数、SSL 配置等） |
| `description` | TEXT | 数据源描述 |
| `test_status` | TEXT | 测试状态 (untested, success, failed) |
| `is_enabled` | BOOLEAN | 启用状态，禁用后不可查询 |
| `updated_by` | INTEGER | 最后修改人 ID |

**业务规则**:

- 数据源名称在**领域内唯一**，不同领域可以有同名数据源
- 数据源类型 `type` 字段**创建后不可修改**
- 禁用数据源后：禁止查询、禁止获取元数据、关闭连接池，但保留权限分配和保存的SQL

### 2.2 保存的SQL表 (bdopsflow_saved_sql)

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

CREATE INDEX IF NOT EXISTS idx_saved_sql_datasource ON bdopsflow_saved_sql(datasource_id);
CREATE INDEX IF NOT EXISTS idx_saved_sql_domain ON bdopsflow_saved_sql(domain_id);
```

**权限控制**:

| 操作 | 创建者 | 同领域用户 | 其他用户 |
|-----|-------|----------|---------|
| 查看公开SQL | ✅ | ✅ | ❌ |
| 查看私有SQL | ✅ | ❌ | ❌ |
| 编辑SQL | ✅ | ❌ | ❌ |
| 删除SQL | ✅ | ❌ | ❌ |
| 执行公开SQL | ✅ | ✅（需数据源权限） | ❌ |

- `is_public = true`：同领域用户可见可执行
- `is_public = false`：仅创建者可见
- 编辑和删除仅限创建者

### 2.3 数据源权限表 (bdopsflow_datasource_permissions)

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_datasource_permissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    datasource_id INTEGER NOT NULL,
    role_id INTEGER NOT NULL,
    permission_type TEXT NOT NULL,
    granted_by INTEGER,
    granted_at TEXT NOT NULL,
    FOREIGN KEY (datasource_id) REFERENCES bdopsflow_datasources(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES bdopsflow_roles(id) ON DELETE CASCADE,
    UNIQUE(datasource_id, role_id, permission_type)
);

CREATE INDEX IF NOT EXISTS idx_ds_perms_datasource ON bdopsflow_datasource_permissions(datasource_id);
CREATE INDEX IF NOT EXISTS idx_ds_perms_role ON bdopsflow_datasource_permissions(role_id);
```

**permission_type**:

- `query`: 查询权限
- `download`: 下载权限

### 2.4 查询历史表 (bdopsflow_query_history)

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

CREATE INDEX IF NOT EXISTS idx_query_history_datasource ON bdopsflow_query_history(datasource_id);
CREATE INDEX IF NOT EXISTS idx_query_history_domain ON bdopsflow_query_history(domain_id);
CREATE INDEX IF NOT EXISTS idx_query_history_created ON bdopsflow_query_history(created_at);
CREATE INDEX IF NOT EXISTS idx_query_history_query_id ON bdopsflow_query_history(query_id);
```

**字段说明**:

- `query_id`: 查询唯一标识，格式为 `q_{timestamp}_{random}`，用于查询取消功能
- `datasource_id`: 可空字段，删除数据源时置 NULL
- `datasource_name`: 冗余存储数据源名称，删除后仍可查看
- `database`: 查询时使用的数据库上下文

**清理策略**:

- 默认保留 30 天的查询历史
- 可通过系统配置 `datasource.history_retention_days` 调整
- 定时任务每天凌晨自动清理过期记录
- 清理时按领域隔离

### 2.5 系统配置表 (bdopsflow_system_config)

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_system_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_key TEXT NOT NULL UNIQUE,
    config_value TEXT NOT NULL,
    description TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_system_config_key ON bdopsflow_system_config(config_key);

-- 初始化默认配置
INSERT OR IGNORE INTO bdopsflow_system_config (config_key, config_value, description, updated_at) VALUES
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
('datasource.test_timeout', '10', '连接测试超时时间(秒)', datetime('now'));
```

### 2.6 配置变更历史表 (bdopsflow_system_config_history)

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

### 2.7 权限初始化数据

数据源模块需在 `deploy/schema.sql` 中追加以下权限初始化语句，与现有权限初始化风格一致：

```sql
-- 数据源管理权限
INSERT OR IGNORE INTO bdopsflow_permissions (resource, action, description) VALUES
('datasource', 'create', '创建数据源'),
('datasource', 'read', '查看数据源'),
('datasource', 'update', '更新数据源'),
('datasource', 'delete', '删除数据源'),
('datasource', 'manage', '完整管理数据源'),
('datasource', 'query', '查询数据'),
('datasource', 'download', '下载数据');

-- 为系统管理员分配数据源所有权限
INSERT OR IGNORE INTO bdopsflow_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM bdopsflow_roles r, bdopsflow_permissions p
WHERE r.code = 'system_admin' AND p.resource = 'datasource';

-- 为领域管理员分配数据源管理权限（不含 delete）
INSERT OR IGNORE INTO bdopsflow_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM bdopsflow_roles r, bdopsflow_permissions p
WHERE r.code = 'domain_admin' AND p.resource = 'datasource' AND p.action IN ('create', 'read', 'update', 'manage', 'query', 'download');

-- 为普通用户分配数据源查看和查询权限
INSERT OR IGNORE INTO bdopsflow_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM bdopsflow_roles r, bdopsflow_permissions p
WHERE r.code = 'user' AND p.resource = 'datasource' AND p.action IN ('read', 'query');
```

***

## 3. 权限设计

### 3.1 新增权限资源类型

在现有的 Permission 模型中新增：

```go
{
    Resource:     "datasource",
    ResourceName: "数据源管理",
    Permissions: []Permission{
        {Resource: "datasource", Action: "create", Description: "创建数据源"},
        {Resource: "datasource", Action: "read", Description: "查看数据源"},
        {Resource: "datasource", Action: "update", Description: "更新数据源"},
        {Resource: "datasource", Action: "delete", Description: "删除数据源"},
        {Resource: "datasource", Action: "manage", Description: "完整管理数据源"},
        {Resource: "datasource", Action: "query", Description: "查询数据"},
        {Resource: "datasource", Action: "download", Description: "下载数据"},
    },
}
```

### 3.2 权限控制矩阵

| 操作 | 系统管理员 | 领域管理员 | 普通用户 |
|-----|----------|----------|---------|
| 查看所有领域数据源 | ✅ | ❌ | ❌ |
| 查看本领域数据源 | ✅ | ✅ | ❌ |
| 查看有权限的数据源 | ✅ | ✅ | ✅ |
| 创建数据源 | ✅ | ✅（本领域） | ❌ |
| 更新数据源 | ✅ | ✅（本领域） | ❌ |
| 删除数据源 | ✅ | ✅（本领域） | ❌ |
| 分配数据源权限 | ✅ | ✅（本领域） | ❌ |
| 查询数据源 | ✅ | ✅（授权） | ✅（授权） |
| 下载查询结果 | ✅ | ✅（授权） | ✅（授权） |

### 3.3 数据源权限检查逻辑

1. 系统管理员可以访问所有数据源
2. 领域管理员可以访问本领域的所有数据源
3. 普通用户只能访问被授权的数据源
4. 权限检查通过 `bdopsflow_datasource_permissions` 表实现
5. 权限检查时同时验证操作者的角色类型和资源所属领域

### 3.4 权限中间件设计

现有项目使用 `middleware.RBACMiddleware()` 和 `middleware.RequireSystemAdmin()` 基于角色的中间件。数据源模块需要细粒度的资源+动作权限控制，新增数据源专用权限中间件：

```go
func DatasourcePermissionMiddleware(dsService *service.DatasourceService, action string) gin.HandlerFunc {
    return func(c *gin.Context) {
        userRole, _ := c.Get("role")
        domainID, _ := c.Get("domain_id")
        userID, _ := c.Get("user_id")

        role := userRole.(string)
        dID := domainID.(int64)
        uID := userID.(int64)

        if role == "system_admin" || role == "admin" {
            c.Next()
            return
        }

        dsIDStr := c.Param("id")
        if dsIDStr == "" {
            dsIDStr = c.Query("datasource_id")
        }
        if dsIDStr == "" {
            handler.Forbidden(c, "datasource ID required")
            c.Abort()
            return
        }

        dsID, _ := strconv.ParseInt(dsIDStr, 10, 64)
        ds, err := dsService.GetDatasource(c.Request.Context(), dsID)
        if err != nil {
            handler.NotFound(c, "datasource not found")
            c.Abort()
            return
        }

        if role == "domain_admin" && ds.DomainID == dID {
            c.Next()
            return
        }

        hasPerm, err := dsService.CheckDatasourcePermission(c.Request.Context(), uID, dsID, action)
        if err != nil || !hasPerm {
            handler.Forbidden(c, "insufficient datasource permission")
            c.Abort()
            return
        }

        c.Next()
    }
}
```

**中间件使用方式**（与现有路由注册风格一致）：

```go
datasources := protected.Group("/datasources")
{
    datasources.GET("", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), dsHandler.List)
    datasources.POST("", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), dsHandler.Create)
    datasources.GET("/:id", DatasourcePermissionMiddleware(dsService, "read"), dsHandler.Get)
    datasources.PUT("/:id", DatasourcePermissionMiddleware(dsService, "update"), dsHandler.Update)
    datasources.DELETE("/:id", DatasourcePermissionMiddleware(dsService, "delete"), dsHandler.Delete)
    datasources.POST("/:id/test", DatasourcePermissionMiddleware(dsService, "read"), dsHandler.TestConnection)
}

query := protected.Group("/query")
{
    query.POST("/execute", DatasourcePermissionMiddleware(dsService, "query"), queryHandler.Execute)
    query.POST("/cancel/:query_id", DatasourcePermissionMiddleware(dsService, "query"), queryHandler.Cancel)
    query.POST("/export", DatasourcePermissionMiddleware(dsService, "download"), queryHandler.Export)
}
```

**与现有中间件的关系**：

| 中间件 | 适用场景 | 数据源模块使用 |
|-------|---------|-------------|
| `RBACMiddleware` | 基于角色的粗粒度控制 | 数据源列表、创建 |
| `RequireSystemAdmin` | 仅系统管理员 | 系统配置管理 |
| `DatasourcePermissionMiddleware` | 数据源细粒度权限 | 查询、下载、单数据源操作 |

***

## 4. Driver 接口设计

### 4.1 统一 Driver 接口

```go
package datasource

type Driver interface {
    Connect(ctx context.Context, config DatasourceConfig) error
    
    TestConnection(ctx context.Context) error
    
    Close() error
    
    Query(ctx context.Context, sql string, args ...interface{}) (*QueryResult, error)
    
    GetDatabases(ctx context.Context) ([]string, error)
    
    GetTables(ctx context.Context, database string) ([]TableInfo, error)
    
    GetColumns(ctx context.Context, database, table string) ([]ColumnInfo, error)
    
    SupportsCancel() bool
}

type DatasourceConfig struct {
    Type               string                 `json:"type"`
    Host               string                 `json:"host"`
    Port               int                    `json:"port"`
    Path               string                 `json:"path"`
    Database           string                 `json:"database"`
    Username           string                 `json:"username"`
    Password           string                 `json:"password"`
    AuthType           string                 `json:"auth_type"`
    ConnectionMode     string                 `json:"connection_mode"`
    ZookeeperQuorum    string                 `json:"zookeeper_quorum"`
    ZookeeperNamespace string                 `json:"zookeeper_namespace"`
    Config             map[string]interface{} `json:"config"`
}

type QueryResult struct {
    Columns  []string
    Rows     [][]interface{}
    RowCount int64
}

type TableInfo struct {
    Name    string `json:"name"`
    Comment string `json:"comment"`
}

type ColumnInfo struct {
    Name     string `json:"name"`
    Type     string `json:"type"`
    Comment  string `json:"comment"`
    Nullable bool   `json:"nullable"`
}
```

### 4.1.1 数据模型定义

与现有 `model/models.go` 风格一致，使用 `db:"column_name" json:"field_name"` 双 tag：

```go
package model

import "time"

type Datasource struct {
    ID          int64      `db:"id" json:"id"`
    Name        string     `db:"name" json:"name"`
    Type        string     `db:"type" json:"type"`
    Host        string     `db:"host" json:"host,omitempty"`
    Port        int        `db:"port" json:"port,omitempty"`
    Path        string     `db:"path" json:"path,omitempty"`
    Database    string     `db:"database" json:"database,omitempty"`
    Username    string     `db:"username" json:"username,omitempty"`
    Password    string     `db:"password" json:"-"` // API 响应不返回密码
    AuthType    string     `db:"auth_type" json:"auth_type"`
    Config      string     `db:"config" json:"config,omitempty"`
    Description string     `db:"description" json:"description,omitempty"`
    DomainID    int64      `db:"domain_id" json:"domain_id"`
    IsEnabled   bool       `db:"is_enabled" json:"is_enabled"`
    TestStatus  string     `db:"test_status" json:"test_status"`
    LastTestAt  *time.Time `db:"last_test_at" json:"last_test_at,omitempty"`
    CreatedBy   *int64     `db:"created_by" json:"created_by,omitempty"`
    UpdatedBy   *int64     `db:"updated_by" json:"updated_by,omitempty"`
    CreatedAt   time.Time  `db:"created_at" json:"created_at"`
    UpdatedAt   time.Time  `db:"updated_at" json:"updated_at"`
}

type SavedSQL struct {
    ID           int64     `db:"id" json:"id"`
    Name         string    `db:"name" json:"name"`
    DatasourceID int64     `db:"datasource_id" json:"datasource_id"`
    SQLText      string    `db:"sql_text" json:"sql_text"`
    Description  string    `db:"description" json:"description,omitempty"`
    CreatedBy    *int64    `db:"created_by" json:"created_by,omitempty"`
    UpdatedBy    *int64    `db:"updated_by" json:"updated_by,omitempty"`
    DomainID     int64     `db:"domain_id" json:"domain_id"`
    IsPublic     bool      `db:"is_public" json:"is_public"`
    CreatedAt    time.Time `db:"created_at" json:"created_at"`
    UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

type DatasourcePermission struct {
    ID             int64  `db:"id" json:"id"`
    DatasourceID   int64  `db:"datasource_id" json:"datasource_id"`
    RoleID         int64  `db:"role_id" json:"role_id"`
    PermissionType string `db:"permission_type" json:"permission_type"`
    GrantedBy      *int64 `db:"granted_by" json:"granted_by,omitempty"`
    GrantedAt      string `db:"granted_at" json:"granted_at"`
}

type QueryHistory struct {
    ID             int64     `db:"id" json:"id"`
    QueryID        string    `db:"query_id" json:"query_id,omitempty"`
    DatasourceID   *int64    `db:"datasource_id" json:"datasource_id,omitempty"`
    DatasourceName string    `db:"datasource_name" json:"datasource_name,omitempty"`
    SQLText        string    `db:"sql_text" json:"sql_text"`
    Database       string    `db:"database" json:"database,omitempty"`
    ExecutionTime  float64   `db:"execution_time" json:"execution_time,omitempty"`
    RowCount       int       `db:"row_count" json:"row_count,omitempty"`
    Status         string    `db:"status" json:"status"`
    ErrorMessage   string    `db:"error_message" json:"error_message,omitempty"`
    ExecutedBy     *int64    `db:"executed_by" json:"executed_by,omitempty"`
    DomainID       int64     `db:"domain_id" json:"domain_id"`
    CreatedAt      time.Time `db:"created_at" json:"created_at"`
}

type SystemConfig struct {
    ID          int64     `db:"id" json:"id"`
    ConfigKey   string    `db:"config_key" json:"config_key"`
    ConfigValue string    `db:"config_value" json:"config_value"`
    Description string    `db:"description" json:"description,omitempty"`
    UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

type SystemConfigHistory struct {
    ID         int64  `db:"id" json:"id"`
    ConfigKey  string `db:"config_key" json:"config_key"`
    OldValue   string `db:"old_value" json:"old_value,omitempty"`
    NewValue   string `db:"new_value" json:"new_value"`
    ChangedBy  *int64 `db:"changed_by" json:"changed_by,omitempty"`
    ChangedAt  string `db:"changed_at" json:"changed_at"`
}
```

**说明**：
- `Datasource.Password` 使用 `json:"-"` 标签，API 响应中不返回密码
- 可空字段使用指针类型 `*int64`、`*time.Time`，与现有 `models.go` 风格一致
- `DatasourceConfig`（Driver 接口用）和 `Datasource`（数据库模型）是两个不同结构体，前者只有 `json` tag，后者有 `db` + `json` 双 tag

### 4.2 Driver 注册表

```go
var driverRegistry = make(map[string]DriverFactory)

type DriverFactory func() Driver

func RegisterDriver(dsType string, factory DriverFactory) {
    driverRegistry[dsType] = factory
}

func GetDriver(dsType string) (Driver, error) {
    factory, ok := driverRegistry[dsType]
    if !ok {
        return nil, fmt.Errorf("unsupported datasource type: %s", dsType)
    }
    return factory(), nil
}
```

### 4.3 支持的数据源类型

| 数据源类型 | 简单密码认证 | LDAP认证 | BasicAuth | Kerberos | 推荐Go库 | 成熟度 | 取消查询支持 |
|----------|------------|---------|----------|---------|---------|-------|------------|
| mysql | ✅ | ❌ | ❌ | ❌ | github.com/go-sql-driver/mysql v1.7.1 | 🟢高 | ✅ |
| hive | ✅ | ✅ | ❌ | ✅ | github.com/beltran/gohive v1.2.0 | 🟡中 | ⚠️需验证 |
| trino | ✅ | ✅ | ❌ | ❌ | github.com/trinodb/trino-go-client v0.6.0 | 🟡中 | ✅ |
| spark | ✅ | ✅ | ❌ | ✅ | 复用gohive | 🟡中 | ⚠️ |
| kyuubi | ✅ | ✅ | ❌ | ✅ | github.com/beltran/gohive v1.2.0 | 🟡中 | ⚠️需验证 |
| starrocks | ✅ | ✅ | ❌ | ❌ | 复用mysql driver | 🟢高 | ✅ |
| doris | ✅ | ✅ | ❌ | ❌ | 复用mysql driver | 🟢高 | ✅ |
| sqlite | ❌ | ❌ | ❌ | ❌ | github.com/mattn/go-sqlite3 | 🟢高 | ✅ |
| rqlite | ❌ | ❌ | ✅ | ❌ | github.com/rqlite/gorqlite v1.24.3 | 🟡中 | ❌ |

**说明**:
- 不支持取消查询的驱动在UI上隐藏取消按钮
- Spark 通过 Thrift Server 连接，复用 Hive Driver
- StarRocks/Doris 兼容 MySQL 协议
- LDAP/Kerberos 认证：客户端只需提供用户凭证，HiveServer2/Trino Coordinator 负责与 LDAP/KDC 交互验证

### 4.4 认证配置详解

#### LDAP/Kerberos 认证原理说明

**重要理解**：Hive/Trino/Spark 等大数据组件的 LDAP/Kerberos 认证**不是**客户端直接连接 LDAP/KDC 服务器进行认证，而是：

1. **服务端已配置**：HiveServer2/Trino Coordinator 等服务端已经配置了 LDAP 服务器或 Kerberos KDC 的信息
2. **客户端认证流程**：
   - 客户端连接 HiveServer2 的 10000 端口（或 Trino 的 8080 端口）
   - 客户端提供 LDAP 用户名和密码（或 Kerberos principal）
   - **服务端作为代理**，将凭证转发给配置的 LDAP/KDC 进行验证
   - 验证通过后，服务端允许客户端连接

3. **配置职责划分**：
   - **服务端配置**：LDAP URL、Base DN、Kerberos realm 等（在 hadoop 集群侧配置）
   - **客户端配置**：只需提供连接地址和用户凭证

#### 简单密码认证 (Hive/Kyuubi/Spark)

HiveServer2/Kyuubi Thrift Server/Spark Thrift Server 有两种连接方式：

**方式一：直接连接单个 HiveServer2 实例（简单模式）**

适合单节点测试环境，直接连接 HiveServer2 的 10000 端口：

```json
{
  "auth_type": "simple",
  "connection_mode": "direct",
  "host": "hiveserver2.example.com",
  "port": 10000,
  "username": "hive_user",
  "password": "plain_password",
  "config": {
    "transport_mode": "binary",
    "ssl": false
  }
}
```

**方式二：ZooKeeper 动态服务发现（高可用模式）**

适合生产环境，通过 ZooKeeper 自动发现可用的 HiveServer2 实例：

```json
{
  "auth_type": "simple",
  "connection_mode": "zookeeper",
  "zookeeper_quorum": "zk1.example.com:2181,zk2.example.com:2181,zk3.example.com:2181",
  "zookeeper_namespace": "hiveserver2",
  "username": "hive_user",
  "password": "plain_password",
  "config": {
    "transport_mode": "binary",
    "ssl": false
  }
}
```

**参数说明**：
两种模式的参数与 LDAP 认证相同，请参考上文 LDAP 认证部分的参数说明。

#### LDAP 认证 (Hive/Kyuubi/Spark)

HiveServer2/Kyuubi Thrift Server/Spark Thrift Server 有两种连接方式：

**方式一：直接连接单个 HiveServer2 实例（简单模式）**

适合单节点测试环境，直接连接 HiveServer2 的 10000 端口：

```json
{
  "auth_type": "ldap",
  "connection_mode": "direct",
  "host": "hiveserver2.example.com",
  "port": 10000,
  "username": "ldap_username",
  "password": "ldap_password",
  "config": {
    "transport_mode": "binary",
    "ssl": false
  }
}
```

**方式二：ZooKeeper 动态服务发现（高可用模式）**

适合生产环境，通过 ZooKeeper 自动发现可用的 HiveServer2 实例：

```json
{
  "auth_type": "ldap",
  "connection_mode": "zookeeper",
  "zookeeper_quorum": "zk1.example.com:2181,zk2.example.com:2181,zk3.example.com:2181",
  "zookeeper_namespace": "hiveserver2",
  "username": "ldap_username",
  "password": "ldap_password",
  "config": {
    "transport_mode": "binary",
    "ssl": false
  }
}
```

**参数说明（两种模式共用）**：

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| `connection_mode` | string | ✅ | 连接模式：`direct`（直接连接）或 `zookeeper`（ZK 服务发现） |
| `username` | string | ✅ | LDAP 用户名 |
| `password` | string | ✅ | LDAP 密码 |
| `transport_mode` | string | ❌ | 传输模式：`binary`（默认）或 `http` |
| `http_path` | string | ❌ | HTTP 模式下的路径，如 `cliservice` |
| `ssl` | boolean | ❌ | 是否启用 SSL |

**直接连接模式特有参数**：

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| `host` | string | ✅ | HiveServer2 主机地址 |
| `port` | int | ✅ | HiveServer2 端口，默认 10000 |

**ZooKeeper 发现模式特有参数**：

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| `zookeeper_quorum` | string | ✅ | ZooKeeper 集群地址，逗号分隔，如 `zk1:2181,zk2:2181` |
| `zookeeper_namespace` | string | ✅ | ZooKeeper 命名空间，默认 `hiveserver2`，与服务端配置一致 |

**连接原理**：
- **ZooKeeper 模式**：客户端连接 ZooKeeper，获取随机选择的 HiveServer2 实例地址，然后连接该实例
- **负载均衡**：ZooKeeper 随机选择可用实例，实现负载均衡
- **高可用**：某个实例挂掉后，重新连接时会自动选择其他可用实例

#### LDAP 认证 (Trino)

Trino Coordinator 已配置 LDAP 服务端：

```json
{
  "auth_type": "ldap",
  "host": "trino.example.com",
  "port": 8080,
  "username": "ldap_username",
  "password": "ldap_password",
  "config": {
    "ssl": true,
    "catalog": "hive",
    "schema": "default"
  }
}
```

**参数说明**：

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| `host` | string | ✅ | Trino Coordinator 服务地址 |
| `port` | int | ✅ | Trino 端口，默认 8080（HTTPS 为 7778） |
| `username` | string | ✅ | LDAP 用户名（由 Trino 转发给 LDAP 验证） |
| `password` | string | ✅ | LDAP 密码 |
| `ssl` | boolean | ❌ | 是否使用 HTTPS（开启 LDAP 后必须启用） |
| `catalog` | string | ❌ | 默认连接的 catalog |
| `schema` | string | ❌ | 默认连接的 schema |

#### LDAP 服务端配置参考（供参考）

以下配置在 HiveServer2/Trino 服务端进行（由集群管理员配置），客户端无需关心：

**HiveServer2 (hive-site.xml)**:
```xml
<property>
  <name>hive.server2.authentication</name>
  <value>LDAP</value>
</property>
<property>
  <name>hive.server2.authentication.ldap.url</name>
  <value>ldap://ldap.example.com:389</value>
</property>
<property>
  <name>hive.server2.authentication.ldap.baseDN</name>
  <value>ou=people,dc=example,dc=com</value>
</property>
```

**Trino (config.properties)**:
```properties
http-server.authentication.type=LDAP
ldap.url=ldap://ldap.example.com:389
ldap.user-bind-pattern=uid=${USER},ou=people,dc=example,dc=com
```

#### Kerberos 认证（预留）

```json
{
  "auth_type": "kerberos",
  "krb5_conf": "/etc/krb5.conf",
  "client_keytab": "/path/to/client.keytab",
  "principal": "hive@EXAMPLE.COM",
  "service_principal": "hive/_HOST@EXAMPLE.COM"
}
```

#### Basic Auth 认证 (Rqlite)

```json
{
  "auth_type": "basic",
  "username": "admin",
  "password": "encrypted_password",
  "hosts": ["10.0.0.1:4001", "10.0.0.2:4001", "10.0.0.3:4001"]
}
```

- Rqlite 支持多节点配置，`hosts` 为集群节点地址列表
- Driver 自动选择可用节点（Leader 优先），节点不可用时自动切换

### 4.5 密码加密与密钥管理

#### 加密算法

使用 AES-256-GCM 加密算法：

```go
type Crypto struct {
    key []byte
}

func NewCrypto(key string) (*Crypto, error) {
    if len(key) != 32 {
        return nil, fmt.Errorf("key must be 32 bytes")
    }
    return &Crypto{key: []byte(key)}, nil
}

func (c *Crypto) Encrypt(plaintext string) (string, error)
func (c *Crypto) Decrypt(ciphertext string) (string, error)
```

#### 密钥生成

```go
func GenerateEncryptionKey() ([]byte, error) {
    key := make([]byte, 32)
    _, err := rand.Read(key)
    if err != nil {
        return nil, err
    }
    return key, nil
}
```

#### 密钥存储配置

```go
type CryptoConfig struct {
    KeySource      string `yaml:"key_source"`       // "env", "file", "kms", "direct"
    KeyEnvVar      string `yaml:"key_env_var"`      // 环境变量名称
    KeyFile        string `yaml:"key_file"`         // 密钥文件路径
    AutoRotateDays int    `yaml:"auto_rotate_days"` // 自动轮换天数，0表示不轮换
}
```

**优先级**:
1. 环境变量方式（推荐生产环境）
2. 文件方式
3. 配置文件直接方式（仅开发环境）
4. KMS方式（预留）

#### 密钥轮换机制

```go
type KeyManager struct {
    currentKey []byte
    oldKeys    [][]byte
    mutex      sync.RWMutex
}

func (km *KeyManager) RotateKey(newKey []byte) error {
    km.mutex.Lock()
    defer km.mutex.Unlock()
    
    if km.currentKey != nil {
        km.oldKeys = append([][]byte{km.currentKey}, km.oldKeys...)
    }
    km.currentKey = newKey
    return nil
}

func (km *KeyManager) Decrypt(ciphertext string) (string, error) {
    km.mutex.RLock()
    defer km.mutex.RUnlock()
    
    if plain, err := decryptWithKey(km.currentKey, ciphertext); err == nil {
        return plain, nil
    }
    
    for _, key := range km.oldKeys {
        if plain, err := decryptWithKey(key, ciphertext); err == nil {
            return plain, nil
        }
    }
    
    return "", fmt.Errorf("all keys failed to decrypt")
}
```

#### 密码重置/恢复机制

- 若丢失加密密钥，所有加密的数据源密码将无法解密
- 提供密码重置工具，支持：
  - 清除所有数据源密码（需管理员确认）
  - 导入导出加密密码（加密导出，需新密钥重新加密）
- 建议定期备份加密密钥

### 4.6 SSL/TLS 连接支持

| 数据源类型 | 支持 SSL | 配置方式 |
|----------|---------|---------|
| mysql | ✅ | config 中设置 `ssl: true` |
| hive | ✅ | config 中设置 `ssl: true` |
| trino | ✅ | 使用 HTTPS 协议 |
| spark | ✅ | config 中设置 `ssl: true` |
| kyuubi | ✅ | config 中设置 `ssl: true` |
| starrocks | ✅ | config 中设置 `ssl: true` |
| doris | ✅ | config 中设置 `ssl: true` |
| sqlite | ❌ | 本地文件，无需加密 |
| rqlite | ✅ | 使用 HTTPS 协议 |

**SSL 配置示例**:

```json
{
  "ssl": true,
  "ssl_skip_verify": false,
  "ssl_cert": "/path/to/client-cert.pem",
  "ssl_key": "/path/to/client-key.pem",
  "ssl_ca": "/path/to/ca-cert.pem"
}
```

### 4.7 数据源默认端口配置

| 数据源类型 | 默认端口 | 前端自动填充 |
|----------|---------|------------|
| mysql | 3306 | ✅ |
| hive | 10000 | ✅ |
| trino | 8080 | ✅ |
| spark | 10000 | ✅ (Thrift Server) |
| kyuubi | 10009 | ✅ |
| starrocks | 9030 | ✅ |
| doris | 9030 | ✅ |
| sqlite | - | 无端口 |
| rqlite | 4001 | ✅ |

***

## 5. 配置设计

### 5.1 配置文件 (scheduler/config.yaml)

```yaml
datasource:
  encryption_key: "your-32-byte-encryption-key-here-change-in-production"
  key_source: "env"
  key_env_var: "BDOPSFLOW_ENCRYPTION_KEY"
  key_file: ""
  auto_rotate_days: 0
```

### 5.2 配置结构

```go
type Config struct {
    DatasourceCrypto DatasourceCryptoConfig `yaml:"datasource"`
}

type DatasourceCryptoConfig struct {
    EncryptionKey   string `yaml:"encryption_key"`
    KeySource       string `yaml:"key_source"`
    KeyEnvVar       string `yaml:"key_env_var"`
    KeyFile         string `yaml:"key_file"`
    AutoRotateDays  int    `yaml:"auto_rotate_days"`
}
```

**命名说明**：
- `DatasourceCryptoConfig`：配置文件中的加密密钥配置，仅含 `encryption_key` 等敏感配置
- `DatasourceConfig`：Driver 接口使用的数据源连接配置，含 host/port/username/password 等
- 两者职责不同，命名明确区分

**与现有 JWT 密钥管理的统一**：

现有 `middleware/auth.go` 中 `jwtSecret` 硬编码为 `"bdopsflow-secret-key"`，而 `config.go` 中有 `JWTSecret` 配置项但未使用。数据源模块实现时一并修复：

1. `middleware/auth.go` 改为从 `config.JWTSecret` 读取 JWT 密钥
2. 配置文件中 `jwt_secret` 和 `datasource.encryption_key` 均为必填项
3. 生产环境部署时通过环境变量或密钥文件注入，禁止硬编码

### 5.3 配置服务实现

```go
type ConfigService struct {
    db    *sql.DB
    cache map[string]string
    mu    sync.RWMutex
}

func (s *ConfigService) Get(key string) string {
    s.mu.RLock()
    defer s.mu.RUnlock()
    if v, ok := s.cache[key]; ok {
        return v
    }
    return defaultValues[key]
}

func (s *ConfigService) Reload() error {
    rows, err := s.db.Query("SELECT config_key, config_value FROM bdopsflow_system_config")
    if err != nil {
        return err
    }
    defer rows.Close()
    
    newCache := make(map[string]string)
    for rows.Next() {
        var key, value string
        if err := rows.Scan(&key, &value); err != nil {
            return err
        }
        newCache[key] = value
    }
    
    s.mu.Lock()
    s.cache = newCache
    s.mu.Unlock()
    return nil
}
```

### 5.4 配置合法性校验

```go
var configValidators = map[string]func(string) error{
    "datasource.cache_ttl": func(v string) error {
        val, err := strconv.Atoi(v)
        if err != nil || val < 0 {
            return fmt.Errorf("must be non-negative integer")
        }
        return nil
    },
    "datasource.query_timeout": func(v string) error {
        val, err := strconv.Atoi(v)
        if err != nil || val < 1 {
            return fmt.Errorf("must be positive integer")
        }
        return nil
    },
    // ... 其他配置校验器
}
```

### 5.5 配置变更生效机制

配置变更后各调度器同步刷新：
1. 更新数据库 `bdopsflow_system_config` 表
2. 记录变更历史到 `bdopsflow_system_config_history` 表
3. 发布 `config_updated` 事件（通过Redis Pub/Sub）
4. 各调度器收到事件后调用 `ConfigService.Reload()`

### 5.6 配置优先级

1. 数据库中的系统配置（优先）
2. 配置文件中的默认值
3. 代码中的硬编码默认值

***

## 6. REST API 设计

### 6.1 数据源管理

| 方法 | 路径 | 说明 | 权限 |
|-----|------|------|------|
| GET | `/api/datasources` | 获取数据源列表 | `datasource:read` |
| GET | `/api/datasources/:id` | 获取单个数据源 | `datasource:read` |
| POST | `/api/datasources` | 创建数据源 | `datasource:create` |
| PUT | `/api/datasources/:id` | 更新数据源 | `datasource:update` |
| DELETE | `/api/datasources/:id` | 删除数据源 | `datasource:delete` |
| POST | `/api/datasources/:id/test` | 测试数据源连接 | `datasource:read` |

### 6.2 SQL 查询

| 方法 | 路径 | 说明 | 权限 |
|-----|------|------|------|
| POST | `/api/query/execute` | 执行SQL查询 | `datasource:query` |
| POST | `/api/query/cancel/:query_id` | 取消查询 | `datasource:query` |
| GET | `/api/query/databases` | 获取数据库列表 | `datasource:query` |
| GET | `/api/query/tables` | 获取表列表 | `datasource:query` |
| GET | `/api/query/columns` | 获取字段列表 | `datasource:query` |
| POST | `/api/query/export` | 导出CSV | `datasource:download` |

### 6.3 保存的 SQL

| 方法 | 路径 | 说明 | 权限 |
|-----|------|------|------|
| GET | `/api/saved-sql` | 获取保存的SQL列表 | `datasource:read` |
| GET | `/api/saved-sql/:id` | 获取单个SQL | `datasource:read` |
| POST | `/api/saved-sql` | 保存SQL | `datasource:read` |
| PUT | `/api/saved-sql/:id` | 更新SQL | `datasource:read` |
| DELETE | `/api/saved-sql/:id` | 删除SQL | `datasource:read` |

### 6.4 查询历史

| 方法 | 路径 | 说明 | 权限 |
|-----|------|------|------|
| GET | `/api/query-history` | 获取查询历史 | `datasource:read` |
| DELETE | `/api/query-history/:id` | 删除历史 | `datasource:delete` |
| DELETE | `/api/query-history/batch` | 批量删除查询历史 | `datasource:delete` |

### 6.5 数据源权限

| 方法 | 路径 | 说明 | 权限 |
|-----|------|------|------|
| GET | `/api/datasources/:id/permissions` | 获取权限列表 | `datasource:manage` |
| POST | `/api/datasources/:id/permissions` | 分配权限 | `datasource:manage` |
| DELETE | `/api/datasources/:id/permissions/:permId` | 移除权限 | `datasource:manage` |

### 6.6 系统配置

| 方法 | 路径 | 说明 | 权限 |
|-----|------|------|------|
| GET | `/api/admin/system-config` | 获取配置列表 | 管理员 |
| PUT | `/api/admin/system-config/:key` | 更新配置 | 管理员 |

### 6.7 API 请求/响应格式

#### 统一响应封装（复用现有）

项目已有统一响应封装 `scheduler/internal/handler/response.go`，数据源模块直接复用：

```go
// 现有 response.go 中的结构和方法
type Response struct {
    Code    int         `json:"code"`
    Status  string      `json:"status"`
    Message string      `json:"message"`
    Data    interface{} `json:"data"`
}

// 数据源 Handler 使用方式
func (h *DatasourceHandler) Get(c *gin.Context) {
    ds, err := h.service.GetDatasource(c.Request.Context(), id)
    if err != nil {
        handler.NotFound(c, "datasource not found")
        return
    }
    handler.Success(c, ds)
}

func (h *DatasourceHandler) Create(c *gin.Context) {
    ds, err := h.service.CreateDatasource(c.Request.Context(), req)
    if err != nil {
        handler.BadRequest(c, err.Error())
        return
    }
    handler.Created(c, ds)
}

func (h *QueryHandler) Execute(c *gin.Context) {
    result, err := h.queryService.Execute(c.Request.Context(), req)
    if err != nil {
        if errors.Is(err, ErrQueryTimeout) {
            handler.Error(c, http.StatusRequestTimeout, err.Error())
            return
        }
        handler.InternalServerError(c, err.Error())
        return
    }
    handler.Success(c, result)
}
```

**错误码扩展**：现有 `response.go` 的 `HTTPStatus()` 将 HTTP 状态码映射为业务码。数据源模块新增自定义业务码（3000~3999），需扩展 `ErrorWithData` 方法：

```go
func ErrorWithCode(c *gin.Context, httpStatus int, code int, message string) {
    c.JSON(httpStatus, Response{
        Code:    code,
        Status:  "error",
        Message: message,
        Data:    nil,
    })
}
```

#### 执行查询

**请求**:
```json
POST /api/query/execute
{
  "datasource_id": 1,
  "sql": "SELECT * FROM users LIMIT 1000",
  "database": "mydb",
  "skip_cache": false
}
```

**响应**:
```json
{
  "code": 0,
  "status": "success",
  "message": "success",
  "data": {
    "columns": ["id", "name", "email"],
    "rows": [
      [1, "admin", "admin@example.com"],
      [2, "user", "user@example.com"]
    ],
    "row_count": 2,
    "from_cache": true,
    "execution_time": 0.123
  }
}
```

**注意**：现有 `handler/response.go` 的 `Response` 结构体不含 `trace_id` 字段。全链路追踪功能需后续迭代扩展 `Response` 结构体，当前版本暂不包含。

#### 创建数据源

**请求**:
```json
POST /api/datasources
{
  "name": "生产MySQL",
  "type": "mysql",
  "host": "10.0.0.1",
  "port": 3306,
  "database": "mydb",
  "username": "readonly",
  "password": "plain_password",
  "auth_type": "simple",
  "config": {},
  "description": "生产环境MySQL只读实例",
  "domain_id": 1,
  "is_enabled": true
}
```

**响应**:
```json
{
  "code": 0,
  "status": "success",
  "message": "success",
  "data": {
    "id": 1,
    "name": "生产MySQL",
    "type": "mysql",
    "host": "10.0.0.1",
    "port": 3306,
    "database": "mydb",
    "username": "readonly",
    "password": "******",
    "auth_type": "simple",
    "description": "生产环境MySQL只读实例",
    "domain_id": 1,
    "is_enabled": true,
    "test_status": "untested",
    "created_by": 1,
    "updated_by": 1,
    "has_query_permission": true,
    "has_download_permission": true
  }
}
```

#### 取消查询

**请求**:
```json
POST /api/query/cancel/q_20260519_abc123
```

**响应**:
```json
{
  "code": 0,
  "status": "success",
  "message": "query cancelled",
  "data": null
}
```

#### 导出CSV

**请求**:
```json
POST /api/query/export
{
  "datasource_id": 1,
  "sql": "SELECT * FROM users LIMIT 1000",
  "database": "mydb",
  "format": "csv"
}
```

**响应**:
- 成功：返回 `Content-Type: text/csv; charset=utf-8`，流式传输 CSV 文件
- 失败：返回标准 JSON 错误响应

#### 批量删除查询历史

**请求**:
```json
DELETE /api/query-history/batch
{
  "ids": [1, 2, 3, 4, 5]
}
```

或按条件删除：
```json
DELETE /api/query-history/batch
{
  "before": "2026-04-01T00:00:00Z",
  "datasource_id": 1
}
```

**响应**:
```json
{
  "code": 0,
  "status": "success",
  "message": "success",
  "data": {
    "deleted_count": 15
  }
}
```

### 6.8 API 分页参数规范

**列表接口统一分页参数**:

| 参数 | 类型 | 默认值 | 说明 |
|-----|------|-------|------|
| page | int | 1 | 页码 |
| page_size | int | 20 | 每页条数 |
| keyword | str | "" | 搜索关键词 |

**分页响应格式**:
```json
{
  "code": 0,
  "status": "success",
  "message": "success",
  "data": {
    "items": [...],
    "total": 100,
    "page": 1,
    "page_size": 20
  }
}
```

**各接口额外筛选参数**:

| 接口 | 额外筛选参数 |
|-----|-----------|
| 数据源列表 | `type`, `domain_id`, `is_enabled` |
| 保存的SQL列表 | `datasource_id`, `is_public` |
| 查询历史 | `datasource_id`, `status`, `start_date`, `end_date` |
| 数据源权限列表 | `role_id`, `permission_type` |

### 6.9 元数据 API 参数

#### 获取数据库列表

```
GET /api/query/databases?datasource_id=1
```

#### 获取表列表

```
GET /api/query/tables?datasource_id=1&database=mydb
```

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| datasource_id | int | ✅ | 数据源 ID |
| database | str | ❌ | 数据库名（部分数据源必填，如 MySQL） |

#### 获取字段列表

```
GET /api/query/columns?datasource_id=1&database=mydb&table=users
```

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| datasource_id | int | ✅ | 数据源 ID |
| database | str | ✅ | 数据库名 |
| table | str | ✅ | 表名 |

**特殊数据源说明**:

| 数据源类型 | databases API 返回 | tables API 的 database 参数 | columns API 的 database 参数 |
|----------|-------------------|--------------------------|---------------------------|
| mysql | SHOW DATABASES | 必填 | 必填 |
| hive | SHOW DATABASES | 必填 | 必填 |
| trino | SHOW CATALOGS | 选填（catalog） | 必填 |
| spark | SHOW DATABASES | 必填 | 必填 |
| kyuubi | SHOW DATABASES | 必填 | 必填 |
| starrocks | SHOW DATABASES | 必填 | 必填 |
| doris | SHOW DATABASES | 必填 | 必填 |
| sqlite | 返回文件路径作为唯一"数据库" | 忽略 | 忽略 |
| rqlite | 返回固定值 `["main"]` | 忽略 | 忽略 |

### 6.10 错误码定义

**数据源模块错误码范围：3000~3999**

| 错误码 | 说明 | HTTP状态码 |
|-------|------|----------|
| 3001 | 数据源不存在 | 404 |
| 3002 | 数据源名称已存在 | 409 |
| 3003 | 数据源类型不支持 | 400 |
| 3004 | 数据源连接失败 | 502 |
| 3005 | 数据源连接超时 | 504 |
| 3006 | 数据源已禁用 | 403 |
| 3007 | SQL语句类型不允许 | 403 |
| 3008 | SQL文本超过最大长度 | 400 |
| 3009 | 查询超时 | 408 |
| 3010 | 并发查询超过限制 | 429 |
| 3011 | 导出行数超过限制 | 400 |
| 3012 | 无数据源查询权限 | 403 |
| 3013 | 无数据源下载权限 | 403 |
| 3014 | 数据源权限已存在 | 409 |
| 3015 | 保存的SQL不存在 | 404 |
| 3016 | 查询历史不存在 | 404 |
| 3017 | 密码加密/解密失败 | 500 |
| 3018 | 数据源配置无效 | 400 |
| 3019 | 查询已取消 | 200 |
| 3020 | 查询执行失败 | 500 |
| 3021 | 连接池资源耗尽 | 503 |
| 3022 | 查询服务暂不可用（Redis不可用） | 503 |
| 3023 | SQL语法错误（由数据源返回） | 400 |
| 3024 | 元数据获取失败 | 502 |
| 3025 | 权限验证服务异常 | 500 |

***

## 7. 前端设计

### 7.1 路由结构

与现有前端路由结构保持一致，所有页面作为 Layout 子路由：

```typescript
const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/Login.vue'),
    meta: { requiresAuth: false },
  },
  {
    path: '/',
    component: () => import('@/views/Layout.vue'),
    meta: { requiresAuth: true },
    children: [
      // 现有路由（与 Layout.vue 菜单对齐）
      { path: '', name: 'Dashboard', component: () => import('@/views/Dashboard.vue') },
      { path: 'tasks', name: 'Tasks', component: () => import('@/views/Tasks.vue') },
      { path: 'logs', name: 'Logs', component: () => import('@/views/Logs.vue') },
      { path: 'executors', name: 'Executors', component: () => import('@/views/Executors.vue') },
      { path: 'profile', name: 'Profile', component: () => import('@/views/Profile.vue') },
      { path: 'admin/users', name: 'AdminUsers', component: () => import('@/views/admin/Users.vue'), meta: { requiresAdmin: true } },
      { path: 'admin/roles', name: 'AdminRoles', component: () => import('@/views/admin/Roles.vue'), meta: { requiresAdmin: true } },
      { path: 'admin/domains', name: 'AdminDomains', component: () => import('@/views/admin/Domains.vue'), meta: { requiresAdmin: true } },

      // 新增数据源路由
      { path: 'datasources', name: 'Datasources', component: () => import('@/views/Datasource/DatasourceList.vue') },
      { path: 'datasources/create', name: 'CreateDatasource', component: () => import('@/views/Datasource/DatasourceForm.vue') },
      { path: 'datasources/:id/edit', name: 'EditDatasource', component: () => import('@/views/Datasource/DatasourceForm.vue') },
      { path: 'datasources/:id/permissions', name: 'DatasourcePermission', component: () => import('@/views/Datasource/DatasourcePermission.vue') },
      { path: 'sql-query', name: 'SQLQuery', component: () => import('@/views/SQLQuery/SQLQuery.vue') },
      { path: 'saved-sql', name: 'SavedSQL', component: () => import('@/views/SQLQuery/SavedSQLList.vue') },
      { path: 'query-history', name: 'QueryHistory', component: () => import('@/views/SQLQuery/QueryHistory.vue') },
      { path: 'admin/system-config', name: 'SystemConfig', component: () => import('@/views/admin/SystemConfig.vue'), meta: { requiresAdmin: true } },
    ],
  },
]
```

**说明**：
- 现有路由与 Layout.vue 实际菜单完全对齐（仪表盘、任务管理、任务日志、执行器、个人设置、系统管理）
- 后端虽有 `/api/workflows` 路由，但 Layout.vue 侧边栏未展示"工作流"菜单项，前端路由中也未包含
- 新增路由全部作为 Layout 子路由，与现有结构一致

### 7.2 文件结构

```
web/src/
├── views/
│   ├── Datasource/
│   │   ├── DatasourceList.vue
│   │   ├── DatasourceForm.vue
│   │   └── DatasourcePermission.vue
│   ├── SQLQuery/
│   │   ├── SQLQuery.vue
│   │   ├── SavedSQLList.vue
│   │   └── QueryHistory.vue
│   └── admin/
│       └── SystemConfig.vue
├── components/
│   └── SQLEditor.vue
├── api/
│   ├── index.ts              # 现有（已有，新增 datasource 相关 API）
│   ├── admin.ts              # 现有（已有）
│   └── datasource.ts         # 新增：数据源相关 API
├── utils/
│   └── api.ts                # 现有（已有，统一 axios 封装，无需修改）
├── stores/
│   ├── auth.ts               # 现有（已有）
│   └── datasource.ts         # 新增：数据源状态管理
└── types/
    ├── index.ts              # 现有（已有，新增 datasource 相关类型）
    └── datasource.ts         # 新增：数据源类型定义
```

#### API 层设计（复用现有）

项目已有 `utils/api.ts` 统一 axios 封装和 `api/index.ts` API 层，数据源模块直接复用：

```typescript
// api/datasource.ts — 新增文件
import api from '@/utils/api'
import type { Datasource, DatasourcePermission, SavedSQL, QueryHistory, QueryResult, SystemConfig } from '@/types/datasource'

export const datasourceAPI = {
  list: (params?: Record<string, unknown>) => api.get('/datasources', { params }),
  get: (id: number) => api.get<Datasource>(`/datasources/${id}`),
  create: (data: Partial<Datasource>) => api.post<Datasource>('/datasources', data),
  update: (id: number, data: Partial<Datasource>) => api.put(`/datasources/${id}`, data),
  delete: (id: number) => api.delete(`/datasources/${id}`),
  testConnection: (id: number) => api.post(`/datasources/${id}/test`),
  getPermissions: (id: number) => api.get<DatasourcePermission[]>(`/datasources/${id}/permissions`),
  assignPermission: (id: number, data: Partial<DatasourcePermission>) => api.post(`/datasources/${id}/permissions`, data),
  removePermission: (id: number, permissionId: number) => api.delete(`/datasources/${id}/permissions/${permissionId}`),
}

export const queryAPI = {
  execute: (data: Record<string, unknown>) => api.post<QueryResult>('/query/execute', data, { timeout: 120000 }),
  cancel: (queryId: string) => api.post(`/query/cancel/${queryId}`),
  getDatabases: (datasourceId: number) => api.get<string[]>('/query/databases', { params: { datasource_id: datasourceId } }),
  getTables: (datasourceId: number, database?: string) => api.get('/query/tables', { params: { datasource_id: datasourceId, database } }),
  getColumns: (datasourceId: number, database: string, table: string) => api.get('/query/columns', { params: { datasource_id: datasourceId, database, table } }),
  exportCSV: (data: Record<string, unknown>) => api.post('/query/export', data, { responseType: 'blob' }),
}

export const savedSQLAPI = {
  list: (params?: Record<string, unknown>) => api.get('/saved-sql', { params }),
  get: (id: number) => api.get<SavedSQL>(`/saved-sql/${id}`),
  create: (data: Partial<SavedSQL>) => api.post<SavedSQL>('/saved-sql', data),
  update: (id: number, data: Partial<SavedSQL>) => api.put(`/saved-sql/${id}`, data),
  delete: (id: number) => api.delete(`/saved-sql/${id}`),
}

export const queryHistoryAPI = {
  list: (params?: Record<string, unknown>) => api.get('/query-history', { params }),
  delete: (id: number) => api.delete(`/query-history/${id}`),
  batchDelete: (data: Record<string, unknown>) => api.delete('/query-history/batch', { data }),
}

export const systemConfigAPI = {
  list: () => api.get<SystemConfig[]>('/admin/system-config'),
  update: (key: string, value: string) => api.put(`/admin/system-config/${key}`, { value }),
}
```

**与现有 API 层的关系**：
- `utils/api.ts`：统一 axios 封装（Token 拦截器、错误处理），无需修改
- `api/index.ts`：现有 API（task/workflow/executor/log/dashboard），无需修改
- `api/datasource.ts`：新增数据源 API，遵循现有命名和风格

### 7.3 SQL 查询页面布局

```
┌─────────────────────────────────────────────────────────────┐
│  数据源选择 [▼]  [刷新元数据]  [帮助]                        │
├───────────────────┬─────────────────────────────────────────┤
│                   │                                         │
│  ┌─────────────┐  │  ┌─────────────────────────────────┐  │
│  │ 数据库树   │  │  │  SQL编辑器                       │  │
│  │ - db1      │  │  │  [格式化] [执行] [保存] [导出]   │  │
│  │   - table1 │  │  │  SELECT * FROM ... LIMIT 1000   │  │
│  │   - table2 │  │  │                                 │  │
│  │ - db2      │  │  └─────────────────────────────────┘  │
│  │            │  │                                         │
│  └─────────────┘  │  ┌─────────────────────────────────┐  │
│                   │  │  查询结果                       │  │
│                   │  │  [表格/JSON切换] [复制]         │  │
│                   │  │  ...数据表格...                 │  │
│  已保存SQL列表   │  │                                 │  │
│  ┌─────────────┐  │  └─────────────────────────────────┘  │
│  │ - 查询1     │  │                                         │
│  │ - 查询2     │  │  查询历史                              │
│  └─────────────┘  │  ...                                   │
└───────────────────┴─────────────────────────────────────────┘
```

### 7.4 数据源表单字段

| 字段 | 类型 | 显示条件 | 说明 |
|-----|------|---------|------|
| 名称 | 文本 | 始终 | |
| 类型 | 下拉 | 始终（编辑时禁用） | MySQL/Hive/Trino等，创建后不可修改 |
| 连接模式 | 下拉 | type in (hive, kyuubi, spark) | `直接连接` 或 `ZooKeeper服务发现`，默认 `直接连接` |
| 主机 | 文本 | type != sqlite 且 (type not in (hive, kyuubi, spark) 或 connection_mode == direct) | |
| 端口 | 数字 | type != sqlite 且 (type not in (hive, kyuubi, spark) 或 connection_mode == direct) | 选择类型后自动填充默认端口 |
| ZooKeeper地址 | 文本 | type in (hive, kyuubi, spark) 且 connection_mode == zookeeper | 逗号分隔的ZK地址，如 `zk1:2181,zk2:2181` |
| ZooKeeper命名空间 | 文本 | type in (hive, kyuubi, spark) 且 connection_mode == zookeeper | 默认 `hiveserver2` |
| 文件路径 | 文本 | type == sqlite | |
| 数据库 | 文本 | 可选 | 默认数据库 |
| 用户名 | 文本 | 可选 | |
| 密码 | 密码 | 可选 | 编辑时显示 `******`，空值保持原密码 |
| 认证类型 | 下拉 | 可选 | 根据数据源类型动态显示选项 |
| 传输模式 | 下拉 | type in (hive, kyuubi, spark) | `binary`（默认）或 `http` |
| HTTP路径 | 文本 | type in (hive, kyuubi, spark) 且 transport_mode == http | 如 `cliservice` |
| SSL | 开关 | type in (hive, kyuubi, trino, starrocks, doris, mysql) | 是否启用SSL |
| 描述 | 文本域 | 可选 | |
| 启用状态 | 开关 | 始终 | 创建时默认启用，可手动禁用 |

**连接模式说明**：
- **直接连接**：直接连接单个 HiveServer2 实例，适合测试环境
- **ZooKeeper服务发现**：通过 ZooKeeper 自动发现可用的 HiveServer2 实例，适合生产环境，支持高可用和负载均衡

**认证类型选项按数据源类型动态显示**:

| 数据源类型 | 认证类型选项 |
|----------|------------|
| mysql | 简单密码 |
| hive | 简单密码 / LDAP |
| trino | 简单密码 / LDAP |
| spark | 简单密码 / LDAP |
| kyuubi | 简单密码 / LDAP |
| starrocks | 简单密码 / LDAP |
| doris | 简单密码 / LDAP |
| sqlite | 无认证 |
| rqlite | 无认证 / Basic Auth |

**说明**：Hive/Trino/Spark 等组件的 LDAP 认证是指通过 HiveServer2/Trino Coordinator 连接到已配置好的 LDAP 服务端进行认证，客户端只需提供 LDAP 用户名和密码。Kerberos 认证类似，客户端只需提供 Kerberos principal 和 keytab。具体的 LDAP/Kerberos 服务端配置由集群管理员在 hadoop 集群侧完成。

### 7.5 导航菜单集成

与现有 Layout.vue 侧边栏菜单对齐，新增"数据查询"分组：

```
┌─────────────────────────┐
│  📊 BDopsFlow           │
├─────────────────────────┤
│  📊 仪表盘              │  ← 现有菜单 (/)
│  📋 任务管理             │  ← 现有菜单 (/tasks)
│  📋 任务日志             │  ← 现有菜单 (/logs)
│  ⚙️ 执行器              │  ← 现有菜单 (/executors)
│  👤 个人设置             │  ← 现有菜单 (/profile)
│  ─────────────────      │
│  🔍 数据查询              │  ← 新增一级菜单
│    ├ 数据源管理            │  (/datasources)
│    ├ SQL 查询             │  (/sql-query)
│    ├ 查询历史             │  (/query-history)
│    └ 已保存 SQL           │  (/saved-sql)
│  ─────────────────      │
│  ⚙️ 系统管理              │  ← 现有菜单（仅管理员可见）
│    ├ 用户管理             │  (/admin/users)
│    ├ 角色管理             │  (/admin/roles)
│    ├ 领域管理             │  (/admin/domains)
│    └ 系统配置             │  (/admin/system-config) ← 新增子菜单
└─────────────────────────┘
```

**与现有 Layout.vue 的对齐说明**：
- 现有菜单项（仪表盘、任务管理、任务日志、执行器、个人设置、系统管理）保持不变
- "数据查询"作为新的一级菜单分组插入在"个人设置"和"系统管理"之间
- "系统配置"作为"系统管理"的新增子菜单项
- Layout.vue 的 `pageTitle` 映射需同步更新

**菜单权限控制**:

| 菜单项 | 可见条件 |
|-------|---------|
| 数据源管理 | `datasource:read` 权限 |
| SQL 查询 | `datasource:query` 权限 |
| 查询历史 | `datasource:read` 权限 |
| 已保存 SQL | `datasource:read` 权限 |
| 系统配置 | 系统管理员或领域管理员 |

### 7.6 SQL 编辑器功能

| 功能 | 实现方式 | 说明 |
|-----|---------|------|
| 语法高亮 | CodeMirror 6 | 支持 SQL 语法着色 |
| SQL 格式化 | sql-formatter 库 | 一键美化 SQL，支持不同方言 |
| 自动补全 | CodeMirror autocompletion 插件 | 补全表名、字段名、SQL 关键字 |
| 表名点击插入 | 元数据树点击表名 | 点击表名自动插入到编辑器光标位置 |
| 字段名点击插入 | 元数据树点击字段名 | 点击字段名自动插入到编辑器光标位置 |
| 多语句支持 | 分号分隔 | 一次只执行光标所在语句或第一条语句 |
| 快捷键 | Ctrl+Enter 执行，Ctrl+Shift+F 格式化 | 常用操作快捷键 |
| SQL 历史 | 编辑器下拉 | 最近执行的 SQL 可快速选择 |
| 执行状态指示 | Loading 动画 | 查询执行中显示加载动画 |

### 7.7 数据源类型展示名映射

| type 值 | 前端展示名 | 图标 | 颜色 |
|--------|----------|------|------|
| mysql | MySQL | 🐬 MySQL 图标 | #4479A1 |
| hive | Apache Hive | 🐝 Hive 图标 | #FDEE21 |
| trino | Trino | 🔵 Trino 图标 | #DD00A1 |
| spark | Spark SQL | 🟠 Spark 图标 | #E25A1C |
| kyuubi | Apache Kyuubi | 🟣 Kyuubi 图标 | #6B57D7 |
| starrocks | StarRocks | ⭐ SR 图标 | #5C2D91 |
| doris | Apache Doris | 🟢 Doris 图标 | #00A8E8 |
| sqlite | SQLite | 📦 SQLite 图标 | #003B57 |
| rqlite | rqlite | 🔗 rqlite 图标 | #4B8BBE |

### 7.8 数据类型展示映射

| 数据库原始类型 | 前端展示类型 | 说明 |
|--------------|------------|------|
| INTEGER/BIGINT | 数字 | 右对齐 |
| FLOAT/DOUBLE/DECIMAL | 数字 | 右对齐，保留2位小数 |
| VARCHAR/TEXT | 文本 | 左对齐 |
| DATE | 日期 | 格式化为 YYYY-MM-DD |
| TIMESTAMP/DATETIME | 日期时间 | 格式化为 YYYY-MM-DD HH:mm:ss |
| BOOLEAN | 布尔 | 显示为 是/否 |
| BLOB/BINARY | 二进制 | 显示为 [BLOB] |
| NULL | 空 | 显示为 - |

### 7.9 前端依赖

```json
{
  "dependencies": {
    "@element-plus/icons-vue": "^2.3.1",
    "axios": "^1.6.2",
    "element-plus": "^2.5.0",
    "pinia": "^2.1.7",
    "vue": "^3.4.0",
    "vue-router": "^4.2.5",
    "sql-formatter": "^13.0.0",
    "codemirror": "^6.0.1",
    "@codemirror/lang-sql": "^6.5.0",
    "@codemirror/autocomplete": "^6.11.0"
  }
}
```

***

## 8. 安全设计

### 8.1 SQL 执行安全控制

#### SQL 语句类型限制

系统默认只允许执行 SELECT 查询语句，禁止执行 DDL/DML 操作：

| SQL 类型 | 默认允许 | 说明 |
|---------|---------|------|
| SELECT | ✅ | 查询语句，默认允许 |
| INSERT | ❌ | 禁止，防止数据篡改 |
| UPDATE | ❌ | 禁止，防止数据篡改 |
| DELETE | ❌ | 禁止，防止数据丢失 |
| DROP | ❌ | 禁止，防止破坏性操作 |
| ALTER | ❌ | 禁止，防止结构变更 |
| CREATE | ❌ | 禁止，防止结构变更 |
| TRUNCATE | ❌ | 禁止，防止数据丢失 |

**实现方式**:
- 后端通过正则匹配 SQL 关键字进行拦截
- 在系统配置中增加 `datasource.allow_write_sql` 开关（默认 false）
- 开启写操作需要管理员权限，且记录审计日志

#### SQL 执行超时

- 默认查询超时时间：60 秒
- 可通过系统配置 `datasource.query_timeout` 动态调整
- 超时后自动取消查询，返回超时错误
- 前端展示超时提示，建议用户优化 SQL 或添加更严格的 WHERE 条件

#### SQL 大小限制

| 配置项 | 默认值 | 说明 |
|-------|-------|------|
| datasource.max_sql_length | 65536 | SQL 文本最大长度（字节） |

- 默认限制 64KB，防止超长 SQL 导致内存溢出
- 前端在编辑器底部显示当前 SQL 长度 / 最大长度
- 超出限制时前端禁止提交，后端返回 `400 Bad Request`

#### 查询结果单元格大小限制

| 配置项 | 默认值 | 说明 |
|-------|-------|------|
| datasource.max_cell_size | 65536 | 单个单元格值最大字节数 |

- 超过限制的单元格值截断并添加 `[TRUNCATED: original_size]` 标记
- BLOB/BINARY 类型始终显示为 `[BLOB: size_bytes]`

### 8.2 并发查询限制

- 每个用户同时执行的查询数量限制：5 个
- 全局并发查询数量限制：50 个
- 可通过系统配置动态调整：
  - `datasource.max_concurrent_per_user`: 单用户并发限制，默认 5
  - `datasource.max_concurrent_global`: 全局并发限制，默认 50
- 超出限制时返回 429 Too Many Requests，前端提示用户等待

### 8.3 数据安全

- 数据源密码使用 AES-256-GCM 加密存储
- 查询结果不在数据库中持久化（仅存储执行状态）
- 导出功能有行数限制
- 密码字段在API响应中始终返回 `"******"`

### 8.4 权限控制

- 严格的权限检查，防止越权访问
- 领域隔离确保数据安全
- 敏感操作记录审计日志

### 8.5 SQL 注入防护

- 使用参数化查询
- 对用户输入的 SQL 进行验证和限制
- 默认添加 LIMIT 子句

***

## 9. 核心功能设计

### 9.1 数据源连接池

#### 连接池设计

```go
type ConnectionPool struct {
    mu          sync.Mutex
    pools       map[int64]*DriverPool  // key: datasource_id
    maxIdle     int                    // 最大空闲连接数，默认 5
    maxOpen     int                    // 最大打开连接数，默认 10
    maxLifetime time.Duration          // 连接最大生命周期，默认 30 分钟
}

type DriverPool struct {
    driver      Driver
    config      DatasourceConfig
    lastUsed    time.Time
}
```

#### 连接池策略

- 每个数据源维护独立的连接池
- 空闲连接超过 10 分钟自动关闭
- 连接使用超过 30 分钟自动重建
- 数据源被禁用或删除时，关闭对应连接池
- 定期清理过期连接（每分钟检查一次）

#### 全局连接数限制

```go
type ConnectionPoolManager struct {
    totalOpen      int32
    maxTotalOpen   int32
    pools          map[int64]*DriverPool
}
```

### 9.2 查询缓存

#### 缓存 Key 设计

```
cache_key = "datasource:query:cache:{datasource_id}:{md5(sql_text)}"
```

- 相同 SQL + 相同数据源 = 相同缓存
- 不同用户执行相同 SQL 共享缓存（因为数据源权限已在查询前验证）
- Key 命名遵循项目规范 `业务域:模块:key`

#### 缓存策略

| 配置项 | 默认值 | 说明 |
|-------|-------|------|
| datasource.cache_ttl | 300 秒 | 缓存过期时间 |
| datasource.cache_max_size | 100MB | 缓存最大内存占用 |

#### 缓存淘汰

- TTL 过期自动淘汰
- 内存超限时淘汰最早未使用的缓存
- 数据源被修改/删除时清除相关缓存
- 手动刷新：前端点击"刷新"按钮时跳过缓存

#### 缓存分层策略

```go
type QueryCache struct {
    localCache  *lru.Cache     // 本地LRU缓存（热点数据）
    redisClient *redis.Client  // Redis全局缓存
}
```

- 查询时先查本地，再查Redis
- 写入时同时写入本地和Redis
- 通过Redis Pub/Sub广播本地缓存失效

### 9.3 查询取消功能

#### 实现机制

- 每个查询执行时生成唯一的 `query_id`，通过 `context.WithCancel` 创建可取消的 context
- 查询执行中通过 `context.Done()` 监听取消信号
- 前端在查询执行期间显示"取消"按钮，查询完成后自动隐藏
- 取消操作仅限查询发起者本人
- 已完成的查询无法取消，返回提示信息
- 并发查询计数在取消后立即释放

### 9.4 并发查询追踪机制

#### Redis Key 设计

```
Key: datasource:query:concurrent:user:{user_id}    → 当前用户并发查询数（计数器）
Key: datasource:query:concurrent:global             → 全局并发查询数（计数器）
Key: datasource:query:running:{query_id}            → 单个查询的元信息（JSON）
```

#### 查询元信息结构

```json
{
  "query_id": "q_20260519_abc123",
  "user_id": 1,
  "datasource_id": 3,
  "sql_text": "SELECT ...",
  "started_at": "2026-05-19T10:00:00Z",
  "cancel_key": "datasource:query:cancel:q_20260519_abc123"
}
```

#### 并发控制流程

```
1. 用户发起查询
2. 检查 Redis 中 datasource:query:concurrent:user:{user_id} 计数 → 超过 max_concurrent_per_user 则拒绝
3. 检查 Redis 中 datasource:query:concurrent:global 计数 → 超过 max_concurrent_global 则拒绝
4. 计数 +1，写入 datasource:query:running:{query_id}
5. 执行查询
6. 查询完成/失败/取消后，计数 -1，删除 datasource:query:running:{query_id}
7. 如果服务异常重启，Redis 中的计数可能不准确 → 定期校准（每分钟扫描 datasource:query:running:* 验证）
```

### 9.5 CSV 导出

#### 导出方式

- **同步导出**：数据量 <= max_export_rows 时同步生成 CSV 并返回
- **异步导出**：数据量 > max_export_rows 时提示用户缩小查询范围

#### 导出流程

```
1. 用户点击"导出CSV"按钮
2. 后端检查用户是否有 download 权限
3. 后端检查结果行数是否超过 max_export_rows
4. 如果超过，返回错误提示
5. 如果未超过，从缓存或重新查询获取数据
6. 生成 CSV 文件（UTF-8 BOM 编码，兼容 Excel）
7. 通过 HTTP 流式返回文件
8. 记录下载审计日志
```

#### CSV 格式规范

- 编码：UTF-8 with BOM（兼容 Excel 中文）
- 分隔符：逗号
- 换行符：\r\n
- 文本字段用双引号包裹
- 文件名格式：`query_{datasource_name}_{timestamp}.csv`

### 9.6 数据源健康检查

#### 健康检查机制

| 配置项 | 默认值 | 说明 |
|-------|-------|------|
| datasource.health_check_interval | 300 | 健康检查间隔（秒），0 为禁用 |

#### 检查逻辑

- 定时任务每隔 `health_check_interval` 秒对所有 `is_enabled = true` 的数据源执行轻量级连接测试
- 测试方式：执行最小代价的查询（如 MySQL 的 `SELECT 1`，Hive 的 `SELECT 1`）
- 更新 `test_status` 和 `last_test_at` 字段
- 健康检查失败的数据源 `test_status` 更新为 `failed`，但不自动禁用
- 健康检查结果在数据源列表中展示状态图标

#### 状态展示

| test_status | 图标 | 颜色 |
|------------|------|------|
| untested | ⚪ | 灰色 |
| success | 🟢 | 绿色 |
| failed | 🔴 | 红色 |

### 9.7 密码更新行为

更新数据源时，密码字段采用"空值保持原密码"策略：

| 场景 | 前端行为 | 后端处理 |
|-----|---------|---------|
| 不修改密码 | 密码输入框显示占位符 `******` | 密码为空时不更新密码字段 |
| 修改密码 | 用户清空后输入新密码 | 密码非空时加密后更新 |
| 新建数据源 | 密码输入框为空 | 密码为空则存储空字符串，非空则加密存储 |

### 9.8 数据源删除级联影响

| 关联数据 | 级联行为 |
|---------|---------|
| 连接池 | 立即关闭所有连接 |
| 查询缓存 | 清除该数据源的所有缓存 |
| 数据源权限 | 级联删除（ON DELETE CASCADE） |
| 保存的 SQL | 级联删除（ON DELETE CASCADE） |
| 查询历史 | 保留（历史记录有审计价值），但 `datasource_id` 置为 NULL |

***

## 10. 多调度器一致性设计

### 10.1 数据源配置变更广播

```go
type ConfigChangeEvent struct {
    Type         string `json:"type"`          // datasource_created, datasource_updated, datasource_deleted, config_updated
    DatasourceID int64  `json:"datasource_id,omitempty"`
    ConfigKey    string `json:"config_key,omitempty"`
    Timestamp    string `json:"timestamp"`
}

func (s *Service) PublishConfigChange(event *ConfigChangeEvent) {
    payload, _ := json.Marshal(event)
    s.redisClient.Publish(ctx, "datasource:config:change", payload)
}

func (s *Service) SubscribeConfigChanges() {
    pubsub := s.redisClient.Subscribe(ctx, "datasource:config:change")
    ch := pubsub.Channel()
    
    for msg := range ch {
        var event ConfigChangeEvent
        json.Unmarshal([]byte(msg.Payload), &event)
        s.handleConfigChange(event)
    }
}
```

### 10.2 数据源健康检查

仅Leader节点执行健康检查：

```go
func (s *Service) isLeader() bool {
    return s.electionManager.IsLeader()
}

func (s *Service) startHealthCheck() {
    if !s.isLeader() {
        return
    }
    // 启动健康检查定时任务
}
```

### 10.3 连接池关闭协调

某调度器检测到数据源被禁用或删除，不立即关闭连接池：
- 通过Redis广播事件
- 所有调度器收到事件后关闭各自的连接池

### 10.4 连接池关闭钩子集成

```go
func (s *Scheduler) Shutdown() error {
    if err := s.datasourceManager.CloseAllPools(); err != nil {
        log.Printf("Error closing datasource pools: %v", err)
    }
    // ... 其他关闭逻辑 ...
    return nil
}
```

***

## 11. Redis 使用策略

### 11.1 现有Redis使用盘点

BDopsFlow主系统已重度依赖Redis：
- **任务分布式锁** (`task:lock:*`)
- **主节点选举** (`scheduler:leader`)
- **指标收集** (`bdopsflow:*`)
- **日志去重** (`task:log:dedup:*`)
- **任务续期追踪** (`task:renew:*`)

### 11.2 数据源模块新增Redis使用

遵循项目 Redis Key 命名规范 `业务域:模块:key`，新增以下 Key：

| Key 模式 | 类型 | 说明 | TTL |
|---------|------|------|-----|
| `datasource:query:cache:{ds_id}:{hash}` | String | 查询缓存（JSON） | cache_ttl 秒 |
| `datasource:query:concurrent:user:{user_id}` | String (Counter) | 当前用户并发查询数 | 查询执行期间 |
| `datasource:query:concurrent:global` | String (Counter) | 全局并发查询数 | 查询执行期间 |
| `datasource:query:running:{query_id}` | String (JSON) | 运行中查询元信息 | query_timeout 秒 |
| `datasource:query:cancel:{query_id}` | String | 查询取消信号 | query_timeout 秒 |
| `datasource:config:change` | Pub/Sub Channel | 配置变更广播 | - |

### 11.3 Redis故障降级

- 若Redis不可用，禁用缓存功能和并发控制，但仍允许执行查询
- 记录错误日志并告警
- 定期检测Redis恢复

***

## 12. 监控与运维

### 12.1 指标收集

集成现有指标系统，新增数据源模块指标：

| 指标名 | 类型 | 说明 |
|-------|------|------|
| datasource_query_total | Counter | 查询总次数，按datasource_id、status标签 |
| datasource_query_duration_seconds | Histogram | 查询耗时分布 |
| datasource_pool_open_connections | Gauge | 各数据源连接池当前打开连接数 |
| datasource_pool_idle_connections | Gauge | 各数据源连接池空闲连接数 |
| datasource_cache_hits_total | Counter | 查询缓存命中次数 |
| datasource_cache_misses_total | Counter | 查询缓存未命中次数 |
| datasource_concurrent_queries | Gauge | 当前并发查询数 |
| datasource_health_check_success | Gauge | 数据源健康状态（1=成功，0=失败） |

### 12.2 告警规则

```yaml
groups:
- name: datasource_alerts
  rules:
  - alert: DatasourceHealthCheckFailed
    expr: datasource_health_check_success == 0
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "数据源 {{ $labels.datasource_name }} 健康检查失败"
  
  - alert: HighQueryLatency
    expr: histogram_quantile(0.95, datasource_query_duration_seconds_bucket) > 30
    for: 5m
    labels:
      severity: warning
  
  - alert: HighConnectionPoolUsage
    expr: datasource_pool_open_connections / datasource_connection_max_open > 0.8
    for: 5m
    labels:
      severity: warning
```

### 12.3 审计日志

#### 需要记录审计日志的操作

| 操作 | 审计级别 | 日志内容 |
|-----|---------|---------|
| 创建数据源 | INFO | 操作人、数据源名称、类型、领域 |
| 更新数据源 | INFO | 操作人、数据源名称、变更字段 |
| 删除数据源 | WARN | 操作人、数据源名称、类型 |
| 测试连接 | DEBUG | 操作人、数据源名称、测试结果 |
| 执行查询 | INFO | 操作人、数据源名称、SQL 摘要（前200字符）、执行时间 |
| 导出 CSV | WARN | 操作人、数据源名称、SQL 摘要、导出行数 |
| 分配数据源权限 | WARN | 操作人、目标角色、权限类型、数据源名称 |
| 移除数据源权限 | WARN | 操作人、目标角色、权限类型、数据源名称 |
| 开启写操作 SQL | WARN | 操作人、数据源名称 |
| 批量删除查询历史 | INFO | 操作人、删除数量 |

#### 审计日志格式

```json
{
  "trace_id": "xxx",
  "action": "datasource.query.execute",
  "actor_id": 1,
  "actor_name": "admin",
  "resource_type": "datasource",
  "resource_id": 3,
  "resource_name": "生产MySQL",
  "domain_id": 1,
  "details": {
    "sql_preview": "SELECT * FROM users ...",
    "execution_time": 0.123,
    "row_count": 50
  },
  "timestamp": "2026-05-19T10:00:00Z"
}
```

***

## 13. 项目结构

### 13.1 后端文件结构

```
scheduler/
├── internal/
│   ├── datasource/
│   │   ├── driver/
│   │   │   ├── base.go
│   │   │   ├── registry.go
│   │   │   ├── mysql.go
│   │   │   ├── hive.go
│   │   │   ├── trino.go
│   │   │   ├── spark.go
│   │   │   ├── kyuubi.go
│   │   │   ├── starrocks.go
│   │   │   ├── doris.go
│   │   │   ├── sqlite.go
│   │   │   └── rqlite.go
│   │   ├── service.go          # 数据源管理服务（直接操作 rqlite，与现有 service 层模式一致）
│   │   ├── query_service.go    # SQL 查询服务
│   │   ├── config_service.go   # 系统配置服务
│   │   ├── crypto.go           # 密码加密
│   │   ├── cache.go            # 查询缓存
│   │   ├── export.go           # CSV 导出
│   │   └── manager.go          # 连接池管理
│   ├── model/
│   │   ├── models.go           # 现有模型（已有）
│   │   ├── datasource.go       # 新增：数据源模型
│   │   ├── saved_sql.go        # 新增：保存的SQL模型
│   │   ├── query_history.go    # 新增：查询历史模型
│   │   └── system_config.go    # 新增：系统配置模型
│   ├── handler/
│   │   ├── auth.go             # 现有（已有）
│   │   ├── task.go             # 现有（已有）
│   │   ├── workflow.go         # 现有（已有）
│   │   ├── datasource.go       # 新增：数据源 Handler
│   │   ├── sql_query.go        # 新增：SQL 查询 Handler
│   │   ├── saved_sql.go        # 新增：保存的SQL Handler
│   │   ├── query_history.go    # 新增：查询历史 Handler
│   │   └── system_config.go    # 新增：系统配置 Handler
│   ├── middleware/
│   │   ├── auth.go             # 现有（已有）
│   │   └── datasource.go       # 新增：数据源权限中间件
│   └── config/
│       └── config.go           # 现有（已有，新增 DatasourceCryptoConfig）
└── pkg/
    └── config/
        └── config.go           # 现有（已有）
```

**与现有代码模式对齐**：
- 不使用独立的 `dao/` 层，service 直接操作 `rqlite.Connection`（与 `permission_service.go`、`scheduler.go` 模式一致）
- model 定义在 `internal/model/` 目录（与 `models.go` 同目录）
- handler 定义在 `internal/handler/` 目录（与现有 handler 同目录）
- 新增 `middleware/datasource.go` 数据源权限中间件

### 13.2 数据库初始化 SQL

完整的 SQL 见 `deploy/schema.sql`。

***

## 14. 实施计划

### 阶段1：基础框架 (优先级：高)

**目标**: 建立核心基础设施

1. [ ] 添加数据库表结构到 `deploy/schema.sql`
2. [ ] 创建数据模型 (`scheduler/internal/model/datasource.go` 等)
3. [ ] 更新配置系统 (`scheduler/internal/config/config.go`)
4. [ ] 实现密码加密工具 (`scheduler/internal/datasource/crypto.go`)
5. [ ] 实现系统配置服务 (`scheduler/internal/datasource/service.go`)

### 阶段2：数据源管理 (优先级：高)

**目标**: 实现数据源 CRUD 功能

1. [ ] 实现 Driver 接口和注册表
2. [ ] 实现 MySQL Driver (简单密码)
3. [ ] 实现 SQLite Driver
4. [ ] 实现数据源 Manager
5. [ ] 实现数据源 CRUD API Handler
6. [ ] 实现数据源权限检查
7. [ ] 前端：数据源列表页面
8. [ ] 前端：数据源表单页面

### 阶段3：SQL 查询功能 (优先级：高)

**目标**: 实现核心查询功能

1. [ ] 实现查询执行服务（含超时控制、SQL 类型校验、大小限制）
2. [ ] 实现查询取消功能（context 取消机制）
3. [ ] 实现并发查询控制（Redis 计数器）
4. [ ] 实现元数据获取 (库/表/字段)
5. [ ] 实现查询缓存
6. [ ] 实现 CSV 导出（POST 方法）
7. [ ] 实现 SQL 查询 API Handler
8. [ ] 前端：SQL 查询主页面
9. [ ] 前端：SQL 编辑器组件 (集成 CodeMirror 6 + sql-formatter)
10. [ ] 前端：查询结果展示

### 阶段4：权限和高级功能 (优先级：中)

**目标**: 完善权限和增强功能

1. [ ] 实现数据源权限管理 API
2. [ ] 实现保存的 SQL 功能
3. [ ] 实现查询历史功能（含批量删除）
4. [ ] 实现系统配置管理 API
5. [ ] 实现数据源健康检查定时任务
6. [ ] 实现审计日志记录
7. [ ] 前端：数据源权限分配页面
8. [ ] 前端：已保存 SQL 列表页面
9. [ ] 前端：系统配置页面
10. [ ] 前端：导航菜单集成

### 阶段5：大数据驱动支持 (优先级：中)

**目标**: 支持大数据数据源和 LDAP 认证

1. [ ] 实现 Hive Driver (含 LDAP)
2. [ ] 实现 Trino Driver (含 LDAP)
3. [ ] 实现 Spark Driver (含 LDAP)
4. [ ] 实现 Kyuubi Driver (含 LDAP)
5. [ ] 实现 StarRocks Driver (含 LDAP)
6. [ ] 实现 Doris Driver (含 LDAP)
7. [ ] 实现 Rqlite Driver (含 Basic Auth)
8. [ ] 测试所有驱动的连接和查询

### 阶段6：测试和优化 (优先级：中)

**目标**: 完善测试和优化

1. [ ] 编写单元测试
2. [ ] 编写集成测试
3. [ ] 性能测试和优化
4. [ ] 安全审计
5. [ ] 文档完善

***

## 15. 技术依赖

### 15.1 后端依赖

```go
require (
    github.com/go-sql-driver/mysql v1.7.1
    github.com/mattn/go-sqlite3 v1.14.17
    github.com/rqlite/gorqlite v1.24.3
    github.com/beltran/gohive v1.2.0
    github.com/trinodb/trino-go-client v0.6.0
    github.com/redis/go-redis/v9 v9.3.0
)
```

### 15.2 前端依赖

```json
{
  "dependencies": {
    "sql-formatter": "^13.0.0",
    "codemirror": "^6.0.1",
    "@codemirror/lang-sql": "^6.5.0",
    "@codemirror/autocomplete": "^6.11.0"
  }
}
```

***

## 16. 附录

### 16.1 配置示例

**config.yaml.example 新增内容**:

```yaml
datasource:
  encryption_key: "your-32-byte-encryption-key-here-change-in-production"
  key_source: "env"
  key_env_var: "BDOPSFLOW_ENCRYPTION_KEY"
  key_file: ""
  auto_rotate_days: 0
```

### 16.2 术语表

| 术语 | 说明 |
|-----|------|
| 数据源 | 可连接的数据库服务 |
| Driver | 特定数据库类型的连接适配器 |
| LDAP | 轻量级目录访问协议，用于身份认证。<br>**重要**：Hive/Trino 等组件的 LDAP 认证是客户端提供 LDAP 用户名密码，由 HiveServer2/Trino 转发给配置的 LDAP 服务端验证，而非客户端直接连接 LDAP |
| Kerberos | 网络认证协议，用于强身份验证。<br>**重要**：Hive/Trino 等组件的 Kerberos 认证是客户端提供 principal/keytab，由 HiveServer2/Trino 转发给 KDC 验证 |
| HiveServer2 | Hive 的服务端组件，提供 JDBC/ODBC 接口，默认端口 10000，支持 LDAP/Kerberos 认证 |
| Trino Coordinator | Trino 的协调节点，处理查询请求，默认端口 8080，支持 LDAP/Kerberos 认证 |
| ZooKeeper | 分布式协调服务，用于 HiveServer2 高可用的服务发现 |
| ZooKeeper服务发现 | 多个 HiveServer2 实例注册到 ZooKeeper，客户端通过 ZooKeeper 自动发现可用实例，实现高可用和负载均衡 |
| ZooKeeperQuorum | ZooKeeper 集群地址，逗号分隔，如 `zk1:2181,zk2:2181` |
| ZooKeeperNamespace | ZooKeeper 中的命名空间，用于 HiveServer2 实例注册，默认 `hiveserver2` |
| CSV | 逗号分隔值，一种数据导出格式 |
| 领域 | 用户和资源的隔离边界 |

### 16.3 预留功能

以下功能为后续迭代预留：

#### 数据源分组/标签功能

```sql
ALTER TABLE bdopsflow_datasources ADD COLUMN tags TEXT;  -- JSON数组格式
ALTER TABLE bdopsflow_datasources ADD COLUMN category TEXT;
```

#### 查询结果分享功能

```
POST /api/query/share
{
  "datasource_id": 1,
  "sql": "SELECT * FROM users LIMIT 100",
  "database": "mydb",
  "expire_hours": 24
}

响应：{ share_url: "https://bdops.example/share/abc123" }
```

公开访问时仅显示查询结果，不允许重新执行。

#### 前端增强

1. **SQL编辑器增强**
   - 保存SQL历史记录
   - 常用SQL片段快捷插入
   - 自动补全：表名、字段名、SQL关键字（基于当前数据源元数据）

2. **查询结果导出增强**
   - 支持XLSX格式导出
   - 导出进度条
   - 大结果分页导出

3. **性能优化提示**
   - 显示查询执行计划（MySQL: EXPLAIN, Hive: EXPLAIN）
   - 扫描行数告警

***

**文档结束**
