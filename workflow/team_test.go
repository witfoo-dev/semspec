package workflow

import (
	"testing"
	"time"
)

// insightAt is a test helper that builds a TeamInsight with the given skill,
// categories, and a CreatedAt offset (in seconds from a fixed base time).
func insightAt(id, skill string, categoryIDs []string, secondsOffset int) TeamInsight {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return TeamInsight{
		ID:          id,
		Skill:       skill,
		CategoryIDs: categoryIDs,
		Summary:     "insight " + id,
		CreatedAt:   base.Add(time.Duration(secondsOffset) * time.Second),
	}
}

func TestTeam_FilterInsights_BySkillOnly(t *testing.T) {
	t.Parallel()

	team := &Team{
		SharedKnowledge: []TeamInsight{
			insightAt("a", "builder", nil, 1),
			insightAt("b", "tester", nil, 2),
			insightAt("c", "reviewer", nil, 3),
			insightAt("d", "builder", nil, 4),
		},
	}

	got := team.FilterInsights("builder", nil, 0)

	if len(got) != 2 {
		t.Fatalf("FilterInsights(builder) len = %d, want 2", len(got))
	}
	// Most recent first: "d" (offset 4) before "a" (offset 1).
	if got[0].ID != "d" {
		t.Errorf("got[0].ID = %q, want %q", got[0].ID, "d")
	}
	if got[1].ID != "a" {
		t.Errorf("got[1].ID = %q, want %q", got[1].ID, "a")
	}
}

func TestTeam_FilterInsights_ByCategoryOnly(t *testing.T) {
	t.Parallel()

	team := &Team{
		SharedKnowledge: []TeamInsight{
			insightAt("a", "builder", []string{"missing_tests"}, 1),
			insightAt("b", "tester", []string{"sop_violation"}, 2),
			insightAt("c", "reviewer", []string{"wrong_pattern"}, 3),
		},
	}

	// Query with no skill but matching category "sop_violation".
	got := team.FilterInsights("", []string{"sop_violation"}, 0)

	if len(got) != 1 {
		t.Fatalf("FilterInsights(sop_violation) len = %d, want 1", len(got))
	}
	if got[0].ID != "b" {
		t.Errorf("got[0].ID = %q, want %q", got[0].ID, "b")
	}
}

func TestTeam_FilterInsights_BySkillAndCategory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		insights   []TeamInsight
		skill      string
		categories []string
		wantIDs    []string
	}{
		{
			name: "skill match only",
			insights: []TeamInsight{
				insightAt("a", "builder", []string{"wrong_pattern"}, 1),
				insightAt("b", "tester", []string{"sop_violation"}, 2),
			},
			skill:      "builder",
			categories: []string{"missing_tests"},
			wantIDs:    []string{"a"},
		},
		{
			name: "category match only",
			insights: []TeamInsight{
				insightAt("a", "builder", []string{"wrong_pattern"}, 1),
				insightAt("b", "tester", []string{"sop_violation"}, 2),
			},
			skill:      "reviewer",
			categories: []string{"sop_violation"},
			wantIDs:    []string{"b"},
		},
		{
			name: "both skill and category match distinct insights",
			insights: []TeamInsight{
				insightAt("a", "builder", nil, 1),
				insightAt("b", "tester", []string{"sop_violation"}, 2),
				insightAt("c", "reviewer", []string{"wrong_pattern"}, 3),
			},
			skill:      "builder",
			categories: []string{"sop_violation"},
			wantIDs:    []string{"b", "a"}, // most recent first
		},
		{
			name: "same insight matches both skill and category — not duplicated",
			insights: []TeamInsight{
				insightAt("a", "builder", []string{"sop_violation"}, 1),
			},
			skill:      "builder",
			categories: []string{"sop_violation"},
			wantIDs:    []string{"a"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			team := &Team{SharedKnowledge: tc.insights}
			got := team.FilterInsights(tc.skill, tc.categories, 0)

			if len(got) != len(tc.wantIDs) {
				t.Fatalf("FilterInsights() len = %d, want %d", len(got), len(tc.wantIDs))
			}
			for i, id := range tc.wantIDs {
				if got[i].ID != id {
					t.Errorf("got[%d].ID = %q, want %q", i, got[i].ID, id)
				}
			}
		})
	}
}

func TestTeam_FilterInsights_UniversalAlwaysIncluded(t *testing.T) {
	t.Parallel()

	team := &Team{
		SharedKnowledge: []TeamInsight{
			insightAt("universal", "", nil, 10), // no skill, no categories
			insightAt("builder-only", "builder", nil, 5),
			insightAt("cat-only", "", []string{"missing_tests"}, 1),
		},
	}

	tests := []struct {
		name       string
		skill      string
		categories []string
		wantIDs    []string
	}{
		{
			name:       "universal included with unrelated skill",
			skill:      "tester",
			categories: nil,
			wantIDs:    []string{"universal"},
		},
		{
			name:       "universal included with unrelated category",
			skill:      "",
			categories: []string{"sop_violation"},
			wantIDs:    []string{"universal"},
		},
		{
			name:       "universal included alongside skill match",
			skill:      "builder",
			categories: nil,
			wantIDs:    []string{"universal", "builder-only"}, // most recent first
		},
		{
			name:       "universal included alongside category match",
			skill:      "",
			categories: []string{"missing_tests"},
			wantIDs:    []string{"universal", "cat-only"}, // most recent first
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := team.FilterInsights(tc.skill, tc.categories, 0)

			if len(got) != len(tc.wantIDs) {
				t.Fatalf("FilterInsights() len = %d, want %d (ids: %v)", len(got), len(tc.wantIDs), ids(got))
			}
			for i, id := range tc.wantIDs {
				if got[i].ID != id {
					t.Errorf("got[%d].ID = %q, want %q", i, got[i].ID, id)
				}
			}
		})
	}
}

func TestTeam_FilterInsights_LimitCapping(t *testing.T) {
	t.Parallel()

	team := &Team{
		SharedKnowledge: []TeamInsight{
			insightAt("a", "builder", nil, 1),
			insightAt("b", "builder", nil, 2),
			insightAt("c", "builder", nil, 3),
			insightAt("d", "builder", nil, 4),
			insightAt("e", "builder", nil, 5),
		},
	}

	tests := []struct {
		name    string
		limit   int
		wantLen int
		wantIDs []string
	}{
		{
			name:    "limit 0 returns all",
			limit:   0,
			wantLen: 5,
			wantIDs: []string{"e", "d", "c", "b", "a"},
		},
		{
			name:    "negative limit returns all",
			limit:   -1,
			wantLen: 5,
			wantIDs: []string{"e", "d", "c", "b", "a"},
		},
		{
			name:    "limit 3 returns top 3 most recent",
			limit:   3,
			wantLen: 3,
			wantIDs: []string{"e", "d", "c"},
		},
		{
			name:    "limit 1 returns only most recent",
			limit:   1,
			wantLen: 1,
			wantIDs: []string{"e"},
		},
		{
			name:    "limit larger than result set returns all",
			limit:   100,
			wantLen: 5,
			wantIDs: []string{"e", "d", "c", "b", "a"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := team.FilterInsights("builder", nil, tc.limit)

			if len(got) != tc.wantLen {
				t.Fatalf("FilterInsights(limit=%d) len = %d, want %d", tc.limit, len(got), tc.wantLen)
			}
			for i, id := range tc.wantIDs {
				if got[i].ID != id {
					t.Errorf("got[%d].ID = %q, want %q", i, got[i].ID, id)
				}
			}
		})
	}
}

func TestTeam_FilterInsights_EmptyKnowledge(t *testing.T) {
	t.Parallel()

	team := &Team{}
	got := team.FilterInsights("builder", []string{"missing_tests"}, 10)

	if len(got) != 0 {
		t.Errorf("FilterInsights on empty knowledge = %d entries, want 0", len(got))
	}
}

func TestTeam_IncrementErrorCount(t *testing.T) {
	t.Run("initialises map on first call", func(t *testing.T) {
		team := &Team{}

		if team.ErrorCounts != nil {
			t.Fatal("ErrorCounts should be nil before first increment")
		}

		team.IncrementErrorCount("missing_tests")

		if team.ErrorCounts == nil {
			t.Fatal("ErrorCounts should be non-nil after first increment")
		}
		if team.ErrorCounts["missing_tests"] != 1 {
			t.Errorf("ErrorCounts[missing_tests] = %d, want 1", team.ErrorCounts["missing_tests"])
		}
	})

	t.Run("increments existing category", func(t *testing.T) {
		team := &Team{}

		team.IncrementErrorCount("sop_violation")
		team.IncrementErrorCount("sop_violation")
		team.IncrementErrorCount("sop_violation")

		if team.ErrorCounts["sop_violation"] != 3 {
			t.Errorf("ErrorCounts[sop_violation] = %d, want 3", team.ErrorCounts["sop_violation"])
		}
	})

	t.Run("tracks multiple categories independently", func(t *testing.T) {
		team := &Team{}

		team.IncrementErrorCount("missing_tests")
		team.IncrementErrorCount("wrong_pattern")
		team.IncrementErrorCount("wrong_pattern")
		team.IncrementErrorCount("scope_violation")

		if team.ErrorCounts["missing_tests"] != 1 {
			t.Errorf("missing_tests = %d, want 1", team.ErrorCounts["missing_tests"])
		}
		if team.ErrorCounts["wrong_pattern"] != 2 {
			t.Errorf("wrong_pattern = %d, want 2", team.ErrorCounts["wrong_pattern"])
		}
		if team.ErrorCounts["scope_violation"] != 1 {
			t.Errorf("scope_violation = %d, want 1", team.ErrorCounts["scope_violation"])
		}
		// Unset category should be zero value.
		if team.ErrorCounts["api_contract_mismatch"] != 0 {
			t.Errorf("api_contract_mismatch = %d, want 0", team.ErrorCounts["api_contract_mismatch"])
		}
	})

	t.Run("uses existing map when already initialised", func(t *testing.T) {
		team := &Team{
			ErrorCounts: map[ErrorCategory]int{"edge_case_missed": 5},
		}

		team.IncrementErrorCount("edge_case_missed")

		if team.ErrorCounts["edge_case_missed"] != 6 {
			t.Errorf("edge_case_missed = %d, want 6", team.ErrorCounts["edge_case_missed"])
		}
	})
}

func TestTeam_TotalErrorCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		errorCounts map[ErrorCategory]int
		want        int
	}{
		{
			name:        "nil map",
			errorCounts: nil,
			want:        0,
		},
		{
			name:        "empty map",
			errorCounts: map[ErrorCategory]int{},
			want:        0,
		},
		{
			name: "single category",
			errorCounts: map[ErrorCategory]int{
				"missing_tests": 4,
			},
			want: 4,
		},
		{
			name: "multiple categories summed",
			errorCounts: map[ErrorCategory]int{
				"missing_tests": 2,
				"wrong_pattern": 3,
				"sop_violation": 1,
			},
			want: 6,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			team := &Team{ErrorCounts: tc.errorCounts}
			if got := team.TotalErrorCount(); got != tc.want {
				t.Errorf("TotalErrorCount() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestTeam_ShouldBench(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		errorCounts map[ErrorCategory]int
		threshold   int
		want        bool
	}{
		{
			name:        "nil map",
			errorCounts: nil,
			threshold:   DefaultBenchingThreshold,
			want:        false,
		},
		{
			name:        "empty map",
			errorCounts: map[ErrorCategory]int{},
			threshold:   DefaultBenchingThreshold,
			want:        false,
		},
		{
			name: "all counts below threshold",
			errorCounts: map[ErrorCategory]int{
				"missing_tests": 1,
				"wrong_pattern": 2,
			},
			threshold: DefaultBenchingThreshold,
			want:      false,
		},
		{
			name: "one count at threshold",
			errorCounts: map[ErrorCategory]int{
				"missing_tests": DefaultBenchingThreshold,
			},
			threshold: DefaultBenchingThreshold,
			want:      true,
		},
		{
			name: "one count above threshold",
			errorCounts: map[ErrorCategory]int{
				"sop_violation": DefaultBenchingThreshold + 1,
			},
			threshold: DefaultBenchingThreshold,
			want:      true,
		},
		{
			name: "multiple categories, one at threshold",
			errorCounts: map[ErrorCategory]int{
				"missing_tests":  1,
				"wrong_pattern":  DefaultBenchingThreshold,
				"scope_violation": 2,
			},
			threshold: DefaultBenchingThreshold,
			want:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			team := &Team{ErrorCounts: tc.errorCounts}
			if got := team.ShouldBench(tc.threshold); got != tc.want {
				t.Errorf("ShouldBench(%d) = %v, want %v", tc.threshold, got, tc.want)
			}
		})
	}
}

func TestTeam_IsBenched(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status TeamStatus
		want   bool
	}{
		{"active", TeamActive, false},
		{"benched", TeamBenched, true},
		{"retired", TeamRetired, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			team := &Team{Status: tc.status}
			if got := team.IsBenched(); got != tc.want {
				t.Errorf("IsBenched() = %v, want %v (status=%q)", got, tc.want, tc.status)
			}
		})
	}
}

// ids extracts TeamInsight IDs for use in test failure messages.
func ids(insights []TeamInsight) []string {
	out := make([]string, len(insights))
	for i, ins := range insights {
		out[i] = ins.ID
	}
	return out
}
