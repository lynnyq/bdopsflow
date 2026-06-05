# BDopsFlow Prometheus 监控指标

## 概述

BDopsFlow 调度中心通过 `/metrics` 端点暴露 Prometheus 指标，该端点位于**非认证区**，无需登录即可访问。

- **端点地址**: `http://<scheduler-host>:<http-port>/metrics`
- **指标命名空间**: `bdopsflow_scheduler`
- **自定义 Registry**: 不包含默认 Go runtime 指标（如 `go_goroutines`、`go_memstats_*`），仅暴露业务指标

## Prometheus 抓取配置

```yaml
scrape_configs:
  - job_name: 'bdopsflow'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: /metrics
    scrape_interval: 15s
```

多节点部署时，建议为每个节点添加 `node_id` 标签：

```yaml
scrape_configs:
  - job_name: 'bdopsflow'
    static_configs:
      - targets: ['scheduler-1:8080']
        labels:
          node_id: 'scheduler-1'
      - targets: ['scheduler-2:8080']
        labels:
          node_id: 'scheduler-2'
    metrics_path: /metrics
    scrape_interval: 15s
```

## 指标列表

### 调度器集群指标

| 指标名 | 类型 | 标签 | 说明 |
|--------|------|------|------|
| `bdopsflow_scheduler_is_leader` | Gauge | `node_id` | 当前节点是否为主调度节点 (1=主节点, 0=从节点) |

**用途**: 多节点部署时识别哪个节点是主调度节点。主节点负责 cron 调度和任务触发，从节点仅转发请求。

**告警示例**:
- 所有节点 `is_leader` 均为 0 → 集群无主，调度停止
- 多个节点 `is_leader` 均为 1 → 脑裂风险

**PromQL 示例**:
```promql
# 查看当前主节点
bdopsflow_scheduler_is_leader == 1
```

---

### 任务指标

| 指标名 | 类型 | 标签 | 说明 |
|--------|------|------|------|
| `bdopsflow_scheduler_tasks_created_total` | Counter | - | 已创建的任务总数 |
| `bdopsflow_scheduler_tasks_triggered_total` | Counter | `source` | 被触发的任务总数，`source` 取值: `manual`(手动), `cron`(定时) |
| `bdopsflow_scheduler_tasks_completed_total` | Counter | - | 已成功完成的任务总数 |
| `bdopsflow_scheduler_tasks_failed_total` | Counter | `reason` | 已失败的任务总数，`reason` 取值: `failed`(执行失败), `timeout`(超时) |
| `bdopsflow_scheduler_tasks_running` | Gauge | - | 当前运行中的任务数 |
| `bdopsflow_scheduler_task_duration_seconds` | Histogram | - | 任务执行耗时分布（秒） |
| `bdopsflow_scheduler_task_retries_total` | Counter | - | 任务重试次数 |

**Buckets**: `0.1, 0.5, 1, 5, 10, 30, 60, 120, 300, 600, 1800, 3600`

**PromQL 示例**:
```promql
# 任务成功率
sum(rate(bdopsflow_scheduler_tasks_completed_total[5m]))
/
sum(rate(bdopsflow_scheduler_tasks_completed_total[5m]) + rate(bdopsflow_scheduler_tasks_failed_total[5m]))

# 任务 P95 执行耗时
histogram_quantile(0.95, sum(rate(bdopsflow_scheduler_task_duration_seconds_bucket[5m])) by (le))

# 每分钟任务触发速率（按来源）
sum(rate(bdopsflow_scheduler_tasks_triggered_total[1m])) by (source)

# 每分钟任务失败速率（按原因）
sum(rate(bdopsflow_scheduler_tasks_failed_total[5m])) by (reason)
```

**告警建议**:
- 任务失败率 > 10% → 检查执行器状态和任务配置
- 任务 P99 耗时 > 300s → 检查任务逻辑或数据量
- 重试次数异常增长 → 检查执行器稳定性

---

### 执行器指标

| 指标名 | 类型 | 标签 | 说明 |
|--------|------|------|------|
| `bdopsflow_scheduler_executors_online` | Gauge | - | 在线执行器数量 |
| `bdopsflow_scheduler_executors_offline` | Gauge | - | 离线执行器数量 |
| `bdopsflow_scheduler_executor_registrations_total` | Counter | - | 执行器注册次数 |
| `bdopsflow_scheduler_executor_heartbeats_total` | Counter | - | 执行器心跳次数 |

**PromQL 示例**:
```promql
# 执行器在线率
bdopsflow_scheduler_executors_online
/
(bdopsflow_scheduler_executors_online + bdopsflow_scheduler_executors_offline)

# 执行器注册频率（检测频繁重启）
rate(bdopsflow_scheduler_executor_registrations_total[5m])
```

**告警建议**:
- 在线执行器数为 0 → 所有执行器离线，任务无法执行
- 注册频率异常高 → 执行器频繁重启，检查执行器稳定性

---

### Webhook 指标

| 指标名 | 类型 | 标签 | 说明 |
|--------|------|------|------|
| `bdopsflow_scheduler_webhook_sent_total` | Counter | `status` | Webhook 发送总数，`status` 取值: `success`, `error` |
| `bdopsflow_scheduler_webhook_duration_seconds` | Histogram | - | Webhook 发送耗时（秒） |

**Buckets**: `0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30`

**PromQL 示例**:
```promql
# Webhook 发送成功率
sum(rate(bdopsflow_scheduler_webhook_sent_total{status="success"}[5m]))
/
sum(rate(bdopsflow_scheduler_webhook_sent_total[5m]))

# Webhook P95 延迟
histogram_quantile(0.95, sum(rate(bdopsflow_scheduler_webhook_duration_seconds_bucket[5m])) by (le))
```

---

### 数据源查询指标

| 指标名 | 类型 | 标签 | 说明 |
|--------|------|------|------|
| `bdopsflow_scheduler_datasource_queries_total` | Counter | `type`, `status` | 数据源查询总数。`type`: 数据源类型(mysql/postgres/clickhouse等)，`status`: `success`/`failed`/`cancelled` |
| `bdopsflow_scheduler_datasource_query_duration_seconds` | Histogram | - | 数据源查询耗时（秒） |

**Buckets**: `0.01, 0.05, 0.1, 0.5, 1, 5, 10, 30, 60`

**PromQL 示例**:
```promql
# 按数据源类型的查询速率
sum(rate(bdopsflow_scheduler_datasource_queries_total[5m])) by (type)

# 按数据源类型的查询失败率
sum(rate(bdopsflow_scheduler_datasource_queries_total{status="failed"}[5m])) by (type)
/
sum(rate(bdopsflow_scheduler_datasource_queries_total[5m])) by (type)

# 数据源查询 P95 耗时
histogram_quantile(0.95, sum(rate(bdopsflow_scheduler_datasource_query_duration_seconds_bucket[5m])) by (le))
```

**告警建议**:
- 某类数据源查询失败率 > 5% → 检查数据源连接和配置
- 查询 P99 耗时 > 30s → 检查 SQL 性能或数据源负载

---

### Cron 调度指标

| 指标名 | 类型 | 标签 | 说明 |
|--------|------|------|------|
| `bdopsflow_scheduler_cron_triggers_total` | Counter | `status` | Cron 触发总数，`status` 取值: `success`(成功), `skipped`(跳过), `failed`(失败) |

**PromQL 示例**:
```promql
# Cron 触发成功率
sum(rate(bdopsflow_scheduler_cron_triggers_total{status="success"}[5m]))
/
sum(rate(bdopsflow_scheduler_cron_triggers_total[5m]))

# Cron 跳过率（任务仍在运行导致跳过）
sum(rate(bdopsflow_scheduler_cron_triggers_total{status="skipped"}[5m]))
```

---

### 认证指标

| 指标名 | 类型 | 标签 | 说明 |
|--------|------|------|------|
| `bdopsflow_scheduler_auth_attempts_total` | Counter | `method`, `status` | 认证尝试总数。`method`: `local`(本地登录), `sso`(SSO登录)；`status`: `success`/`failed` |

**PromQL 示例**:
```promql
# 登录失败率
sum(rate(bdopsflow_scheduler_auth_attempts_total{status="failed"}[5m]))
/
sum(rate(bdopsflow_scheduler_auth_attempts_total[5m]))

# 按方式的登录频率
sum(rate(bdopsflow_scheduler_auth_attempts_total[5m])) by (method)
```

**告警建议**:
- 登录失败率突增 → 可能存在暴力破解攻击
- SSO 登录全部失败 → 检查 SSO 服务可用性

---

## Grafana 仪表盘建议

### 核心面板

1. **集群状态**: `bdopsflow_scheduler_is_leader` → Stat Panel，显示当前主节点
2. **任务概览**: 任务创建/完成/失败速率 → Time Series
3. **任务耗时**: `task_duration_seconds` P50/P95/P99 → Time Series
4. **执行器状态**: 在线/离线数量 → Gauge
5. **数据源查询**: 按类型的查询速率和失败率 → Stacked Area
6. **Webhook 健康**: 发送成功率和延迟 → Time Series
7. **Cron 健康**: 触发成功/跳过/失败 → Pie Chart
8. **认证安全**: 登录成功/失败趋势 → Bar Chart

### 关键告警规则

```yaml
groups:
  - name: bdopsflow
    rules:
      - alert: BDopsFlowNoLeader
        expr: sum(bdopsflow_scheduler_is_leader) == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "BDopsFlow 集群无主节点"
          description: "所有调度器节点均非主节点，cron 调度和任务触发已停止"

      - alert: BDopsFlowNoExecutors
        expr: bdopsflow_scheduler_executors_online == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "没有在线执行器"
          description: "所有执行器均离线，任务无法执行"

      - alert: BDopsFlowHighTaskFailureRate
        expr: |
          sum(rate(bdopsflow_scheduler_tasks_failed_total[5m]))
          /
          (sum(rate(bdopsflow_scheduler_tasks_completed_total[5m])) + sum(rate(bdopsflow_scheduler_tasks_failed_total[5m])))
          > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "任务失败率超过 10%"

      - alert: BDopsFlowHighLoginFailureRate
        expr: |
          sum(rate(bdopsflow_scheduler_auth_attempts_total{status="failed"}[5m]))
          /
          sum(rate(bdopsflow_scheduler_auth_attempts_total[5m]))
          > 0.3
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "登录失败率超过 30%，可能存在暴力破解"
```
