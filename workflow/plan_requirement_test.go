package workflow

import (
	"context"
	"testing"
	"time"
)

func TestSaveLoadRequirements_RoundTrip(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	plan, err := m.CreatePlan(ctx, "test-plan", "Test Plan")
	if err != nil {
		t.Fatalf("CreatePlan() error: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	requirements := []Requirement{
		{
			ID:          "requirement.test-plan.1",
			PlanID:      plan.ID,
			Title:       "First Requirement",
			Description: "Description of first requirement",
			Status:      RequirementStatusActive,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "requirement.test-plan.2",
			PlanID:      plan.ID,
			Title:       "Second Requirement",
			Description: "Description of second requirement",
			Status:      RequirementStatusSuperseded,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	if err := m.SaveRequirements(ctx, requirements, plan.Slug); err != nil {
		t.Fatalf("SaveRequirements() error: %v", err)
	}

	got, err := m.LoadRequirements(ctx, plan.Slug)
	if err != nil {
		t.Fatalf("LoadRequirements() error: %v", err)
	}

	if len(got) != len(requirements) {
		t.Fatalf("LoadRequirements() returned %d items, want %d", len(got), len(requirements))
	}

	for i, want := range requirements {
		if got[i].ID != want.ID {
			t.Errorf("[%d] ID = %q, want %q", i, got[i].ID, want.ID)
		}
		if got[i].PlanID != want.PlanID {
			t.Errorf("[%d] PlanID = %q, want %q", i, got[i].PlanID, want.PlanID)
		}
		if got[i].Title != want.Title {
			t.Errorf("[%d] Title = %q, want %q", i, got[i].Title, want.Title)
		}
		if got[i].Status != want.Status {
			t.Errorf("[%d] Status = %q, want %q", i, got[i].Status, want.Status)
		}
		if !got[i].CreatedAt.Equal(want.CreatedAt) {
			t.Errorf("[%d] CreatedAt = %v, want %v", i, got[i].CreatedAt, want.CreatedAt)
		}
	}
}

func TestLoadRequirements_MissingFile_ReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	plan, err := m.CreatePlan(ctx, "new-plan", "New Plan")
	if err != nil {
		t.Fatalf("CreatePlan() error: %v", err)
	}

	got, err := m.LoadRequirements(ctx, plan.Slug)
	if err != nil {
		t.Fatalf("LoadRequirements() on missing file should not error, got: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("LoadRequirements() = %d items, want 0", len(got))
	}
}

func TestSaveRequirements_InvalidSlug(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	err := m.SaveRequirements(ctx, []Requirement{}, "invalid slug!")
	if err == nil {
		t.Error("SaveRequirements() with invalid slug should return error")
	}
}

func TestLoadRequirements_InvalidSlug(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	_, err := m.LoadRequirements(ctx, "invalid slug!")
	if err == nil {
		t.Error("LoadRequirements() with invalid slug should return error")
	}
}
