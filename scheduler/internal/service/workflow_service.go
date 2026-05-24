package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/dag"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	rqlite "github.com/rqlite/gorqlite"
)

func (s *SchedulerService) GetWorkflow(ctx context.Context, id int64) (*model.Workflow, error) {
	query := `
		SELECT id, name, description, domain_id, dag_config, cron_expression,
		       is_enabled, created_by, created_at, updated_at
		FROM bdopsflow_workflows WHERE id = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{id},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	if !qr.Next() {
		return nil, fmt.Errorf("workflow not found")
	}

	wf := &model.Workflow{}
	if err := scanWorkflowResult(&qr, wf); err != nil {
		return nil, err
	}

	return wf, nil
}

func (s *SchedulerService) ListWorkflows(ctx context.Context, domainID int64, role string, page, pageSize int) ([]*model.Workflow, int, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	if page <= 0 {
		page = 1
	}

	isSystemAdmin := role == "system_admin" || role == "admin"

	var whereClause string
	var args []interface{}

	if isSystemAdmin {
		whereClause = ""
	} else {
		whereClause = " WHERE domain_id = ?"
		args = append(args, domainID)
	}

	countQuery := "SELECT COUNT(*) FROM bdopsflow_workflows" + whereClause
	var countQr rqlite.QueryResult
	var err error

	if len(args) > 0 {
		countStmt := rqlite.ParameterizedStatement{
			Query:     countQuery,
			Arguments: args,
		}
		countQr, err = s.DB.QueryOneParameterized(countStmt)
	} else {
		countQr, err = s.DB.QueryOne(countQuery)
	}

	if err != nil {
		return nil, 0, err
	}
	if countQr.Err != nil {
		return nil, 0, countQr.Err
	}

	var total int
	if countQr.Next() {
		row, _ := countQr.Slice()
		total = int(rowInt64(row[0]))
	}

	offset := (page - 1) * pageSize
	dataQuery := `
		SELECT id, name, description, domain_id, dag_config, cron_expression,
		       is_enabled, created_by, created_at, updated_at
		FROM bdopsflow_workflows` + whereClause + " ORDER BY created_at DESC LIMIT ? OFFSET ?"

	dataArgs := make([]interface{}, len(args))
	copy(dataArgs, args)
	dataArgs = append(dataArgs, pageSize, offset)

	var qr rqlite.QueryResult
	if len(dataArgs) > 0 {
		stmt := rqlite.ParameterizedStatement{
			Query:     dataQuery,
			Arguments: dataArgs,
		}
		qr, err = s.DB.QueryOneParameterized(stmt)
	} else {
		qr, err = s.DB.QueryOne(dataQuery)
	}

	if err != nil {
		return nil, 0, err
	}
	if qr.Err != nil {
		return nil, 0, qr.Err
	}

	var bdopsflow_workflows []*model.Workflow
	for qr.Next() {
		wf := &model.Workflow{}
		if err := scanWorkflowResult(&qr, wf); err != nil {
			return nil, 0, err
		}
		bdopsflow_workflows = append(bdopsflow_workflows, wf)
	}

	return bdopsflow_workflows, total, nil
}

func (s *SchedulerService) CreateWorkflow(ctx context.Context, query string, args ...interface{}) (*model.Workflow, error) {
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: args,
	}
	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if result.Err != nil {
		return nil, result.Err
	}

	id := result.LastInsertID
	return s.GetWorkflow(ctx, id)
}

func (s *SchedulerService) UpdateWorkflow(ctx context.Context, id int64, wf *model.Workflow) error {
	query := `
		UPDATE bdopsflow_workflows SET name = ?, description = ?, dag_config = ?, cron_expression = ?,
		                    is_enabled = ?, updated_at = ?
		WHERE id = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query: query,
		Arguments: []interface{}{
			wf.Name, wf.Description, wf.DAGConfig,
			wf.CronExpression, wf.IsEnabled, time.Now().Format("2006-01-02 15:04:05"), id,
		},
	}

	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}
	if result.Err != nil {
		return result.Err
	}

	return nil
}

func (s *SchedulerService) DeleteWorkflow(ctx context.Context, id int64) error {
	query := `DELETE FROM bdopsflow_workflows WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{id},
	}
	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}
	if result.Err != nil {
		return result.Err
	}
	return nil
}

func (s *SchedulerService) CreateWorkflowExecution(ctx context.Context, workflowID int64) (*model.WorkflowExecution, error) {
	executionID := fmt.Sprintf("wf-%d-%d", workflowID, time.Now().UnixNano())
	nodeStates := "{}"

	query := `
		INSERT INTO bdopsflow_workflow_executions (workflow_id, execution_id, status, node_states, created_at)
		VALUES (?, ?, 'pending', ?, ?)
	`

	now := time.Now().Format("2006-01-02 15:04:05")
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{workflowID, executionID, nodeStates, now},
	}
	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if result.Err != nil {
		return nil, result.Err
	}

	id := result.LastInsertID
	return s.GetWorkflowExecution(ctx, id)
}

func (s *SchedulerService) GetWorkflowExecution(ctx context.Context, id int64) (*model.WorkflowExecution, error) {
	query := `
		SELECT id, workflow_id, execution_id, status, start_time, end_time, node_states, created_at
		FROM bdopsflow_workflow_executions WHERE id = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{id},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	if !qr.Next() {
		return nil, fmt.Errorf("workflow execution not found")
	}

	we := &model.WorkflowExecution{}
	if err := scanWorkflowExecutionResult(&qr, we); err != nil {
		return nil, err
	}

	return we, nil
}

func (s *SchedulerService) GetWorkflowExecutionByExecutionID(ctx context.Context, executionID string) (*model.WorkflowExecution, error) {
	query := `
		SELECT id, workflow_id, execution_id, status, start_time, end_time, node_states, created_at
		FROM bdopsflow_workflow_executions WHERE execution_id = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{executionID},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	if !qr.Next() {
		return nil, fmt.Errorf("workflow execution not found")
	}

	we := &model.WorkflowExecution{}
	if err := scanWorkflowExecutionResult(&qr, we); err != nil {
		return nil, err
	}

	return we, nil
}

func (s *SchedulerService) ListWorkflowExecutions(ctx context.Context, workflowID int64) ([]*model.WorkflowExecution, error) {
	query := `
		SELECT id, workflow_id, execution_id, status, start_time, end_time, node_states, created_at
		FROM bdopsflow_workflow_executions WHERE workflow_id = ?
		ORDER BY created_at DESC
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{workflowID},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var executions []*model.WorkflowExecution
	for qr.Next() {
		we := &model.WorkflowExecution{}
		if err := scanWorkflowExecutionResult(&qr, we); err != nil {
			return nil, err
		}
		executions = append(executions, we)
	}

	return executions, nil
}

func (s *SchedulerService) UpdateWorkflowExecutionStatus(ctx context.Context, executionID string, status string) error {
	query := `
		UPDATE bdopsflow_workflow_executions
		SET status = ?,
		    start_time = CASE WHEN start_time IS NULL OR start_time = '' THEN ? ELSE start_time END,
		    end_time = CASE WHEN ? IN ('success', 'failed') THEN ? ELSE end_time END
		WHERE execution_id = ?
	`

	now := time.Now().Format("2006-01-02 15:04:05")
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{status, now, status, now, executionID},
	}
	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}
	if result.Err != nil {
		return result.Err
	}

	return nil
}

func (s *SchedulerService) UpdateWorkflowExecutionNodeStates(ctx context.Context, executionID string, nodeStates string) error {
	query := `UPDATE bdopsflow_workflow_executions SET node_states = ? WHERE execution_id = ?`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{nodeStates, executionID},
	}
	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}
	if result.Err != nil {
		return result.Err
	}

	return nil
}

func (s *SchedulerService) TriggerWorkflow(ctx context.Context, workflowID int64) (*model.WorkflowExecution, error) {
	wf, err := s.GetWorkflow(ctx, workflowID)
	if err != nil {
		return nil, err
	}

	we, err := s.CreateWorkflowExecution(ctx, workflowID)
	if err != nil {
		return nil, err
	}

	dagConfig, err := dag.ParseDAGConfig(wf.DAGConfig)
	if err != nil {
		return nil, fmt.Errorf("parse dag config failed: %w", err)
	}

	validator := dag.NewDAGValidator(*dagConfig)
	topoOrder, err := validator.TopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("topological sort failed: %w", err)
	}

	nodeStates := make(map[string]string)
	for _, node := range dagConfig.Nodes {
		nodeStates[node.ID] = "pending"
	}
	nodeStatesJSON, _ := json.Marshal(nodeStates)
	s.UpdateWorkflowExecutionNodeStates(ctx, we.ExecutionID, string(nodeStatesJSON))
	s.UpdateWorkflowExecutionStatus(ctx, we.ExecutionID, "running")

	go s.runWorkflowAsync(ctx, we.ExecutionID, workflowID, dagConfig, topoOrder)

	return we, nil
}

func (s *SchedulerService) runWorkflowAsync(ctx context.Context, executionID string, workflowID int64, dagConfig *dag.DAGConfig, topoOrder []string) {
	s.AddTaskLog(ctx, executionID, 0, "", "info", "Workflow execution started")

	nodeStates := make(map[string]string)
	for _, node := range dagConfig.Nodes {
		nodeStates[node.ID] = "pending"
	}

	for _, nodeID := range topoOrder {
		var node *dag.DAGNode
		for i := range dagConfig.Nodes {
			if dagConfig.Nodes[i].ID == nodeID {
				node = &dagConfig.Nodes[i]
				break
			}
		}
		if node == nil {
			continue
		}

		nodeStates[nodeID] = "running"
		nodeStatesJSON, err := json.Marshal(nodeStates)
		if err != nil {
			slog.Error("failed to marshal node states", "error", err, "node_id", nodeID)
		} else {
			s.UpdateWorkflowExecutionNodeStates(ctx, executionID, string(nodeStatesJSON))
		}
		s.AddTaskLog(ctx, executionID, 0, nodeID, "info", fmt.Sprintf("Node %s started", node.Name))

		time.Sleep(1 * time.Second)

		nodeStates[nodeID] = "success"
		nodeStatesJSON, err = json.Marshal(nodeStates)
		if err != nil {
			slog.Error("failed to marshal node states", "error", err, "node_id", nodeID)
		} else {
			s.UpdateWorkflowExecutionNodeStates(ctx, executionID, string(nodeStatesJSON))
		}
		s.AddTaskLog(ctx, executionID, 0, nodeID, "info", fmt.Sprintf("Node %s completed", node.Name))
	}

	s.UpdateWorkflowExecutionStatus(ctx, executionID, "success")
	s.AddTaskLog(ctx, executionID, 0, "", "info", "Workflow execution completed successfully")
}
