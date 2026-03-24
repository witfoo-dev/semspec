package planapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/c360studio/semspec/workflow"
)

// ---------------------------------------------------------------------------
// Retrospective view (reactive execution mode)
// ---------------------------------------------------------------------------

// RetrospectiveCompletedTask is a completed task entry in the retrospective view.
type RetrospectiveCompletedTask struct {
	// TaskID is the full entity ID of the completed task.
	TaskID string `json:"task_id"`
	// Prompt is the task description (prompt text).
	Prompt string `json:"prompt"`
	// CompletedAt is when the task finished execution.
	CompletedAt *string `json:"completed_at,omitempty"`
}

// RetrospectiveScenario groups completed tasks under a scenario.
type RetrospectiveScenario struct {
	// ScenarioID is the scenario entity ID.
	ScenarioID string `json:"scenario_id"`
	// ScenarioTitle is a human-readable title derived from the Given/When/Then fields.
	ScenarioTitle string `json:"scenario_title"`
	// CompletedTasks lists tasks that completed while satisfying this scenario.
	CompletedTasks []RetrospectiveCompletedTask `json:"completed_tasks"`
}

// RetrospectivePhase represents one requirement with its nested scenario groups.
type RetrospectivePhase struct {
	// RequirementID is the requirement entity ID.
	RequirementID string `json:"requirement_id"`
	// RequirementTitle is the requirement title.
	RequirementTitle string `json:"requirement_title"`
	// Scenarios lists the scenarios that belong to this requirement.
	Scenarios []RetrospectiveScenario `json:"scenarios"`
}

// RetrospectiveResponse is the full response body for GET /plans/{slug}/phases/retrospective.
type RetrospectiveResponse struct {
	// PlanSlug is the slug of the plan.
	PlanSlug string `json:"plan_slug"`
	// Phases groups completed work by Requirement → Scenario → Task.
	Phases []RetrospectivePhase `json:"phases"`
}

// handlePhasesRetrospective handles GET /plans/{slug}/phases/retrospective.
//
// This endpoint is the reactive-mode equivalent of the classic phases view. Instead of
// showing pre-generated phases and tasks, it reconstructs a retrospective grouping of
// completed work from the filesystem:
//
//  1. Load requirements for the plan
//  2. Load scenarios; group them by parent requirement
//  3. Load completed tasks; correlate each task to its scenario(s) via task.ScenarioIDs
//  4. Return the nested structure: Requirement → Scenario → completed Task
//
// Tasks without a ScenarioID are omitted from the retrospective view because they cannot
// be attributed to a specific scenario (they predate reactive mode or were created
// manually outside the scenario pipeline).
func (c *Component) handlePhasesRetrospective(w http.ResponseWriter, r *http.Request, slug string) {
	manager := c.getManager(w)
	if manager == nil {
		return
	}

	ctx := r.Context()
	c.mu.RLock()
	kvStore := c.kvStore
	c.mu.RUnlock()

	// Verify the plan exists.
	if !workflow.PlanExists(ctx, kvStore, slug) {
		http.Error(w, "Plan not found", http.StatusNotFound)
		return
	}

	// Load requirements.
	requirements, err := workflow.LoadRequirements(ctx, kvStore, slug)
	if err != nil {
		c.logger.Error("Failed to load requirements for retrospective", "slug", slug, "error", err)
		http.Error(w, "Failed to load requirements", http.StatusInternalServerError)
		return
	}

	// Load scenarios and build a lookup: requirementID → []Scenario.
	scenarios, err := workflow.LoadScenarios(ctx, kvStore, slug)
	if err != nil {
		c.logger.Error("Failed to load scenarios for retrospective", "slug", slug, "error", err)
		http.Error(w, "Failed to load scenarios", http.StatusInternalServerError)
		return
	}

	scenariosByReq := make(map[string][]workflow.Scenario, len(requirements))
	for _, s := range scenarios {
		scenariosByReq[s.RequirementID] = append(scenariosByReq[s.RequirementID], s)
	}

	// Load tasks and build a lookup: scenarioID → []completed Task.
	tasks, err := manager.LoadTasks(ctx, slug)
	if err != nil {
		c.logger.Error("Failed to load tasks for retrospective", "slug", slug, "error", err)
		http.Error(w, "Failed to load tasks", http.StatusInternalServerError)
		return
	}

	completedByScenario := make(map[string][]RetrospectiveCompletedTask)
	for _, t := range tasks {
		if t.Status != workflow.TaskStatusCompleted {
			continue
		}
		for _, sid := range t.ScenarioIDs {
			var completedAt *string
			if t.CompletedAt != nil {
				s := t.CompletedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
				completedAt = &s
			}
			completedByScenario[sid] = append(completedByScenario[sid], RetrospectiveCompletedTask{
				TaskID:      t.ID,
				Prompt:      t.Description,
				CompletedAt: completedAt,
			})
		}
	}

	// Build the response by iterating requirements in order.
	phases := make([]RetrospectivePhase, 0, len(requirements))
	for _, req := range requirements {
		reqScenarios := scenariosByReq[req.ID]
		scenarioGroups := make([]RetrospectiveScenario, 0, len(reqScenarios))
		for _, scen := range reqScenarios {
			title := strings.Join(scen.Then, "; ")
			if title == "" {
				title = fmt.Sprintf("Given %s, when %s", scen.Given, scen.When)
			}
			scenarioGroups = append(scenarioGroups, RetrospectiveScenario{
				ScenarioID:     scen.ID,
				ScenarioTitle:  title,
				CompletedTasks: completedByScenario[scen.ID],
			})
		}
		phases = append(phases, RetrospectivePhase{
			RequirementID:    req.ID,
			RequirementTitle: req.Title,
			Scenarios:        scenarioGroups,
		})
	}

	resp := RetrospectiveResponse{
		PlanSlug: slug,
		Phases:   phases,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		c.logger.Warn("Failed to encode retrospective response", "error", err)
	}
}
