package question

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/c360studio/semspec/workflow"
	"github.com/c360studio/semstreams/agentic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStore captures stored questions for assertions.
type mockStore struct {
	stored []*workflow.Question
}

func (m *mockStore) Store(_ context.Context, q *workflow.Question) error {
	m.stored = append(m.stored, q)
	return nil
}

func TestExecute_MissingTopic(t *testing.T) {
	exec := NewExecutor(nil, nil)
	result, err := exec.Execute(context.Background(), agentic.ToolCall{
		ID:        "call-1",
		Arguments: map[string]any{"question": "what?"},
	})

	require.NoError(t, err)
	assert.Contains(t, result.Error, "topic")
}

func TestExecute_MissingQuestion(t *testing.T) {
	exec := NewExecutor(nil, nil)
	result, err := exec.Execute(context.Background(), agentic.ToolCall{
		ID:        "call-1",
		Arguments: map[string]any{"topic": "env.sandbox"},
	})

	require.NoError(t, err)
	assert.Contains(t, result.Error, "question")
}

func TestListTools(t *testing.T) {
	exec := NewExecutor(nil, nil)
	tools := exec.ListTools()

	require.Len(t, tools, 1)
	assert.Equal(t, "raise_question", tools[0].Name)

	// Verify required parameters
	params := tools[0].Parameters
	required, ok := params["required"].([]string)
	require.True(t, ok)
	assert.Contains(t, required, "topic")
	assert.Contains(t, required, "question")
}

func TestExecute_HappyPath_Knowledge(t *testing.T) {
	store := &mockStore{}
	exec := NewExecutor(store, nil)

	result, err := exec.Execute(context.Background(), agentic.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"topic":    "api.users",
			"question": "How do I create a user?",
			"context":  "Building auth system",
		},
	})

	require.NoError(t, err)
	assert.Empty(t, result.Error)
	require.Len(t, store.stored, 1)

	q := store.stored[0]
	assert.Equal(t, "api.users", q.Topic)
	assert.Equal(t, "How do I create a user?", q.Question)
	assert.Equal(t, "Building auth system", q.Context)
	assert.Equal(t, workflow.QuestionCategoryKnowledge, q.Category)
	assert.Equal(t, workflow.QuestionUrgencyNormal, q.Urgency)
	assert.Equal(t, "agent", q.FromAgent)

	// Parse response JSON
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content), &resp))
	assert.Equal(t, q.ID, resp["question_id"])
	assert.Equal(t, "knowledge", resp["category"])
}

func TestExecute_HappyPath_Environment(t *testing.T) {
	store := &mockStore{}
	exec := NewExecutor(store, nil)

	result, err := exec.Execute(context.Background(), agentic.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"topic":      "environment.sandbox.missing-tool",
			"question":   "cargo is not installed",
			"category":   "environment",
			"urgency":    "blocking",
			"from_agent": "developer",
			"metadata": map[string]any{
				"command":      "cargo build",
				"exit_code":    "127",
				"missing_tool": "cargo",
			},
		},
	})

	require.NoError(t, err)
	assert.Empty(t, result.Error)
	require.Len(t, store.stored, 1)

	q := store.stored[0]
	assert.Equal(t, workflow.QuestionCategoryEnvironment, q.Category)
	assert.Equal(t, workflow.QuestionUrgencyBlocking, q.Urgency)
	assert.Equal(t, "developer", q.FromAgent)
	assert.Equal(t, "cargo build", q.Metadata["command"])
	assert.Equal(t, "127", q.Metadata["exit_code"])
	assert.Equal(t, "cargo", q.Metadata["missing_tool"])
}

func TestExecute_UnknownCategory(t *testing.T) {
	store := &mockStore{}
	exec := NewExecutor(store, nil)

	result, err := exec.Execute(context.Background(), agentic.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"topic":    "foo",
			"question": "bar",
			"category": "bogus",
		},
	})

	require.NoError(t, err)
	assert.Contains(t, result.Error, "unknown category")
	assert.Empty(t, store.stored) // nothing stored
}

func TestExecute_PropagatesTraceContext(t *testing.T) {
	store := &mockStore{}
	exec := NewExecutor(store, nil)

	_, err := exec.Execute(context.Background(), agentic.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"topic":    "test",
			"question": "test?",
			"trace_id": "trace-abc",
			"task_id":  "task-xyz",
		},
	})

	require.NoError(t, err)
	require.Len(t, store.stored, 1)
	assert.Equal(t, "trace-abc", store.stored[0].TraceID)
	assert.Equal(t, "task-xyz", store.stored[0].TaskID)
}

func TestParseMetadata_MapStringAny(t *testing.T) {
	args := map[string]any{
		"metadata": map[string]any{
			"command":      "cargo build",
			"exit_code":    "127",
			"missing_tool": "cargo",
		},
	}

	m := parseMetadata(args)
	assert.Equal(t, "cargo build", m["command"])
	assert.Equal(t, "127", m["exit_code"])
	assert.Equal(t, "cargo", m["missing_tool"])
}

func TestParseMetadata_Missing(t *testing.T) {
	args := map[string]any{"topic": "foo"}
	m := parseMetadata(args)
	assert.Nil(t, m)
}

func TestParseMetadata_NumericValues(t *testing.T) {
	args := map[string]any{
		"metadata": map[string]any{
			"exit_code": float64(127), // JSON numbers are float64
		},
	}

	m := parseMetadata(args)
	assert.Equal(t, "127", m["exit_code"])
}

func TestStringArg(t *testing.T) {
	args := map[string]any{
		"present": "hello",
		"number":  42,
	}

	assert.Equal(t, "hello", stringArg(args, "present"))
	assert.Equal(t, "", stringArg(args, "missing"))
	assert.Equal(t, "", stringArg(args, "number")) // wrong type
}

func TestErrorResult(t *testing.T) {
	call := agentic.ToolCall{ID: "call-1"}
	result := errorResult(call, "something broke")

	assert.Equal(t, "call-1", result.CallID)
	assert.Equal(t, "something broke", result.Content)
	assert.Equal(t, "something broke", result.Error)
}

func TestJsonResult(t *testing.T) {
	call := agentic.ToolCall{ID: "call-2"}
	data := map[string]string{"status": "ok"}

	result, err := jsonResult(call, data)
	require.NoError(t, err)
	assert.Equal(t, "call-2", result.CallID)

	var parsed map[string]string
	require.NoError(t, json.Unmarshal([]byte(result.Content), &parsed))
	assert.Equal(t, "ok", parsed["status"])
}
