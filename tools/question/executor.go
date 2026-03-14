// Package question implements the raise_question tool executor.
// It allows agents to explicitly create structured questions that are
// routed to the appropriate answerer (human, agent, or team).
package question

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/c360studio/semspec/workflow"
	"github.com/c360studio/semspec/workflow/answerer"
	"github.com/c360studio/semstreams/agentic"
)

const toolName = "raise_question"

// QuestionStorer is the subset of workflow.QuestionStore needed by this tool.
type QuestionStorer interface {
	Store(ctx context.Context, q *workflow.Question) error
}

// Executor implements agentic.ToolExecutor for the raise_question tool.
type Executor struct {
	store  QuestionStorer
	router *answerer.Router // optional — nil means no routing
}

// NewExecutor constructs a raise_question Executor.
func NewExecutor(store QuestionStorer, router *answerer.Router) *Executor {
	return &Executor{
		store:  store,
		router: router,
	}
}

// Execute creates a question from the tool call arguments, stores it,
// and optionally routes it to an answerer. Returns the question ID.
func (e *Executor) Execute(ctx context.Context, call agentic.ToolCall) (agentic.ToolResult, error) {
	topic := stringArg(call.Arguments, "topic")
	if topic == "" {
		return errorResult(call, `missing required argument "topic"`), nil
	}

	questionText := stringArg(call.Arguments, "question")
	if questionText == "" {
		return errorResult(call, `missing required argument "question"`), nil
	}

	questionCtx := stringArg(call.Arguments, "context")
	category := stringArg(call.Arguments, "category")
	urgency := stringArg(call.Arguments, "urgency")
	fromAgent := stringArg(call.Arguments, "from_agent")
	if fromAgent == "" {
		fromAgent = "agent"
	}

	// Parse metadata from arguments.
	metadata := parseMetadata(call.Arguments)

	// Validate and resolve category.
	cat := workflow.QuestionCategoryKnowledge
	switch workflow.QuestionCategory(category) {
	case workflow.QuestionCategoryEnvironment:
		cat = workflow.QuestionCategoryEnvironment
	case workflow.QuestionCategoryApproval:
		cat = workflow.QuestionCategoryApproval
	case workflow.QuestionCategoryKnowledge, "":
		cat = workflow.QuestionCategoryKnowledge
	default:
		return errorResult(call, fmt.Sprintf("unknown category %q; valid values: knowledge, environment, approval", category)), nil
	}

	q := workflow.NewCategorizedQuestion(fromAgent, topic, questionText, questionCtx, cat, metadata)

	// Override urgency if provided.
	switch workflow.QuestionUrgency(urgency) {
	case workflow.QuestionUrgencyLow:
		q.Urgency = workflow.QuestionUrgencyLow
	case workflow.QuestionUrgencyHigh:
		q.Urgency = workflow.QuestionUrgencyHigh
	case workflow.QuestionUrgencyBlocking:
		q.Urgency = workflow.QuestionUrgencyBlocking
	}

	// Propagate trace context from the tool call.
	if traceID := stringArg(call.Arguments, "trace_id"); traceID != "" {
		q.TraceID = traceID
	}
	if taskID := stringArg(call.Arguments, "task_id"); taskID != "" {
		q.TaskID = taskID
	}

	// Store the question.
	if err := e.store.Store(ctx, q); err != nil {
		return agentic.ToolResult{}, fmt.Errorf("store question: %w", err)
	}

	// Route if router is available.
	var routeMsg string
	if e.router != nil {
		result, err := e.router.RouteQuestion(ctx, q)
		if err != nil {
			routeMsg = fmt.Sprintf("stored but routing failed: %v", err)
		} else {
			routeMsg = result.Message
			// Re-store if routing modified assignment fields.
			if q.AssignedTo != "" {
				if storeErr := e.store.Store(ctx, q); storeErr != nil {
					routeMsg += fmt.Sprintf(" (warning: failed to persist assignment: %v)", storeErr)
				}
			}
		}
	}

	response := map[string]any{
		"question_id": q.ID,
		"status":      string(q.Status),
		"category":    string(q.Category),
		"urgency":     string(q.Urgency),
	}
	if routeMsg != "" {
		response["routing"] = routeMsg
	}

	return jsonResult(call, response)
}

// ListTools returns the single tool definition for raise_question.
func (e *Executor) ListTools() []agentic.ToolDefinition {
	return []agentic.ToolDefinition{{
		Name:        toolName,
		Description: "Raise a question to a human or agent when you encounter a blocker you cannot resolve autonomously. Use category 'environment' for missing tools or sandbox issues, 'approval' for decisions requiring human sign-off, and 'knowledge' for information gaps.",
		Parameters: map[string]any{
			"type":     "object",
			"required": []string{"topic", "question"},
			"properties": map[string]any{
				"topic": map[string]any{
					"type":        "string",
					"description": "Hierarchical topic for routing (e.g., 'environment.sandbox.missing-tool', 'approval.deployment')",
				},
				"question": map[string]any{
					"type":        "string",
					"description": "The question or request text",
				},
				"context": map[string]any{
					"type":        "string",
					"description": "Background context to help the answerer",
				},
				"category": map[string]any{
					"type":        "string",
					"enum":        []string{"knowledge", "environment", "approval"},
					"description": "Question category (default: knowledge)",
				},
				"urgency": map[string]any{
					"type":        "string",
					"enum":        []string{"low", "normal", "high", "blocking"},
					"description": "Urgency level (default: normal). Use 'blocking' to pause execution until answered.",
				},
				"metadata": map[string]any{
					"type":        "object",
					"description": "Structured key-value metadata (e.g., {\"command\": \"cargo build\", \"exit_code\": \"127\", \"missing_tool\": \"cargo\"})",
					"additionalProperties": map[string]any{
						"type": "string",
					},
				},
				"from_agent": map[string]any{
					"type":        "string",
					"description": "Identity of the agent raising the question (e.g., 'developer', 'architect'). Defaults to 'agent'.",
				},
				"trace_id": map[string]any{
					"type":        "string",
					"description": "Trace ID for correlation (optional)",
				},
				"task_id": map[string]any{
					"type":        "string",
					"description": "Task ID this question relates to (optional)",
				},
			},
		},
	}}
}

// stringArg extracts a string argument from the tool call.
func stringArg(args map[string]any, key string) string {
	v, ok := args[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// parseMetadata extracts the metadata map from arguments.
func parseMetadata(args map[string]any) map[string]string {
	raw, ok := args["metadata"]
	if !ok {
		return nil
	}

	switch v := raw.(type) {
	case map[string]any:
		m := make(map[string]string, len(v))
		for k, val := range v {
			m[k] = fmt.Sprintf("%v", val)
		}
		return m
	case map[string]string:
		return v
	default:
		return nil
	}
}

func errorResult(call agentic.ToolCall, msg string) agentic.ToolResult {
	return agentic.ToolResult{
		CallID:  call.ID,
		Content: msg,
		Error:   msg,
	}
}

func jsonResult(call agentic.ToolCall, data any) (agentic.ToolResult, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return agentic.ToolResult{}, fmt.Errorf("marshal result: %w", err)
	}
	return agentic.ToolResult{
		CallID:  call.ID,
		Content: string(b),
	}, nil
}
