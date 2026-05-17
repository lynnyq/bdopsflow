# BDopsFlow 分布式工作流调度平台

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.24+-blue.svg" alt="Go">
  <img src="https://img.shields.io/badge/Vue-3.4-green.svg" alt="Vue">
  <img src="https://img.shields.io/badge/gRPC-Enabled-brightgreen.svg" alt="gRPC">
  <img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License">
</p>

生产级、高可用、无单点、可集群、自动故障转移的分布式工作流调度平台，支持完整的 RBAC 权限管理和多租户隔离。

## ✨ 核心特性

- ✅ **分布式架构**：调度中心集群 + 执行器集群，完全解耦
- ✅ **gRPC 通信**：调度中心与执行器 100% 使用 gRPC 通信
- ✅ **工作流 DAG**：支持任务依赖和工作流编排
- ✅ **高可用**：主节点选举、故障自动转移
- ✅ **幂等控制**：Redis 分布式锁 + 状态机，防止重复执行
- ✅ **锁续期机制**：执行器心跳续期锁 TTL，防止任务卡死
- ✅ **任务执行**：支持 HTTP、Shell 任务类型
- ✅ **超时重试**：超时控制、可配置重试策略
- ✅ **RBAC 权限管理**：完整的角色权限控制系统
- ✅ **多租户**：领域级资源隔离
- ✅ **可观测**：任务执行历史、日志记录
- ✅ **Webhook 回调**：灵活的推送时机配置（成功/失败/全部）
- ✅ **Cron 兼容**：支持 5 位分钟级和 6 位秒级 Cron 表达式
- ✅ **前端**：Vue3 + Element Plus

## 🛠️ 技术栈

### 后端

- **语言**：Go 1.24+
- **框架**：Gin (HTTP API), gRPC (通信)
- **数据库**：rqlite (分布式 SQLite)
- **缓存/锁**：Redis
- **协议**：Protocol Buffers
- **密码加密**：bcrypt

### 前端

- **框架**：Vue 3 + TypeScript
- **构建**：Vite
- **UI 组件库**：Element Plus
- **状态管理**：Pinia

## 📁 项目结构

```
bdopsflow/
├── docs/                             # 文档目录
│   ├── ARCHITECTURE.md              # 架构设计文档
│   ├── DEPLOYMENT.md                # 部署文档
│   ├── API.md                       # API 接口文档
│   ├── SCHEDULER_CENTER.md          # 调度中心功能说明
│   ├── DATABASE.md                  # 数据库设计文档
│   ├── GRPC.md                      # gRPC 通信协议
│   ├── FRONTEND.md                  # 前端使用指南
│   ├── EXECUTOR.md                  # 执行器使用指南
│   └── WEBHOOK.md                   # Webhook 接入指南
├── deploy/                          # 部署文件
│   ├── Dockerfile.scheduler
│   ├── Dockerfile.executor
│   ├── Dockerfile.web
│   ├── docker-compose.yml
│   └── schema.sql                   # 数据库初始化脚本
├── scheduler/                       # 调度中心
│   ├── cmd/main.go
│   ├── internal/
│   │   ├── config/                  # 配置管理
│   │   ├── model/                   # 数据模型
│   │   ├── service/                 # 业务逻辑
│   │   ├── handler/                 # HTTP 处理器
│   │   ├── grpcserver/             # gRPC 服务端
│   │   ├── middleware/             # 中间件（JWT/RBAC）
│   │   ├── cron/                   # Cron 调度器
│   │   ├── dag/                    # DAG 工作流
│   │   ├── lineage/                # 血缘关系
│   │   └── webhook/                # Webhook 服务
│   └── pkg/
│       ├── election/                # 主节点选举
│       └── lock/                    # 分布式锁
├── executor/                        # 执行器
│   ├── cmd/main.go
│   └── internal/
│       ├── config/
│       ├── executor/                # 任务执行器
│       ├── pool/                    # 协程池
│       ├── grpcclient/              # gRPC 客户端
│       └── logger/                  # 日志管理
├── proto/                           # Protobuf 定义
│   ├── executor.proto
│   ├── executor.pb.go
│   └── executor_grpc.pb.go
├── web/                             # Vue3 前端
│   ├── src/
│   │   ├── api/                     # API 调用
│   │   ├── components/              # 组件
│   │   ├── views/                   # 页面组件
│   │   ├── router/                  # 路由配置
│   │   ├── stores/                  # Pinia 状态管理
│   │   └── types/                   # TypeScript 类型
│   └── package.json
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

## 🚀 快速开始

### 前置条件

- Go 1.24+
- Redis 7.0+
- rqlite 8.0+
- Node.js 18+ (前端)
- Git

### 1. 克隆项目

```bash
git clone https://github.com/lynnyq/bdopsflow.git
cd bdopsflow
```

### 2. 启动依赖服务

#### 启动 Redis

```bash
# 使用 Docker（推荐）
docker run -d --name bdopsflow-redis -p 6379:6379 redis:7-alpine
```

#### 启动 rqlite

```bash
# 使用 Docker（推荐）
docker run -d --name bdopsflow-rqlite -p 4001:4001 rqlite/rqlite:latest

# 等待几秒让 rqlite 启动
sleep 3

# 初始化数据库
curl -XPOST 'http://localhost:4001/db/load?pretty' --data-binary @deploy/schema.sql
```

### 3. 编译并启动调度中心

```bash
cd scheduler

# 编译
go build -o bin/scheduler ./cmd/main.go

# 复制配置文件
cp config.yaml.example config.yaml

# 运行
./bin/scheduler
```

预期输出：

```
Connected to Redis successfully
Connected to rqlite successfully
gRPC server listening on port 50051
HTTP server listening on port 8080
```

### 4. 编译并启动执行器（新开终端）

```bash
cd executor

# 编译
go build -o bin/executor ./cmd/main.go

# 复制配置文件
cp config.yaml.example config.yaml

# 运行
./bin/executor
```

预期输出：

```
[Executor] Registered with scheduler successfully
[Executor] Subscribed to tasks
[Executor] Executor executor-1 started (capacity: 10)
```

### 5. 启动前端（新开终端）

```bash
cd web

# 安装依赖
npm install

# 启动开发服务器
npm run dev
```

访问 http://localhost:5173

### 6. 登录系统

默认管理员账号：

- 用户名：`admin`
- 密码：`admin123`

## 🏛️ 权限系统

### 预设角色

| 角色 | 说明 |
|------|------|
| 系统管理员 | 系统最高权限，可管理所有资源 |
| 领域管理员 | 领域级管理权限 |
| 普通用户 | 基础查看和操作权限 |

### 权限资源

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

### 领域隔离

- 资源绑定到领域
- 执行器可分配到领域
- 用户可在不同领域拥有不同角色
- 跨领域资源不可见

## 📖 文档

| 文档 | 说明 |
|------|------|
| [架构设计文档](docs/ARCHITECTURE.md) | 系统架构详细说明 |
| [部署文档](docs/DEPLOYMENT.md) | 生产环境部署指南 |
| [API 接口文档](docs/API.md) | 完整 API 接口说明 |
| [调度中心功能说明](docs/SCHEDULER_CENTER.md) | 调度中心核心功能详解 |
| [数据库设计文档](docs/DATABASE.md) | 数据库结构与优化指南 |
| [gRPC 通信协议](docs/GRPC.md) | 执行器通信协议定义 |
| [前端使用指南](docs/FRONTEND.md) | Web 界面操作手册 |
| [执行器使用指南](docs/EXECUTOR.md) | 执行器配置与部署 |
| [Webhook 接入指南](docs/WEBHOOK.md) | Webhook 配置和使用 |

## 🐳 Docker 部署

使用 Docker Compose 一键启动所有服务：

```bash
cd deploy
docker-compose up -d
```

服务地址：

- 前端：http://localhost:3000
- 调度中心 HTTP API：http://localhost:8080
- 调度中心 gRPC：localhost:50051
- Redis：localhost:6379

## ⚙️ 配置说明

### 调度中心配置 (scheduler/config.yaml)

```yaml
app:
  http_port: "8080"
  grpc_port: "50051"

database:
  rqlite_dsn: "http://localhost:4001"

redis:
  addr: "localhost:6379"
  password: ""
  db: 0

jwt:
  secret: "your-secret-key-change-in-production"
  expiry_hours: 24

log:
  level: "info"
  format: "json"
```

### 执行器配置 (executor/config.yaml)

```yaml
app:
  executor_id: "executor-1"
  executor_name: "executor-1"
  capacity: 10

scheduler:
  addr: "localhost:50051"
  timeout: 30

log:
  level: "info"
  format: "json"
```

## 🔧 开发调试

### 运行测试

```bash
# 后端测试
cd scheduler
go test -v ./...

cd ../executor
go test -v ./...
```

### 前端开发

```bash
cd web

# 开发模式
npm run dev

# 类型检查
npm run type-check

# 构建生产版本
npm run build
```

## 🔐 安全建议

1. 生产环境必须修改默认密码
2. 使用 HTTPS 加密通信
3. 配置 Redis 密码认证
4. 启用防火墙规则
5. 遵循最小权限原则
6. 定期备份数据
7. 日志审计
8. 使用强 JWT 密钥（至少 32 字符）

## 📄 许可证

MIT License

***

**享受使用 BDopsFlow！** 🎉