package grpcclient

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/lynnyq/bdopsflow/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type TaskRunner interface {
	Execute(ctx context.Context, task *pb.Task, client *MultiClient)
}

type TaskRunnerStats interface {
	TaskRunner
	GetRunningTasks() int32
	GetRunningExecutionIds() []string
	GetRunningTaskStates() []*pb.RunningTaskState // 新增
	UpdateCapacity(newCapacity int32) error
}

// 包装结构体用于避免 atomic.Value 存储 nil 导致的 panic
type connWrapper struct {
	conn *grpc.ClientConn
}

type clientWrapper struct {
	client pb.ExecutorServiceClient
}

type streamWrapper struct {
	stream pb.ExecutorService_SubscribeTaskClient
}

type taskRunnerWrapper struct {
	runner TaskRunnerStats
}

type nameWrapper struct {
	name string
}

type MultiClient struct {
	schedulerAddrs []string
	currentIndex   atomic.Int64
	conn           atomic.Value // stores connWrapper
	client         atomic.Value // stores clientWrapper
	stream         atomic.Value // stores streamWrapper
	stopCh         chan struct{}
	reconnectCh    chan struct{} // 触发重连的信号通道
	taskRunner     atomic.Value // stores taskRunnerWrapper
	executorName   atomic.Value // stores nameWrapper
	mu             sync.RWMutex
	isConnected    atomic.Bool
	connectMu      sync.Mutex
	lastSchedulerId atomic.Value // stores nameWrapper
	needFullSync    atomic.Bool   // 是否需要在下一次心跳发送完整状态
}

func NewMultiClient(schedulerAddrs []string) (*MultiClient, error) {
	if len(schedulerAddrs) == 0 {
		return nil, nil
	}

	client := &MultiClient{
		schedulerAddrs: schedulerAddrs,
		stopCh:         make(chan struct{}),
		reconnectCh:    make(chan struct{}, 1),
	}

	// 初始化 atomic.Value，使用包装结构体，避免 nil 问题
	client.conn.Store(connWrapper{})
	client.client.Store(clientWrapper{})
	client.stream.Store(streamWrapper{})
	client.taskRunner.Store(taskRunnerWrapper{})
	client.executorName.Store(nameWrapper{})
	client.lastSchedulerId.Store(nameWrapper{})

	slog.Info("multi-client created",
		"scheduler_count", len(schedulerAddrs),
		"addresses", schedulerAddrs,
	)

	return client, nil
}

func (c *MultiClient) getCurrentAddr() string {
	idx := c.currentIndex.Load()
	return c.schedulerAddrs[idx%int64(len(c.schedulerAddrs))]
}

func (c *MultiClient) nextAddr() {
	c.currentIndex.Add(1)
}

func (c *MultiClient) connect(addr string) (*grpc.ClientConn, pb.ExecutorServiceClient, error) {
	conn, err := grpc.Dial(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		return nil, nil, err
	}
	return conn, pb.NewExecutorServiceClient(conn), nil
}

func (c *MultiClient) connectToScheduler() (string, error) {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()

	// 保存第一个可用的非主节点连接，以防万一没有找到主节点
	var fallbackConn *grpc.ClientConn
	var fallbackClient pb.ExecutorServiceClient
	var fallbackAddr string

	// 第一遍：优先寻找主节点
	for i := 0; i < len(c.schedulerAddrs); i++ {
		addr := c.getCurrentAddr()
		slog.Info("attempting to connect to scheduler", "addr", addr, "attempt", c.currentIndex.Load()+1)

		conn, client, err := c.connect(addr)
		if err != nil {
			slog.Warn("failed to connect to scheduler, trying next",
				"addr", addr,
				"error", err,
			)
			c.nextAddr()
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		resp, err := client.Heartbeat(ctx, &pb.HeartbeatRequest{
			ExecutorName: "",
			CurrentLoad:  0,
		})
		if err != nil {
			slog.Warn("scheduler not responding, trying next",
				"addr", addr,
				"error", err,
			)
			conn.Close()
			c.nextAddr()
			continue
		}

		// 检查这个节点是否是当前的主节点
		if resp != nil && resp.IsLeader {
			// 找到主节点，立即使用
			c.conn.Store(connWrapper{conn: conn})
			c.client.Store(clientWrapper{client: client})
			c.isConnected.Store(true)

			slog.Info("successfully connected to LEADER scheduler", "addr", addr, "scheduler_node_id", resp.SchedulerNodeId)
			return addr, nil
		}

		// 不是主节点，保存为备用
		slog.Info("connected to non-leader scheduler, keeping as fallback", "addr", addr, "scheduler_node_id", resp.SchedulerNodeId)
		if fallbackConn == nil {
			fallbackConn = conn
			fallbackClient = client
			fallbackAddr = addr
		} else {
			conn.Close()
		}
		c.nextAddr()
	}

	// 如果没有找到主节点，使用备用节点
	if fallbackConn != nil {
		c.conn.Store(connWrapper{conn: fallbackConn})
		c.client.Store(clientWrapper{client: fallbackClient})
		c.isConnected.Store(true)

		slog.Warn("no leader found, using fallback scheduler", "addr", fallbackAddr)
		return fallbackAddr, nil
	}

	c.isConnected.Store(false)
	return "", nil
}

func (c *MultiClient) ensureConnected() error {
	if c.isConnected.Load() {
		var client pb.ExecutorServiceClient
		loadedWrapper := c.client.Load()
		if wrapper, ok := loadedWrapper.(clientWrapper); ok {
			client = wrapper.client
		}

		if client != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			var executorName string
			nameWrapper := c.executorName.Load().(nameWrapper)
			executorName = nameWrapper.name

			_, err := client.Heartbeat(ctx, &pb.HeartbeatRequest{
				ExecutorName: executorName,
				CurrentLoad:  0,
			})
			if err == nil {
				return nil
			}
			slog.Warn("connection check failed, reconnecting", "error", err)
		}
	}

	_, err := c.connectToScheduler()
	return err
}

func (c *MultiClient) reconnect() {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()

	oldWrapper := c.conn.Load()
	if wrapper, ok := oldWrapper.(connWrapper); ok && wrapper.conn != nil {
		wrapper.conn.Close()
	}

	// 保持包装结构不变，只清空内部值，避免 atomic.Value panic
	c.conn.Store(connWrapper{})
	c.client.Store(clientWrapper{})
	c.stream.Store(streamWrapper{})
	c.isConnected.Store(false)

	c.nextAddr()
}

func (c *MultiClient) ReportResult(req *pb.ReportTaskResultRequest) error {
	if err := c.ensureConnected(); err != nil {
		return err
	}

	var client pb.ExecutorServiceClient
	loadedWrapper := c.client.Load()
	if wrapper, ok := loadedWrapper.(clientWrapper); ok {
		client = wrapper.client
	}

	if client == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.ReportTaskResult(ctx, req)
	return err
}

func (c *MultiClient) ReportLog(req *pb.ReportTaskLogRequest) error {
	if err := c.ensureConnected(); err != nil {
		return err
	}

	var client pb.ExecutorServiceClient
	loadedWrapper := c.client.Load()
	if wrapper, ok := loadedWrapper.(clientWrapper); ok {
		client = wrapper.client
	}

	if client == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.ReportTaskLog(ctx, req)
	return err
}

func (c *MultiClient) Subscribe(name, address string, capacity int32, runner TaskRunner) error {
	c.taskRunner.Store(taskRunnerWrapper{runner: runner.(TaskRunnerStats)})

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	currentCapacity := capacity

	for {
		select {
		case <-c.stopCh:
			return nil
		case <-ticker.C:
			if err := c.ensureConnected(); err != nil {
				slog.Warn("failed to connect to any scheduler, retrying", "error", err)
				c.reconnect()
				continue
			}

			ctx := context.Background()

			var client pb.ExecutorServiceClient
			loadedWrapper := c.client.Load()
			if wrapper, ok := loadedWrapper.(clientWrapper); ok {
				client = wrapper.client
			}

			if client == nil {
				c.reconnect()
				continue
			}

			regResp, err := client.Register(ctx, &pb.RegisterRequest{
				Name:     name,
				Address:  address,
				Capacity: currentCapacity,
			})
			if err != nil {
				slog.Warn("register failed, reconnecting", "error", err)
				c.reconnect()
				continue
			}

			if !regResp.Success {
				slog.Warn("register failed from server", "message", regResp.Message)
				c.reconnect()
				continue
			}

			executorName := regResp.ExecutorName
			c.executorName.Store(nameWrapper{name: executorName})

			slog.Info("executor registered",
				"executor_name", executorName,
				"success", regResp.Success,
				"message", regResp.Message,
				"scheduler", c.getCurrentAddr(),
			)

			stream, err := client.SubscribeTask(ctx, &pb.SubscribeTaskRequest{
				ExecutorName: executorName,
			})

			if err != nil {
				slog.Warn("subscribe failed, reconnecting", "error", err)
				c.reconnect()
				continue
			}
			c.stream.Store(streamWrapper{stream: stream})

			go c.heartbeatLoop(executorName, currentCapacity)

			slog.Info("task subscription started, waiting for tasks")
			
			// 启动一个 goroutine 来接收任务
			taskCh := make(chan *pb.Task, 10)
			errCh := make(chan error, 1)
			go func() {
				for {
					var recvStream pb.ExecutorService_SubscribeTaskClient
					loadedStreamWrapper := c.stream.Load()
					if wrapper, ok := loadedStreamWrapper.(streamWrapper); ok {
						recvStream = wrapper.stream
					}

					if recvStream == nil {
						slog.Warn("stream lost in receiver goroutine")
						errCh <- nil
						return
					}

					task, err := recvStream.Recv()
					if err != nil {
						slog.Error("stream receive error", "error", err)
						errCh <- err
						return
					}

					select {
					case taskCh <- task:
					case <-c.stopCh:
						return
					}
				}
			}()

			// 内部循环处理消息和重连信号
			innerLoop:
			for {
				select {
				case <-c.reconnectCh:
					slog.Info("received reconnect signal, reconnecting")
					c.reconnect()
					break innerLoop

				case err := <-errCh:
					slog.Error("stream error, reconnecting", "error", err)
					c.reconnect()
					break innerLoop

				case task := <-taskCh:
					slog.Info("task received",
						"task_id", task.TaskId,
						"execution_id", task.ExecutionId,
						"type", task.Type,
					)
					go runner.Execute(context.Background(), task, c)

				case <-c.stopCh:
					break innerLoop
				}
			}
		}
	}
}

func (c *MultiClient) heartbeatLoop(executorName string, currentCapacity int32) {
	heartbeatTicker := time.NewTicker(10 * time.Second)
	defer heartbeatTicker.Stop()

	var isReconnect bool = true // 首次心跳标记为重连

	for {
		select {
		case <-heartbeatTicker.C:
			if !c.isConnected.Load() {
				return
			}

			var client pb.ExecutorServiceClient
			loadedWrapper := c.client.Load()
			if wrapper, ok := loadedWrapper.(clientWrapper); ok {
				client = wrapper.client
			}

			if client == nil {
				return
			}

			var taskRunner TaskRunnerStats
			runnerWrapper := c.taskRunner.Load().(taskRunnerWrapper)
			taskRunner = runnerWrapper.runner

			currentLoad := int32(0)
			runningExecIds := []string{}
			var runningTasks []*pb.RunningTaskState

			// 检查是否需要发送完整任务状态
			needFullSync := c.needFullSync.Load()

			if taskRunner != nil {
				currentLoad = taskRunner.GetRunningTasks()
				runningExecIds = taskRunner.GetRunningExecutionIds()
				// 如果需要全量同步或者是重连，获取详细任务状态
				if needFullSync || isReconnect {
					runningTasks = taskRunner.GetRunningTaskStates()
				}
			}

			slog.Debug("sending heartbeat",
				"executor_name", executorName,
				"current_load", currentLoad,
				"running_executions", len(runningExecIds),
				"is_reconnect", isReconnect,
				"need_full_sync", needFullSync,
			)

			resp, err := client.Heartbeat(context.Background(), &pb.HeartbeatRequest{
				ExecutorName:         executorName,
				CurrentLoad:          currentLoad,
				RunningExecutionIds:  runningExecIds,
				RunningTasks:         runningTasks,
				IsReconnect:          isReconnect,
			})

			if err != nil {
				slog.Warn("heartbeat failed", "error", err)
				isReconnect = true // 下次心跳为重连
				return
			}

			// 处理响应
			if resp != nil {
				// 检查调度器是否变化
				if resp.SchedulerNodeId != "" {
					lastWrapper := c.lastSchedulerId.Load().(nameWrapper)
					if lastWrapper.name != resp.SchedulerNodeId {
						slog.Info("detected scheduler change, will reconnect",
							"old_scheduler", lastWrapper.name,
							"new_scheduler", resp.SchedulerNodeId,
						)
						c.lastSchedulerId.Store(nameWrapper{name: resp.SchedulerNodeId})
						// 发送重连信号
						select {
						case c.reconnectCh <- struct{}{}:
						default:
						}
						return // 退出让主循环重连
					} else {
						isReconnect = false
					}
				}

				// 更新是否需要全量同步标志
				c.needFullSync.Store(resp.NeedFullSync)

				// 处理容量更新
				if resp.TargetCapacity > 0 && resp.TargetCapacity != currentCapacity {
					slog.Info("received capacity update from scheduler",
						"old_capacity", currentCapacity,
						"new_capacity", resp.TargetCapacity,
					)
					if taskRunner != nil {
						if err := taskRunner.UpdateCapacity(resp.TargetCapacity); err == nil {
							currentCapacity = resp.TargetCapacity
							slog.Info("capacity updated successfully", "new_capacity", currentCapacity)
						} else {
							slog.Error("failed to update capacity", "error", err)
						}
					}
				}
			} else {
				isReconnect = false
			}

		case <-c.stopCh:
			return
		}
	}
}

// 新增：同步正在运行的任务
func (c *MultiClient) SyncRunningTasks(executorName string) ([]*pb.RunningTaskState, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}

	var client pb.ExecutorServiceClient
	loadedWrapper := c.client.Load()
	if wrapper, ok := loadedWrapper.(clientWrapper); ok {
		client = wrapper.client
	}

	if client == nil {
		return nil, fmt.Errorf("no client available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.SyncRunningTasks(ctx, &pb.SyncRunningTasksRequest{
		ExecutorName: executorName,
	})

	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("sync failed: %s", resp.Message)
	}

	return resp.RunningTasks, nil
}

func (c *MultiClient) Close() {
	close(c.stopCh)

	oldWrapper := c.conn.Load()
	if wrapper, ok := oldWrapper.(connWrapper); ok && wrapper.conn != nil {
		wrapper.conn.Close()
	}
}

func (c *MultiClient) GetExecutorName() string {
	nameWrapper := c.executorName.Load().(nameWrapper)
	return nameWrapper.name
}
