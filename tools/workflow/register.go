package workflow

import (
	agentictools "github.com/c360studio/semstreams/processor/agentic-tools"
)

// manifestClient is the package-level singleton for graph manifest fetching.
var manifestClient *ManifestClient

// GetManifestClient returns the package-level manifest client singleton.
// Returns nil if the graph gateway URL is not configured.
func GetManifestClient() *ManifestClient {
	return manifestClient
}

func init() {
	// Initialize manifest client for graph knowledge summaries.
	manifestClient = NewManifestClient(getGatewayURL(), nil)

	// Register graph tools only (graph_search, graph_query, graph_summary).
	// Document, constitution, and grep tools are dropped — bash handles these.
	graphExec := NewGraphExecutor()
	for _, tool := range graphExec.ListTools() {
		_ = agentictools.RegisterTool(tool.Name, graphExec)
	}
}
