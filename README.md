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
- ✅ **任务执行**：支持 HTTP、Shell 任务类型
- ✅ **超时重试**：超时控制、可配置重试策略
- ✅ **JWT 认证**：完整的用户认证和 RBAC 权限管理
- ✅ **多租户**：一级领域划分
- ✅ **可观测**：任务执行历史、日志记录
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
│   └── PHASES.md            # 开发阶段计划
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
docker run -d --name bdopsflow-rqlite -p 4001:4001 rqlite/rqlite:latest

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

访问 http://localhost:5173

### 6. 初始化系统

首次启动后，需要注册管理员账号：

```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "admin123",
    "role": "admin",
    "email": "admin@example.com"
  }'
```

然后就可以使用 admin / admin123 登录系统了！

## 📖 文档

- [架构设计文档](docs/ARCHITECTURE.md)
- [部署文档](docs/DEPLOYMENT.md)
- [开发阶段计划](docs/PHASES.md)

## 🔌 API 接口

### 认证接口

| 方法 | 接口 | 说明 |
|------|------|------|
| POST | /api/auth/login | 用户登录 |
| POST | /api/auth/register | 用户注册 |
| GET | /api/auth/current | 获取当前用户信息 |

### 任务接口

| 方法 | 接口 | 说明 |
|------|------|------|
| GET | /api/tasks | 获取任务列表 |
| POST | /api/tasks | 创建任务 |
| GET | /api/tasks/:id | 获取任务详情 |
| PUT | /api/tasks/:id | 更新任务 |
| DELETE | /api/tasks/:id | 删除任务 |
| POST | /api/tasks/:id/trigger | 手动触发任务 |
| GET | /api/tasks/:id/executions | 获取任务执行历史 |

### 工作流接口

| 方法 | 接口 | 说明 |
|------|------|------|
| GET | /api/workflows | 获取工作流列表 |
| POST | /api/workflows | 创建工作流 |
| GET | /api/workflows/:id | 获取工作流详情 |
| PUT | /api/workflows/:id | 更新工作流 |
| DELETE | /api/workflows/:id | 删除工作流 |

### 执行器接口

| 方法 | 接口 | 说明 |
|------|------|------|
| GET | /api/executors | 获取执行器列表 |
| GET | /api/executors/:id | 获取执行器详情 |
| DELETE | /api/executors/:id | 删除执行器 |

### 其他接口

| 方法 | 接口 | 说明 |
|------|------|------|
| GET | /health | 健康检查 |

## 💡 任务配置示例

### HTTP 任务

```json
{
  "name": "健康检查任务",
  "type": "http",
  "config": {
    "url": "https://api.example.com/health",
    "method": "GET",
    "headers": {
      "Authorization": "Bearer token"
    },
    "timeout": 10000
  },
  "timeout_seconds": 30,
  "retry_count": 3,
  "retry_interval": 5,
  "domain_id": 1
}
```

### Shell 任务

```json
{
  "name": "备份任务",
  "type": "shell",
  "config": {
    "script": "pg_dump -U postgres mydb > /backup/db-$(date +%Y%m%d).sql"
  },
  "timeout_seconds": 300,
  "retry_count": 2,
  "retry_interval": 10,
  "domain_id": 1
}
```

## 🐳 Docker Compose 部署

使用 Docker Compose 一键启动所有服务（开发环境）：

```bash
cd deploy
docker-compose up -d
```

服务地址：
- 前端：http://localhost:3000
- 调度中心 HTTP API：http://localhost:8080
- 调度中心 gRPC：localhost:50051
- Redis：localhost:6379

生产环境部署请参考 [部署文档](docs/DEPLOYMENT.md)。

## ⚙️ 配置说明

### 调度中心配置

| 环境变量 | 默认值 | 说明 |
|---------|-------|------|
| HTTP_PORT | 8080 | HTTP API 服务端口 |
| GRPC_PORT | 50051 | gRPC 服务端口 |
| RQLITE_DSN | http://localhost:4001 | rqlite HTTP API 地址，多节点用逗号分隔 |
| REDIS_ADDR | localhost:6379 | Redis 连接地址 |
| REDIS_PASSWORD | (空) | Redis 密码 |
| REDIS_DB | 0 | Redis 数据库编号 |

### 执行器配置

| 环境变量 | 默认值 | 说明 |
|---------|-------|------|
| EXECUTOR_ID | executor-1 | 执行器唯一标识 |
| EXECUTOR_NAME | executor-1 | 执行器显示名称 |
| SCHEDULER_ADDR | localhost:50051 | 调度中心 gRPC 地址 |
| CAPACITY | 10 | 最大并发执行任务数 |

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

### 代码规范

- Go：遵循 Go 官方代码规范
- TypeScript：使用 ESLint + Prettier

## 📊 当前进度

- ✅ 阶段一：核心架构搭建
- ✅ 阶段二：调度中心核心功能
- ✅ 阶段三：执行器核心功能
- ⚠️ 阶段四：完整功能实现（部分完成）
- ✅ 阶段五：Vue3 前端开发（大部分完成）
- ⚠️ 阶段六：测试、优化、部署（部分完成）

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

## 🤝 贡献指南

欢迎贡献代码、报告问题或提出建议！

## 📄 许可证

MIT License

## 📧 技术支持

- 提交 Issue：GitHub Issues
- 文档：[docs 目录](docs/)

---

**享受使用 BDopsFlow！** 🎉
