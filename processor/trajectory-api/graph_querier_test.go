package trajectoryapi

import (
	"testing"
	"time"

	agvocab "github.com/c360studio/semstreams/vocabulary/agentic"
)

func TestEntityToStepRecord_ModelCall(t *testing.T) {
	timestamp := time.Date(2026, 3, 17, 10, 30, 0, 0, time.UTC)

	entity := graphEntity{
		ID: "semspec.semspec-dev.agent.agentic-loop.step.loop-456-0",
		Triples: []graphTriple{
			{Predicate: agvocab.StepType, Object: "model_call"},
			{Predicate: agvocab.StepIndex, Object: float64(0)},
			{Predicate: agvocab.StepLoop, Object: "semspec.semspec-dev.agent.agentic-loop.execution.loop-456"},
			{Predicate: agvocab.StepTimestamp, Object: timestamp.Format(time.RFC3339)},
			{Predicate: agvocab.StepDuration, Object: float64(5000)},
			{Predicate: agvocab.StepModel, Object: "claude-sonnet"},
			{Predicate: agvocab.StepTokensIn, Object: float64(1000)},
			{Predicate: agvocab.StepTokensOut, Object: float64(500)},
			{Predicate: agvocab.StepCapability, Object: "planning"},
			{Predicate: agvocab.StepProvider, Object: "anthropic"},
			{Predicate: agvocab.StepRetries, Object: float64(1)},
		},
	}

	record := entityToStepRecord(entity)

	if record.EntityID != entity.ID {
		t.Errorf("EntityID = %q, want %q", record.EntityID, entity.ID)
	}
	if record.Type != "model_call" {
		t.Errorf("Type = %q, want %q", record.Type, "model_call")
	}
	if record.Index != 0 {
		t.Errorf("Index = %d, want 0", record.Index)
	}
	if record.LoopEntityID != "semspec.semspec-dev.agent.agentic-loop.execution.loop-456" {
		t.Errorf("LoopEntityID = %q", record.LoopEntityID)
	}
	if !record.Timestamp.Equal(timestamp) {
		t.Errorf("Timestamp = %v, want %v", record.Timestamp, timestamp)
	}
	if record.DurationMs != 5000 {
		t.Errorf("DurationMs = %d, want 5000", record.DurationMs)
	}
	if record.Model != "claude-sonnet" {
		t.Errorf("Model = %q, want %q", record.Model, "claude-sonnet")
	}
	if record.TokensIn != 1000 {
		t.Errorf("TokensIn = %d, want 1000", record.TokensIn)
	}
	if record.TokensOut != 500 {
		t.Errorf("TokensOut = %d, want 500", record.TokensOut)
	}
	if record.Capability != "planning" {
		t.Errorf("Capability = %q, want %q", record.Capability, "planning")
	}
	if record.Provider != "anthropic" {
		t.Errorf("Provider = %q, want %q", record.Provider, "anthropic")
	}
	if record.Retries != 1 {
		t.Errorf("Retries = %d, want 1", record.Retries)
	}
}

func TestEntityToStepRecord_ToolCall(t *testing.T) {
	timestamp := time.Date(2026, 3, 17, 10, 30, 5, 0, time.UTC)

	entity := graphEntity{
		ID: "semspec.semspec-dev.agent.agentic-loop.step.loop-456-1",
		Triples: []graphTriple{
			{Predicate: agvocab.StepType, Object: "tool_call"},
			{Predicate: agvocab.StepIndex, Object: float64(1)},
			{Predicate: agvocab.StepLoop, Object: "semspec.semspec-dev.agent.agentic-loop.execution.loop-456"},
			{Predicate: agvocab.StepTimestamp, Object: timestamp.Format(time.RFC3339)},
			{Predicate: agvocab.StepDuration, Object: float64(250)},
			{Predicate: agvocab.StepToolName, Object: "file_read"},
			{Predicate: agvocab.StepCapability, Object: "coding"},
		},
	}

	record := entityToStepRecord(entity)

	if record.Type != "tool_call" {
		t.Errorf("Type = %q, want %q", record.Type, "tool_call")
	}
	if record.Index != 1 {
		t.Errorf("Index = %d, want 1", record.Index)
	}
	if record.ToolName != "file_read" {
		t.Errorf("ToolName = %q, want %q", record.ToolName, "file_read")
	}
	if record.DurationMs != 250 {
		t.Errorf("DurationMs = %d, want 250", record.DurationMs)
	}
	if record.Capability != "coding" {
		t.Errorf("Capability = %q, want %q", record.Capability, "coding")
	}
	// Tool-call-only fields should be zero.
	if record.Model != "" {
		t.Errorf("Model = %q, want empty (tool_call has no model)", record.Model)
	}
	if record.TokensIn != 0 {
		t.Errorf("TokensIn = %d, want 0 (tool_call has no tokens)", record.TokensIn)
	}
}

func TestEntityToStepRecord_MissingTimestamp(t *testing.T) {
	entity := graphEntity{
		ID: "semspec.semspec-dev.agent.agentic-loop.step.loop-456-0",
		Triples: []graphTriple{
			{Predicate: agvocab.StepType, Object: "model_call"},
			{Predicate: agvocab.StepIndex, Object: float64(0)},
		},
	}

	record := entityToStepRecord(entity)

	if !record.Timestamp.IsZero() {
		t.Errorf("Timestamp should be zero when missing, got %v", record.Timestamp)
	}
}

func TestSortStepsByIndex(t *testing.T) {
	now := time.Now()
	steps := []*StepRecord{
		{Index: 3, Timestamp: now.Add(3 * time.Second)},
		{Index: 1, Timestamp: now.Add(1 * time.Second)},
		{Index: 0, Timestamp: now},
		{Index: 2, Timestamp: now.Add(2 * time.Second)},
	}

	sortStepsByIndex(steps)

	for i, step := range steps {
		if step.Index != i {
			t.Errorf("steps[%d].Index = %d, want %d (not sorted correctly)", i, step.Index, i)
		}
	}
}

func TestParseEntitiesFromData(t *testing.T) {
	data := map[string]any{
		"entitiesByPredicate": []any{
			map[string]any{
				"id": "semspec.semspec-dev.agent.agentic-loop.step.loop-1-0",
				"triples": []any{
					map[string]any{
						"predicate": agvocab.StepType,
						"object":    "model_call",
					},
					map[string]any{
						"predicate": agvocab.StepLoop,
						"object":    "semspec.semspec-dev.agent.agentic-loop.execution.loop-1",
					},
				},
			},
			map[string]any{
				"id": "semspec.semspec-dev.agent.agentic-loop.step.loop-1-1",
				"triples": []any{
					map[string]any{
						"predicate": agvocab.StepType,
						"object":    "tool_call",
					},
				},
			},
		},
	}

	entities := parseEntitiesFromData(data, "entitiesByPredicate")
	if len(entities) != 2 {
		t.Fatalf("parseEntitiesFromData() returned %d entities, want 2", len(entities))
	}
	if entities[0].ID != "semspec.semspec-dev.agent.agentic-loop.step.loop-1-0" {
		t.Errorf("entities[0].ID = %q", entities[0].ID)
	}
	if entities[1].ID != "semspec.semspec-dev.agent.agentic-loop.step.loop-1-1" {
		t.Errorf("entities[1].ID = %q", entities[1].ID)
	}

	// Non-existent key returns nil.
	nilEntities := parseEntitiesFromData(data, "nonexistent")
	if nilEntities != nil {
		t.Errorf("parseEntitiesFromData(nonexistent) = %v, want nil", nilEntities)
	}
}

func TestParseGraphEntity(t *testing.T) {
	entityMap := map[string]any{
		"id": "semspec.semspec-dev.agent.agentic-loop.step.loop-1-0",
		"triples": []any{
			map[string]any{
				"predicate": agvocab.StepType,
				"object":    "model_call",
			},
			map[string]any{
				"predicate": agvocab.StepTokensIn,
				"object":    float64(4096),
			},
		},
	}

	entity := parseGraphEntity(entityMap)

	if entity.ID != "semspec.semspec-dev.agent.agentic-loop.step.loop-1-0" {
		t.Errorf("ID = %q", entity.ID)
	}
	if len(entity.Triples) != 2 {
		t.Fatalf("len(Triples) = %d, want 2", len(entity.Triples))
	}
	if entity.Triples[0].Predicate != agvocab.StepType {
		t.Errorf("Triples[0].Predicate = %q, want %q", entity.Triples[0].Predicate, agvocab.StepType)
	}
	if entity.Triples[0].Object != "model_call" {
		t.Errorf("Triples[0].Object = %v, want model_call", entity.Triples[0].Object)
	}
}

func TestExtractRelationshipObjects(t *testing.T) {
	loopEntityID := "semspec.semspec-dev.agent.agentic-loop.execution.loop-1"
	step0EntityID := "semspec.semspec-dev.agent.agentic-loop.step.loop-1-0"
	step1EntityID := "semspec.semspec-dev.agent.agentic-loop.step.loop-1-1"

	data := map[string]any{
		"relationships": []any{
			map[string]any{
				"predicate": agvocab.LoopHasStep,
				"object":    step0EntityID,
			},
			map[string]any{
				"predicate": agvocab.LoopHasStep,
				"object":    step1EntityID,
			},
			map[string]any{
				"predicate": "agent.loop.outcome",
				"object":    "success",
			},
		},
	}

	_ = loopEntityID // entityId is passed as query variable, not in response

	objects := extractRelationshipObjects(data, agvocab.LoopHasStep)
	if len(objects) != 2 {
		t.Fatalf("extractRelationshipObjects() returned %d objects, want 2", len(objects))
	}
	if objects[0] != step0EntityID {
		t.Errorf("objects[0] = %q, want %q", objects[0], step0EntityID)
	}
	if objects[1] != step1EntityID {
		t.Errorf("objects[1] = %q, want %q", objects[1], step1EntityID)
	}

	// Non-matching predicate returns nil.
	objects = extractRelationshipObjects(data, "agent.loop.parent")
	if len(objects) != 0 {
		t.Errorf("extractRelationshipObjects(non-matching) = %v, want empty", objects)
	}
}

func TestGetString(t *testing.T) {
	predicates := map[string]any{
		"string_val": "hello",
		"float_val":  float64(123.45),
		"int_val":    42,
	}

	if got := getString(predicates, "string_val"); got != "hello" {
		t.Errorf("getString(string_val) = %q, want %q", got, "hello")
	}
	if got := getString(predicates, "float_val"); got != "123.45" {
		t.Errorf("getString(float_val) = %q, want %q", got, "123.45")
	}
	if got := getString(predicates, "nonexistent"); got != "" {
		t.Errorf("getString(nonexistent) = %q, want empty", got)
	}
}

func TestGetInt(t *testing.T) {
	predicates := map[string]any{
		"float_val":  float64(42),
		"int_val":    100,
		"string_val": "200",
	}

	if got := getInt(predicates, "float_val"); got != 42 {
		t.Errorf("getInt(float_val) = %d, want 42", got)
	}
	if got := getInt(predicates, "int_val"); got != 100 {
		t.Errorf("getInt(int_val) = %d, want 100", got)
	}
	if got := getInt(predicates, "string_val"); got != 200 {
		t.Errorf("getInt(string_val) = %d, want 200", got)
	}
	if got := getInt(predicates, "nonexistent"); got != 0 {
		t.Errorf("getInt(nonexistent) = %d, want 0", got)
	}
}

func TestGetInt64(t *testing.T) {
	predicates := map[string]any{
		"float_val":  float64(9999),
		"int64_val":  int64(123456789),
		"string_val": "42",
	}

	if got := getInt64(predicates, "float_val"); got != 9999 {
		t.Errorf("getInt64(float_val) = %d, want 9999", got)
	}
	if got := getInt64(predicates, "int64_val"); got != 123456789 {
		t.Errorf("getInt64(int64_val) = %d, want 123456789", got)
	}
	if got := getInt64(predicates, "string_val"); got != 42 {
		t.Errorf("getInt64(string_val) = %d, want 42", got)
	}
	if got := getInt64(predicates, "nonexistent"); got != 0 {
		t.Errorf("getInt64(nonexistent) = %d, want 0", got)
	}
}

func TestSanitizeGraphQLString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "normal string unchanged",
			input: "loop-123-abc",
			want:  "loop-123-abc",
		},
		{
			name:  "null byte removed",
			input: "loop\x00id",
			want:  "loopid",
		},
		{
			name:  "backslash escaped",
			input: "loop\\id",
			want:  "loop\\\\id",
		},
		{
			name:  "both null and backslash",
			input: "loop\x00\\id",
			want:  "loop\\\\id",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "entity id format unchanged",
			input: "semspec.semspec-dev.agent.agentic-loop.execution.abc123",
			want:  "semspec.semspec-dev.agent.agentic-loop.execution.abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeGraphQLString(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeGraphQLString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetTripleValue(t *testing.T) {
	entity := graphEntity{
		Triples: []graphTriple{
			{Predicate: agvocab.StepType, Object: "model_call"},
			{Predicate: agvocab.StepTokensIn, Object: float64(4096)},
		},
	}

	if got := getTripleValue(entity, agvocab.StepType); got != "model_call" {
		t.Errorf("getTripleValue(StepType) = %q, want %q", got, "model_call")
	}
	// Non-string values return empty.
	if got := getTripleValue(entity, agvocab.StepTokensIn); got != "" {
		t.Errorf("getTripleValue(StepTokensIn) = %q, want empty (not a string)", got)
	}
	if got := getTripleValue(entity, "nonexistent"); got != "" {
		t.Errorf("getTripleValue(nonexistent) = %q, want empty", got)
	}
}
