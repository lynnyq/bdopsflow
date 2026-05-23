package dag

import (
	"encoding/json"
	"testing"
)

func TestDAGConfig_Parse(t *testing.T) {
	jsonStr := `{
		"nodes": [
			{
				"id": "node1",
				"name": "Task 1",
				"type": "http",
				"config": {},
				"position": {"x": 100, "y": 100}
			},
			{
				"id": "node2",
				"name": "Task 2",
				"type": "shell",
				"config": {},
				"position": {"x": 200, "y": 200}
			}
		],
		"connections": [
			{"from": "node1", "to": "node2"}
		]
	}`

	config, err := ParseDAGConfig(jsonStr)
	if err != nil {
		t.Fatalf("Failed to parse DAG config: %v", err)
	}

	if len(config.Nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(config.Nodes))
	}

	if len(config.Connections) != 1 {
		t.Errorf("Expected 1 connection, got %d", len(config.Connections))
	}

	if config.Connections[0].From != "node1" {
		t.Errorf("Expected connection from node1, got %s", config.Connections[0].From)
	}
}

func TestDAGValidator_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  DAGConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid DAG",
			config: DAGConfig{
				Nodes: []DAGNode{
					{ID: "node1", Name: "Task 1", Type: "http"},
					{ID: "node2", Name: "Task 2", Type: "shell"},
				},
				Connections: []DAGConnection{
					{From: "node1", To: "node2"},
				},
			},
			wantErr: false,
		},
		{
			name: "node not defined",
			config: DAGConfig{
				Nodes: []DAGNode{
					{ID: "node1", Name: "Task 1", Type: "http"},
				},
				Connections: []DAGConnection{
					{From: "node1", To: "node2"},
				},
			},
			wantErr: true,
			errMsg:  "node node2 is referenced in connections but not defined",
		},
		{
			name: "self loop",
			config: DAGConfig{
				Nodes: []DAGNode{
					{ID: "node1", Name: "Task 1", Type: "http"},
				},
				Connections: []DAGConnection{
					{From: "node1", To: "node1"},
				},
			},
			wantErr: true,
			errMsg:  "self-loop detected",
		},
		{
			name: "cycle detected",
			config: DAGConfig{
				Nodes: []DAGNode{
					{ID: "node1", Name: "Task 1", Type: "http"},
					{ID: "node2", Name: "Task 2", Type: "shell"},
					{ID: "node3", Name: "Task 3", Type: "delay"},
				},
				Connections: []DAGConnection{
					{From: "node1", To: "node2"},
					{From: "node2", To: "node3"},
					{From: "node3", To: "node1"},
				},
			},
			wantErr: true,
			errMsg:  "cycle detected",
		},
		{
			name: "duplicate connection",
			config: DAGConfig{
				Nodes: []DAGNode{
					{ID: "node1", Name: "Task 1", Type: "http"},
					{ID: "node2", Name: "Task 2", Type: "shell"},
				},
				Connections: []DAGConnection{
					{From: "node1", To: "node2"},
					{From: "node1", To: "node2"},
				},
			},
			wantErr: true,
			errMsg:  "duplicate connection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewDAGValidator(tt.config)
			err := validator.Validate()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got nil")
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestDAGValidator_TopologicalSort(t *testing.T) {
	config := DAGConfig{
		Nodes: []DAGNode{
			{ID: "node1", Name: "Task 1", Type: "http"},
			{ID: "node2", Name: "Task 2", Type: "shell"},
			{ID: "node3", Name: "Task 3", Type: "delay"},
		},
		Connections: []DAGConnection{
			{From: "node1", To: "node2"},
			{From: "node2", To: "node3"},
		},
	}

	validator := NewDAGValidator(config)
	order, err := validator.TopologicalSort()
	if err != nil {
		t.Fatalf("Failed to sort: %v", err)
	}

	if len(order) != 3 {
		t.Errorf("Expected 3 nodes in order, got %d", len(order))
	}

	node1Index := -1
	node2Index := -1
	node3Index := -1
	for i, nodeID := range order {
		if nodeID == "node1" {
			node1Index = i
		}
		if nodeID == "node2" {
			node2Index = i
		}
		if nodeID == "node3" {
			node3Index = i
		}
	}

	if node1Index >= node2Index {
		t.Errorf("node1 should come before node2")
	}
	if node2Index >= node3Index {
		t.Errorf("node2 should come before node3")
	}
}

func TestDAGValidator_GetDependencies(t *testing.T) {
	config := DAGConfig{
		Nodes: []DAGNode{
			{ID: "node1", Name: "Task 1", Type: "http"},
			{ID: "node2", Name: "Task 2", Type: "shell"},
			{ID: "node3", Name: "Task 3", Type: "delay"},
		},
		Connections: []DAGConnection{
			{From: "node1", To: "node3"},
			{From: "node2", To: "node3"},
		},
	}

	validator := NewDAGValidator(config)
	deps := validator.GetDependencies("node3")

	if len(deps) != 2 {
		t.Errorf("Expected 2 dependencies for node3, got %d", len(deps))
	}

	hasNode1 := false
	hasNode2 := false
	for _, dep := range deps {
		if dep == "node1" {
			hasNode1 = true
		}
		if dep == "node2" {
			hasNode2 = true
		}
	}

	if !hasNode1 || !hasNode2 {
		t.Errorf("Expected both node1 and node2 as dependencies")
	}
}

func TestDAGValidator_GetRootNodes(t *testing.T) {
	config := DAGConfig{
		Nodes: []DAGNode{
			{ID: "node1", Name: "Task 1", Type: "http"},
			{ID: "node2", Name: "Task 2", Type: "shell"},
			{ID: "node3", Name: "Task 3", Type: "delay"},
		},
		Connections: []DAGConnection{
			{From: "node1", To: "node3"},
			{From: "node2", To: "node3"},
		},
	}

	validator := NewDAGValidator(config)
	roots := validator.GetRootNodes()

	if len(roots) != 2 {
		t.Errorf("Expected 2 root nodes, got %d", len(roots))
	}
}

func TestDAGValidator_GetLeafNodes(t *testing.T) {
	config := DAGConfig{
		Nodes: []DAGNode{
			{ID: "node1", Name: "Task 1", Type: "http"},
			{ID: "node2", Name: "Task 2", Type: "shell"},
			{ID: "node3", Name: "Task 3", Type: "delay"},
		},
		Connections: []DAGConnection{
			{From: "node1", To: "node3"},
			{From: "node2", To: "node3"},
		},
	}

	validator := NewDAGValidator(config)
	leaves := validator.GetLeafNodes()

	if len(leaves) != 1 {
		t.Errorf("Expected 1 leaf node, got %d", len(leaves))
	}

	if leaves[0] != "node3" {
		t.Errorf("Expected node3 as leaf, got %s", leaves[0])
	}
}

func TestParseDAGConfig_Empty(t *testing.T) {
	config, err := ParseDAGConfig("")
	if err != nil {
		t.Fatalf("Failed to parse empty DAG config: %v", err)
	}

	if len(config.Nodes) != 0 {
		t.Errorf("Expected 0 nodes, got %d", len(config.Nodes))
	}
}

func TestValidateDAGConfig(t *testing.T) {
	validConfig := `{
		"nodes": [
			{"id": "node1", "name": "Task 1", "type": "http", "config": {}, "position": {"x": 0, "y": 0}}
		],
		"connections": []
	}`

	err := ValidateDAGConfig(validConfig)
	if err != nil {
		t.Errorf("Expected no error for valid config, got: %v", err)
	}

	invalidConfig := `{
		"nodes": [
			{"id": "node1", "name": "Task 1", "type": "http", "config": {}, "position": {"x": 0, "y": 0}}
		],
		"connections": [
			{"from": "node1", "to": "node1"}
		]
	}`

	err = ValidateDAGConfig(invalidConfig)
	if err == nil {
		t.Errorf("Expected error for self-loop config")
	}
}

func TestGetExecutionOrder(t *testing.T) {
	config := `{
		"nodes": [
			{"id": "node1", "name": "Task 1", "type": "http", "config": {}, "position": {"x": 0, "y": 0}},
			{"id": "node2", "name": "Task 2", "type": "shell", "config": {}, "position": {"x": 100, "y": 100}},
			{"id": "node3", "name": "Task 3", "type": "delay", "config": {}, "position": {"x": 200, "y": 200}}
		],
		"connections": [
			{"from": "node1", "to": "node2"},
			{"from": "node2", "to": "node3"}
		]
	}`

	order, err := GetExecutionOrder(config)
	if err != nil {
		t.Fatalf("Failed to get execution order: %v", err)
	}

	if len(order) != 3 {
		t.Errorf("Expected 3 nodes in order, got %d", len(order))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestDAGConfig_JSON(t *testing.T) {
	config := DAGConfig{
		Nodes: []DAGNode{
			{
				ID:         "node1",
				Name:       "Test Node",
				Type:       "http",
				Config:     map[string]interface{}{"url": "http://example.com"},
				Position:   Position{X: 100.0, Y: 200.0},
				TimeoutSec: 30,
				RetryCount: 3,
			},
		},
		Connections: []DAGConnection{
			{From: "node1", To: "node2"},
		},
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal DAG config: %v", err)
	}

	var parsed DAGConfig
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal DAG config: %v", err)
	}

	if len(parsed.Nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(parsed.Nodes))
	}

	if parsed.Nodes[0].Name != "Test Node" {
		t.Errorf("Expected node name 'Test Node', got %s", parsed.Nodes[0].Name)
	}
}
