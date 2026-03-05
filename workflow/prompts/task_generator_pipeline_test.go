package prompts

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPipelineTaskGeneratorPrompt_ContainsScenarioSection(t *testing.T) {
	params := TaskGeneratorParams{
		Title:   "Auth Plan",
		Goal:    "Add JWT authentication",
		Context: "No auth yet",
		Scenarios: []ScenarioInfo{
			{
				ID:               "scenario.auth-plan.1.1",
				RequirementTitle: "Login",
				Given:            "an unauthenticated user",
				When:             "they submit valid credentials",
				Then:             []string{"response is 200", "JWT token returned"},
			},
			{
				ID:               "scenario.auth-plan.1.2",
				RequirementTitle: "Login",
				Given:            "an unauthenticated user",
				When:             "they submit wrong password",
				Then:             []string{"response is 401"},
			},
		},
	}

	prompt := PipelineTaskGeneratorPrompt(params)

	if !strings.Contains(prompt, "Behavioral Scenarios") {
		t.Error("pipeline prompt must include Behavioral Scenarios section")
	}
	for _, s := range params.Scenarios {
		if !strings.Contains(prompt, s.ID) {
			t.Errorf("pipeline prompt missing scenario ID %q", s.ID)
		}
	}
}

func TestPipelineTaskGeneratorPrompt_OutputFormatUsesScenarioIDs(t *testing.T) {
	params := TaskGeneratorParams{
		Title:   "Plan",
		Goal:    "Build API",
		Context: "Context",
		Scenarios: []ScenarioInfo{
			{
				ID:    "scenario.plan.1.1",
				Given: "given",
				When:  "when",
				Then:  []string{"then"},
			},
		},
	}

	prompt := PipelineTaskGeneratorPrompt(params)

	if !strings.Contains(prompt, `"scenario_ids"`) {
		t.Error("pipeline prompt output format must use scenario_ids field")
	}
	if strings.Contains(prompt, `"acceptance_criteria"`) {
		t.Error("pipeline prompt output format must NOT include acceptance_criteria field")
	}
}

func TestPipelineTaskGeneratorPrompt_ContainsPlanDetails(t *testing.T) {
	params := TaskGeneratorParams{
		Title:   "Rate Limiter Plan",
		Goal:    "Add rate limiting to the /login endpoint",
		Context: "Brute force attacks detected on login",
		Scenarios: []ScenarioInfo{
			{ID: "scenario.rate-limiter.1.1", Given: "g", When: "w", Then: []string{"t"}},
		},
	}

	prompt := PipelineTaskGeneratorPrompt(params)

	if !strings.Contains(prompt, params.Title) {
		t.Errorf("prompt missing title %q", params.Title)
	}
	if !strings.Contains(prompt, params.Goal) {
		t.Errorf("prompt missing goal %q", params.Goal)
	}
	if !strings.Contains(prompt, params.Context) {
		t.Errorf("prompt missing context %q", params.Context)
	}
}

func TestPipelineTaskGeneratorPrompt_WithPhases(t *testing.T) {
	params := TaskGeneratorParams{
		Title:   "Plan",
		Goal:    "Build something",
		Context: "Context",
		Phases: []PhaseInfo{
			{ID: "phase.my-plan-1", Sequence: 1, Name: "Phase 1: Foundation", Description: "Setup"},
			{ID: "phase.my-plan-2", Sequence: 2, Name: "Phase 2: Implementation", Description: "Core logic"},
		},
		Scenarios: []ScenarioInfo{
			{ID: "scenario.plan.1.1", Given: "g", When: "w", Then: []string{"t"}},
		},
	}

	prompt := PipelineTaskGeneratorPrompt(params)

	if !strings.Contains(prompt, "phase_id") {
		t.Error("pipeline prompt with phases must include phase_id field")
	}
	for _, p := range params.Phases {
		if !strings.Contains(prompt, p.ID) {
			t.Errorf("prompt missing phase ID %q", p.ID)
		}
	}
}

func TestPipelineTaskGeneratorPrompt_EmptyScenarios(t *testing.T) {
	params := TaskGeneratorParams{
		Title:     "Plan",
		Goal:      "Build something",
		Context:   "Context",
		Scenarios: nil,
	}

	// Should not panic — returns a prompt without scenario section
	prompt := PipelineTaskGeneratorPrompt(params)

	if !strings.Contains(prompt, params.Goal) {
		t.Errorf("prompt missing goal even with no scenarios")
	}
}

func TestPipelineTaskGeneratorResponse_JSONDeserialization(t *testing.T) {
	raw := `{
		"tasks": [
			{
				"description": "Implement JWT token generation",
				"type": "implement",
				"depends_on": [],
				"scenario_ids": ["scenario.auth.1.1", "scenario.auth.1.2"],
				"files": ["internal/auth/token.go"]
			},
			{
				"description": "Add login endpoint",
				"type": "implement",
				"depends_on": ["task.auth.1"],
				"scenario_ids": ["scenario.auth.2.1"],
				"files": ["cmd/api/login.go"]
			}
		]
	}`

	var resp PipelineTaskGeneratorResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(resp.Tasks) != 2 {
		t.Fatalf("len(Tasks) = %d, want 2", len(resp.Tasks))
	}

	t0 := resp.Tasks[0]
	if t0.Description != "Implement JWT token generation" {
		t.Errorf("Tasks[0].Description = %q", t0.Description)
	}
	if t0.Type != "implement" {
		t.Errorf("Tasks[0].Type = %q, want implement", t0.Type)
	}
	if len(t0.ScenarioIDs) != 2 {
		t.Errorf("len(Tasks[0].ScenarioIDs) = %d, want 2", len(t0.ScenarioIDs))
	}
	if t0.ScenarioIDs[0] != "scenario.auth.1.1" {
		t.Errorf("Tasks[0].ScenarioIDs[0] = %q", t0.ScenarioIDs[0])
	}
	if len(t0.Files) != 1 || t0.Files[0] != "internal/auth/token.go" {
		t.Errorf("Tasks[0].Files = %v", t0.Files)
	}

	t1 := resp.Tasks[1]
	if len(t1.DependsOn) != 1 || t1.DependsOn[0] != "task.auth.1" {
		t.Errorf("Tasks[1].DependsOn = %v", t1.DependsOn)
	}
}

func TestPipelineGeneratedTask_NoAcceptanceCriteria(t *testing.T) {
	raw := `{
		"description": "Implement rate limiter",
		"type": "implement",
		"scenario_ids": ["scenario.plan.1.1"]
	}`

	var task PipelineGeneratedTask
	if err := json.Unmarshal([]byte(raw), &task); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	// PipelineGeneratedTask must not have AcceptanceCriteria field
	// Verify it deserializes cleanly without acceptance_criteria
	if len(task.ScenarioIDs) != 1 {
		t.Errorf("ScenarioIDs = %v, want 1 element", task.ScenarioIDs)
	}
}

// TestPipelineTaskGeneratorPrompt_DoesNotModifyExistingBehavior verifies that
// PipelineTaskGeneratorPrompt is a new function and doesn't change TaskGeneratorPrompt.
func TestPipelineTaskGeneratorPrompt_DoesNotModifyExistingBehavior(t *testing.T) {
	params := TaskGeneratorParams{
		Title:   "Plan",
		Goal:    "Build something",
		Context: "Context",
	}

	// Single-shot prompt must still produce acceptance_criteria format
	singleShotPrompt := TaskGeneratorPrompt(params)
	if !strings.Contains(singleShotPrompt, `"acceptance_criteria"`) {
		t.Error("TaskGeneratorPrompt must still include acceptance_criteria in output format")
	}
	if strings.Contains(singleShotPrompt, `"scenario_ids"`) {
		t.Error("TaskGeneratorPrompt must NOT include scenario_ids — that's pipeline only")
	}
}
