// Package tree implements the query_agent_tree tool executor.
// It is a thin wrapper around agentgraph.Helper that exposes agent hierarchy
// inspection operations (get_children, get_tree, get_status) to the LLM via
// the agentic ToolExecutor contract.
package tree

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/c360studio/semstreams/agentic"
)

const (
	toolName        = "query_agent_tree"
	defaultMaxDepth = 10

	opGetChildren = "get_children"
	opGetTree     = "get_tree"
	opGetStatus   = "get_status"
)

// GraphQuerier is the subset of agentgraph.Helper methods used by Executor.
// Exporting it allows callers to pass the concrete agentgraph.Helper without
// importing the agentgraph package directly.
type GraphQuerier interface {
	GetChildren(ctx context.Context, loopID string) ([]string, error)
	GetTree(ctx context.Context, rootLoopID string, maxDepth int) ([]string, error)
	GetStatus(ctx context.Context, loopID string) (string, error)
}

// Executor implements agentic.ToolExecutor for the query_agent_tree tool.
// All public methods are safe for concurrent use — the struct itself holds no
// mutable state beyond the GraphQuerier, which is expected to be thread-safe.
type Executor struct {
	graph GraphQuerier
}

// NewExecutor constructs an Executor backed by the provided GraphQuerier.
// graph must not be nil; passing nil will cause panics at call time.
func NewExecutor(graph GraphQuerier) *Executor {
	return &Executor{graph: graph}
}

// Execute dispatches the tool call to the appropriate graph operation and
// returns the result as a JSON-encoded Content string.
// Argument validation errors and graph query errors are surfaced as non-nil
// ToolResult.Error strings rather than Go errors — the caller (agentic-tools)
// forwards these to the LLM as structured tool results.
func (e *Executor) Execute(ctx context.Context, call agentic.ToolCall) (agentic.ToolResult, error) {
	op, ok := stringArg(call.Arguments, "operation")
	if !ok || op == "" {
		return errorResult(call, `missing required argument "operation"`), nil
	}

	switch op {
	case opGetChildren:
		return e.execGetChildren(ctx, call)
	case opGetTree:
		return e.execGetTree(ctx, call)
	case opGetStatus:
		return e.execGetStatus(ctx, call)
	default:
		return errorResult(call, fmt.Sprintf("unknown operation %q; valid values: get_children, get_tree, get_status", op)), nil
	}
}

// ListTools returns the single tool definition for query_agent_tree.
func (e *Executor) ListTools() []agentic.ToolDefinition {
	return []agentic.ToolDefinition{{
		Name:        toolName,
		Description: "Query the agent hierarchy to inspect parent-child relationships, list children of a loop, or check loop status.",
		Parameters: map[string]any{
			"type":     "object",
			"required": []string{"operation"},
			"properties": map[string]any{
				"operation": map[string]any{
					"type":        "string",
					"enum":        []string{opGetChildren, opGetTree, opGetStatus},
					"description": "The query operation to perform",
				},
				"loop_id": map[string]any{
					"type":        "string",
					"description": "Target loop ID. Required for get_children and get_status. Defaults to calling loop for get_tree.",
				},
				"max_depth": map[string]any{
					"type":        "integer",
					"description": "Maximum tree traversal depth (get_tree only, default 10)",
				},
			},
		},
	}}
}

// -- operation handlers --

func (e *Executor) execGetChildren(ctx context.Context, call agentic.ToolCall) (agentic.ToolResult, error) {
	loopID, ok := stringArg(call.Arguments, "loop_id")
	if !ok || loopID == "" {
		return errorResult(call, `get_children requires "loop_id" argument`), nil
	}

	children, err := e.graph.GetChildren(ctx, loopID)
	if err != nil {
		return errorResult(call, fmt.Sprintf("get_children failed: %s", err)), nil
	}

	return jsonResult(call, children)
}

func (e *Executor) execGetTree(ctx context.Context, call agentic.ToolCall) (agentic.ToolResult, error) {
	// loop_id defaults to the calling agent's own loop when not provided.
	loopID, ok := stringArg(call.Arguments, "loop_id")
	if !ok || loopID == "" {
		loopID = call.LoopID
	}

	maxDepth := defaultMaxDepth
	if d, ok := intArg(call.Arguments, "max_depth"); ok && d > 0 {
		maxDepth = d
	}

	ids, err := e.graph.GetTree(ctx, loopID, maxDepth)
	if err != nil {
		return errorResult(call, fmt.Sprintf("get_tree failed: %s", err)), nil
	}

	return jsonResult(call, ids)
}

func (e *Executor) execGetStatus(ctx context.Context, call agentic.ToolCall) (agentic.ToolResult, error) {
	loopID, ok := stringArg(call.Arguments, "loop_id")
	if !ok || loopID == "" {
		return errorResult(call, `get_status requires "loop_id" argument`), nil
	}

	status, err := e.graph.GetStatus(ctx, loopID)
	if err != nil {
		return errorResult(call, fmt.Sprintf("get_status failed: %s", err)), nil
	}

	return jsonResult(call, map[string]string{"loop_id": loopID, "status": status})
}

// -- helpers --

// jsonResult marshals v to JSON and returns a successful ToolResult.
func jsonResult(call agentic.ToolCall, v any) (agentic.ToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return errorResult(call, fmt.Sprintf("failed to marshal result: %s", err)), nil
	}
	return agentic.ToolResult{
		CallID:  call.ID,
		Content: string(data),
		LoopID:  call.LoopID,
		TraceID: call.TraceID,
	}, nil
}

// errorResult returns a ToolResult carrying an error message with no Go error.
func errorResult(call agentic.ToolCall, msg string) agentic.ToolResult {
	return agentic.ToolResult{
		CallID:  call.ID,
		Error:   msg,
		LoopID:  call.LoopID,
		TraceID: call.TraceID,
	}
}

// stringArg extracts a string value from arguments by key.
func stringArg(args map[string]any, key string) (string, bool) {
	v, ok := args[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// intArg extracts an integer value from arguments by key.
// JSON numbers unmarshalled into map[string]any arrive as float64, so both
// float64 and int are handled.
func intArg(args map[string]any, key string) (int, bool) {
	v, ok := args[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		return int(n), true
	}
	return 0, false
}
