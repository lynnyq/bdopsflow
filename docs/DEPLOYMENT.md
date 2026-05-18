# BDopsFlow 部署文档

本文档详细描述了 BDopsFlow 分布式工作流调度平台的部署方式，包括开发环境、生产环境的部署步骤和配置说明。

## 目录

- [环境要求](#环境要求)
- [开发环境部署](#开发环境部署)
- [生产环境部署](#生产环境部署)
- [Docker 部署](#docker-部署)
- [Kubernetes 部署](#kubernetes-部署)
- [配置详解](#配置详解)
- [监控与运维](#监控与运维)

---

## 环境要求

### 基础组件

| 组件 | 版本要求 | 说明 |
|------|----------|------|
| Go | 1.24+ | 后端运行环境 |
| Node.js | 18+ | 前端构建环境 |
| Redis | 7.0+ | 分布式锁、缓存、主节点选举 |
| rqlite | 8.0+ | 分布式数据库 |
| Docker | 20.0+ | 容器化部署（可选） |

### 硬件要求

**调度中心**：
- CPU：2 核+
- 内存：4 GB+
- 磁盘：20 GB+

**执行器**：
- CPU：2 核+
- 内存：2 GB+
- 磁盘：10 GB+

**Redis**：
- CPU：1 核+
- 内存：2 GB+
- 磁盘：10 GB+

**rqlite**：
- CPU：1 核+
- 内存：2 GB+
- 磁盘：50 GB+

---

## 开发环境部署

### 1. 安装依赖

```bash
# 安装 Go（macOS）
brew install go

# 安装 Node.js（macOS）
brew install node

# 安装 Redis（macOS）
brew install redis

# 启动 Redis
brew services start redis
```

### 2. 启动 rqlite（单节点）

```bash
# 使用 Docker 启动 rqlite
docker run -d --name bdopsflow-rqlite \
  -p 4001:4001 \
  -p 4002:4002 \
  rqlite/rqlite:latest

# 等待启动
sleep 3

# 初始化数据库
curl -XPOST 'http://localhost:4001/db/load?pretty' \
  --data-binary @deploy/schema.sql
```

### 3. 编译并启动调度中心

```bash
cd scheduler

# 编译
go build -o bin/scheduler ./cmd/main.go

# 复制配置文件
cp config.yaml.example config.yaml

# 启动
./bin/scheduler
```

预期输出：
```
scheduler starting http_port=8080 grpc_port=50051 config_file=config.yaml redis_mode=single rqlite_tls=false rqlite_has_auth=false
using Redis single mode addr=localhost:6379
connected to Redis
attempting to connect to rqlite addr=http://localhost:4001 index=0
successfully connected to rqlite addr=http://localhost:4001
gRPC server listening on port 50051
HTTP server listening on port 8080
```

### 4. 编译并启动执行器

```bash
cd executor

# 编译
go build -o bin/executor ./cmd/main.go

# 复制配置文件
cp config.yaml.example config.yaml

# 启动
./bin/executor
```

### 5. 启动前端

```bash
cd web

# 安装依赖
npm install

# 启动开发服务器
npm run dev
```

访问 http://localhost:5173

---

## 生产环境部署

### 1. 部署 Redis（哨兵模式）

推荐使用 Redis Sentinel 实现高可用，生产环境至少部署 3 个哨兵节点。

#### 完整的 Docker Compose 部署（3 哨兵模式）

创建 `docker-compose-redis.yml`：
```yaml
version: '3.8'
services:
  # Redis 主节点
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

  # Redis 从节点 1
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

  # Redis 从节点 2
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

  # 哨兵节点 1
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

  # 哨兵节点 2
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

  # 哨兵节点 3
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

#### Scheduler 配置（使用 Redis Sentinel）

```yaml
redis:
  mode: "sentinel"
  master_name: "mymaster"
  sentinel_addrs:
    - "sentinel1:26379"
    - "sentinel2:26379"
    - "sentinel3:26379"
  password: "your-redis-password"
  sentinel_password: ""
  db: 0
```

### 2. 部署 rqlite（3 节点集群）

rqlite 使用 Raft 协议实现高可用，生产环境至少 3 节点部署。支持密码认证和 TLS 加密。

#### 准备认证配置

创建 `auth.json`：
```json
[
  {
    "username": "admin",
    "password": "your-rqlite-password",
    "perms": ["all"]
  }
]
```

#### 完整的 Docker Compose 部署（3 节点带认证）

创建 `docker-compose-rqlite.yml`：
```yaml
version: '3.8'
services:
  # rqlite 节点 1（引导节点）
  rqlite1:
    image: rqlite/rqlite:latest
    container_name: rqlite1
    ports:
      - "4001:4001"
      - "4002:4002"
    volumes:
      - rqlite1-data:/data
      - ./auth.json:/auth.json
    command: >
      -node-id 1
      -http-addr 0.0.0.0:4001
      -raft-addr 0.0.0.0:4002
      -data-dir /data
      -auth /auth.json
      -bootstrap-expect 3
    networks:
      - rqlite-network
    healthcheck:
      test: ["CMD", "curl", "-f", "-u", "admin:your-rqlite-password", "http://localhost:4001/status"]
      interval: 10s
      timeout: 5s
      retries: 3

  # rqlite 节点 2
  rqlite2:
    image: rqlite/rqlite:latest
    container_name: rqlite2
    ports:
      - "4011:4001"
      - "4012:4002"
    volumes:
      - rqlite2-data:/data
      - ./auth.json:/auth.json
    command: >
      -node-id 2
      -http-addr 0.0.0.0:4001
      -raft-addr 0.0.0.0:4002
      -data-dir /data
      -auth /auth.json
      -join http://admin:your-rqlite-password@rqlite1:4001
      -bootstrap-expect 3
    networks:
      - rqlite-network
    depends_on:
      rqlite1:
        condition: service_healthy

  # rqlite 节点 3
  rqlite3:
    image: rqlite/rqlite:latest
    container_name: rqlite3
    ports:
      - "4021:4001"
      - "4022:4002"
    volumes:
      - rqlite3-data:/data
      - ./auth.json:/auth.json
    command: >
      -node-id 3
      -http-addr 0.0.0.0:4001
      -raft-addr 0.0.0.0:4002
      -data-dir /data
      -auth /auth.json
      -join http://admin:your-rqlite-password@rqlite1:4001
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

# 等待集群启动后初始化数据库
sleep 15
curl -XPOST -u admin:your-rqlite-password 'http://localhost:4001/db/load?pretty' \
  --data-binary @deploy/schema.sql
```

#### Scheduler 配置（连接 rqlite 3 节点集群）

```yaml
database:
  # 配置所有 rqlite 节点地址，客户端会自动故障转移
  rqlite_addrs:
    - "http://rqlite1:4001"
    - "http://rqlite2:4001"
    - "http://rqlite3:4001"
  # rqlite 认证信息
  rqlite_user: "admin"
  rqlite_password: "your-rqlite-password"
  # 是否使用 TLS 连接（需要 rqlite 服务端配置 TLS 证书）
  rqlite_tls: false
```

### 3. 部署调度中心集群（3 节点高可用）

调度中心采用 **主节点选举** 机制，使用 Redis 实现分布式锁，确保同一时间只有一个主节点执行任务调度，其他节点作为备用节点提供 API 和 gRPC 服务。

#### 3.1 配置说明

**节点配置** (config.yaml)：
```yaml
app:
  # 节点唯一标识，可选配置。如果不配置会自动生成 UUID
  # 也可以通过环境变量 APP_NODE_ID 设置
  node_id: "scheduler-1"
  
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
  # 必须使用 Redis，用于主节点选举
  mode: "sentinel"
  master_name: "mymaster"
  sentinel_addrs:
    - "sentinel1:26379"
    - "sentinel2:26379"
    - "sentinel3:26379"
  password: "your-redis-password"
  db: 0

jwt:
  secret: "your-production-secret-key-at-least-32-chars"
  expiry_hours: 24

log:
  level: "info"
  format: "json"
```

**主节点选举机制**：
- 使用 Redis SETNX 实现分布式锁
- 锁的 TTL 为 15 秒，主节点每 5 秒续期一次
- 如果主节点故障，备用节点会在 15 秒内自动选举出新的主节点
- 只有主节点会启动 Cron 调度器并执行任务调度
- 所有节点都可以提供 HTTP API 和 gRPC 服务

#### 3.2 编译并部署 3 个调度节点

```bash
cd scheduler

# 编译
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/scheduler ./cmd/main.go

# 复制二进制文件到 3 台服务器
# scp bin/scheduler user@scheduler1:/opt/bdopsflow/scheduler/
# scp bin/scheduler user@scheduler2:/opt/bdopsflow/scheduler/
# scp bin/scheduler user@scheduler3:/opt/bdopsflow/scheduler/
```

#### 3.3 在每个节点上创建配置文件

**scheduler1** (config.yaml):
```yaml
app:
  node_id: "scheduler-1"
  http_port: "8080"
  grpc_port: "50051"
# ... 其他配置同上
```

**scheduler2** (config.yaml):
```yaml
app:
  node_id: "scheduler-2"
  http_port: "8080"
  grpc_port: "50051"
# ... 其他配置同上
```

**scheduler3** (config.yaml):
```yaml
app:
  node_id: "scheduler-3"
  http_port: "8080"
  grpc_port: "50051"
# ... 其他配置同上
```

#### 3.4 使用 systemd 管理每个节点

在每个调度节点上创建 systemd 服务文件：
```ini
# /etc/systemd/system/bdopsflow-scheduler.service
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

#### 3.5 配置负载均衡

在所有调度节点前部署负载均衡器（如 Nginx、HAProxy）：

**Nginx 配置示例**：
```nginx
upstream scheduler_http {
    server scheduler1:8080;
    server scheduler2:8080;
    server scheduler3:8080;
    keepalive 32;
}

upstream scheduler_grpc {
    server scheduler1:50051;
    server scheduler2:50051;
    server scheduler3:50051;
    keepalive 32;
}

server {
    listen 80;
    server_name bdopsflow.example.com;

    # HTTP API
    location / {
        proxy_pass http://scheduler_http;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }

    # gRPC
    location /scheduler.SchedulerService/ {
        grpc_pass grpc://scheduler_grpc;
    }
}
```

#### 3.6 健康检查与监控

每个调度节点的健康检查接口会返回节点状态和是否为主节点：
```bash
curl http://scheduler1:8080/health
```

预期响应：
```json
{
  "status": "ok",
  "node_id": "scheduler-1",
  "is_leader": true,
  "checks": {
    "redis": "ok",
    "database": "ok",
    "tables": "ok",
    "scheduler": "ok"
  }
}
```

#### 3.7 验证主节点选举

1. 查看三个节点的日志，确认只有一个节点显示 "becoming leader"
2. 停止主节点进程，等待 15-20 秒，检查备用节点是否成为新的主节点
3. 重新启动旧主节点，确认它会作为备用节点加入集群

**使用 systemd 管理**：
```ini
# /etc/systemd/system/bdopsflow-scheduler.service
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

### 4. 部署执行器

**编译**：
```bash
cd executor
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/executor ./cmd/main.go
```

**配置** (config.yaml)：
```yaml
app:
  executor_id: "executor-prod-1"
  executor_name: "executor-prod-1"
  capacity: 20

scheduler:
  addr: "scheduler-lb:50051"
  timeout: 30

log:
  level: "info"
  format: "json"
```

**使用 systemd 管理**：
```ini
# /etc/systemd/system/bdopsflow-executor.service
[Unit]
Description=BDopsFlow Executor
After=network.target

[Service]
Type=simple
User=bdopsflow
WorkingDirectory=/opt/bdopsflow/executor
ExecStart=/opt/bdopsflow/executor/bin/executor
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
systemctl daemon-reload
systemctl enable bdopsflow-executor
systemctl start bdopsflow-executor
```

### 5. 部署前端

**构建**：
```bash
cd web
npm install
npm run build
```

**使用 Nginx 托管**：
```nginx
# /etc/nginx/sites-available/bdopsflow
server {
    listen 80;
    server_name bdopsflow.example.com;

    root /opt/bdopsflow/web/dist;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    location /api {
        proxy_pass http://scheduler-lb:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

---

## Docker 部署

### 1. Docker Compose 开发环境部署（单节点）

创建 `docker-compose-dev.yml`：
```yaml
version: '3.8'

services:
  redis:
    image: redis:7-alpine
    container_name: bdopsflow-redis
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
    command: redis-server --appendonly yes
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 3

  rqlite:
    image: rqlite/rqlite:latest
    container_name: bdopsflow-rqlite
    ports:
      - "4001:4001"
      - "4002:4002"
    volumes:
      - rqlite-data:/data
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:4001/status"]
      interval: 10s
      timeout: 5s
      retries: 3

  scheduler:
    build:
      context: .
      dockerfile: deploy/Dockerfile.scheduler
    container_name: bdopsflow-scheduler
    ports:
      - "8080:8080"
      - "50051:50051"
    depends_on:
      redis:
        condition: service_healthy
      rqlite:
        condition: service_healthy
    environment:
      - APP_HTTP_PORT=8080
      - APP_GRPC_PORT=50051
      - DATABASE_RQLITE_ADDRS=http://rqlite:4001
      - REDIS_MODE=single
      - REDIS_ADDR=redis:6379
      - JWT_SECRET=your-secret-key

  executor:
    build:
      context: .
      dockerfile: deploy/Dockerfile.executor
    container_name: bdopsflow-executor
    depends_on:
      - scheduler
    environment:
      - APP_EXECUTOR_ID=executor-1
      - APP_EXECUTOR_NAME=executor-1
      - APP_CAPACITY=10
      - SCHEDULER_ADDR=scheduler:50051

  web:
    build:
      context: .
      dockerfile: deploy/Dockerfile.web
    container_name: bdopsflow-web
    ports:
      - "3000:80"
    depends_on:
      - scheduler

volumes:
  redis-data:
  rqlite-data:
```

### 2. Docker Compose 生产环境部署（3 节点调度集群）

创建 `docker-compose-prod.yml`：
```yaml
version: '3.8'

services:
  # Redis（生产环境建议使用 Sentinel 模式）
  redis:
    image: redis:7-alpine
    container_name: bdopsflow-redis
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
    command: redis-server --appendonly yes
    networks:
      - bdopsflow
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 3

  # rqlite 集群（3 节点）
  rqlite1:
    image: rqlite/rqlite:latest
    container_name: bdopsflow-rqlite1
    ports:
      - "4001:4001"
    command: rqlited -http-addr 0.0.0.0:4001 -raft-addr 0.0.0.0:4002 -data-dir /data
    volumes:
      - rqlite1-data:/data
    networks:
      - bdopsflow
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:4001/status"]
      interval: 10s
      timeout: 5s
      retries: 3

  rqlite2:
    image: rqlite/rqlite:latest
    container_name: bdopsflow-rqlite2
    ports:
      - "4002:4001"
    command: rqlited -http-addr 0.0.0.0:4001 -raft-addr 0.0.0.0:4002 -data-dir /data -join http://rqlite1:4001
    volumes:
      - rqlite2-data:/data
    networks:
      - bdopsflow
    depends_on:
      rqlite1:
        condition: service_healthy

  rqlite3:
    image: rqlite/rqlite:latest
    container_name: bdopsflow-rqlite3
    ports:
      - "4003:4001"
    command: rqlited -http-addr 0.0.0.0:4001 -raft-addr 0.0.0.0:4002 -data-dir /data -join http://rqlite1:4001
    volumes:
      - rqlite3-data:/data
    networks:
      - bdopsflow
    depends_on:
      rqlite1:
        condition: service_healthy

  # 调度器集群（3 节点）
  scheduler1:
    build:
      context: .
      dockerfile: deploy/Dockerfile.scheduler
    container_name: bdopsflow-scheduler1
    ports:
      - "8080:8080"
      - "50051:50051"
    depends_on:
      redis:
        condition: service_healthy
      rqlite1:
        condition: service_healthy
    networks:
      - bdopsflow
    environment:
      - APP_NODE_ID=scheduler1
      - APP_HTTP_PORT=8080
      - APP_GRPC_PORT=50051
      - DATABASE_RQLITE_ADDRS=http://rqlite1:4001,http://rqlite2:4001,http://rqlite3:4001
      - REDIS_MODE=single
      - REDIS_ADDR=redis:6379
      - JWT_SECRET=your-secret-key

  scheduler2:
    build:
      context: .
      dockerfile: deploy/Dockerfile.scheduler
    container_name: bdopsflow-scheduler2
    ports:
      - "8081:8080"
      - "50052:50051"
    depends_on:
      redis:
        condition: service_healthy
      rqlite1:
        condition: service_healthy
    networks:
      - bdopsflow
    environment:
      - APP_NODE_ID=scheduler2
      - APP_HTTP_PORT=8080
      - APP_GRPC_PORT=50051
      - DATABASE_RQLITE_ADDRS=http://rqlite1:4001,http://rqlite2:4001,http://rqlite3:4001
      - REDIS_MODE=single
      - REDIS_ADDR=redis:6379
      - JWT_SECRET=your-secret-key

  scheduler3:
    build:
      context: .
      dockerfile: deploy/Dockerfile.scheduler
    container_name: bdopsflow-scheduler3
    ports:
      - "8082:8080"
      - "50053:50051"
    depends_on:
      redis:
        condition: service_healthy
      rqlite1:
        condition: service_healthy
    networks:
      - bdopsflow
    environment:
      - APP_NODE_ID=scheduler3
      - APP_HTTP_PORT=8080
      - APP_GRPC_PORT=50051
      - DATABASE_RQLITE_ADDRS=http://rqlite1:4001,http://rqlite2:4001,http://rqlite3:4001
      - REDIS_MODE=single
      - REDIS_ADDR=redis:6379
      - JWT_SECRET=your-secret-key

  # 执行器
  executor:
    build:
      context: .
      dockerfile: deploy/Dockerfile.executor
    container_name: bdopsflow-executor
    depends_on:
      - scheduler1
    networks:
      - bdopsflow
    environment:
      - APP_EXECUTOR_ID=executor-1
      - APP_EXECUTOR_NAME=executor-1
      - APP_CAPACITY=10
      # 配置多个调度器地址用于故障转移
      - SCHEDULER_ADDR=scheduler1:50051

  # 前端（连接到 scheduler1）
  web:
    build:
      context: .
      dockerfile: deploy/Dockerfile.web
    container_name: bdopsflow-web
    ports:
      - "3000:80"
    depends_on:
      - scheduler1
    networks:
      - bdopsflow

volumes:
  redis-data:
  rqlite1-data:
  rqlite2-data:
  rqlite3-data:

networks:
  bdopsflow:
    driver: bridge
```

### 3. 启动服务

**开发环境（单节点）**：
```bash
# 启动所有服务
docker-compose -f docker-compose-dev.yml up -d

# 查看日志
docker-compose -f docker-compose-dev.yml logs -f

# 初始化数据库（首次启动）
sleep 10
curl -XPOST 'http://localhost:4001/db/load?pretty' \
  --data-binary @deploy/schema.sql
```

**生产环境（3 节点调度集群）**：
```bash
# 启动所有服务
docker-compose -f docker-compose-prod.yml up -d

# 查看日志
docker-compose -f docker-compose-prod.yml logs -f

# 初始化数据库（首次启动）
sleep 15
curl -XPOST 'http://localhost:4001/db/load?pretty' \
  --data-binary @deploy/schema.sql
```

### 4. 验证调度集群

检查每个调度节点的状态：
```bash
# 检查 scheduler1（应该是 leader）
curl http://localhost:8080/health

# 检查 scheduler2（应该是 follower）
curl http://localhost:8081/health

# 检查 scheduler3（应该是 follower）
curl http://localhost:8082/health
```

测试主节点故障转移：
```bash
# 停止主节点
docker stop bdopsflow-scheduler1

# 等待 20 秒后检查剩余节点
sleep 20
curl http://localhost:8081/health
# 应该看到其中一个节点成为新的 leader
```

---

## Kubernetes 部署

### 1. 创建命名空间

```yaml
# namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: bdopsflow
```

### 2. 部署 Redis（哨兵模式）

可以使用 Helm chart 或自定义部署，建议使用成熟的 Redis Operator。

### 3. 部署 rqlite（3 节点 StatefulSet）

```yaml
# rqlite.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: rqlite-auth
  namespace: bdopsflow
data:
  auth.json: |
    [
      {
        "username": "admin",
        "password": "your-rqlite-password",
        "perms": ["all"]
      }
    ]
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: rqlite
  namespace: bdopsflow
spec:
  serviceName: rqlite
  replicas: 3
  selector:
    matchLabels:
      app: rqlite
  template:
    metadata:
      labels:
        app: rqlite
    spec:
      containers:
      - name: rqlite
        image: rqlite/rqlite:latest
        ports:
        - containerPort: 4001
          name: http
        - containerPort: 4002
          name: raft
        volumeMounts:
        - name: data
          mountPath: /data
        - name: auth
          mountPath: /auth.json
          subPath: auth.json
        env:
        - name: NODE_ID
          valueFrom:
            fieldRef:
              fieldPath: metadata.annotations['node-id']
        command:
        - /bin/sh
        - -c
        - |
          if [ "$(hostname)" = "rqlite-0" ]; then
            rqlited -node-id 1 -http-addr 0.0.0.0:4001 -raft-addr 0.0.0.0:4002 -data-dir /data -auth /auth.json -bootstrap-expect 3
          else
            rqlited -node-id $(($(hostname | cut -d'-' -f2) + 1)) -http-addr 0.0.0.0:4001 -raft-addr 0.0.0.0:4002 -data-dir /data -auth /auth.json -join http://admin:your-rqlite-password@rqlite-0.rqlite:4001 -bootstrap-expect 3
          fi
      volumes:
      - name: auth
        configMap:
          name: rqlite-auth
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 10Gi
---
apiVersion: v1
kind: Service
metadata:
  name: rqlite
  namespace: bdopsflow
spec:
  selector:
    app: rqlite
  ports:
  - port: 4001
    targetPort: 4001
    name: http
```

### 4. 部署调度中心集群（3 节点高可用）

```yaml
# scheduler.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: scheduler
  namespace: bdopsflow
spec:
  serviceName: scheduler
  replicas: 3
  selector:
    matchLabels:
      app: scheduler
  template:
    metadata:
      labels:
        app: scheduler
    spec:
      containers:
      - name: scheduler
        image: bdopsflow/scheduler:latest
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 50051
          name: grpc
        env:
        # 使用 Pod 名称作为 node_id（scheduler-0, scheduler-1, scheduler-2）
        - name: APP_NODE_ID
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: DATABASE_RQLITE_ADDRS
          value: "http://rqlite-0.rqlite:4001,http://rqlite-1.rqlite:4001,http://rqlite-2.rqlite:4001"
        - name: DATABASE_RQLITE_USER
          value: "admin"
        - name: DATABASE_RQLITE_PASSWORD
          value: "your-rqlite-password"
        - name: REDIS_MODE
          value: "sentinel"
        - name: REDIS_MASTER_NAME
          value: "mymaster"
        - name: REDIS_SENTINEL_ADDRS
          value: "redis-sentinel:26379"
        - name: REDIS_PASSWORD
          value: "your-redis-password"
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: bdopsflow-secrets
              key: jwt-secret
        resources:
          requests:
            cpu: 200m
            memory: 512Mi
          limits:
            cpu: 1000m
            memory: 1Gi
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: scheduler
  namespace: bdopsflow
spec:
  selector:
    app: scheduler
  ports:
  - name: http
    port: 8080
    targetPort: 8080
  - name: grpc
    port: 50051
    targetPort: 50051
```

### 5. 部署执行器

```yaml
# executor.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: executor
  namespace: bdopsflow
spec:
  replicas: 3
  selector:
    matchLabels:
      app: executor
  template:
    metadata:
      labels:
        app: executor
    spec:
      containers:
      - name: executor
        image: bdopsflow/executor:latest
        env:
        - name: SCHEDULER_ADDR
          value: "scheduler:50051"
        - name: APP_CAPACITY
          value: "20"
        resources:
          requests:
            cpu: 200m
            memory: 256Mi
          limits:
            cpu: 500m
            memory: 512Mi
```

---

## 配置详解

### 调度中心配置

| 配置项 | 环境变量 | 默认值 | 说明 |
|--------|----------|--------|------|
| app.node_id | APP_NODE_ID | 自动生成 UUID | 节点唯一标识，用于主节点选举 |
| app.http_port | APP_HTTP_PORT | 8080 | HTTP API 端口 |
| app.grpc_port | APP_GRPC_PORT | 50051 | gRPC 端口 |
| database.rqlite_addrs | DATABASE_RQLITE_ADDRS | ["http://localhost:4001"] | rqlite 多节点地址列表（逗号分隔） |
| database.rqlite_user | DATABASE_RQLITE_USER | "" | rqlite 用户名 |
| database.rqlite_password | DATABASE_RQLITE_PASSWORD | "" | rqlite 密码 |
| database.rqlite_tls | DATABASE_RQLITE_TLS | false | 是否使用 TLS 连接 rqlite |
| redis.mode | REDIS_MODE | single | Redis 模式：single 或 sentinel |
| redis.addr | REDIS_ADDR | localhost:6379 | Redis 单实例地址 |
| redis.password | REDIS_PASSWORD | (空) | Redis 密码 |
| redis.db | REDIS_DB | 0 | Redis 数据库 |
| redis.master_name | REDIS_MASTER_NAME | mymaster | Redis Sentinel 主节点名称 |
| redis.sentinel_addrs | REDIS_SENTINEL_ADDRS | (空) | Redis Sentinel 节点地址列表（逗号分隔） |
| redis.sentinel_password | REDIS_SENTINEL_PASSWORD | (空) | Redis Sentinel 密码 |
| jwt.secret | JWT_SECRET | (必填) | JWT 密钥 |
| jwt.expiry_hours | JWT_EXPIRY_HOURS | 24 | Token 过期时间 |
| log.level | LOG_LEVEL | info | 日志级别 |
| log.format | LOG_FORMAT | json | 日志格式 |

### 执行器配置

| 配置项 | 环境变量 | 默认值 | 说明 |
|--------|----------|--------|------|
| app.executor_id | APP_EXECUTOR_ID | executor-1 | 执行器唯一 ID |
| app.executor_name | APP_EXECUTOR_NAME | executor-1 | 执行器名称 |
| app.capacity | APP_CAPACITY | 10 | 最大并发任务数 |
| scheduler.addr | SCHEDULER_ADDR | localhost:50051 | 调度中心地址 |
| scheduler.timeout | SCHEDULER_TIMEOUT | 30 | 连接超时（秒） |
| log.level | LOG_LEVEL | info | 日志级别 |

---

## 监控与运维

### 健康检查

**调度中心**：
```bash
# HTTP 健康检查
curl http://localhost:8080/health

# 预期响应
{"status": "ok"}
```

**Redis**：
```bash
# 单实例
redis-cli -a your-redis-password ping
# 预期响应: PONG

# Sentinel
redis-cli -p 26379 sentinel get-master-addr-by-name mymaster
```

**rqlite**：
```bash
# 单节点
curl http://localhost:4001/status?pretty

# 带认证
curl -u admin:your-rqlite-password http://localhost:4001/status?pretty
```

### 日志管理

**查看调度中心日志**：
```bash
# systemd
journalctl -u bdopsflow-scheduler -f

# Docker
docker logs -f bdopsflow-scheduler
```

**查看执行器日志**：
```bash
# systemd
journalctl -u bdopsflow-executor -f

# Docker
docker logs -f bdopsflow-executor
```

### 性能监控

推荐使用 Prometheus + Grafana 进行监控。

**关键指标**：
- 任务执行成功率
- 任务平均执行时长
- 执行器在线数量
- 执行器负载
- API 请求 QPS
- 数据库查询延迟

### 备份策略

**rqlite 备份**：
```bash
# 创建备份（带认证）
curl -u admin:your-rqlite-password http://localhost:4001/db/backup > backup-$(date +%Y%m%d).sqlite

# 定时备份（crontab）
0 2 * * * curl -u admin:your-rqlite-password http://localhost:4001/db/backup > /backup/rqlite-$(date +\%Y\%m\%d).sqlite
```

**Redis 备份**：
```bash
# RDB 备份
redis-cli -a your-redis-password BGSAVE

# 复制 RDB 文件
cp /var/lib/redis/dump.rdb /backup/redis-$(date +%Y%m%d).rdb
```

### 故障恢复

**调度中心故障**：
1. 多实例部署，自动故障转移
2. 检查日志定位问题
3. 重启服务

**执行器故障**：
1. 任务自动重新分发到其他执行器
2. 检查执行器日志
3. 重启执行器

**数据库故障**：
1. rqlite 集群自动选举新主节点
2. 客户端自动连接到可用节点
3. 从备份恢复数据（如需要）

**Redis 故障**：
1. Sentinel 自动故障转移
2. 从备份恢复数据（如需要）

---

## 安全加固

### 1. 网络安全

- 使用防火墙限制端口访问
- 仅开放必要端口（80/443 对外，其他内网）
- 使用 VPN 或专线连接各组件

### 2. 认证安全

- 使用强 JWT 密钥（至少 32 字符）
- 定期轮换密钥
- 设置合理的 Token 过期时间
- rqlite 启用密码认证
- Redis 启用密码认证

### 3. 数据安全

- rqlite 和 Redis 启用密码认证
- 数据库连接使用 TLS（生产环境建议）
- 定期备份数据
- 敏感数据脱敏存储

### 4. 容器安全

- 使用非 root 用户运行容器
- 限制容器资源
- 定期更新基础镜像
