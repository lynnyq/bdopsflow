package handler

import (
	"context"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
)

type TaskServicer interface {
	CreateTask(ctx context.Context, query string, args ...interface{}) (*model.Task, error)
	GetTaskByID(ctx context.Context, id int64) (*model.Task, error)
	ListTasks(ctx context.Context, domainID int64, role string, page, pageSize int) ([]*model.Task, int, error)
	UpdateTask(ctx context.Context, id int64, task *model.Task) error
	DeleteTask(ctx context.Context, id int64) error
	TriggerTask(ctx context.Context, taskID int64) (string, error)
	GetTaskExecutions(ctx context.Context, taskID int64) ([]*model.TaskExecution, error)
	GetTaskLogs(ctx context.Context, executionID string) ([]*model.TaskLog, error)
	ListExecutorsByDomain(ctx context.Context, domainID int64) ([]*model.Executor, error)
	GetDomainName(ctx context.Context, domainID int64) string
}