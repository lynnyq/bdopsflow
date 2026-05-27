# BDopsFlow 部署指南

本文档提供了 BDopsFlow 在各种环境下的完整部署说明。

## 目录

- [系统要求](#系统要求)
- [快速部署](#快速部署)
- [生产部署](#生产部署)
- [健康检查与监控](#健康检查与监控)
- [高可用配置](#高可用配置)
- [备份与恢复](#备份与恢复)
- [常见问题](#常见问题)

## 系统要求

### 最低要求

- **操作系统**: Linux (Ubuntu 20.04+, CentOS 8+), macOS 11+
- **CPU**: 2 核
- **内存**: 4 GB RAM
- **磁盘**: 20 GB 可用空间
- **Go**: 1.24+ (仅开发环境需要)
- **Node.js**: 18+ (仅开发环境需要)
- **rqlite**: 7.0+
- **Redis**: 6.0+

### 推荐配置 (生产环境)

- **操作系统**: Linux (Ubuntu 22.04 LTS)
- **CPU**: 4+ 核
- **内存**: 8+ GB RAM
- **磁盘**: 100 GB+ SSD
- **rqlite**: 集群模式 (3+ 节点)
- **Redis**: 主从复制或 Sentinel 模式

## 快速部署

### 使用 Docker Compose (推荐)

1. 克隆仓库:
```bash
git clone https://github.com/lynnyq/bdopsflow.git
cd bdopsflow
```

2. 创建配置文件:
```bash
cp scheduler/config.yaml.example scheduler/config.yaml
cp executor/config.yaml.example executor/config.yaml
```

3. 使用 Docker Compose 启动:
```bash
docker-compose up -d
```

4. 访问应用:
- Web UI: http://localhost:8080
- 默认账号: admin / admin123

### 手动部署

#### 1. 准备基础设施

```bash
# 启动 rqlite
docker run -d --name rqlite -p 4001:4001 rqlite/rqlite:latest

# 启动 Redis
docker run -d --name redis -p 6379:6379 redis:7-alpine
```

#### 2. 编译并启动调度器

```bash
cd scheduler
go build -o bin/scheduler ./cmd
./bin/scheduler -config config.yaml
```

#### 3. 编译并启动执行器

```bash
cd executor
go build -o bin/executor ./cmd
./bin/executor -config config.yaml
```

#### 4. 构建并启动前端

```bash
cd web
npm install
npm run build
# 前端会嵌入到调度器中
```

## 生产部署

### 1. 系统调优

#### Linux 内核参数

```bash
# /etc/sysctl.conf
net.core.somaxconn = 65535
net.ipv4.tcp_max_syn_backlog = 65535
net.ipv4.tcp_tw_reuse = 1
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
vm.swappiness = 10
```

应用配置:
```bash
sysctl -p
```

#### 文件描述符限制

```bash
# /etc/security/limits.conf
* soft nofile 65536
* hard nofile 65536
```

### 2. rqlite 集群部署

创建 3 节点 rqlite 集群:

```bash
# 节点 1
docker run -d --name rqlite-1 -p 4001:4001 rqlite/rqlite -http-addr 0.0.0.0:4001 -raft-addr 0.0.0.0:4002

# 节点 2 (加入集群)
docker run -d --name rqlite-2 -p 4002:4001 rqlite/rqlite -http-addr 0.0.0.0:4001 -raft-addr 0.0.0.0:4002 -join http://node1-ip:4001

# 节点 3 (加入集群)
docker run -d --name rqlite-3 -p 4003:4001 rqlite/rqlite -http-addr 0.0.0.0:4001 -raft-addr 0.0.0.0:4002 -join http://node1-ip:4001
```

更新调度器配置 (`config.yaml`):
```yaml
database:
  rqlite_addrs:
    - "http://node1-ip:4001"
    - "http://node2-ip:4001"
    - "http://node3-ip:4001"
```

### 3. Redis 高可用

使用 Redis Sentinel:

```yaml
redis:
  mode: "sentinel"
  master_name: "mymaster"
  sentinel_addrs:
    - "sentinel1:26379"
    - "sentinel2:26379"
    - "sentinel3:26379"
  sentinel_password: "your-sentinel-password"
  password: "your-redis-password"
  db: 0
```

### 4. 反向代理配置

使用 Nginx:

```nginx
upstream bdopsflow {
    server 127.0.0.1:8080;
    keepalive 32;
}

server {
    listen 80;
    server_name your-domain.com;
    
    client_max_body_size 10M;
    
    location / {
        proxy_pass http://bdopsflow;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        proxy_http_version 1.1;
        proxy_set_header Connection "";
    }
    
    # SSE (实时日志) 需要特殊配置
    location /api/tasks/*/logs/stream {
        proxy_pass http://bdopsflow;
        proxy_set_header Host $host;
        proxy_buffering off;
        proxy_cache off;
        proxy_set_header Connection '';
        proxy_http_version 1.1;
        chunked_transfer_encoding off;
    }
}
```

## 健康检查与监控

### 健康检查端点

BDopsFlow 提供了标准的 Kubernetes 健康检查端点:

- **Liveness Probe**: `/healthz` - 检查服务是否存活
- **Readiness Probe**: `/readyz` - 检查服务是否就绪
- **传统端点**: `/health` - 综合健康状态
- **Metrics**: `/metrics` - 性能指标

#### Liveness 检查

```bash
curl http://localhost:8080/healthz
```

响应:
```json
{
  "status": "ok",
  "version": "1.0.0",
  "uptime": "1h23m45s"
}
```

#### Readiness 检查

```bash
curl http://localhost:8080/readyz
```

响应包含所有组件的详细健康状态:
```json
{
  "status": "passing",
  "timestamp": "2024-01-01T00:00:00Z",
  "version": "1.0.0",
  "checks": [
    {
      "name": "redis",
      "status": "passing",
      "message": "Redis connection healthy",
      "timestamp": "2024-01-01T00:00:00Z",
      "duration": 1234567
    },
    {
      "name": "rqlite",
      "status": "passing",
      "message": "RQLite connection healthy",
      "timestamp": "2024-01-01T00:00:00Z",
      "duration": 234567
    },
    {
      "name": "disk_space",
      "status": "passing",
      "message": "Disk usage healthy: 50%",
      "metadata": {
        "usage_percent": 50.0
      }
    }
  ]
}
```

#### Metrics 端点

```bash
curl http://localhost:8080/metrics
```

### Kubernetes 配置示例

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bdopsflow-scheduler
spec:
  replicas: 3
  selector:
    matchLabels:
      app: bdopsflow-scheduler
  template:
    metadata:
      labels:
        app: bdopsflow-scheduler
    spec:
      containers:
      - name: scheduler
        image: bdopsflow/scheduler:latest
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 50051
          name: grpc
        livenessProbe:
          httpGet:
            path: /healthz
            port: http
          initialDelaySeconds: 10
          periodSeconds: 30
          timeoutSeconds: 5
        readinessProbe:
          httpGet:
            path: /readyz
            port: http
          initialDelaySeconds: 5
          periodSeconds: 10
          timeoutSeconds: 3
        resources:
          requests:
            cpu: "500m"
            memory: "512Mi"
          limits:
            cpu: "2000m"
            memory: "2Gi"
```

### 监控集成

#### Prometheus 配置

虽然当前的 `/metrics` 端点返回 JSON，但你可以通过配置使其适应 Prometheus:

```yaml
scrape_configs:
  - job_name: 'bdopsflow'
    scrape_interval: 15s
    metrics_path: '/metrics'
    static_configs:
      - targets: ['localhost:8080']
```

## 高可用配置

### 调度器高可用

BDopsFlow 支持多调度器部署，通过 Redis 进行 Leader 选举:

```yaml
app:
  node_id: "scheduler-1"  # 每个节点唯一 ID
```

部署 3 个调度器实例，系统会自动选举一个 Leader 处理任务。

### 执行器高可用

部署多个执行器节点，通过标签进行任务调度:

```yaml
executor:
  name: "executor-1"
  capacity: 10  # 并发执行数
  tags: ["default", "cpu-intensive"]
```

## 备份与恢复

### 数据备份

1. 备份 rqlite 数据:
```bash
# 在线备份
curl http://rqlite-addr:4001/db/backup -o backup.sqlite3

# 定时备份脚本
#!/bin/bash
BACKUP_DIR="/var/backups/bdopsflow"
DATE=$(date +%Y%m%d_%H%M%S)
mkdir -p $BACKUP_DIR
curl http://localhost:4001/db/backup -o $BACKUP_DIR/backup_$DATE.sqlite3
find $BACKUP_DIR -name "backup_*.sqlite3" -mtime +7 -delete
```

2. 备份配置和密钥:
```bash
tar -czf bdopsflow-config.tar.gz config.yaml
```

### 数据恢复

```bash
# 恢复 rqlite
curl -XPOST http://rqlite-addr:4001/db/load -H "Content-Type: application/octet-stream" --data-binary @backup.sqlite3
```

## 常见问题

### Q: 如何查看日志?

A:
```bash
# Docker Compose
docker-compose logs -f scheduler
docker-compose logs -f executor

# Systemd
journalctl -u bdopsflow-scheduler -f
```

### Q: 性能调优建议?

A:
- 增加 Redis 内存
- 调整 rqlite  raft 超时参数
- 增加执行器容量
- 启用查询缓存

### Q: 如何升级版本?

A:
1. 备份数据
2. 拉取新镜像
3. 滚动更新 (Kubernetes) 或逐台重启
4. 验证功能正常

### Q: 端口占用?

A: 默认端口:
- 8080: HTTP API / Web UI
- 50051: gRPC (调度器-执行器通信)
- 4001: rqlite
- 6379: Redis

可通过配置文件修改。

## 更多资源

- [安全配置指南](./SECURITY.md)
- [使用指南](./GUIDE.md)
- [API 文档](./API.md)
