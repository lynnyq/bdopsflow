# BDopsFlow 架构设计文档

本文档详细描述了 BDopsFlow 分布式工作流调度平台的系统架构、核心组件和设计原理。

## 目录

- [系统架构概览](#系统架构概览)
- [核心组件](#核心组件)
- [通信机制](#通信机制)
- [数据模型](#数据模型)
- [权限系统架构](#权限系统架构)
- [审计日志架构](#审计日志架构)
- [数据源查询架构](#数据源查询架构)
- [高可用设计](#高可用设计)
- [任务调度机制](#任务调度机制)
- [执行器管理](#执行器管理)
- [锁续期机制](#锁续期机制)

---

## 系统架构概览

BDopsFlow 采用分布式架构设计，由调度中心（Scheduler）和执行器（Executor）两个核心组件组成。

```
┌─────────────────────────────────────────────────────────────────┐
│                         用户界面层                              │
│                    (Vue3 + Element Plus)                        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                       调度中心 (Scheduler)                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│  │ HTTP API    │  │ gRPC Server │  │ Cron 调度器 │             │
│  │ (Gin)       │  │             │  │             │             │
│  └─────────────┘  └─────────────┘  └─────────────┘             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│  │ 任务服务    │  │ 执行器管理  │  │ 分布式锁    │             │
│  └─────────────┘  └─────────────┘  └─────────────┘             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│  │ 权限服务    │  │ RBAC 中间件 │  │ 领域管理    │             │
│  └─────────────┘  └─────────────┘  └─────────────┘             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│  │ 审计日志    │  │ 数据源管理  │  │ SQL 查询    │             │
│  └─────────────┘  └─────────────┘  └─────────────┘             │
└─────────────────────────────────────────────────────────────────┘
         │                    │                    │
         ▼                    ▼                    ▼
┌─────────────┐    ┌─────────────────┐    ┌─────────────┐
│   rqlite    │    │     Redis       │    │  Executor   │
│  (数据库)   │    │ (缓存/锁/选举)  │    │  (执行器)   │
└─────────────┘    └─────────────────┘    └─────────────┘
                                   │
                                   ▼
                         ┌─────────────────┐
                         │   任务执行      │
                         │ (HTTP/Shell)    │
                         └─────────────────┘
```

### 架构特点

1. **完全解耦**：调度中心和执行器独立部署，通过 gRPC 通信
2. **分布式存储**：使用 rqlite 分布式数据库，支持集群部署
3. **高可用**：支持多调度中心实例，通过 Redis 选举主节点
4. **弹性扩展**：执行器可动态注册和下线，自动负载均衡
5. **RBAC 权限**：完整的角色权限控制和多租户隔离
6. **全量审计**：中间件+Handler 协作模式，自动记录所有写操作
7. **数据源查询**：支持 9 种数据源类型，统一 SQL 查询接口

---

## 核心组件

### 1. 调度中心 (Scheduler)

调度中心是整个系统的核心，负责任务的调度、管理和监控。

**主要职责**：
- 提供 HTTP API 接口供前端调用
- 提供 gRPC 服务供执行器连接
- 管理任务的生命周期
- 调度定时任务（Cron）
- 管理执行器注册和心跳
- 分发任务到执行器
- 处理任务执行结果
- 清理卡死任务
- 提供完整的权限控制（RBAC）
- 领域资源隔离管理

**目录结构**：
```
scheduler/
├── cmd/main.go                 # 启动入口
├── internal/
│   ├── config/                 # 配置管理
│   ├── model/                  # 数据模型
│   │   ├── models.go            # 基础模型
│   │   ├── permission.go        # 权限模型
│   │   ├── role.go             # 角色模型
│   │   ├── user_role.go        # 用户角色关系
│   │   ├── domain_executor.go  # 领域执行器关系
│   │   ├── audit_log.go        # 审计日志模型
│   │   ├── datasource.go       # 数据源模型
│   │   └── query_history.go    # 查询历史模型
│   ├── service/                # 业务逻辑
│   │   ├── scheduler.go         # 核心调度服务
│   │   ├── permission_service.go # 权限服务
│   │   ├── role_admin.go        # 角色管理
│   │   ├── user_admin.go        # 用户管理
│   │   ├── domain_admin.go      # 领域管理
│   │   ├── executor_domain.go   # 执行器领域分配
│   │   └── audit_log.go         # 审计日志服务
│   ├── handler/                # HTTP 处理器
│   │   ├── task.go             # 任务接口
│   │   ├── workflow.go         # 工作流接口
│   │   ├── executor.go         # 执行器接口
│   │   ├── auth.go             # 认证接口
│   │   ├── log.go              # 日志接口
│   │   ├── role_admin.go       # 角色管理接口
│   │   ├── user_admin.go       # 用户管理接口
│   │   ├── domain_admin.go     # 领域管理接口
│   │   ├── permission_handler.go # 权限接口
│   │   ├── audit_log.go        # 审计日志接口
│   │   ├── datasource.go       # 数据源接口
│   │   ├── query.go            # SQL 查询接口
│   │   └── system_config.go    # 系统配置接口
│   ├── datasource/             # 数据源管理
│   │   ├── driver/             # 数据源驱动
│   │   │   ├── base.go         # 驱动接口定义
│   │   │   ├── mysql.go        # MySQL 驱动
│   │   │   ├── sqlite.go       # SQLite 驱动
│   │   │   ├── hive.go         # Hive 驱动
│   │   │   ├── kyuubi.go       # Kyuubi 驱动
│   │   │   ├── spark.go        # Spark 驱动
│   │   │   ├── trino.go        # Trino 驱动
│   │   │   ├── starrocks.go    # StarRocks 驱动
│   │   │   ├── doris.go        # Doris 驱动
│   │   │   └── rqlite_driver.go # Rqlite 驱动
│   │   ├── manager.go          # 连接池管理
│   │   ├── service.go          # 数据源服务
│   │   └── crypto.go           # 密码加密
│   ├── grpcserver/             # gRPC 服务端
│   ├── middleware/             # 中间件
│   │   ├── auth.go             # JWT/RBAC 中间件
│   │   └── audit.go            # 审计日志中间件
│   ├── cron/                   # Cron 调度
│   ├── lineage/                # 血缘关系
│   └── webhook/                # Webhook 服务
└── pkg/
    ├── election/               # 主节点选举
    └── lock/                   # 分布式锁
```

### 2. 执行器 (Executor)

执行器负责实际执行任务，支持 HTTP 和 Shell 两种任务类型。

**主要职责**：
- 向调度中心注册
- 维护心跳连接
- 接收并执行任务
- 上报任务执行结果
- 上报执行日志
- 追踪运行中的任务

**目录结构**：
```
executor/
├── cmd/main.go                 # 启动入口
└── internal/
    ├── config/                 # 配置管理
    ├── executor/               # 任务执行器
    │   └── task_executor.go    # 执行逻辑
    ├── pool/                   # 协程池
    ├── grpcclient/             # gRPC 客户端
    └── logger/                 # 日志管理
```

### 3. 前端 (Web)

基于 Vue3 的管理界面，提供任务、工作流、执行器的可视化管理。

**主要功能**：
- 用户登录认证
- 任务管理（创建、编辑、删除、触发）
- 工作流管理
- 执行器监控
- 执行日志查看
- 仪表盘统计
- 用户管理（仅管理员）
- 角色管理（仅管理员）
- 领域管理（仅管理员）
- 执行器分配（仅管理员）
- 数据源管理（配置、测试、权限）
- SQL 查询（执行、导出、历史、保存 SQL）
- 审计日志查看（仅系统管理员）
- 系统配置管理（仅系统管理员）

---

## 通信机制

### HTTP API

前端与调度中心通过 HTTP REST API 通信。

**认证方式**：JWT Token

```
请求头: Authorization: Bearer <token>
```

**主要接口**：
- `/api/auth/*` - 认证相关
- `/api/tasks/*` - 任务管理
- `/api/workflows/*` - 工作流管理
- `/api/executors/*` - 执行器管理
- `/api/logs/*` - 日志查询
- `/api/admin/*` - 管理员接口（用户、角色、领域、审计日志、系统配置）
- `/api/permissions/*` - 权限相关接口
- `/api/datasources/*` - 数据源管理
- `/api/query/*` - SQL 查询与导出

### gRPC 通信

调度中心与执行器通过 gRPC 双向流通信。

**Proto 定义**：

```protobuf
service ExecutorService {
  rpc Register(RegisterRequest) returns (RegisterResponse);
  rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
  rpc SubscribeTask(SubscribeTaskRequest) returns (stream Task);
  rpc ReportTaskResult(ReportTaskResultRequest) returns (ReportTaskResultResponse);
  rpc ReportTaskLog(ReportTaskLogRequest) returns (ReportTaskLogResponse);
}
```

**通信流程**：

```
Executor                          Scheduler
   │                                 │
   │── Register ─────────────────▶│  注册
   │◀── RegisterResponse ──────────│
   │                                 │
   │── Heartbeat (周期性) ────────▶│  心跳
   │◀── HeartbeatResponse ─────────│
   │                                 │
   │── SubscribeTask ─────────────▶│  订阅任务
   │◀── stream Task ───────────────│  接收任务
   │                                 │
   │── ReportTaskResult ──────────▶│  上报结果
   │◀── ReportTaskResultResponse ──│
```

---

## 数据模型

### 数据库表

所有表名已添加 `bdopsflow_` 前缀：

| 表名 | 说明 |
|------|------|
| bdopsflow_domains | 领域表 |
| bdopsflow_users | 用户表 |
| bdopsflow_workflows | 工作流表 |
| bdopsflow_tasks | 任务表 |
| bdopsflow_task_executions | 任务执行记录表 |
| bdopsflow_executors | 执行器表 |
| bdopsflow_workflow_executions | 工作流执行记录表 |
| bdopsflow_task_dependencies | 任务依赖表 |
| bdopsflow_task_logs | 任务执行日志表 |
| bdopsflow_roles | 角色表 |
| bdopsflow_permissions | 权限表 |
| bdopsflow_role_permissions | 角色权限映射表 |
| bdopsflow_user_roles | 用户角色映射表 |
| bdopsflow_domain_executors | 执行器领域分配表 |
| bdopsflow_datasources | 数据源表 |
| bdopsflow_saved_sql | 保存的SQL表 |
| bdopsflow_datasource_permissions | 数据源权限表 |
| bdopsflow_query_history | 查询历史表 |
| bdopsflow_system_config | 系统配置表 |
| bdopsflow_system_config_history | 配置变更历史表 |
| bdopsflow_audit_logs | 审计日志表 |

### ER 图

```
┌─────────────────────┐     ┌───────────────────┐
│ bdopsflow_roles    │     │ bdopsflow_domains│
├─────────────────────┤     ├───────────────────┤
│ id                 │     │ id               │
│ name               │     │ name             │
│ code               │     │ description      │
│ is_system          │     │ created_at       │
│ domain_id          │◀────│                  │
│ created_at         │     └───────────────────┘
└─────────────────────┘           ▲
         │                       │
         │                       │
         ▼                       │
┌─────────────────────┐           │
│bdopsflow_role_perms │           │
├─────────────────────┤           │
│ id                  │           │
│ role_id            │───────────┘
│ permission_id       │
│ created_at          │
└─────────────────────┘
         │
         ▼
┌─────────────────────┐
│bdopsflow_permissions│
├─────────────────────┤
│ id                  │
│ resource            │
│ action              │
│ description         │
│ created_at          │
└─────────────────────┘


┌─────────────────────┐
│ bdopsflow_users    │
├─────────────────────┤
│ id                  │
│ username            │
│ real_name           │
│ phone               │
│ password            │
│ email               │
│ domain_id           │◀───────────────────────────┐
│ role                │                           │
│ is_active           │                           │
│ last_login_at      │                           │
│ created_by          │                           │
│ created_at          │                           │
│ updated_at          │                           │
└─────────────────────┘                           │
         │                                        │
         │                                        │
         ▼                                        │
┌─────────────────────┐                           │
│bdopsflow_user_roles│                           │
├─────────────────────┤                           │
│ id                  │                           │
│ user_id             │───────────────────────────┘
│ role_id             │
│ domain_id           │◀───────────────────────────┘
│ created_at          │                           │
└─────────────────────┘                           │
         │                                        │
         │                                        │
         ▼                                        │
┌─────────────────────┐                           │
│ bdopsflow_workflows│                           │
├─────────────────────┤                           │
│ id                  │                           │
│ name                │                           │
│ description         │                           │
│ domain_id           │◀───────────────────────────┘
│ dag_config          │
│ cron_expression     │
│ is_enabled          │
│ created_by          │
│ created_at          │
│ updated_at          │
└─────────────────────┘
         │
         │
         ▼
┌─────────────────────┐
│  bdopsflow_tasks   │
├─────────────────────┤
│ id                  │
│ workflow_id         │
│ name                │
│ type                │
│ config              │
│ cron_expression     │
│ timeout_seconds     │
│ retry_count         │
│ retry_interval      │
│ is_enabled          │
│ status              │
│ domain_id           │
│ webhook_config      │
│ assigned_executor_id│
│ created_by          │
│ created_at          │
│ updated_at          │
└─────────────────────┘
         │
         │
         ▼
┌─────────────────────┐
│bdopsflow_task_execs │
├─────────────────────┤
│ id                  │
│ task_id             │
│ execution_id        │
│ executor_id         │
│ status              │
│ start_time          │
│ end_time            │
│ output              │
│ error               │
│ retry_times         │
│ created_at          │
└─────────────────────┘

┌─────────────────────┐
│ bdopsflow_executors│
├─────────────────────┤
│ id                  │
│ name                │
│ address             │
│ status              │
│ last_heartbeat      │
│ capacity            │
│ current_load        │
│ is_global           │
│ created_at          │
│ updated_at          │
└─────────────────────┘
         │
         │
         ▼
┌─────────────────────┐
│bdopsflow_domain_exec│
├─────────────────────┤
│ id                  │
│ domain_id           │
│ executor_id         │
│ assigned_by         │
│ created_at          │
└─────────────────────┘
```

---

## 权限系统架构

### 设计理念

BDopsFlow 采用 RBAC（基于角色的访问控制）架构，结合领域隔离实现多租户能力。

```
权限检查流程：

[用户] → [JWT 认证] → [获取用户角色] → [获取角色权限] → [权限验证]
              │
              └→ [领域检查] → [资源领域验证]
```

### 核心概念

#### 1. 资源（Resource）

系统中的可访问对象：

| 资源 | 说明 |
|------|------|
| user | 用户管理 |
| role | 角色管理 |
| permission | 权限查看 |
| domain | 领域管理 |
| executor | 执行器管理 |
| task | 任务管理 |
| log | 日志管理 |
| workflow | 工作流管理 |
| datasource | 数据源管理 |
| audit_log | 审计日志 |
| config | 系统配置 |

#### 2. 操作（Action）

对资源可执行的操作：

| 操作 | 说明 |
|------|------|
| create | 创建 |
| read | 读取 |
| update | 更新 |
| delete | 删除 |
| trigger | 触发（任务专用） |
| assign | 分配（执行器专用） |
| manage | 管理（所有操作） |

#### 3. 角色（Role）

权限的集合，可分配给用户：

| 系统角色 | 说明 | 范围 |
|---------|------|------|
| 系统管理员 | 系统最高权限，可管理所有资源 | 全局 |
| 领域管理员 | 领域级管理权限 | 指定领域 |
| 普通用户 | 基础查看和操作权限 | 指定领域 |

#### 4. 领域（Domain）

资源隔离的边界：

- 所有任务、工作流都绑定到领域
- 执行器可分配到一个或多个领域
- 执行器可设置为全局（所有领域可用）
- 用户在不同领域可拥有不同角色

### 权限检查流程

```
1. 解析 JWT Token，获取用户信息
   ↓
2. 检查用户是否活跃
   ↓
3. 获取用户在请求资源领域的角色
   ↓
4. 检查用户是否有请求操作的权限
   ↓
5. 特殊规则：
   - 系统管理员拥有所有权限
   - 资源领域必须与用户权限领域匹配
```

### 数据模型关系

```
User (1) ─── (N) UserRole (N) ─── (1) Role
                              │
                              └── (N) RolePermission (N) ─── (1) Permission

Role (N) ─── (1) Domain (可空)

Task (N) ─── (1) Domain
Workflow (N) ─── (1) Domain
Executor (N) ─── (N) Domain (通过 DomainExecutor)
```

### 预设权限分配

#### 系统管理员
- 所有资源的所有权限
- 跨领域访问权限
- 审计日志查看、删除、管理
- 系统配置管理

#### 领域管理员
- task: create/read/update/delete/trigger/manage
- executor: read/assign/manage
- log: read/delete/manage
- workflow: create/read/update/delete/manage
- datasource: create/read/update/manage/query/download
- permission: read

#### 普通用户
- task: read/trigger
- executor: read
- log: read
- workflow: read
- datasource: read/query

### 中间件实现

权限检查在 Gin 中间件中实现：

```go
// 伪代码示例
func RBACMiddleware(permissionService *PermissionService) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 1. 从 JWT 获取用户 ID
        userID := getUserIDFromJWT(c)
        
        // 2. 解析请求的资源和操作
        resource := parseResource(c.Request.URL.Path)
        action := parseAction(c.Request.Method)
        
        // 3. 从请求中获取领域 ID（如适用）
        domainID := getDomainIDFromRequest(c)
        
        // 4. 检查权限
        hasPermission := permissionService.CheckPermission(
            userID,
            domainID,
            resource,
            action,
        )
        
        if !hasPermission {
            c.AbortWithStatusJSON(403, gin.H{"error": "Forbidden"})
            return
        }
        
        c.Next()
    }
}
```

---

## 审计日志架构

### 设计理念

BDopsFlow 采用"中间件+Handler 协作"模式实现全量审计日志，自动记录所有写操作（POST/PUT/DELETE），无需在每个 Handler 中手动编写日志逻辑。

### 架构模式

```
请求 → JWT认证中间件 → 审计中间件 → RBAC中间件 → Handler
                                    │                │
                                    │   c.Next()     │ c.Set("audit_*")
                                    │◄───────────────│
                                    │                │
                                    ▼                │
                          resolveAuditInfo()         │
                          + Handler覆盖              │
                                    │                │
                                    ▼                │
                          异步写入审计日志（goroutine） │
```

### 核心组件

#### 1. 审计中间件 (`middleware/audit.go`)

- 仅拦截 POST/PUT/DELETE 请求
- `c.Next()` 后收集信息，不阻塞请求响应
- `resolveAuditInfo()` 通过路由规则表解析 resource 和 action
- Handler 可通过 `c.Set()` 覆盖默认值

#### 2. 路由解析规则

| 规则类型 | 说明 | 示例 |
|---------|------|------|
| 精确匹配 | `routeAuditRules` 完整路径匹配 | `/api/auth/login` → auth/login |
| 前缀匹配 | `routePrefixRules` 路径前缀匹配 | `/api/tasks/` → task |
| 关键词推断 | 路径关键词推断 action | `/trigger` → trigger |

#### 3. Handler 埋点

Handler 通过 `c.Set()` 传递业务语义：

```go
c.Set("audit_action", "assign")
c.Set("audit_resource_id", fmt.Sprintf("%d", id))
c.Set("audit_resource_name", name)
c.Set("audit_detail", detail)
```

#### 4. 审计日志服务 (`service/audit_log.go`)

- `Create()`: 写入审计日志
- `List()`: 多条件筛选和分页查询
- `CleanExpired()`: 根据保留天数清理过期日志
- `GetRetentionDays()`: 从系统配置读取保留天数（默认90天）

#### 5. 定时清理

调度中心启动时创建定时协程，每24小时自动清理过期审计日志：

```go
go func() {
    ticker := time.NewTicker(24 * time.Hour)
    for range ticker.C {
        retentionDays := auditLogService.GetRetentionDays()
        auditLogService.CleanExpired(ctx, retentionDays)
    }
}()
```

### 审计日志数据模型

| 字段 | 说明 |
|------|------|
| user_id | 操作用户ID |
| username | 操作用户名 |
| role | 用户角色 |
| domain_id | 所属领域ID |
| action | 操作类型（create/update/delete/login/trigger等） |
| resource | 资源类型（task/datasource/user等） |
| resource_id | 资源ID |
| resource_name | 资源名称 |
| status | 操作结果（success/failure） |
| ip_address | 客户端IP |
| user_agent | 客户端UA |
| request_method | HTTP方法 |
| request_path | 请求路径 |
| detail | 操作详情 |
| created_at | 操作时间 |

### 权限控制

审计日志仅系统管理员可查看，权限定义：

| 权限 | 说明 |
|------|------|
| audit_log:read | 查看审计日志 |
| audit_log:delete | 删除审计日志 |
| audit_log:manage | 完整管理审计日志 |

---

## 数据源查询架构

### 设计理念

BDopsFlow 提供统一的数据源管理和 SQL 查询能力，支持 9 种数据源类型，通过 Driver 接口模式实现多数据源适配。

### 支持的数据源类型

| 类型 | 说明 | 连接方式 |
|------|------|---------|
| MySQL | MySQL 数据库 | 直连 |
| SQLite | SQLite 数据库 | 文件路径 |
| Hive | Hive 数据仓库 | Thrift 协议 |
| Kyuubi | Kyuubi SQL 引擎 | Thrift 协议 |
| Spark | Spark Thrift Server | Thrift 协议 |
| Trino | Trino 查询引擎 | HTTP REST |
| StarRocks | StarRocks 数据库 | MySQL 协议 |
| Doris | Apache Doris | MySQL 协议 |
| Rqlite | Rqlite 分布式数据库 | HTTP REST |

### 核心组件

#### 1. Driver 接口 (`datasource/driver/base.go`)

```go
type Driver interface {
    Connect() error
    Close() error
    Ping() error
    GetDatabases() ([]string, error)
    GetTables(database string) ([]string, error)
    GetColumns(database, table string) ([]ColumnInfo, error)
    Query(database, sql string, limit int) (*QueryResult, error)
    UseDatabase(database string) error
}
```

#### 2. 连接池管理 (`datasource/manager.go`)

- 按 `datasource_id` 缓存单个连接
- 连接生命周期管理（创建、复用、关闭）
- 密码加密存储（AES）

#### 3. 查询服务 (`handler/query.go`)

- SQL 执行与结果返回
- CSV 导出
- 查询历史记录
- 保存 SQL
- 并发查询控制（单用户/全局限制）
- 查询结果缓存（Redis）

#### 4. UseDatabase 模式

查询前切换到指定数据库，查询后恢复原始数据库，避免并发状态污染：

```
查询流程：
1. 获取连接
2. 保存当前数据库
3. UseDatabase(targetDB)
4. 执行 SQL（normalizeSQL 去除末尾分号）
5. UseDatabase(originalDB) 恢复
```

### 数据源权限

数据源支持独立的权限控制，通过 `datasource_permissions` 表实现：

| 权限类型 | 说明 |
|---------|------|
| read | 查看数据源 |
| query | 执行查询 |
| download | 导出数据 |
| update | 修改数据源 |
| delete | 删除数据源 |

---

### 1. 分布式锁

使用 Redis 分布式锁确保任务不会重复执行：

```
锁 Key: task:lock:{task_id}:{execution_id}
锁 TTL: 60 秒（自动续期）
```

### 2. 主节点选举

多调度中心实例通过 Redis 选举主节点：

```
选举 Key: scheduler:leader
选举 TTL: 30 秒
```

只有主节点执行任务调度，从节点待命。

### 3. 执行器心跳

执行器每 10 秒发送心跳，调度中心检测心跳超时（60 秒）标记离线。

### 4. 数据库高可用

rqlite 支持 Raft 共识协议，多节点部署保证数据一致性。

---

## 任务调度机制

### Cron 调度

调度中心内置 Cron 调度器，支持标准 5 位 Cron 表达式。

**调度流程**：
1. 每分钟扫描启用的任务
2. 计算下次执行时间
3. 到达执行时间时触发任务
4. 获取分布式锁防止重复执行

### 执行器选择策略

**自动选择**：
1. 过滤在线执行器（心跳在 30 秒内）
2. 过滤有可用容量的执行器
3. 优先选择任务所属领域的执行器
4. 选择当前负载最低的执行器

**指定执行器**：
- 如果任务配置了 `assigned_executor_id`，则只分发到该执行器

---

## 执行器管理

### 注册流程

```
Executor                          Scheduler
   │                                  │
   │── Register ───────────────────▶│
   │   - name                       │
   │   - address                    │
   │   - capacity                   │
   │                                  │
   │◀── RegisterResponse ───────────│
   │   - success: true              │
```

### 心跳机制

**心跳间隔**：10 秒

**心跳内容**：
- `executor_name`：执行器名称
- `current_load`：当前负载（运行中任务数）
- `running_execution_ids`：运行中的执行 ID 列表

**心跳处理**：
1. 更新执行器最后心跳时间
2. 更新执行器当前负载
3. 对运行中的任务进行锁续期

### 离线检测

调度中心每 60 秒检测执行器心跳：
- 超过 60 秒无心跳 → 标记为 offline
- 清理该执行器上的任务

---

## 锁续期机制

为防止执行器异常退出后任务卡死，实现了锁续期机制。

### 工作原理

```
Executor                          Scheduler                        Redis
   │                                  │                               │
   │── Heartbeat ───────────────────▶│                               │
   │   - running_execution_ids       │                               │
   │                                  │                               │
   │                                  │── Renew Lock TTL ────────────▶│
   │                                  │   for each execution_id       │
   │                                  │                               │
   │                                  │◀── Lock Renewed ───────────────│
   │◀── HeartbeatResponse ────────────│                               │
```

### 续期规则

- **锁 TTL**：60 秒
- **续期间隔**：每 10 秒（心跳时续期）
- **续期条件**：执行器心跳携带运行中的任务 ID

### 卡死任务检测

调度中心每 60 秒检测：
1. 检查所有 running 状态的任务
2. 检查锁是否存在或续期状态
3. 连续 3 次未续期 → 标记任务为 failed

---

## 配置说明

### 调度中心配置

| 配置项 | 环境变量 | 默认值 | 说明 |
|--------|----------|--------|------|
| app.http_port | APP_HTTP_PORT | 8080 | HTTP API 端口 |
| app.grpc_port | APP_GRPC_PORT | 50051 | gRPC 端口 |
| database.rqlite_addrs | DATABASE_RQLITE_ADDRS | ["http://localhost:4001"] | rqlite 多节点地址列表（逗号分隔） |
| database.rqlite_user | DATABASE_RQLITE_USER | (空) | rqlite 用户名 |
| database.rqlite_password | DATABASE_RQLITE_PASSWORD | (空) | rqlite 密码 |
| database.rqlite_tls | DATABASE_RQLITE_TLS | false | 是否使用 TLS 连接 rqlite |
| redis.mode | REDIS_MODE | single | Redis 模式：single 或 sentinel |
| redis.addr | REDIS_ADDR | localhost:6379 | Redis 单实例地址 |
| redis.password | REDIS_PASSWORD | (空) | Redis 密码 |
| redis.db | REDIS_DB | 0 | Redis 数据库 |
| redis.master_name | REDIS_MASTER_NAME | mymaster | Redis Sentinel 主节点名称 |
| redis.sentinel_addrs | REDIS_SENTINEL_ADDRS | (空) | Redis Sentinel 节点地址列表（逗号分隔） |
| redis.sentinel_password | REDIS_SENTINEL_PASSWORD | (空) | Redis Sentinel 密码 |
| jwt.secret | JWT_SECRET | (必填) | JWT 密钥 |
| jwt.expiry_hours | JWT_EXPIRY_HOURS | 24 | Token 过期时间 |
| log.level | LOG_LEVEL | info | 日志级别 |
| log.format | LOG_FORMAT | json | 日志格式 |

### 执行器配置

执行器支持配置文件和命令行参数两种配置方式，命令行参数优先级高于配置文件。

#### 配置文件参数

| 配置项 | 环境变量 | 默认值 | 说明 |
|--------|----------|--------|------|
| app.executor_name | APP_EXECUTOR_NAME | (必填) | 执行器名称（必需） |
| app.capacity | APP_CAPACITY | 10 | 最大并发任务数 |
| app.hostname | APP_HOSTNAME | 系统主机名 | 执行器注册地址 |
| scheduler.addr | SCHEDULER_ADDR | (必填) | 调度中心地址（必需） |
| scheduler.timeout | SCHEDULER_TIMEOUT | 30 | 连接超时（秒） |
| log.level | LOG_LEVEL | info | 日志级别 |
| log.format | LOG_FORMAT | json | 日志格式 |

#### 命令行参数（优先级高于配置文件）

| 参数 | 说明 |
|------|------|
| --executor-name | 执行器名称（必需） |
| --scheduler-addr | 调度中心地址（必需） |
| --capacity | 并发任务数，默认 10 |
| --timeout | 超时时间，默认 30 秒 |
| --hostname | 主机名/IP，默认系统主机名 |
| --log-level | 日志级别 |
| --log-format | 日志格式 |

---

## 扩展性设计

### 添加新任务类型

1. 在执行器中实现新的任务执行器
2. 在 `task_executor.go` 的 `Execute` 方法中添加新类型处理
3. 更新前端任务类型选项

### 添加新权限资源

1. 在 `permission.go` 模型中定义新资源常量
2. 在数据库初始化 SQL 中添加相应权限记录
3. 在前端添加权限配置界面
4. 在 RBAC 中间件中添加解析逻辑
5. 在审计中间件 `routeAuditRules` 或 `routePrefixRules` 中添加路由规则

### 添加新数据源类型

1. 在 `datasource/driver/` 中实现 Driver 接口
2. 在 `datasource/manager.go` 中注册新驱动
3. 在前端 `DatasourceForm.vue` 中添加新类型表单模板
4. 在审计中间件路由规则中添加数据源相关路径

### 水平扩展

- **调度中心**：多实例部署，通过 Redis 选举主节点
- **执行器**：动态注册，自动负载均衡
- **数据库**：rqlite 集群部署
- **Redis**：主从或集群模式
