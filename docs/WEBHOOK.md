# Webhook 使用指南

## 概述

BDopsFlow 的 Webhook 功能允许在任务执行完成后，自动将执行结果推送到外部系统。Webhook 配置集中在系统设置中管理，任务通过下拉选择关联 Webhook。

## 核心概念

- **Webhook**：定义了推送目标地址、HTTP 方法、自定义请求头和签名密钥。按领域隔离，每个领域独立管理自己的 Webhook。
- **任务关联**：任务创建时选择一个 Webhook，并配置推送时机（成功/失败/跳过/每次）。
- **签名验证**：如果配置了签名密钥，推送时会在 Header 中携带 HMAC-SHA256 签名，接收方可验证来源真实性。

## 创建 Webhook

1. 进入 **管理后台 → Webhook管理**
2. 点击 **创建Webhook** 按钮
3. 填写以下信息：
   - **名称**（必填）：Webhook 的标识，如"钉钉通知"
   - **URL**（必填）：接收推送的地址
   - **HTTP方法**：默认 POST，可选 PUT/GET
   - **自定义Headers**：Key-Value 格式，如 `Authorization: Bearer xxx`
   - **签名密钥**（可选）：用于 HMAC-SHA256 签名验证
   - **描述**：Webhook 用途说明

## 任务关联 Webhook

1. 创建或编辑任务时，在 **Webhook推送配置** 区域
2. 从下拉列表中选择一个 Webhook
3. 选择推送时机：
   - **任务成功**：仅在任务执行成功时推送
   - **任务失败**：仅在任务执行失败时推送
   - **任务跳过**：仅在任务被跳过时推送
   - **每次执行**：无论结果如何都推送

## 推送 Payload 格式

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

| 字段 | 说明 |
|------|------|
| event | 事件类型：success / failed / skipped |
| timestamp | Unix 时间戳 |
| delivery_id | 推送唯一 ID（UUID） |
| task.id | 任务 ID |
| task.name | 任务名称 |
| task.type | 任务类型 |
| execution.id | 执行记录 ID |
| execution.status | 执行状态 |
| execution.output | 执行输出 |
| execution.error | 错误信息 |
| execution.duration_ms | 执行耗时（毫秒） |

## 签名验证

如果 Webhook 配置了签名密钥，推送请求会包含以下 Header：

| Header | 说明 |
|--------|------|
| X-Webhook-Signature | `sha256=<hex_digest>`，HMAC-SHA256 签名 |
| X-Webhook-Event | 事件类型 |
| X-Webhook-Delivery | 推送唯一 ID |

### 验证示例（Python）

```python
import hmac
import hashlib

def verify_signature(secret, payload, signature_header):
    if not signature_header:
        return False
    expected = hmac.new(
        secret.encode('utf-8'),
        payload,
        hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(f'sha256={expected}', signature_header)
```

### 验证示例（Go）

```go
import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
)

func verifySignature(secret string, body []byte, signature string) bool {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(body)
    expected := fmt.Sprintf("sha256=%x", mac.Sum(nil))
    return hmac.Equal([]byte(expected), []byte(signature))
}
```

## 常见平台配置

### 钉钉机器人

1. 创建钉钉群机器人，获取 Webhook URL
2. 安全设置选择"加签"，记录签名密钥
3. 在 BDopsFlow 中创建 Webhook：
   - URL：钉钉机器人 Webhook 地址
   - 签名密钥：钉钉加签密钥
   - 自定义 Headers：`Content-Type: application/json`

### 企业微信机器人

1. 创建企业微信群机器人，获取 Webhook URL
2. 在 BDopsFlow 中创建 Webhook：
   - URL：企业微信机器人 Webhook 地址
   - 自定义 Headers：`Content-Type: application/json`

### 飞书机器人

1. 创建飞书群机器人，获取 Webhook URL
2. 在 BDopsFlow 中创建 Webhook：
   - URL：飞书机器人 Webhook 地址
   - 签名密钥：飞书签名校验密钥（如启用）
   - 自定义 Headers：`Content-Type: application/json`

## API 参考

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/webhooks?domain_id=1` | 获取领域下的 Webhook 列表 |
| POST | `/api/webhooks` | 创建 Webhook |
| PUT | `/api/webhooks/:id` | 更新 Webhook |
| DELETE | `/api/webhooks/:id` | 删除 Webhook |
| POST | `/api/webhooks/:id/test` | 测试 Webhook 连通性 |

## 常见问题

**Q: 删除 Webhook 后，关联的任务会怎样？**
A: 关联任务的 webhook_id 会被置空，不再推送通知。任务本身不受影响。

**Q: Webhook 推送失败会影响任务执行吗？**
A: 不会。Webhook 推送是异步的，失败只会记录日志，不影响任务执行结果。

**Q: 一个任务可以关联多个 Webhook 吗？**
A: 目前一个任务只能关联一个 Webhook。如需推送到多个地址，建议使用中间服务转发。

**Q: 推送有重试机制吗？**
A: 当前版本推送失败不自动重试，后续版本会支持配置重试策略。
