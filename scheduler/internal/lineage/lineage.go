package lineage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type Service struct {
	db *sql.DB
}

type TaskNode struct {
	ID       int64       `json:"id"`
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Status   string      `json:"status"`
	Children []*TaskNode `json:"children,omitempty"`
	Parents  []*TaskNode `json:"parents,omitempty"`
}

type LineageGraph struct {
	Tasks     []*TaskNode `json:"bdopsflow_tasks"`
	Relations []Relation  `json:"relations"`
}

type Relation struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) GetTaskLineage(ctx context.Context, taskID int64) (*LineageGraph, error) {
	graph := &LineageGraph{
		Tasks:     make([]*TaskNode, 0),
		Relations: make([]Relation, 0),
	}

	visited := make(map[int64]bool)
	if err := s.buildLineage(ctx, taskID, graph, visited, true); err != nil {
		return nil, err
	}

	visited = make(map[int64]bool)
	if err := s.buildLineage(ctx, taskID, graph, visited, false); err != nil {
		return nil, err
	}

	return graph, nil
}

func (s *Service) buildLineage(ctx context.Context, taskID int64, graph *LineageGraph, visited map[int64]bool, upstream bool) error {
	if visited[taskID] {
		return nil
	}
	visited[taskID] = true

	task, err := s.getTask(ctx, taskID)
	if err != nil {
		return err
	}

	graph.Tasks = append(graph.Tasks, task)

	var relatedTasks []int64
	if upstream {
		relatedTasks, err = s.getParentTasks(ctx, taskID)
	} else {
		relatedTasks, err = s.getChildTasks(ctx, taskID)
	}

	if err != nil {
		return err
	}

	for _, relatedID := range relatedTasks {
		if upstream {
			graph.Relations = append(graph.Relations, Relation{From: relatedID, To: taskID})
		} else {
			graph.Relations = append(graph.Relations, Relation{From: taskID, To: relatedID})
		}

		if err := s.buildLineage(ctx, relatedID, graph, visited, upstream); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) getTask(ctx context.Context, taskID int64) (*TaskNode, error) {
	query := `
		SELECT id, name, type, status
		FROM bdopsflow_tasks
		WHERE id = ?
	`

	node := &TaskNode{}
	err := s.db.QueryRowContext(ctx, query, taskID).Scan(&node.ID, &node.Name, &node.Type, &node.Status)
	if err != nil {
		return nil, fmt.Errorf("failed to get task %d: %w", taskID, err)
	}

	return node, nil
}

func (s *Service) getParentTasks(ctx context.Context, taskID int64) ([]int64, error) {
	query := `
		SELECT parent_task_id
		FROM bdopsflow_task_dependencies
		WHERE task_id = ?
	`

	rows, err := s.db.QueryContext(ctx, query, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var parents []int64
	for rows.Next() {
		var parentID int64
		if err := rows.Scan(&parentID); err != nil {
			return nil, err
		}
		parents = append(parents, parentID)
	}

	return parents, nil
}

func (s *Service) getChildTasks(ctx context.Context, taskID int64) ([]int64, error) {
	query := `
		SELECT task_id
		FROM bdopsflow_task_dependencies
		WHERE parent_task_id = ?
	`

	rows, err := s.db.QueryContext(ctx, query, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var children []int64
	for rows.Next() {
		var childID int64
		if err := rows.Scan(&childID); err != nil {
			return nil, err
		}
		children = append(children, childID)
	}

	return children, nil
}

func (s *Service) GetWorkflowLineage(ctx context.Context, workflowID int64) (*LineageGraph, error) {
	query := `
		SELECT id, name, type, status
		FROM bdopsflow_tasks
		WHERE workflow_id = ?
	`

	rows, err := s.db.QueryContext(ctx, query, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	graph := &LineageGraph{
		Tasks:     make([]*TaskNode, 0),
		Relations: make([]Relation, 0),
	}

	taskMap := make(map[int64]*TaskNode)
	for rows.Next() {
		node := &TaskNode{}
		if err := rows.Scan(&node.ID, &node.Name, &node.Type, &node.Status); err != nil {
			return nil, err
		}
		graph.Tasks = append(graph.Tasks, node)
		taskMap[node.ID] = node
	}

	for _, task := range graph.Tasks {
		parents, err := s.getParentTasks(ctx, task.ID)
		if err != nil {
			return nil, err
		}
		for _, parentID := range parents {
			if _, exists := taskMap[parentID]; exists {
				graph.Relations = append(graph.Relations, Relation{From: parentID, To: task.ID})
			}
		}
	}

	return graph, nil
}

func (s *Service) AddDependency(ctx context.Context, taskID, parentTaskID int64) error {
	now := time.Now().Format(time.RFC3339Nano)
	query := `
		INSERT INTO bdopsflow_task_dependencies (task_id, parent_task_id, created_at)
		VALUES (?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query, taskID, parentTaskID, now)
	if err != nil {
		return fmt.Errorf("failed to add dependency: %w", err)
	}

	return nil
}

func (s *Service) RemoveDependency(ctx context.Context, taskID, parentTaskID int64) error {
	query := `
		DELETE FROM bdopsflow_task_dependencies
		WHERE task_id = ? AND parent_task_id = ?
	`

	_, err := s.db.ExecContext(ctx, query, taskID, parentTaskID)
	if err != nil {
		return fmt.Errorf("failed to remove dependency: %w", err)
	}

	return nil
}

func (s *Service) GetTaskImpact(ctx context.Context, taskID int64) ([]*TaskNode, error) {
	visited := make(map[int64]bool)
	var impacted []*TaskNode

	var collectImpact func(int64) error
	collectImpact = func(id int64) error {
		if visited[id] {
			return nil
		}
		visited[id] = true

		children, err := s.getChildTasks(ctx, id)
		if err != nil {
			return err
		}

		for _, childID := range children {
			if !visited[childID] {
				task, err := s.getTask(ctx, childID)
				if err != nil {
					return err
				}
				impacted = append(impacted, task)

				if err := collectImpact(childID); err != nil {
					return err
				}
			}
		}

		return nil
	}

	if err := collectImpact(taskID); err != nil {
		return nil, err
	}

	return impacted, nil
}

func (g *LineageGraph) ToJSON() (string, error) {
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
