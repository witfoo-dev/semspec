package workflow

import (
	"context"
	"testing"
)

func TestProjectEntityID(t *testing.T) {
	tests := []struct {
		slug     string
		expected string
	}{
		{"default", ProjectEntityID("default")},
		{"my-project", ProjectEntityID("my-project")},
		{"auth-service", ProjectEntityID("auth-service")},
	}

	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			got := ProjectEntityID(tt.slug)
			if got != tt.expected {
				t.Errorf("ProjectEntityID(%q) = %q, want %q", tt.slug, got, tt.expected)
			}
		})
	}
}

func TestListProjectPlans(t *testing.T) {
	ctx := context.Background()

	// Without a KV store, ListProjectPlans returns empty results (no storage available).
	result, err := ListProjectPlans(ctx, nil, "multi-plan")
	if err != nil {
		t.Fatalf("ListProjectPlans() error = %v", err)
	}
	if len(result.Plans) != 0 {
		t.Errorf("len(Plans) = %d, want 0 (nil KV returns empty)", len(result.Plans))
	}
}
