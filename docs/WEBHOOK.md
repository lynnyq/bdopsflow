# Webhook 接入指南

## 目录
1. [什么是Webhook](#什么是webhook)
2. [接入步骤](#接入步骤)
3. [Webhook配置](#webhook配置)
4. [事件类型](#事件类型)
5. [Payload字段说明](#payload字段说明)
6. [签名验证](#签名验证)
7. [重试机制](#重试机制)
8. [示例代码](#示例代码)

---

## 什么是Webhook

Webhook是一种HTTP回调机制，当任务执行完成后，系统会向您配置的URL发送HTTP POST请求，通知任务的执行结果。

通过Webhook，您可以：
- 实时接收任务执行结果
- 自动处理成功或失败的任务
- 构建自动化工作流
- 集成到您的监控系统

---

## 接入步骤

### 1. 准备接收Webhook的服务

首先，您需要有一个可以接收HTTP POST请求的服务端点。例如：

```
https://your-domain.com/webhook/bdopsflow
```

这个端点需要：
- 能够接收POST请求
- 能够处理JSON格式的请求体
- 返回2xx状态码表示接收成功

### 2. 创建任务时配置Webhook

在创建或更新任务时，在`webhook_config`字段中配置Webhook信息：

```json
{
  "url": "https://your-domain.com/webhook/bdopsflow",
  "method": "POST",
  "headers": {
    "X-Webhook-Secret": "your-secret-key",
    "Content-Type": "application/json"
  },
  "events": ["success", "failed"]
}
```

### 3. 测试Webhook

您可以通过手动触发任务来测试Webhook是否正常工作：

```bash
# 触发任务执行
POST /api/v1/tasks/{task_id}/trigger
```

### 4. 处理Webhook请求

在您的服务中处理接收到的Webhook请求，验证请求并执行相应的业务逻辑。

---

## Webhook配置

### 配置字段说明

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| url | string | 是 | - | Webhook接收地址 |
| method | string | 否 | POST | HTTP请求方法 (GET, POST, PUT等) |
| headers | object | 否 | {} | 自定义请求头 |
| events | array | 否 | ["*"] | 需要接收的事件类型列表 |

### events配置示例

| 配置 | 说明 |
|------|------|
| `["success"]` | 只接收任务成功事件 |
| `["failed"]` | 只接收任务失败事件 |
| `["success", "failed"]` | 接收成功和失败事件 |
| `["*"]` | 接收所有事件 |
| 不配置 | 默认接收所有事件 |

---

## 事件类型

### 支持的事件

| 事件类型 | 说明 |
|----------|------|
| success | 任务执行成功 |
| failed | 任务执行失败 |

---

## Payload字段说明

### 成功事件Payload

```json
{
  "event": "success",
  "timestamp": 1704067200,
  "task_id": 1,
  "execution_id": "exec-1234567890",
  "status": "success",
  "output": "任务输出内容",
  "error": "",
  "duration_ms": 5234,
  "metadata": {
    "task_name": "示例任务",
    "task_type": "http"
  }
}
```

### 失败事件Payload

```json
{
  "event": "failed",
  "timestamp": 1704067200,
  "task_id": 1,
  "execution_id": "exec-1234567890",
  "status": "failed",
  "output": "",
  "error": "请求超时",
  "duration_ms": 30000,
  "metadata": {
    "task_name": "示例任务",
    "task_type": "http"
  }
}
```

### 字段详细说明

| 字段 | 类型 | 说明 |
|------|------|------|
| event | string | 事件类型 (success/failed) |
| timestamp | integer | 事件发生时间戳 (Unix时间戳，秒) |
| task_id | integer | 任务ID |
| execution_id | string | 执行ID，唯一标识一次执行 |
| status | string | 执行状态 (success/failed) |
| output | string | 任务输出内容，JSON或Text格式的响应会输出到这里 |
| error | string | 错误信息，仅失败时有值 |
| duration_ms | integer | 执行耗时，单位毫秒 |
| metadata | object | 元数据，包含任务名称、类型等额外信息 |

---

## 签名验证

为了确保Webhook请求的安全性，建议在接收时验证请求签名。

### 使用自定义Header验证

在配置Webhook时添加自定义Header：

```json
{
  "url": "https://your-domain.com/webhook/bdopsflow",
  "headers": {
    "X-Webhook-Secret": "your-secret-key-123456"
  },
  "events": ["success", "failed"]
}
```

在接收端验证Header：

```javascript
// Node.js示例
app.post('/webhook/bdopsflow', (req, res) => {
  const secret = req.headers['x-webhook-secret'];
  
  if (secret !== 'your-secret-key-123456') {
    return res.status(401).send('Invalid secret');
  }
  
  // 处理Webhook
  console.log('Received webhook:', req.body);
  res.status(200).send('OK');
});
```

---

## 重试机制

如果Webhook请求失败（非2xx状态码或网络错误），系统会自动重试：

- 最大重试次数：3次
- 重试间隔：指数退避策略
  - 第1次重试：1秒后
  - 第2次重试：4秒后
  - 第3次重试：9秒后

### 重试日志

系统会记录每次重试的日志，您可以通过日志查看Webhook发送情况。

---

## 示例代码

### Node.js (Express)

```javascript
const express = require('express');
const bodyParser = require('body-parser');

const app = express();
app.use(bodyParser.json());

app.post('/webhook/bdopsflow', (req, res) => {
  const payload = req.body;
  
  console.log('收到Webhook:', {
    event: payload.event,
    task_id: payload.task_id,
    status: payload.status,
    duration: payload.duration_ms
  });
  
  // 根据事件类型处理
  if (payload.event === 'success') {
    console.log('任务执行成功:', payload.output);
    // 处理成功逻辑
  } else if (payload.event === 'failed') {
    console.log('任务执行失败:', payload.error);
    // 处理失败逻辑，如发送告警
  }
  
  // 返回200表示接收成功
  res.status(200).json({ status: 'ok' });
});

const PORT = 3000;
app.listen(PORT, () => {
  console.log(`Webhook服务运行在端口 ${PORT}`);
});
```

### Python (Flask)

```python
from flask import Flask, request, jsonify

app = Flask(__name__)

@app.route('/webhook/bdopsflow', methods=['POST'])
def webhook():
    payload = request.json
    
    print(f'收到Webhook: event={payload["event"]}, task_id={payload["task_id"]}')
    
    if payload['event'] == 'success':
        print(f'任务执行成功: {payload["output"]}')
        # 处理成功逻辑
    elif payload['event'] == 'failed':
        print(f'任务执行失败: {payload["error"]}')
        # 处理失败逻辑
    
    return jsonify({'status': 'ok'}), 200

if __name__ == '__main__':
    app.run(port=3000)
```

### Go

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type WebhookPayload struct {
	Event       string                 `json:"event"`
	Timestamp   int64                  `json:"timestamp"`
	TaskID      int64                  `json:"task_id"`
	ExecutionID string                 `json:"execution_id"`
	Status      string                 `json:"status"`
	Output      string                 `json:"output"`
	Error       string                 `json:"error"`
	DurationMs  int64                  `json:"duration_ms"`
	Metadata    map[string]interface{} `json:"metadata"`
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var payload WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	log.Printf("收到Webhook: event=%s, task_id=%d, status=%s", 
		payload.Event, payload.TaskID, payload.Status)
	
	switch payload.Event {
	case "success":
		log.Printf("任务执行成功: %s", payload.Output)
		// 处理成功逻辑
	case "failed":
		log.Printf("任务执行失败: %s", payload.Error)
		// 处理失败逻辑
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func main() {
	http.HandleFunc("/webhook/bdopsflow", webhookHandler)
	
	fmt.Println("Webhook服务运行在端口 3000")
	log.Fatal(http.ListenAndServe(":3000", nil))
}
```

### cURL 测试

您可以使用cURL模拟Webhook请求进行测试：

```bash
# 测试成功事件
curl -X POST https://your-domain.com/webhook/bdopsflow \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Secret: your-secret-key" \
  -d '{
    "event": "success",
    "timestamp": 1704067200,
    "task_id": 1,
    "execution_id": "exec-1234567890",
    "status": "success",
    "output": "{\"result\": \"ok\"}",
    "error": "",
    "duration_ms": 5234,
    "metadata": {
      "task_name": "示例任务",
      "task_type": "http"
    }
  }'

# 测试失败事件
curl -X POST https://your-domain.com/webhook/bdopsflow \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Secret: your-secret-key" \
  -d '{
    "event": "failed",
    "timestamp": 1704067200,
    "task_id": 1,
    "execution_id": "exec-1234567890",
    "status": "failed",
    "output": "",
    "error": "请求超时",
    "duration_ms": 30000,
    "metadata": {
      "task_name": "示例任务",
      "task_type": "http"
    }
  }'
```

---

## 最佳实践

1. **快速响应**：尽快返回2xx状态码，避免触发重试机制
2. **幂等性**：确保相同的Webhook可以被安全地多次处理
3. **异步处理**：接收Webhook后异步处理业务逻辑
4. **日志记录**：记录所有收到的Webhook，便于排查问题
5. **监控告警**：监控Webhook接收成功率，及时发现问题
6. **安全验证**：使用签名或Token验证请求来源

---

## 常见问题

### Q: Webhook没有收到怎么办？

A: 请检查：
1. Webhook URL是否可以从公网访问
2. 服务器防火墙是否开放了相应端口
3. 查看任务执行日志，确认Webhook是否发送成功
4. 使用cURL模拟请求测试您的服务

### Q: 如何只接收失败事件？

A: 在配置`events`时只包含`"failed"`：
```json
{
  "url": "https://your-domain.com/webhook",
  "events": ["failed"]
}
```

### Q: Webhook超时时间是多少？

A: Webhook请求的超时时间是30秒，请确保您的服务能在30秒内响应。

### Q: 可以配置多个Webhook吗？

A: 目前每个任务只能配置一个Webhook。如果需要多个接收地址，可以在您的服务中进行转发。
