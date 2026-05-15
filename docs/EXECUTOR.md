# BDopsFlow 执行器使用指南

本文档详细描述了执行器的配置、部署、使用方法和注意事项。

## 目录

- [执行器概述](#执行器概述)
- [配置说明](#配置说明)
- [启动执行器](#启动执行器)
- [任务类型](#任务类型)
- [执行器管理](#执行器管理)
- [心跳机制](#心跳机制)
- [锁续期机制](#锁续期机制)
- [多执行器部署](#多执行器部署)
- [故障处理](#故障处理)

---

## 执行器概述

执行器（Executor）是 BDopsFlow 的任务执行组件，负责接收调度中心分发的任务并实际执行。

### 核心功能

- 向调度中心注册并维护心跳
- 接收并执行任务（HTTP、Shell）
- 上报任务执行结果和日志
- 追踪运行中的任务
- 支持锁续期机制防止任务卡死

### 架构

```
┌─────────────────────────────────────────┐
│              Executor                    │
│  ┌─────────────┐  ┌─────────────┐       │
│  │ gRPC Client │  │ Task Pool   │       │
│  │             │  │ (协程池)    │       │
│  └─────────────┘  └─────────────┘       │
│  ┌─────────────┐  ┌─────────────┐       │
│  │ Task Runner │  │ Logger      │       │
│  │ (执行器)    │  │             │       │
│  └─────────────┘  └─────────────┘       │
└─────────────────────────────────────────┘
           │
           ▼ gRPC
┌─────────────────────────────────────────┐
│             Scheduler                    │
└─────────────────────────────────────────┘
```

---

## 配置说明

### 配置文件 (config.yaml)

```yaml
# 执行器基本配置
app:
  executor_id: "executor-1"      # 执行器唯一标识（必填）
  executor_name: "executor-1"    # 执行器显示名称
  capacity: 10                   # 最大并发任务数

# 调度中心配置
scheduler:
  addr: "localhost:50051"        # 调度中心 gRPC 地址
  timeout: 30                    # 连接超时（秒）

# 日志配置
log:
  level: "info"                  # 日志级别：debug/info/warn/error
  format: "json"                 # 日志格式：json/text
```

### 环境变量配置

所有配置项都可以通过环境变量设置，环境变量优先级高于配置文件：

| 环境变量 | 对应配置项 | 默认值 |
|----------|------------|--------|
| APP_EXECUTOR_ID | app.executor_id | executor-1 |
| APP_EXECUTOR_NAME | app.executor_name | executor-1 |
| APP_CAPACITY | app.capacity | 10 |
| SCHEDULER_ADDR | scheduler.addr | localhost:50051 |
| SCHEDULER_TIMEOUT | scheduler.timeout | 30 |
| LOG_LEVEL | log.level | info |
| LOG_FORMAT | log.format | json |

### 命令行参数

```bash
./executor -config /path/to/config.yaml -hostname 192.168.1.100:50051
```

| 参数 | 说明 |
|------|------|
| -config | 配置文件路径 |
| -hostname | 执行器对外地址（IP:端口），用于调度器识别 |

---

## 启动执行器

### 直接启动

```bash
cd executor

# 编译
go build -o bin/executor ./cmd/main.go

# 复制配置文件
cp config.yaml.example config.yaml

# 启动
./bin/executor
```

### 使用 Docker 启动

```bash
docker run -d \
  --name bdopsflow-executor \
  -e APP_EXECUTOR_ID=executor-1 \
  -e APP_EXECUTOR_NAME=executor-1 \
  -e APP_CAPACITY=10 \
  -e SCHEDULER_ADDR=scheduler:50051 \
  bdopsflow/executor:latest
```

### 使用 systemd 管理

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

### 启动日志

正常启动后会看到以下日志：

```
INFO  executor starting
      executor_id=executor-1
      executor_name=executor-1
      scheduler_addr=localhost:50051
      capacity=10
INFO  connected to scheduler
INFO  registered with scheduler successfully
INFO  subscribed to tasks
INFO  executor running
      executor_id=executor-1
      name=executor-1
```

---

## 任务类型

### HTTP 任务

执行 HTTP 请求任务。

**任务配置**：

```json
{
  "url": "https://api.example.com/endpoint",
  "method": "GET",
  "headers": {
    "Authorization": "Bearer token",
    "Content-Type": "application/json"
  },
  "body": "{\"key\":\"value\"}",
  "timeout": 10000
}
```

**配置字段说明**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| url | string | 是 | 请求 URL |
| method | string | 否 | 请求方法，默认 GET |
| headers | object | 否 | 请求头 |
| body | string | 否 | 请求体（POST/PUT 时使用） |
| timeout | int | 否 | 请求超时（毫秒），默认 10000 |

**执行流程**：
1. 构建 HTTP 请求
2. 设置请求头和请求体
3. 发送请求
4. 等待响应
5. 返回响应内容

**输出格式**：

```json
{
  "status_code": 200,
  "headers": {...},
  "body": "..."
}
```

### Shell 任务

执行 Shell 命令任务。

**任务配置**：

```json
{
  "script": "echo 'Hello World' && date"
}
```

**配置字段说明**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| script | string | 是 | Shell 脚本内容 |

**执行流程**：
1. 创建临时脚本文件
2. 执行脚本
3. 捕获标准输出和错误输出
4. 返回执行结果

**输出格式**：

```json
{
  "stdout": "Hello World\n2026-05-15 10:00:00\n",
  "stderr": "",
  "exit_code": 0
}
```

---

## 执行器管理

### 查看执行器状态

通过 API 查看执行器列表：

```bash
curl -X GET http://localhost:8080/api/executors \
  -H "Authorization: Bearer <token>"
```

**响应示例**：

```json
[
  {
    "id": 1,
    "executor_id": "executor-1",
    "name": "executor-1",
    "address": "192.168.1.100:50051",
    "status": "online",
    "last_heartbeat": "2026-05-15 10:30:00",
    "capacity": 10,
    "current_load": 3,
    "created_at": "2026-05-15 10:00:00",
    "updated_at": "2026-05-15 10:30:00"
  }
]
```

**状态说明**：

| 状态 | 说明 |
|------|------|
| online | 在线，正常接收任务 |
| offline | 离线，心跳超时 |

### 负载计算

执行器负载 = 当前运行任务数 / 最大容量

```
负载百分比 = (current_load / capacity) * 100%
```

**示例**：
- capacity = 10, current_load = 3 → 负载 30%
- capacity = 10, current_load = 10 → 负载 100%（满载）

### 删除执行器

```bash
curl -X DELETE http://localhost:8080/api/executors/1 \
  -H "Authorization: Bearer <token>"
```

---

## 心跳机制

### 心跳间隔

执行器每 **10 秒** 向调度中心发送心跳。

### 心跳内容

```protobuf
message HeartbeatRequest {
  string executor_id = 1;              // 执行器 ID
  int32 current_load = 2;              // 当前负载
  repeated string running_execution_ids = 3;  // 运行中的执行 ID
}
```

### 心跳处理

调度中心收到心跳后：
1. 更新执行器最后心跳时间
2. 更新执行器当前负载
3. 对运行中的任务进行锁续期

### 离线检测

调度中心每 60 秒检测执行器心跳：
- 超过 60 秒无心跳 → 标记为 offline
- 清理该执行器上的任务

---

## 锁续期机制

为防止执行器异常退出后任务卡死，实现了锁续期机制。

### 工作原理

```
┌──────────┐     ┌───────────┐     ┌───────┐
│ Executor │────▶│ Scheduler │────▶│ Redis │
└──────────┘     └───────────┘     └───────┘
     │                │                │
     │ Heartbeat      │ Renew Lock     │
     │ + running_ids  │ TTL            │
     │                │                │
```

### 续期规则

- **锁 TTL**：60 秒
- **续期间隔**：每 10 秒（心跳时续期）
- **续期条件**：执行器心跳携带运行中的任务 ID

### 卡死任务检测

调度中心每 60 秒检测：
1. 检查所有 running 状态的任务
2. 检查锁是否存在或续期状态
3. 连续 3 次未续期 → 标记任务为 failed

---

## 多执行器部署

### 部署架构

```
                    ┌───────────────┐
                    │   Scheduler   │
                    └───────────────┘
                           │
           ┌───────────────┼───────────────┐
           │               │               │
           ▼               ▼               ▼
    ┌──────────┐    ┌──────────┐    ┌──────────┐
    │Executor-1│    │Executor-2│    │Executor-3│
    │capacity:10│   │capacity:20│   │capacity:15│
    └──────────┘    └──────────┘    └──────────┘
```

### 负载均衡策略

调度中心自动选择负载最低的执行器：

1. 过滤在线执行器（心跳在 30 秒内）
2. 过滤有可用容量的执行器
3. 选择当前负载最低的执行器

**示例**：
- Executor-1: 3/10 (30%)
- Executor-2: 5/20 (25%) ← 选择
- Executor-3: 8/15 (53%)

### 指定执行器

任务可以指定执行器：

```json
{
  "name": "特定执行器任务",
  "type": "http",
  "config": {...},
  "assigned_executor_id": "executor-1"
}
```

---

## 故障处理

### 执行器异常退出

当执行器异常退出时：
1. 调度中心检测到心跳超时
2. 标记执行器为 offline
3. 清理该执行器上的任务
4. 卡死的任务会被自动标记为 failed

### 任务重试

任务执行失败后会自动重试：

- 重试次数：由 `retry_count` 配置
- 重试间隔：由 `retry_interval` 配置（秒）

### 执行器重启

执行器重启后：
1. 重新向调度中心注册
2. 重新订阅任务
3. 之前运行中的任务会被标记为 failed（锁续期失败）

### 日志排查

查看执行器日志：

```bash
# systemd
journalctl -u bdopsflow-executor -f

# Docker
docker logs -f bdopsflow-executor

# 文件日志
tail -f /var/log/bdopsflow/executor.log
```

**关键日志**：

```
# 任务接收
INFO  received task
      task_id=1
      execution_id=exec-xxx
      type=http

# 任务执行
INFO  executing task
      task_id=1
      execution_id=exec-xxx

# 任务完成
INFO  task completed
      task_id=1
      execution_id=exec-xxx
      status=success
      duration=1.5s

# 任务失败
ERROR task failed
      task_id=1
      execution_id=exec-xxx
      error=connection timeout
```

---

## 性能调优

### 容量设置

根据机器配置设置合理的容量：

| 机器配置 | 建议容量 |
|----------|----------|
| 2核4G | 5-10 |
| 4核8G | 10-20 |
| 8核16G | 20-50 |

### 资源限制

使用 Docker 时可以限制资源：

```bash
docker run -d \
  --name bdopsflow-executor \
  --cpus=2 \
  --memory=4g \
  -e APP_CAPACITY=20 \
  bdopsflow/executor:latest
```

### 连接池

执行器内部使用协程池管理任务执行，无需额外配置连接池。

---

## 安全注意事项

### 1. 网络隔离

- 执行器应部署在内网
- 仅与调度中心通信
- 不对外暴露端口

### 2. Shell 任务安全

- Shell 任务执行前会验证脚本内容
- 建议限制可执行的命令范围
- 避免执行不可信的脚本

### 3. 权限控制

- 使用非 root 用户运行执行器
- 限制执行器文件系统访问权限

---

## 常见问题

### Q: 执行器无法连接调度中心？

**排查步骤**：
1. 检查调度中心是否运行
2. 检查网络连通性：`telnet scheduler-host 50051`
3. 检查防火墙规则
4. 检查配置中的调度中心地址

### Q: 执行器注册失败？

**排查步骤**：
1. 检查 executor_id 是否唯一
2. 检查调度中心日志
3. 检查 gRPC 连接状态

### Q: 任务执行超时？

**排查步骤**：
1. 检查任务配置的 timeout_seconds
2. 检查目标服务是否可达
3. 检查执行器网络状况

### Q: 执行器负载一直为 0？

**排查步骤**：
1. 检查是否有任务被分发到该执行器
2. 检查执行器是否在线
3. 检查执行器容量是否已满

### Q: 任务卡死不退出？

**排查步骤**：
1. 检查锁续期机制是否正常
2. 检查执行器心跳是否正常
3. 检查调度中心清理任务是否运行
