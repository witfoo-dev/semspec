// Package graphutil provides shared graph write helpers used across orchestrator
// components. Centralising writeTriple and portSubject here removes the
// verbatim copy that previously existed in review-orchestrator,
// execution-orchestrator, scenario-executor, plan-coordinator, and
// change-proposal-handler.
package graphutil

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/c360studio/semstreams/component"
	"github.com/c360studio/semstreams/graph"
	"github.com/c360studio/semstreams/message"
	"github.com/c360studio/semstreams/natsclient"
)

// TripleWriter provides graph triple write capabilities via NATS request/reply.
// It wraps a NATS client and logger, eliminating per-component boilerplate for
// the writeTriple pattern.
//
// Usage:
//
//	tw := &graphutil.TripleWriter{
//	    NATSClient:    deps.NATSClient,
//	    Logger:        logger,
//	    ComponentName: "my-component",
//	}
//	if err := tw.WriteTriple(ctx, entityID, wf.Phase, "generating"); err != nil {
//	    // handle error
//	}
type TripleWriter struct {
	NATSClient    *natsclient.Client
	Logger        *slog.Logger
	ComponentName string
}

// WriteTriple sends an AddTripleRequest to graph-ingest via NATS request/reply.
// graph-ingest handles CAS writes to ENTITY_STATES KV and returns a KVRevision.
//
// Pass numeric values (int, int64, float64) directly — do not format them as
// strings. The graph store accepts any JSON-serialisable object value.
//
// Returns an error on failure; callers should error-check critical triples
// (e.g., workflow.phase) and can safely ignore non-critical ones with _.
func (tw *TripleWriter) WriteTriple(ctx context.Context, entityID, predicate string, object any) error {
	req := graph.AddTripleRequest{
		Triple: message.Triple{
			Subject:    entityID,
			Predicate:  predicate,
			Object:     object,
			Source:     tw.ComponentName,
			Timestamp:  time.Now(),
			Confidence: 1.0,
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		tw.Logger.Warn("Failed to marshal triple request", "predicate", predicate, "error", err)
		return fmt.Errorf("marshal triple request: %w", err)
	}

	if tw.NATSClient == nil {
		return nil
	}

	respData, err := tw.NATSClient.Request(ctx, "graph.mutation.triple.add", data, 5*time.Second)
	if err != nil {
		tw.Logger.Warn("Triple write request failed",
			"predicate", predicate, "entity_id", entityID, "error", err)
		return fmt.Errorf("triple write request: %w", err)
	}

	var resp graph.AddTripleResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		tw.Logger.Warn("Failed to unmarshal triple response", "predicate", predicate, "error", err)
		return fmt.Errorf("unmarshal triple response: %w", err)
	}

	if !resp.Success {
		tw.Logger.Warn("Triple write rejected by graph-ingest",
			"predicate", predicate, "entity_id", entityID, "error", resp.Error)
		return fmt.Errorf("triple write rejected: %s", resp.Error)
	}

	return nil
}

// PortSubject extracts the subject string from a port's config.
// Works with both NATSPort and JetStreamPort configurations.
// Returns an empty string if the port has no config or no subjects.
func PortSubject(port component.Port) string {
	if port.Config == nil {
		return ""
	}
	switch cfg := port.Config.(type) {
	case component.NATSPort:
		return cfg.Subject
	case component.JetStreamPort:
		if len(cfg.Subjects) > 0 {
			return cfg.Subjects[0]
		}
	}
	return ""
}
