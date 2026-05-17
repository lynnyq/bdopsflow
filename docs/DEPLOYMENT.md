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

### 3. 部署调度中心

**编译**：
```bash
cd scheduler
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/scheduler ./cmd/main.go
```

**配置** (config.yaml)：
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
  password: "your-redis-password"
  db: 0

jwt:
  secret: "your-production-secret-key-at-least-32-chars"
  expiry_hours: 24

log:
  level: "info"
  format: "json"
```

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

### Docker Compose 完整部署（开发环境）

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

### 启动服务

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

### 4. 部署调度中心

```yaml
# scheduler.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: scheduler
  namespace: bdopsflow
spec:
  replicas: 2
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
