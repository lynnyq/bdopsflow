# Webhook 重新设计

## 背景

当前 webhook 配置直接嵌入在 Task 模型的 `webhook_config` 字段中（JSON 字符串），存在以下问题：

1. **无法集中管理**：每个任务独立配置 webhook URL，相同地址需重复填写
2. **无法复用**：多个任务推送同一地址时，需逐个配置
3. **Handler 空壳**：现有 WebhookHandler 的 Create/List/Delete 未实现持久化
4. **无签名验证**：推送消息缺少来源验证机制

## 设计目标

- Webhook 配置从任务中抽离，在系统设置中集中管理
- 任务创建时通过下拉选择关联 webhook
- 支持领域级别的 webhook 隔离
- 推送消息支持 HMAC 签名验证

## 设计决策

| 决策项 | 选择 | 理由 |
|--------|------|------|
| 作用域 | 领域级别 | 每个领域独立管理自己的 webhook |
| 关联数量 | 单个 webhook | 一个任务关联一个 webhook，简化逻辑 |
| 推送时机配置位置 | 任务级别 | 不同任务可选择不同推送时机 |
| 存储方案 | 独立表 + 外键关联 | 集中管理、数据一致、一处修改全局生效 |
| 删除策略 | 置空关联 | 删除 webhook 时关联任务的 webhook_id 置 NULL |

## 数据模型

### 新增 `webhooks` 表

```sql
CREATE TABLE IF NOT EXISTS webhooks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    method TEXT DEFAULT 'POST',
    headers TEXT DEFAULT '{}',
    secret TEXT DEFAULT '',
    domain_id INTEGER NOT NULL,
    is_enabled BOOLEAN DEFAULT TRUE,
    description TEXT DEFAULT '',
    created_by INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

字段说明：

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键自增 |
| name | TEXT | Webhook 名称，如"钉钉通知"、"企业微信告警" |
| url | TEXT | Webhook 接收地址 |
| method | TEXT | HTTP 方法，默认 POST |
| headers | TEXT | 自定义请求头，JSON 格式，如 `{"Authorization":"Bearer xxx"}` |
| secret | TEXT | HMAC-SHA256 签名密钥，用于接收方验证推送来源 |
| domain_id | INTEGER | 所属领域 ID |
| is_enabled | BOOLEAN | 是否启用，禁用后关联任务不再推送 |
| description | TEXT | 描述说明 |
| created_by | INTEGER | 创建人 ID |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

### 修改 `tasks` 表

**删除字段**：`webhook_config TEXT`（保留列但代码不再使用，因 rqlite/SQLite 不支持 DROP COLUMN）

**新增字段**：

```sql
ALTER TABLE tasks ADD COLUMN webhook_id INTEGER;
ALTER TABLE tasks ADD COLUMN webhook_events TEXT DEFAULT '[]';
```

| 字段 | 类型 | 说明 |
|------|------|------|
| webhook_id | INTEGER | 关联 webhooks 表，NULL 表示不推送 |
| webhook_events | TEXT | 推送时机，JSON 数组，如 `["success","failed"]` |

### 数据迁移

需将现有 `webhook_config` 中的数据迁移到 `webhooks` 表：

1. 解析每个任务的 `webhook_config` JSON
2. 按 domain_id 分组，相同 URL 合并为一条 webhook 记录
3. 将任务的 `webhook_id` 指向新创建的 webhook 记录
4. 将 `webhook_config` 中的 events 提取到 `webhook_events` 字段
5. 迁移完成后保留 `webhook_config` 列但不再使用（rqlite/SQLite 不支持 DROP COLUMN），代码层面忽略该字段

## 后端设计

### API 设计

| 方法 | 路径 | 说明 | 权限 |
|------|------|------|------|
| GET | `/api/v1/webhooks` | 获取当前领域的 webhook 列表 | 领域管理员+ |
| POST | `/api/v1/webhooks` | 创建 webhook | 领域管理员+ |
| PUT | `/api/v1/webhooks/:id` | 更新 webhook | 领域管理员+ |
| DELETE | `/api/v1/webhooks/:id` | 删除 webhook（关联任务置空） | 领域管理员+ |
| POST | `/api/v1/webhooks/:id/test` | 测试 webhook 连通性 | 领域管理员+ |
| POST | `/api/v1/webhooks/trigger` | 手动触发任务 webhook 推送 | 登录用户 |

#### 请求/响应结构

**创建 Webhook**：

```json
// POST /api/v1/webhooks
{
  "name": "钉钉通知",
  "url": "https://oapi.dingtalk.com/robot/send?access_token=xxx",
  "method": "POST",
  "headers": {"Content-Type": "application/json"},
  "secret": "SECxxx",
  "domain_id": 1,
  "description": "钉钉机器人通知"
}

// Response
{
  "code": 0,
  "message": "webhook created",
  "data": {
    "id": 1,
    "name": "钉钉通知",
    "url": "https://oapi.dingtalk.com/robot/send?access_token=xxx",
    "method": "POST",
    "headers": {"Content-Type": "application/json"},
    "secret": "***",
    "domain_id": 1,
    "is_enabled": true,
    "description": "钉钉机器人通知",
    "created_by": 1,
    "created_at": "2026-05-22T10:00:00Z",
    "updated_at": "2026-05-22T10:00:00Z"
  }
}
```

**获取 Webhook 列表**：

```json
// GET /api/v1/webhooks?domain_id=1
{
  "code": 0,
  "data": {
    "items": [...]
  }
}
```

**测试 Webhook**：

```json
// POST /api/v1/webhooks/:id/test
// Response
{
  "code": 0,
  "message": "test webhook sent successfully",
  "data": {
    "status_code": 200,
    "response_time_ms": 150
  }
}
```

### 服务层

**新增 WebhookService**：

```go
type WebhookService struct {
    db *gorqlite.Connection
}

func (s *WebhookService) Create(ctx context.Context, webhook *model.Webhook) (*model.Webhook, error)
func (s *WebhookService) Update(ctx context.Context, id int64, webhook *model.Webhook) error
func (s *WebhookService) Delete(ctx context.Context, id int64) error
func (s *WebhookService) List(ctx context.Context, domainID int64) ([]*model.Webhook, error)
func (s *WebhookService) GetByID(ctx context.Context, id int64) (*model.Webhook, error)
func (s *WebhookService) Test(ctx context.Context, id int64) (*WebhookTestResult, error)
```

**SchedulerService 变更**：

`SendWebhookNotification` 方法改造流程：

1. 检查 `task.webhook_id` 是否为空，为空则跳过
2. 通过 `webhook_id` 查询 webhook 记录
3. 检查 webhook 是否启用（`is_enabled`）
4. 检查 `task.webhook_events` 是否匹配当前事件
5. 构建推送 payload，计算 HMAC-SHA256 签名
6. 发送 HTTP 请求

### 推送签名机制

```
签名算法: HMAC-SHA256(secret, payload_body)
Header: X-Webhook-Signature: sha256=<hex_digest>
Header: X-Webhook-Event: <event_name>
Header: X-Webhook-Delivery: <uuid>
```

签名计算示例：

```go
func computeSignature(secret string, body []byte) string {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(body)
    return fmt.Sprintf("sha256=%x", mac.Sum(nil))
}
```

### 推送 Payload 结构

```json
{
  "event": "success",
  "timestamp": 1747900800,
  "delivery_id": "550e8400-e29b-41d4-a716-446655440000",
  "task": {
    "id": 1,
    "name": "数据同步任务",
    "type": "http"
  },
  "execution": {
    "id": "exec-abc123",
    "status": "success",
    "output": "...",
    "error": "",
    "duration_ms": 1500
  }
}
```

### Model 新增

```go
type Webhook struct {
    ID          int64     `db:"id" json:"id"`
    Name        string    `db:"name" json:"name"`
    URL         string    `db:"url" json:"url"`
    Method      string    `db:"method" json:"method"`
    Headers     string    `db:"headers" json:"headers"`
    Secret      string    `db:"secret" json:"secret,omitempty"`
    DomainID    int64     `db:"domain_id" json:"domain_id"`
    IsEnabled   bool      `db:"is_enabled" json:"is_enabled"`
    Description string    `db:"description" json:"description"`
    CreatedBy   *int64    `db:"created_by" json:"created_by,omitempty"`
    CreatedAt   time.Time `db:"created_at" json:"created_at"`
    UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}
```

### Task Model 变更

```go
type Task struct {
    // ... 保留现有字段 ...
    WebhookID      *int64  `db:"webhook_id" json:"webhook_id"`           // 替代 WebhookConfig
    WebhookEvents  string  `db:"webhook_events" json:"webhook_events"`   // 新增
    // 删除: WebhookConfig string `db:"webhook_config" json:"webhook_config"`
}
```

## 前端设计

### 1. Webhook 管理页面

**位置**：管理后台 → 新增「Webhook 管理」菜单项（与系统配置同级）

**路由**：`/admin/webhooks` → `WebhookManagement.vue`

**页面布局**：

- 顶部工具栏：搜索框 + 创建按钮
- 表格列：名称、URL、HTTP 方法、状态（启用/禁用 switch）、描述、创建时间、操作（编辑/测试/删除）
- 创建/编辑弹窗字段：
  - 名称（必填）
  - URL（必填）
  - HTTP 方法（下拉，默认 POST）
  - 自定义 Headers（Key-Value 动态表单，可增删行）
  - 签名密钥（可选，用于 HMAC 验证）
  - 描述

### 2. 任务表单变更

**移除**：当前「Webhook推送配置」section 中的 URL 输入框

**替换为**：
- Webhook 下拉选择框：加载当前领域的 webhook 列表（仅显示 `is_enabled=true`），显示名称，值为 webhook_id
- 推送时机多选框：保持不变（成功/失败/跳过/每次），仅在选择 webhook 后显示

### 3. 路由变更

```typescript
{
  path: 'admin/webhooks',
  name: 'WebhookManagement',
  component: () => import('@/views/admin/WebhookManagement.vue'),
  meta: { requiresAdmin: true },
}
```

### 4. 前端类型与 API

**新增类型**：

```typescript
export interface Webhook {
  id: number
  name: string
  url: string
  method: string
  headers: string
  secret: string
  domain_id: number
  is_enabled: boolean
  description: string
  created_by?: number
  created_at: string
  updated_at: string
}
```

**新增 API**：

```typescript
export const webhookAPI = {
  list: (domainId: number) => api.get(`/webhooks?domain_id=${domainId}`),
  create: (data: Partial<Webhook>) => api.post('/webhooks', data),
  update: (id: number, data: Partial<Webhook>) => api.put(`/webhooks/${id}`, data),
  delete: (id: number) => api.delete(`/webhooks/${id}`),
  test: (id: number) => api.post(`/webhooks/${id}/test`),
}
```

**Task 类型变更**：

```typescript
export interface Task {
  // ... 保留现有字段 ...
  webhook_id: number | null       // 替代 webhook_config
  webhook_events: string          // 新增，如 '["success","failed"]'
  // 删除: webhook_config: string
}
```

## 错误处理

| 场景 | 处理方式 |
|------|----------|
| 创建 webhook 时 URL 为空 | 返回 400，提示"url is required" |
| 创建 webhook 时 name 为空 | 返回 400，提示"name is required" |
| 删除 webhook 时有关联任务 | 正常删除，关联任务的 webhook_id 置 NULL |
| 推送时 webhook 已禁用 | 跳过推送，记录日志 |
| 推送时目标地址不可达 | 记录错误日志，不影响任务执行 |
| 测试 webhook 失败 | 返回失败详情（状态码/错误信息） |

## 测试策略

### 后端单元测试

1. WebhookService CRUD 操作测试
2. WebhookHandler 请求参数校验测试
3. 签名计算正确性测试
4. 删除 webhook 时关联任务置空测试
5. 推送时机过滤逻辑测试
6. Webhook 禁用时不推送测试

### 前端测试

1. Webhook 管理页面渲染测试
2. 任务表单 webhook 下拉选择交互测试
3. API 调用 mock 测试

## 文档补充

实现完成后需补充以下文档：

1. **API 文档**：更新 `docs/API.md`，新增 webhook 管理 API 说明
2. **数据库文档**：更新 `docs/DATABASE.md`，新增 webhooks 表说明，修改 tasks 表变更
3. **架构文档**：更新 `docs/ARCHITECTURE.md`，补充 webhook 模块说明
4. **Webhook 使用指南**：新增 `docs/WEBHOOK.md`，包含：
   - Webhook 概念说明
   - 创建和配置 webhook 步骤
   - 任务关联 webhook 步骤
   - 推送 payload 格式说明
   - 签名验证方法
   - 常见平台配置示例（钉钉、企业微信、飞书、Slack）
