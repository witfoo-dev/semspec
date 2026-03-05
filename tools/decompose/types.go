// Package decompose implements the decompose_task tool executor.
// It is a passthrough tool — the LLM provides the DAG structure in its tool
// call arguments, the executor validates it and echoes it back. The parent
// agent then decides whether to spawn nodes individually or delegate to the
// DAG execution workflow.
package decompose

import (
	"fmt"
)

// TaskDAG represents a directed acyclic graph of subtasks.
type TaskDAG struct {
	Nodes []TaskNode `json:"nodes"`
}

// TaskNode represents a single subtask in a DAG.
type TaskNode struct {
	ID        string   `json:"id"`
	Prompt    string   `json:"prompt"`
	Role      string   `json:"role"`
	DependsOn []string `json:"depends_on"`
}

// Validate checks the DAG for structural correctness.
// It verifies that:
//   - At least one node is present
//   - All node IDs are unique
//   - All DependsOn references point to existing node IDs
//   - No node references itself as a dependency
//   - The graph contains no cycles (via depth-first search)
// maxDAGNodes caps the number of nodes accepted in a single DAG to prevent
// resource exhaustion from malformed or adversarial LLM responses.
const maxDAGNodes = 100

func (d *TaskDAG) Validate() error {
	if len(d.Nodes) == 0 {
		return fmt.Errorf("dag must contain at least one node")
	}
	if len(d.Nodes) > maxDAGNodes {
		return fmt.Errorf("dag exceeds maximum node count (%d > %d)", len(d.Nodes), maxDAGNodes)
	}

	// Build an index of node IDs for O(1) membership checks.
	nodeIndex := make(map[string]struct{}, len(d.Nodes))
	for _, n := range d.Nodes {
		if _, exists := nodeIndex[n.ID]; exists {
			return fmt.Errorf("duplicate node ID %q", n.ID)
		}
		nodeIndex[n.ID] = struct{}{}
	}

	// Validate dependency references and self-references.
	for _, n := range d.Nodes {
		for _, dep := range n.DependsOn {
			if dep == n.ID {
				return fmt.Errorf("node %q depends on itself", n.ID)
			}
			if _, exists := nodeIndex[dep]; !exists {
				return fmt.Errorf("node %q depends on unknown node %q", n.ID, dep)
			}
		}
	}

	// Build an adjacency list for cycle detection.
	adj := make(map[string][]string, len(d.Nodes))
	for _, n := range d.Nodes {
		adj[n.ID] = n.DependsOn
	}

	// Detect cycles via recursive DFS with three-color marking:
	//   white (0) = unvisited, gray (1) = in current path, black (2) = done.
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(d.Nodes))

	var visit func(id string) error
	visit = func(id string) error {
		color[id] = gray
		for _, dep := range adj[id] {
			switch color[dep] {
			case gray:
				return fmt.Errorf("cycle detected: node %q and node %q are in a cycle", id, dep)
			case white:
				if err := visit(dep); err != nil {
					return err
				}
			}
			// black: already fully explored, no cycle through this path
		}
		color[id] = black
		return nil
	}

	for _, n := range d.Nodes {
		if color[n.ID] == white {
			if err := visit(n.ID); err != nil {
				return err
			}
		}
	}

	return nil
}
