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

### 2. 启动 rqlite

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
Connected to Redis successfully
Connected to rqlite successfully
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

### 1. 部署 Redis 集群

推荐使用 Redis Sentinel 或 Redis Cluster 实现高可用。

**Redis Sentinel 配置**：

```conf
# sentinel.conf
sentinel monitor mymaster 192.168.1.100 6379 2
sentinel down-after-milliseconds mymaster 30000
sentinel parallel-syncs mymaster 1
sentinel failover-timeout mymaster 180000
```

### 2. 部署 rqlite 集群

rqlite 使用 Raft 协议实现高可用，建议至少 3 节点部署。

**节点 1**：
```bash
rqlited -node-id 1 \
  -http-addr 0.0.0.0:4001 \
  -raft-addr 0.0.0.0:4002 \
  -data-dir /data/rqlite1 \
  -bootstrap-expect 3
```

**节点 2**：
```bash
rqlited -node-id 2 \
  -http-addr 0.0.0.0:4001 \
  -raft-addr 0.0.0.0:4002 \
  -data-dir /data/rqlite2 \
  -join http://node1:4002 \
  -bootstrap-expect 3
```

**节点 3**：
```bash
rqlited -node-id 3 \
  -http-addr 0.0.0.0:4001 \
  -raft-addr 0.0.0.0:4002 \
  -data-dir /data/rqlite3 \
  -join http://node1:4002 \
  -bootstrap-expect 3
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
  rqlite_dsn: "http://rqlite-lb:4001"

redis:
  addr: "redis-sentinel:26379"
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

### Docker Compose 完整部署

```yaml
# docker-compose.yml
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
      - DATABASE_RQLITE_DSN=http://rqlite:4001
      - REDIS_ADDR=redis:6379
      - JWT_SECRET=your-production-secret-key

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
docker-compose up -d

# 查看日志
docker-compose logs -f

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

### 2. 部署 Redis

```yaml
# redis.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
  namespace: bdopsflow
spec:
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
        ports:
        - containerPort: 6379
        resources:
          requests:
            cpu: 100m
            memory: 256Mi
          limits:
            cpu: 500m
            memory: 512Mi
---
apiVersion: v1
kind: Service
metadata:
  name: redis
  namespace: bdopsflow
spec:
  selector:
    app: redis
  ports:
  - port: 6379
    targetPort: 6379
```

### 3. 部署 rqlite

```yaml
# rqlite.yaml
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
        - name: DATABASE_RQLITE_DSN
          value: "http://rqlite:4001"
        - name: REDIS_ADDR
          value: "redis:6379"
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
| database.rqlite_dsn | DATABASE_RQLITE_DSN | http://localhost:4001 | rqlite 地址 |
| redis.addr | REDIS_ADDR | localhost:6379 | Redis 地址 |
| redis.password | REDIS_PASSWORD | (空) | Redis 密码 |
| redis.db | REDIS_DB | 0 | Redis 数据库 |
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
redis-cli ping
# 预期响应: PONG
```

**rqlite**：
```bash
curl http://localhost:4001/status?pretty
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
# 创建备份
curl http://localhost:4001/db/backup > backup-$(date +%Y%m%d).sqlite

# 定时备份（crontab）
0 2 * * * curl http://localhost:4001/db/backup > /backup/rqlite-$(date +\%Y\%m\%d).sqlite
```

**Redis 备份**：
```bash
# RDB 备份
redis-cli BGSAVE

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
2. 从备份恢复数据（如需要）

**Redis 故障**：
1. 使用 Sentinel 自动故障转移
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

### 3. 数据安全

- Redis 启用密码认证
- 数据库连接使用 TLS
- 定期备份数据

### 4. 容器安全

- 使用非 root 用户运行容器
- 限制容器资源
- 定期更新基础镜像
