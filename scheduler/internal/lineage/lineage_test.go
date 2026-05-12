package lineage

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	schema := `
	CREATE TABLE tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		status TEXT DEFAULT 'pending',
		workflow_id INTEGER
	);

	CREATE TABLE task_dependencies (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id INTEGER NOT NULL,
		parent_task_id INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(task_id, parent_task_id)
	);
	`

	_, err = db.Exec(schema)
	if err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	return db
}

func TestNewService(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	service := NewService(db)
	if service == nil {
		t.Fatal("expected service to be created")
	}
}

func TestService_GetTaskLineage(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		INSERT INTO tasks (id, name, type) VALUES 
			(1, 'Task A', 'http'),
			(2, 'Task B', 'http'),
			(3, 'Task C', 'http'),
			(4, 'Task D', 'http')
	`)
	if err != nil {
		t.Fatalf("failed to insert tasks: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO task_dependencies (task_id, parent_task_id) VALUES
			(2, 1),
			(3, 2),
			(4, 2)
	`)
	if err != nil {
		t.Fatalf("failed to insert dependencies: %v", err)
	}

	service := NewService(db)
	ctx := context.Background()

	graph, err := service.GetTaskLineage(ctx, 2)
	if err != nil {
		t.Errorf("failed to get lineage: %v", err)
	}

	if len(graph.Tasks) != 5 {
		t.Errorf("expected 5 tasks in lineage, got %d", len(graph.Tasks))
	}

	if len(graph.Relations) != 3 {
		t.Errorf("expected 3 relations, got %d", len(graph.Relations))
	}
}

func TestService_GetWorkflowLineage(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		INSERT INTO tasks (id, name, type, workflow_id) VALUES 
			(1, 'Task A', 'http', 1),
			(2, 'Task B', 'http', 1),
			(3, 'Task C', 'http', 1),
			(4, 'Task D', 'http', 2)
	`)
	if err != nil {
		t.Fatalf("failed to insert tasks: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO task_dependencies (task_id, parent_task_id) VALUES
			(2, 1),
			(3, 2)
	`)
	if err != nil {
		t.Fatalf("failed to insert dependencies: %v", err)
	}

	service := NewService(db)
	ctx := context.Background()

	graph, err := service.GetWorkflowLineage(ctx, 1)
	if err != nil {
		t.Errorf("failed to get workflow lineage: %v", err)
	}

	if len(graph.Tasks) != 3 {
		t.Errorf("expected 3 tasks in workflow lineage, got %d", len(graph.Tasks))
	}
}

func TestService_AddDependency(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		INSERT INTO tasks (id, name, type) VALUES (1, 'Task A', 'http'), (2, 'Task B', 'http')
	`)
	if err != nil {
		t.Fatalf("failed to insert tasks: %v", err)
	}

	service := NewService(db)
	ctx := context.Background()

	err = service.AddDependency(ctx, 2, 1)
	if err != nil {
		t.Errorf("failed to add dependency: %v", err)
	}

	parents, err := service.getParentTasks(ctx, 2)
	if err != nil {
		t.Errorf("failed to get parent tasks: %v", err)
	}

	if len(parents) != 1 || parents[0] != 1 {
		t.Errorf("expected parent task 1, got %v", parents)
	}
}

func TestService_RemoveDependency(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		INSERT INTO tasks (id, name, type) VALUES (1, 'Task A', 'http'), (2, 'Task B', 'http')
	`)
	if err != nil {
		t.Fatalf("failed to insert tasks: %v", err)
	}

	_, err = db.Exec(`INSERT INTO task_dependencies (task_id, parent_task_id) VALUES (2, 1)`)
	if err != nil {
		t.Fatalf("failed to insert dependency: %v", err)
	}

	service := NewService(db)
	ctx := context.Background()

	err = service.RemoveDependency(ctx, 2, 1)
	if err != nil {
		t.Errorf("failed to remove dependency: %v", err)
	}

	parents, err := service.getParentTasks(ctx, 2)
	if err != nil {
		t.Errorf("failed to get parent tasks: %v", err)
	}

	if len(parents) != 0 {
		t.Errorf("expected no parent tasks, got %v", parents)
	}
}

func TestService_GetTaskImpact(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		INSERT INTO tasks (id, name, type) VALUES 
			(1, 'Task A', 'http'),
			(2, 'Task B', 'http'),
			(3, 'Task C', 'http'),
			(4, 'Task D', 'http')
	`)
	if err != nil {
		t.Fatalf("failed to insert tasks: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO task_dependencies (task_id, parent_task_id) VALUES
			(2, 1),
			(3, 2),
			(4, 2)
	`)
	if err != nil {
		t.Fatalf("failed to insert dependencies: %v", err)
	}

	service := NewService(db)
	ctx := context.Background()

	impacted, err := service.GetTaskImpact(ctx, 1)
	if err != nil {
		t.Errorf("failed to get task impact: %v", err)
	}

	if len(impacted) != 3 {
		t.Errorf("expected 3 impacted tasks, got %d", len(impacted))
	}
}

func TestLineageGraph_ToJSON(t *testing.T) {
	graph := &LineageGraph{
		Tasks: []*TaskNode{
			{ID: 1, Name: "Task A", Type: "http", Status: "success"},
			{ID: 2, Name: "Task B", Type: "shell", Status: "pending"},
		},
		Relations: []Relation{
			{From: 1, To: 2},
		},
	}

	jsonStr, err := graph.ToJSON()
	if err != nil {
		t.Errorf("failed to convert to JSON: %v", err)
	}

	if jsonStr == "" {
		t.Error("expected non-empty JSON string")
	}

	if jsonStr[0] != '{' {
		t.Error("expected JSON object")
	}
}

func TestService_getParentTasks(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		INSERT INTO tasks (id, name, type) VALUES (1, 'Task A', 'http'), (2, 'Task B', 'http')
	`)
	if err != nil {
		t.Fatalf("failed to insert tasks: %v", err)
	}

	_, err = db.Exec(`INSERT INTO task_dependencies (task_id, parent_task_id) VALUES (2, 1)`)
	if err != nil {
		t.Fatalf("failed to insert dependency: %v", err)
	}

	service := NewService(db)
	ctx := context.Background()

	parents, err := service.getParentTasks(ctx, 2)
	if err != nil {
		t.Errorf("failed to get parent tasks: %v", err)
	}

	if len(parents) != 1 {
		t.Errorf("expected 1 parent, got %d", len(parents))
	}
}

func TestService_getChildTasks(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		INSERT INTO tasks (id, name, type) VALUES (1, 'Task A', 'http'), (2, 'Task B', 'http')
	`)
	if err != nil {
		t.Fatalf("failed to insert tasks: %v", err)
	}

	_, err = db.Exec(`INSERT INTO task_dependencies (task_id, parent_task_id) VALUES (2, 1)`)
	if err != nil {
		t.Fatalf("failed to insert dependency: %v", err)
	}

	service := NewService(db)
	ctx := context.Background()

	children, err := service.getChildTasks(ctx, 1)
	if err != nil {
		t.Errorf("failed to get child tasks: %v", err)
	}

	if len(children) != 1 {
		t.Errorf("expected 1 child, got %d", len(children))
	}
}
