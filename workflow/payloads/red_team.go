package payloads

import (
	"encoding/json"
	"fmt"

	"github.com/c360studio/semstreams/message"
)

// ---------------------------------------------------------------------------
// Red-team challenge result
// ---------------------------------------------------------------------------

// RedTeamChallengeResult is the output of the red-team challenge stage.
// The red team produces a structured critique of the blue team's work and
// is encouraged (but not required) to write adversarial tests.
type RedTeamChallengeResult struct {
	// Critique fields (always required)
	Issues       []RedTeamIssue `json:"issues"`
	OverallScore int            `json:"overall_score"` // 1-5
	Summary      string         `json:"summary"`

	// Adversarial tests (optional but encouraged — boosts thoroughness score)
	TestFiles   []RedTeamTestFile `json:"test_files,omitempty"`
	TestsPassed *bool             `json:"tests_passed,omitempty"`
	TestOutput  string            `json:"test_output,omitempty"`
}

// RedTeamIssue describes a single finding from the red-team challenge.
type RedTeamIssue struct {
	Description  string `json:"description"`
	CategoryID   string `json:"category_id,omitempty"`
	Severity     string `json:"severity"` // "critical", "major", "minor", "nit"
	FilePath     string `json:"file_path,omitempty"`
	SuggestedFix string `json:"suggested_fix,omitempty"`
}

// RedTeamTestFile is an adversarial test file written by the red team.
type RedTeamTestFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Purpose string `json:"purpose"`
}

// RedTeamChallengeResultType is the message type for red-team challenge results.
var RedTeamChallengeResultType = message.Type{
	Domain:   "workflow",
	Category: "red-team-challenge-result",
	Version:  "v1",
}

// Schema implements message.Payload.
func (r *RedTeamChallengeResult) Schema() message.Type {
	return RedTeamChallengeResultType
}

// Validate implements message.Payload.
// A valid challenge must have at least one issue or one test file so the red
// team cannot submit an empty result, and the overall score must be in range.
func (r *RedTeamChallengeResult) Validate() error {
	if len(r.Issues) == 0 && len(r.TestFiles) == 0 {
		return fmt.Errorf("red-team challenge must contain at least one issue or test file")
	}
	if r.OverallScore < 1 || r.OverallScore > 5 {
		return fmt.Errorf("overall_score must be between 1 and 5, got %d", r.OverallScore)
	}
	validSeverities := map[string]bool{
		"critical": true,
		"major":    true,
		"minor":    true,
		"nit":      true,
	}
	for i, issue := range r.Issues {
		if issue.Description == "" {
			return fmt.Errorf("issues[%d]: description is required", i)
		}
		if !validSeverities[issue.Severity] {
			return fmt.Errorf("issues[%d]: severity %q is not valid; must be critical, major, minor, or nit", i, issue.Severity)
		}
	}
	return nil
}

// MarshalJSON implements json.Marshaler.
func (r *RedTeamChallengeResult) MarshalJSON() ([]byte, error) {
	type Alias RedTeamChallengeResult
	return json.Marshal((*Alias)(r))
}

// UnmarshalJSON implements json.Unmarshaler.
func (r *RedTeamChallengeResult) UnmarshalJSON(data []byte) error {
	type Alias RedTeamChallengeResult
	return json.Unmarshal(data, (*Alias)(r))
}
