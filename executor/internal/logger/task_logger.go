package logger

import (
	"context"
	"fmt"
	"time"

	pb "github.com/lynnyq/bdopsflow/proto"
)

type TaskLogger struct {
	executionID string
	taskID      int64
	client      LogReporter
}

type LogReporter interface {
	ReportLog(ctx context.Context, req *pb.ReportTaskLogRequest) error
}

type GRPCLogReporter struct {
	client pb.ExecutorServiceClient
}

func NewGRPCLogReporter(client pb.ExecutorServiceClient) *GRPCLogReporter {
	return &GRPCLogReporter{client: client}
}

func (r *GRPCLogReporter) ReportLog(ctx context.Context, req *pb.ReportTaskLogRequest) error {
	_, err := r.client.ReportTaskLog(ctx, req)
	return err
}

func NewTaskLogger(executionID string, taskID int64, client LogReporter) *TaskLogger {
	return &TaskLogger{
		executionID: executionID,
		taskID:      taskID,
		client:      client,
	}
}

func (l *TaskLogger) Info(message string) {
	l.log("info", message)
}

func (l *TaskLogger) Error(message string) {
	l.log("error", message)
}

func (l *TaskLogger) Debug(message string) {
	l.log("debug", message)
}

func (l *TaskLogger) log(level, message string) {
	fmt.Printf("[%s] [%s] %s\n", level, l.executionID, message)

	if l.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_ = l.client.ReportLog(ctx, &pb.ReportTaskLogRequest{
			ExecutionId: l.executionID,
			TaskId:      l.taskID,
			LogLevel:    level,
			LogContent:  message,
			Timestamp:   time.Now().Unix(),
		})
	}
}

func (l *TaskLogger) Infof(format string, args ...interface{}) {
	l.Info(fmt.Sprintf(format, args...))
}

func (l *TaskLogger) Errorf(format string, args ...interface{}) {
	l.Error(fmt.Sprintf(format, args...))
}

func (l *TaskLogger) Debugf(format string, args ...interface{}) {
	l.Debug(fmt.Sprintf(format, args...))
}
