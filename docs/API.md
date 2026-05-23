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
- [数据源接口](#数据源接口)
- [查询接口](#查询接口)
- [审计日志接口](#审计日志接口)
- [系统配置接口](#系统配置接口)
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
    "real_name": "系统管理员",
    "phone": "",
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
| real_name | string | 否 | 姓名 |
| phone | string | 否 | 手机号 |
| password | string | 是 | 密码 |
| role | string | 否 | 用户角色 |
| email | string | 否 | 邮箱地址 |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "user1",
    "real_name": "张三",
    "phone": "13800138000",
    "password": "password123",
    "email": "user1@example.com"
  }'
```

**正常响应**：

```json
{
  "id": 2,
  "username": "user1",
  "real_name": "张三",
  "phone": "13800138000",
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

**接口地址**：`GET /api/tasks`

**权限要求**：已登录用户

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/tasks \
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

**接口地址**：`POST /api/tasks`

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

**接口地址**：`PUT /api/tasks/:id`

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
curl -X PUT http://localhost:8080/api/tasks/1 \
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

**接口地址**：`DELETE /api/tasks/:id`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 任务 ID |

**请求示例**：

```bash
curl -X DELETE http://localhost:8080/api/tasks/1 \
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

**接口地址**：`POST /api/tasks/:id/trigger`

**权限要求**：system_admin 或 domain_admin 或 operator

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 任务 ID |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/tasks/1/trigger \
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

**接口地址**：`GET /api/workflows`

**权限要求**：已登录用户

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/workflows \
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

**接口地址**：`POST /api/workflows`

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

**接口地址**：`PUT /api/workflows/:id`

**权限要求**：system_admin 或 domain_admin 或 operator

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 工作流 ID |

**请求参数**：所有字段均为可选

**请求示例**：

```bash
curl -X PUT http://localhost:8080/api/workflows/1 \
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

**接口地址**：`DELETE /api/workflows/:id`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 工作流 ID |

**请求示例**：

```bash
curl -X DELETE http://localhost:8080/api/workflows/1 \
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

**接口地址**：`POST /api/workflows/:id/trigger`

**权限要求**：system_admin 或 domain_admin 或 operator

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 工作流 ID |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/workflows/1/trigger \
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

**接口地址**：`GET /api/workflows/:id/executions`

**权限要求**：已登录用户

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 工作流 ID |

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/workflows/1/executions \
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

**接口地址**：`GET /api/workflows/executions/:executionId`

**权限要求**：已登录用户

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| executionId | string | 是 | 执行 ID |

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/workflows/executions/wf-exec-20260515-abc123 \
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

**接口地址**：`GET /api/workflows/executions/:executionId/logs`

**权限要求**：已登录用户

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| executionId | string | 是 | 执行 ID |

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/workflows/executions/wf-exec-20260515-abc123/logs \
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

**接口地址**：`GET /api/executors`

**权限要求**：已登录用户

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/executors \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
[
  {
    "id": 1,
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

**接口地址**：`GET /api/executors/:name`

**权限要求**：已登录用户

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| name | string | 是 | 执行器名称 |

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/executors/executor-1 \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "id": 1,
  "name": "executor-1",
  "address": "192.168.1.100:50051",
  "status": "online",
  "last_heartbeat": "2026-05-15 10:30:00",
  "capacity": 10,
  "current_load": 3,
  "created_at": "2026-05-15 10:00:00",
  "updated_at": "2026-05-15 10:30:00"
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | name is required | name参数为空 |
| 401 | unauthorized | 未授权或Token无效 |

**特殊说明**：无

---

### 标记执行器在线

**接口地址**：`POST /api/executors/:name/online`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| name | string | 是 | 执行器名称 |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/executors/executor-1/online \
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
| 400 | name is required | name参数为空 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 标记执行器离线

**接口地址**：`POST /api/executors/:name/offline`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| name | string | 是 | 执行器名称 |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/executors/executor-1/offline \
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
| 400 | name is required | name参数为空 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 更新执行器容量

**接口地址**：`PUT /api/executors/:name/capacity`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| name | string | 是 | 执行器名称 |

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| capacity | int64 | 是 | 最大并发任务数（必须大于等于1） |

**请求示例**：

```bash
curl -X PUT http://localhost:8080/api/executors/executor-1/capacity \
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
| 400 | name is required | name参数为空 |
| 400 | invalid request: capacity must be a positive integer | capacity必须是正整数 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 删除执行器

**接口地址**：`DELETE /api/executors/:name`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| name | string | 是 | 执行器名称 |

**请求示例**：

```bash
curl -X DELETE http://localhost:8080/api/executors/executor-1 \
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
| 400 | name is required | name参数为空 |
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

**接口地址**：`GET /api/admin/permissions`

**权限要求**：system_admin

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/admin/permissions \
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

**接口地址**：`GET /api/admin/users`

**权限要求**：system_admin

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/admin/users \
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

**接口地址**：`GET /api/admin/users/:id`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 用户 ID |

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/admin/users/1 \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "user": {...},
  "roles": [...]
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

**接口地址**：`POST /api/admin/users`

**权限要求**：system_admin

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| username | string | 是 | 用户名（最少3个字符，最大50个字符，字母数字） |
| email | string | 是 | 邮箱地址 |
| password | string | 是 | 密码（最少6个字符） |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/admin/users \
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

**接口地址**：`PUT /api/admin/users/:id`

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
curl -X PUT http://localhost:8080/api/admin/users/1 \
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

**接口地址**：`DELETE /api/admin/users/:id`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 用户 ID |

**请求示例**：

```bash
curl -X DELETE http://localhost:8080/api/admin/users/1 \
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

**接口地址**：`POST /api/admin/users/:id/roles`

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
curl -X POST http://localhost:8080/api/admin/users/1/roles \
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
  "message": "roles assigned successfully"
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

**接口地址**：`GET /api/admin/users/:id/roles`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 用户 ID |

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/admin/users/1/roles \
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

**接口地址**：`POST /api/admin/users/:id/domains`

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
curl -X POST http://localhost:8080/api/admin/users/1/domains \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "domain_ids": [1, 2]
  }'
```

**正常响应**：

```json
{
  "message": "domains assigned successfully"
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

**接口地址**：`POST /api/admin/users/:id/reset-password`

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
curl -X POST http://localhost:8080/api/admin/users/1/reset-password \
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

**接口地址**：`GET /api/admin/roles`

**权限要求**：system_admin

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/admin/roles \
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

**接口地址**：`GET /api/admin/roles/:id`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 角色 ID |

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/admin/roles/1 \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "id": 1,
  "name": "系统管理员",
  "code": "system_admin",
  "description": "系统最高权限角色",
  "permissions": [...]
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 404 | role not found | 角色不存在 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

#### 创建角色

**接口地址**：`POST /api/admin/roles`

**权限要求**：system_admin

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| name | string | 是 | 角色名称 |
| code | string | 是 | 角色编码（唯一） |
| description | string | 否 | 角色描述 |
| permission_ids | int64[] | 否 | 权限 ID 数组 |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/admin/roles \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "运维人员",
    "code": "operator",
    "description": "运维操作角色",
    "permission_ids": [1, 2, 3]
  }'
```

**正常响应**：返回创建的角色对象

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | name is required | 角色名称为空 |
| 400 | code is required | 角色编码为空 |
| 400 | 解析错误 | 请求体格式不正确 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

#### 更新角色

**接口地址**：`PUT /api/admin/roles/:id`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 角色 ID |

**请求参数**：所有字段均为可选

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| name | string | 否 | 角色名称 |
| code | string | 否 | 角色编码 |
| description | string | 否 | 角色描述 |

**请求示例**：

```bash
curl -X PUT http://localhost:8080/api/admin/roles/1 \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "更新后的角色名",
    "description": "更新后的描述"
  }'
```

**正常响应**：返回更新后的角色对象

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 400 | 解析错误 | 请求体格式不正确 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 404 | role not found | 角色不存在 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

#### 删除角色

**接口地址**：`DELETE /api/admin/roles/:id`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 角色 ID |

**请求示例**：

```bash
curl -X DELETE http://localhost:8080/api/admin/roles/1 \
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

### 领域管理

#### 获取领域列表

**接口地址**：`GET /api/admin/domains`

**权限要求**：system_admin

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/admin/domains \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "items": [
    {
      "id": 1,
      "name": "默认领域",
      "description": "系统默认领域",
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

#### 创建领域

**接口地址**：`POST /api/admin/domains`

**权限要求**：system_admin

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| name | string | 是 | 领域名称 |
| description | string | 否 | 领域描述 |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/admin/domains \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "新领域",
    "description": "领域描述"
  }'
```

**正常响应**：返回创建的领域对象

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | name is required | 领域名称为空 |
| 400 | 解析错误 | 请求体格式不正确 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

#### 更新领域

**接口地址**：`PUT /api/admin/domains/:id`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 领域 ID |

**请求参数**：所有字段均为可选

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| name | string | 否 | 领域名称 |
| description | string | 否 | 领域描述 |

**请求示例**：

```bash
curl -X PUT http://localhost:8080/api/admin/domains/1 \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "更新后的领域名",
    "description": "更新后的描述"
  }'
```

**正常响应**：返回更新后的领域对象

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 400 | 解析错误 | 请求体格式不正确 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 404 | domain not found | 领域不存在 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

#### 删除领域

**接口地址**：`DELETE /api/admin/domains/:id`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 领域 ID |

**请求示例**：

```bash
curl -X DELETE http://localhost:8080/api/admin/domains/1 \
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

## 数据源接口

### 获取数据源列表

**接口地址**：`GET /api/datasources`

**权限要求**：已登录用户

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/datasources \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "items": [
    {
      "id": 1,
      "name": "生产MySQL",
      "type": "mysql",
      "host": "192.168.1.100",
      "port": 3306,
      "database": "production",
      "username": "readonly",
      "auth_type": "password",
      "connection_mode": "direct",
      "is_enabled": true,
      "domain_id": 1,
      "description": "生产环境MySQL数据源",
      "allow_write_sql": false,
      "created_at": "2026-05-15 10:00:00",
      "updated_at": "2026-05-15 10:00:00"
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

### 获取支持的数据源类型

**接口地址**：`GET /api/datasources/types`

**权限要求**：已登录用户

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/datasources/types \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "items": ["mysql", "postgresql", "clickhouse", "doris", "hive", "rqlite"]
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 401 | unauthorized | 未授权或Token无效 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 测试数据源连接(参数)

**接口地址**：`POST /api/datasources/test`

**权限要求**：system_admin 或 domain_admin

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| type | string | 是 | 数据源类型 |
| host | string | 是 | 主机地址 |
| port | int | 是 | 端口号 |
| database | string | 是 | 数据库名 |
| username | string | 是 | 用户名 |
| password | string | 是 | 密码 |
| connection_mode | string | 否 | 连接模式（direct/proxy） |
| zk_hosts | string | 否 | ZooKeeper 地址（Hive使用） |
| zk_path | string | 否 | ZooKeeper 路径（Hive使用） |
| rqlite_hosts | string | 否 | rqlite 集群地址 |
| config | string | 否 | 额外配置（JSON格式） |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/datasources/test \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "mysql",
    "host": "192.168.1.100",
    "port": 3306,
    "database": "test_db",
    "username": "root",
    "password": "password123"
  }'
```

**正常响应**：

```json
{
  "success": true,
  "message": "connection successful"
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | type is required | 数据源类型为空 |
| 400 | host is required | 主机地址为空 |
| 400 | 解析错误 | 请求体格式不正确 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 创建数据源

**接口地址**：`POST /api/datasources`

**权限要求**：system_admin 或 domain_admin

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| name | string | 是 | 数据源名称 |
| type | string | 是 | 数据源类型 |
| host | string | 是 | 主机地址 |
| port | int | 是 | 端口号 |
| path | string | 否 | 连接路径 |
| database | string | 是 | 数据库名 |
| username | string | 是 | 用户名 |
| password | string | 是 | 密码 |
| auth_type | string | 否 | 认证类型 |
| connection_mode | string | 否 | 连接模式（direct/proxy） |
| zk_hosts | string | 否 | ZooKeeper 地址（Hive使用） |
| zk_path | string | 否 | ZooKeeper 路径（Hive使用） |
| rqlite_hosts | string | 否 | rqlite 集群地址 |
| config | string | 否 | 额外配置（JSON格式） |
| description | string | 否 | 描述 |
| domain_id | int64 | 否 | 所属领域 ID |
| is_enabled | bool | 否 | 是否启用，默认 true |
| allow_write_sql | bool | 否 | 是否允许写SQL，默认 false |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/datasources \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "生产MySQL",
    "type": "mysql",
    "host": "192.168.1.100",
    "port": 3306,
    "database": "production",
    "username": "readonly",
    "password": "password123",
    "auth_type": "password",
    "connection_mode": "direct",
    "description": "生产环境MySQL数据源",
    "domain_id": 1,
    "is_enabled": true,
    "allow_write_sql": false
  }'
```

**正常响应**：返回创建的数据源对象

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | name is required | 数据源名称为空 |
| 400 | type is required | 数据源类型为空 |
| 400 | 解析错误 | 请求体格式不正确 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 获取数据源详情

**接口地址**：`GET /api/datasources/:id`

**权限要求**：datasource read

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 数据源 ID |

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/datasources/1 \
  -H "Authorization: Bearer <token>"
```

**正常响应**：同创建数据源响应

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 404 | datasource not found | 数据源不存在 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 更新数据源

**接口地址**：`PUT /api/datasources/:id`

**权限要求**：datasource update

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 数据源 ID |

**请求参数**：所有字段均为可选，只更新提供的字段

**请求示例**：

```bash
curl -X PUT http://localhost:8080/api/datasources/1 \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "更新后的数据源名",
    "is_enabled": false
  }'
```

**正常响应**：返回更新后的数据源对象

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 400 | 解析错误 | 请求体格式不正确 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 404 | datasource not found | 数据源不存在 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 删除数据源

**接口地址**：`DELETE /api/datasources/:id`

**权限要求**：datasource delete

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 数据源 ID |

**请求示例**：

```bash
curl -X DELETE http://localhost:8080/api/datasources/1 \
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

### 测试数据源连接(ID)

**接口地址**：`POST /api/datasources/:id/test`

**权限要求**：datasource read

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 数据源 ID |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/datasources/1/test \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "success": true,
  "message": "connection successful"
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | id must be positive | ID必须为正数 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 404 | datasource not found | 数据源不存在 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 授权数据源权限

**接口地址**：`POST /api/datasources/:id/permissions`

**权限要求**：system_admin 或 domain_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 数据源 ID |

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| role_id | int64 | 否 | 角色 ID（role_id 或 user_id 至少一个） |
| user_id | int64 | 否 | 用户 ID（role_id 或 user_id 至少一个） |
| permission_type | string | 是 | 权限类型：query/download/manage |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/datasources/1/permissions \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "role_id": 2,
    "permission_type": "query"
  }'
```

**正常响应**：返回创建的权限对象

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | permission_type is required | 权限类型为空 |
| 400 | 解析错误 | 请求体格式不正确 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 更新数据源权限

**接口地址**：`PUT /api/datasources/:id/permissions/:perm_id`

**权限要求**：system_admin 或 domain_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 数据源 ID |
| perm_id | int64 | 是 | 权限 ID |

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| permission_type | string | 是 | 权限类型：query/download/manage |

**请求示例**：

```bash
curl -X PUT http://localhost:8080/api/datasources/1/permissions/1 \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "permission_type": "manage"
  }'
```

**正常响应**：返回更新后的权限对象

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 400 | permission_type is required | 权限类型为空 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 404 | permission not found | 权限不存在 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 撤销数据源权限

**接口地址**：`DELETE /api/datasources/:id/permissions/:perm_id`

**权限要求**：system_admin 或 domain_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 数据源 ID |
| perm_id | int64 | 是 | 权限 ID |

**请求示例**：

```bash
curl -X DELETE http://localhost:8080/api/datasources/1/permissions/1 \
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
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 404 | permission not found | 权限不存在 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 获取数据源权限列表

**接口地址**：`GET /api/datasources/:id/permissions`

**权限要求**：datasource manage

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 数据源 ID |

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/datasources/1/permissions \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "items": [
    {
      "id": 1,
      "datasource_id": 1,
      "role_id": 2,
      "user_id": null,
      "permission_type": "query",
      "created_at": "2026-05-15 10:00:00"
    }
  ]
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 获取数据源元数据

**接口地址**：`GET /api/datasources/:id/metadata`

**权限要求**：datasource query

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 数据源 ID |

**查询参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| database | string | 否 | 数据库名（部分数据源支持） |

**请求示例**：

```bash
curl -X GET "http://localhost:8080/api/datasources/1/metadata?database=production" \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "databases": ["production", "staging"],
  "tables": [
    {
      "name": "users",
      "columns": [
        {"name": "id", "type": "bigint", "nullable": false},
        {"name": "username", "type": "varchar(50)", "nullable": false}
      ]
    }
  ]
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | invalid id | ID参数无效 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 404 | datasource not found | 数据源不存在 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

## 查询接口

### 执行SQL查询

**接口地址**：`POST /api/query/execute`

**权限要求**：datasource query

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| datasource_id | int64 | 是 | 数据源 ID |
| sql | string | 是 | SQL 语句 |
| database | string | 否 | 数据库名 |
| limit | int | 否 | 返回行数限制，默认 1000 |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/query/execute \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "datasource_id": 1,
    "sql": "SELECT * FROM users LIMIT 10",
    "database": "production",
    "limit": 10
  }'
```

**正常响应**：

```json
{
  "query_id": "query-20260515-abc123",
  "columns": ["id", "username", "email"],
  "rows": [
    [1, "admin", "admin@example.com"],
    [2, "user1", "user1@example.com"]
  ],
  "total": 2,
  "duration_ms": 150
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | datasource_id is required | 数据源ID为空 |
| 400 | sql is required | SQL语句为空 |
| 400 | 解析错误 | 请求体格式不正确 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：
- 查询结果默认限制 1000 行
- 仅允许 SELECT 查询（除非数据源开启了 allow_write_sql）

---

### 取消查询

**接口地址**：`POST /api/query/cancel/:query_id`

**权限要求**：datasource query

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| query_id | string | 是 | 查询 ID |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/query/cancel/query-20260515-abc123 \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "message": "query cancelled"
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | query_id is required | query_id参数为空 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 404 | query not found | 查询不存在 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 导出CSV

**接口地址**：`POST /api/query/export`

**权限要求**：datasource download

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| datasource_id | int64 | 是 | 数据源 ID |
| sql | string | 是 | SQL 语句 |
| database | string | 否 | 数据库名 |
| limit | int | 否 | 返回行数限制 |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/query/export \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "datasource_id": 1,
    "sql": "SELECT * FROM users LIMIT 100",
    "database": "production"
  }'
```

**正常响应**：CSV 文件流（Content-Type: text/csv）

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | datasource_id is required | 数据源ID为空 |
| 400 | sql is required | SQL语句为空 |
| 400 | 解析错误 | 请求体格式不正确 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：返回 CSV 文件下载流

---

### 获取查询历史

**接口地址**：`GET /api/query/history`

**权限要求**：已登录用户

**查询参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| page | int | 否 | 页码，默认 1 |
| page_size | int | 否 | 每页数量，默认 20 |
| datasource_id | int64 | 否 | 数据源 ID |
| status | string | 否 | 查询状态 |

**请求示例**：

```bash
curl -X GET "http://localhost:8080/api/query/history?page=1&page_size=20" \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "items": [
    {
      "id": 1,
      "query_id": "query-20260515-abc123",
      "datasource_id": 1,
      "sql": "SELECT * FROM users LIMIT 10",
      "database": "production",
      "status": "success",
      "duration_ms": 150,
      "row_count": 10,
      "created_by": 1,
      "created_at": "2026-05-15 10:00:00"
    }
  ],
  "total": 50,
  "page": 1,
  "page_size": 20
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 401 | unauthorized | 未授权或Token无效 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 删除查询历史

**接口地址**：`DELETE /api/query/history/:id`

**权限要求**：已登录用户

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 查询历史 ID |

**请求示例**：

```bash
curl -X DELETE http://localhost:8080/api/query/history/1 \
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
| 401 | unauthorized | 未授权或Token无效 |
| 404 | history not found | 查询历史不存在 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 批量删除查询历史

**接口地址**：`POST /api/query/history/batch-delete`

**权限要求**：已登录用户

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| ids | int64[] | 是 | 要删除的查询历史 ID 数组 |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/query/history/batch-delete \
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

### 获取保存的SQL列表

**接口地址**：`GET /api/query/saved-sql`

**权限要求**：已登录用户

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/query/saved-sql \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "items": [
    {
      "id": 1,
      "name": "用户统计查询",
      "datasource_id": 1,
      "sql_text": "SELECT COUNT(*) FROM users",
      "description": "统计用户总数",
      "is_public": true,
      "created_by": 1,
      "created_at": "2026-05-15 10:00:00",
      "updated_at": "2026-05-15 10:00:00"
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

### 保存SQL

**接口地址**：`POST /api/query/saved-sql`

**权限要求**：已登录用户

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| name | string | 是 | SQL 名称 |
| datasource_id | int64 | 是 | 数据源 ID |
| sql_text | string | 是 | SQL 内容 |
| description | string | 否 | 描述 |
| is_public | bool | 否 | 是否公开，默认 false |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/query/saved-sql \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "用户统计查询",
    "datasource_id": 1,
    "sql_text": "SELECT COUNT(*) FROM users",
    "description": "统计用户总数",
    "is_public": true
  }'
```

**正常响应**：返回创建的保存SQL对象

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | name is required | SQL名称为空 |
| 400 | sql_text is required | SQL内容为空 |
| 400 | 解析错误 | 请求体格式不正确 |
| 401 | unauthorized | 未授权或Token无效 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 删除保存的SQL

**接口地址**：`DELETE /api/query/saved-sql/:id`

**权限要求**：已登录用户

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | 是 | 保存的SQL ID |

**请求示例**：

```bash
curl -X DELETE http://localhost:8080/api/query/saved-sql/1 \
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
| 401 | unauthorized | 未授权或Token无效 |
| 404 | saved sql not found | 保存的SQL不存在 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

## 审计日志接口

### 获取审计日志列表

**接口地址**：`GET /api/admin/audit-logs`

**权限要求**：system_admin

**查询参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| username | string | 否 | 用户名筛选 |
| action | string | 否 | 操作类型筛选 |
| resource | string | 否 | 资源类型筛选 |
| status | string | 否 | 状态筛选 |
| start_time | string | 否 | 开始时间 |
| end_time | string | 否 | 结束时间 |
| page | int | 否 | 页码，默认 1 |
| page_size | int | 否 | 每页数量，默认 20 |

**请求示例**：

```bash
curl -X GET "http://localhost:8080/api/admin/audit-logs?page=1&page_size=20" \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "items": [
    {
      "id": 1,
      "username": "admin",
      "action": "login",
      "resource": "auth",
      "resource_id": "",
      "status": "success",
      "ip_address": "192.168.1.1",
      "user_agent": "Mozilla/5.0",
      "detail": "用户登录成功",
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
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 获取审计日志统计

**接口地址**：`GET /api/admin/audit-logs/stats`

**权限要求**：system_admin

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/admin/audit-logs/stats \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "total": 1000,
  "by_action": {
    "login": 500,
    "query_execute": 300,
    "datasource_create": 50
  },
  "by_status": {
    "success": 950,
    "failed": 50
  }
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

### 清理过期审计日志

**接口地址**：`POST /api/admin/audit-logs/clean`

**权限要求**：system_admin

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| retention_days | int | 是 | 保留天数 |

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/admin/audit-logs/clean \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "retention_days": 90
  }'
```

**正常响应**：

```json
{
  "message": "cleaned successfully",
  "deleted_count": 150
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | retention_days is required | 保留天数为空 |
| 400 | 解析错误 | 请求体格式不正确 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

### 获取审计日志保留天数

**接口地址**：`GET /api/admin/audit-logs/retention`

**权限要求**：system_admin

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/admin/audit-logs/retention \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "retention_days": 90
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

### 更新审计日志保留天数

**接口地址**：`PUT /api/admin/audit-logs/retention`

**权限要求**：system_admin

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| retention_days | int | 是 | 保留天数 |

**请求示例**：

```bash
curl -X PUT http://localhost:8080/api/admin/audit-logs/retention \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "retention_days": 180
  }'
```

**正常响应**：

```json
{
  "message": "retention updated",
  "retention_days": 180
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | retention_days is required | 保留天数为空 |
| 400 | 解析错误 | 请求体格式不正确 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

## 系统配置接口

### 获取系统配置列表

**接口地址**：`GET /api/admin/system-config`

**权限要求**：system_admin

**请求示例**：

```bash
curl -X GET http://localhost:8080/api/admin/system-config \
  -H "Authorization: Bearer <token>"
```

**正常响应**：

```json
{
  "items": [
    {
      "key": "audit_retention_days",
      "value": "90",
      "description": "审计日志保留天数",
      "updated_at": "2026-05-15 10:00:00"
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

### 更新系统配置

**接口地址**：`PUT /api/admin/system-config/:key`

**权限要求**：system_admin

**路径参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| key | string | 是 | 配置键名 |

**请求参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| value | string | 是 | 配置值 |

**请求示例**：

```bash
curl -X PUT http://localhost:8080/api/admin/system-config/audit_retention_days \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "value": "180"
  }'
```

**正常响应**：

```json
{
  "key": "audit_retention_days",
  "value": "180",
  "updated_at": "2026-05-15 10:00:00"
}
```

**错误返回**：

| 状态码 | 错误信息 | 说明 |
|--------|----------|------|
| 400 | value is required | 配置值为空 |
| 400 | 解析错误 | 请求体格式不正确 |
| 401 | unauthorized | 未授权或Token无效 |
| 403 | Forbidden | 权限不足 |
| 404 | config key not found | 配置键不存在 |
| 500 | 内部错误 | 服务器内部错误 |

**特殊说明**：无

---

## 数据模型