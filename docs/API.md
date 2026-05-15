# BDopsFlow API 接口文档

本文档详细描述了 BDopsFlow 调度平台的所有 HTTP API 接口，包括请求参数、响应格式和使用示例。

## 目录

- [认证接口](#认证接口)
  - [用户登录](#用户登录)
  - [用户注册](#用户注册)
  - [获取当前用户](#获取当前用户)
- [任务接口](#任务接口)
  - [获取任务列表](#获取任务列表)
  - [创建任务](#创建任务)
  - [获取任务详情](#获取任务详情)
  - [更新任务](#更新任务)
  - [删除任务](#删除任务)
  - [手动触发任务](#手动触发任务)
  - [获取任务执行历史](#获取任务执行历史)
  - [获取执行日志](#获取执行日志)
- [工作流接口](#工作流接口)
  - [获取工作流列表](#获取工作流列表)
  - [创建工作流](#创建工作流)
  - [获取工作流详情](#获取工作流详情)
  - [更新工作流](#更新工作流)
  - [删除工作流](#删除工作流)
  - [触发工作流](#触发工作流)
  - [获取工作流执行历史](#获取工作流执行历史)
- [执行器接口](#执行器接口)
  - [获取执行器列表](#获取执行器列表)
  - [获取执行器详情](#获取执行器详情)
  - [删除执行器](#删除执行器)
- [日志接口](#日志接口)
  - [获取执行日志列表](#获取执行日志列表)
  - [获取执行统计](#获取执行统计)
  - [删除执行记录](#删除执行记录)
  - [批量删除执行记录](#批量删除执行记录)
- [数据模型](#数据模型)

---

## 通用说明

### 基础 URL

```
http://localhost:8080
```

### 认证方式

除了登录和注册接口外，所有接口都需要在请求头中携带 JWT Token：

```
Authorization: Bearer <token>
```

### 通用响应格式

#### 成功响应

```json
{
  "id": 1,
  "name": "task-name",
  ...
}
```

#### 错误响应

```json
{
  "error": "错误信息描述"
}
```

### HTTP 状态码

| 状态码 | 说明 |
|--------|------|
| 200 | 请求成功 |
| 201 | 创建成功 |
| 400 | 请求参数错误 |
| 401 | 未授权（未登录或 Token 无效） |
| 403 | 权限不足 |
| 404 | 资源不存在 |
| 500 | 服务器内部错误 |

---

## 认证接口

### 用户登录

**接口地址**：`POST /api/auth/login`

**权限要求**：无（公开接口）

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| username | string | 是 | 用户名 |
| password | string | 是 | 密码 |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "admin123"
  }'
```

**响应示例**：

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": 1,
    "username": "admin",
    "role": "admin",
    "email": "admin@example.com",
    "domain_id": 1
  }
}
```

**响应字段说明**：

| 字段名 | 类型 | 说明 |
|--------|------|------|
| token | string | JWT Token，用于后续请求认证 |
| user.id | int64 | 用户 ID |
| user.username | string | 用户名 |
| user.role | string | 用户角色：admin（管理员）、operator（操作员）、viewer（查看者） |
| user.email | string | 邮箱地址 |
| user.domain_id | int64 | 所属领域 ID |

---

### 用户注册

**接口地址**：`POST /api/auth/register`

**权限要求**：无（公开接口）

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| username | string | 是 | 用户名（唯一） |
| password | string | 是 | 密码 |
| role | string | 否 | 用户角色，默认为 operator |
| email | string | 否 | 邮箱地址 |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "operator1",
    "password": "password123",
    "role": "operator",
    "email": "operator1@example.com"
  }'
```

**响应示例**：

```json
{
  "id": 2,
  "username": "operator1",
  "role": "operator",
  "email": "operator1@example.com"
}
```

---

### 获取当前用户

**接口地址**：`GET /api/auth/current`

**权限要求**：已登录用户

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/auth/current \
  -H "Authorization: Bearer <token>"
```

**响应示例**：

```json
{
  "id": 1,
  "username": "admin",
  "role": "admin",
  "email": "admin@example.com",
  "domain_id": 1
}
```

---

## 任务接口

### 获取任务列表

**接口地址**：`GET /api/tasks`

**权限要求**：已登录用户

**请求参数**：无

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/tasks \
  -H "Authorization: Bearer <token>"
```

**响应示例**：

```json
{
  "items": [
    {
      "id": 1,
      "workflow_id": null,
      "name": "健康检查任务",
      "type": "http",
      "config": "{\"url\":\"https://api.example.com/health\",\"method\":\"GET\"}",
      "cron_expression": "*/5 * * * *",
      "timeout_seconds": 30,
      "retry_count": 3,
      "retry_interval": 5,
      "is_enabled": true,
      "status": "pending",
      "domain_id": 1,
      "webhook_config": "",
      "assigned_executor_id": "",
      "created_by": 1,
      "created_at": "2026-05-15T10:00:00Z",
      "updated_at": "2026-05-15T10:00:00Z",
      "next_execution_time": "2026-05-15T10:05:00Z",
      "last_execution_status": "success"
    }
  ]
}
```

**响应字段说明**：

| 字段名 | 类型 | 说明 |
|--------|------|------|
| id | int64 | 任务 ID |
| workflow_id | int64/null | 所属工作流 ID，null 表示独立任务 |
| name | string | 任务名称 |
| type | string | 任务类型：http、shell |
| config | string | 任务配置（JSON 字符串） |
| cron_expression | string | Cron 表达式 |
| timeout_seconds | int32 | 超时时间（秒） |
| retry_count | int32 | 最大重试次数 |
| retry_interval | int32 | 重试间隔（秒） |
| is_enabled | bool | 是否启用 |
| status | string | 任务状态：pending、running、success、failed |
| domain_id | int64 | 所属领域 ID |
| webhook_config | string | Webhook 配置（JSON 字符串） |
| assigned_executor_id | string | 指定执行器 ID，空表示自动选择 |
| created_by | int64 | 创建者用户 ID |
| created_at | string | 创建时间（RFC3339 格式） |
| updated_at | string | 更新时间（RFC3339 格式） |
| next_execution_time | string | 下次执行时间 |
| last_execution_status | string | 最后一次执行状态 |

---

### 创建任务

**接口地址**：`POST /api/tasks`

**权限要求**：admin 或 operator

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| name | string | 是 | 任务名称 |
| type | string | 是 | 任务类型：http、shell |
| config | object/string | 是 | 任务配置 |
| workflow_id | int64 | 否 | 所属工作流 ID |
| cron_expression | string | 否 | Cron 表达式 |
| timeout_seconds | int32 | 否 | 超时时间（秒），默认 300 |
| retry_count | int32 | 否 | 最大重试次数，默认 3 |
| retry_interval | int32 | 否 | 重试间隔（秒），默认 5 |
| is_enabled | bool | 否 | 是否启用，默认 false |
| domain_id | int64 | 否 | 所属领域 ID，默认 1 |
| webhook_config | string | 否 | Webhook 配置 |
| assigned_executor_id | string | 否 | 指定执行器 ID |

**HTTP 任务配置**：

```json
{
  "url": "https://api.example.com/endpoint",
  "method": "GET|POST|PUT|DELETE",
  "headers": {
    "Authorization": "Bearer token",
    "Content-Type": "application/json"
  },
  "body": "{}",
  "timeout": 10000
}
```

**Shell 任务配置**：

```json
{
  "script": "echo 'Hello World'"
}
```

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "API健康检查",
    "type": "http",
    "config": {
      "url": "https://api.example.com/health",
      "method": "GET",
      "timeout": 10000
    },
    "cron_expression": "*/5 * * * *",
    "timeout_seconds": 30,
    "retry_count": 3,
    "retry_interval": 5,
    "is_enabled": true,
    "domain_id": 1
  }'
```

**响应示例**：

```json
{
  "id": 1,
  "name": "API健康检查",
  "type": "http",
  "config": "{\"url\":\"https://api.example.com/health\",\"method\":\"GET\",\"timeout\":10000}",
  "cron_expression": "*/5 * * * *",
  "timeout_seconds": 30,
  "retry_count": 3,
  "retry_interval": 5,
  "is_enabled": true,
  "status": "pending",
  "domain_id": 1,
  "webhook_config": "",
  "assigned_executor_id": "",
  "created_by": 1,
  "created_at": "2026-05-15T10:00:00Z",
  "updated_at": "2026-05-15T10:00:00Z"
}
```

---

### 获取任务详情

**接口地址**：`GET /api/tasks/:id`

**权限要求**：已登录用户

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 任务 ID |

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/tasks/1 \
  -H "Authorization: Bearer <token>"
```

**响应示例**：同创建任务响应

---

### 更新任务

**接口地址**：`PUT /api/tasks/:id`

**权限要求**：admin 或 operator

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 任务 ID |

**请求参数**：所有字段均为可选，只更新提供的字段

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| name | string | 否 | 任务名称 |
| type | string | 否 | 任务类型 |
| config | object/string | 否 | 任务配置 |
| cron_expression | string | 否 | Cron 表达式 |
| timeout_seconds | int32 | 否 | 超时时间 |
| retry_count | int32 | 否 | 最大重试次数 |
| retry_interval | int32 | 否 | 重试间隔 |
| is_enabled | bool | 否 | 是否启用 |
| domain_id | int64 | 否 | 所属领域 ID |
| webhook_config | string | 否 | Webhook 配置 |
| assigned_executor_id | string | 否 | 指定执行器 ID |

**请求示例**：

```bash
curl -X PUT http://localhost:8080/api/tasks/1 \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "更新后的任务名",
    "is_enabled": false
  }'
```

**响应示例**：返回更新后的任务对象

---

### 删除任务

**接口地址**：`DELETE /api/tasks/:id`

**权限要求**：admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 任务 ID |

**请求示例**：

```bash
curl -X DELETE http://localhost:8080/api/tasks/1 \
  -H "Authorization: Bearer <token>"
```

**响应示例**：

```json
{
  "message": "deleted"
}
```

---

### 手动触发任务

**接口地址**：`POST /api/tasks/:id/trigger`

**权限要求**：admin 或 operator

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 任务 ID |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/tasks/1/trigger \
  -H "Authorization: Bearer <token>"
```

**响应示例**：

```json
{
  "message": "triggered",
  "execution_id": "exec-20260515-abc123"
}
```

**响应字段说明**：

| 字段名 | 类型 | 说明 |
|--------|------|------|
| message | string | 操作结果 |
| execution_id | string | 执行 ID，可用于查询执行状态和日志 |

---

### 获取任务执行历史

**接口地址**：`GET /api/tasks/:id/executions`

**权限要求**：已登录用户

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 任务 ID |

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/tasks/1/executions \
  -H "Authorization: Bearer <token>"
```

**响应示例**：

```json
[
  {
    "id": 1,
    "task_id": 1,
    "execution_id": "exec-20260515-abc123",
    "executor_id": "executor-1",
    "status": "success",
    "start_time": "2026-05-15T10:00:00Z",
    "end_time": "2026-05-15T10:00:05Z",
    "output": "{\"status\":\"ok\"}",
    "error": "",
    "retry_times": 0,
    "created_at": "2026-05-15T10:00:00Z"
  }
]
```

**响应字段说明**：

| 字段名 | 类型 | 说明 |
|--------|------|------|
| id | int64 | 执行记录 ID |
| task_id | int64 | 任务 ID |
| execution_id | string | 执行 ID |
| executor_id | string | 执行器 ID |
| status | string | 执行状态：pending、running、success、failed |
| start_time | string | 开始时间 |
| end_time | string | 结束时间 |
| output | string | 执行输出 |
| error | string | 错误信息 |
| retry_times | int32 | 已重试次数 |
| created_at | string | 创建时间 |

---

### 获取执行日志

**接口地址**：`GET /api/tasks/executions/:executionId/logs`

**权限要求**：已登录用户

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| executionId | string | 是 | 执行 ID |

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/tasks/executions/exec-20260515-abc123/logs \
  -H "Authorization: Bearer <token>"
```

**响应示例**：

```json
[
  {
    "id": 1,
    "execution_id": "exec-20260515-abc123",
    "task_id": 1,
    "executor_id": "executor-1",
    "node_id": "node-1",
    "log_level": "info",
    "message": "Task started",
    "log_time": "2026-05-15T10:00:00Z"
  }
]
```

---

## 工作流接口

### 获取工作流列表

**接口地址**：`GET /api/workflows`

**权限要求**：已登录用户

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/workflows \
  -H "Authorization: Bearer <token>"
```

**响应示例**：

```json
[
  {
    "id": 1,
    "name": "数据处理工作流",
    "description": "ETL数据处理流程",
    "domain_id": 1,
    "dag_config": "{\"nodes\":[...],\"edges\":[...]}",
    "cron_expression": "0 2 * * *",
    "is_enabled": true,
    "created_by": 1,
    "created_at": "2026-05-15T10:00:00Z",
    "updated_at": "2026-05-15T10:00:00Z"
  }
]
```

**响应字段说明**：

| 字段名 | 类型 | 说明 |
|--------|------|------|
| id | int64 | 工作流 ID |
| name | string | 工作流名称 |
| description | string | 描述 |
| domain_id | int64 | 所属领域 ID |
| dag_config | string | DAG 配置（JSON 字符串） |
| cron_expression | string | Cron 表达式 |
| is_enabled | bool | 是否启用 |
| created_by | int64 | 创建者用户 ID |
| created_at | string | 创建时间 |
| updated_at | string | 更新时间 |

---

### 创建工作流

**接口地址**：`POST /api/workflows`

**权限要求**：admin 或 operator

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| name | string | 是 | 工作流名称 |
| description | string | 否 | 描述 |
| domain_id | int64 | 否 | 所属领域 ID，默认 1 |
| dag_config | string | 否 | DAG 配置 |
| cron_expression | string | 否 | Cron 表达式 |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/workflows \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "数据处理工作流",
    "description": "每日ETL处理",
    "dag_config": "{\"nodes\":[{\"id\":\"task1\",\"type\":\"http\"}],\"edges\":[]}",
    "cron_expression": "0 2 * * *"
  }'
```

**响应示例**：返回创建的工作流对象

---

### 获取工作流详情

**接口地址**：`GET /api/workflows/:id`

**权限要求**：已登录用户

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 工作流 ID |

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/workflows/1 \
  -H "Authorization: Bearer <token>"
```

---

### 更新工作流

**接口地址**：`PUT /api/workflows/:id`

**权限要求**：admin 或 operator

**请求参数**：所有字段均为可选

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| name | string | 否 | 工作流名称 |
| description | string | 否 | 描述 |
| domain_id | int64 | 否 | 所属领域 ID |
| dag_config | string | 否 | DAG 配置 |
| cron_expression | string | 否 | Cron 表达式 |
| is_enabled | bool | 否 | 是否启用 |

---

### 删除工作流

**接口地址**：`DELETE /api/workflows/:id`

**权限要求**：admin

---

### 触发工作流

**接口地址**：`POST /api/workflows/:id/trigger`

**权限要求**：admin 或 operator

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/workflows/1/trigger \
  -H "Authorization: Bearer <token>"
```

**响应示例**：

```json
{
  "id": 1,
  "workflow_id": 1,
  "execution_id": "wf-exec-20260515-xyz789",
  "status": "running",
  "start_time": "2026-05-15T10:00:00Z",
  "node_states": "{}",
  "created_at": "2026-05-15T10:00:00Z"
}
```

---

### 获取工作流执行历史

**接口地址**：`GET /api/workflows/:id/executions`

**权限要求**：已登录用户

---

## 执行器接口

### 获取执行器列表

**接口地址**：`GET /api/executors`

**权限要求**：已登录用户

**请求示例**：

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

**响应字段说明**：

| 字段名 | 类型 | 说明 |
|--------|------|------|
| id | int64 | 执行器记录 ID |
| executor_id | string | 执行器唯一标识 |
| name | string | 执行器名称 |
| address | string | 执行器地址（IP:端口） |
| status | string | 状态：online、offline |
| last_heartbeat | string | 最后心跳时间 |
| capacity | int64 | 最大并发任务数 |
| current_load | int64 | 当前运行任务数 |
| created_at | string | 注册时间 |
| updated_at | string | 更新时间 |

---

### 获取执行器详情

**接口地址**：`GET /api/executors/:id`

**权限要求**：已登录用户

---

### 删除执行器

**接口地址**：`DELETE /api/executors/:id`

**权限要求**：admin

**请求示例**：

```bash
curl -X DELETE http://localhost:8080/api/executors/1 \
  -H "Authorization: Bearer <token>"
```

**响应示例**：

```json
{
  "message": "deleted"
}
```

---

## 日志接口

### 获取执行日志列表

**接口地址**：`GET /api/logs`

**权限要求**：已登录用户

**查询参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| page | int | 否 | 页码，默认 1 |
| page_size | int | 否 | 每页数量，默认 20，最大 100 |
| id | int64 | 否 | 执行记录 ID |
| execution_id | string | 否 | 执行 ID |
| executor_name | string | 否 | 执行器名称 |
| task_name | string | 否 | 任务名称 |
| status | string | 否 | 执行状态 |
| start_time_from | string | 否 | 开始时间起始 |
| start_time_to | string | 否 | 开始时间结束 |
| end_time_from | string | 否 | 结束时间起始 |
| end_time_to | string | 否 | 结束时间结束 |
| duration_min | int | 否 | 执行时长最小值（秒） |
| duration_max | int | 否 | 执行时长最大值（秒） |

**请求示例**：

```bash
curl -X GET "http://localhost:8080/api/logs?page=1&page_size=20&status=failed" \
  -H "Authorization: Bearer <token>"
```

**响应示例**：

```json
{
  "data": [
    {
      "id": 1,
      "task_id": 1,
      "execution_id": "exec-20260515-abc123",
      "executor_id": "executor-1",
      "executor_name": "executor-1",
      "task_name": "健康检查任务",
      "task_type": "http",
      "status": "success",
      "start_time": "2026-05-15T10:00:00Z",
      "end_time": "2026-05-15T10:00:05Z",
      "output": "{\"status\":\"ok\"}",
      "error": "",
      "retry_times": 0,
      "created_at": "2026-05-15T10:00:00Z"
    }
  ],
  "total": 100,
  "page": 1,
  "page_size": 20
}
```

---

### 获取执行统计

**接口地址**：`GET /api/logs/stats`

**权限要求**：已登录用户

**查询参数**：同获取执行日志列表

**请求示例**：

```bash
curl -X GET "http://localhost:8080/api/logs/stats" \
  -H "Authorization: Bearer <token>"
```

**响应示例**：

```json
{
  "total": 1000,
  "success": 950,
  "failed": 50,
  "success_rate": 95.0,
  "avg_duration": 5.5,
  "max_duration": 120.0,
  "min_duration": 0.1
}
```

---

### 删除执行记录

**接口地址**：`DELETE /api/logs/:id`

**权限要求**：已登录用户

---

### 批量删除执行记录

**接口地址**：`POST /api/logs/batch-delete`

**权限要求**：已登录用户

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| ids | int64[] | 是 | 要删除的执行记录 ID 数组 |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/logs/batch-delete \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "ids": [1, 2, 3]
  }'
```

**响应示例**：

```json
{
  "message": "deleted successfully"
}
```

---

## 数据模型

### Task（任务）

| 字段名 | 类型 | 说明 |
|--------|------|------|
| id | int64 | 任务 ID |
| workflow_id | int64/null | 所属工作流 ID |
| name | string | 任务名称 |
| type | string | 任务类型：http、shell |
| config | string | 任务配置（JSON） |
| cron_expression | string | Cron 表达式 |
| timeout_seconds | int32 | 超时时间（秒） |
| retry_count | int32 | 最大重试次数 |
| retry_interval | int32 | 重试间隔（秒） |
| is_enabled | bool | 是否启用 |
| status | string | 任务状态 |
| domain_id | int64 | 所属领域 ID |
| webhook_config | string | Webhook 配置 |
| assigned_executor_id | string | 指定执行器 ID |
| created_by | int64 | 创建者 ID |
| created_at | time | 创建时间 |
| updated_at | time | 更新时间 |

### TaskExecution（任务执行记录）

| 字段名 | 类型 | 说明 |
|--------|------|------|
| id | int64 | 执行记录 ID |
| task_id | int64 | 任务 ID |
| execution_id | string | 执行 ID |
| executor_id | string | 执行器 ID |
| status | string | 执行状态：pending、running、success、failed |
| start_time | time | 开始时间 |
| end_time | time | 结束时间 |
| output | string | 执行输出 |
| error | string | 错误信息 |
| retry_times | int32 | 已重试次数 |
| created_at | time | 创建时间 |

### Executor（执行器）

| 字段名 | 类型 | 说明 |
|--------|------|------|
| id | int64 | 执行器记录 ID |
| executor_id | string | 执行器唯一标识 |
| name | string | 执行器名称 |
| address | string | 执行器地址 |
| status | string | 状态：online、offline |
| last_heartbeat | time | 最后心跳时间 |
| capacity | int64 | 最大并发任务数 |
| current_load | int64 | 当前运行任务数 |
| created_at | time | 注册时间 |
| updated_at | time | 更新时间 |

### Workflow（工作流）

| 字段名 | 类型 | 说明 |
|--------|------|------|
| id | int64 | 工作流 ID |
| name | string | 工作流名称 |
| description | string | 描述 |
| domain_id | int64 | 所属领域 ID |
| dag_config | string | DAG 配置（JSON） |
| cron_expression | string | Cron 表达式 |
| is_enabled | bool | 是否启用 |
| created_by | int64 | 创建者 ID |
| created_at | time | 创建时间 |
| updated_at | time | 更新时间 |

### User（用户）

| 字段名 | 类型 | 说明 |
|--------|------|------|
| id | int64 | 用户 ID |
| username | string | 用户名 |
| password | string | 密码（加密存储） |
| email | string | 邮箱 |
| domain_id | int64 | 所属领域 ID |
| role | string | 角色：admin、operator、viewer |
| created_at | time | 创建时间 |
| updated_at | time | 更新时间 |

---

## Cron 表达式说明

系统支持标准 5 位 Cron 表达式：

```
┌───────────── 分钟 (0 - 59)
│ ┌───────────── 小时 (0 - 23)
│ │ ┌───────────── 日期 (1 - 31)
│ │ │ ┌───────────── 月份 (1 - 12)
│ │ │ │ ┌───────────── 星期 (0 - 6，0 = 周日)
│ │ │ │ │
* * * * *
```

**常用示例**：

| 表达式 | 说明 |
|--------|------|
| `*/5 * * * *` | 每 5 分钟执行一次 |
| `0 * * * *` | 每小时整点执行 |
| `0 2 * * *` | 每天凌晨 2 点执行 |
| `0 0 * * 1` | 每周一凌晨执行 |
| `0 0 1 * *` | 每月 1 号凌晨执行 |

---

## 错误码说明

| 错误信息 | 说明 |
|----------|------|
| invalid id | ID 参数无效 |
| id must be positive | ID 必须为正数 |
| name is required | 名称不能为空 |
| type is required | 类型不能为空 |
| invalid credentials | 用户名或密码错误 |
| unauthorized | 未授权 |
| task not found | 任务不存在 |
| workflow not found | 工作流不存在 |
| executor not found | 执行器不存在 |
