# BDopsFlow 任务日志系统文档

本文档详细描述 BDopsFlow 分布式工作流调度平台的任务日志系统设计、实时传输机制和展示流程。

## 目录

- [日志系统概览](#日志系统概览)
- [任务日志数据模型](#任务日志数据模型)
- [实时日志传输流程](#实时日志传输流程)
- [gRPC 协议定义](#grpc-协议定义)
- [SSE 实时推送机制](#sse-实时推送机制)
- [日志去重机制](#日志去重机制)
- [前端展示实现](#前端展示实现)
- [查询和过滤功能](#查询和过滤功能)
- [故障恢复场景处理](#故障恢复场景处理)
- [最佳实践](#最佳实践)

---

## 日志系统概览

BDopsFlow 采用多层日志架构，实现从执行器到前端的完整实时日志链路：

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        任务执行器 (Executor)                             │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  Shell/HTTP 任务执行                                               │   │
│  │  stdout/stderr 实时捕获                                            │   │
│  │  sendOutputLog() / sendLog()                                       │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                  │                                        │
│                                  ▼                                        │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  MultiClient.ReportLog()                                         │   │
│  │  gRPC 上报到调度中心                                              │   │
│  └──────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                      调度中心 (Scheduler)                               │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  gRPC Server.ReportTaskLog()                                     │   │
│  │  SchedulerService.AddTaskLog()                                    │   │
│  │  存入 bdopsflow_task_logs 表                                       │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                  │                                        │
│                                  ▼                                        │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  StreamLogs API (Gin Handler)                                     │   │
│  │  Server-Sent Events (SSE) 推送                                     │   │
│  │  每 1 秒轮询数据库获取新日志                                       │   │
│  └──────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                        前端 (Vue3 + SSE)                                │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  EventSource 连接 /api/logs/stream                                │   │
│  │  TaskLogViewer 组件实时展示                                        │   │
│  │  支持自动滚动、日志级别过滤                                       │   │
│  └──────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 任务日志数据模型

### 数据库表结构

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_task_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    execution_id TEXT NOT NULL,           -- 执行ID
    task_id INTEGER NOT NULL,             -- 任务ID
    executor_id INTEGER,                  -- 执行器ID
    node_id TEXT,                         -- 节点ID（可选）
    log_level TEXT NOT NULL DEFAULT 'info', -- 日志级别：info/error/warn/debug/stdout/stderr
    message TEXT NOT NULL,                -- 日志内容
    log_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP -- 日志时间
);

-- 创建索引提高查询性能
CREATE INDEX IF NOT EXISTS idx_task_logs_execution_id ON bdopsflow_task_logs(execution_id);
CREATE INDEX IF NOT EXISTS idx_task_logs_task_id ON bdopsflow_task_logs(task_id);
CREATE INDEX IF NOT EXISTS idx_task_logs_log_time ON bdopsflow_task_logs(log_time);
CREATE INDEX IF NOT EXISTS idx_task_logs_log_level ON bdopsflow_task_logs(log_level);
```

### Go 模型定义

```go
type TaskLog struct {
    ID          int64     `json:"id"`
    ExecutionID string    `json:"execution_id"`
    TaskID      int64     `json:"task_id"`
    ExecutorID  *int64    `json:"executor_id,omitempty"`
    NodeID      string    `json:"node_id,omitempty"`
    LogLevel    string    `json:"log_level"`
    Message     string    `json:"message"`
    LogTime     time.Time `json:"log_time"`
}
```

### 日志级别说明

| 日志级别 | 说明 | 使用场景 |
|---------|------|---------|
| info | 一般信息 | 任务开始、完成等里程碑事件 |
| warn | 警告信息 | 不影响执行但需要注意的情况 |
| error | 错误信息 | 任务执行失败、异常退出 |
| debug | 调试信息 | 开发调试用的详细信息 |
| stdout | 标准输出 | Shell 命令的标准输出流 |
| stderr | 标准错误 | Shell 命令的标准错误流 |

---

## 实时日志传输流程

### 1. 执行器端日志捕获和上报

#### Shell 任务日志捕获

在 `executor/internal/executor/task_executor.go` 中实现实时捕获：

```go
func (e *TaskExecutor) executeShell(ctx context.Context, task *pb.Task, client *grpcclient.MultiClient) (string, error) {
    var config struct {
        Script string `json:"script"`
    }
    if err := json.Unmarshal([]byte(task.Config), &config); err != nil {
        return "", fmt.Errorf("invalid shell config: %w", err)
    }

    cmd := exec.CommandContext(ctx, "bash", "-c", config.Script)
    
    // 创建管道捕获实时输出
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return "", fmt.Errorf("create stdout pipe: %w", err)
    }
    stderr, err := cmd.StderrPipe()
    if err != nil {
        return "", fmt.Errorf("create stderr pipe: %w", err)
    }

    var fullOutput, fullError bytes.Buffer
    
    // 异步捕获标准输出
    go func() {
        buf := make([]byte, 1024)
        for {
            n, err := stdout.Read(buf)
            if n > 0 {
                chunk := string(buf[:n])
                fullOutput.WriteString(chunk)
                sendOutputLog(client, task, "stdout", chunk) // 实时上报
            }
            if err != nil {
                break
            }
        }
    }()
    
    // 异步捕获标准错误
    go func() {
        buf := make([]byte, 1024)
        for {
            n, err := stderr.Read(buf)
            if n > 0 {
                chunk := string(buf[:n])
                fullError.WriteString(chunk)
                sendOutputLog(client, task, "stderr", chunk) // 实时上报
            }
            if err != nil {
                break
            }
        }
    }()

    // 启动命令
    if err := cmd.Start(); err != nil {
        return "", fmt.Errorf("start command: %w", err)
    }

    // 等待命令完成
    err = cmd.Wait()
    
    output := fullOutput.String()
    if fullError.Len() > 0 {
        output += "\n[stderr]\n" + fullError.String()
    }

    if err != nil {
        sendLog(client, task, "error", fmt.Sprintf("Shell execution error: %v", err))
        return output, fmt.Errorf("shell execution failed: %w", err)
    }

    sendLog(client, task, "info", fmt.Sprintf("Shell execution completed, output length: %d", len(output)))
    return output, nil
}
```

#### HTTP 任务日志捕获

```go
func (e *TaskExecutor) executeHTTP(ctx context.Context, task *pb.Task, client *grpcclient.MultiClient) (string, error) {
    var config struct {
        URL     string `json:"url"`
        Method  string `json:"method"`
        Body    string `json:"body"`
        Headers string `json:"headers"`
    }
    if err := json.Unmarshal([]byte(task.Config), &config); err != nil {
        return "", fmt.Errorf("invalid http config: %w", err)
    }

    if config.Method == "" {
        config.Method = "GET"
    }

    sendLog(client, task, "info", fmt.Sprintf("Sending HTTP %s request to: %s", config.Method, config.URL))
    
    // ... (执行 HTTP 请求)
    
    sendLog(client, task, "info", fmt.Sprintf("HTTP %s request completed successfully", config.Method))
    return string(bodyBytes), nil
}
```

#### 日志发送函数

```go
func sendOutputLog(client *grpcclient.MultiClient, task *pb.Task, logType string, message string) {
    if client == nil {
        return
    }
    err := client.ReportLog(&pb.ReportTaskLogRequest{
        ExecutionId: task.ExecutionId,
        TaskId:      task.TaskId,
        LogLevel:    logType,
        LogContent:  message,
        Timestamp:   time.Now().Unix(),
    })
    if err != nil {
        slog.Error("failed to report output log", "error", err, "execution_id", task.ExecutionId)
    }
}

func sendLog(client *grpcclient.MultiClient, task *pb.Task, level string, message string) {
    if client == nil {
        return
    }
    err := client.ReportLog(&pb.ReportTaskLogRequest{
        ExecutionId: task.ExecutionId,
        TaskId:      task.TaskId,
        LogLevel:    level,
        LogContent:  message,
        Timestamp:   time.Now().Unix(),
    })
    if err != nil {
        slog.Error("failed to report log", "error", err, "execution_id", task.ExecutionId)
    }
}
```

### 2. gRPC 客户端实现

在 `executor/internal/grpcclient/client.go` 中：

```go
func (c *MultiClient) ReportLog(req *pb.ReportTaskLogRequest) error {
    if err := c.ensureConnected(); err != nil {
        return err
    }

    var client pb.ExecutorServiceClient
    loadedClient := c.client.Load()
    if loadedClient != nil {
        var ok bool
        client, ok = loadedClient.(pb.ExecutorServiceClient)
        if !ok {
            client = nil
        }
    }

    if client == nil {
        return nil
    }

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    _, err := client.ReportTaskLog(ctx, req)
    return err
}
```

### 3. 调度器端接收和存储

在 `scheduler/internal/grpcserver/server.go` 中：

```go
func (s *Server) ReportTaskLog(ctx context.Context, req *pb.ReportTaskLogRequest) (*pb.ReportTaskLogResponse, error) {
    s.scheduler.AddTaskLog(ctx, req.ExecutionId, req.TaskId, "", req.LogLevel, req.LogContent)
    return &pb.ReportTaskLogResponse{
        Success: true,
        Message: "log recorded",
    }, nil
}
```

在 `scheduler/internal/service/scheduler.go` 中存储：

```go
func (s *SchedulerService) AddTaskLog(ctx context.Context, executionID string, taskID int64, nodeID string, logLevel string, message string) error {
    // 首先获取执行记录中的 executor_id
    var executorID interface{} = nil
    execQuery := `SELECT executor_id FROM bdopsflow_task_executions WHERE execution_id = ? LIMIT 1`
    execStmt := rqlite.ParameterizedStatement{
        Query:     execQuery,
        Arguments: []interface{}{executionID},
    }
    execQr, err := s.DB.QueryOneParameterized(execStmt)
    if err == nil && execQr.Err == nil && execQr.Next() {
        row, _ := execQr.Slice()
        rawID := rowInt64(row[0])
        if rawID > 0 {
            executorID = rawID
        }
    }

    // 实现简单的去重机制：避免短时间内相同的日志重复记录
    dedupEnabled := true
    if dedupEnabled && s.redis != nil {
        logHash := fmt.Sprintf("%x", []byte(fmt.Sprintf("%s-%s-%s-%s", executionID, nodeID, logLevel, message)))
        dedupKey := fmt.Sprintf("task:log:dedup:%s", logHash)
        
        exists, _ := s.redis.Exists(ctx, dedupKey).Result()
        if exists > 0 {
            slog.Debug("Skipping duplicate task log", 
                "execution_id", executionID, 
                "log_level", logLevel)
            return nil
        }
        
        s.redis.Set(ctx, dedupKey, "1", 30*time.Second)
    }

    // 尝试插入带 executor_id 的新表结构
    query := `
        INSERT INTO bdopsflow_task_logs (execution_id, task_id, executor_id, node_id, log_level, message, log_time)
        VALUES (?, ?, ?, ?, ?, ?, ?)
    `

    now := time.Now().Format("2006-01-02 15:04:05")
    stmt := rqlite.ParameterizedStatement{
        Query:     query,
        Arguments: []interface{}{executionID, taskID, executorID, nodeID, logLevel, message, now},
    }
    result, err := s.DB.WriteOneParameterized(stmt)
    
    // 如果失败，回退到旧表结构
    if err != nil || result.Err != nil {
        slog.Debug("Falling back to old insert format for bdopsflow_task_logs")
        fallbackQuery := `
            INSERT INTO bdopsflow_task_logs (execution_id, task_id, node_id, log_level, message, log_time)
            VALUES (?, ?, ?, ?, ?, ?)
        `
        fallbackStmt := rqlite.ParameterizedStatement{
            Query:     fallbackQuery,
            Arguments: []interface{}{executionID, taskID, nodeID, logLevel, message, now},
        }
        result, err = s.DB.WriteOneParameterized(fallbackStmt)
        if err != nil {
            return err
        }
        if result.Err != nil {
            return result.Err
        }
    }

    return nil
}
```

---

## gRPC 协议定义

### ReportTaskLog 方法

在 `proto/executor.proto` 中：

```protobuf
message ReportTaskLogRequest {
    string execution_id = 1;           // 执行ID
    int64 task_id = 2;                 // 任务ID
    string log_level = 3;              // 日志级别
    string log_content = 4;            // 日志内容
    int64 timestamp = 5;               // Unix 时间戳（秒）
}

message ReportTaskLogResponse {
    bool success = 1;                  // 是否成功
    string message = 2;                // 响应消息
}

service ExecutorService {
    // ... 其他方法
    rpc ReportTaskLog(ReportTaskLogRequest) returns (ReportTaskLogResponse);
}
```

### 心跳中的任务状态同步（增强功能）

在最新的协议中，心跳请求可以携带运行中任务的详细状态：

```protobuf
message RunningTaskState {
    string execution_id = 1;
    int64 task_id = 2;
    int32 progress = 3;           // 进度 0-100
    string progress_message = 4;  // 进度信息
    int64 start_time = 5;         // 开始时间戳（Unix秒）
    string status = 6;            // 任务状态：running, pending 等
}

message HeartbeatRequest {
    string executor_name = 1;
    int32 current_load = 2;
    repeated string running_execution_ids = 3;
    repeated RunningTaskState running_tasks = 4;  // 新增：详细任务状态
    bool is_reconnect = 5;                        // 新增：是否重连
}

message HeartbeatResponse {
    bool success = 1;
    string message = 2;
    int32 target_capacity = 3;
    bool need_full_sync = 4;        // 新增：是否需要全量同步
    string scheduler_node_id = 5;  // 新增：调度器节点ID
    bool is_new_leader = 6;        // 新增：是否新leader
}
```

---

## SSE 实时推送机制

### 1. API 端点

在 `scheduler/cmd/main.go` 中注册路由：

```go
protected.GET("/logs/stream", taskHandler.StreamLogs)
```

### 2. 处理器实现

在 `scheduler/internal/handler/task.go` 中：

```go
func (h *TaskHandler) StreamLogs(c *gin.Context) {
    executionID := c.Query("execution_id")
    if safeString(executionID) == "" {
        slog.Warn("TaskHandler.StreamLogs: execution_id required")
        BadRequest(c, "execution_id required")
        return
    }

    // 设置 SSE 响应头
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    c.Header("Access-Control-Allow-Origin", "*")

    slog.Info("TaskHandler.StreamLogs: starting stream", "execution_id", executionID)

    ctx := c.Request.Context()
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    var lastLogID int64
    var lastOutputHash uint64
    var lastErrorHash uint64

    for {
        select {
        case <-ctx.Done():
            slog.Debug("TaskHandler.StreamLogs: context cancelled", "execution_id", executionID)
            return
        case <-ticker.C:
            // 获取新日志
            logs, err := h.svc.GetTaskLogs(ctx, executionID)
            if err != nil {
                slog.Warn("TaskHandler.StreamLogs: failed to get logs", 
                    "execution_id", executionID, "error", err)
            } else {
                for _, log := range logs {
                    if log.ID > lastLogID {
                        lastLogID = log.ID
                        data := fmt.Sprintf(`{"id":%d,"execution_id":"%s","task_id":%d,"node_id":"%s","log_level":"%s","message":"%s","log_time":"%s"}`,
                            log.ID, log.ExecutionID, log.TaskID, safeString(log.NodeID), log.LogLevel,
                            escapeJSON(log.Message), log.LogTime.Format(TimeResponseFormat))
                        c.Writer.Write([]byte("data: " + data + "\n\n"))
                        c.Writer.Flush()
                    }
                }
            }

            // 同时获取执行记录更新（output/error/status）
            if len(logs) > 0 {
                taskID := logs[0].TaskID
                executions, execErr := h.svc.GetTaskExecutions(ctx, taskID)
                if execErr != nil {
                    slog.Warn("TaskHandler.StreamLogs: failed to get executions", 
                        "task_id", taskID, "error", execErr)
                } else {
                    for _, exec := range executions {
                        if exec.ExecutionID == executionID {
                            outputHash := fnvHash(exec.Output)
                            errorHash := fnvHash(exec.Error)

                            if outputHash != lastOutputHash || errorHash != lastErrorHash {
                                lastOutputHash = outputHash
                                lastErrorHash = errorHash

                                data, _ := json.Marshal(map[string]interface{}{
                                    "type":       "execution_update",
                                    "status":     exec.Status,
                                    "output":     safeString(exec.Output),
                                    "error":      safeString(exec.Error),
                                    "start_time": safeTimePtr(exec.StartTime.Time),
                                    "end_time":   safeTimePtr(exec.EndTime.Time),
                                })
                                c.Writer.Write([]byte("data: " + string(data) + "\n\n"))
                                c.Writer.Flush()
                            }
                            break
                        }
                    }
                }
            }

            // 发送心跳保持连接
            c.Writer.Write([]byte(": heartbeat\n\n"))
            c.Writer.Flush()
        }
    }
}
```

### 3. 查询日志函数

```go
func (s *SchedulerService) GetTaskLogs(ctx context.Context, executionID string) ([]*model.TaskLog, error) {
    query := `
        SELECT id, execution_id, task_id, executor_id, node_id, log_level, message, log_time
        FROM bdopsflow_task_logs WHERE execution_id = ?
        ORDER BY log_time ASC
    `

    stmt := rqlite.ParameterizedStatement{
        Query:     query,
        Arguments: []interface{}{executionID},
    }
    qr, err := s.DB.QueryOneParameterized(stmt)
    if err != nil {
        return nil, err
    }
    if qr.Err != nil {
        return nil, qr.Err
    }

    var logs []*model.TaskLog
    for qr.Next() {
        tl := &model.TaskLog{}
        if err := scanTaskLogResult(&qr, tl); err != nil {
            return nil, err
        }
        logs = append(logs, tl)
    }

    return logs, nil
}
```

---

## 日志去重机制

### 1. Redis 去重

为防止短时间内重复日志，使用 Redis 进行去重：

```go
func (s *SchedulerService) AddTaskLog(ctx context.Context, executionID string, taskID int64, nodeID string, logLevel string, message string) error {
    // 去重机制
    dedupEnabled := true
    if dedupEnabled && s.redis != nil {
        logHash := fmt.Sprintf("%x", []byte(fmt.Sprintf("%s-%s-%s-%s", executionID, nodeID, logLevel, message)))
        dedupKey := fmt.Sprintf("task:log:dedup:%s", logHash)
        
        exists, _ := s.redis.Exists(ctx, dedupKey).Result()
        if exists > 0 {
            slog.Debug("Skipping duplicate task log", 
                "execution_id", executionID, 
                "log_level", logLevel)
            return nil
        }
        
        s.redis.Set(ctx, dedupKey, "1", 30*time.Second)
    }
    
    // ... 存储日志
}
```

### 2. 恢复事件专用去重

对于调度器切换时的恢复日志，使用独立的去重机制：

```go
func (s *SchedulerService) addRecoveryLogSafe(ctx context.Context, executionID string, taskID int64, logLevel string, message string) error {
    dedupKey := fmt.Sprintf("task:log:dedup:%s:recovery", executionID)
    exists, err := s.redis.Exists(ctx, dedupKey).Result()
    if err == nil && exists > 0 {
        slog.Debug("Skipping duplicate recovery log", "execution_id", executionID)
        return nil
    }
    
    s.redis.Set(ctx, dedupKey, "1", time.Hour) // 1小时内不重复
    
    return s.AddTaskLog(ctx, executionID, taskID, "", logLevel, message)
}
```

---

## 前端展示实现

### 1. Vue 组件：TaskLogViewer

在 `web/src/components/TaskLogViewer.vue` 中：

```vue
<template>
  <div class="task-log-viewer">
    <div class="log-header">
      <h3>执行日志</h3>
      <div class="log-level-filter">
        <button 
          v-for="level in logLevels" 
          :key="level"
          :class="{ active: activeLevels.includes(level) }"
          @click="toggleLevel(level)"
        >
          {{ level.toUpperCase() }}
        </button>
      </div>
    </div>
    
    <div class="status-bar">
      <span :class="currentStatus">{{ currentStatusText }}</span>
      <span v-if="isConnecting" class="connecting">连接中...</span>
    </div>
    
    <div class="log-body" ref="logBodyRef">
      <div 
        v-for="log in filteredLogs" 
        :key="log.id"
        :class="['log-entry', `log-${log.log_level}`]"
      >
        <span class="log-time">{{ formatTime(log.log_time) }}</span>
        <span class="log-level">[{{ log.log_level.toUpperCase() }}]</span>
        <span class="log-message">{{ log.message }}</span>
      </div>
    </div>
    
    <div class="output-section" v-if="realtimeOutput || realtimeError">
      <div v-if="realtimeOutput" class="stdout">
        <h4>标准输出</h4>
        <pre>{{ realtimeOutput }}</pre>
      </div>
      <div v-if="realtimeError" class="stderr">
        <h4>标准错误</h4>
        <pre>{{ realtimeError }}</pre>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'

interface LogEntry {
  id: number
  execution_id: string
  task_id: number
  log_level: string
  message: string
  log_time: string
}

const props = defineProps<{
  executionId: string
  executionStatus?: string
}>()

const logBodyRef = ref<HTMLElement>()
const logs = ref<LogEntry[]>([])
const isConnecting = ref(false)
const eventSource = ref<EventSource | null>(null)
const activeLevels = ref(['info', 'warn', 'error', 'stdout', 'stderr'])
const realtimeOutput = ref('')
const realtimeError = ref('')
const currentStatus = ref(props.executionStatus || 'pending')

const logLevels = ['info', 'warn', 'error', 'stdout', 'stderr', 'debug']

const filteredLogs = computed(() => {
  return logs.value.filter(log => activeLevels.value.includes(log.log_level))
})

const currentStatusText = computed(() => {
  const statusMap: Record<string, string> = {
    pending: '等待中',
    running: '运行中',
    success: '成功',
    failed: '失败',
  }
  return statusMap[currentStatus.value] || currentStatus.value
})

const toggleLevel = (level: string) => {
  const idx = activeLevels.value.indexOf(level)
  if (idx >= 0) {
    activeLevels.value.splice(idx, 1)
  } else {
    activeLevels.value.push(level)
  }
}

const formatTime = (timeStr: string) => {
  const date = new Date(timeStr)
  return date.toLocaleString()
}

const scrollToBottom = () => {
  nextTick(() => {
    if (logBodyRef.value) {
      logBodyRef.value.scrollTop = logBodyRef.value.scrollHeight
    }
  })
}

const loadHistoryLogs = async () => {
  if (!props.executionId) return
  
  try {
    const response = await fetch(`/api/tasks/executions/${props.executionId}/logs`)
    if (response.ok) {
      logs.value = await response.json()
      scrollToBottom()
    }
  } catch (error) {
    console.error('Failed to load history logs:', error)
  }
}

const connectSSE = () => {
  if (!props.executionId) return
  
  isConnecting.value = true
  
  const token = localStorage.getItem('token') || ''
  const url = `/api/logs/stream?execution_id=${props.executionId}&token=${token}`
  
  eventSource.value = new EventSource(url)
  
  eventSource.value.onmessage = (event) => {
    if (event.data.startsWith(': heartbeat')) return
    
    try {
      const data = JSON.parse(event.data)
      
      if (data.type === 'execution_update') {
        // 更新执行状态
        currentStatus.value = data.status
        realtimeOutput.value = data.output
        realtimeError.value = data.error
      } else {
        // 添加新日志
        logs.value.push(data)
        scrollToBottom()
      }
    } catch (error) {
      console.error('Failed to parse SSE message:', error)
    }
  }
  
  eventSource.value.onopen = () => {
    isConnecting.value = false
    console.log('SSE connected')
  }
  
  eventSource.value.onerror = () => {
    console.error('SSE connection error, reconnecting...')
    isConnecting.value = true
    // 5秒后重连
    setTimeout(connectSSE, 5000)
  }
}

onMounted(() => {
  loadHistoryLogs()
  connectSSE()
})

onUnmounted(() => {
  if (eventSource.value) {
    eventSource.value.close()
  }
})

watch(() => props.executionId, () => {
  logs.value = []
  realtimeOutput.value = ''
  realtimeError.value = ''
  currentStatus.value = props.executionStatus || 'pending'
  if (eventSource.value) {
    eventSource.value.close()
  }
  loadHistoryLogs()
  connectSSE()
})
</script>

<style scoped>
.task-log-viewer {
  height: 500px;
  display: flex;
  flex-direction: column;
  border: 1px solid #ddd;
  border-radius: 4px;
}

.log-header {
  padding: 10px;
  border-bottom: 1px solid #ddd;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.log-level-filter button {
  margin: 0 4px;
  padding: 4px 8px;
  border-radius: 4px;
  border: 1px solid #ccc;
  background: white;
  cursor: pointer;
}

.log-level-filter button.active {
  background: #409eff;
  color: white;
  border-color: #409eff;
}

.status-bar {
  padding: 8px 10px;
  background: #f5f5f5;
  font-size: 14px;
}

.status-bar .running { color: #409eff; }
.status-bar .success { color: #67c23a; }
.status-bar .failed { color: #f56c6c; }

.log-body {
  flex: 1;
  overflow-y: auto;
  padding: 10px;
  font-family: monospace;
  font-size: 13px;
}

.log-entry {
  padding: 4px 0;
  border-bottom: 1px solid #f0f0f0;
}

.log-info { color: #606266; }
.log-warn { color: #e6a23c; }
.log-error { color: #f56c6c; }
.log-stdout { color: #409eff; }
.log-stderr { color: #f56c6c; }

.log-time {
  color: #909399;
  margin-right: 10px;
}

.log-message {
  white-space: pre-wrap;
}

.output-section {
  border-top: 1px solid #ddd;
  padding: 10px;
}

.output-section pre {
  background: #f5f5f5;
  padding: 10px;
  border-radius: 4px;
  max-height: 200px;
  overflow: auto;
}
</style>
```

---

## 查询和过滤功能

### 1. 日志查询 API

在 `scheduler/internal/handler/log.go` 中：

```go
func (h *LogHandler) List(c *gin.Context) {
    var req model.LogListRequest
    if err := c.ShouldBindQuery(&req); err != nil {
        BadRequest(c, err.Error())
        return
    }

    ctx := c.Request.Context()
    logs, total, err := h.svc.ListLogs(ctx, req)
    if err != nil {
        slog.Error("LogHandler.List: failed to list logs", "error", err)
        InternalServerError(c, "failed to list logs")
        return
    }

    Success(c, gin.H{
        "items": logs,
        "total": total,
        "page":  req.Page,
        "size":  req.Size,
    })
}

func (h *LogHandler) GetStats(c *gin.Context) {
    var req model.LogListRequest
    if err := c.ShouldBindQuery(&req); err != nil {
        BadRequest(c, err.Error())
        return
    }

    ctx := c.Request.Context()
    stats, err := h.svc.GetLogStats(ctx, req)
    if err != nil {
        slog.Error("LogHandler.GetStats: failed to get stats", "error", err)
        InternalServerError(c, "failed to get stats")
        return
    }

    Success(c, stats)
}
```

### 2. 数据库查询实现

在 `scheduler/internal/service/log_filter_test.go` 等文件中实现高级过滤：

```go
type LogListRequest struct {
    ExecutionID    string `form:"execution_id"`
    TaskID         int64  `form:"task_id"`
    ExecutorName   string `form:"executor_name"`
    TaskName       string `form:"task_name"`
    Status         string `form:"status"`
    StartTimeFrom  string `form:"start_time_from"`
    StartTimeTo    string `form:"start_time_to"`
    EndTimeFrom    string `form:"end_time_from"`
    EndTimeTo      string `form:"end_time_to"`
    DurationMin    int64  `form:"duration_min"`
    DurationMax    int64  `form:"duration_max"`
    Page           int    `form:"page,default=1"`
    Size           int    `form:"size,default=20"`
}
```

---

## 故障恢复场景处理

### 1. 主调度器故障，新调度器接管

当主调度器故障，新调度器成为 leader 时：

1. **执行器端检测到调度器变化**
   - 通过心跳响应中的 `scheduler_node_id` 检测变化
   - 下次心跳时设置 `is_reconnect = true`

2. **新调度器通过心跳获取任务状态**
   - 心跳响应中设置 `need_full_sync = true`
   - 执行器在下次心跳中发送 `running_tasks` 详细状态

3. **任务状态同步**
   - 新调度器更新任务进度
   - 添加恢复日志（使用 `addRecoveryLogSafe` 去重）

4. **任务锁续期**
   - 新调度器重新获取并续期任务锁

相关代码在 `scheduler/internal/service/scheduler.go` 中：

```go
func (s *SchedulerService) RecoverRunningTasksOnBecomeLeader(ctx context.Context) error {
    slog.Info("recovering running tasks on becoming leader")

    query := `
        SELECT execution_id, task_id, executor_id, status, start_time, progress, progress_msg
        FROM bdopsflow_task_executions
        WHERE status = 'running'
    `

    qr, err := s.DB.QueryOne(query)
    if err != nil {
        return err
    }
    if qr.Err != nil {
        return qr.Err
    }

    recoveredCount := 0
    failedCount := 0
    validatedCount := 0

    for qr.Next() {
        row, err := qr.Slice()
        if err != nil {
            continue
        }

        executionID := rowString(row[0])
        taskID := rowInt64(row[1])
        startTimeStr := rowString(row[4])
        progress := int32(rowInt(row[5]))
        progressMsg := rowString(row[6])

        slog.Debug("recovering running task",
            "execution_id", executionID,
            "task_id", taskID)

        // 检查执行器是否还在线且心跳正常
        executor, err := s.GetExecutorByID(ctx, rowInt64(row[2]))
        executorOnline := err == nil && executor.Status == "online"

        // 检查任务锁是否还存在
        lockKey := fmt.Sprintf("task:lock:%s", executionID)
        lockExists, _ := s.redis.Exists(ctx, lockKey).Result()

        // 检查任务是否已经超时
        taskTimeout := false
        if startTimeStr != "" {
            if startTime, err := time.Parse("2006-01-02 15:04:05", startTimeStr); err == nil {
                if time.Since(startTime) > 2*time.Hour {
                    taskTimeout = true
                }
            }
        }

        // 如果执行器离线、锁不存在，或者任务超时，标记任务失败
        if !executorOnline || lockExists == 0 || taskTimeout {
            var reason string
            if !executorOnline {
                reason = "scheduler failover: executor is offline"
            } else if lockExists == 0 {
                reason = "scheduler failover: task lock not found"
            } else {
                reason = "scheduler failover: task execution timeout"
            }
            
            s.forceFailTask(ctx, executionID, taskID, reason)
            failedCount++
            continue
        }

        // 任务看起来还在正常运行，更新任务锁和相关状态
        lockTTL := 300
        if err := s.redis.Set(ctx, lockKey, "leader_recovered", time.Duration(lockTTL)*time.Second).Err(); err != nil {
            slog.Warn("failed to set task lock during recovery", "execution_id", executionID, "error", err)
        }

        renewKey := fmt.Sprintf("task:renew:%s", executionID)
        if err := s.redis.Set(ctx, renewKey, time.Now().Unix(), time.Duration(lockTTL)*time.Second).Err(); err != nil {
            slog.Warn("failed to set task renew timestamp during recovery", "execution_id", executionID, "error", err)
        }

        failCountKey := fmt.Sprintf("task:renew:fail:count:%s", executionID)
        s.redis.Del(ctx, failCountKey)

        // 记录恢复事件
        s.addRecoveryLogSafe(ctx, executionID, taskID, "info", 
            fmt.Sprintf("Task recovered by new leader, progress: %d%%, message: %s", progress, progressMsg))

        recoveredCount++
        validatedCount++
    }

    slog.Info("finished recovering running tasks",
        "recovered_count", recoveredCount,
        "failed_count", failedCount,
        "validated_count", validatedCount)
    return nil
}
```

### 2. gRPC 服务端增强

在 `scheduler/internal/grpcserver/server.go` 中：

```go
func (s *Server) SetNodeId(nodeID string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.nodeID = nodeID
}

func (s *Server) MarkAsNewLeader() {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.isNewLeader = true
    // 标记所有连接的执行器需要同步
    for name := range s.executors {
        s.needExecSync[name] = true
    }
}

func (s *Server) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
    // ... 原有逻辑
    
    // 处理详细任务状态
    if len(req.RunningTasks) > 0 {
        slog.Debug("received detailed running tasks from executor",
            "executor_name", req.ExecutorName,
            "task_count", len(req.RunningTasks))
        
        execIds := make([]string, 0, len(req.RunningTasks))
        for _, task := range req.RunningTasks {
            execIds = append(execIds, task.ExecutionId)
            s.scheduler.UpdateTaskProgress(ctx, task.ExecutionId, task.Progress, task.ProgressMessage)
        }
        
        err := s.scheduler.UpdateExecutorHeartbeatWithRunningTasks(ctx, req.ExecutorName, req.CurrentLoad, execIds)
        if err != nil {
            slog.Warn("failed to update executor heartbeat", "executor_name", req.ExecutorName, "error", err)
        }
    }
    
    s.mu.Lock()
    nodeID := s.nodeID
    isNewLeader := s.isNewLeader
    needFullSync := s.needExecSync[req.ExecutorName]
    if needFullSync {
        delete(s.needExecSync, req.ExecutorName)
    }
    s.mu.Unlock()
    
    return &pb.HeartbeatResponse{
        Success:        true,
        Message:        "ok",
        TargetCapacity: targetCapacity,
        NeedFullSync:   needFullSync || (req.IsReconnect && isNewLeader),
        SchedulerNodeId: nodeID,
        IsNewLeader:    isNewLeader,
    }, nil
}
```

---

## 最佳实践

### 1. 日志级别使用

- **stdout/stderr**：仅用于 Shell 命令的实际输出流
- **info**：任务开始、完成、重要进度里程碑
- **warn**：非致命错误、重试、超时警告
- **error**：任务失败、严重错误
- **debug**：开发调试信息，生产环境可过滤

### 2. 性能优化

- 使用索引优化查询性能
- 定期清理旧日志（建议保留 30-90 天）
- 避免在日志中记录敏感信息
- 使用批量插入（如果需要高吞吐）

### 3. 监控和告警

```sql
-- 统计失败任务数
SELECT COUNT(*) FROM bdopsflow_task_executions WHERE status = 'failed' AND created_at > NOW() - INTERVAL 1 HOUR;

-- 统计执行器错误日志
SELECT executor_id, COUNT(*) as error_count 
FROM bdopsflow_task_logs 
WHERE log_level = 'error' 
AND log_time > NOW() - INTERVAL 1 HOUR 
GROUP BY executor_id;
```

### 4. 容量规划

- rqlite 磁盘占用：每个日志记录约 200-500 字节
- 1 万次任务执行，每次 100 条日志：约 2-5 GB
- Redis 去重键：每个键约 100 字节，TTL 30秒，内存占用可控

---

## 附录：完整流程时序图

```
┌──────────┐          ┌──────────┐          ┌──────────┐          ┌──────────┐
│  执行器   │          │gRPC客户  │          │调度中心  │          │  前端    │
└────┬─────┘          └────┬─────┘          └────┬─────┘          └────┬─────┘
     │                     │                     │                     │
     │  Shell 执行输出     │                     │                     │
     │────────────────────>│                     │                     │
     │  chunk (1KB)        │                     │                     │
     │                     │  ReportTaskLog      │                     │
     │                     │────────────────────>│                     │
     │                     │                     │  Redis去重检查      │
     │                     │                     │───────────┐         │
     │                     │                     │           │         │
     │                     │                     │<──────────┘         │
     │                     │                     │  插入数据库          │
     │                     │                     │───────────┐         │
     │                     │                     │           │         │
     │                     │                     │<──────────┘         │
     │                     │<────────────────────│                     │
     │                     │   ACK               │                     │
     │                     │                     │                     │
     │                     │                     │  SSE 轮询（1秒）     │
     │                     │                     │<────────────────────│
     │                     │                     │  查询新日志          │
     │                     │                     │───────────┐         │
     │                     │                     │           │         │
     │                     │                     │<──────────┘         │
     │                     │                     │                     │
     │                     │                     │  SSE 推送新日志      │
     │                     │                     │────────────────────>│
     │                     │                     │  data: {...}        │
     │                     │                     │                     │  解析并展示
     │                     │                     │                     │──────────┐
     │                     │                     │                     │          │
     │                     │                     │                     │<─────────┘
     │                     │                     │                     │
     │                     │                     │  : heartbeat         │
     │                     │                     │────────────────────>│
```

---

## 相关文档

- [部署文档](DEPLOYMENT.md) - 详细的部署指南
- [架构文档](ARCHITECTURE.md) - 系统架构设计
- [gRPC 文档](GRPC.md) - gRPC 协议详解
- [API 文档](API.md) - RESTful API 接口
