package model

import (
	"time"

	rqlite "github.com/rqlite/gorqlite"
)

type User struct {
	ID            int64      `db:"id" json:"id"`
	Username      string     `db:"username" json:"username"`
	Password      string     `db:"password" json:"-"`
	RealName      string     `db:"real_name" json:"real_name"`
	Phone         string     `db:"phone" json:"phone"`
	Email         string     `db:"email" json:"email"`
	IsActive      bool       `db:"is_active" json:"is_active"`
	LastLoginAt   *time.Time `db:"last_login_at" json:"last_login_at"`
	CreatedBy     *int64     `db:"created_by" json:"created_by,omitempty"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at" json:"updated_at"`
	RoleIDs       []int64    `db:"-" json:"role_ids,omitempty"`
	DomainIDs     []int64    `db:"-" json:"domain_ids,omitempty"`
	DomainNames   []string   `db:"-" json:"domain_names,omitempty"`
	RoleCodes     []string   `db:"-" json:"role_codes,omitempty"`
}

type Domain struct {
	ID          int64     `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

type Workflow struct {
	ID             int64     `db:"id" json:"id"`
	Name           string    `db:"name" json:"name"`
	Description    string    `db:"description" json:"description"`
	DomainID       int64     `db:"domain_id" json:"domain_id"`
	DAGConfig      string    `db:"dag_config" json:"dag_config"`
	CronExpression string    `db:"cron_expression" json:"cron_expression"`
	IsEnabled      bool      `db:"is_enabled" json:"is_enabled"`
	CreatedBy      *int64    `db:"created_by" json:"created_by"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
}

type Task struct {
	ID                  int64     `db:"id" json:"id"`
	WorkflowID          *int64    `db:"workflow_id" json:"workflow_id"`
	Name                string    `db:"name" json:"name"`
	Type                string    `db:"type" json:"type"`
	Config              string    `db:"config" json:"config"`
	CronExpression      string    `db:"cron_expression" json:"cron_expression"`
	TimeoutSeconds      int32     `db:"timeout_seconds" json:"timeout_seconds"`
	RetryCount          int32     `db:"retry_count" json:"retry_count"`
	RetryInterval       int32     `db:"retry_interval" json:"retry_interval"`
	IsEnabled           bool      `db:"is_enabled" json:"is_enabled"`
	Status              string    `db:"status" json:"status"`
	DomainID            int64     `db:"domain_id" json:"domain_id"`
	WebhookID     *int64  `db:"webhook_id" json:"webhook_id"`
	WebhookEvents string  `db:"webhook_events" json:"webhook_events"`
	AssignedExecutorID  int64     `db:"assigned_executor_id" json:"assigned_executor_id"`
	CreatedBy           int64     `db:"created_by" json:"created_by"`
	CreatedByName       string    `db:"-" json:"created_by_name"`
	CreatedAt           time.Time `db:"created_at" json:"created_at"`
	UpdatedAt           time.Time `db:"updated_at" json:"updated_at"`
	NextExecutionTime   string    `db:"-" json:"next_execution_time"`
	LastExecutionStatus string    `db:"-" json:"last_execution_status"`
}

type TaskExecution struct {
	ID          int64           `db:"id" json:"id"`
	TaskID      int64           `db:"task_id" json:"task_id"`
	ExecutionID string          `db:"execution_id" json:"execution_id"`
	ExecutorID  int64           `db:"executor_id" json:"executor_id"`
	Status      string          `db:"status" json:"status"`
	StartTime   rqlite.NullTime `db:"start_time" json:"start_time"`
	EndTime     rqlite.NullTime `db:"end_time" json:"end_time"`
	Output      string          `db:"output" json:"output"`
	Error       string          `db:"error" json:"error"`
	RetryTimes  int32           `db:"retry_times" json:"retry_times"`
	CreatedAt   time.Time       `db:"created_at" json:"created_at"`
	Progress    int32           `db:"progress" json:"progress"`
	ProgressMsg string          `db:"progress_msg" json:"progress_msg"`
	UpdatedAt   time.Time       `db:"updated_at" json:"updated_at"`
}

func (te *TaskExecution) GetStartTime() *time.Time {
	if te.StartTime.Valid {
		return &te.StartTime.Time
	}
	return nil
}

func (te *TaskExecution) GetEndTime() *time.Time {
	if te.EndTime.Valid {
		return &te.EndTime.Time
	}
	return nil
}

type Executor struct {
	ID            int64           `db:"id" json:"id"`
	Name          string          `db:"name" json:"name"`
	Address       string          `db:"address" json:"address"`
	Status        string          `db:"status" json:"status"`
	LastHeartbeat rqlite.NullTime `db:"last_heartbeat" json:"last_heartbeat"`
	Capacity      int64           `db:"capacity" json:"capacity"`
	CurrentLoad   int64           `db:"current_load" json:"current_load"`
	IsGlobal      bool            `db:"is_global" json:"is_global"`
	CreatedAt     time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time       `db:"updated_at" json:"updated_at"`
}

func (e *Executor) GetLastHeartbeat() *time.Time {
	if e.LastHeartbeat.Valid {
		return &e.LastHeartbeat.Time
	}
	return nil
}

type WorkflowExecution struct {
	ID         int64           `db:"id" json:"id"`
	WorkflowID int64           `db:"workflow_id" json:"workflow_id"`
	ExecutionID string         `db:"execution_id" json:"execution_id"`
	Status     string         `db:"status" json:"status"`
	StartTime  rqlite.NullTime `db:"start_time" json:"start_time"`
	EndTime    rqlite.NullTime `db:"end_time" json:"end_time"`
	NodeStates string         `db:"node_states" json:"node_states"`
	CreatedAt  time.Time      `db:"created_at" json:"created_at"`
}

func (we *WorkflowExecution) GetStartTime() *time.Time {
	if we.StartTime.Valid {
		return &we.StartTime.Time
	}
	return nil
}

func (we *WorkflowExecution) GetEndTime() *time.Time {
	if we.EndTime.Valid {
		return &we.EndTime.Time
	}
	return nil
}

type TaskLog struct {
	ID          int64     `db:"id" json:"id"`
	ExecutionID string    `db:"execution_id" json:"execution_id"`
	TaskID      int64     `db:"task_id" json:"task_id"`
	ExecutorID  int64     `db:"executor_id" json:"executor_id"`
	NodeID      string    `db:"node_id" json:"node_id"`
	LogLevel    string    `db:"log_level" json:"log_level"`
	Message     string    `db:"message" json:"message"`
	LogTime     time.Time `db:"log_time" json:"log_time"`
}

type Webhook struct {
	ID          int64     `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	URL         string    `db:"url" json:"url"`
	Method      string    `db:"method" json:"method"`
	Headers     string    `db:"headers" json:"headers"`
	Secret      string    `db:"secret" json:"secret,omitempty"`
	DomainID    int64     `db:"domain_id" json:"domain_id"`
	IsEnabled   bool      `db:"is_enabled" json:"is_enabled"`
	Description string    `db:"description" json:"description"`
	CreatedBy   *int64    `db:"created_by" json:"created_by,omitempty"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}
