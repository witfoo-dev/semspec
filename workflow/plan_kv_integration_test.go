//go:build integration

package workflow

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/c360studio/semspec/workflow/graphutil"
	"github.com/c360studio/semstreams/natsclient"
)

// newTestTripleWriter creates a TripleWriter backed by a real NATS for integration tests.
func newTestTripleWriter(t *testing.T) *graphutil.TripleWriter {
	t.Helper()
	tc := natsclient.NewTestClient(t,
		natsclient.WithKVBuckets("ENTITY_STATES"),
	)
	return &graphutil.TripleWriter{
		NATSClient:    tc.Client,
		Logger:        slog.Default(),
		ComponentName: "test",
	}
}

func TestKV_CreateAndLoadPlan(t *testing.T) {
	tw := newTestTripleWriter(t)
	ctx := context.Background()

	plan, err := CreatePlan(ctx, tw, "test-plan", "Test Plan")
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}

	if plan.Slug != "test-plan" {
		t.Errorf("Slug = %q, want %q", plan.Slug, "test-plan")
	}
	if plan.Title != "Test Plan" {
		t.Errorf("Title = %q, want %q", plan.Title, "Test Plan")
	}

	loaded, err := LoadPlan(ctx, tw, "test-plan")
	if err != nil {
		t.Fatalf("LoadPlan: %v", err)
	}

	if loaded.Slug != plan.Slug {
		t.Errorf("loaded Slug = %q, want %q", loaded.Slug, plan.Slug)
	}
	if loaded.Title != plan.Title {
		t.Errorf("loaded Title = %q, want %q", loaded.Title, plan.Title)
	}
}

func TestKV_PlanExists(t *testing.T) {
	tw := newTestTripleWriter(t)
	ctx := context.Background()

	if PlanExists(ctx, tw, "nonexistent") {
		t.Error("PlanExists should return false for nonexistent plan")
	}

	if _, err := CreatePlan(ctx, tw, "exists", "Exists"); err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}

	if !PlanExists(ctx, tw, "exists") {
		t.Error("PlanExists should return true after creation")
	}
}

func TestKV_SetPlanStatus(t *testing.T) {
	tw := newTestTripleWriter(t)
	ctx := context.Background()

	plan, err := CreatePlan(ctx, tw, "status-test", "Status Test")
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}

	plan.Status = StatusCreated
	if err := SetPlanStatus(ctx, tw, plan, StatusDrafted); err != nil {
		t.Fatalf("SetPlanStatus to drafted: %v", err)
	}

	loaded, err := LoadPlan(ctx, tw, "status-test")
	if err != nil {
		t.Fatalf("LoadPlan after status change: %v", err)
	}
	if loaded.EffectiveStatus() != StatusDrafted {
		t.Errorf("Status = %q, want %q", loaded.EffectiveStatus(), StatusDrafted)
	}
}

func TestKV_SaveAndLoadRequirements(t *testing.T) {
	tw := newTestTripleWriter(t)
	ctx := context.Background()

	if _, err := CreatePlan(ctx, tw, "req-test", "Req Test"); err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}

	now := time.Now().Truncate(time.Second)
	reqs := []Requirement{
		{
			ID:          "req-001",
			PlanID:      PlanEntityID("req-test"),
			Title:       "First Requirement",
			Description: "Do the first thing",
			Status:      RequirementStatusActive,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "req-002",
			PlanID:      PlanEntityID("req-test"),
			Title:       "Second Requirement",
			Description: "Do the second thing",
			Status:      RequirementStatusActive,
			DependsOn:   []string{"req-001"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	if err := SaveRequirements(ctx, tw, reqs, "req-test"); err != nil {
		t.Fatalf("SaveRequirements: %v", err)
	}

	loaded, err := LoadRequirements(ctx, tw, "req-test")
	if err != nil {
		t.Fatalf("LoadRequirements: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("LoadRequirements returned %d, want 2", len(loaded))
	}
}

func TestKV_SaveAndLoadScenarios(t *testing.T) {
	tw := newTestTripleWriter(t)
	ctx := context.Background()

	if _, err := CreatePlan(ctx, tw, "scen-test", "Scenario Test"); err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}

	now := time.Now().Truncate(time.Second)
	reqs := []Requirement{
		{ID: "req-001", PlanID: PlanEntityID("scen-test"), Title: "Req", Status: RequirementStatusActive, CreatedAt: now, UpdatedAt: now},
	}
	if err := SaveRequirements(ctx, tw, reqs, "scen-test"); err != nil {
		t.Fatalf("SaveRequirements: %v", err)
	}

	scenarios := []Scenario{
		{
			ID:            "scen-001",
			RequirementID: "req-001",
			Given:         "A system",
			When:          "Something happens",
			Then:          []string{"Result A", "Result B"},
			Status:        ScenarioStatusPending,
			CreatedAt:     now,
		},
	}

	if err := SaveScenarios(ctx, tw, scenarios, "scen-test"); err != nil {
		t.Fatalf("SaveScenarios: %v", err)
	}

	loaded, err := LoadScenarios(ctx, tw, "scen-test")
	if err != nil {
		t.Fatalf("LoadScenarios: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("LoadScenarios returned %d, want 1", len(loaded))
	}

	if len(loaded[0].Then) != 2 {
		t.Errorf("Then has %d items, want 2", len(loaded[0].Then))
	}
}

func TestKV_SaveAndLoadChangeProposals(t *testing.T) {
	tw := newTestTripleWriter(t)
	ctx := context.Background()

	if _, err := CreatePlan(ctx, tw, "cp-test", "CP Test"); err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}

	now := time.Now().Truncate(time.Second)
	proposals := []ChangeProposal{
		{
			ID:             "cp-001",
			PlanID:         PlanEntityID("cp-test"),
			Title:          "Change Auth",
			Rationale:      "Need SAML",
			Status:         ChangeProposalStatusProposed,
			ProposedBy:     "reviewer",
			AffectedReqIDs: []string{"req-001", "req-002"},
			CreatedAt:      now,
		},
	}

	if err := SaveChangeProposals(ctx, tw, proposals, "cp-test"); err != nil {
		t.Fatalf("SaveChangeProposals: %v", err)
	}

	loaded, err := LoadChangeProposals(ctx, tw, "cp-test")
	if err != nil {
		t.Fatalf("LoadChangeProposals: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("LoadChangeProposals returned %d, want 1", len(loaded))
	}

	if len(loaded[0].AffectedReqIDs) != 2 {
		t.Errorf("AffectedReqIDs has %d items, want 2", len(loaded[0].AffectedReqIDs))
	}
}

func TestKV_DeletePlan(t *testing.T) {
	tw := newTestTripleWriter(t)
	ctx := context.Background()

	if _, err := CreatePlan(ctx, tw, "delete-me", "Delete Me"); err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}

	if !PlanExists(ctx, tw, "delete-me") {
		t.Fatal("plan should exist before delete")
	}

	if err := DeletePlan(ctx, tw, "delete-me"); err != nil {
		t.Fatalf("DeletePlan: %v", err)
	}

	if PlanExists(ctx, tw, "delete-me") {
		t.Error("plan should not exist after delete")
	}
}

func TestKV_ListPlans(t *testing.T) {
	tw := newTestTripleWriter(t)
	ctx := context.Background()

	if _, err := CreatePlan(ctx, tw, "plan-a", "Plan A"); err != nil {
		t.Fatalf("CreatePlan A: %v", err)
	}
	if _, err := CreatePlan(ctx, tw, "plan-b", "Plan B"); err != nil {
		t.Fatalf("CreatePlan B: %v", err)
	}

	result, err := ListPlans(ctx, tw)
	if err != nil {
		t.Fatalf("ListPlans: %v", err)
	}

	if len(result.Plans) != 2 {
		t.Errorf("ListPlans returned %d plans, want 2", len(result.Plans))
	}
}

func TestKV_ApprovePlan(t *testing.T) {
	tw := newTestTripleWriter(t)
	ctx := context.Background()

	plan, err := CreatePlan(ctx, tw, "approve-test", "Approve Test")
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}

	if err := ApprovePlan(ctx, tw, plan); err != nil {
		t.Fatalf("ApprovePlan: %v", err)
	}

	loaded, err := LoadPlan(ctx, tw, "approve-test")
	if err != nil {
		t.Fatalf("LoadPlan: %v", err)
	}
	if loaded.EffectiveStatus() != StatusApproved {
		t.Errorf("Status = %q, want %q", loaded.EffectiveStatus(), StatusApproved)
	}
	if !loaded.Approved {
		t.Error("Approved should be true")
	}

	// Double approve should fail
	if err := ApprovePlan(ctx, tw, loaded); err == nil {
		t.Error("ApprovePlan on already-approved plan should fail")
	}
}

func TestKV_UpdatePlan(t *testing.T) {
	tw := newTestTripleWriter(t)
	ctx := context.Background()

	if _, err := CreatePlan(ctx, tw, "update-test", "Original Title"); err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}

	newTitle := "Updated Title"
	newGoal := "New Goal"
	updated, err := UpdatePlan(ctx, tw, "update-test", UpdatePlanRequest{
		Title: &newTitle,
		Goal:  &newGoal,
	})
	if err != nil {
		t.Fatalf("UpdatePlan: %v", err)
	}
	if updated.Title != newTitle {
		t.Errorf("Title = %q, want %q", updated.Title, newTitle)
	}
	if updated.Goal != newGoal {
		t.Errorf("Goal = %q, want %q", updated.Goal, newGoal)
	}

	loaded, err := LoadPlan(ctx, tw, "update-test")
	if err != nil {
		t.Fatalf("LoadPlan: %v", err)
	}
	if loaded.Title != newTitle {
		t.Errorf("persisted Title = %q, want %q", loaded.Title, newTitle)
	}
}

func TestKV_UpdatePlan_StateGuard(t *testing.T) {
	tw := newTestTripleWriter(t)
	ctx := context.Background()

	plan, err := CreatePlan(ctx, tw, "guard-test", "Guard Test")
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}

	// Walk valid transition path to implementing
	transitions := []Status{StatusDrafted, StatusReviewed, StatusApproved, StatusRequirementsGenerated, StatusScenariosGenerated, StatusReadyForExecution, StatusImplementing}
	for _, target := range transitions {
		if err := SetPlanStatus(ctx, tw, plan, target); err != nil {
			t.Fatalf("SetPlanStatus %s → %s: %v", plan.Status, target, err)
		}
		plan.Status = target
	}

	newTitle := "Nope"
	if _, err := UpdatePlan(ctx, tw, "guard-test", UpdatePlanRequest{Title: &newTitle}); err == nil {
		t.Error("UpdatePlan on implementing plan should fail")
	}
}

func TestKV_ArchiveAndUnarchive(t *testing.T) {
	tw := newTestTripleWriter(t)
	ctx := context.Background()

	if _, err := CreatePlan(ctx, tw, "archive-test", "Archive Test"); err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}

	if err := ArchivePlan(ctx, tw, "archive-test"); err != nil {
		t.Fatalf("ArchivePlan: %v", err)
	}

	loaded, err := LoadPlan(ctx, tw, "archive-test")
	if err != nil {
		t.Fatalf("LoadPlan after archive: %v", err)
	}
	if loaded.EffectiveStatus() != StatusArchived {
		t.Errorf("Status = %q, want %q", loaded.EffectiveStatus(), StatusArchived)
	}

	if err := UnarchivePlan(ctx, tw, "archive-test"); err != nil {
		t.Fatalf("UnarchivePlan: %v", err)
	}

	loaded, err = LoadPlan(ctx, tw, "archive-test")
	if err != nil {
		t.Fatalf("LoadPlan after unarchive: %v", err)
	}
	if loaded.EffectiveStatus() != StatusComplete {
		t.Errorf("Status = %q, want %q", loaded.EffectiveStatus(), StatusComplete)
	}
}

func TestKV_ResetPlan(t *testing.T) {
	tw := newTestTripleWriter(t)
	ctx := context.Background()

	plan, err := CreatePlan(ctx, tw, "reset-test", "Reset Test")
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}

	// created → drafted → rejected
	plan.Status = StatusCreated
	if err := SetPlanStatus(ctx, tw, plan, StatusDrafted); err != nil {
		t.Fatalf("SetPlanStatus drafted: %v", err)
	}
	plan.Status = StatusDrafted
	if err := SetPlanStatus(ctx, tw, plan, StatusRejected); err != nil {
		t.Fatalf("SetPlanStatus rejected: %v", err)
	}

	if err := ResetPlan(ctx, tw, "reset-test"); err != nil {
		t.Fatalf("ResetPlan: %v", err)
	}

	loaded, err := LoadPlan(ctx, tw, "reset-test")
	if err != nil {
		t.Fatalf("LoadPlan after reset: %v", err)
	}
	if loaded.EffectiveStatus() != StatusApproved {
		t.Errorf("Status = %q, want %q", loaded.EffectiveStatus(), StatusApproved)
	}
}

func TestKV_CreatePlan_DuplicateRejected(t *testing.T) {
	tw := newTestTripleWriter(t)
	ctx := context.Background()

	if _, err := CreatePlan(ctx, tw, "dupe-test", "First"); err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}

	_, err := CreatePlan(ctx, tw, "dupe-test", "Second")
	if err == nil {
		t.Fatal("CreatePlan with duplicate slug should fail")
	}
}

func TestKV_InvalidSlug(t *testing.T) {
	tw := newTestTripleWriter(t)
	ctx := context.Background()

	slugs := []string{"", "../escape", "has spaces", "UPPERCASE", "a/b"}
	for _, slug := range slugs {
		if _, err := CreatePlan(ctx, tw, slug, "Bad"); err == nil {
			t.Errorf("CreatePlan(%q) should fail", slug)
		}
		if _, err := LoadPlan(ctx, tw, slug); err == nil {
			t.Errorf("LoadPlan(%q) should fail", slug)
		}
	}
}

func TestKV_RequirementDAGValidation(t *testing.T) {
	tw := newTestTripleWriter(t)
	ctx := context.Background()

	if _, err := CreatePlan(ctx, tw, "dag-test", "DAG Test"); err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}

	now := time.Now().Truncate(time.Second)

	reqs := []Requirement{
		{ID: "req-self", PlanID: PlanEntityID("dag-test"), Title: "Self", Status: RequirementStatusActive, DependsOn: []string{"req-self"}, CreatedAt: now, UpdatedAt: now},
	}
	if err := SaveRequirements(ctx, tw, reqs, "dag-test"); err == nil {
		t.Error("SaveRequirements with self-reference should fail")
	}

	cycleReqs := []Requirement{
		{ID: "req-a", PlanID: PlanEntityID("dag-test"), Title: "A", Status: RequirementStatusActive, DependsOn: []string{"req-b"}, CreatedAt: now, UpdatedAt: now},
		{ID: "req-b", PlanID: PlanEntityID("dag-test"), Title: "B", Status: RequirementStatusActive, DependsOn: []string{"req-a"}, CreatedAt: now, UpdatedAt: now},
	}
	if err := SaveRequirements(ctx, tw, cycleReqs, "dag-test"); err == nil {
		t.Error("SaveRequirements with cycle should fail")
	}
}

func TestKV_CrossPlanIsolation_Scenarios(t *testing.T) {
	tw := newTestTripleWriter(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	if _, err := CreatePlan(ctx, tw, "plan-x", "Plan X"); err != nil {
		t.Fatalf("CreatePlan X: %v", err)
	}
	if _, err := CreatePlan(ctx, tw, "plan-y", "Plan Y"); err != nil {
		t.Fatalf("CreatePlan Y: %v", err)
	}

	reqsX := []Requirement{{ID: "req-x1", PlanID: PlanEntityID("plan-x"), Title: "X Req", Status: RequirementStatusActive, CreatedAt: now, UpdatedAt: now}}
	reqsY := []Requirement{{ID: "req-y1", PlanID: PlanEntityID("plan-y"), Title: "Y Req", Status: RequirementStatusActive, CreatedAt: now, UpdatedAt: now}}

	if err := SaveRequirements(ctx, tw, reqsX, "plan-x"); err != nil {
		t.Fatalf("SaveRequirements X: %v", err)
	}
	if err := SaveRequirements(ctx, tw, reqsY, "plan-y"); err != nil {
		t.Fatalf("SaveRequirements Y: %v", err)
	}

	scenX := []Scenario{{ID: "sc-x1", RequirementID: "req-x1", Given: "X", When: "X happens", Then: []string{"X result"}, Status: ScenarioStatusPending, CreatedAt: now}}
	scenY := []Scenario{{ID: "sc-y1", RequirementID: "req-y1", Given: "Y", When: "Y happens", Then: []string{"Y result"}, Status: ScenarioStatusPending, CreatedAt: now}}

	if err := SaveScenarios(ctx, tw, scenX, "plan-x"); err != nil {
		t.Fatalf("SaveScenarios X: %v", err)
	}
	if err := SaveScenarios(ctx, tw, scenY, "plan-y"); err != nil {
		t.Fatalf("SaveScenarios Y: %v", err)
	}

	loadedX, err := LoadScenarios(ctx, tw, "plan-x")
	if err != nil {
		t.Fatalf("LoadScenarios X: %v", err)
	}
	if len(loadedX) != 1 || loadedX[0].ID != "sc-x1" {
		t.Errorf("plan-x scenarios: got %d, want 1 with ID sc-x1", len(loadedX))
	}

	loadedY, err := LoadScenarios(ctx, tw, "plan-y")
	if err != nil {
		t.Fatalf("LoadScenarios Y: %v", err)
	}
	if len(loadedY) != 1 || loadedY[0].ID != "sc-y1" {
		t.Errorf("plan-y scenarios: got %d, want 1 with ID sc-y1", len(loadedY))
	}
}

func TestKV_CrossPlanIsolation_ChangeProposals(t *testing.T) {
	tw := newTestTripleWriter(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	if _, err := CreatePlan(ctx, tw, "iso-a", "Iso A"); err != nil {
		t.Fatalf("CreatePlan A: %v", err)
	}
	if _, err := CreatePlan(ctx, tw, "iso-b", "Iso B"); err != nil {
		t.Fatalf("CreatePlan B: %v", err)
	}

	propA := []ChangeProposal{{ID: "cp-a1", PlanID: PlanEntityID("iso-a"), Title: "A prop", Status: ChangeProposalStatusProposed, CreatedAt: now}}
	propB := []ChangeProposal{{ID: "cp-b1", PlanID: PlanEntityID("iso-b"), Title: "B prop", Status: ChangeProposalStatusProposed, CreatedAt: now}}

	if err := SaveChangeProposals(ctx, tw, propA, "iso-a"); err != nil {
		t.Fatalf("SaveChangeProposals A: %v", err)
	}
	if err := SaveChangeProposals(ctx, tw, propB, "iso-b"); err != nil {
		t.Fatalf("SaveChangeProposals B: %v", err)
	}

	loadedA, err := LoadChangeProposals(ctx, tw, "iso-a")
	if err != nil {
		t.Fatalf("LoadChangeProposals A: %v", err)
	}
	if len(loadedA) != 1 || loadedA[0].ID != "cp-a1" {
		t.Errorf("plan iso-a proposals: got %d, want 1 with ID cp-a1", len(loadedA))
	}

	loadedB, err := LoadChangeProposals(ctx, tw, "iso-b")
	if err != nil {
		t.Fatalf("LoadChangeProposals B: %v", err)
	}
	if len(loadedB) != 1 || loadedB[0].ID != "cp-b1" {
		t.Errorf("plan iso-b proposals: got %d, want 1 with ID cp-b1", len(loadedB))
	}
}

func TestKV_NilTripleWriterSafety(t *testing.T) {
	ctx := context.Background()

	// PlanExists with nil tw should return false, not panic
	if PlanExists(ctx, nil, "test-slug") {
		t.Error("PlanExists(nil) should return false")
	}

	// LoadPlan with nil tw should return error, not panic
	if _, err := LoadPlan(ctx, nil, "test-slug"); err == nil {
		t.Error("LoadPlan(nil) should return error")
	}

	// SavePlan with nil tw should not panic
	plan := &Plan{Slug: "test", Title: "Test"}
	if err := SavePlan(ctx, nil, plan); err != nil {
		t.Errorf("SavePlan(nil) should silently succeed, got: %v", err)
	}
}
