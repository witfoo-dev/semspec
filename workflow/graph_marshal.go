package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/c360studio/semspec/vocabulary/semspec"
	"github.com/c360studio/semspec/workflow/graphutil"
	"github.com/c360studio/semstreams/message"
)

// writePlanTriples writes all Plan fields as individual triples to ENTITY_STATES.
// Same pattern as execution-orchestrator: one WriteTriple call per field.
func writePlanTriples(ctx context.Context, tw *graphutil.TripleWriter, plan *Plan) error {
	if tw == nil {
		return nil
	}
	entityID := PlanEntityID(plan.Slug)

	// Core identity
	_ = tw.WriteTriple(ctx, entityID, semspec.PlanSlug, plan.Slug)
	_ = tw.WriteTriple(ctx, entityID, semspec.PlanTitle, plan.Title)
	_ = tw.WriteTriple(ctx, entityID, semspec.DCTitle, plan.Title)
	if err := tw.WriteTriple(ctx, entityID, semspec.PredicatePlanStatus, string(plan.EffectiveStatus())); err != nil {
		return fmt.Errorf("write plan status: %w", err)
	}
	_ = tw.WriteTriple(ctx, entityID, semspec.PlanCreatedAt, plan.CreatedAt.Format(time.RFC3339))

	// Project association
	if plan.ProjectID != "" {
		_ = tw.WriteTriple(ctx, entityID, semspec.PlanProject, plan.ProjectID)
	}

	// Plan content
	if plan.Goal != "" {
		_ = tw.WriteTriple(ctx, entityID, semspec.PlanGoal, plan.Goal)
	}
	if plan.Context != "" {
		_ = tw.WriteTriple(ctx, entityID, semspec.PlanContext, plan.Context)
	}

	// Approval
	_ = tw.WriteTriple(ctx, entityID, semspec.PlanApproved, fmt.Sprintf("%t", plan.Approved))
	if plan.ApprovedAt != nil {
		_ = tw.WriteTriple(ctx, entityID, semspec.PlanApprovedAt, plan.ApprovedAt.Format(time.RFC3339))
	}

	// Review
	if plan.ReviewVerdict != "" {
		_ = tw.WriteTriple(ctx, entityID, semspec.PlanReviewVerdict, plan.ReviewVerdict)
	}
	if plan.ReviewSummary != "" {
		_ = tw.WriteTriple(ctx, entityID, semspec.PlanReviewSummary, plan.ReviewSummary)
	}
	if plan.ReviewedAt != nil {
		_ = tw.WriteTriple(ctx, entityID, semspec.PlanReviewedAt, plan.ReviewedAt.Format(time.RFC3339))
	}
	if len(plan.ReviewFindings) > 0 {
		_ = tw.WriteTriple(ctx, entityID, semspec.PlanReviewFindings, string(plan.ReviewFindings))
	}
	if plan.ReviewFormattedFindings != "" {
		_ = tw.WriteTriple(ctx, entityID, semspec.PlanReviewFormattedFindings, plan.ReviewFormattedFindings)
	}
	if plan.ReviewIteration > 0 {
		_ = tw.WriteTriple(ctx, entityID, semspec.PlanReviewIteration, plan.ReviewIteration)
	}

	// Error annotations
	if plan.LastError != "" {
		_ = tw.WriteTriple(ctx, entityID, semspec.PlanLastError, plan.LastError)
	}
	if plan.LastErrorAt != nil {
		_ = tw.WriteTriple(ctx, entityID, semspec.PlanLastErrorAt, plan.LastErrorAt.Format(time.RFC3339))
	}

	// Scope (JSON string)
	if scopeJSON, err := json.Marshal(plan.Scope); err == nil && string(scopeJSON) != "{}" {
		_ = tw.WriteTriple(ctx, entityID, semspec.PlanScope, string(scopeJSON))
	}

	// Execution trace IDs (JSON array)
	if len(plan.ExecutionTraceIDs) > 0 {
		if traceJSON, err := json.Marshal(plan.ExecutionTraceIDs); err == nil {
			_ = tw.WriteTriple(ctx, entityID, semspec.PlanExecutionTraceIDs, string(traceJSON))
		}
	}

	// LLM call history (JSON)
	if plan.LLMCallHistory != nil {
		if historyJSON, err := json.Marshal(plan.LLMCallHistory); err == nil {
			_ = tw.WriteTriple(ctx, entityID, semspec.PlanLLMCallHistory, string(historyJSON))
		}
	}

	return nil
}

// planFromTripleMap reconstructs a Plan from a predicate→value map.
// Same pattern as execution-orchestrator reconciliation.
func planFromTripleMap(entityID string, triples map[string]string) *Plan {
	plan := &Plan{
		ID:   entityID,
		Slug: triples[semspec.PlanSlug],
	}

	if v := triples[semspec.PlanTitle]; v != "" {
		plan.Title = v
	}
	if v := triples[semspec.PredicatePlanStatus]; v != "" {
		plan.Status = Status(v)
	}
	if v := triples[semspec.PlanGoal]; v != "" {
		plan.Goal = v
	}
	if v := triples[semspec.PlanContext]; v != "" {
		plan.Context = v
	}
	if v := triples[semspec.PlanProject]; v != "" {
		plan.ProjectID = v
	}
	if v := triples[semspec.PlanCreatedAt]; v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			plan.CreatedAt = t
		}
	}

	// Approval
	plan.Approved = triples[semspec.PlanApproved] == "true"
	if v := triples[semspec.PlanApprovedAt]; v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			plan.ApprovedAt = &t
		}
	}

	// Review
	plan.ReviewVerdict = triples[semspec.PlanReviewVerdict]
	plan.ReviewSummary = triples[semspec.PlanReviewSummary]
	if v := triples[semspec.PlanReviewedAt]; v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			plan.ReviewedAt = &t
		}
	}
	if v := triples[semspec.PlanReviewFindings]; v != "" {
		plan.ReviewFindings = json.RawMessage(v)
	}
	plan.ReviewFormattedFindings = triples[semspec.PlanReviewFormattedFindings]
	if v := triples[semspec.PlanReviewIteration]; v != "" {
		plan.ReviewIteration, _ = strconv.Atoi(v)
	}

	// Error annotations
	plan.LastError = triples[semspec.PlanLastError]
	if v := triples[semspec.PlanLastErrorAt]; v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			plan.LastErrorAt = &t
		}
	}

	// Scope
	if v := triples[semspec.PlanScope]; v != "" {
		_ = json.Unmarshal([]byte(v), &plan.Scope)
	}

	// Execution trace IDs
	if v := triples[semspec.PlanExecutionTraceIDs]; v != "" {
		_ = json.Unmarshal([]byte(v), &plan.ExecutionTraceIDs)
	}

	// LLM call history
	if v := triples[semspec.PlanLLMCallHistory]; v != "" {
		var history LLMCallHistory
		if err := json.Unmarshal([]byte(v), &history); err == nil {
			plan.LLMCallHistory = &history
		}
	}

	return plan
}

// PlanTriples builds graph triples for a Plan entity as a slice.
// Used by graph_marshal side-effect publishing. For direct writes use writePlanTriples.
func PlanTriples(plan *Plan) []message.Triple {
	entityID := PlanEntityID(plan.Slug)

	triples := []message.Triple{
		{Subject: entityID, Predicate: semspec.PlanTitle, Object: plan.Title},
		{Subject: entityID, Predicate: semspec.PlanSlug, Object: plan.Slug},
		{Subject: entityID, Predicate: semspec.PredicatePlanStatus, Object: string(plan.EffectiveStatus())},
		{Subject: entityID, Predicate: semspec.PlanCreatedAt, Object: plan.CreatedAt.Format(time.RFC3339)},
		{Subject: entityID, Predicate: semspec.DCTitle, Object: plan.Title},
	}

	if plan.Goal != "" {
		triples = append(triples, message.Triple{Subject: entityID, Predicate: semspec.PlanGoal, Object: plan.Goal})
	}
	if plan.Context != "" {
		triples = append(triples, message.Triple{Subject: entityID, Predicate: semspec.PlanContext, Object: plan.Context})
	}
	if plan.ProjectID != "" {
		triples = append(triples, message.Triple{Subject: entityID, Predicate: semspec.PlanProject, Object: plan.ProjectID})
	}

	return triples
}

// RequirementTriples builds graph triples for a Requirement entity.
func RequirementTriples(planSlug string, req *Requirement) []message.Triple {
	entityID := RequirementEntityID(req.ID)
	planEntityID := PlanEntityID(planSlug)

	triples := []message.Triple{
		{Subject: entityID, Predicate: semspec.RequirementTitle, Object: req.Title},
		{Subject: entityID, Predicate: semspec.RequirementStatus, Object: string(req.Status)},
		{Subject: entityID, Predicate: semspec.RequirementPlan, Object: planEntityID},
		{Subject: entityID, Predicate: semspec.RequirementCreatedAt, Object: req.CreatedAt.Format(time.RFC3339)},
		{Subject: entityID, Predicate: semspec.RequirementUpdatedAt, Object: req.UpdatedAt.Format(time.RFC3339)},
		{Subject: entityID, Predicate: semspec.DCTitle, Object: req.Title},
	}

	if req.Description != "" {
		triples = append(triples, message.Triple{Subject: entityID, Predicate: semspec.RequirementDescription, Object: req.Description})
	}

	for _, depID := range req.DependsOn {
		depEntityID := RequirementEntityID(depID)
		triples = append(triples, message.Triple{Subject: entityID, Predicate: semspec.RequirementDependsOn, Object: depEntityID})
	}

	return triples
}

// ScenarioTriples builds graph triples for a Scenario entity.
func ScenarioTriples(planSlug string, s *Scenario) []message.Triple {
	entityID := ScenarioEntityID(s.ID)
	requirementEntityID := RequirementEntityID(s.RequirementID)

	title := s.When
	if len(title) > 100 {
		title = title[:97] + "..."
	}

	triples := []message.Triple{
		{Subject: entityID, Predicate: semspec.ScenarioGiven, Object: s.Given},
		{Subject: entityID, Predicate: semspec.ScenarioWhen, Object: s.When},
		{Subject: entityID, Predicate: semspec.ScenarioStatus, Object: string(s.Status)},
		{Subject: entityID, Predicate: semspec.ScenarioRequirement, Object: requirementEntityID},
		{Subject: entityID, Predicate: semspec.ScenarioCreatedAt, Object: s.CreatedAt.Format(time.RFC3339)},
		{Subject: entityID, Predicate: semspec.DCTitle, Object: title},
	}

	for _, then := range s.Then {
		triples = append(triples, message.Triple{Subject: entityID, Predicate: semspec.ScenarioThen, Object: then})
	}

	_ = planSlug
	return triples
}

// ChangeProposalTriples builds graph triples for a ChangeProposal entity.
func ChangeProposalTriples(planSlug string, p *ChangeProposal) []message.Triple {
	entityID := ChangeProposalEntityID(p.ID)
	planEntityID := PlanEntityID(planSlug)

	title := p.Title
	if len([]rune(title)) > 100 {
		title = string([]rune(title)[:97]) + "..."
	}

	triples := []message.Triple{
		{Subject: entityID, Predicate: semspec.ChangeProposalTitle, Object: p.Title},
		{Subject: entityID, Predicate: semspec.ChangeProposalStatus, Object: string(p.Status)},
		{Subject: entityID, Predicate: semspec.ChangeProposalProposedBy, Object: p.ProposedBy},
		{Subject: entityID, Predicate: semspec.ChangeProposalPlan, Object: planEntityID},
		{Subject: entityID, Predicate: semspec.ChangeProposalCreatedAt, Object: p.CreatedAt.Format(time.RFC3339)},
		{Subject: entityID, Predicate: semspec.DCTitle, Object: title},
	}

	if p.Rationale != "" {
		triples = append(triples, message.Triple{Subject: entityID, Predicate: semspec.ChangeProposalRationale, Object: p.Rationale})
	}
	if p.DecidedAt != nil {
		triples = append(triples, message.Triple{Subject: entityID, Predicate: semspec.ChangeProposalDecidedAt, Object: p.DecidedAt.Format(time.RFC3339)})
	}

	for _, reqID := range p.AffectedReqIDs {
		requirementEntityID := RequirementEntityID(reqID)
		triples = append(triples, message.Triple{Subject: entityID, Predicate: semspec.ChangeProposalMutates, Object: requirementEntityID})
	}

	return triples
}
