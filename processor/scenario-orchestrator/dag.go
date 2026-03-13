package scenarioorchestrator

import "github.com/c360studio/semspec/workflow"

// requirementComplete returns true when every scenario belonging to req is in a
// terminal-passing state (passing or skipped).
//
// A requirement with no scenarios is considered incomplete because a
// requirement without any verification scenarios cannot be definitively
// satisfied.
func requirementComplete(reqID string, reqScenarios map[string][]workflow.Scenario) bool {
	ss, ok := reqScenarios[reqID]
	if !ok || len(ss) == 0 {
		return false
	}
	for _, s := range ss {
		if s.Status != workflow.ScenarioStatusPassing && s.Status != workflow.ScenarioStatusSkipped {
			return false
		}
	}
	return true
}

// filterReadyScenarios applies requirement-DAG gating to a set of scenario
// references and returns only those whose owning requirement is "ready":
//
//  1. All DependsOn requirements of that requirement are complete
//     (every scenario passing or skipped).
//  2. The scenario itself is in a dispatchable state (pending or dirty via
//     the trigger's ScenarioRef list — the caller is responsible for
//     pre-filtering to dispatchable refs).
//
// Requirements without DependsOn (root requirements) are always unblocked by
// upstream; they are dispatched as long as they have pending/dirty scenarios.
//
// Parameters:
//   - refs: the candidate ScenarioRef list from the OrchestratorTrigger.
//   - requirements: all requirements for the plan.
//   - allScenarios: all scenarios for the plan (used to compute completion).
//
// Returns the subset of refs that should be dispatched.
func filterReadyScenarios(
	refs []ScenarioRef,
	requirements []workflow.Requirement,
	allScenarios []workflow.Scenario,
) []ScenarioRef {
	// Index requirements by ID for O(1) lookup.
	reqIndex := make(map[string]workflow.Requirement, len(requirements))
	for _, r := range requirements {
		reqIndex[r.ID] = r
	}

	// Group all scenarios by their RequirementID.
	reqScenarios := make(map[string][]workflow.Scenario, len(requirements))
	for _, s := range allScenarios {
		reqScenarios[s.RequirementID] = append(reqScenarios[s.RequirementID], s)
	}

	// Pre-compute which requirements are fully complete.
	complete := make(map[string]bool, len(requirements))
	for _, r := range requirements {
		complete[r.ID] = requirementComplete(r.ID, reqScenarios)
	}

	// Index the candidate refs by ScenarioID so we can match against
	// full scenario records efficiently.
	refIndex := make(map[string]ScenarioRef, len(refs))
	for _, ref := range refs {
		refIndex[ref.ScenarioID] = ref
	}

	// For each requirement, determine if all its upstream deps are complete.
	// If so, collect any candidate refs that belong to it.
	var ready []ScenarioRef
	for _, req := range requirements {
		if !depsComplete(req, complete) {
			continue
		}
		// All deps satisfied — include candidate refs belonging to this req.
		for _, s := range reqScenarios[req.ID] {
			if ref, ok := refIndex[s.ID]; ok {
				ready = append(ready, ref)
			}
		}
	}

	// Handle refs whose scenario is not found in allScenarios at all
	// (e.g. the scenario was created after the last load, or the requirement
	// has no record). Fall back to passing them through so they are not
	// silently dropped — this preserves backward-compatible behavior when
	// the plan has no requirements file.
	if len(requirements) == 0 {
		return refs
	}

	return ready
}

// depsComplete returns true when every requirement listed in req.DependsOn is
// present in the complete map and marked true.
func depsComplete(req workflow.Requirement, complete map[string]bool) bool {
	for _, depID := range req.DependsOn {
		if !complete[depID] {
			return false
		}
	}
	return true
}
