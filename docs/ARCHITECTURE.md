# BDopsFlow 分布式工作流调度平台 - 架构设计文档

## 1. 项目概述

BDopsFlow 是一套生产级、高可用、无单点、可集群、自动故障转移的分布式工作流调度平台。

### 核心特性

- ✅ 分布式架构（调度中心集群 + 执行器集群）
- ✅ 工作流DAG支持
- ✅ 监控可观测性
- ✅ Webhook推送
- ✅ Dashboard仪表盘
- ✅ 多租户一级领域划分与RBAC权限管理
- ✅ JWT认证
- ✅ 任务执行历史
- ✅ 执行器自动发现与心跳

## 2. 技术栈

### 后端

- **语言**: Go 1.24+
- **框架**: Gin（HTTP框架）
- **数据库**: rqlite（分布式 SQLite，开发和生产环境统一使用）
- **缓存**: Redis（分布式锁/缓存）
- **通信**: gRPC + Protobuf
- **密码加密**: bcrypt

### 前端

- **框架**: Vue3 + Vite + TypeScript
- **UI库**: Element Plus
- **状态管理**: Pinia
- **设计风格**: Neo-Brutalist（新野兽派）

### 架构

- 调度中心(集群) ←gRPC→ 执行器节点(集群)，完全解耦
- 调度中心 ↔ 执行器 100% 使用 gRPC 通信

## 3. 系统架构

### 3.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                         前端 (Vue3)                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │ Dashboard│  │ 任务管理  │  │ 执行器   │  │ 工作流    │        │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘        │
└───────┼─────────────┼─────────────┼─────────────┼──────────────┘
        │             │             │             │
        │ HTTP/REST   │             │             │
        ▼             ▼             ▼             ▼
┌─────────────────────────────────────────────────────────────────┐
│                    调度中心集群 (无状态)                          │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  Gin HTTP Server  │  gRPC Server  │  调度引擎              │  │
│  │  (API接口)         │  (执行器通信)  │  (主节点选举/任务扫描) │  │
│  └───────────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  Service Layer  │  Repository Layer  │  Model Layer       │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                          │              │
                          │ gRPC         │ gRPC
                          ▼              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    执行器集群 (无状态)                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │
│  │  执行器-1   │  │  执行器-2   │  │  执行器-N   │            │
│  │ (gRPC客户端)│  │ (gRPC客户端)│  │ (gRPC客户端)│            │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘            │
└─────────┼─────────────────┼─────────────────┼───────────────────┘
          │                 │                 │
          └─────────────────┴─────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
        ▼                   ▼                   ▼
┌──────────────┐   ┌──────────────┐   ┌──────────────┐
│   Redis      │   │   rqlite     │   │   rqlite     │
│ (分布式锁/    │   │  (节点1)     │   │  (节点2)     │
│  缓存/选举)   │   └──────────────┘   └──────────────┘
└──────────────┘            │                   │
                            │                   │
                      ┌─────┴─────┐             │
                      │           │             │
                      ▼           ▼             ▼
                ┌──────────────┐         ┌──────────────┐
                │   rqlite     │         │  Webhook     │
                │  (节点3)     │         │  推送服务     │
                └──────────────┘         └──────────────┘
```

### 3.2 核心模块说明

#### 调度中心模块

- **Gin HTTP Server**：提供 REST API 给前端调用
- **gRPC Server**：与执行器通信（注册、心跳、任务下发、结果回传）
- **调度引擎**：
  - 主节点选举（Redis 锁）
  - 任务扫描（Cron 定时）
  - 负载分发（选择空闲执行器）
  - 工作流 DAG 解析
- **Service Layer**：业务逻辑层
- **Repository Layer**：数据访问层
- **Model Layer**：数据模型层

#### 执行器模块

- **gRPC Client**：连接调度中心
- **注册/心跳**：自动注册，5s 心跳上报
- **协程池**：控制并发，任务排队
- **任务执行器**：
  - HTTP 执行器
  - Shell 执行器
- **日志回传**：实时回传执行日志

#### 存储层

- **Redis**：分布式锁、主节点选举、缓存
- **rqlite 集群**：3节点，分布式一致性数据库

## 4. 数据库设计

### 4.1 核心表结构

#### users（用户表）

```sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,
    email TEXT,
    domain_id INTEGER,
    role TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

#### domains（领域表）

```sql
CREATE TABLE domains (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

#### workflows（工作流表）

```sql
CREATE TABLE workflows (
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
    FOREIGN KEY (domain_id) REFERENCES domains(id)
);
```

#### tasks（任务表）

```sql
CREATE TABLE tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_id INTEGER,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    config TEXT NOT NULL,
    cron_expression TEXT,
    timeout_seconds INTEGER DEFAULT 0,
    retry_count INTEGER DEFAULT 3,
    retry_interval INTEGER DEFAULT 5,
    is_enabled BOOLEAN DEFAULT 1,
    status TEXT DEFAULT 'pending',
    domain_id INTEGER NOT NULL,
    webhook_config TEXT,
    created_by INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (workflow_id) REFERENCES workflows(id),
    FOREIGN KEY (domain_id) REFERENCES domains(id)
);
```

#### task\_executions（任务执行记录表）

```sql
CREATE TABLE task_executions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    execution_id TEXT NOT NULL UNIQUE,
    executor_id TEXT NOT NULL,
    status TEXT NOT NULL,
    start_time DATETIME,
    end_time DATETIME,
    output TEXT,
    error TEXT,
    retry_times INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES tasks(id)
);
```

#### executors（执行器节点表）

```sql
CREATE TABLE executors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    executor_id TEXT NOT NULL UNIQUE,
    name TEXT,
    address TEXT NOT NULL,
    status TEXT DEFAULT 'online',
    last_heartbeat DATETIME,
    capacity INTEGER DEFAULT 10,
    current_load INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## 5. Protobuf 协议设计

详见 [proto/executor.proto](../proto/executor.proto)

## 6. 核心设计原则

### 6.1 幂等性设计

基于 Redis 分布式锁 + 状态机的强幂等实现：

- 每个任务执行分配唯一 execution\_id
- 执行前获取 Redis 锁，防止重复执行
- 通过状态机（pending → running → success/failed）保证状态一致性

### 6.2 高可用性

- 调度中心：多实例无状态，Redis 锁选举主节点
- 执行器：自动注册、心跳保活、故障自动剔除
- 数据库：3节点 rqlite 集群，保证数据一致性

### 6.3 可靠性

- 任务不重复、不丢失
- 支持重试、超时控制
- 执行日志全量存储

### 6.4 安全性

- 密码使用 bcrypt 加密存储
- JWT Token 认证
- RBAC 权限控制（admin/operator/viewer）

## 7. 分阶段实施计划

详见 [PHASES.md](./PHASES.md)

## 8. API 接口说明

### 8.1 认证接口

| 接口                      | 方法     | 说明     |
| ----------------------- | ------ | ------ |
| POST /api/auth/login    | 登录     | <br /> |
| POST /api/auth/register | 注册     | <br /> |
| GET /api/auth/current   | 获取当前用户 | <br /> |

### 8.2 任务接口

| 接口                            | 方法     | 说明     |
| ----------------------------- | ------ | ------ |
| GET /api/tasks                | 任务列表   | <br /> |
| POST /api/tasks               | 创建任务   | <br /> |
| GET /api/tasks/:id            | 任务详情   | <br /> |
| PUT /api/tasks/:id            | 更新任务   | <br /> |
| DELETE /api/tasks/:id         | 删除任务   | <br /> |
| POST /api/tasks/:id/trigger   | 触发任务   | <br /> |
| GET /api/tasks/:id/executions | 任务执行历史 | <br /> |

### 8.3 工作流接口

| 接口                        | 方法    | 说明     |
| ------------------------- | ----- | ------ |
| GET /api/workflows        | 工作流列表 | <br /> |
| POST /api/workflows       | 创建工作流 | <br /> |
| GET /api/workflows/:id    | 工作流详情 | <br /> |
| PUT /api/workflows/:id    | 更新工作流 | <br /> |
| DELETE /api/workflows/:id | 删除工作流 | <br /> |

### 8.4 执行器接口

| 接口                        | 方法    | 说明     |
| ------------------------- | ----- | ------ |
| GET /api/executors        | 执行器列表 | <br /> |
| GET /api/executors/:id    | 执行器详情 | <br /> |
| DELETE /api/executors/:id | 删除执行器 | <br /> |

