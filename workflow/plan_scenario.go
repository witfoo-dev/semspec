package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ScenariosJSONFile is the filename for machine-readable scenario storage (JSON format).
const ScenariosJSONFile = "scenarios.json"

// SaveScenarios saves scenarios to .semspec/projects/default/plans/{slug}/scenarios.json.
func (m *Manager) SaveScenarios(ctx context.Context, scenarios []Scenario, slug string) error {
	if err := ValidateSlug(slug); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	scenPath := filepath.Join(m.ProjectPlanPath(DefaultProjectSlug, slug), ScenariosJSONFile)

	dir := filepath.Dir(scenPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(scenarios, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal scenarios: %w", err)
	}

	if err := os.WriteFile(scenPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write scenarios: %w", err)
	}

	return nil
}

// LoadScenarios loads scenarios from .semspec/projects/default/plans/{slug}/scenarios.json.
func (m *Manager) LoadScenarios(ctx context.Context, slug string) ([]Scenario, error) {
	if err := ValidateSlug(slug); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	scenPath := filepath.Join(m.ProjectPlanPath(DefaultProjectSlug, slug), ScenariosJSONFile)

	data, err := os.ReadFile(scenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Scenario{}, nil
		}
		return nil, fmt.Errorf("failed to read scenarios: %w", err)
	}

	var scenarios []Scenario
	if err := json.Unmarshal(data, &scenarios); err != nil {
		return nil, fmt.Errorf("failed to parse scenarios: %w", err)
	}

	return scenarios, nil
}
