// Package agentgraph provides graph entity helpers for agent hierarchy tracking.
// It maps agent loops, tasks, and DAG executions to SemStreams graph entities
// with relationship triples, enabling tree queries via the existing graph
// query infrastructure.
//
// Entity IDs follow the 6-part format: org.platform.domain.system.type.instance
// Example loop:  semspec.local.agent.loop.loop.<loopID>
// Example task:  semspec.local.agent.loop.task.<taskID>
// Example DAG:   semspec.local.agent.loop.dag.<dagID>
package agentgraph
