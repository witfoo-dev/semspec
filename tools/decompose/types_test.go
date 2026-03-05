package decompose_test

import (
	"strings"
	"testing"

	"github.com/c360studio/semspec/tools/decompose"
)

// -- helpers --

// dag builds a TaskDAG from a slice of TaskNodes for concise test setup.
func dag(nodes ...decompose.TaskNode) decompose.TaskDAG {
	return decompose.TaskDAG{Nodes: nodes}
}

// taskNode is a convenience builder for a TaskNode with optional dependencies.
func taskNode(id, prompt, role string, deps ...string) decompose.TaskNode {
	return decompose.TaskNode{
		ID:        id,
		Prompt:    prompt,
		Role:      role,
		DependsOn: deps,
	}
}

// -- tests --

func TestValidate_SingleNode_Valid(t *testing.T) {
	t.Parallel()

	d := dag(taskNode("a", "Do something", "worker"))
	if err := d.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil for single valid node", err)
	}
}

func TestValidate_LinearChain_Valid(t *testing.T) {
	t.Parallel()

	d := dag(
		taskNode("a", "Step A", "worker"),
		taskNode("b", "Step B", "worker", "a"),
		taskNode("c", "Step C", "worker", "b"),
	)
	if err := d.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil for linear chain", err)
	}
}

func TestValidate_ParallelNodesWithSharedDep_Valid(t *testing.T) {
	t.Parallel()

	d := dag(
		taskNode("root", "Root task", "planner"),
		taskNode("a", "Branch A", "worker", "root"),
		taskNode("b", "Branch B", "worker", "root"),
		taskNode("merge", "Merge results", "analyst", "a", "b"),
	)
	if err := d.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil for diamond DAG", err)
	}
}

func TestValidate_EmptyNodes_ReturnsError(t *testing.T) {
	t.Parallel()

	d := decompose.TaskDAG{} // zero value, no nodes
	if err := d.Validate(); err == nil {
		t.Error("Validate() = nil, want error for empty nodes")
	}
}

func TestValidate_DuplicateNodeIDs_ReturnsError(t *testing.T) {
	t.Parallel()

	d := dag(
		taskNode("dup", "First", "worker"),
		taskNode("dup", "Second with same ID", "worker"),
	)

	err := d.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want error for duplicate node IDs")
	}
	if !strings.Contains(err.Error(), "dup") {
		t.Errorf("error = %q, want mention of duplicate ID %q", err.Error(), "dup")
	}
}

func TestValidate_InvalidDependencyRef_ReturnsError(t *testing.T) {
	t.Parallel()

	d := dag(
		taskNode("a", "Valid", "worker"),
		taskNode("b", "Depends on ghost", "worker", "nonexistent"),
	)

	err := d.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want error for unknown dependency")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error = %q, want mention of unknown node %q", err.Error(), "nonexistent")
	}
}

func TestValidate_SelfReference_ReturnsError(t *testing.T) {
	t.Parallel()

	d := dag(taskNode("loop", "Self-referencing", "worker", "loop"))

	err := d.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want error for self-reference")
	}
	if !strings.Contains(err.Error(), "loop") {
		t.Errorf("error = %q, want mention of self-referencing node %q", err.Error(), "loop")
	}
}

func TestValidate_SimpleCycleTwoNodes_ReturnsError(t *testing.T) {
	t.Parallel()

	// A depends on B, B depends on A.
	d := dag(
		taskNode("a", "Task A", "worker", "b"),
		taskNode("b", "Task B", "worker", "a"),
	)

	err := d.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want cycle detection error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "cycle") {
		t.Errorf("error = %q, want mention of cycle", err.Error())
	}
}

func TestValidate_ThreeNodeCycle_ReturnsError(t *testing.T) {
	t.Parallel()

	// A → B → C → A
	d := dag(
		taskNode("a", "Task A", "worker", "c"),
		taskNode("b", "Task B", "worker", "a"),
		taskNode("c", "Task C", "worker", "b"),
	)

	err := d.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want cycle detection error for 3-node cycle")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "cycle") {
		t.Errorf("error = %q, want mention of cycle", err.Error())
	}
}

func TestValidate_MultipleDisconnectedComponents_Valid(t *testing.T) {
	t.Parallel()

	// Two independent chains with no relationship between them.
	d := dag(
		taskNode("chain1-a", "Chain 1 Step A", "worker"),
		taskNode("chain1-b", "Chain 1 Step B", "worker", "chain1-a"),
		taskNode("chain2-a", "Chain 2 Step A", "analyst"),
		taskNode("chain2-b", "Chain 2 Step B", "analyst", "chain2-a"),
	)

	if err := d.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil for disconnected valid DAG", err)
	}
}
