# BDopsFlow 核心功能参考

本文档详细说明 BDopsFlow 分布式工作流调度平台的所有核心功能实现。

## 目录

- [主节点选举](#主节点选举)
- [分布式任务锁](#分布式任务锁)
- [DAG 工作流编排](#dag-工作流编排)
- [Webhook 回调系统](#webhook-回调系统)
- [任务调度系统](#任务调度系统)
- [血缘关系追踪](#血缘关系追踪)
- [指标收集系统](#指标收集系统)
- [RBAC 权限管理](#rbac-权限管理)
- [领域隔离](#领域隔离)
- [执行器-调度器通信](#执行器-调度器通信)
- [故障恢复机制](#故障恢复机制)
- [数据源查询系统](#数据源查询系统)
- [审计日志系统](#审计日志系统)

---

## 主节点选举

基于 Redis 实现的主节点选举机制，确保只有一个调度器节点成为主节点执行调度任务。

### 实现细节

**位置**: `scheduler/pkg/election/leader.go`

**核心特性**:
- 基于 Redis `SetNX` 原语实现选举
- 主节点通过定期刷新 TTL 维持领导地位
- 支持获取/释放领导权的回调函数
- 故障自动检测和新选举

### 核心数据结构

```go
type LeaderElection struct {
    client    *redis.Client
    leaderKey string
    nodeID    string
    ttl       time.Duration
    isLeader  bool
    onAcquire func()
    onRelease func()
}
```

### 选举流程

1. **尝试获取领导权**: 调用 `SetNX` 尝试设置 leader key
2. **成功成为主节点**: 触发 `OnAcquire` 回调
3. **维持领导地位**: 定期 `Expire` 刷新 TTL
4. **释放领导权**: 调用 `Del` 删除 key，触发 `OnRelease`

### 使用示例

```go
election := election.NewLeaderElection(
    redisClient,
    "bdopsflow:leader",
    nodeID,
    15*time.Second,
)

election.OnAcquire(func() {
    fmt.Println("成为主节点，开始调度任务")
    cronScheduler.Start()
})

election.OnRelease(func() {
    fmt.Println("失去主节点地位")
})

election.Start(ctx)
```

---

## 分布式任务锁

基于 Redis 的分布式锁实现，防止任务重复执行。

### 实现细节

**位置**: `scheduler/pkg/lock/redis_lock.go`

**核心特性**:
- `TryLock`: 尝试获取锁，立即返回
- `Unlock`: 释放锁
- `KeepAlive`: 续约锁的 TTL

### 核心数据结构

```go
type RedisLock struct {
    client *redis.Client
    prefix string
}
```

### 使用场景

1. **Cron 任务执行前**: 获取锁，防止多节点重复触发
2. **任务执行中**: 通过锁超时防止任务卡死
3. **任务完成后**: 释放锁

---

## DAG 工作流编排

支持复杂工作流编排的有向无环图（DAG）系统。

### 实现细节

**位置**: `scheduler/internal/dag/dag.go`

**核心特性**:
- 节点验证：检查节点是否存在
- 循环检测：使用拓扑排序检测循环
- 依赖分析：获取节点的前置/后置依赖
- 执行顺序计算：通过拓扑排序得到执行序列

### 核心数据结构

```go
type DAGConfig struct {
    Nodes       []DAGNode       `json:"nodes"`
    Connections []DAGConnection `json:"connections"`
}

type DAGNode struct {
    ID          string                 `json:"id"`
    Name        string                 `json:"name"`
    Type        string                 `json:"type"`
    Config      map[string]interface{} `json:"config"`
    Position    Position               `json:"position"`
    TimeoutSec  int                    `json:"timeout_seconds"`
    RetryCount  int                    `json:"retry_count"`
}

type DAGConnection struct {
    From string `json:"from"`
    To   string `json:"to"`
}
```

### DAG 验证流程

1. **检查节点存在**: 验证连接引用的节点是否都定义
2. **检查重复连接**: 防止重复的边
3. **检查自环**: 禁止节点指向自身
4. **检查循环**: 拓扑排序检测循环依赖

### 工作流执行

当工作流被触发时：
1. 解析 DAG 配置
2. 验证 DAG 结构
3. 计算拓扑排序
4. 按顺序执行节点，确保所有前置节点完成
5. 记录每个节点的执行状态

---

## Webhook 回调系统

灵活的任务事件回调通知系统。

### 实现细节

**位置**: `scheduler/internal/webhook/webhook.go`

**核心特性**:
- 支持自定义 URL、HTTP 方法、请求头
- 支持事件类型过滤
- 失败重试机制
- 支持从 map 动态加载配置

### 核心数据结构

```go
type WebhookConfig struct {
    URL      string            `json:"url"`
    Method   string            `json:"method"`
    Headers  map[string]string `json:"headers"`
    Events   []string          `json:"events"`
}

type WebhookPayload struct {
    Event       string      `json:"event"`
    Timestamp   int64       `json:"timestamp"`
    TaskID      int64       `json:"task_id"`
    ExecutionID string      `json:"execution_id"`
    Status      string      `json:"status"`
    Output      string      `json:"output"`
    Error       string      `json:"error"`
    Duration    int64       `json:"duration_ms"`
    Metadata    interface{} `json:"metadata,omitempty"`
}
```

### 支持的事件类型

- `task_started`: 任务开始执行
- `task_completed`: 任务执行成功
- `task_failed`: 任务执行失败
- `*`: 所有事件

### 配置示例

```json
{
    "url": "https://api.example.com/webhook",
    "method": "POST",
    "headers": {
        "X-API-Key": "secret-key",
        "Content-Type": "application/json"
    },
    "events": ["task_completed", "task_failed"]
}
```

### 发送流程

1. 检查事件是否匹配配置
2. 序列化 payload 为 JSON
3. 发送 HTTP 请求
4. 验证响应状态码
5. 失败时按指数退避重试

---

## 任务调度系统

基于 Robfig Cron 的定时任务调度系统。

### 实现细节

**位置**: `scheduler/internal/cron/cron_scheduler.go`

**核心特性**:
- 支持标准 5 字段 Cron 表达式
- 支持 6 字段秒级 Cron 表达式
- 主节点唯一调度：只有 leader 执行调度
- 暂停/恢复：支持全局暂停调度
- 任务注册/取消注册：动态管理调度任务

### 核心数据结构

```go
type CronScheduler struct {
    cron        *cron.Cron
    svc         *service.SchedulerService
    redis       *redis.Client
    taskEntries map[int64]cron.EntryID
    mu          sync.RWMutex
    paused      bool
    isLeader    bool
    started     bool
    startTime   time.Time
}
```

### 调度流程

1. **成为主节点**: 触发 `OnBecomeLeader`
2. **加载任务**: 从数据库加载所有启用的任务
3. **注册 Cron**: 使用 cron.AddFunc 注册
4. **触发执行**: Cron 触发时执行任务
5. **获取锁**: 分布式锁防止重复执行
6. **调用 Trigger**: 触发任务执行

### Cron 表达式支持

**5 字段格式**:
```
┌───────────── 分钟 (0-59)
│ ┌───────────── 小时 (0-23)
│ │ ┌───────────── 日期 (1-31)
│ │ │ ┌───────────── 月份 (1-12)
│ │ │ │ ┌───────────── 星期 (0-6, 0=周日)
│ │ │ │ │
* * * * *
```

**6 字段格式**:
```
┌───────────── 秒 (0-59)
│ ┌───────────── 分钟 (0-59)
│ │ ┌───────────── 小时 (0-23)
│ │ │ ┌───────────── 日期 (1-31)
│ │ │ │ ┌───────────── 月份 (1-12)
│ │ │ │ │ ┌───────────── 星期 (0-6, 0=周日)
│ │ │ │ │ │
* * * * * *
```

### API 接口

```go
// 注册任务
func (cs *CronScheduler) RegisterTask(taskID int64, cronExpr string)

// 取消注册任务
func (cs *CronScheduler) UnregisterTask(taskID int64)

// 暂停调度
func (cs *CronScheduler) Pause()

// 恢复调度
func (cs *CronScheduler) Resume()

// 检查是否暂停
func (cs *CronScheduler) IsPaused() bool
```

---

## 血缘关系追踪

追踪任务之间的依赖关系和影响范围。

### 实现细节

**位置**: `scheduler/internal/lineage/lineage.go`

**核心特性**:
- 获取任务的上游/下游血缘关系
- 获取工作流的完整血缘图
- 获取任务的影响范围（下游任务）
- 管理任务依赖关系

### 核心数据结构

```go
type TaskNode struct {
    ID       int64       `json:"id"`
    Name     string      `json:"name"`
    Type     string      `json:"type"`
    Status   string      `json:"status"`
    Children []*TaskNode `json:"children,omitempty"`
    Parents  []*TaskNode `json:"parents,omitempty"`
}

type LineageGraph struct {
    Tasks     []*TaskNode `json:"tasks"`
    Relations []Relation  `json:"relations"`
}

type Relation struct {
    From int64 `json:"from"`
    To   int64 `json:"to"`
}
```

### 查询示例

**获取任务上游血缘**:
```go
graph, err := lineageService.GetTaskLineage(ctx, taskID)
// 返回包含该任务所有上游依赖的图
```

**获取任务影响范围**:
```go
impacted, err := lineageService.GetTaskImpact(ctx, taskID)
// 返回所有依赖该任务的下游任务
```

---

## 指标收集系统

收集和管理系统运行时指标。

### 实现细节

**位置**: `scheduler/internal/metrics/metrics.go`

**核心特性**:
- 计数器（Counter）：统计事件发生次数
- 仪表盘（Gauge）：记录当前状态值
- 直方图（Histogram）：统计分布和平均值
- Redis 持久化：保存指标快照
- 后台自动收集：定期保存指标

### 核心数据结构

```go
type MetricsCollector struct {
    redis              *redis.Client
    metrics            map[string]*Counter
    gauges             map[string]*Gauge
    histograms         map[string]*Histogram
    mu                 sync.RWMutex
    collectionInterval time.Duration
}

type Counter struct {
    Name  string
    Value int64
    mu    sync.Mutex
}

type Gauge struct {
    Name  string
    Value float64
    mu    sync.Mutex
}

type Histogram struct {
    Name   string
    Values []float64
    mu     sync.Mutex
}
```

### 支持的指标

| 指标名 | 类型 | 描述 |
|--------|------|------|
| `bdopsflow:tasks:created` | Counter | 任务创建次数 |
| `bdopsflow:tasks:completed` | Counter | 任务成功完成次数 |
| `bdopsflow:tasks:failed` | Counter | 任务失败次数 |
| `bdopsflow:tasks:running` | Gauge | 当前运行任务数 |
| `bdopsflow:executors:online` | Gauge | 在线执行器数 |
| `bdopsflow:executors:offline` | Gauge | 离线执行器数 |
| `bdopsflow:task:duration_seconds` | Histogram | 任务执行耗时分布 |
| `bdopsflow:workflow:created` | Counter | 工作流创建次数 |
| `bdopsflow:workflow:running` | Gauge | 当前运行工作流数 |

### 使用示例

```go
collector := metrics.NewMetricsCollector(redisClient)
collector.StartBackgroundCollection(ctx)

// 记录任务创建
collector.RegisterCounter(metrics.MetricTasksCreated).Inc(1)

// 记录任务耗时
collector.RegisterHistogram(metrics.MetricTaskDuration).Observe(duration.Seconds())

// 设置在线执行器数
collector.RegisterGauge(metrics.MetricExecutorsOnline).Set(10)
```

---

## RBAC 权限管理

基于角色的访问控制系统，支持细粒度权限管理。

### 实现细节

**位置**: 
- `scheduler/internal/service/permission_service.go`: 权限检查逻辑
- `scheduler/internal/middleware/auth.go`: 中间件和 JWT

### 核心概念

1. **用户（User）**: 系统使用者
2. **角色（Role）**: 权限集合
3. **权限（Permission）**: 资源+操作的组合
4. **领域（Domain）**: 资源隔离单位

### 数据模型

**角色（Role）**:
```go
type Role struct {
    ID          int64   `db:"id"`
    Name        string  `db:"name"`
    Code        string  `db:"code"`
    Description string  `db:"description"`
    IsSystem    bool    `db:"is_system"`
    DomainID    *int64  `db:"domain_id"`
}
```

**权限（Permission）**:
```go
type Permission struct {
    ID          int64  `db:"id"`
    Resource    string `db:"resource"`
    Action      string `db:"action"`
    Description string `db:"description"`
}
```

### 系统预置角色

| 角色代码 | 名称 | 描述 |
|---------|------|------|
| `system_admin` | 系统管理员 | 拥有所有权限，可管理所有领域 |
| `admin` | 管理员 | 拥有指定领域的所有权限 |
| `domain_admin` | 领域管理员 | 管理特定领域的资源 |
| `user` | 普通用户 | 基础任务操作权限 |

### 权限检查流程

```go
// 1. 检查是否为系统管理员
isAdmin, err := permissionService.IsSystemAdmin(ctx, userID)
if isAdmin {
    return true
}

// 2. 获取用户角色
roles, err := permissionService.GetUserRoles(ctx, userID)

// 3. 检查每个角色是否有指定权限
for _, role := range roles {
    hasPerm := checkRolePermission(role, resource, action)
    
    // 4. 检查领域访问权限
    if hasPerm && canAccessDomain(role, domainID) {
        return true
    }
}

return false
```

### 中间件示例

```go
// JWT 认证
protected.Use(middleware.JWTAuthMiddleware())

// RBAC 检查
task.POST("", middleware.RBACMiddleware("admin", "domain_admin"), handler.Create)

// 系统管理员检查
domain.POST("", middleware.RequireSystemAdmin(permissionService), handler.Create)
```

### JWT 认证

```go
// 生成 token
token, err := middleware.GenerateToken(userID, username, role, domainID)

// 解析 token
claims, err := middleware.ParseToken(token)
```

---

## 领域隔离

实现多租户资源隔离的机制。

### 实现细节

**位置**: 
- `scheduler/internal/service/domain_admin.go`: 领域管理
- `scheduler/internal/service/executor_domain.go`: 执行器领域分配

### 核心概念

- **领域（Domain）**: 资源的隔离边界
- **领域执行器（DomainExecutor）**: 执行器与领域的关联
- **全局执行器**: 可服务所有领域的执行器

### 数据模型

```go
type Domain struct {
    ID          int64  `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
}

type DomainExecutor struct {
    DomainID   int64 `json:"domain_id"`
    ExecutorID int64 `json:"executor_id"`
}
```

### 执行器分配策略

1. **检查分配的执行器**: 优先使用分配给任务领域的执行器
2. **检查全局执行器**: 使用全局可用的执行器
3. **负载均衡**: 选择当前负载最低的执行器

### 领域管理功能

- 创建/删除/更新领域
- 查看领域统计（用户数、执行器数、任务数）
- 分配执行器到领域
- 移除执行器领域关系

---

## 执行器-调度器通信

基于 gRPC 的双向通信协议。

### 实现细节

**位置**: 
- `proto/executor.proto`: 协议定义
- `scheduler/internal/grpcserver/server.go`: 服务端实现
- `executor/internal/grpcclient/client.go`: 客户端实现

### gRPC 服务定义

```protobuf
service ExecutorService {
    // 执行器注册
    rpc Register(RegisterRequest) returns (RegisterResponse)
    
    // 心跳上报
    rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse)
    
    // 订阅任务（流式）
    rpc SubscribeTask(SubscribeTaskRequest) returns (stream Task)
    
    // 任务结果上报
    rpc ReportTaskResult(ReportTaskResultRequest) returns (ReportTaskResultResponse)
    
    // 任务日志上报
    rpc ReportTaskLog(ReportTaskLogRequest) returns (ReportTaskLogResponse)
    
    // 任务进度上报
    rpc ReportTaskProgress(ReportTaskProgressRequest) returns (ReportTaskProgressResponse)
    
    // 同步运行任务状态（新leader时）
    rpc SyncRunningTasks(SyncRunningTasksRequest) returns (SyncRunningTasksResponse)
}
```

### 心跳数据结构

```protobuf
message HeartbeatRequest {
    string executor_name = 1;
    int32 current_load = 2;
    repeated string running_execution_ids = 3;
    repeated RunningTaskState running_tasks = 4;  // 详细任务状态
    bool is_reconnect = 5;  // 是否为重新连接
}

message RunningTaskState {
    string execution_id = 1;
    int64 task_id = 2;
    int32 progress = 3;
    string progress_message = 4;
    int64 start_time = 5;
    string status = 6;
}

message HeartbeatResponse {
    bool success = 1;
    string message = 2;
    int32 target_capacity = 3;
    bool need_full_sync = 4;  // 是否需要全量同步
    string scheduler_node_id = 5;  // 调度器节点ID
    bool is_new_leader = 6;  // 是否为新leader
}
```

### 通信流程

1. **执行器启动**: 连接调度器并注册
2. **订阅任务**: 接收新任务通知
3. **执行任务**: 执行任务并实时上报日志
4. **心跳上报**: 定期上报状态和运行任务
5. **结果上报**: 任务完成后上报最终结果

### 任务生命周期

```
pending → running → completed/failed
   ↑                     ↓
   └─────────────────────┘
       (超时/重试)
```

---

## 故障恢复机制

调度器故障时的完整任务恢复系统。

### 实现细节

**位置**: 
- `scheduler/internal/service/scheduler.go`: `RecoverRunningTasksOnBecomeLeader`
- `scheduler/internal/cron/cron_scheduler.go`: `OnBecomeLeader`

### 恢复流程

当新调度器成为主节点时：

1. **标记为新 Leader**: 设置 `is_new_leader` 标志
2. **恢复正在执行的任务**: 调用 `RecoverRunningTasksOnBecomeLeader`
3. **同步执行器状态**: 执行器下次心跳时发送详细任务状态
4. **更新任务锁**: 刷新任务锁 TTL

### 任务恢复逻辑

```go
func RecoverRunningTasksOnBecomeLeader(ctx context.Context) error {
    // 1. 查询所有 status = 'running' 的任务
    executions, err := queryRunningExecutions()
    
    for _, exec := range executions {
        // 2. 验证执行器状态
        executor, err := GetExecutorByID(exec.ExecutorID)
        if err != nil || executor.Status != "online" {
            // 执行器离线 → 标记任务失败
            forceFailTask(exec.ExecutionID, "executor_offline")
            continue
        }
        
        // 3. 验证任务锁
        lockExists, _ := checkTaskLock(exec.ExecutionID)
        if !lockExists {
            // 锁不存在 → 标记任务失败
            forceFailTask(exec.ExecutionID, "lock_missing")
            continue
        }
        
        // 4. 检查任务超时
        if time.Since(exec.StartTime) > 2*time.Hour {
            forceFailTask(exec.ExecutionID, "task_timeout")
            continue
        }
        
        // 5. 刷新任务锁
        renewTaskLock(exec.ExecutionID)
        
        // 6. 添加恢复日志（去重）
        addRecoveryLogSafe(exec.ExecutionID, "recovered_by_new_leader")
        
        recoveredCount++
    }
    
    return nil
}
```

### 任务锁管理

```go
// 刷新任务锁
func renewTaskLock(ctx context.Context, executionID string) error {
    lockKey := fmt.Sprintf("task:lock:%s", executionID)
    renewKey := fmt.Sprintf("task:renew:%s", executionID)
    
    // 刷新锁 TTL
    redisClient.Set(ctx, lockKey, "recovered", 5*time.Minute)
    redisClient.Set(ctx, renewKey, time.Now().Unix(), 5*time.Minute)
    
    return nil
}

// 清理过期锁
func cleanupStaleTaskLocks() {
    // 1. 查询状态异常但锁仍然存在的任务
    // 2. 查询数据库不再是 running 的任务
    // 3. 使用 SCAN 安全删除锁
}
```

### 日志去重机制

恢复事件日志可能被多次调用，使用 Redis 实现去重：

```go
func addRecoveryLogSafe(ctx context.Context, executionID string, message string) {
    dedupKey := fmt.Sprintf("task:log:dedup:%s:recovery", executionID)
    
    // 检查是否已记录
    exists, _ := redisClient.Exists(ctx, dedupKey).Result()
    if exists > 0 {
        return  // 跳过重复记录
    }
    
    // 设置去重标记（1小时）
    redisClient.Set(ctx, dedupKey, "1", 1*time.Hour)
    
    // 记录日志
    AddTaskLog(ctx, executionID, ...)
}
```

---

## 数据源查询系统

### 实现细节

**位置**: 
- `scheduler/internal/datasource/`: 数据源管理和驱动
- `scheduler/internal/handler/datasource.go`: 数据源接口
- `scheduler/internal/handler/query.go`: SQL 查询接口
- `web/src/views/Datasource/`: 前端数据源管理页面
- `web/src/views/SQLQuery/`: 前端 SQL 查询页面

### 核心特性
- 支持 9 种数据源类型：MySQL、SQLite、Hive、Kyuubi、Spark、Trino、StarRocks、Doris、Rqlite
- Driver 接口模式：统一接口，各数据源独立实现
- 连接池管理：按 datasource_id 缓存连接，自动生命周期管理
- UseDatabase 模式：查询前切换数据库，查询后恢复，避免并发污染
- SQL 标准化：自动去除末尾分号，兼容 Hive/Spark/Kyuubi Thrift 协议
- 密码加密存储：AES 加密存储数据源密码
- 数据源权限控制：独立权限体系（read/query/download/update/delete）
- 并发查询控制：单用户和全局并发限制
- 查询结果缓存：Redis 缓存查询结果
- CSV 导出：支持查询结果导出
- 查询历史：自动记录查询历史
- 保存 SQL：常用 SQL 保存和复用
- Rqlite 多节点：支持 single/multi 连接模式
- Trino catalog.schema：兼容 Trino 的 catalog.schema 模型
- 元数据浏览：数据库、表、列信息查看

### Driver 接口

```go
type Driver interface {
    Connect() error
    Close() error
    Ping() error
    GetDatabases() ([]string, error)
    GetTables(database string) ([]string, error)
    GetColumns(database, table string) ([]ColumnInfo, error)
    Query(database, sql string, limit int) (*QueryResult, error)
    UseDatabase(database string) error
}
```

### 查询流程

1. 获取数据源连接
2. 保存当前数据库
3. UseDatabase(targetDB) 切换数据库
4. normalizeSQL() 去除末尾分号
5. 执行 SQL 查询
6. UseDatabase(originalDB) 恢复数据库
7. 返回查询结果

---

## 审计日志系统

### 实现细节

**位置**: 
- `scheduler/internal/middleware/audit.go`: 审计日志中间件
- `scheduler/internal/service/audit_log.go`: 审计日志服务
- `scheduler/internal/handler/audit_log.go`: 审计日志接口
- `scheduler/internal/model/audit_log.go`: 审计日志模型
- `web/src/views/admin/AuditLogs.vue`: 前端审计日志管理页面

### 核心特性
- 全量审计：自动记录所有写操作（POST/PUT/DELETE）
- 中间件+Handler 协作：中间件自动捕获基础信息，Handler 通过 c.Set() 传递业务语义
- 异步写入：goroutine 异步写入审计日志，不阻塞请求响应
- 路由解析规则：精确匹配 → 前缀匹配 → 关键词推断，三级解析
- Handler 埋点覆盖：支持 audit_action/audit_resource/audit_resource_id/audit_resource_name/audit_detail
- 定时清理：每24小时自动清理过期审计日志
- 可配置保留天数：默认90天，通过系统配置可调整
- 仅系统管理员可查看：权限隔离，审计日志仅 system_admin 可访问

### 审计中间件工作流程

```
1. 拦截 POST/PUT/DELETE 请求
2. c.Next() 执行后续处理
3. resolveAuditInfo() 解析 resource 和 action
4. Handler c.Set() 覆盖默认值
5. 从 JWT 上下文获取用户信息
6. 构建审计日志对象
7. goroutine 异步写入数据库
```

### 审计操作类型

| 操作 | 说明 | 触发路径 |
|------|------|---------|
| create | 创建资源 | POST /api/tasks, POST /api/datasources 等 |
| update | 更新资源 | PUT /api/tasks/:id, PUT /api/datasources/:id 等 |
| delete | 删除资源 | DELETE /api/tasks/:id 等 |
| login | 用户登录 | POST /api/auth/login |
| register | 用户注册 | POST /api/auth/register |
| trigger | 触发任务 | POST /api/tasks/:id/trigger |
| assign | 分配角色/权限 | POST /api/admin/users/:id/roles |
| reset_password | 重置密码 | POST /api/admin/users/:id/reset-password |
| online | 执行器上线 | POST /api/executors/:name/online |
| offline | 执行器下线 | POST /api/executors/:name/offline |
| test_connection | 测试连接 | POST /api/datasources/test |
| config_change | 配置变更 | PUT /api/admin/system-config/:key |
| execute | 执行查询 | POST /api/query/execute |
| export | 导出数据 | POST /api/query/export |

---

## 相关文档

- [开发指南](./DEVELOPMENT.md) - 完整的开发、部署、使用指南
- [架构设计](./ARCHITECTURE.md) - 系统架构和技术设计
- [API 文档](./API.md) - RESTful API 接口文档
- [任务日志系统](./LOGGING.md) - 任务日志详细实现
