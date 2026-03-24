package workflow

import (
	"time"

	"github.com/c360studio/semspec/vocabulary/semspec"
	"github.com/c360studio/semstreams/message"
)

// PlanTriples builds graph triples for a Plan entity.
// Extracted from plan-api/graph.go:publishPlanEntity.
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
// Extracted from plan-api/graph.go:publishRequirementEntity.
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
// Extracted from plan-api/graph.go:publishScenarioEntity.
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

	_ = planSlug // available for future plan-scoped graph prefixes
	return triples
}

// ChangeProposalTriples builds graph triples for a ChangeProposal entity.
// Extracted from plan-api/graph.go:publishChangeProposalEntity.
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
