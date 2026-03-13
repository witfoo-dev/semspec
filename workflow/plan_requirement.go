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

// ValidateRequirementDAG validates that the DependsOn references within the
// provided requirements form a valid directed acyclic graph. It checks that:
//   - All DependsOn entries reference IDs that exist within the slice
//   - No requirement references itself
//   - There are no cycles (detected via DFS with three-color marking)
//
// An empty slice or a slice where no requirement has DependsOn entries is
// always valid. The algorithm is structurally identical to the DAG validation
// in tools/decompose/types.go.
func ValidateRequirementDAG(requirements []Requirement) error {
	// Build an index of requirement IDs for O(1) membership checks.
	idIndex := make(map[string]struct{}, len(requirements))
	for _, r := range requirements {
		idIndex[r.ID] = struct{}{}
	}

	// Validate dependency references and self-references before DFS.
	for _, r := range requirements {
		for _, dep := range r.DependsOn {
			if dep == r.ID {
				return fmt.Errorf("requirement %q depends on itself", r.ID)
			}
			if _, exists := idIndex[dep]; !exists {
				return fmt.Errorf("requirement %q depends on unknown requirement %q", r.ID, dep)
			}
		}
	}

	// Build an adjacency list for cycle detection.
	adj := make(map[string][]string, len(requirements))
	for _, r := range requirements {
		adj[r.ID] = r.DependsOn
	}

	// Detect cycles via recursive DFS with three-color marking:
	//   white (0) = unvisited, gray (1) = in current path, black (2) = done.
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(requirements))

	var visit func(id string) error
	visit = func(id string) error {
		color[id] = gray
		for _, dep := range adj[id] {
			switch color[dep] {
			case gray:
				return fmt.Errorf("cycle detected: requirement %q and requirement %q are in a cycle", id, dep)
			case white:
				if err := visit(dep); err != nil {
					return err
				}
			}
			// black: already fully explored, no cycle through this path
		}
		color[id] = black
		return nil
	}

	for _, r := range requirements {
		if color[r.ID] == white {
			if err := visit(r.ID); err != nil {
				return err
			}
		}
	}

	return nil
}

// SaveRequirements saves requirements to .semspec/projects/default/plans/{slug}/requirements.json.
func (m *Manager) SaveRequirements(ctx context.Context, requirements []Requirement, slug string) error {
	if err := ValidateSlug(slug); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := ValidateRequirementDAG(requirements); err != nil {
		return fmt.Errorf("invalid requirement DAG: %w", err)
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
