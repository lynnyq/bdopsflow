# BDopsFlow 前端 Nginx 部署文档

## 目录
1. [部署概述](#部署概述)
2. [前置条件](#前置条件)
3. [构建前端](#构建前端)
4. [基础 Nginx 配置](#基础-nginx-配置)
5. [反向代理配置](#反向代理配置)
6. [生产环境配置](#生产环境配置)
7. [Docker 部署](#docker-部署)
8. [常见问题](#常见问题)

---

## 部署概述

BDopsFlow 前端是基于 Vue 3 + Vite 构建的单页应用（SPA）。本文档详细介绍如何使用 Nginx 部署该应用，包括：
- 前端静态资源服务
- 后端 API 反向代理
- SPA 路由回退支持
- HTTPS 和安全配置
- 性能优化
- Docker 部署方案

---

## 前置条件

### 1. 系统要求

- **操作系统**: Linux (Ubuntu 20.04+ / CentOS 7+ / Debian 11+) 或其他 POSIX 兼容系统
- **Nginx 版本**: 1.18.0+ (推荐使用最新稳定版)
- **Node.js**: 18.x+ (仅用于构建前端)
- **npm**: 9.x+

### 2. 环境准备

#### 安装 Node.js 和 npm (Ubuntu/Debian)

```bash
# 安装 Node.js 18
curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
sudo apt-get install -y nodejs

# 验证安装
node --version
npm --version
```

#### 安装 Nginx (Ubuntu/Debian)

```bash
sudo apt-get update
sudo apt-get install -y nginx

# 启动 Nginx
sudo systemctl start nginx
sudo systemctl enable nginx

# 验证安装
nginx -v
```

#### 安装 Nginx (CentOS/RHEL)

```bash
sudo yum install -y nginx
sudo systemctl start nginx
sudo systemctl enable nginx
```

---

## 构建前端

### 1. 进入前端目录

```bash
cd /path/to/bdopsflow/web
```

### 2. 安装依赖

```bash
npm install
```

### 3. 生产环境构建

```bash
npm run build
```

构建成功后，静态资源会生成在 `dist/` 目录下：

```
dist/
├── assets/
│   ├── index-*.css
│   ├── index-*.js
│   └── ...
├── index.html
└── favicon.ico (如果有)
```

---

## 基础 Nginx 配置

### 1. 创建 Nginx 配置文件

创建 `/etc/nginx/sites-available/bdopsflow`：

```nginx
server {
    listen 80;
    server_name your-domain.com;  # 替换为你的域名或 IP

    # 前端应用根目录
    root /var/www/bdopsflow/dist;
    index index.html;

    # 日志配置
    access_log /var/log/nginx/bdopsflow_access.log;
    error_log /var/log/nginx/bdopsflow_error.log;

    # 静态资源缓存配置
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    # HTML 文件不缓存
    location ~* \.html$ {
        expires -1;
        add_header Cache-Control "no-cache, private, must-revalidate";
    }

    # SPA 路由回退 - 关键！
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Gzip 压缩
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript 
               application/javascript application/xml+rss application/json
               application/xml image/svg+xml;
}
```

### 2. 部署前端资源

```bash
# 创建目标目录
sudo mkdir -p /var/www/bdopsflow

# 复制构建产物
sudo cp -r /path/to/bdopsflow/web/dist/* /var/www/bdopsflow/

# 设置权限
sudo chown -R www-data:www-data /var/www/bdopsflow
sudo chmod -R 755 /var/www/bdopsflow
```

### 3. 启用站点

```bash
# 创建软链接
sudo ln -s /etc/nginx/sites-available/bdopsflow /etc/nginx/sites-enabled/

# 测试配置文件语法
sudo nginx -t

# 重载 Nginx
sudo systemctl reload nginx
```

---

## 反向代理配置

如果你的后端服务和前端部署在同一台机器或不同机器上，需要配置反向代理。

### 完整配置示例

```nginx
server {
    listen 80;
    server_name your-domain.com;

    root /var/www/bdopsflow/dist;
    index index.html;

    access_log /var/log/nginx/bdopsflow_access.log;
    error_log /var/log/nginx/bdopsflow_error.log;

    # 前端静态资源
    location / {
        try_files $uri $uri/ /index.html;
    }

    # 后端 API 反向代理
    location /api/ {
        proxy_pass http://localhost:8080/api/;  # 指向你的后端服务
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # 超时设置
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }

    # 健康检查端点
    location /health {
        proxy_pass http://localhost:8080/health;
        proxy_set_header Host $host;
    }

    # WebSocket 支持（如果需要）
    location /ws/ {
        proxy_pass http://localhost:8080/ws/;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # 静态资源缓存
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    location ~* \.html$ {
        expires -1;
        add_header Cache-Control "no-cache, private, must-revalidate";
    }

    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript 
               application/javascript application/xml+rss application/json
               application/xml image/svg+xml;
}
```

---

## 生产环境配置

### 1. HTTPS 配置 (使用 Let's Encrypt)

#### 安装 Certbot

```bash
# Ubuntu/Debian
sudo apt-get install -y certbot python3-certbot-nginx

# CentOS/RHEL
sudo yum install -y certbot python3-certbot-nginx
```

#### 获取 SSL 证书

```bash
sudo certbot --nginx -d your-domain.com
```

Certbot 会自动配置 HTTPS。

#### HTTPS 配置示例

```nginx
server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name your-domain.com;

    root /var/www/bdopsflow/dist;
    index index.html;

    # SSL 证书配置
    ssl_certificate /etc/letsencrypt/live/your-domain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/your-domain.com/privkey.pem;
    
    # SSL 安全配置
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;

    # HSTS (可选但推荐)
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

    access_log /var/log/nginx/bdopsflow_access.log;
    error_log /var/log/nginx/bdopsflow_error.log;

    # 安全头
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "no-referrer-when-downgrade" always;

    location / {
        try_files $uri $uri/ /index.html;
    }

    location /api/ {
        proxy_pass http://localhost:8080/api/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # 静态资源缓存
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_comp_level 6;
    gzip_types text/plain text/css text/xml text/javascript 
               application/javascript application/xml+rss application/json
               application/xml image/svg+xml;
}

# HTTP 重定向到 HTTPS
server {
    listen 80;
    listen [::]:80;
    server_name your-domain.com;
    
    location / {
        return 301 https://$server_name$request_uri;
    }
}
```

### 2. 多调度器负载均衡配置

如果部署了多个调度器节点，可以配置 Nginx 作为负载均衡器：

```nginx
# 上游服务器组
upstream bdopsflow_backend {
    server localhost:8080;  # scheduler1
    server localhost:8081;  # scheduler2
    server localhost:8082;  # scheduler3

    # 负载均衡策略
    ip_hash;  # 或 least_conn;
    keepalive 32;
}

server {
    listen 443 ssl http2;
    server_name your-domain.com;

    root /var/www/bdopsflow/dist;
    index index.html;

    ssl_certificate /etc/letsencrypt/live/your-domain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/your-domain.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;

    location / {
        try_files $uri $uri/ /index.html;
    }

    # 代理到上游组
    location /api/ {
        proxy_pass http://bdopsflow_backend/api/;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # 超时设置
        proxy_connect_timeout 30s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }

    location /health {
        proxy_pass http://bdopsflow_backend/health;
    }

    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_comp_level 6;
    gzip_types text/plain text/css text/xml text/javascript 
               application/javascript application/xml+rss application/json
               application/xml image/svg+xml;
}
```

---

## Docker 部署

### 1. 使用官方 Dockerfile

项目已经提供了 `deploy/Dockerfile.web`：

```dockerfile
# Build stage
FROM node:18-alpine AS builder

WORKDIR /app

COPY package*.json ./
RUN npm install

COPY . .

RUN npm run build

# Production stage
FROM nginx:alpine

COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
```

### 2. 自定义 Nginx 配置

创建 `deploy/nginx.conf`：

```nginx
server {
    listen 80;
    server_name _;
    root /usr/share/nginx/html;
    index index.html;

    access_log /var/log/nginx/bdopsflow_access.log;
    error_log /var/log/nginx/bdopsflow_error.log;

    location / {
        try_files $uri $uri/ /index.html;
    }

    # 如果使用 Docker Compose 部署，可以通过容器名代理
    location /api/ {
        proxy_pass http://scheduler1:8080/api/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript 
               application/javascript application/xml+rss application/json
               application/xml image/svg+xml;
}
```

更新 `Dockerfile.web`：

```dockerfile
# Build stage
FROM node:18-alpine AS builder

WORKDIR /app

COPY package*.json ./
RUN npm install

COPY . .

RUN npm run build

# Production stage
FROM nginx:alpine

# 复制自定义配置
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf

# 复制构建产物
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
```

### 3. Docker Compose 集成

修改 `deploy/docker-compose.yml`，添加 web 服务：

```yaml
version: '3.8'

services:
  redis:
    image: redis:7-alpine
    container_name: bdopsflow-redis
    ports:
      - "6379:6379"
    networks:
      - bdopsflow

  rqlite1:
    image: rqlite/rqlite:latest
    container_name: bdopsflow-rqlite1
    ports:
      - "4001:4001"
    command: rqlited -http-addr http://0.0.0.0:4001 -raft-addr 0.0.0.0:4002 ~/node1
    networks:
      - bdopsflow

  rqlite2:
    image: rqlite/rqlite:latest
    container_name: bdopsflow-rqlite2
    ports:
      - "4002:4001"
    command: rqlited -http-addr http://0.0.0.0:4001 -raft-addr 0.0.0.0:4002 -raft-join http://rqlite1:4001 ~/node2
    depends_on:
      - rqlite1
    networks:
      - bdopsflow

  rqlite3:
    image: rqlite/rqlite:latest
    container_name: bdopsflow-rqlite3
    ports:
      - "4003:4001"
    command: rqlited -http-addr http://0.0.0.0:4001 -raft-addr 0.0.0.0:4002 -raft-join http://rqlite1:4001 ~/node3
    depends_on:
      - rqlite1
    networks:
      - bdopsflow

  # 独立的 Nginx 服务
  web:
    build:
      context: ..
      dockerfile: deploy/Dockerfile.web
    container_name: bdopsflow-web
    ports:
      - "80:80"
      - "443:443"  # 如果需要 HTTPS
    depends_on:
      - scheduler1
    volumes:
      # 如果需要持久化日志
      - ./nginx/logs:/var/log/nginx
      # 如果需要 SSL 证书
      # - ./ssl:/etc/nginx/ssl
    networks:
      - bdopsflow

  scheduler1:
    build:
      context: ..
      dockerfile: deploy/Dockerfile.scheduler
    container_name: bdopsflow-scheduler1
    ports:
      - "8080:8080"
      - "50051:50051"
    environment:
      - APP_NODE_ID=scheduler1
      - HTTP_PORT=8080
      - GRPC_PORT=50051
      - DATABASE_RQLITE_ADDRS=http://rqlite1:4001,http://rqlite2:4001,http://rqlite3:4001
      - REDIS_ADDR=redis:6379
      - DATASOURCE_ENCRYPTION_KEY=dev-docker-32byte-key-chg-prod-1
    depends_on:
      - redis
      - rqlite1
    networks:
      - bdopsflow

  scheduler2:
    build:
      context: ..
      dockerfile: deploy/Dockerfile.scheduler
    container_name: bdopsflow-scheduler2
    ports:
      - "8081:8080"
      - "50052:50051"
    environment:
      - APP_NODE_ID=scheduler2
      - HTTP_PORT=8080
      - GRPC_PORT=50051
      - DATABASE_RQLITE_ADDRS=http://rqlite1:4001,http://rqlite2:4001,http://rqlite3:4001
      - REDIS_ADDR=redis:6379
      - DATASOURCE_ENCRYPTION_KEY=dev-docker-32byte-key-chg-prod-1
    depends_on:
      - redis
      - rqlite1
    networks:
      - bdopsflow

  scheduler3:
    build:
      context: ..
      dockerfile: deploy/Dockerfile.scheduler
    container_name: bdopsflow-scheduler3
    ports:
      - "8082:8080"
      - "50053:50051"
    environment:
      - APP_NODE_ID=scheduler3
      - HTTP_PORT=8080
      - GRPC_PORT=50051
      - DATABASE_RQLITE_ADDRS=http://rqlite1:4001,http://rqlite2:4001,http://rqlite3:4001
      - REDIS_ADDR=redis:6379
      - DATASOURCE_ENCRYPTION_KEY=dev-docker-32byte-key-chg-prod-1
    depends_on:
      - redis
      - rqlite1
    networks:
      - bdopsflow

  executor:
    build:
      context: ..
      dockerfile: deploy/Dockerfile.executor
    container_name: bdopsflow-executor
    environment:
      - EXECUTOR_ID=executor-1
      - EXECUTOR_NAME=executor-1
      - SCHEDULER_ADDR=scheduler1:50051
      - CAPACITY=10
    depends_on:
      - scheduler1
    networks:
      - bdopsflow

networks:
  bdopsflow:
    driver: bridge
```

### 4. 启动服务

```bash
cd deploy

# 构建并启动
docker-compose up -d --build

# 查看日志
docker-compose logs -f web
```

---

## 常见问题

### 1. SPA 路由刷新 404

**问题**: 在非首页路由刷新页面返回 404

**解决方案**: 确保配置了 `try_files $uri $uri/ /index.html;`

```nginx
location / {
    try_files $uri $uri/ /index.html;
}
```

### 2. 静态资源缓存问题

**问题**: 代码更新后，用户浏览器仍然加载旧版本

**解决方案**:
- 确保构建文件名包含 hash（Vite 会自动处理）
- 配置正确的缓存头
- 对于重要更新，给用户提供清除缓存的提示

### 3. API 代理失败

**问题**: 前端 API 请求失败或超时

**解决方案**:
1. 检查后端服务是否正常运行
2. 验证代理地址配置正确
3. 检查防火墙规则
4. 增加超时时间
5. 查看 Nginx 错误日志 `/var/log/nginx/bdopsflow_error.log`

### 4. CORS 问题

**问题**: 浏览器报错跨域问题

**解决方案**: 如果 Nginx 反向代理配置正确，通常不会有 CORS 问题。如果前后端分离部署，请在后端配置 CORS。

### 5. 文件上传大小限制

**问题**: 上传大文件失败

**解决方案**: 在 Nginx 配置中添加：

```nginx
client_max_body_size 100M;  # 根据需要调整大小
```

---

## 维护与监控

### 1. 日志轮转

创建 `/etc/logrotate.d/bdopsflow`：

```
/var/log/nginx/bdopsflow_*.log {
    daily
    rotate 14
    compress
    delaycompress
    notifempty
    create 0640 www-data adm
    sharedscripts
    postrotate
        [ -f /var/run/nginx.pid ] && kill -USR1 `cat /var/run/nginx.pid`
    endscript
}
```

### 2. 监控 Nginx 状态

启用 status 模块：

```nginx
server {
    listen 127.0.0.1:8080;
    
    location /nginx_status {
        stub_status on;
        access_log off;
        allow 127.0.0.1;
        deny all;
    }
}
```

### 3. 性能调优

在 `nginx.conf` 中配置：

```nginx
worker_processes auto;
worker_connections 2048;
use epoll;

keepalive_timeout 65;
keepalive_requests 100;
```

---

## 快速部署命令总结

### 传统部署

```bash
# 1. 构建
cd web && npm install && npm run build

# 2. 部署
sudo mkdir -p /var/www/bdopsflow
sudo cp -r dist/* /var/www/bdopsflow/
sudo chown -R www-data:www-data /var/www/bdopsflow

# 3. 配置 Nginx
# 编辑 /etc/nginx/sites-available/bdopsflow

# 4. 启用并重启
sudo ln -s /etc/nginx/sites-available/bdopsflow /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

### Docker 部署

```bash
cd deploy

# 一键部署
docker-compose up -d --build

# 查看状态
docker-compose ps

# 查看日志
docker-compose logs -f web
```
