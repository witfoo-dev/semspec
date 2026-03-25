package websearch

import (
	"os"

	agentictools "github.com/c360studio/semstreams/processor/agentic-tools"
)

// Register registers the web_search tool if BRAVE_SEARCH_API_KEY is set.
func Register() {
	apiKey := os.Getenv("BRAVE_SEARCH_API_KEY")
	if apiKey == "" {
		return
	}
	provider := NewBraveProvider(apiKey)
	exec := NewExecutor(provider)
	_ = agentictools.RegisterTool("web_search", exec)
}
