# BDopsFlow 分布式工作流调度平台 - 部署文档

## 重要提示

> **注意：** 本项目已完全从 SQLite 迁移至 **rqlite**，使用分布式 SQLite 提供高可用性和一致性。不再支持独立的 SQLite 数据库。

## 系统要求

### 开发环境
- Go 1.24+
- Redis 7.0+
- rqlite 8.0+ (分布式 SQLite)
- Node.js 18+ (前端)
- Docker & Docker Compose (可选)

### 生产环境
- Go 1.24+
- Redis 7.0+ (集群推荐)
- rqlite 8.0+ (3节点集群，最低1节点)
- Node.js 18+ (前端构建)
- Docker & Docker Compose
- Linux/amd64 或 Linux/arm64

## 一、开发环境快速开始

### 1.1 启动依赖服务

#### 1.1.1 启动 Redis

使用 Docker (推荐)：

```bash
docker run -d --name bdopsflow-redis -p 6379:6379 redis:7-alpine
```

或本地安装：

```bash
# macOS
brew install redis
brew services start redis

# Ubuntu/Debian
sudo apt update
sudo apt install redis-server
sudo systemctl start redis

# 验证
redis-cli ping
# 应返回 PONG
```

#### 1.1.2 启动 rqlite

使用 Docker (推荐)：

```bash
# 启动单个 rqlite 节点（开发环境）
docker run -d \
  --name bdopsflow-rqlite \
  -p 4001:4001 \
  -v bdopsflow-rqlite-data:/rqlite/file \
  rqlite/rqlite:latest
```

验证 rqlite 是否启动成功：

```bash
curl http://localhost:4001/status?pretty
# 应返回节点状态信息
```

### 1.2 初始化数据库

上传 schema.sql 到 rqlite 集群：

```bash
# 等待 rqlite 启动完成
sleep 3

# 初始化数据库 schema
curl -XPOST 'http://localhost:4001/db/load?pretty' --data-binary @deploy/schema.sql

# 验证表是否创建成功
curl -XPOST 'http://localhost:4001/db/query?pretty' \
  -d '["SELECT name FROM sqlite_master WHERE type=\"table\""]'
```

预期输出应包含以下表：
- users
- domains
- workflows
- tasks
- task_executions
- executors

### 1.3 编译并启动调度中心

```bash
cd /path/to/bdopsflow

# 编译调度中心
cd scheduler
go build -o bin/scheduler ./cmd/main.go

# 创建数据目录（如果不存在）
mkdir -p bin
```

启动调度中心（使用环境变量）：

```bash
# 配置环境变量
export HTTP_PORT=8080
export GRPC_PORT=50051
export RQLITE_DSN=http://localhost:4001
export REDIS_ADDR=localhost:6379
export REDIS_PASSWORD=

# 启动调度中心
./bin/scheduler
```

或者使用命令行参数（如已配置）：

```bash
./bin/scheduler
```

预期输出：
```
Connected to Redis successfully
Connected to rqlite successfully
gRPC server listening on port 50051
HTTP server listening on port 8080
```

### 1.4 启动执行器

打开新终端：

```bash
cd /path/to/bdopsflow

# 编译执行器
cd executor
go build -o bin/executor ./cmd/main.go

# 配置环境变量
export EXECUTOR_ID=executor-1
export EXECUTOR_NAME=executor-1
export SCHEDULER_ADDR=localhost:50051
export CAPACITY=10

# 启动执行器
./bin/executor
```

预期输出：
```
[Executor] Registered with scheduler successfully
[Executor] Subscribed to tasks
[Executor] Executor executor-1 started (capacity: 10)
```

### 1.5 启动前端

打开新终端：

```bash
cd /path/to/bdopsflow/web

# 安装依赖
npm install

# 开发模式
npm run dev
```

访问 http://localhost:5173 （Vite 默认端口）

### 1.6 初始化系统

首次启动后，需要创建管理员用户。可以通过注册接口或手动初始化：

```bash
# 使用 API 注册管理员
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "admin123",
    "role": "admin",
    "email": "admin@example.com"
  }'
```

验证用户创建成功：

```bash
# 登录获取 token
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "admin123"
  }'
```

现在就可以使用 admin / admin123 登录系统了！

### 1.7 一键启动脚本（推荐）

为了简化开发环境启动，可以创建启动脚本：

创建 `scripts/dev-start.sh`：

```bash
#!/bin/bash
set -e

echo "=== Starting BDopsFlow Development Environment ==="

# 1. 启动 Redis
echo "Starting Redis..."
docker start bdopsflow-redis 2>/dev/null || \
docker run -d --name bdopsflow-redis -p 6379:6379 redis:7-alpine

# 2. 启动 rqlite
echo "Starting rqlite..."
docker start bdopsflow-rqlite 2>/dev/null || \
docker run -d \
  --name bdopsflow-rqlite \
  -p 4001:4001 \
  -v bdopsflow-rqlite-data:/rqlite/file \
  rqlite/rqlite:latest

# 3. 初始化数据库（如果需要）
sleep 3
echo "Initializing database..."
curl -XPOST 'http://localhost:4001/db/load?pretty' --data-binary @deploy/schema.sql 2>/dev/null || true

# 4. 编译调度中心
echo "Building scheduler..."
cd scheduler
go build -o bin/scheduler ./cmd/main.go

# 5. 启动调度中心
echo "Starting scheduler..."
export HTTP_PORT=8080
export GRPC_PORT=50051
export RQLITE_DSN=http://localhost:4001
export REDIS_ADDR=localhost:6379
./bin/scheduler &

echo "=== Scheduler started on http://localhost:8080 ==="
echo "=== gRPC server started on port 50051 ==="
echo ""
echo "Now you can:"
echo "  1. Open http://localhost:5173 in browser"
echo "  2. Start executor: cd executor && go run ./cmd/main.go"
```

赋予执行权限并运行：

```bash
chmod +x scripts/dev-start.sh
./scripts/dev-start.sh
```

## 二、使用 Docker Compose 启动（开发环境）

### 2.1 启动所有服务

```bash
cd /path/to/bdopsflow/deploy

# 启动所有服务（开发环境）
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止服务
docker-compose down
```

服务地址：
- 前端：http://localhost:3000 (或 http://localhost:5173 开发模式)
- 调度中心 HTTP API：http://localhost:8080
- 调度中心 gRPC：localhost:50051
- Redis：localhost:6379
- rqlite：localhost:4001

### 2.2 初始化管理员账号

```bash
# 注册管理员
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "admin123",
    "role": "admin",
    "email": "admin@example.com"
  }'
```

## 三、生产环境部署

### 3.1 架构规划

BDopsFlow 生产推荐架构：

```
                        ┌───────────────┐
                        │   Nginx/LB    │
                        │ (反向代理)     │
                        └──────┬────────┘
                               │
                    ┌──────────┴──────────┐
                    │                     │
             ┌──────▼──────┐      ┌──────▼──────┐
             │ Scheduler-1 │      │ Scheduler-2 │
             └──────┬──────┘      └──────┬──────┘
                    └──────────┬──────────┘
                               │
        ┌──────────────────────┼──────────────────────┐
        │                      │                      │
   ┌────▼────┐           ┌─────▼─────┐        ┌─────▼─────┐
   │ Redis   │           │ rqlite-1  │        │ rqlite-2  │
   │ Cluster │           │ (Leader)  │        │ (Follower)│
   └────┬────┘           └─────┬─────┘        └─────┬─────┘
        │                      │                      │
        └──────────────────────┼──────────────────────┘
                               │
                        ┌──────▼──────┐
                        │  rqlite-3   │
                        │ (Follower)  │
                        └─────────────┘
                               │
                    ┌──────────┴──────────┐
                    │                     │
             ┌──────▼──────┐      ┌──────▼──────┐
             │  Executor-1 │      │  Executor-2 │
             └─────────────┘      └─────────────┘
```

### 3.2 部署 Redis

#### 单机 Redis

```bash
docker run -d \
  --name bdopsflow-redis \
  -p 6379:6379 \
  -v redis-data:/data \
  --restart unless-stopped \
  redis:7-alpine \
  redis-server --appendonly yes --requirepass "your_redis_password"
```

#### Redis 集群

参考 Redis 官方文档部署 3 主 3 从集群。

### 3.3 部署 rqlite 集群

#### 节点 1 (Leader)

```bash
docker run -d \
  --name rqlite-1 \
  -p 4001:4001 \
  -p 4002:4002 \
  -v rqlite-1-data:/rqlite/file \
  --restart unless-stopped \
  rqlite/rqlite:latest \
  -http-addr 0.0.0.0:4001 \
  -raft-addr 0.0.0.0:4002
```

#### 节点 2 (Follower)

```bash
docker run -d \
  --name rqlite-2 \
  -p 4011:4001 \
  -p 4012:4002 \
  -v rqlite-2-data:/rqlite/file \
  --restart unless-stopped \
  rqlite/rqlite:latest \
  -http-addr 0.0.0.0:4001 \
  -raft-addr 0.0.0.0:4002 \
  -join http://rqlite-1:4001
```

#### 节点 3 (Follower)

```bash
docker run -d \
  --name rqlite-3 \
  -p 4021:4001 \
  -p 4022:4002 \
  -v rqlite-3-data:/rqlite/file \
  --restart unless-stopped \
  rqlite/rqlite:latest \
  -http-addr 0.0.0.0:4001 \
  -raft-addr 0.0.0.0:4002 \
  -join http://rqlite-1:4001
```

#### 初始化数据库

```bash
# 上传 schema.sql 到集群
curl -XPOST 'http://localhost:4001/db/load?pretty' --data-binary @deploy/schema.sql

# 验证集群状态
curl http://localhost:4001/status?pretty | grep -A 5 "store"
```

### 3.4 部署调度中心集群

#### 调度中心 1

```bash
docker run -d \
  --name scheduler-1 \
  -p 8080:8080 \
  -p 50051:50051 \
  --restart unless-stopped \
  -e APP_HTTP_PORT=8080 \
  -e APP_GRPC_PORT=50051 \
  -e DATABASE_RQLITE_DSN=http://rqlite-1:4001,http://rqlite-2:4001,http://rqlite-3:4001 \
  -e REDIS_ADDR=redis:6379 \
  -e REDIS_PASSWORD=your_redis_password \
  -e JWT_SECRET=your_jwt_secret \
  bdopsflow/scheduler:latest
```

#### 调度中心 2

```bash
docker run -d \
  --name scheduler-2 \
  -p 8081:8080 \
  -p 50052:50051 \
  --restart unless-stopped \
  -e APP_HTTP_PORT=8080 \
  -e APP_GRPC_PORT=50051 \
  -e DATABASE_RQLITE_DSN=http://rqlite-1:4001,http://rqlite-2:4001,http://rqlite-3:4001 \
  -e REDIS_ADDR=redis:6379 \
  -e REDIS_PASSWORD=your_redis_password \
  -e JWT_SECRET=your_jwt_secret \
  bdopsflow/scheduler:latest
```

### 3.5 部署执行器集群

#### 执行器 1

```bash
docker run -d \
  --name executor-1 \
  --restart unless-stopped \
  -e APP_EXECUTOR_ID=executor-1 \
  -e APP_EXECUTOR_NAME=executor-1 \
  -e SCHEDULER_ADDR=scheduler-1:50051,scheduler-2:50051 \
  -e APP_CAPACITY=20 \
  bdopsflow/executor:latest
```

#### 执行器 2

```bash
docker run -d \
  --name executor-2 \
  --restart unless-stopped \
  -e APP_EXECUTOR_ID=executor-2 \
  -e APP_EXECUTOR_NAME=executor-2 \
  -e SCHEDULER_ADDR=scheduler-1:50051,scheduler-2:50051 \
  -e APP_CAPACITY=20 \
  bdopsflow/executor:latest
```

### 3.6 部署前端

```bash
docker run -d \
  --name bdopsflow-web \
  -p 80:80 \
  --restart unless-stopped \
  bdopsflow/web:latest
```

### 3.7 配置 Nginx 反向代理

```nginx
upstream scheduler_http {
    server scheduler-1:8080;
    server scheduler-2:8081;
}

server {
    listen 80;
    server_name bdopsflow.example.com;

    # 前端静态资源
    location / {
        proxy_pass http://bdopsflow-web;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # API 代理
    location /api/ {
        proxy_pass http://scheduler_http/api/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # gRPC 代理 (需要 Nginx 1.13+)
    location / {
        grpc_pass grpc://scheduler_http;
        grpc_set_header Host $host;
    }

    # 健康检查
    location /health {
        proxy_pass http://scheduler_http/health;
    }
}
```

## 四、Docker Compose 生产环境部署

创建 `docker-compose.prod.yml`：

```yaml
version: '3.8'

services:
  # Redis
  redis:
    image: redis:7-alpine
    command: redis-server --appendonly yes --requirepass "${REDIS_PASSWORD}"
    volumes:
      - redis-data:/data
    networks:
      - bdopsflow
    restart: unless-stopped

  # rqlite 集群
  rqlite-1:
    image: rqlite/rqlite:latest
    command: -http-addr 0.0.0.0:4001 -raft-addr 0.0.0.0:4002
    volumes:
      - rqlite-1-data:/rqlite/file
    networks:
      - bdopsflow
    restart: unless-stopped

  rqlite-2:
    image: rqlite/rqlite:latest
    command: -http-addr 0.0.0.0:4001 -raft-addr 0.0.0.0:4002 -join http://rqlite-1:4001
    volumes:
      - rqlite-2-data:/rqlite/file
    depends_on:
      - rqlite-1
    networks:
      - bdopsflow
    restart: unless-stopped

  rqlite-3:
    image: rqlite/rqlite:latest
    command: -http-addr 0.0.0.0:4001 -raft-addr 0.0.0.0:4002 -join http://rqlite-1:4001
    volumes:
      - rqlite-3-data:/rqlite/file
    depends_on:
      - rqlite-1
    networks:
      - bdopsflow
    restart: unless-stopped

  # 调度中心
  scheduler-1:
    build:
      context: ..
      dockerfile: deploy/Dockerfile.scheduler
    environment:
      - APP_HTTP_PORT=8080
      - APP_GRPC_PORT=50051
      - DATABASE_RQLITE_DSN=http://rqlite-1:4001,http://rqlite-2:4001,http://rqlite-3:4001
      - REDIS_ADDR=redis:6379
      - REDIS_PASSWORD=${REDIS_PASSWORD}
      - JWT_SECRET=${JWT_SECRET:-default-secret-change-in-production}
    depends_on:
      - redis
      - rqlite-1
    networks:
      - bdopsflow
    restart: unless-stopped

  scheduler-2:
    build:
      context: ..
      dockerfile: deploy/Dockerfile.scheduler
    environment:
      - APP_HTTP_PORT=8080
      - APP_GRPC_PORT=50051
      - DATABASE_RQLITE_DSN=http://rqlite-1:4001,http://rqlite-2:4001,http://rqlite-3:4001
      - REDIS_ADDR=redis:6379
      - REDIS_PASSWORD=${REDIS_PASSWORD}
      - JWT_SECRET=${JWT_SECRET:-default-secret-change-in-production}
    depends_on:
      - redis
      - rqlite-1
    networks:
      - bdopsflow
    restart: unless-stopped

  # 执行器
  executor-1:
    build:
      context: ..
      dockerfile: deploy/Dockerfile.executor
    environment:
      - APP_EXECUTOR_ID=executor-1
      - APP_EXECUTOR_NAME=executor-1
      - SCHEDULER_ADDR=scheduler-1:50051,scheduler-2:50051
      - APP_CAPACITY=20
    depends_on:
      - scheduler-1
    networks:
      - bdopsflow
    restart: unless-stopped

  executor-2:
    build:
      context: ..
      dockerfile: deploy/Dockerfile.executor
    environment:
      - APP_EXECUTOR_ID=executor-2
      - APP_EXECUTOR_NAME=executor-2
      - SCHEDULER_ADDR=scheduler-1:50051,scheduler-2:50051
      - APP_CAPACITY=20
    depends_on:
      - scheduler-1
    networks:
      - bdopsflow
    restart: unless-stopped

  # 前端
  web:
    build:
      context: ..
      dockerfile: deploy/Dockerfile.web
    ports:
      - "80:80"
    depends_on:
      - scheduler-1
    networks:
      - bdopsflow
    restart: unless-stopped

networks:
  bdopsflow:
    driver: bridge

volumes:
  redis-data:
  rqlite-1-data:
  rqlite-2-data:
  rqlite-3-data:
```

启动生产环境：

```bash
# 设置环境变量
export REDIS_PASSWORD=your_strong_password

# 启动服务
docker-compose -f docker-compose.prod.yml up -d

# 初始化数据库
sleep 10
curl -XPOST 'http://localhost:4001/db/load?pretty' --data-binary @deploy/schema.sql
```

## 五、配置文件详解

### 5.1 配置方式

系统支持三种配置方式，优先级从高到低为：

1. **环境变量**（最高优先级）
2. **YAML 配置文件**（config.yaml）
3. **默认值**（最低优先级）

### 5.2 YAML 配置文件（推荐）

系统支持使用 `config.yaml` 配置文件管理配置，这种方式更易于维护和版本控制。

#### 5.2.1 配置文件路径

系统会按以下顺序查找配置文件：

1. **命令行指定**（最高优先级）：`-config /path/to/config.yaml`
2. **当前目录**：`config.yaml` 或 `config.yml`
3. **可执行文件同目录**：`$(dirname $(which scheduler))/config.yaml`
4. **系统目录**：`/etc/bdopsflow/config.yaml`

#### 5.2.2 调度中心配置

复制示例配置文件：

```bash
cd scheduler
cp config.yaml.example config.yaml
```

编辑 `config.yaml`：

```yaml
# 调度中心配置
app:
  http_port: "8080"
  grpc_port: "50051"

# 数据库配置
database:
  rqlite_dsn: "http://localhost:4001"

# Redis 配置
redis:
  addr: "localhost:6379"
  password: ""
  db: 0

# JWT 配置
jwt:
  secret: "your-secret-key-change-in-production"
  expiry_hours: 24

# 日志配置
log:
  level: "info"
  format: "json"
```

#### 5.2.3 执行器配置

复制示例配置文件：

```bash
cd executor
cp config.yaml.example config.yaml
```

编辑 `config.yaml`：

```yaml
# 执行器配置
app:
  executor_id: "executor-1"
  executor_name: "executor-1"
  capacity: 10

# Scheduler gRPC 地址
scheduler:
  addr: "localhost:50051"
  timeout: 30

# 日志配置
log:
  level: "info"
  format: "json"
```

### 5.3 命令行参数

#### 调度中心

```bash
# 使用默认配置文件（config.yaml）
./scheduler

# 指定配置文件路径
./scheduler -config /path/to/config.yaml

# 查看帮助
./scheduler -h
```

参数说明：
- `-config string`：配置文件路径（可选）

#### 执行器

```bash
# 使用默认配置文件（config.yaml）
./executor

# 指定配置文件路径
./executor -config /path/to/config.yaml

# 查看帮助
./executor -h
```

参数说明：
- `-config string`：配置文件路径（可选）

### 5.4 环境变量配置

#### 5.4.1 调度中心环境变量

| 环境变量 | 默认值 | 说明 | 必须 |
|---------|-------|------|------|
| APP_HTTP_PORT | 8080 | HTTP API 服务端口 | 否 |
| APP_GRPC_PORT | 50051 | gRPC 服务端口（与执行器通信） | 否 |
| DATABASE_RQLITE_DSN | http://localhost:4001 | rqlite HTTP API 地址，生产环境多节点用逗号分隔 | 是 |
| REDIS_ADDR | localhost:6379 | Redis 连接地址 | 否 |
| REDIS_PASSWORD | (空) | Redis 密码 | 否 |
| REDIS_DB | 0 | Redis 数据库编号 | 否 |
| JWT_SECRET | (空) | JWT 密钥 | 否 |
| JWT_EXPIRY_HOURS | 24 | JWT 过期时间（小时） | 否 |
| LOG_LEVEL | info | 日志级别 | 否 |
| LOG_FORMAT | json | 日志格式 | 否 |

**开发环境配置：**
```bash
export APP_HTTP_PORT=8080
export APP_GRPC_PORT=50051
export DATABASE_RQLITE_DSN=http://localhost:4001
export REDIS_ADDR=localhost:6379
export REDIS_PASSWORD=
```

**生产环境配置：**
```bash
export APP_HTTP_PORT=8080
export APP_GRPC_PORT=50051
export DATABASE_RQLITE_DSN=http://rqlite-1:4001,http://rqlite-2:4001,http://rqlite-3:4001
export REDIS_ADDR=redis-cluster:6379
export REDIS_PASSWORD=your_strong_password
export JWT_SECRET=your-production-secret-key
```

#### 5.3.2 执行器环境变量

| 环境变量 | 默认值 | 说明 | 必须 |
|---------|-------|------|------|
| APP_EXECUTOR_ID | executor-1 | 执行器唯一标识 | 否 |
| APP_EXECUTOR_NAME | executor-1 | 执行器显示名称 | 否 |
| APP_CAPACITY | 10 | 最大并发执行任务数 | 否 |
| SCHEDULER_ADDR | localhost:50051 | 调度中心 gRPC 地址，多节点用逗号分隔 | 否 |
| SCHEDULER_TIMEOUT | 30 | gRPC 连接超时（秒） | 否 |
| LOG_LEVEL | info | 日志级别 | 否 |
| LOG_FORMAT | json | 日志格式 | 否 |

**开发环境配置：**
```bash
export APP_EXECUTOR_ID=executor-1
export APP_EXECUTOR_NAME=executor-1
export SCHEDULER_ADDR=localhost:50051
export APP_CAPACITY=10
```

**生产环境配置：**
```bash
export APP_EXECUTOR_ID=executor-prod-1
export APP_EXECUTOR_NAME=生产执行器-1
export SCHEDULER_ADDR=scheduler-1:50051,scheduler-2:50051
export APP_CAPACITY=50
```

### 5.4 配置优先级与混合使用

配置文件和环境变量可以混合使用，环境变量会覆盖配置文件中的值。

**示例：**

1. `config.yaml` 中设置基础配置
2. 通过环境变量覆盖特定配置

```bash
# config.yaml 中设置
jwt:
  secret: "default-secret"

# 通过环境变量覆盖
export JWT_SECRET="my-production-secret"
```

### 5.5 前端配置

前端配置通过 Vite 环境变量设置：

```bash
# .env.development
VITE_API_BASE_URL=http://localhost:8080

# .env.production
VITE_API_BASE_URL=https://api.bdopsflow.example.com
```

## 六、监控与日志

### 6.1 健康检查

```bash
# 调度中心健康检查
curl http://localhost:8080/health
# 预期: {"status":"ok"}

# rqlite 健康检查
curl http://localhost:4001/status?pretty
# 应返回节点状态信息

# Redis 健康检查
redis-cli -h localhost ping
# 应返回 PONG
```

### 6.2 日志查看

```bash
# Docker 日志
docker logs -f scheduler-1
docker logs -f executor-1

# 查看特定时间段的日志
docker logs --since 1h scheduler-1

# rqlite 日志
docker logs -f rqlite-1
```

### 6.3 指标采集

可通过 Prometheus + Grafana 进行监控（待实现）。

## 七、故障排查

### 7.1 执行器无法连接调度中心

**问题：** 执行器启动后无法注册到调度中心

**排查步骤：**
```bash
# 1. 检查调度中心是否正常运行
curl http://localhost:8080/health

# 2. 检查 gRPC 端口是否可访问
telnet localhost 50051

# 3. 检查网络连通性
ping scheduler-1

# 4. 检查防火墙规则
sudo iptables -L -n
```

**解决方案：**
- 确认调度中心 gRPC 服务已启动
- 检查防火墙是否开放 50051 端口
- 确认网络策略允许容器间通信

### 7.2 任务执行失败

**问题：** 任务状态一直是 pending 或 failed

**排查步骤：**
```bash
# 1. 检查执行器是否在线
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/executors

# 2. 检查任务配置是否正确
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/tasks/$TASK_ID

# 3. 查看执行器日志
docker logs -f executor-1
```

**常见原因：**
- 任务配置 JSON 格式错误
- HTTP 任务目标地址不可达
- Shell 任务命令语法错误
- 执行器容量已满

### 7.3 数据库连接失败

**问题：** 调度中心启动时报数据库连接错误

**排查步骤：**
```bash
# 1. 检查 rqlite 集群状态
curl http://localhost:4001/status?pretty

# 2. 检查 rqlite 日志
docker logs rqlite-1

# 3. 验证 schema 是否存在
curl -XPOST 'http://localhost:4001/db/query?pretty' \
  -d '["SELECT name FROM sqlite_master WHERE type=\"table\""]'
```

**常见问题：**
- rqlite 节点未启动
- 网络连接问题
- schema 未初始化

## 八、性能调优

### 8.1 调度中心优化

```bash
# 增加 Go 最大并发数
export GOMAXPROCS=8

# 使用连接池（代码内配置）
# 数据库连接池大小
# Redis 连接池大小
```

### 8.2 执行器优化

```bash
# 增加执行器容量
export CAPACITY=100

# 按任务类型分配不同执行器
# EXECUTOR_TYPE=http / shell
```

### 8.3 rqlite 集群优化

- 确保节点之间网络延迟低
- 使用 SSD 存储提高 I/O 性能
- 合理配置快照和压缩策略

## 九、安全建议

1. **修改默认密码：** 所有默认密码必须修改
2. **启用 HTTPS：** 生产环境使用 SSL/TLS
3. **Redis 认证：** 配置强密码
4. **网络隔离：** 使用私有网络部署内部服务
5. **定期备份：** 定期备份 rqlite 和 Redis 数据
6. **日志审计：** 收集并审计操作日志
7. **权限最小化：** 遵循 RBAC 最小权限原则

## 十、备份与恢复

### 10.1 备份 rqlite 数据

```bash
# 备份
curl -XGET 'http://localhost:4001/db/backup' -o backup-$(date +%Y%m%d).sqlite3

# 恢复
curl -XPOST 'http://localhost:4001/db/load?pretty' --data-binary @backup-20240101.sqlite3
```

### 10.2 备份 Redis 数据

```bash
# 备份
redis-cli -a your_password BGSAVE

# 从 dump.rdb 恢复
# 重启 Redis 时会自动加载
```

## 十一、升级指南

### 11.1 版本升级流程

1. 备份数据库
2. 逐个升级调度中心节点
3. 逐个升级执行器节点
4. 验证服务正常
5. 升级前端

### 11.2 零停机升级

使用滚动升级策略，确保至少有一个调度中心和执行器始终在线。

---

## 十二、从 SQLite 迁移到 rqlite

### 12.1 迁移说明

本项目已于 2026 年 5 月从独立的 SQLite 数据库完全迁移至 **rqlite** 分布式数据库，主要改进包括：

- ✅ **高可用性** - 支持多节点集群部署
- ✅ **数据一致性** - 使用 Raft 共识协议
- ✅ **水平扩展** - 可动态添加/移除节点
- ✅ **HTTP API** - 简化的数据库访问方式
- ✅ **向后兼容** - 保留原有的表结构和 SQL 语法

### 12.2 数据迁移（如需）

如果你有旧版本的 SQLite 数据需要迁移：

```bash
# 1. 备份旧 SQLite 数据库
cp old_data.db backup.sqlite3

# 2. 导出为 SQL
sqlite3 old_data.db .dump > backup.sql

# 3. 上传到 rqlite
curl -XPOST 'http://localhost:4001/db/load?pretty' --data-binary @backup.sql
```

### 12.3 开发注意事项

1. **不再支持 `database/sql` 驱动** - 所有数据库操作通过 rqlite HTTP API 完成
2. **测试环境** - service 包的数据库测试默认跳过，需要真实 rqlite 服务器运行完整测试
3. **依赖** - 项目依赖 `github.com/rqlite/gorqlite` 包，不再依赖 `github.com/mattn/go-sqlite3`

---

## 技术支持

如遇到问题，请查看项目 Issue 或提交 Bug 报告。
