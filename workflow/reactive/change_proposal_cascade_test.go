package reactive

import (
	"context"
	"testing"
	"time"

	workflow "github.com/c360studio/semspec/workflow"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func setupCascadeFixture(t *testing.T) (context.Context, *workflow.Manager, string, *workflow.Plan) {
	t.Helper()
	ctx := context.Background()
	m := workflow.NewManager(t.TempDir())
	plan, err := m.CreatePlan(ctx, "cascade-plan", "Cascade Plan")
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	return ctx, m, "cascade-plan", plan
}

func saveRequirements(t *testing.T, ctx context.Context, m *workflow.Manager, slug string, reqs []workflow.Requirement) {
	t.Helper()
	if err := m.SaveRequirements(ctx, reqs, slug); err != nil {
		t.Fatalf("SaveRequirements: %v", err)
	}
}

func saveScenarios(t *testing.T, ctx context.Context, m *workflow.Manager, slug string, scs []workflow.Scenario) {
	t.Helper()
	if err := m.SaveScenarios(ctx, scs, slug); err != nil {
		t.Fatalf("SaveScenarios: %v", err)
	}
}

func saveTasks(t *testing.T, ctx context.Context, m *workflow.Manager, slug string, tasks []workflow.Task) {
	t.Helper()
	if err := m.SaveTasks(ctx, tasks, slug); err != nil {
		t.Fatalf("SaveTasks: %v", err)
	}
}

func makeReq(slug, id, title string) workflow.Requirement {
	now := time.Now()
	return workflow.Requirement{
		ID:        id,
		PlanID:    workflow.PlanEntityID(slug),
		Title:     title,
		Status:    workflow.RequirementStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func makeScenario(id, reqID string) workflow.Scenario {
	now := time.Now()
	return workflow.Scenario{
		ID:            id,
		RequirementID: reqID,
		Given:         "some precondition",
		When:          "some action",
		Then:          []string{"some outcome"},
		Status:        workflow.ScenarioStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func makeTask(planID, id string, seq int, status workflow.TaskStatus, scenarioIDs []string) workflow.Task {
	return workflow.Task{
		ID:          id,
		PlanID:      planID,
		Sequence:    seq,
		Description: "task " + id,
		Status:      status,
		ScenarioIDs: scenarioIDs,
		CreatedAt:   time.Now(),
	}
}

// ---------------------------------------------------------------------------
// Test 1: Happy path — proposal affects 1 requirement → 2 scenarios → 3 tasks
// ---------------------------------------------------------------------------

func TestCascadeChangeProposal_HappyPath(t *testing.T) {
	ctx, m, slug, plan := setupCascadeFixture(t)

	req1 := makeReq(slug, "requirement.cascade-plan.1", "Auth requirement")
	saveRequirements(t, ctx, m, slug, []workflow.Requirement{req1})

	sc1 := makeScenario("scenario.cascade-plan.1.1", req1.ID)
	sc2 := makeScenario("scenario.cascade-plan.1.2", req1.ID)
	saveScenarios(t, ctx, m, slug, []workflow.Scenario{sc1, sc2})

	task1 := makeTask(plan.ID, "task.cascade-plan.1", 1, workflow.TaskStatusApproved, []string{sc1.ID})
	task2 := makeTask(plan.ID, "task.cascade-plan.2", 2, workflow.TaskStatusApproved, []string{sc2.ID})
	task3 := makeTask(plan.ID, "task.cascade-plan.3", 3, workflow.TaskStatusApproved, []string{sc1.ID, sc2.ID})
	saveTasks(t, ctx, m, slug, []workflow.Task{task1, task2, task3})

	proposal := &workflow.ChangeProposal{
		ID:             "change-proposal.cascade-plan.1",
		PlanID:         plan.ID,
		AffectedReqIDs: []string{req1.ID},
	}

	result, err := CascadeChangeProposal(ctx, m, slug, proposal)
	if err != nil {
		t.Fatalf("CascadeChangeProposal: %v", err)
	}

	if result.TasksDirtied != 3 {
		t.Errorf("expected 3 tasks dirtied, got %d", result.TasksDirtied)
	}
	if len(result.AffectedTaskIDs) != 3 {
		t.Errorf("expected 3 affected task IDs, got %d: %v", len(result.AffectedTaskIDs), result.AffectedTaskIDs)
	}
	if len(result.AffectedScenarioIDs) != 2 {
		t.Errorf("expected 2 affected scenario IDs, got %d", len(result.AffectedScenarioIDs))
	}

	tasks, _ := m.LoadTasks(ctx, slug)
	for _, tk := range tasks {
		if tk.Status != workflow.TaskStatusDirty {
			t.Errorf("task %q: expected dirty, got %q", tk.ID, tk.Status)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 2: Partial overlap — only tasks linked to affected requirement's scenarios get dirty
// ---------------------------------------------------------------------------

func TestCascadeChangeProposal_PartialOverlap(t *testing.T) {
	ctx, m, slug, plan := setupCascadeFixture(t)

	req1 := makeReq(slug, "requirement.cascade-plan.1", "Affected requirement")
	req2 := makeReq(slug, "requirement.cascade-plan.2", "Unaffected requirement")
	saveRequirements(t, ctx, m, slug, []workflow.Requirement{req1, req2})

	sc1 := makeScenario("scenario.cascade-plan.1.1", req1.ID) // affected
	sc2 := makeScenario("scenario.cascade-plan.2.1", req2.ID) // not affected
	saveScenarios(t, ctx, m, slug, []workflow.Scenario{sc1, sc2})

	taskAffected := makeTask(plan.ID, "task.cascade-plan.1", 1, workflow.TaskStatusApproved, []string{sc1.ID})
	taskUnaffected := makeTask(plan.ID, "task.cascade-plan.2", 2, workflow.TaskStatusApproved, []string{sc2.ID})
	saveTasks(t, ctx, m, slug, []workflow.Task{taskAffected, taskUnaffected})

	proposal := &workflow.ChangeProposal{
		ID:             "change-proposal.cascade-plan.1",
		PlanID:         plan.ID,
		AffectedReqIDs: []string{req1.ID}, // only req1
	}

	result, err := CascadeChangeProposal(ctx, m, slug, proposal)
	if err != nil {
		t.Fatalf("CascadeChangeProposal: %v", err)
	}

	if result.TasksDirtied != 1 {
		t.Errorf("expected 1 task dirtied, got %d", result.TasksDirtied)
	}
	if len(result.AffectedTaskIDs) != 1 || result.AffectedTaskIDs[0] != taskAffected.ID {
		t.Errorf("expected only %q affected, got %v", taskAffected.ID, result.AffectedTaskIDs)
	}

	tasks, _ := m.LoadTasks(ctx, slug)
	taskMap := make(map[string]workflow.TaskStatus)
	for _, tk := range tasks {
		taskMap[tk.ID] = tk.Status
	}

	if taskMap[taskAffected.ID] != workflow.TaskStatusDirty {
		t.Errorf("affected task: expected dirty, got %q", taskMap[taskAffected.ID])
	}
	if taskMap[taskUnaffected.ID] != workflow.TaskStatusApproved {
		t.Errorf("unaffected task: expected approved (unchanged), got %q", taskMap[taskUnaffected.ID])
	}
}

// ---------------------------------------------------------------------------
// Test 3: Terminal tasks (completed/failed) are NOT dirtied
// ---------------------------------------------------------------------------

func TestCascadeChangeProposal_TerminalTasksSkipped(t *testing.T) {
	ctx, m, slug, plan := setupCascadeFixture(t)

	req1 := makeReq(slug, "requirement.cascade-plan.1", "Auth requirement")
	saveRequirements(t, ctx, m, slug, []workflow.Requirement{req1})

	sc1 := makeScenario("scenario.cascade-plan.1.1", req1.ID)
	saveScenarios(t, ctx, m, slug, []workflow.Scenario{sc1})

	taskCompleted := makeTask(plan.ID, "task.cascade-plan.1", 1, workflow.TaskStatusCompleted, []string{sc1.ID})
	taskFailed := makeTask(plan.ID, "task.cascade-plan.2", 2, workflow.TaskStatusFailed, []string{sc1.ID})
	taskActive := makeTask(plan.ID, "task.cascade-plan.3", 3, workflow.TaskStatusApproved, []string{sc1.ID})
	saveTasks(t, ctx, m, slug, []workflow.Task{taskCompleted, taskFailed, taskActive})

	proposal := &workflow.ChangeProposal{
		ID:             "change-proposal.cascade-plan.1",
		PlanID:         plan.ID,
		AffectedReqIDs: []string{req1.ID},
	}

	result, err := CascadeChangeProposal(ctx, m, slug, proposal)
	if err != nil {
		t.Fatalf("CascadeChangeProposal: %v", err)
	}

	// Only the active (non-terminal) task should be dirtied.
	if result.TasksDirtied != 1 {
		t.Errorf("expected 1 task dirtied (terminal tasks skipped), got %d", result.TasksDirtied)
	}

	tasks, _ := m.LoadTasks(ctx, slug)
	taskMap := make(map[string]workflow.TaskStatus)
	for _, tk := range tasks {
		taskMap[tk.ID] = tk.Status
	}

	if taskMap[taskCompleted.ID] != workflow.TaskStatusCompleted {
		t.Errorf("completed task should stay completed, got %q", taskMap[taskCompleted.ID])
	}
	if taskMap[taskFailed.ID] != workflow.TaskStatusFailed {
		t.Errorf("failed task should stay failed, got %q", taskMap[taskFailed.ID])
	}
	if taskMap[taskActive.ID] != workflow.TaskStatusDirty {
		t.Errorf("active task should become dirty, got %q", taskMap[taskActive.ID])
	}
}

// ---------------------------------------------------------------------------
// Test 4: No affected scenarios → empty cascade result
// ---------------------------------------------------------------------------

func TestCascadeChangeProposal_NoAffectedScenarios(t *testing.T) {
	ctx, m, slug, plan := setupCascadeFixture(t)

	req1 := makeReq(slug, "requirement.cascade-plan.1", "Orphan requirement")
	saveRequirements(t, ctx, m, slug, []workflow.Requirement{req1})
	// No scenarios linked to req1.

	task1 := makeTask(plan.ID, "task.cascade-plan.1", 1, workflow.TaskStatusApproved, nil)
	saveTasks(t, ctx, m, slug, []workflow.Task{task1})

	proposal := &workflow.ChangeProposal{
		ID:             "change-proposal.cascade-plan.1",
		PlanID:         plan.ID,
		AffectedReqIDs: []string{req1.ID},
	}

	result, err := CascadeChangeProposal(ctx, m, slug, proposal)
	if err != nil {
		t.Fatalf("CascadeChangeProposal: %v", err)
	}

	if result.TasksDirtied != 0 {
		t.Errorf("expected 0 tasks dirtied, got %d", result.TasksDirtied)
	}
	if len(result.AffectedScenarioIDs) != 0 {
		t.Errorf("expected 0 affected scenario IDs, got %d", len(result.AffectedScenarioIDs))
	}
	if len(result.AffectedTaskIDs) != 0 {
		t.Errorf("expected 0 affected task IDs, got %d", len(result.AffectedTaskIDs))
	}

	// Task should be unchanged.
	tasks, _ := m.LoadTasks(ctx, slug)
	if tasks[0].Status != workflow.TaskStatusApproved {
		t.Errorf("task should stay approved, got %q", tasks[0].Status)
	}
}

// ---------------------------------------------------------------------------
// Test 5: Already-dirty task stays dirty, not double-counted
// ---------------------------------------------------------------------------

func TestCascadeChangeProposal_AlreadyDirtyStaysDirty(t *testing.T) {
	ctx, m, slug, plan := setupCascadeFixture(t)

	req1 := makeReq(slug, "requirement.cascade-plan.1", "Auth requirement")
	saveRequirements(t, ctx, m, slug, []workflow.Requirement{req1})

	sc1 := makeScenario("scenario.cascade-plan.1.1", req1.ID)
	saveScenarios(t, ctx, m, slug, []workflow.Scenario{sc1})

	// Task is already dirty from a previous cascade.
	taskAlreadyDirty := makeTask(plan.ID, "task.cascade-plan.1", 1, workflow.TaskStatusDirty, []string{sc1.ID})
	saveTasks(t, ctx, m, slug, []workflow.Task{taskAlreadyDirty})

	proposal := &workflow.ChangeProposal{
		ID:             "change-proposal.cascade-plan.1",
		PlanID:         plan.ID,
		AffectedReqIDs: []string{req1.ID},
	}

	result, err := CascadeChangeProposal(ctx, m, slug, proposal)
	if err != nil {
		t.Fatalf("CascadeChangeProposal: %v", err)
	}

	// TasksDirtied is 0 because it was already dirty — no status change.
	if result.TasksDirtied != 0 {
		t.Errorf("expected 0 newly dirtied (already dirty), got %d", result.TasksDirtied)
	}
	// But it should still appear in AffectedTaskIDs (it's in scope).
	if len(result.AffectedTaskIDs) != 1 {
		t.Errorf("expected 1 affected task ID, got %d", len(result.AffectedTaskIDs))
	}

	tasks, _ := m.LoadTasks(ctx, slug)
	if tasks[0].Status != workflow.TaskStatusDirty {
		t.Errorf("already-dirty task should remain dirty, got %q", tasks[0].Status)
	}
}

// ---------------------------------------------------------------------------
// Test 6: Proposal with no affected requirements → empty cascade
// ---------------------------------------------------------------------------

func TestCascadeChangeProposal_NoAffectedRequirements(t *testing.T) {
	ctx, m, slug, _ := setupCascadeFixture(t)

	proposal := &workflow.ChangeProposal{
		ID:             "change-proposal.cascade-plan.1",
		AffectedReqIDs: nil,
	}

	result, err := CascadeChangeProposal(ctx, m, slug, proposal)
	if err != nil {
		t.Fatalf("CascadeChangeProposal: %v", err)
	}

	if result.TasksDirtied != 0 {
		t.Errorf("expected 0 tasks dirtied, got %d", result.TasksDirtied)
	}
	if len(result.AffectedRequirementIDs) != 0 {
		t.Errorf("expected 0 affected requirement IDs, got %d", len(result.AffectedRequirementIDs))
	}
}

// ---------------------------------------------------------------------------
// Test 7: Nil proposal returns error
// ---------------------------------------------------------------------------

func TestCascadeChangeProposal_NilProposal(t *testing.T) {
	ctx := context.Background()
	m := workflow.NewManager(t.TempDir())

	_, err := CascadeChangeProposal(ctx, m, "any-slug", nil)
	if err == nil {
		t.Fatal("expected error for nil proposal, got nil")
	}
}
