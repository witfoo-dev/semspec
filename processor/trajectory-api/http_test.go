package trajectoryapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExtractIDFromPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		prefix   string
		expected string
	}{
		{
			name:     "basic loop ID extraction",
			path:     "/trajectory-api/loops/loop-123",
			prefix:   "/loops/",
			expected: "loop-123",
		},
		{
			name:     "trace ID extraction",
			path:     "/trajectory-api/traces/trace-456",
			prefix:   "/traces/",
			expected: "trace-456",
		},
		{
			name:     "ID with trailing slash",
			path:     "/trajectory-api/loops/loop-123/",
			prefix:   "/loops/",
			expected: "loop-123",
		},
		{
			name:     "ID with additional segments",
			path:     "/trajectory-api/loops/loop-123/extra/path",
			prefix:   "/loops/",
			expected: "loop-123",
		},
		{
			name:     "empty path",
			path:     "/trajectory-api/loops/",
			prefix:   "/loops/",
			expected: "",
		},
		{
			name:     "prefix not found",
			path:     "/other-api/traces/trace-123",
			prefix:   "/loops/",
			expected: "",
		},
		{
			name:     "UUID format ID",
			path:     "/trajectory-api/loops/550e8400-e29b-41d4-a716-446655440000",
			prefix:   "/loops/",
			expected: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:     "ID with spaces gets trimmed",
			path:     "/trajectory-api/loops/ loop-123 ",
			prefix:   "/loops/",
			expected: "loop-123",
		},
		{
			name:     "trace ID containing dots",
			path:     "/trajectory-api/traces/abc.def.ghi",
			prefix:   "/traces/",
			expected: "abc.def.ghi",
		},
		{
			name:     "loop ID with version-like format",
			path:     "/trajectory-api/loops/loop-v1.2.3",
			prefix:   "/loops/",
			expected: "loop-v1.2.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractIDFromPath(tt.path, tt.prefix)
			if result != tt.expected {
				t.Errorf("extractIDFromPath(%q, %q) = %q, want %q",
					tt.path, tt.prefix, result, tt.expected)
			}
		})
	}
}

func TestBuildTrajectory(t *testing.T) {
	c := &Component{}
	now := time.Now()
	endTime := now.Add(5 * time.Second)

	loopState := &LoopState{
		ID:        "loop-123",
		TraceID:   "trace-456",
		Status:    "completed",
		Iteration: 3,
		StartedAt: &now,
		EndedAt:   &endTime,
	}

	steps := []*StepRecord{
		{
			EntityID:   "semspec.semspec-dev.agent.agentic-loop.step.loop-123-0",
			Type:       "model_call",
			Index:      0,
			Timestamp:  now,
			DurationMs: 1000,
			Model:      "claude-sonnet",
			Provider:   "anthropic",
			Capability: "planning",
			TokensIn:   100,
			TokensOut:  50,
		},
		{
			EntityID:   "semspec.semspec-dev.agent.agentic-loop.step.loop-123-1",
			Type:       "tool_call",
			Index:      1,
			Timestamp:  now.Add(1500 * time.Millisecond),
			DurationMs: 200,
			ToolName:   "file_read",
			Capability: "coding",
		},
		{
			EntityID:   "semspec.semspec-dev.agent.agentic-loop.step.loop-123-2",
			Type:       "model_call",
			Index:      2,
			Timestamp:  now.Add(2 * time.Second),
			DurationMs: 2000,
			Model:      "claude-sonnet",
			Provider:   "anthropic",
			Capability: "coding",
			TokensIn:   200,
			TokensOut:  100,
		},
	}

	trajectory := c.buildTrajectory(loopState, steps, false)

	if trajectory.LoopID != "loop-123" {
		t.Errorf("LoopID = %q, want %q", trajectory.LoopID, "loop-123")
	}
	if trajectory.TraceID != "trace-456" {
		t.Errorf("TraceID = %q, want %q", trajectory.TraceID, "trace-456")
	}
	if trajectory.Status != "completed" {
		t.Errorf("Status = %q, want %q", trajectory.Status, "completed")
	}
	if trajectory.Steps != 3 {
		t.Errorf("Steps = %d, want 3", trajectory.Steps)
	}
	if trajectory.ModelCalls != 2 {
		t.Errorf("ModelCalls = %d, want 2", trajectory.ModelCalls)
	}
	if trajectory.ToolCalls != 1 {
		t.Errorf("ToolCalls = %d, want 1", trajectory.ToolCalls)
	}
	if trajectory.TokensIn != 300 {
		t.Errorf("TokensIn = %d, want 300 (100+200)", trajectory.TokensIn)
	}
	if trajectory.TokensOut != 150 {
		t.Errorf("TokensOut = %d, want 150 (50+100)", trajectory.TokensOut)
	}
	// Duration is calculated from loop state start/end timestamps.
	expectedDuration := endTime.Sub(now).Milliseconds()
	if trajectory.DurationMs != expectedDuration {
		t.Errorf("DurationMs = %d, want %d", trajectory.DurationMs, expectedDuration)
	}
	// Entries should not be included when includeEntries is false.
	if len(trajectory.Entries) != 0 {
		t.Errorf("Entries count = %d, want 0 (includeEntries=false)", len(trajectory.Entries))
	}
}

func TestBuildTrajectory_WithEntries(t *testing.T) {
	c := &Component{}
	now := time.Now()

	loopState := &LoopState{
		ID:      "loop-123",
		TraceID: "trace-456",
		Status:  "running",
	}

	steps := []*StepRecord{
		{
			EntityID:   "semspec.semspec-dev.agent.agentic-loop.step.loop-123-0",
			Type:       "model_call",
			Index:      0,
			Timestamp:  now,
			DurationMs: 1000,
			Model:      "claude-sonnet",
			Provider:   "anthropic",
			Capability: "planning",
			TokensIn:   100,
			TokensOut:  50,
			Retries:    1,
		},
		{
			EntityID:   "semspec.semspec-dev.agent.agentic-loop.step.loop-123-1",
			Type:       "tool_call",
			Index:      1,
			Timestamp:  now.Add(1500 * time.Millisecond),
			DurationMs: 250,
			ToolName:   "graph_query",
			Capability: "planning",
		},
	}

	trajectory := c.buildTrajectory(loopState, steps, true)

	if len(trajectory.Entries) != 2 {
		t.Fatalf("Entries count = %d, want 2", len(trajectory.Entries))
	}

	entry0 := trajectory.Entries[0]
	if entry0.Type != "model_call" {
		t.Errorf("Entry[0].Type = %q, want %q", entry0.Type, "model_call")
	}
	if entry0.Model != "claude-sonnet" {
		t.Errorf("Entry[0].Model = %q, want %q", entry0.Model, "claude-sonnet")
	}
	if entry0.Provider != "anthropic" {
		t.Errorf("Entry[0].Provider = %q, want %q", entry0.Provider, "anthropic")
	}
	if entry0.Capability != "planning" {
		t.Errorf("Entry[0].Capability = %q, want %q", entry0.Capability, "planning")
	}
	if entry0.TokensIn != 100 {
		t.Errorf("Entry[0].TokensIn = %d, want 100", entry0.TokensIn)
	}
	if entry0.TokensOut != 50 {
		t.Errorf("Entry[0].TokensOut = %d, want 50", entry0.TokensOut)
	}
	if entry0.DurationMs != 1000 {
		t.Errorf("Entry[0].DurationMs = %d, want 1000", entry0.DurationMs)
	}
	if entry0.Retries != 1 {
		t.Errorf("Entry[0].Retries = %d, want 1", entry0.Retries)
	}

	entry1 := trajectory.Entries[1]
	if entry1.Type != "tool_call" {
		t.Errorf("Entry[1].Type = %q, want %q", entry1.Type, "tool_call")
	}
	if entry1.ToolName != "graph_query" {
		t.Errorf("Entry[1].ToolName = %q, want %q", entry1.ToolName, "graph_query")
	}
}

func TestBuildTrajectory_EmptySteps(t *testing.T) {
	c := &Component{}
	now := time.Now()

	loopState := &LoopState{
		ID:        "loop-123",
		TraceID:   "trace-456",
		Status:    "completed",
		Iteration: 0,
		StartedAt: &now,
	}

	trajectory := c.buildTrajectory(loopState, nil, true)

	if trajectory.LoopID != "loop-123" {
		t.Errorf("LoopID = %q, want %q", trajectory.LoopID, "loop-123")
	}
	if trajectory.ModelCalls != 0 {
		t.Errorf("ModelCalls = %d, want 0", trajectory.ModelCalls)
	}
	if trajectory.ToolCalls != 0 {
		t.Errorf("ToolCalls = %d, want 0", trajectory.ToolCalls)
	}
	if trajectory.TokensIn != 0 {
		t.Errorf("TokensIn = %d, want 0", trajectory.TokensIn)
	}
	if len(trajectory.Entries) != 0 {
		t.Errorf("Entries count = %d, want 0", len(trajectory.Entries))
	}
}

func TestBuildTrajectory_DurationFromSteps(t *testing.T) {
	c := &Component{}
	now := time.Now()

	// Loop state without endedAt — duration is sum of step durations.
	loopState := &LoopState{
		ID:        "loop-123",
		TraceID:   "trace-456",
		Status:    "running",
		StartedAt: &now,
		EndedAt:   nil,
	}

	steps := []*StepRecord{
		{Type: "model_call", Index: 0, Timestamp: now, DurationMs: 500, TokensIn: 10, TokensOut: 5},
		{Type: "tool_call", Index: 1, Timestamp: now.Add(time.Second), DurationMs: 700},
	}

	trajectory := c.buildTrajectory(loopState, steps, false)

	// Without loop EndedAt, duration is sum of step durations.
	expectedDuration := int64(1200)
	if trajectory.DurationMs != expectedDuration {
		t.Errorf("DurationMs = %d, want %d (sum of step durations)", trajectory.DurationMs, expectedDuration)
	}
}

func TestHandleGetLoopTrajectory_MethodNotAllowed(t *testing.T) {
	c := &Component{}

	req := httptest.NewRequest(http.MethodPost, "/trajectory-api/loops/loop-123", nil)
	w := httptest.NewRecorder()

	c.handleGetLoopTrajectory(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleGetLoopTrajectory_MissingID(t *testing.T) {
	c := &Component{}

	req := httptest.NewRequest(http.MethodGet, "/trajectory-api/loops/", nil)
	w := httptest.NewRecorder()

	c.handleGetLoopTrajectory(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleGetTraceTrajectory_MethodNotAllowed(t *testing.T) {
	c := &Component{}

	req := httptest.NewRequest(http.MethodPost, "/trajectory-api/traces/trace-123", nil)
	w := httptest.NewRecorder()

	c.handleGetTraceTrajectory(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleGetTraceTrajectory_MissingID(t *testing.T) {
	c := &Component{}

	req := httptest.NewRequest(http.MethodGet, "/trajectory-api/traces/", nil)
	w := httptest.NewRecorder()

	c.handleGetTraceTrajectory(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// TestTrajectoryResponseFormat verifies the JSON structure of trajectory responses.
func TestTrajectoryResponseFormat(t *testing.T) {
	c := &Component{}
	now := time.Now()
	endTime := now.Add(2 * time.Second)

	loopState := &LoopState{
		ID:        "loop-test",
		TraceID:   "trace-test",
		Status:    "completed",
		Iteration: 2,
		StartedAt: &now,
		EndedAt:   &endTime,
	}

	steps := []*StepRecord{
		{
			Type:       "model_call",
			Index:      0,
			Timestamp:  now,
			DurationMs: 500,
			Model:      "claude-sonnet",
			Provider:   "anthropic",
			Capability: "planning",
			TokensIn:   50,
			TokensOut:  25,
		},
	}

	trajectory := c.buildTrajectory(loopState, steps, true)

	// Verify JSON marshaling works correctly.
	data, err := json.Marshal(trajectory)
	if err != nil {
		t.Fatalf("Failed to marshal trajectory: %v", err)
	}

	var unmarshaled Trajectory
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal trajectory: %v", err)
	}

	if unmarshaled.LoopID != trajectory.LoopID {
		t.Errorf("Unmarshaled LoopID = %q, want %q", unmarshaled.LoopID, trajectory.LoopID)
	}
	if unmarshaled.TraceID != trajectory.TraceID {
		t.Errorf("Unmarshaled TraceID = %q, want %q", unmarshaled.TraceID, trajectory.TraceID)
	}
	if unmarshaled.Status != trajectory.Status {
		t.Errorf("Unmarshaled Status = %q, want %q", unmarshaled.Status, trajectory.Status)
	}
	if unmarshaled.Steps != trajectory.Steps {
		t.Errorf("Unmarshaled Steps = %d, want %d", unmarshaled.Steps, trajectory.Steps)
	}
	if unmarshaled.ModelCalls != trajectory.ModelCalls {
		t.Errorf("Unmarshaled ModelCalls = %d, want %d", unmarshaled.ModelCalls, trajectory.ModelCalls)
	}
	if unmarshaled.TokensIn != trajectory.TokensIn {
		t.Errorf("Unmarshaled TokensIn = %d, want %d", unmarshaled.TokensIn, trajectory.TokensIn)
	}
	if unmarshaled.TokensOut != trajectory.TokensOut {
		t.Errorf("Unmarshaled TokensOut = %d, want %d", unmarshaled.TokensOut, trajectory.TokensOut)
	}
	if len(unmarshaled.Entries) != len(trajectory.Entries) {
		t.Errorf("Unmarshaled Entries count = %d, want %d", len(unmarshaled.Entries), len(trajectory.Entries))
	}
}

// TestRegisterHTTPHandlers verifies handler registration.
func TestRegisterHTTPHandlers(t *testing.T) {
	c := &Component{}
	mux := http.NewServeMux()

	// This should not panic.
	c.RegisterHTTPHandlers("/trajectory-api/", mux)

	tests := []struct {
		name         string
		path         string
		method       string
		expectedCode int
	}{
		{
			name:         "loops handler registered - missing ID",
			path:         "/trajectory-api/loops/",
			method:       http.MethodGet,
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "traces handler registered - missing ID",
			path:         "/trajectory-api/traces/",
			method:       http.MethodGet,
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "loops handler wrong method",
			path:         "/trajectory-api/loops/test-id",
			method:       http.MethodPost,
			expectedCode: http.StatusMethodNotAllowed,
		},
		{
			name:         "traces handler wrong method",
			path:         "/trajectory-api/traces/test-id",
			method:       http.MethodPost,
			expectedCode: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Status = %d, want %d (body: %s)", w.Code, tt.expectedCode, w.Body.String())
			}
		})
	}
}

// TestLoopStateJSONSerialization verifies LoopState JSON marshaling.
func TestLoopStateJSONSerialization(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	endTime := now.Add(5 * time.Second)

	state := &LoopState{
		ID:        "loop-123",
		TraceID:   "trace-456",
		Status:    "completed",
		Role:      "planner",
		Model:     "claude-sonnet",
		Iteration: 5,
		StartedAt: &now,
		EndedAt:   &endTime,
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("Failed to marshal LoopState: %v", err)
	}

	var unmarshaled LoopState
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal LoopState: %v", err)
	}

	if unmarshaled.ID != state.ID {
		t.Errorf("ID = %q, want %q", unmarshaled.ID, state.ID)
	}
	if unmarshaled.TraceID != state.TraceID {
		t.Errorf("TraceID = %q, want %q", unmarshaled.TraceID, state.TraceID)
	}
	if unmarshaled.Status != state.Status {
		t.Errorf("Status = %q, want %q", unmarshaled.Status, state.Status)
	}
	if unmarshaled.Role != state.Role {
		t.Errorf("Role = %q, want %q", unmarshaled.Role, state.Role)
	}
	if unmarshaled.Model != state.Model {
		t.Errorf("Model = %q, want %q", unmarshaled.Model, state.Model)
	}
	if unmarshaled.Iteration != state.Iteration {
		t.Errorf("Iteration = %d, want %d", unmarshaled.Iteration, state.Iteration)
	}
}

// TestTrajectoryEntryJSONSerialization verifies TrajectoryEntry JSON marshaling.
func TestTrajectoryEntryJSONSerialization(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	entry := TrajectoryEntry{
		Type:       "model_call",
		Timestamp:  now,
		DurationMs: 1500,
		Model:      "claude-sonnet",
		Provider:   "anthropic",
		Capability: "coding",
		TokensIn:   500,
		TokensOut:  250,
		Retries:    2,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal TrajectoryEntry: %v", err)
	}

	var unmarshaled TrajectoryEntry
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal TrajectoryEntry: %v", err)
	}

	if unmarshaled.Type != entry.Type {
		t.Errorf("Type = %q, want %q", unmarshaled.Type, entry.Type)
	}
	if unmarshaled.Model != entry.Model {
		t.Errorf("Model = %q, want %q", unmarshaled.Model, entry.Model)
	}
	if unmarshaled.Provider != entry.Provider {
		t.Errorf("Provider = %q, want %q", unmarshaled.Provider, entry.Provider)
	}
	if unmarshaled.Capability != entry.Capability {
		t.Errorf("Capability = %q, want %q", unmarshaled.Capability, entry.Capability)
	}
	if unmarshaled.DurationMs != entry.DurationMs {
		t.Errorf("DurationMs = %d, want %d", unmarshaled.DurationMs, entry.DurationMs)
	}
	if unmarshaled.TokensIn != entry.TokensIn {
		t.Errorf("TokensIn = %d, want %d", unmarshaled.TokensIn, entry.TokensIn)
	}
	if unmarshaled.TokensOut != entry.TokensOut {
		t.Errorf("TokensOut = %d, want %d", unmarshaled.TokensOut, entry.TokensOut)
	}
	if unmarshaled.Retries != entry.Retries {
		t.Errorf("Retries = %d, want %d", unmarshaled.Retries, entry.Retries)
	}
}

// TestHandleGetWorkflowTrajectory tests the workflow trajectory endpoint.
func TestHandleGetWorkflowTrajectory(t *testing.T) {
	c := &Component{}

	tests := []struct {
		name         string
		method       string
		url          string
		expectedCode int
	}{
		{
			name:         "missing slug",
			method:       http.MethodGet,
			url:          "/trajectory-api/workflows/",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "wrong method - POST",
			method:       http.MethodPost,
			url:          "/trajectory-api/workflows/test-plan",
			expectedCode: http.StatusMethodNotAllowed,
		},
		{
			name:         "valid request without manager returns 503",
			method:       http.MethodGet,
			url:          "/trajectory-api/workflows/test-plan",
			expectedCode: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.url, nil)
			w := httptest.NewRecorder()

			c.handleGetWorkflowTrajectory(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Status = %d, want %d", w.Code, tt.expectedCode)
			}
		})
	}
}

// TestHandleGetContextStats tests the context utilization stats endpoint.
func TestHandleGetContextStats(t *testing.T) {
	c := &Component{}

	tests := []struct {
		name         string
		method       string
		url          string
		expectedCode int
	}{
		{
			name:         "missing parameters",
			method:       http.MethodGet,
			url:          "/trajectory-api/context-stats",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "wrong method",
			method:       http.MethodPost,
			url:          "/trajectory-api/context-stats?trace_id=test",
			expectedCode: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.url, nil)
			w := httptest.NewRecorder()

			c.handleGetContextStats(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Status = %d, want %d", w.Code, tt.expectedCode)
			}
		})
	}
}

// TestBuildWorkflowTrajectory tests workflow-level aggregation logic.
func TestBuildWorkflowTrajectory(t *testing.T) {
	c := &Component{}
	now := time.Now()

	steps := []*StepRecord{
		// Planning phase
		{
			Type:       "model_call",
			Index:      0,
			Timestamp:  now,
			DurationMs: 10000,
			Capability: "planning",
			TokensIn:   15000,
			TokensOut:  3000,
		},
		// Review phase
		{
			Type:       "model_call",
			Index:      1,
			Timestamp:  now.Add(1 * time.Minute),
			DurationMs: 15000,
			Capability: "reviewing",
			TokensIn:   28000,
			TokensOut:  5000,
		},
		// Execution - coding
		{
			Type:       "model_call",
			Index:      2,
			Timestamp:  now.Add(2 * time.Minute),
			DurationMs: 20000,
			Capability: "coding",
			TokensIn:   64000,
			TokensOut:  18000,
		},
		// Tool call (not counted in model metrics)
		{
			Type:       "tool_call",
			Index:      3,
			Timestamp:  now.Add(3 * time.Minute),
			DurationMs: 200,
			ToolName:   "file_read",
		},
	}

	slug := "test-workflow"
	traceIDs := []string{"trace-1", "trace-2"}

	wt := c.buildWorkflowTrajectory(slug, "approved", traceIDs, steps, &now, nil)

	if wt.Slug != slug {
		t.Errorf("Slug = %q, want %q", wt.Slug, slug)
	}
	if wt.Status != "approved" {
		t.Errorf("Status = %q, want %q", wt.Status, "approved")
	}
	if len(wt.TraceIDs) != 2 {
		t.Errorf("TraceIDs count = %d, want 2", len(wt.TraceIDs))
	}

	if wt.Totals == nil {
		t.Fatal("Totals is nil")
	}

	// Only model_call steps contribute to token counts (3 model calls, 1 tool call).
	expectedTokensIn := 15000 + 28000 + 64000
	if wt.Totals.TokensIn != expectedTokensIn {
		t.Errorf("Totals.TokensIn = %d, want %d", wt.Totals.TokensIn, expectedTokensIn)
	}

	expectedTokensOut := 3000 + 5000 + 18000
	if wt.Totals.TokensOut != expectedTokensOut {
		t.Errorf("Totals.TokensOut = %d, want %d", wt.Totals.TokensOut, expectedTokensOut)
	}

	if wt.Totals.CallCount != 3 {
		t.Errorf("Totals.CallCount = %d, want 3 (model calls only)", wt.Totals.CallCount)
	}
}

// TestGetWorkflowTrajectory_IncludesAllPhases verifies phase mapping.
func TestGetWorkflowTrajectory_IncludesAllPhases(t *testing.T) {
	c := &Component{}
	now := time.Now()

	steps := []*StepRecord{
		{Type: "model_call", Index: 0, Timestamp: now, Capability: "planning", TokensIn: 10000, TokensOut: 2000},
		{Type: "model_call", Index: 1, Timestamp: now.Add(1 * time.Minute), Capability: "reviewing", TokensIn: 15000, TokensOut: 3000},
		{Type: "model_call", Index: 2, Timestamp: now.Add(2 * time.Minute), Capability: "reviewing", TokensIn: 12000, TokensOut: 2500},
		{Type: "model_call", Index: 3, Timestamp: now.Add(3 * time.Minute), Capability: "coding", TokensIn: 20000, TokensOut: 5000},
		{Type: "model_call", Index: 4, Timestamp: now.Add(4 * time.Minute), Capability: "writing", TokensIn: 8000, TokensOut: 1500},
	}

	wt := c.buildWorkflowTrajectory("test-workflow", "approved", []string{"workflow-trace"}, steps, &now, nil)

	// Verify all three phases are present.
	expectedPhases := []string{"planning", "review", "execution"}
	for _, phase := range expectedPhases {
		if wt.Phases[phase] == nil {
			t.Errorf("Phase %q not found in Phases map", phase)
		}
	}

	// Verify planning phase metrics.
	planningPhase := wt.Phases["planning"]
	if planningPhase == nil {
		t.Fatal("Planning phase is nil")
	}
	if planningPhase.CallCount != 1 {
		t.Errorf("planning.CallCount = %d, want 1", planningPhase.CallCount)
	}
	if planningPhase.TokensIn != 10000 {
		t.Errorf("planning.TokensIn = %d, want 10000", planningPhase.TokensIn)
	}

	// Verify review phase metrics (2 calls).
	reviewPhase := wt.Phases["review"]
	if reviewPhase == nil {
		t.Fatal("Review phase is nil")
	}
	if reviewPhase.CallCount != 2 {
		t.Errorf("review.CallCount = %d, want 2", reviewPhase.CallCount)
	}
	if reviewPhase.TokensIn != 27000 {
		t.Errorf("review.TokensIn = %d, want 27000", reviewPhase.TokensIn)
	}

	// Verify execution phase metrics (coding + writing = 2 calls).
	executionPhase := wt.Phases["execution"]
	if executionPhase == nil {
		t.Fatal("Execution phase is nil")
	}
	if executionPhase.CallCount != 2 {
		t.Errorf("execution.CallCount = %d, want 2", executionPhase.CallCount)
	}
	if executionPhase.TokensIn != 28000 {
		t.Errorf("execution.TokensIn = %d, want 28000", executionPhase.TokensIn)
	}
	if executionPhase.Capabilities["coding"] == nil {
		t.Error("execution.Capabilities[coding] is nil")
	}
	if executionPhase.Capabilities["writing"] == nil {
		t.Error("execution.Capabilities[writing] is nil")
	}

	if wt.Totals.CallCount != 5 {
		t.Errorf("Totals.CallCount = %d, want 5", wt.Totals.CallCount)
	}
}

// TestBuildContextStats tests context utilization calculation from step records.
func TestBuildContextStats(t *testing.T) {
	c := &Component{}

	steps := []*StepRecord{
		{Type: "model_call", Capability: "planning", TokensIn: 45000, TokensOut: 5000},
		{Type: "model_call", Capability: "coding", TokensIn: 64000, TokensOut: 8000},
		{Type: "model_call", Capability: "writing", TokensIn: 18000, TokensOut: 2000},
		// Tool calls should not be counted.
		{Type: "tool_call", ToolName: "file_read"},
	}

	stats := c.buildContextStats(steps)

	if stats.Summary == nil {
		t.Fatal("Summary is nil")
	}
	// Only model_call steps count.
	if stats.Summary.TotalCalls != 3 {
		t.Errorf("TotalCalls = %d, want 3", stats.Summary.TotalCalls)
	}

	if stats.ByCapability == nil {
		t.Fatal("ByCapability is nil")
	}
	if len(stats.ByCapability) != 3 {
		t.Errorf("ByCapability count = %d, want 3", len(stats.ByCapability))
	}

	planningStats, ok := stats.ByCapability["planning"]
	if !ok {
		t.Fatal("planning capability not found")
	}
	if planningStats.CallCount != 1 {
		t.Errorf("planning CallCount = %d, want 1", planningStats.CallCount)
	}
}

// TestLoopEntityID verifies the entity ID construction.
func TestLoopEntityID(t *testing.T) {
	c := &Component{
		config: Config{
			Org:      "semspec",
			Platform: "semspec-dev",
		},
	}

	got := c.loopEntityID("abc123")
	want := "semspec.semspec-dev.agent.agentic-loop.execution.abc123"
	if got != want {
		t.Errorf("loopEntityID() = %q, want %q", got, want)
	}
}

// TestLoopEntityID_MissingOrg verifies graceful degradation when org/platform not set.
func TestLoopEntityID_MissingOrg(t *testing.T) {
	c := &Component{
		config: Config{}, // no org or platform
	}

	got := c.loopEntityID("abc123")
	if got != "" {
		t.Errorf("loopEntityID() = %q, want empty when org/platform not configured", got)
	}
}
