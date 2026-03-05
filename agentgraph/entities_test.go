package agentgraph_test

import (
	"strings"
	"testing"

	"github.com/c360studio/semspec/agentgraph"
)

func TestLoopEntityID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		loopID string
		want   string
	}{
		{
			name:   "simple alphanumeric loop ID",
			loopID: "abc123",
			want:   "semspec.local.agentic.orchestrator.loop.abc123",
		},
		{
			name:   "uuid-style loop ID",
			loopID: "550e8400-e29b-41d4-a716-446655440000",
			want:   "semspec.local.agentic.orchestrator.loop.550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:   "single character loop ID",
			loopID: "1",
			want:   "semspec.local.agentic.orchestrator.loop.1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := agentgraph.LoopEntityID(tc.loopID)
			if got != tc.want {
				t.Errorf("LoopEntityID(%q) = %q, want %q", tc.loopID, got, tc.want)
			}
		})
	}
}

func TestLoopEntityID_SixParts(t *testing.T) {
	t.Parallel()

	got := agentgraph.LoopEntityID("myloop")
	parts := strings.Split(got, ".")
	if len(parts) != 6 {
		t.Errorf("LoopEntityID produced %d parts, want 6: %q", len(parts), got)
	}
}

func TestLoopEntityID_DifferentIDsProduceDifferentEntityIDs(t *testing.T) {
	t.Parallel()

	ids := []string{"loop-1", "loop-2", "loop-3", "alpha", "beta"}
	seen := make(map[string]string)
	for _, loopID := range ids {
		eid := agentgraph.LoopEntityID(loopID)
		if prev, conflict := seen[eid]; conflict {
			t.Errorf("LoopEntityID collision: loopIDs %q and %q both produced %q", prev, loopID, eid)
		}
		seen[eid] = loopID
	}
}

func TestLoopEntityID_PanicsOnEmpty(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Error("LoopEntityID(\"\") should panic but did not")
		}
	}()
	agentgraph.LoopEntityID("")
}

func TestLoopEntityID_PanicsOnDot(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Error("LoopEntityID(\"with.dot\") should panic but did not")
		}
	}()
	agentgraph.LoopEntityID("with.dot")
}

func TestTaskEntityID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		taskID string
		want   string
	}{
		{
			name:   "simple task ID",
			taskID: "task-001",
			want:   "semspec.local.agentic.orchestrator.task.task-001",
		},
		{
			name:   "numeric task ID",
			taskID: "42",
			want:   "semspec.local.agentic.orchestrator.task.42",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := agentgraph.TaskEntityID(tc.taskID)
			if got != tc.want {
				t.Errorf("TaskEntityID(%q) = %q, want %q", tc.taskID, got, tc.want)
			}
		})
	}
}

func TestTaskEntityID_SixParts(t *testing.T) {
	t.Parallel()

	got := agentgraph.TaskEntityID("mytask")
	parts := strings.Split(got, ".")
	if len(parts) != 6 {
		t.Errorf("TaskEntityID produced %d parts, want 6: %q", len(parts), got)
	}
}

func TestDAGEntityID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		dagID string
		want  string
	}{
		{
			name:  "simple dag ID",
			dagID: "dag-001",
			want:  "semspec.local.agentic.orchestrator.dag.dag-001",
		},
		{
			name:  "uuid-style dag ID",
			dagID: "abcdef123456",
			want:  "semspec.local.agentic.orchestrator.dag.abcdef123456",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := agentgraph.DAGEntityID(tc.dagID)
			if got != tc.want {
				t.Errorf("DAGEntityID(%q) = %q, want %q", tc.dagID, got, tc.want)
			}
		})
	}
}

func TestLoopAndTaskEntityIDs_AreDistinct(t *testing.T) {
	t.Parallel()

	const sharedID = "same-id"
	loopEID := agentgraph.LoopEntityID(sharedID)
	taskEID := agentgraph.TaskEntityID(sharedID)
	if loopEID == taskEID {
		t.Errorf("LoopEntityID and TaskEntityID should differ for the same instance ID, both returned %q", loopEID)
	}
}

func TestLoopTaskDAGEntityIDs_AreAllDistinct(t *testing.T) {
	t.Parallel()

	const sharedID = "same-id"
	loopEID := agentgraph.LoopEntityID(sharedID)
	taskEID := agentgraph.TaskEntityID(sharedID)
	dagEID := agentgraph.DAGEntityID(sharedID)

	ids := []string{loopEID, taskEID, dagEID}
	seen := make(map[string]bool)
	for _, id := range ids {
		if seen[id] {
			t.Errorf("entity ID collision: %q produced by multiple entity ID functions", id)
		}
		seen[id] = true
	}
}

func TestLoopTypePrefix(t *testing.T) {
	t.Parallel()

	want := "semspec.local.agentic.orchestrator.loop"
	got := agentgraph.LoopTypePrefix()
	if got != want {
		t.Errorf("LoopTypePrefix() = %q, want %q", got, want)
	}
}

func TestTaskTypePrefix(t *testing.T) {
	t.Parallel()

	want := "semspec.local.agentic.orchestrator.task"
	got := agentgraph.TaskTypePrefix()
	if got != want {
		t.Errorf("TaskTypePrefix() = %q, want %q", got, want)
	}
}

func TestLoopTypePrefix_MatchesLoopEntityIDPrefix(t *testing.T) {
	t.Parallel()

	prefix := agentgraph.LoopTypePrefix()
	eid := agentgraph.LoopEntityID("some-loop")
	if !strings.HasPrefix(eid, prefix+".") {
		t.Errorf("LoopEntityID(%q) = %q does not start with LoopTypePrefix %q + \".\"", "some-loop", eid, prefix)
	}
}

func TestTaskTypePrefix_MatchesTaskEntityIDPrefix(t *testing.T) {
	t.Parallel()

	prefix := agentgraph.TaskTypePrefix()
	eid := agentgraph.TaskEntityID("some-task")
	if !strings.HasPrefix(eid, prefix+".") {
		t.Errorf("TaskEntityID(%q) = %q does not start with TaskTypePrefix %q + \".\"", "some-task", eid, prefix)
	}
}

func TestLoopEntityIDParsed(t *testing.T) {
	t.Parallel()

	eid := agentgraph.LoopEntityIDParsed("myloop")

	checks := []struct {
		field string
		got   string
		want  string
	}{
		{"Org", eid.Org, agentgraph.OrgDefault},
		{"Platform", eid.Platform, agentgraph.PlatformDefault},
		{"Domain", eid.Domain, agentgraph.DomainAgentic},
		{"System", eid.System, agentgraph.SystemOrchestrator},
		{"Type", eid.Type, agentgraph.TypeLoop},
		{"Instance", eid.Instance, "myloop"},
	}

	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("LoopEntityIDParsed field %s = %q, want %q", c.field, c.got, c.want)
		}
	}
}

func TestParseEntityID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		entityID     string
		wantInstance string
		wantOK       bool
	}{
		{
			name:         "valid loop entity ID",
			entityID:     agentgraph.LoopEntityID("abc123"),
			wantInstance: "abc123",
			wantOK:       true,
		},
		{
			name:         "valid task entity ID",
			entityID:     agentgraph.TaskEntityID("task-001"),
			wantInstance: "task-001",
			wantOK:       true,
		},
		{
			name:         "valid DAG entity ID",
			entityID:     agentgraph.DAGEntityID("dag-xyz"),
			wantInstance: "dag-xyz",
			wantOK:       true,
		},
		{
			name:     "malformed: too few parts",
			entityID: "semspec.local.agentic",
			wantOK:   false,
		},
		{
			name:     "malformed: empty string",
			entityID: "",
			wantOK:   false,
		},
		{
			name:     "malformed: seven parts",
			entityID: "a.b.c.d.e.f.g",
			wantOK:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			instance, ok := agentgraph.ParseEntityID(tc.entityID)
			if ok != tc.wantOK {
				t.Fatalf("ParseEntityID(%q) ok = %v, want %v", tc.entityID, ok, tc.wantOK)
			}
			if tc.wantOK && instance != tc.wantInstance {
				t.Errorf("ParseEntityID(%q) instance = %q, want %q", tc.entityID, instance, tc.wantInstance)
			}
		})
	}
}

func TestValidateInstanceID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{name: "valid simple ID", id: "abc123", wantErr: false},
		{name: "valid hyphenated ID", id: "loop-abc-123", wantErr: false},
		{name: "valid UUID-style", id: "550e8400-e29b-41d4-a716-446655440000", wantErr: false},
		{name: "empty string", id: "", wantErr: true},
		{name: "contains dot", id: "has.dot", wantErr: true},
		{name: "multiple dots", id: "a.b.c", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := agentgraph.ValidateInstanceID(tc.id)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateInstanceID(%q) error = %v, wantErr %v", tc.id, err, tc.wantErr)
			}
		})
	}
}
