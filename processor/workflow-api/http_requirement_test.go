package workflowapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/c360studio/semspec/workflow"
)

func TestExtractSlugRequirementAndAction(t *testing.T) {
	tests := []struct {
		name              string
		path              string
		wantSlug          string
		wantRequirementID string
		wantAction        string
	}{
		{
			name:              "get requirement",
			path:              "/workflow-api/plans/my-feature/requirements/requirement.my-feature.1",
			wantSlug:          "my-feature",
			wantRequirementID: "requirement.my-feature.1",
			wantAction:        "",
		},
		{
			name:              "deprecate requirement",
			path:              "/workflow-api/plans/my-feature/requirements/requirement.my-feature.1/deprecate",
			wantSlug:          "my-feature",
			wantRequirementID: "requirement.my-feature.1",
			wantAction:        "deprecate",
		},
		{
			name:              "with trailing slash",
			path:              "/workflow-api/plans/test-slug/requirements/requirement.test-slug.2/",
			wantSlug:          "test-slug",
			wantRequirementID: "requirement.test-slug.2",
			wantAction:        "",
		},
		{
			name:              "invalid - missing requirements segment",
			path:              "/workflow-api/plans/test-slug/something/requirement.test.1",
			wantSlug:          "",
			wantRequirementID: "",
			wantAction:        "",
		},
		{
			name:              "invalid - insufficient parts",
			path:              "/workflow-api/plans/test-slug/requirements",
			wantSlug:          "",
			wantRequirementID: "",
			wantAction:        "",
		},
		{
			name:              "no plans segment",
			path:              "/workflow-api/other/my-feature/requirements/r.1",
			wantSlug:          "",
			wantRequirementID: "",
			wantAction:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSlug, gotReqID, gotAction := extractSlugRequirementAndAction(tt.path)
			if gotSlug != tt.wantSlug {
				t.Errorf("slug = %q, want %q", gotSlug, tt.wantSlug)
			}
			if gotReqID != tt.wantRequirementID {
				t.Errorf("requirementID = %q, want %q", gotReqID, tt.wantRequirementID)
			}
			if gotAction != tt.wantAction {
				t.Errorf("action = %q, want %q", gotAction, tt.wantAction)
			}
		})
	}
}

func TestHandleListRequirements(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	t.Setenv("SEMSPEC_REPO_PATH", tmpDir)

	m := workflow.NewManager(tmpDir)
	slug := "test-plan"

	// Create a plan so the slug validates
	_, err := m.CreatePlan(ctx, slug, "Test Plan")
	if err != nil {
		t.Fatalf("CreatePlan() error = %v", err)
	}

	// Pre-populate requirements
	reqs := []workflow.Requirement{
		{ID: "requirement.test-plan.1", PlanID: "plan.test-plan", Title: "First requirement", Status: workflow.RequirementStatusActive},
		{ID: "requirement.test-plan.2", PlanID: "plan.test-plan", Title: "Second requirement", Status: workflow.RequirementStatusDeprecated},
	}
	if err := m.SaveRequirements(ctx, reqs, slug); err != nil {
		t.Fatalf("SaveRequirements() error = %v", err)
	}

	c := setupTestComponent(t)

	req := httptest.NewRequest(http.MethodGet, "/workflow-api/plans/"+slug+"/requirements", nil)
	w := httptest.NewRecorder()

	c.handleListRequirements(w, req, slug)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var got []workflow.Requirement
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(got) != 2 {
		t.Errorf("len(requirements) = %d, want 2", len(got))
	}
}

func TestHandleListRequirements_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("SEMSPEC_REPO_PATH", tmpDir)

	c := setupTestComponent(t)

	slug := "empty-plan"
	// Create plan directory structure by creating the plan
	m := workflow.NewManager(tmpDir)
	_, err := m.CreatePlan(context.Background(), slug, "Empty Plan")
	if err != nil {
		t.Fatalf("CreatePlan() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/workflow-api/plans/"+slug+"/requirements", nil)
	w := httptest.NewRecorder()

	c.handleListRequirements(w, req, slug)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var got []workflow.Requirement
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("len(requirements) = %d, want 0", len(got))
	}
}

func TestHandleCreateRequirement(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	t.Setenv("SEMSPEC_REPO_PATH", tmpDir)

	m := workflow.NewManager(tmpDir)
	slug := "create-req-plan"
	_, err := m.CreatePlan(ctx, slug, "Create Req Plan")
	if err != nil {
		t.Fatalf("CreatePlan() error = %v", err)
	}

	c := setupTestComponent(t)

	body, _ := json.Marshal(CreateRequirementHTTPRequest{
		Title:       "User can log in",
		Description: "The system must allow users to authenticate",
	})

	req := httptest.NewRequest(http.MethodPost, "/workflow-api/plans/"+slug+"/requirements", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c.handleCreateRequirement(w, req, slug)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var got workflow.Requirement
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.Title != "User can log in" {
		t.Errorf("title = %q, want %q", got.Title, "User can log in")
	}
	if got.Status != workflow.RequirementStatusActive {
		t.Errorf("status = %q, want %q", got.Status, workflow.RequirementStatusActive)
	}
	if got.ID == "" {
		t.Error("ID is empty")
	}
}

func TestHandleCreateRequirement_MissingTitle(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("SEMSPEC_REPO_PATH", tmpDir)

	m := workflow.NewManager(tmpDir)
	slug := "missing-title-plan"
	_, err := m.CreatePlan(context.Background(), slug, "Missing Title Plan")
	if err != nil {
		t.Fatalf("CreatePlan() error = %v", err)
	}

	c := setupTestComponent(t)

	body, _ := json.Marshal(CreateRequirementHTTPRequest{Description: "No title here"})

	req := httptest.NewRequest(http.MethodPost, "/workflow-api/plans/"+slug+"/requirements", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c.handleCreateRequirement(w, req, slug)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleGetRequirement(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	t.Setenv("SEMSPEC_REPO_PATH", tmpDir)

	m := workflow.NewManager(tmpDir)
	slug := "get-req-plan"
	_, err := m.CreatePlan(ctx, slug, "Get Req Plan")
	if err != nil {
		t.Fatalf("CreatePlan() error = %v", err)
	}

	reqID := "requirement.get-req-plan.1"
	reqs := []workflow.Requirement{
		{ID: reqID, PlanID: "plan.get-req-plan", Title: "Auth requirement", Status: workflow.RequirementStatusActive},
	}
	if err := m.SaveRequirements(ctx, reqs, slug); err != nil {
		t.Fatalf("SaveRequirements() error = %v", err)
	}

	c := setupTestComponent(t)

	req := httptest.NewRequest(http.MethodGet, "/workflow-api/plans/"+slug+"/requirements/"+reqID, nil)
	w := httptest.NewRecorder()

	c.handleGetRequirement(w, req, slug, reqID)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var got workflow.Requirement
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.ID != reqID {
		t.Errorf("ID = %q, want %q", got.ID, reqID)
	}
}

func TestHandleGetRequirement_NotFound(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	t.Setenv("SEMSPEC_REPO_PATH", tmpDir)

	m := workflow.NewManager(tmpDir)
	slug := "notfound-req-plan"
	_, err := m.CreatePlan(ctx, slug, "NotFound Req Plan")
	if err != nil {
		t.Fatalf("CreatePlan() error = %v", err)
	}

	c := setupTestComponent(t)

	req := httptest.NewRequest(http.MethodGet, "/workflow-api/plans/"+slug+"/requirements/nonexistent", nil)
	w := httptest.NewRecorder()

	c.handleGetRequirement(w, req, slug, "nonexistent")

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleUpdateRequirement(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	t.Setenv("SEMSPEC_REPO_PATH", tmpDir)

	m := workflow.NewManager(tmpDir)
	slug := "update-req-plan"
	_, err := m.CreatePlan(ctx, slug, "Update Req Plan")
	if err != nil {
		t.Fatalf("CreatePlan() error = %v", err)
	}

	reqID := "requirement.update-req-plan.1"
	reqs := []workflow.Requirement{
		{ID: reqID, PlanID: "plan.update-req-plan", Title: "Old title", Status: workflow.RequirementStatusActive},
	}
	if err := m.SaveRequirements(ctx, reqs, slug); err != nil {
		t.Fatalf("SaveRequirements() error = %v", err)
	}

	c := setupTestComponent(t)

	newTitle := "New title"
	body, _ := json.Marshal(UpdateRequirementHTTPRequest{Title: &newTitle})

	req := httptest.NewRequest(http.MethodPatch, "/workflow-api/plans/"+slug+"/requirements/"+reqID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c.handleUpdateRequirement(w, req, slug, reqID)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var got workflow.Requirement
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.Title != newTitle {
		t.Errorf("title = %q, want %q", got.Title, newTitle)
	}
}

func TestHandleDeleteRequirement(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	t.Setenv("SEMSPEC_REPO_PATH", tmpDir)

	m := workflow.NewManager(tmpDir)
	slug := "delete-req-plan"
	_, err := m.CreatePlan(ctx, slug, "Delete Req Plan")
	if err != nil {
		t.Fatalf("CreatePlan() error = %v", err)
	}

	reqID := "requirement.delete-req-plan.1"
	reqs := []workflow.Requirement{
		{ID: reqID, PlanID: "plan.delete-req-plan", Title: "To be deleted", Status: workflow.RequirementStatusActive},
	}
	if err := m.SaveRequirements(ctx, reqs, slug); err != nil {
		t.Fatalf("SaveRequirements() error = %v", err)
	}

	c := setupTestComponent(t)

	req := httptest.NewRequest(http.MethodDelete, "/workflow-api/plans/"+slug+"/requirements/"+reqID, nil)
	w := httptest.NewRecorder()

	c.handleDeleteRequirement(w, req, slug, reqID)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}

	// Verify it's gone
	remaining, err := m.LoadRequirements(ctx, slug)
	if err != nil {
		t.Fatalf("LoadRequirements() error = %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected 0 requirements after delete, got %d", len(remaining))
	}
}

func TestHandleDeprecateRequirement(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	t.Setenv("SEMSPEC_REPO_PATH", tmpDir)

	m := workflow.NewManager(tmpDir)
	slug := "deprecate-req-plan"
	_, err := m.CreatePlan(ctx, slug, "Deprecate Req Plan")
	if err != nil {
		t.Fatalf("CreatePlan() error = %v", err)
	}

	reqID := "requirement.deprecate-req-plan.1"
	reqs := []workflow.Requirement{
		{ID: reqID, PlanID: "plan.deprecate-req-plan", Title: "To be deprecated", Status: workflow.RequirementStatusActive},
	}
	if err := m.SaveRequirements(ctx, reqs, slug); err != nil {
		t.Fatalf("SaveRequirements() error = %v", err)
	}

	c := setupTestComponent(t)

	req := httptest.NewRequest(http.MethodPost, "/workflow-api/plans/"+slug+"/requirements/"+reqID+"/deprecate", nil)
	w := httptest.NewRecorder()

	c.handleDeprecateRequirement(w, req, slug, reqID)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var got workflow.Requirement
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.Status != workflow.RequirementStatusDeprecated {
		t.Errorf("status = %q, want %q", got.Status, workflow.RequirementStatusDeprecated)
	}
}

func TestHandleDeprecateRequirement_AlreadyDeprecated(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	t.Setenv("SEMSPEC_REPO_PATH", tmpDir)

	m := workflow.NewManager(tmpDir)
	slug := "already-deprecated-plan"
	_, err := m.CreatePlan(ctx, slug, "Already Deprecated Plan")
	if err != nil {
		t.Fatalf("CreatePlan() error = %v", err)
	}

	reqID := "requirement.already-deprecated-plan.1"
	reqs := []workflow.Requirement{
		{ID: reqID, PlanID: "plan.already-deprecated-plan", Title: "Already deprecated", Status: workflow.RequirementStatusDeprecated},
	}
	if err := m.SaveRequirements(ctx, reqs, slug); err != nil {
		t.Fatalf("SaveRequirements() error = %v", err)
	}

	c := setupTestComponent(t)

	req := httptest.NewRequest(http.MethodPost, "/workflow-api/plans/"+slug+"/requirements/"+reqID+"/deprecate", nil)
	w := httptest.NewRecorder()

	c.handleDeprecateRequirement(w, req, slug, reqID)

	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", w.Code, http.StatusConflict)
	}
}
