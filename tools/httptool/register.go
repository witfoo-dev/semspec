package httptool

import (
	agentictools "github.com/c360studio/semstreams/processor/agentic-tools"
)

func init() {
	// Register http_request with no NATS client.
	// Graph persistence is disabled until RegisterWithNATS is called during startup.
	exec := NewExecutor(nil)
	for _, tool := range exec.ListTools() {
		_ = agentictools.RegisterTool(tool.Name, exec)
	}
}

// RegisterWithNATS re-registers the http_request tool with a live NATS client,
// enabling graph persistence. Call once during component startup after NATS is
// connected. The new executor replaces the init()-registered nil-NATS executor.
func RegisterWithNATS(nc NATSClient) {
	exec := NewExecutor(nc)
	for _, tool := range exec.ListTools() {
		_ = agentictools.RegisterTool(tool.Name, exec)
	}
}
