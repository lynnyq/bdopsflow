## BDopsFlow

> 分布式运维调度平台

### 核心特性

- **Dify 风格工作流**：可视化 DAG 编排，9 种节点类型（Start/End/HTTP/Shell/IF-ELSE/Delay/Webhook/变量聚合/数据转换），条件分支，并行执行，变量引用 `{{node_id.field}}`
- **X6 可视化画布**：基于 AntV X6 的流程编辑器，拖拽编排，条件分支多出口连线，运行态可视化
- **分布式架构**：Scheduler + Executor 分离，gRPC 通信，Leader Election 高可用
- **RBAC 多租户**：system_admin/domain_admin/user 三级角色，领域隔离，数据源权限控制
- **数据源查询**：9 种数据库驱动，SQL 编辑器，查询缓存，并发控制，CSV 导出
- **实时日志**：gRPC 流式传输 → Redis Pub/Sub → SSE 推送
- **Webhook 回调**：HMAC-SHA256 签名验证，指数退避重试
- **审计日志**：全操作审计，中间件+Handler 协作模式
- **SSO 登录**：双模式登录（本地+SSO），自动创建用户

### 技术栈

| 层 | 技术 |
|---|------|
| 后端 | Go 1.24+, Gin, gRPC, gorqlite |
| 前端 | Vue 3, TypeScript, Element Plus, AntV X6 |
| 存储 | rqlite (Raft SQL), Redis |
| 通信 | HTTP REST, gRPC, SSE |

### 项目结构

```
bdopsflow/
├── scheduler/          # 调度中心
│   ├── cmd/           # 入口 + 路由
│   └── internal/
│       ├── workflow/   # Dify 风格工作流引擎
│       │   └── executors/  # 9 种节点执行器
│       ├── dag/        # DAG 验证 + 拓扑排序
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
│       ├── components/workflow/  # X6 工作流编辑器
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

### 工作流节点类型

| 节点 | 说明 | 输出示例 |
|------|------|---------|
| Start | 工作流入口，定义输入变量 | variables |
| End | 工作流出口，定义输出变量 | outputs |
| HTTP | HTTP 请求 | status_code, response |
| Shell | Shell 脚本执行 | stdout, stderr, exit_code |
| IF/ELSE | 条件分支（true/false 双出口） | result, branch |
| Delay | 延迟等待 | waited_seconds |
| Webhook | Webhook 通知 | status_code, response |
| 变量聚合 | 合并多个输入变量 | merged_variables |
| 数据转换 | 表达式/脚本转换 | result |

### RBAC 权限模型

| 角色 | 权限范围 |
|------|---------|
| system_admin | 全局管理，跨域访问 |
| domain_admin | 领域内管理 |
| user | 领域内基本操作 |

领域隔离规则：
- 非管理员只能操作自己 domain_id 下的资源
- 创建资源时，非管理员强制使用 JWT 中的 domain_id
- 数据源支持 6 种权限级别：read/update/delete/query/download/manage

### 📖 文档

| 文档 | 说明 |
|------|------|
| [使用指南](docs/GUIDE.md) | 完整的使用指南（架构、功能、API、错误码、部署、安全） |
| [配置指南](docs/CONFIGURATION.md) | 配置项详解、示例配置、密钥管理、安全检查清单 |

### License

MIT
