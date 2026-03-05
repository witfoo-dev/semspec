package workflowapi

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
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// UpdateRequirementHTTPRequest is the HTTP request body for PATCH /plans/{slug}/requirements/{reqId}.
type UpdateRequirementHTTPRequest struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
}

// extractSlugRequirementAndAction extracts slug, requirementID, and action from paths like:
// /workflow-api/plans/{slug}/requirements/{reqId}
// /workflow-api/plans/{slug}/requirements/{reqId}/deprecate
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
func (c *Component) handleListRequirements(w http.ResponseWriter, r *http.Request, slug string) {
	manager := c.getManager(w)
	if manager == nil {
		return
	}

	requirements, err := manager.LoadRequirements(r.Context(), slug)
	if err != nil {
		c.logger.Error("Failed to load requirements", "slug", slug, "error", err)
		http.Error(w, "Failed to load requirements", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(requirements); err != nil {
		c.logger.Warn("Failed to encode response", "error", err)
	}
}

// handleGetRequirement handles GET /plans/{slug}/requirements/{reqId}.
func (c *Component) handleGetRequirement(w http.ResponseWriter, r *http.Request, slug, requirementID string) {
	manager := c.getManager(w)
	if manager == nil {
		return
	}

	requirements, err := manager.LoadRequirements(r.Context(), slug)
	if err != nil {
		c.logger.Error("Failed to load requirements", "slug", slug, "error", err)
		http.Error(w, "Failed to load requirements", http.StatusInternalServerError)
		return
	}

	for _, req := range requirements {
		if req.ID == requirementID {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(req); err != nil {
				c.logger.Warn("Failed to encode response", "error", err)
			}
			return
		}
	}

	http.Error(w, "Requirement not found", http.StatusNotFound)
}

// handleCreateRequirement handles POST /plans/{slug}/requirements.
func (c *Component) handleCreateRequirement(w http.ResponseWriter, r *http.Request, slug string) {
	manager := c.getManager(w)
	if manager == nil {
		return
	}

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

	requirements, err := manager.LoadRequirements(r.Context(), slug)
	if err != nil {
		c.logger.Error("Failed to load requirements", "slug", slug, "error", err)
		http.Error(w, "Failed to load requirements", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	id := fmt.Sprintf("requirement.%s.%d", slug, len(requirements)+1)

	newReq := workflow.Requirement{
		ID:          id,
		PlanID:      workflow.PlanEntityID(slug),
		Title:       req.Title,
		Description: req.Description,
		Status:      workflow.RequirementStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	requirements = append(requirements, newReq)

	if err := manager.SaveRequirements(r.Context(), requirements, slug); err != nil {
		c.logger.Error("Failed to save requirements", "slug", slug, "error", err)
		http.Error(w, "Failed to save requirement", http.StatusInternalServerError)
		return
	}

	c.logger.Info("Requirement created via REST API", "slug", slug, "requirement_id", newReq.ID)

	// Publish to graph (best-effort)
	if err := c.publishRequirementEntity(r.Context(), slug, &newReq); err != nil {
		c.logger.Warn("Failed to publish requirement entity to graph", "requirement_id", newReq.ID, "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(newReq); err != nil {
		c.logger.Warn("Failed to encode response", "error", err)
	}
}

// handleUpdateRequirement handles PATCH /plans/{slug}/requirements/{reqId}.
func (c *Component) handleUpdateRequirement(w http.ResponseWriter, r *http.Request, slug, requirementID string) {
	manager := c.getManager(w)
	if manager == nil {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodySize)

	var req UpdateRequirementHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	requirements, err := manager.LoadRequirements(r.Context(), slug)
	if err != nil {
		c.logger.Error("Failed to load requirements", "slug", slug, "error", err)
		http.Error(w, "Failed to load requirements", http.StatusInternalServerError)
		return
	}

	idx := -1
	for i, existing := range requirements {
		if existing.ID == requirementID {
			idx = i
			break
		}
	}
	if idx == -1 {
		http.Error(w, "Requirement not found", http.StatusNotFound)
		return
	}

	if req.Title != nil {
		requirements[idx].Title = *req.Title
	}
	if req.Description != nil {
		requirements[idx].Description = *req.Description
	}
	requirements[idx].UpdatedAt = time.Now()

	if err := manager.SaveRequirements(r.Context(), requirements, slug); err != nil {
		c.logger.Error("Failed to save requirements", "slug", slug, "error", err)
		http.Error(w, "Failed to save requirement", http.StatusInternalServerError)
		return
	}

	// Publish to graph (best-effort)
	if err := c.publishRequirementEntity(r.Context(), slug, &requirements[idx]); err != nil {
		c.logger.Warn("Failed to publish requirement entity to graph", "requirement_id", requirementID, "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(requirements[idx]); err != nil {
		c.logger.Warn("Failed to encode response", "error", err)
	}
}

// handleDeleteRequirement handles DELETE /plans/{slug}/requirements/{reqId}.
func (c *Component) handleDeleteRequirement(w http.ResponseWriter, r *http.Request, slug, requirementID string) {
	manager := c.getManager(w)
	if manager == nil {
		return
	}

	requirements, err := manager.LoadRequirements(r.Context(), slug)
	if err != nil {
		c.logger.Error("Failed to load requirements", "slug", slug, "error", err)
		http.Error(w, "Failed to load requirements", http.StatusInternalServerError)
		return
	}

	idx := -1
	for i, existing := range requirements {
		if existing.ID == requirementID {
			idx = i
			break
		}
	}
	if idx == -1 {
		http.Error(w, "Requirement not found", http.StatusNotFound)
		return
	}

	requirements = append(requirements[:idx], requirements[idx+1:]...)

	if err := manager.SaveRequirements(r.Context(), requirements, slug); err != nil {
		c.logger.Error("Failed to save requirements", "slug", slug, "error", err)
		http.Error(w, "Failed to delete requirement", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleDeprecateRequirement handles POST /plans/{slug}/requirements/{reqId}/deprecate.
func (c *Component) handleDeprecateRequirement(w http.ResponseWriter, r *http.Request, slug, requirementID string) {
	manager := c.getManager(w)
	if manager == nil {
		return
	}

	requirements, err := manager.LoadRequirements(r.Context(), slug)
	if err != nil {
		c.logger.Error("Failed to load requirements", "slug", slug, "error", err)
		http.Error(w, "Failed to load requirements", http.StatusInternalServerError)
		return
	}

	idx := -1
	for i, existing := range requirements {
		if existing.ID == requirementID {
			idx = i
			break
		}
	}
	if idx == -1 {
		http.Error(w, "Requirement not found", http.StatusNotFound)
		return
	}

	if !requirements[idx].Status.CanTransitionTo(workflow.RequirementStatusDeprecated) {
		http.Error(w, "Cannot deprecate requirement in current status", http.StatusConflict)
		return
	}

	requirements[idx].Status = workflow.RequirementStatusDeprecated
	requirements[idx].UpdatedAt = time.Now()

	if err := manager.SaveRequirements(r.Context(), requirements, slug); err != nil {
		c.logger.Error("Failed to save requirements", "slug", slug, "error", err)
		http.Error(w, "Failed to deprecate requirement", http.StatusInternalServerError)
		return
	}

	// Publish to graph (best-effort)
	if err := c.publishRequirementEntity(r.Context(), slug, &requirements[idx]); err != nil {
		c.logger.Warn("Failed to publish requirement entity to graph", "requirement_id", requirementID, "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(requirements[idx]); err != nil {
		c.logger.Warn("Failed to encode response", "error", err)
	}
}
