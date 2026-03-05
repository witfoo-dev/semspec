package workflow

import (
	"testing"
)

func TestPlanStatus_IsValid_NewStatuses(t *testing.T) {
	tests := []struct {
		status Status
		want   bool
	}{
		{StatusRequirementsGenerated, true},
		{StatusScenariosGenerated, true},
		// Existing statuses still valid
		{StatusCreated, true},
		{StatusDrafted, true},
		{StatusReviewed, true},
		{StatusApproved, true},
		{StatusPhasesGenerated, true},
		{StatusPhasesApproved, true},
		{StatusTasksGenerated, true},
		{StatusTasksApproved, true},
		{StatusImplementing, true},
		{StatusComplete, true},
		{StatusArchived, true},
		{StatusRejected, true},
		// Invalid
		{"", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Errorf("Status(%q).IsValid() = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestPlanStatus_CanTransitionTo_NewStatuses(t *testing.T) {
	tests := []struct {
		from Status
		to   Status
		want bool
	}{
		// approved -> requirements_generated (new flow)
		{StatusApproved, StatusRequirementsGenerated, true},
		// approved -> phases_generated (legacy direct flow still valid)
		{StatusApproved, StatusPhasesGenerated, true},
		// requirements_generated -> scenarios_generated
		{StatusRequirementsGenerated, StatusScenariosGenerated, true},
		// requirements_generated -> rejected
		{StatusRequirementsGenerated, StatusRejected, true},
		// requirements_generated -> phases_generated (invalid, must go through scenarios)
		{StatusRequirementsGenerated, StatusPhasesGenerated, false},
		// scenarios_generated -> phases_generated
		{StatusScenariosGenerated, StatusPhasesGenerated, true},
		// scenarios_generated -> rejected
		{StatusScenariosGenerated, StatusRejected, true},
		// scenarios_generated -> requirements_generated (invalid)
		{StatusScenariosGenerated, StatusRequirementsGenerated, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			if got := tt.from.CanTransitionTo(tt.to); got != tt.want {
				t.Errorf("(%q).CanTransitionTo(%q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}
