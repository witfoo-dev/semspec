package scenarioorchestrator

import (
	"sort"
	"testing"

	"github.com/c360studio/semspec/workflow"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// scenarioIDs extracts ScenarioID strings from a slice of ScenarioRef for
// comparison without depending on slice ordering.
func scenarioIDs(refs []ScenarioRef) []string {
	ids := make([]string, len(refs))
	for i, r := range refs {
		ids[i] = r.ScenarioID
	}
	sort.Strings(ids)
	return ids
}

// makeReq builds a Requirement with the given id and optional upstream deps.
func makeReq(id string, deps ...string) workflow.Requirement {
	return workflow.Requirement{
		ID:        id,
		PlanID:    "test-plan",
		Title:     id,
		Status:    workflow.RequirementStatusActive,
		DependsOn: deps,
	}
}

// makeScenario builds a Scenario owned by reqID with the given status.
func makeScenario(id, reqID string, status workflow.ScenarioStatus) workflow.Scenario {
	return workflow.Scenario{
		ID:            id,
		RequirementID: reqID,
		Status:        status,
	}
}

// makeRef builds a ScenarioRef for use in trigger.Scenarios.
func makeRef(scenarioID string) ScenarioRef {
	return ScenarioRef{ScenarioID: scenarioID, Prompt: "test prompt for " + scenarioID}
}

// ---------------------------------------------------------------------------
// requirementComplete
// ---------------------------------------------------------------------------

func TestRequirementComplete_AllPassing(t *testing.T) {
	reqScenarios := map[string][]workflow.Scenario{
		"r1": {
			makeScenario("s1", "r1", workflow.ScenarioStatusPassing),
			makeScenario("s2", "r1", workflow.ScenarioStatusPassing),
		},
	}
	if !requirementComplete("r1", reqScenarios) {
		t.Error("requirementComplete() = false, want true when all scenarios are passing")
	}
}

func TestRequirementComplete_AllSkipped(t *testing.T) {
	reqScenarios := map[string][]workflow.Scenario{
		"r1": {
			makeScenario("s1", "r1", workflow.ScenarioStatusSkipped),
		},
	}
	if !requirementComplete("r1", reqScenarios) {
		t.Error("requirementComplete() = false, want true when all scenarios are skipped")
	}
}

func TestRequirementComplete_MixedPassingAndSkipped(t *testing.T) {
	reqScenarios := map[string][]workflow.Scenario{
		"r1": {
			makeScenario("s1", "r1", workflow.ScenarioStatusPassing),
			makeScenario("s2", "r1", workflow.ScenarioStatusSkipped),
		},
	}
	if !requirementComplete("r1", reqScenarios) {
		t.Error("requirementComplete() = false, want true for mixed passing+skipped")
	}
}

func TestRequirementComplete_OneFailing(t *testing.T) {
	reqScenarios := map[string][]workflow.Scenario{
		"r1": {
			makeScenario("s1", "r1", workflow.ScenarioStatusPassing),
			makeScenario("s2", "r1", workflow.ScenarioStatusFailing),
		},
	}
	if requirementComplete("r1", reqScenarios) {
		t.Error("requirementComplete() = true, want false when a scenario is failing")
	}
}

func TestRequirementComplete_OnePending(t *testing.T) {
	reqScenarios := map[string][]workflow.Scenario{
		"r1": {
			makeScenario("s1", "r1", workflow.ScenarioStatusPending),
		},
	}
	if requirementComplete("r1", reqScenarios) {
		t.Error("requirementComplete() = true, want false when a scenario is pending")
	}
}

func TestRequirementComplete_NoScenarios(t *testing.T) {
	reqScenarios := map[string][]workflow.Scenario{
		"r1": {},
	}
	if requirementComplete("r1", reqScenarios) {
		t.Error("requirementComplete() = true, want false for requirement with no scenarios")
	}
}

func TestRequirementComplete_RequirementNotInMap(t *testing.T) {
	// An unrecognised requirement ID is treated as incomplete.
	if requirementComplete("unknown", map[string][]workflow.Scenario{}) {
		t.Error("requirementComplete() = true, want false for unknown requirement ID")
	}
}

// ---------------------------------------------------------------------------
// filterReadyScenarios — root requirements (no deps)
// ---------------------------------------------------------------------------

func TestFilterReadyScenarios_NoDependencies_AllDispatched(t *testing.T) {
	// Root requirements have no DependsOn — they should always be dispatched
	// as long as they have pending/dirty scenarios in the refs list.
	reqs := []workflow.Requirement{
		makeReq("r1"),
		makeReq("r2"),
	}
	allScenarios := []workflow.Scenario{
		makeScenario("s1", "r1", workflow.ScenarioStatusPending),
		makeScenario("s2", "r2", workflow.ScenarioStatusPending),
	}
	refs := []ScenarioRef{makeRef("s1"), makeRef("s2")}

	got := filterReadyScenarios(refs, reqs, allScenarios)
	if len(got) != 2 {
		t.Errorf("filterReadyScenarios() returned %d refs, want 2", len(got))
	}
}

func TestFilterReadyScenarios_RootRequirement_PartialRefs(t *testing.T) {
	// Only s1 is in the trigger (s2 might already be dispatched or isn't pending).
	reqs := []workflow.Requirement{makeReq("r1")}
	allScenarios := []workflow.Scenario{
		makeScenario("s1", "r1", workflow.ScenarioStatusPending),
		makeScenario("s2", "r1", workflow.ScenarioStatusPassing),
	}
	refs := []ScenarioRef{makeRef("s1")} // s2 already passing, not in refs

	got := filterReadyScenarios(refs, reqs, allScenarios)
	if len(got) != 1 || got[0].ScenarioID != "s1" {
		t.Errorf("filterReadyScenarios() = %v, want [s1]", scenarioIDs(got))
	}
}

// ---------------------------------------------------------------------------
// filterReadyScenarios — dependency blocking
// ---------------------------------------------------------------------------

func TestFilterReadyScenarios_DependentBlockedByIncompleteUpstream(t *testing.T) {
	// r2 depends on r1; r1 has a failing scenario so r2 is blocked.
	reqs := []workflow.Requirement{
		makeReq("r1"),
		makeReq("r2", "r1"),
	}
	allScenarios := []workflow.Scenario{
		makeScenario("s1", "r1", workflow.ScenarioStatusFailing),
		makeScenario("s2", "r2", workflow.ScenarioStatusPending),
	}
	refs := []ScenarioRef{makeRef("s1"), makeRef("s2")}

	got := filterReadyScenarios(refs, reqs, allScenarios)
	ids := scenarioIDs(got)
	for _, id := range ids {
		if id == "s2" {
			t.Error("filterReadyScenarios() included s2, but r2 should be blocked by failing r1")
		}
	}
}

func TestFilterReadyScenarios_DependentUnblockedWhenUpstreamComplete(t *testing.T) {
	// r2 depends on r1; r1 is passing, so r2 scenarios should be dispatched.
	reqs := []workflow.Requirement{
		makeReq("r1"),
		makeReq("r2", "r1"),
	}
	allScenarios := []workflow.Scenario{
		makeScenario("s1", "r1", workflow.ScenarioStatusPassing),
		makeScenario("s2", "r2", workflow.ScenarioStatusPending),
	}
	refs := []ScenarioRef{makeRef("s2")} // s1 already passing, not in refs

	got := filterReadyScenarios(refs, reqs, allScenarios)
	if len(got) != 1 || got[0].ScenarioID != "s2" {
		t.Errorf("filterReadyScenarios() = %v, want [s2]", scenarioIDs(got))
	}
}

// ---------------------------------------------------------------------------
// filterReadyScenarios — independent requirements dispatch in parallel
// ---------------------------------------------------------------------------

func TestFilterReadyScenarios_IndependentRequirementsDispatchedTogether(t *testing.T) {
	// r1 and r2 are independent (no deps between them); both should be dispatched.
	reqs := []workflow.Requirement{
		makeReq("r1"),
		makeReq("r2"),
	}
	allScenarios := []workflow.Scenario{
		makeScenario("s1", "r1", workflow.ScenarioStatusPending),
		makeScenario("s2", "r2", workflow.ScenarioStatusPending),
	}
	refs := []ScenarioRef{makeRef("s1"), makeRef("s2")}

	got := filterReadyScenarios(refs, reqs, allScenarios)
	gotIDs := scenarioIDs(got)
	wantIDs := []string{"s1", "s2"}

	if len(gotIDs) != len(wantIDs) {
		t.Fatalf("filterReadyScenarios() returned %v, want %v", gotIDs, wantIDs)
	}
	for i, id := range gotIDs {
		if id != wantIDs[i] {
			t.Errorf("gotIDs[%d] = %q, want %q", i, id, wantIDs[i])
		}
	}
}

// ---------------------------------------------------------------------------
// filterReadyScenarios — failing sibling does not block unrelated requirements
// ---------------------------------------------------------------------------

func TestFilterReadyScenarios_FailingScenarioBlocksDependentsNotSiblings(t *testing.T) {
	// r1 has a failing scenario (s1).
	// r2 depends on r1 — blocked.
	// r3 is independent — should still be dispatched.
	reqs := []workflow.Requirement{
		makeReq("r1"),
		makeReq("r2", "r1"),
		makeReq("r3"),
	}
	allScenarios := []workflow.Scenario{
		makeScenario("s1", "r1", workflow.ScenarioStatusFailing),
		makeScenario("s2", "r2", workflow.ScenarioStatusPending),
		makeScenario("s3", "r3", workflow.ScenarioStatusPending),
	}
	refs := []ScenarioRef{makeRef("s1"), makeRef("s2"), makeRef("s3")}

	got := filterReadyScenarios(refs, reqs, allScenarios)
	gotIDs := scenarioIDs(got)

	// s2 must not be dispatched (r2 blocked by r1).
	for _, id := range gotIDs {
		if id == "s2" {
			t.Error("filterReadyScenarios() dispatched s2, but r2 should be blocked by failing r1")
		}
	}

	// s1 (r1 itself has no deps) and s3 (r3 independent) must be dispatched.
	found := make(map[string]bool)
	for _, id := range gotIDs {
		found[id] = true
	}
	for _, expected := range []string{"s1", "s3"} {
		if !found[expected] {
			t.Errorf("filterReadyScenarios() did not dispatch %s, but it should be ready", expected)
		}
	}
}

// ---------------------------------------------------------------------------
// filterReadyScenarios — diamond dependency
// ---------------------------------------------------------------------------

func TestFilterReadyScenarios_DiamondDependency(t *testing.T) {
	// Diamond: A → B, A → C, B → D, C → D
	// D is blocked until both B and C pass.
	reqs := []workflow.Requirement{
		makeReq("A"),
		makeReq("B", "A"),
		makeReq("C", "A"),
		makeReq("D", "B", "C"),
	}

	t.Run("D blocked when B passing but C still pending", func(t *testing.T) {
		allScenarios := []workflow.Scenario{
			makeScenario("sA", "A", workflow.ScenarioStatusPassing),
			makeScenario("sB", "B", workflow.ScenarioStatusPassing),
			makeScenario("sC", "C", workflow.ScenarioStatusPending), // C not done
			makeScenario("sD", "D", workflow.ScenarioStatusPending),
		}
		refs := []ScenarioRef{makeRef("sC"), makeRef("sD")}

		got := filterReadyScenarios(refs, reqs, allScenarios)
		gotIDs := scenarioIDs(got)

		for _, id := range gotIDs {
			if id == "sD" {
				t.Error("filterReadyScenarios() dispatched sD, but D should be blocked until both B and C pass")
			}
		}
		// sC (r=C, unblocked dep A is passing) should be dispatched.
		found := false
		for _, id := range gotIDs {
			if id == "sC" {
				found = true
			}
		}
		if !found {
			t.Error("filterReadyScenarios() did not dispatch sC, but C's dep (A) is complete")
		}
	})

	t.Run("D dispatched when both B and C pass", func(t *testing.T) {
		allScenarios := []workflow.Scenario{
			makeScenario("sA", "A", workflow.ScenarioStatusPassing),
			makeScenario("sB", "B", workflow.ScenarioStatusPassing),
			makeScenario("sC", "C", workflow.ScenarioStatusPassing),
			makeScenario("sD", "D", workflow.ScenarioStatusPending),
		}
		refs := []ScenarioRef{makeRef("sD")}

		got := filterReadyScenarios(refs, reqs, allScenarios)
		if len(got) != 1 || got[0].ScenarioID != "sD" {
			t.Errorf("filterReadyScenarios() = %v, want [sD] once both B and C are complete", scenarioIDs(got))
		}
	})

	t.Run("D blocked when both B and C are still pending", func(t *testing.T) {
		allScenarios := []workflow.Scenario{
			makeScenario("sA", "A", workflow.ScenarioStatusPassing),
			makeScenario("sB", "B", workflow.ScenarioStatusPending),
			makeScenario("sC", "C", workflow.ScenarioStatusPending),
			makeScenario("sD", "D", workflow.ScenarioStatusPending),
		}
		refs := []ScenarioRef{makeRef("sB"), makeRef("sC"), makeRef("sD")}

		got := filterReadyScenarios(refs, reqs, allScenarios)
		gotIDs := scenarioIDs(got)

		for _, id := range gotIDs {
			if id == "sD" {
				t.Error("filterReadyScenarios() dispatched sD, but D should be blocked until both B and C pass")
			}
		}
		// sB and sC should be dispatched (A is complete).
		found := make(map[string]bool)
		for _, id := range gotIDs {
			found[id] = true
		}
		for _, expected := range []string{"sB", "sC"} {
			if !found[expected] {
				t.Errorf("filterReadyScenarios() did not dispatch %s", expected)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// filterReadyScenarios — no requirements (backward compatibility)
// ---------------------------------------------------------------------------

func TestFilterReadyScenarios_NoRequirements_PassthroughAll(t *testing.T) {
	// When the plan has no requirements, all refs should pass through unchanged
	// to preserve backward compatibility.
	refs := []ScenarioRef{makeRef("s1"), makeRef("s2"), makeRef("s3")}

	got := filterReadyScenarios(refs, nil, nil)
	if len(got) != len(refs) {
		t.Errorf("filterReadyScenarios() with no requirements returned %d refs, want %d", len(got), len(refs))
	}
}

func TestFilterReadyScenarios_EmptyRefs(t *testing.T) {
	reqs := []workflow.Requirement{makeReq("r1")}
	allScenarios := []workflow.Scenario{
		makeScenario("s1", "r1", workflow.ScenarioStatusPending),
	}

	got := filterReadyScenarios(nil, reqs, allScenarios)
	if len(got) != 0 {
		t.Errorf("filterReadyScenarios() with empty refs returned %d, want 0", len(got))
	}
}

func TestFilterReadyScenarios_RefsForUnknownRequirement(t *testing.T) {
	// A ScenarioRef whose scenario does not appear in allScenarios should be
	// silently excluded (not cause a panic or be passed through).
	reqs := []workflow.Requirement{makeReq("r1")}
	allScenarios := []workflow.Scenario{
		makeScenario("s1", "r1", workflow.ScenarioStatusPassing),
	}
	// s-orphan is not in allScenarios and therefore cannot be matched to a requirement.
	refs := []ScenarioRef{makeRef("s-orphan")}

	got := filterReadyScenarios(refs, reqs, allScenarios)
	// Orphaned refs have no owning requirement in allScenarios and are dropped.
	if len(got) != 0 {
		t.Errorf("filterReadyScenarios() = %v, want empty (orphaned ref not in allScenarios)", scenarioIDs(got))
	}
}

// ---------------------------------------------------------------------------
// filterReadyScenarios — multi-scenario requirement (partial completion)
// ---------------------------------------------------------------------------

func TestFilterReadyScenarios_MultiScenarioReq_OnePendingBlocksDependent(t *testing.T) {
	// r1 has two scenarios: one passing, one pending.
	// r2 depends on r1 — it should remain blocked.
	reqs := []workflow.Requirement{
		makeReq("r1"),
		makeReq("r2", "r1"),
	}
	allScenarios := []workflow.Scenario{
		makeScenario("s1a", "r1", workflow.ScenarioStatusPassing),
		makeScenario("s1b", "r1", workflow.ScenarioStatusPending),
		makeScenario("s2", "r2", workflow.ScenarioStatusPending),
	}
	refs := []ScenarioRef{makeRef("s1b"), makeRef("s2")}

	got := filterReadyScenarios(refs, reqs, allScenarios)
	gotIDs := scenarioIDs(got)

	for _, id := range gotIDs {
		if id == "s2" {
			t.Error("filterReadyScenarios() dispatched s2 while r1 still has a pending scenario")
		}
	}
	// s1b (r1 has no deps) should be dispatched so r1 can complete.
	found := false
	for _, id := range gotIDs {
		if id == "s1b" {
			found = true
		}
	}
	if !found {
		t.Error("filterReadyScenarios() did not dispatch s1b; r1 has no deps and should be ready")
	}
}

// ---------------------------------------------------------------------------
// depsComplete
// ---------------------------------------------------------------------------

func TestDepsComplete_NoDeps(t *testing.T) {
	req := makeReq("r1") // no DependsOn
	if !depsComplete(req, map[string]bool{}) {
		t.Error("depsComplete() = false for requirement with no deps, want true")
	}
}

func TestDepsComplete_AllDepsComplete(t *testing.T) {
	req := makeReq("r3", "r1", "r2")
	complete := map[string]bool{"r1": true, "r2": true}
	if !depsComplete(req, complete) {
		t.Error("depsComplete() = false when all deps are complete, want true")
	}
}

func TestDepsComplete_OneDepIncomplete(t *testing.T) {
	req := makeReq("r3", "r1", "r2")
	complete := map[string]bool{"r1": true, "r2": false}
	if depsComplete(req, complete) {
		t.Error("depsComplete() = true when one dep is incomplete, want false")
	}
}

func TestDepsComplete_DepMissingFromMap(t *testing.T) {
	req := makeReq("r2", "r1")
	// r1 not in the map at all — treated as false (zero value).
	if depsComplete(req, map[string]bool{}) {
		t.Error("depsComplete() = true when dep is absent from completion map, want false")
	}
}
