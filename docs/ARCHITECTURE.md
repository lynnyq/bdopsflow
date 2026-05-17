# BDopsFlow 架构设计文档

本文档详细描述了 BDopsFlow 分布式工作流调度平台的系统架构、核心组件和设计原理。

## 目录

- [系统架构概览](#系统架构概览)
- [核心组件](#核心组件)
- [通信机制](#通信机制)
- [数据模型](#数据模型)
- [权限系统架构](#权限系统架构)
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
│   │   └── domain_executor.go  # 领域执行器关系
│   ├── service/                # 业务逻辑
│   │   ├── scheduler.go         # 核心调度服务
│   │   ├── permission_service.go # 权限服务
│   │   ├── role_admin.go        # 角色管理
│   │   ├── user_admin.go        # 用户管理
│   │   ├── domain_admin.go      # 领域管理
│   │   └── executor_domain.go   # 执行器领域分配
│   ├── handler/                # HTTP 处理器
│   │   ├── task.go             # 任务接口
│   │   ├── workflow.go         # 工作流接口
│   │   ├── executor.go         # 执行器接口
│   │   ├── auth.go             # 认证接口
│   │   ├── log.go              # 日志接口
│   │   ├── role_admin.go       # 角色管理接口
│   │   ├── user_admin.go       # 用户管理接口
│   │   ├── domain_admin.go     # 领域管理接口
│   │   └── permission_handler.go # 权限接口
│   ├── grpcserver/             # gRPC 服务端
│   ├── middleware/             # 中间件
│   │   └── auth.go             # JWT/RBAC 中间件
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
- `/api/admin/*` - 管理员接口（用户、角色、领域）
- `/api/permissions/*` - 权限相关接口

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
│ executor_id         │
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

#### 领域管理员
- task: create/read/update/delete/trigger/manage
- executor: read/assign/manage
- log: read/delete/manage
- workflow: create/read/update/delete/manage
- permission: read

#### 普通用户
- task: read/trigger
- executor: read
- log: read
- workflow: read

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

## 高可用设计

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
   │   - executor_id                │
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
- `executor_id`：执行器 ID
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
| database.rqlite_dsn | DATABASE_RQLITE_DSN | http://localhost:4001 | rqlite 地址 |
| redis.addr | REDIS_ADDR | localhost:6379 | Redis 地址 |
| redis.password | REDIS_PASSWORD | (空) | Redis 密码 |
| redis.db | REDIS_DB | 0 | Redis 数据库 |
| jwt.secret | JWT_SECRET | (必填) | JWT 密钥 |
| jwt.expiry_hours | JWT_EXPIRY_HOURS | 24 | Token 过期时间 |
| log.level | LOG_LEVEL | info | 日志级别 |
| log.format | LOG_FORMAT | json | 日志格式 |

### 执行器配置

| 配置项 | 环境变量 | 默认值 | 说明 |
|--------|----------|--------|------|
| app.executor_id | APP_EXECUTOR_ID | executor-1 | 执行器唯一 ID |
| app.executor_name | APP_EXECUTOR_NAME | executor-1 | 执行器名称 |
| app.capacity | APP_CAPACITY | 10 | 最大并发任务数 |
| scheduler.addr | SCHEDULER_ADDR | localhost:50051 | 调度中心地址 |
| scheduler.timeout | SCHEDULER_TIMEOUT | 30 | 连接超时（秒） |

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

### 水平扩展

- **调度中心**：多实例部署，通过 Redis 选举主节点
- **执行器**：动态注册，自动负载均衡
- **数据库**：rqlite 集群部署
- **Redis**：主从或集群模式
