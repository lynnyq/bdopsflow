package dag

import (
	"encoding/json"
	"fmt"
)

type DAGConfig struct {
	Nodes       []DAGNode       `json:"nodes"`
	Connections []DAGConnection `json:"connections"`
}

type DAGNode struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Config     map[string]interface{} `json:"config"`
	Position   Position                `json:"position"`
	TimeoutSec int                    `json:"timeout_seconds"`
	RetryCount int                    `json:"retry_count"`
}

type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type DAGConnection struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type DAGValidator struct {
	nodes       map[string]*DAGNode
	connections []DAGConnection
}

func NewDAGValidator(config DAGConfig) *DAGValidator {
	nodes := make(map[string]*DAGNode)
	for i := range config.Nodes {
		nodes[config.Nodes[i].ID] = &config.Nodes[i]
	}
	return &DAGValidator{
		nodes:       nodes,
		connections: config.Connections,
	}
}

func (v *DAGValidator) Validate() error {
	if err := v.checkNodesExist(); err != nil {
		return err
	}
	if err := v.checkNoDuplicateConnections(); err != nil {
		return err
	}
	if err := v.checkNoSelfLoops(); err != nil {
		return err
	}
	if err := v.checkNoCycles(); err != nil {
		return err
	}
	return nil
}

func (v *DAGValidator) checkNodesExist() error {
	connectedNodes := make(map[string]bool)
	for _, conn := range v.connections {
		connectedNodes[conn.From] = true
		connectedNodes[conn.To] = true
	}

	for nodeID := range connectedNodes {
		if _, exists := v.nodes[nodeID]; !exists {
			return fmt.Errorf("node %s is referenced in connections but not defined in nodes", nodeID)
		}
	}
	return nil
}

func (v *DAGValidator) checkNoDuplicateConnections() error {
	seen := make(map[string]bool)
	for _, conn := range v.connections {
		key := fmt.Sprintf("%s->%s", conn.From, conn.To)
		if seen[key] {
			return fmt.Errorf("duplicate connection from %s to %s", conn.From, conn.To)
		}
		seen[key] = true
	}
	return nil
}

func (v *DAGValidator) checkNoSelfLoops() error {
	for _, conn := range v.connections {
		if conn.From == conn.To {
			return fmt.Errorf("self-loop detected on node %s", conn.From)
		}
	}
	return nil
}

func (v *DAGValidator) checkNoCycles() error {
	inDegree := make(map[string]int)
	for nodeID := range v.nodes {
		inDegree[nodeID] = 0
	}
	
	for _, conn := range v.connections {
		inDegree[conn.To]++
	}

	var queue []string
	for nodeID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, nodeID)
		}
	}

	visitedCount := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		visitedCount++

		for _, conn := range v.connections {
			if conn.From == node {
				inDegree[conn.To]--
				if inDegree[conn.To] == 0 {
					queue = append(queue, conn.To)
				}
			}
		}
	}

	if visitedCount != len(v.nodes) {
		return fmt.Errorf("cycle detected in DAG")
	}
	return nil
}

func (v *DAGValidator) TopologicalSort() ([]string, error) {
	if err := v.Validate(); err != nil {
		return nil, err
	}

	inDegree := make(map[string]int)
	for nodeID := range v.nodes {
		inDegree[nodeID] = 0
	}
	
	for _, conn := range v.connections {
		inDegree[conn.To]++
	}

	var queue []string
	for nodeID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, nodeID)
		}
	}

	var result []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		for _, conn := range v.connections {
			if conn.From == node {
				inDegree[conn.To]--
				if inDegree[conn.To] == 0 {
					queue = append(queue, conn.To)
				}
			}
		}
	}

	return result, nil
}

func (v *DAGValidator) GetDependencies(nodeID string) []string {
	var deps []string
	for _, conn := range v.connections {
		if conn.To == nodeID {
			deps = append(deps, conn.From)
		}
	}
	return deps
}

func (v *DAGValidator) GetDependents(nodeID string) []string {
	var dependents []string
	for _, conn := range v.connections {
		if conn.From == nodeID {
			dependents = append(dependents, conn.To)
		}
	}
	return dependents
}

func (v *DAGValidator) GetRootNodes() []string {
	var roots []string
	hasIncoming := make(map[string]bool)
	for _, conn := range v.connections {
		hasIncoming[conn.To] = true
	}

	for nodeID := range v.nodes {
		if !hasIncoming[nodeID] {
			roots = append(roots, nodeID)
		}
	}
	return roots
}

func (v *DAGValidator) GetLeafNodes() []string {
	var leaves []string
	hasOutgoing := make(map[string]bool)
	for _, conn := range v.connections {
		hasOutgoing[conn.From] = true
	}

	for nodeID := range v.nodes {
		if !hasOutgoing[nodeID] {
			leaves = append(leaves, nodeID)
		}
	}
	return leaves
}

func ParseDAGConfig(jsonStr string) (*DAGConfig, error) {
	if jsonStr == "" {
		return &DAGConfig{
			Nodes:       []DAGNode{},
			Connections: []DAGConnection{},
		}, nil
	}

	var config DAGConfig
	if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
		return nil, fmt.Errorf("failed to parse DAG config: %w", err)
	}
	return &config, nil
}

func ValidateDAGConfig(jsonStr string) error {
	config, err := ParseDAGConfig(jsonStr)
	if err != nil {
		return err
	}

	validator := NewDAGValidator(*config)
	return validator.Validate()
}

func GetExecutionOrder(jsonStr string) ([]string, error) {
	config, err := ParseDAGConfig(jsonStr)
	if err != nil {
		return nil, err
	}

	validator := NewDAGValidator(*config)
	return validator.TopologicalSort()
}
