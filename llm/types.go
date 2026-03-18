// Package llm provides a provider-agnostic LLM client with retry and fallback support.
package llm

import (
	"context"
	"sort"
	"time"

	"github.com/c360studio/semstreams/message"
)

// CallRecord represents a single LLM API call with full context for trajectory tracking.
type CallRecord struct {
	// RequestID uniquely identifies this LLM call.
	RequestID string `json:"request_id"`

	// TraceID correlates this call with other messages in the same request flow.
	TraceID string `json:"trace_id"`

	// LoopID is the agent loop that initiated this call (if any).
	LoopID string `json:"loop_id,omitempty"`

	// Capability is the semantic capability requested (planning, writing, coding, etc.).
	Capability string `json:"capability"`

	// Model is the actual model that was used for this call.
	Model string `json:"model"`

	// Provider is the LLM provider (anthropic, ollama, openai, etc.).
	Provider string `json:"provider"`

	// Messages is the input message history sent to the LLM.
	Messages []Message `json:"messages"`

	// Response is the generated content from the LLM.
	Response string `json:"response"`

	// PromptTokens is the number of input/prompt tokens consumed.
	PromptTokens int `json:"prompt_tokens"`

	// CompletionTokens is the number of output/completion tokens generated.
	CompletionTokens int `json:"completion_tokens"`

	// TotalTokens is the total tokens consumed (prompt + completion).
	TotalTokens int `json:"total_tokens"`

	// ContextBudget is the maximum context window size for this model (optional).
	ContextBudget int `json:"context_budget,omitempty"`

	// ContextTruncated indicates if context was truncated to fit budget (optional).
	ContextTruncated bool `json:"context_truncated,omitempty"`

	// FinishReason indicates why generation stopped (stop, length, tool_use, etc.).
	FinishReason string `json:"finish_reason"`

	// StartedAt is when the LLM call began.
	StartedAt time.Time `json:"started_at"`

	// CompletedAt is when the LLM call finished.
	CompletedAt time.Time `json:"completed_at"`

	// DurationMs is the call duration in milliseconds.
	DurationMs int64 `json:"duration_ms"`

	// Error contains any error message if the call failed.
	Error string `json:"error,omitempty"`

	// Retries is the number of retry attempts made.
	Retries int `json:"retries"`

	// FallbacksUsed lists models tried before success (if fallback was needed).
	FallbacksUsed []string `json:"fallbacks_used,omitempty"`

	// MessagesCount is the number of messages (used for graph representation).
	MessagesCount int `json:"messages_count,omitempty"`

	// ResponsePreview is a truncated response preview (first 500 chars).
	ResponsePreview string `json:"response_preview,omitempty"`

	// StorageRef is kept for backwards compatibility.
	StorageRef *message.StorageReference `json:"storage_ref,omitempty"`
}

// ToolCallRecord represents a single tool execution with context for trajectory tracking.
type ToolCallRecord struct {
	// CallID uniquely identifies this tool call.
	CallID string `json:"call_id"`

	// TraceID correlates this call with other messages in the same request flow.
	TraceID string `json:"trace_id"`

	// LoopID is the agent loop that initiated this call (if any).
	LoopID string `json:"loop_id,omitempty"`

	// ToolName is the name of the tool executed (e.g. "file_read", "git_status").
	ToolName string `json:"tool_name"`

	// Parameters is the JSON-encoded tool parameters (truncated for storage).
	Parameters string `json:"parameters"`

	// Result is the truncated output from the tool execution.
	Result string `json:"result"`

	// Status is the execution status ("success", "error").
	Status string `json:"status"`

	// Error contains any error message if the call failed.
	Error string `json:"error,omitempty"`

	// StartedAt is when the tool call began.
	StartedAt time.Time `json:"started_at"`

	// CompletedAt is when the tool call finished.
	CompletedAt time.Time `json:"completed_at"`

	// DurationMs is the call duration in milliseconds.
	DurationMs int64 `json:"duration_ms"`
}

// SortByStartTime sorts LLM call records chronologically by StartedAt.
// Exported for use by trajectory-api and other packages.
func SortByStartTime(records []*CallRecord) {
	sort.Slice(records, func(i, j int) bool {
		return records[i].StartedAt.Before(records[j].StartedAt)
	})
}

// SortToolCallsByStartTime sorts tool call records chronologically by StartedAt.
// Exported for use by trajectory-api and other packages.
func SortToolCallsByStartTime(records []*ToolCallRecord) {
	sort.Slice(records, func(i, j int) bool {
		return records[i].StartedAt.Before(records[j].StartedAt)
	})
}

// TraceContext holds trace information extracted from context.
type TraceContext struct {
	TraceID string
	LoopID  string
}

// traceContextKey is the context key for trace information.
type traceContextKey struct{}

// WithTraceContext adds trace information to a context.
func WithTraceContext(ctx context.Context, tc TraceContext) context.Context {
	return context.WithValue(ctx, traceContextKey{}, tc)
}

// GetTraceContext extracts trace information from a context.
func GetTraceContext(ctx context.Context) TraceContext {
	if tc, ok := ctx.Value(traceContextKey{}).(TraceContext); ok {
		return tc
	}
	return TraceContext{}
}
