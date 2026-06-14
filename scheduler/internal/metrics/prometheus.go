package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	// Namespace Prometheus 指标命名空间
	Namespace = "bdopsflow"
	// SubsystemScheduler 调度器子系统
	SubsystemScheduler = "scheduler"
)

// Registry 自定义 Prometheus Registry，不包含默认 Go runtime 指标
var Registry = prometheus.NewRegistry()

// Handler 返回 /metrics 端点使用的 HTTP Handler
func Handler() interface{} {
	return promhttp.HandlerFor(Registry, promhttp.HandlerOpts{})
}

func newCounter(opts prometheus.CounterOpts) prometheus.Counter {
	c := prometheus.NewCounter(opts)
	Registry.MustRegister(c)
	return c
}

func newCounterVec(opts prometheus.CounterOpts, labels []string) *prometheus.CounterVec {
	c := prometheus.NewCounterVec(opts, labels)
	Registry.MustRegister(c)
	return c
}

func newGauge(opts prometheus.GaugeOpts) prometheus.Gauge {
	g := prometheus.NewGauge(opts)
	Registry.MustRegister(g)
	return g
}

func newGaugeVec(opts prometheus.GaugeOpts, labels []string) *prometheus.GaugeVec {
	g := prometheus.NewGaugeVec(opts, labels)
	Registry.MustRegister(g)
	return g
}

func newHistogram(opts prometheus.HistogramOpts) prometheus.Histogram {
	h := prometheus.NewHistogram(opts)
	Registry.MustRegister(h)
	return h
}

func newHistogramVec(opts prometheus.HistogramOpts, labels []string) *prometheus.HistogramVec {
	h := prometheus.NewHistogramVec(opts, labels)
	Registry.MustRegister(h)
	return h
}

// ==================== 调度器集群指标 ====================

var (
	// SchedulerIsLeader 当前节点是否为主调度节点 (1=主节点, 0=从节点)
	SchedulerIsLeader = newGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "is_leader",
		Help:      "Whether this scheduler node is the leader (1=leader, 0=follower)",
	}, []string{"node_id"})
)

// ==================== 任务指标 ====================

var (
	// TasksCreated 已创建的任务总数
	TasksCreated = newCounter(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "tasks_created_total",
		Help:      "Total number of tasks created",
	})

	// TasksTriggered 被触发的任务总数（含手动和 cron）
	TasksTriggered = newCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "tasks_triggered_total",
		Help:      "Total number of task triggers by source",
	}, []string{"source"})

	// TasksCompleted 已完成的任务总数
	TasksCompleted = newCounter(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "tasks_completed_total",
		Help:      "Total number of tasks completed successfully",
	})

	// TasksFailed 已失败的任务总数
	TasksFailed = newCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "tasks_failed_total",
		Help:      "Total number of tasks failed by reason",
	}, []string{"reason"})

	// TasksRunning 当前运行中的任务数
	TasksRunning = newGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "tasks_running",
		Help:      "Number of tasks currently running",
	})

	// TaskDurationSeconds 任务执行耗时分布
	TaskDurationSeconds = newHistogram(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "task_duration_seconds",
		Help:      "Task execution duration in seconds",
		Buckets:   []float64{0.1, 0.5, 1, 5, 10, 30, 60, 120, 300, 600, 1800, 3600},
	})

	// TaskRetries 任务重试次数
	TaskRetries = newCounter(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "task_retries_total",
		Help:      "Total number of task retries",
	})
)

// ==================== 执行器指标 ====================

var (
	// ExecutorsOnline 在线执行器数量
	ExecutorsOnline = newGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "executors_online",
		Help:      "Number of online executors",
	})

	// ExecutorsOffline 离线执行器数量
	ExecutorsOffline = newGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "executors_offline",
		Help:      "Number of offline executors",
	})

	// ExecutorRegistrations 执行器注册次数
	ExecutorRegistrations = newCounter(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "executor_registrations_total",
		Help:      "Total number of executor registrations",
	})

	// ExecutorHeartbeats 执行器心跳次数
	ExecutorHeartbeats = newCounter(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "executor_heartbeats_total",
		Help:      "Total number of executor heartbeats received",
	})
)

// ==================== Webhook 指标 ====================

var (
	// WebhookSent Webhook 发送总数
	WebhookSent = newCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "webhook_sent_total",
		Help:      "Total number of webhook notifications sent",
	}, []string{"status"})

	// WebhookDurationSeconds Webhook 发送耗时
	WebhookDurationSeconds = newHistogram(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "webhook_duration_seconds",
		Help:      "Webhook notification delivery duration in seconds",
		Buckets:   []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
	})
)

// ==================== 数据源查询指标 ====================

var (
	// DatasourceQueries 数据源查询总数
	DatasourceQueries = newCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "datasource_queries_total",
		Help:      "Total number of datasource queries",
	}, []string{"type", "status"})

	// DatasourceQueryDurationSeconds 数据源查询耗时
	DatasourceQueryDurationSeconds = newHistogram(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "datasource_query_duration_seconds",
		Help:      "Datasource query execution duration in seconds",
		Buckets:   []float64{0.01, 0.05, 0.1, 0.5, 1, 5, 10, 30, 60},
	})
)

// ==================== Cron 调度指标 ====================

var (
	// CronTriggers Cron 触发总数
	CronTriggers = newCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "cron_triggers_total",
		Help:      "Total number of cron task triggers",
	}, []string{"status"})
)

// ==================== 清理服务指标 ====================

var (
	// CleanupRuns 清理任务执行次数
	CleanupRuns = newCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "cleanup_runs_total",
		Help:      "Total number of cleanup routine runs",
	}, []string{"type"})

	// CleanupTasksCleaned 清理的卡住任务数量
	CleanupTasksCleaned = newCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "cleanup_tasks_cleaned_total",
		Help:      "Total number of stuck tasks cleaned",
	}, []string{"reason"})

	// CleanupExecutorsMarkedOffline 标记为离线的执行器数量
	CleanupExecutorsMarkedOffline = newCounter(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "cleanup_executors_offline_total",
		Help:      "Total number of executors marked as offline",
	})

	// CleanupLocksCleaned 清理的过期锁数量
	CleanupLocksCleaned = newCounter(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "cleanup_locks_cleaned_total",
		Help:      "Total number of stale locks cleaned",
	})

	// CleanupErrors 清理服务错误次数
	CleanupErrors = newCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "cleanup_errors_total",
		Help:      "Total number of cleanup errors",
	}, []string{"type"})

	// CleanupDurationSeconds 清理任务执行耗时
	CleanupDurationSeconds = newHistogram(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "cleanup_duration_seconds",
		Help:      "Cleanup routine execution duration in seconds",
		Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
	})
)

// ==================== 认证指标 ====================

var (
	// AuthAttempts 认证尝试总数
	AuthAttempts = newCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "auth_attempts_total",
		Help:      "Total number of authentication attempts",
	}, []string{"method", "status"})
)

// ==================== HTTP 请求指标 ====================

var (
	// HTTPRequestsTotal HTTP 请求总数
	HTTPRequestsTotal = newCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "http_requests_total",
		Help:      "Total number of HTTP requests",
	}, []string{"method", "path", "status"})

	// HTTPRequestDurationSeconds HTTP 请求耗时
	HTTPRequestDurationSeconds = newHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "http_request_duration_seconds",
		Help:      "HTTP request duration in seconds",
		Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"method", "path"})

	// HTTPRequestSizeBytes HTTP 请求大小
	HTTPRequestSizeBytes = newHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "http_request_size_bytes",
		Help:      "HTTP request size in bytes",
		Buckets:   prometheus.ExponentialBuckets(100, 10, 7),
	}, []string{"method", "path"})

	// HTTPResponseSizeBytes HTTP 响应大小
	HTTPResponseSizeBytes = newHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "http_response_size_bytes",
		Help:      "HTTP response size in bytes",
		Buckets:   prometheus.ExponentialBuckets(100, 10, 7),
	}, []string{"method", "path"})
)

// ==================== 缓存指标 ====================

var (
	// CacheHits 缓存命中次数
	CacheHits = newCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "cache_hits_total",
		Help:      "Total number of cache hits",
	}, []string{"cache_type"})

	// CacheMisses 缓存未命中次数
	CacheMisses = newCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "cache_misses_total",
		Help:      "Total number of cache misses",
	}, []string{"cache_type"})

	// CacheSize 缓存大小
	CacheSize = newGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "cache_size",
		Help:      "Current cache size",
	}, []string{"cache_type"})
)

// ==================== 数据库连接池指标 ====================

var (
	// DBPoolConnections 数据库连接池连接数
	DBPoolConnections = newGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "db_pool_connections",
		Help:      "Number of database pool connections",
	}, []string{"pool_type", "state"})

	// DBPoolWaitCount 数据库连接池等待次数
	DBPoolWaitCount = newCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "db_pool_wait_total",
		Help:      "Total number of times waited for a connection",
	}, []string{"pool_type"})

	// DBPoolWaitDurationSeconds 数据库连接池等待耗时
	DBPoolWaitDurationSeconds = newHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "db_pool_wait_duration_seconds",
		Help:      "Time spent waiting for a connection in seconds",
		Buckets:   []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
	}, []string{"pool_type"})
)

// ==================== gRPC 指标 ====================

var (
	// GRPCRequestsTotal gRPC 请求总数
	GRPCRequestsTotal = newCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "grpc_requests_total",
		Help:      "Total number of gRPC requests",
	}, []string{"method", "status"})

	// GRPCRequestDurationSeconds gRPC 请求耗时
	GRPCRequestDurationSeconds = newHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "grpc_request_duration_seconds",
		Help:      "gRPC request duration in seconds",
		Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
	}, []string{"method"})
)

// ==================== 系统资源指标 ====================

var (
	// Goroutines 当前 goroutine 数量
	Goroutines = newGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "goroutines",
		Help:      "Number of goroutines",
	})

	// MemoryUsageBytes 内存使用量
	MemoryUsageBytes = newGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: SubsystemScheduler,
		Name:      "memory_usage_bytes",
		Help:      "Memory usage in bytes",
	}, []string{"type"})
)

// SetLeaderStatus 设置当前节点的 leader 状态指标
func SetLeaderStatus(nodeID string, isLeader bool) {
	value := float64(0)
	if isLeader {
		value = 1
	}
	SchedulerIsLeader.WithLabelValues(nodeID).Set(value)
}
