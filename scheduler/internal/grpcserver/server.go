package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	pb "github.com/lynnyq/bdopsflow/proto"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
	"google.golang.org/grpc"
)

type Server struct {
	pb.UnimplementedExecutorServiceServer
	port            string
	grpcServer      *grpc.Server
	scheduler       *service.SchedulerService
	mu              sync.RWMutex
	executors       map[string]*executorConn
	lis             net.Listener
	nodeId          string
	isNewLeader     bool
	isLeader        bool           // 当前是否是主节点
	needExecSync    map[string]bool // 记录哪些执行器需要同步
}

type executorConn struct {
	stream pb.ExecutorService_SubscribeTaskServer
}

func NewServer(port string, scheduler *service.SchedulerService) *Server {
	s := &Server{
		port:         port,
		scheduler:    scheduler,
		executors:    make(map[string]*executorConn),
		needExecSync: make(map[string]bool),
	}

	scheduler.SetTaskDispatcher(s.dispatchTask)

	return s
}

// 设置节点 ID
func (s *Server) SetNodeId(nodeId string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodeId = nodeId
}

// 标记成为新 leader，需要执行器同步
func (s *Server) MarkAsNewLeader() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isNewLeader = true
	s.isLeader = true
	for name := range s.executors {
		s.needExecSync[name] = true
	}

	go func() {
		time.Sleep(30 * time.Second)
		s.mu.Lock()
		s.isNewLeader = false
		s.mu.Unlock()
		slog.Info("[gRPC] isNewLeader flag reset after timeout")
	}()
}

// 设置当前是否是 leader
func (s *Server) SetLeader(isLeader bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isLeader = isLeader
}

func (s *Server) dispatchTask(executorName string, task *pb.Task) error {
	s.mu.RLock()
	conn, ok := s.executors[executorName]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("executor %s not connected", executorName)
	}

	slog.Info("dispatching task to executor",
		"executor_name", executorName,
		"task_id", task.TaskId,
		"execution_id", task.ExecutionId,
	)

	return conn.stream.Send(task)
}

func (s *Server) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	slog.Info("executor register request",
		"name", req.Name,
		"address", req.Address,
	)

	executorName, err := s.scheduler.RegisterExecutor(ctx, req.Name, req.Address, req.Capacity)
	if err != nil {
		slog.Error("executor register failed", "error", err)
		resp := &pb.RegisterResponse{
			Success: false,
			Message: err.Error(),
		}
		if errors.Is(err, service.ErrExecutorDuplicate) {
			resp.Duplicate = true
		}
		return resp, nil
	}

	slog.Info("executor registered successfully", "executor_name", executorName)

	return &pb.RegisterResponse{
		Success:      true,
		Message:      "registered",
		ExecutorName: executorName,
	}, nil
}

func (s *Server) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	if req.ExecutorName == "" {
		// 只返回基本信息的心跳响应
		s.mu.RLock()
		nodeId := s.nodeId
		isLeader := s.isLeader
		s.mu.RUnlock()
		
		return &pb.HeartbeatResponse{
			Success:         true,
			Message:         "ok",
			SchedulerNodeId: nodeId,
			IsLeader:        isLeader,
		}, nil
	}

	// 处理正在运行的任务信息
	if len(req.RunningTasks) > 0 {
		// 如果收到详细任务信息，更新调度器的任务状态
		slog.Debug("received detailed running tasks from executor",
			"executor_name", req.ExecutorName,
			"task_count", len(req.RunningTasks),
		)
		
		// 提取 execution ids
		execIds := make([]string, 0, len(req.RunningTasks))
		for _, task := range req.RunningTasks {
			execIds = append(execIds, task.ExecutionId)
			// 更新任务进度信息
			s.scheduler.UpdateTaskProgress(ctx, task.ExecutionId, task.Progress, task.ProgressMessage)
		}
		
		err := s.scheduler.UpdateExecutorHeartbeatWithRunningTasks(ctx, req.ExecutorName, req.CurrentLoad, execIds)
		if err != nil {
			slog.Warn("failed to update executor heartbeat", "executor_name", req.ExecutorName, "error", err)
		}
	} else {
		// 只使用 execution ids
		err := s.scheduler.UpdateExecutorHeartbeatWithRunningTasks(ctx, req.ExecutorName, req.CurrentLoad, req.RunningExecutionIds)
		if err != nil {
			slog.Warn("failed to update executor heartbeat", "executor_name", req.ExecutorName, "error", err)
		}
	}

	targetCapacity, _ := s.scheduler.GetExecutorTargetCapacity(ctx, req.ExecutorName)

	// 检查是否需要让执行器下次同步详细信息
	s.mu.Lock()
	nodeId := s.nodeId
	isNewLeader := s.isNewLeader
	isLeader := s.isLeader
	needFullSync := s.needExecSync[req.ExecutorName]
	// 处理完后清除标记
	if needFullSync {
		delete(s.needExecSync, req.ExecutorName)
	}
	s.mu.Unlock()

	return &pb.HeartbeatResponse{
		Success:         true,
		Message:         "ok",
		TargetCapacity:  targetCapacity,
		NeedFullSync:    needFullSync || (req.IsReconnect && isNewLeader),
		SchedulerNodeId: nodeId,
		IsNewLeader:     isNewLeader,
		IsLeader:        isLeader,
	}, nil
}

// 新增：同步正在运行的任务
func (s *Server) SyncRunningTasks(ctx context.Context, req *pb.SyncRunningTasksRequest) (*pb.SyncRunningTasksResponse, error) {
	slog.Info("sync running tasks requested", "executor_name", req.ExecutorName)
	
	// 标记该执行器需要在下次心跳时同步
	s.mu.Lock()
	s.needExecSync[req.ExecutorName] = true
	s.mu.Unlock()
	
	return &pb.SyncRunningTasksResponse{
		Success: true,
		Message: "please send running tasks in next heartbeat",
	}, nil
}

func (s *Server) SubscribeTask(req *pb.SubscribeTaskRequest, stream pb.ExecutorService_SubscribeTaskServer) error {
	executorName := req.ExecutorName

	s.mu.Lock()
	s.executors[executorName] = &executorConn{
		stream: stream,
	}
	s.mu.Unlock()

	slog.Info("executor subscribed", "executor_name", executorName)

	<-stream.Context().Done()

	s.mu.Lock()
	delete(s.executors, executorName)
	s.mu.Unlock()

	slog.Info("executor disconnected", "executor_name", executorName)
	return nil
}

func (s *Server) ReportTaskResult(ctx context.Context, req *pb.ReportTaskResultRequest) (*pb.ReportTaskResultResponse, error) {
	slog.Info("task result received",
		"execution_id", req.ExecutionId,
		"status", req.Status,
		"task_id", req.TaskId,
	)

	s.scheduler.UpdateExecutionResult(ctx, req.ExecutionId, req.Status, req.Output, req.Error)

	if req.Status == "failed" {
		slog.Info("task execution failed, checking retry policy",
			"execution_id", req.ExecutionId,
			"task_id", req.TaskId,
		)

		go func() {
			if err := s.scheduler.HandleTaskFailure(context.Background(), req.TaskId, req.ExecutionId, req.Output, req.Error); err != nil {
				slog.Error("handle task failure failed",
					"execution_id", req.ExecutionId,
					"task_id", req.TaskId,
					"error", err,
				)
			}
		}()
	} else {
		s.scheduler.UpdateTaskStatusByID(ctx, req.TaskId, "success")
		s.scheduler.SendWebhookNotification(ctx, req.TaskId, req.ExecutionId, "success", req.Output, req.Error, 0)
	}

	return &pb.ReportTaskResultResponse{
		Success: true,
		Message: "result recorded",
	}, nil
}

func (s *Server) ReportTaskLog(ctx context.Context, req *pb.ReportTaskLogRequest) (*pb.ReportTaskLogResponse, error) {
	slog.Info("task log received",
		"execution_id", req.ExecutionId,
		"level", req.LogLevel,
	)

	s.scheduler.AddTaskLog(ctx, req.ExecutionId, req.TaskId, "", req.LogLevel, req.LogContent)

	return &pb.ReportTaskLogResponse{
		Success: true,
		Message: "log recorded",
	}, nil
}

func (s *Server) ReportTaskProgress(ctx context.Context, req *pb.ReportTaskProgressRequest) (*pb.ReportTaskProgressResponse, error) {
	slog.Debug("task progress received",
		"execution_id", req.ExecutionId,
		"progress", req.Progress,
		"task_id", req.TaskId,
	)

	if err := s.scheduler.UpdateTaskProgress(ctx, req.ExecutionId, req.Progress, req.Message); err != nil {
		slog.Warn("failed to update task progress", "execution_id", req.ExecutionId, "error", err)
	}

	return &pb.ReportTaskProgressResponse{
		Success: true,
		Message: "progress updated",
	}, nil
}

func (s *Server) Start() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", s.port))
	if err != nil {
		return err
	}
	s.lis = lis

	s.grpcServer = grpc.NewServer()
	pb.RegisterExecutorServiceServer(s.grpcServer, s)

	slog.Info("gRPC server listening", "port", s.port)
	return s.grpcServer.Serve(lis)
}

func (s *Server) Stop() {
	if s.grpcServer != nil {
		done := make(chan struct{})
		go func() {
			s.grpcServer.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
			slog.Info("gRPC server stopped gracefully")
		case <-time.After(3 * time.Second):
			slog.Warn("gRPC server graceful stop timed out, forcing stop")
			s.grpcServer.Stop()
		}
	}
}

func (s *Server) IsExecutorConnected(executorName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.executors[executorName]
	return ok
}

func (s *Server) GetConnectedExecutorNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.executors))
	for name := range s.executors {
		names = append(names, name)
	}
	return names
}