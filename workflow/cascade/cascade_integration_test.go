//go:build integration

package cascade

import (
	"context"
	"testing"

	"github.com/c360studio/semspec/workflow"
	"github.com/c360studio/semstreams/natsclient"
)

// setupCascadeFixture creates a real KV-backed fixture with a plan, requirements,
// and scenarios seeded for integration tests.
func setupCascadeFixture(t *testing.T) (*natsclient.KVStore, string) {
	t.Helper()
	ctx := context.Background()

	tc := natsclient.NewTestClient(t)
	kv := tc.KV
	slug := "cascade-test"

	if _, err := workflow.CreatePlan(ctx, kv, slug, "Cascade Test Plan"); err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}

	reqs := []workflow.Requirement{
		{ID: "req-1", PlanID: workflow.PlanEntityID(slug), Title: "Auth", Status: workflow.RequirementStatusActive},
		{ID: "req-2", PlanID: workflow.PlanEntityID(slug), Title: "Logging", Status: workflow.RequirementStatusActive},
	}
	if err := workflow.SaveRequirements(ctx, kv, reqs, slug); err != nil {
		t.Fatalf("SaveRequirements: %v", err)
	}

	scenarios := []workflow.Scenario{
		{ID: "sc-1", RequirementID: "req-1", Given: "a user"},
		{ID: "sc-2", RequirementID: "req-1", Given: "a token"},
		{ID: "sc-3", RequirementID: "req-2", Given: "log files"},
	}
	if err := workflow.SaveScenarios(ctx, kv, scenarios, slug); err != nil {
		t.Fatalf("SaveScenarios: %v", err)
	}

	return kv, slug
}

func TestChangeProposal_AffectsOneRequirement(t *testing.T) {
	kv, slug := setupCascadeFixture(t)

	proposal := &workflow.ChangeProposal{
		ID:             "cp-1",
		AffectedReqIDs: []string{"req-1"}, // affects sc-1, sc-2
	}

	result, err := ChangeProposal(context.Background(), kv, slug, proposal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.AffectedRequirementIDs) != 1 {
		t.Errorf("AffectedRequirementIDs = %d, want 1", len(result.AffectedRequirementIDs))
	}
	if len(result.AffectedScenarioIDs) != 2 {
		t.Errorf("AffectedScenarioIDs = %d, want 2 (sc-1, sc-2)", len(result.AffectedScenarioIDs))
	}
}

func TestChangeProposal_AffectsAllRequirements(t *testing.T) {
	kv, slug := setupCascadeFixture(t)

	proposal := &workflow.ChangeProposal{
		ID:             "cp-1",
		AffectedReqIDs: []string{"req-1", "req-2"},
	}

	result, err := ChangeProposal(context.Background(), kv, slug, proposal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.AffectedScenarioIDs) != 3 {
		t.Errorf("AffectedScenarioIDs = %d, want 3", len(result.AffectedScenarioIDs))
	}
}

func TestChangeProposal_NoMatchingScenarios(t *testing.T) {
	kv, slug := setupCascadeFixture(t)

	proposal := &workflow.ChangeProposal{
		ID:             "cp-1",
		AffectedReqIDs: []string{"req-nonexistent"},
	}

	result, err := ChangeProposal(context.Background(), kv, slug, proposal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.AffectedScenarioIDs) != 0 {
		t.Errorf("AffectedScenarioIDs = %d, want 0", len(result.AffectedScenarioIDs))
	}
}
