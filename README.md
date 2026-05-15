# BDopsFlow 分布式工作流调度平台

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.24+-blue.svg" alt="Go">
  <img src="https://img.shields.io/badge/Vue-3.4-green.svg" alt="Vue">
  <img src="https://img.shields.io/badge/gRPC-Enabled-brightgreen.svg" alt="gRPC">
  <img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License">
</p>

> **重要更新**：本项目已于 2026 年 5 月完全从 SQLite 迁移至 **rqlite**，现在开发和生产环境统一使用分布式 rqlite 数据库。

生产级、高可用、无单点、可集群、自动故障转移的分布式工作流调度平台。

## ✨ 核心特性

- ✅ **分布式架构**：调度中心集群 + 执行器集群，完全解耦
- ✅ **gRPC 通信**：调度中心与执行器 100% 使用 gRPC 通信
- ✅ **工作流 DAG**：支持任务依赖和工作流编排
- ✅ **高可用**：主节点选举、故障自动转移
- ✅ **幂等控制**：Redis 分布式锁 + 状态机，防止重复执行
- ✅ **锁续期机制**：执行器心跳续期锁 TTL，防止任务卡死
- ✅ **任务执行**：支持 HTTP、Shell 任务类型
- ✅ **超时重试**：超时控制、可配置重试策略
- ✅ **JWT 认证**：完整的用户认证和 RBAC 权限管理
- ✅ **多租户**：一级领域划分
- ✅ **可观测**：任务执行历史、日志记录
- ✅ **Webhook 回调**：灵活的推送时机配置（成功/失败/全部）
- ✅ **Cron 兼容**：支持 5 位分钟级和 6 位秒级 Cron 表达式
- ✅ **前端**：Vue3 + Element Plus + Neo-Brutalist 设计风格

## 🛠️ 技术栈

### 后端

- **语言**：Go 1.24+
- **框架**：Gin (HTTP API), gRPC (通信)
- **数据库**：rqlite (分布式 SQLite) - 开发和生产环境统一使用
- **缓存/锁**：Redis
- **协议**：Protocol Buffers
- **密码加密**：bcrypt

### 前端

- **框架**：Vue 3 + TypeScript
- **构建**：Vite
- **UI 组件库**：Element Plus
- **状态管理**：Pinia
- **设计风格**：Neo-Brutalist

## 📁 项目结构

```
bdopsflow/
├── docs/                    # 文档
│   ├── ARCHITECTURE.md      # 架构设计
│   ├── DEPLOYMENT.md        # 部署文档
│   ├── PHASES.md            # 开发阶段计划
│   ├── API.md               # API 接口文档
│   └── WEBHOOK.md           # Webhook 接入指南
├── deploy/                  # 部署文件
│   ├── Dockerfile.scheduler
│   ├── Dockerfile.executor
│   ├── Dockerfile.web
│   ├── docker-compose.yml
│   └── schema.sql           # 数据库 schema
├── scheduler/               # 调度中心
│   ├── cmd/
│   │   └── main.go
│   ├── internal/
│   │   ├── config/          # 配置管理
│   │   ├── model/           # 数据模型
│   │   ├── service/         # 业务逻辑
│   │   ├── handler/         # HTTP 处理器
│   │   ├── grpcserver/      # gRPC 服务端
│   │   ├── middleware/      # JWT/RBAC 中间件
│   │   └── cron/            # Cron 调度
│   └── pkg/
│       ├── election/        # 主节点选举
│       └── lock/            # 分布式锁
├── executor/                # 执行器
│   ├── cmd/
│   │   └── main.go
│   └── internal/
│       ├── config/
│       ├── executor/        # 任务执行器
│       ├── pool/            # 协程池
│       └── grpcclient/      # gRPC 客户端
├── proto/                   # Protobuf 定义
│   └── executor.proto
├── web/                     # Vue3 前端
│   ├── src/
│   │   ├── api/             # API 调用
│   │   ├── views/           # 页面组件
│   │   ├── stores/          # Pinia 状态管理
│   │   ├── router/          # 路由配置
│   │   ├── types/           # TypeScript 类型
│   │   └── utils/
│   └── package.json
├── .gitignore
├── go.mod
├── go.sum
├── Makefile
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

# 或使用本地安装的 Redis
redis-server
```

#### 启动 rqlite

```bash
# 使用 Docker（推荐）
docker run -d --name bdopsflow-rqlite --hostname localhost -p 4001:4001 rqlite/rqlite:latest

# 等待几秒让 rqlite 启动
sleep 3

# 初始化数据库 schema
curl -XPOST 'http://localhost:4001/db/load?pretty' --data-binary @deploy/schema.sql
```

### 3. 编译并启动调度中心

```bash
cd scheduler

# 编译
go build -o bin/scheduler ./cmd/main.go

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

访问 <http://localhost:5173>

### 6. 登录系统

默认管理员账号：

- 用户名：`admin`
- 密码：`admin123`

## 📖 文档

- [架构设计文档](docs/ARCHITECTURE.md) - 系统架构详细说明
- [部署文档](docs/DEPLOYMENT.md) - 生产环境部署指南
- [API 接口文档](docs/API.md) - 完整 API 接口说明
- [Webhook 接入指南](docs/WEBHOOK.md) - Webhook 配置和使用

## 🔌 API 接口概览

### 认证接口

| 方法   | 接口                 | 说明       |
| ---- | ------------------ | -------- |
| POST | /api/auth/login    | 用户登录     |
| POST | /api/auth/register | 用户注册     |
| GET  | /api/auth/current  | 获取当前用户信息 |

### 任务接口

| 方法     | 接口                        | 说明       |
| ------ | ------------------------- | -------- |
| GET    | /api/tasks                | 获取任务列表   |
| POST   | /api/tasks                | 创建任务     |
| GET    | /api/tasks/:id            | 获取任务详情   |
| PUT    | /api/tasks/:id            | 更新任务     |
| DELETE | /api/tasks/:id            | 删除任务     |
| POST   | /api/tasks/:id/trigger    | 手动触发任务   |
| GET    | /api/tasks/:id/executions | 获取任务执行历史 |

### 工作流接口

| 方法     | 接口                         | 说明      |
| ------ | -------------------------- | ------- |
| GET    | /api/workflows             | 获取工作流列表 |
| POST   | /api/workflows             | 创建工作流   |
| GET    | /api/workflows/:id         | 获取工作流详情 |
| PUT    | /api/workflows/:id         | 更新工作流   |
| DELETE | /api/workflows/:id         | 删除工作流   |
| POST   | /api/workflows/:id/trigger | 触发工作流   |

### 执行器接口

| 方法     | 接口                 | 说明      |
| ------ | ------------------ | ------- |
| GET    | /api/executors     | 获取执行器列表 |
| GET    | /api/executors/:id | 获取执行器详情 |
| DELETE | /api/executors/:id | 删除执行器   |

详细 API 文档请参考 [API.md](docs/API.md)

## 🐳 Docker Compose 部署

使用 Docker Compose 一键启动所有服务（开发环境）：

```bash
cd deploy
docker-compose up -d
```

服务地址：

- 前端：<http://localhost:3000>
- 调度中心 HTTP API：<http://localhost:8080>
- 调度中心 gRPC：localhost:50051
- Redis：localhost:6379

生产环境部署请参考 [部署文档](docs/DEPLOYMENT.md)。

## ⚙️ 配置说明

### 调度中心配置 (scheduler/config.yaml)

```yaml
app:
  http_port: "8080"        # HTTP API 端口
  grpc_port: "50051"       # gRPC 服务端口

database:
  rqlite_dsn: "http://localhost:4001"  # rqlite 连接地址

redis:
  addr: "localhost:6379"   # Redis 地址
  password: ""             # Redis 密码
  db: 0                    # Redis 数据库

jwt:
  secret: "your-secret-key-change-in-production"  # JWT 密钥
  expiry_hours: 24         # Token 过期时间（小时）

log:
  level: "info"            # 日志级别
  format: "json"           # 日志格式
```

### 执行器配置 (executor/config.yaml)

```yaml
app:
  executor_id: "executor-1"     # 执行器唯一 ID
  executor_name: "executor-1"   # 执行器显示名称
  capacity: 10                  # 最大并发任务数

scheduler:
  addr: "localhost:50051"       # 调度中心 gRPC 地址
  timeout: 30                   # 连接超时（秒）

log:
  level: "info"
  format: "json"
```

详细配置说明请参考 [部署文档](docs/DEPLOYMENT.md)。

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

1. **生产环境必须修改默认密码**
2. **使用 HTTPS 加密通信**
3. **配置 Redis 密码认证**
4. **启用防火墙规则**
5. **遵循最小权限原则**
6. **定期备份数据**
7. **日志审计**

## 🐛 常见问题

### 调度中心启动失败？

```bash
# 检查 Redis 连接
redis-cli ping
# 应返回：PONG

# 检查 rqlite 是否正常运行
curl http://localhost:4001/status?pretty
```

### rqlite 连接问题？

```bash
# 检查 rqlite 容器状态
docker ps | grep rqlite

# 查看 rqlite 日志
docker logs bdopsflow-rqlite

# 验证 schema 是否存在
curl -XPOST 'http://localhost:4001/db/query?pretty' \
  -d '["SELECT name FROM sqlite_master WHERE type=\"table\""]'
```

### 执行器无法连接调度中心？

```bash
# 检查调度中心 gRPC 端口
telnet localhost 50051

# 检查防火墙
sudo iptables -L -n
```

### 任务执行失败？

1. 检查执行器是否在线
2. 检查任务配置是否正确
3. 查看执行器日志
4. 确认网络连通性

## 📄 许可证

MIT License

***

**享受使用 BDopsFlow！** 🎉
