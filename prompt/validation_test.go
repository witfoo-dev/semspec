package prompt

import "testing"

func TestLooksLikeQuestion(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{
			name: "empty string",
			text: "",
			want: false,
		},
		{
			name: "code with code fences is real work",
			text: "Here's my implementation:\n```go\nfunc main() {}\n```\nDoes this look right?",
			want: false,
		},
		{
			name: "short text with question phrase",
			text: "I need more information about the authentication flow before I can implement this.",
			want: true,
		},
		{
			name: "short text asking for clarification",
			text: "Could you clarify what the expected output format should be?",
			want: true,
		},
		{
			name: "short text with before i can proceed",
			text: "Before I can proceed, I need to understand the database schema.",
			want: true,
		},
		{
			name: "real work output without code fences",
			text: "I implemented the authentication middleware. It validates JWT tokens and extracts the user ID from the claims. The middleware is applied to all /api routes.",
			want: false,
		},
		{
			name: "majority question marks",
			text: "What database should I use?\nWhich ORM is preferred?\nShould I add migrations?",
			want: true,
		},
		{
			name: "minority question marks",
			text: "I created the handler.\nIt validates input.\nIt returns JSON.\nShould I add tests?",
			want: false,
		},
		{
			name: "please clarify phrase",
			text: "Please clarify the expected behavior when the user is not authenticated.",
			want: true,
		},
		{
			name: "do you want me to phrase",
			text: "Do you want me to also update the tests for this change?",
			want: true,
		},
		{
			name: "long text with question phrase is not flagged",
			text: string(make([]byte, 2001)) + "could you provide more details",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LooksLikeQuestion(tt.text)
			if got != tt.want {
				t.Errorf("LooksLikeQuestion() = %v, want %v", got, tt.want)
			}
		})
	}
}
