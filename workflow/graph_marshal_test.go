package workflow

import (
	"testing"
	"time"

	"github.com/c360studio/semstreams/graph"
)

func TestPlanRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	plan := &Plan{
		Slug:      "test-plan",
		Title:     "Test Plan",
		Status:    StatusApproved,
		Goal:      "Build something",
		Context:   "Because reasons",
		ProjectID: ProjectEntityID("default"),
		CreatedAt: now,
	}

	triples := PlanTriples(plan)
	entity := &graph.EntityState{
		ID:      PlanEntityID(plan.Slug),
		Triples: triples,
	}

	got, err := PlanFromEntity(entity)
	if err != nil {
		t.Fatalf("PlanFromEntity: %v", err)
	}

	if got.Slug != plan.Slug {
		t.Errorf("Slug = %q, want %q", got.Slug, plan.Slug)
	}
	if got.Title != plan.Title {
		t.Errorf("Title = %q, want %q", got.Title, plan.Title)
	}
	if got.Status != plan.Status {
		t.Errorf("Status = %q, want %q", got.Status, plan.Status)
	}
	if got.Goal != plan.Goal {
		t.Errorf("Goal = %q, want %q", got.Goal, plan.Goal)
	}
	if got.Context != plan.Context {
		t.Errorf("Context = %q, want %q", got.Context, plan.Context)
	}
	if got.ProjectID != plan.ProjectID {
		t.Errorf("ProjectID = %q, want %q", got.ProjectID, plan.ProjectID)
	}
	if !got.CreatedAt.Equal(plan.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, plan.CreatedAt)
	}
}

func TestPlanRoundTrip_EmptyOptionalFields(t *testing.T) {
	plan := &Plan{
		Slug:      "minimal",
		Title:     "Minimal Plan",
		Status:    StatusCreated,
		CreatedAt: time.Now().Truncate(time.Second),
	}

	triples := PlanTriples(plan)
	entity := &graph.EntityState{ID: PlanEntityID(plan.Slug), Triples: triples}
	got, err := PlanFromEntity(entity)
	if err != nil {
		t.Fatalf("PlanFromEntity: %v", err)
	}

	if got.Goal != "" {
		t.Errorf("Goal = %q, want empty", got.Goal)
	}
	if got.Context != "" {
		t.Errorf("Context = %q, want empty", got.Context)
	}
	if got.ProjectID != "" {
		t.Errorf("ProjectID = %q, want empty", got.ProjectID)
	}
}

func TestRequirementRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	req := &Requirement{
		ID:          "req-001",
		PlanID:      PlanEntityID("test-plan"),
		Title:       "Auth requirement",
		Description: "Implement OAuth2 flow",
		Status:      RequirementStatusActive,
		DependsOn:   []string{"req-000", "req-base"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	triples := RequirementTriples("test-plan", req)
	entity := &graph.EntityState{
		ID:      RequirementEntityID(req.ID),
		Triples: triples,
	}

	got, err := RequirementFromEntity(entity)
	if err != nil {
		t.Fatalf("RequirementFromEntity: %v", err)
	}

	if got.ID != req.ID {
		t.Errorf("ID = %q, want %q", got.ID, req.ID)
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
	if got.PlanID != req.PlanID {
		t.Errorf("PlanID = %q, want %q", got.PlanID, req.PlanID)
	}
	if len(got.DependsOn) != len(req.DependsOn) {
		t.Fatalf("DependsOn len = %d, want %d", len(got.DependsOn), len(req.DependsOn))
	}
	for i, dep := range got.DependsOn {
		if dep != req.DependsOn[i] {
			t.Errorf("DependsOn[%d] = %q, want %q", i, dep, req.DependsOn[i])
		}
	}
}

func TestScenarioRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	s := &Scenario{
		ID:            "scenario-001",
		RequirementID: "req-001",
		Given:         "A user is logged in",
		When:          "They request a token refresh",
		Then:          []string{"A new access token is returned", "The old token is invalidated", "The refresh token rotates"},
		Status:        ScenarioStatusPending,
		CreatedAt:     now,
	}

	triples := ScenarioTriples("test-plan", s)
	entity := &graph.EntityState{
		ID:      ScenarioEntityID(s.ID),
		Triples: triples,
	}

	got, err := ScenarioFromEntity(entity)
	if err != nil {
		t.Fatalf("ScenarioFromEntity: %v", err)
	}

	if got.ID != s.ID {
		t.Errorf("ID = %q, want %q", got.ID, s.ID)
	}
	if got.RequirementID != s.RequirementID {
		t.Errorf("RequirementID = %q, want %q", got.RequirementID, s.RequirementID)
	}
	if got.Given != s.Given {
		t.Errorf("Given = %q, want %q", got.Given, s.Given)
	}
	if got.When != s.When {
		t.Errorf("When = %q, want %q", got.When, s.When)
	}
	if got.Status != s.Status {
		t.Errorf("Status = %q, want %q", got.Status, s.Status)
	}
	if len(got.Then) != len(s.Then) {
		t.Fatalf("Then len = %d, want %d", len(got.Then), len(s.Then))
	}
	for i, then := range got.Then {
		if then != s.Then[i] {
			t.Errorf("Then[%d] = %q, want %q", i, then, s.Then[i])
		}
	}
}

func TestChangeProposalRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	decidedAt := now.Add(time.Hour)
	p := &ChangeProposal{
		ID:             "cp-001",
		PlanID:         PlanEntityID("test-plan"),
		Title:          "Swap OAuth for SAML",
		Rationale:      "Enterprise requirement",
		Status:         ChangeProposalStatusAccepted,
		ProposedBy:     "reviewer",
		AffectedReqIDs: []string{"req-001", "req-002", "req-003"},
		CreatedAt:      now,
		DecidedAt:      &decidedAt,
	}

	triples := ChangeProposalTriples("test-plan", p)
	entity := &graph.EntityState{
		ID:      ChangeProposalEntityID(p.ID),
		Triples: triples,
	}

	got, err := ChangeProposalFromEntity(entity)
	if err != nil {
		t.Fatalf("ChangeProposalFromEntity: %v", err)
	}

	if got.ID != p.ID {
		t.Errorf("ID = %q, want %q", got.ID, p.ID)
	}
	if got.Title != p.Title {
		t.Errorf("Title = %q, want %q", got.Title, p.Title)
	}
	if got.Rationale != p.Rationale {
		t.Errorf("Rationale = %q, want %q", got.Rationale, p.Rationale)
	}
	if got.Status != p.Status {
		t.Errorf("Status = %q, want %q", got.Status, p.Status)
	}
	if got.ProposedBy != p.ProposedBy {
		t.Errorf("ProposedBy = %q, want %q", got.ProposedBy, p.ProposedBy)
	}
	if got.PlanID != p.PlanID {
		t.Errorf("PlanID = %q, want %q", got.PlanID, p.PlanID)
	}
	if got.DecidedAt == nil {
		t.Fatal("DecidedAt is nil, want non-nil")
	}
	if !got.DecidedAt.Equal(*p.DecidedAt) {
		t.Errorf("DecidedAt = %v, want %v", got.DecidedAt, p.DecidedAt)
	}
	if len(got.AffectedReqIDs) != len(p.AffectedReqIDs) {
		t.Fatalf("AffectedReqIDs len = %d, want %d", len(got.AffectedReqIDs), len(p.AffectedReqIDs))
	}
	for i, id := range got.AffectedReqIDs {
		if id != p.AffectedReqIDs[i] {
			t.Errorf("AffectedReqIDs[%d] = %q, want %q", i, id, p.AffectedReqIDs[i])
		}
	}
}

func TestNilEntity(t *testing.T) {
	if _, err := PlanFromEntity(nil); err == nil {
		t.Error("PlanFromEntity(nil) should return error")
	}
	if _, err := RequirementFromEntity(nil); err == nil {
		t.Error("RequirementFromEntity(nil) should return error")
	}
	if _, err := ScenarioFromEntity(nil); err == nil {
		t.Error("ScenarioFromEntity(nil) should return error")
	}
	if _, err := ChangeProposalFromEntity(nil); err == nil {
		t.Error("ChangeProposalFromEntity(nil) should return error")
	}
}
