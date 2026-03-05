package reactive

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	workflow "github.com/c360studio/semspec/workflow"
	"github.com/c360studio/semspec/workflow/phases"
	reactiveEngine "github.com/c360studio/semstreams/processor/reactive"
	"github.com/c360studio/semstreams/processor/reactive/testutil"
)

// ---------------------------------------------------------------------------
// Definition-level tests
// ---------------------------------------------------------------------------

func TestTaskExecutionWorkflow_Definition(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)

	if def.ID != "task-execution-loop" {
		t.Errorf("expected ID 'task-execution-loop', got %q", def.ID)
	}

	expectedRules := []struct {
		id         string
		actionType reactiveEngine.ActionType
	}{
		{"accept-trigger", reactiveEngine.ActionMutate},
		{"dispatch-develop", reactiveEngine.ActionPublish}, // PublishWithMutation → ActionPublish
		{"develop-completed", reactiveEngine.ActionMutate},
		{"dispatch-validate", reactiveEngine.ActionPublish}, // PublishWithMutation → ActionPublish
		{"validate-completed", reactiveEngine.ActionMutate},
		{"validation-passed", reactiveEngine.ActionPublish},
		{"validation-failed-retry", reactiveEngine.ActionMutate},
		{"validation-failed-escalate", reactiveEngine.ActionPublish},
		{"dispatch-review", reactiveEngine.ActionPublish}, // PublishWithMutation → ActionPublish
		{"review-completed", reactiveEngine.ActionMutate},
		{"handle-approved", reactiveEngine.ActionComplete},
		{"handle-fixable-retry", reactiveEngine.ActionPublish},
		{"handle-max-retries", reactiveEngine.ActionPublish},
		{"handle-misscoped", reactiveEngine.ActionComplete},
		{"handle-too-big", reactiveEngine.ActionComplete},
		{"handle-unknown-rejection", reactiveEngine.ActionPublish},
		{"handle-error", reactiveEngine.ActionPublish},
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
	if def.MaxIterations != 3 {
		t.Errorf("expected MaxIterations 3, got %d", def.MaxIterations)
	}
}

func TestTaskExecutionWorkflow_StateFactory(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)

	got := def.StateFactory()
	if got == nil {
		t.Fatal("StateFactory returned nil")
	}
	_, ok := got.(*TaskExecutionState)
	if !ok {
		t.Errorf("StateFactory returned wrong type: %T", got)
	}
}

// ---------------------------------------------------------------------------
// accept-trigger rule tests
// ---------------------------------------------------------------------------

func TestTaskExecutionWorkflow_AcceptTrigger(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "accept-trigger")

	state := &TaskExecutionState{}
	trigger := &workflow.TriggerPayload{
		Slug:             "my-project",
		Prompt:           "Implement feature X",
		TaskID:           "task-001",
		Model:            "gpt-4",
		ContextRequestID: "ctx-abc",
	}

	ctx := &reactiveEngine.RuleContext{
		State:   state,
		Message: trigger,
	}

	// Condition should always be true.
	if len(rule.Conditions) == 0 {
		t.Fatal("accept-trigger has no conditions")
	}
	for _, cond := range rule.Conditions {
		if !cond.Evaluate(ctx) {
			t.Errorf("condition %q should be true for accept-trigger", cond.Description)
		}
	}

	// Apply mutator.
	if err := rule.Action.MutateState(ctx, nil); err != nil {
		t.Fatalf("MutateState failed: %v", err)
	}

	if state.Slug != "my-project" {
		t.Errorf("expected Slug 'my-project', got %q", state.Slug)
	}
	if state.TaskID != "task-001" {
		t.Errorf("expected TaskID 'task-001', got %q", state.TaskID)
	}
	if state.Prompt != "Implement feature X" {
		t.Errorf("expected Prompt 'Implement feature X', got %q", state.Prompt)
	}
	if state.Model != "gpt-4" {
		t.Errorf("expected Model 'gpt-4', got %q", state.Model)
	}
	if state.ContextRequestID != "ctx-abc" {
		t.Errorf("expected ContextRequestID 'ctx-abc', got %q", state.ContextRequestID)
	}
	if state.Phase != phases.TaskExecDeveloping {
		t.Errorf("expected phase %q, got %q", phases.TaskExecDeveloping, state.Phase)
	}
	if state.ID == "" {
		t.Error("expected state ID to be populated")
	}
	if !strings.HasPrefix(state.ID, "task-execution.my-project.") {
		t.Errorf("expected ID to start with 'task-execution.my-project.', got %q", state.ID)
	}
	if state.WorkflowID != "task-execution-loop" {
		t.Errorf("expected WorkflowID 'task-execution-loop', got %q", state.WorkflowID)
	}
	if state.Status != reactiveEngine.StatusRunning {
		t.Errorf("expected StatusRunning, got %v", state.Status)
	}
}

func TestTaskExecutionWorkflow_AcceptTrigger_SecondTriggerPreservesID(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "accept-trigger")

	state := &TaskExecutionState{}
	state.ID = "task-execution.existing.task-001"
	state.WorkflowID = "task-execution-loop"

	trigger := &workflow.TriggerPayload{
		Slug:   "existing",
		TaskID: "task-001",
	}
	ctx := &reactiveEngine.RuleContext{State: state, Message: trigger}

	if err := rule.Action.MutateState(ctx, nil); err != nil {
		t.Fatalf("MutateState failed: %v", err)
	}

	if state.ID != "task-execution.existing.task-001" {
		t.Errorf("ID should be preserved, got %q", state.ID)
	}
}

// ---------------------------------------------------------------------------
// dispatch-develop rule tests
// ---------------------------------------------------------------------------

func TestTaskExecutionWorkflow_DevelopConditions(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "dispatch-develop")

	t.Run("matches developing phase", func(t *testing.T) {
		state := taskExecDevelopingState("proj", "t1")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertAllConditionsPass(t, rule, ctx)
	})

	t.Run("does not match validating phase", func(t *testing.T) {
		state := taskExecDevelopingState("proj", "t1")
		state.Phase = phases.TaskExecValidating
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("does not match developing_dispatched phase", func(t *testing.T) {
		state := taskExecDevelopingState("proj", "t1")
		state.Phase = phases.TaskExecDevelopingDispatched
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})
}

func TestTaskExecutionWorkflow_DeveloperPayload_FirstAttempt(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "dispatch-develop")

	state := taskExecDevelopingState("proj", "t1")
	state.Prompt = "Implement the login handler"
	state.Model = "claude-3"
	state.ContextRequestID = "ctx-123"
	ctx := &reactiveEngine.RuleContext{State: state}

	payload, err := rule.Action.BuildPayload(ctx)
	if err != nil {
		t.Fatalf("BuildPayload failed: %v", err)
	}
	req, ok := payload.(*DeveloperRequest)
	if !ok {
		t.Fatalf("expected *DeveloperRequest, got %T", payload)
	}
	if req.Slug != "proj" {
		t.Errorf("expected Slug 'proj', got %q", req.Slug)
	}
	if req.DeveloperTaskID != "t1" {
		t.Errorf("expected DeveloperTaskID 't1', got %q", req.DeveloperTaskID)
	}
	if req.Prompt != "Implement the login handler" {
		t.Errorf("expected Prompt 'Implement the login handler', got %q", req.Prompt)
	}
	if req.Model != "claude-3" {
		t.Errorf("expected Model 'claude-3', got %q", req.Model)
	}
	if req.ContextRequestID != "ctx-123" {
		t.Errorf("expected ContextRequestID 'ctx-123', got %q", req.ContextRequestID)
	}
	if req.Revision {
		t.Error("expected Revision to be false on first iteration")
	}
}

func TestTaskExecutionWorkflow_DeveloperPayload_ValidationRevision(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "dispatch-develop")

	state := taskExecDevelopingState("proj", "t1")
	state.Prompt = "Implement the login handler"
	state.DeveloperOutput = json.RawMessage(`"function login() { return true; }"`)
	state.Iteration = 1
	state.RevisionSource = "validation"
	state.CheckResults = json.RawMessage(`[{"check":"compile","passed":false,"message":"undefined: foo"}]`)
	ctx := &reactiveEngine.RuleContext{State: state}

	payload, err := rule.Action.BuildPayload(ctx)
	if err != nil {
		t.Fatalf("BuildPayload failed: %v", err)
	}
	req, ok := payload.(*DeveloperRequest)
	if !ok {
		t.Fatalf("expected *DeveloperRequest, got %T", payload)
	}
	if !req.Revision {
		t.Error("expected Revision to be true on second iteration")
	}
	// Check that the prompt includes the original task
	if !strings.Contains(req.Prompt, "# Original Task") {
		t.Errorf("expected revision prompt to contain original task section, got: %q", req.Prompt)
	}
	if !strings.Contains(req.Prompt, "Implement the login handler") {
		t.Errorf("expected revision prompt to contain original prompt, got: %q", req.Prompt)
	}
	// Check that the prompt includes the previous response
	if !strings.Contains(req.Prompt, "# Your Previous Response") {
		t.Errorf("expected revision prompt to contain previous response section, got: %q", req.Prompt)
	}
	if !strings.Contains(req.Prompt, "function login()") {
		t.Errorf("expected revision prompt to contain previous developer output, got: %q", req.Prompt)
	}
	// Check that it mentions structural validation failure
	if !strings.Contains(req.Prompt, "Structural Validation Failed") {
		t.Errorf("expected revision prompt to mention structural validation, got: %q", req.Prompt)
	}
	if !strings.Contains(req.Prompt, "undefined: foo") {
		t.Errorf("expected revision prompt to contain check results, got: %q", req.Prompt)
	}
}

func TestTaskExecutionWorkflow_DeveloperPayload_ReviewRevision(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "dispatch-develop")

	state := taskExecDevelopingState("proj", "t1")
	state.Prompt = "Implement the user service"
	state.DeveloperOutput = json.RawMessage(`"class UserService { save() {} }"`)
	state.FilesModified = []string{"services/user.go"}
	state.Iteration = 1
	state.RevisionSource = "review"
	state.Feedback = "Missing error handling in the service layer"
	ctx := &reactiveEngine.RuleContext{State: state}

	payload, err := rule.Action.BuildPayload(ctx)
	if err != nil {
		t.Fatalf("BuildPayload failed: %v", err)
	}
	req, ok := payload.(*DeveloperRequest)
	if !ok {
		t.Fatalf("expected *DeveloperRequest, got %T", payload)
	}
	if !req.Revision {
		t.Error("expected Revision to be true on second iteration")
	}
	// Check that the prompt includes the original task
	if !strings.Contains(req.Prompt, "# Original Task") {
		t.Errorf("expected revision prompt to contain original task section, got: %q", req.Prompt)
	}
	if !strings.Contains(req.Prompt, "Implement the user service") {
		t.Errorf("expected revision prompt to contain original prompt, got: %q", req.Prompt)
	}
	// Check that the prompt includes the previous response
	if !strings.Contains(req.Prompt, "# Your Previous Response") {
		t.Errorf("expected revision prompt to contain previous response section, got: %q", req.Prompt)
	}
	if !strings.Contains(req.Prompt, "class UserService") {
		t.Errorf("expected revision prompt to contain previous developer output, got: %q", req.Prompt)
	}
	// Check that files modified are included
	if !strings.Contains(req.Prompt, "# Files Modified") {
		t.Errorf("expected revision prompt to contain files modified section, got: %q", req.Prompt)
	}
	if !strings.Contains(req.Prompt, "services/user.go") {
		t.Errorf("expected revision prompt to list modified files, got: %q", req.Prompt)
	}
	// Check that it mentions code review rejection
	if !strings.Contains(req.Prompt, "Code Review Rejection") {
		t.Errorf("expected revision prompt to mention code review rejection, got: %q", req.Prompt)
	}
	if !strings.Contains(req.Prompt, "Missing error handling in the service layer") {
		t.Errorf("expected revision prompt to contain feedback, got: %q", req.Prompt)
	}
}

func TestTaskExecutionWorkflow_DispatchDevelopMutation(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "dispatch-develop")

	t.Run("sets phase to developing_dispatched", func(t *testing.T) {
		state := taskExecDevelopingState("proj", "t1")
		ctx := &reactiveEngine.RuleContext{State: state}

		if err := rule.Action.MutateState(ctx, nil); err != nil {
			t.Fatalf("MutateState failed: %v", err)
		}
		if state.Phase != phases.TaskExecDevelopingDispatched {
			t.Errorf("expected phase %q, got %q", phases.TaskExecDevelopingDispatched, state.Phase)
		}
	})
}

func TestTaskExecutionWorkflow_DevelopCompletedConditions(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "develop-completed")

	t.Run("matches developed phase", func(t *testing.T) {
		state := taskExecDevelopingState("proj", "t1")
		state.Phase = phases.TaskExecDeveloped
		ctx := &reactiveEngine.RuleContext{State: state}
		assertAllConditionsPass(t, rule, ctx)
	})

	t.Run("does not match developing phase", func(t *testing.T) {
		state := taskExecDevelopingState("proj", "t1")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("does not match developing_dispatched phase", func(t *testing.T) {
		state := taskExecDevelopingState("proj", "t1")
		state.Phase = phases.TaskExecDevelopingDispatched
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})
}

func TestTaskExecutionWorkflow_DevelopCompletedMutation(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "develop-completed")

	t.Run("transitions to validating phase", func(t *testing.T) {
		state := taskExecDevelopingState("proj", "t1")
		state.Phase = phases.TaskExecDeveloped
		ctx := &reactiveEngine.RuleContext{State: state}

		if err := rule.Action.MutateState(ctx, nil); err != nil {
			t.Fatalf("MutateState failed: %v", err)
		}
		if state.Phase != phases.TaskExecValidating {
			t.Errorf("expected phase %q, got %q", phases.TaskExecValidating, state.Phase)
		}
	})
}

// ---------------------------------------------------------------------------
// dispatch-validate rule tests
// ---------------------------------------------------------------------------

func TestTaskExecutionWorkflow_ValidateConditions(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "dispatch-validate")

	t.Run("matches validating phase", func(t *testing.T) {
		state := taskExecValidatingState("proj", "t1")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertAllConditionsPass(t, rule, ctx)
	})

	t.Run("does not match developing phase", func(t *testing.T) {
		state := taskExecValidatingState("proj", "t1")
		state.Phase = phases.TaskExecDeveloping
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("does not match validating_dispatched phase", func(t *testing.T) {
		state := taskExecValidatingState("proj", "t1")
		state.Phase = phases.TaskExecValidatingDispatched
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})
}

func TestTaskExecutionWorkflow_ValidationPayload(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "dispatch-validate")

	state := taskExecValidatingState("proj", "t1")
	state.FilesModified = []string{"main.go", "service.go"}
	ctx := &reactiveEngine.RuleContext{State: state}

	payload, err := rule.Action.BuildPayload(ctx)
	if err != nil {
		t.Fatalf("BuildPayload failed: %v", err)
	}
	req, ok := payload.(*ValidationRequest)
	if !ok {
		t.Fatalf("expected *ValidationRequest, got %T", payload)
	}
	if req.Slug != "proj" {
		t.Errorf("expected Slug 'proj', got %q", req.Slug)
	}
	if len(req.FilesModified) != 2 || req.FilesModified[0] != "main.go" {
		t.Errorf("expected FilesModified ['main.go','service.go'], got %v", req.FilesModified)
	}
}

func TestTaskExecutionWorkflow_DispatchValidateMutation(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "dispatch-validate")

	t.Run("sets phase to validating_dispatched", func(t *testing.T) {
		state := taskExecValidatingState("proj", "t1")
		ctx := &reactiveEngine.RuleContext{State: state}

		if err := rule.Action.MutateState(ctx, nil); err != nil {
			t.Fatalf("MutateState failed: %v", err)
		}
		if state.Phase != phases.TaskExecValidatingDispatched {
			t.Errorf("expected phase %q, got %q", phases.TaskExecValidatingDispatched, state.Phase)
		}
	})
}

func TestTaskExecutionWorkflow_ValidateCompletedConditions(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "validate-completed")

	t.Run("matches validated phase", func(t *testing.T) {
		state := taskExecValidatingState("proj", "t1")
		state.Phase = phases.TaskExecValidated
		ctx := &reactiveEngine.RuleContext{State: state}
		assertAllConditionsPass(t, rule, ctx)
	})

	t.Run("does not match validating phase", func(t *testing.T) {
		state := taskExecValidatingState("proj", "t1")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("does not match validating_dispatched phase", func(t *testing.T) {
		state := taskExecValidatingState("proj", "t1")
		state.Phase = phases.TaskExecValidatingDispatched
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})
}

func TestTaskExecutionWorkflow_ValidateCompletedMutation(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "validate-completed")

	t.Run("transitions to validation_checked phase", func(t *testing.T) {
		state := taskExecValidatingState("proj", "t1")
		state.Phase = phases.TaskExecValidated
		ctx := &reactiveEngine.RuleContext{State: state}

		if err := rule.Action.MutateState(ctx, nil); err != nil {
			t.Fatalf("MutateState failed: %v", err)
		}
		if state.Phase != phases.TaskExecValidationChecked {
			t.Errorf("expected phase %q, got %q", phases.TaskExecValidationChecked, state.Phase)
		}
	})
}

// ---------------------------------------------------------------------------
// validation-passed rule tests
// ---------------------------------------------------------------------------

func TestTaskExecutionWorkflow_ValidationPassed(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "validation-passed")

	t.Run("conditions pass when validation checked and passed=true", func(t *testing.T) {
		state := taskExecValidationCheckedState("proj", "t1", true)
		ctx := &reactiveEngine.RuleContext{State: state}
		assertAllConditionsPass(t, rule, ctx)
	})

	t.Run("conditions fail when validation failed", func(t *testing.T) {
		state := taskExecValidationCheckedState("proj", "t1", false)
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("conditions fail for wrong phase", func(t *testing.T) {
		state := taskExecValidationCheckedState("proj", "t1", true)
		state.Phase = phases.TaskExecValidating
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("builds validation-passed event", func(t *testing.T) {
		state := taskExecValidationCheckedState("proj", "t1", true)
		state.ChecksRun = 7
		ctx := &reactiveEngine.RuleContext{State: state}

		payload, err := rule.Action.BuildPayload(ctx)
		if err != nil {
			t.Fatalf("BuildPayload failed: %v", err)
		}
		vp, ok := payload.(*TaskValidationPassedPayload)
		if !ok {
			t.Fatalf("expected *TaskValidationPassedPayload, got %T", payload)
		}
		if vp.TaskID != "t1" {
			t.Errorf("expected TaskID 't1', got %q", vp.TaskID)
		}
		if vp.ChecksRun != 7 {
			t.Errorf("expected ChecksRun 7, got %d", vp.ChecksRun)
		}
	})

	t.Run("mutation transitions to reviewing", func(t *testing.T) {
		state := taskExecValidationCheckedState("proj", "t1", true)
		ctx := &reactiveEngine.RuleContext{State: state}

		if err := rule.Action.MutateState(ctx, nil); err != nil {
			t.Fatalf("MutateState failed: %v", err)
		}
		if state.Phase != phases.TaskExecReviewing {
			t.Errorf("expected phase %q, got %q", phases.TaskExecReviewing, state.Phase)
		}
	})
}

// ---------------------------------------------------------------------------
// validation-failed-retry rule tests
// ---------------------------------------------------------------------------

func TestTaskExecutionWorkflow_ValidationFailedRetry(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "validation-failed-retry")

	t.Run("conditions pass when failed and under retry limit", func(t *testing.T) {
		state := taskExecValidationCheckedState("proj", "t1", false)
		state.Iteration = 0
		ctx := &reactiveEngine.RuleContext{State: state}
		assertAllConditionsPass(t, rule, ctx)
	})

	t.Run("conditions fail when passed=true", func(t *testing.T) {
		state := taskExecValidationCheckedState("proj", "t1", true)
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("conditions fail at max iterations", func(t *testing.T) {
		state := taskExecValidationCheckedState("proj", "t1", false)
		state.Iteration = 3
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("mutation increments iteration, sets revision_source=validation, phase=developing", func(t *testing.T) {
		state := taskExecValidationCheckedState("proj", "t1", false)
		state.Iteration = 0
		ctx := &reactiveEngine.RuleContext{State: state}

		if err := rule.Action.MutateState(ctx, nil); err != nil {
			t.Fatalf("MutateState failed: %v", err)
		}
		if state.Phase != phases.TaskExecDeveloping {
			t.Errorf("expected phase %q, got %q", phases.TaskExecDeveloping, state.Phase)
		}
		if state.Iteration != 1 {
			t.Errorf("expected Iteration 1, got %d", state.Iteration)
		}
		if state.RevisionSource != "validation" {
			t.Errorf("expected RevisionSource 'validation', got %q", state.RevisionSource)
		}
	})
}

// ---------------------------------------------------------------------------
// validation-failed-escalate rule tests
// ---------------------------------------------------------------------------

func TestTaskExecutionWorkflow_ValidationFailedEscalate(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "validation-failed-escalate")

	t.Run("conditions pass when failed at max iterations", func(t *testing.T) {
		state := taskExecValidationCheckedState("proj", "t1", false)
		state.Iteration = 3
		ctx := &reactiveEngine.RuleContext{State: state}
		assertAllConditionsPass(t, rule, ctx)
	})

	t.Run("conditions fail when iteration is under max", func(t *testing.T) {
		state := taskExecValidationCheckedState("proj", "t1", false)
		state.Iteration = 2
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("conditions fail when validation passed", func(t *testing.T) {
		state := taskExecValidationCheckedState("proj", "t1", true)
		state.Iteration = 3
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("conditions fail when already completed", func(t *testing.T) {
		state := taskExecValidationCheckedState("proj", "t1", false)
		state.Iteration = 3
		state.Status = reactiveEngine.StatusCompleted
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("conditions fail when already escalated", func(t *testing.T) {
		state := taskExecValidationCheckedState("proj", "t1", false)
		state.Iteration = 3
		state.Status = reactiveEngine.StatusEscalated
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("builds escalation event", func(t *testing.T) {
		state := taskExecValidationCheckedState("proj", "t1", false)
		state.Iteration = 3
		ctx := &reactiveEngine.RuleContext{State: state}

		payload, err := rule.Action.BuildPayload(ctx)
		if err != nil {
			t.Fatalf("BuildPayload failed: %v", err)
		}
		esc, ok := payload.(*TaskExecEscalatePayload)
		if !ok {
			t.Fatalf("expected *TaskExecEscalatePayload, got %T", payload)
		}
		if esc.Slug != "proj" {
			t.Errorf("expected Slug 'proj', got %q", esc.Slug)
		}
		if esc.TaskID != "t1" {
			t.Errorf("expected TaskID 't1', got %q", esc.TaskID)
		}
		if esc.Reason == "" {
			t.Error("expected non-empty Reason")
		}
	})

	t.Run("mutation marks execution as escalated", func(t *testing.T) {
		state := taskExecValidationCheckedState("proj", "t1", false)
		state.Iteration = 3
		ctx := &reactiveEngine.RuleContext{State: state}

		if err := rule.Action.MutateState(ctx, nil); err != nil {
			t.Fatalf("MutateState failed: %v", err)
		}
		if state.Status != reactiveEngine.StatusEscalated {
			t.Errorf("expected StatusEscalated, got %v", state.Status)
		}
	})
}

// ---------------------------------------------------------------------------
// dispatch-review rule tests
// ---------------------------------------------------------------------------

func TestTaskExecutionWorkflow_ReviewConditions(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "dispatch-review")

	t.Run("matches reviewing phase", func(t *testing.T) {
		state := taskExecReviewingState("proj", "t1")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertAllConditionsPass(t, rule, ctx)
	})

	t.Run("does not match developing phase", func(t *testing.T) {
		state := taskExecReviewingState("proj", "t1")
		state.Phase = phases.TaskExecDeveloping
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("does not match reviewing_dispatched phase", func(t *testing.T) {
		state := taskExecReviewingState("proj", "t1")
		state.Phase = phases.TaskExecReviewingDispatched
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})
}

func TestTaskExecutionWorkflow_ReviewPayload(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "dispatch-review")

	state := taskExecReviewingState("proj", "t1")
	state.DeveloperOutput = json.RawMessage(`{"files_written":3}`)
	ctx := &reactiveEngine.RuleContext{State: state}

	payload, err := rule.Action.BuildPayload(ctx)
	if err != nil {
		t.Fatalf("BuildPayload failed: %v", err)
	}
	req, ok := payload.(*TaskCodeReviewRequest)
	if !ok {
		t.Fatalf("expected *TaskCodeReviewRequest, got %T", payload)
	}
	if req.Slug != "proj" {
		t.Errorf("expected Slug 'proj', got %q", req.Slug)
	}
	if req.DeveloperTask != "t1" {
		t.Errorf("expected DeveloperTask 't1', got %q", req.DeveloperTask)
	}
	if string(req.Output) != `{"files_written":3}` {
		t.Errorf("unexpected Output: %s", req.Output)
	}
}

func TestTaskExecutionWorkflow_DispatchReviewMutation(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "dispatch-review")

	t.Run("sets phase to reviewing_dispatched", func(t *testing.T) {
		state := taskExecReviewingState("proj", "t1")
		ctx := &reactiveEngine.RuleContext{State: state}

		if err := rule.Action.MutateState(ctx, nil); err != nil {
			t.Fatalf("MutateState failed: %v", err)
		}
		if state.Phase != phases.TaskExecReviewingDispatched {
			t.Errorf("expected phase %q, got %q", phases.TaskExecReviewingDispatched, state.Phase)
		}
	})
}

func TestTaskExecutionWorkflow_ReviewCompletedConditions(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "review-completed")

	t.Run("matches reviewed phase", func(t *testing.T) {
		state := taskExecReviewingState("proj", "t1")
		state.Phase = phases.TaskExecReviewed
		ctx := &reactiveEngine.RuleContext{State: state}
		assertAllConditionsPass(t, rule, ctx)
	})

	t.Run("does not match reviewing phase", func(t *testing.T) {
		state := taskExecReviewingState("proj", "t1")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("does not match reviewing_dispatched phase", func(t *testing.T) {
		state := taskExecReviewingState("proj", "t1")
		state.Phase = phases.TaskExecReviewingDispatched
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})
}

func TestTaskExecutionWorkflow_ReviewCompletedMutation(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "review-completed")

	t.Run("transitions to evaluated phase", func(t *testing.T) {
		state := taskExecReviewingState("proj", "t1")
		state.Phase = phases.TaskExecReviewed
		ctx := &reactiveEngine.RuleContext{State: state}

		if err := rule.Action.MutateState(ctx, nil); err != nil {
			t.Fatalf("MutateState failed: %v", err)
		}
		if state.Phase != phases.TaskExecEvaluated {
			t.Errorf("expected phase %q, got %q", phases.TaskExecEvaluated, state.Phase)
		}
	})
}

// ---------------------------------------------------------------------------
// handle-approved rule tests
// ---------------------------------------------------------------------------

func TestTaskExecutionWorkflow_HandleApproved(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "handle-approved")

	t.Run("conditions pass for approved evaluated state", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "approved", "")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertAllConditionsPass(t, rule, ctx)
	})

	t.Run("conditions fail for non-approved verdict", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "rejected", "fixable")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("conditions fail for non-evaluated phase", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "approved", "")
		state.Phase = phases.TaskExecReviewing
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("builds correct complete event", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "approved", "")
		state.Iteration = 2
		ctx := &reactiveEngine.RuleContext{State: state}

		payload, err := rule.Action.BuildPayload(ctx)
		if err != nil {
			t.Fatalf("BuildPayload failed: %v", err)
		}
		complete, ok := payload.(*TaskCompletePayload)
		if !ok {
			t.Fatalf("expected *TaskCompletePayload, got %T", payload)
		}
		if complete.TaskID != "t1" {
			t.Errorf("expected TaskID 't1', got %q", complete.TaskID)
		}
		if complete.Iterations != 2 {
			t.Errorf("expected Iterations 2, got %d", complete.Iterations)
		}
	})
}

// ---------------------------------------------------------------------------
// handle-fixable-retry rule tests
// ---------------------------------------------------------------------------

func TestTaskExecutionWorkflow_HandleFixableRetry(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "handle-fixable-retry")

	t.Run("conditions pass for fixable rejection under retry limit", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "rejected", "fixable")
		state.Iteration = 0
		ctx := &reactiveEngine.RuleContext{State: state}
		assertAllConditionsPass(t, rule, ctx)
	})

	t.Run("conditions fail for approved verdict", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "approved", "")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("conditions fail for non-fixable rejection type", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "rejected", "misscoped")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("conditions fail at max iterations", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "rejected", "fixable")
		state.Iteration = 3
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("builds rejection-categorized event", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "rejected", "fixable")
		ctx := &reactiveEngine.RuleContext{State: state}

		payload, err := rule.Action.BuildPayload(ctx)
		if err != nil {
			t.Fatalf("BuildPayload failed: %v", err)
		}
		rej, ok := payload.(*TaskRejectionCategorizedPayload)
		if !ok {
			t.Fatalf("expected *TaskRejectionCategorizedPayload, got %T", payload)
		}
		if rej.Type != "fixable" {
			t.Errorf("expected Type 'fixable', got %q", rej.Type)
		}
	})

	t.Run("mutation increments iteration, sets revision_source=review, phase=developing", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "rejected", "fixable")
		state.Feedback = "Fix the null pointer"
		state.Iteration = 0
		ctx := &reactiveEngine.RuleContext{State: state}

		if err := rule.Action.MutateState(ctx, nil); err != nil {
			t.Fatalf("MutateState failed: %v", err)
		}
		if state.Phase != phases.TaskExecDeveloping {
			t.Errorf("expected phase %q, got %q", phases.TaskExecDeveloping, state.Phase)
		}
		if state.Iteration != 1 {
			t.Errorf("expected Iteration 1, got %d", state.Iteration)
		}
		if state.RevisionSource != "review" {
			t.Errorf("expected RevisionSource 'review', got %q", state.RevisionSource)
		}
		if state.Verdict != "" {
			t.Errorf("expected Verdict to be cleared, got %q", state.Verdict)
		}
	})
}

// ---------------------------------------------------------------------------
// handle-max-retries rule tests
// ---------------------------------------------------------------------------

func TestTaskExecutionWorkflow_HandleMaxRetries(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "handle-max-retries")

	t.Run("conditions pass for fixable rejection at max iterations", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "rejected", "fixable")
		state.Iteration = 3
		ctx := &reactiveEngine.RuleContext{State: state}
		assertAllConditionsPass(t, rule, ctx)
	})

	t.Run("conditions fail when iteration is under max", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "rejected", "fixable")
		state.Iteration = 2
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("builds max-retries escalation event with feedback", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "rejected", "fixable")
		state.Iteration = 3
		state.Feedback = "Still missing error handling"
		ctx := &reactiveEngine.RuleContext{State: state}

		payload, err := rule.Action.BuildPayload(ctx)
		if err != nil {
			t.Fatalf("BuildPayload failed: %v", err)
		}
		esc, ok := payload.(*TaskExecEscalatePayload)
		if !ok {
			t.Fatalf("expected *TaskExecEscalatePayload, got %T", payload)
		}
		if esc.TaskID != "t1" {
			t.Errorf("expected TaskID 't1', got %q", esc.TaskID)
		}
		if !strings.Contains(esc.Reason, "max reviewer retries") {
			t.Errorf("expected Reason to mention max reviewer retries, got %q", esc.Reason)
		}
		if esc.LastFeedback != "Still missing error handling" {
			t.Errorf("expected LastFeedback 'Still missing error handling', got %q", esc.LastFeedback)
		}
	})

	t.Run("mutation marks execution as escalated", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "rejected", "fixable")
		state.Iteration = 3
		ctx := &reactiveEngine.RuleContext{State: state}

		if err := rule.Action.MutateState(ctx, nil); err != nil {
			t.Fatalf("MutateState failed: %v", err)
		}
		if state.Status != reactiveEngine.StatusEscalated {
			t.Errorf("expected StatusEscalated, got %v", state.Status)
		}
	})
}

// ---------------------------------------------------------------------------
// handle-misscoped rule tests
// ---------------------------------------------------------------------------

func TestTaskExecutionWorkflow_HandleMisscoped(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "handle-misscoped")

	t.Run("conditions pass for misscoped rejection", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "rejected", "misscoped")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertAllConditionsPass(t, rule, ctx)
	})

	t.Run("conditions pass for architectural rejection", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "rejected", "architectural")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertAllConditionsPass(t, rule, ctx)
	})

	t.Run("conditions fail for fixable rejection", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "rejected", "fixable")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("builds plan-refinement trigger", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "rejected", "misscoped")
		state.Feedback = "Task requires changes to the database schema"
		ctx := &reactiveEngine.RuleContext{State: state}

		payload, err := rule.Action.BuildPayload(ctx)
		if err != nil {
			t.Fatalf("BuildPayload failed: %v", err)
		}
		trig, ok := payload.(*PlanRefinementTriggerPayload)
		if !ok {
			t.Fatalf("expected *PlanRefinementTriggerPayload, got %T", payload)
		}
		if trig.OriginalTaskID != "t1" {
			t.Errorf("expected OriginalTaskID 't1', got %q", trig.OriginalTaskID)
		}
		if trig.PlanSlug != "proj" {
			t.Errorf("expected PlanSlug 'proj', got %q", trig.PlanSlug)
		}
		if trig.RejectionType != "misscoped" {
			t.Errorf("expected RejectionType 'misscoped', got %q", trig.RejectionType)
		}
		if trig.Feedback != "Task requires changes to the database schema" {
			t.Errorf("expected Feedback, got %q", trig.Feedback)
		}
	})
}

// ---------------------------------------------------------------------------
// handle-too-big rule tests
// ---------------------------------------------------------------------------

func TestTaskExecutionWorkflow_HandleTooBig(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "handle-too-big")

	t.Run("conditions pass for too_big rejection", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "rejected", "too_big")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertAllConditionsPass(t, rule, ctx)
	})

	t.Run("conditions fail for misscoped rejection", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "rejected", "misscoped")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("builds task-decomposition trigger", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "rejected", "too_big")
		state.Feedback = "Split into auth and profile sub-tasks"
		ctx := &reactiveEngine.RuleContext{State: state}

		payload, err := rule.Action.BuildPayload(ctx)
		if err != nil {
			t.Fatalf("BuildPayload failed: %v", err)
		}
		trig, ok := payload.(*TaskDecompositionTriggerPayload)
		if !ok {
			t.Fatalf("expected *TaskDecompositionTriggerPayload, got %T", payload)
		}
		if trig.OriginalTaskID != "t1" {
			t.Errorf("expected OriginalTaskID 't1', got %q", trig.OriginalTaskID)
		}
		if trig.PlanSlug != "proj" {
			t.Errorf("expected PlanSlug 'proj', got %q", trig.PlanSlug)
		}
		if trig.Feedback != "Split into auth and profile sub-tasks" {
			t.Errorf("expected Feedback, got %q", trig.Feedback)
		}
	})
}

// ---------------------------------------------------------------------------
// handle-unknown-rejection rule tests
// ---------------------------------------------------------------------------

func TestTaskExecutionWorkflow_HandleUnknownRejection(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "handle-unknown-rejection")

	t.Run("conditions pass for unknown rejection type", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "rejected", "unexpected_type")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertAllConditionsPass(t, rule, ctx)
	})

	t.Run("conditions fail for fixable rejection", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "rejected", "fixable")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("conditions fail for misscoped rejection", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "rejected", "misscoped")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("conditions fail for architectural rejection", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "rejected", "architectural")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("conditions fail for too_big rejection", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "rejected", "too_big")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("builds escalation event mentioning unknown type", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "rejected", "unexpected_type")
		ctx := &reactiveEngine.RuleContext{State: state}

		payload, err := rule.Action.BuildPayload(ctx)
		if err != nil {
			t.Fatalf("BuildPayload failed: %v", err)
		}
		esc, ok := payload.(*TaskExecEscalatePayload)
		if !ok {
			t.Fatalf("expected *TaskExecEscalatePayload, got %T", payload)
		}
		if !strings.Contains(esc.Reason, "unexpected_type") {
			t.Errorf("expected Reason to contain rejection type, got %q", esc.Reason)
		}
	})

	t.Run("mutation marks execution as escalated", func(t *testing.T) {
		state := taskExecEvaluatedState("proj", "t1", "rejected", "unexpected_type")
		ctx := &reactiveEngine.RuleContext{State: state}

		if err := rule.Action.MutateState(ctx, nil); err != nil {
			t.Fatalf("MutateState failed: %v", err)
		}
		if state.Status != reactiveEngine.StatusEscalated {
			t.Errorf("expected StatusEscalated, got %v", state.Status)
		}
	})
}

// ---------------------------------------------------------------------------
// handle-error rule tests
// ---------------------------------------------------------------------------

func TestTaskExecutionWorkflow_HandleError(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "handle-error")

	t.Run("conditions pass for developer_failed phase", func(t *testing.T) {
		state := taskExecFailedState("proj", "t1", phases.TaskExecDeveloperFailed, "developer crashed")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertAllConditionsPass(t, rule, ctx)
	})

	t.Run("conditions pass for reviewer_failed phase", func(t *testing.T) {
		state := taskExecFailedState("proj", "t1", phases.TaskExecReviewerFailed, "reviewer timed out")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertAllConditionsPass(t, rule, ctx)
	})

	t.Run("conditions pass for validation_error phase", func(t *testing.T) {
		state := taskExecFailedState("proj", "t1", phases.TaskExecValidationError, "validator crashed")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertAllConditionsPass(t, rule, ctx)
	})

	t.Run("conditions fail for non-failure phase", func(t *testing.T) {
		state := taskExecDevelopingState("proj", "t1")
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, rule, ctx)
	})

	t.Run("builds correct error event with slug and task_id", func(t *testing.T) {
		state := taskExecFailedState("proj", "t1", phases.TaskExecDeveloperFailed, "developer crashed")
		ctx := &reactiveEngine.RuleContext{State: state}

		payload, err := rule.Action.BuildPayload(ctx)
		if err != nil {
			t.Fatalf("BuildPayload failed: %v", err)
		}
		errPayload, ok := payload.(*TaskExecErrorPayload)
		if !ok {
			t.Fatalf("expected *TaskExecErrorPayload, got %T", payload)
		}
		if errPayload.Slug != "proj" {
			t.Errorf("expected Slug 'proj', got %q", errPayload.Slug)
		}
		if errPayload.TaskID != "t1" {
			t.Errorf("expected TaskID 't1', got %q", errPayload.TaskID)
		}
		if errPayload.Error != "developer crashed" {
			t.Errorf("expected Error 'developer crashed', got %q", errPayload.Error)
		}
	})

	t.Run("builds error event with fallback message when Error is empty", func(t *testing.T) {
		state := taskExecFailedState("proj", "t1", phases.TaskExecReviewerFailed, "")
		ctx := &reactiveEngine.RuleContext{State: state}

		payload, err := rule.Action.BuildPayload(ctx)
		if err != nil {
			t.Fatalf("BuildPayload failed: %v", err)
		}
		errPayload := payload.(*TaskExecErrorPayload)
		if !strings.Contains(errPayload.Error, phases.TaskExecReviewerFailed) {
			t.Errorf("expected fallback error to mention phase, got %q", errPayload.Error)
		}
	})

	t.Run("mutation marks execution as failed", func(t *testing.T) {
		state := taskExecFailedState("proj", "t1", phases.TaskExecDeveloperFailed, "timeout")
		ctx := &reactiveEngine.RuleContext{State: state}

		if err := rule.Action.MutateState(ctx, nil); err != nil {
			t.Fatalf("MutateState failed: %v", err)
		}
		if state.Status != reactiveEngine.StatusFailed {
			t.Errorf("expected StatusFailed, got %v", state.Status)
		}
	})
}

// ---------------------------------------------------------------------------
// Event payload JSON roundtrip tests
// ---------------------------------------------------------------------------

func TestTaskExecutionWorkflow_EventPayloadJSON(t *testing.T) {
	t.Run("TaskValidationPassedPayload roundtrip", func(t *testing.T) {
		original := &TaskValidationPassedPayload{
			StructuralValidationPassedEvent: workflow.StructuralValidationPassedEvent{
				TaskID:    "task-001",
				ChecksRun: 5,
			},
		}
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		// Wire format must match the inner event (no wrapper).
		var event workflow.StructuralValidationPassedEvent
		if err := json.Unmarshal(data, &event); err != nil {
			t.Fatalf("Unmarshal as StructuralValidationPassedEvent failed: %v", err)
		}
		if event.TaskID != "task-001" || event.ChecksRun != 5 {
			t.Errorf("roundtrip mismatch: %+v", event)
		}

		var restored TaskValidationPassedPayload
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("Unmarshal back failed: %v", err)
		}
		if restored.TaskID != "task-001" {
			t.Errorf("expected TaskID 'task-001', got %q", restored.TaskID)
		}
	})

	t.Run("TaskRejectionCategorizedPayload roundtrip", func(t *testing.T) {
		original := &TaskRejectionCategorizedPayload{
			RejectionCategorizedEvent: workflow.RejectionCategorizedEvent{
				Type: "fixable",
			},
		}
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		var restored TaskRejectionCategorizedPayload
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if restored.Type != "fixable" {
			t.Errorf("expected Type 'fixable', got %q", restored.Type)
		}
	})

	t.Run("TaskCompletePayload roundtrip", func(t *testing.T) {
		original := &TaskCompletePayload{
			TaskExecutionCompleteEvent: workflow.TaskExecutionCompleteEvent{
				TaskID:     "task-001",
				Iterations: 2,
			},
		}
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		var restored TaskCompletePayload
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if restored.TaskID != "task-001" {
			t.Errorf("expected TaskID 'task-001', got %q", restored.TaskID)
		}
		if restored.Iterations != 2 {
			t.Errorf("expected Iterations 2, got %d", restored.Iterations)
		}
	})

	t.Run("TaskExecEscalatePayload roundtrip", func(t *testing.T) {
		original := &TaskExecEscalatePayload{
			EscalationEvent: workflow.EscalationEvent{
				Slug:      "proj",
				TaskID:    "t1",
				Reason:    "max retries exceeded",
				Iteration: 3,
			},
		}
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		var restored TaskExecEscalatePayload
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if restored.Reason != "max retries exceeded" {
			t.Errorf("expected Reason 'max retries exceeded', got %q", restored.Reason)
		}
		if restored.TaskID != "t1" {
			t.Errorf("expected TaskID 't1', got %q", restored.TaskID)
		}
	})

	t.Run("TaskExecErrorPayload roundtrip", func(t *testing.T) {
		original := &TaskExecErrorPayload{
			UserSignalErrorEvent: workflow.UserSignalErrorEvent{
				Slug:   "proj",
				TaskID: "t1",
				Error:  "developer crashed",
			},
		}
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		var restored TaskExecErrorPayload
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if restored.Error != "developer crashed" {
			t.Errorf("expected Error 'developer crashed', got %q", restored.Error)
		}
		if restored.TaskID != "t1" {
			t.Errorf("expected TaskID 't1', got %q", restored.TaskID)
		}
	})

	t.Run("PlanRefinementTriggerPayload roundtrip", func(t *testing.T) {
		original := &PlanRefinementTriggerPayload{
			OriginalTaskID: "t1",
			Feedback:       "Needs schema changes",
			PlanSlug:       "my-plan",
			RejectionType:  "misscoped",
		}
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		var restored PlanRefinementTriggerPayload
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if restored.OriginalTaskID != "t1" {
			t.Errorf("expected OriginalTaskID 't1', got %q", restored.OriginalTaskID)
		}
		if restored.PlanSlug != "my-plan" {
			t.Errorf("expected PlanSlug 'my-plan', got %q", restored.PlanSlug)
		}
		if restored.RejectionType != "misscoped" {
			t.Errorf("expected RejectionType 'misscoped', got %q", restored.RejectionType)
		}
	})

	t.Run("TaskDecompositionTriggerPayload roundtrip", func(t *testing.T) {
		original := &TaskDecompositionTriggerPayload{
			OriginalTaskID: "t2",
			Feedback:       "Too large, split into 3",
			PlanSlug:       "big-plan",
		}
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		var restored TaskDecompositionTriggerPayload
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if restored.OriginalTaskID != "t2" {
			t.Errorf("expected OriginalTaskID 't2', got %q", restored.OriginalTaskID)
		}
		if restored.Feedback != "Too large, split into 3" {
			t.Errorf("expected Feedback, got %q", restored.Feedback)
		}
	})
}

// ---------------------------------------------------------------------------
// Integration tests using TestEngine
// ---------------------------------------------------------------------------

func TestTaskExecutionWorkflow_HappyPath(t *testing.T) {
	engine := testutil.NewTestEngine(t)
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)

	if err := engine.RegisterWorkflow(def); err != nil {
		t.Fatalf("RegisterWorkflow failed: %v", err)
	}

	const key = "task-execution.happy-proj.task-001"

	// Step 1: Start in developing phase.
	triggerHappyPathDeveloping(t, engine, key)

	// Steps 2-3: Developer and validation callbacks.
	state := triggerHappyPathValidating(t, engine, key)

	// Step 4: Apply validation-passed mutator (moves to reviewing).
	applyValidationPassedMutator(t, engine, def, key, state)

	// Step 5: Simulate review callback — approved.
	triggerHappyPathApproved(t, engine, key, state)

	// Verify handle-approved rule fires correctly.
	verifyApprovedRulePayload(t, engine, def, key, state)
}

// triggerHappyPathDeveloping seeds the initial state and asserts the engine
// transitions into the developing phase with running status.
func triggerHappyPathDeveloping(t *testing.T, engine *testutil.TestEngine, key string) {
	t.Helper()
	initial := &TaskExecutionState{
		ExecutionState: reactiveEngine.ExecutionState{
			ID:         key,
			WorkflowID: "task-execution-loop",
			Phase:      phases.TaskExecDeveloping,
			Status:     reactiveEngine.StatusRunning,
			Iteration:  0,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
		Slug:   "happy-proj",
		TaskID: "task-001",
		Prompt: "Implement login",
	}
	if err := engine.TriggerKV(context.Background(), key, initial); err != nil {
		t.Fatalf("TriggerKV failed: %v", err)
	}
	engine.AssertPhase(key, phases.TaskExecDeveloping)
	engine.AssertStatus(key, reactiveEngine.StatusRunning)
}

// triggerHappyPathValidating simulates the developer completing its work (step 2)
// and the validator reporting success (step 3). Returns the current state pointer
// for the caller to use in subsequent steps.
func triggerHappyPathValidating(t *testing.T, engine *testutil.TestEngine, key string) *TaskExecutionState {
	t.Helper()
	state := &TaskExecutionState{}
	if err := engine.GetStateAs(key, state); err != nil {
		t.Fatalf("GetStateAs failed: %v", err)
	}
	// Developer callback
	state.FilesModified = []string{"login.go"}
	state.DeveloperOutput = json.RawMessage(`{"files_written":1}`)
	state.LLMRequestIDs = []string{"llm-dev-1"}
	state.Phase = phases.TaskExecValidating
	if err := engine.TriggerKV(context.Background(), key, state); err != nil {
		t.Fatalf("TriggerKV (validating) failed: %v", err)
	}
	engine.AssertPhase(key, phases.TaskExecValidating)

	// Validation callback — passed
	if err := engine.GetStateAs(key, state); err != nil {
		t.Fatalf("GetStateAs failed: %v", err)
	}
	state.ValidationPassed = true
	state.ChecksRun = 4
	state.Phase = phases.TaskExecValidationChecked
	if err := engine.TriggerKV(context.Background(), key, state); err != nil {
		t.Fatalf("TriggerKV (validation_checked) failed: %v", err)
	}
	engine.AssertPhase(key, phases.TaskExecValidationChecked)
	return state
}

// applyValidationPassedMutator runs the validation-passed rule's MutateState and
// triggers the KV update so the engine transitions to the reviewing phase (step 4).
func applyValidationPassedMutator(
	t *testing.T,
	engine *testutil.TestEngine,
	def *reactiveEngine.Definition,
	key string,
	state *TaskExecutionState,
) {
	t.Helper()
	validationPassedRule := findRule(t, def, "validation-passed")
	if err := engine.GetStateAs(key, state); err != nil {
		t.Fatalf("GetStateAs failed: %v", err)
	}
	ctx := &reactiveEngine.RuleContext{State: state}
	if err := validationPassedRule.Action.MutateState(ctx, nil); err != nil {
		t.Fatalf("validation-passed MutateState failed: %v", err)
	}
	if err := engine.TriggerKV(context.Background(), key, state); err != nil {
		t.Fatalf("TriggerKV (reviewing) failed: %v", err)
	}
	engine.AssertPhase(key, phases.TaskExecReviewing)
}

// triggerHappyPathApproved simulates the reviewer returning an approved verdict (step 5).
func triggerHappyPathApproved(
	t *testing.T,
	engine *testutil.TestEngine,
	key string,
	state *TaskExecutionState,
) {
	t.Helper()
	if err := engine.GetStateAs(key, state); err != nil {
		t.Fatalf("GetStateAs failed: %v", err)
	}
	state.Verdict = "approved"
	state.ReviewerLLMRequestIDs = []string{"llm-rev-1"}
	state.Phase = phases.TaskExecEvaluated
	if err := engine.TriggerKV(context.Background(), key, state); err != nil {
		t.Fatalf("TriggerKV (evaluated) failed: %v", err)
	}
	engine.AssertPhase(key, phases.TaskExecEvaluated)
}

// verifyApprovedRulePayload asserts the handle-approved rule's conditions pass
// and that BuildPayload returns a *TaskCompletePayload.
func verifyApprovedRulePayload(
	t *testing.T,
	engine *testutil.TestEngine,
	def *reactiveEngine.Definition,
	key string,
	state *TaskExecutionState,
) {
	t.Helper()
	approvedRule := findRule(t, def, "handle-approved")
	if err := engine.GetStateAs(key, state); err != nil {
		t.Fatalf("GetStateAs failed: %v", err)
	}
	ctx := &reactiveEngine.RuleContext{State: state}
	assertAllConditionsPass(t, approvedRule, ctx)

	payload, err := approvedRule.Action.BuildPayload(ctx)
	if err != nil {
		t.Fatalf("BuildPayload failed: %v", err)
	}
	if _, ok := payload.(*TaskCompletePayload); !ok {
		t.Errorf("expected *TaskCompletePayload, got %T", payload)
	}
}

func TestTaskExecutionWorkflow_ValidationFailureThenRetry(t *testing.T) {
	engine := testutil.NewTestEngine(t)
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)

	if err := engine.RegisterWorkflow(def); err != nil {
		t.Fatalf("RegisterWorkflow failed: %v", err)
	}

	const key = "task-execution.retry-proj.task-002"

	// Start in developing.
	state := &TaskExecutionState{
		ExecutionState: reactiveEngine.ExecutionState{
			ID:         key,
			WorkflowID: "task-execution-loop",
			Phase:      phases.TaskExecDeveloping,
			Status:     reactiveEngine.StatusRunning,
			Iteration:  0,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
		Slug:   "retry-proj",
		TaskID: "task-002",
	}
	if err := engine.TriggerKV(context.Background(), key, state); err != nil {
		t.Fatalf("TriggerKV failed: %v", err)
	}

	// Validation fails.
	if err := engine.GetStateAs(key, state); err != nil {
		t.Fatalf("GetStateAs failed: %v", err)
	}
	state.ValidationPassed = false
	state.ChecksRun = 2
	state.CheckResults = json.RawMessage(`[{"check":"compile","passed":false}]`)
	state.Phase = phases.TaskExecValidationChecked
	if err := engine.TriggerKV(context.Background(), key, state); err != nil {
		t.Fatalf("TriggerKV (validation_checked) failed: %v", err)
	}
	engine.AssertPhase(key, phases.TaskExecValidationChecked)

	// Apply validation-failed-retry mutator.
	retryRule := findRule(t, def, "validation-failed-retry")
	if err := engine.GetStateAs(key, state); err != nil {
		t.Fatalf("GetStateAs failed: %v", err)
	}
	ctx := &reactiveEngine.RuleContext{State: state}
	if err := retryRule.Action.MutateState(ctx, nil); err != nil {
		t.Fatalf("retry MutateState failed: %v", err)
	}
	if err := engine.TriggerKV(context.Background(), key, state); err != nil {
		t.Fatalf("TriggerKV (developing) failed: %v", err)
	}
	engine.AssertPhase(key, phases.TaskExecDeveloping)
	engine.AssertIteration(key, 1)

	// Verify RevisionSource is "validation".
	if err := engine.GetStateAs(key, state); err != nil {
		t.Fatalf("GetStateAs failed: %v", err)
	}
	if state.RevisionSource != "validation" {
		t.Errorf("expected RevisionSource 'validation', got %q", state.RevisionSource)
	}
}

func TestTaskExecutionWorkflow_FixableRejectionThenApproved(t *testing.T) {
	engine := testutil.NewTestEngine(t)
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)

	if err := engine.RegisterWorkflow(def); err != nil {
		t.Fatalf("RegisterWorkflow failed: %v", err)
	}

	const key = "task-execution.fixable-proj.task-003"

	// Start in evaluated with fixable rejection.
	state := &TaskExecutionState{
		ExecutionState: reactiveEngine.ExecutionState{
			ID:         key,
			WorkflowID: "task-execution-loop",
			Phase:      phases.TaskExecEvaluated,
			Status:     reactiveEngine.StatusRunning,
			Iteration:  0,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
		Slug:          "fixable-proj",
		TaskID:        "task-003",
		Verdict:       "rejected",
		RejectionType: "fixable",
		Feedback:      "Add input validation",
	}
	if err := engine.TriggerKV(context.Background(), key, state); err != nil {
		t.Fatalf("TriggerKV failed: %v", err)
	}
	engine.AssertPhase(key, phases.TaskExecEvaluated)

	// Apply fixable-retry mutator.
	fixableRule := findRule(t, def, "handle-fixable-retry")
	if err := engine.GetStateAs(key, state); err != nil {
		t.Fatalf("GetStateAs failed: %v", err)
	}
	ctx := &reactiveEngine.RuleContext{State: state}
	if err := fixableRule.Action.MutateState(ctx, nil); err != nil {
		t.Fatalf("fixable MutateState failed: %v", err)
	}
	if err := engine.TriggerKV(context.Background(), key, state); err != nil {
		t.Fatalf("TriggerKV (developing) failed: %v", err)
	}
	engine.AssertPhase(key, phases.TaskExecDeveloping)
	engine.AssertIteration(key, 1)

	// RevisionSource should be "review".
	if err := engine.GetStateAs(key, state); err != nil {
		t.Fatalf("GetStateAs failed: %v", err)
	}
	if state.RevisionSource != "review" {
		t.Errorf("expected RevisionSource 'review', got %q", state.RevisionSource)
	}
	if state.Verdict != "" {
		t.Errorf("expected Verdict to be cleared, got %q", state.Verdict)
	}

	// Now approved on second attempt.
	if err := engine.GetStateAs(key, state); err != nil {
		t.Fatalf("GetStateAs failed: %v", err)
	}
	state.Phase = phases.TaskExecEvaluated
	state.Verdict = "approved"
	state.RejectionType = ""
	if err := engine.TriggerKV(context.Background(), key, state); err != nil {
		t.Fatalf("TriggerKV (approved) failed: %v", err)
	}

	approvedRule := findRule(t, def, "handle-approved")
	if err := engine.GetStateAs(key, state); err != nil {
		t.Fatalf("GetStateAs failed: %v", err)
	}
	ctx = &reactiveEngine.RuleContext{State: state}
	assertAllConditionsPass(t, approvedRule, ctx)
}

func TestTaskExecutionWorkflow_MaxRetriesEscalation(t *testing.T) {
	engine := testutil.NewTestEngine(t)
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)

	if err := engine.RegisterWorkflow(def); err != nil {
		t.Fatalf("RegisterWorkflow failed: %v", err)
	}

	const key = "task-execution.escalation-proj.task-004"

	// At max iterations with a fixable rejection.
	state := &TaskExecutionState{
		ExecutionState: reactiveEngine.ExecutionState{
			ID:         key,
			WorkflowID: "task-execution-loop",
			Phase:      phases.TaskExecEvaluated,
			Status:     reactiveEngine.StatusRunning,
			Iteration:  3,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
		Slug:          "escalation-proj",
		TaskID:        "task-004",
		Verdict:       "rejected",
		RejectionType: "fixable",
		Feedback:      "Still not right",
	}
	if err := engine.TriggerKV(context.Background(), key, state); err != nil {
		t.Fatalf("TriggerKV failed: %v", err)
	}

	// handle-fixable-retry should NOT fire.
	fixableRule := findRule(t, def, "handle-fixable-retry")
	if err := engine.GetStateAs(key, state); err != nil {
		t.Fatalf("GetStateAs failed: %v", err)
	}
	ctx := &reactiveEngine.RuleContext{State: state}
	allPass := true
	for _, cond := range fixableRule.Conditions {
		if !cond.Evaluate(ctx) {
			allPass = false
			break
		}
	}
	if allPass {
		t.Error("handle-fixable-retry should NOT fire when iteration >= max")
	}

	// handle-max-retries SHOULD fire.
	maxRetryRule := findRule(t, def, "handle-max-retries")
	assertAllConditionsPass(t, maxRetryRule, ctx)

	// Apply escalation mutator.
	if err := maxRetryRule.Action.MutateState(ctx, nil); err != nil {
		t.Fatalf("escalation MutateState failed: %v", err)
	}
	if err := engine.TriggerKV(context.Background(), key, state); err != nil {
		t.Fatalf("TriggerKV (escalated) failed: %v", err)
	}
	engine.AssertStatus(key, reactiveEngine.StatusEscalated)
}

func TestTaskExecutionWorkflow_MisscopedExitsWithPlanRefinement(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "handle-misscoped")

	state := taskExecEvaluatedState("proj", "t5", "rejected", "misscoped")
	state.Feedback = "Scope exceeds plan boundaries"
	ctx := &reactiveEngine.RuleContext{State: state}

	assertAllConditionsPass(t, rule, ctx)

	payload, err := rule.Action.BuildPayload(ctx)
	if err != nil {
		t.Fatalf("BuildPayload failed: %v", err)
	}
	trig, ok := payload.(*PlanRefinementTriggerPayload)
	if !ok {
		t.Fatalf("expected *PlanRefinementTriggerPayload, got %T", payload)
	}
	if trig.RejectionType != "misscoped" {
		t.Errorf("expected RejectionType 'misscoped', got %q", trig.RejectionType)
	}
}

func TestTaskExecutionWorkflow_ArchitecturalExitsWithPlanRefinement(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "handle-misscoped")

	state := taskExecEvaluatedState("proj", "t6", "rejected", "architectural")
	state.Feedback = "Requires fundamental architecture change"
	ctx := &reactiveEngine.RuleContext{State: state}

	assertAllConditionsPass(t, rule, ctx)

	payload, err := rule.Action.BuildPayload(ctx)
	if err != nil {
		t.Fatalf("BuildPayload failed: %v", err)
	}
	trig, ok := payload.(*PlanRefinementTriggerPayload)
	if !ok {
		t.Fatalf("expected *PlanRefinementTriggerPayload, got %T", payload)
	}
	if trig.RejectionType != "architectural" {
		t.Errorf("expected RejectionType 'architectural', got %q", trig.RejectionType)
	}
}

func TestTaskExecutionWorkflow_TooBigExitsWithDecomposition(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)
	rule := findRule(t, def, "handle-too-big")

	state := taskExecEvaluatedState("proj", "t7", "rejected", "too_big")
	state.Feedback = "Needs to be split into 4 sub-tasks"
	ctx := &reactiveEngine.RuleContext{State: state}

	assertAllConditionsPass(t, rule, ctx)

	payload, err := rule.Action.BuildPayload(ctx)
	if err != nil {
		t.Fatalf("BuildPayload failed: %v", err)
	}
	trig, ok := payload.(*TaskDecompositionTriggerPayload)
	if !ok {
		t.Fatalf("expected *TaskDecompositionTriggerPayload, got %T", payload)
	}
	if trig.PlanSlug != "proj" {
		t.Errorf("expected PlanSlug 'proj', got %q", trig.PlanSlug)
	}
}

// ---------------------------------------------------------------------------
// Task execution test helpers
// ---------------------------------------------------------------------------

// taskExecDevelopingState returns a TaskExecutionState in the developing phase.
func taskExecDevelopingState(slug, taskID string) *TaskExecutionState {
	return &TaskExecutionState{
		ExecutionState: reactiveEngine.ExecutionState{
			ID:        "task-execution." + slug + "." + taskID,
			Phase:     phases.TaskExecDeveloping,
			Status:    reactiveEngine.StatusRunning,
			Iteration: 0,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Slug:   slug,
		TaskID: taskID,
	}
}

// taskExecValidatingState returns a TaskExecutionState in the validating phase.
func taskExecValidatingState(slug, taskID string) *TaskExecutionState {
	s := taskExecDevelopingState(slug, taskID)
	s.Phase = phases.TaskExecValidating
	s.FilesModified = []string{"main.go"}
	s.DeveloperOutput = json.RawMessage(`{"files_written":1}`)
	return s
}

// taskExecValidationCheckedState returns a TaskExecutionState in the
// validation_checked phase with the given passed status.
func taskExecValidationCheckedState(slug, taskID string, passed bool) *TaskExecutionState {
	s := taskExecDevelopingState(slug, taskID)
	s.Phase = phases.TaskExecValidationChecked
	s.ValidationPassed = passed
	s.ChecksRun = 3
	return s
}

// taskExecReviewingState returns a TaskExecutionState in the reviewing phase.
func taskExecReviewingState(slug, taskID string) *TaskExecutionState {
	s := taskExecValidatingState(slug, taskID)
	s.Phase = phases.TaskExecReviewing
	s.ValidationPassed = true
	s.ChecksRun = 3
	return s
}

// taskExecEvaluatedState returns a TaskExecutionState in the evaluated phase
// with the given verdict and rejection type.
func taskExecEvaluatedState(slug, taskID, verdict, rejectionType string) *TaskExecutionState {
	s := taskExecDevelopingState(slug, taskID)
	s.Phase = phases.TaskExecEvaluated
	s.Verdict = verdict
	s.RejectionType = rejectionType
	return s
}

// taskExecFailedState returns a TaskExecutionState in a failure phase.
func taskExecFailedState(slug, taskID, phase, errMsg string) *TaskExecutionState {
	s := taskExecDevelopingState(slug, taskID)
	s.Phase = phase
	s.Error = errMsg
	return s
}

// ---------------------------------------------------------------------------
// Quality gate enforcement tests
//
// These tests verify the safety property that quality gates cannot be bypassed:
//   - A failed validation must NOT allow progression to reviewing
//   - A rejected review must NOT allow progression to evaluated/approved
//   - validation-passed and validation-failed-retry/escalate are mutually exclusive
//   - handle-approved and fixable-retry/escalate are mutually exclusive
// ---------------------------------------------------------------------------

// TestTaskExecutionQualityGate_ValidationBlocksReviewing verifies that when
// ValidationPassed=false, the validation-passed rule (which transitions to
// reviewing) does NOT fire, and validation-failed-retry DOES fire. This is
// the primary safety check: failed validation cannot bypass the retry gate
// to reach the reviewer.
func TestTaskExecutionQualityGate_ValidationBlocksReviewing(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)

	passedRule := findRule(t, def, "validation-passed")
	retryRule := findRule(t, def, "validation-failed-retry")

	// Both rules watch the same phase (validation_checked). When validation
	// fails, exactly ONE of them should fire. This test confirms that failure
	// state satisfies retry conditions but not passed conditions, and vice versa.

	t.Run("failed validation: passed rule blocked, retry rule fires", func(t *testing.T) {
		state := taskExecValidationCheckedState("qg-proj", "qg-t1", false)
		state.Iteration = 0
		ctx := &reactiveEngine.RuleContext{State: state}

		// validation-passed must NOT fire when validation failed.
		allPassedConditionsPass := true
		for _, cond := range passedRule.Conditions {
			if !cond.Evaluate(ctx) {
				allPassedConditionsPass = false
				break
			}
		}
		if allPassedConditionsPass {
			t.Error("quality gate violated: validation-passed rule fired despite ValidationPassed=false")
		}

		// validation-failed-retry MUST fire.
		assertAllConditionsPass(t, retryRule, ctx)
	})

	t.Run("passed validation: retry rule blocked, passed rule fires", func(t *testing.T) {
		state := taskExecValidationCheckedState("qg-proj", "qg-t2", true)
		state.Iteration = 0
		ctx := &reactiveEngine.RuleContext{State: state}

		// validation-passed MUST fire.
		assertAllConditionsPass(t, passedRule, ctx)

		// validation-failed-retry must NOT fire.
		allRetryConditionsPass := true
		for _, cond := range retryRule.Conditions {
			if !cond.Evaluate(ctx) {
				allRetryConditionsPass = false
				break
			}
		}
		if allRetryConditionsPass {
			t.Error("quality gate violated: validation-failed-retry rule fired despite ValidationPassed=true")
		}
	})
}

// TestTaskExecutionQualityGate_ValidationWithZeroChecksStillGates verifies that
// when ChecksRun=0 but ValidationPassed=true (empty checklist case), the workflow
// still correctly advances to reviewing. The gate is on ValidationPassed, not on
// ChecksRun, so a zero-check pass is valid.
func TestTaskExecutionQualityGate_ValidationWithZeroChecksStillGates(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)

	t.Run("zero checks passed=true progresses to reviewing", func(t *testing.T) {
		state := taskExecValidationCheckedState("qg-proj", "qg-t3", true)
		state.ChecksRun = 0 // override — no checklist ran
		ctx := &reactiveEngine.RuleContext{State: state}

		passedRule := findRule(t, def, "validation-passed")
		assertAllConditionsPass(t, passedRule, ctx)

		// Mutation must advance to reviewing — not stay stuck.
		if err := passedRule.Action.MutateState(ctx, nil); err != nil {
			t.Fatalf("MutateState failed: %v", err)
		}
		if state.Phase != phases.TaskExecReviewing {
			t.Errorf("expected phase %q after zero-check pass, got %q",
				phases.TaskExecReviewing, state.Phase)
		}
	})

	t.Run("zero checks passed=false blocks progression", func(t *testing.T) {
		state := taskExecValidationCheckedState("qg-proj", "qg-t4", false)
		state.ChecksRun = 0
		state.Iteration = 0
		ctx := &reactiveEngine.RuleContext{State: state}

		passedRule := findRule(t, def, "validation-passed")
		assertSomeConditionFails(t, passedRule, ctx)

		// Must not transition to reviewing via mutation.
		originalPhase := state.Phase
		_ = passedRule.Action.MutateState(ctx, nil) //nolint:errcheck — expect no-op or error
		// Phase should remain unchanged (condition blocked the rule from running in production).
		// Here we confirm the condition gate itself rejects the state, which is the safety invariant.
		// The MutateState call is only meaningful if conditions pass; we verify conditions fail.
		_ = originalPhase
	})
}

// TestTaskExecutionQualityGate_ReviewBlocksApproval verifies that when the
// reviewer rejects a task, the handle-approved rule does NOT fire. This ensures
// a rejected task cannot bypass the review gate and complete as approved.
func TestTaskExecutionQualityGate_ReviewBlocksApproval(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)

	approvedRule := findRule(t, def, "handle-approved")
	fixableRule := findRule(t, def, "handle-fixable-retry")

	t.Run("rejected task: approved rule blocked", func(t *testing.T) {
		state := taskExecEvaluatedState("qg-proj", "qg-t5", "rejected", "fixable")
		ctx := &reactiveEngine.RuleContext{State: state}

		assertSomeConditionFails(t, approvedRule, ctx)
	})

	t.Run("approved task: fixable-retry rule blocked", func(t *testing.T) {
		state := taskExecEvaluatedState("qg-proj", "qg-t6", "approved", "")
		ctx := &reactiveEngine.RuleContext{State: state}

		assertSomeConditionFails(t, fixableRule, ctx)
	})

	t.Run("all known rejection types block approval", func(t *testing.T) {
		rejectionTypes := []string{"fixable", "misscoped", "architectural", "too_big", "unknown_type"}
		for _, rt := range rejectionTypes {
			rt := rt
			t.Run(rt, func(t *testing.T) {
				state := taskExecEvaluatedState("qg-proj", "qg-t7", "rejected", rt)
				ctx := &reactiveEngine.RuleContext{State: state}
				assertSomeConditionFails(t, approvedRule, ctx)
			})
		}
	})
}

// TestTaskExecutionQualityGate_ReviewingRequiresValidationPass verifies that
// the reviewing phase can only be entered after a successful validation. A task
// with ValidationPassed=false must not satisfy the reviewing dispatch rule's
// prerequisite path. We verify this by checking that the validation-passed rule
// (which is the only path into the reviewing phase) requires ValidationPassed=true.
func TestTaskExecutionQualityGate_ReviewingRequiresValidationPass(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)

	// The only transition into the reviewing phase is via the validation-passed rule's
	// mutation. The dispatch-review rule then picks up from reviewing phase.
	// We verify that a state at validation_checked with passed=false cannot
	// satisfy the validation-passed rule (the gate into reviewing).
	reviewDispatchRule := findRule(t, def, "dispatch-review")
	validationPassedRule := findRule(t, def, "validation-passed")

	t.Run("dispatch-review does not fire from validation_checked with failed validation", func(t *testing.T) {
		// A state with failed validation is in validation_checked, not reviewing.
		// dispatch-review watches for the reviewing phase — it should not match.
		state := taskExecValidationCheckedState("qg-proj", "qg-t8", false)
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, reviewDispatchRule, ctx)
	})

	t.Run("dispatch-review does not fire from validation_checked with passed validation", func(t *testing.T) {
		// Even a passed validation at validation_checked phase cannot directly
		// trigger dispatch-review — it must go through validation-passed mutation first.
		state := taskExecValidationCheckedState("qg-proj", "qg-t9", true)
		ctx := &reactiveEngine.RuleContext{State: state}
		assertSomeConditionFails(t, reviewDispatchRule, ctx)
	})

	t.Run("validation-passed mutation is the sole gateway into reviewing", func(t *testing.T) {
		// Apply the validation-passed mutation to a passed state and confirm it
		// produces the reviewing phase — the one and only legal entry point.
		state := taskExecValidationCheckedState("qg-proj", "qg-t10", true)
		ctx := &reactiveEngine.RuleContext{State: state}
		assertAllConditionsPass(t, validationPassedRule, ctx)

		if err := validationPassedRule.Action.MutateState(ctx, nil); err != nil {
			t.Fatalf("MutateState failed: %v", err)
		}
		if state.Phase != phases.TaskExecReviewing {
			t.Errorf("expected reviewing phase after validation-passed mutation, got %q", state.Phase)
		}

		// Now dispatch-review conditions should pass.
		ctxAfter := &reactiveEngine.RuleContext{State: state}
		assertAllConditionsPass(t, reviewDispatchRule, ctxAfter)
	})
}

// TestTaskExecutionQualityGate_EscalateAndRetryMutuallyExclusive verifies that
// the escalation and retry rules at the validation boundary are mutually exclusive:
// - validation-failed-retry only fires when under the iteration limit
// - validation-failed-escalate only fires when at or over the iteration limit
// They cannot both fire for the same state.
func TestTaskExecutionQualityGate_EscalateAndRetryMutuallyExclusive(t *testing.T) {
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)

	retryRule := findRule(t, def, "validation-failed-retry")
	escalateRule := findRule(t, def, "validation-failed-escalate")

	for iter := 0; iter <= 4; iter++ {
		iter := iter
		t.Run(fmt.Sprintf("iteration_%d", iter), func(t *testing.T) {
			state := taskExecValidationCheckedState("qg-proj", "qg-t11", false)
			state.Iteration = iter
			ctx := &reactiveEngine.RuleContext{State: state}

			retryFires := true
			for _, cond := range retryRule.Conditions {
				if !cond.Evaluate(ctx) {
					retryFires = false
					break
				}
			}

			escalateFires := true
			// not-completed guard: state is not yet completed
			for _, cond := range escalateRule.Conditions {
				if !cond.Evaluate(ctx) {
					escalateFires = false
					break
				}
			}

			// Safety property: at most one of the two rules fires for any given state.
			if retryFires && escalateFires {
				t.Errorf("iteration %d: both validation-failed-retry and validation-failed-escalate fire — mutual exclusion violated", iter)
			}

			// At least one must fire for a failed, non-completed state (max=3).
			if iter < 3 && !retryFires {
				t.Errorf("iteration %d: expected validation-failed-retry to fire (under limit), but it did not", iter)
			}
			if iter >= 3 && !escalateFires {
				t.Errorf("iteration %d: expected validation-failed-escalate to fire (at/over limit), but it did not", iter)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Full pipeline integration tests (quality gate safety properties)
// ---------------------------------------------------------------------------

// TestTaskExecutionQualityGate_FullPipeline_ValidationFailStaysBlocked is an
// integration test verifying that when validation fails, the task stays in
// validation_checked phase (not reviewing) and the retry rule is the correct
// next step — not dispatch-review.
func TestTaskExecutionQualityGate_FullPipeline_ValidationFailStaysBlocked(t *testing.T) {
	engine := testutil.NewTestEngine(t)
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)

	if err := engine.RegisterWorkflow(def); err != nil {
		t.Fatalf("RegisterWorkflow failed: %v", err)
	}

	const key = "task-execution.qg-pipe.task-vfail"

	// Seed the state as validation_checked with a failure.
	state := &TaskExecutionState{
		ExecutionState: reactiveEngine.ExecutionState{
			ID:         key,
			WorkflowID: "task-execution-loop",
			Phase:      phases.TaskExecValidationChecked,
			Status:     reactiveEngine.StatusRunning,
			Iteration:  0,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
		Slug:             "qg-pipe",
		TaskID:           "task-vfail",
		ValidationPassed: false,
		ChecksRun:        3,
		CheckResults:     json.RawMessage(`[{"check":"go-test","passed":false,"message":"FAIL: TestFoo"}]`),
	}

	if err := engine.TriggerKV(context.Background(), key, state); err != nil {
		t.Fatalf("TriggerKV failed: %v", err)
	}

	// Confirm we are in validation_checked (failed).
	engine.AssertPhase(key, phases.TaskExecValidationChecked)
	engine.AssertStatus(key, reactiveEngine.StatusRunning)

	// Confirm dispatch-review conditions do NOT pass — the phase is wrong.
	reviewDispatchRule := findRule(t, def, "dispatch-review")
	if err := engine.GetStateAs(key, state); err != nil {
		t.Fatalf("GetStateAs failed: %v", err)
	}
	ctx := &reactiveEngine.RuleContext{State: state}
	assertSomeConditionFails(t, reviewDispatchRule, ctx)

	// Confirm validation-failed-retry conditions DO pass.
	retryRule := findRule(t, def, "validation-failed-retry")
	assertAllConditionsPass(t, retryRule, ctx)

	// Apply the retry mutator and confirm we return to developing — NOT reviewing.
	if err := retryRule.Action.MutateState(ctx, nil); err != nil {
		t.Fatalf("retry MutateState failed: %v", err)
	}
	if err := engine.TriggerKV(context.Background(), key, state); err != nil {
		t.Fatalf("TriggerKV (retry) failed: %v", err)
	}
	engine.AssertPhase(key, phases.TaskExecDeveloping)
	engine.AssertIteration(key, 1)

	if err := engine.GetStateAs(key, state); err != nil {
		t.Fatalf("GetStateAs failed: %v", err)
	}
	if state.Phase == phases.TaskExecReviewing {
		t.Error("quality gate violated: task entered reviewing phase after a failed validation")
	}
	if state.RevisionSource != "validation" {
		t.Errorf("expected RevisionSource 'validation', got %q", state.RevisionSource)
	}
}

// TestTaskExecutionQualityGate_FullPipeline_ValidatePassReviewRejectRetry is an
// integration test that drives the full validate-pass → review-reject → retry-develop
// pipeline and confirms the task returns to developing with the correct revision source.
func TestTaskExecutionQualityGate_FullPipeline_ValidatePassReviewRejectRetry(t *testing.T) {
	engine := testutil.NewTestEngine(t)
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)

	if err := engine.RegisterWorkflow(def); err != nil {
		t.Fatalf("RegisterWorkflow failed: %v", err)
	}

	const key = "task-execution.qg-pipe.task-vrev"

	// Step 1: Seed state at validation_checked with passed=true.
	state := &TaskExecutionState{
		ExecutionState: reactiveEngine.ExecutionState{
			ID:         key,
			WorkflowID: "task-execution-loop",
			Phase:      phases.TaskExecValidationChecked,
			Status:     reactiveEngine.StatusRunning,
			Iteration:  0,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
		Slug:             "qg-pipe",
		TaskID:           "task-vrev",
		Prompt:           "Implement the feature",
		ValidationPassed: true,
		ChecksRun:        5,
	}
	if err := engine.TriggerKV(context.Background(), key, state); err != nil {
		t.Fatalf("TriggerKV (validation_checked) failed: %v", err)
	}
	engine.AssertPhase(key, phases.TaskExecValidationChecked)

	// Step 2: Apply validation-passed mutation → reviewing.
	validationPassedRule := findRule(t, def, "validation-passed")
	if err := engine.GetStateAs(key, state); err != nil {
		t.Fatalf("GetStateAs failed: %v", err)
	}
	ctx := &reactiveEngine.RuleContext{State: state}
	assertAllConditionsPass(t, validationPassedRule, ctx)

	if err := validationPassedRule.Action.MutateState(ctx, nil); err != nil {
		t.Fatalf("validation-passed MutateState failed: %v", err)
	}
	if err := engine.TriggerKV(context.Background(), key, state); err != nil {
		t.Fatalf("TriggerKV (reviewing) failed: %v", err)
	}
	engine.AssertPhase(key, phases.TaskExecReviewing)

	// Step 3: Simulate reviewer setting evaluated phase with a fixable rejection.
	if err := engine.GetStateAs(key, state); err != nil {
		t.Fatalf("GetStateAs failed: %v", err)
	}
	state.Phase = phases.TaskExecEvaluated
	state.Verdict = "rejected"
	state.RejectionType = "fixable"
	state.Feedback = "Missing input validation on all endpoints"
	if err := engine.TriggerKV(context.Background(), key, state); err != nil {
		t.Fatalf("TriggerKV (evaluated) failed: %v", err)
	}
	engine.AssertPhase(key, phases.TaskExecEvaluated)

	// Step 4: Confirm handle-approved does NOT fire.
	approvedRule := findRule(t, def, "handle-approved")
	if err := engine.GetStateAs(key, state); err != nil {
		t.Fatalf("GetStateAs failed: %v", err)
	}
	ctx = &reactiveEngine.RuleContext{State: state}
	assertSomeConditionFails(t, approvedRule, ctx)

	// Step 5: Apply fixable-retry mutation → developing.
	fixableRule := findRule(t, def, "handle-fixable-retry")
	assertAllConditionsPass(t, fixableRule, ctx)

	if err := fixableRule.Action.MutateState(ctx, nil); err != nil {
		t.Fatalf("fixable-retry MutateState failed: %v", err)
	}
	if err := engine.TriggerKV(context.Background(), key, state); err != nil {
		t.Fatalf("TriggerKV (developing) failed: %v", err)
	}
	engine.AssertPhase(key, phases.TaskExecDeveloping)
	engine.AssertIteration(key, 1)

	assertReviewRetryState(t, engine, key)
}

// assertReviewRetryState verifies the state after a fixable review rejection
// returns the task to developing with the correct revision metadata.
func assertReviewRetryState(t *testing.T, engine *testutil.TestEngine, key string) {
	t.Helper()
	state := &TaskExecutionState{}
	if err := engine.GetStateAs(key, state); err != nil {
		t.Fatalf("GetStateAs failed: %v", err)
	}
	if state.Phase == phases.TaskExecEvaluated {
		t.Error("quality gate violated: task stayed in evaluated after fixable rejection")
	}
	if state.RevisionSource != "review" {
		t.Errorf("expected RevisionSource 'review', got %q", state.RevisionSource)
	}
	if state.Verdict != "" {
		t.Errorf("expected Verdict to be cleared after fixable retry, got %q", state.Verdict)
	}
	if state.Feedback != "Missing input validation on all endpoints" {
		t.Errorf("expected Feedback to be preserved after fixable retry, got %q", state.Feedback)
	}
}

// TestTaskExecutionQualityGate_FullPipeline_ValidatePassReviewApprove is an
// integration test that drives the complete happy-path quality gate pipeline:
// validation passes → reviewer approves → workflow completes.
func TestTaskExecutionQualityGate_FullPipeline_ValidatePassReviewApprove(t *testing.T) {
	engine := testutil.NewTestEngine(t)
	def := BuildTaskExecutionLoopWorkflow(testStateBucket)

	if err := engine.RegisterWorkflow(def); err != nil {
		t.Fatalf("RegisterWorkflow failed: %v", err)
	}

	const key = "task-execution.qg-pipe.task-vapprove"

	// Seed from validation_checked with passed=true.
	state := &TaskExecutionState{
		ExecutionState: reactiveEngine.ExecutionState{
			ID:         key,
			WorkflowID: "task-execution-loop",
			Phase:      phases.TaskExecValidationChecked,
			Status:     reactiveEngine.StatusRunning,
			Iteration:  0,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
		Slug:             "qg-pipe",
		TaskID:           "task-vapprove",
		ValidationPassed: true,
		ChecksRun:        8,
	}
	if err := engine.TriggerKV(context.Background(), key, state); err != nil {
		t.Fatalf("TriggerKV (validation_checked) failed: %v", err)
	}

	// Gate 1: validation-passed mutation → reviewing.
	validationPassedRule := findRule(t, def, "validation-passed")
	if err := engine.GetStateAs(key, state); err != nil {
		t.Fatalf("GetStateAs failed: %v", err)
	}
	if err := validationPassedRule.Action.MutateState(&reactiveEngine.RuleContext{State: state}, nil); err != nil {
		t.Fatalf("validation-passed MutateState failed: %v", err)
	}
	if err := engine.TriggerKV(context.Background(), key, state); err != nil {
		t.Fatalf("TriggerKV (reviewing) failed: %v", err)
	}
	engine.AssertPhase(key, phases.TaskExecReviewing)

	// Gate 2: reviewer approves → evaluated.
	if err := engine.GetStateAs(key, state); err != nil {
		t.Fatalf("GetStateAs failed: %v", err)
	}
	state.Phase = phases.TaskExecEvaluated
	state.Verdict = "approved"
	if err := engine.TriggerKV(context.Background(), key, state); err != nil {
		t.Fatalf("TriggerKV (evaluated) failed: %v", err)
	}
	engine.AssertPhase(key, phases.TaskExecEvaluated)

	// Confirm all rejection rules do NOT fire when approved.
	if err := engine.GetStateAs(key, state); err != nil {
		t.Fatalf("GetStateAs failed: %v", err)
	}
	ctx := &reactiveEngine.RuleContext{State: state}
	for _, ruleID := range []string{"handle-fixable-retry", "handle-max-retries", "handle-misscoped", "handle-too-big", "handle-unknown-rejection"} {
		rule := findRule(t, def, ruleID)
		allPass := true
		for _, cond := range rule.Conditions {
			if !cond.Evaluate(ctx) {
				allPass = false
				break
			}
		}
		if allPass {
			t.Errorf("quality gate violated: rule %q should not fire on approved verdict", ruleID)
		}
	}

	// Confirm handle-approved fires.
	approvedRule := findRule(t, def, "handle-approved")
	assertAllConditionsPass(t, approvedRule, ctx)

	payload, err := approvedRule.Action.BuildPayload(ctx)
	if err != nil {
		t.Fatalf("BuildPayload failed: %v", err)
	}
	complete, ok := payload.(*TaskCompletePayload)
	if !ok {
		t.Fatalf("expected *TaskCompletePayload, got %T", payload)
	}
	if complete.TaskID != "task-vapprove" {
		t.Errorf("expected TaskID 'task-vapprove', got %q", complete.TaskID)
	}
}
