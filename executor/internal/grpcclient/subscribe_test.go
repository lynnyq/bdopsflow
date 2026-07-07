package grpcclient

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	pb "github.com/lynnyq/bdopsflow/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// configurableMockServer 可配置的 mock 服务器，支持 SubscribeTask 流式和多种 Heartbeat 响应
type configurableMockServer struct {
	pb.UnimplementedExecutorServiceServer

	mu sync.Mutex

	// Register 配置
	registerSuccess   bool
	registerDuplicate bool
	registerMessage   string

	// Heartbeat 配置
	heartbeatResp *pb.HeartbeatResponse
	heartbeatFn   func(req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error)

	// SubscribeTask 配置
	subscribeTasks []*pb.Task // 要发送的任务列表
	subscribeErr   error      // SubscribeTask 返回的错误
	subscribeDelay time.Duration

	// 状态记录
	heartbeatCount int32
	registerCount  int32
}

func (m *configurableMockServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	m.mu.Lock()
	m.registerCount++
	m.mu.Unlock()

	return &pb.RegisterResponse{
		Success:      m.registerSuccess,
		Message:      m.registerMessage,
		ExecutorName: req.Name,
		Duplicate:    m.registerDuplicate,
	}, nil
}

func (m *configurableMockServer) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	m.mu.Lock()
	m.heartbeatCount++
	m.mu.Unlock()

	if m.heartbeatFn != nil {
		return m.heartbeatFn(req)
	}
	if m.heartbeatResp != nil {
		return m.heartbeatResp, nil
	}
	return &pb.HeartbeatResponse{
		Success:         true,
		Message:         "ok",
		SchedulerNodeId: "test-node",
		IsLeader:        true,
	}, nil
}

func (m *configurableMockServer) SubscribeTask(req *pb.SubscribeTaskRequest, stream pb.ExecutorService_SubscribeTaskServer) error {
	if m.subscribeErr != nil {
		return m.subscribeErr
	}

	// 发送任务
	for _, task := range m.subscribeTasks {
		if m.subscribeDelay > 0 {
			time.Sleep(m.subscribeDelay)
		}
		if err := stream.Send(task); err != nil {
			return err
		}
	}

	// 发送完任务后阻塞，直到客户端断开
	<-stream.Context().Done()
	return stream.Context().Err()
}

func (m *configurableMockServer) SyncRunningTasks(ctx context.Context, req *pb.SyncRunningTasksRequest) (*pb.SyncRunningTasksResponse, error) {
	return &pb.SyncRunningTasksResponse{Success: true, Message: "ok"}, nil
}

func (m *configurableMockServer) ReportTaskResult(ctx context.Context, req *pb.ReportTaskResultRequest) (*pb.ReportTaskResultResponse, error) {
	return &pb.ReportTaskResultResponse{Success: true, Message: "ok"}, nil
}

func (m *configurableMockServer) ReportTaskLog(ctx context.Context, req *pb.ReportTaskLogRequest) (*pb.ReportTaskLogResponse, error) {
	return &pb.ReportTaskLogResponse{Success: true, Message: "ok"}, nil
}

func (m *configurableMockServer) ReportTaskProgress(ctx context.Context, req *pb.ReportTaskProgressRequest) (*pb.ReportTaskProgressResponse, error) {
	return &pb.ReportTaskProgressResponse{Success: true, Message: "ok"}, nil
}

// startConfigurableBufconnServer 启动可配置的 bufconn 服务器
func startConfigurableBufconnServer(t *testing.T, server *configurableMockServer) (*bufconn.Listener, func()) {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	pb.RegisterExecutorServiceServer(srv, server)

	go func() {
		_ = srv.Serve(lis)
	}()

	return lis, func() {
		srv.Stop()
		_ = lis.Close()
	}
}

// dialBufconnForClient 使用 bufconn 拨号并返回 conn 和 client
func dialBufconnForClient(lis *bufconn.Listener) (*grpc.ClientConn, pb.ExecutorServiceClient, error) {
	conn, err := grpc.Dial("bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(2*time.Second),
	)
	if err != nil {
		return nil, nil, err
	}
	return conn, pb.NewExecutorServiceClient(conn), nil
}

// setupConnectedClient 创建并配置一个已连接的 MultiClient（使用 bufconn）。
// 返回的 cleanup 函数仅负责关闭 bufconn 服务器，不调用 client.Close()。
// 调用方需自行管理 client.Close()，且只能调用一次。
func setupConnectedClient(t *testing.T, server *configurableMockServer) (*MultiClient, func()) {
	t.Helper()

	lis, serverCleanup := startConfigurableBufconnServer(t, server)

	client, _ := NewMultiClient([]string{"bufnet"})

	conn, grpcClient, err := dialBufconnForClient(lis)
	if err != nil {
		serverCleanup()
		client.Close()
		t.Fatalf("dial bufconn failed: %v", err)
	}

	client.conn.Store(connWrapper{conn: conn})
	client.client.Store(clientWrapper{client: grpcClient})
	client.isConnected.Store(true)

	// 仅返回服务器清理函数，不包含 client.Close()
	return client, serverCleanup
}

// === Subscribe 测试 ===

// TestMultiClient_Subscribe_Success 测试 Subscribe 成功路径
// 覆盖 client.go:325 Subscribe 的主要成功路径
func TestMultiClient_Subscribe_Success(t *testing.T) {
	server := &configurableMockServer{
		registerSuccess: true,
		registerMessage: "registered",
		subscribeTasks: []*pb.Task{
			{TaskId: 1, ExecutionId: "exec-1", Type: "http"},
		},
	}

	client, serverCleanup := setupConnectedClient(t, server)
	defer serverCleanup()
	// 不 defer client.Close()，在测试体中显式调用一次

	runner := &MockTaskRunner{}

	// 在 goroutine 中运行 Subscribe
	done := make(chan error, 1)
	go func() {
		done <- client.Subscribe("test-executor", "localhost:50052", 10, runner)
	}()

	// 等待任务被接收
	time.Sleep(500 * time.Millisecond)

	// 关闭客户端，触发 Subscribe 退出（仅调用一次 Close）
	client.Close()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Subscribe 应在 Close 后返回 nil, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Subscribe 未在超时内退出")
	}

	// 验证 Register 被调用
	if server.registerCount == 0 {
		t.Error("Register 应被调用")
	}
}

// TestMultiClient_Subscribe_StopBeforeStart 测试在 Subscribe 启动前关闭 stopCh
// 覆盖 client.go:339 的 stopCh 检查
func TestMultiClient_Subscribe_StopBeforeStart(t *testing.T) {
	server := &configurableMockServer{
		registerSuccess: true,
	}

	client, serverCleanup := setupConnectedClient(t, server)
	defer serverCleanup()
	// 不 defer client.Close()，下面显式调用

	runner := &MockTaskRunner{}

	// 先关闭 stopCh（仅调用一次 Close）
	client.Close()

	// Subscribe 应立即返回
	done := make(chan error, 1)
	go func() {
		done <- client.Subscribe("test-executor", "localhost:50052", 10, runner)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Subscribe 应在 stopCh 关闭后返回 nil, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Subscribe 应在 stopCh 关闭后立即退出")
	}
}

// TestMultiClient_Subscribe_ConnectionFailure 测试 Subscribe 在连接失败时的重试行为
// 覆盖 client.go:344-355 的 ensureConnected 错误路径
func TestMultiClient_Subscribe_ConnectionFailure(t *testing.T) {
	// 使用不可达地址，ensureConnected 会失败
	client, _ := NewMultiClient([]string{"127.0.0.1:1"})
	// 不 defer client.Close()，在测试体中显式调用一次

	runner := &MockTaskRunner{}

	// 在 goroutine 中运行 Subscribe
	done := make(chan error, 1)
	go func() {
		done <- client.Subscribe("test-executor", "localhost:50052", 10, runner)
	}()

	// 等待一小段时间让 Subscribe 进入连接重试循环
	time.Sleep(2 * time.Second)

	// 关闭客户端（仅调用一次 Close）
	client.Close()

	// grpc.Dial 的 WithTimeout 为 5 秒，Close 后 Subscribe 可能仍在 grpc.Dial 中
	// 需要等待 grpc.Dial 超时后 Subscribe 才能检查 stopCh 并退出
	select {
	case <-done:
		// Subscribe 应在 Close 和 grpc.Dial 超时后退出
	case <-time.After(10 * time.Second):
		t.Fatal("Subscribe 应在 Close 后退出")
	}
}

// TestMultiClient_Subscribe_DuplicateRejection 测试 Subscribe 在 Register 被拒绝（重复）时返回错误
// 覆盖 client.go:396-399 的 duplicate 分支
func TestMultiClient_Subscribe_DuplicateRejection(t *testing.T) {
	server := &configurableMockServer{
		registerSuccess:   false,
		registerDuplicate: true,
		registerMessage:   "executor already online",
	}

	client, serverCleanup := setupConnectedClient(t, server)
	defer serverCleanup()
	defer client.Close() // Subscribe 会立即返回错误，不需要中途 Close

	runner := &MockTaskRunner{}

	// Subscribe 应在 Register 被拒绝后立即返回错误
	done := make(chan error, 1)
	go func() {
		done <- client.Subscribe("test-executor", "localhost:50052", 10, runner)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Error("Subscribe 应在 duplicate 注册时返回错误")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Subscribe 应在 duplicate 注册后立即返回错误")
	}
}

// TestMultiClient_Subscribe_RegisterNotSuccess 测试 Subscribe 在 Register 失败（非 duplicate）时重试
// 覆盖 client.go:400-410 的 register failed 分支
func TestMultiClient_Subscribe_RegisterNotSuccess(t *testing.T) {
	server := &configurableMockServer{
		registerSuccess:   false,
		registerDuplicate: false,
		registerMessage:   "registration rejected",
	}

	client, serverCleanup := setupConnectedClient(t, server)
	defer serverCleanup()
	// 不 defer client.Close()，在测试体中显式调用一次

	runner := &MockTaskRunner{}

	// Subscribe 应在 Register 失败后重试
	done := make(chan error, 1)
	go func() {
		done <- client.Subscribe("test-executor", "localhost:50052", 10, runner)
	}()

	// 等待重试
	time.Sleep(2 * time.Second)

	// 关闭客户端（仅调用一次 Close）
	client.Close()

	select {
	case <-done:
		// Subscribe 应在 Close 后退出
	case <-time.After(3 * time.Second):
		t.Fatal("Subscribe 应在 Close 后退出")
	}
}

// TestMultiClient_Subscribe_ReceiveTasks 测试 Subscribe 接收并执行任务
// 覆盖 client.go:490-496 的 task receiving 分支
func TestMultiClient_Subscribe_ReceiveTasks(t *testing.T) {
	server := &configurableMockServer{
		registerSuccess: true,
		subscribeTasks: []*pb.Task{
			{TaskId: 1, ExecutionId: "exec-1", Type: "http"},
			{TaskId: 2, ExecutionId: "exec-2", Type: "shell"},
		},
	}

	client, serverCleanup := setupConnectedClient(t, server)
	defer serverCleanup()
	// 不 defer client.Close()，在测试体中显式调用一次

	runner := &MockTaskRunner{}

	// 在 goroutine 中运行 Subscribe
	done := make(chan error, 1)
	go func() {
		done <- client.Subscribe("test-executor", "localhost:50052", 10, runner)
	}()

	// 等待任务被接收和处理
	time.Sleep(1 * time.Second)

	// 关闭客户端（仅调用一次 Close）
	client.Close()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Subscribe 未在超时内退出")
	}
}

// === heartbeatLoop 测试 ===

// TestMultiClient_HeartbeatLoop_StopChannel 测试 heartbeatLoop 在 stopCh 关闭时退出
// 覆盖 client.go:648-650 的 stopCh 分支
func TestMultiClient_HeartbeatLoop_StopChannel(t *testing.T) {
	client, _ := NewMultiClient([]string{"localhost:50051"})
	// 不 defer client.Close()，在测试体中显式调用一次

	done := make(chan struct{}, 1)
	go func() {
		client.heartbeatLoop("test-executor", 10)
		close(done)
	}()

	// 立即关闭（仅调用一次 Close）
	client.Close()

	select {
	case <-done:
		// heartbeatLoop 应在 Close 后退出
	case <-time.After(2 * time.Second):
		t.Fatal("heartbeatLoop 应在 Close 后退出")
	}
}

// TestMultiClient_HeartbeatLoop_NotConnected 测试 heartbeatLoop 在 ticker 触发且未连接时退出
// 覆盖 client.go:534-536 的 !isConnected 分支
func TestMultiClient_HeartbeatLoop_NotConnected(t *testing.T) {
	client, _ := NewMultiClient([]string{"localhost:50051"})
	// 不 defer client.Close()，在测试体中显式调用一次

	// isConnected 为 false
	client.isConnected.Store(false)

	done := make(chan struct{}, 1)
	go func() {
		client.heartbeatLoop("test-executor", 10)
		close(done)
	}()

	// 等待 ticker 触发（10秒），然后 !isConnected 检查会让 heartbeatLoop 退出
	select {
	case <-done:
		// heartbeatLoop 在 ticker 触发后检测到未连接，返回
	case <-time.After(12 * time.Second):
		client.Close()
		t.Fatal("heartbeatLoop 应在 ticker 触发后退出（未连接时）")
	}

	// 确保 client 被 Close
	client.Close()
}

// TestMultiClient_HeartbeatLoop_NilClient 测试 heartbeatLoop 在 ticker 触发且 client 为 nil 时退出
// 覆盖 client.go:544-546 的 nil client 分支
func TestMultiClient_HeartbeatLoop_NilClient(t *testing.T) {
	client, _ := NewMultiClient([]string{"localhost:50051"})
	// 不 defer client.Close()，在测试体中显式调用一次

	// isConnected 为 true 但 client 为 nil
	client.isConnected.Store(true)
	client.client.Store(clientWrapper{client: nil})

	done := make(chan struct{}, 1)
	go func() {
		client.heartbeatLoop("test-executor", 10)
		close(done)
	}()

	// 等待 ticker 触发（10秒），然后 nil client 检查会让 heartbeatLoop 退出
	select {
	case <-done:
		// heartbeatLoop 在 ticker 触发后检测到 nil client，返回
	case <-time.After(12 * time.Second):
		client.Close()
		t.Fatal("heartbeatLoop 应在 ticker 触发后退出（nil client 时）")
	}

	// 确保 client 被 Close
	client.Close()
}

// TestMultiClient_HeartbeatLoop_SuccessWithUpdates 测试 heartbeatLoop 成功心跳并处理多种响应
// 一次 11 秒等待覆盖：成功心跳、容量更新、取消任务、needFullSync
// 覆盖 client.go:559-566, 576-582, 591, 613, 616-629, 632-643
func TestMultiClient_HeartbeatLoop_SuccessWithUpdates(t *testing.T) {
	server := &configurableMockServer{
		heartbeatResp: &pb.HeartbeatResponse{
			Success:           true,
			Message:           "ok",
			SchedulerNodeId:   "test-node",
			IsLeader:          true,
			TargetCapacity:    20,
			CancelExecutionIds: []string{"exec-1", "exec-2"},
			NeedFullSync:      true,
		},
	}

	client, serverCleanup := setupConnectedClient(t, server)
	defer serverCleanup()
	// 不 defer client.Close()，在测试体中显式调用一次

	// 设置 lastSchedulerId 为 test-node（与响应一致，不触发调度器变化）
	client.lastSchedulerId.Store(nameWrapper{name: "test-node"})

	runner := &MockTaskRunner{
		capacity: 10,
		runningStates: []*pb.RunningTaskState{
			{ExecutionId: "exec-1", Progress: 50},
		},
	}
	client.taskRunner.Store(taskRunnerWrapper{runner: runner})

	// 设置 needFullSync 为 true，覆盖 needFullSync 分支
	client.needFullSync.Store(true)

	done := make(chan struct{}, 1)
	go func() {
		client.heartbeatLoop("test-executor", 10)
		close(done)
	}()

	// 等待 ticker 触发（10秒）
	time.Sleep(11 * time.Second)

	// 验证心跳被调用
	server.mu.Lock()
	heartbeatCount := server.heartbeatCount
	server.mu.Unlock()
	if heartbeatCount == 0 {
		t.Error("Heartbeat 应被调用至少一次")
	}

	// 验证容量被更新
	if got := runner.Capacity(); got != 20 {
		t.Errorf("容量应被更新为 20, got %d", got)
	}

	// 验证取消任务被调用
	if got := runner.CancelledTasks(); len(got) < 2 {
		t.Errorf("应取消 2 个任务, got %d", len(got))
	}

	// 关闭客户端（仅调用一次 Close）
	client.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("heartbeatLoop 应在 Close 后退出")
	}
}

// TestMultiClient_HeartbeatLoop_SchedulerChange 测试 heartbeatLoop 检测调度器变化
// 覆盖 client.go:593-606 的 scheduler change 分支
func TestMultiClient_HeartbeatLoop_SchedulerChange(t *testing.T) {
	// 返回 node-2，与 lastSchedulerId=node-1 不同，触发调度器变化
	server := &configurableMockServer{
		heartbeatResp: &pb.HeartbeatResponse{
			Success:         true,
			SchedulerNodeId: "node-2",
			IsLeader:        true,
		},
	}

	client, serverCleanup := setupConnectedClient(t, server)
	defer serverCleanup()
	// 不 defer client.Close()，在测试体中显式调用一次

	// 先设置 lastSchedulerId 为 node-1，首次心跳返回 node-2 触发变化检测
	client.lastSchedulerId.Store(nameWrapper{name: "node-1"})

	runner := &MockTaskRunner{}
	client.taskRunner.Store(taskRunnerWrapper{runner: runner})

	done := make(chan struct{}, 1)
	go func() {
		client.heartbeatLoop("test-executor", 10)
		close(done)
	}()

	// 等待 ticker 触发（10秒），第一次心跳返回 node-2（与 lastSchedulerId=node-1 不同）
	// 触发调度器变化分支，heartbeatLoop 返回
	select {
	case <-done:
		// heartbeatLoop 在检测到调度器变化后退出
	case <-time.After(12 * time.Second):
		client.Close()
		t.Fatal("heartbeatLoop 应在检测到调度器变化后退出")
	}

	// 确保 client 被 Close
	client.Close()

	// 验证心跳被调用
	if server.heartbeatCount == 0 {
		t.Error("Heartbeat 应被调用")
	}
}

// TestMultiClient_HeartbeatLoop_HeartbeatError 测试 heartbeatLoop 在 Heartbeat 返回错误时退出
// 覆盖 client.go:584-588 的 heartbeat error 分支
func TestMultiClient_HeartbeatLoop_HeartbeatError(t *testing.T) {
	server := &configurableMockServer{
		heartbeatFn: func(req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
			return nil, fmt.Errorf("scheduler internal error")
		},
	}

	client, serverCleanup := setupConnectedClient(t, server)
	defer serverCleanup()
	// 不 defer client.Close()，在测试体中显式调用一次

	runner := &MockTaskRunner{}
	client.taskRunner.Store(taskRunnerWrapper{runner: runner})

	done := make(chan struct{}, 1)
	go func() {
		client.heartbeatLoop("test-executor", 10)
		close(done)
	}()

	// 等待 ticker 触发（10秒），Heartbeat 返回错误，heartbeatLoop 退出
	select {
	case <-done:
		// heartbeatLoop 在 Heartbeat 错误后退出
	case <-time.After(12 * time.Second):
		client.Close()
		t.Fatal("heartbeatLoop 应在 Heartbeat 错误后退出")
	}

	// 确保 client 被 Close
	client.Close()
}

// TestMultiClient_HeartbeatLoop_NilResponse 测试 heartbeatLoop 在 resp 为 nil 时
// 覆盖 client.go:644-646 的 nil resp 分支
func TestMultiClient_HeartbeatLoop_NilResponse(t *testing.T) {
	server := &configurableMockServer{
		heartbeatFn: func(req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
			return nil, nil
		},
	}

	client, serverCleanup := setupConnectedClient(t, server)
	defer serverCleanup()
	// 不 defer client.Close()，在测试体中显式调用一次

	runner := &MockTaskRunner{}
	client.taskRunner.Store(taskRunnerWrapper{runner: runner})

	done := make(chan struct{}, 1)
	go func() {
		client.heartbeatLoop("test-executor", 10)
		close(done)
	}()

	// 等待 ticker 触发（10秒），Heartbeat 返回 nil，heartbeatLoop 继续循环
	// 需要再等待下一个 ticker 或 Close
	time.Sleep(11 * time.Second)

	// 关闭客户端（仅调用一次 Close）
	client.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("heartbeatLoop 应在 Close 后退出")
	}
}
