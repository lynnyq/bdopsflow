# BDopsFlow gRPC 通信协议文档

本文档详细描述了 BDopsFlow 调度平台中调度中心与执行器之间的 gRPC 通信协议定义、消息格式和交互流程。

## 目录

- [协议概述](#协议概述)
- [Proto 定义](#proto-定义)
- [服务接口](#服务接口)
- [消息类型](#消息类型)
- [交互流程](#交互流程)
- [错误处理](#错误处理)
- [实现示例](#实现示例)

---

## 协议概述

### 通信架构

```
┌─────────────────┐                           ┌─────────────────┐
│     Scheduler   │◄─────── gRPC ────────────│    Executor     │
│   (调度中心)    │                           │   (执行器)      │
│                 │                           │                 │
│  - gRPC Server │                           │  - gRPC Client  │
│  - Task CRUD   │                           │  - Task Runner  │
│  - Executor    │                           │  - Pool         │
│    Management  │                           │  - Logger       │
└─────────────────┘                           └─────────────────┘
```

### 协议特点

| 特点 | 说明 |
|------|------|
| 双向流 | 支持 Server Push（任务下发、日志上报） |
| 高效序列化 | 使用 Protocol Buffers |
| 强类型 | 编译时类型检查 |
| 版本兼容 | 支持增量字段扩展 |

### 连接配置

| 参数 | 值 |
|------|-----|
| 端口 | 50051 |
| 协议 | gRPC over HTTP/2 |
| 序列化 | Protocol Buffers v3 |

---

## Proto 定义

### executor.proto

```protobuf
syntax = "proto3";

package bdopsflow;

option go_package = "github.com/lynnyq/bdopsflow/proto";

service ExecutorService {
  // 执行器注册
  rpc Register(RegisterRequest) returns (RegisterResponse);
  
  // 心跳保活
  rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
  
  // 订阅任务
  rpc SubscribeTask(SubscribeTaskRequest) returns (stream Task);
  
  // 上报执行结果
  rpc ReportTaskResult(ReportTaskResultRequest) returns (ReportTaskResultResponse);
  
  // 上报任务日志
  rpc ReportTaskLog(ReportTaskLogRequest) returns (ReportTaskLogResponse);
}
```

---

## 服务接口

### 1. Register - 执行器注册

执行器启动时调用此接口向调度中心注册。

**请求**：`RegisterRequest`

```protobuf
message RegisterRequest {
  string executor_id = 1;  // 执行器唯一标识
  string name = 2;          // 执行器显示名称
  string address = 3;       // 执行器地址
  int32 capacity = 4;       // 最大并发任务数
}
```

**响应**：`RegisterResponse`

```protobuf
message RegisterResponse {
  bool success = 1;         // 注册是否成功
  string message = 2;        // 结果消息
}
```

**字段说明**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| executor_id | string | 是 | 执行器唯一标识，不可重复 |
| name | string | 是 | 执行器显示名称 |
| address | string | 否 | 执行器监听地址（IP:Port） |
| capacity | int32 | 是 | 最大并发任务数 |

**调用示例**：

```go
// Go 实现示例
req := &pb.RegisterRequest{
    ExecutorId: "executor-1",
    Name:       "执行器-1",
    Address:    "192.168.1.100:50051",
    Capacity:   10,
}
resp, err := client.Register(ctx, req)
if err != nil {
    log.Fatalf("注册失败: %v", err)
}
if !resp.Success {
    log.Fatalf("注册被拒绝: %s", resp.Message)
}
```

---

### 2. Heartbeat - 心跳保活

执行器定期调用此接口发送心跳，维持与调度中心的连接。

**请求**：`HeartbeatRequest`

```protobuf
message HeartbeatRequest {
  string executor_id = 1;                    // 执行器 ID
  int32 current_load = 2;                   // 当前负载
  repeated string running_execution_ids = 3; // 运行中的执行 ID 列表
}
```

**响应**：`HeartbeatResponse`

```protobuf
message HeartbeatResponse {
  bool success = 1;           // 心跳是否成功
  string message = 2;         // 结果消息
  int32 target_capacity = 3;  // 目标容量（可能变化）
}
```

**字段说明**：

| 字段 | 类型 | 说明 |
|------|------|------|
| executor_id | string | 执行器唯一标识 |
| current_load | int32 | 当前运行中的任务数 |
| running_execution_ids | string[] | 所有运行中任务的执行 ID 列表 |
| target_capacity | int32 | 调度中心期望的容量（用于动态调整） |

**心跳频率**：

- 默认间隔：10 秒
- 可通过配置调整
- 超过 60 秒无心跳自动标记离线

**调用示例**：

```go
// Go 实现示例
for {
    select {
    case <-ticker.C:
        req := &pb.HeartbeatRequest{
            ExecutorId:          "executor-1",
            CurrentLoad:         int32(executor.GetLoad()),
            RunningExecutionIds: executor.GetRunningExecutionIDs(),
        }
        resp, err := client.Heartbeat(ctx, req)
        if err != nil {
            log.Printf("心跳失败: %v", err)
        }
        if resp.Success {
            // 应用目标容量
            executor.SetCapacity(resp.TargetCapacity)
        }
    }
}
```

---

### 3. SubscribeTask - 订阅任务

执行器调用此接口订阅任务流，调度中心通过此流推送待执行任务。

**请求**：`SubscribeTaskRequest`

```protobuf
message SubscribeTaskRequest {
  string executor_id = 1;  // 执行器 ID
}
```

**响应**：流式 `Task` 消息

```protobuf
message Task {
  int64 task_id = 1;           // 任务 ID
  string execution_id = 2;     // 执行 ID
  string type = 3;             // 任务类型：http、shell
  string config = 4;           // 任务配置（JSON）
  int32 timeout_seconds = 5;   // 超时时间
  int32 retry_count = 6;       // 最大重试次数
  int32 retry_interval = 7;    // 重试间隔（秒）
}
```

**字段说明**：

| 字段 | 类型 | 说明 |
|------|------|------|
| task_id | int64 | 任务数据库 ID |
| execution_id | string | 此次执行的唯一标识 |
| type | string | 任务类型：http、shell |
| config | string | 任务配置 JSON 字符串 |
| timeout_seconds | int32 | 任务超时时间（秒） |
| retry_count | int32 | 最大重试次数 |
| retry_interval | int32 | 失败重试间隔（秒） |

**任务配置示例**：

```json
// HTTP 任务
{
  "url": "https://api.example.com/health",
  "method": "GET",
  "headers": {
    "Authorization": "Bearer token"
  },
  "body": "",
  "timeout": 10000
}

// Shell 任务
{
  "script": "echo 'Hello World' && sleep 1"
}
```

**调用示例**：

```go
// Go 实现示例
stream, err := client.SubscribeTask(ctx, &pb.SubscribeTaskRequest{
    ExecutorId: "executor-1",
})
if err != nil {
    log.Fatalf("订阅任务流失败: %v", err)
}

for {
    task, err := stream.Recv()
    if err == io.EOF {
        log.Println("任务流结束")
        break
    }
    if err != nil {
        log.Printf("接收任务失败: %v", err)
        break
    }
    
    // 执行任务
    go executor.RunTask(task)
}
```

---

### 4. ReportTaskResult - 上报执行结果

任务执行完成后，执行器调用此接口上报执行结果。

**请求**：`ReportTaskResultRequest`

```protobuf
message ReportTaskResultRequest {
  string execution_id = 1;    // 执行 ID
  int64 task_id = 2;          // 任务 ID
  string status = 3;          // 执行状态：success、failed
  string output = 4;           // 执行输出
  string error = 5;            // 错误信息
  int64 start_time = 6;        // 开始时间（Unix 时间戳，毫秒）
  int64 end_time = 7;          // 结束时间（Unix 时间戳，毫秒）
  int32 retry_times = 8;       // 重试次数
}
```

**响应**：`ReportTaskResultResponse`

```protobuf
message ReportTaskResultResponse {
  bool success = 1;   // 是否成功
  string message = 2; // 结果消息
}
```

**字段说明**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| execution_id | string | 是 | 执行 ID（必须与收到的 Task 中一致） |
| task_id | int64 | 是 | 任务 ID |
| status | string | 是 | 执行状态：success、failed |
| output | string | 否 | 执行输出内容 |
| error | string | 否 | 错误信息（失败时必须提供） |
| start_time | int64 | 是 | 开始时间（Unix 毫秒时间戳） |
| end_time | int64 | 是 | 结束时间（Unix 毫秒时间戳） |
| retry_times | int32 | 否 | 此次执行的重试次数 |

**状态值**：

| 状态值 | 说明 |
|--------|------|
| success | 任务成功执行 |
| failed | 任务执行失败 |

**调用示例**：

```go
// Go 实现示例
startTime := time.Now()
output, err := executor.ExecuteTask(task)
endTime := time.Now()

var status string
var errMsg string
if err != nil {
    status = "failed"
    errMsg = err.Error()
} else {
    status = "success"
}

req := &pb.ReportTaskResultRequest{
    ExecutionId: task.ExecutionId,
    TaskId:       task.TaskId,
    Status:       status,
    Output:       output,
    Error:        errMsg,
    StartTime:    startTime.UnixMilli(),
    EndTime:      endTime.UnixMilli(),
    RetryTimes:   0,
}

resp, err := client.ReportTaskResult(ctx, req)
if err != nil {
    log.Printf("上报结果失败: %v", err)
}
```

---

### 5. ReportTaskLog - 上报任务日志

任务执行过程中，执行器调用此接口上报实时日志。

**请求**：`ReportTaskLogRequest`

```protobuf
message ReportTaskLogRequest {
  string execution_id = 1;  // 执行 ID
  int64 task_id = 2;        // 任务 ID
  string log_level = 3;     // 日志级别：info、warn、error
  string log_content = 4;   // 日志内容
  int64 timestamp = 5;      // 日志时间戳（Unix 毫秒）
}
```

**响应**：`ReportTaskLogResponse`

```protobuf
message ReportTaskLogResponse {
  bool success = 1;   // 是否成功
  string message = 2; // 结果消息
}
```

**字段说明**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| execution_id | string | 是 | 执行 ID |
| task_id | int64 | 是 | 任务 ID |
| log_level | string | 是 | 日志级别：info、warn、error |
| log_content | string | 是 | 日志内容 |
| timestamp | int64 | 是 | 日志时间（Unix 毫秒时间戳） |

**日志级别**：

| 级别 | 说明 | 用途 |
|------|------|------|
| info | 信息 | 一般运行信息 |
| warn | 警告 | 异常但不中断 |
| error | 错误 | 执行错误信息 |

**调用示例**：

```go
// Go 实现示例
func (e *Executor) log(executionID string, taskID int64, level, content string) {
    req := &pb.ReportTaskLogRequest{
        ExecutionId: executionID,
        TaskId:      taskID,
        LogLevel:    level,
        LogContent:  content,
        Timestamp:   time.Now().UnixMilli(),
    }
    resp, err := e.client.ReportTaskLog(context.Background(), req)
    if err != nil {
        e.logger.Printf("上报日志失败: %v", err)
    }
    if !resp.Success {
        e.logger.Printf("日志被拒绝: %s", resp.Message)
    }
}

// 使用示例
e.log(task.ExecutionId, task.TaskId, "info", "开始执行 HTTP 请求")
e.log(task.ExecutionId, task.TaskId, "info", "请求发送成功，状态码: 200")
e.log(task.ExecutionId, task.TaskId, "error", "连接超时: context deadline exceeded")
```

---

## 交互流程

### 执行器启动流程

```
Executor                                    Scheduler
   │                                            │
   │  ──── Register ───────────────────────▶   │
   │       executor_id: "executor-1"          │
   │       name: "执行器-1"                    │
   │       capacity: 10                       │
   │                                            │
   │  ◀──── RegisterResponse ───────────────── │
   │       success: true                       │
   │       message: "注册成功"                 │
   │                                            │
   │  ──── SubscribeTask ──────────────────▶   │
   │       executor_id: "executor-1"           │
   │                                            │
   │  ◀──── stream Task (任务推送) ─────────── │
   │       task_id: 1                          │
   │       execution_id: "exec-xxx"            │
   │                                            │
```

### 任务执行流程

```
Executor                                    Scheduler
   │                                            │
   │  ◀──── Task ────────────────────────────  │
   │       task_id: 1                          │
   │       execution_id: "exec-xxx"            │
   │       type: "http"                        │
   │       config: "{...}"                     │
   │                                            │
   │  [执行任务...]                             │
   │                                            │
   │  ──── ReportTaskResult ────────────────▶  │
   │       execution_id: "exec-xxx"            │
   │       status: "success"                   │
   │       output: "{...}"                     │
   │       start_time: 1704067200000          │
   │       end_time: 1704067205000            │
   │                                            │
   │  ◀──── ReportTaskResultResponse ──────── │
   │       success: true                       │
```

### 心跳与锁续期流程

```
Executor                                    Scheduler
   │                                            │
   │  ──── Heartbeat ──────────────────────▶  │
   │       executor_id: "executor-1"           │
   │       current_load: 3                     │
   │       running_execution_ids:              │
   │         - "exec-1"                        │
   │         - "exec-2"                        │
   │         - "exec-3"                        │
   │                                            │
   │  ◀──── HeartbeatResponse ────────────────│
   │       success: true                       │
   │       target_capacity: 10                 │
   │                                            │
   │  [续期锁 TTL]                              │
   │     - task:lock:exec-1 (60s)              │
   │     - task:lock:exec-2 (60s)              │
   │     - task:lock:exec-3 (60s)              │
```

### 完整时序图

```
时间轴
  │
  ▼
  │────────┬────────┬────────┬────────┬────────┬────────┬────────┬────────│
          │        │        │        │        │        │        │        │
          
Executor  │  Register   │Heartbeat│        │Heartbeat│        │Heartbeat│
          │             │        │        │        │        │        │
          ├─────────────┼────────┼────────┼────────┼────────┼────────┤
          
Scheduler │      Register    │Heartbeat│        │Heartbeat│        │Heartbeat
          │      Response    │Response │        │Response │        │Response
          │                   │        │        │        │        │
          │                   │ SubscribeTask    │        │        │
          │                   │ Stream    │        │        │        │
          │                   │     │     │        │        │        │
          │                   │     ▼     │        │        │        │
          │                   │   [任务]  │        │        │        │
          │                   │     │     │        │        │        │
          │                   │     ▼     │        │        │        │
          │                   │  [执行]   │        │        │        │
          │                   │     │     │        │        │        │
          │                   │     ▼     │        │        │        │
          │                   │  [结果]   │        │        │        │
          │                   │     │     │        │        │        │
          │                   │     ▼     │        │        │        │
          │                   │ ReportResult    │        │        │
          │                   │     │     │        │        │        │
```

---

## 错误处理

### 错误码定义

| 错误码 | 说明 | 处理建议 |
|--------|------|----------|
| GRPC_OK | 成功 | - |
| GRPC_CANCELLED | 客户端取消 | 重试 |
| GRPC_UNKNOWN | 未知错误 | 检查日志 |
| GRPC_INVALID_ARGUMENT | 参数无效 | 检查请求参数 |
| GRPC_DEADLINE_EXCEEDED | 超时 | 增加超时时间 |
| GRPC_NOT_FOUND | 资源不存在 | 检查 ID |
| GRPC_ALREADY_EXISTS | 资源已存在 | 使用新 ID |
| GRPC_PERMISSION_DENIED | 权限不足 | 检查权限 |
| GRPC_RESOURCE_EXHAUSTED | 资源耗尽 | 等待或扩容 |
| GRPC_FAILED_PRECONDITION | 前置条件不满足 | 按顺序调用 |

### 常见错误处理

#### 1. 注册失败

```go
resp, err := client.Register(ctx, req)
if err != nil {
    // 网络错误
    log.Printf("注册失败（网络）: %v", err)
    time.Sleep(5 * time.Second)
    goto RETRY
}
if !resp.Success {
    switch resp.Message {
    case "executor already registered":
        // 执行器已注册，可能是重启
        log.Println("执行器已注册，尝试更新")
    case "capacity exceeds limit":
        // 容量超限
        log.Println("容量超限，请减小 capacity")
    default:
        log.Printf("注册被拒绝: %s", resp.Message)
    }
}
```

#### 2. 订阅流中断

```go
stream, err := client.SubscribeTask(ctx, req)
if err != nil {
    log.Printf("订阅失败: %v", err)
    time.Sleep(5 * time.Second)
    goto RETRY
}

for {
    task, err := stream.Recv()
    if err != nil {
        if err == io.EOF {
            log.Println("流正常关闭")
            break
        }
        if status.Code(err) == codes.Unavailable {
            log.Println("连接断开，重新订阅")
            goto RETRY
        }
        log.Printf("接收任务失败: %v", err)
        break
    }
    // 处理任务
    processTask(task)
}
```

#### 3. 结果上报失败

```go
resp, err := client.ReportTaskResult(ctx, req)
if err != nil {
    // 重试上报
    for i := 0; i < 3; i++ {
        time.Sleep(time.Duration(i+1) * time.Second)
        resp, err = client.ReportTaskResult(ctx, req)
        if err == nil && resp.Success {
            log.Println("结果上报成功")
            break
        }
    }
    if err != nil || !resp.Success {
        // 记录本地，待后续补偿
        localStore.Store(req.ExecutionId, req)
    }
}
```

---

## 实现示例

### Go 执行器实现

```go
package main

import (
    "context"
    "log"
    "time"

    pb "github.com/lynnyq/bdopsflow/proto"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

type Executor struct {
    id       string
    name     string
    capacity int32
    client   pb.ExecutorServiceClient
    pool     *Pool // 协程池
}

func NewExecutor(id, name string, capacity int32) *Executor {
    return &Executor{
        id:       id,
        name:     name,
        capacity: capacity,
        pool:     NewPool(int(capacity)),
    }
}

func (e *Executor) Connect(addr string) error {
    conn, err := grpc.Dial(addr, 
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        return err
    }
    e.client = pb.NewExecutorServiceClient(conn)
    return nil
}

func (e *Executor) Register(ctx context.Context) error {
    req := &pb.RegisterRequest{
        ExecutorId: e.id,
        Name:       e.name,
        Address:    "localhost:50052",
        Capacity:   e.capacity,
    }
    resp, err := e.client.Register(ctx, req)
    if err != nil {
        return err
    }
    if !resp.Success {
        return fmt.Errorf("注册失败: %s", resp.Message)
    }
    log.Println("注册成功")
    return nil
}

func (e *Executor) StartHeartbeat(ctx context.Context) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            req := &pb.HeartbeatRequest{
                ExecutorId:          e.id,
                CurrentLoad:         int32(e.pool.Running()),
                RunningExecutionIds: e.pool.GetRunningIDs(),
            }
            resp, err := e.client.Heartbeat(ctx, req)
            if err != nil {
                log.Printf("心跳失败: %v", err)
                continue
            }
            e.capacity = resp.TargetCapacity
            e.pool.SetCapacity(int(e.capacity))
        }
    }
}

func (e *Executor) SubscribeAndExecute(ctx context.Context) error {
    stream, err := e.client.SubscribeTask(ctx, &pb.SubscribeTaskRequest{
        ExecutorId: e.id,
    })
    if err != nil {
        return err
    }

    for {
        task, err := stream.Recv()
        if err != nil {
            return err
        }

        // 提交到协程池执行
        e.pool.Submit(func() {
            e.executeTask(context.Background(), task)
        })
    }
}

func (e *Executor) executeTask(ctx context.Context, task *pb.Task) {
    startTime := time.Now()
    
    // 上报开始日志
    e.reportLog(ctx, task, "info", "开始执行任务")
    
    // 执行任务
    output, err := e.run(task)
    
    // 记录结果
    var status string
    var errMsg string
    if err != nil {
        status = "failed"
        errMsg = err.Error()
        e.reportLog(ctx, task, "error", fmt.Sprintf("执行失败: %s", errMsg))
    } else {
        status = "success"
        e.reportLog(ctx, task, "info", "执行成功")
    }
    
    // 上报结果
    e.reportResult(ctx, task, status, output, errMsg, startTime)
}

func (e *Executor) run(task *pb.Task) (string, error) {
    switch task.Type {
    case "http":
        return e.runHTTP(task)
    case "shell":
        return e.runShell(task)
    default:
        return "", fmt.Errorf("未知任务类型: %s", task.Type)
    }
}

func (e *Executor) reportLog(ctx context.Context, task *pb.Task, level, content string) {
    req := &pb.ReportTaskLogRequest{
        ExecutionId: task.ExecutionId,
        TaskId:      task.TaskId,
        LogLevel:   level,
        LogContent:  content,
        Timestamp:   time.Now().UnixMilli(),
    }
    e.client.ReportTaskLog(ctx, req)
}

func (e *Executor) reportResult(ctx context.Context, task *pb.Task, status, output, errMsg string, startTime time.Time) {
    req := &pb.ReportTaskResultRequest{
        ExecutionId: task.ExecutionId,
        TaskId:      task.TaskId,
        Status:      status,
        Output:      output,
        Error:       errMsg,
        StartTime:   startTime.UnixMilli(),
        EndTime:     time.Now().UnixMilli(),
        RetryTimes:  0,
    }
    e.client.ReportTaskResult(ctx, req)
}

func main() {
    executor := NewExecutor("executor-1", "执行器-1", 10)
    
    ctx := context.Background()
    if err := executor.Connect("localhost:50051"); err != nil {
        log.Fatalf("连接调度中心失败: %v", err)
    }
    
    if err := executor.Register(ctx); err != nil {
        log.Fatalf("注册失败: %v", err)
    }
    
    go executor.StartHeartbeat(ctx)
    
    if err := executor.SubscribeAndExecute(ctx); err != nil {
        log.Fatalf("订阅任务失败: %v", err)
    }
}
```

### Python 执行器实现

```python
import grpc
from concurrent import futures
from typing import List
import_pb2
import_pb2_grpc

class Executor:
    def __init__(self, executor_id: str, name: str, capacity: int):
        self.executor_id = executor_id
        self.name = name
        self.capacity = capacity
        self.stub = None
        self.running_tasks = []
    
    def connect(self, address: str):
        channel = grpc.insecure_channel(address)
        self.stub = import_pb2_grpc.ExecutorServiceStub(channel)
    
    def register(self) -> bool:
        request = import_pb2.RegisterRequest(
            executor_id=self.executor_id,
            name=self.name,
            address="localhost:50052",
            capacity=self.capacity
        )
        response = self.stub.Register(request)
        return response.success
    
    def heartbeat_loop(self):
        while True:
            request = import_pb2.HeartbeatRequest(
                executor_id=self.executor_id,
                current_load=len(self.running_tasks),
                running_execution_ids=list(self.running_tasks)
            )
            try:
                response = self.stub.Heartbeat(request)
                if response.target_capacity != self.capacity:
                    self.capacity = response.target_capacity
            except grpc.RpcError as e:
                print(f"心跳失败: {e}")
            time.sleep(10)
    
    def subscribe_and_execute(self):
        request = import_pb2.SubscribeTaskRequest(
            executor_id=self.executor_id
        )
        try:
            for task in self.stub.SubscribeTask(request):
                threading.Thread(
                    target=self.execute_task,
                    args=(task,)
                ).start()
        except grpc.RpcError as e:
            print(f"订阅任务失败: {e}")
    
    def execute_task(self, task):
        self.running_tasks.append(task.execution_id)
        start_time = time.time()
        
        try:
            # 执行任务
            output, error = self.run_task(task)
            status = "success" if error is None else "failed"
        finally:
            self.running_tasks.remove(task.execution_id)
        
        # 上报结果
        self.report_result(task, status, output, error, start_time)
    
    def run_task(self, task):
        if task.type == "http":
            return self.run_http(task)
        elif task.type == "shell":
            return self.run_shell(task)
        else:
            return "", f"未知任务类型: {task.type}"
    
    def report_result(self, task, status, output, error, start_time):
        request = import_pb2.ReportTaskResultRequest(
            execution_id=task.execution_id,
            task_id=task.task_id,
            status=status,
            output=output,
            error=error or "",
            start_time=int(start_time * 1000),
            end_time=int(time.time() * 1000),
            retry_times=0
        )
        self.stub.ReportTaskResult(request)

if __name__ == "__main__":
    executor = Executor("executor-1", "执行器-1", 10)
    executor.connect("localhost:50051")
    
    if not executor.register():
        print("注册失败")
        sys.exit(1)
    
    print("注册成功")
    
    # 启动心跳
    threading.Thread(target=executor.heartbeat_loop, daemon=True).start()
    
    # 订阅任务
    executor.subscribe_and_execute()
```

---

## 协议扩展

### 添加新接口

1. 在 `executor.proto` 中添加新的 RPC 方法
2. 重新生成代码：`protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative executor.proto`
3. 在调度中心实现接口
4. 在执行器调用接口

### 添加新消息类型

1. 在 `executor.proto` 中定义新消息
2. 重新生成代码
3. 使用新增字段时注意兼容性

### 版本兼容性

| 兼容性规则 | 说明 |
|------------|------|
| 禁止删除字段 | 保持向后兼容 |
| 禁止修改字段编号 | 会破坏序列化 |
| 新增字段使用新编号 | 旧版本忽略新字段 |
| 注释变更安全 | 不影响协议本身 |

---

## 调试工具

### grpcurl

```bash
# 调用 Register
grpcurl -plaintext -d '{
  "executor_id": "test-1",
  "name": "测试执行器",
  "capacity": 5
}' localhost:50051 bdopsflow.ExecutorService/Register

# 调用 Heartbeat
grpcurl -plaintext -d '{
  "executor_id": "test-1",
  "current_load": 2,
  "running_execution_ids": ["exec-1", "exec-2"]
}' localhost:50051 bdopsflow.ExecutorService/Heartbeat

# 列出服务方法
grpcurl -plaintext localhost:50051 list
```

### Evans

```bash
# 启动 Evans REPL
 Evans -p 50051 -plaintext

# 连接后可以交互式调用
package bdopsflow
service ExecutorService
```
