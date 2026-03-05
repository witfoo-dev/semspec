package workflow

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRequirementStatus_IsValid(t *testing.T) {
	tests := []struct {
		status RequirementStatus
		want   bool
	}{
		{RequirementStatusActive, true},
		{RequirementStatusDeprecated, true},
		{RequirementStatusSuperseded, true},
		{"", false},
		{"unknown", false},
		{"Active", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Errorf("RequirementStatus(%q).IsValid() = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestRequirementStatus_CanTransitionTo(t *testing.T) {
	tests := []struct {
		from RequirementStatus
		to   RequirementStatus
		want bool
	}{
		// active -> valid transitions
		{RequirementStatusActive, RequirementStatusDeprecated, true},
		{RequirementStatusActive, RequirementStatusSuperseded, true},
		// active -> invalid
		{RequirementStatusActive, RequirementStatusActive, false},
		// superseded -> valid
		{RequirementStatusSuperseded, RequirementStatusActive, true},
		// superseded -> invalid
		{RequirementStatusSuperseded, RequirementStatusDeprecated, false},
		// deprecated is terminal
		{RequirementStatusDeprecated, RequirementStatusActive, false},
		{RequirementStatusDeprecated, RequirementStatusSuperseded, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			if got := tt.from.CanTransitionTo(tt.to); got != tt.want {
				t.Errorf("(%q).CanTransitionTo(%q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestRequirement_JSONRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	req := Requirement{
		ID:          "requirement.my-plan.1",
		PlanID:      "semspec.local.project.default.plan.my-plan",
		Title:       "User Authentication",
		Description: "The system must authenticate users securely",
		Status:      RequirementStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}

	var got Requirement
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	if got.ID != req.ID {
		t.Errorf("ID = %q, want %q", got.ID, req.ID)
	}
	if got.PlanID != req.PlanID {
		t.Errorf("PlanID = %q, want %q", got.PlanID, req.PlanID)
	}
	if got.Title != req.Title {
		t.Errorf("Title = %q, want %q", got.Title, req.Title)
	}
	if got.Description != req.Description {
		t.Errorf("Description = %q, want %q", got.Description, req.Description)
	}
	if got.Status != req.Status {
		t.Errorf("Status = %q, want %q", got.Status, req.Status)
	}
	if !got.CreatedAt.Equal(req.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, req.CreatedAt)
	}
}
