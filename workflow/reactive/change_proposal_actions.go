package reactive

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	workflow "github.com/c360studio/semspec/workflow"
	"github.com/c360studio/semspec/workflow/phases"
	"github.com/nats-io/nats.go/jetstream"
)

// cascadeStateStore is the minimal KV interface needed by CascadeExecutor.
// Using an interface instead of the full jetstream.KeyValue keeps tests simple.
type cascadeStateStore interface {
	Get(ctx context.Context, key string) (jetstream.KeyValueEntry, error)
	Update(ctx context.Context, key string, value []byte, revision uint64) (uint64, error)
	Put(ctx context.Context, key string, value []byte) (uint64, error)
}

// ---------------------------------------------------------------------------
// CascadeExecutor
// ---------------------------------------------------------------------------

// CascadeExecutor performs the cascade logic for an accepted ChangeProposal.
// It traverses Requirement → Scenario → Task edges via the filesystem-backed
// Manager, marks affected tasks dirty, updates the proposal to archived, and
// transitions the reactive workflow KV state to cascade_complete.
//
// This is invoked by the change-proposal-cascade processor component when it
// receives a message on workflow.async.change-proposal-cascade.
//
// The cascade follows the ADR-024 spec (handle-accepted rule action):
//  1. Load ChangeProposal to get AffectedReqIDs.
//  2. For each Requirement in AffectedReqIDs, find Scenarios (HAS_SCENARIO edge).
//  3. For each Scenario, find Tasks whose ScenarioIDs contain the scenario (SATISFIED_BY edge).
//  4. Set each affected Task status to dirty and persist.
//  5. Archive the ChangeProposal.
//  6. Transition KV state to cascade_complete with AffectedTaskIDs populated.
type CascadeExecutor struct {
	manager     *workflow.Manager
	stateBucket cascadeStateStore
	logger      *slog.Logger
}

// NewCascadeExecutor creates a CascadeExecutor with the given dependencies.
// The stateBucket must support Get, Put, and Update operations (a subset of jetstream.KeyValue).
func NewCascadeExecutor(manager *workflow.Manager, stateBucket cascadeStateStore, logger *slog.Logger) *CascadeExecutor {
	return &CascadeExecutor{
		manager:     manager,
		stateBucket: stateBucket,
		logger:      logger,
	}
}

// NewCascadeExecutorFromKV creates a CascadeExecutor from a full jetstream.KeyValue bucket.
// Use this in production code; use NewCascadeExecutor for tests with a minimal mock.
func NewCascadeExecutorFromKV(manager *workflow.Manager, stateBucket jetstream.KeyValue, logger *slog.Logger) *CascadeExecutor {
	return NewCascadeExecutor(manager, stateBucket, logger)
}

// Execute runs the cascade for the given request.
func (e *CascadeExecutor) Execute(ctx context.Context, req *ChangeProposalCascadeRequest) error {
	slug := req.Slug

	// Step 1: Load the ChangeProposal.
	proposals, err := e.manager.LoadChangeProposals(ctx, slug)
	if err != nil {
		return fmt.Errorf("load change proposals: %w", err)
	}

	proposalIdx := -1
	for i := range proposals {
		if proposals[i].ID == req.ProposalID {
			proposalIdx = i
			break
		}
	}
	if proposalIdx == -1 {
		return fmt.Errorf("change proposal %q not found in plan %q", req.ProposalID, slug)
	}
	proposal := &proposals[proposalIdx]

	// Step 2: Build a set of affected requirement IDs for fast lookup.
	affectedReqs := make(map[string]bool, len(proposal.AffectedReqIDs))
	for _, id := range proposal.AffectedReqIDs {
		affectedReqs[id] = true
	}

	// Step 3: Load all scenarios and filter to those belonging to affected requirements
	// (HAS_SCENARIO edge: Requirement → Scenario via Scenario.RequirementID).
	allScenarios, err := e.manager.LoadScenarios(ctx, slug)
	if err != nil {
		return fmt.Errorf("load scenarios: %w", err)
	}

	affectedScenarioIDs := make(map[string]bool)
	for _, sc := range allScenarios {
		if affectedReqs[sc.RequirementID] {
			affectedScenarioIDs[sc.ID] = true
		}
	}

	if len(affectedScenarioIDs) == 0 {
		e.logger.Warn("cascade: no scenarios found for affected requirements; nothing to dirty",
			"proposal_id", req.ProposalID,
			"affected_req_count", len(proposal.AffectedReqIDs),
		)
	}

	// Step 4: Load all tasks and find those with overlapping ScenarioIDs
	// (SATISFIED_BY edge: Scenario → Task via Task.ScenarioIDs).
	tasks, err := e.manager.LoadTasks(ctx, slug)
	if err != nil {
		return fmt.Errorf("load tasks: %w", err)
	}

	now := time.Now()
	var affectedTaskIDs []string

	for i := range tasks {
		if isTaskAffected(&tasks[i], affectedScenarioIDs) {
			if tasks[i].Status.CanTransitionTo(workflow.TaskStatusDirty) {
				tasks[i].Status = workflow.TaskStatusDirty
				affectedTaskIDs = append(affectedTaskIDs, tasks[i].ID)
			}
		}
	}

	// Step 5: Persist updated tasks (if any were dirtied).
	if len(affectedTaskIDs) > 0 {
		if err := e.manager.SaveTasks(ctx, tasks, slug); err != nil {
			return fmt.Errorf("save dirty tasks: %w", err)
		}
		e.logger.Info("cascade: marked tasks dirty",
			"proposal_id", req.ProposalID,
			"dirty_count", len(affectedTaskIDs),
			"task_ids", affectedTaskIDs,
		)
	}

	// Step 6: Archive the ChangeProposal.
	if proposals[proposalIdx].Status.CanTransitionTo(workflow.ChangeProposalStatusArchived) {
		proposals[proposalIdx].Status = workflow.ChangeProposalStatusArchived
		proposals[proposalIdx].DecidedAt = &now
	}

	if err := e.manager.SaveChangeProposals(ctx, proposals, slug); err != nil {
		return fmt.Errorf("archive change proposal: %w", err)
	}

	// Step 7: Transition KV state to cascade_complete with affected task IDs.
	stateKey := "change-proposal." + slug + "." + req.ProposalID
	if err := e.transitionToCascadeComplete(ctx, stateKey, affectedTaskIDs); err != nil {
		return fmt.Errorf("transition to cascade_complete: %w", err)
	}

	e.logger.Info("cascade: complete",
		"proposal_id", req.ProposalID,
		"slug", slug,
		"dirty_count", len(affectedTaskIDs),
	)

	return nil
}

// transitionToCascadeComplete reads the current KV state, sets the phase to
// cascade_complete and stores the affected task IDs, then writes it back with
// optimistic concurrency via the entry revision. Retries up to 3 times on
// revision conflict (another writer updated the key between Get and Update).
func (e *CascadeExecutor) transitionToCascadeComplete(ctx context.Context, stateKey string, affectedTaskIDs []string) error {
	const maxRetries = 3
	for attempt := range maxRetries {
		entry, err := e.stateBucket.Get(ctx, stateKey)
		if err != nil {
			return fmt.Errorf("get workflow state %q: %w", stateKey, err)
		}

		var state ChangeProposalState
		if err := json.Unmarshal(entry.Value(), &state); err != nil {
			return fmt.Errorf("unmarshal state: %w", err)
		}

		state.Phase = phases.ChangeProposalCascadeComplete
		state.AffectedTaskIDs = affectedTaskIDs
		state.UpdatedAt = time.Now()

		stateData, err := json.Marshal(state)
		if err != nil {
			return fmt.Errorf("marshal state: %w", err)
		}

		if _, err := e.stateBucket.Update(ctx, stateKey, stateData, entry.Revision()); err != nil {
			if attempt < maxRetries-1 {
				e.logger.Warn("KV revision conflict, retrying",
					"key", stateKey, "attempt", attempt+1)
				continue
			}
			return fmt.Errorf("update state after %d attempts: %w", maxRetries, err)
		}
		return nil
	}
	return nil // unreachable
}

// transitionToFailure marks the workflow state as cascade_failed.
// Called by the cascade component when Execute returns an error.
// Retries up to 3 times on revision conflict.
func (e *CascadeExecutor) transitionToFailure(ctx context.Context, stateKey, errMsg string) error {
	const maxRetries = 3
	for attempt := range maxRetries {
		entry, err := e.stateBucket.Get(ctx, stateKey)
		if err != nil {
			return fmt.Errorf("get workflow state %q: %w", stateKey, err)
		}

		var state ChangeProposalState
		if err := json.Unmarshal(entry.Value(), &state); err != nil {
			return fmt.Errorf("unmarshal state: %w", err)
		}

		state.Phase = phases.ChangeProposalCascadeFailed
		state.Error = errMsg
		state.UpdatedAt = time.Now()

		stateData, err := json.Marshal(state)
		if err != nil {
			return fmt.Errorf("marshal state: %w", err)
		}

		if _, err := e.stateBucket.Update(ctx, stateKey, stateData, entry.Revision()); err != nil {
			if attempt < maxRetries-1 {
				e.logger.Warn("KV revision conflict on failure transition, retrying",
					"key", stateKey, "attempt", attempt+1)
				continue
			}
			return fmt.Errorf("update failure state after %d attempts: %w", maxRetries, err)
		}
		return nil
	}
	return nil // unreachable
}

// isTaskAffected returns true if any of the task's ScenarioIDs are in the affected set.
func isTaskAffected(task *workflow.Task, affectedScenarioIDs map[string]bool) bool {
	for _, sid := range task.ScenarioIDs {
		if affectedScenarioIDs[sid] {
			return true
		}
	}
	return false
}
