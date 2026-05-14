# API 接口文档

## 目录
1. [概述](#概述)
2. [认证接口](#认证接口)
3. [任务接口](#任务接口)
4. [工作流接口](#工作流接口)
5. [执行器接口](#执行器接口)
6. [日志接口](#日志接口)
7. [Webhook 接口](#webhook-接口)

---

## 概述

### 基础信息
- **基础 URL: `http://your-domain/api/v1`
- **数据格式**: JSON
- **认证方式**: Bearer Token

### 通用响应格式

#### 成功响应
```json
{
  "data": {}
}
```

#### 错误响应
```json
{
  "error": "错误描述信息"
}
```

### 状态码说明
| 状态码 | 说明 |
|--------|------|
| 200 | 请求成功 |
| 201 | 资源创建成功 |
| 400 | 请求参数错误 |
| 401 | 未授权，需要登录 |
| 404 | 资源未找到 |
| 500 | 服务器内部错误 |

---

## 认证接口

### 用户登录
**接口地址**: `POST /auth/login`

**请求参数**:
```json
{
  "username": "用户名",
  "password": "密码"
}
```

**响应示例**:
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

### 获取当前用户信息
**接口地址**: `GET /auth/current`

**请求头**:
```
Authorization: Bearer {token}
```

**响应示例**:
```json
{
  "id": 1,
  "username": "admin",
  "role": "admin",
  "email": "admin@example.com",
  "domain_id": 1
}
```

### 用户注册
**接口地址**: `POST /auth/register`

**请求参数**:
```json
{
  "username": "用户名",
  "password": "密码",
  "role": "角色",
  "email": "邮箱"
}
```

**响应示例**:
```json
{
  "id": 2,
  "username": "newuser",
  "role": "operator",
  "email": "newuser@example.com"
}
```

---

## 任务接口

### 获取任务列表
**接口地址**: `GET /tasks`

**请求头**:
```
Authorization: Bearer {token}
```

**响应示例**:
```json
{
  "items": [
    {
      "id": 1,
      "workflow_id": null,
      "name": "示例任务",
      "type": "http",
      "config": "{\"url\":\"https://example.com\"}",
      "cron_expression": "0 * * * *",
      "timeout_seconds": 300,
      "retry_count": 3,
      "retry_interval": 5,
      "is_enabled": true,
      "status": "pending",
      "domain_id": 1,
      "webhook_config": "{\"url\":\"https://your-webhook-url\",\"events\":[\"success\"]}",
      "created_by": 1,
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z",
      "next_execution_time": "2024-01-01T01:00:00Z",
      "last_execution_status": "success"
    }
  ]
}
```

### 获取单个任务详情
**接口地址**: `GET /tasks/{id}`

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | integer | 是 | 任务ID |

**响应示例**:
```json
{
  "id": 1,
  "workflow_id": null,
  "name": "示例任务",
  "type": "http",
  "config": "{\"url\":\"https://example.com\"}",
  "cron_expression": "0 * * * *",
  "timeout_seconds": 300,
  "retry_count": 3,
  "retry_interval": 5,
  "is_enabled": true,
  "status": "pending",
  "domain_id": 1,
  "webhook_config": "{\"url\":\"https://your-webhook-url\",\"events\":[\"success\"]}",
  "created_by": 1,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

### 创建任务
**接口地址**: `POST /tasks`

**请求参数**:
```json
{
  "workflow_id": null,
  "name": "示例任务",
  "type": "http",
  "config": {
    "url": "https://example.com",
    "method": "GET",
    "headers": {}
  },
  "cron_expression": "0 * * * *",
  "timeout_seconds": 300,
  "retry_count": 3,
  "retry_interval": 5,
  "is_enabled": true,
  "domain_id": 1,
  "webhook_config": "{\"url\":\"https://your-webhook-url\",\"events\":[\"success\",\"failed\"]}"
}
```

**参数说明**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | 任务名称 |
| type | string | 是 | 任务类型 (http, shell等) |
| config | object/string | 是 | 任务配置 |
| cron_expression | string | 否 | Cron表达式，支持5位或6位格式 |
| timeout_seconds | integer | 否 | 超时时间(秒)，默认300 |
| retry_count | integer | 否 | 重试次数，默认3 |
| retry_interval | integer | 否 | 重试间隔(秒)，默认5 |
| is_enabled | boolean | 否 | 是否启用，默认false |
| webhook_config | string | 否 | Webhook配置JSON字符串 |

**Webhook配置格式:
```json
{
  "url": "https://your-webhook-url",
  "method": "POST",
  "headers": {
    "X-Custom-Header": "value"
  },
  "events": ["success", "failed"]
}
```

**响应示例**:
```json
{
  "id": 2,
  "name": "示例任务",
  "type": "http",
  "config": "{\"url\":\"https://example.com\"}",
  "cron_expression": "0 * * * *",
  "timeout_seconds": 300,
  "retry_count": 3,
  "retry_interval": 5,
  "is_enabled": true,
  "status": "pending",
  "domain_id": 1,
  "webhook_config": "{\"url\":\"https://your-webhook-url\",\"events\":[\"success\",\"failed\"]}",
  "created_by": 1,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

### 更新任务
**接口地址**: `PUT /tasks/{id}`

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | integer | 是 | 任务ID |

**请求参数**:
```json
{
  "name": "更新后的任务名称",
  "cron_expression": "*/30 * * * *",
  "is_enabled": false
}
```

**响应示例**:
```json
{
  "id": 1,
  "name": "更新后的任务名称",
  "type": "http",
  "config": "{\"url\":\"https://example.com\"}",
  "cron_expression": "*/30 * * * *",
  "timeout_seconds": 300,
  "retry_count": 3,
  "retry_interval": 5,
  "is_enabled": false,
  "status": "pending",
  "domain_id": 1,
  "webhook_config": "{\"url\":\"https://your-webhook-url\",\"events\":[\"success\"]}",
  "created_by": 1,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

### 删除任务
**接口地址**: `DELETE /tasks/{id}`

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | integer | 是 | 任务ID |

**响应示例**:
```json
{
  "message": "deleted"
}
```

### 手动触发任务
**接口地址**: `POST /tasks/{id}/trigger`

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | integer | 是 | 任务ID |

**响应示例**:
```json
{
  "message": "triggered",
  "execution_id": "exec-1234567890"
}
```

### 获取任务执行记录
**接口地址**: `GET /tasks/{id}/executions`

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | integer | 是 | 任务ID |

**响应示例**:
```json
[
  {
    "id": 1,
    "task_id": 1,
    "execution_id": "exec-1234567890",
    "executor_id": "executor-1",
    "status": "success",
    "start_time": "2024-01-01T00:00:00Z",
    "end_time": "2024-01-01T00:00:05Z",
    "output": "任务输出内容",
    "error": "",
    "retry_times": 0,
    "created_at": "2024-01-01T00:00:00Z"
  }
]
```

### 获取任务执行日志
**接口地址**: `GET /tasks/executions/{executionId}/logs`

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| executionId | string | 是 | 执行ID |

**响应示例**:
```json
[
  {
    "id": 1,
    "execution_id": "exec-1234567890",
    "task_id": 1,
    "executor_id": "executor-1",
    "node_id": "",
    "log_level": "info",
    "message": "任务开始执行",
    "log_time": "2024-01-01T00:00:00Z"
  }
]
```

---

## 工作流接口

### 获取工作流列表
**接口地址**: `GET /workflows`

**响应示例**:
```json
[
  {
    "id": 1,
    "name": "示例工作流",
    "description": "工作流描述",
    "domain_id": 1,
    "dag_config": "{\"nodes\":[],\"edges\":[]}",
    "cron_expression": "0 0 * * *",
    "is_enabled": true,
    "created_by": 1,
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
]
```

### 获取单个工作流详情
**接口地址**: `GET /workflows/{id}`

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | integer | 是 | 工作流ID |

**响应示例**:
```json
{
  "id": 1,
  "name": "示例工作流",
  "description": "工作流描述",
  "domain_id": 1,
  "dag_config": "{\"nodes\":[],\"edges\":[]}",
  "cron_expression": "0 0 * * *",
  "is_enabled": true,
  "created_by": 1,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

### 创建工作流
**接口地址**: `POST /workflows`

**请求参数**:
```json
{
  "name": "示例工作流",
  "description": "工作流描述",
  "domain_id": 1,
  "dag_config": "{\"nodes\":[],\"edges\":[]}",
  "cron_expression": "0 0 * * *",
  "is_enabled": true
}
```

**响应示例**:
```json
{
  "id": 2,
  "name": "示例工作流",
  "description": "工作流描述",
  "domain_id": 1,
  "dag_config": "{\"nodes\":[],\"edges\":[]}",
  "cron_expression": "0 0 * * *",
  "is_enabled": true,
  "created_by": 1,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

### 更新工作流
**接口地址**: `PUT /workflows/{id}`

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | integer | 是 | 工作流ID |

**响应示例**:
```json
{
  "id": 1,
  "name": "更新后的工作流",
  "description": "更新后的描述",
  "domain_id": 1,
  "dag_config": "{\"nodes\":[],\"edges\":[]}",
  "cron_expression": "0 0 * * *",
  "is_enabled": true,
  "created_by": 1,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

### 删除工作流
**接口地址**: `DELETE /workflows/{id}`

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | integer | 是 | 工作流ID |

**响应示例**:
```json
{
  "message": "deleted"
}
```

### 手动触发工作流
**接口地址**: `POST /workflows/{id}/trigger`

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | integer | 是 | 工作流ID |

**响应示例**:
```json
{
  "execution_id": "workflow-exec-1234567890"
}
```

### 获取工作流执行记录
**接口地址**: `GET /workflows/{id}/executions`

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | integer | 是 | 工作流ID |

**响应示例**:
```json
[
  {
    "id": 1,
    "workflow_id": 1,
    "execution_id": "workflow-exec-1234567890",
    "status": "success",
    "start_time": "2024-01-01T00:00:00Z",
    "end_time": "2024-01-01T00:00:10Z",
    "node_states": "{\"node1\":\"success\"}",
    "created_at": "2024-01-01T00:00:00Z"
  }
]
```

### 获取工作流执行详情
**接口地址**: `GET /workflows/executions/{executionId}`

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| executionId | string | 是 | 执行ID |

**响应示例**:
```json
{
  "id": 1,
  "workflow_id": 1,
  "execution_id": "workflow-exec-1234567890",
  "status": "success",
  "start_time": "2024-01-01T00:00:00Z",
  "end_time": "2024-01-01T00:00:10Z",
  "node_states": "{\"node1\":\"success\"}",
  "created_at": "2024-01-01T00:00:00Z"
}
```

### 获取工作流执行日志
**接口地址**: `GET /workflows/executions/{executionId}/logs`

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| executionId | string | 是 | 执行ID |

**响应示例**:
```json
[
  {
    "id": 1,
    "execution_id": "workflow-exec-1234567890",
    "task_id": 1,
    "executor_id": "executor-1",
    "node_id": "node1",
    "log_level": "info",
    "message": "节点执行开始",
    "log_time": "2024-01-01T00:00:00Z"
  }
]
```

---

## 执行器接口

### 获取执行器列表
**接口地址**: `GET /executors`

**响应示例**:
```json
[
  {
    "id": 1,
    "executor_id": "executor-1",
    "name": "执行器1",
    "address": "http://executor-1:8080",
    "status": "online",
    "last_heartbeat": "2024-01-01T00:00:00Z",
    "capacity": 10,
    "current_load": 2,
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
]
```

### 获取单个执行器详情
**接口地址**: `GET /executors/{id}`

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | string | 是 | 执行器ID |

**响应示例**:
```json
{
  "id": 1,
  "executor_id": "executor-1",
  "name": "执行器1",
  "address": "http://executor-1:8080",
  "status": "online",
  "last_heartbeat": "2024-01-01T00:00:00Z",
  "capacity": 10,
  "current_load": 2,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

---

## 日志接口

### 获取执行日志列表
**接口地址**: `GET /logs`

**请求参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| executor_name | string | 否 | 执行器名称筛选 |
| task_name | string | 否 | 任务名称筛选 |
| task_type | string | 否 | 任务类型筛选 |
| status | string | 否 | 状态筛选 |
| page | integer | 否 | 页码 |
| page_size | integer | 否 | 每页数量 |

**响应示例**:
```json
{
  "items": [
    {
      "id": 1,
      "task_id": 1,
      "execution_id": "exec-1234567890",
      "executor_id": "executor-1",
      "status": "success",
      "start_time": "2024-01-01T00:00:00Z",
      "end_time": "2024-01-01T00:00:05Z",
      "output": "输出内容",
      "error": "",
      "retry_times": 0,
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "total": 100,
  "page": 1,
  "page_size": 20
}
```

### 获取执行统计
**接口地址**: `GET /logs/stats`

**请求参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| executor_name | string | 否 | 执行器名称筛选 |
| task_name | string | 否 | 任务名称筛选 |
| task_type | string | 否 | 任务类型筛选 |
| status | string | 否 | 状态筛选 |

**响应示例**:
```json
{
  "success": 50,
  "failed": 10,
  "pending": 5,
  "running": 2
}
```

### 删除日志
**接口地址**: `DELETE /logs/{id}`

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | integer | 是 | 日志ID |

**响应示例**:
```json
{
  "message": "deleted"
}
```

### 批量删除日志
**接口地址**: `POST /logs/batch-delete`

**请求参数**:
```json
{
  "ids": [1, 2, 3]
}
```

**响应示例**:
```json
{
  "message": "deleted"
}
```

---

## Webhook 接口

关于Webhook的详细使用说明请参考 [Webhook接入指南](./WEBHOOK.md)。
