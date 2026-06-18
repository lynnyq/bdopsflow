# 个人设置 API Token 功能设计

> 日期：2026-06-18
> 状态：待审核

## 1. 概述

在个人设置页面增加 API Token 功能，允许用户生成一个长期有效的 API Key，该 Key 与用户权限一致，可用于调用系统所有 API 接口。适用于 CI/CD、脚本调用、自动化运维等场景。

**核心决策：**
- 采用长期 API Key（不过期，用户主动吊销）
- 每用户仅一个 API Key，重新生成替换旧的
- 复用 Bearer Token 认证方式，中间件自动识别
- 数据库存储 API Key 哈希值，支持吊销和审计

## 2. 数据库设计

### 2.1 新增表 `bdopsflow_api_tokens`

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_api_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    token_encrypted TEXT NOT NULL,
    token_prefix TEXT NOT NULL,
    last_used_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES bdopsflow_users(id) ON DELETE CASCADE,
    UNIQUE(user_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bdopsflow_api_tokens_user_id ON bdopsflow_api_tokens(user_id);
```

**字段说明：**
- `user_id`：关联用户，UNIQUE 约束保证每用户只有一个 Token
- `token_encrypted`：Token 明文使用项目现有 RSA 加密后的密文，用于验证和用户查看
- `token_prefix`：Token 前8位，用于展示识别（如 `bdf_a1b2...`）
- `last_used_at`：最后使用时间，便于用户了解 Token 使用情况
- `created_at`：创建时间

### 2.2 迁移脚本

文件：`deploy/migrations/v4_api_token.sql`

## 3. Token 生成规则

- 前缀：`bdf_`（标识 BDopsFlow API Token）
- 格式：`bdf_` + 32字节随机十六进制字符串 = 总长度 67 字符
- 示例：`bdf_a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8`
- 存储：数据库保存 RSA 加密后的 Token 密文（复用项目现有 RSA 密钥对，与本地登录密码加密同一套方案）

## 4. 后端设计

### 4.1 Model 层

文件：`scheduler/internal/model/api_token.go`

```go
type APIToken struct {
    ID              int64      `db:"id" json:"id"`
    UserID          int64      `db:"user_id" json:"user_id"`
    TokenEncrypted  string     `db:"token_encrypted" json:"-"`
    TokenPrefix     string     `db:"token_prefix" json:"token_prefix"`
    LastUsedAt      *time.Time `db:"last_used_at" json:"last_used_at,omitempty"`
    CreatedAt       time.Time  `db:"created_at" json:"created_at"`
}
```

### 4.2 Service 层

文件：`scheduler/internal/service/api_token.go`

**核心方法：**
- `GenerateToken(ctx, userID)` → 生成新 Token，覆盖旧 Token，返回明文
- `GetTokenInfo(ctx, userID)` → 获取当前 Token 信息（不含明文）
- `RevealToken(ctx, userID)` → 解密返回 Token 明文（支持多次查看）
- `RevokeToken(ctx, userID)` → 吊销 Token
- `ValidateToken(ctx, tokenString)` → 验证 Token，返回用户信息（供中间件调用）

**Token 生成流程：**
1. 生成随机 Token 明文
2. 使用项目现有 RSA 公钥加密 Token 明文（`rsaUtil.EncryptLarge`）
3. 删除该用户旧 Token（DELETE）
4. 插入新 Token 记录（INSERT，含 token_encrypted 和 token_prefix）
5. 返回明文 Token

**Token 验证流程：**
1. 从数据库查询该用户的 Token 记录（按 user_id 唯一索引）
2. 使用 RSA 私钥解密 `token_encrypted` 得到明文
3. 比对解密后的明文与请求中的 Token 是否一致
4. 匹配则返回对应用户信息
5. 异步更新 `last_used_at`

**Token 查看流程：**
1. 从数据库查询该用户的 Token 记录
2. 使用 RSA 私钥解密 `token_encrypted` 得到明文
3. 返回明文 Token

### 4.3 Handler 层

文件：`scheduler/internal/handler/api_token.go`

**API 端点（需认证）：**

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/auth/api-token` | 生成/重新生成 API Token |
| GET | `/api/auth/api-token` | 获取当前 Token 信息（遮掩显示） |
| GET | `/api/auth/api-token/reveal` | 查看 Token 明文（支持多次） |
| DELETE | `/api/auth/api-token` | 吊销 API Token |

**POST /api/auth/api-token 响应：**
```json
{
  "code": 0,
  "data": {
    "token": "bdf_a1b2c3d4...",
    "token_prefix": "bdf_a1b2...",
    "created_at": "2026-06-18T10:00:00Z"
  }
}
```

**GET /api/auth/api-token 响应：**
```json
{
  "code": 0,
  "data": {
    "has_token": true,
    "token_prefix": "bdf_a1b2...",
    "last_used_at": "2026-06-18T09:30:00Z",
    "created_at": "2026-06-18T10:00:00Z"
  }
}
```

**GET /api/auth/api-token/reveal 响应：**
```json
{
  "code": 0,
  "data": {
    "token": "bdf_a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8"
  }
}
```

### 4.4 中间件改造

文件：`scheduler/internal/middleware/auth.go`

修改 `JWTAuthMiddleware`，在 JWT 解析失败后尝试 API Token 验证：

```
1. 从 Authorization: Bearer xxx 提取 token
2. 尝试 JWT 解析
3. JWT 成功 → 走现有流程
4. JWT 失败 → 检查是否以 "bdf_" 开头
5. 是 → 调用 apiTokenService.ValidateToken() 验证
6. 验证成功 → 设置 user_id/username/real_name/current_domain_id 到 context
7. 继续后续中间件（InjectUserRole、RequirePermission 等）
```

**关键点：** API Token 认证成功后，需要从数据库查询用户的 username、real_name、current_domain_id，确保后续权限检查正常工作。

### 4.5 路由注册

文件：`scheduler/cmd/routes.go`

在 `protected` 路由组中添加：
```go
protected.POST("/auth/api-token", apiTokenHandler.Generate)
protected.GET("/auth/api-token", apiTokenHandler.GetInfo)
protected.GET("/auth/api-token/reveal", apiTokenHandler.Reveal)
protected.DELETE("/auth/api-token", apiTokenHandler.Revoke)
```

## 5. 审计日志

### 5.1 审计事件

在审计中间件 `scheduler/internal/middleware/audit.go` 中注册 API Token 相关路由规则：

| 路由 | 资源 | 动作 |
|------|------|------|
| `/api/auth/api-token` (POST) | `api_token` | `generate` |
| `/api/auth/api-token/reveal` (GET) | `api_token` | `reveal` |
| `/api/auth/api-token` (DELETE) | `api_token` | `revoke` |

### 5.2 审计记录内容

- **生成 Token**：resource=`api_token`, action=`generate`, resource_name=用户名, detail=`token_prefix=xxx`
- **查看 Token**：resource=`api_token`, action=`reveal`, resource_name=用户名, detail=`token_prefix=xxx`
- **吊销 Token**：resource=`api_token`, action=`revoke`, resource_name=用户名, detail=`token_prefix=xxx`
- **API Token 调用接口**：通过现有审计中间件自动记录，user_id/username 等信息从 Token 验证结果中获取，与 JWT 认证行为一致

### 5.3 审计日志查询

API Token 的操作记录可在现有审计日志页面通过 `resource=api_token` 筛选查看。

## 6. 前端设计

### 6.1 Profile 页面改造

文件：`web/src/views/Profile.vue`

在个人信息卡片和修改密码卡片之间，新增 **API Token** 卡片：

**UI 布局：**
```
┌─────────────────────────────────────────┐
│ API Token                          刷新 │
├─────────────────────────────────────────┤
│                                         │
│  状态：未创建 / 已创建                    │
│  Token：••••••••••••••••••••••••••••   │
│         [查看] [复制]                    │
│  创建时间：2026-06-18 10:00:00          │
│  最后使用：2026-06-18 09:30:00          │
│                                         │
│  [生成 Token]  [吊销 Token]             │
│                                         │
│  ⚠️ 提示：                               │
│  - Token 权限与当前用户一致               │
│  - 重新生成会使旧 Token 立即失效          │
│  - 请妥善保管 Token，避免泄露             │
│                                         │
└─────────────────────────────────────────┘
```

**交互流程：**
1. 首次进入：显示"未创建"状态，只有"生成 Token"按钮
2. 点击"生成 Token"：弹出确认对话框，确认后生成，显示明文 Token（可复制）
3. 已有 Token：默认遮掩显示（`••••••••`），点击"查看"按钮调用 reveal 接口获取明文并展示，点击"复制"按钮复制到剪贴板
4. 重新生成：弹出确认对话框（警告旧 Token 将失效），确认后生成新 Token
5. 吊销：弹出确认对话框，确认后删除 Token

### 6.2 API 层

文件：`web/src/api/index.ts`

在 `authAPI` 中新增：
```typescript
apiToken: {
  generate: () => api.post<{ token: string; token_prefix: string; created_at: string }>('/auth/api-token'),
  getInfo: () => api.get<{ has_token: boolean; token_prefix: string; last_used_at: string; created_at: string }>('/auth/api-token'),
  reveal: () => api.get<{ token: string }>('/auth/api-token/reveal'),
  revoke: () => api.delete('/auth/api-token'),
}
```

## 7. 安全考虑

1. **Token RSA 加密存储**：数据库存储 RSA 加密后的 Token 密文，复用项目现有 RSA 密钥对（与本地登录密码加密同一套方案），即使数据库泄露也无法直接获取 Token 明文
2. **前缀标识**：`bdf_` 前缀使中间件可快速区分 API Key 和 JWT，避免无效解析
3. **单用户单 Token**：降低 Token 泄露风险，便于管理
4. **用户禁用联动**：API Token 验证时检查用户 `is_active` 状态，禁用用户 Token 自动失效
5. **审计追踪**：所有 Token 操作（生成、查看、吊销）和通过 Token 的 API 调用均记录审计日志
6. **查看审计**：每次查看 Token 明文都记录审计日志，便于追踪异常查看行为
7. **前端遮掩**：Token 默认遮掩显示，需主动点击查看，降低屏幕截图泄露风险

## 8. 变更文件清单

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `deploy/schema.sql` | 修改 | 新增 bdopsflow_api_tokens 表 |
| `deploy/migrations/v4_api_token.sql` | 新增 | 迁移脚本 |
| `scheduler/internal/model/api_token.go` | 新增 | API Token 模型 |
| `scheduler/internal/service/api_token.go` | 新增 | API Token 服务 |
| `scheduler/internal/handler/api_token.go` | 新增 | API Token 处理器 |
| `scheduler/internal/middleware/auth.go` | 修改 | JWT 中间件增加 API Token 识别 |
| `scheduler/internal/middleware/audit.go` | 修改 | 新增 API Token 审计路由规则 |
| `scheduler/cmd/routes.go` | 修改 | 注册 API Token 路由 |
| `scheduler/cmd/app.go` | 修改 | 初始化 API Token 服务和处理器 |
| `web/src/api/index.ts` | 修改 | 新增 API Token 接口 |
| `web/src/views/Profile.vue` | 修改 | 新增 API Token 卡片 |
