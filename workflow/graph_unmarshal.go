package workflow

import (
	"fmt"
	"strings"
	"time"

	"github.com/c360studio/semspec/vocabulary/semspec"
	"github.com/c360studio/semstreams/graph"
)

// PlanFromEntity reconstructs a Plan from a graph EntityState.
// Uses graph.GetPropertyValueTyped for scalar properties and direct
// triple iteration for relationship predicates (which GetPropertyValue skips).
func PlanFromEntity(entity *graph.EntityState) (*Plan, error) {
	if entity == nil {
		return nil, fmt.Errorf("nil entity")
	}

	plan := &Plan{}

	if v, ok := graph.GetPropertyValueTyped[string](entity, semspec.PlanSlug); ok {
		plan.Slug = v
	}
	if v, ok := graph.GetPropertyValueTyped[string](entity, semspec.PlanTitle); ok {
		plan.Title = v
	}
	if v, ok := graph.GetPropertyValueTyped[string](entity, semspec.PredicatePlanStatus); ok {
		plan.Status = Status(v)
	}
	if v, ok := graph.GetPropertyValueTyped[string](entity, semspec.PlanGoal); ok {
		plan.Goal = v
	}
	if v, ok := graph.GetPropertyValueTyped[string](entity, semspec.PlanContext); ok {
		plan.Context = v
	}
	if v, ok := graph.GetPropertyValueTyped[string](entity, semspec.PlanCreatedAt); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			plan.CreatedAt = t
		}
	}

	// ProjectID is a relationship triple (entity ID as object)
	for _, t := range entity.Triples {
		if t.Predicate == semspec.PlanProject {
			if v, ok := t.Object.(string); ok {
				plan.ProjectID = v
			}
		}
	}

	plan.ID = entity.ID
	return plan, nil
}

// RequirementFromEntity reconstructs a Requirement from a graph EntityState.
func RequirementFromEntity(entity *graph.EntityState) (*Requirement, error) {
	if entity == nil {
		return nil, fmt.Errorf("nil entity")
	}

	req := &Requirement{ID: extractRequirementID(entity.ID)}

	if v, ok := graph.GetPropertyValueTyped[string](entity, semspec.RequirementTitle); ok {
		req.Title = v
	}
	if v, ok := graph.GetPropertyValueTyped[string](entity, semspec.RequirementDescription); ok {
		req.Description = v
	}
	if v, ok := graph.GetPropertyValueTyped[string](entity, semspec.RequirementStatus); ok {
		req.Status = RequirementStatus(v)
	}
	if v, ok := graph.GetPropertyValueTyped[string](entity, semspec.RequirementCreatedAt); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			req.CreatedAt = t
		}
	}
	if v, ok := graph.GetPropertyValueTyped[string](entity, semspec.RequirementUpdatedAt); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			req.UpdatedAt = t
		}
	}

	// Relationship triples: plan and depends_on are entity IDs
	for _, t := range entity.Triples {
		v, ok := t.Object.(string)
		if !ok {
			continue
		}
		switch t.Predicate {
		case semspec.RequirementPlan:
			req.PlanID = v
		case semspec.RequirementDependsOn:
			req.DependsOn = append(req.DependsOn, extractRequirementID(v))
		}
	}

	return req, nil
}

// ScenarioFromEntity reconstructs a Scenario from a graph EntityState.
func ScenarioFromEntity(entity *graph.EntityState) (*Scenario, error) {
	if entity == nil {
		return nil, fmt.Errorf("nil entity")
	}

	s := &Scenario{ID: extractScenarioID(entity.ID)}

	if v, ok := graph.GetPropertyValueTyped[string](entity, semspec.ScenarioGiven); ok {
		s.Given = v
	}
	if v, ok := graph.GetPropertyValueTyped[string](entity, semspec.ScenarioWhen); ok {
		s.When = v
	}
	if v, ok := graph.GetPropertyValueTyped[string](entity, semspec.ScenarioStatus); ok {
		s.Status = ScenarioStatus(v)
	}
	if v, ok := graph.GetPropertyValueTyped[string](entity, semspec.ScenarioCreatedAt); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			s.CreatedAt = t
		}
	}

	// Multi-valued Then + relationship RequirementID
	for _, t := range entity.Triples {
		v, ok := t.Object.(string)
		if !ok {
			continue
		}
		switch t.Predicate {
		case semspec.ScenarioThen:
			s.Then = append(s.Then, v)
		case semspec.ScenarioRequirement:
			s.RequirementID = extractRequirementID(v)
		}
	}

	return s, nil
}

// ChangeProposalFromEntity reconstructs a ChangeProposal from a graph EntityState.
func ChangeProposalFromEntity(entity *graph.EntityState) (*ChangeProposal, error) {
	if entity == nil {
		return nil, fmt.Errorf("nil entity")
	}

	p := &ChangeProposal{ID: extractChangeProposalID(entity.ID)}

	if v, ok := graph.GetPropertyValueTyped[string](entity, semspec.ChangeProposalTitle); ok {
		p.Title = v
	}
	if v, ok := graph.GetPropertyValueTyped[string](entity, semspec.ChangeProposalRationale); ok {
		p.Rationale = v
	}
	if v, ok := graph.GetPropertyValueTyped[string](entity, semspec.ChangeProposalStatus); ok {
		p.Status = ChangeProposalStatus(v)
	}
	if v, ok := graph.GetPropertyValueTyped[string](entity, semspec.ChangeProposalProposedBy); ok {
		p.ProposedBy = v
	}
	if v, ok := graph.GetPropertyValueTyped[string](entity, semspec.ChangeProposalCreatedAt); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			p.CreatedAt = t
		}
	}
	if v, ok := graph.GetPropertyValueTyped[string](entity, semspec.ChangeProposalDecidedAt); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			p.DecidedAt = &t
		}
	}

	// Relationship triples: plan and mutates are entity IDs
	for _, t := range entity.Triples {
		v, ok := t.Object.(string)
		if !ok {
			continue
		}
		switch t.Predicate {
		case semspec.ChangeProposalPlan:
			p.PlanID = v
		case semspec.ChangeProposalMutates:
			p.AffectedReqIDs = append(p.AffectedReqIDs, extractRequirementID(v))
		}
	}

	return p, nil
}

// extractRequirementID extracts the raw requirement ID from an entity ID.
// Entity IDs have format: semspec.local.wf.plan.req.{id}
// Returns the original ID if it doesn't match the expected prefix.
func extractRequirementID(entityID string) string {
	if id, ok := strings.CutPrefix(entityID, EntityPrefix()+".wf.plan.req."); ok {
		return id
	}
	return entityID
}

// extractScenarioID extracts the raw scenario ID from an entity ID.
func extractScenarioID(entityID string) string {
	if id, ok := strings.CutPrefix(entityID, EntityPrefix()+".wf.plan.scenario."); ok {
		return id
	}
	return entityID
}

// extractChangeProposalID extracts the raw change proposal ID from an entity ID.
func extractChangeProposalID(entityID string) string {
	if id, ok := strings.CutPrefix(entityID, EntityPrefix()+".wf.plan.proposal."); ok {
		return id
	}
	return entityID
}
