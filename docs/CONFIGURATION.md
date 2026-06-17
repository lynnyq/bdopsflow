# BDopsFlow 配置指南

本文档详细说明 BDopsFlow 分布式工作流调度平台的所有配置项，包括调度中心配置、执行器配置、环境变量、密钥管理、系统运行时配置和安全检查清单。

## 目录

- [1. 调度中心配置](#1-调度中心配置)
- [2. 执行器配置](#2-执行器配置)
- [3. 环境变量参考](#3-环境变量参考)
- [4. 密钥管理](#4-密钥管理)
- [5. 安全检查清单](#5-安全检查清单)
- [6. 调度中心 CLI 命令](#6-调度中心-cli-命令)
- [7. 系统运行时配置](#7-系统运行时配置)
- [8. 数据源加密密钥轮换](#8-数据源加密密钥轮换)

---

## 1. 调度中心配置

### 1.1 完整配置示例

#### 开发环境

```yaml
app:
  http_port: "8080"
  grpc_port: "50051"
  node_id: ""
  advertise_addr: ""
  allow_register: false
  cors_allow_origins: []

rsa:
  public_key: ""
  private_key: ""

sso:
  enabled: false
  url: ""
  public_key: ""
  timeout: 10

database:
  rqlite_addrs:
    - "http://localhost:4001"
  rqlite_user: ""
  rqlite_password: ""
  rqlite_tls: false

redis:
  mode: "single"
  addr: "localhost:6379"
  password: ""
  db: 0

jwt:
  secret: "dev-secret-key-change-in-production"
  expiry_hours: 24

log:
  level: "info"
  format: "json"
  path: ""

datasource:
  encryption_key: "dev-docker-32byte-key-chg-prod-1"
  key_source: "direct"
  key_env_var: "BDOPSFLOW_ENCRYPTION_KEY"
  key_file: ""
  auto_rotate_days: 0
```

#### 生产环境

```yaml
app:
  http_port: "8080"
  grpc_port: "50051"
  node_id: ""
  advertise_addr: "10.0.1.5:8080"
  allow_register: false
  cors_allow_origins:
    - "https://bdopsflow.example.com"

rsa:
  public_key: ""
  private_key: ""

sso:
  enabled: true
  url: "http://sso.example.com/account"
  public_key: ""
  timeout: 10

database:
  rqlite_addrs:
    - "http://rqlite-node1:4001"
    - "http://rqlite-node2:4001"
    - "http://rqlite-node3:4001"
  rqlite_user: ""
  rqlite_password: ""
  rqlite_tls: false

redis:
  mode: "sentinel"
  password: ""
  db: 0
  master_name: "mymaster"
  sentinel_addrs:
    - "redis-sentinel1:26379"
    - "redis-sentinel2:26379"
    - "redis-sentinel3:26379"
  sentinel_password: ""

jwt:
  secret: ""
  expiry_hours: 24

log:
  level: "warn"
  format: "json"
  path: "/var/log/bdopsflow/scheduler.log"

datasource:
  encryption_key: ""
  key_source: "env"
  key_env_var: "BDOPSFLOW_ENCRYPTION_KEY"
  key_file: ""
  auto_rotate_days: 90
```

### 1.2 配置项详解

#### app（应用配置）

| 配置项 | YAML 路径 | Go 类型 | 必填/选填 | 默认值 | 说明 |
|--------|----------|---------|----------|--------|------|
| HTTP 端口 | `app.http_port` | string | 选填 | `"8080"` | HTTP API 监听端口 |
| gRPC 端口 | `app.grpc_port` | string | 选填 | `"50051"` | gRPC 服务监听端口 |
| 节点 ID | `app.node_id` | string | 选填 | `""` | 调度中心节点标识，集群部署时每个节点必须唯一 |
| 对外宣告地址 | `app.advertise_addr` | string | 选填 | `""` | 集群部署时节点对外可达的 HTTP 地址，格式 `host:port`，类似 etcd 的 `--advertise-client-urls`。单节点部署可留空（默认使用 `127.0.0.1:<http_port>`），多节点部署时必须配置为其他节点可访问的地址，否则非主节点转发请求会失败。若仅填写主机名或 IP 未指定端口，系统将自动补全为 `app.http_port` 配置的端口号 |
| 允许注册 | `app.allow_register` | bool | 选填 | `false` | 是否允许用户自行注册 |
| CORS 来源 | `app.cors_allow_origins` | []string | 选填 | `[]` | 允许的跨域来源列表，详见下方说明 |

**CORS 来源配置说明**

`app.cors_allow_origins` 控制浏览器跨域请求（CORS）的访问策略，根据配置值不同有以下行为：

| 配置值 | 行为 | 适用场景 |
|--------|------|---------|
| `[]`（空数组/默认） | 允许所有来源，响应头设置 `Access-Control-Allow-Origin: *` | 开发环境、内网部署 |
| 指定来源列表 | 仅允许列表中的来源访问，匹配请求 `Origin` 头 | 生产环境（推荐） |

**配置示例**：

```yaml
app:
  cors_allow_origins: []
```

```yaml
app:
  cors_allow_origins:
    - "https://bdopsflow.example.com"
    - "https://ops.example.com"
```

```yaml
app:
  cors_allow_origins:
    - "http://localhost:3000"
    - "http://localhost:5173"
```

**环境变量方式**（逗号分隔）：

```bash
export APP_CORS_ALLOW_ORIGINS="https://bdopsflow.example.com,https://ops.example.com"
```

**注意事项**：

- 来源必须包含完整的协议和域名（如 `https://example.com`），可包含端口号（如 `http://localhost:3000`）
- 当配置了指定来源时，系统会根据请求的 `Origin` 头进行精确匹配，未匹配的来源将被拒绝
- 配置了指定来源时，响应头会设置 `Vary: Origin`，确保 CDN/代理正确缓存
- 生产环境强烈建议配置为具体的域名列表，避免使用空数组（允许所有来源）
- 内置 Web UI（`web.enabled: true`）不受此配置影响，前后端同源无需跨域

**advertise_addr 配置说明**

`app.advertise_addr` 用于集群部署时指定当前节点对外可达的 HTTP 地址，其他调度中心节点通过此地址将请求转发到主节点。

端口处理规则：

| 配置值 | 实际生效地址 | 说明 |
|--------|------------|------|
| `""`（空/默认） | `127.0.0.1:<http_port>` | 单节点部署默认值，仅本机可达 |
| `"10.0.1.5"` | `10.0.1.5:<http_port>` | 未指定端口时自动补全 `http_port` |
| `"10.0.1.5:8080"` | `10.0.1.5:8080` | 完整指定，直接使用 |
| `"scheduler-node1"` | `scheduler-node1:<http_port>` | 主机名同理，自动补全端口 |

注意事项：

- **必须指向调度器直接监听的 HTTP 端口**，而非 Nginx 等反向代理端口。例如调度器监听 `8080`，Nginx 代理 `80→8080`，则 `advertise_addr` 应配置为 `10.0.1.5:8080` 而非 `10.0.1.5:80`
- 如果 `advertise_addr` 中的端口与 `http_port` 不一致，启动时会打印警告日志，请确认是否配置正确
- 多节点部署时，确保各节点之间可以通过 `advertise_addr` 直接访问调度器的 HTTP API

配置示例：

```yaml
app:
  http_port: "8080"
  advertise_addr: "10.0.1.5:8080"   # 正确：指向调度器直接监听的端口
```

```yaml
app:
  http_port: "8080"
  advertise_addr: "10.0.1.5"        # 正确：自动补全为 10.0.1.5:8080
```

```yaml
app:
  http_port: "8080"
  advertise_addr: "10.0.1.5:80"     # 错误：80 是 Nginx 端口，转发请求会打到 Nginx 而非调度器
```

#### rsa（RSA 密钥配置）

| 配置项 | YAML 路径 | Go 类型 | 必填/选填 | 默认值 | 说明 |
|--------|----------|---------|----------|--------|------|
| 公钥 | `rsa.public_key` | string | 选填 | `""` | 本地 RSA 公钥（PKCS#8，Base64），用于前端加密登录密码 |
| 私钥 | `rsa.private_key` | string | 选填 | `""` | 本地 RSA 私钥（PKCS#8，Base64），用于后端解密登录密码 |

使用 `./scheduler keygen` 命令生成密钥对。

#### sso（SSO 登录配置）

| 配置项 | YAML 路径 | Go 类型 | 必填/选填 | 默认值 | 说明 |
|--------|----------|---------|----------|--------|------|
| 启用 SSO | `sso.enabled` | bool | 选填 | `false` | 是否启用 SSO 登录 |
| SSO 地址 | `sso.url` | string | 条件必填 | `""` | SSO 验证接口地址，启用 SSO 时必填 |
| SSO 公钥 | `sso.public_key` | string | 条件必填 | `""` | SSO RSA 公钥（PKCS#8，Base64），启用 SSO 时必填 |
| 超时时间 | `sso.timeout` | int | 选填 | `10` | SSO 请求超时时间（秒），最小 1 秒 |

> **注意**：`sso.public_key` 与 `rsa.public_key` 是两套独立的密钥。SSO 公钥加密的密码后端不解密，原样转发给 SSO 服务验证。

#### database（数据库配置）

| 配置项 | YAML 路径 | Go 类型 | 必填/选填 | 默认值 | 说明 |
|--------|----------|---------|----------|--------|------|
| rqlite 地址 | `database.rqlite_addrs` | []string | 选填 | `["http://localhost:4001"]` | rqlite 节点地址列表，支持多节点 |
| rqlite 用户名 | `database.rqlite_user` | string | 选填 | `""` | rqlite 认证用户名 |
| rqlite 密码 | `database.rqlite_password` | string | 选填 | `""` | rqlite 认证密码 |
| rqlite TLS | `database.rqlite_tls` | bool | 选填 | `false` | 是否使用 TLS 连接 rqlite |

#### redis（Redis 配置）

| 配置项 | YAML 路径 | Go 类型 | 必填/选填 | 默认值 | 说明 |
|--------|----------|---------|----------|--------|------|
| 模式 | `redis.mode` | string | 选填 | `"single"` | Redis 模式：`single` 或 `sentinel` |
| 地址 | `redis.addr` | string | 选填 | `"localhost:6379"` | Redis 单实例地址（single 模式） |
| 密码 | `redis.password` | string | 选填 | `""` | Redis 密码 |
| 数据库 | `redis.db` | int | 选填 | `0` | Redis 数据库编号 |
| 主节点名称 | `redis.master_name` | string | 选填 | `"mymaster"` | Sentinel 主节点名称（sentinel 模式） |
| Sentinel 地址 | `redis.sentinel_addrs` | []string | 条件必填 | `[]` | Sentinel 节点地址列表（sentinel 模式必填） |
| Sentinel 密码 | `redis.sentinel_password` | string | 选填 | `""` | Sentinel 节点密码 |

**Redis 模式说明**：

- `single`：单实例模式，使用 `redis.addr` 连接
- `sentinel`：哨兵模式，使用 `redis.sentinel_addrs` 连接 Sentinel，通过 `redis.master_name` 发现主节点

#### jwt（JWT 配置）

| 配置项 | YAML 路径 | Go 类型 | 必填/选填 | 默认值 | 说明 |
|--------|----------|---------|----------|--------|------|
| 密钥 | `jwt.secret` | string | 必填 | `"your-secret-key-change-in-production"` | JWT 签名密钥，生产环境必须修改 |
| 过期时间 | `jwt.expiry_hours` | int | 选填 | `24` | Token 过期时间（小时） |

#### log（日志配置）

| 配置项 | YAML 路径 | Go 类型 | 必填/选填 | 默认值 | 说明 |
|--------|----------|---------|----------|--------|------|
| 日志级别 | `log.level` | string | 选填 | `"info"` | 日志级别：`debug`、`info`、`warn`、`error` |
| 日志格式 | `log.format` | string | 选填 | `"json"` | 日志格式：`json` 或 `text` |
| 日志文件路径 | `log.path` | string | 选填 | `""` | 日志输出文件路径，为空时输出到标准输出（stdout）。配置后支持通过 `kill -HUP` 信号重新打开日志文件，配合 logrotate 使用 |

**日志文件配置说明**

当 `log.path` 配置为非空值时，日志将输出到指定文件。这对于生产环境部署非常有用，可以配合 logrotate 工具进行日志轮转。

配置示例：

```yaml
log:
  level: "info"
  format: "json"
  path: "/var/log/bdopsflow/scheduler.log"
```

**SIGHUP 信号处理**

调度中心支持通过 `kill -HUP` 信号触发以下操作：

1. **重新加载配置文件**：重新读取配置文件中的配置项
2. **重新打开日志文件**：关闭旧的日志文件句柄，打开新的日志文件

这使得 logrotate 可以正常工作，示例 logrotate 配置：

```
/var/log/bdopsflow/scheduler.log {
    daily
    rotate 7
    compress
    missingok
    postrotate
        systemctl reload bdopsflow-scheduler
    endscript
}
```

**systemd 服务配置示例**

创建 `/etc/systemd/system/bdopsflow-scheduler.service`：

```ini
[Unit]
Description=BDopsFlow Scheduler
After=network.target
After=rqlite.service
After=redis.service

[Service]
Type=simple
User=bdopsflow
Group=bdopsflow
WorkingDirectory=/opt/bdopsflow
ExecStart=/opt/bdopsflow/scheduler -config /etc/bdopsflow/config.yaml
Restart=always
RestartSec=5
StandardOutput=null
StandardError=journal+console

[Install]
WantedBy=multi-user.target
```

**注意**：
- 启用服务并设置开机自启：
  ```bash
  systemctl daemon-reload
  systemctl enable bdopsflow-scheduler
  systemctl start bdopsflow-scheduler
  ```
- 使用 `systemctl reload bdopsflow-scheduler` 可以触发配置重载（systemd 会自动发送 SIGHUP 信号）

**注意**：通过 SIGHUP 重新加载的配置项包括：
- `log.level` - 日志级别
- `log.format` - 日志格式  
- `log.path` - 日志文件路径
- `app.http_port` - HTTP 端口（不支持热更新，仅记录日志）
- `app.grpc_port` - gRPC 端口（不支持热更新，仅记录日志）
- `app.advertise_addr` - 对外宣告地址
- `app.allow_register` - 允许注册
- `app.cors_allow_origins` - CORS 来源列表

#### datasource（数据源加密配置）

| 配置项 | YAML 路径 | Go 类型 | 必填/选填 | 默认值 | 说明 |
|--------|----------|---------|----------|--------|------|
| 加密密钥 | `datasource.encryption_key` | string | 选填 | `"change-in-prod-32byte-key1-here1"` | AES-256-GCM 加密密钥，必须为 32 字节 |
| 密钥来源 | `datasource.key_source` | string | 选填 | `"direct"` | 密钥获取方式：`direct`、`env`、`file` |
| 环境变量名 | `datasource.key_env_var` | string | 选填 | `"BDOPSFLOW_ENCRYPTION_KEY"` | 密钥环境变量名（key_source=env 时使用） |
| 密钥文件路径 | `datasource.key_file` | string | 选填 | `""` | 密钥文件路径（key_source=file 时使用） |
| 自动轮换天数 | `datasource.auto_rotate_days` | int | 选填 | `0` | 密钥自动轮换天数，0 表示不轮换 |

**密钥来源说明**：

| 来源 | 说明 | 使用场景 |
|------|------|---------|
| `direct` | 直接从配置文件读取 `encryption_key` | 开发环境 |
| `env` | 从环境变量读取，变量名由 `key_env_var` 指定 | 生产环境（推荐） |
| `file` | 从文件读取，路径由 `key_file` 指定 | 生产环境（Kubernetes Secret） |

#### 其他内部字段

| 配置项 | YAML 路径 | Go 类型 | 必填/选填 | 默认值 | 说明 |
|--------|----------|---------|----------|--------|------|
| 配置文件路径 | `-config`（CLI 参数） | string | 选填 | `""` | 配置文件路径，通过 CLI `-config` 参数传入，非 YAML 配置项 |

---

## 2. 执行器配置

### 2.1 完整配置示例

```yaml
app:
  executor_name: "executor-1"
  capacity: 10
  hostname: ""

scheduler:
  addr: "localhost:50051"
  addrs: ""
  timeout: 30

log:
  level: "info"
  format: "json"
```

### 2.2 配置项详解

#### app（应用配置）

| 配置项 | YAML 路径 | Go 类型 | 必填/选填 | 默认值 | 说明 |
|--------|----------|---------|----------|--------|------|
| 执行器名称 | `app.executor_name` | string | 必填 | `""` | 执行器唯一标识，必填 |
| 容量 | `app.capacity` | int32 | 选填 | `10` | 最大并发任务数 |
| 主机名 | `app.hostname` | string | 选填 | 系统主机名 | 执行器注册地址，默认自动检测系统主机名，可通过配置或命令行参数覆盖 |

#### scheduler（调度器连接配置）

| 配置项 | YAML 路径 | Go 类型 | 必填/选填 | 默认值 | 说明 |
|--------|----------|---------|----------|--------|------|
| 单调度器地址 | `scheduler.addr` | string | 条件必填 | `""` | 单个调度器 gRPC 地址（向后兼容） |
| 多调度器地址 | `scheduler.addrs` | string | 条件必填 | `""` | 多个调度器 gRPC 地址（逗号分隔），优先于 `addr` |
| 超时时间 | `scheduler.timeout` | int | 选填 | `30` | gRPC 连接超时时间（秒） |

> **注意**：同时配置 `addr` 和 `addrs` 时，`addrs` 优先。`addrs` 和 `addr` 至少配置一个，否则验证失败。

#### log（日志配置）

| 配置项 | YAML 路径 | Go 类型 | 必填/选填 | 默认值 | 说明 |
|--------|----------|---------|----------|--------|------|
| 日志级别 | `log.level` | string | 选填 | `"info"` | 日志级别：`debug`、`info`、`warn`、`error` |
| 日志格式 | `log.format` | string | 选填 | `"json"` | 日志格式：`json` 或 `text` |

#### 其他内部字段

| 配置项 | YAML 路径 | Go 类型 | 必填/选填 | 默认值 | 说明 |
|--------|----------|---------|----------|--------|------|
| 配置文件路径 | `--config`（CLI 参数） | string | 选填 | `""` | 配置文件路径，通过 CLI `--config` 参数传入，非 YAML 配置项 |

### 2.3 命令行参数

命令行参数优先级高于配置文件，用于覆盖配置文件中的值。

| 参数 | 覆盖配置项 | Go 类型 | 默认值 | 说明 |
|------|----------|---------|--------|------|
| `--config` | - | string | `""` | 配置文件路径 |
| `--executor-name` | `app.executor_name` | string | - | 执行器名称（必需） |
| `--scheduler-addr` | `scheduler.addr` | string | - | 调度器 gRPC 地址（单个，向后兼容） |
| `--scheduler-addrs` | `scheduler.addrs` | string | - | 调度器 gRPC 地址（逗号分隔，多个调度器） |
| `--capacity` | `app.capacity` | int | `10` | 并发任务数 |
| `--timeout` | `scheduler.timeout` | int | `30` | 超时时间（秒） |
| `--hostname` | `app.hostname` | string | 系统主机名 | 覆盖主机名或 IP，用于执行器注册 |
| `--log-level` | `log.level` | string | `"info"` | 日志级别：debug, info, warn, error |
| `--log-format` | `log.format` | string | `"json"` | 日志格式：json, text |

**使用示例**：

```bash
executor --executor-name my-exec --scheduler-addr localhost:50051
executor --executor-name my-exec --scheduler-addrs host1:50051,host2:50051 --capacity 20
```

### 2.4 配置方法说明

执行器配置提供以下关键方法，用于地址解析、校验和合并：

#### GetSchedulerAddresses()

获取调度器地址列表，优先级逻辑：

1. 若 `SchedulerAddrs` 非空，返回 `SchedulerAddrs`（多调度器地址）
2. 若 `SchedulerAddr` 非空，返回 `[SchedulerAddr]`（单调度器地址）
3. 两者均为空，返回 `nil`

```go
addrs := cfg.GetSchedulerAddresses()
// SchedulerAddrs > SchedulerAddr
```

#### Validate()

校验配置是否满足最低要求：

- `executor_name` 不能为空
- 至少配置一个调度器地址（`scheduler.addr` 或 `scheduler.addrs`）

校验失败时返回 `RequiredError`，包含缺失字段名。

#### Merge()

将命令行参数合并到配置中，命令行参数优先级高于配置文件。合并规则：

- 仅当命令行参数非零值时才覆盖配置文件值
- `string` 类型：非空字符串覆盖
- `int32`/`int` 类型：大于 0 时覆盖
- `[]string` 类型：非空切片覆盖

```go
cfg.Merge(executorName, capacity, schedulerAddr, schedulerAddrs, timeout, hostname, logLevel, logFormat)
```

---

## 3. 环境变量参考

### 3.1 调度中心环境变量

每个配置项都可以通过环境变量覆盖，环境变量名与 YAML 路径对应（大写，下划线分隔）。

| 环境变量 | 对应配置 | Go 类型 | 默认值 |
|---------|---------|---------|--------|
| `APP_HTTP_PORT` | `app.http_port` | string | `"8080"` |
| `APP_GRPC_PORT` | `app.grpc_port` | string | `"50051"` |
| `APP_NODE_ID` | `app.node_id` | string | `""` |
| `APP_ADVERTISE_ADDR` | `app.advertise_addr` | string | `""` |
| `APP_ALLOW_REGISTER` | `app.allow_register` | bool | `false` |
| `APP_CORS_ALLOW_ORIGINS` | `app.cors_allow_origins` | []string | `[]` |
| `RSA_PUBLIC_KEY` | `rsa.public_key` | string | `""` |
| `RSA_PRIVATE_KEY` | `rsa.private_key` | string | `""` |
| `SSO_ENABLED` | `sso.enabled` | bool | `false` |
| `SSO_URL` | `sso.url` | string | `""` |
| `SSO_PUBLIC_KEY` | `sso.public_key` | string | `""` |
| `SSO_TIMEOUT` | `sso.timeout` | int | `10` |
| `DATABASE_RQLITE_ADDRS` | `database.rqlite_addrs` | []string | `["http://localhost:4001"]` |
| `DATABASE_RQLITE_USER` | `database.rqlite_user` | string | `""` |
| `DATABASE_RQLITE_PASSWORD` | `database.rqlite_password` | string | `""` |
| `DATABASE_RQLITE_TLS` | `database.rqlite_tls` | bool | `false` |
| `REDIS_MODE` | `redis.mode` | string | `"single"` |
| `REDIS_ADDR` | `redis.addr` | string | `"localhost:6379"` |
| `REDIS_PASSWORD` | `redis.password` | string | `""` |
| `REDIS_DB` | `redis.db` | int | `0` |
| `REDIS_MASTER_NAME` | `redis.master_name` | string | `"mymaster"` |
| `REDIS_SENTINEL_ADDRS` | `redis.sentinel_addrs` | []string | `[]` |
| `REDIS_SENTINEL_PASSWORD` | `redis.sentinel_password` | string | `""` |
| `JWT_SECRET` | `jwt.secret` | string | `"your-secret-key-change-in-production"` |
| `JWT_EXPIRY_HOURS` | `jwt.expiry_hours` | int | `24` |
| `LOG_LEVEL` | `log.level` | string | `"info"` |
| `LOG_FORMAT` | `log.format` | string | `"json"` |
| `LOG_PATH` | `log.path` | string | `""` |
| `DATASOURCE_ENCRYPTION_KEY` | `datasource.encryption_key` | string | `"change-in-prod-32byte-key1-here1"` |
| `DATASOURCE_KEY_SOURCE` | `datasource.key_source` | string | `"direct"` |
| `DATASOURCE_KEY_ENV_VAR` | `datasource.key_env_var` | string | `"BDOPSFLOW_ENCRYPTION_KEY"` |
| `DATASOURCE_KEY_FILE` | `datasource.key_file` | string | `""` |
| `DATASOURCE_AUTO_ROTATE_DAYS` | `datasource.auto_rotate_days` | int | `0` |

> **注意**：`DATASOURCE_ENCRYPTION_KEY` 是通用环境变量覆盖机制，直接覆盖 `datasource.encryption_key` 配置值。而 `BDOPSFLOW_ENCRYPTION_KEY` 是 `datasource.key_env_var` 的默认值，当 `key_source: "env"` 时，系统会从该环境变量名读取加密密钥。两者是不同的机制：前者是配置框架的通用覆盖，后者是数据源加密的专用密钥来源。

### 3.2 执行器环境变量

| 环境变量 | 对应配置 | Go 类型 | 默认值 |
|---------|---------|---------|--------|
| `APP_EXECUTOR_NAME` | `app.executor_name` | string | `""` |
| `APP_CAPACITY` | `app.capacity` | int32 | `10` |
| `APP_HOSTNAME` | `app.hostname` | string | 系统主机名 |
| `SCHEDULER_ADDR` | `scheduler.addr` | string | `""` |
| `SCHEDULER_ADDRS` | `scheduler.addrs` | string | `""` |
| `SCHEDULER_TIMEOUT` | `scheduler.timeout` | int | `30` |
| `LOG_LEVEL` | `log.level` | string | `"info"` |
| `LOG_FORMAT` | `log.format` | string | `"json"` |

### 3.3 数组类型环境变量

对于 `[]string` 类型的配置项，环境变量使用逗号分隔：

```bash
export DATABASE_RQLITE_ADDRS="http://node1:4001,http://node2:4001,http://node3:4001"
export REDIS_SENTINEL_ADDRS="sentinel1:26379,sentinel2:26379,sentinel3:26379"
export APP_CORS_ALLOW_ORIGINS="https://app1.example.com,https://app2.example.com"
```

---

## 4. 密钥管理

### 4.1 密钥类型概览

BDopsFlow 涉及以下密钥：

| 密钥 | 用途 | 存储位置 | 算法 |
|------|------|---------|------|
| JWT 密钥 | Token 签名验证 | 配置文件或环境变量 | HMAC-SHA256 |
| 数据源加密密钥 | 数据源密码加密 | 配置文件/环境变量/文件 | AES-256-GCM |
| RSA 密钥对 | 本地登录密码加解密 | 配置文件 | RSA-PKCS#8 |
| SSO 公钥 | SSO 登录密码加密 | 配置文件 | RSA-PKCS#8 |
| Redis 密码 | Redis 认证 | 配置文件 | - |
| rqlite 密码 | rqlite 认证 | 配置文件 | - |
| Webhook Secret | HMAC 签名验证 | 数据库 | HMAC-SHA256 |

### 4.2 RSA 密钥对生成

使用调度中心内置命令生成 RSA 密钥对：

```bash
./scheduler keygen
```

生成的密钥为 PKCS#8 格式，Base64 编码，不含 PEM 头尾。将输出的公钥和私钥分别填入 `rsa.public_key` 和 `rsa.private_key`。

### 4.3 数据源加密密钥管理

#### 密钥来源

| 来源 | 配置 | 适用场景 |
|------|------|---------|
| 直接配置 | `key_source: "direct"` | 开发环境，密钥直接写在配置文件中 |
| 环境变量 | `key_source: "env"` | 生产环境（推荐），从环境变量读取 |
| 文件 | `key_source: "file"` | 生产环境，从文件读取（如 Kubernetes Secret 挂载） |

#### 密钥轮换

配置 `auto_rotate_days` 启用自动轮换：

- `0`：不轮换（默认）
- `90`：每 90 天轮换一次（生产环境推荐）

轮换流程：

1. 系统生成新的 32 字节随机密钥
2. 使用新密钥重新加密所有数据源密码
3. 更新当前活跃密钥
4. 保留旧密钥用于解密历史数据

#### 密钥要求

- 必须为 32 字节（AES-256）
- 生产环境必须修改默认值 `change-in-prod-32byte-key1-here1`
- 推荐使用密码学安全的随机数生成器

### 4.4 JWT 密钥管理

#### 密钥要求

- 生产环境必须修改默认值 `your-secret-key-change-in-production`
- 使用至少 32 字符的随机字符串
- 建议通过环境变量 `JWT_SECRET` 传入

#### 生成建议

```bash
openssl rand -base64 48
```

#### 密钥轮换

JWT 密钥轮换会导致所有已发放的 Token 失效，用户需要重新登录。建议：

1. 在低峰期进行轮换
2. 提前通知用户
3. 新旧密钥交替使用过渡期（需代码支持）

### 4.5 SSO 公钥管理

- SSO 公钥由 SSO 服务方提供，格式为 PKCS#8 Base64 编码
- 与本地 RSA 密钥对完全独立
- SSO 公钥变更时需同步更新配置

### 4.6 Kubernetes 部署密钥管理

使用 Kubernetes Secret 管理敏感配置：

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: bdopsflow-secrets
type: Opaque
stringData:
  jwt-secret: "your-production-jwt-secret-at-least-32-chars"
  encryption-key: "your-32byte-encryption-key-here1"
  redis-password: "your-redis-password"
  rqlite-password: "your-rqlite-password"
```

在 Deployment 中引用：

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bdopsflow-scheduler
spec:
  template:
    spec:
      containers:
        - name: scheduler
          envFrom:
            - secretRef:
                name: bdopsflow-secrets
          volumeMounts:
            - name: encryption-key
              mountPath: /etc/bdopsflow/keys
      volumes:
        - name: encryption-key
          secret:
            secretName: bdopsflow-secrets
            items:
              - key: encryption-key
                path: encryption.key
```

对应配置：

```yaml
datasource:
  key_source: "file"
  key_file: "/etc/bdopsflow/keys/encryption.key"
```

---

## 5. 安全检查清单

### 5.1 生产环境部署前检查

#### 密钥与认证

| 检查项 | 说明 | 状态 |
|--------|------|------|
| JWT 密钥已修改 | 默认值 `your-secret-key-change-in-production` 已替换为强随机字符串 | ☐ |
| 数据源加密密钥已修改 | 默认值 `change-in-prod-32byte-key1-here1` 已替换 | ☐ |
| 数据源密钥来源为 env 或 file | 生产环境不使用 `key_source: "direct"` | ☐ |
| RSA 密钥对已生成 | 使用 `./scheduler keygen` 生成，未使用默认密钥 | ☐ |
| Redis 密码已设置 | `redis.password` 不为空 | ☐ |
| JWT 过期时间合理 | `jwt.expiry_hours` 不超过 24 小时 | ☐ |
| 用户注册已关闭 | `app.allow_register` 为 `false` | ☐ |

#### 网络安全

| 检查项 | 说明 | 状态 |
|--------|------|------|
| CORS 来源已限制 | `app.cors_allow_origins` 不为空数组，仅包含合法域名 | ☐ |
| Redis 端口不对外暴露 | Redis 仅内网可访问 | ☐ |
| rqlite 端口不对外暴露 | rqlite 仅内网可访问 | ☐ |
| gRPC 端口仅对执行器开放 | 执行器通过内网连接调度中心 | ☐ |
| HTTPS 已启用 | 前端和 API 使用 HTTPS | ☐ |
| rqlite TLS 已启用 | `database.rqlite_tls` 为 `true`（如需加密传输） | ☐ |

#### 权限与审计

| 检查项 | 说明 | 状态 |
|--------|------|------|
| 默认管理员密码已修改 | `admin/admin123` 已修改 | ☐ |
| 审计日志功能正常 | 写操作均记录审计日志 | ☐ |
| 审计日志保留天数合理 | 至少 90 天 | ☐ |
| 数据源权限已配置 | 数据源查询权限按需分配 | ☐ |
| allow_write_sql 谨慎开启 | 仅必要时开启数据源写操作 | ☐ |
| api_test.allow_private_network 谨慎开启 | 仅在可信环境中开启接口测试内网访问，防止 SSRF 攻击 | ☐ |

#### 日志与监控

| 检查项 | 说明 | 状态 |
|--------|------|------|
| 日志级别为 warn 或 info | 生产环境不使用 `debug` 级别 | ☐ |
| 日志格式为 json | 便于日志收集和分析 | ☐ |
| 日志中不包含敏感信息 | 密码、密钥等不出现在日志中 | ☐ |

#### 高可用

| 检查项 | 说明 | 状态 |
|--------|------|------|
| Redis 使用哨兵模式 | `redis.mode` 为 `sentinel` | ☐ |
| rqlite 多节点部署 | 至少 3 个 rqlite 节点 | ☐ |
| 调度中心多节点部署 | 至少 2 个调度中心节点 | ☐ |
| node_id 已配置 | 每个调度中心节点有唯一的 `node_id` | ☐ |
| advertise_addr 已配置 | 多节点部署时 `app.advertise_addr` 已配置为其他节点可达的地址，避免转发请求到 `127.0.0.1` | ☐ |
| 数据源密钥轮换已启用 | `auto_rotate_days` 大于 0 | ☐ |

### 5.2 SSO 配置检查

| 检查项 | 说明 | 状态 |
|--------|------|------|
| SSO URL 可达 | 调度中心后端可访问 SSO 服务 | ☐ |
| SSO 公钥正确 | 使用 SSO 服务方提供的最新公钥 | ☐ |
| SSO 超时合理 | `sso.timeout` 不小于 5 秒 | ☐ |
| SSO 用户权限已配置 | SSO 自动创建的用户有合适的权限 | ☐ |

### 5.3 Webhook 安全检查

| 检查项 | 说明 | 状态 |
|--------|------|------|
| Webhook 使用 HTTPS | 回调 URL 使用 HTTPS 协议 | ☐ |
| Webhook Secret 已配置 | 启用 HMAC 签名验证 | ☐ |
| Webhook 目标地址受控 | 限制回调目标地址范围 | ☐ |

### 5.4 定期安全审查

| 审查项 | 频率 | 说明 |
|--------|------|------|
| 审计日志审查 | 每周 | 检查异常操作和权限变更 |
| 密钥轮换 | 每 90 天 | 数据源加密密钥自动轮换 |
| 用户权限审查 | 每月 | 检查用户角色和权限分配 |
| 数据源权限审查 | 每月 | 检查数据源访问权限 |
| 依赖安全更新 | 每月 | 检查并更新有安全漏洞的依赖 |
| 密码策略审查 | 每季度 | 检查弱密码用户 |

---

## 6. 调度中心 CLI 命令

调度中心二进制文件 `scheduler` 支持以下子命令：

### 6.1 启动调度器

```bash
./scheduler                    # 使用默认配置启动
./scheduler -config my.yml     # 使用指定配置文件启动
./scheduler -advertise-addr 10.0.1.5:8080  # 指定集群对外宣告地址
./scheduler -config my.yml -advertise-addr 10.0.1.5:8080  # 同时指定配置文件和宣告地址
```

**命令行参数**：

| 参数 | 覆盖配置项 | Go 类型 | 默认值 | 说明 |
|------|----------|---------|--------|------|
| `-config` | - | string | `""` | 配置文件路径 |
| `-advertise-addr` | `app.advertise_addr` | string | `""` | 集群部署时节点对外可达的 HTTP 地址（格式 `host:port`），优先级高于配置文件 |

### 6.2 生成 RSA 密钥对

```bash
./scheduler keygen
```

生成 PKCS#8 格式的 RSA 密钥对，输出 Base64 编码的公钥和私钥，可直接复制到配置文件的 `rsa.public_key` 和 `rsa.private_key` 字段。

输出示例：

```yaml
rsa:
  public_key: "MIIBIjANBg..."
  private_key: "MIIEvgIBAD..."
```

### 6.3 加密密码

```bash
./scheduler encrypt-password --config <config_file> --password <password>
```

使用配置文件中的 RSA 公钥加密密码，输出格式为 `RSA_ENCRYPTED:<ciphertext>`。用于手动加密需要存储的密码。

参数说明：

| 参数 | 必填 | 说明 |
|------|------|------|
| `--config` | 是 | 配置文件路径，需包含 `rsa.public_key` |
| `--password` | 是 | 待加密的明文密码 |

### 6.4 解密密码

```bash
./scheduler decrypt-password --config <config_file> --ciphertext <ciphertext>
```

使用配置文件中的 RSA 私钥解密密码。`ciphertext` 可带或不带 `RSA_ENCRYPTED:` 前缀。

参数说明：

| 参数 | 必填 | 说明 |
|------|------|------|
| `--config` | 是 | 配置文件路径，需包含 `rsa.private_key` |
| `--ciphertext` | 是 | 加密后的密文，支持 `RSA_ENCRYPTED:` 前缀 |

### 6.5 帮助

```bash
./scheduler help
./scheduler -h
./scheduler --help
```

---

## 7. 系统运行时配置

系统运行时配置通过 API 管理，存储在 rqlite 数据库中，无需重启服务即可生效。配置每 5 分钟自动从数据库重新加载。

### 7.1 查询配置

| Key | 类型 | 默认值 | 范围 | 单位 | 说明 |
|-----|------|--------|------|------|------|
| `datasource.default_limit` | number | 1000 | 1 - 100000 | 行 | 默认查询返回行数，限制单次查询结果集大小 |
| `datasource.max_export_rows` | number | 1000 | 1 - 1000000 | 行 | CSV 导出最大行数，超过此限制将截断结果 |
| `datasource.query_timeout` | number | 60 | 1 - 3600 | 秒 | 查询超时秒数，超时后自动取消查询 |
| `datasource.max_sql_length` | number | 65536 | 1024 - 1048576 | 字符 | SQL 最大长度，防止超长 SQL 影响性能 |

### 7.2 并发配置

| Key | 类型 | 默认值 | 范围 | 单位 | 说明 |
|-----|------|--------|------|------|------|
| `datasource.max_concurrent_per_user` | number | 5 | 1 - 50 | 个 | 单用户最大并发查询数，超过限制将排队等待 |
| `datasource.max_concurrent_global` | number | 50 | 1 - 500 | 个 | 全局最大并发查询数，超过限制将排队等待 |

### 7.3 安全配置

| Key | 类型 | 默认值 | 范围 | 单位 | 说明 |
|-----|------|--------|------|------|------|
| `datasource.allow_write_sql` | bool | false | - | - | 是否允许写操作 SQL（INSERT/UPDATE/DELETE），全局兜底控制，每个数据源可独立设置 DML 权限 |
| `datasource.max_cell_size` | number | 65536 | 1024 - 10485760 | 字节 | 单元格最大大小，超过此大小将截断显示 |
| `api_test.allow_private_network` | bool | false | - | - | 是否允许接口测试访问内网（私有 IP）地址。开启后 HTTP/gRPC 执行器可访问内网地址；关闭时仅允许访问公网地址，防止 SSRF 攻击。默认关闭 |

### 7.4 缓存配置

| Key | 类型 | 默认值 | 范围 | 单位 | 说明 |
|-----|------|--------|------|------|------|
| `datasource.cache_ttl` | number | 300 | 0 - 86400 | 秒 | 查询缓存 TTL，数据源元数据（表结构、列信息等）缓存的存活时间 |
| `datasource.cache_max_size` | number | 100 | 1 - 10000 | 条 | 缓存最大条目数，超过后采用 LRU 淘汰策略 |

### 7.5 连接池配置

| Key | 类型 | 默认值 | 范围 | 单位 | 说明 |
|-----|------|--------|------|------|------|
| `datasource.connection_max_idle` | number | 5 | 1 - 100 | 个 | 最大空闲连接数，每个数据源连接池中允许保持的最大空闲连接 |
| `datasource.connection_max_open` | number | 10 | 1 - 200 | 个 | 最大打开连接数，包括活跃和空闲连接 |
| `datasource.connection_max_lifetime` | number | 1800 | 60 - 86400 | 秒 | 连接最大生命周期，超时后连接将被关闭并重建 |
| `datasource.health_check_interval` | number | 300 | 30 - 3600 | 秒 | 健康检查间隔，定期检测数据源连接是否可用 |
| `datasource.test_timeout` | number | 10 | 1 - 120 | 秒 | 连接测试超时，超时未响应视为连接失败 |

### 7.6 其他配置

| Key | 类型 | 默认值 | 范围 | 单位 | 说明 |
|-----|------|--------|------|------|------|
| `datasource.history_retention_days` | number | 30 | 1 - 365 | 天 | 查询历史保留天数，超过此天数的记录将被自动清理 |

### 7.7 系统配置

| Key | 类型 | 默认值 | 说明 |
|-----|------|--------|------|
| `web.enabled` | bool | false | 是否启用内置 Web UI，启用后可通过调度器监听端口直接访问 Web UI，无需单独部署前端 |

---

## 8. 数据源加密密钥轮换

### 8.1 轮换概述

数据源加密密钥用于加密存储在数据库中的数据源连接密码（AES-256-GCM）。定期轮换密钥是安全最佳实践，可降低密钥泄露风险。

### 8.2 手动轮换流程

1. **生成新加密密钥**

   生成一个 32 字节的密码学安全随机密钥：

   ```bash
   openssl rand -base64 32 | head -c 32
   ```

2. **更新配置中的新密钥**

   推荐使用 `key_source: "env"` 方式，通过环境变量传入新密钥：

   ```bash
   export BDOPSFLOW_ENCRYPTION_KEY="<new-32byte-key>"
   ```

   或更新配置文件：

   ```yaml
   datasource:
     encryption_key: "<new-32byte-key>"
   ```

3. **重启调度中心**

   重启后，调度中心会使用新密钥。所有旧的加密值将在读取时自动使用旧密钥解密，并在写入时使用新密钥重新加密。

4. **验证所有数据源正常**

   - 逐一测试每个数据源的连接
   - 确认查询功能正常
   - 检查日志中是否有解密错误

5. **确认轮换完成**

   - 所有数据源密码已使用新密钥加密
   - 旧密钥不再需要

### 8.3 自动轮换提醒

配置 `auto_rotate_days > 0` 可启用自动轮换提醒：

```yaml
datasource:
  auto_rotate_days: 90
```

- 系统会在密钥使用超过指定天数后发出轮换提醒
- `auto_rotate_days: 0` 表示不启用自动提醒（默认）
- 生产环境推荐设置为 `90`（每 90 天提醒一次）

> **注意**：`auto_rotate_days` 仅提供提醒功能，不会自动执行密钥轮换。轮换操作仍需管理员手动完成。

### 8.4 轮换注意事项

- **避免并发轮换**：同一时间只允许一个轮换操作，避免数据不一致
- **备份旧密钥**：轮换前备份当前密钥，以防需要回滚
- **低峰期操作**：建议在业务低峰期进行轮换，减少对在线业务的影响
- **多节点同步**：集群部署时，确保所有节点使用相同的新密钥后再依次重启
- **密钥来源推荐**：生产环境推荐使用 `key_source: "env"` 或 `key_source: "file"`，避免密钥明文存储在配置文件中

---

## 9. 集群部署指南

### 9.1 集群架构介绍

BDopsFlow 支持高可用集群部署，主要组件包括：

```
┌─────────────────────────────────────────────────────────────┐
│                        Nginx 负载均衡                          │
│                         192.168.1.50                          │
└─────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
┌───────▼───────┐    ┌────────▼───────┐    ┌────────▼───────┐
│  Scheduler-1  │    │  Scheduler-2   │    │  Scheduler-3   │
│ 192.168.1.10  │    │ 192.168.1.11   │    │ 192.168.1.12   │
└───────────────┘    └───────────────┘    └───────────────┘
        │                     │                     │
        └─────────────────────┼─────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
┌───────▼───────┐    ┌────────▼───────┐    ┌────────▼───────┐
│ rqlite-node1  │    │ rqlite-node2   │    │ rqlite-node3   │
│ 192.168.1.100 │    │ 192.168.1.101  │    │ 192.168.1.102  │
└───────────────┘    └───────────────┘    └───────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
┌───────▼───────┐    ┌────────▼───────┐    ┌────────▼───────┐
│ Sentinel-1    │    │ Sentinel-2     │    │ Sentinel-3     │
│ 192.168.1.200 │    │ 192.168.1.201  │    │ 192.168.1.202  │
└───────────────┘    └───────────────┘    └───────────────┘
        │                     │                     │
        └─────────────────────┼─────────────────────┘
                              │
                     ┌────────▼───────┐
                     │  Redis Master  │
                     │  (自动切换)     │
                     └────────────────┘
```

**集群特性**：
- **调度中心**：多个节点通过 leader 选举实现高可用，同一时间只有一个节点作为 leader 执行任务调度
- **rqlite**：分布式数据库，3 节点保证数据一致性和高可用
- **Redis**：哨兵模式实现自动故障转移
- **Nginx**：负载均衡和反向代理

### 9.2 多节点调度中心部署

#### 节点 1 配置 (scheduler-1)

```yaml
app:
  http_port: "8080"
  grpc_port: "50051"
  node_id: "scheduler-1"
  advertise_addr: "192.168.1.10:8080"
  allow_register: false
  cors_allow_origins:
    - "https://bdopsflow.example.com"

database:
  rqlite_addrs:
    - "http://192.168.1.100:4001"
    - "http://192.168.1.101:4001"
    - "http://192.168.1.102:4001"

redis:
  mode: "sentinel"
  password: "your-redis-password"
  db: 0
  master_name: "mymaster"
  sentinel_addrs:
    - "192.168.1.200:26379"
    - "192.168.1.201:26379"
    - "192.168.1.202:26379"
  sentinel_password: ""

log:
  level: "info"
  format: "json"
  path: "/var/log/bdopsflow/scheduler.log"
```

#### 节点 2 配置 (scheduler-2)

```yaml
app:
  http_port: "8080"
  grpc_port: "50051"
  node_id: "scheduler-2"
  advertise_addr: "192.168.1.11:8080"
  allow_register: false
  cors_allow_origins:
    - "https://bdopsflow.example.com"

# 其他配置与节点 1 相同
```

#### 节点 3 配置 (scheduler-3)

```yaml
app:
  http_port: "8080"
  grpc_port: "50051"
  node_id: "scheduler-3"
  advertise_addr: "192.168.1.12:8080"
  allow_register: false
  cors_allow_origins:
    - "https://bdopsflow.example.com"

# 其他配置与节点 1 相同
```

**关键配置说明**：
- `node_id`：每个节点必须唯一，用于 leader 选举和标识
- `advertise_addr`：必须配置为其他节点可访问的地址，格式为 `host:port`
- 所有节点的 `rqlite_addrs` 和 `redis` 配置必须相同

### 9.3 rqlite 3 节点集群配置

#### rqlite 节点 1 配置

创建 `/etc/rqlite/config.yaml`：

```yaml
http-addr: "0.0.0.0:4001"
raft-addr: "0.0.0.0:4002"
join:
  - "192.168.1.100:4002"
  - "192.168.1.101:4002"
  - "192.168.1.102:4002"
node-id: "rqlite-node1"
data-dir: "/var/lib/rqlite"
```

systemd 服务 `/etc/systemd/system/rqlite.service`：

```ini
[Unit]
Description=rqlite - lightweight, distributed relational database
After=network.target

[Service]
Type=simple
User=rqlite
Group=rqlite
ExecStart=/usr/local/bin/rqlited -config /etc/rqlite/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

#### rqlite 节点 2 配置

```yaml
http-addr: "0.0.0.0:4001"
raft-addr: "0.0.0.0:4002"
join:
  - "192.168.1.100:4002"
  - "192.168.1.101:4002"
  - "192.168.1.102:4002"
node-id: "rqlite-node2"
data-dir: "/var/lib/rqlite"
```

#### rqlite 节点 3 配置

```yaml
http-addr: "0.0.0.0:4001"
raft-addr: "0.0.0.0:4002"
join:
  - "192.168.1.100:4002"
  - "192.168.1.101:4002"
  - "192.168.1.102:4002"
node-id: "rqlite-node3"
data-dir: "/var/lib/rqlite"
```

**启动步骤**：

1. 首先启动第一个节点，无需 `join` 配置
2. 启动第二个和第三个节点，配置 `join` 参数连接到集群
3. 验证集群状态：
   ```bash
   curl http://192.168.1.100:4001/status
   ```

### 9.4 Redis 哨兵模式配置

#### Redis 主节点配置

创建 `/etc/redis/redis.conf`：

```conf
bind 0.0.0.0
port 6379
daemonize no
supervised systemd
pidfile /var/run/redis/redis-server.pid
logfile /var/log/redis/redis-server.log
dir /var/lib/redis
requirepass your-redis-password
```

#### Redis 哨兵配置

创建 `/etc/redis/sentinel.conf`（所有哨兵节点相同）：

```conf
port 26379
daemonize no
pidfile /var/run/redis/redis-sentinel.pid
logfile /var/log/redis/redis-sentinel.log
dir /tmp

sentinel monitor mymaster 192.168.1.200 6379 2
sentinel auth-pass mymaster your-redis-password
sentinel down-after-milliseconds mymaster 5000
sentinel parallel-syncs mymaster 1
sentinel failover-timeout mymaster 10000
```

**哨兵配置说明**：
- `sentinel monitor mymaster 192.168.1.200 6379 2`：监控名为 `mymaster` 的 master，2 个哨兵同意即可故障转移
- `sentinel auth-pass`：master 密码
- `down-after-milliseconds`：5000ms 无响应视为下线
- `failover-timeout`：故障转移超时时间 10000ms

#### Redis 哨兵 systemd 服务

创建 `/etc/systemd/system/redis-sentinel.service`：

```ini
[Unit]
Description=Redis Sentinel
After=network.target

[Service]
Type=simple
User=redis
Group=redis
ExecStart=/usr/bin/redis-sentinel /etc/redis/sentinel.conf
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

**启动步骤**：

1. 启动 Redis master
2. 启动所有 Redis 哨兵节点
3. 验证哨兵状态：
   ```bash
   redis-cli -p 26379 info sentinel
   ```

### 9.5 Nginx 反向代理和负载均衡配置

创建 `/etc/nginx/conf.d/bdopsflow.conf`：

```nginx
upstream bdopsflow_schedulers {
    least_conn;
    server 192.168.1.10:8080;
    server 192.168.1.11:8080;
    server 192.168.1.12:8080;
    keepalive 32;
}

server {
    listen 80;
    server_name bdopsflow.example.com;

    # 重定向到 HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name bdopsflow.example.com;

    # SSL 证书配置
    ssl_certificate /etc/nginx/ssl/bdopsflow.crt;
    ssl_certificate_key /etc/nginx/ssl/bdopsflow.key;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    # 日志配置
    access_log /var/log/nginx/bdopsflow_access.log;
    error_log /var/log/nginx/bdopsflow_error.log;

    # 客户端最大请求体大小（上传 SQL 文件等）
    client_max_body_size 100M;

    # 核心配置：查询接口单独超时配置
    # =======================================

    # 1. 查询执行接口 - 最长 30 分钟
    location ~ ^/api/v?\d*/query/execute$ {
        proxy_pass http://bdopsflow_schedulers;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

        # 关键配置：延长查询超时时间
        proxy_connect_timeout 60s;       # 连接超时
        proxy_send_timeout 1800s;        # 发送超时 (30分钟)
        proxy_read_timeout 1800s;        # 读取超时 (30分钟)
        proxy_request_buffering off;
        proxy_buffering off;
    }

    # 2. 查询取消接口 - 快速响应
    location ~ ^/api/v?\d*/query/cancel/[^/]+$ {
        proxy_pass http://bdopsflow_schedulers;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

        proxy_connect_timeout 10s;
        proxy_send_timeout 30s;
        proxy_read_timeout 30s;
    }

    # 3. 数据导出接口 - 30分钟
    location ~ ^/api/v?\d*/query/export$ {
        proxy_pass http://bdopsflow_schedulers;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

        proxy_connect_timeout 60s;
        proxy_send_timeout 1800s;
        proxy_read_timeout 1800s;
    }

    # 4. 数据源元数据查询 - 5分钟
    location ~ ^/api/v?\d*/datasources/[^/]+/metadata$ {
        proxy_pass http://bdopsflow_schedulers;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

        proxy_connect_timeout 60s;
        proxy_send_timeout 300s;
        proxy_read_timeout 300s;
    }

    # 5. SSE 支持（用于日志实时推送）
    # Server-Sent Events 保持长连接，无需 WebSocket 升级
    location ~ ^/api/v?\d*/logs/stream$ {
        proxy_pass http://bdopsflow_schedulers;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

        # SSE 关键配置：保持长连接
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 86400s;  # 24小时，适合长连接 SSE
    }

    # 6. 其他 API - 5 分钟
    location /api/ {
        proxy_pass http://bdopsflow_schedulers;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

        proxy_connect_timeout 60s;
        proxy_send_timeout 300s;
        proxy_read_timeout 300s;
    }

    # 7. 前端静态资源（包含 /assets/* 等）
    location / {
        root /var/www/bdopsflow;
        index index.html;
        try_files $uri $uri/ /index.html;
    }

    # 8. 健康检查端点
    location /health {
        proxy_pass http://bdopsflow_schedulers/health;
        access_log off;
    }

    # 注意：/metrics 指标接口建议直接连接后端节点，不通过 Nginx 代理
    # 原因：每个调度器节点有独立的指标，需要 Prometheus 直接采集所有节点
    # Prometheus 配置示例：
    # scrape_configs:
    #   - job_name: 'bdopsflow-scheduler'
    #     static_configs:
    #       - targets: ['192.168.1.10:8080', '192.168.1.11:8080', '192.168.1.12:8080']

    # 错误页面
    error_page 500 502 503 504 /50x.html;
    location = /50x.html {
        root /usr/share/nginx/html;
    }
}
```

**查询接口超时配置说明**

Hive 等大数据查询可能执行时间超过 1 分钟，需要针对不同接口配置不同的超时时间：

| 接口 | 超时时间 | 说明 |
|-----|--------|------|
| `/api/query/execute` | 30 分钟 | 查询执行接口，支持慢查询 |
| `/api/query/export` | 30 分钟 | 数据导出接口 |
| `/api/query/cancel/:query_id` | 30 秒 | 查询取消接口，应快速响应 |
| `/api/datasources/:id/metadata` | 5 分钟 | 元数据查询（表/列等） |
| `/api/logs/stream` | 24 小时 | SSE 日志推送，长连接 |
| `/health` | - | 健康检查，禁用日志 |
| `/metrics` | - | **直接采集**（不通过 Nginx） |
| 其他 API | 5 分钟 | 通用接口 |

**location 匹配顺序说明**

Nginx location 的匹配顺序非常重要，必须遵循以下原则：

1. **精确匹配的 location**（`=`）放在最前面
2. **带正则表达式的 location**（`~`）按声明顺序匹配
3. **前缀匹配的 location**（无修饰符）按最长匹配优先

当前配置顺序：
```
1. /api/v?\d*/query/execute$     (正则，查询执行 30分钟)
2. /api/v?\d*/query/cancel/...   (正则，查询取消 30秒)
3. /api/v?\d*/query/export$      (正则，数据导出 30分钟)
4. /api/v?\d*/datasources/...    (正则，元数据查询 5分钟)
5. /api/v?\d*/logs/stream$        (正则，SSE 24小时)
6. /api/                          (前缀，其他 API 5分钟)
7. /                               (前缀，前端 HTML SPA，包含 /assets/*)
8. /health                        (精确)

注意：/metrics 不通过 Nginx 代理，Prometheus 直接采集所有调度器节点
```

**关键参数说明**

```nginx
# 连接超时：建立 TCP 连接的超时时间
proxy_connect_timeout 60s;

# 发送超时：向后端发送请求的超时时间
proxy_send_timeout 1800s;  # 30分钟

# 读取超时：从后端读取响应的超时时间
proxy_read_timeout 1800s;  # 30分钟
```

**负载均衡策略说明**：
- `least_conn`：最少连接数策略，将请求转发到当前连接数最少的节点
- `keepalive`：保持与后端服务器的长连接，提高性能

**验证 Nginx 配置**：
```bash
nginx -t
systemctl reload nginx
```

**注意事项**：
1. **前端部署**：前端独立部署，Nginx 直接服务静态文件（`/var/www/bdopsflow`），不通过后端内置 Web
2. **后端内置 Web 禁用**：默认 `web.enabled: false`，所有 API 请求通过 Nginx 代理到调度器
3. **后端服务也需要相应配置**：确保后端（scheduler）的查询超时设置也足够长
4. **生产环境建议**：30分钟足够长，但根据实际业务调整
5. **SSE 连接超时**：SSE 使用 `/api/*/logs/stream` 接口，超时配置为 24 小时
6. **监控**：建议监控慢查询，避免恶意查询占用资源
7. **取消查询**：确保取消查询功能正常，避免查询堆积

### 9.6 完整部署步骤

1. **基础环境准备**
   ```bash
   # 在所有节点上安装必要的依赖
   apt-get update
   apt-get install -y nginx redis-server rqlite
   ```

2. **部署 rqlite 集群**
   - 配置并启动 3 个 rqlite 节点
   - 验证集群状态

3. **部署 Redis 哨兵集群**
   - 配置并启动 Redis master
   - 配置并启动 3 个 Redis 哨兵节点
   - 验证哨兵状态

4. **构建前端**
   ```bash
   cd web
   npm install
   npm run build
   ```

5. **部署前端静态文件**
   ```bash
   # 创建前端目录
   mkdir -p /var/www/bdopsflow

   # 复制前端构建文件
   cp -r web/dist/* /var/www/bdopsflow/

   # 设置权限
   chown -R www-data:www-data /var/www/bdopsflow
   ```

6. **部署调度中心**
   - 准备配置文件（各节点 node_id 和 advertise_addr 不同）
   - 配置 systemd 服务（确保 `web.enabled: false`）
   - 启动所有调度中心节点

7. **配置 Nginx**
   - 部署 SSL 证书
   - 配置负载均衡
   - 验证配置：`nginx -t`
   - 启动/重载 Nginx：`systemctl reload nginx`

8. **验证部署**
   - 访问 Web UI 验证
   - 测试任务调度
   - 测试故障转移

### 9.7 监控和维护

**健康检查**：
- 调度中心：访问 `http://<scheduler-addr>/health`
- rqlite：访问 `http://<rqlite-addr>:4001/status`
- Redis 哨兵：`redis-cli -p 26379 info sentinel`

**日志轮转**：
- 参考本文档第 1.2 节中的 logrotate 配置

**配置重载**：
- 修改配置后使用 `systemctl reload bdopsflow-scheduler` 触发重载

---

## 相关文档

- [使用指南](./GUIDE.md) - 综合使用指南
- [架构设计](./ARCHITECTURE.md) - 系统架构和技术设计
- [开发指南](./DEVELOPMENT.md) - 开发环境搭建和最佳实践
- [SSO 登录指南](./sso-login-guide.md) - SSO 第三方登录配置
