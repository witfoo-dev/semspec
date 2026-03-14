package prompt

import "strings"

// questionPhrases are phrases that indicate a submission is a question, not work.
var questionPhrases = []string{
	"could you provide",
	"could you clarify",
	"can you explain",
	"i need more information",
	"i need clarification",
	"before i can proceed",
	"before i proceed",
	"i'm not sure how to",
	"i'm unsure how to",
	"what should i",
	"how should i",
	"where should i",
	"which approach",
	"is it correct to",
	"should i use",
	"do you want me to",
	"would you like me to",
	"please clarify",
	"please provide",
	"i have a question",
}

// LooksLikeQuestion returns true if the text appears to be a question or request
// for information rather than completed work. This catches agents that misuse
// work submission to ask questions instead of using a clarification tool.
func LooksLikeQuestion(text string) bool {
	if text == "" {
		return false
	}

	lower := strings.ToLower(text)

	// If it contains code fences, assume it's real work even if it has questions.
	if strings.Contains(lower, "```") {
		return false
	}

	// Short submissions with question phrases are likely questions.
	if len(text) < 2000 {
		for _, phrase := range questionPhrases {
			if strings.Contains(lower, phrase) {
				return true
			}
		}
	}

	// Fallback: if >50% of non-empty lines end with "?", it's a question.
	lines := strings.Split(text, "\n")
	var nonEmpty, questions int
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		nonEmpty++
		if strings.HasSuffix(trimmed, "?") {
			questions++
		}
	}

	if nonEmpty == 0 {
		return false
	}
	return float64(questions)/float64(nonEmpty) > 0.5
}
