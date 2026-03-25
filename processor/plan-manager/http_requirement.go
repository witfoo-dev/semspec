package planmanager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/c360studio/semspec/workflow"
)

// Requirement HTTP request/response types

// CreateRequirementHTTPRequest is the HTTP request body for POST /plans/{slug}/requirements.
type CreateRequirementHTTPRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	DependsOn   []string `json:"depends_on,omitempty"`
}

// UpdateRequirementHTTPRequest is the HTTP request body for PATCH /plans/{slug}/requirements/{reqId}.
type UpdateRequirementHTTPRequest struct {
	Title       *string  `json:"title,omitempty"`
	Description *string  `json:"description,omitempty"`
	DependsOn   []string `json:"depends_on,omitempty"`
}

// extractSlugRequirementAndAction extracts slug, requirementID, and action from paths like:
// /plan-api/plans/{slug}/requirements/{reqId}
// /plan-api/plans/{slug}/requirements/{reqId}/deprecate
func extractSlugRequirementAndAction(path string) (slug, requirementID, action string) {
	idx := strings.Index(path, "/plans/")
	if idx == -1 {
		return "", "", ""
	}

	remainder := path[idx+len("/plans/"):]
	parts := strings.Split(strings.TrimSuffix(remainder, "/"), "/")

	// Need at least 3 parts: slug, "requirements", requirementID
	if len(parts) < 3 {
		return "", "", ""
	}

	if parts[1] != "requirements" {
		return "", "", ""
	}

	slug = parts[0]
	requirementID = parts[2]

	if len(parts) > 3 {
		action = parts[3]
	}

	return slug, requirementID, action
}

// handlePlanRequirements handles top-level requirement collection endpoints.
func (c *Component) handlePlanRequirements(w http.ResponseWriter, r *http.Request, slug string) {
	switch r.Method {
	case http.MethodGet:
		c.handleListRequirements(w, r, slug)
	case http.MethodPost:
		c.handleCreateRequirement(w, r, slug)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleRequirementByID handles requirement-specific endpoints: GET, PATCH, DELETE, and actions.
func (c *Component) handleRequirementByID(w http.ResponseWriter, r *http.Request, slug, requirementID, action string) {
	switch action {
	case "":
		switch r.Method {
		case http.MethodGet:
			c.handleGetRequirement(w, r, slug, requirementID)
		case http.MethodPatch:
			c.handleUpdateRequirement(w, r, slug, requirementID)
		case http.MethodDelete:
			c.handleDeleteRequirement(w, r, slug, requirementID)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	case "deprecate":
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		c.handleDeprecateRequirement(w, r, slug, requirementID)
	case "scenarios":
		c.handleListScenariosByRequirement(w, r, slug, requirementID)
	default:
		http.Error(w, "Unknown endpoint", http.StatusNotFound)
	}
}

// handleListRequirements handles GET /plans/{slug}/requirements.
// Reads from the requirement cache — never hits the graph.
func (c *Component) handleListRequirements(w http.ResponseWriter, _ *http.Request, slug string) {
	requirements := c.requirements.listByPlan(slug)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(requirements); err != nil {
		c.logger.Warn("Failed to encode response", "error", err)
	}
}

// handleGetRequirement handles GET /plans/{slug}/requirements/{reqId}.
// O(1) cache lookup by ID.
func (c *Component) handleGetRequirement(w http.ResponseWriter, _ *http.Request, _, requirementID string) {
	req, ok := c.requirements.get(requirementID)
	if !ok {
		http.Error(w, "Requirement not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(req); err != nil {
		c.logger.Warn("Failed to encode response", "error", err)
	}
}

// handleCreateRequirement handles POST /plans/{slug}/requirements.
// Validates DAG with existing requirements + the new one, then writes a single entity.
func (c *Component) handleCreateRequirement(w http.ResponseWriter, r *http.Request, slug string) {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodySize)

	var req CreateRequirementHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	existing := c.requirements.listByPlan(slug)

	now := time.Now()
	id := fmt.Sprintf("requirement.%s.%d", slug, len(existing)+1)

	newReq := workflow.Requirement{
		ID:          id,
		PlanID:      workflow.PlanEntityID(slug),
		Title:       req.Title,
		Description: req.Description,
		Status:      workflow.RequirementStatusActive,
		DependsOn:   req.DependsOn,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Validate DAG with existing + new requirement.
	if len(req.DependsOn) > 0 {
		candidate := append(existing, newReq)
		if err := workflow.ValidateRequirementDAG(candidate); err != nil {
			writeJSONError(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
	}

	// Single entity write.
	if err := c.requirements.save(r.Context(), &newReq); err != nil {
		c.logger.Error("Failed to save requirement", "slug", slug, "error", err)
		http.Error(w, "Failed to save requirement", http.StatusInternalServerError)
		return
	}

	c.logger.Info("Requirement created via REST API", "slug", slug, "requirement_id", newReq.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(newReq); err != nil {
		c.logger.Warn("Failed to encode response", "error", err)
	}
}

// handleUpdateRequirement handles PATCH /plans/{slug}/requirements/{reqId}.
// Updates a single entity. DAG validation only when dependencies change.
func (c *Component) handleUpdateRequirement(w http.ResponseWriter, r *http.Request, slug, requirementID string) {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodySize)

	var req UpdateRequirementHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	existing, ok := c.requirements.get(requirementID)
	if !ok {
		http.Error(w, "Requirement not found", http.StatusNotFound)
		return
	}

	if req.Title != nil {
		existing.Title = *req.Title
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	depsChanged := req.DependsOn != nil
	if depsChanged {
		existing.DependsOn = req.DependsOn
	}
	existing.UpdatedAt = time.Now()

	// Validate DAG only when dependencies changed.
	if depsChanged {
		all := c.requirements.listByPlan(slug)
		// Replace this requirement in the list for validation.
		for i, r := range all {
			if r.ID == requirementID {
				all[i] = *existing
				break
			}
		}
		if err := workflow.ValidateRequirementDAG(all); err != nil {
			writeJSONError(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
	}

	// Single entity write.
	if err := c.requirements.save(r.Context(), existing); err != nil {
		c.logger.Error("Failed to save requirement", "slug", slug, "error", err)
		http.Error(w, "Failed to save requirement", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(existing); err != nil {
		c.logger.Warn("Failed to encode response", "error", err)
	}
}

// handleDeleteRequirement handles DELETE /plans/{slug}/requirements/{reqId}.
// Cascade: deletes transitive dependents and their scenarios.
func (c *Component) handleDeleteRequirement(w http.ResponseWriter, r *http.Request, slug, requirementID string) {
	if _, ok := c.requirements.get(requirementID); !ok {
		http.Error(w, "Requirement not found", http.StatusNotFound)
		return
	}

	// Compute blast radius from cache.
	all := c.requirements.listByPlan(slug)
	toRemove := requirementBlastRadius(all, requirementID)

	// Delete each requirement in the blast radius.
	for id := range toRemove {
		if err := c.requirements.delete(r.Context(), id); err != nil {
			c.logger.Error("Failed to delete requirement", "id", id, "error", err)
			http.Error(w, "Failed to delete requirement", http.StatusInternalServerError)
			return
		}
	}

	// Cascade: delete scenarios for all removed requirements.
	scenarios := c.scenarios.listByPlan(slug, c.requirements)
	for i := range scenarios {
		if toRemove[scenarios[i].RequirementID] {
			_ = c.scenarios.delete(r.Context(), scenarios[i].ID)
		}
	}

	c.logger.Info("Deleted requirement with cascade",
		"slug", slug,
		"requirement_id", requirementID,
		"removed_count", len(toRemove))

	w.WriteHeader(http.StatusNoContent)
}

// requirementBlastRadius computes the set of requirement IDs that would be
// affected by removing the given root ID. This includes the root itself plus
// any requirements that transitively depend on it.
func requirementBlastRadius(requirements []workflow.Requirement, rootID string) map[string]bool {
	toRemove := map[string]bool{rootID: true}

	// Iterate until no new dependents are found (handles transitive chains).
	changed := true
	for changed {
		changed = false
		for _, req := range requirements {
			if toRemove[req.ID] {
				continue
			}
			for _, dep := range req.DependsOn {
				if toRemove[dep] {
					toRemove[req.ID] = true
					changed = true
					break
				}
			}
		}
	}

	return toRemove
}

// handleDeprecateRequirement handles POST /plans/{slug}/requirements/{reqId}/deprecate.
// Cascade: deprecates transitive dependents and removes their scenarios.
func (c *Component) handleDeprecateRequirement(w http.ResponseWriter, r *http.Request, slug, requirementID string) {
	target, ok := c.requirements.get(requirementID)
	if !ok {
		http.Error(w, "Requirement not found", http.StatusNotFound)
		return
	}

	if !target.Status.CanTransitionTo(workflow.RequirementStatusDeprecated) {
		http.Error(w, "Cannot deprecate requirement in current status", http.StatusConflict)
		return
	}

	// Compute blast radius from cache.
	all := c.requirements.listByPlan(slug)
	toDeprecate := requirementBlastRadius(all, requirementID)

	// Deprecate each requirement in the blast radius.
	now := time.Now()
	for _, req := range all {
		if toDeprecate[req.ID] && req.Status != workflow.RequirementStatusDeprecated {
			updated := req
			updated.Status = workflow.RequirementStatusDeprecated
			updated.UpdatedAt = now
			if err := c.requirements.save(r.Context(), &updated); err != nil {
				c.logger.Error("Failed to deprecate requirement", "id", req.ID, "error", err)
			}
		}
	}

	// Cascade: delete scenarios for deprecated requirements.
	scenarios := c.scenarios.listByPlan(slug, c.requirements)
	for i := range scenarios {
		if toDeprecate[scenarios[i].RequirementID] {
			_ = c.scenarios.delete(r.Context(), scenarios[i].ID)
		}
	}

	c.logger.Info("Deprecated requirement with cascade",
		"slug", slug,
		"requirement_id", requirementID,
		"deprecated_count", len(toDeprecate))

	// Re-read the target to get updated status.
	if updated, ok := c.requirements.get(requirementID); ok {
		target = updated
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(target); err != nil {
		c.logger.Warn("Failed to encode response", "error", err)
	}
}
