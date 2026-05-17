# BDopsFlow 数据库设计文档

本文档详细描述了 BDopsFlow 调度平台所使用的 rqlite 分布式数据库的设计方案、表结构、索引优化和数据关系。

## 目录

- [数据库概述](#数据库概述)
- [表结构设计](#表结构设计)
- [索引优化](#索引优化)
- [数据关系](#数据关系)
- [初始化数据](#初始化数据)
- [备份与恢复](#备份与恢复)
- [性能优化](#性能优化)

---

## 数据库概述

### 技术选型

- **数据库**：rqlite v8.0+
- **类型**：分布式 SQLite
- **共识协议**：Raft 分布式一致性协议
- **特点**：高可用、强一致性、SQL 支持

### 连接配置

#### 开发环境（单节点）

```yaml
database:
  rqlite_addrs:
    - "http://localhost:4001"
  rqlite_user: ""
  rqlite_password: ""
  rqlite_tls: false
```

#### 生产环境（多节点集群）

```yaml
database:
  rqlite_addrs:
    - "http://rqlite1:4001"
    - "http://rqlite2:4001"
    - "http://rqlite3:4001"
  rqlite_user: "admin"
  rqlite_password: "your-rqlite-password"
  rqlite_tls: false
```

### 部署模式

| 模式 | 说明 | 适用场景 |
|------|------|----------|
| 单节点 | 只有一个 rqlite 实例 | 开发、测试环境 |
| 集群 | 3+ 节点 Raft 集群 | 生产环境 |

### 单节点部署

```bash
docker run -d --name bdopsflow-rqlite \
  -p 4001:4001 \
  -p 4002:4002 \
  rqlite/rqlite:latest
```

### 集群部署（3 节点）

```bash
# 节点 1（引导节点）
docker run -d --name rqlite1 \
  -p 4001:4001 \
  -p 4002:4002 \
  -v ./auth.json:/auth.json \
  rqlite/rqlite:latest \
  -node-id 1 \
  -http-addr 0.0.0.0:4001 \
  -raft-addr 0.0.0.0:4002 \
  -data-dir /data \
  -auth /auth.json \
  -bootstrap-expect 3

# 节点 2
docker run -d --name rqlite2 \
  -p 4011:4001 \
  -p 4012:4002 \
  -v ./auth.json:/auth.json \
  rqlite/rqlite:latest \
  -node-id 2 \
  -http-addr 0.0.0.0:4001 \
  -raft-addr 0.0.0.0:4002 \
  -data-dir /data \
  -auth /auth.json \
  -join http://admin:your-rqlite-password@rqlite1:4001 \
  -bootstrap-expect 3

# 节点 3
docker run -d --name rqlite3 \
  -p 4021:4001 \
  -p 4022:4002 \
  -v ./auth.json:/auth.json \
  rqlite/rqlite:latest \
  -node-id 3 \
  -http-addr 0.0.0.0:4001 \
  -raft-addr 0.0.0.0:4002 \
  -data-dir /data \
  -auth /auth.json \
  -join http://admin:your-rqlite-password@rqlite1:4001 \
  -bootstrap-expect 3
```

### 认证配置文件 auth.json

```json
[
  {
    "username": "admin",
    "password": "your-rqlite-password",
    "perms": ["all"]
  }
]
```

---

## 表结构设计

### 表命名规范

所有表名统一添加 `bdopsflow_` 前缀，便于识别和迁移。

### 表列表

| 表名 | 说明 | 核心功能 |
|------|------|----------|
| bdopsflow_domains | 领域表 | 资源隔离 |
| bdopsflow_users | 用户表 | 用户认证 |
| bdopsflow_workflows | 工作流表 | DAG 工作流 |
| bdopsflow_tasks | 任务表 | 定时任务 |
| bdopsflow_task_executions | 任务执行记录表 | 执行历史 |
| bdopsflow_executors | 执行器表 | 执行器管理 |
| bdopsflow_workflow_executions | 工作流执行记录表 | 工作流历史 |
| bdopsflow_task_dependencies | 任务依赖表 | 依赖关系 |
| bdopsflow_task_logs | 任务执行日志表 | 日志存储 |
| bdopsflow_roles | 角色表 | 权限管理 |
| bdopsflow_permissions | 权限表 | 权限定义 |
| bdopsflow_role_permissions | 角色权限映射表 | 角色权限关系 |
| bdopsflow_user_roles | 用户角色映射表 | 用户角色关系 |
| bdopsflow_domain_executors | 执行器领域分配表 | 领域执行器关系 |

---

## 核心表结构

### 1. 领域表 (bdopsflow_domains)

领域是资源隔离的基本单位，所有资源都绑定到特定领域。

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_domains (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**字段说明**：

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | INTEGER | PRIMARY KEY, AUTOINCREMENT | 主键 |
| name | TEXT | NOT NULL, UNIQUE | 领域名称（唯一） |
| description | TEXT | - | 领域描述 |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |
| updated_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 更新时间 |

**索引**：

| 索引名 | 字段 | 类型 | 说明 |
|--------|------|------|------|
| idx_bdopsflow_domains_name | name | UNIQUE | 名称唯一性 |

---

### 2. 用户表 (bdopsflow_users)

存储用户信息，包括认证凭证和基础信息。

```sql
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
```

**字段说明**：

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | INTEGER | PRIMARY KEY, AUTOINCREMENT | 主键 |
| username | TEXT | NOT NULL, UNIQUE | 用户名（唯一） |
| password | TEXT | NOT NULL | bcrypt 加密密码 |
| email | TEXT | - | 邮箱 |
| domain_id | INTEGER | FK | 默认领域 ID |
| role | TEXT | NOT NULL | 角色标识 |
| is_active | BOOLEAN | DEFAULT 1 | 是否激活 |
| last_login_at | DATETIME | - | 最后登录时间 |
| created_by | INTEGER | - | 创建者 ID |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |
| updated_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 更新时间 |

**索引**：

| 索引名 | 字段 | 类型 | 说明 |
|--------|------|------|------|
| idx_bdopsflow_users_username | username | UNIQUE | 用户名唯一性 |
| idx_bdopsflow_users_domain_id | domain_id | - | 领域查询 |
| idx_bdopsflow_users_role | role | - | 角色查询 |
| idx_bdopsflow_users_is_active | is_active | - | 激活状态查询 |

---

### 3. 任务表 (bdopsflow_tasks)

存储定时任务配置信息。

```sql
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
    assigned_executor_id TEXT,
    created_by INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (workflow_id) REFERENCES bdopsflow_workflows(id) ON DELETE CASCADE,
    FOREIGN KEY (domain_id) REFERENCES bdopsflow_domains(id) ON DELETE CASCADE
);
```

**字段说明**：

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | INTEGER | PRIMARY KEY, AUTOINCREMENT | 主键 |
| workflow_id | INTEGER | FK | 所属工作流 ID |
| name | TEXT | NOT NULL | 任务名称 |
| type | TEXT | NOT NULL | 任务类型：http、shell |
| config | TEXT | NOT NULL | 任务配置（JSON） |
| cron_expression | TEXT | - | Cron 表达式 |
| timeout_seconds | INTEGER | DEFAULT 300 | 超时时间（秒） |
| retry_count | INTEGER | DEFAULT 3 | 最大重试次数 |
| retry_interval | INTEGER | DEFAULT 5 | 重试间隔（秒） |
| is_enabled | BOOLEAN | DEFAULT 1 | 是否启用 |
| status | TEXT | DEFAULT 'pending' | 任务状态 |
| domain_id | INTEGER | NOT NULL, FK | 所属领域 ID |
| webhook_config | TEXT | - | Webhook 配置 |
| assigned_executor_id | TEXT | - | 指定执行器 ID |
| created_by | INTEGER | - | 创建者 ID |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |
| updated_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 更新时间 |

**任务状态说明**：

| 状态 | 说明 |
|------|------|
| pending | 待执行 |
| running | 运行中 |
| success | 成功 |
| failed | 失败 |

**索引**：

| 索引名 | 字段 | 类型 | 说明 |
|--------|------|------|------|
| idx_bdopsflow_tasks_workflow_id | workflow_id | - | 工作流查询 |
| idx_bdopsflow_tasks_domain_id | domain_id | - | 领域查询 |
| idx_bdopsflow_tasks_is_enabled | is_enabled | - | 启用状态查询 |
| idx_bdopsflow_tasks_type | type | - | 类型查询 |
| idx_bdopsflow_tasks_status | status | - | 状态查询 |
| idx_bdopsflow_tasks_cron_enabled | is_enabled, cron_expression | - | Cron 调度查询 |
| idx_bdopsflow_tasks_assigned_executor | assigned_executor_id | - | 指定执行器查询 |

---

### 4. 任务执行记录表 (bdopsflow_task_executions)

记录每次任务执行的详细信息。

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_task_executions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    execution_id TEXT NOT NULL UNIQUE,
    executor_id TEXT,
    executor_name TEXT,
    task_name TEXT,
    task_type TEXT,
    status TEXT NOT NULL,
    start_time DATETIME,
    end_time DATETIME,
    output TEXT,
    error TEXT,
    retry_times INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES bdopsflow_tasks(id) ON DELETE CASCADE
);
```

**字段说明**：

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | INTEGER | PRIMARY KEY, AUTOINCREMENT | 主键 |
| task_id | INTEGER | NOT NULL, FK | 任务 ID |
| execution_id | TEXT | NOT NULL, UNIQUE | 执行 ID（唯一） |
| executor_id | TEXT | - | 执行器 ID |
| executor_name | TEXT | - | 执行器名称 |
| task_name | TEXT | - | 任务名称（冗余） |
| task_type | TEXT | - | 任务类型（冗余） |
| status | TEXT | NOT NULL | 执行状态 |
| start_time | DATETIME | - | 开始时间 |
| end_time | DATETIME | - | 结束时间 |
| output | TEXT | - | 执行输出 |
| error | TEXT | - | 错误信息 |
| retry_times | INTEGER | DEFAULT 0 | 重试次数 |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |

**索引**：

| 索引名 | 字段 | 类型 | 说明 |
|--------|------|------|------|
| idx_bdopsflow_task_executions_task_id | task_id | - | 任务查询 |
| idx_bdopsflow_task_executions_execution_id | execution_id | UNIQUE | 执行 ID 查询 |
| idx_bdopsflow_task_executions_executor_id | executor_id | - | 执行器查询 |
| idx_bdopsflow_task_executions_status | status | - | 状态查询 |
| idx_bdopsflow_task_executions_start_time | start_time | - | 时间范围查询 |

---

### 5. 执行器表 (bdopsflow_executors)

存储执行器注册信息。

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_executors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    executor_id TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    address TEXT,
    status TEXT DEFAULT 'offline',
    last_heartbeat DATETIME,
    capacity INTEGER DEFAULT 10,
    current_load INTEGER DEFAULT 0,
    is_global BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**字段说明**：

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | INTEGER | PRIMARY KEY, AUTOINCREMENT | 主键 |
| executor_id | TEXT | NOT NULL, UNIQUE | 执行器唯一 ID |
| name | TEXT | NOT NULL | 执行器名称 |
| address | TEXT | - | 执行器地址 |
| status | TEXT | DEFAULT 'offline' | 在线状态 |
| last_heartbeat | DATETIME | - | 最后心跳时间 |
| capacity | INTEGER | DEFAULT 10 | 最大并发数 |
| current_load | INTEGER | DEFAULT 0 | 当前负载 |
| is_global | BOOLEAN | DEFAULT 0 | 是否全局执行器 |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |
| updated_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 更新时间 |

**索引**：

| 索引名 | 字段 | 类型 | 说明 |
|--------|------|------|------|
| idx_bdopsflow_executors_executor_id | executor_id | UNIQUE | 执行器 ID 唯一性 |
| idx_bdopsflow_executors_status | status | - | 状态查询 |

---

### 6. 工作流表 (bdopsflow_workflows)

存储工作流配置信息。

```sql
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
```

**字段说明**：

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | INTEGER | PRIMARY KEY, AUTOINCREMENT | 主键 |
| name | TEXT | NOT NULL | 工作流名称 |
| description | TEXT | - | 工作流描述 |
| domain_id | INTEGER | NOT NULL, FK | 所属领域 ID |
| dag_config | TEXT | - | DAG 配置（JSON） |
| cron_expression | TEXT | - | Cron 表达式 |
| is_enabled | BOOLEAN | DEFAULT 1 | 是否启用 |
| created_by | INTEGER | - | 创建者 ID |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |
| updated_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 更新时间 |

**索引**：

| 索引名 | 字段 | 类型 | 说明 |
|--------|------|------|------|
| idx_bdopsflow_workflows_name | name | - | 名称查询 |
| idx_bdopsflow_workflows_domain_id | domain_id | - | 领域查询 |
| idx_bdopsflow_workflows_is_enabled | is_enabled | - | 启用状态查询 |

---

### 7. 角色表 (bdopsflow_roles)

存储角色定义信息。

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_roles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    code TEXT NOT NULL UNIQUE,
    description TEXT,
    is_system BOOLEAN DEFAULT 0,
    domain_id INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (domain_id) REFERENCES bdopsflow_domains(id) ON DELETE CASCADE
);
```

**字段说明**：

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | INTEGER | PRIMARY KEY, AUTOINCREMENT | 主键 |
| name | TEXT | NOT NULL | 角色名称 |
| code | TEXT | NOT NULL, UNIQUE | 角色代码（唯一） |
| description | TEXT | - | 角色描述 |
| is_system | BOOLEAN | DEFAULT 0 | 是否系统角色 |
| domain_id | INTEGER | FK | 所属领域（系统角色为空） |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |
| updated_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 更新时间 |

**索引**：

| 索引名 | 字段 | 类型 | 说明 |
|--------|------|------|------|
| idx_bdopsflow_roles_code | code | UNIQUE | 代码唯一性 |
| idx_bdopsflow_roles_domain_id | domain_id | - | 领域查询 |

---

### 8. 权限表 (bdopsflow_permissions)

存储权限定义信息。

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_permissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    resource TEXT NOT NULL,
    action TEXT NOT NULL,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(resource, action)
);
```

**字段说明**：

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | INTEGER | PRIMARY KEY, AUTOINCREMENT | 主键 |
| resource | TEXT | NOT NULL | 资源标识 |
| action | TEXT | NOT NULL | 操作标识 |
| description | TEXT | - | 权限描述 |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |

**索引**：

| 索引名 | 字段 | 类型 | 说明 |
|--------|------|------|------|
| idx_bdopsflow_permissions_resource_action | resource, action | UNIQUE | 资源操作唯一性 |

---

## 索引优化

### rqlite 索引特点

1. **自动索引**：PRIMARY KEY 和 UNIQUE 约束自动创建索引
2. **部分索引**：支持 WHERE 子句创建部分索引
3. **表达式索引**：支持在表达式上创建索引

### 索引策略

#### 高频查询索引

```sql
-- 任务列表查询（领域 + 状态）
CREATE INDEX idx_bdopsflow_tasks_domain_status 
ON bdopsflow_tasks(domain_id, status);

-- 执行记录查询（任务 + 时间）
CREATE INDEX idx_bdopsflow_task_executions_task_time 
ON bdopsflow_task_executions(task_id, start_time DESC);

-- 用户查询（领域 + 角色）
CREATE INDEX idx_bdopsflow_users_domain_role 
ON bdopsflow_users(domain_id, role);
```

#### 部分索引

```sql
-- 只索引启用的定时任务
CREATE INDEX idx_bdopsflow_tasks_cron_enabled 
ON bdopsflow_tasks(is_enabled, cron_expression) 
WHERE cron_expression != '';

-- 只索引在线执行器
CREATE INDEX idx_bdopsflow_executors_online 
ON bdopsflow_executors(status, last_heartbeat) 
WHERE status = 'online';
```

### 索引维护

```sql
-- 查看索引使用情况
PRAGMA index_list('bdopsflow_tasks');
PRAGMA index_info('idx_bdopsflow_tasks_domain_id');
```

---

## 数据关系

### ER 关系图

```
┌─────────────────────┐
│ bdopsflow_domains   │ (1)
├─────────────────────┤
│ id                  │
│ name                │
│ description         │
└─────────────────────┘
        │
        │ 1:N
        ▼
┌─────────────────────┐     ┌─────────────────────┐
│ bdopsflow_users     │     │ bdopsflow_workflows │
├─────────────────────┤     ├─────────────────────┤
│ id                  │     │ id                  │
│ domain_id ──────────┼────►│ domain_id           │
│ role                │     │ dag_config          │
└─────────────────────┘     └─────────────────────┘
        │                            │
        │ 1:N                        │ 1:N
        ▼                            ▼
┌─────────────────────┐     ┌─────────────────────┐
│  bdopsflow_tasks    │     │  bdopsflow_tasks    │
├─────────────────────┤     ├─────────────────────┤
│ id                  │◄────│ workflow_id         │
│ workflow_id         │     │ name                │
│ domain_id           │     │ type                │
│ cron_expression     │     │ config              │
└─────────────────────┘     └─────────────────────┘
        │
        │ 1:N
        ▼
┌─────────────────────────────┐
│ bdopsflow_task_executions  │
├─────────────────────────────┤
│ id                          │
│ task_id                     │
│ execution_id                 │
│ executor_id                 │
│ status                      │
│ start_time                  │
│ end_time                    │
└─────────────────────────────┘
```

### 级联删除规则

| 父表 | 子表 | 删除规则 | 说明 |
|------|------|----------|------|
| domains | users | SET NULL | 删除领域，用户领域置空 |
| domains | workflows | CASCADE | 删除领域，工作流一并删除 |
| domains | roles | CASCADE | 删除领域，角色一并删除 |
| workflows | tasks | CASCADE | 删除工作流，任务一并删除 |
| tasks | task_executions | CASCADE | 删除任务，执行记录一并删除 |

---

## 初始化数据

### 默认管理员

```sql
-- 管理员账户
INSERT INTO bdopsflow_users (username, password, email, role, is_active)
VALUES ('admin', '$2a$10$...', 'admin@example.com', 'system_admin', 1);
```

### 默认角色

```sql
-- 系统管理员
INSERT INTO bdopsflow_roles (name, code, description, is_system)
VALUES ('系统管理员', 'system_admin', '系统最高权限', 1);

-- 领域管理员
INSERT INTO bdopsflow_roles (name, code, description, is_system)
VALUES ('领域管理员', 'domain_admin', '领域级管理权限', 1);

-- 普通用户
INSERT INTO bdopsflow_roles (name, code, description, is_system)
VALUES ('普通用户', 'user', '基础权限', 1);
```

### 默认权限

```sql
-- 任务权限
INSERT INTO bdopsflow_permissions (resource, action, description)
VALUES 
  ('task', 'create', '创建任务'),
  ('task', 'read', '查看任务'),
  ('task', 'update', '更新任务'),
  ('task', 'delete', '删除任务'),
  ('task', 'trigger', '触发任务'),
  ('task', 'manage', '管理任务');

-- 工作流权限
INSERT INTO bdopsflow_permissions (resource, action, description)
VALUES 
  ('workflow', 'create', '创建工作流'),
  ('workflow', 'read', '查看工作流'),
  ('workflow', 'update', '更新工作流'),
  ('workflow', 'delete', '删除工作流'),
  ('workflow', 'manage', '管理工作流');
```

---

## 备份与恢复

### 手动备份

```bash
# 使用 rqlite API 备份
curl http://localhost:4001/db/backup > backup_$(date +%Y%m%d).sql

# 使用文件系统备份
cp /var/lib/rqlite/file.db /backup/backup.db
```

### 自动备份脚本

```bash
#!/bin/bash
# backup.sh

BACKUP_DIR="/backup/rqlite"
DATE=$(date +%Y%m%d_%H%M%S)

# 创建备份目录
mkdir -p $BACKUP_DIR

# 备份数据库
curl -s http://localhost:4001/db/backup > $BACKUP_DIR/backup_$DATE.sql

# 保留最近 7 天备份
find $BACKUP_DIR -name "backup_*.sql" -mtime +7 -delete

echo "Backup completed: $BACKUP_DIR/backup_$DATE.sql"
```

### 恢复数据

```bash
# 停止 rqlite
docker stop bdopsflow-rqlite

# 恢复数据
curl -X POST 'http://localhost:4001/db/load?pretty' --data-binary @backup.sql

# 重启 rqlite
docker start bdopsflow-rqlite
```

---

## 性能优化

### 查询优化

#### 避免全表扫描

```sql
-- 不推荐：LIKE 前面带通配符
SELECT * FROM bdopsflow_tasks WHERE name LIKE '%test%';

-- 推荐：使用索引列
SELECT * FROM bdopsflow_tasks WHERE domain_id = 1 AND status = 'pending';
```

#### 分页查询优化

```sql
-- 不推荐：OFFSET 大值
SELECT * FROM bdopsflow_task_executions 
ORDER BY id LIMIT 20 OFFSET 10000;

-- 推荐：使用游标分页
SELECT * FROM bdopsflow_task_executions 
WHERE id < 10000 
ORDER BY id DESC LIMIT 20;
```

### 连接池配置

```yaml
database:
  max_open_conns: 25
  max_idle_conns: 10
  conn_max_lifetime: 3600
```

### 监控指标

```bash
# 查看数据库状态
curl http://localhost:4001/status

# 查看节点信息
curl http://localhost:4001/nodes

# 查看健康状态
curl http://localhost:4001/health
```

### 常见性能问题

| 问题 | 原因 | 解决方案 |
|------|------|----------|
| 查询慢 | 缺少索引 | 添加适当索引 |
| 写入慢 | 事务过大 | 减小事务范围 |
| 内存高 | 缓存过大 | 调整缓存配置 |
| 连接池耗尽 | 连接泄漏 | 检查连接释放 |

---

## 迁移指南

### 添加新表

1. 在 schema.sql 中添加表定义
2. 添加适当的索引
3. 更新本文档

### 添加新字段

```sql
-- 添加字段
ALTER TABLE bdopsflow_tasks ADD COLUMN new_field TEXT;

-- 添加索引
CREATE INDEX idx_bdopsflow_tasks_new_field ON bdopsflow_tasks(new_field);
```

### 数据迁移

```sql
-- 迁移数据示例
UPDATE bdopsflow_tasks 
SET new_field = old_field 
WHERE old_field IS NOT NULL;
```

---

## 最佳实践

1. **索引策略**
   - 为高频查询字段创建索引
   - 使用部分索引减少索引大小
   - 定期分析索引使用情况

2. **事务管理**
   - 保持事务简短
   - 避免长事务锁定
   - 使用批量操作减少事务数量

3. **数据清理**
   - 定期清理过期执行记录
   - 归档历史数据
   - 压缩数据库文件

4. **监控告警**
   - 监控查询响应时间
   - 监控磁盘使用空间
   - 监控连接池使用率

5. **备份策略**
   - 每日全量备份
   - 实时增量备份
   - 异地容灾备份
