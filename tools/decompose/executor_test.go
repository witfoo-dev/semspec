package decompose_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/c360studio/semstreams/agentic"

	"github.com/c360studio/semspec/tools/decompose"
)

// -- helpers --

// makeCall builds a ToolCall for decompose_task with the given arguments.
func makeCall(id, loopID, traceID string, args map[string]any) agentic.ToolCall {
	return agentic.ToolCall{
		ID:        id,
		Name:      "decompose_task",
		Arguments: args,
		LoopID:    loopID,
		TraceID:   traceID,
	}
}

// mustUnmarshalDAGResponse unmarshals the Content field into the response
// envelope and returns the embedded dag.
func mustUnmarshalDAGResponse(t *testing.T, content string) decompose.TaskDAG {
	t.Helper()
	var envelope struct {
		Goal string          `json:"goal"`
		DAG  decompose.TaskDAG `json:"dag"`
	}
	if err := json.Unmarshal([]byte(content), &envelope); err != nil {
		t.Fatalf("unmarshal response envelope from %q: %v", content, err)
	}
	return envelope.DAG
}

// nodes builds the []any structure that Execute expects — matching what the
// JSON unmarshaller produces when decoding tool call arguments.
func nodes(ns ...map[string]any) []any {
	out := make([]any, len(ns))
	for i, n := range ns {
		out[i] = n
	}
	return out
}

// node is a convenience builder for a single node map.
func node(id, prompt, role string, deps ...string) map[string]any {
	m := map[string]any{
		"id":     id,
		"prompt": prompt,
		"role":   role,
	}
	if len(deps) > 0 {
		raw := make([]any, len(deps))
		for i, d := range deps {
			raw[i] = d
		}
		m["depends_on"] = raw
	}
	return m
}

// -- tests --

func TestExecutor_ValidDAGWithDependencies_ReturnsValidatedJSON(t *testing.T) {
	t.Parallel()

	exec := decompose.NewExecutor()
	call := makeCall("call-1", "loop-1", "trace-1", map[string]any{
		"goal": "Build a market analysis report",
		"nodes": nodes(
			node("node-1", "Research current market data", "researcher"),
			node("node-2", "Analyze findings from research", "analyst", "node-1"),
		),
	})

	result, err := exec.Execute(context.Background(), call)

	if err != nil {
		t.Fatalf("Execute() unexpected Go error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Execute() result.Error = %q, want empty", result.Error)
	}
	if result.CallID != "call-1" {
		t.Errorf("CallID = %q, want %q", result.CallID, "call-1")
	}

	dag := mustUnmarshalDAGResponse(t, result.Content)
	if len(dag.Nodes) != 2 {
		t.Fatalf("dag.Nodes len = %d, want 2", len(dag.Nodes))
	}
	if dag.Nodes[0].ID != "node-1" {
		t.Errorf("Nodes[0].ID = %q, want %q", dag.Nodes[0].ID, "node-1")
	}
	if dag.Nodes[1].ID != "node-2" {
		t.Errorf("Nodes[1].ID = %q, want %q", dag.Nodes[1].ID, "node-2")
	}
	if len(dag.Nodes[1].DependsOn) != 1 || dag.Nodes[1].DependsOn[0] != "node-1" {
		t.Errorf("Nodes[1].DependsOn = %v, want [node-1]", dag.Nodes[1].DependsOn)
	}
}

func TestExecutor_LinearChain_Valid(t *testing.T) {
	t.Parallel()

	exec := decompose.NewExecutor()
	call := makeCall("call-2", "loop-1", "", map[string]any{
		"goal": "Process data pipeline",
		"nodes": nodes(
			node("a", "Step A", "worker"),
			node("b", "Step B", "worker", "a"),
			node("c", "Step C", "worker", "b"),
		),
	})

	result, err := exec.Execute(context.Background(), call)

	if err != nil {
		t.Fatalf("Execute() unexpected Go error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Execute() result.Error = %q, want empty for linear chain", result.Error)
	}

	dag := mustUnmarshalDAGResponse(t, result.Content)
	if len(dag.Nodes) != 3 {
		t.Fatalf("dag.Nodes len = %d, want 3", len(dag.Nodes))
	}
}

func TestExecutor_ParallelTasksWithSharedDependency_Valid(t *testing.T) {
	t.Parallel()

	// A and B are independent; C depends on both.
	exec := decompose.NewExecutor()
	call := makeCall("call-3", "loop-1", "", map[string]any{
		"goal": "Parallel research then synthesis",
		"nodes": nodes(
			node("a", "Research topic A", "researcher"),
			node("b", "Research topic B", "researcher"),
			node("c", "Synthesize A and B", "analyst", "a", "b"),
		),
	})

	result, err := exec.Execute(context.Background(), call)

	if err != nil {
		t.Fatalf("Execute() unexpected Go error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Execute() result.Error = %q, want empty for parallel tasks", result.Error)
	}

	dag := mustUnmarshalDAGResponse(t, result.Content)
	if len(dag.Nodes) != 3 {
		t.Fatalf("dag.Nodes len = %d, want 3", len(dag.Nodes))
	}
	last := dag.Nodes[2]
	if len(last.DependsOn) != 2 {
		t.Errorf("Nodes[2].DependsOn len = %d, want 2", len(last.DependsOn))
	}
}

func TestExecutor_MissingGoal_ReturnsError(t *testing.T) {
	t.Parallel()

	exec := decompose.NewExecutor()
	call := makeCall("call-4", "loop-1", "", map[string]any{
		"nodes": nodes(node("a", "Do something", "worker")),
		// no "goal"
	})

	result, err := exec.Execute(context.Background(), call)

	if err != nil {
		t.Fatalf("Execute() unexpected Go error: %v", err)
	}
	if result.Error == "" {
		t.Error("Execute() result.Error is empty, want error about missing goal")
	}
	if result.Content != "" {
		t.Errorf("Execute() result.Content = %q, want empty on error", result.Content)
	}
}

func TestExecutor_MissingNodes_ReturnsError(t *testing.T) {
	t.Parallel()

	exec := decompose.NewExecutor()
	call := makeCall("call-5", "loop-1", "", map[string]any{
		"goal": "Do something without nodes",
		// no "nodes"
	})

	result, err := exec.Execute(context.Background(), call)

	if err != nil {
		t.Fatalf("Execute() unexpected Go error: %v", err)
	}
	if result.Error == "" {
		t.Error("Execute() result.Error is empty, want error about missing nodes")
	}
}

func TestExecutor_EmptyNodesArray_ReturnsError(t *testing.T) {
	t.Parallel()

	exec := decompose.NewExecutor()
	call := makeCall("call-6", "loop-1", "", map[string]any{
		"goal":  "Empty decomposition",
		"nodes": []any{},
	})

	result, err := exec.Execute(context.Background(), call)

	if err != nil {
		t.Fatalf("Execute() unexpected Go error: %v", err)
	}
	if result.Error == "" {
		t.Error("Execute() result.Error is empty, want error about empty nodes array")
	}
}

func TestExecutor_DuplicateNodeIDs_ReturnsError(t *testing.T) {
	t.Parallel()

	exec := decompose.NewExecutor()
	call := makeCall("call-7", "loop-1", "", map[string]any{
		"goal": "Duplicate IDs",
		"nodes": nodes(
			node("dup", "First task", "worker"),
			node("dup", "Second task with same ID", "worker"),
		),
	})

	result, err := exec.Execute(context.Background(), call)

	if err != nil {
		t.Fatalf("Execute() unexpected Go error: %v", err)
	}
	if result.Error == "" {
		t.Error("Execute() result.Error is empty, want error about duplicate node IDs")
	}
	if !strings.Contains(result.Error, "dup") {
		t.Errorf("result.Error = %q, want mention of duplicate ID %q", result.Error, "dup")
	}
}

func TestExecutor_InvalidDependencyReference_ReturnsError(t *testing.T) {
	t.Parallel()

	exec := decompose.NewExecutor()
	call := makeCall("call-8", "loop-1", "", map[string]any{
		"goal": "Node depends on non-existent node",
		"nodes": nodes(
			node("a", "Valid node", "worker"),
			node("b", "Depends on ghost", "worker", "ghost-node"),
		),
	})

	result, err := exec.Execute(context.Background(), call)

	if err != nil {
		t.Fatalf("Execute() unexpected Go error: %v", err)
	}
	if result.Error == "" {
		t.Error("Execute() result.Error is empty, want error about unknown dependency")
	}
	if !strings.Contains(result.Error, "ghost-node") {
		t.Errorf("result.Error = %q, want mention of unknown node %q", result.Error, "ghost-node")
	}
}

func TestExecutor_SelfReference_ReturnsError(t *testing.T) {
	t.Parallel()

	exec := decompose.NewExecutor()
	call := makeCall("call-9", "loop-1", "", map[string]any{
		"goal": "Node depends on itself",
		"nodes": nodes(
			node("self-loop", "Task that needs itself", "worker", "self-loop"),
		),
	})

	result, err := exec.Execute(context.Background(), call)

	if err != nil {
		t.Fatalf("Execute() unexpected Go error: %v", err)
	}
	if result.Error == "" {
		t.Error("Execute() result.Error is empty, want error about self-reference")
	}
	if !strings.Contains(result.Error, "self-loop") {
		t.Errorf("result.Error = %q, want mention of self-referencing node %q", result.Error, "self-loop")
	}
}

func TestExecutor_CycleTwoNodes_ReturnsError(t *testing.T) {
	t.Parallel()

	// A depends on B, B depends on A.
	exec := decompose.NewExecutor()
	call := makeCall("call-10", "loop-1", "", map[string]any{
		"goal": "Cycle between two nodes",
		"nodes": nodes(
			node("a", "Task A", "worker", "b"),
			node("b", "Task B", "worker", "a"),
		),
	})

	result, err := exec.Execute(context.Background(), call)

	if err != nil {
		t.Fatalf("Execute() unexpected Go error: %v", err)
	}
	if result.Error == "" {
		t.Error("Execute() result.Error is empty, want cycle detection error")
	}
	if !strings.Contains(strings.ToLower(result.Error), "cycle") {
		t.Errorf("result.Error = %q, want mention of cycle", result.Error)
	}
}

func TestExecutor_ResultCarriesLoopAndTraceIDs(t *testing.T) {
	t.Parallel()

	exec := decompose.NewExecutor()
	call := makeCall("call-11", "loop-xyz", "trace-abc", map[string]any{
		"goal":  "Propagation check",
		"nodes": nodes(node("n1", "Single node", "worker")),
	})

	result, err := exec.Execute(context.Background(), call)

	if err != nil {
		t.Fatalf("Execute() unexpected Go error: %v", err)
	}
	if result.LoopID != "loop-xyz" {
		t.Errorf("LoopID = %q, want %q", result.LoopID, "loop-xyz")
	}
	if result.TraceID != "trace-abc" {
		t.Errorf("TraceID = %q, want %q", result.TraceID, "trace-abc")
	}
}

func TestExecutor_ListTools_ReturnsOneDefinition(t *testing.T) {
	t.Parallel()

	exec := decompose.NewExecutor()
	tools := exec.ListTools()

	if len(tools) != 1 {
		t.Fatalf("ListTools() returned %d definitions, want 1", len(tools))
	}

	def := tools[0]
	if def.Name != "decompose_task" {
		t.Errorf("tool Name = %q, want %q", def.Name, "decompose_task")
	}
	if def.Description == "" {
		t.Error("tool Description is empty")
	}
	if def.Parameters == nil {
		t.Fatal("tool Parameters is nil")
	}

	required, ok := def.Parameters["required"].([]string)
	if !ok {
		t.Fatalf("Parameters[required] type = %T, want []string", def.Parameters["required"])
	}
	if len(required) != 2 {
		t.Fatalf("required len = %d, want 2", len(required))
	}
	wantRequired := map[string]bool{"goal": true, "nodes": true}
	for _, r := range required {
		if !wantRequired[r] {
			t.Errorf("unexpected required field %q", r)
		}
	}
}

func TestExecutor_GoalReturnedInResponse(t *testing.T) {
	t.Parallel()

	exec := decompose.NewExecutor()
	call := makeCall("call-12", "loop-1", "", map[string]any{
		"goal":  "Build a knowledge graph",
		"nodes": nodes(node("n1", "Gather sources", "researcher")),
	})

	result, err := exec.Execute(context.Background(), call)

	if err != nil {
		t.Fatalf("Execute() unexpected Go error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Execute() result.Error = %q, want empty", result.Error)
	}

	var envelope struct {
		Goal string `json:"goal"`
	}
	if err := json.Unmarshal([]byte(result.Content), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if envelope.Goal != "Build a knowledge graph" {
		t.Errorf("envelope.Goal = %q, want %q", envelope.Goal, "Build a knowledge graph")
	}
}
