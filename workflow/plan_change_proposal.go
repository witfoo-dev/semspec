package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/c360studio/semspec/vocabulary/semspec"
	"github.com/c360studio/semspec/workflow/graphutil"
)

// ChangeProposalsJSONFile is the filename for machine-readable change proposal storage (JSON format).
const ChangeProposalsJSONFile = "change_proposals.json"

// SaveChangeProposals saves change proposals to ENTITY_STATES as triples.
// Each proposal is stored as a separate entity keyed by ChangeProposalEntityID.
// Multi-valued fields (AffectedReqIDs) are stored as JSON arrays.
func SaveChangeProposals(ctx context.Context, tw *graphutil.TripleWriter, proposals []ChangeProposal, slug string) error {
	if err := ValidateSlug(slug); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	planEntityID := PlanEntityID(slug)
	for i := range proposals {
		if proposals[i].PlanID == "" {
			proposals[i].PlanID = planEntityID
		}
		if err := writeChangeProposalTriples(ctx, tw, &proposals[i]); err != nil {
			return fmt.Errorf("save change proposal %s: %w", proposals[i].ID, err)
		}
	}

	return nil
}

// writeChangeProposalTriples writes all ChangeProposal fields as individual triples.
func writeChangeProposalTriples(ctx context.Context, tw *graphutil.TripleWriter, p *ChangeProposal) error {
	if tw == nil {
		return nil
	}
	entityID := ChangeProposalEntityID(p.ID)

	title := p.Title
	if len([]rune(title)) > 100 {
		title = string([]rune(title)[:97]) + "..."
	}

	_ = tw.WriteTriple(ctx, entityID, semspec.ChangeProposalTitle, p.Title)
	_ = tw.WriteTriple(ctx, entityID, semspec.DCTitle, title)
	if err := tw.WriteTriple(ctx, entityID, semspec.ChangeProposalStatus, string(p.Status)); err != nil {
		return fmt.Errorf("write change proposal status: %w", err)
	}
	_ = tw.WriteTriple(ctx, entityID, semspec.ChangeProposalProposedBy, p.ProposedBy)
	_ = tw.WriteTriple(ctx, entityID, semspec.ChangeProposalPlan, p.PlanID)
	_ = tw.WriteTriple(ctx, entityID, semspec.ChangeProposalCreatedAt, p.CreatedAt.Format(time.RFC3339))

	if p.Rationale != "" {
		_ = tw.WriteTriple(ctx, entityID, semspec.ChangeProposalRationale, p.Rationale)
	}
	if p.DecidedAt != nil {
		_ = tw.WriteTriple(ctx, entityID, semspec.ChangeProposalDecidedAt, p.DecidedAt.Format(time.RFC3339))
	}

	// Store AffectedReqIDs as JSON array to avoid multi-value collapse.
	if affectedJSON, err := json.Marshal(p.AffectedReqIDs); err == nil {
		_ = tw.WriteTriple(ctx, entityID, semspec.ChangeProposalMutates, string(affectedJSON))
	}

	return nil
}

// changeProposalFromTripleMap reconstructs a ChangeProposal from a predicate→value map.
func changeProposalFromTripleMap(entityID string, triples map[string]string) ChangeProposal {
	p := ChangeProposal{
		ID:     extractChangeProposalID(entityID),
		PlanID: triples[semspec.ChangeProposalPlan],
	}

	if v := triples[semspec.ChangeProposalTitle]; v != "" {
		p.Title = v
	}
	if v := triples[semspec.ChangeProposalStatus]; v != "" {
		p.Status = ChangeProposalStatus(v)
	}
	if v := triples[semspec.ChangeProposalProposedBy]; v != "" {
		p.ProposedBy = v
	}
	if v := triples[semspec.ChangeProposalRationale]; v != "" {
		p.Rationale = v
	}
	if v := triples[semspec.ChangeProposalCreatedAt]; v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			p.CreatedAt = t
		}
	}
	if v := triples[semspec.ChangeProposalDecidedAt]; v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			p.DecidedAt = &t
		}
	}
	// AffectedReqIDs stored as JSON array of raw requirement IDs.
	if v := triples[semspec.ChangeProposalMutates]; v != "" {
		_ = json.Unmarshal([]byte(v), &p.AffectedReqIDs)
	}
	if p.AffectedReqIDs == nil {
		p.AffectedReqIDs = []string{}
	}

	return p
}

// extractChangeProposalID extracts the raw change proposal ID from the entity ID.
// Entity ID format: {prefix}.wf.plan.proposal.{id}
func extractChangeProposalID(entityID string) string {
	prefix := EntityPrefix() + ".wf.plan.proposal."
	if len(entityID) > len(prefix) {
		return entityID[len(prefix):]
	}
	return entityID
}

// LoadChangeProposals loads change proposals for a plan from ENTITY_STATES triples.
func LoadChangeProposals(ctx context.Context, tw *graphutil.TripleWriter, slug string) ([]ChangeProposal, error) {
	if err := ValidateSlug(slug); err != nil {
		return nil, err
	}

	if tw == nil {
		return []ChangeProposal{}, nil
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	prefix := EntityPrefix() + ".wf.plan.proposal."
	entities, err := tw.ReadEntitiesByPrefix(ctx, prefix, 500)
	if err != nil {
		return []ChangeProposal{}, nil
	}

	planEntityID := PlanEntityID(slug)
	var proposals []ChangeProposal

	for entityID, triples := range entities {
		p := changeProposalFromTripleMap(entityID, triples)
		if p.PlanID == planEntityID {
			proposals = append(proposals, p)
		}
	}

	if proposals == nil {
		proposals = []ChangeProposal{}
	}

	return proposals, nil
}
