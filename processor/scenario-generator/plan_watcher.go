package scenariogenerator

// plan_watcher.go — KV twofer self-trigger for scenario generation.
//
// The scenario-generator watches PLAN_STATES for any plan that transitions to
// "requirements_generated" status. This means the plan-manager's KV write IS
// the trigger: no separate NATS publish or workflow step is needed to kick off
// generation.
//
// The existing JetStream consumer (workflow.async.scenario-generator) is kept
// as the primary trigger path and as a backward-compatible fallback for any
// direct dispatches. The KV watcher is additive — it fires when the plan-manager
// KV twofer fires, without needing an explicit downstream publish.
//
// TODO(scenario-kv-trigger): generateFromKVTrigger currently logs a warning
// and returns without dispatching LLM calls because scenario generation
// requires per-requirement data (ID, title, description) that is not carried
// in the PLAN_STATES KV value — only aggregate plan metadata lives there.
//
// Completing the dispatch requires one of:
//   a. A plan.query.requirements NATS request/reply subject served by
//      plan-manager that returns cached requirements for a given slug.
//   b. Enriching the PLAN_STATES KV value with a requirements snapshot
//      (breaks single-responsibility; preferred: option a).
//
// Once option (a) is implemented, replace the TODO block in
// generateFromKVTrigger with a NATS request to plan.query.requirements
// and fan-out one generateScenarios call per requirement.

import (
	"context"
	"encoding/json"

	"github.com/c360studio/semspec/workflow"
	"github.com/nats-io/nats.go/jetstream"
)

// watchPlanStates watches PLAN_STATES for plans transitioning to
// "requirements_generated" and triggers scenario generation inline. Runs until
// ctx is cancelled.
//
// js is obtained once in Start() and passed here to avoid a second JetStream()
// call, matching the pattern used by requirement-generator's plan_watcher.go.
func (c *Component) watchPlanStates(ctx context.Context, js jetstream.JetStream) {
	bucket, err := js.KeyValue(ctx, c.config.PlanStateBucket)
	if err != nil {
		c.logger.Warn("PLAN_STATES bucket not available, will rely on async triggers only",
			"bucket", c.config.PlanStateBucket, "error", err)
		return
	}

	watcher, err := bucket.WatchAll(ctx)
	if err != nil {
		c.logger.Warn("Failed to watch PLAN_STATES, will rely on async triggers only",
			"bucket", c.config.PlanStateBucket, "error", err)
		return
	}
	defer watcher.Stop()

	c.logger.Info("Watching PLAN_STATES for requirements_generated plans",
		"bucket", c.config.PlanStateBucket)

	for entry := range watcher.Updates() {
		// nil entry signals end of initial values replay — skip silently.
		if entry == nil {
			continue
		}

		// Only react to puts; deletes and purges are irrelevant.
		if entry.Operation() != jetstream.KeyValuePut {
			continue
		}

		var plan workflow.Plan
		if err := json.Unmarshal(entry.Value(), &plan); err != nil {
			c.logger.Debug("Skipping unrecognised PLAN_STATES entry",
				"key", entry.Key(), "error", err)
			continue
		}

		if plan.Status != workflow.StatusRequirementsGenerated {
			continue
		}

		if plan.Goal == "" {
			c.logger.Debug("Plan has requirements_generated but goal not set, skipping KV trigger",
				"slug", plan.Slug)
			continue
		}

		c.logger.Info("KV trigger: requirements generated, dispatching scenario generation",
			"slug", plan.Slug)

		// Dispatch in a goroutine so the watcher loop is never blocked.
		go c.generateFromKVTrigger(ctx, &plan)
	}
}

// generateFromKVTrigger is called when the KV watcher sees a plan transition
// to "requirements_generated". It needs per-requirement data (ID, title,
// description) to dispatch individual scenario generation calls.
//
// TODO(scenario-kv-trigger): implement requirement fetch via
// plan.query.requirements NATS request/reply once plan-manager exposes that
// subject. Until then this logs a warning so the gap is visible in production
// logs while the primary JetStream consumer continues to work normally.
func (c *Component) generateFromKVTrigger(_ context.Context, plan *workflow.Plan) {
	c.logger.Warn("KV-triggered scenario generation not yet implemented: "+
		"requires plan.query.requirements subject from plan-manager",
		"slug", plan.Slug,
		"todo", "scenario-kv-trigger")
}
