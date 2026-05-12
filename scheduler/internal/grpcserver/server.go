package grpcserver

import (
	"context"
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
	port      string
	grpcServer *grpc.Server
	scheduler *service.SchedulerService
	mu        sync.RWMutex
	executors map[string]*executorConn
	lis       net.Listener
}

type executorConn struct {
	stream  pb.ExecutorService_SubscribeTaskServer
	address string
	name    string
}

func NewServer(port string, scheduler *service.SchedulerService) *Server {
	s := &Server{
		port:      port,
		scheduler: scheduler,
		executors: make(map[string]*executorConn),
	}

	scheduler.SetTaskDispatcher(s.dispatchTask)

	return s
}

func (s *Server) dispatchTask(executorID string, task *pb.Task) error {
	s.mu.RLock()
	conn, ok := s.executors[executorID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("executor %s not connected", executorID)
	}

	slog.Info("dispatching task to executor",
		"executor_id", executorID,
		"task_id", task.TaskId,
		"execution_id", task.ExecutionId,
	)

	return conn.stream.Send(task)
}

func (s *Server) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	slog.Info("executor register request",
		"executor_id", req.ExecutorId,
		"name", req.Name,
	)

	s.scheduler.RegisterExecutor(ctx, req.ExecutorId, req.Name, req.Address, req.Capacity)

	return &pb.RegisterResponse{
		Success: true,
		Message: "registered",
	}, nil
}

func (s *Server) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	s.scheduler.UpdateExecutorHeartbeat(ctx, req.ExecutorId, req.CurrentLoad)
	return &pb.HeartbeatResponse{
		Success: true,
		Message: "ok",
	}, nil
}

func (s *Server) SubscribeTask(req *pb.SubscribeTaskRequest, stream pb.ExecutorService_SubscribeTaskServer) error {
	executorID := req.ExecutorId

	s.mu.Lock()
	s.executors[executorID] = &executorConn{
		stream: stream,
	}
	s.mu.Unlock()

	slog.Info("executor subscribed", "executor_id", executorID)

	<-stream.Context().Done()

	s.mu.Lock()
	delete(s.executors, executorID)
	s.mu.Unlock()

	slog.Info("executor disconnected", "executor_id", executorID)
	return nil
}

func (s *Server) ReportTaskResult(ctx context.Context, req *pb.ReportTaskResultRequest) (*pb.ReportTaskResultResponse, error) {
	slog.Info("task result received",
		"execution_id", req.ExecutionId,
		"status", req.Status,
	)

	s.scheduler.UpdateExecutionResult(ctx, req.ExecutionId, req.Status, req.Output, req.Error)

	status := "success"
	if req.Status == "failed" {
		status = "failed"
	}
	s.scheduler.UpdateTaskStatusByID(ctx, req.TaskId, status)

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
		// 使用 goroutine 来执行 GracefulStop，有超时
		done := make(chan struct{})
		go func() {
			s.grpcServer.GracefulStop()
			close(done)
		}()

		// 等待最多 3 秒，之后强制停止
		select {
		case <-done:
			slog.Info("gRPC server stopped gracefully")
		case <-time.After(3 * time.Second):
			slog.Warn("gRPC server graceful stop timed out, forcing stop")
			s.grpcServer.Stop()
		}
	}
}