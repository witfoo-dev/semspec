package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ChangeProposalsJSONFile is the filename for machine-readable change proposal storage (JSON format).
const ChangeProposalsJSONFile = "change_proposals.json"

// SaveChangeProposals saves change proposals to .semspec/projects/default/plans/{slug}/change_proposals.json.
func (m *Manager) SaveChangeProposals(ctx context.Context, proposals []ChangeProposal, slug string) error {
	if err := ValidateSlug(slug); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	proposalPath := filepath.Join(m.ProjectPlanPath(DefaultProjectSlug, slug), ChangeProposalsJSONFile)

	dir := filepath.Dir(proposalPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(proposals, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal change proposals: %w", err)
	}

	if err := os.WriteFile(proposalPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write change proposals: %w", err)
	}

	return nil
}

// LoadChangeProposals loads change proposals from .semspec/projects/default/plans/{slug}/change_proposals.json.
func (m *Manager) LoadChangeProposals(ctx context.Context, slug string) ([]ChangeProposal, error) {
	if err := ValidateSlug(slug); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	proposalPath := filepath.Join(m.ProjectPlanPath(DefaultProjectSlug, slug), ChangeProposalsJSONFile)

	data, err := os.ReadFile(proposalPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []ChangeProposal{}, nil
		}
		return nil, fmt.Errorf("failed to read change proposals: %w", err)
	}

	var proposals []ChangeProposal
	if err := json.Unmarshal(data, &proposals); err != nil {
		return nil, fmt.Errorf("failed to parse change proposals: %w", err)
	}

	return proposals, nil
}
