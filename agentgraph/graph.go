package agentgraph

import (
	"context"
	"fmt"
	"time"

	gtypes "github.com/c360studio/semstreams/graph"
	"github.com/c360studio/semstreams/graph/datamanager"
	"github.com/c360studio/semstreams/graph/query"
	"github.com/c360studio/semstreams/message"
	"github.com/c360studio/semstreams/pkg/types"
)

// Agentic relationship and property predicates for graph triples.
//
// These predicate strings must match the constants in vocabulary/semspec/predicates.go,
// which registers them with the vocabulary system via init().
const (
	// PredicateSpawned records a parent loop spawning a child loop.
	// Direction: parent loop entity -> child loop entity.
	PredicateSpawned = "agentic.loop.spawned"

	// PredicateLoopTask records the association between a loop and a task it owns.
	// Direction: loop entity -> task entity.
	PredicateLoopTask = "agentic.loop.task"

	// PredicateDependsOn records a task-to-task dependency (DAG edge).
	// Direction: dependent task entity -> prerequisite task entity.
	PredicateDependsOn = "agentic.task.depends_on"

	// PredicateRole records the functional role of a loop (e.g., "planner", "executor").
	PredicateRole = "agentic.loop.role"

	// PredicateModel records the LLM model identifier used by a loop.
	PredicateModel = "agentic.loop.model"

	// PredicateStatus records the current lifecycle status of a loop.
	PredicateStatus = "agentic.loop.status"
)

// EntityStore combines EntityManager with TripleManager.
// The semstreams datamanager.Manager concrete type satisfies both; this local
// interface lets Helper depend only on the behaviour it needs rather than on
// the concrete type.
type EntityStore interface {
	datamanager.EntityManager
	datamanager.TripleManager
}

// Helper provides graph operations for agent hierarchy tracking.
// It is a thin façade over EntityStore and query.Client that speaks
// in agent-domain terms (loop IDs, task IDs) rather than raw entity IDs.
//
// All methods are safe for concurrent use — they delegate directly to the
// underlying interfaces without holding additional state.
type Helper struct {
	entities EntityStore
	queries  query.Client
}

// NewHelper constructs a Helper.
// Both arguments are required; passing nil will cause panics at call time.
func NewHelper(entities EntityStore, queries query.Client) *Helper {
	return &Helper{
		entities: entities,
		queries:  queries,
	}
}

// RecordLoopCreated creates a graph entity for a newly-started loop and attaches
// property triples for role, model, and initial status.
// It is idempotent: if the entity already exists it will be updated via UpsertEntity.
func (h *Helper) RecordLoopCreated(ctx context.Context, loopID, role, model string) error {
	entityID := LoopEntityID(loopID)
	now := time.Now()

	triples := []message.Triple{
		propertyTriple(entityID, PredicateRole, role, now),
		propertyTriple(entityID, PredicateModel, model, now),
		propertyTriple(entityID, PredicateStatus, "created", now),
	}

	entity := &gtypes.EntityState{
		ID:          entityID,
		Triples:     triples,
		MessageType: message.Type{Domain: DomainAgentic, Category: TypeLoop, Version: "v1"},
		UpdatedAt:   now,
	}

	if _, err := h.entities.UpsertEntity(ctx, entity); err != nil {
		return fmt.Errorf("agentgraph: record loop created %q: %w", loopID, err)
	}
	return nil
}

// RecordSpawn creates the child loop entity (with role and model) and then
// creates the parent→child relationship triple using PredicateSpawned.
// Both operations must succeed; a failure in either step returns an error.
func (h *Helper) RecordSpawn(ctx context.Context, parentLoopID, childLoopID, role, model string) error {
	if err := h.RecordLoopCreated(ctx, childLoopID, role, model); err != nil {
		return fmt.Errorf("agentgraph: record spawn — child entity: %w", err)
	}

	parentEntityID := LoopEntityID(parentLoopID)
	childEntityID := LoopEntityID(childLoopID)

	if err := h.entities.CreateRelationship(ctx, parentEntityID, childEntityID, PredicateSpawned, nil); err != nil {
		return fmt.Errorf("agentgraph: record spawn — relationship %q -> %q: %w",
			parentLoopID, childLoopID, err)
	}
	return nil
}

// RecordLoopStatus updates the status property triple on an existing loop entity.
// It uses UpdateEntityWithTriples to atomically replace the status predicate.
func (h *Helper) RecordLoopStatus(ctx context.Context, loopID, status string) error {
	entityID := LoopEntityID(loopID)
	now := time.Now()

	updated := propertyTriple(entityID, PredicateStatus, status, now)

	entity, err := h.entities.GetEntity(ctx, entityID)
	if err != nil {
		return fmt.Errorf("agentgraph: record loop status — get entity %q: %w", loopID, err)
	}

	if _, err := h.entities.UpdateEntityWithTriples(
		ctx,
		entity,
		[]message.Triple{updated},
		[]string{PredicateStatus},
	); err != nil {
		return fmt.Errorf("agentgraph: record loop status %q -> %q: %w", loopID, status, err)
	}
	return nil
}

// GetChildren returns the loop IDs of all direct children of the given loop.
// It queries outgoing PredicateSpawned relationships and extracts the Instance
// component from each resulting entity ID.
func (h *Helper) GetChildren(ctx context.Context, loopID string) ([]string, error) {
	entityID := LoopEntityID(loopID)

	childEntityIDs, err := h.queries.GetOutgoingRelationships(ctx, entityID, PredicateSpawned)
	if err != nil {
		return nil, fmt.Errorf("agentgraph: get children of %q: %w", loopID, err)
	}

	children := make([]string, 0, len(childEntityIDs))
	for _, eid := range childEntityIDs {
		parsed, parseErr := types.ParseEntityID(eid)
		if parseErr != nil {
			// Skip malformed IDs rather than failing the whole query; the graph
			// may hold entities created by other systems with different formats.
			continue
		}
		children = append(children, parsed.Instance)
	}
	return children, nil
}

// GetTree returns the entity IDs of all loop entities reachable from rootLoopID
// by following PredicateSpawned edges up to maxDepth hops.
// The root entity itself is included in the result.
// Callers should pass a reasonable maxDepth (e.g. 10) to bound traversal cost.
func (h *Helper) GetTree(ctx context.Context, rootLoopID string, maxDepth int) ([]string, error) {
	q := query.PathQuery{
		StartEntity:     LoopEntityID(rootLoopID),
		MaxDepth:        maxDepth,
		MaxNodes:        1000,
		MaxTime:         10 * time.Second,
		PredicateFilter: []string{PredicateSpawned},
		DecayFactor:     1.0,
		MaxPaths:        0,
	}

	result, err := h.queries.ExecutePathQuery(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("agentgraph: get tree from %q: %w", rootLoopID, err)
	}

	ids := make([]string, 0, len(result.Entities))
	for _, entity := range result.Entities {
		ids = append(ids, entity.ID)
	}
	return ids, nil
}

// GetStatus returns the current status value stored on a loop entity's
// PredicateStatus triple. If the entity exists but carries no status triple,
// an empty string is returned without error. This is the MVP path; callers
// that need live mutable state should read the AGENT_LOOPS KV bucket instead.
func (h *Helper) GetStatus(ctx context.Context, loopID string) (string, error) {
	entityID := LoopEntityID(loopID)

	entity, err := h.entities.GetEntity(ctx, entityID)
	if err != nil {
		return "", fmt.Errorf("agentgraph: get status — get entity %q: %w", loopID, err)
	}

	for _, t := range entity.Triples {
		if t.Predicate == PredicateStatus {
			if s, ok := t.Object.(string); ok {
				return s, nil
			}
		}
	}
	return "", nil
}

// propertyTriple constructs a property triple for a loop entity.
// Confidence is set to 1.0 because the values come directly from authoritative
// Semspec internal state rather than inferred or sensor data.
func propertyTriple(subject, predicate string, value any, ts time.Time) message.Triple {
	return message.Triple{
		Subject:    subject,
		Predicate:  predicate,
		Object:     value,
		Source:     SourceSemspec,
		Timestamp:  ts,
		Confidence: 1.0,
	}
}
