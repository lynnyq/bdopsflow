# BDopsFlow 性能测试方案

## 一、测试工具

| 工具 | 用途 | 安装命令 |
|------|------|----------|
| **hey** | HTTP 压测（推荐，Go 编写，支持 JSON body） | `go install github.com/rakyll/hey@latest` |
| **ab** | HTTP 压测（ApacheBench，macOS 自带） | 系统自带 |
| **curl** | 单请求调试 + 获取 Token | 系统自带 |

> hey 安装后路径：`~/go/bin/hey`，需加入 PATH 或使用全路径

---

## 二、前置准备

### 2.1 启动服务

```bash
# 1. 确保 rqlite 和 Redis 已启动
# 2. 启动 scheduler
cd /path/to/bdopsflow
go run ./scheduler/cmd/... --config config.yaml
```

### 2.2 获取 JWT Token

```bash
# 登录获取 token（替换用户名密码）
TOKEN=$(curl -s http://localhost:8080/api/auth/login \
  -X POST \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin).get('data',{}).get('token',''))")

echo "Token: ${TOKEN:0:20}..."

# 验证 token 有效
curl -s http://localhost:8080/api/auth/current \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool
```

---

## 三、一键压测脚本

项目已内置压测脚本 `scripts/benchmark.sh`，直接运行：

```bash
# 基础用法（仅测试无需认证的接口）
./scripts/benchmark.sh

# 完整用法（测试所有 API）
./scripts/benchmark.sh http://localhost:8080 "$TOKEN"
```

脚本会自动：
- 测试 16 个 API 端点
- 每个端点发送 500 请求、20 并发
- 输出 QPS / Avg / P50 / P95 / P99 / 成功率 / 错误数
- 生成汇总表格和性能评级（A/B/C/D/F）
- 原始数据保存到 `/tmp/bdopsflow_benchmark_*`

---

## 四、手动压测命令

如果需要单独测试某个 API 或调整参数，使用以下命令：

### 4.1 基础参数说明

```
hey -n <总请求数> -c <并发数> -m <方法> -H "Authorization: Bearer $TOKEN" <URL>
```

| 参数 | 含义 | 推荐值 |
|------|------|--------|
| `-n` | 总请求数 | 200 / 500 / 1000 |
| `-c` | 并发数 | 10 / 20 / 50 |
| `-m` | HTTP 方法 | GET / POST |
| `-H` | 请求头 | Authorization |
| `-d` | 请求 body | JSON 字符串 |

### 4.2 无需认证的接口

```bash
# 健康检查（基线测试，应最快）
hey -n 1000 -c 50 http://localhost:8080/health
```

### 4.3 高频读取接口

```bash
# 任务列表
hey -n 500 -c 20 -m GET \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/tasks?page=1&page_size=20"

# 执行器列表
hey -n 500 -c 20 -m GET \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/executors"

# 日志列表
hey -n 500 -c 20 -m GET \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/logs?page=1&page_size=20"

# 日志统计
hey -n 500 -c 20 -m GET \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/logs/stats"

# Dashboard 统计
hey -n 500 -c 20 -m GET \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/dashboard/stats"

# Dashboard 趋势
hey -n 500 -c 20 -m GET \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/dashboard/trends"

# 调度器状态
hey -n 500 -c 20 -m GET \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/dashboard/scheduler/status"

# Dashboard 健康检查
hey -n 500 -c 20 -m GET \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/dashboard/health"
```

### 4.4 管理接口

```bash
# 用户列表
hey -n 500 -c 20 -m GET \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/admin/users"

# 角色列表
hey -n 500 -c 20 -m GET \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/admin/roles"

# 领域列表
hey -n 500 -c 20 -m GET \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/admin/domains"

# 审计日志
hey -n 500 -c 20 -m GET \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/admin/audit-logs?page=1&page_size=20"
```

### 4.5 数据源接口

```bash
# 数据源列表
hey -n 500 -c 20 -m GET \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/datasources"

# 数据源类型
hey -n 500 -c 20 -m GET \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/datasources/types"
```

### 4.6 Webhook 接口

```bash
# Webhook 列表
hey -n 500 -c 20 -m GET \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/webhooks"
```

### 4.7 写入接口

```bash
# 创建任务（注意：会产生真实数据，测试后需清理）
hey -n 100 -c 10 -m POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"bench-test","type":"http","config":"{\"url\":\"http://localhost:8080/health\",\"method\":\"GET\",\"timeout\":5}","timeout_seconds":60,"retry_count":0,"retry_interval":5,"is_enabled":false}' \
  "http://localhost:8080/api/tasks"
```

### 4.8 查询历史接口

```bash
# 查询历史
hey -n 500 -c 20 -m GET \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/query/history?page=1&page_size=20"

# 已保存 SQL
hey -n 500 -c 20 -m GET \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/query/saved?page=1&page_size=20"
```

---

## 五、渐进式压测方案

从低到高逐步增加负载，观察系统拐点：

### 阶段 1：基准测试（低负载）

```bash
# 200 请求，10 并发 — 确认系统基本可用
hey -n 200 -c 10 -m GET \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/tasks?page=1&page_size=20"
```

### 阶段 2：常规负载

```bash
# 500 请求，20 并发 — 模拟日常使用
hey -n 500 -c 20 -m GET \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/tasks?page=1&page_size=20"
```

### 阶段 3：高负载

```bash
# 1000 请求，50 并发 — 模拟高峰期
hey -n 1000 -c 50 -m GET \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/tasks?page=1&page_size=20"
```

### 阶段 4：极限测试

```bash
# 2000 请求，100 并发 — 寻找系统瓶颈
hey -n 2000 -c 100 -m GET \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/tasks?page=1&page_size=20"
```

---

## 六、结果解读

### 6.1 hey 输出关键字段

```
Summary:
  Total:        2.5432 secs    ← 总耗时
  Requests/sec: 196.60         ← QPS（每秒请求数，核心指标）

Response time histogram:
  Average:       0.1017 secs   ← 平均响应时间
  Fastest:       0.0123 secs   ← 最快响应
  Slowest:       0.5432 secs   ← 最慢响应

Latency distribution:
  10% in 0.0234 secs
  50% in 0.0876 secs          ← P50（中位数）
  90% in 0.2134 secs          ← P90
  95% in 0.3210 secs          ← P95（重点关注）
  99% in 0.4876 secs          ← P99（长尾延迟）

Status code distribution:
  [200] 500 responses          ← 成功数
```

### 6.2 性能评级标准

| 评级 | QPS | P95 延迟 | 说明 |
|------|-----|----------|------|
| **A** | ≥ 2000 | < 50ms | 优秀，生产级性能 |
| **B** | ≥ 1000 | < 100ms | 良好，满足大多数场景 |
| **C** | ≥ 500 | < 200ms | 一般，需要关注优化 |
| **D** | ≥ 100 | < 500ms | 较差，有明显瓶颈 |
| **F** | < 100 | ≥ 500ms | 不可接受，必须优化 |

### 6.3 不同接口的合理预期

| 接口类型 | 预期 QPS | 预期 P95 | 说明 |
|----------|----------|----------|------|
| `/health` | 5000+ | < 5ms | 纯内存，无 DB |
| 列表查询（简单） | 500-2000 | 10-50ms | 单表查询 + 分页 |
| 列表查询（JOIN） | 200-800 | 30-100ms | 多表关联 |
| 统计聚合 | 100-500 | 50-200ms | COUNT/GROUP BY |
| 写入操作 | 200-1000 | 20-80ms | INSERT/UPDATE |

---

## 七、常见瓶颈排查

### 7.1 QPS 低 + P99 高

**可能原因**：rqlite 查询慢
```bash
# 检查 rqlite 响应时间
curl -s http://localhost:4001/status | python3 -m json.tool
```

### 7.2 QPS 低 + 错误率高

**可能原因**：连接池耗尽或 Redis 超时
```bash
# 检查 Redis 连接
redis-cli info clients
redis-cli info stats | grep rejected
```

### 7.3 P95 正常但 P99 极高

**可能原因**：GC 暂停或 goroutine 调度延迟
```bash
# 开启 pprof 分析（服务运行中）
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=10
```

### 7.4 并发增加但 QPS 不增长

**可能原因**：锁竞争或连接池限制
```bash
# 查看 goroutine 数量
curl -s http://localhost:8080/debug/pprof/goroutine?debug=1 | head -20
```

---

## 八、测试报告模板

完成测试后，请记录以下信息：

```
测试日期：____________________
测试环境：CPU ____核  内存 ____GB  磁盘 ____SSD/HDD
软件版本：scheduler ____  rqlite ____  Redis ____

| API 端点 | QPS | Avg(ms) | P50(ms) | P95(ms) | P99(ms) | 成功率 | 评级 |
|----------|-----|---------|---------|---------|---------|--------|------|
| /health  |     |         |         |         |         |        |      |
| /api/tasks |   |         |         |         |         |        |      |
| /api/executors | |       |         |         |         |        |      |
| /api/logs |    |         |         |         |         |        |      |
| /api/logs/stats | |      |         |         |         |        |      |
| /api/dashboard/stats | |  |         |         |         |        |      |
| /api/dashboard/trends | | |         |         |         |        |      |
| /api/admin/users | |     |         |         |         |        |      |
| /api/admin/roles | |     |         |         |         |        |      |
| /api/datasources | |    |         |         |         |        |      |
| /api/webhooks | |      |         |         |         |        |      |
| POST /api/tasks | |    |         |         |         |        |      |

综合评级：____
瓶颈分析：____________________
优化建议：____________________
```
