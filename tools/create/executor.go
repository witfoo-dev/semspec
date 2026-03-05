package create

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/c360studio/semstreams/agentic"
)

const toolName = "create_tool"

// Executor implements agentic.ToolExecutor for the create_tool tool.
// It is a passthrough executor: the LLM provides a FlowSpec in its tool call
// arguments, the executor validates it and returns the validated spec as JSON.
//
// This is an MVP — no actual tool registration or execution takes place.
// In a future phase, validated specs would be registered with agentic-tools
// so subsequent agent calls can invoke the dynamically created tool.
//
// All public methods are safe for concurrent use — the struct holds no
// mutable state.
type Executor struct{}

// NewExecutor constructs a create_tool Executor.
func NewExecutor() *Executor {
	return &Executor{}
}

// ListTools returns the single tool definition for create_tool.
func (e *Executor) ListTools() []agentic.ToolDefinition {
	return []agentic.ToolDefinition{{
		Name:        toolName,
		Description: "Define a new tool by providing a FlowSpec that describes the tool's name, description, input parameters, processor pipeline, and data wiring. The validated spec is returned for confirmation. In a future phase, the tool will be registered and available for immediate use.",
		Parameters: map[string]any{
			"type":     "object",
			"required": []string{"name", "description", "processors"},
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Unique tool name. Alphanumeric plus underscores, max 64 characters.",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Human-readable description of what the tool does.",
				},
				"parameters": map[string]any{
					"type":        "object",
					"description": "JSON Schema object describing the tool's input parameters.",
				},
				"processors": map[string]any{
					"type":        "array",
					"description": "Ordered list of processor steps that form the tool's execution pipeline.",
					"items": map[string]any{
						"type":     "object",
						"required": []string{"id", "component"},
						"properties": map[string]any{
							"id": map[string]any{
								"type":        "string",
								"description": "Unique step ID within the flow. Referenced by wiring rules.",
							},
							"component": map[string]any{
								"type":        "string",
								"description": "Registered semstreams component type (e.g., 'llm-agent').",
							},
							"config": map[string]any{
								"type":        "object",
								"description": "Step-specific configuration overrides.",
							},
						},
					},
				},
				"wiring": map[string]any{
					"type":        "array",
					"description": "Data routing rules between processors and the tool's input/output boundaries.",
					"items": map[string]any{
						"type":     "object",
						"required": []string{"from", "to"},
						"properties": map[string]any{
							"from": map[string]any{
								"type":        "string",
								"description": "Source path: 'input.<field>' or '<step-id>.output.<field>'.",
							},
							"to": map[string]any{
								"type":        "string",
								"description": "Destination path: '<step-id>.input.<field>' or 'output.<field>'.",
							},
						},
					},
				},
			},
		},
	}}
}

// Execute validates the FlowSpec provided by the LLM in the tool call arguments
// and returns the validated spec as JSON in ToolResult.Content.
//
// Validation errors are surfaced as non-nil ToolResult.Error strings rather
// than Go errors. Go errors are reserved for infrastructure failures that
// the agentic-tools dispatcher should treat as fatal — none arise in this
// passthrough implementation.
func (e *Executor) Execute(_ context.Context, call agentic.ToolCall) (agentic.ToolResult, error) {
	spec, err := parseFlowSpec(call.Arguments)
	if err != nil {
		return errorResult(call, err.Error()), nil
	}

	if err := spec.Validate(); err != nil {
		return errorResult(call, fmt.Sprintf("invalid flow spec: %s", err)), nil
	}

	return jsonResult(call, spec)
}

// ---------------------------------------------------------------------------
// Argument parsing
// ---------------------------------------------------------------------------

// parseFlowSpec converts the raw arguments map from a ToolCall into a FlowSpec.
// It performs a round-trip through JSON to handle the nested structures that
// arrive as map[string]any from the agentic tool call deserialization.
func parseFlowSpec(args map[string]any) (*FlowSpec, error) {
	data, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("marshal arguments: %w", err)
	}

	var spec FlowSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parse flow spec from arguments: %w", err)
	}
	return &spec, nil
}

// ---------------------------------------------------------------------------
// Result helpers
// ---------------------------------------------------------------------------

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

// errorResult returns a ToolResult carrying an error message.
// The distinction between ToolResult.Error and a Go error matters: Go errors
// signal infrastructure failures to the dispatcher, while ToolResult.Error is
// forwarded to the LLM as structured feedback.
func errorResult(call agentic.ToolCall, msg string) agentic.ToolResult {
	return agentic.ToolResult{
		CallID:  call.ID,
		Error:   msg,
		LoopID:  call.LoopID,
		TraceID: call.TraceID,
	}
}
