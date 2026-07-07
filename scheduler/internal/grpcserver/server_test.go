package grpcserver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	pb "github.com/lynnyq/bdopsflow/proto"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	"github.com/redis/go-redis/v9"
	rqlite "github.com/rqlite/gorqlite"
	"google.golang.org/grpc/metadata"
)

// errorDB 是 database.DB 的 mock 实现，所有方法都返回错误。
// 用于测试 grpcserver 方法在 DB 异常时的错误处理路径。
type errorDB struct{}

func (m *errorDB) QueryOne(query string) (rqlite.QueryResult, error) {
	return rqlite.QueryResult{}, errors.New("db error")
}

func (m *errorDB) QueryOneParameterized(stmt rqlite.ParameterizedStatement) (rqlite.QueryResult, error) {
	return rqlite.QueryResult{}, errors.New("db error")
}

func (m *errorDB) WriteOneParameterized(stmt rqlite.ParameterizedStatement) (rqlite.WriteResult, error) {
	return rqlite.WriteResult{}, errors.New("db error")
}

func (m *errorDB) WriteParameterized(stmts []rqlite.ParameterizedStatement) ([]rqlite.WriteResult, error) {
	return nil, errors.New("db error")
}

// 编译期接口检查
var _ database.DB = (*errorDB)(nil)

// newTestServerWithErrorDB 创建使用 errorDB 和 miniredis 的 Server，用于测试 DB 异常路径。
func newTestServerWithErrorDB(t *testing.T) *Server {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	scheduler := service.NewSchedulerService(&errorDB{}, rdb)
	return NewServer("0", scheduler)
}

// mockSubscribeStream 实现 pb.ExecutorService_SubscribeTaskServer 接口用于测试
type mockSubscribeStream struct {
	ctx    context.Context
	sendFn func(*pb.Task) error
}

func (m *mockSubscribeStream) Send(task *pb.Task) error {
	if m.sendFn != nil {
		return m.sendFn(task)
	}
	return nil
}

func (m *mockSubscribeStream) SetHeader(metadata.MD) error  { return nil }
func (m *mockSubscribeStream) SendHeader(metadata.MD) error { return nil }
func (m *mockSubscribeStream) SetTrailer(metadata.MD)       {}
func (m *mockSubscribeStream) Context() context.Context     { return m.ctx }
func (m *mockSubscribeStream) SendMsg(interface{}) error    { return nil }
func (m *mockSubscribeStream) RecvMsg(interface{}) error    { return nil }

// newTestServer 创建用于测试的 Server 实例，使用 nil DB 的 SchedulerService。
// 仅适用于不依赖 DB 操作的测试。
func newTestServer(t *testing.T) *Server {
	t.Helper()
	scheduler := service.NewSchedulerService(nil, nil)
	return NewServer("0", scheduler)
}

// ------------------------------------------------------------
// NewServer
// ------------------------------------------------------------

func TestNewServer(t *testing.T) {
	scheduler := service.NewSchedulerService(nil, nil)
	srv := NewServer("50051", scheduler)

	if srv == nil {
		t.Fatal("NewServer should return non-nil server")
	}
	if srv.port != "50051" {
		t.Errorf("port = %v, want 50051", srv.port)
	}
	if srv.scheduler == nil {
		t.Error("scheduler should not be nil")
	}
	if srv.executors == nil {
		t.Error("executors map should be initialized")
	}
	if srv.needExecSync == nil {
		t.Error("needExecSync map should be initialized")
	}
	if srv.cancelExecIds == nil {
		t.Error("cancelExecIds map should be initialized")
	}
}

func TestNewServer_EmptyPort(t *testing.T) {
	scheduler := service.NewSchedulerService(nil, nil)
	srv := NewServer("", scheduler)

	if srv.port != "" {
		t.Errorf("port = %v, want empty string", srv.port)
	}
}

// ------------------------------------------------------------
// SetNodeId / SetLeader
// ------------------------------------------------------------

func TestSetNodeId(t *testing.T) {
	srv := newTestServer(t)

	srv.SetNodeId("node-1")
	srv.mu.RLock()
	nodeId := srv.nodeId
	srv.mu.RUnlock()
	if nodeId != "node-1" {
		t.Errorf("nodeId = %v, want node-1", nodeId)
	}
}

func TestSetLeader(t *testing.T) {
	srv := newTestServer(t)

	srv.SetLeader(true)
	srv.mu.RLock()
	isLeader := srv.isLeader
	srv.mu.RUnlock()
	if !isLeader {
		t.Error("isLeader should be true")
	}

	srv.SetLeader(false)
	srv.mu.RLock()
	isLeader = srv.isLeader
	srv.mu.RUnlock()
	if isLeader {
		t.Error("isLeader should be false")
	}
}

// ------------------------------------------------------------
// AddCancelExecutionId
// ------------------------------------------------------------

func TestAddCancelExecutionId(t *testing.T) {
	srv := newTestServer(t)

	// 添加多个取消 ID
	srv.AddCancelExecutionId("exec-1", "task-a")
	srv.AddCancelExecutionId("exec-1", "task-b")
	srv.AddCancelExecutionId("exec-2", "task-c")

	srv.mu.RLock()
	defer srv.mu.RUnlock()

	if len(srv.cancelExecIds["exec-1"]) != 2 {
		t.Errorf("exec-1 should have 2 cancel IDs, got %d", len(srv.cancelExecIds["exec-1"]))
	}
	if len(srv.cancelExecIds["exec-2"]) != 1 {
		t.Errorf("exec-2 should have 1 cancel ID, got %d", len(srv.cancelExecIds["exec-2"]))
	}

	// 验证具体值
	if srv.cancelExecIds["exec-1"][0] != "task-a" {
		t.Errorf("first cancel ID = %v, want task-a", srv.cancelExecIds["exec-1"][0])
	}
	if srv.cancelExecIds["exec-1"][1] != "task-b" {
		t.Errorf("second cancel ID = %v, want task-b", srv.cancelExecIds["exec-1"][1])
	}
}

// ------------------------------------------------------------
// MarkAsNewLeader
// ------------------------------------------------------------

func TestMarkAsNewLeader(t *testing.T) {
	srv := newTestServer(t)

	srv.MarkAsNewLeader()

	srv.mu.RLock()
	isNewLeader := srv.isNewLeader
	isLeader := srv.isLeader
	srv.mu.RUnlock()

	if !isNewLeader {
		t.Error("isNewLeader should be true after MarkAsNewLeader")
	}
	if !isLeader {
		t.Error("isLeader should be true after MarkAsNewLeader")
	}

	// 等待后台 goroutine 重置 isNewLeader 标志（30秒超时）
	// 这里只验证标志在调用后立即为 true，不等待重置
}

func TestMarkAsNewLeader_MarksExistingExecutorsForSync(t *testing.T) {
	srv := newTestServer(t)

	// 先添加一个已连接的执行器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &mockSubscribeStream{ctx: ctx}
	srv.executors["test-exec"] = &executorConn{stream: stream}

	srv.MarkAsNewLeader()

	srv.mu.RLock()
	needSync := srv.needExecSync["test-exec"]
	srv.mu.RUnlock()

	if !needSync {
		t.Error("existing executor should be marked for sync after MarkAsNewLeader")
	}
}

// ------------------------------------------------------------
// dispatchTask
// ------------------------------------------------------------

func TestDispatchTask_ExecutorNotConnected(t *testing.T) {
	srv := newTestServer(t)

	err := srv.dispatchTask("nonexistent-exec", &pb.Task{
		TaskId:      1,
		ExecutionId: "exec-1",
	})
	if err == nil {
		t.Error("dispatchTask should fail when executor is not connected")
	}
}

func TestDispatchTask_Success(t *testing.T) {
	srv := newTestServer(t)

	// 使用可取消的 context 模拟连接
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sentTasks := make(chan *pb.Task, 1)
	stream := &mockSubscribeStream{
		ctx: ctx,
		sendFn: func(task *pb.Task) error {
			sentTasks <- task
			return nil
		},
	}
	srv.executors["test-exec"] = &executorConn{stream: stream}

	task := &pb.Task{
		TaskId:      42,
		ExecutionId: "exec-42",
		Type:        "http",
	}
	err := srv.dispatchTask("test-exec", task)
	if err != nil {
		t.Errorf("dispatchTask failed: %v", err)
	}

	select {
	case sent := <-sentTasks:
		if sent.TaskId != 42 {
			t.Errorf("sent task ID = %v, want 42", sent.TaskId)
		}
		if sent.ExecutionId != "exec-42" {
			t.Errorf("sent execution ID = %v, want exec-42", sent.ExecutionId)
		}
	case <-time.After(1 * time.Second):
		t.Error("task was not sent within timeout")
	}
}

func TestDispatchTask_SendError(t *testing.T) {
	srv := newTestServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sendErr := errors.New("send failed")
	stream := &mockSubscribeStream{
		ctx:    ctx,
		sendFn: func(task *pb.Task) error { return sendErr },
	}
	srv.executors["test-exec"] = &executorConn{stream: stream}

	err := srv.dispatchTask("test-exec", &pb.Task{TaskId: 1})
	if err == nil {
		t.Error("dispatchTask should fail when Send returns error")
	}
	if !errors.Is(err, sendErr) {
		t.Errorf("dispatchTask error = %v, want %v", err, sendErr)
	}
}

// ------------------------------------------------------------
// Heartbeat (with empty ExecutorName - doesn't need DB)
// ------------------------------------------------------------

func TestHeartbeat_EmptyExecutorName(t *testing.T) {
	srv := newTestServer(t)
	srv.SetNodeId("test-node")
	srv.SetLeader(true)

	resp, err := srv.Heartbeat(context.Background(), &pb.HeartbeatRequest{
		ExecutorName: "",
	})
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}
	if !resp.Success {
		t.Error("response should be successful")
	}
	if resp.SchedulerNodeId != "test-node" {
		t.Errorf("SchedulerNodeId = %v, want test-node", resp.SchedulerNodeId)
	}
	if !resp.IsLeader {
		t.Error("IsLeader should be true")
	}
}

func TestHeartbeat_EmptyExecutorName_NotLeader(t *testing.T) {
	srv := newTestServer(t)
	srv.SetNodeId("node-2")
	srv.SetLeader(false)

	resp, err := srv.Heartbeat(context.Background(), &pb.HeartbeatRequest{
		ExecutorName: "",
	})
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}
	if resp.IsLeader {
		t.Error("IsLeader should be false")
	}
}

// ------------------------------------------------------------
// SyncRunningTasks
// ------------------------------------------------------------

func TestSyncRunningTasks(t *testing.T) {
	srv := newTestServer(t)

	resp, err := srv.SyncRunningTasks(context.Background(), &pb.SyncRunningTasksRequest{
		ExecutorName: "test-exec",
	})
	if err != nil {
		t.Fatalf("SyncRunningTasks failed: %v", err)
	}
	if !resp.Success {
		t.Error("response should be successful")
	}

	// 验证执行器被标记为需要同步
	srv.mu.RLock()
	needSync := srv.needExecSync["test-exec"]
	srv.mu.RUnlock()
	if !needSync {
		t.Error("executor should be marked for sync")
	}
}

// ------------------------------------------------------------
// IsExecutorConnected
// ------------------------------------------------------------

func TestIsExecutorConnected_NotConnected(t *testing.T) {
	srv := newTestServer(t)

	if srv.IsExecutorConnected("nonexistent") {
		t.Error("IsExecutorConnected should return false for non-existent executor")
	}
}

func TestIsExecutorConnected_Connected(t *testing.T) {
	srv := newTestServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &mockSubscribeStream{ctx: ctx}
	srv.executors["test-exec"] = &executorConn{stream: stream}

	if !srv.IsExecutorConnected("test-exec") {
		t.Error("IsExecutorConnected should return true for connected executor")
	}
}

// ------------------------------------------------------------
// GetConnectedExecutorNames
// ------------------------------------------------------------

func TestGetConnectedExecutorNames_Empty(t *testing.T) {
	srv := newTestServer(t)

	names := srv.GetConnectedExecutorNames()
	if len(names) != 0 {
		t.Errorf("expected 0 names, got %d", len(names))
	}
}

func TestGetConnectedExecutorNames_Multiple(t *testing.T) {
	srv := newTestServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &mockSubscribeStream{ctx: ctx}

	srv.executors["exec-1"] = &executorConn{stream: stream}
	srv.executors["exec-2"] = &executorConn{stream: stream}
	srv.executors["exec-3"] = &executorConn{stream: stream}

	names := srv.GetConnectedExecutorNames()
	if len(names) != 3 {
		t.Errorf("expected 3 names, got %d", len(names))
	}

	// 验证所有执行器名称都在列表中
	nameMap := make(map[string]bool)
	for _, n := range names {
		nameMap[n] = true
	}
	for _, expected := range []string{"exec-1", "exec-2", "exec-3"} {
		if !nameMap[expected] {
			t.Errorf("expected %s in names, got %v", expected, names)
		}
	}
}

// ------------------------------------------------------------
// SubscribeTask
// ------------------------------------------------------------

func TestSubscribeTask_RegisterAndDisconnect(t *testing.T) {
	srv := newTestServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	stream := &mockSubscribeStream{ctx: ctx}

	// 在单独的 goroutine 中运行 SubscribeTask
	done := make(chan struct{})
	go func() {
		_ = srv.SubscribeTask(&pb.SubscribeTaskRequest{
			ExecutorName: "test-exec",
		}, stream)
		close(done)
	}()

	// 等待一小段时间让执行器注册
	time.Sleep(50 * time.Millisecond)

	// 验证执行器已注册
	if !srv.IsExecutorConnected("test-exec") {
		t.Error("executor should be connected after SubscribeTask")
	}

	// 取消 context 模拟断开连接
	cancel()

	// 等待 SubscribeTask 返回
	select {
	case <-done:
		// 预期：SubscribeTask 返回
	case <-time.After(1 * time.Second):
		t.Fatal("SubscribeTask should return after context cancel")
	}

	// 验证执行器已注销
	if srv.IsExecutorConnected("test-exec") {
		t.Error("executor should be disconnected after context cancel")
	}
}

// ------------------------------------------------------------
// Stop
// ------------------------------------------------------------

func TestStop_NilGrpcServer(t *testing.T) {
	srv := newTestServer(t)

	// grpcServer 为 nil 时，Stop 不应 panic
	srv.Stop()
}

// ------------------------------------------------------------
// Heartbeat 取消任务 ID 测试
// ------------------------------------------------------------

func TestHeartbeat_ReturnsCancelExecutionIds(t *testing.T) {
	srv := newTestServer(t)
	srv.SetNodeId("test-node")

	// 预设取消任务 ID
	srv.AddCancelExecutionId("exec-1", "task-to-cancel-1")
	srv.AddCancelExecutionId("exec-1", "task-to-cancel-2")

	// 注意：带非空 ExecutorName 的 Heartbeat 会调用 scheduler 方法需要 DB
	// 这里只测试空 ExecutorName 的路径
	resp, err := srv.Heartbeat(context.Background(), &pb.HeartbeatRequest{
		ExecutorName: "",
	})
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}
	if !resp.Success {
		t.Error("response should be successful")
	}
	// 空 ExecutorName 时不会返回 cancel IDs
	if len(resp.CancelExecutionIds) != 0 {
		t.Errorf("CancelExecutionIds should be empty for empty executor name, got %v", resp.CancelExecutionIds)
	}
}

// ------------------------------------------------------------
// 并发安全测试
// ------------------------------------------------------------

func TestServer_ConcurrentAccess(t *testing.T) {
	srv := newTestServer(t)

	done := make(chan struct{})

	// 并发添加取消 ID
	for i := 0; i < 10; i++ {
		go func(i int) {
			defer func() { done <- struct{}{} }()
			srv.AddCancelExecutionId("exec-1", "task-"+string(rune('A'+i)))
		}(i)
	}

	// 并发读取已连接执行器
	for i := 0; i < 5; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			_ = srv.GetConnectedExecutorNames()
		}()
	}

	// 并发检查连接状态
	for i := 0; i < 5; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			_ = srv.IsExecutorConnected("exec-1")
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 20; i++ {
		<-done
	}
}

// ------------------------------------------------------------
// 使用 errorDB 的 DB 异常路径测试
// ------------------------------------------------------------

// TestRegister_DBError 测试 Register 在 DB 异常时返回失败响应
func TestRegister_DBError(t *testing.T) {
	srv := newTestServerWithErrorDB(t)

	resp, err := srv.Register(context.Background(), &pb.RegisterRequest{
		Name:     "test-executor",
		Address:  "localhost:50052",
		Capacity: 10,
	})
	if err != nil {
		t.Fatalf("Register should not return gRPC error, got: %v", err)
	}
	if resp.Success {
		t.Error("Register should return Success=false when DB fails")
	}
	if resp.Message == "" {
		t.Error("Message should contain error description")
	}
	if resp.Duplicate {
		t.Error("Duplicate should be false for generic DB error")
	}
}

// TestHeartbeat_WithExecutorName_DBError 测试带执行器名的心跳在 DB 异常时仍返回响应
func TestHeartbeat_WithExecutorName_DBError(t *testing.T) {
	srv := newTestServerWithErrorDB(t)
	srv.SetNodeId("test-node")

	resp, err := srv.Heartbeat(context.Background(), &pb.HeartbeatRequest{
		ExecutorName:        "test-executor",
		CurrentLoad:         5,
		RunningExecutionIds: []string{"exec-1", "exec-2"},
	})
	if err != nil {
		t.Fatalf("Heartbeat should not return gRPC error, got: %v", err)
	}
	// 即使 DB 异常，Heartbeat 仍应返回成功响应
	if !resp.Success {
		t.Error("Heartbeat should return Success=true even with DB errors")
	}
	if resp.SchedulerNodeId != "test-node" {
		t.Errorf("SchedulerNodeId = %v, want test-node", resp.SchedulerNodeId)
	}
}

// TestHeartbeat_WithRunningTasks_DBError 测试带运行任务详情的心跳在 DB 异常时的行为
func TestHeartbeat_WithRunningTasks_DBError(t *testing.T) {
	srv := newTestServerWithErrorDB(t)

	resp, err := srv.Heartbeat(context.Background(), &pb.HeartbeatRequest{
		ExecutorName: "test-executor",
		CurrentLoad:  3,
		RunningTasks: []*pb.RunningTaskState{
			{ExecutionId: "exec-1", Progress: 50},
			{ExecutionId: "exec-2", Progress: 80},
		},
	})
	if err != nil {
		t.Fatalf("Heartbeat should not return gRPC error, got: %v", err)
	}
	if !resp.Success {
		t.Error("Heartbeat should return Success=true")
	}
}

// TestHeartbeat_IsReconnectAndNewLeader 测试重连且新 leader 时的心跳响应
func TestHeartbeat_IsReconnectAndNewLeader(t *testing.T) {
	srv := newTestServerWithErrorDB(t)
	srv.SetNodeId("node-1")

	// 标记为新 leader
	srv.MarkAsNewLeader()

	resp, err := srv.Heartbeat(context.Background(), &pb.HeartbeatRequest{
		ExecutorName: "test-executor",
		IsReconnect:  true,
	})
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}
	if !resp.Success {
		t.Error("Heartbeat should return Success=true")
	}
	// IsReconnect && isNewLeader 时 NeedFullSync 应为 true
	if !resp.NeedFullSync {
		t.Error("NeedFullSync should be true when IsReconnect && isNewLeader")
	}
	if !resp.IsNewLeader {
		t.Error("IsNewLeader should be true")
	}
}

// TestHeartbeat_CancelExecutionIdsReturned 测试心跳返回取消任务 ID
func TestHeartbeat_CancelExecutionIdsReturned(t *testing.T) {
	srv := newTestServerWithErrorDB(t)
	srv.SetNodeId("test-node")

	// 预设取消任务 ID
	srv.AddCancelExecutionId("test-executor", "cancel-1")
	srv.AddCancelExecutionId("test-executor", "cancel-2")

	resp, err := srv.Heartbeat(context.Background(), &pb.HeartbeatRequest{
		ExecutorName: "test-executor",
	})
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}
	if len(resp.CancelExecutionIds) != 2 {
		t.Errorf("CancelExecutionIds len = %v, want 2", len(resp.CancelExecutionIds))
	}

	// 第二次心跳不应再返回取消 ID（已被清除）
	resp2, err := srv.Heartbeat(context.Background(), &pb.HeartbeatRequest{
		ExecutorName: "test-executor",
	})
	if err != nil {
		t.Fatalf("second Heartbeat failed: %v", err)
	}
	if len(resp2.CancelExecutionIds) != 0 {
		t.Errorf("second heartbeat CancelExecutionIds should be empty, got %v", resp2.CancelExecutionIds)
	}
}

// TestReportTaskLog_DBError 测试 ReportTaskLog 在 DB 异常时仍返回成功
func TestReportTaskLog_DBError(t *testing.T) {
	srv := newTestServerWithErrorDB(t)

	resp, err := srv.ReportTaskLog(context.Background(), &pb.ReportTaskLogRequest{
		TaskId:      1,
		ExecutionId: "exec-1",
		LogLevel:    "info",
		LogContent:  "test log",
	})
	if err != nil {
		t.Fatalf("ReportTaskLog should not return gRPC error, got: %v", err)
	}
	if !resp.Success {
		t.Error("ReportTaskLog should return Success=true")
	}
}

// TestReportTaskProgress_DBError 测试 ReportTaskProgress 在 DB 异常时仍返回成功
func TestReportTaskProgress_DBError(t *testing.T) {
	srv := newTestServerWithErrorDB(t)

	resp, err := srv.ReportTaskProgress(context.Background(), &pb.ReportTaskProgressRequest{
		TaskId:      1,
		ExecutionId: "exec-1",
		Progress:    75,
		Message:     "in progress",
	})
	if err != nil {
		t.Fatalf("ReportTaskProgress should not return gRPC error, got: %v", err)
	}
	if !resp.Success {
		t.Error("ReportTaskProgress should return Success=true")
	}
}

// TestReportTaskResult_SuccessStatus 测试 ReportTaskResult 成功状态
func TestReportTaskResult_SuccessStatus(t *testing.T) {
	srv := newTestServerWithErrorDB(t)

	resp, err := srv.ReportTaskResult(context.Background(), &pb.ReportTaskResultRequest{
		TaskId:      1,
		ExecutionId: "exec-1",
		Status:      "success",
		Output:      "task completed",
	})
	if err != nil {
		t.Fatalf("ReportTaskResult should not return gRPC error, got: %v", err)
	}
	if !resp.Success {
		t.Error("ReportTaskResult should return Success=true")
	}
}

// TestReportTaskResult_FailedStatus 测试 ReportTaskResult 失败状态
func TestReportTaskResult_FailedStatus(t *testing.T) {
	srv := newTestServerWithErrorDB(t)

	resp, err := srv.ReportTaskResult(context.Background(), &pb.ReportTaskResultRequest{
		TaskId:      1,
		ExecutionId: "exec-1",
		Status:      "failed",
		Error:       "task failed",
	})
	if err != nil {
		t.Fatalf("ReportTaskResult should not return gRPC error, got: %v", err)
	}
	if !resp.Success {
		t.Error("ReportTaskResult should return Success=true")
	}

	// 等待后台 goroutine 完成 HandleTaskFailure
	time.Sleep(100 * time.Millisecond)
}

// TestReportTaskResult_WithStartTime 测试带开始时间的任务结果
func TestReportTaskResult_WithStartTime(t *testing.T) {
	srv := newTestServerWithErrorDB(t)

	now := time.Now().Unix()
	resp, err := srv.ReportTaskResult(context.Background(), &pb.ReportTaskResultRequest{
		TaskId:      1,
		ExecutionId: "exec-1",
		Status:      "success",
		StartTime:   now - 60,
		EndTime:     now,
	})
	if err != nil {
		t.Fatalf("ReportTaskResult should not return gRPC error, got: %v", err)
	}
	if !resp.Success {
		t.Error("ReportTaskResult should return Success=true")
	}
}

// TestReportTaskResult_WithOnlyStartTime 测试只有开始时间的任务结果
func TestReportTaskResult_WithOnlyStartTime(t *testing.T) {
	srv := newTestServerWithErrorDB(t)

	now := time.Now().Unix()
	resp, err := srv.ReportTaskResult(context.Background(), &pb.ReportTaskResultRequest{
		TaskId:      1,
		ExecutionId: "exec-1",
		Status:      "success",
		StartTime:   now - 30,
	})
	if err != nil {
		t.Fatalf("ReportTaskResult should not return gRPC error, got: %v", err)
	}
	if !resp.Success {
		t.Error("ReportTaskResult should return Success=true")
	}
}

// ------------------------------------------------------------
// 编译期接口检查
// ------------------------------------------------------------

var _ pb.ExecutorService_SubscribeTaskServer = (*mockSubscribeStream)(nil)
