package planapi

import (
	"context"
	"encoding/json"

	"github.com/c360studio/semspec/workflow"
	"github.com/c360studio/semspec/workflow/payloads"
	"github.com/c360studio/semstreams/message"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
)

// triggerRequirementGeneration publishes a RequirementGeneratorRequest to JetStream
// after a human approves a plan via POST /promote (round 1).
func (c *Component) triggerRequirementGeneration(ctx context.Context, plan *workflow.Plan) {
	req := &payloads.RequirementGeneratorRequest{
		ExecutionID: uuid.New().String(),
		Slug:        plan.Slug,
		Title:       plan.Title,
		TraceID:     latestTraceID(plan),
	}

	baseMsg := message.NewBaseMessage(req.Schema(), req, "plan-api")
	data, err := json.Marshal(baseMsg)
	if err != nil {
		c.logger.Error("Failed to marshal requirement generator request",
			"slug", plan.Slug, "error", err)
		return
	}

	if err := c.natsClient.PublishToStream(ctx, "workflow.async.requirement-generator", data); err != nil {
		c.logger.Error("Failed to trigger requirement generation",
			"slug", plan.Slug, "error", err)
		return
	}

	c.logger.Info("Triggered requirement generation (human approval)",
		"slug", plan.Slug, "trace_id", req.TraceID)
}

// handleRequirementsGeneratedEvent updates plan status when requirements are generated.
// Orchestration (dispatching scenario generators) is handled by plan-coordinator.
func (c *Component) handleRequirementsGeneratedEvent(ctx context.Context, event *workflow.RequirementsGeneratedEvent) {
	if event.Slug == "" {
		c.logger.Warn("Requirements generated event missing slug")
		return
	}

	manager := c.newManager()
	if manager == nil {
		return
	}

	// Update plan.json status so HTTP API reflects the transition.
	if plan, err := manager.LoadPlan(ctx, event.Slug); err == nil {
		if err := manager.SetPlanStatus(ctx, plan, workflow.StatusRequirementsGenerated); err != nil {
			c.logger.Debug("Failed to transition plan to requirements_generated",
				"slug", event.Slug, "error", err)
		}
	}
}

// handleScenariosGeneratedEvent updates plan status when scenarios are generated.
// Orchestration (dispatching reviewer round 2) is handled by plan-coordinator.
func (c *Component) handleScenariosGeneratedEvent(ctx context.Context, event *workflow.ScenariosGeneratedEvent) {
	if event.Slug == "" {
		c.logger.Warn("Scenarios generated event missing slug")
		return
	}

	manager := c.newManager()
	if manager == nil {
		return
	}

	// Update plan.json status so HTTP API reflects the transition.
	if plan, err := manager.LoadPlan(ctx, event.Slug); err == nil {
		if err := manager.SetPlanStatus(ctx, plan, workflow.StatusScenariosGenerated); err != nil {
			c.logger.Debug("Failed to transition plan to scenarios_generated",
				"slug", event.Slug, "error", err)
		}
	}
}

// latestTraceID extracts the most recent trace ID from a plan's execution history.
func latestTraceID(plan *workflow.Plan) string {
	if len(plan.ExecutionTraceIDs) > 0 {
		return plan.ExecutionTraceIDs[len(plan.ExecutionTraceIDs)-1]
	}
	return ""
}

// Wire requirements/scenarios generated events into the event dispatcher.

func init() {
	// Register the new event types for BaseMessage deserialization.
	// These use simple struct payloads published by the requirement-generator
	// and scenario-generator components.
}

// dispatchCascadeEvent routes cascade events to status-update handlers.
// Called from processWorkflowEvent to handle the cascade subjects.
func (c *Component) dispatchCascadeEvent(ctx context.Context, msg jetstream.Msg) bool {
	switch msg.Subject() {
	case workflow.RequirementsGenerated.Pattern:
		event, err := payloads.ParseReactivePayload[workflow.RequirementsGeneratedEvent](msg.Data())
		if err != nil {
			c.logger.Warn("Failed to parse requirements generated event", "error", err)
			return true
		}
		c.handleRequirementsGeneratedEvent(ctx, event)
		return true

	case workflow.ScenariosGenerated.Pattern:
		event, err := payloads.ParseReactivePayload[workflow.ScenariosGeneratedEvent](msg.Data())
		if err != nil {
			c.logger.Warn("Failed to parse scenarios generated event", "error", err)
			return true
		}
		c.handleScenariosGeneratedEvent(ctx, event)
		return true
	}

	return false
}
