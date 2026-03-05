package reactive

import (
	"fmt"

	reactiveEngine "github.com/c360studio/semstreams/processor/reactive"
)

// RegisterAll registers all semspec reactive workflow definitions and result
// types with the engine. Called from main.go after the reactive-workflow
// component is created.
//
// Usage:
//
//	comp := reactiveEngine.NewComponent(...)
//	if err := semspecWorkflows.RegisterAll(comp.Engine()); err != nil {
//	    log.Fatal("failed to register workflows:", err)
//	}
func RegisterAll(engine *reactiveEngine.Engine) error {
	if engine == nil {
		return fmt.Errorf("reactive engine is nil")
	}

	// Register callback result types first so the engine can deserialize
	// component outputs when callbacks arrive.
	if err := RegisterResultTypes(engine.Registry()); err != nil {
		return fmt.Errorf("register result types: %w", err)
	}

	// Use the engine's configured bucket name to avoid fragile coupling
	// between hardcoded strings and the JSON config.
	stateBucket := engine.StateBucket()

	// Register workflow definitions.
	// Each workflow is implemented in its own file and added here as completed.

	// Phase 2: plan-review-loop
	def := BuildPlanReviewWorkflow(stateBucket)
	if err := engine.RegisterWorkflow(def); err != nil {
		return fmt.Errorf("register plan-review-loop: %w", err)
	}

	// Phase 4a: phase-review-loop
	def = BuildPhaseReviewWorkflow(stateBucket)
	if err := engine.RegisterWorkflow(def); err != nil {
		return fmt.Errorf("register phase-review-loop: %w", err)
	}

	// Phase 4b: task-review-loop
	def = BuildTaskReviewWorkflow(stateBucket)
	if err := engine.RegisterWorkflow(def); err != nil {
		return fmt.Errorf("register task-review-loop: %w", err)
	}

	// Phase 5: task-execution-loop
	def = BuildTaskExecutionLoopWorkflow(stateBucket)
	if err := engine.RegisterWorkflow(def); err != nil {
		return fmt.Errorf("register task-execution-loop: %w", err)
	}

	// Coordination loop: parallel planner fan-out/fan-in
	def = BuildCoordinationLoopWorkflow(stateBucket)
	if err := engine.RegisterWorkflow(def); err != nil {
		return fmt.Errorf("register coordination-loop: %w", err)
	}

	// ADR-024: change-proposal-loop (OODA loop for ChangeProposal lifecycle + cascade)
	def = BuildChangeProposalLoopWorkflow(stateBucket)
	if err := engine.RegisterWorkflow(def); err != nil {
		return fmt.Errorf("register change-proposal-loop: %w", err)
	}

	// ADR-025: dag-execution-loop (reactive DAG execution for decompose_task output)
	def = BuildDAGExecutionWorkflow(stateBucket)
	if err := engine.RegisterWorkflow(def); err != nil {
		return fmt.Errorf("register dag-execution-loop: %w", err)
	}

	// ADR-025 Phase 4: scenario-execution-loop (scenario → decompose → DAG execution)
	def = BuildScenarioExecutionWorkflow(stateBucket)
	if err := engine.RegisterWorkflow(def); err != nil {
		return fmt.Errorf("register scenario-execution-loop: %w", err)
	}

	return nil
}
