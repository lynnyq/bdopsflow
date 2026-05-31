## BDopsFlow

> 分布式运维调度平台

### 核心特性

- **分布式架构**：Scheduler + Executor 分离，gRPC 通信，Leader Election 高可用
- **RBAC 多租户**：纯 RBAC 权限模型，角色继承，多领域支持，数据源/Webhook 实例级权限控制，菜单权限自动推导
- **数据源查询**：9 种数据库驱动，SQL 编辑器，查询缓存，并发控制，CSV 导出
- **实时日志**：gRPC 流式传输 → Redis Pub/Sub → SSE 推送
- **Webhook 回调**：HMAC-SHA256 签名验证，指数退避重试
- **审计日志**：全操作审计，中间件+Handler 协作模式
- **SSO 登录**：双模式登录（本地+SSO），自动创建用户

### 技术栈

| 层 | 技术 |
|---|------|
| 后端 | Go 1.24+, Gin, gRPC, gorqlite |
| 前端 | Vue 3, TypeScript, Element Plus |
| 存储 | rqlite (Raft SQL), Redis |
| 通信 | HTTP REST, gRPC, SSE |

### 项目结构

```
bdopsflow/
├── scheduler/          # 调度中心
│   ├── cmd/           # 入口 + 路由
│   └── internal/
│       ├── handler/    # HTTP 处理器
│       ├── service/    # 业务逻辑
│       ├── middleware/  # JWT/RBAC/审计/数据源权限
│       ├── datasource/ # 数据源管理 + 查询
│       ├── cron/       # Cron 调度器
│       ├── webhook/    # Webhook 服务
│       └── config/     # 配置
├── executor/           # 执行器
│   └── internal/
│       ├── executor/   # 任务执行（HTTP/Shell）
│       └── config/     # 配置
├── web/                # 前端
│   └── src/
│       ├── views/      # 页面
│       └── api/        # API 接口
└── deploy/             # 部署配置
    └── schema.sql      # 数据库 Schema
```

### 快速开始

#### 环境要求

- Go 1.24+
- Node.js 18+
- rqlite
- Redis

#### Docker Compose 一键启动

```bash
git clone https://github.com/lynnyq/bdopsflow.git
cd bdopsflow
docker-compose up -d
```

访问 http://localhost:8080，默认账号 admin/admin123

#### 手动启动

1. 启动 rqlite 和 Redis
2. 启动调度中心：
```bash
cd scheduler
cp config.yaml.example config.yaml
go run cmd/main.go
```
3. 启动执行器：
```bash
cd executor
cp config.yaml.example config.yaml
go run cmd/main.go
```
4. 启动前端：
```bash
cd web
npm install
npm run dev
```

### RBAC 权限模型

采用纯 RBAC 权限模型，用户与角色通过 `user_roles` 表关联，权限通过 `role_permissions` 表管理，支持角色继承和多领域切换。

| 角色 | 权限范围 |
|------|---------|
| system_admin | 全局管理，所有资源权限 |
| domain_admin | 领域内管理，继承普通用户权限 |
| user | 领域内基本操作（查看、触发、查询） |

核心特性：
- 角色继承：子角色自动继承父角色的所有权限
- 多领域支持：用户可属于多个领域，通过领域切换器切换
- 菜单自动推导：前端菜单根据资源权限自动显示/隐藏
- 实例级权限：数据源和 Webhook 支持细粒度的实例权限控制
- 数据源支持 6 种权限级别：query/read/update/delete/download/manage
- Webhook 支持 5 种权限级别：read/update/delete/trigger/manage

### 📖 文档

| 文档 | 说明 |
|------|------|
| [使用指南](docs/GUIDE.md) | 完整的使用指南（架构、功能、API、错误码、部署、安全） |
| [配置指南](docs/CONFIGURATION.md) | 配置项详解、示例配置、密钥管理、安全检查清单 |

### License

MIT
