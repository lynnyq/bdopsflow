# 任务日志清理功能设计文档

> 日期：2026-11-09
> 状态：已确认
> 方案：SchedulerService 内直接加清理方法 + 独立 6 小时 ticker

## 1. 概述

新增任务日志清理功能，支持执行记录和过程日志两类数据的独立清理。清理方式包括系统配置驱动的定时自动清理和前端页面按条件筛选的手动清理。

**重要澄清**：审计日志自动清理**已存在**（`scheduler/cmd/app.go` 第 358-388 行），独立 goroutine，启动立即执行一次，之后每 24 小时执行一次。本设计文档中"审计日志"相关项仅为完整覆盖，不做改动。

**核心需求（brainstorming 已确认）：**

| # | 决策项 | 选择 |
|---|--------|------|
| 1 | 清理粒度 | 执行记录 + 过程日志，**独立清理、独立配置** |
| 2 | 定时策略 | **独立 ticker，每 6 小时**检查一次 |
| 3 | 手动清理筛选 | **时间范围 + 任务名 + 执行器** |
| 4 | 级联关系 | 清执行记录 → 级联删过程日志；清过程日志 → 不碰执行记录 |
| 5 | 自动清理开关 | 保留天数设为 **0** 时禁用自动清理 |

## 2. 背景：现有数据模型与清理现状

### 2.1 两类目标表

**`bdopsflow_task_executions`（执行记录）**
- 每次任务执行的摘要：execution_id、task_id、executor_id、status、start_time、end_time、duration、result_message
- 前端"任务日志"页面展示的就是这张表的分页列表

**`bdopsflow_task_logs`（过程日志）**
- 每次执行中 executor 上报的单条日志流：execution_id、task_id、log_level、message、log_time
- 通过 execution_id 关联到执行记录（**DB 层无外键约束**，application 层维护关联）
- 已有索引：`idx_bdopsflow_task_logs_execution_id`、`idx_bdopsflow_task_logs_log_time`

### 2.2 现有清理能力

| 目标 | 自动清理 | 手动清理 |
|------|----------|----------|
| 卡住任务、离线执行器、过期锁 | ✅ 60 秒 ticker | — |
| 审计日志 `bdopsflow_audit_logs` | ✅ 已有独立 goroutine，**24 小时**（`scheduler/cmd/app.go` L358-388） | ✅ 按钮触发 `POST /admin/audit-logs/clean` |
| 历史记录 `bdopsflow_datasource_history` | — | —（`datasource.history_retention_days` 配置存在但未实现自动清理） |
| 执行记录 `bdopsflow_task_executions` | ❌ 缺失 | ❌ 无按条件清理（只有按 ID 单删/批量删） |
| 过程日志 `bdopsflow_task_logs` | ❌ 缺失 | ❌ 无 |

**审计日志清理逻辑已实现可参考**：`AuditLogService.CleanExpired()` 采用子查询 + 分批删除（每批 1000 条），context 超时保护，避免 rqlite 大表锁库。

**审计日志自动清理 goroutine 实现**：`scheduler/cmd/app.go` 第 358-388 行，启动立即执行一次，之后每 24 小时，30 秒超时 context，recover 保护。

## 3. 新增系统配置项

### 3.1 配置清单

| 配置 Key | 默认值 | 含义 | 最小值 | 最大值 | 禁用方式 |
|----------|--------|------|--------|--------|----------|
| `task_execution.retention_days` | 30 | 执行记录保留天数 | 0（禁用自动清理） | 3650 | 设为 0 |
| `task_log.retention_days` | 30 | 过程日志保留天数 | 0（禁用自动清理） | 3650 | 设为 0 |
| `audit_log.retention_days` | 90 | 审计日志保留天数（**已有，自动清理已在 app.go 独立实现**） | 0 | 3650 | 设为 0 |

### 3.2 改动点

- **`defaultConfigValues`**：新增 2 个 key
- **`configMetaList`**：新增 2 个 `ConfigMeta`，group 归为"其他"，与 `audit_log.retention_days` 同行
- **数据库迁移**：新增配置项初始化到 `bdopsflow_system_config`

### 3.3 配置组分组

三个清理配置项统一归入 **"其他"** 组（与现有 `audit_log.retention_days`、`datasource.history_retention_days` 同组）。

## 4. 后端 Service 设计

### 4.1 执行记录清理

**归属**：`SchedulerService`（执行记录 CRUD 已在此 Service 中）

```go
// CleanExpiredExecutions 清理过期的执行记录。
// 级联删除关联的过程日志（同一 DB 事务内）。
//
// 清理条件：
//   - filters.RetentionDays > 0 且 execution.end_time < now - RetentionDays
//   - 如指定 filters.TaskName、filters.ExecutorName，则叠加 AND 条件
//   - 如指定 filters.TimeFrom/TimeTo，则叠加 end_time 范围
//
// 实现策略：分批删除（每批 1000 条），先收集 execution_id → 删日志 → 删执行记录。
// 每批都用 context + 超时保护，避免长时间独占锁。
func (s *SchedulerService) CleanExpiredExecutions(ctx context.Context, filters ExecutionCleanupFilter) (int64, error)
```

**级联删除逻辑（执行记录清理时）：**
1. 用子查询收集一批过期执行记录的 `execution_id` 集合
2. 同一事务中：`DELETE FROM bdopsflow_task_logs WHERE execution_id IN (...)`
3. 同一事务中：`DELETE FROM bdopsflow_task_executions WHERE id IN (...)`
4. 重复直到该批删除量 < batchSize（1000）

**为什么级联删除过程日志**：执行记录是过程日志的父实体，父记录不存在后子日志变为孤儿数据（看不到是哪次执行的），清理时应一并清除保证数据一致性。

### 4.2 过程日志清理

**归属**：`SchedulerService`

```go
// CleanExpiredTaskLogs 清理过期的过程日志。
// 不触碰执行记录（执行记录清理与过程日志清理是独立操作）。
//
// 清理条件：
//   - filters.RetentionDays > 0 且 log_time < now - RetentionDays
//   - 如指定 filters.TaskName、filters.ExecutorName，则 JOIN 对应表筛选
//   - 如指定 filters.TimeFrom/TimeTo，则叠加 log_time 范围
//
// 实现策略：分批删除（每批 1000 条），子查询 + LIMIT 兼容 rqlite。
func (s *SchedulerService) CleanExpiredTaskLogs(ctx context.Context, filters TaskLogCleanupFilter) (int64, error)
```

### 4.3 Filter 结构体

```go
// ExecutionCleanupFilter 执行记录清理筛选条件
type ExecutionCleanupFilter struct {
    RetentionDays int    // 保留天数（必需，>0 才执行）
    TimeFrom      string // 可选，end_time 下限（YYYY-MM-DD 格式）
    TimeTo        string // 可选，end_time 上限（YYYY-MM-DD 格式）
    TaskName      string // 可选，任务名模糊匹配
    ExecutorName  string // 可选，执行器名模糊匹配
}

// TaskLogCleanupFilter 过程日志清理筛选条件
type TaskLogCleanupFilter struct {
    RetentionDays int    // 保留天数（必需，>0 才执行）
    TimeFrom      string // 可选，log_time 下限
    TimeTo        string // 可选，log_time 上限
    TaskName      string // 可选，任务名模糊匹配（JOIN bdopsflow_tasks）
    ExecutorName  string // 可选，执行器名模糊匹配（JOIN bdopsflow_executors）
}
```

### 4.4 配置读取方法

```go
// GetExecutionRetentionDays 读取执行记录保留天数。0 表示禁用自动清理。
func (s *SchedulerService) GetExecutionRetentionDays() int

// GetTaskLogRetentionDays 读取过程日志保留天数。0 表示禁用自动清理。
func (s *SchedulerService) GetTaskLogRetentionDays() int
```

从 `system_config.GetGlobalService().GetInt(key)` 读取，异常时回退到默认值 30。

### 4.5 错误处理规范

所有清理方法遵循：
- context 超时或取消时，**返回已删除数量 + ctx.Err()**，不静默丢失进度
- DB 写入错误 → 记录 slog.Error（带 deleted_so_far 计数），返回 error
- 0 条被删 → 正常返回 `(0, nil)`，上层据此判断"没有过期数据"

## 5. 后端 Handler & API 设计

### 5.1 API 列表

| Method | Path | Handler | 权限 | 用途 |
|--------|------|---------|------|------|
| POST | `/api/admin/logs/clean-executions` | `LogHandler.CleanExecutions` | system_admin | 手动清理执行记录 |
| POST | `/api/admin/logs/clean-task-logs` | `LogHandler.CleanTaskLogs` | system_admin | 手动清理过程日志 |

### 5.2 请求体

```json
// POST /api/admin/logs/clean-executions
{
  "retention_days": 30,           // 可选，不传则用系统配置；如系统配置为 0 则必须显式传值
  "time_from": "2025-10-01",      // 可选
  "time_to": "2025-10-31",        // 可选
  "task_name": "daily_backup",    // 可选，模糊匹配
  "executor_name": "exec-01"      // 可选，模糊匹配
}

// POST /api/admin/logs/clean-task-logs
{
  "retention_days": 30,           // 同上
  "time_from": "2025-10-01",
  "time_to": "2025-10-31",
  "task_name": "daily_backup",
  "executor_name": "exec-01"
}
```

**retention_days 取值规则**：
- 客户端显式传入且 > 0 → 使用传入值
- 客户端未传入 → 使用系统配置
- 两者都为 0 → BadRequest 400（手动清理不能完全禁用，必须指定有效天数）
- 显式传入 0 → BadRequest 400（同上）

### 5.3 响应体

```json
{
  "code": 0,
  "message": "清理完成",
  "data": {
    "deleted_count": 1523,
    "retention_days": 30
  }
}
```

### 5.4 Handler 规范

- 每个 Handler 包裹 `defer recover()` panic 保护
- 为清理操作设置 **5 分钟 context 超时**（大批量清理可能耗时较长）
- 参数校验：retention_days 必须 >= 0；TimeFrom/TimeTo 格式校验（`2006-01-02`）
- 域权限校验：与现有 LogHandler 保持一致（domain_id、role、user_id）

### 5.5 审计日志记录

清理操作属于管理动作，应写入审计日志：
- action: `clean`
- resource: `task_execution` / `task_log` / `audit_log`
- resource_id: 不适用（批量操作）
- detail: 包含 retention_days、筛选条件、deleted_count

## 6. 定时清理 Ticker 设计

### 6.1 启动位置

**选择：在 `scheduler/cmd/app.go` 中新建独立 goroutine**（与审计日志清理 goroutine 并列），而不是 `StartCleanupRoutine()` 内。

理由：
- 审计日志清理 goroutine 已在 app.go 中独立启动（24 小时），保持相同模式便于维护
- SchedulerService 不持有 sysConfigService 和 auditLogService，避免为了 ticker 而注入额外依赖
- cleanup_service.go 的 stopCleanupCh 由 StopCleanupRoutine() 关闭，日志清理是后台常驻任务不应被 StopCleanupRoutine 关闭

位置：`scheduler/cmd/app.go` 中，在审计日志清理 goroutine 之后追加。

### 6.2 ticker 参数

- **间隔**：`6 * time.Hour`（独立 ticker，不与现有 60 秒 ticker 耦合，也不与 24 小时审计日志 ticker 耦合）
- **Leader 检查**：`app.schedulerService.IsLeader()`，非 leader 实例跳过执行
- **首次执行**：启动后 **立即执行一次**（参考审计日志 goroutine 的模式）
- **recover 保护**：goroutine 入口加 defer recover（与审计日志 goroutine 模式一致）
- **优雅停止**：通过 `os.Signal` 或 context（参考 app 关闭流程）

### 6.3 每次执行流程

```
启动 goroutine (app.go):
  1. defer recover() panic 保护
  2. 立即执行 doLogCleanup()
  3. time.NewTicker(6 * time.Hour)
  4. for range ticker.C:
     a. 检查 IsLeader()
     b. doLogCleanup()

doLogCleanup():
  并发执行两个清理（两个独立 context）：
    - cleanTaskExecutions()   → schedulerService.CleanExpiredExecutions(ctx, filter)
    - cleanTaskLogs()         → schedulerService.CleanExpiredTaskLogs(ctx, filter)
  每个清理独立 context（5 分钟超时），互不干扰
  每个清理前检查 retention_days > 0，为 0 则跳过
  全部完成后汇总日志："log cleanup cycle done, exec=X, task_logs=Y"
```

**为什么审计日志清理不合并**：审计日志清理已有独立 goroutine（24 小时周期），职责明确、运行稳定，无需改动。执行记录/过程日志清理是新增能力，独立 ticker 更清晰。

### 6.4 并发控制

两个清理在同一 tick 内用 goroutine 并发执行，各自持有独立 context。选择并发而非串行的理由：
- 两张表独立，清理互不干扰
- 清理时间可能较长（万级数据），并发可缩短整体耗时
- 每个清理方法已有分批 + context 超时保护，不会互相阻塞

### 6.5 优雅停止

新 ticker goroutine 在 app.go 中启动，由 app 的优雅关闭流程统一处理（与审计日志清理 goroutine 一样，使用 `ticker.Stop()` 和正常 goroutine 退出）。

## 7. 前端设计

### 7.1 改动页面

**任务日志页面**：`web/src/views/Logs.vue`（路由 `/logs`，组件已在 router/index.ts 注册）

新增两个按钮，放在页面顶部工具栏，样式参考审计日志页面的"清理过期"按钮：

- **清理过期执行记录**（主色调按钮）→ 弹出对话框，可选筛选条件（时间范围 + 任务名 + 执行器 + 保留天数），确认后调用 `POST /admin/logs/clean-executions`
- **清理过程日志**（次级按钮）→ 同上对话框，调用 `POST /admin/logs/clean-task-logs`

对话框使用 Element Plus 的 `el-dialog` 或 `el-drawer`，表单控件：
- 保留天数：number input（只读，显示系统当前配置；允许覆盖）
- 时间范围：`el-date-picker` type="daterange"
- 任务名：`el-input` placeholder="模糊匹配"
- 执行器名：`el-input` placeholder="模糊匹配"

点击确认时需弹出二次确认框，显示将删除的数据量统计（如有）。

### 7.2 系统配置页面

自动展示新增的两个配置项（因为 `configMetaList` 扩展后，系统配置页面读取元数据即可渲染）。

### 7.3 API 层

在 `web/src/api/` 下（推测 `logs.ts` 或类似文件）新增：

```typescript
cleanExecutions: (data: ExecutionCleanupParams) =>
  api.post('/admin/logs/clean-executions', data),

cleanTaskLogs: (data: TaskLogCleanupParams) =>
  api.post('/admin/logs/clean-task-logs', data),
```

### 7.4 其他页面

- **审计日志页面**：现有手动清理按钮保留不变，无需改动（只是后端多了自动清理）
- **路由/权限**：无需改动，复用现有日志页面和 system_admin 权限

## 8. 数据库迁移

**迁移文件**：`deploy/migrations/v9_task_log_cleanup_init.sql`（当前最新迁移为 v8_grafana_dashboards.sql）

内容：
```sql
-- 初始化任务日志清理相关系统配置项
INSERT OR IGNORE INTO bdopsflow_system_config (config_key, config_value, description, updated_at) VALUES
('task_execution.retention_days', '30', '执行记录保留天数，超过此天数的记录将被自动清理。设为 0 禁用自动清理。', datetime('now')),
('task_log.retention_days', '30', '过程日志保留天数，超过此天数的日志将被自动清理。设为 0 禁用自动清理。', datetime('now'));
```

**schema.sql 更新**：同步更新初始化配置项 INSERT 语句（v5 版本已有相同模式）。

## 9. 错误处理与边界情况

| 场景 | 处理策略 |
|------|----------|
| 定时清理时系统配置读不到 | 回退默认值（30 天），但记录 Warn 日志 |
| 定时清理中某张表清理失败 | 记录 Error，继续下一张表清理，最终汇总全部结果 |
| 手动清理请求 retention_days=0 | BadRequest 400，提示不能为 0（手动清理必须指定有效天数） |
| 手动清理请求 time_from > time_to | BadRequest 400 |
| 过程日志清理时时间字段为 NULL（执行中产生但未写入完成） | rqlite DELETE 不会匹配 NULL 条件，安全跳过 |
| 单次清理遇到大量数据（> 10万条） | 分批删除 + context 超时保护，超时后返回已删除数量 + ctx.Err() |
| rqlite 分布式环境多节点同时执行 | IsLeader() 守卫，只有 leader 执行清理 |
| 手动清理时没有符合条件的数据 | 返回 `{deleted_count: 0}` 正常响应，不视为错误 |

## 10. 测试策略

### 10.1 后端单元测试

| 测试对象 | 测试用例 |
|----------|----------|
| `CleanExpiredExecutions` | ① 过期执行记录被删除 ② 级联删除关联过程日志 ③ 未过期记录保留 ④ 空结果返回 0 ⑤ TaskName/ExecutorName 筛选生效 ⑥ TimeFrom/TimeTo 筛选生效 ⑦ 保留天数 = 0 时跳过 ⑧ context 超时返回已删除数量 |
| `CleanExpiredTaskLogs` | ① 过期日志被删除 ② 执行记录不受影响 ③ 空结果返回 0 ④ 各筛选条件生效 ⑤ 保留天数 = 0 时跳过 |
| Handler | ① 参数校验（非法 retention_days、时间格式） ② context 超时处理 ③ 响应格式正确 ④ 空筛选条件默认使用系统配置 |
| TickRoutine | ① 启动后立即执行一次 ② IsLeader 守卫 ③ 非 leader 跳过 ④ stopCleanupCh 触发退出 |

### 10.2 集成测试

- 完整清理流程：创建测试执行记录 + 过程日志 + 审计日志 → 把 retention_days 临时设为 0 触发手动清理 → 验证删除正确 → 还原配置

### 10.3 测试数据构造

- 在 `execution_service_test.go` 和现有 MockDB 体系基础上扩展
- 构造 3 组执行记录：刚创建的（不删）、刚过期的（保留天数+1）、已过期很久的（保留天数+30）

## 11. 实现文件清单

### 后端修改

| 文件 | 改动 |
|------|------|
| `scheduler/internal/service/execution_service.go` | 新增 `CleanExpiredExecutions`、`CleanExpiredTaskLogs`、filter struct、配置读取方法 |
| `scheduler/cmd/app.go` | 新增日志清理 ticker goroutine（在审计日志 goroutine 之后），sysConfigService 需注入 schedulerService 供 ticker 使用 |
| `scheduler/cmd/routes.go` | 在 `admin` group 中注册两条新路由 |
| `scheduler/internal/handler/log.go` | 新增 `CleanExecutions`、`CleanTaskLogs` handler，新增 `defer recover()` 到新 handler |
| `scheduler/internal/system_config/service.go` | `defaultConfigValues` + `configMetaList` 新增 2 个 key |
| `scheduler/internal/service/cleanup_service.go` | **无需改动**（日志清理 ticker 独立在 app.go） |
| `scheduler/internal/service/audit_log.go` | **无需改动**（自动清理已在 app.go 独立实现） |

### 后端新增

| 文件 | 用途 |
|------|------|
| `scheduler/internal/service/execution_cleanup_test.go` | 清理相关单元测试 |
| `deploy/migrations/v9_task_log_cleanup_init.sql` | 数据库迁移（新增系统配置项） |

### 前端修改

| 文件 | 改动 |
|------|------|
| `web/src/api/` 下的日志 API 文件 | 新增 `cleanExecutions`、`cleanTaskLogs` 方法 |
| 任务日志页面组件 | 新增两个清理按钮 + 筛选对话框 |
| 系统配置页面 | 自动展示新增配置项（无需显式改动，configMetaList 驱动） |

### Schema 修改

| 文件 | 改动 |
|------|------|
| `deploy/schema.sql` | 同步新增的 2 个配置项初始化语句 |

---

*设计文档已完成，等待用户 review 确认后进入实现规划阶段。*
