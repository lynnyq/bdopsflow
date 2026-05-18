# BDopsFlow API 接口文档

本文档详细描述了 BDopsFlow 调度平台的所有 HTTP API 接口，包含完整的字段类型说明、请求示例、正常响应、错误返回和特殊说明。

## 目录

- [通用说明](#通用说明)
- [认证接口](#认证接口)
- [任务接口](#任务接口)
- [工作流接口](#工作流接口)
- [执行器接口](#执行器接口)
- [日志接口](#日志接口)
- [仪表盘接口](#仪表盘接口)
- [管理员接口](#管理员接口)
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

所有接口统一使用以下响应格式：

#### 成功响应

```json
{
  "code": 0,
  "status": "success",
  "message": "success",
  "data": {
    "id": 1,
    "name": "task-name",
    ...
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| code | int | 业务状态码，0 表示成功，非 0 表示失败 |
| status | string | 状态："success" 或 "error" |
| message | string | 提示信息 |
| data | any | 实际数据，成功时返回 |

#### 错误响应

```json
{
  "code": 400,
  "status": "error",
  "message": "错误信息描述",
  "data": null
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| code | int | 业务状态码，对应 HTTP 状态码 |
| status | string | 状态："error" |
| message | string | 错误信息描述 |
| data | null | 错误时为 null |

### HTTP 状态码

| 状态码 | 说明 |
|--------|------|
| 200 | 请求成功 |
| 201 | 创建成功 |
| 204 | 删除成功 |
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

**正常响应**：

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": 1,
    "username": "admin",
    "role": "system_admin",
    "email": "admin@example.com",
    "domain_id": 0
  }
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | 解析错误 | 请求体格式不正确 |
| 401 | invalid credentials | 用户名或密码错误 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 用户注册

**接口地址**：`POST /api/auth/register`

**权限要求**：无（公开接口）

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| username | string | 是 | 用户名（唯一） |
| password | string | 是 | 密码 |
| role | string | 否 | 用户角色 |
| email | string | 否 | 邮箱地址 |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "user1",
    "password": "password123",
    "email": "user1@example.com"
  }'
```

**正常响应**：

```json
{
  "id": 2,
  "username": "user1",
  "role": "operator",
  "email": "user1@example.com"
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | 解析错误 | 请求体格式不正确 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 获取当前用户

**接口地址**：`GET /api/auth/current`

**权限要求**：已登录用户

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/auth/current \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "id": 1,
  "username": "admin",
  "role": "system_admin",
  "email": "admin@example.com",
  "domain_id": 0
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 401 | unauthorized | 未授权或Token无效 |
| 404 | user not found | 用户不存在 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 更新当前用户信息

**接口地址**：`PUT /api/auth/profile`

**权限要求**：已登录用户

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| email | string | 是 | 邮箱地址 |

**请求示例**：

```bash
curl -X PUT http://localhost:8080/api/auth/profile \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "newemail@example.com"
  }'
```

**正常响应**：返回更新后的用户信息

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | 解析错误 | 请求体格式不正确 |
| 401 | unauthorized | 未授权或Token无效 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 修改当前用户密码

**接口地址**：`POST /api/auth/change-password`

**权限要求**：已登录用户

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| old_password | string | 是 | 旧密码 |
| new_password | string | 是 | 新密码 |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/auth/change-password \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "old_password": "oldpass123",
    "new_password": "newpass456"
  }'
```

**正常响应**：

```json
{
  "message": "password changed successfully"
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | wrong old password | 旧密码错误 |
| 400 | password too short | 新密码太短 |
| 401 | unauthorized | 未授权或Token无效 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

## 任务接口

### 获取任务列表

**接口地址**：`GET /api/bdopsflow_tasks`

**权限要求**：已登录用户

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/bdopsflow_tasks \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

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
      "created_at": "2026-05-15 10:00:00",
      "updated_at": "2026-05-15 10:00:00",
      "next_execution_time": "2026-05-15 10:05:00",
      "last_execution_status": "success"
    }
  ]
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 401 | unauthorized | 未授权或Token无效 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 创建任务

**接口地址**：`POST /api/bdopsflow_tasks`

**权限要求**：system_admin 或 domain_admin 或 operator

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| name | string | 是 | 任务名称 |
| type | string | 是 | 任务类型：http、shell |
| config | object/string | 是 | 任务配置 |
| workflow_id | int64 | 否 | 所属工作流 ID |
| cron_expression | string | 否 | Cron 表达式 |
| timeout_seconds | int32 | 否 | 超时时间（秒），默认 300 |
| retry_max | int32 | 否 | 最大重试次数（兼容性字段），默认 3 |
| retry_delay_seconds | int32 | 否 | 重试间隔（秒，兼容性字段），默认 5 |
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
curl -X POST http://localhost:8080/api/bdopsflow_tasks \
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

**正常响应**：返回创建的任务对象

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
  "created_at": "2026-05-15 10:00:00",
  "updated_at": "2026-05-15 10:00:00"
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | name is required | 任务名称为空 |
| 400 | type is required | 任务类型为空 |
| 400 | 解析错误 | 请求体格式不正确 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：
- `config` 字段支持对象或字符串格式
- 同时支持 `retry_max/retry_delay_seconds` 和 `retry_count/retry_interval` 两种字段组合，后者优先
- `assigned_executor_id` 为空时系统会自动选择执行器

---

### 获取任务详情

**接口地址**：`GET /api/bdopsflow_tasks/:id`

**权限要求**：已登录用户

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 任务 ID |

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/bdopsflow_tasks/1 \
  -H "Authorization: Bearer <token>"
```

**正常响应**：同创建任务响应

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 401 | unauthorized | 未授权或Token无效 |
| 404 | task not found | 任务不存在 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 更新任务

**接口地址**：`PUT /api/bdopsflow_tasks/:id`

**权限要求**：system_admin 或 domain_admin 或 operator

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
| retry_max | int32 | 否 | 最大重试次数（兼容性字段） |
| retry_delay_seconds | int32 | 否 | 重试间隔（秒，兼容性字段） |
| retry_count | int32 | 否 | 最大重试次数 |
| retry_interval | int32 | 否 | 重试间隔（秒） |
| is_enabled | bool | 否 | 是否启用 |
| domain_id | int64 | 否 | 所属领域 ID |
| webhook_config | string | 否 | Webhook 配置 |
| assigned_executor_id | string | 否 | 指定执行器 ID |

**请求示例**：

```bash
curl -X PUT http://localhost:8080/api/bdopsflow_tasks/1 \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "更新后的任务名",
    "is_enabled": false
  }'
```

**正常响应**：返回更新后的任务对象

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 400 | 解析错误 | 请求体格式不正确 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 404 | task not found | 任务不存在 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：
- `config` 字段支持对象或字符串格式
- 同时支持 `retry_max/retry_delay_seconds` 和 `retry_count/retry_interval` 两种字段组合，后者优先
- `assigned_executor_id` 可以设置为空字符串来清除指定执行器

---

### 删除任务

**接口地址**：`DELETE /api/bdopsflow_tasks/:id`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 任务 ID |

**请求示例**：

```bash
curl -X DELETE http://localhost:8080/api/bdopsflow_tasks/1 \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "message": "deleted"
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 手动触发任务

**接口地址**：`POST /api/bdopsflow_tasks/:id/trigger`

**权限要求**：system_admin 或 domain_admin 或 operator

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 任务 ID |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/bdopsflow_tasks/1/trigger \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "message": "triggered",
  "execution_id": "exec-20260515-abc123"
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 获取任务执行历史

**接口地址**：`GET /api/bdopsflow_tasks/:id/executions`

**权限要求**：已登录用户

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 任务 ID |

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/bdopsflow_tasks/1/executions \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
[
  {
    "id": 1,
    "task_id": 1,
    "execution_id": "exec-20260515-abc123",
    "executor_id": "executor-1",
    "executor_name": "executor-1",
    "task_name": "健康检查任务",
    "task_type": "http",
    "status": "success",
    "start_time": "2026-05-15 10:00:00",
    "end_time": "2026-05-15 10:00:05",
    "output": "{\"status\":\"ok\"}",
    "error": "",
    "retry_times": 0,
    "created_at": "2026-05-15 10:00:00"
  }
]
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 401 | unauthorized | 未授权或Token无效 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 获取执行日志

**接口地址**：`GET /api/bdopsflow_tasks/executions/:executionId/logs`

**权限要求**：已登录用户

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| executionId | string | 是 | 执行 ID |

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/bdopsflow_tasks/executions/exec-20260515-abc123/logs \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

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
    "log_time": "2026-05-15 10:00:00"
  }
]
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | executionId required | executionId参数为空 |
| 401 | unauthorized | 未授权或Token无效 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 日志流（SSE）

**接口地址**：`GET /api/logs/stream?execution_id=<execution_id>`

**权限要求**：已登录用户

**查询参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| execution_id | string | 是 | 执行 ID |

**请求示例**：

```bash
curl -X GET "http://localhost:8080/api/logs/stream?execution_id=exec-20260515-abc123" \
  -H "Authorization: Bearer <token>" \
  -H "Accept: text/event-stream"
```

**正常响应**：Server-Sent Events 流式响应

```
data: {"id":1,"execution_id":"exec-20260515-abc123","task_id":1,"node_id":"node-1","log_level":"info","message":"Task started","log_time":"2026-05-15 10:00:00"}

data: {"type":"execution_update","status":"running","output":"","error":"","start_time":"2026-05-15 10:00:00","end_time":null}

: heartbeat
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | execution_id required | execution_id参数为空 |
| 401 | unauthorized | 未授权或Token无效 |

**特殊说明**：
- 使用 Server-Sent Events (SSE) 协议
- 包含两种类型的数据：日志信息和执行状态更新
- 包含心跳保活消息

---

## 工作流接口

### 获取工作流列表

**接口地址**：`GET /api/bdopsflow_workflows`

**权限要求**：已登录用户

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/bdopsflow_workflows \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
[
  {
    "id": 1,
    "name": "数据处理工作流",
    "description": "ETL数据处理流程",
    "domain_id": 1,
    "dag_config": "{\"nodes\":[],\"edges\":[]}",
    "cron_expression": "0 2 * * *",
    "is_enabled": true,
    "created_by": 1,
    "created_at": "2026-05-15 10:00:00",
    "updated_at": "2026-05-15 10:00:00"
  }
]
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 401 | unauthorized | 未授权或Token无效 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 创建工作流

**接口地址**：`POST /api/bdopsflow_workflows`

**权限要求**：system_admin 或 domain_admin 或 operator

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
curl -X POST http://localhost:8080/api/bdopsflow_workflows \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "数据处理工作流",
    "description": "每日ETL处理",
    "dag_config": "{\"nodes\":[{\"id\":\"task1\",\"type\":\"http\"}],\"edges\":[]}",
    "cron_expression": "0 2 * * *"
  }'
```

**正常响应**：返回创建的工作流对象

```json
{
  "id": 1,
  "name": "数据处理工作流",
  "description": "每日ETL处理",
  "domain_id": 1,
  "dag_config": "{\"nodes\":[{\"id\":\"task1\",\"type\":\"http\"}],\"edges\":[]}",
  "cron_expression": "0 2 * * *",
  "is_enabled": true,
  "created_by": 1,
  "created_at": "2026-05-15 10:00:00",
  "updated_at": "2026-05-15 10:00:00"
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | name is required | 工作流名称为空 |
| 400 | 解析错误 | 请求体格式不正确 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 获取工作流详情

**接口地址**：`GET /api/bdopsflow_workflows/:id`

**权限要求**：已登录用户

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 工作流 ID |

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/bdopsflow_workflows/1 \
  -H "Authorization: Bearer <token>"
```

**正常响应**：同创建工作流响应

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 401 | unauthorized | 未授权或Token无效 |
| 404 | workflow not found | 工作流不存在 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 更新工作流

**接口地址**：`PUT /api/bdopsflow_workflows/:id`

**权限要求**：system_admin 或 domain_admin 或 operator

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 工作流 ID |

**请求参数**：所有字段均为可选

**请求示例**：

```bash
curl -X PUT http://localhost:8080/api/bdopsflow_workflows/1 \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "更新后的工作流名",
    "description": "更新后的描述"
  }'
```

**正常响应**：返回更新后的工作流对象

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 400 | 解析错误 | 请求体格式不正确 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 删除工作流

**接口地址**：`DELETE /api/bdopsflow_workflows/:id`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 工作流 ID |

**请求示例**：

```bash
curl -X DELETE http://localhost:8080/api/bdopsflow_workflows/1 \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "message": "deleted"
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 触发工作流

**接口地址**：`POST /api/bdopsflow_workflows/:id/trigger`

**权限要求**：system_admin 或 domain_admin 或 operator

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 工作流 ID |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/bdopsflow_workflows/1/trigger \
  -H "Authorization: Bearer <token>"
```

**正常响应**：返回工作流执行对象

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 获取工作流执行历史

**接口地址**：`GET /api/bdopsflow_workflows/:id/executions`

**权限要求**：已登录用户

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 工作流 ID |

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/bdopsflow_workflows/1/executions \
  -H "Authorization: Bearer <token>"
```

**正常响应**：工作流执行列表

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 401 | unauthorized | 未授权或Token无效 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 获取工作流执行详情

**接口地址**：`GET /api/bdopsflow_workflows/executions/:executionId`

**权限要求**：已登录用户

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| executionId | string | 是 | 执行 ID |

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/bdopsflow_workflows/executions/wf-exec-20260515-abc123 \
  -H "Authorization: Bearer <token>"
```

**正常响应**：工作流执行对象

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | executionId required | executionId参数为空 |
| 401 | unauthorized | 未授权或Token无效 |
| 404 | workflow execution not found | 工作流执行不存在 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 获取工作流执行日志

**接口地址**：`GET /api/bdopsflow_workflows/executions/:executionId/logs`

**权限要求**：已登录用户

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| executionId | string | 是 | 执行 ID |

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/bdopsflow_workflows/executions/wf-exec-20260515-abc123/logs \
  -H "Authorization: Bearer <token>"
```

**正常响应**：同任务执行日志响应

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | executionId required | executionId参数为空 |
| 401 | unauthorized | 未授权或Token无效 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

## 执行器接口

### 获取执行器列表

**接口地址**：`GET /api/bdopsflow_executors`

**权限要求**：已登录用户

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/bdopsflow_executors \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

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

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 401 | unauthorized | 未授权或Token无效 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 获取执行器详情

**接口地址**：`GET /api/bdopsflow_executors/:id`

**权限要求**：已登录用户

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 执行器 ID |

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/bdopsflow_executors/1 \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "message": "ok"
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 401 | unauthorized | 未授权或Token无效 |

**特殊说明**：无

---

### 标记执行器在线

**接口地址**：`POST /api/bdopsflow_executors/:id/online`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | string | 是 | 执行器 ID |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/bdopsflow_executors/executor-1/online \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "message": "online"
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | executor_id is required | executor_id参数为空 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 标记执行器离线

**接口地址**：`POST /api/bdopsflow_executors/:id/offline`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | string | 是 | 执行器 ID |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/bdopsflow_executors/executor-1/offline \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "message": "offline"
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | executor_id is required | executor_id参数为空 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 更新执行器容量

**接口地址**：`PUT /api/bdopsflow_executors/:id/capacity`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | string | 是 | 执行器 ID |

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| capacity | int64 | 是 | 最大并发任务数（必须大于等于1） |

**请求示例**：

```bash
curl -X PUT http://localhost:8080/api/bdopsflow_executors/executor-1/capacity \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "capacity": 20
  }'
```

**正常响应**：

```json
{
  "message": "capacity updated",
  "capacity": 20
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | executor_id is required | executor_id参数为空 |
| 400 | invalid request: capacity must be a positive integer | capacity必须是正整数 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 删除执行器

**接口地址**：`DELETE /api/bdopsflow_executors/:id`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | string | 是 | 执行器 ID |

**请求示例**：

```bash
curl -X DELETE http://localhost:8080/api/bdopsflow_executors/executor-1 \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "message": "deleted"
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | executor_id is required | executor_id参数为空 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

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

**正常响应**：

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
      "start_time": "2026-05-15 10:00:00",
      "end_time": "2026-05-15 10:00:05",
      "output": "{\"status\":\"ok\"}",
      "error": "",
      "retry_times": 0,
      "created_at": "2026-05-15 10:00:00"
    }
  ],
  "total": 100,
  "page": 1,
  "page_size": 20
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 401 | unauthorized | 未授权或Token无效 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：
- `page` 和 `page_size` 超出范围时会自动修正为有效值
- 所有筛选参数都是可选的，可以组合使用

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

**正常响应**：

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

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 401 | unauthorized | 未授权或Token无效 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 删除执行记录

**接口地址**：`DELETE /api/logs/:id`

**权限要求**：已登录用户

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 执行记录 ID |

**请求示例**：

```bash
curl -X DELETE http://localhost:8080/api/logs/1 \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "message": "deleted successfully"
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 401 | unauthorized | 未授权或Token无效 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

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

**正常响应**：

```json
{
  "message": "deleted successfully"
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | no ids provided | ids数组为空 |
| 400 | 解析错误 | 请求体格式不正确 |
| 401 | unauthorized | 未授权或Token无效 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

## 仪表盘接口

### 获取统计数据

**接口地址**：`GET /api/dashboard/stats`

**权限要求**：已登录用户

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/dashboard/stats \
  -H "Authorization: Bearer <token>"
```

**正常响应**：仪表盘统计数据

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 401 | unauthorized | 未授权或Token无效 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 获取趋势数据

**接口地址**：`GET /api/dashboard/trends`

**权限要求**：已登录用户

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/dashboard/trends \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "items": [
    ...
  ]
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 401 | unauthorized | 未授权或Token无效 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 获取调度器状态

**接口地址**：`GET /api/dashboard/scheduler/status`

**权限要求**：已登录用户

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/dashboard/scheduler/status \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "paused": false
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 401 | unauthorized | 未授权或Token无效 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 暂停调度器

**接口地址**：`POST /api/dashboard/scheduler/pause`

**权限要求**：system_admin

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/dashboard/scheduler/pause \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "message": "scheduler paused"
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 恢复调度器

**接口地址**：`POST /api/dashboard/scheduler/resume`

**权限要求**：system_admin

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/dashboard/scheduler/resume \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "message": "scheduler resumed"
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

## 管理员接口

### 权限管理

#### 获取所有权限

**接口地址**：`GET /api/admin/bdopsflow_permissions`

**权限要求**：system_admin

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/admin/bdopsflow_permissions \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "items": [...],
  "groups": [...]
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 用户管理

#### 获取用户列表

**接口地址**：`GET /api/admin/bdopsflow_users`

**权限要求**：system_admin

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/admin/bdopsflow_users \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "items": [
    {
      "id": 1,
      "username": "admin",
      "email": "admin@example.com",
      ...
    }
  ]
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

#### 获取用户详情

**接口地址**：`GET /api/admin/bdopsflow_users/:id`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 用户 ID |

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/admin/bdopsflow_users/1 \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "user": {...},
  "bdopsflow_roles": [...]
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

#### 创建用户

**接口地址**：`POST /api/admin/bdopsflow_users`

**权限要求**：system_admin

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| username | string | 是 | 用户名（最少3个字符，最大50个字符，字母数字） |
| email | string | 是 | 邮箱地址 |
| password | string | 是 | 密码（最少6个字符） |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/admin/bdopsflow_users \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "newuser",
    "email": "newuser@example.com",
    "password": "password123"
  }'
```

**正常响应**：返回创建的用户对象

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | 参数验证错误 | 用户名或密码不满足要求 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

#### 更新用户

**接口地址**：`PUT /api/admin/bdopsflow_users/:id`

**权限要求**：system_admin 或 domain_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 用户 ID |

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| username | string | 是 | 用户名 |
| email | string | 是 | 邮箱地址 |
| role | string | 是 | 角色：system_admin, domain_admin, user |
| is_active | bool | 否 | 是否激活 |

**请求示例**：

```bash
curl -X PUT http://localhost:8080/api/admin/bdopsflow_users/1 \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "updateduser",
    "email": "updated@example.com",
    "role": "domain_admin",
    "is_active": true
  }'
```

**正常响应**：返回更新后的用户对象

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 404 | user not found | 用户不存在 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

#### 删除用户

**接口地址**：`DELETE /api/admin/bdopsflow_users/:id`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 用户 ID |

**请求示例**：

```bash
curl -X DELETE http://localhost:8080/api/admin/bdopsflow_users/1 \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

#### 分配用户角色

**接口地址**：`POST /api/admin/bdopsflow_users/:id/bdopsflow_roles`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 用户 ID |

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| role_ids | int64[] | 是 | 角色 ID 数组 |
| domain_ids | int64[] | 否 | 领域 ID 数组 |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/admin/bdopsflow_users/1/bdopsflow_roles \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "role_ids": [2, 3],
    "domain_ids": [1]
  }'
```

**正常响应**：

```json
{
  "message": "bdopsflow_roles assigned successfully"
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

#### 获取用户角色

**接口地址**：`GET /api/admin/bdopsflow_users/:id/bdopsflow_roles`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 用户 ID |

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/admin/bdopsflow_users/1/bdopsflow_roles \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "items": [...]
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

#### 分配用户领域

**接口地址**：`POST /api/admin/bdopsflow_users/:id/bdopsflow_domains`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 用户 ID |

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| domain_ids | int64[] | 是 | 领域 ID 数组 |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/admin/bdopsflow_users/1/bdopsflow_domains \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "domain_ids": [1, 2]
  }'
```

**正常响应**：

```json
{
  "message": "bdopsflow_domains assigned successfully"
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

#### 重置用户密码

**接口地址**：`POST /api/admin/bdopsflow_users/:id/reset-password`

**权限要求**：system_admin 或 domain_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 用户 ID |

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| new_password | string | 是 | 新密码 |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/admin/bdopsflow_users/1/reset-password \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "new_password": "newpass123"
  }'
```

**正常响应**：

```json
{
  "message": "password reset successfully"
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 404 | user not found | 用户不存在 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 角色管理

#### 获取角色列表

**接口地址**：`GET /api/admin/bdopsflow_roles`

**权限要求**：system_admin

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/admin/bdopsflow_roles \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "items": [
    {
      "id": 1,
      "name": "系统管理员",
      "code": "system_admin",
      ...
    }
  ]
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

#### 获取角色详情

**接口地址**：`GET /api/admin/bdopsflow_roles/:id`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------