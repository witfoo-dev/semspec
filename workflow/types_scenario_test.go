package workflow

import (
	"encoding/json"
	"testing"
	"time"
)

func TestScenarioStatus_IsValid(t *testing.T) {
	tests := []struct {
		status ScenarioStatus
		want   bool
	}{
		{ScenarioStatusPending, true},
		{ScenarioStatusPassing, true},
		{ScenarioStatusFailing, true},
		{ScenarioStatusSkipped, true},
		{"", false},
		{"unknown", false},
		{"Pending", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Errorf("ScenarioStatus(%q).IsValid() = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestScenarioStatus_CanTransitionTo(t *testing.T) {
	tests := []struct {
		from ScenarioStatus
		to   ScenarioStatus
		want bool
	}{
		// pending transitions
		{ScenarioStatusPending, ScenarioStatusPassing, true},
		{ScenarioStatusPending, ScenarioStatusFailing, true},
		{ScenarioStatusPending, ScenarioStatusSkipped, true},
		{ScenarioStatusPending, ScenarioStatusPending, false},
		// passing transitions
		{ScenarioStatusPassing, ScenarioStatusFailing, true},
		{ScenarioStatusPassing, ScenarioStatusPending, false},
		{ScenarioStatusPassing, ScenarioStatusSkipped, false},
		// failing transitions
		{ScenarioStatusFailing, ScenarioStatusPassing, true},
		{ScenarioStatusFailing, ScenarioStatusPending, false},
		{ScenarioStatusFailing, ScenarioStatusSkipped, false},
		// skipped transitions
		{ScenarioStatusSkipped, ScenarioStatusPending, true},
		{ScenarioStatusSkipped, ScenarioStatusPassing, false},
		{ScenarioStatusSkipped, ScenarioStatusFailing, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			if got := tt.from.CanTransitionTo(tt.to); got != tt.want {
				t.Errorf("(%q).CanTransitionTo(%q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestScenario_JSONRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	scen := Scenario{
		ID:            "scenario.my-plan.1.1",
		RequirementID: "requirement.my-plan.1",
		Given:         "a user exists with valid credentials",
		When:          "the user submits the login form",
		Then:          []string{"a session token is returned", "the user is redirected to the dashboard"},
		Status:        ScenarioStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	data, err := json.Marshal(scen)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}

	var got Scenario
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	if got.ID != scen.ID {
		t.Errorf("ID = %q, want %q", got.ID, scen.ID)
	}
	if got.RequirementID != scen.RequirementID {
		t.Errorf("RequirementID = %q, want %q", got.RequirementID, scen.RequirementID)
	}
	if got.Given != scen.Given {
		t.Errorf("Given = %q, want %q", got.Given, scen.Given)
	}
	if got.When != scen.When {
		t.Errorf("When = %q, want %q", got.When, scen.When)
	}
	if len(got.Then) != len(scen.Then) {
		t.Fatalf("Then len = %d, want %d", len(got.Then), len(scen.Then))
	}
	for i, v := range scen.Then {
		if got.Then[i] != v {
			t.Errorf("Then[%d] = %q, want %q", i, got.Then[i], v)
		}
	}
	if got.Status != scen.Status {
		t.Errorf("Status = %q, want %q", got.Status, scen.Status)
	}
	if !got.CreatedAt.Equal(scen.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, scen.CreatedAt)
	}
}
