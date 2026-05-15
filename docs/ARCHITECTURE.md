# BDopsFlow 架构设计文档

本文档详细描述了 BDopsFlow 分布式工作流调度平台的系统架构、核心组件和设计原理。

## 目录

- [系统架构概览](#系统架构概览)
- [核心组件](#核心组件)
- [通信机制](#通信机制)
- [数据流](#数据流)
- [高可用设计](#高可用设计)
- [任务调度机制](#任务调度机制)
- [执行器管理](#执行器管理)
- [锁续期机制](#锁续期机制)
- [数据模型](#数据模型)

---

## 系统架构概览

BDopsFlow 采用分布式架构设计，由调度中心（Scheduler）和执行器（Executor）两个核心组件组成。

```
┌─────────────────────────────────────────────────────────────────┐
│                         用户界面层                               │
│                    (Vue3 + Element Plus)                        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                       调度中心 (Scheduler)                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │
│  │ HTTP API    │  │ gRPC Server │  │ Cron 调度器 │              │
│  │ (Gin)       │  │             │  │             │              │
│  └─────────────┘  └─────────────┘  └─────────────┘              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │
│  │ 任务服务    │  │ 执行器管理  │  │ 分布式锁    │              │
│  └─────────────┘  └─────────────┘  └─────────────┘              │
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

**目录结构**：
```
scheduler/
├── cmd/main.go                 # 启动入口
├── internal/
│   ├── config/                 # 配置管理
│   ├── model/                  # 数据模型
│   ├── service/                # 业务逻辑
│   │   └── scheduler.go        # 核心调度服务
│   ├── handler/                # HTTP 处理器
│   │   ├── task.go             # 任务接口
│   │   ├── workflow.go         # 工作流接口
│   │   ├── executor.go         # 执行器接口
│   │   ├── auth.go             # 认证接口
│   │   └── log.go              # 日志接口
│   ├── grpcserver/             # gRPC 服务端
│   │   └── server.go
│   ├── middleware/             # 中间件
│   │   └── auth.go             # JWT/RBAC
│   ├── cron/                   # Cron 调度
│   │   └── cron_scheduler.go
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
├── internal/
│   ├── config/                 # 配置管理
│   ├── executor/               # 任务执行器
│   │   └── task_executor.go    # 执行逻辑
│   ├── pool/                   # 协程池
│   │   └── pool.go
│   ├── grpcclient/             # gRPC 客户端
│   │   └── client.go
│   └── logger/                 # 日志管理
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
   │──── Register ─────────────────▶│  注册
   │◀─── RegisterResponse ──────────│
   │                                 │
   │──── Heartbeat (周期性) ────────▶│  心跳
   │◀─── HeartbeatResponse ─────────│
   │                                 │
   │──── SubscribeTask ─────────────▶│  订阅任务
   │◀─── stream Task ───────────────│  接收任务
   │                                 │
   │──── ReportTaskResult ──────────▶│  上报结果
   │◀─── ReportTaskResultResponse ──│
```

---

## 数据流

### 任务创建流程

```
用户 → 前端 → HTTP API → 调度中心 → rqlite (存储)
```

### 任务执行流程

```
1. Cron 调度器触发 或 用户手动触发
2. 调度中心获取分布式锁
3. 选择可用执行器（负载最低）
4. 通过 gRPC 发送任务到执行器
5. 执行器执行任务
6. 执行器上报结果
7. 调度中心更新任务状态
8. 触发 Webhook（如配置）
```

### 任务状态流转

```
pending → running → success
                  ↘ failed
                  ↘ timeout
```

---

## 高可用设计

### 1. 分布式锁

使用 Redis 分布式锁确保任务不会重复执行：

```go
锁 Key: task:lock:{task_id}:{execution_id}
锁 TTL: 60 秒（自动续期）
```

### 2. 主节点选举

多调度中心实例通过 Redis 选举主节点：

```go
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
3. 选择当前负载最低的执行器

**指定执行器**：
- 如果任务配置了 `assigned_executor_id`，则只分发到该执行器

---

## 执行器管理

### 注册流程

```
Executor                    Scheduler
   │                           │
   │── Register ─────────────▶│
   │   - executor_id          │
   │   - name                 │
   │   - address              │
   │   - capacity             │
   │                           │
   │◀── RegisterResponse ─────│
   │   - success: true        │
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
Executor                    Scheduler                    Redis
   │                           │                          │
   │── Heartbeat ─────────────▶│                          │
   │   - running_execution_ids │                          │
   │                           │                          │
   │                           │── Renew Lock TTL ───────▶│
   │                           │   for each execution_id  │
   │                           │                          │
   │                           │◀── Lock Renewed ─────────│
   │◀── HeartbeatResponse ─────│                          │
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

## 数据模型

### ER 图

```
┌─────────────┐     ┌─────────────┐     ┌─────────────────┐
│   users     │     │  domains    │     │   workflows     │
├─────────────┤     ├─────────────┤     ├─────────────────┤
│ id          │     │ id          │     │ id              │
│ username    │     │ name        │     │ name            │
│ password    │     │ description │     │ description     │
│ email       │     │ created_at  │     │ dag_config      │
│ domain_id   │────▶│             │     │ cron_expression │
│ role        │     └─────────────┘     │ is_enabled      │
│ created_at  │                         │ domain_id       │
└─────────────┘                         │ created_by      │
                                        └─────────────────┘
                                              │
                                              ▼
┌─────────────────┐     ┌─────────────────────┐
│     tasks       │     │   task_executions   │
├─────────────────┤     ├─────────────────────┤
│ id              │     │ id                  │
│ workflow_id     │────▶│ task_id             │
│ name            │     │ execution_id        │
│ type            │     │ executor_id         │
│ config          │     │ status              │
│ cron_expression │     │ start_time          │
│ timeout_seconds │     │ end_time            │
│ retry_count     │     │ output              │
│ retry_interval  │     │ error               │
│ is_enabled      │     │ retry_times         │
│ status          │     │ created_at          │
│ domain_id       │     └─────────────────────┘
│ webhook_config  │
│ assigned_executor_id │
│ created_by      │
└─────────────────┘

┌─────────────────┐     ┌─────────────────┐
│   executors     │     │   task_logs     │
├─────────────────┤     ├─────────────────┤
│ id              │     │ id              │
│ executor_id     │     │ execution_id    │
│ name            │     │ task_id         │
│ address         │     │ executor_id     │
│ status          │     │ node_id         │
│ last_heartbeat  │     │ log_level       │
│ capacity        │     │ message         │
│ current_load    │     │ log_time        │
│ created_at      │     └─────────────────┘
│ updated_at      │
└─────────────────┘
```

### 表结构说明

#### users（用户表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| username | TEXT | 用户名（唯一） |
| password | TEXT | 密码（bcrypt 加密） |
| email | TEXT | 邮箱 |
| domain_id | INTEGER | 所属领域 ID |
| role | TEXT | 角色：admin/operator/viewer |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

#### tasks（任务表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| workflow_id | INTEGER | 所属工作流 ID |
| name | TEXT | 任务名称 |
| type | TEXT | 任务类型：http/shell |
| config | TEXT | 任务配置（JSON） |
| cron_expression | TEXT | Cron 表达式 |
| timeout_seconds | INTEGER | 超时时间（秒） |
| retry_count | INTEGER | 最大重试次数 |
| retry_interval | INTEGER | 重试间隔（秒） |
| is_enabled | BOOLEAN | 是否启用 |
| status | TEXT | 任务状态 |
| domain_id | INTEGER | 所属领域 ID |
| webhook_config | TEXT | Webhook 配置 |
| assigned_executor_id | TEXT | 指定执行器 ID |
| created_by | INTEGER | 创建者 ID |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

#### task_executions（任务执行记录表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| task_id | INTEGER | 任务 ID |
| execution_id | TEXT | 执行 ID（唯一） |
| executor_id | TEXT | 执行器 ID |
| status | TEXT | 执行状态 |
| start_time | DATETIME | 开始时间 |
| end_time | DATETIME | 结束时间 |
| output | TEXT | 执行输出 |
| error | TEXT | 错误信息 |
| retry_times | INTEGER | 已重试次数 |
| created_at | DATETIME | 创建时间 |

#### executors（执行器表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| executor_id | TEXT | 执行器唯一标识 |
| name | TEXT | 执行器名称 |
| address | TEXT | 执行器地址 |
| status | TEXT | 状态：online/offline |
| last_heartbeat | DATETIME | 最后心跳时间 |
| capacity | INTEGER | 最大并发任务数 |
| current_load | INTEGER | 当前运行任务数 |
| created_at | DATETIME | 注册时间 |
| updated_at | DATETIME | 更新时间 |

#### workflows（工作流表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| name | TEXT | 工作流名称 |
| description | TEXT | 描述 |
| domain_id | INTEGER | 所属领域 ID |
| dag_config | TEXT | DAG 配置（JSON） |
| cron_expression | TEXT | Cron 表达式 |
| is_enabled | BOOLEAN | 是否启用 |
| created_by | INTEGER | 创建者 ID |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

#### task_logs（任务日志表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| execution_id | TEXT | 执行 ID |
| task_id | INTEGER | 任务 ID |
| executor_id | TEXT | 执行器 ID |
| node_id | TEXT | 节点 ID |
| log_level | TEXT | 日志级别 |
| message | TEXT | 日志内容 |
| log_time | DATETIME | 日志时间 |

---

## 配置说明

### 调度中心配置

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| app.http_port | 8080 | HTTP API 端口 |
| app.grpc_port | 50051 | gRPC 端口 |
| database.rqlite_dsn | http://localhost:4001 | rqlite 地址 |
| redis.addr | localhost:6379 | Redis 地址 |
| redis.password | (空) | Redis 密码 |
| redis.db | 0 | Redis 数据库 |
| jwt.secret | (必填) | JWT 密钥 |
| jwt.expiry_hours | 24 | Token 过期时间 |

### 执行器配置

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| app.executor_id | executor-1 | 执行器唯一 ID |
| app.executor_name | executor-1 | 执行器名称 |
| app.capacity | 10 | 最大并发任务数 |
| scheduler.addr | localhost:50051 | 调度中心地址 |
| scheduler.timeout | 30 | 连接超时（秒） |

---

## 监控指标

### 关键指标

- **任务执行成功率**：成功任务数 / 总任务数
- **任务平均执行时长**：所有任务执行时长的平均值
- **执行器在线率**：在线执行器数 / 总执行器数
- **执行器负载**：当前运行任务数 / 最大容量

### 日志级别

- `DEBUG`：调试信息
- `INFO`：正常运行信息
- `WARN`：警告信息
- `ERROR`：错误信息

---

## 扩展性设计

### 添加新任务类型

1. 在执行器中实现新的任务执行器
2. 在 `task_executor.go` 的 `Execute` 方法中添加新类型处理
3. 更新前端任务类型选项

### 添加新 API 接口

1. 在 `handler/` 目录下添加新的处理器
2. 在 `main.go` 中注册路由
3. 更新 API 文档

### 水平扩展

- **调度中心**：多实例部署，通过 Redis 选举主节点
- **执行器**：动态注册，自动负载均衡
- **数据库**：rqlite 集群部署
- **Redis**：主从或集群模式
