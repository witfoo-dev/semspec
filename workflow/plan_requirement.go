package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// RequirementsJSONFile is the filename for machine-readable requirement storage (JSON format).
const RequirementsJSONFile = "requirements.json"

// SaveRequirements saves requirements to .semspec/projects/default/plans/{slug}/requirements.json.
func (m *Manager) SaveRequirements(ctx context.Context, requirements []Requirement, slug string) error {
	if err := ValidateSlug(slug); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	reqPath := filepath.Join(m.ProjectPlanPath(DefaultProjectSlug, slug), RequirementsJSONFile)

	dir := filepath.Dir(reqPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(requirements, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal requirements: %w", err)
	}

	if err := os.WriteFile(reqPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write requirements: %w", err)
	}

	return nil
}

// LoadRequirements loads requirements from .semspec/projects/default/plans/{slug}/requirements.json.
func (m *Manager) LoadRequirements(ctx context.Context, slug string) ([]Requirement, error) {
	if err := ValidateSlug(slug); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	reqPath := filepath.Join(m.ProjectPlanPath(DefaultProjectSlug, slug), RequirementsJSONFile)

	data, err := os.ReadFile(reqPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Requirement{}, nil
		}
		return nil, fmt.Errorf("failed to read requirements: %w", err)
	}

	var requirements []Requirement
	if err := json.Unmarshal(data, &requirements); err != nil {
		return nil, fmt.Errorf("failed to parse requirements: %w", err)
	}

	return requirements, nil
}
