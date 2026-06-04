# BDopsFlow 项目规则

> 此文件由 Trae Solo 自动读取，确保开发时遵循 AGENTS.md 规范。

## 项目概述

BDopsFlow 是一个分布式运维调度平台，采用 Go + Vue3 全栈架构。

**核心功能：**
- 分布式任务调度（HTTP 和 Shell 任务）
- 多数据源查询（支持 9 种数据库）
- RBAC 多租户权限管理
- SSO 登录支持
- 实时日志推送
- Webhook 回调
- 审计日志

**技术栈：**
- 后端：Go 1.24+, Gin, gRPC
- 前端：Vue 3, TypeScript, Element Plus, CodeMirror 6
- 存储：rqlite (分布式 SQLite), Redis
- 通信：HTTP REST, gRPC, SSE

## 目录结构

```
bdopsflow/
├── scheduler/          # 调度中心
│   ├── cmd/
│   │   ├── main.go     # 入口文件
│   │   └── app.go      # 应用核心
│   ├── internal/
│   │   ├── config/     # 配置管理
│   │   ├── handler/    # HTTP 处理器
│   │   ├── service/    # 业务逻辑
│   │   ├── datasource/ # 数据源驱动
│   │   ├── cron/       # Cron 调度器
│   │   ├── middleware/ # 中间件
│   │   └── model/      # 数据模型
│   └── web/            # 嵌入的 Web UI
├── executor/           # 执行器
│   ├── cmd/
│   │   └── main.go     # 入口文件
│   └── internal/
│       ├── executor/   # 任务执行
│       ├── grpcclient/ # gRPC 客户端
│       └── pool/       # 协程池
├── web/                # 前端项目
│   ├── src/
│   │   ├── views/      # 页面组件
│   │   ├── api/        # API 层
│   │   ├── stores/     # Pinia 状态管理
│   │   └── router/     # 路由
│   └── package.json
├── deploy/             # 部署配置
│   └── schema.sql      # 数据库架构
├── proto/              # gRPC 协议定义
└── docs/               # 文档
```

## 技能启用规则

- **需求开发** → 自动启用 `spec_coding` 技能
- **代码评审** → 启用 `code_review` 技能

## 通用规范

- **代码风格**：遵循各语言的官方规范
  - Go：使用 `gofmt` 格式化
  - TypeScript/Vue：使用项目配置的 lint 规则
- **文档优先**：修改功能时同步更新相关文档
- **测试优先**：新增功能时添加相应的单元测试
- **提交规范**：明确的 commit message，避免大而全的提交

## 代码规范

### 错误处理规范
- **所有 error 必须接收校验**，禁止使用 `_` 忽略错误
- 错误处理应考虑上下文，提供清晰的错误信息
- 如确实不需要处理的错误，必须注释说明原因

### 安全规范
- **密码、地址、密钥统一从配置/环境变量读取**，禁止硬编码
- 敏感信息加密存储
- 遵循配置项使用项目已有的密钥管理机制

### 日志规范
- **统一项目日志组件**，禁用 `fmt.Println` 在上线代码
- 使用 `slog` 或项目封装的日志组件
- 日志分级记录：debug/info/warn/error
- 日志中避免输出敏感信息

### 依赖管理规范
- **新增第三方依赖需要标注用途**，不随意引入无用包
- 先评估现有依赖是否能满足需求
- 更新依赖前评估变更范围和风险

### 性能规范
- **高频代码减少临时对象创建**，兼顾内存与性能
- 合理使用对象池、缓存等优化手段
- 避免不必要的字符串拼接、切片扩容优化

### 代码修改准则

- **原有正常逻辑不主动重构**，最小范围修复 BUG
- **新功能向下兼容**老接口、老数据表结构
- **大改动简要备注修改思路
- 禁止裸 panic、跨层操作 DB、冗余代码、过度设计封装

## 调度中心 (scheduler) 开发规范

**核心组件：**
- `cmd/app.go`：应用初始化和生命周期管理
- `internal/config/`：配置加载和验证
- `internal/handler/`：HTTP 路由处理
- `internal/service/`：业务逻辑层
- `internal/datasource/`：数据源驱动实现
- `internal/middleware/`：JWT、权限、审计等中间件

**新增数据源驱动：**
1. 在 `scheduler/internal/datasource/driver/` 中创建新文件
2. 实现 `Driver` 接口
3. 在 `datasource/manager.go` 的 `NewDriver` 函数中注册
4. 添加相应的测试用例

**权限模型：**
- 使用 `middleware.RequirePermission` 进行权限检查
- 新增资源时在 `model/permission.go` 中添加权限定义
- 前端菜单根据权限自动推导

## 执行器 (executor) 开发规范

**核心组件：**
- `internal/executor/task_executor.go`：任务执行核心
- `internal/grpcclient/`：gRPC 通信
- `internal/pool/`：协程池管理

**新增任务类型：**
1. 在 `task_executor.go` 中扩展 `ExecuteTask` 函数
2. 添加相应的参数验证逻辑
3. 更新 proto 定义（如需要）

## 前端 (web) 开发规范

**技术选型：**
- Vue 3 (Composition API) + TypeScript
- Element Plus 组件库
- Pinia 状态管理
- Vue Router 路由
- CodeMirror 6 SQL 编辑器

**文件结构：**
- `views/SQLQuery/`：SQL 查询相关组件（含编辑器优化）
- `api/`：按模块组织的 API 封装
- `stores/auth.ts`：认证状态管理
- `router/index.ts`：路由配置

**新增页面：**
1. 在 `views/` 中创建页面组件
2. 在 `router/index.ts` 中添加路由
3. 在 `config/menuPermissionMap.ts` 中配置菜单权限

**SQL 编辑器优化：**
- 编辑器实现在 `views/SQLQuery/SQLQuery.vue`
- 自动补全配置在同文件的 `sqlCompletions`
- 支持智能上下文补全、快捷键绑定（Ctrl+Enter/Cmd+Enter 执行）
- 使用 CodeMirror 6 扩展系统

## 数据库变更规范

**Schema 管理：**
- 主 schema：`deploy/schema.sql`
- 迁移脚本放在 `deploy/migrations/` 目录
- 迁移文件命名：`v{版本}_{描述}.sql`
- 记录所有 DDL 变更

**新增字段/表：**
1. 更新 `schema.sql`
2. 创建迁移脚本
3. 更新相关 `model/` 定义
4. 如涉及数据加密，更新加密逻辑

## 配置管理规范

**配置项：**
- 调度中心配置：`scheduler/config.yaml.example`
- 执行器配置：`executor/config.yaml.example`
- 系统运行时配置：通过 API 管理，存储在数据库

**新增配置项：**
1. 更新配置结构体（如需要）
2. 更新配置示例文件
3. 在文档中记录新配置项
4. 如需运行时配置，更新系统配置 API

## 测试规范

**测试文件位置：**
- Go 测试：与被测试文件同目录，`{name}_test.go`
- 前端测试：同目录或 `__tests__/` 子目录

**运行测试：**
```bash
# Go 测试
cd scheduler && go test ./internal/... -v
cd executor && go test ./internal/... -v

# 前端测试
cd web && npm test
```

## 常用命令

**开发环境：**
```bash
# 启动调度中心
cd scheduler && go run cmd/main.go

# 启动执行器
cd executor && go run cmd/main.go --executor-name my-exec --scheduler-addr localhost:50051

# 启动前端
cd web && npm run dev
```

**构建：**
```bash
# 调度中心
cd scheduler && go build -o bin/scheduler cmd/main.go

# 执行器
cd executor && go build -o bin/executor cmd/main.go

# 前端
cd web && npm run build
```

**密钥管理：**
```bash
# 生成 RSA 密钥对
./scheduler keygen

# 加密/解密密码
./scheduler encrypt-password --config config.yaml --password plaintext
./scheduler decrypt-password --config config.yaml --ciphertext ciphertext
```

## 常见场景处理

### 场景 1：修复 Bug
1. 定位问题代码
2. 编写测试用例复现问题
3. 最小范围修复问题
4. 确保测试通过
5. 更新相关文档（如需要）

### 场景 2：新增功能
1. 明确需求，确认与现有架构的兼容性
2. 启用 `spec_coding` 技能进行需求开发
3. 设计实现方案（必要时先讨论）
4. 实现后端功能（接口、业务逻辑）
5. 实现前端功能
6. 添加测试
7. 更新文档

### 场景 3：优化性能
1. 识别性能瓶颈
2. 设计优化方案
3. 实现优化（减少临时对象创建等）
4. 添加性能基准测试
5. 更新文档

### 场景 4：安全修复
1. 评估安全风险
2. 设计修复方案
3. 实现修复
4. 添加安全测试
5. 更新安全文档
6. 如涉及配置变更，更新配置文档

### 场景 5：代码评审
1. 启用 `code_review` 技能
2. 检查是否有忽略的 error
3. 检查是否有硬编码的敏感信息
4. 检查日志使用是否规范
5. 检查是否有不必要的依赖引入
6. 检查是否有过度设计

## 文档维护

**主要文档：**
- `README.md`：项目概览和快速开始
- `docs/GUIDE.md`：详细使用指南
- `docs/CONFIGURATION.md`：配置文档
- `AGENTS.md`：开发指南

**新增功能时：**
1. 更新相关文档
2. 在文档中添加示例
3. 更新 API 参考（如需要）

## 问题排查

**常见问题：**
- 编译错误：检查 Go/Node 版本，运行 `go mod tidy` 或 `npm install`
- 数据库连接问题：检查 rqlite/Redis 配置
- 权限问题：确保用户角色和权限配置正确
- 前端 API 调用失败：检查网络和 CORS 配置

**日志排查：**
- 调度中心日志：默认 INFO 级别，可配置
- 执行器日志：包含任务执行详情
- 审计日志：记录所有重要操作

