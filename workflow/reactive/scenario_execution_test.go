package reactive

import (
	"testing"
	"time"

	"github.com/c360studio/semspec/tools/decompose"
	reactiveEngine "github.com/c360studio/semstreams/processor/reactive"
)

// ---------------------------------------------------------------------------
// Definition-level tests
// ---------------------------------------------------------------------------

func TestScenarioExecutionWorkflow_Definition(t *testing.T) {
	def := BuildScenarioExecutionWorkflow(testStateBucket)

	if def.ID != ScenarioExecutionWorkflowID {
		t.Errorf("expected ID %q, got %q", ScenarioExecutionWorkflowID, def.ID)
	}
	if def.ID != "scenario-execution-loop" {
		t.Errorf("expected literal ID 'scenario-execution-loop', got %q", def.ID)
	}

	expectedRules := []struct {
		id         string
		actionType reactiveEngine.ActionType
	}{
		{"accept-trigger", reactiveEngine.ActionMutate},
		{"dispatch-decompose", reactiveEngine.ActionPublish},
		{"handle-decomposed", reactiveEngine.ActionPublish},
		{"handle-dag-complete", reactiveEngine.ActionMutate},
		{"handle-dag-failed", reactiveEngine.ActionMutate},
		{"handle-complete", reactiveEngine.ActionComplete},
		{"handle-failed", reactiveEngine.ActionComplete},
	}

	if len(def.Rules) != len(expectedRules) {
		t.Fatalf("expected %d rules, got %d", len(expectedRules), len(def.Rules))
	}

	for i, want := range expectedRules {
		rule := def.Rules[i]
		if rule.ID != want.id {
			t.Errorf("rule[%d]: expected ID %q, got %q", i, want.id, rule.ID)
		}
		if rule.Action.Type != want.actionType {
			t.Errorf("rule[%d] %q: expected action type %v, got %v",
				i, want.id, want.actionType, rule.Action.Type)
		}
	}

	if def.StateBucket != testStateBucket {
		t.Errorf("expected state bucket %q, got %q", testStateBucket, def.StateBucket)
	}
}

func TestScenarioExecutionWorkflow_StateFactory(t *testing.T) {
	def := BuildScenarioExecutionWorkflow(testStateBucket)

	got := def.StateFactory()
	if got == nil {
		t.Fatal("StateFactory returned nil")
	}
	_, ok := got.(*ScenarioExecutionState)
	if !ok {
		t.Errorf("StateFactory returned wrong type: %T", got)
	}
}

// ---------------------------------------------------------------------------
// accept-trigger tests
// ---------------------------------------------------------------------------

func TestScenarioExecution_AcceptTrigger_InitializesState(t *testing.T) {
	def := BuildScenarioExecutionWorkflow(testStateBucket)
	rule := findRule(t, def, "accept-trigger")

	trigger := &ScenarioExecutionTriggerPayload{
		ScenarioID: "scenario-001",
		Prompt:     "implement user auth",
		Role:       "developer",
		Model:      "gpt-4",
		TraceID:    "trace-abc",
	}

	state := &ScenarioExecutionState{}
	ctx := &reactiveEngine.RuleContext{State: state, Message: trigger}

	// All conditions must pass.
	for _, cond := range rule.Conditions {
		if !cond.Evaluate(ctx) {
			t.Errorf("condition %q should pass", cond.Description)
		}
	}

	if err := rule.Action.MutateState(ctx, nil); err != nil {
		t.Fatalf("MutateState failed: %v", err)
	}

	if state.ScenarioID != "scenario-001" {
		t.Errorf("expected ScenarioID 'scenario-001', got %q", state.ScenarioID)
	}
	if state.Prompt != "implement user auth" {
		t.Errorf("expected Prompt 'implement user auth', got %q", state.Prompt)
	}
	if state.Role != "developer" {
		t.Errorf("expected Role 'developer', got %q", state.Role)
	}
	if state.Model != "gpt-4" {
		t.Errorf("expected Model 'gpt-4', got %q", state.Model)
	}
	if state.TraceID != "trace-abc" {
		t.Errorf("expected TraceID 'trace-abc', got %q", state.TraceID)
	}
	if state.Phase != ScenarioPhaseDecomposing {
		t.Errorf("expected phase %q, got %q", ScenarioPhaseDecomposing, state.Phase)
	}
	if state.ID != "scenario-execution.scenario-001" {
		t.Errorf("expected ID 'scenario-execution.scenario-001', got %q", state.ID)
	}
	if state.WorkflowID != ScenarioExecutionWorkflowID {
		t.Errorf("expected WorkflowID %q, got %q", ScenarioExecutionWorkflowID, state.WorkflowID)
	}
	if state.Status != reactiveEngine.StatusRunning {
		t.Errorf("expected StatusRunning, got %v", state.Status)
	}
}

func TestScenarioExecution_AcceptTrigger_SecondTriggerPreservesID(t *testing.T) {
	def := BuildScenarioExecutionWorkflow(testStateBucket)
	rule := findRule(t, def, "accept-trigger")

	trigger := &ScenarioExecutionTriggerPayload{
		ScenarioID: "scenario-002",
		Prompt:     "implement payment",
	}

	state := &ScenarioExecutionState{}
	// Pre-populate as if first trigger already ran.
	state.ID = "scenario-execution.scenario-002"
	state.WorkflowID = ScenarioExecutionWorkflowID

	ctx := &reactiveEngine.RuleContext{State: state, Message: trigger}
	if err := rule.Action.MutateState(ctx, nil); err != nil {
		t.Fatalf("MutateState failed: %v", err)
	}

	// ID must not be overwritten on re-trigger.
	if state.ID != "scenario-execution.scenario-002" {
		t.Errorf("ID should be preserved on re-trigger, got %q", state.ID)
	}
}

func TestScenarioExecution_AcceptTrigger_MissingScenarioID(t *testing.T) {
	def := BuildScenarioExecutionWorkflow(testStateBucket)
	rule := findRule(t, def, "accept-trigger")

	trigger := &ScenarioExecutionTriggerPayload{
		ScenarioID: "",
		Prompt:     "some prompt",
	}
	state := &ScenarioExecutionState{}
	ctx := &reactiveEngine.RuleContext{State: state, Message: trigger}

	if err := rule.Action.MutateState(ctx, nil); err == nil {
		t.Fatal("expected error for missing scenario_id")
	}
}

func TestScenarioExecution_AcceptTrigger_MissingPrompt(t *testing.T) {
	def := BuildScenarioExecutionWorkflow(testStateBucket)
	rule := findRule(t, def, "accept-trigger")

	trigger := &ScenarioExecutionTriggerPayload{
		ScenarioID: "scenario-003",
		Prompt:     "",
	}
	state := &ScenarioExecutionState{}
	ctx := &reactiveEngine.RuleContext{State: state, Message: trigger}

	if err := rule.Action.MutateState(ctx, nil); err == nil {
		t.Fatal("expected error for missing prompt")
	}
}

// ---------------------------------------------------------------------------
// dispatch-decompose rule tests
// ---------------------------------------------------------------------------

func TestScenarioExecution_DispatchDecompose_ConditionsMatchDecomposing(t *testing.T) {
	def := BuildScenarioExecutionWorkflow(testStateBucket)
	rule := findRule(t, def, "dispatch-decompose")

	state := scenarioExecDecomposingState("scenario-010")
	ctx := &reactiveEngine.RuleContext{State: state}
	assertAllConditionsPass(t, rule, ctx)
}

func TestScenarioExecution_DispatchDecompose_ConditionsFailOnOtherPhases(t *testing.T) {
	def := BuildScenarioExecutionWorkflow(testStateBucket)
	rule := findRule(t, def, "dispatch-decompose")

	for _, phase := range []string{ScenarioPhaseDecomposed, ScenarioPhaseExecuting, ScenarioPhaseComplete, ScenarioPhaseFailed} {
		state := scenarioExecDecomposingState("scenario-011")
		state.Phase = phase
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	}
}

func TestScenarioExecution_DispatchDecompose_BuildsRequest(t *testing.T) {
	def := BuildScenarioExecutionWorkflow(testStateBucket)
	rule := findRule(t, def, "dispatch-decompose")

	state := scenarioExecDecomposingState("scenario-012")
	state.Role = "developer"
	state.Model = "claude-3"
	state.TraceID = "trace-xyz"

	ctx := &reactiveEngine.RuleContext{State: state}
	payload, err := rule.Action.BuildPayload(ctx)
	if err != nil {
		t.Fatalf("BuildPayload failed: %v", err)
	}

	req, ok := payload.(*ScenarioDecomposeRequest)
	if !ok {
		t.Fatalf("expected *ScenarioDecomposeRequest, got %T", payload)
	}
	if req.ScenarioID != "scenario-012" {
		t.Errorf("expected ScenarioID 'scenario-012', got %q", req.ScenarioID)
	}
	if req.Prompt == "" {
		t.Error("expected non-empty Prompt in decompose request")
	}
	if req.Role != "developer" {
		t.Errorf("expected Role 'developer', got %q", req.Role)
	}
	if req.Model != "claude-3" {
		t.Errorf("expected Model 'claude-3', got %q", req.Model)
	}
	if req.TraceID != "trace-xyz" {
		t.Errorf("expected TraceID 'trace-xyz', got %q", req.TraceID)
	}
	if req.ExecutionID != state.ID {
		t.Errorf("expected ExecutionID %q, got %q", state.ID, req.ExecutionID)
	}
}

// ---------------------------------------------------------------------------
// handle-decomposed mutator tests
// ---------------------------------------------------------------------------

func TestScenarioExecution_HandleDecomposed_ValidDAGTransitionsToExecuting(t *testing.T) {
	def := BuildScenarioExecutionWorkflow(testStateBucket)
	rule := findRule(t, def, "handle-decomposed")

	dag := singleNodeDAG("node-a")
	state := scenarioExecDecomposingState("scenario-020")
	state.Phase = ScenarioPhaseDecomposed

	msg := &ScenarioDecomposedPayload{
		ExecutionID: state.ID,
		ScenarioID:  "scenario-020",
		DAG:         dag,
	}
	ctx := &reactiveEngine.RuleContext{State: state, Message: msg}

	if err := rule.Action.MutateState(ctx, nil); err != nil {
		t.Fatalf("MutateState failed: %v", err)
	}

	if state.Phase != ScenarioPhaseExecuting {
		t.Errorf("expected phase %q, got %q", ScenarioPhaseExecuting, state.Phase)
	}
	if len(state.DecomposedDAG.Nodes) != 1 {
		t.Errorf("expected 1 DAG node, got %d", len(state.DecomposedDAG.Nodes))
	}
	if state.DAGExecutionID == "" {
		t.Error("expected non-empty DAGExecutionID after valid decomposition")
	}
	if state.DAGExecutionID == state.ID {
		t.Error("DAGExecutionID should be a unique ID, not the scenario execution ID")
	}
}

func TestScenarioExecution_HandleDecomposed_InvalidDAGTransitionsToFailed(t *testing.T) {
	def := BuildScenarioExecutionWorkflow(testStateBucket)
	rule := findRule(t, def, "handle-decomposed")

	// Empty DAG is invalid.
	state := scenarioExecDecomposingState("scenario-021")
	state.Phase = ScenarioPhaseDecomposed

	msg := &ScenarioDecomposedPayload{
		ExecutionID: state.ID,
		ScenarioID:  "scenario-021",
		DAG:         decompose.TaskDAG{Nodes: nil},
	}
	ctx := &reactiveEngine.RuleContext{State: state, Message: msg}

	if err := rule.Action.MutateState(ctx, nil); err != nil {
		t.Fatalf("MutateState should not return error; failure is state transition: %v", err)
	}

	if state.Phase != ScenarioPhaseFailed {
		t.Errorf("expected phase %q for invalid DAG, got %q", ScenarioPhaseFailed, state.Phase)
	}
	if state.Error == "" {
		t.Error("expected non-empty Error after invalid DAG")
	}
}

func TestScenarioExecution_HandleDecomposed_CyclicDAGTransitionsToFailed(t *testing.T) {
	def := BuildScenarioExecutionWorkflow(testStateBucket)
	rule := findRule(t, def, "handle-decomposed")

	// Build a cyclic DAG: a→b→a.
	cyclicDAG := decompose.TaskDAG{
		Nodes: []decompose.TaskNode{
			{ID: "a", Prompt: "do a", Role: "developer", DependsOn: []string{"b"}},
			{ID: "b", Prompt: "do b", Role: "developer", DependsOn: []string{"a"}},
		},
	}

	state := scenarioExecDecomposingState("scenario-022")
	state.Phase = ScenarioPhaseDecomposed

	msg := &ScenarioDecomposedPayload{
		ExecutionID: state.ID,
		ScenarioID:  "scenario-022",
		DAG:         cyclicDAG,
	}
	ctx := &reactiveEngine.RuleContext{State: state, Message: msg}

	if err := rule.Action.MutateState(ctx, nil); err != nil {
		t.Fatalf("MutateState should not return error: %v", err)
	}

	if state.Phase != ScenarioPhaseFailed {
		t.Errorf("expected phase %q for cyclic DAG, got %q", ScenarioPhaseFailed, state.Phase)
	}
}

// ---------------------------------------------------------------------------
// handle-dag-complete mutator tests
// ---------------------------------------------------------------------------

func TestScenarioExecution_HandleDAGComplete_TransitionsToComplete(t *testing.T) {
	def := BuildScenarioExecutionWorkflow(testStateBucket)
	rule := findRule(t, def, "handle-dag-complete")

	state := scenarioExecExecutingState("scenario-030")

	msg := &DAGExecutionCompletePayload{
		ExecutionID:    state.DAGExecutionID,
		ScenarioID:     "scenario-030",
		CompletedNodes: []string{"node-a", "node-b"},
	}
	ctx := &reactiveEngine.RuleContext{State: state, Message: msg}

	if err := rule.Action.MutateState(ctx, nil); err != nil {
		t.Fatalf("MutateState failed: %v", err)
	}

	if state.Phase != ScenarioPhaseComplete {
		t.Errorf("expected phase %q, got %q", ScenarioPhaseComplete, state.Phase)
	}
	if len(state.CompletedNodes) != 2 {
		t.Errorf("expected 2 completed nodes, got %d: %v", len(state.CompletedNodes), state.CompletedNodes)
	}
}

func TestScenarioExecution_HandleDAGComplete_PreservesCompletedNodesList(t *testing.T) {
	def := BuildScenarioExecutionWorkflow(testStateBucket)
	rule := findRule(t, def, "handle-dag-complete")

	state := scenarioExecExecutingState("scenario-031")

	msg := &DAGExecutionCompletePayload{
		ExecutionID:    state.DAGExecutionID,
		ScenarioID:     "scenario-031",
		CompletedNodes: []string{"alpha", "beta", "gamma"},
	}
	ctx := &reactiveEngine.RuleContext{State: state, Message: msg}

	if err := rule.Action.MutateState(ctx, nil); err != nil {
		t.Fatalf("MutateState failed: %v", err)
	}

	expected := []string{"alpha", "beta", "gamma"}
	if len(state.CompletedNodes) != len(expected) {
		t.Fatalf("expected %d completed nodes, got %d", len(expected), len(state.CompletedNodes))
	}
	for i, name := range expected {
		if state.CompletedNodes[i] != name {
			t.Errorf("CompletedNodes[%d]: expected %q, got %q", i, name, state.CompletedNodes[i])
		}
	}
}

// ---------------------------------------------------------------------------
// handle-dag-failed mutator tests
// ---------------------------------------------------------------------------

func TestScenarioExecution_HandleDAGFailed_TransitionsToFailed(t *testing.T) {
	def := BuildScenarioExecutionWorkflow(testStateBucket)
	rule := findRule(t, def, "handle-dag-failed")

	state := scenarioExecExecutingState("scenario-040")

	msg := &DAGExecutionFailedPayload{
		ExecutionID: state.DAGExecutionID,
		ScenarioID:  "scenario-040",
		FailedNodes: []string{"node-x"},
	}
	ctx := &reactiveEngine.RuleContext{State: state, Message: msg}

	if err := rule.Action.MutateState(ctx, nil); err != nil {
		t.Fatalf("MutateState failed: %v", err)
	}

	if state.Phase != ScenarioPhaseFailed {
		t.Errorf("expected phase %q, got %q", ScenarioPhaseFailed, state.Phase)
	}
	if len(state.FailedNodes) != 1 || state.FailedNodes[0] != "node-x" {
		t.Errorf("expected FailedNodes=['node-x'], got %v", state.FailedNodes)
	}
	if state.Error == "" {
		t.Error("expected non-empty Error after DAG failure")
	}
}

func TestScenarioExecution_HandleDAGFailed_SetsErrorStatus(t *testing.T) {
	def := BuildScenarioExecutionWorkflow(testStateBucket)
	rule := findRule(t, def, "handle-dag-failed")

	state := scenarioExecExecutingState("scenario-041")

	msg := &DAGExecutionFailedPayload{
		ExecutionID: state.DAGExecutionID,
		ScenarioID:  "scenario-041",
		FailedNodes: []string{"node-a", "node-b"},
	}
	ctx := &reactiveEngine.RuleContext{State: state, Message: msg}

	if err := rule.Action.MutateState(ctx, nil); err != nil {
		t.Fatalf("MutateState failed: %v", err)
	}

	// FailExecution marks status as failed.
	if state.Status == reactiveEngine.StatusRunning {
		t.Error("expected non-running status after DAG failure")
	}
}

// ---------------------------------------------------------------------------
// handle-complete and handle-failed rule condition tests
// ---------------------------------------------------------------------------

func TestScenarioExecution_HandleComplete_ConditionsMatchOnlyComplete(t *testing.T) {
	def := BuildScenarioExecutionWorkflow(testStateBucket)
	rule := findRule(t, def, "handle-complete")

	t.Run("matches complete phase", func(t *testing.T) {
		state := scenarioExecCompleteState("scenario-050")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertAllConditionsPass(t, rule, ctx)
	})

	t.Run("does not match executing phase", func(t *testing.T) {
		state := scenarioExecExecutingState("scenario-051")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("does not match when already terminal", func(t *testing.T) {
		state := scenarioExecCompleteState("scenario-052")
		state.Status = reactiveEngine.StatusCompleted
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})
}

func TestScenarioExecution_HandleFailed_ConditionsMatchOnlyFailed(t *testing.T) {
	def := BuildScenarioExecutionWorkflow(testStateBucket)
	rule := findRule(t, def, "handle-failed")

	t.Run("matches failed phase", func(t *testing.T) {
		state := scenarioExecFailedState("scenario-060")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertAllConditionsPass(t, rule, ctx)
	})

	t.Run("does not match complete phase", func(t *testing.T) {
		state := scenarioExecCompleteState("scenario-061")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})
}

func TestScenarioExecution_HandleComplete_BuildsCompleteEvent(t *testing.T) {
	def := BuildScenarioExecutionWorkflow(testStateBucket)
	rule := findRule(t, def, "handle-complete")

	state := scenarioExecCompleteState("scenario-070")
	state.DAGExecutionID = "exec-100"
	state.CompletedNodes = []string{"a", "b", "c"}

	ctx := &reactiveEngine.RuleContext{State: state}
	payload, err := rule.Action.BuildPayload(ctx)
	if err != nil {
		t.Fatalf("BuildPayload failed: %v", err)
	}

	event, ok := payload.(*ScenarioCompletePayload)
	if !ok {
		t.Fatalf("expected *ScenarioCompletePayload, got %T", payload)
	}
	if event.ScenarioID != "scenario-070" {
		t.Errorf("expected ScenarioID 'scenario-070', got %q", event.ScenarioID)
	}
	if event.DAGExecutionID != "exec-100" {
		t.Errorf("expected DAGExecutionID 'exec-100', got %q", event.DAGExecutionID)
	}
	if len(event.CompletedNodes) != 3 {
		t.Errorf("expected 3 completed nodes, got %d", len(event.CompletedNodes))
	}
}

func TestScenarioExecution_HandleFailed_BuildsFailedEvent(t *testing.T) {
	def := BuildScenarioExecutionWorkflow(testStateBucket)
	rule := findRule(t, def, "handle-failed")

	state := scenarioExecFailedState("scenario-080")
	state.DAGExecutionID = "exec-200"
	state.FailedNodes = []string{"node-x"}
	state.Error = "DAG execution failed: 1 nodes failed"

	ctx := &reactiveEngine.RuleContext{State: state}
	payload, err := rule.Action.BuildPayload(ctx)
	if err != nil {
		t.Fatalf("BuildPayload failed: %v", err)
	}

	event, ok := payload.(*ScenarioFailedPayload)
	if !ok {
		t.Fatalf("expected *ScenarioFailedPayload, got %T", payload)
	}
	if event.ScenarioID != "scenario-080" {
		t.Errorf("expected ScenarioID 'scenario-080', got %q", event.ScenarioID)
	}
	if event.DAGExecutionID != "exec-200" {
		t.Errorf("expected DAGExecutionID 'exec-200', got %q", event.DAGExecutionID)
	}
	if len(event.FailedNodes) != 1 {
		t.Errorf("expected 1 failed node, got %d", len(event.FailedNodes))
	}
	if event.Reason == "" {
		t.Error("expected non-empty Reason in failed event")
	}
}

// ---------------------------------------------------------------------------
// GetExecutionState test
// ---------------------------------------------------------------------------

func TestScenarioExecutionState_GetExecutionState(t *testing.T) {
	state := &ScenarioExecutionState{}
	state.ID = "scenario-execution.scenario-001"
	state.Phase = ScenarioPhaseDecomposing

	es := state.GetExecutionState()
	if es == nil {
		t.Fatal("GetExecutionState returned nil")
	}
	if es.ID != "scenario-execution.scenario-001" {
		t.Errorf("expected ID 'scenario-execution.scenario-001', got %q", es.ID)
	}
	if es.Phase != ScenarioPhaseDecomposing {
		t.Errorf("expected phase %q, got %q", ScenarioPhaseDecomposing, es.Phase)
	}

	// Mutation through the pointer must be reflected in state.
	es.Phase = ScenarioPhaseComplete
	if state.Phase != ScenarioPhaseComplete {
		t.Error("mutation through GetExecutionState() should update state.Phase")
	}
}

// ---------------------------------------------------------------------------
// Payload validation tests
// ---------------------------------------------------------------------------

func TestScenarioExecutionTriggerPayload_Validate(t *testing.T) {
	t.Run("valid payload", func(t *testing.T) {
		p := &ScenarioExecutionTriggerPayload{ScenarioID: "s1", Prompt: "do it"}
		if err := p.Validate(); err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("missing scenario_id", func(t *testing.T) {
		p := &ScenarioExecutionTriggerPayload{Prompt: "do it"}
		if err := p.Validate(); err == nil {
			t.Error("expected error for missing scenario_id")
		}
	})

	t.Run("missing prompt", func(t *testing.T) {
		p := &ScenarioExecutionTriggerPayload{ScenarioID: "s1"}
		if err := p.Validate(); err == nil {
			t.Error("expected error for missing prompt")
		}
	})
}

func TestScenarioDecomposedPayload_Validate(t *testing.T) {
	t.Run("valid payload", func(t *testing.T) {
		p := &ScenarioDecomposedPayload{
			ExecutionID: "exec-1",
			ScenarioID:  "s1",
			DAG:         singleNodeDAG("n1"),
		}
		if err := p.Validate(); err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("missing execution_id", func(t *testing.T) {
		p := &ScenarioDecomposedPayload{ScenarioID: "s1", DAG: singleNodeDAG("n1")}
		if err := p.Validate(); err == nil {
			t.Error("expected error for missing execution_id")
		}
	})

	t.Run("missing scenario_id", func(t *testing.T) {
		p := &ScenarioDecomposedPayload{ExecutionID: "exec-1", DAG: singleNodeDAG("n1")}
		if err := p.Validate(); err == nil {
			t.Error("expected error for missing scenario_id")
		}
	})

	t.Run("empty DAG", func(t *testing.T) {
		p := &ScenarioDecomposedPayload{
			ExecutionID: "exec-1",
			ScenarioID:  "s1",
			DAG:         decompose.TaskDAG{},
		}
		if err := p.Validate(); err == nil {
			t.Error("expected error for empty DAG")
		}
	})
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// scenarioExecDecomposingState builds a ScenarioExecutionState in "decomposing" phase.
func scenarioExecDecomposingState(scenarioID string) *ScenarioExecutionState {
	return &ScenarioExecutionState{
		ExecutionState: reactiveEngine.ExecutionState{
			ID:         "scenario-execution." + scenarioID,
			WorkflowID: ScenarioExecutionWorkflowID,
			Phase:      ScenarioPhaseDecomposing,
			Status:     reactiveEngine.StatusRunning,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
		ScenarioID: scenarioID,
		Prompt:     "implement " + scenarioID,
	}
}

// scenarioExecExecutingState builds a ScenarioExecutionState in "executing" phase.
func scenarioExecExecutingState(scenarioID string) *ScenarioExecutionState {
	state := scenarioExecDecomposingState(scenarioID)
	state.Phase = ScenarioPhaseExecuting
	state.DAGExecutionID = "exec-" + scenarioID
	return state
}

// scenarioExecCompleteState builds a ScenarioExecutionState in "complete" phase.
func scenarioExecCompleteState(scenarioID string) *ScenarioExecutionState {
	state := scenarioExecDecomposingState(scenarioID)
	state.Phase = ScenarioPhaseComplete
	return state
}

// scenarioExecFailedState builds a ScenarioExecutionState in "failed" phase.
func scenarioExecFailedState(scenarioID string) *ScenarioExecutionState {
	state := scenarioExecDecomposingState(scenarioID)
	state.Phase = ScenarioPhaseFailed
	return state
}
