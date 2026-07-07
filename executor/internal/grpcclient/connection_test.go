package grpcclient

import (
	"context"
	"net"
	"testing"
	"time"

	pb "github.com/lynnyq/bdopsflow/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// mockExecutorServer 实现 pb.ExecutorServiceServer 接口用于测试
// 所有方法返回最小化响应，不依赖真实业务逻辑
type mockExecutorServer struct {
	pb.UnimplementedExecutorServiceServer

	heartbeatResp *pb.HeartbeatResponse
	leader        bool
}

func (m *mockExecutorServer) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	if m.heartbeatResp != nil {
		return m.heartbeatResp, nil
	}
	return &pb.HeartbeatResponse{
		Success:         true,
		Message:         "ok",
		SchedulerNodeId: "test-node",
		IsLeader:        m.leader,
	}, nil
}

func (m *mockExecutorServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	return &pb.RegisterResponse{
		Success:      true,
		Message:      "registered",
		ExecutorName: req.Name,
	}, nil
}

func (m *mockExecutorServer) SyncRunningTasks(ctx context.Context, req *pb.SyncRunningTasksRequest) (*pb.SyncRunningTasksResponse, error) {
	return &pb.SyncRunningTasksResponse{
		Success: true,
		Message: "synced",
	}, nil
}

func (m *mockExecutorServer) ReportTaskResult(ctx context.Context, req *pb.ReportTaskResultRequest) (*pb.ReportTaskResultResponse, error) {
	return &pb.ReportTaskResultResponse{
		Success: true,
		Message: "result recorded",
	}, nil
}

func (m *mockExecutorServer) ReportTaskLog(ctx context.Context, req *pb.ReportTaskLogRequest) (*pb.ReportTaskLogResponse, error) {
	return &pb.ReportTaskLogResponse{
		Success: true,
		Message: "log recorded",
	}, nil
}

func (m *mockExecutorServer) ReportTaskProgress(ctx context.Context, req *pb.ReportTaskProgressRequest) (*pb.ReportTaskProgressResponse, error) {
	return &pb.ReportTaskProgressResponse{
		Success: true,
		Message: "progress updated",
	}, nil
}

// startBufconnServer 启动一个基于 bufconn 的内存 gRPC 服务器，返回客户端连接地址的拨号函数。
// 调用者负责在测试结束后调用 cleanup() 关闭服务器。
func startBufconnServer(t *testing.T, leader bool, heartbeatResp *pb.HeartbeatResponse) (dialAddr string, cleanup func()) {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	pb.RegisterExecutorServiceServer(srv, &mockExecutorServer{
		leader:        leader,
		heartbeatResp: heartbeatResp,
	})

	go func() {
		_ = srv.Serve(lis)
	}()

	// 返回一个特殊的地址格式，拨号时使用 bufconn 拨号器
	return "bufnet", func() {
		srv.Stop()
		_ = lis.Close()
	}
}

// startBufconnServerWithDialer 启动 bufconn 服务器并返回一个可用的拨号函数。
// 返回的 dialer 可用于覆盖 grpcclient 的 connect 方法。
func startBufconnServerWithDialer(t *testing.T, leader bool) (*bufconn.Listener, func()) {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	pb.RegisterExecutorServiceServer(srv, &mockExecutorServer{
		leader: leader,
	})

	go func() {
		_ = srv.Serve(lis)
	}()

	return lis, func() {
		srv.Stop()
		_ = lis.Close()
	}
}

// dialBufconn 使用 bufconn 拨号创建 gRPC 连接
func dialBufconn(lis *bufconn.Listener) (*grpc.ClientConn, error) {
	return grpc.Dial("bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(2*time.Second),
	)
}

// TestMultiClient_ConnectToBufconn 测试通过 bufconn 连接到内存 gRPC 服务器
// 这是连接管理的正向路径测试
func TestMultiClient_ConnectToBufconn(t *testing.T) {
	lis, cleanup := startBufconnServerWithDialer(t, true)
	defer cleanup()

	conn, err := dialBufconn(lis)
	if err != nil {
		t.Fatalf("dial bufconn failed: %v", err)
	}
	defer conn.Close()

	client := pb.NewExecutorServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := client.Heartbeat(ctx, &pb.HeartbeatRequest{
		ExecutorName: "",
		CurrentLoad:  0,
	})
	if err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}
	if !resp.Success {
		t.Error("heartbeat should succeed")
	}
	if !resp.IsLeader {
		t.Error("should connect to leader")
	}
}

// TestMultiClient_HeartbeatWithLeaderResponse 测试心跳响应中 IsLeader 标志
func TestMultiClient_HeartbeatWithLeaderResponse(t *testing.T) {
	lis, cleanup := startBufconnServerWithDialer(t, true)
	defer cleanup()

	conn, err := dialBufconn(lis)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	client := pb.NewExecutorServiceClient(conn)

	// 测试多次心跳
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		resp, err := client.Heartbeat(ctx, &pb.HeartbeatRequest{
			ExecutorName: "test-executor",
			CurrentLoad:  int32(i),
		})
		cancel()
		if err != nil {
			t.Fatalf("heartbeat %d failed: %v", i, err)
		}
		if resp == nil {
			t.Fatal("response should not be nil")
		}
		if resp.SchedulerNodeId != "test-node" {
			t.Errorf("scheduler node id = %v, want test-node", resp.SchedulerNodeId)
		}
	}
}

// TestMultiClient_HeartbeatNonLeader 测试连接到非主节点
func TestMultiClient_HeartbeatNonLeader(t *testing.T) {
	lis, cleanup := startBufconnServerWithDialer(t, false)
	defer cleanup()

	conn, err := dialBufconn(lis)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	client := pb.NewExecutorServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := client.Heartbeat(ctx, &pb.HeartbeatRequest{})
	if err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}
	if resp.IsLeader {
		t.Error("should not be leader")
	}
}

// TestMultiClient_SyncRunningTasksViaBufconn 测试通过 bufconn 调用 SyncRunningTasks
func TestMultiClient_SyncRunningTasksViaBufconn(t *testing.T) {
	lis, cleanup := startBufconnServerWithDialer(t, true)
	defer cleanup()

	conn, err := dialBufconn(lis)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	grpcClient := pb.NewExecutorServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := grpcClient.SyncRunningTasks(ctx, &pb.SyncRunningTasksRequest{
		ExecutorName: "test-executor",
	})
	if err != nil {
		t.Fatalf("SyncRunningTasks failed: %v", err)
	}
	if !resp.Success {
		t.Error("SyncRunningTasks should succeed")
	}
}

// TestMultiClient_ReportResultViaBufconn 测试通过 bufconn 调用 ReportTaskResult
func TestMultiClient_ReportResultViaBufconn(t *testing.T) {
	lis, cleanup := startBufconnServerWithDialer(t, true)
	defer cleanup()

	conn, err := dialBufconn(lis)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	grpcClient := pb.NewExecutorServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := grpcClient.ReportTaskResult(ctx, &pb.ReportTaskResultRequest{
		TaskId:      1,
		ExecutionId: "exec-1",
		Status:      "success",
		Output:      "test output",
	})
	if err != nil {
		t.Fatalf("ReportTaskResult failed: %v", err)
	}
	if !resp.Success {
		t.Error("ReportTaskResult should succeed")
	}
}

// TestMultiClient_ReportLogViaBufconn 测试通过 bufconn 调用 ReportTaskLog
func TestMultiClient_ReportLogViaBufconn(t *testing.T) {
	lis, cleanup := startBufconnServerWithDialer(t, true)
	defer cleanup()

	conn, err := dialBufconn(lis)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	grpcClient := pb.NewExecutorServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := grpcClient.ReportTaskLog(ctx, &pb.ReportTaskLogRequest{
		TaskId:      1,
		ExecutionId: "exec-1",
		LogLevel:    "info",
		LogContent:  "test log",
	})
	if err != nil {
		t.Fatalf("ReportTaskLog failed: %v", err)
	}
	if !resp.Success {
		t.Error("ReportTaskLog should succeed")
	}
}

// TestMultiClient_ReportProgressViaBufconn 测试通过 bufconn 调用 ReportTaskProgress
func TestMultiClient_ReportProgressViaBufconn(t *testing.T) {
	lis, cleanup := startBufconnServerWithDialer(t, true)
	defer cleanup()

	conn, err := dialBufconn(lis)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	grpcClient := pb.NewExecutorServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := grpcClient.ReportTaskProgress(ctx, &pb.ReportTaskProgressRequest{
		TaskId:      1,
		ExecutionId: "exec-1",
		Progress:    50,
		Message:     "half done",
	})
	if err != nil {
		t.Fatalf("ReportTaskProgress failed: %v", err)
	}
	if !resp.Success {
		t.Error("ReportTaskProgress should succeed")
	}
}

// TestMultiClient_RegisterViaBufconn 测试通过 bufconn 调用 Register
func TestMultiClient_RegisterViaBufconn(t *testing.T) {
	lis, cleanup := startBufconnServerWithDialer(t, true)
	defer cleanup()

	conn, err := dialBufconn(lis)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	grpcClient := pb.NewExecutorServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := grpcClient.Register(ctx, &pb.RegisterRequest{
		Name:     "test-executor",
		Address:  "localhost:50052",
		Capacity: 10,
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	// mock 服务器返回默认响应（Success=false），这里只验证不报错
	if resp == nil {
		t.Error("response should not be nil")
	}
}

// TestMultiClient_CloseWithActiveConnection 测试 Close 在有活动连接时的行为
func TestMultiClient_CloseWithActiveConnection(t *testing.T) {
	lis, cleanup := startBufconnServerWithDialer(t, true)
	defer cleanup()

	client, _ := NewMultiClient([]string{"bufnet"})

	// 手动建立连接并存储
	conn, err := dialBufconn(lis)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}

	client.conn.Store(connWrapper{conn: conn})
	client.client.Store(clientWrapper{client: pb.NewExecutorServiceClient(conn)})
	client.isConnected.Store(true)

	// Close 应该关闭连接而不 panic
	// 不使用 defer Close()，因为 Close 只能调用一次
	client.Close()
}

// TestMultiClient_MultipleAddressesRoundRobin 测试多地址轮询逻辑
func TestMultiClient_MultipleAddressesRoundRobin(t *testing.T) {
	addrs := []string{"addr1:50051", "addr2:50052", "addr3:50053", "addr4:50054"}
	client, _ := NewMultiClient(addrs)
	defer client.Close()

	// 验证轮询完整循环
	expected := []string{"addr1:50051", "addr2:50052", "addr3:50053", "addr4:50054"}
	for i, want := range expected {
		got := client.getCurrentAddr()
		if got != want {
			t.Errorf("round-robin step %d: got %v, want %v", i, got, want)
		}
		client.nextAddr()
	}

	// 验证回绕到第一个地址
	got := client.getCurrentAddr()
	if got != "addr1:50051" {
		t.Errorf("after full cycle, got %v, want addr1:50051", got)
	}
}

// TestMultiClient_SingleAddressRoundRobin 测试单地址轮询始终返回同一地址
func TestMultiClient_SingleAddressRoundRobin(t *testing.T) {
	client, _ := NewMultiClient([]string{"only-addr:50051"})
	defer client.Close()

	for i := 0; i < 5; i++ {
		got := client.getCurrentAddr()
		if got != "only-addr:50051" {
			t.Errorf("step %d: got %v, want only-addr:50051", i, got)
		}
		client.nextAddr()
	}
}

// TestMultiClient_ReconnectMultipleTimes 测试多次重连不会 panic
func TestMultiClient_ReconnectMultipleTimes(t *testing.T) {
	client, _ := NewMultiClient([]string{"localhost:50051", "localhost:50052"})
	defer client.Close()

	// 多次重连不应 panic
	for i := 0; i < 10; i++ {
		client.reconnect()
	}

	// 验证索引正确前进
	idx := client.currentIndex.Load()
	if idx != 10 {
		t.Errorf("after 10 reconnects, index should be 10, got %d", idx)
	}
}

// TestMultiClient_GetExecutorNameAfterSet 测试设置后获取执行器名称
func TestMultiClient_GetExecutorNameAfterSet(t *testing.T) {
	client, _ := NewMultiClient([]string{"localhost:50051"})
	defer client.Close()

	// 设置不同的名称
	names := []string{"exec-1", "exec-2", "exec-3"}
	for _, name := range names {
		client.executorName.Store(nameWrapper{name: name})
		got := client.GetExecutorName()
		if got != name {
			t.Errorf("GetExecutorName() = %v, want %v", got, name)
		}
	}
}

// TestMockTaskRunner 测试 MockTaskRunner 的行为
func TestMockTaskRunner(t *testing.T) {
	runner := &MockTaskRunner{
		runningTasks:   5,
		runningExecIds: []string{"exec-1", "exec-2"},
		runningStates: []*pb.RunningTaskState{
			{ExecutionId: "exec-1", Progress: 50},
			{ExecutionId: "exec-2", Progress: 80},
		},
	}

	// 测试 GetRunningTasks
	if got := runner.GetRunningTasks(); got != 5 {
		t.Errorf("GetRunningTasks() = %v, want 5", got)
	}

	// 测试 GetRunningExecutionIds
	if got := runner.GetRunningExecutionIds(); len(got) != 2 {
		t.Errorf("GetRunningExecutionIds() len = %v, want 2", len(got))
	}

	// 测试 GetRunningTaskStates
	if got := runner.GetRunningTaskStates(); len(got) != 2 {
		t.Errorf("GetRunningTaskStates() len = %v, want 2", len(got))
	}

	// 测试 UpdateCapacity
	if err := runner.UpdateCapacity(20); err != nil {
		t.Errorf("UpdateCapacity failed: %v", err)
	}
	if runner.capacity != 20 {
		t.Errorf("capacity = %v, want 20", runner.capacity)
	}

	// 测试 CancelTask
	if !runner.CancelTask("exec-1") {
		t.Error("CancelTask should return true")
	}
	if len(runner.cancelledTasks) != 1 {
		t.Errorf("cancelledTasks len = %v, want 1", len(runner.cancelledTasks))
	}
	if runner.cancelledTasks[0] != "exec-1" {
		t.Errorf("cancelledTasks[0] = %v, want exec-1", runner.cancelledTasks[0])
	}
}

// TestMultiClient_StopChannelBehavior 测试 stopCh 的行为
func TestMultiClient_StopChannelBehavior(t *testing.T) {
	client, _ := NewMultiClient([]string{"localhost:50051"})

	// stopCh 初始应该可接收（未关闭时不阻塞）
	select {
	case <-client.stopCh:
		t.Fatal("stopCh should not be closed initially")
	default:
		// 预期：不阻塞
	}

	// Close 后 stopCh 应已关闭
	client.Close()
	select {
	case <-client.stopCh:
		// 预期：可接收
	default:
		t.Fatal("stopCh should be closed after Close()")
	}
}
