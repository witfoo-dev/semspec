package prompts

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRequirementGeneratorPrompt_ContainsGoalAndContext(t *testing.T) {
	params := RequirementGeneratorParams{
		Title:   "Add Authentication",
		Goal:    "Implement JWT-based authentication for the API",
		Context: "The API currently has no authentication. We need to protect all endpoints.",
	}

	prompt := RequirementGeneratorPrompt(params)

	if !strings.Contains(prompt, params.Goal) {
		t.Errorf("prompt missing goal %q", params.Goal)
	}
	if !strings.Contains(prompt, params.Context) {
		t.Errorf("prompt missing context %q", params.Context)
	}
	if !strings.Contains(prompt, params.Title) {
		t.Errorf("prompt missing title %q", params.Title)
	}
}

func TestRequirementGeneratorPrompt_ContainsScopeFields(t *testing.T) {
	params := RequirementGeneratorParams{
		Title:          "Auth Plan",
		Goal:           "Add auth",
		Context:        "No auth yet",
		ScopeInclude:   []string{"internal/auth/", "cmd/"},
		ScopeExclude:   []string{"vendor/"},
		ScopeProtected: []string{"go.mod"},
	}

	prompt := RequirementGeneratorPrompt(params)

	for _, path := range params.ScopeInclude {
		if !strings.Contains(prompt, path) {
			t.Errorf("prompt missing ScopeInclude path %q", path)
		}
	}
	for _, path := range params.ScopeExclude {
		if !strings.Contains(prompt, path) {
			t.Errorf("prompt missing ScopeExclude path %q", path)
		}
	}
	for _, path := range params.ScopeProtected {
		if !strings.Contains(prompt, path) {
			t.Errorf("prompt missing ScopeProtected path %q", path)
		}
	}
}

func TestRequirementGeneratorPrompt_DefaultScopeValues(t *testing.T) {
	params := RequirementGeneratorParams{
		Title:   "Minimal Plan",
		Goal:    "Do something",
		Context: "Context here",
	}

	prompt := RequirementGeneratorPrompt(params)

	if !strings.Contains(prompt, "all files") {
		t.Error("prompt missing default 'all files' for empty ScopeInclude")
	}
}

func TestRequirementGeneratorPrompt_InstructsJSONOutput(t *testing.T) {
	params := RequirementGeneratorParams{
		Title:   "Test Plan",
		Goal:    "Build something",
		Context: "Context",
	}

	prompt := RequirementGeneratorPrompt(params)

	if !strings.Contains(prompt, `"requirements"`) {
		t.Error("prompt must instruct JSON output with 'requirements' array")
	}
	if !strings.Contains(prompt, `"title"`) {
		t.Error("prompt must show title field in JSON example")
	}
	if !strings.Contains(prompt, `"description"`) {
		t.Error("prompt must show description field in JSON example")
	}
	if !strings.Contains(prompt, "Return ONLY") {
		t.Error("prompt must instruct to return ONLY JSON")
	}
}

func TestRequirementGeneratorPrompt_ContainsSOPRequirements(t *testing.T) {
	params := RequirementGeneratorParams{
		Title:           "API Plan",
		Goal:            "Build REST API",
		Context:         "Greenfield",
		SOPRequirements: []string{"All endpoints must be versioned", "Error responses must use RFC 7807 format"},
	}

	prompt := RequirementGeneratorPrompt(params)

	for _, sop := range params.SOPRequirements {
		if !strings.Contains(prompt, sop) {
			t.Errorf("prompt missing SOP requirement %q", sop)
		}
	}
}

func TestRequirementGeneratorResponse_JSONDeserialization(t *testing.T) {
	raw := `{
		"requirements": [
			{
				"title": "Input Validation",
				"description": "The API must validate all request parameters."
			},
			{
				"title": "Authentication",
				"description": "Users must authenticate with JWT tokens."
			}
		]
	}`

	var resp RequirementGeneratorResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(resp.Requirements) != 2 {
		t.Fatalf("len(Requirements) = %d, want 2", len(resp.Requirements))
	}
	if resp.Requirements[0].Title != "Input Validation" {
		t.Errorf("Requirements[0].Title = %q, want %q", resp.Requirements[0].Title, "Input Validation")
	}
	if resp.Requirements[1].Description != "Users must authenticate with JWT tokens." {
		t.Errorf("Requirements[1].Description = %q, want %q",
			resp.Requirements[1].Description, "Users must authenticate with JWT tokens.")
	}
}

func TestRequirementGeneratorPrompt_StayOnGoalInstruction(t *testing.T) {
	params := RequirementGeneratorParams{
		Title:   "Rate Limiter",
		Goal:    "Add rate limiting to the /login endpoint",
		Context: "Brute force attacks detected",
	}

	prompt := RequirementGeneratorPrompt(params)

	if !strings.Contains(prompt, "Stay On Goal") {
		t.Error("prompt must include Stay On Goal constraint")
	}
}
