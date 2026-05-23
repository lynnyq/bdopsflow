# BDopsFlow 开发指南

本文档为 BDopsFlow 分布式工作流调度平台提供完整的开发环境搭建、部署指南和最佳实践。

## 目录

- [环境要求](#环境要求)
- [快速开始（单节点开发）](#快速开始单节点开发)
- [多节点集群部署](#多节点集群部署)
- [使用 Makefile 快捷命令](#使用-makefile-快捷命令)
- [开发调试技巧](#开发调试技巧)
- [添加审计日志埋点](#添加审计日志埋点)
- [添加新数据源类型](#添加新数据源类型)
- [测试指南](#测试指南)
- [常见问题](#常见问题)

---

## 环境要求

### 必需组件

| 组件 | 版本要求 | 说明 |
|------|----------|------|
| Go | 1.24+ | 后端语言 |
| Node.js | 18+ | 前端构建 |
| Redis | 7.0+ | 分布式锁、缓存、主节点选举 |
| rqlite | 8.0+ | 分布式数据库 |
| Docker | 20.0+ | 容器化部署（可选） |

### 硬件要求（开发环境）

- CPU: 2 核以上
- 内存: 4 GB 以上
- 磁盘: 20 GB 以上

---

## 快速开始（单节点开发）

### 1. 克隆项目

```bash
git clone https://github.com/lynnyq/bdopsflow.git
cd bdopsflow
```

### 2. 启动依赖服务

#### 方式 A：使用 Docker（推荐）

项目包含预配置的 Docker Compose 文件用于开发环境：

```bash
# 启动 Redis 和 rqlite
cd deploy
docker-compose up -d redis rqlite1

# 等待服务启动
sleep 5

# 初始化数据库
curl -XPOST 'http://localhost:4001/db/load?pretty' \
    --data-binary @deploy/schema.sql
```

#### 方式 B：手动启动服务

```bash
# 启动 Redis
docker run -d --name bdopsflow-redis \
    -p 6379:6379 \
    redis:7-alpine \
    redis-server --appendonly yes

# 启动 rqlite
docker run -d --name bdopsflow-rqlite \
    -p 4001:4001 \
    -p 4002:4002 \
    rqlite/rqlite:latest

# 初始化数据库
sleep 5
curl -XPOST 'http://localhost:4001/db/load?pretty' \
    --data-binary @deploy/schema.sql
```

### 3. 编译和启动调度中心

```bash
cd scheduler

# 复制配置文件
cp config.yaml.example config.yaml

# 编译（可选）
go build -o bin/scheduler ./cmd/main.go

# 方式 A：直接运行
./bin/scheduler

# 方式 B：使用 go run
go run ./cmd/main.go

# 方式 C：使用 Makefile
cd ..
make scheduler-run
```

**验证调度中心启动**

```bash
# 健康检查
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

### 4. 编译和启动执行器

```bash
cd executor

# 复制配置文件
cp config.yaml.example config.yaml

# 编译（可选）
go build -o bin/executor ./cmd/main.go

# 方式 A：直接运行
./bin/executor

# 方式 B：使用 go run
go run ./cmd/main.go

# 方式 C：使用 Makefile
cd ..
make executor-run
```

### 5. 启动前端

```bash
cd web

# 安装依赖
npm install

# 启动开发服务器
npm run dev
```

访问 http://localhost:5173，使用默认账号登录：

- 用户名：`admin`
- 密码：`admin123`

### 6. 使用 Docker Compose 一键启动（全环境）

如果你想跳过上述步骤，直接启动完整的开发环境：

```bash
cd deploy
docker-compose up -d

# 等待所有服务启动
sleep 15

# 访问
# - 前端：http://localhost:3000
# - 调度中心 API：http://localhost:8080
# - 调度中心 gRPC：localhost:50051
```

---

## 多节点集群部署

本节介绍如何部署具有高可用性的多节点集群。

### 架构概述

- **3 个调度中心节点**：通过 Redis 选举主节点，实现故障自动转移
- **3 个 rqlite 节点**：通过 Raft 协议保证数据一致性
- **Redis Sentinel**：Redis 高可用（生产环境建议）
- **多执行器节点**：动态注册和负载均衡

### 1. 部署 Redis（哨兵模式）

创建 `docker-compose-redis.yml`：

```yaml
version: '3.8'
services:
  redis-master:
    image: redis:7-alpine
    container_name: redis-master
    command: >
      redis-server
      --bind 0.0.0.0
      --port 6379
      --requirepass your-redis-password
      --masterauth your-redis-password
      --appendonly yes
    ports:
      - "6379:6379"
    volumes:
      - redis-master-data:/data
    networks:
      - redis-network
    healthcheck:
      test: ["CMD", "redis-cli", "-a", "your-redis-password", "ping"]
      interval: 10s
      timeout: 5s
      retries: 3

  redis-slave1:
    image: redis:7-alpine
    container_name: redis-slave1
    command: >
      redis-server
      --bind 0.0.0.0
      --port 6379
      --requirepass your-redis-password
      --masterauth your-redis-password
      --replicaof redis-master 6379
      --appendonly yes
    ports:
      - "6380:6379"
    volumes:
      - redis-slave1-data:/data
    networks:
      - redis-network
    depends_on:
      redis-master:
        condition: service_healthy

  redis-slave2:
    image: redis:7-alpine
    container_name: redis-slave2
    command: >
      redis-server
      --bind 0.0.0.0
      --port 6379
      --requirepass your-redis-password
      --masterauth your-redis-password
      --replicaof redis-master 6379
      --appendonly yes
    ports:
      - "6381:6379"
    volumes:
      - redis-slave2-data:/data
    networks:
      - redis-network
    depends_on:
      redis-master:
        condition: service_healthy

  sentinel1:
    image: redis:7-alpine
    container_name: redis-sentinel1
    command: >
      redis-sentinel
      --port 26379
      --sentinel monitor mymaster redis-master 6379 2
      --sentinel auth-pass mymaster your-redis-password
      --sentinel down-after-milliseconds mymaster 5000
      --sentinel parallel-syncs mymaster 1
      --sentinel failover-timeout mymaster 10000
    ports:
      - "26379:26379"
    networks:
      - redis-network
    depends_on:
      - redis-master
      - redis-slave1
      - redis-slave2

  sentinel2:
    image: redis:7-alpine
    container_name: redis-sentinel2
    command: >
      redis-sentinel
      --port 26379
      --sentinel monitor mymaster redis-master 6379 2
      --sentinel auth-pass mymaster your-redis-password
      --sentinel down-after-milliseconds mymaster 5000
      --sentinel parallel-syncs mymaster 1
      --sentinel failover-timeout mymaster 10000
    ports:
      - "26380:26379"
    networks:
      - redis-network
    depends_on:
      - redis-master
      - redis-slave1
      - redis-slave2

  sentinel3:
    image: redis:7-alpine
    container_name: redis-sentinel3
    command: >
      redis-sentinel
      --port 26379
      --sentinel monitor mymaster redis-master 6379 2
      --sentinel auth-pass mymaster your-redis-password
      --sentinel down-after-milliseconds mymaster 5000
      --sentinel parallel-syncs mymaster 1
      --sentinel failover-timeout mymaster 10000
    ports:
      - "26381:26379"
    networks:
      - redis-network
    depends_on:
      - redis-master
      - redis-slave1
      - redis-slave2

volumes:
  redis-master-data:
  redis-slave1-data:
  redis-slave2-data:

networks:
  redis-network:
    driver: bridge
```

启动 Redis 集群：

```bash
docker-compose -f docker-compose-redis.yml up -d
```

### 2. 部署 rqlite 集群

创建 `docker-compose-rqlite.yml`：

```yaml
version: '3.8'
services:
  rqlite1:
    image: rqlite/rqlite:latest
    container_name: bdopsflow-rqlite1
    ports:
      - "4001:4001"
      - "4002:4002"
    volumes:
      - rqlite1-data:/data
    command: >
      -node-id 1
      -http-addr 0.0.0.0:4001
      -raft-addr 0.0.0.0:4002
      -data-dir /data
      -bootstrap-expect 3
    networks:
      - rqlite-network
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:4001/status"]
      interval: 10s
      timeout: 5s
      retries: 3

  rqlite2:
    image: rqlite/rqlite:latest
    container_name: bdopsflow-rqlite2
    ports:
      - "4011:4001"
      - "4012:4002"
    volumes:
      - rqlite2-data:/data
    command: >
      -node-id 2
      -http-addr 0.0.0.0:4001
      -raft-addr 0.0.0.0:4002
      -data-dir /data
      -join http://rqlite1:4001
      -bootstrap-expect 3
    networks:
      - rqlite-network
    depends_on:
      rqlite1:
        condition: service_healthy

  rqlite3:
    image: rqlite/rqlite:latest
    container_name: bdopsflow-rqlite3
    ports:
      - "4021:4001"
      - "4022:4002"
    volumes:
      - rqlite3-data:/data
    command: >
      -node-id 3
      -http-addr 0.0.0.0:4001
      -raft-addr 0.0.0.0:4002
      -data-dir /data
      -join http://rqlite1:4001
      -bootstrap-expect 3
    networks:
      - rqlite-network
    depends_on:
      rqlite1:
        condition: service_healthy

volumes:
  rqlite1-data:
  rqlite2-data:
  rqlite3-data:

networks:
  rqlite-network:
    driver: bridge
```

启动 rqlite 集群：

```bash
docker-compose -f docker-compose-rqlite.yml up -d

# 等待集群启动
sleep 15

# 初始化数据库
curl -XPOST 'http://localhost:4001/db/load?pretty' \
    --data-binary @schema.sql
```

### 3. 配置和启动调度中心集群

创建配置文件 `config.prod.yaml`（节点 1）：

```yaml
app:
  node_id: "scheduler-1"
  http_port: "8080"
  grpc_port: "50051"

database:
  rqlite_addrs:
    - "http://192.168.1.100:4001"
    - "http://192.168.1.101:4001"
    - "http://192.168.1.102:4001"
  rqlite_user: ""
  rqlite_password: ""
  rqlite_tls: false

redis:
  mode: "sentinel"
  master_name: "mymaster"
  sentinel_addrs:
    - "192.168.1.100:26379"
    - "192.168.1.101:26379"
    - "192.168.1.102:26379"
  password: "your-redis-password"
  db: 0

jwt:
  secret: "your-production-secret-key-at-least-32-chars"
  expiry_hours: 24

log:
  level: "info"
  format: "json"
```

**在所有 3 个节点上重复此配置**，修改以下内容：

- `app.node_id`：分别为 `scheduler-1`, `scheduler-2`, `scheduler-3`
- `app.http_port` 和 `app.grpc_port`：如果在同一台机器上，需要使用不同端口

**编译并启动（每个节点）**：

```bash
cd scheduler
cp config.prod.yaml config.yaml

# 编译
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/scheduler ./cmd/main.go

# 启动
./bin/scheduler
```

**或者使用 systemd 管理**（创建 `/etc/systemd/system/bdopsflow-scheduler.service`）：

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

```bash
systemctl daemon-reload
systemctl enable bdopsflow-scheduler
systemctl start bdopsflow-scheduler
```

**验证集群**：

```bash
# 检查每个节点的状态
curl http://node1:8080/health
curl http://node2:8080/health
curl http://node3:8080/health
```

应该只有一个节点返回 `"is_leader": true`。

### 4. 配置和启动执行器集群

执行器配置文件示例 `config.prod.yaml`：

```yaml
app:
  executor_id: "executor-prod-1"
  executor_name: "executor-prod-1"
  capacity: 20

scheduler:
  # 多调度器模式
  addrs: "scheduler1:50051,scheduler2:50051,scheduler3:50051"
  timeout: 30

log:
  level: "info"
  format: "json"
```

**编译和启动**：

```bash
cd executor
cp config.prod.yaml config.yaml

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/executor ./cmd/main.go
./bin/executor
```

### 5. 部署前端

```bash
cd web

# 构建生产版本
npm install
npm run build

# 使用 Nginx 托管
cat > /etc/nginx/sites-available/bdopsflow << 'EOF'
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
EOF

# 启用站点
ln -s /etc/nginx/sites-available/bdopsflow /etc/nginx/sites-enabled/
nginx -s reload
```

---

## 使用 Makefile 快捷命令

项目根目录的 `Makefile` 提供了常用开发命令：

### 编译相关

```bash
# 编译所有组件
make build

# 编译调度中心
make scheduler-build

# 编译执行器
make executor-build

# 编译前端
make web-build

# 编译 Protobuf
make proto
```

### 运行相关

```bash
# 运行调度中心
make scheduler-run

# 运行执行器
make executor-run

# 运行前端开发服务器
make web-dev

# 使用 Docker 运行完整环境
make docker-up

# 停止 Docker 环境
make docker-down
```

### 测试相关

```bash
# 运行所有后端测试
make test

# 运行调度中心测试
make scheduler-test

# 运行执行器测试
make executor-test

# 运行前端测试
make web-test

# 运行测试并显示覆盖率
make coverage
```

### 依赖相关

```bash
# 安装 Protobuf 工具
make proto-deps

# 下载 Go 依赖
make deps

# 更新依赖
make update-deps
```

### 格式化和清理

```bash
# 格式化代码
make fmt

# 清理编译产物
make clean
```

---

## 开发调试技巧

### 1. 日志级别配置

编辑 `config.yaml`：

```yaml
log:
  level: "debug"  # debug, info, warn, error
  format: "text" # text, json
```

### 2. 本地开发时使用单个调度器

直接使用默认配置启动单个调度器即可，自动成为 leader。

### 3. 模拟调度器故障转移

```bash
# 找到当前 leader
curl http://localhost:8080/health

# 终止 leader 进程
pkill -f scheduler

# 等待 15-30 秒
sleep 20

# 检查其他节点是否成为新 leader
curl http://localhost:8081/health
```

### 4. 查看任务执行日志

在前端的任务执行详情页可以实时查看日志，或者通过 API：

```bash
# 获取任务执行的日志（需要登录获取 token）
curl -H "Authorization: Bearer <token>" \
    "http://localhost:8080/api/tasks/executions/<execution-id>/logs"
```

### 5. 使用开发模式启动前端（代理 API）

前端开发服务器已配置代理，无需修改 API 地址：

```bash
cd web
npm run dev
```

访问 http://localhost:5173，所有 `/api` 请求自动代理到 `http://localhost:8080/api`。

---

## 测试指南

### 运行后端测试

```bash
# 调度中心测试
cd scheduler
go test -v ./...

# 执行器测试
cd ../executor
go test -v ./...

# 或者从根目录运行所有测试
cd ..
make test
```

### 运行前端测试

```bash
cd web
npm run test
```

### 生成测试覆盖率报告

```bash
cd scheduler
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
open coverage.html
```

### 集成测试

可以通过以下步骤进行简单的集成测试：

1. 启动所有依赖服务
2. 启动调度中心和执行器
3. 使用前端创建并触发一个 Shell 任务
4. 检查任务是否成功执行
5. 查看任务执行日志

### 审计日志测试

```bash
# 运行审计中间件路由解析测试
cd scheduler
go test -v ./internal/middleware/ -run TestResolve

# 运行审计日志服务测试
go test -v ./internal/service/ -run TestAudit
```

---

## 添加审计日志埋点

当新增写操作接口时，需要在审计中间件中注册路由规则：

1. 如果是全新路径，在 `routeAuditRules` 中添加精确匹配规则：
```go
var routeAuditRules = map[string]auditRouteRule{
    "/api/new-feature": {Resource: "new_feature", Action: "create"},
}
```

2. 如果是已有路径前缀下的新资源，在 `routePrefixRules` 中添加前缀匹配：
```go
var routePrefixRules = []struct {
    Prefix   string
    Resource string
}{
    {"/api/new-feature/", "new_feature"},
}
```

3. 在 Handler 中通过 `c.Set()` 传递业务语义：
```go
func (h *Handler) Create(c *gin.Context) {
    // ... 业务逻辑
    c.Set("audit_resource_id", fmt.Sprintf("%d", id))
    c.Set("audit_resource_name", name)
}
```

4. 编写路由解析测试用例（参考 `middleware/audit_test.go`）

---

## 添加新数据源类型

1. 在 `scheduler/internal/datasource/driver/` 中创建新驱动文件，实现 Driver 接口：
```go
type NewDriver struct {
    config *DatasourceConfig
    conn   interface{}
}

func (d *NewDriver) Connect() error { ... }
func (d *NewDriver) Close() error { ... }
func (d *NewDriver) Ping() error { ... }
func (d *NewDriver) GetDatabases() ([]string, error) { ... }
func (d *NewDriver) GetTables(database string) ([]string, error) { ... }
func (d *NewDriver) GetColumns(database, table string) ([]ColumnInfo, error) { ... }
func (d *NewDriver) Query(database, sql string, limit int) (*QueryResult, error) { ... }
func (d *NewDriver) UseDatabase(database string) error { ... }
```

2. 在 `datasource/manager.go` 的 `CreateDriver` 方法中注册新类型

3. 在前端 `web/src/views/Datasource/DatasourceForm.vue` 中添加新类型表单模板

4. 在审计中间件路由规则中添加数据源相关路径

5. 编写驱动单元测试

---

## 常见问题

### 1. rqlite 连接失败

**问题**：`Failed to connect to rqlite`

**解决方案**：
- 确认 rqlite 服务正在运行
- 检查配置文件中的 `rqlite_addrs` 是否正确
- 查看日志获取详细错误信息

### 2. Redis 连接失败

**问题**：`Failed to connect to Redis`

**解决方案**：
- 确认 Redis 服务正在运行
- 检查 Redis 配置（密码、地址）
- 如果使用哨兵模式，确认 `sentinel_addrs` 配置正确

### 3. 执行器无法连接调度中心

**问题**：执行器无法注册或发送心跳

**解决方案**：
- 确认调度中心正在运行
- 检查 gRPC 端口是否开放
- 确认网络可达性
- 查看执行器日志获取详细信息

### 4. 任务执行失败但无日志

**问题**：任务执行失败，但任务日志为空

**解决方案**：
- 检查 `bdopsflow_task_executions` 表中的错误信息
- 查看执行器日志获取执行详情
- 确认执行器有足够的权限执行该任务

### 5. 主节点选举问题

**问题**：没有节点成为 leader，或多个节点同时认为自己是 leader

**解决方案**：
- 确认所有节点连接到同一个 Redis
- 检查 Redis 是否正常工作
- 查看调度中心日志中的选举相关信息

### 6. 审计日志未记录

**问题**：操作了接口但审计日志表中没有记录

**解决方案**：
- 确认操作是否为 POST/PUT/DELETE 请求（GET 请求不记录）
- 检查审计中间件是否已注册到路由组
- 查看调度中心日志是否有 "failed to write audit log" 错误
- 确认 `resolveAuditInfo()` 是否能正确解析该路由

### 7. 数据源连接失败

**问题**：创建数据源后测试连接失败

**解决方案**：
- 确认目标数据库服务正在运行
- 检查网络连通性和防火墙规则
- 确认用户名密码正确
- 对于 Hive/Kyuubi/Spark，确认 ZooKeeper 地址正确
- 对于 Rqlite 多节点模式，确认至少一个节点可用
- 查看调度中心日志获取详细错误信息

---

## 相关文档

- [架构设计](ARCHITECTURE.md) - 系统架构和设计理念
- [数据库设计](DATABASE.md) - 数据库表结构和配置说明
- [API 文档](API.md) - RESTful API 接口
- [任务日志系统](LOGGING.md) - 任务日志实现详解
- [核心功能参考](FEATURES.md) - 所有核心功能实现详解
