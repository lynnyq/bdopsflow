# BDopsFlow 使用指南

本文档是 BDopsFlow 分布式工作流调度平台的综合使用指南，涵盖项目概述、架构设计、快速开始、功能详解、API参考、错误码参考、数据库设计、部署指南、开发指南和安全建议。

## 目录

- [1. 项目概述](#1-项目概述)
- [2. 架构设计](#2-架构设计)
- [3. 快速开始](#3-快速开始)
- [4. 功能详解](#4-功能详解)
- [5. API参考](#5-api参考)
- [6. 错误码参考](#6-错误码参考)
- [7. 数据库设计](#7-数据库设计)
- [8. 部署指南](#8-部署指南)
- [9. 开发指南](#9-开发指南)
- [10. 安全建议](#10-安全建议)

***

## 1. 项目概述

BDopsFlow 是一个分布式工作流调度平台，提供任务调度、工作流编排、数据源查询和权限管理能力。

### 核心能力

| 能力         | 说明                                                                      |
| ---------- | ----------------------------------------------------------------------- |
| 任务调度       | 支持 Cron 定时调度和手动触发，支持 HTTP 和 Shell 两种任务类型                                |
| 工作流编排      | DAG 有向无环图编排，拓扑排序执行，支持任务依赖和血缘追踪                                          |
| 数据源查询      | 支持 MySQL、SQLite、Hive、Kyuubi、Spark、Trino、StarRocks、Doris、Rqlite 共 9 种数据源 |
| 权限管理       | RBAC 角色权限控制，领域隔离实现多租户                                                   |
| 审计日志       | 全量审计所有写操作，中间件+Handler 协作模式                                              |
| 高可用        | 多调度中心实例，Redis 主节点选举，rqlite 分布式数据库                                       |
| Webhook 回调 | 任务事件通知，支持 HMAC 签名验证                                                     |
| SSO 登录     | 支持第三方 SSO 统一认证，RSA 公钥加密传输                                               |

### 技术栈

| 组件      | 技术                              |
| ------- | ------------------------------- |
| 后端语言    | Go 1.24+                        |
| 前端框架    | Vue3 + Element Plus             |
| 数据库     | rqlite（分布式 SQLite，Raft 共识）      |
| 缓存/锁/选举 | Redis 7.0+                      |
| 通信协议    | gRPC（调度器-执行器）、HTTP REST（前端-API） |
| 密码加密    | AES-256-GCM（数据源密码）、bcrypt（用户密码） |
| 认证      | JWT Token                       |

***

## 2. 架构设计

### 2.1 系统架构

BDopsFlow 采用分布式架构，由调度中心（Scheduler）和执行器（Executor）两个核心组件组成。

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
```

### 2.2 架构特点

1. **完全解耦**：调度中心和执行器独立部署，通过 gRPC 通信
2. **分布式存储**：使用 rqlite 分布式数据库，支持集群部署
3. **高可用**：支持多调度中心实例，通过 Redis 选举主节点
4. **弹性扩展**：执行器可动态注册和下线，自动负载均衡
5. **RBAC 权限**：完整的角色权限控制和多租户隔离
6. **全量审计**：中间件+Handler 协作模式，自动记录所有写操作
7. **数据源查询**：支持 9 种数据源类型，统一 SQL 查询接口

### 2.3 通信机制

#### HTTP API

前端与调度中心通过 HTTP REST API 通信，认证方式为 JWT Token：

```
请求头: Authorization: Bearer <token>
```

主要接口分组：

| 路径前缀                 | 说明        |
| -------------------- | --------- |
| `/api/auth/*`        | 认证相关      |
| `/api/tasks/*`       | 任务管理      |
| `/api/workflows/*`   | 工作流管理     |
| `/api/executors/*`   | 执行器管理     |
| `/api/logs/*`        | 日志查询      |
| `/api/admin/*`       | 管理员接口     |
| `/api/permissions/*` | 权限相关      |
| `/api/datasources/*` | 数据源管理     |
| `/api/query/*`       | SQL 查询与导出 |

#### gRPC 通信

调度中心与执行器通过 gRPC 双向流通信：

```protobuf
service ExecutorService {
  rpc Register(RegisterRequest) returns (RegisterResponse);
  rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
  rpc SubscribeTask(SubscribeTaskRequest) returns (stream Task);
  rpc ReportTaskResult(ReportTaskResultRequest) returns (ReportTaskResultResponse);
  rpc ReportTaskLog(ReportTaskLogRequest) returns (ReportTaskLogResponse);
  rpc ReportTaskProgress(ReportTaskProgressRequest) returns (ReportTaskProgressResponse);
  rpc SyncRunningTasks(SyncRunningTasksRequest) returns (SyncRunningTasksResponse);
}
```

### 2.4 中间件链

请求通过以下中间件链依次处理：

```
请求 → CORS → JWT认证 → 审计中间件 → RBAC中间件 → Handler
```

**中间件类型**：

| 中间件 | 说明 |
|--------|------|
| **JWTAuthMiddleware** | JWT认证，解析Token提取user\_id/username/real\_name/role/domain\_id到gin.Context |
| **RBACMiddleware(roles...)** | 角色检查，允许指定角色列表，用户角色需在列表中方可通过 |
| **RequireSystemAdmin** | 要求system\_admin角色，等价于RBACMiddleware("system\_admin") |
| **RequireAdminOrDomainAdmin** | 要求admin或domain\_admin角色，等价于RBACMiddleware("admin", "domain\_admin") |
| **DatasourcePermissionMiddleware(dsService, permission)** | 数据源权限检查，支持read/update/delete/query/download/manage六种权限 |
| **AuditMiddleware(auditService)** | 审计日志记录，自动记录所有POST/PUT/DELETE操作，Handler通过c.Set()补充业务信息 |

### 2.5 JWT Claims 结构

```json
{
  "user_id": 1,
  "username": "admin",
  "real_name": "管理员",
  "role": "system_admin",
  "domain_id": 1,
  "iss": "bdopsflow",
  "exp": 1716422400,
  "iat": 1716336000
}
```

### 2.6 目录结构

**调度中心**：

```
scheduler/
├── cmd/main.go                 # 启动入口
├── internal/
│   ├── config/                 # 配置管理
│   ├── model/                  # 数据模型
│   ├── service/                # 业务逻辑
│   ├── handler/                # HTTP 处理器
│   ├── datasource/             # 数据源管理
│   │   └── driver/             # 数据源驱动（9种）
│   ├── grpcserver/             # gRPC 服务端
│   ├── middleware/             # 中间件（JWT/RBAC/审计）
│   ├── cron/                   # Cron 调度
│   ├── lineage/                # 血缘关系
│   └── webhook/                # Webhook 服务
└── pkg/
    ├── election/               # 主节点选举
    └── lock/                   # 分布式锁
```

**执行器**：

```
executor/
├── cmd/main.go                 # 启动入口
└── internal/
    ├── config/                 # 配置管理
    ├── executor/               # 任务执行器
    ├── pool/                   # 协程池
    ├── grpcclient/             # gRPC 客户端
    └── logger/                 # 日志管理
```

***

## 3. 快速开始

### 3.1 环境要求

| 组件      | 版本要求  | 说明            |
| ------- | ----- | ------------- |
| Go      | 1.24+ | 后端语言          |
| Node.js | 18+   | 前端构建          |
| Redis   | 7.0+  | 分布式锁、缓存、主节点选举 |
| rqlite  | 8.0+  | 分布式数据库        |
| Docker  | 20.0+ | 容器化部署（可选）     |

### 3.2 启动依赖服务

#### 方式 A：使用 Docker（推荐）

```bash
cd deploy
docker-compose up -d redis rqlite1
sleep 5
curl -XPOST 'http://localhost:4001/db/load?pretty' \
    --data-binary @deploy/schema.sql
```

#### 方式 B：手动启动

```bash
docker run -d --name bdopsflow-redis -p 6379:6379 \
    redis:7-alpine redis-server --appendonly yes

docker run -d --name bdopsflow-rqlite -p 4001:4001 -p 4002:4002 \
    rqlite/rqlite:latest

sleep 5
curl -XPOST 'http://localhost:4001/db/load?pretty' \
    --data-binary @deploy/schema.sql
```

### 3.3 启动调度中心

```bash
cd scheduler
cp config.yaml.example config.yaml
go run ./cmd/main.go
```

验证启动：

```bash
curl http://localhost:8080/health
```

预期响应：

```json
{
    "status": "ok",
    "node_id": "...",
    "is_leader": true,
    "checks": {
        "redis": "ok",
        "database": "ok",
        "tables": "ok",
        "scheduler": "ok"
    }
}
```

### 3.4 启动执行器

```bash
cd executor
cp config.yaml.example config.yaml
go run ./cmd/main.go
```

### 3.5 启动前端

```bash
cd web
npm install
npm run dev
```

访问 <http://localhost:5173，使用默认账号登录：>

- 用户名：`admin`
- 密码：`admin123`

### 3.6 Docker Compose 一键启动

```bash
cd deploy
docker-compose up -d
sleep 15
```

访问地址：

| 服务        | 地址                      |
| --------- | ----------------------- |
| 前端        | <http://localhost:3000> |
| 调度中心 API  | <http://localhost:8080> |
| 调度中心 gRPC | localhost:50051         |

***

## 4. 功能详解

### 4.1 主节点选举

基于 Redis `SetNX` 原语实现的主节点选举机制，确保只有一个调度器节点执行调度任务。

**选举流程**：

1. 调用 `SetNX` 尝试设置 leader key（`scheduler:leader`，TTL 30秒）
2. 成功成为主节点，触发 `OnAcquire` 回调，启动 Cron 调度器
3. 定期 `Expire` 刷新 TTL 维持领导地位
4. 释放领导权时调用 `Del` 删除 key，触发 `OnRelease`

**使用示例**：

```go
election := election.NewLeaderElection(
    redisClient,
    "bdopsflow:leader",
    nodeID,
    15*time.Second,
)

election.OnAcquire(func() {
    cronScheduler.Start()
})

election.OnRelease(func() {
    fmt.Println("失去主节点地位")
})

election.Start(ctx)
```

### 4.2 分布式任务锁

基于 Redis 的分布式锁，防止任务重复执行。

| 特性    | 说明                                   |
| ----- | ------------------------------------ |
| 锁 Key | `task:lock:{task_id}:{execution_id}` |
| 锁 TTL | 60 秒（自动续期）                           |
| 续期间隔  | 每 10 秒（心跳时续期）                        |
| 续期条件  | 执行器心跳携带运行中的任务 ID                     |

**锁续期机制**：

```
Executor → Heartbeat(running_execution_ids) → Scheduler → Renew Lock TTL
```

**卡死任务检测**：调度中心每 60 秒检测 running 状态的任务，连续 3 次未续期则标记为 failed。

**RedisLock API**：

```
TryLock(ctx) - 非阻塞获取锁
Lock(ctx) - 阻塞获取锁（带重试）
Unlock(ctx) - 释放锁
Renew(ctx) - 续期锁TTL
```

- 锁 Key 格式：`bdopsflow:lock:{name}`
- TTL：60秒，续期间隔10秒

### 4.3 DAG 工作流编排

支持复杂工作流编排的有向无环图（DAG）系统。

**DAG 配置结构**：

```json
{
    "nodes": [
        {
            "id": "task1",
            "name": "数据抽取",
            "type": "http",
            "config": {},
            "position": {"x": 100, "y": 200},
            "timeout_seconds": 300,
            "retry_count": 3
        }
    ],
    "connections": [
        {"from": "task1", "to": "task2"}
    ]
}
```

**DAG 验证流程**：

1. 检查节点ID是否重复
2. 检查连接中的节点是否存在
3. 检查重复连接
4. 检查自环（from === to）
5. Kahn算法拓扑排序检测循环依赖

**TopologicalSort 算法（Kahn's algorithm）**：

1. 计算每个节点的入度
2. 入度为0的节点入队
3. 依次出队，减少后继节点入度
4. 若出队数 < 节点数，则存在环

**工作流执行**：

1. 解析 DAG 配置
2. 验证 DAG 结构
3. 计算拓扑排序
4. 按顺序执行节点，确保所有前置节点完成
5. 记录每个节点的执行状态

### 4.4 任务调度系统

基于 Robfig Cron 的定时任务调度系统。

**Cron 表达式支持**：

5 字段格式：

```
┌───────────── 分钟 (0-59)
│ ┌───────────── 小时 (0-23)
│ │ ┌───────────── 日期 (1-31)
│ │ │ ┌───────────── 月份 (1-12)
│ │ │ │ ┌───────────── 星期 (0-6, 0=周日)
│ │ │ │ │
* * * * *
```

6 字段格式（秒级）：

```
┌───────────── 秒 (0-59)
│ ┌───────────── 分钟 (0-59)
│ │ ┌───────────── 小时 (0-23)
│ │ │ ┌───────────── 日期 (1-31)
│ │ │ │ ┌───────────── 月份 (1-12)
│ │ │ │ │ ┌───────────── 星期 (0-6, 0=周日)
│ │ │ │ │ │
* * * * * *
```

**调度流程**：

1. 成为主节点后触发 `OnBecomeLeader`
2. 从数据库加载所有启用的任务
3. 使用 `cron.AddFunc` 注册
4. Cron 触发时获取分布式锁防止重复执行
5. 调用 Trigger 触发任务执行

**执行器选择策略**：

1. 过滤在线执行器（心跳在 30 秒内）
2. 过滤有可用容量的执行器
3. 优先选择任务所属领域的执行器
4. 选择当前负载最低的执行器

如果任务配置了 `assigned_executor_id`，则只分发到该执行器。

### 4.5 Webhook 回调系统

灵活的任务事件回调通知系统。

**支持的事件类型**：

| 事件               | 说明     |
| ---------------- | ------ |
| `task_started`   | 任务开始执行 |
| `task_completed` | 任务执行成功 |
| `task_failed`    | 任务执行失败 |
| `*`              | 所有事件   |

**配置示例**：

```json
{
    "url": "https://api.example.com/webhook",
    "method": "POST",
    "headers": {
        "X-API-Key": "secret-key",
        "Content-Type": "application/json"
    },
    "events": ["task_completed", "task_failed"]
}
```

**发送流程**：

1. 检查事件是否匹配配置
2. 序列化 payload 为 JSON
3. 发送 HTTP 请求
4. 验证响应状态码
5. 失败时按指数退避重试

**HMAC-SHA256 签名验证**：

- 请求头: `X-BDopsFlow-Signature: sha256=<hex_signature>`
- 签名内容: `timestamp.payload_json`
- 验证方式: 接收方使用相同secret计算HMAC-SHA256，比对签名

**推送 payload 格式**：

```json
{
  "event": "task_completed",
  "timestamp": "2026-05-23T10:00:00Z",
  "task": {
    "id": 1,
    "name": "数据同步",
    "type": "http",
    "domain_id": 1
  },
  "execution": {
    "execution_id": "exec-xxx",
    "status": "completed",
    "start_time": "2026-05-23T10:00:00Z",
    "end_time": "2026-05-23T10:00:05Z",
    "output": "...",
    "error": ""
  }
}
```

**重试策略**：失败后指数退避重试，最多3次，间隔1s/2s/4s

### 4.6 血缘关系追踪

追踪任务之间的依赖关系和影响范围。

| 功能     | 说明             |
| ------ | -------------- |
| 获取上游血缘 | 查询任务的所有上游依赖    |
| 获取影响范围 | 查询所有依赖该任务的下游任务 |
| 工作流血缘图 | 获取工作流的完整血缘关系图  |

### 4.7 指标收集系统

收集和管理系统运行时指标，支持 Redis 持久化。

| 指标名                               | 类型        | 描述       |
| --------------------------------- | --------- | -------- |
| `bdopsflow:tasks:created`         | Counter   | 任务创建次数   |
| `bdopsflow:tasks:completed`       | Counter   | 任务成功完成次数 |
| `bdopsflow:tasks:failed`          | Counter   | 任务失败次数   |
| `bdopsflow:tasks:running`         | Gauge     | 当前运行任务数  |
| `bdopsflow:executors:online`      | Gauge     | 在线执行器数   |
| `bdopsflow:executors:offline`     | Gauge     | 离线执行器数   |
| `bdopsflow:task:duration_seconds` | Histogram | 任务执行耗时分布 |
| `bdopsflow:workflow:created`      | Counter   | 工作流创建次数  |
| `bdopsflow:workflow:running`      | Gauge     | 当前运行工作流数 |

### 4.8 RBAC 权限管理

基于角色的访问控制系统，支持细粒度权限管理和领域隔离。

**核心概念**：

| 概念             | 说明          |
| -------------- | ----------- |
| 用户（User）       | 系统使用者       |
| 角色（Role）       | 权限集合        |
| 权限（Permission） | 资源+操作的组合    |
| 领域（Domain）     | 资源隔离单位（多租户） |

**系统预置角色**：

| 角色                   | 说明             | 范围   |
| -------------------- | -------------- | ---- |
| 系统管理员（system\_admin） | 系统最高权限，可管理所有资源 | 全局   |
| 领域管理员（domain\_admin） | 领域级管理权限        | 指定领域 |
| 普通用户（user）           | 基础查看和操作权限      | 指定领域 |

**权限检查流程**：

```
1. 解析 JWT Token，获取用户信息
2. 检查用户是否活跃
3. 获取用户在请求资源领域的角色
4. 检查用户是否有请求操作的权限
5. 特殊规则：系统管理员拥有所有权限
```

**资源与操作**：

| 资源         | 支持的操作                                                    |
| ---------- | -------------------------------------------------------- |
| user       | create, read, update, delete, manage                     |
| role       | create, read, update, delete, manage                     |
| permission | read                                                     |
| domain     | create, read, update, delete, manage                     |
| executor   | read, assign, manage                                     |
| task       | create, read, update, delete, trigger, manage            |
| log        | read, delete, manage                                     |
| workflow   | create, read, update, delete, manage                     |
| datasource | create, read, update, delete, manage, query, download    |
| webhook    | create, read, update, delete, manage                     |
| audit\_log | read, delete, manage                                     |
| menu       | dashboard, task, log, executor, datasource, sql\_query 等 |

### 4.9 领域隔离

实现多租户资源隔离的机制。

- 所有任务、工作流都绑定到领域
- 执行器可分配到一个或多个领域
- 执行器可设置为全局（所有领域可用）
- 用户在不同领域可拥有不同角色

**执行器分配策略**：

1. 优先使用分配给任务领域的执行器
2. 使用全局可用的执行器
3. 选择当前负载最低的执行器

### 4.10 数据源查询系统

支持 9 种数据源类型的统一 SQL 查询接口。

| 类型        | 说明                  | 连接方式      |
| --------- | ------------------- | --------- |
| MySQL     | MySQL 数据库           | 直连        |
| SQLite    | SQLite 数据库          | 文件路径      |
| Hive      | Hive 数据仓库           | Thrift 协议 |
| Kyuubi    | Kyuubi SQL 引擎       | Thrift 协议 |
| Spark     | Spark Thrift Server | Thrift 协议 |
| Trino     | Trino 查询引擎          | HTTP REST |
| StarRocks | StarRocks 数据库       | MySQL 协议  |
| Doris     | Apache Doris        | MySQL 协议  |
| Rqlite    | Rqlite 分布式数据库       | HTTP REST |

**核心特性**：

- Driver 接口模式：统一接口，各数据源独立实现
- 连接池管理：按 datasource\_id 缓存连接，自动生命周期管理
- UseDatabase 模式：查询前切换数据库，查询后恢复，避免并发污染
- SQL 标准化：自动去除末尾分号，兼容 Hive/Spark/Kyuubi Thrift 协议
- 密码加密存储：AES-256-GCM 加密存储数据源密码
- 并发查询控制：单用户和全局并发限制
- 查询结果缓存：Redis 缓存查询结果
- CSV 导出：支持查询结果导出

**查询流程**：

1. 获取数据源连接
2. 保存当前数据库
3. UseDatabase(targetDB) 切换数据库
4. normalizeSQL() 去除末尾分号
5. 执行 SQL 查询
6. UseDatabase(originalDB) 恢复数据库
7. 返回查询结果

**数据源权限**：

| 权限类型     | 说明    |
| -------- | ----- |
| read     | 查看数据源 |
| query    | 执行查询  |
| download | 导出数据  |
| update   | 修改数据源 |
| delete   | 删除数据源 |

**Driver 接口设计**：

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

**连接池管理**：

- 按datasource_id缓存连接实例（sync.Map）
- 连接参数：max_idle=5, max_open=10, max_lifetime=1800s
- 健康检查：每300秒检测连接可用性
- 自动关闭：数据源删除时关闭对应连接

**UseDatabase 模式**：

1. 保存当前数据库 `originalDB := currentDB`
2. `UseDatabase(targetDB)` 切换到目标数据库
3. 执行SQL查询
4. `UseDatabase(originalDB)` 恢复原数据库
5. 避免并发查询时数据库上下文污染

**SQL 标准化**：

- 自动去除末尾分号（兼容Hive/Spark/Kyuubi Thrift协议）
- `normalizeSQL(sql)` 函数处理

**密码加密存储**：

- 算法：AES-256-GCM
- 密钥来源：配置文件/环境变量/文件（key_source: direct/env/file）
- 密钥轮换：auto_rotate_days配置自动轮换周期
- 加密流程：plaintext → AES-256-GCM(key, nonce) → base64(nonce+ciphertext+tag)

**并发控制**：

- 单用户限制：max_concurrent_per_user（默认5）
- 全局限制：max_concurrent_global（默认50）
- 实现：Redis INCR/DECR + Lua脚本保证原子性
- 超限返回错误码 17002

**查询缓存**：

- Redis Key: `bdopsflow:query:cache:{datasource_id}:{database}:{sql_hash}`
- TTL: cache_ttl（默认300秒）
- 最大内存: cache_max_size（默认100MB）
- 失效策略：数据源更新时清除相关缓存

**安全控制**：

- SQL注入预防：isSelectOnly检查（仅允许SELECT/SHOW/DESCRIBE/EXPLAIN开头）
- allow_write_sql配置允许写操作
- 查询超时：query_timeout（默认60秒）
- 最大行数：default_limit（默认1000行）
- SQL长度限制：max_sql_length（默认65536字节）
- 单元格大小限制：max_cell_size（默认65536字节）

**内置 Web UI**：

- 调度中心内置 Web UI，可通过系统配置 `web.enabled` 控制是否启用
- 默认值为 `false`，启用后访问根路径 `/` 即可使用内置 Web UI
- 通过 API `PUT /api/admin/system-config/web.enabled` 可动态切换

**查询取消**：

- POST /api/query/cancel/:query_id
- 通过context cancel取消正在执行的查询

**CSV导出**：

- POST /api/query/export
- 最大导出行数：max_export_rows（默认1000行）
- 响应Content-Type: text/csv

**元数据API**：

- GET /api/datasources/:id/metadata
- 返回：databases列表、tables列表、columns列表
- 支持按数据库过滤

### 4.11 审计日志系统

全量审计所有写操作（POST/PUT/DELETE），采用中间件+Handler 协作模式。

**工作流程**：

```
请求 → JWT认证中间件 → 审计中间件 → RBAC中间件 → Handler
                                    │                │
                                    │   c.Next()     │ c.Set("audit_*")
                                    │◄───────────────│
                                    ▼
                          resolveAuditInfo() + Handler覆盖
                                    │
                                    ▼
                          异步写入审计日志（goroutine）
```

**审计操作类型**：

| 操作               | 说明      | 触发路径                                           |
| ---------------- | ------- | ---------------------------------------------- |
| create           | 创建资源    | POST /api/tasks, POST /api/datasources 等       |
| update           | 更新资源    | PUT /api/tasks/:id, PUT /api/datasources/:id 等 |
| delete           | 删除资源    | DELETE /api/tasks/:id 等                        |
| login            | 用户登录    | POST /api/auth/login                           |
| register         | 用户注册    | POST /api/auth/register                        |
| trigger          | 触发任务    | POST /api/tasks/:id/trigger                    |
| assign           | 分配角色/权限 | POST /api/admin/users/:id/roles                |
| reset\_password  | 重置密码    | POST /api/admin/users/:id/reset-password       |
| online           | 执行器上线   | POST /api/executors/:name/online               |
| offline          | 执行器下线   | POST /api/executors/:name/offline              |
| test\_connection | 测试连接    | POST /api/datasources/test                     |
| config\_change   | 配置变更    | PUT /api/admin/system-config/:key              |
| execute          | 执行查询    | POST /api/query/execute                        |
| export           | 导出数据    | POST /api/query/export                         |

**定时清理**：每 24 小时自动清理过期审计日志，默认保留 90 天。

**中间件+Handler协作模式详细流程**：

```
1. 审计中间件（c.Next()前）：
   - 解析JWT获取user_id, username, role, domain_id
   - 记录request_method, request_path, ip_address, user_agent
   
2. Handler处理业务逻辑：
   - c.Set("audit_action", "create")      // 覆盖默认action
   - c.Set("audit_resource_id", "123")     // 资源ID
   - c.Set("audit_resource_name", "任务A") // 资源名称
   - c.Set("audit_detail", "创建HTTP任务")  // 操作详情
   
3. 审计中间件（c.Next()后）：
   - resolveAuditInfo()：合并中间件和Handler设置的审计信息
   - Handler设置的值优先级高于中间件默认值
   - 启动goroutine异步写入审计日志
```

**路由匹配规则**：

- 精确匹配：routeAuditRules（如 `/api/tasks` → resource=task, action=create）
- 前缀匹配：routePrefixRules（如 `/api/tasks/` → resource=task）
- 动态参数：自动提取路径中的ID作为resource_id

### 4.12 SSO 登录

支持第三方 SSO 统一认证登录。

**核心流程**：

```
用户访问 /login
  │
  ├─ SSO 模式（默认）
  │   前端用 SSO 公钥加密密码
  │   → POST /api/auth/sso-login
  │   → 后端转发到 SSO 服务验证
  │   → 验证成功：查找/创建本地用户 → 生成 JWT → 登录成功
  │
  └─ 本地模式（isSso=false）
      前端用本地公钥加密密码
      → POST /api/auth/login
      → 后端本地 RSA 解密 + bcrypt 校验
      → 登录成功/失败
```

**SSO 用户处理**：

- SSO 验证成功后，查找本地数据库是否已有该用户
- 已有用户：更新 `last_login_at`，生成 JWT
- 新用户：自动创建（角色为 `user`，无本地密码），生成 JWT

**两套 RSA 密钥**：

| 配置                                   | 用途         | 来源                      |
| ------------------------------------ | ---------- | ----------------------- |
| `rsa.public_key` / `rsa.private_key` | 本地登录密码加解密  | `./scheduler keygen` 生成 |
| `sso.public_key`                     | SSO 登录密码加密 | SSO 服务方提供               |

**SSO响应处理**：

| SSO响应码 | 含义 | 处理方式 |
|----------|------|---------|
| 3000 | 认证成功 | 查找/创建本地用户，生成JWT |
| 3001 | 认证失败 | 返回SSO错误信息 |
| 网络错误 | SSO服务不可用 | 返回"SSO登录失败，请稍后再试" |

**SSO自动创建用户规则**：

| 字段 | 来源 |
|------|------|
| username | SSO loginName |
| real_name | SSO idCardName |
| phone | SSO mobileNo |
| email | SSO email |
| role | 固定为 user |
| password | 空（无本地密码） |

### 4.13 故障恢复机制

当新调度器成为主节点时的完整任务恢复系统。

**恢复流程**：

1. 标记为新 Leader
2. 恢复正在执行的任务（`RecoverRunningTasksOnBecomeLeader`）
3. 同步执行器状态（执行器下次心跳时发送详细任务状态）
4. 更新任务锁

**任务恢复逻辑**：

1. 查询所有 `status = 'running'` 的任务
2. 验证执行器状态（离线 → 标记任务失败）
3. 验证任务锁（锁不存在 → 标记任务失败）
4. 检查任务超时（超时 → 标记任务失败）
5. 刷新任务锁
6. 添加恢复日志（Redis 去重）

**卡死任务检测**：

- 调度中心每60秒扫描所有running状态的任务
- 检查任务锁是否存在（Redis GET task:lock:{task_id}:{execution_id}）
- 连续3次未续期（锁不存在）→ 标记任务为failed
- 添加恢复日志（Redis去重，避免重复写入）

**清理例程**：

| 清理任务 | 执行间隔 | 说明 |
|---------|---------|------|
| 卡死任务检测 | 60秒 | 检查running任务的锁是否存在 |
| 死亡任务清理 | 60秒 | 清理执行器离线且超时的任务 |
| 离线执行器清理 | 120秒 | 清理长时间无心跳的执行器 |
| 过期锁清理 | 300秒 | 清理残留的任务锁 |
| 审计日志清理 | 24小时 | 清理超过保留天数的审计日志 |

### 4.14 执行器管理

**注册流程**：执行器启动后通过 gRPC 向调度中心注册，提供名称、地址和容量。

**心跳机制**：

| 参数   | 值                                                      |
| ---- | ------------------------------------------------------ |
| 心跳间隔 | 10 秒                                                   |
| 离线检测 | 60 秒无心跳标记为 offline                                     |
| 心跳内容 | executor\_name, current\_load, running\_execution\_ids |

**容量管理**：支持动态调整执行器容量，调度中心通过心跳响应下发目标容量。

### 4.15 实时日志系统

**日志传输链路**：

```
Executor → gRPC ReportTaskLog → Scheduler → Redis Pub/Sub → SSE → Frontend
```

**gRPC 日志传输**：

- 执行器实时捕获任务stdout/stderr
- 通过gRPC流式传输到调度中心
- 调度中心写入数据库并发布到Redis channel

**SSE 实时推送**：

- GET /api/logs/stream
- Content-Type: text/event-stream
- 事件格式: `data: {"execution_id":"xxx","level":"info","message":"...","time":"..."}`
- 支持按execution_id过滤

**日志去重**：

- Redis Set: `bdopsflow:recovery:log:{execution_id}`
- 恢复日志写入前检查是否已存在
- 避免故障恢复时重复写入日志

**前端日志展示**：

- LogViewer组件：实时滚动显示
- 日志级别过滤（info/warn/error）
- 自动滚动到底部
- 支持暂停/恢复滚动

### 4.16 RSA 密码加密

**密钥生成**：

```bash
./scheduler keygen
# 输出: rsa_public_key 和 rsa_private_key（Base64编码PKCS#8格式，不含PEM头尾）
```

**密码传输流程**：

```
1. 前端获取公钥: GET /api/auth/public-key
2. 前端RSA公钥加密密码: rsaEncrypt(password, publicKey)
3. 提交加密密码: POST /api/auth/login {password: "encrypted..."}
4. 后端RSA私钥解密: rsaDecrypt(encryptedPassword, privateKey)
5. 后端bcrypt校验: bcrypt.CompareHashAndPassword(hash, decryptedPassword)
```

**配置密码加密CLI**：

```bash
# 加密配置文件中的密码
./scheduler encrypt-password --key <public_key> <plaintext>

# 解密配置文件中的密码
./scheduler decrypt-password --key <private_key> <ciphertext>
```

### 4.17 执行器任务执行

执行器支持 HTTP 和 Shell 两种任务类型，通过 goroutine 池控制并发执行。

**HTTP 任务**：

| 属性 | 说明 |
|------|------|
| 支持方法 | GET/POST/PUT/DELETE/PATCH |
| 请求头 | 自定义 HTTP headers（JSON 格式） |
| 请求体 | 支持 JSON/FORM |
| 超时 | 可配置（默认 60 秒） |
| 成功判断 | HTTP 状态码 2xx |
| 重定向 | 自动跟随 |

**Shell 任务**：

| 属性 | 说明 |
|------|------|
| 执行方式 | /bin/sh -c "script" |
| 超时 | 可配置（默认 60 秒） |
| 输出 | stdout/stderr 实时采集 |
| 退出码 | 0=成功，非0=失败 |

**任务池**：

- goroutine 池控制并发
- 容量由执行器配置的 capacity 决定
- 超出容量时任务排队等待

**gRPC 重连**：

- 初始连接失败时指数退避重试
- 运行中断连时自动重连
- 支持多调度器地址（SchedulerAddrs 优先于 SchedulerAddr）

***

## 5. API参考

### 5.1 通用说明

**基础 URL**：`http://localhost:8080`

**认证方式**：除登录和注册接口外，所有接口需携带 JWT Token：

```
Authorization: Bearer <token>
```

**通用响应格式**：

成功响应：

```json
{
  "code": 0,
  "status": "success",
  "message": "success",
  "data": {}
}
```

错误响应：

```json
{
  "code": 400,
  "status": "error",
  "message": "错误信息描述",
  "data": null
}
```

### 5.2 认证接口

| 方法   | 路径                          | 说明       | 权限  |
| ---- | --------------------------- | -------- | --- |
| POST | `/api/auth/login`           | 用户登录     | 公开  |
| POST | `/api/auth/sso-login`       | SSO 登录   | 公开  |
| POST | `/api/auth/register`        | 用户注册     | 配置控制 |
| GET  | `/api/auth/public-key`      | 获取公钥     | 公开  |
| GET  | `/api/auth/current`         | 获取当前用户   | 已登录 |
| PUT  | `/api/auth/profile`         | 更新当前用户信息 | 已登录 |
| POST | `/api/auth/change-password` | 修改当前用户密码 | 已登录 |

> 注册接口仅在配置 `app.allow_register: true` 时可用，生产环境默认关闭。

### 5.3 任务接口

| 方法     | 路径                                        | 说明       | 权限                          |
| ------ | ----------------------------------------- | -------- | --------------------------- |
| GET    | `/api/tasks`                              | 获取任务列表   | 已登录                         |
| POST   | `/api/tasks`                              | 创建任务     | system\_admin/domain\_admin |
| GET    | `/api/tasks/:id`                          | 获取任务详情   | 已登录                         |
| PUT    | `/api/tasks/:id`                          | 更新任务     | system\_admin/domain\_admin |
| DELETE | `/api/tasks/:id`                          | 删除任务     | system\_admin               |
| POST   | `/api/tasks/:id/trigger`                  | 手动触发任务   | system\_admin/domain\_admin |
| GET    | `/api/tasks/:id/executions`               | 获取任务执行历史 | 已登录                         |
| GET    | `/api/tasks/executions/:executionId/logs` | 获取执行日志   | 已登录                         |

**HTTP 任务配置**：

```json
{
  "url": "https://api.example.com/endpoint",
  "method": "GET",
  "headers": {"Authorization": "Bearer token"},
  "body": "{}",
  "timeout": 10000
}
```

**Shell 任务配置**：

```json
{
  "script": "echo 'Hello World'"
}
```

### 5.4 工作流接口

| 方法     | 路径                                            | 说明        | 权限                          |
| ------ | --------------------------------------------- | --------- | --------------------------- |
| GET    | `/api/workflows`                              | 获取工作流列表   | 已登录                         |
| POST   | `/api/workflows`                              | 创建工作流     | system\_admin/domain\_admin |
| GET    | `/api/workflows/:id`                          | 获取工作流详情   | 已登录                         |
| PUT    | `/api/workflows/:id`                          | 更新工作流     | system\_admin/domain\_admin |
| DELETE | `/api/workflows/:id`                          | 删除工作流     | system\_admin               |
| POST   | `/api/workflows/:id/trigger`                  | 触发工作流     | system\_admin/domain\_admin |
| GET    | `/api/workflows/:id/executions`               | 获取工作流执行历史 | 已登录                         |
| GET    | `/api/workflows/executions/:executionId`      | 获取工作流执行详情 | 已登录                         |
| GET    | `/api/workflows/executions/:executionId/logs` | 获取工作流执行日志 | 已登录                         |

### 5.5 执行器接口

| 方法     | 路径                                    | 说明         | 权限                                    |
| ------ | ------------------------------------- | ---------- | ------------------------------------- |
| GET    | `/api/executors`                      | 获取执行器列表    | 已登录                                   |
| GET    | `/api/executors/:name`                | 获取执行器详情    | 已登录                                   |
| POST   | `/api/executors/:name/online`         | 标记执行器在线    | system\_admin                         |
| POST   | `/api/executors/:name/offline`        | 标记执行器离线    | system\_admin                         |
| PUT    | `/api/executors/:name/capacity`       | 更新执行器容量    | system\_admin                         |
| DELETE | `/api/executors/:name`                | 删除执行器      | system\_admin                         |
| GET    | `/api/executors/:name/domains`        | 获取执行器领域分配  | system\_admin                         |
| POST   | `/api/executors/:name/domains`        | 分配执行器领域    | system\_admin                         |
| DELETE | `/api/executors/:name/domains/:domain_id` | 移除执行器领域    | system\_admin                         |
| GET    | `/api/executors/:name/tasks`          | 获取执行器已分配任务 | admin/system\_admin/domain\_admin     |
| GET    | `/api/executors/:name/can-delete`     | 检查执行器是否可删除 | 已登录                                   |

### 5.6 日志接口

| 方法     | 路径                       | 说明       | 权限  |
| ------ | ------------------------ | -------- | --- |
| GET    | `/api/logs`              | 获取执行日志列表 | 已登录 |
| GET    | `/api/logs/stats`        | 获取执行统计   | 已登录 |
| DELETE | `/api/logs/:id`          | 删除执行记录   | 已登录 |
| POST   | `/api/logs/batch-delete` | 批量删除执行记录 | 已登录 |
| GET    | `/api/logs/stream`       | 日志流（SSE） | 已登录 |

### 5.7 仪表盘接口

| 方法   | 路径                                | 说明      | 权限            |
| ---- | --------------------------------- | ------- | ------------- |
| GET  | `/api/dashboard/stats`            | 获取统计数据  | 已登录           |
| GET  | `/api/dashboard/trends`           | 获取趋势数据  | 已登录           |
| GET  | `/api/dashboard/health`           | 健康检查    | 已登录           |
| GET  | `/api/dashboard/scheduler/status` | 获取调度器状态 | 已登录           |
| POST | `/api/dashboard/scheduler/pause`  | 暂停调度器   | system\_admin |
| POST | `/api/dashboard/scheduler/resume` | 恢复调度器   | system\_admin |

### 5.8 管理员接口

#### 用户管理

| 方法     | 路径                                    | 说明       | 权限                          |
| ------ | ------------------------------------- | -------- | --------------------------- |
| GET    | `/api/admin/users`                    | 获取用户列表   | system\_admin               |
| GET    | `/api/admin/users/by-domain`          | 按领域获取用户列表 | system\_admin/domain\_admin |
| GET    | `/api/admin/users/:id`                | 获取用户详情   | system\_admin               |
| POST   | `/api/admin/users`                    | 创建用户     | system\_admin               |
| PUT    | `/api/admin/users/:id`                | 更新用户     | system\_admin/domain\_admin |
| DELETE | `/api/admin/users/:id`                | 删除用户     | system\_admin               |
| GET    | `/api/admin/users/:id/roles`          | 获取用户角色   | system\_admin               |
| POST   | `/api/admin/users/:id/roles`          | 分配用户角色   | system\_admin               |
| POST   | `/api/admin/users/:id/domains`        | 分配用户领域   | system\_admin               |
| POST   | `/api/admin/users/:id/reset-password` | 重置用户密码   | system\_admin/domain\_admin |

#### 角色管理

| 方法     | 路径                                  | 说明     | 权限            |
| ------ | ----------------------------------- | ------ | ------------- |
| GET    | `/api/admin/roles`                  | 获取角色列表 | system\_admin |
| GET    | `/api/admin/roles/:id`              | 获取角色详情 | system\_admin |
| POST   | `/api/admin/roles`                  | 创建角色   | system\_admin |
| PUT    | `/api/admin/roles/:id`              | 更新角色   | system\_admin |
| DELETE | `/api/admin/roles/:id`              | 删除角色   | system\_admin |
| GET    | `/api/admin/roles/:id/permissions`  | 获取角色权限 | system\_admin |
| POST   | `/api/admin/roles/:id/permissions`  | 分配角色权限 | system\_admin |

#### 领域管理

| 方法     | 路径                       | 说明     | 权限            |
| ------ | ------------------------ | ------ | ------------- |
| GET    | `/api/admin/domains`     | 获取领域列表 | system\_admin |
| POST   | `/api/admin/domains`     | 创建领域   | system\_admin |
| PUT    | `/api/admin/domains/:id` | 更新领域   | system\_admin |
| DELETE | `/api/admin/domains/:id` | 删除领域   | system\_admin |

#### 权限管理

| 方法  | 路径                       | 说明     | 权限            |
| --- | ------------------------ | ------ | ------------- |
| GET | `/api/admin/permissions` | 获取所有权限 | system\_admin |

### 5.9 数据源接口

| 方法     | 路径                                          | 说明         | 权限                          |
| ------ | ------------------------------------------- | ---------- | --------------------------- |
| GET    | `/api/datasources`                          | 获取数据源列表    | 已登录                         |
| GET    | `/api/datasources/types`                    | 获取支持的数据源类型 | 已登录                         |
| POST   | `/api/datasources`                          | 创建数据源      | system\_admin/domain\_admin |
| GET    | `/api/datasources/:id`                      | 获取数据源详情    | datasource read             |
| PUT    | `/api/datasources/:id`                      | 更新数据源      | datasource update           |
| DELETE | `/api/datasources/:id`                      | 删除数据源      | datasource delete           |
| POST   | `/api/datasources/test`                     | 测试连接(参数)   | system\_admin/domain\_admin |
| POST   | `/api/datasources/:id/test`                 | 测试连接(ID)   | datasource read             |
| POST   | `/api/datasources/:id/permissions`          | 授权数据源权限    | system\_admin/domain\_admin |
| PUT    | `/api/datasources/:id/permissions/:perm_id` | 更新数据源权限    | system\_admin/domain\_admin |
| DELETE | `/api/datasources/:id/permissions/:perm_id` | 撤销数据源权限    | system\_admin/domain\_admin |
| GET    | `/api/datasources/:id/permissions`          | 获取数据源权限列表  | datasource manage           |
| GET    | `/api/datasources/:id/metadata`             | 获取数据源元数据   | datasource query            |

### 5.10 查询接口

| 方法     | 路径                                | 说明         | 权限                  |
| ------ | --------------------------------- | ---------- | ------------------- |
| POST   | `/api/query/execute`              | 执行SQL查询    | datasource query    |
| POST   | `/api/query/cancel/:query_id`     | 取消查询       | datasource query    |
| POST   | `/api/query/export`               | 导出CSV      | datasource download |
| GET    | `/api/query/history`              | 获取查询历史     | 已登录                 |
| DELETE | `/api/query/history/:id`          | 删除查询历史     | 已登录                 |
| POST   | `/api/query/history/batch-delete` | 批量删除查询历史   | 已登录                 |
| GET    | `/api/query/saved-sql`            | 获取保存的SQL列表 | 已登录                 |
| POST   | `/api/query/saved-sql`            | 保存SQL      | 已登录                 |
| DELETE | `/api/query/saved-sql/:id`        | 删除保存的SQL   | 已登录                 |

### 5.11 Webhook接口

| 方法   | 路径                        | 说明           | 权限                                |
| ---- | ------------------------- | ------------ | --------------------------------- |
| GET  | `/api/webhooks`           | 获取 Webhook 列表 | 已登录                               |
| POST | `/api/webhooks`           | 创建 Webhook   | system\_admin/domain\_admin       |
| GET  | `/api/webhooks/:id`       | 获取 Webhook 详情 | 已登录                               |
| PUT  | `/api/webhooks/:id`       | 更新 Webhook   | system\_admin/domain\_admin       |
| DELETE | `/api/webhooks/:id`      | 删除 Webhook   | system\_admin                     |
| POST | `/api/webhooks/:id/test`  | 测试 Webhook   | admin/system\_admin/domain\_admin |

### 5.12 审计日志接口

| 方法   | 路径                                | 说明       | 权限            |
| ---- | --------------------------------- | -------- | ------------- |
| GET  | `/api/admin/audit-logs`           | 获取审计日志列表 | system\_admin |
| GET  | `/api/admin/audit-logs/stats`     | 获取审计日志统计 | system\_admin |
| POST | `/api/admin/audit-logs/clean`     | 清理过期审计日志 | system\_admin |
| GET  | `/api/admin/audit-logs/retention` | 获取保留天数   | system\_admin |
| PUT  | `/api/admin/audit-logs/retention` | 更新保留天数   | system\_admin |

### 5.13 系统配置接口

| 方法  | 路径                              | 说明       | 权限            |
| --- | ------------------------------- | -------- | ------------- |
| GET | `/api/admin/system-config`      | 获取系统配置列表 | system\_admin |
| PUT | `/api/admin/system-config/:key` | 更新系统配置   | system\_admin |

### 5.14 公共接口

| 方法  | 路径                  | 说明       | 权限 |
| --- | ------------------- | -------- | -- |
| GET | `/health`            | 健康检查     | 无  |
| GET | `/`                  | 内置 Web UI | 无  |
| GET | `/assets/*filepath`  | 静态资源     | 无  |

***

## 6. 错误码参考

### 6.1 通用错误码

| 错误码 | 常量名                 | 说明                 |
| --- | ------------------- | ------------------ |
| 0   | `CodeSuccess`       | 成功                 |
| 400 | `CodeBadRequest`    | 请求参数错误             |
| 401 | `CodeUnauthorized`  | 未授权（未登录或 Token 无效） |
| 403 | `CodeForbidden`     | 权限不足               |
| 404 | `CodeNotFound`      | 资源不存在              |
| 409 | `CodeConflict`      | 资源冲突               |
| 500 | `CodeInternalError` | 服务器内部错误            |

### 6.2 基础设施错误码

| 错误码  | 常量名                 | 说明       |
| ---- | ------------------- | -------- |
| 5001 | `CodeDatabaseError` | 数据库错误    |
| 5002 | `CodeRedisError`    | Redis 错误 |

### 6.3 任务相关错误码

| 错误码   | 常量名                       | 说明       |
| ----- | ------------------------- | -------- |
| 10001 | `CodeTaskRunning`         | 任务正在运行中  |
| 10002 | `CodeTaskLocked`          | 任务已被锁定   |
| 10003 | `CodeTaskNotFound`        | 任务不存在    |
| 10004 | `CodeExecutorNotFound`    | 执行器不存在   |
| 10005 | `CodeExecutorOffline`     | 执行器离线    |
| 10006 | `CodeExecutorNoCapacity`  | 执行器无可用容量 |
| 10007 | `CodeNoAvailableExecutor` | 无可用执行器   |
| 10008 | `CodeDispatchFailed`      | 任务分发失败   |

### 6.4 用户相关错误码

| 错误码   | 常量名                      | 说明             |
| ----- | ------------------------ | -------------- |
| 11001 | `CodeUserNotFound`       | 用户不存在          |
| 11002 | `CodeUserExists`         | 用户已存在          |
| 11003 | `CodeInvalidCredentials` | 凭证无效（用户名或密码错误） |
| 11004 | `CodeUserInactive`       | 用户已停用          |
| 11005 | `CodeWrongPassword`      | 密码错误           |
| 11006 | `CodePasswordWeak`       | 密码强度不足         |

### 6.5 角色相关错误码

| 错误码   | 常量名                       | 说明              |
| ----- | ------------------------- | --------------- |
| 12001 | `CodeRoleNotFound`        | 角色不存在           |
| 12002 | `CodeRoleExists`          | 角色已存在           |
| 12003 | `CodeRoleSystemProtected` | 系统角色受保护，不可删除/修改 |

### 6.6 领域相关错误码

| 错误码   | 常量名                      | 说明           |
| ----- | ------------------------ | ------------ |
| 13001 | `CodeDomainNotFound`     | 领域不存在        |
| 13002 | `CodeDomainHasResources` | 领域下仍有资源，无法删除 |

### 6.7 权限相关错误码

| 错误码   | 常量名                    | 说明    |
| ----- | ---------------------- | ----- |
| 14001 | `CodePermissionDenied` | 权限不足  |
| 14002 | `CodePermissionExists` | 权限已存在 |

### 6.8 工作流相关错误码

| 错误码   | 常量名                    | 说明     |
| ----- | ---------------------- | ------ |
| 15001 | `CodeWorkflowNotFound` | 工作流不存在 |

### 6.9 数据源相关错误码

| 错误码   | 常量名                           | 说明       |
| ----- | ----------------------------- | -------- |
| 16001 | `CodeDatasourceNotFound`      | 数据源不存在   |
| 16002 | `CodeDatasourceExists`        | 数据源已存在   |
| 16003 | `CodeDatasourceConnectFailed` | 数据源连接失败  |
| 16004 | `CodeDatasourceNameExists`    | 数据源名称已存在 |

### 6.10 查询相关错误码

| 错误码   | 常量名                        | 说明            |
| ----- | -------------------------- | ------------- |
| 17001 | `CodeQueryError`           | 查询执行错误        |
| 17002 | `CodeConcurrentLimit`      | 并发查询超限        |
| 17003 | `CodeQueryNoDatasource`    | 查询未指定数据源      |
| 17004 | `CodeQueryDisabled`        | 查询功能已禁用       |
| 17005 | `CodeQueryConnectFailed`   | 查询连接失败        |
| 17006 | `CodeQuerySelectOnly`      | 仅允许 SELECT 查询 |
| 17007 | `CodeQueryTimeout`         | 查询超时          |
| 17008 | `CodeQueryHistoryNotFound` | 查询历史不存在       |
| 17009 | `CodeSavedSQLNotFound`     | 保存的SQL不存在     |

***

## 7. 数据库设计

BDopsFlow 使用 rqlite 分布式数据库，所有表名使用 `bdopsflow_` 前缀。

### 7.1 表清单

| 序号 | 表名                                 | 说明          |
| -- | ---------------------------------- | ----------- |
| 1  | bdopsflow\_domains                 | 领域表         |
| 2  | bdopsflow\_users                   | 用户表         |
| 3  | bdopsflow\_workflows               | 工作流表        |
| 4  | bdopsflow\_tasks                   | 任务表         |
| 5  | bdopsflow\_task\_executions        | 任务执行记录表     |
| 6  | bdopsflow\_executors               | 执行器节点表      |
| 7  | bdopsflow\_workflow\_executions    | 工作流执行记录表    |
| 8  | bdopsflow\_task\_dependencies      | 任务依赖表（血缘关系） |
| 9  | bdopsflow\_task\_logs              | 任务执行日志表     |
| 10 | bdopsflow\_roles                   | 角色表         |
| 11 | bdopsflow\_permissions             | 权限表         |
| 12 | bdopsflow\_role\_permissions       | 角色权限映射表     |
| 13 | bdopsflow\_user\_roles             | 用户角色映射表     |
| 14 | bdopsflow\_domain\_executors       | 执行器领域分配表    |
| 15 | bdopsflow\_datasources             | 数据源表        |
| 16 | bdopsflow\_saved\_sql              | 保存的SQL表     |
| 17 | bdopsflow\_datasource\_permissions | 数据源权限表      |
| 18 | bdopsflow\_query\_history          | 查询历史表       |
| 19 | bdopsflow\_system\_config          | 系统配置表       |
| 20 | bdopsflow\_system\_config\_history | 配置变更历史表     |
| 21 | bdopsflow\_audit\_logs             | 审计日志表       |
| 22 | bdopsflow\_webhooks                | Webhook配置表  |

### 7.2 基础功能表

#### bdopsflow\_domains（领域表）

| 字段          | 类型          | 说明   |
| ----------- | ----------- | ---- |
| id          | INTEGER PK  | 自增主键 |
| name        | TEXT UNIQUE | 领域名称 |
| description | TEXT        | 描述   |
| created\_at | DATETIME    | 创建时间 |
| updated\_at | DATETIME    | 更新时间 |

#### bdopsflow\_users（用户表）

| 字段              | 类型          | 说明         |
| --------------- | ----------- | ---------- |
| id              | INTEGER PK  | 自增主键       |
| username        | TEXT UNIQUE | 用户名        |
| real\_name      | TEXT        | 真实姓名       |
| phone           | TEXT        | 手机号        |
| password        | TEXT        | 密码（bcrypt） |
| email           | TEXT        | 邮箱         |
| domain\_id      | INTEGER FK  | 所属领域       |
| role            | TEXT        | 角色         |
| is\_active      | BOOLEAN     | 是否激活       |
| last\_login\_at | DATETIME    | 最后登录时间     |
| created\_by     | INTEGER     | 创建人        |
| created\_at     | DATETIME    | 创建时间       |
| updated\_at     | DATETIME    | 更新时间       |

#### bdopsflow\_workflows（工作流表）

| 字段               | 类型         | 说明           |
| ---------------- | ---------- | ------------ |
| id               | INTEGER PK | 自增主键         |
| name             | TEXT       | 工作流名称        |
| description      | TEXT       | 描述           |
| domain\_id       | INTEGER FK | 所属领域         |
| dag\_config      | TEXT       | DAG 配置（JSON） |
| cron\_expression | TEXT       | Cron 表达式     |
| is\_enabled      | BOOLEAN    | 是否启用         |
| created\_by      | INTEGER    | 创建人          |
| created\_at      | DATETIME   | 创建时间         |
| updated\_at      | DATETIME   | 更新时间         |

#### bdopsflow\_tasks（任务表）

| 字段                     | 类型         | 说明                |
| ---------------------- | ---------- | ----------------- |
| id                     | INTEGER PK | 自增主键              |
| workflow\_id           | INTEGER FK | 所属工作流             |
| name                   | TEXT       | 任务名称              |
| type                   | TEXT       | 任务类型（http/shell）  |
| config                 | TEXT       | 任务配置（JSON）        |
| cron\_expression       | TEXT       | Cron 表达式          |
| timeout\_seconds       | INTEGER    | 超时时间，默认 300       |
| retry\_count           | INTEGER    | 重试次数，默认 3         |
| retry\_interval        | INTEGER    | 重试间隔（秒），默认 5      |
| is\_enabled            | BOOLEAN    | 是否启用              |
| status                 | TEXT       | 状态，默认 pending     |
| domain\_id             | INTEGER FK | 所属领域              |
| webhook\_config        | TEXT       | Webhook 配置        |
| webhook\_id            | INTEGER FK | Webhook ID        |
| webhook\_events        | TEXT       | Webhook 事件，默认 \[] |
| assigned\_executor\_id | INTEGER FK | 指定执行器             |
| created\_by            | INTEGER    | 创建人               |
| created\_at            | DATETIME   | 创建时间              |
| updated\_at            | DATETIME   | 更新时间              |

#### bdopsflow\_task\_executions（任务执行记录表）

| 字段            | 类型          | 说明        |
| ------------- | ----------- | --------- |
| id            | INTEGER PK  | 自增主键      |
| task\_id      | INTEGER FK  | 任务 ID     |
| execution\_id | TEXT UNIQUE | 执行 ID     |
| executor\_id  | INTEGER FK  | 执行器 ID    |
| status        | TEXT        | 执行状态      |
| start\_time   | DATETIME    | 开始时间      |
| end\_time     | DATETIME    | 结束时间      |
| output        | TEXT        | 输出        |
| error         | TEXT        | 错误信息      |
| retry\_times  | INTEGER     | 重试次数，默认 0 |
| progress      | INTEGER     | 进度，默认 0   |
| progress\_msg | TEXT        | 进度消息      |
| created\_at   | DATETIME    | 创建时间      |
| updated\_at   | DATETIME    | 更新时间      |

#### bdopsflow\_executors（执行器节点表）

| 字段              | 类型          | 说明           |
| --------------- | ----------- | ------------ |
| id              | INTEGER PK  | 自增主键         |
| name            | TEXT UNIQUE | 执行器名称        |
| address         | TEXT        | 地址           |
| status          | TEXT        | 状态，默认 online |
| last\_heartbeat | DATETIME    | 最后心跳时间       |
| capacity        | INTEGER     | 容量，默认 10     |
| current\_load   | INTEGER     | 当前负载，默认 0    |
| is\_global      | BOOLEAN     | 是否全局，默认 0    |
| created\_at     | DATETIME    | 创建时间         |
| updated\_at     | DATETIME    | 更新时间         |

#### bdopsflow\_workflow\_executions（工作流执行记录表）

| 字段            | 类型          | 说明         |
| ------------- | ----------- | ---------- |
| id            | INTEGER PK  | 自增主键       |
| workflow\_id  | INTEGER FK  | 工作流 ID     |
| execution\_id | TEXT UNIQUE | 执行 ID      |
| status        | TEXT        | 执行状态       |
| start\_time   | DATETIME    | 开始时间       |
| end\_time     | DATETIME    | 结束时间       |
| node\_states  | TEXT        | 节点状态（JSON） |
| created\_at   | DATETIME    | 创建时间       |

#### bdopsflow\_task\_dependencies（任务依赖表）

| 字段               | 类型         | 说明     |
| ---------------- | ---------- | ------ |
| id               | INTEGER PK | 自增主键   |
| task\_id         | INTEGER FK | 任务 ID  |
| parent\_task\_id | INTEGER FK | 父任务 ID |
| created\_at      | DATETIME   | 创建时间   |

约束：`UNIQUE(task_id, parent_task_id)`

#### bdopsflow\_task\_logs（任务执行日志表）

| 字段            | 类型         | 说明           |
| ------------- | ---------- | ------------ |
| id            | INTEGER PK | 自增主键         |
| execution\_id | TEXT       | 执行 ID        |
| task\_id      | INTEGER FK | 任务 ID        |
| executor\_id  | INTEGER FK | 执行器 ID       |
| node\_id      | TEXT       | 节点 ID        |
| log\_level    | TEXT       | 日志级别，默认 info |
| message       | TEXT       | 日志消息         |
| log\_time     | DATETIME   | 日志时间         |

### 7.3 权限管理表

#### bdopsflow\_roles（角色表）

| 字段          | 类型          | 说明          |
| ----------- | ----------- | ----------- |
| id          | INTEGER PK  | 自增主键        |
| name        | TEXT        | 角色名称        |
| code        | TEXT UNIQUE | 角色编码        |
| description | TEXT        | 描述          |
| is\_system  | BOOLEAN     | 是否系统角色，默认 0 |
| domain\_id  | INTEGER FK  | 所属领域        |
| created\_at | DATETIME    | 创建时间        |
| updated\_at | DATETIME    | 更新时间        |

#### bdopsflow\_permissions（权限表）

| 字段          | 类型         | 说明   |
| ----------- | ---------- | ---- |
| id          | INTEGER PK | 自增主键 |
| resource    | TEXT       | 资源   |
| action      | TEXT       | 操作   |
| description | TEXT       | 描述   |
| created\_at | DATETIME   | 创建时间 |

约束：`UNIQUE(resource, action)`

#### bdopsflow\_role\_permissions（角色权限映射表）

| 字段             | 类型         | 说明    |
| -------------- | ---------- | ----- |
| id             | INTEGER PK | 自增主键  |
| role\_id       | INTEGER FK | 角色 ID |
| permission\_id | INTEGER FK | 权限 ID |
| created\_at    | DATETIME   | 创建时间  |

约束：`UNIQUE(role_id, permission_id)`

#### bdopsflow\_user\_roles（用户角色映射表）

| 字段          | 类型         | 说明    |
| ----------- | ---------- | ----- |
| id          | INTEGER PK | 自增主键  |
| user\_id    | INTEGER FK | 用户 ID |
| role\_id    | INTEGER FK | 角色 ID |
| domain\_id  | INTEGER FK | 领域 ID |
| created\_at | DATETIME   | 创建时间  |

约束：`UNIQUE(user_id, role_id, domain_id)`

#### bdopsflow\_domain\_executors（执行器领域分配表）

| 字段           | 类型         | 说明     |
| ------------ | ---------- | ------ |
| id           | INTEGER PK | 自增主键   |
| domain\_id   | INTEGER FK | 领域 ID  |
| executor\_id | INTEGER FK | 执行器 ID |
| assigned\_by | INTEGER FK | 分配人    |
| created\_at  | DATETIME   | 创建时间   |

约束：`UNIQUE(domain_id, executor_id)`

### 7.4 数据查询表

#### bdopsflow\_datasources（数据源表）

| 字段                | 类型         | 说明               |
| ----------------- | ---------- | ---------------- |
| id                | INTEGER PK | 自增主键             |
| name              | TEXT       | 数据源名称            |
| type              | TEXT       | 数据源类型            |
| host              | TEXT       | 主机地址             |
| port              | INTEGER    | 端口               |
| path              | TEXT       | 连接路径             |
| database          | TEXT       | 数据库名             |
| username          | TEXT       | 用户名              |
| password          | TEXT       | 密码（AES加密）        |
| auth\_type        | TEXT       | 认证类型，默认 simple   |
| connection\_mode  | TEXT       | 连接模式，默认 single   |
| zk\_hosts         | TEXT       | ZooKeeper 地址     |
| zk\_path          | TEXT       | ZooKeeper 路径     |
| rqlite\_hosts     | TEXT       | rqlite 集群地址      |
| config            | TEXT       | 额外配置（JSON）       |
| description       | TEXT       | 描述               |
| domain\_id        | INTEGER FK | 所属领域             |
| is\_enabled       | BOOLEAN    | 是否启用，默认 1        |
| allow\_write\_sql | BOOLEAN    | 是否允许写SQL，默认 0    |
| test\_status      | TEXT       | 测试状态，默认 untested |
| last\_test\_at    | DATETIME   | 最后测试时间           |
| created\_by       | INTEGER    | 创建人              |
| updated\_by       | INTEGER    | 更新人              |
| created\_at       | DATETIME   | 创建时间             |
| updated\_at       | DATETIME   | 更新时间             |

约束：`UNIQUE(name, domain_id)`

#### bdopsflow\_saved\_sql（保存的SQL表）

| 字段             | 类型         | 说明        |
| -------------- | ---------- | --------- |
| id             | INTEGER PK | 自增主键      |
| name           | TEXT       | SQL 名称    |
| datasource\_id | INTEGER FK | 数据源 ID    |
| sql\_text      | TEXT       | SQL 内容    |
| description    | TEXT       | 描述        |
| created\_by    | INTEGER    | 创建人       |
| updated\_by    | INTEGER    | 更新人       |
| domain\_id     | INTEGER FK | 所属领域      |
| is\_public     | BOOLEAN    | 是否公开，默认 0 |
| created\_at    | DATETIME   | 创建时间      |
| updated\_at    | DATETIME   | 更新时间      |

#### bdopsflow\_datasource\_permissions（数据源权限表）

| 字段               | 类型         | 说明     |
| ---------------- | ---------- | ------ |
| id               | INTEGER PK | 自增主键   |
| datasource\_id   | INTEGER FK | 数据源 ID |
| role\_id         | INTEGER FK | 角色 ID  |
| user\_id         | INTEGER FK | 用户 ID  |
| permission\_type | TEXT       | 权限类型   |
| granted\_by      | INTEGER    | 授权人    |
| granted\_at      | TEXT       | 授权时间   |

约束：`CHECK(role_id IS NOT NULL OR user_id IS NOT NULL)`，`UNIQUE(datasource_id, role_id, permission_type)`，`UNIQUE(datasource_id, user_id, permission_type)`

#### bdopsflow\_query\_history（查询历史表）

| 字段               | 类型         | 说明      |
| ---------------- | ---------- | ------- |
| id               | INTEGER PK | 自增主键    |
| query\_id        | TEXT       | 查询 ID   |
| datasource\_id   | INTEGER FK | 数据源 ID  |
| datasource\_name | TEXT       | 数据源名称   |
| sql\_text        | TEXT       | SQL 内容  |
| database         | TEXT       | 数据库名    |
| execution\_time  | REAL       | 执行耗时（秒） |
| row\_count       | INTEGER    | 返回行数    |
| status           | TEXT       | 执行状态    |
| error\_message   | TEXT       | 错误信息    |
| executed\_by     | INTEGER    | 执行人     |
| domain\_id       | INTEGER FK | 所属领域    |
| created\_at      | DATETIME   | 创建时间    |

#### bdopsflow\_system\_config（系统配置表）

| 字段            | 类型          | 说明   |
| ------------- | ----------- | ---- |
| id            | INTEGER PK  | 自增主键 |
| config\_key   | TEXT UNIQUE | 配置键  |
| config\_value | TEXT        | 配置值  |
| description   | TEXT        | 描述   |
| updated\_at   | DATETIME    | 更新时间 |

#### bdopsflow\_system\_config\_history（配置变更历史表）

| 字段          | 类型         | 说明   |
| ----------- | ---------- | ---- |
| id          | INTEGER PK | 自增主键 |
| config\_key | TEXT       | 配置键  |
| old\_value  | TEXT       | 旧值   |
| new\_value  | TEXT       | 新值   |
| changed\_by | INTEGER FK | 变更人  |
| changed\_at | TEXT       | 变更时间 |

### 7.5 审计日志表

#### bdopsflow\_audit\_logs（审计日志表）

| 字段              | 类型         | 说明      |
| --------------- | ---------- | ------- |
| id              | INTEGER PK | 自增主键    |
| user\_id        | INTEGER    | 操作用户 ID |
| username        | TEXT       | 操作用户名   |
| real\_name      | TEXT       | 真实姓名    |
| role            | TEXT       | 用户角色    |
| domain\_id      | INTEGER    | 所属领域 ID |
| action          | TEXT       | 操作类型    |
| resource        | TEXT       | 资源类型    |
| resource\_id    | TEXT       | 资源 ID   |
| resource\_name  | TEXT       | 资源名称    |
| status          | TEXT       | 操作结果    |
| ip\_address     | TEXT       | 客户端 IP  |
| user\_agent     | TEXT       | 客户端 UA  |
| request\_method | TEXT       | HTTP 方法 |
| request\_path   | TEXT       | 请求路径    |
| detail          | TEXT       | 操作详情    |
| created\_at     | DATETIME   | 操作时间    |

### 7.6 Webhook表

#### bdopsflow\_webhooks（Webhook配置表）

| 字段          | 类型         | 说明              |
| ----------- | ---------- | --------------- |
| id          | INTEGER PK | 自增主键            |
| name        | TEXT       | Webhook 名称      |
| url         | TEXT       | 回调 URL          |
| method      | TEXT       | HTTP 方法，默认 POST |
| headers     | TEXT       | 请求头，默认 {}       |
| secret      | TEXT       | 签名密钥，默认空        |
| domain\_id  | INTEGER FK | 所属领域            |
| is\_enabled | BOOLEAN    | 是否启用，默认 1       |
| description | TEXT       | 描述              |
| created\_by | INTEGER    | 创建人             |
| created\_at | DATETIME   | 创建时间            |
| updated\_at | DATETIME   | 更新时间            |

约束：`UNIQUE(name, domain_id)`

### 7.7 初始化数据

**预设角色**：

| 角色代码          | 名称    | 说明             |
| ------------- | ----- | -------------- |
| system\_admin | 系统管理员 | 系统最高权限，可管理所有资源 |
| domain\_admin | 领域管理员 | 领域级管理权限        |
| user          | 普通用户  | 基础查看和操作权限      |

**默认管理员**：

- 用户名：`admin`
- 密码：`admin123`
- 角色：`system_admin`

**系统配置默认值**：

| 配置键                                   | 默认值   | 说明             |
| ------------------------------------- | ----- | -------------- |
| web.enabled                           | false | 是否启用内置 Web UI  |
| datasource.default\_limit             | 1000  | SQL 查询默认限制行数   |
| datasource.max\_export\_rows          | 1000  | CSV 导出最大行数     |
| datasource.cache\_ttl                 | 300   | 查询结果缓存 TTL（秒）  |
| datasource.cache\_max\_size           | 100   | 缓存最大内存占用（MB）   |
| datasource.query\_timeout             | 60    | 查询超时时间（秒）      |
| datasource.max\_concurrent\_per\_user | 5     | 单用户并发查询限制      |
| datasource.max\_concurrent\_global    | 50    | 全局并发查询限制       |
| datasource.allow\_write\_sql          | false | 是否允许写操作 SQL    |
| datasource.history\_retention\_days   | 30    | 查询历史保留天数       |
| datasource.connection\_max\_idle      | 5     | 连接池最大空闲连接数     |
| datasource.connection\_max\_open      | 10    | 连接池最大打开连接数     |
| datasource.connection\_max\_lifetime  | 1800  | 连接最大生命周期（秒）    |
| datasource.max\_sql\_length           | 65536 | SQL 文本最大长度（字节） |
| datasource.max\_cell\_size            | 65536 | 单个单元格值最大字节数    |
| datasource.health\_check\_interval    | 300   | 健康检查间隔（秒）      |
| datasource.test\_timeout              | 10    | 连接测试超时时间（秒）    |
| audit\_log.retention\_days            | 90    | 审计日志保留天数       |

***

## 8. 部署指南

### 8.1 单节点部署

参见 [3. 快速开始](#3-快速开始) 章节。

### 8.2 多节点集群部署

#### 架构概述

- 3 个调度中心节点：通过 Redis 选举主节点，实现故障自动转移
- 3 个 rqlite 节点：通过 Raft 协议保证数据一致性
- Redis Sentinel：Redis 高可用
- 多执行器节点：动态注册和负载均衡

#### 部署 Redis（哨兵模式）

1 主 2 从 + 3 哨兵，确保 Redis 高可用。

#### 部署 rqlite 集群

3 节点 rqlite 集群，使用 `-bootstrap-expect 3` 参数启动。

```bash
docker-compose -f docker-compose-rqlite.yml up -d
sleep 15
curl -XPOST 'http://localhost:4001/db/load?pretty' \
    --data-binary @schema.sql
```

#### 配置调度中心集群

每个节点使用不同的 `node_id`，配置 Sentinel 模式的 Redis 连接，以及多节点 rqlite 地址。

#### 配置执行器集群

执行器支持多调度器地址：

```yaml
scheduler:
  addrs: "scheduler1:50051,scheduler2:50051,scheduler3:50051"
```

#### 部署前端

使用 Nginx 托管前端静态文件，反向代理 API 请求：

```nginx
server {
    listen 80;
    server_name your-domain.com;
    root /opt/bdopsflow/web/dist;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    location /api {
        proxy_pass http://scheduler-cluster:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

#### 使用 systemd 管理服务

```ini
[Unit]
Description=BDopsFlow Scheduler
After=network.target

[Service]
Type=simple
User=bdopsflow
WorkingDirectory=/opt/bdopsflow/scheduler
ExecStart=/opt/bdopsflow/scheduler/bin/scheduler
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

#### 验证集群

```bash
curl http://node1:8080/health
curl http://node2:8080/health
curl http://node3:8080/health
```

应该只有一个节点返回 `"is_leader": true`。

### 8.3 HTTPS 配置

```nginx
server {
    listen 443 ssl http2;
    server_name your-domain.com;
    
    ssl_certificate /etc/nginx/ssl/cert.pem;
    ssl_certificate_key /etc/nginx/ssl/key.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    
    # Security headers
    add_header X-Frame-Options DENY;
    add_header X-Content-Type-Options nosniff;
    add_header X-XSS-Protection "1; mode=block";
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    
    root /opt/bdopsflow/web/dist;
    index index.html;
    
    location / {
        try_files $uri $uri/ /index.html;
    }
    
    location /api {
        proxy_pass http://scheduler-cluster:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
    
    # SSE support
    location /api/logs/stream {
        proxy_pass http://scheduler-cluster:8080;
        proxy_http_version 1.1;
        proxy_set_header Connection '';
        proxy_buffering off;
        proxy_cache off;
        chunked_transfer_encoding off;
    }
    
    # Gzip compression
    gzip on;
    gzip_types text/plain text/css application/json application/javascript text/xml;
    gzip_min_length 1000;
}
```

### 8.4 Docker 多阶段构建

```dockerfile
# Scheduler
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o scheduler ./scheduler/cmd

FROM alpine:3.19
RUN adduser -D -g '' bdopsflow
USER bdopsflow
COPY --from=builder /app/scheduler /app/scheduler
COPY --from=builder /app/scheduler/config.yaml.example /app/config.yaml.example
EXPOSE 8080 50051
CMD ["/app/scheduler"]
```

***

## 9. 开发指南

### 9.1 Makefile 快捷命令

#### 编译相关

```bash
make build              # 编译所有组件
make scheduler-build    # 编译调度中心
make executor-build     # 编译执行器
make web-build          # 编译前端
make proto              # 编译 Protobuf
```

#### 运行相关

```bash
make scheduler-run      # 运行调度中心
make executor-run       # 运行执行器
make web-dev            # 运行前端开发服务器
make docker-up          # 使用 Docker 运行完整环境
make docker-down        # 停止 Docker 环境
```

#### 测试相关

```bash
make test               # 运行所有后端测试
make scheduler-test     # 运行调度中心测试
make executor-test      # 运行执行器测试
make web-test           # 运行前端测试
make coverage           # 运行测试并显示覆盖率
```

#### 格式化和清理

```bash
make fmt                # 格式化代码
make clean              # 清理编译产物
```

### 9.2 添加审计日志埋点

1. 在 `routeAuditRules` 中添加精确匹配规则：

```go
var routeAuditRules = map[string]auditRouteRule{
    "/api/new-feature": {Resource: "new_feature", Action: "create"},
}
```

1. 在 `routePrefixRules` 中添加前缀匹配：

```go
var routePrefixRules = []struct {
    Prefix   string
    Resource string
}{
    {"/api/new-feature/", "new_feature"},
}
```

1. 在 Handler 中通过 `c.Set()` 传递业务语义：

```go
c.Set("audit_action", "assign")
c.Set("audit_resource_id", fmt.Sprintf("%d", id))
c.Set("audit_resource_name", name)
c.Set("audit_detail", detail)
```

1. 编写路由解析测试用例

### 9.3 添加新数据源类型

1. 在 `scheduler/internal/datasource/driver/` 中创建新驱动文件，实现 Driver 接口：

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

1. 在 `datasource/manager.go` 的 `CreateDriver` 方法中注册新类型
2. 在前端 `DatasourceForm.vue` 中添加新类型表单模板
3. 在审计中间件路由规则中添加数据源相关路径
4. 编写驱动单元测试

### 9.4 添加新权限资源

1. 在 `permission.go` 模型中定义新资源常量
2. 在数据库初始化 SQL 中添加相应权限记录
3. 在前端添加权限配置界面
4. 在 RBAC 中间件中添加解析逻辑
5. 在审计中间件路由规则中添加路由规则

### 9.5 测试指南

```bash
# 后端测试
cd scheduler && go test -v ./...
cd executor && go test -v ./...

# 测试覆盖率
cd scheduler
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# 审计中间件测试
cd scheduler && go test -v ./internal/middleware/ -run TestResolve

# 审计日志服务测试
cd scheduler && go test -v ./internal/service/ -run TestAudit
```

***

## 10. 安全建议

### 10.1 密钥管理

- **JWT 密钥**：生产环境必须修改默认值 `your-secret-key-change-in-production`，使用至少 32 字符的随机字符串
- **数据源加密密钥**：生产环境必须修改默认值 `change-in-prod-32byte-key1-here1`，使用 `key_source: "env"` 从环境变量读取
- **Redis 密码**：生产环境必须设置 Redis 密码
- **RSA 密钥对**：使用 `./scheduler keygen` 生成，不要使用默认密钥

### 10.2 网络安全

- 使用 HTTPS 部署前端和 API
- 配置 `cors_allow_origins` 限制跨域来源
- 使用防火墙限制 Redis 和 rqlite 的访问来源
- gRPC 端口仅对执行器开放

### 10.3 密码安全

- 用户密码使用 bcrypt 加密存储
- 数据源密码使用 AES-256-GCM 加密存储
- SSO 登录密码使用 RSA 公钥加密传输
- 建议开启 `auto_rotate_days` 定期轮换数据源加密密钥

### 10.4 权限最小化

- 遵循最小权限原则分配角色
- 系统管理员账号仅用于系统管理
- 普通用户仅分配必要的权限
- 数据源权限独立控制，避免过度授权

### 10.5 审计日志

- 确保审计日志功能正常开启
- 定期检查审计日志中的异常操作
- 设置合理的日志保留天数（建议 90 天以上）
- 审计日志仅系统管理员可查看

### 10.6 SSO 安全

- SSO 公钥和本地公钥完全独立，互不影响
- SSO 用户本地不存储密码
- SSO 请求设置超时时间，防止服务不可用时长时间阻塞
- SSO 自动创建的用户角色为 `user`，需管理员手动分配权限

### 10.7 数据源安全

- 默认仅允许 SELECT 查询，`allow_write_sql` 需谨慎开启
- 配置合理的查询超时时间和并发限制
- 数据源连接使用只读账号
- 定期检查数据源权限分配

### 10.8 Webhook 安全

- 使用 HTTPS 回调地址
- 配置 `secret` 字段启用 HMAC 签名验证
- 验证回调响应状态码
- 限制 Webhook 回调的目标地址范围

### 10.9 部署安全

- 不要以 root 用户运行服务
- 使用 systemd 管理服务，配置自动重启
- 定期更新依赖版本
- 容器镜像使用非 root 用户
- rqlite 启用 TLS 连接（生产环境）
- Redis 启用密码认证和 TLS

***

## 相关文档

- [配置指南](./CONFIGURATION.md) - 详细的配置说明和示例

