package workflow

import (
	"encoding/json"
	"testing"
)

func TestNewQuestion(t *testing.T) {
	q := NewQuestion("my-agent", "api.users", "How do I create a user?", "Building user creation endpoint")

	// Verify ID format
	if len(q.ID) < 3 || q.ID[:2] != "q-" {
		t.Errorf("ID should start with 'q-', got %q", q.ID)
	}

	if q.FromAgent != "my-agent" {
		t.Errorf("FromAgent = %q, want %q", q.FromAgent, "my-agent")
	}
	if q.Topic != "api.users" {
		t.Errorf("Topic = %q, want %q", q.Topic, "api.users")
	}
	if q.Question != "How do I create a user?" {
		t.Errorf("Question = %q, want %q", q.Question, "How do I create a user?")
	}
	if q.Context != "Building user creation endpoint" {
		t.Errorf("Context = %q, want %q", q.Context, "Building user creation endpoint")
	}
	if q.Status != QuestionStatusPending {
		t.Errorf("Status = %q, want %q", q.Status, QuestionStatusPending)
	}
	if q.Urgency != QuestionUrgencyNormal {
		t.Errorf("Urgency = %q, want %q", q.Urgency, QuestionUrgencyNormal)
	}
	if q.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestAnswerPayload_Validation(t *testing.T) {
	tests := []struct {
		name    string
		payload AnswerPayload
		wantErr bool
	}{
		{
			name: "valid payload",
			payload: AnswerPayload{
				QuestionID: "q-123",
				Answer:     "The answer is 42",
			},
			wantErr: false,
		},
		{
			name: "missing question_id",
			payload: AnswerPayload{
				QuestionID: "",
				Answer:     "The answer",
			},
			wantErr: true,
		},
		{
			name: "missing answer",
			payload: AnswerPayload{
				QuestionID: "q-123",
				Answer:     "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.payload.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewQuestion_DefaultsCategory(t *testing.T) {
	q := NewQuestion("agent", "api.test", "question?", "ctx")
	if q.Category != QuestionCategoryKnowledge {
		t.Errorf("Category = %q, want %q", q.Category, QuestionCategoryKnowledge)
	}
}

func TestNewCategorizedQuestion(t *testing.T) {
	meta := map[string]string{
		"command":      "cargo build",
		"exit_code":    "127",
		"missing_tool": "cargo",
	}
	q := NewCategorizedQuestion("developer", "environment.sandbox.missing-tool",
		"cargo is not installed", "tried to build rust project",
		QuestionCategoryEnvironment, meta)

	if q.Category != QuestionCategoryEnvironment {
		t.Errorf("Category = %q, want %q", q.Category, QuestionCategoryEnvironment)
	}
	if q.Metadata["command"] != "cargo build" {
		t.Errorf("Metadata[command] = %q, want %q", q.Metadata["command"], "cargo build")
	}
	if q.Metadata["exit_code"] != "127" {
		t.Errorf("Metadata[exit_code] = %q, want %q", q.Metadata["exit_code"], "127")
	}
	if q.Metadata["missing_tool"] != "cargo" {
		t.Errorf("Metadata[missing_tool] = %q, want %q", q.Metadata["missing_tool"], "cargo")
	}
	// Verify it inherits defaults from NewQuestion
	if q.Status != QuestionStatusPending {
		t.Errorf("Status = %q, want %q", q.Status, QuestionStatusPending)
	}
	if q.Urgency != QuestionUrgencyNormal {
		t.Errorf("Urgency = %q, want %q", q.Urgency, QuestionUrgencyNormal)
	}
}

func TestAnswerAction_JSONRoundTrip(t *testing.T) {
	action := &AnswerAction{
		Type:       "install_package",
		Parameters: map[string]string{"packages": "cargo,rustfmt"},
	}

	data, err := json.Marshal(action)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded AnswerAction
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Type != "install_package" {
		t.Errorf("Type = %q, want %q", decoded.Type, "install_package")
	}
	if decoded.Parameters["packages"] != "cargo,rustfmt" {
		t.Errorf("Parameters[packages] = %q, want %q", decoded.Parameters["packages"], "cargo,rustfmt")
	}
}

func TestQuestion_ActionInJSON(t *testing.T) {
	q := NewCategorizedQuestion("agent", "env.sandbox", "need cargo", "",
		QuestionCategoryEnvironment, nil)
	q.Action = &AnswerAction{
		Type:       "install_package",
		Parameters: map[string]string{"packages": "cargo"},
	}

	data, err := json.Marshal(q)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Question
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Action == nil {
		t.Fatal("Action should not be nil after round-trip")
	}
	if decoded.Action.Type != "install_package" {
		t.Errorf("Action.Type = %q, want %q", decoded.Action.Type, "install_package")
	}
}

func TestQuestion_BackwardCompatible_NoCategory(t *testing.T) {
	// Simulate deserializing an old question without category/metadata/action
	old := `{"id":"q-abc","from_agent":"planner","topic":"api.test","question":"what?","urgency":"normal","status":"pending","created_at":"2026-01-01T00:00:00Z"}`

	var q Question
	if err := json.Unmarshal([]byte(old), &q); err != nil {
		t.Fatalf("Unmarshal old question: %v", err)
	}

	if q.Category != "" {
		t.Errorf("Category should be empty for old questions, got %q", q.Category)
	}
	if q.Metadata != nil {
		t.Errorf("Metadata should be nil for old questions, got %v", q.Metadata)
	}
	if q.Action != nil {
		t.Errorf("Action should be nil for old questions")
	}
}

func TestAnswerPayload_Schema(t *testing.T) {
	p := &AnswerPayload{}
	schema := p.Schema()

	if schema.Domain != "question" {
		t.Errorf("Schema().Domain = %q, want %q", schema.Domain, "question")
	}
	if schema.Category != "answer" {
		t.Errorf("Schema().Category = %q, want %q", schema.Category, "answer")
	}
	if schema.Version != "v1" {
		t.Errorf("Schema().Version = %q, want %q", schema.Version, "v1")
	}
}
