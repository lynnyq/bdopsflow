package grpcclient

import (
	"context"
	"log/slog"
	"sync"
	"time"

	pb "github.com/lynnyq/bdopsflow/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type TaskRunner interface {
	Execute(ctx context.Context, task *pb.Task, client *Client)
}

type TaskRunnerStats interface {
	TaskRunner
	GetRunningTasks() int32
	GetRunningExecutionIds() []string
	UpdateCapacity(newCapacity int32) error
}

type Client struct {
	conn          *grpc.ClientConn
	client        pb.ExecutorServiceClient
	stream        pb.ExecutorService_SubscribeTaskClient
	schedulerAddr string
	stopCh        chan struct{}
	taskRunner    TaskRunnerStats
	executorName  string
	mu            sync.RWMutex
}

func NewClient(schedulerAddr string) (*Client, error) {
	conn, err := grpc.Dial(schedulerAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		slog.Warn("initial connection to scheduler failed, will retry", "addr", schedulerAddr, "error", err)
		return &Client{
			schedulerAddr: schedulerAddr,
			stopCh:        make(chan struct{}),
		}, nil
	}

	return &Client{
		conn:          conn,
		client:        pb.NewExecutorServiceClient(conn),
		schedulerAddr: schedulerAddr,
		stopCh:        make(chan struct{}),
	}, nil
}

func (c *Client) ensureConnected() error {
	if c.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		c.mu.RLock()
		executorName := c.executorName
		c.mu.RUnlock()
		_, err := c.client.Heartbeat(ctx, &pb.HeartbeatRequest{
			ExecutorName: executorName,
			CurrentLoad:  0,
		})
		if err == nil {
			return nil
		}
		slog.Warn("connection check failed, reconnecting", "error", err)
	}

	conn, err := grpc.Dial(c.schedulerAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		return err
	}

	c.conn = conn
	c.client = pb.NewExecutorServiceClient(conn)
	return nil
}

func (c *Client) ReportResult(req *pb.ReportTaskResultRequest) error {
	if err := c.ensureConnected(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := c.client.ReportTaskResult(ctx, req)
	return err
}

func (c *Client) ReportLog(req *pb.ReportTaskLogRequest) error {
	if err := c.ensureConnected(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := c.client.ReportTaskLog(ctx, req)
	return err
}

func (c *Client) Subscribe(name, address string, capacity int32, runner TaskRunner) error {
	c.taskRunner = runner.(TaskRunnerStats)

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	currentCapacity := capacity

	for {
		select {
		case <-c.stopCh:
			return nil
		case <-ticker.C:
			if err := c.ensureConnected(); err != nil {
				slog.Warn("failed to connect to scheduler, retrying", "error", err)
				continue
			}

			ctx := context.Background()

			regResp, err := c.client.Register(ctx, &pb.RegisterRequest{
				Name:     name,
				Address:  address,
				Capacity: currentCapacity,
			})
			if err != nil {
				slog.Warn("register failed, retrying", "error", err)
				continue
			}

			if !regResp.Success {
				slog.Warn("register failed from server", "message", regResp.Message)
				continue
			}

			executorName := regResp.ExecutorName
			c.mu.Lock()
			c.executorName = executorName
			c.mu.Unlock()

			slog.Info("executor registered", "executor_name", executorName, "success", regResp.Success, "message", regResp.Message)

			stream, err := c.client.SubscribeTask(ctx, &pb.SubscribeTaskRequest{
				ExecutorName: executorName,
			})
			if err != nil {
				slog.Warn("subscribe failed, retrying", "error", err)
				continue
			}
			c.stream = stream

			go func() {
				heartbeatTicker := time.NewTicker(10 * time.Second)
				defer heartbeatTicker.Stop()
				for {
					select {
					case <-heartbeatTicker.C:
						if c.client != nil {
							currentLoad := int32(0)
							runningExecIds := []string{}
							if c.taskRunner != nil {
								currentLoad = c.taskRunner.GetRunningTasks()
								runningExecIds = c.taskRunner.GetRunningExecutionIds()
							}
							c.mu.RLock()
							execName := c.executorName
							c.mu.RUnlock()
							slog.Info("sending heartbeat",
								"executor_name", execName,
								"current_load", currentLoad,
								"running_executions", len(runningExecIds),
							)
							resp, err := c.client.Heartbeat(context.Background(), &pb.HeartbeatRequest{
								ExecutorName:         execName,
								CurrentLoad:          currentLoad,
								RunningExecutionIds:  runningExecIds,
							})
							if err == nil && resp != nil && resp.TargetCapacity > 0 && resp.TargetCapacity != currentCapacity {
								slog.Info("received capacity update from scheduler",
									"old_capacity", currentCapacity,
									"new_capacity", resp.TargetCapacity)
								if c.taskRunner != nil {
									if err := c.taskRunner.UpdateCapacity(resp.TargetCapacity); err == nil {
										currentCapacity = resp.TargetCapacity
										slog.Info("capacity updated successfully", "new_capacity", currentCapacity)
									} else {
										slog.Error("failed to update capacity", "error", err)
									}
								}
							}
						}
					case <-c.stopCh:
						return
					}
				}
			}()

			slog.Info("waiting for tasks")
			for {
				task, err := stream.Recv()
				if err != nil {
					slog.Error("stream receive error, reconnecting", "error", err)
					break
				}

				slog.Info("task received",
					"task_id", task.TaskId,
					"execution_id", task.ExecutionId,
					"type", task.Type,
				)
				go runner.Execute(context.Background(), task, c)
			}
		}
	}
}

func (c *Client) Close() {
	close(c.stopCh)
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *Client) GetExecutorName() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.executorName
}
