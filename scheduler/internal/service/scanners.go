package service

import (
	"fmt"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	rqlite "github.com/rqlite/gorqlite"
)

func scanTaskResult(qr *rqlite.QueryResult, task *model.Task) error {
	row, err := qr.Slice()
	if err != nil {
		return err
	}

	task.ID = rowInt64(row[0])
	if v := rowInt64(row[1]); v > 0 {
		task.WorkflowID = &v
	}
	task.Name = rowString(row[2])
	task.Type = rowString(row[3])
	task.Config = rowString(row[4])
	task.CronExpression = rowString(row[5])
	task.TimeoutSeconds = int32(rowInt64(row[6]))
	task.RetryCount = int32(rowInt64(row[7]))
	task.RetryInterval = int32(rowInt64(row[8]))
	task.IsEnabled = rowBool(row[9])
	task.Status = rowString(row[10])
	task.DomainID = rowInt64(row[11])
	if !isEmpty(row[12]) {
		webhookID := rowInt64(row[12])
		task.WebhookID = &webhookID
	}
	task.WebhookEvents = rowString(row[13])
	task.AssignedExecutorID = rowInt64(row[14])
	task.CreatedBy = rowInt64(row[15])
	task.CreatedAt = parseDateTime(row[16])
	task.UpdatedAt = parseDateTime(row[17])
	return nil
}

func scanWorkflowResult(qr *rqlite.QueryResult, wf *model.Workflow) error {
	row, err := qr.Slice()
	if err != nil {
		return err
	}

	wf.ID = rowInt64(row[0])
	wf.Name = rowString(row[1])
	wf.Description = rowString(row[2])
	wf.DomainID = rowInt64(row[3])
	wf.DAGConfig = rowString(row[4])
	wf.CronExpression = rowString(row[5])
	wf.IsEnabled = rowBool(row[6])

	if v := rowInt64(row[7]); v > 0 {
		wf.CreatedBy = &v
	}

	wf.CreatedAt = parseDateTime(row[8])
	wf.UpdatedAt = parseDateTime(row[9])
	return nil
}

func scanExecutorResult(qr *rqlite.QueryResult, exec *model.Executor) error {
	row, err := qr.Slice()
	if err != nil {
		return err
	}
	exec.ID = rowInt64(row[0])
	exec.Name = rowString(row[1])
	exec.Address = rowString(row[2])
	exec.Status = rowString(row[3])
	if t := parseDateTime(row[4]); !t.IsZero() {
		exec.LastHeartbeat = rqlite.NullTime{Time: t, Valid: true}
	}
	exec.Capacity = rowInt64(row[5])
	exec.CurrentLoad = rowInt64(row[6])
	exec.CreatedAt = parseDateTime(row[7])
	exec.UpdatedAt = parseDateTime(row[8])
	return nil
}

func scanExecutionResult(qr *rqlite.QueryResult, exec *model.TaskExecution) error {
	row, err := qr.Slice()
	if err != nil {
		return err
	}

	exec.ID = rowInt64(row[0])
	exec.TaskID = rowInt64(row[1])
	exec.ExecutionID = rowString(row[2])
	exec.ExecutorID = rowInt64(row[3])
	exec.Status = rowString(row[4])

	if t := parseDateTime(row[5]); !t.IsZero() {
		exec.StartTime = rqlite.NullTime{Time: t, Valid: true}
	}

	if t := parseDateTime(row[6]); !t.IsZero() {
		exec.EndTime = rqlite.NullTime{Time: t, Valid: true}
	}

	exec.Output = rowString(row[7])
	exec.Error = rowString(row[8])
	exec.RetryTimes = int32(rowInt64(row[9]))

	exec.CreatedAt = parseDateTime(row[10])

	if len(row) > 11 {
		exec.Progress = int32(rowInt64(row[11]))
	}
	if len(row) > 12 {
		exec.ProgressMsg = rowString(row[12])
	}
	if len(row) > 13 {
		exec.UpdatedAt = parseDateTime(row[13])
	}

	return nil
}

func scanWorkflowExecutionResult(qr *rqlite.QueryResult, we *model.WorkflowExecution) error {
	row, err := qr.Slice()
	if err != nil {
		return err
	}
	we.ID = rowInt64(row[0])
	we.WorkflowID = rowInt64(row[1])
	we.ExecutionID = rowString(row[2])
	we.Status = rowString(row[3])

	if t := parseDateTime(row[4]); !t.IsZero() {
		we.StartTime = rqlite.NullTime{Time: t, Valid: true}
	}

	if t := parseDateTime(row[5]); !t.IsZero() {
		we.EndTime = rqlite.NullTime{Time: t, Valid: true}
	}

	we.NodeStates = rowString(row[6])
	we.CreatedAt = parseDateTime(row[7])

	return nil
}

func scanTaskLogResult(qr *rqlite.QueryResult, tl *model.TaskLog) error {
	row, err := qr.Slice()
	if err != nil {
		return err
	}
	tl.ID = rowInt64(row[0])
	tl.ExecutionID = rowString(row[1])
	tl.TaskID = rowInt64(row[2])
	tl.ExecutorID = rowInt64(row[3])
	tl.NodeID = rowString(row[4])
	tl.LogLevel = rowString(row[5])
	tl.Message = rowString(row[6])
	tl.LogTime = parseDateTime(row[7])

	return nil
}

func rowInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case float64:
		return int64(val)
	case string:
		var n int64
		fmt.Sscanf(val, "%d", &n)
		return n
	}
	return 0
}

func RowInt64(v interface{}) int64 {
	return rowInt64(v)
}

func rowInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case string:
		var n int
		fmt.Sscanf(val, "%d", &n)
		return n
	}
	return 0
}

func rowString(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

func RowString(v interface{}) string {
	return rowString(v)
}

func rowBool(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case int64:
		return val != 0
	case float64:
		return val != 0
	}
	return false
}

func RowBool(v interface{}) bool {
	return rowBool(v)
}

func rowFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	case string:
		var n float64
		fmt.Sscanf(val, "%f", &n)
		return n
	}
	return 0
}
