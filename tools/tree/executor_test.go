// Package tree_test contains black-box tests for the query_agent_tree tool executor.
package tree_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/c360studio/semstreams/agentic"

	"github.com/c360studio/semspec/tools/tree"
)

// ----------------------------------------------------------------------------
// Mock GraphQuerier
// ----------------------------------------------------------------------------

// mockGraph is a thread-safe test double for tree.GraphQuerier.
// All configurable fields are set before the test begins; no mutation occurs
// after the Executor is created, so reads during Execute are safe behind mu.
type mockGraph struct {
	mu sync.Mutex

	// GetChildren
	children    []string
	childrenErr error

	// GetTree
	treeIDs []string
	treeErr error

	// GetStatus
	status    string
	statusErr error

	// Captured arguments (written on each call).
	lastGetChildrenLoopID string
	lastGetTreeLoopID     string
	lastGetTreeMaxDepth   int
	lastGetStatusLoopID   string
}

func (m *mockGraph) GetChildren(_ context.Context, loopID string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastGetChildrenLoopID = loopID
	if m.childrenErr != nil {
		return nil, m.childrenErr
	}
	if m.children == nil {
		return []string{}, nil
	}
	return m.children, nil
}

func (m *mockGraph) GetTree(_ context.Context, rootLoopID string, maxDepth int) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastGetTreeLoopID = rootLoopID
	m.lastGetTreeMaxDepth = maxDepth
	if m.treeErr != nil {
		return nil, m.treeErr
	}
	if m.treeIDs == nil {
		return []string{}, nil
	}
	return m.treeIDs, nil
}

func (m *mockGraph) GetStatus(_ context.Context, loopID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastGetStatusLoopID = loopID
	if m.statusErr != nil {
		return "", m.statusErr
	}
	return m.status, nil
}

// capturedGetTree returns the captured arguments under the lock.
func (m *mockGraph) capturedGetTree() (loopID string, maxDepth int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastGetTreeLoopID, m.lastGetTreeMaxDepth
}

// capturedGetChildren returns the captured loop ID under the lock.
func (m *mockGraph) capturedGetChildren() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastGetChildrenLoopID
}

// capturedGetStatus returns the captured loop ID under the lock.
func (m *mockGraph) capturedGetStatus() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastGetStatusLoopID
}

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

func makeCall(id, loopID, traceID string, args map[string]any) agentic.ToolCall {
	return agentic.ToolCall{
		ID:        id,
		Name:      "query_agent_tree",
		Arguments: args,
		LoopID:    loopID,
		TraceID:   traceID,
	}
}

func mustUnmarshalStrings(t *testing.T, content string) []string {
	t.Helper()
	var result []string
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		t.Fatalf("unmarshal []string from %q: %v", content, err)
	}
	return result
}

// assertMeta verifies CallID, LoopID, and TraceID are propagated from the
// ToolCall into the ToolResult.
func assertMeta(t *testing.T, result agentic.ToolResult, call agentic.ToolCall) {
	t.Helper()
	if result.CallID != call.ID {
		t.Errorf("CallID = %q, want %q", result.CallID, call.ID)
	}
	if result.LoopID != call.LoopID {
		t.Errorf("LoopID = %q, want %q", result.LoopID, call.LoopID)
	}
	if result.TraceID != call.TraceID {
		t.Errorf("TraceID = %q, want %q", result.TraceID, call.TraceID)
	}
}

// ----------------------------------------------------------------------------
// TestListTools
// ----------------------------------------------------------------------------

func TestListTools_ReturnsOneDefinitionWithExpectedShape(t *testing.T) {
	t.Parallel()

	exec := tree.NewExecutor(&mockGraph{})
	tools := exec.ListTools()

	if len(tools) != 1 {
		t.Fatalf("ListTools() returned %d definitions, want 1", len(tools))
	}

	def := tools[0]

	if def.Name != "query_agent_tree" {
		t.Errorf("Name = %q, want %q", def.Name, "query_agent_tree")
	}
	if def.Description == "" {
		t.Error("Description must not be empty")
	}
	if def.Parameters == nil {
		t.Fatal("Parameters must not be nil")
	}

	// "required" must list exactly ["operation"].
	required, ok := def.Parameters["required"].([]string)
	if !ok {
		t.Fatalf("Parameters[\"required\"] type = %T, want []string", def.Parameters["required"])
	}
	if len(required) != 1 || required[0] != "operation" {
		t.Errorf("required = %v, want [operation]", required)
	}

	// All three property keys must be declared.
	props, ok := def.Parameters["properties"].(map[string]any)
	if !ok {
		t.Fatalf("Parameters[\"properties\"] type = %T, want map[string]any", def.Parameters["properties"])
	}
	for _, wantKey := range []string{"operation", "loop_id", "max_depth"} {
		if _, exists := props[wantKey]; !exists {
			t.Errorf("property %q missing from parameters", wantKey)
		}
	}

	// operation enum must list all three operations.
	opProp, ok := props["operation"].(map[string]any)
	if !ok {
		t.Fatalf("properties[\"operation\"] type = %T, want map[string]any", props["operation"])
	}
	enum, ok := opProp["enum"].([]string)
	if !ok {
		t.Fatalf("properties[\"operation\"][\"enum\"] type = %T, want []string", opProp["enum"])
	}
	enumSet := make(map[string]bool, len(enum))
	for _, e := range enum {
		enumSet[e] = true
	}
	for _, wantOp := range []string{"get_children", "get_tree", "get_status"} {
		if !enumSet[wantOp] {
			t.Errorf("enum missing %q", wantOp)
		}
	}
}

// ----------------------------------------------------------------------------
// TestExecute_MissingOperation
// ----------------------------------------------------------------------------

func TestExecute_MissingOperation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args map[string]any
	}{
		{
			name: "no arguments at all",
			args: map[string]any{},
		},
		{
			name: "operation key absent with other keys present",
			args: map[string]any{"loop_id": "loop-1"},
		},
		{
			name: "operation is empty string",
			args: map[string]any{"operation": ""},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			exec := tree.NewExecutor(&mockGraph{})
			call := makeCall("c1", "loop-1", "trace-1", tt.args)

			result, err := exec.Execute(context.Background(), call)

			if err != nil {
				t.Fatalf("unexpected Go error: %v", err)
			}
			if result.Error == "" {
				t.Fatal("expected ToolResult.Error to be set, got empty string")
			}
			if !strings.Contains(result.Error, "operation") {
				t.Errorf("error %q does not mention \"operation\"", result.Error)
			}
			if result.Content != "" {
				t.Errorf("Content = %q, want empty on argument error", result.Content)
			}
			assertMeta(t, result, call)
		})
	}
}

// ----------------------------------------------------------------------------
// TestExecute_UnknownOperation
// ----------------------------------------------------------------------------

func TestExecute_UnknownOperation(t *testing.T) {
	t.Parallel()

	exec := tree.NewExecutor(&mockGraph{})
	call := makeCall("c2", "loop-1", "trace-1", map[string]any{
		"operation": "delete_all_agents",
	})

	result, err := exec.Execute(context.Background(), call)

	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected ToolResult.Error to be set for unknown operation")
	}
	if !strings.Contains(result.Error, "delete_all_agents") {
		t.Errorf("error %q does not contain unknown op name", result.Error)
	}
	assertMeta(t, result, call)
}

// ----------------------------------------------------------------------------
// TestExecute_GetChildren
// ----------------------------------------------------------------------------

func TestExecute_GetChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          map[string]any
		mock          *mockGraph
		wantErrSubstr string // non-empty → expect ToolResult.Error containing this
		wantChildren  []string
	}{
		{
			name:          "missing loop_id argument",
			args:          map[string]any{"operation": "get_children"},
			mock:          &mockGraph{},
			wantErrSubstr: "loop_id",
		},
		{
			name:          "empty loop_id argument",
			args:          map[string]any{"operation": "get_children", "loop_id": ""},
			mock:          &mockGraph{},
			wantErrSubstr: "loop_id",
		},
		{
			name:         "successful: returns child IDs",
			args:         map[string]any{"operation": "get_children", "loop_id": "parent-loop"},
			mock:         &mockGraph{children: []string{"child-a", "child-b"}},
			wantChildren: []string{"child-a", "child-b"},
		},
		{
			name:         "successful: empty children slice",
			args:         map[string]any{"operation": "get_children", "loop_id": "leaf-loop"},
			mock:         &mockGraph{}, // nil children → returns []string{}
			wantChildren: []string{},
		},
		{
			name:          "graph error surfaced as ToolResult.Error",
			args:          map[string]any{"operation": "get_children", "loop_id": "bad-loop"},
			mock:          &mockGraph{childrenErr: errors.New("nats: timeout")},
			wantErrSubstr: "nats: timeout",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			exec := tree.NewExecutor(tt.mock)
			call := makeCall("c3", "caller-loop", "trace-2", tt.args)

			result, err := exec.Execute(context.Background(), call)

			if err != nil {
				t.Fatalf("unexpected Go error: %v", err)
			}
			assertMeta(t, result, call)

			if tt.wantErrSubstr != "" {
				if result.Error == "" {
					t.Fatal("expected ToolResult.Error, got empty string")
				}
				if !strings.Contains(result.Error, tt.wantErrSubstr) {
					t.Errorf("error %q does not contain %q", result.Error, tt.wantErrSubstr)
				}
				return
			}

			if result.Error != "" {
				t.Fatalf("unexpected ToolResult.Error: %s", result.Error)
			}

			got := mustUnmarshalStrings(t, result.Content)
			if len(got) != len(tt.wantChildren) {
				t.Fatalf("children count = %d, want %d — got %v", len(got), len(tt.wantChildren), got)
			}
			for i, c := range got {
				if c != tt.wantChildren[i] {
					t.Errorf("children[%d] = %q, want %q", i, c, tt.wantChildren[i])
				}
			}

			// Verify mock received the correct loop_id.
			if loopArg, _ := tt.args["loop_id"].(string); loopArg != "" {
				if got := tt.mock.capturedGetChildren(); got != loopArg {
					t.Errorf("GetChildren called with %q, want %q", got, loopArg)
				}
			}
		})
	}
}

// ----------------------------------------------------------------------------
// TestExecute_GetTree
// ----------------------------------------------------------------------------

func TestExecute_GetTree(t *testing.T) {
	t.Parallel()

	const callLoopID = "calling-loop"

	tests := []struct {
		name           string
		args           map[string]any
		mock           *mockGraph
		wantErrSubstr  string
		wantTree       []string
		wantRootLoopID string // expected value forwarded to GetTree
		wantMaxDepth   int    // expected maxDepth forwarded to GetTree
	}{
		{
			name:           "uses loop_id from arguments",
			args:           map[string]any{"operation": "get_tree", "loop_id": "explicit-root"},
			mock:           &mockGraph{treeIDs: []string{"explicit-root", "child-x"}},
			wantTree:       []string{"explicit-root", "child-x"},
			wantRootLoopID: "explicit-root",
			wantMaxDepth:   10,
		},
		{
			name:           "falls back to call.LoopID when no loop_id arg",
			args:           map[string]any{"operation": "get_tree"},
			mock:           &mockGraph{treeIDs: []string{callLoopID}},
			wantTree:       []string{callLoopID},
			wantRootLoopID: callLoopID,
			wantMaxDepth:   10,
		},
		{
			name:           "falls back to call.LoopID when loop_id is empty string",
			args:           map[string]any{"operation": "get_tree", "loop_id": ""},
			mock:           &mockGraph{treeIDs: []string{callLoopID}},
			wantTree:       []string{callLoopID},
			wantRootLoopID: callLoopID,
			wantMaxDepth:   10,
		},
		{
			name:           "custom max_depth as int",
			args:           map[string]any{"operation": "get_tree", "loop_id": "root-1", "max_depth": 3},
			mock:           &mockGraph{treeIDs: []string{"root-1"}},
			wantTree:       []string{"root-1"},
			wantRootLoopID: "root-1",
			wantMaxDepth:   3,
		},
		{
			name:           "custom max_depth as float64 (JSON unmarshal style)",
			args:           map[string]any{"operation": "get_tree", "loop_id": "root-2", "max_depth": float64(5)},
			mock:           &mockGraph{treeIDs: []string{"root-2"}},
			wantTree:       []string{"root-2"},
			wantRootLoopID: "root-2",
			wantMaxDepth:   5,
		},
		{
			name:           "zero max_depth is ignored, default 10 used",
			args:           map[string]any{"operation": "get_tree", "loop_id": "root-3", "max_depth": 0},
			mock:           &mockGraph{treeIDs: []string{"root-3"}},
			wantTree:       []string{"root-3"},
			wantRootLoopID: "root-3",
			wantMaxDepth:   10,
		},
		{
			name:          "graph error surfaced as ToolResult.Error",
			args:          map[string]any{"operation": "get_tree", "loop_id": "bad-root"},
			mock:          &mockGraph{treeErr: errors.New("query: depth exceeded")},
			wantErrSubstr: "query: depth exceeded",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			exec := tree.NewExecutor(tt.mock)
			call := makeCall("c4", callLoopID, "trace-3", tt.args)

			result, err := exec.Execute(context.Background(), call)

			if err != nil {
				t.Fatalf("unexpected Go error: %v", err)
			}
			assertMeta(t, result, call)

			if tt.wantErrSubstr != "" {
				if result.Error == "" {
					t.Fatal("expected ToolResult.Error, got empty string")
				}
				if !strings.Contains(result.Error, tt.wantErrSubstr) {
					t.Errorf("error %q does not contain %q", result.Error, tt.wantErrSubstr)
				}
				return
			}

			if result.Error != "" {
				t.Fatalf("unexpected ToolResult.Error: %s", result.Error)
			}

			got := mustUnmarshalStrings(t, result.Content)
			if len(got) != len(tt.wantTree) {
				t.Fatalf("tree count = %d, want %d — got %v", len(got), len(tt.wantTree), got)
			}
			for i, id := range got {
				if id != tt.wantTree[i] {
					t.Errorf("tree[%d] = %q, want %q", i, id, tt.wantTree[i])
				}
			}

			// Verify arguments forwarded to the mock.
			gotRootLoopID, gotMaxDepth := tt.mock.capturedGetTree()
			if gotRootLoopID != tt.wantRootLoopID {
				t.Errorf("GetTree rootLoopID = %q, want %q", gotRootLoopID, tt.wantRootLoopID)
			}
			if gotMaxDepth != tt.wantMaxDepth {
				t.Errorf("GetTree maxDepth = %d, want %d", gotMaxDepth, tt.wantMaxDepth)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// TestExecute_GetStatus
// ----------------------------------------------------------------------------

func TestExecute_GetStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          map[string]any
		mock          *mockGraph
		wantErrSubstr string
		wantStatus    string
	}{
		{
			name:          "missing loop_id argument",
			args:          map[string]any{"operation": "get_status"},
			mock:          &mockGraph{},
			wantErrSubstr: "loop_id",
		},
		{
			name:          "empty loop_id argument",
			args:          map[string]any{"operation": "get_status", "loop_id": ""},
			mock:          &mockGraph{},
			wantErrSubstr: "loop_id",
		},
		{
			name:       "successful: returns running status",
			args:       map[string]any{"operation": "get_status", "loop_id": "agent-loop-42"},
			mock:       &mockGraph{status: "running"},
			wantStatus: "running",
		},
		{
			name:       "successful: returns completed status",
			args:       map[string]any{"operation": "get_status", "loop_id": "done-loop"},
			mock:       &mockGraph{status: "completed"},
			wantStatus: "completed",
		},
		{
			name:          "graph error surfaced as ToolResult.Error",
			args:          map[string]any{"operation": "get_status", "loop_id": "err-loop"},
			mock:          &mockGraph{statusErr: errors.New("entity not found")},
			wantErrSubstr: "entity not found",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			exec := tree.NewExecutor(tt.mock)
			call := makeCall("c5", "caller-loop", "trace-4", tt.args)

			result, err := exec.Execute(context.Background(), call)

			if err != nil {
				t.Fatalf("unexpected Go error: %v", err)
			}
			assertMeta(t, result, call)

			if tt.wantErrSubstr != "" {
				if result.Error == "" {
					t.Fatal("expected ToolResult.Error, got empty string")
				}
				if !strings.Contains(result.Error, tt.wantErrSubstr) {
					t.Errorf("error %q does not contain %q", result.Error, tt.wantErrSubstr)
				}
				return
			}

			if result.Error != "" {
				t.Fatalf("unexpected ToolResult.Error: %s", result.Error)
			}

			// Response must be a JSON object with loop_id and status fields.
			var body map[string]string
			if err := json.Unmarshal([]byte(result.Content), &body); err != nil {
				t.Fatalf("Content is not a valid JSON object: %v — content: %s", err, result.Content)
			}

			loopArg := tt.args["loop_id"].(string)
			if body["loop_id"] != loopArg {
				t.Errorf("body[\"loop_id\"] = %q, want %q", body["loop_id"], loopArg)
			}
			if body["status"] != tt.wantStatus {
				t.Errorf("body[\"status\"] = %q, want %q", body["status"], tt.wantStatus)
			}

			// Verify mock received the correct loop_id.
			if got := tt.mock.capturedGetStatus(); got != loopArg {
				t.Errorf("GetStatus called with %q, want %q", got, loopArg)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// TestExecute_MetaPropagation — CallID/LoopID/TraceID always round-trip.
// ----------------------------------------------------------------------------

func TestExecute_MetaPropagation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
	}{
		{
			name: "propagated on operation error",
			args: map[string]any{"operation": "get_children"}, // missing loop_id
		},
		{
			name: "propagated on unknown operation",
			args: map[string]any{"operation": "noop"},
		},
		{
			name: "propagated on missing operation key",
			args: map[string]any{},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			exec := tree.NewExecutor(&mockGraph{})
			call := makeCall("my-id", "my-loop", "my-trace", tc.args)

			result, _ := exec.Execute(context.Background(), call)

			assertMeta(t, result, call)
		})
	}
}
