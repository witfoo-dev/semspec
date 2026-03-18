package workflow

import (
	"sort"
	"time"
)

// TeamStatus represents the lifecycle state of a team.
type TeamStatus string

const (
	// TeamActive indicates the team is ready to accept work.
	TeamActive TeamStatus = "active"

	// TeamBenched indicates the team has been excluded from selection due to
	// accumulated errors. Benched teams are not dispatched.
	TeamBenched TeamStatus = "benched"

	// TeamRetired indicates the team has been permanently removed from the roster.
	TeamRetired TeamStatus = "retired"
)

// Team is a persistent team identity with shared accumulated knowledge.
// Teams contain agents that can fill any pipeline role (tester, builder, reviewer).
// Teams are stored as graph entities and accumulate collective review scores,
// error counts, and shared knowledge (insights) across task executions.
type Team struct {
	// ID is a UUID that serves as the entity instance identifier in the graph.
	ID string

	// Name is the human-readable team name, e.g. "alpha", "bravo".
	Name string

	// Status is the current lifecycle state of the team.
	Status TeamStatus

	// MemberIDs are the agent entity IDs belonging to this team.
	// Agents are NOT duplicated — they keep their own entities.
	MemberIDs []string

	// SharedKnowledge accumulates lessons from red-team critiques and
	// reviewer feedback. Filtered by skill and error category before
	// injection into agent prompts.
	SharedKnowledge []TeamInsight

	// TeamStats are collective running averages from reviews of team output.
	TeamStats ReviewStats

	// RedTeamStats are running averages from reviewer assessments of this
	// team's red-team critique quality (accuracy, thoroughness, fairness).
	RedTeamStats ReviewStats

	// ErrorCounts tracks accumulated errors attributed to the team as a whole.
	// The map is created on first use via IncrementErrorCount.
	ErrorCounts map[ErrorCategory]int

	// CreatedAt is when the team entity was first created.
	CreatedAt time.Time

	// UpdatedAt is when the team entity was last modified.
	UpdatedAt time.Time
}

// TeamInsight is a single piece of shared knowledge extracted from feedback.
// Classified by error categories and skill for filtered prompt injection.
type TeamInsight struct {
	// ID is a UUID for this insight.
	ID string

	// Source identifies where this insight came from.
	// Values: "reviewer-feedback", "red-team-critique-feedback"
	Source string

	// ScenarioID is which scenario produced this insight.
	ScenarioID string

	// Summary is a 1-2 sentence actionable lesson.
	Summary string

	// CategoryIDs are linked error categories (e.g. "missing_tests", "sop_violation").
	// Used for filtering: only insights matching the current error patterns are injected.
	CategoryIDs []string

	// Skill is the role/stage this insight applies to: "tester", "builder", "reviewer", "red-team".
	// Used for filtering: only insights matching the current dispatch stage are injected.
	Skill string

	// CreatedAt is when this insight was recorded.
	CreatedAt time.Time
}

// IncrementErrorCount increments the accumulated count for the given error category.
// The ErrorCounts map is initialised lazily on first call.
func (t *Team) IncrementErrorCount(category ErrorCategory) {
	if t.ErrorCounts == nil {
		t.ErrorCounts = make(map[ErrorCategory]int)
	}
	t.ErrorCounts[category]++
}

// TotalErrorCount returns the sum of all error category counts.
func (t *Team) TotalErrorCount() int {
	total := 0
	for _, count := range t.ErrorCounts {
		total += count
	}
	return total
}

// IsBenched returns true if the team is in the benched state.
func (t *Team) IsBenched() bool {
	return t.Status == TeamBenched
}

// ShouldBench returns true if any error category count has reached or exceeded
// the given threshold, indicating the team should be benched.
func (t *Team) ShouldBench(threshold int) bool {
	for _, count := range t.ErrorCounts {
		if count >= threshold {
			return true
		}
	}
	return false
}

// FilterInsights returns insights matching the given skill OR any of the given
// categories, up to limit entries. Insights with an empty Skill and empty
// CategoryIDs are universal and always included. Results are ordered most
// recent first. If limit is zero or negative, all matching insights are returned.
func (t *Team) FilterInsights(skill string, categories []string, limit int) []TeamInsight {
	categorySet := make(map[string]bool, len(categories))
	for _, c := range categories {
		categorySet[c] = true
	}

	var matched []TeamInsight
	for _, insight := range t.SharedKnowledge {
		if matchesInsight(insight, skill, categorySet) {
			matched = append(matched, insight)
		}
	}

	// Most recent first.
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})

	if limit > 0 && len(matched) > limit {
		return matched[:limit]
	}
	return matched
}

// matchesInsight returns true if the insight should be included for the given
// skill and category set. Universal insights (no skill and no categories) always
// match. Otherwise the insight matches if its Skill equals the requested skill
// OR if any of its CategoryIDs appear in the category set.
func matchesInsight(insight TeamInsight, skill string, categorySet map[string]bool) bool {
	isUniversal := insight.Skill == "" && len(insight.CategoryIDs) == 0
	if isUniversal {
		return true
	}

	if skill != "" && insight.Skill == skill {
		return true
	}

	for _, id := range insight.CategoryIDs {
		if categorySet[id] {
			return true
		}
	}

	return false
}
