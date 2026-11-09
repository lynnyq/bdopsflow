# Implementation Plan: 任务日志清理功能

- **Status**: Draft
- **Spec**: [2026-11-09-task-log-cleanup-design.md](../specs/2026-11-09-task-log-cleanup-design.md)
- **Date**: 2026-11-09
- **Author**: writing-plans

## Goals

- [x] 新增系统配置项 `task_execution.retention_days`（默认 30）和 `task_log.retention_days`（默认 30）
- [x] Service 层实现 `CleanExpiredExecutions`（级联删过程日志）和 `CleanExpiredTaskLogs`（独立清理）
- [x] 2 个手动清理 API：`POST /api/admin/logs/clean-executions`、`POST /api/admin/logs/clean-task-logs`
- [x] 独立 6 小时 ticker goroutine（在 `scheduler/cmd/app.go`），启动立即执行一次，leader 检查
- [x] 前端 `web/src/views/Logs.vue` 新增"清理过期执行记录"和"清理过程日志"按钮 + 筛选对话框
- [x] 单元测试覆盖清理方法核心路径

## Out of Scope

- 审计日志清理（已在 `scheduler/cmd/app.go` L358-388 独立实现，24 小时 ticker）
- `datasource.history_retention_days` 的自动清理（配置项存在但清理未实现，独立需求）
- 前端清理对话框的数据量预估（先简化，后续迭代）
- Grafana 监控指标扩展（后续迭代）

## Tech Stack (resolved)

- **分批删除模式**：与 `AuditLogService.CleanExpired()` 完全一致，使用 `DELETE ... WHERE id IN (SELECT id ... LIMIT 1000)` 子查询（rqlite 兼容，不依赖 SQLITE_ENABLE_UPDATE_DELETE_LIMIT）
- **context 超时**：手动清理 API 5 分钟，定时 ticker 内部 5 分钟（与手动一致）
- **filter struct**：在 `execution_service.go` 中定义，避免独立文件增加复杂度
- **sysConfigService 注入**：SchedulerService 通过 `system_config.GetGlobalService()` 单例访问（无需显式注入，与 AuditLogService 模式一致）

## Phases

---

### Phase 1: 系统配置 + Schema + 数据库迁移

**Goal**: 新增 2 个系统配置项，前后端可通过统一 API 读写。

**Tasks**:

- [ ] 1.1 更新 `scheduler/internal/system_config/service.go` 的 `defaultConfigValues` 新增：
  ```go
  "task_execution.retention_days": "30",
  "task_log.retention_days":       "30",
  ```

- [ ] 1.2 更新同文件 `configMetaList` 新增 2 条 ConfigMeta（group: "其他"，与 audit_log.retention_days 同行）：
  ```go
  {
      Key: "task_execution.retention_days", Label: "执行记录保留天数",
      Description:  "执行记录的保留天数，超过此天数的记录将被自动清理（同时级联删除关联的过程日志）。设为 0 禁用自动清理。",
      Type:        "number", DefaultValue: "30", MinValue: intPtr(0), MaxValue: intPtr(3650), Unit: "天",
      Group: "其他",
  },
  {
      Key: "task_log.retention_days", Label: "过程日志保留天数",
      Description:  "过程日志的保留天数，超过此天数的日志将被自动清理。设为 0 禁用自动清理。",
      Type:        "number", DefaultValue: "30", MinValue: intPtr(0), MaxValue: intPtr(3650), Unit: "天",
      Group: "其他",
  },
  ```

- [ ] 1.3 新增迁移文件 `deploy/migrations/v9_task_log_cleanup_init.sql`：
  ```sql
  INSERT OR IGNORE INTO bdopsflow_system_config (config_key, config_value, description, updated_at) VALUES
  ('task_execution.retention_days', '30', '执行记录保留天数，超过此天数的记录将被自动清理（同时级联删除关联的过程日志）。设为 0 禁用自动清理。', datetime('now')),
  ('task_log.retention_days', '30', '过程日志保留天数，超过此天数的日志将被自动清理。设为 0 禁用自动清理。', datetime('now'));
  ```

- [ ] 1.4 更新 `deploy/schema.sql` 初始化配置 INSERT 语句，同步新增 2 个 key（找到现有 `audit_log.retention_days` 所在的 VALUES 列表追加）

**Verification**:
- `cd scheduler && go build ./...` 退出 0
- 配置页面（前端或 curl）能看到 2 个新配置项，值为 30
- 调用 `PUT /api/admin/system-config/task_execution.retention_days` 能正常修改

---

### Phase 2: Service 层清理方法

**Goal**: 在 SchedulerService 中实现核心清理逻辑（分批 + 级联 + context 保护）。

**Tasks**:

- [ ] 2.1 在 `scheduler/internal/service/execution_service.go` 新增 filter struct：
  ```go
  type ExecutionCleanupFilter struct {
      RetentionDays int    // 保留天数（必需，>0 才执行）
      TimeFrom      string // 可选，end_time 下限（YYYY-MM-DD 格式）
      TimeTo        string // 可选，end_time 上限（YYYY-MM-DD 格式）
      TaskName      string // 可选，任务名模糊匹配
      ExecutorName  string // 可选，执行器名模糊匹配
  }

  type TaskLogCleanupFilter struct {
      RetentionDays int    // 保留天数（必需，>0 才执行）
      TimeFrom      string // 可选，log_time 下限
      TimeTo        string // 可选，log_time 上限
      TaskName      string // 可选，JOIN bdopsflow_tasks 模糊匹配
      ExecutorName  string // 可选，JOIN bdopsflow_executors 模糊匹配
  }
  ```

- [ ] 2.2 新增配置读取方法：
  ```go
  func (s *SchedulerService) GetExecutionRetentionDays() int {
      svc := system_config.GetGlobalService()
      if svc != nil {
          days := svc.GetInt("task_execution.retention_days")
          if days >= 0 {
              return days
          }
      }
      return 30
  }

  func (s *SchedulerService) GetTaskLogRetentionDays() int {
      svc := system_config.GetGlobalService()
      if svc != nil {
          days := svc.GetInt("task_log.retention_days")
          if days >= 0 {
              return days
          }
      }
      return 30
  }
  ```
  注意：需在文件 import 中新增 `sysconfig "github.com/lynnyq/bdopsflow/scheduler/internal/system_config"`（如尚未引入）。

- [ ] 2.3 实现 `CleanExpiredExecutions`：
  - cutoffTime = now - RetentionDays
  - 分批删除（每批 1000），每批步骤：
    1. 子查询收集一批 execution_id：`SELECT execution_id FROM bdopsflow_task_executions WHERE end_time < ? [AND TaskName/ExecutorName/TimeFrom/TimeTo 条件] LIMIT 1000`
    2. 同事务删过程日志：`DELETE FROM bdopsflow_task_logs WHERE execution_id IN (...)`
    3. 同事务删执行记录：`DELETE FROM bdopsflow_task_executions WHERE id IN (...)`
  - 循环直到本批删除 < batchSize
  - context 检查、错误日志、返回已删除数量
  - TaskName/ExecutorName 条件需 JOIN bdopsflow_tasks / bdopsflow_executors
  - 参考 AuditLogService.CleanExpired 的模式

- [ ] 2.4 实现 `CleanExpiredTaskLogs`：
  - cutoffTime = now - RetentionDays
  - 分批删除（每批 1000），子查询：`DELETE FROM bdopsflow_task_logs WHERE id IN (SELECT id FROM bdopsflow_task_logs WHERE log_time < ? [AND JOIN 条件] LIMIT 1000)`
  - 不触碰执行记录
  - TaskName/ExecutorName 条件需 JOIN bdopsflow_tasks / bdopsflow_executors
  - context 检查、错误日志

**Verification**:
- `cd scheduler && go build ./...` 退出 0
- `cd scheduler && go vet ./internal/service/...` 退出 0

---

### Phase 3: Handler + API 路由

**Goal**: 暴露 2 个手动清理 API，参数校验 + 域权限 + 审计日志记录。

**Tasks**:

- [ ] 3.1 在 `scheduler/internal/handler/log.go` 新增 2 个 handler：

  `CleanExecutions(c *gin.Context)`:
  - defer recover() panic 保护
  - 解析请求体：retention_days（可选 int）、time_from、time_to、task_name、executor_name
  - 计算最终 retentionDays：显式传入 > 0 → 用传入值；否则用系统配置；两者都为 0 → 400 BadRequest
  - 时间格式校验（`2006-01-02`）
  - time_from > time_to → 400
  - context 5 分钟超时
  - 域权限校验（domain_id、role、user_id）
  - 调用 schedulerService.CleanExpiredExecutions
  - 写审计日志（action=clean, resource=task_execution, detail={retention_days, filters, deleted_count}）
  - 返回 `{code: 0, data: {deleted_count, retention_days}}`

  `CleanTaskLogs(c *gin.Context)`:
  - 同上模式，调用 `schedulerService.CleanExpiredTaskLogs`
  - 审计日志 resource=task_log

- [ ] 3.2 在 `scheduler/cmd/routes.go` 的 `admin` group 中（audit-logs 路由附近）新增：
  ```go
  logsAdmin := admin.Group("/logs")
  logsAdmin.POST("/clean-executions", middleware.RequireSystemAdmin(app.permissionService), logHandler.CleanExecutions)
  logsAdmin.POST("/clean-task-logs", middleware.RequireSystemAdmin(app.permissionService), logHandler.CleanTaskLogs)
  ```
  注意：`logHandler` 已在 L109 初始化，但当前注册在 `protected.Group("/logs")` 下。新路由注册在 `admin` group 内（system_admin 权限）。

**Verification**:
- `cd scheduler && go build ./...` 退出 0
- curl 调用（需 JWT token）：
  ```bash
  curl -X POST http://localhost:8080/api/admin/logs/clean-executions \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"retention_days": 30}'
  ```
  预期返回 `{"code":0,"data":{"deleted_count":0,"retention_days":30}}`
- 参数校验：retention_days=0 返回 400

---

### Phase 4: 定时清理 Ticker

**Goal**: app.go 中新增 6 小时 ticker goroutine，并发清理执行记录和过程日志。

**Tasks**:

- [ ] 4.1 在 `scheduler/cmd/app.go` 审计日志清理 goroutine（L388 `}()`）之后追加新 goroutine：
  ```go
  // 任务日志（执行记录 + 过程日志）自动清理 goroutine：启动立即执行一次，之后每 6 小时执行一次。
  go func() {
      defer func() {
          if r := recover(); r != nil {
              slog.Error("task log cleanup goroutine panicked", "recover", r)
          }
      }()

      runLogCleanup := func() {
          // leader 检查
          if !schedulerService.IsLeader() {
              return
          }
          // 并发清理两个表
          var execDeleted, logDeleted int64
          var execErr, logErr error
          var wg sync.WaitGroup
          wg.Add(2)

          go func() {
              defer wg.Done()
              days := schedulerService.GetExecutionRetentionDays()
              if days <= 0 {
                  return
              }
              ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
              defer cancel()
              execDeleted, execErr = schedulerService.CleanExpiredExecutions(ctx, service.ExecutionCleanupFilter{RetentionDays: days})
          }()

          go func() {
              defer wg.Done()
              days := schedulerService.GetTaskLogRetentionDays()
              if days <= 0 {
                  return
              }
              ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
              defer cancel()
              logDeleted, logErr = schedulerService.CleanExpiredTaskLogs(ctx, service.TaskLogCleanupFilter{RetentionDays: days})
          }()

          wg.Wait()
          slog.Info("task log cleanup cycle done",
              "exec_deleted", execDeleted, "exec_err", execErr,
              "task_log_deleted", logDeleted, "task_log_err", logErr,
          )
      }

      runLogCleanup()
      ticker := time.NewTicker(6 * time.Hour)
      defer ticker.Stop()
      for range ticker.C {
          runLogCleanup()
      }
  }()
  ```

- [ ] 4.2 确保 import 中有 `sync`（如未引入）

**Verification**:
- `cd scheduler && go build ./...` 退出 0
- 启动 scheduler，日志中可见 `"task log cleanup cycle done, exec_deleted=X, task_log_deleted=Y"`
- 非 leader 实例不执行（leader 转移后下次 tick 正常执行）

---

### Phase 5: 前端改造

**Goal**: Logs.vue 新增清理按钮 + 筛选对话框，API 层新增 2 个方法。

**前置**: 先读取 `web/src/views/Logs.vue` 和 `web/src/api/admin.ts`（或相关 API 文件）的完整内容，了解现有结构后再改造。

**Tasks**:

- [ ] 5.1 API 层：在 `web/src/api/` 下合适位置（推测 `admin.ts` 或新建 `logs.ts`）新增：
  ```typescript
  cleanExecutions: (data: {
    retention_days?: number
    time_from?: string
    time_to?: string
    task_name?: string
    executor_name?: string
  }) => api.post('/admin/logs/clean-executions', data),

  cleanTaskLogs: (data: {
    retention_days?: number
    time_from?: string
    time_to?: string
    task_name?: string
    executor_name?: string
  }) => api.post('/admin/logs/clean-task-logs', data),
  ```

- [ ] 5.2 `web/src/views/Logs.vue` 顶部工具栏新增 2 个按钮（参考 `AuditLogs.vue` L41-43 的"清理过期"按钮样式）：
  - **清理过期执行记录**（warning 类型）
  - **清理过程日志**（warning 类型，次级）
  - 两个按钮都需 admin 权限（v-if 判断）

- [ ] 5.3 新增筛选对话框（el-dialog）：
  - 标题：清理执行记录 / 清理过程日志（根据按钮切换）
  - 表单：保留天数（只读显示系统配置，允许覆盖）、时间范围（el-date-picker type="daterange"）、任务名（el-input）、执行器名（el-input）
  - 确认按钮二次确认（ElMessageBox.confirm）
  - 成功后显示删除条数，刷新列表

**Verification**:
- `cd web && npm run build` 退出 0
- 浏览器访问 /logs，admin 用户可见 2 个清理按钮
- 点击按钮 → 对话框 → 确认 → API 调用 → toast 显示结果

---

### Phase 6: 单元测试

**Goal**: 核心清理路径有测试覆盖，mockDB 体系下验证 SQL 语句和返回值。

**前置**: 先读取 `scheduler/internal/service/execution_service_test.go` 中 MockDB 构造方式（`newSchedulerWithDB`、`newSchedulerWithDBAndRedis`）。

**Tasks**:

- [ ] 6.1 在 `scheduler/internal/service/` 下（可新建 `execution_cleanup_test.go` 或追加到现有测试文件）新增 `TestCleanExpiredExecutions` 系列：
  - 过期执行记录被删除（MockDB 返回有过期数据）
  - 未过期记录保留
  - 级联删除过程日志（验证 WriteStmts 中有两次 DELETE）
  - TaskName 筛选 JOIN 生效
  - RetentionDays=0 跳过
  - context 超时

- [ ] 6.2 `TestCleanExpiredTaskLogs` 系列：
  - 过期日志被删除
  - 执行记录不受影响
  - 空结果返回 0

**Verification**:
- `cd scheduler && go test ./internal/service/... -run TestCleanExpired -v` 全部 PASS
- `cd scheduler && go test ./... -v` 无回归

---

## Risks & Open Questions

1. **SchedulerService 是否已 import system_config？** 需要在 execution_service.go 中新增 import，如果已有则跳过。
2. **前端 API 文件位置**：需先确认是追加到 `admin.ts` 还是新建 `logs.ts`，倾向新建 `logs.ts` 更清晰。
3. **rqlite 分批 DELETE 性能**：每批 1000 条 DELETE 在 rqlite 分布式环境是否有锁竞争？参考审计日志清理的 1000 批次，应该安全。
4. **ExecutionCleanupFilter 的 JOIN 条件正确性**：任务名模糊匹配需要 JOIN bdopsflow_tasks ON task_id；执行器名模糊匹配需要 JOIN bdopsflow_executors ON executor_id；需确保执行器表中 executor_id 可能为 NULL 时的处理（LEFT JOIN）。
5. **迁移文件执行**：v9 迁移上线后，新配置项会被 INSERT OR IGNORE 到 bdopsflow_system_config，旧部署升级后配置页面自动可见。

## Definition of Done

- 所有 Phase 完成（Phase 1-4 后端 + Phase 5 前端 + Phase 6 测试）
- `go build ./...` 和 `npm run build` 均退出 0
- `go test ./...` 全部 PASS
- 手动清理 API curl 验证通过
- 启动 scheduler 后日志中可见 `"task log cleanup cycle done"`
- 配置页面可见 2 个新增配置项
- 前端按钮可点击、对话框可弹出、API 调用成功
