package httptool

import (
	agentictools "github.com/c360studio/semstreams/processor/agentic-tools"
)

// Register registers the http_request tool with an optional NATS client.
// Pass nil for nc to register without graph persistence.
func Register(nc NATSClient) {
	exec := NewExecutor(nc)
	_ = agentictools.RegisterTool("http_request", exec)
}
