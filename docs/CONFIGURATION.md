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

## 相关文档

- [使用指南](./GUIDE.md) - 综合使用指南
- [架构设计](./ARCHITECTURE.md) - 系统架构和技术设计
- [开发指南](./DEVELOPMENT.md) - 开发环境搭建和最佳实践
- [SSO 登录指南](./sso-login-guide.md) - SSO 第三方登录配置
