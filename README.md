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
- ✅ **任务恢复**：调度器切换时自动恢复执行中的任务
- ✅ **实时日志**：任务执行日志实时传输与展示
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
│   ├── DEVELOPMENT.md               # 开发、部署和使用指南
│   ├── FEATURES.md                  # 核心功能参考（所有功能实现详解）
│   ├── LOGGING.md                   # 任务日志系统文档
│   ├── ARCHITECTURE.md              # 架构设计文档
│   └── API.md                       # API 接口文档
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
| [开发指南](docs/DEVELOPMENT.md) | 完整的开发、部署和使用指南 |
| [数据库设计](docs/DATABASE.md) | 数据库表结构和配置说明 |
| [核心功能参考](docs/FEATURES.md) | 所有核心功能实现详解 |
| [任务日志系统](docs/LOGGING.md) | 实时日志传输与展示实现详解 |
| [架构设计](docs/ARCHITECTURE.md) | 系统架构和技术设计文档 |
| [API 接口](docs/API.md) | RESTful API 接口文档 |

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

#### 开发环境（单节点）
```yaml
app:
  http_port: "8080"
  grpc_port: "50051"

database:
  rqlite_addrs:
    - "http://localhost:4001"
  rqlite_user: ""
  rqlite_password: ""
  rqlite_tls: false

redis:
  mode: "single"
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

#### 生产环境（集群模式）
```yaml
app:
  http_port: "8080"
  grpc_port: "50051"

database:
  rqlite_addrs:
    - "http://rqlite1:4001"
    - "http://rqlite2:4001"
    - "http://rqlite3:4001"
  rqlite_user: "admin"
  rqlite_password: "your-rqlite-password"
  rqlite_tls: false

redis:
  mode: "sentinel"
  master_name: "mymaster"
  sentinel_addrs:
    - "sentinel1:26379"
    - "sentinel2:26379"
    - "sentinel3:26379"
  sentinel_password: ""
  password: "your-redis-password"
  db: 0

jwt:
  secret: "your-secure-secret-key-change-in-production"
  expiry_hours: 24

log:
  level: "info"
  format: "json"
```

### 执行器配置 (executor/config.yaml)

执行器支持配置文件和命令行参数两种配置方式，命令行参数优先级高于配置文件。

#### 配置文件示例

```yaml
app:
  executor_name: "executor-1"  # 必需：执行器名称
  capacity: 10                  # 可选：并发任务数，默认 10

scheduler:
  addr: "localhost:50051"       # 必需：调度中心 gRPC 地址
  timeout: 30                   # 可选：超时时间，默认 30 秒

log:
  level: "info"                 # 可选：日志级别，默认 info
  format: "json"                # 可选：日志格式，默认 json
```

#### 命令行参数（优先级高于配置文件）

```bash
# 必需参数
./executor --executor-name "my-executor" --scheduler-addr "localhost:50051"

# 可选参数
./executor --executor-name "my-executor" \
            --scheduler-addr "localhost:50051" \
            --capacity 20 \
            --timeout 60 \
            --log-level debug

# 使用配置文件 + 命令行覆盖
./executor --config /path/to/config.yaml \
            --executor-name "override-name" \
            --capacity 30
```

#### 参数说明

| 参数 | 配置文件字段 | 必需 | 默认值 | 说明 |
|------|-------------|------|--------|------|
| --executor-name | app.executor_name | 是 | - | 执行器唯一名称 |
| --scheduler-addr | scheduler.addr | 是 | - | 调度中心 gRPC 地址 |
| --capacity | app.capacity | 否 | 10 | 最大并发任务数 |
| --timeout | scheduler.timeout | 否 | 30 | gRPC 超时时间（秒） |
| --hostname | app.hostname | 否 | 系统主机名 | 执行器注册地址 |
| --log-level | log.level | 否 | info | 日志级别 |
| --log-format | log.format | 否 | json | 日志格式 |

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