package grpcclient

import (
	"context"
	"log/slog"
	"time"

	pb "github.com/lynnyq/bdopsflow/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn          *grpc.ClientConn
	client        pb.ExecutorServiceClient
	stream        pb.ExecutorService_SubscribeTaskClient
	schedulerAddr string
	stopCh        chan struct{}
}

func NewClient(schedulerAddr string) (*Client, error) {
	// 使用异步连接，不阻塞启动，不等待连接成功
	conn, err := grpc.Dial(schedulerAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// 不使用 WithBlock，让连接在后台建立
	)
	if err != nil {
		// 即使连接失败也不返回错误，后续会自动重连
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
		// 简单检查连接是否还活着
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, err := c.client.Heartbeat(ctx, &pb.HeartbeatRequest{
			ExecutorId:  "check",
			CurrentLoad: 0,
		})
		if err == nil {
			return nil
		}
		slog.Warn("connection check failed, reconnecting", "error", err)
	}

	// 尝试重新连接
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

type TaskRunner interface {
	Execute(ctx context.Context, task *pb.Task, client *Client)
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

func (c *Client) Subscribe(executorID, name, address string, capacity int32, runner TaskRunner) error {
	// 使用一个 ticker 来处理重连
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

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
				ExecutorId: executorID,
				Name:       name,
				Address:    address,
				Capacity:   capacity,
			})
			if err != nil {
				slog.Warn("register failed, retrying", "error", err)
				continue
			}
			slog.Info("executor registered", "success", regResp.Success, "message", regResp.Message)

			stream, err := c.client.SubscribeTask(ctx, &pb.SubscribeTaskRequest{
				ExecutorId: executorID,
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
							c.client.Heartbeat(context.Background(), &pb.HeartbeatRequest{
								ExecutorId:  executorID,
								CurrentLoad: 0,
							})
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