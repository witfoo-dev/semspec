package agentgraph_test

import (
	"context"
	"errors"
	"testing"
	"time"

	gtypes "github.com/c360studio/semstreams/graph"
	"github.com/c360studio/semstreams/graph/datamanager"
	"github.com/c360studio/semstreams/graph/query"
	"github.com/c360studio/semstreams/message"

	"github.com/c360studio/semspec/agentgraph"
	semspecvocab "github.com/c360studio/semspec/vocabulary/semspec"
)

// -- mock implementations --

// mockEntityManager is a minimal in-memory EntityManager for testing.
// Only the methods exercised by Helper are implemented; others panic to surface
// unexpected calls in tests.
type mockEntityManager struct {
	upserted      []*gtypes.EntityState
	updated       []*gtypes.EntityState
	entities      map[string]*gtypes.EntityState
	relationships []relationship
	upsertErr     error
	updateErr     error
	getErr        error
	relErr        error
}

type relationship struct {
	from, to, predicate string
}

func newMockEntityManager() *mockEntityManager {
	return &mockEntityManager{
		entities: make(map[string]*gtypes.EntityState),
	}
}

func (m *mockEntityManager) UpsertEntity(_ context.Context, entity *gtypes.EntityState) (*gtypes.EntityState, error) {
	if m.upsertErr != nil {
		return nil, m.upsertErr
	}
	m.upserted = append(m.upserted, entity)
	m.entities[entity.ID] = entity
	return entity, nil
}

func (m *mockEntityManager) GetEntity(_ context.Context, id string) (*gtypes.EntityState, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if e, ok := m.entities[id]; ok {
		return e, nil
	}
	return &gtypes.EntityState{ID: id, Triples: nil}, nil
}

func (m *mockEntityManager) UpdateEntityWithTriples(_ context.Context, entity *gtypes.EntityState, add []message.Triple, _ []string) (*gtypes.EntityState, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	m.updated = append(m.updated, entity)
	existing, ok := m.entities[entity.ID]
	if !ok {
		existing = entity
	}
	existing.Triples = append(existing.Triples, add...)
	m.entities[entity.ID] = existing
	return existing, nil
}

func (m *mockEntityManager) CreateRelationship(_ context.Context, from, to, predicate string, _ map[string]any) error {
	if m.relErr != nil {
		return m.relErr
	}
	m.relationships = append(m.relationships, relationship{from: from, to: to, predicate: predicate})
	return nil
}

// Remaining EntityManager interface methods — not exercised by Helper but required
// for interface satisfaction.

func (m *mockEntityManager) CreateEntity(_ context.Context, e *gtypes.EntityState) (*gtypes.EntityState, error) {
	panic("CreateEntity not implemented in mockEntityManager")
}
func (m *mockEntityManager) UpdateEntity(_ context.Context, e *gtypes.EntityState) (*gtypes.EntityState, error) {
	panic("UpdateEntity not implemented in mockEntityManager")
}
func (m *mockEntityManager) DeleteEntity(_ context.Context, _ string) error {
	panic("DeleteEntity not implemented in mockEntityManager")
}
func (m *mockEntityManager) ExistsEntity(_ context.Context, _ string) (bool, error) {
	panic("ExistsEntity not implemented in mockEntityManager")
}
func (m *mockEntityManager) BatchGet(_ context.Context, _ []string) ([]*gtypes.EntityState, error) {
	panic("BatchGet not implemented in mockEntityManager")
}
func (m *mockEntityManager) ListWithPrefix(_ context.Context, _ string) ([]string, error) {
	panic("ListWithPrefix not implemented in mockEntityManager")
}
func (m *mockEntityManager) CreateEntityWithTriples(_ context.Context, e *gtypes.EntityState, _ []message.Triple) (*gtypes.EntityState, error) {
	panic("CreateEntityWithTriples not implemented in mockEntityManager")
}
func (m *mockEntityManager) BatchWrite(_ context.Context, _ []datamanager.EntityWrite) error {
	panic("BatchWrite not implemented in mockEntityManager")
}
func (m *mockEntityManager) List(_ context.Context, _ string) ([]string, error) {
	panic("List not implemented in mockEntityManager")
}
func (m *mockEntityManager) AddTriple(_ context.Context, _ message.Triple) error {
	panic("AddTriple not implemented in mockEntityManager")
}
func (m *mockEntityManager) RemoveTriple(_ context.Context, _, _ string) error {
	panic("RemoveTriple not implemented in mockEntityManager")
}
func (m *mockEntityManager) DeleteRelationship(_ context.Context, _, _, _ string) error {
	panic("DeleteRelationship not implemented in mockEntityManager")
}

// mockQueryClient is a minimal query.Client for testing.
type mockQueryClient struct {
	outgoing     map[string][]string // entityID -> list of child entity IDs
	pathEntities []*gtypes.EntityState
	outgoingErr  error
	pathErr      error
}

func newMockQueryClient() *mockQueryClient {
	return &mockQueryClient{
		outgoing: make(map[string][]string),
	}
}

func (m *mockQueryClient) GetOutgoingRelationships(_ context.Context, entityID, _ string) ([]string, error) {
	if m.outgoingErr != nil {
		return nil, m.outgoingErr
	}
	return m.outgoing[entityID], nil
}

func (m *mockQueryClient) ExecutePathQuery(_ context.Context, _ query.PathQuery) (*query.PathResult, error) {
	if m.pathErr != nil {
		return nil, m.pathErr
	}
	return &query.PathResult{Entities: m.pathEntities}, nil
}

// Remaining query.Client interface methods — not exercised by Helper.

func (m *mockQueryClient) GetEntity(_ context.Context, _ string) (*gtypes.EntityState, error) {
	panic("GetEntity not implemented in mockQueryClient")
}
func (m *mockQueryClient) GetEntitiesByType(_ context.Context, _ string) ([]*gtypes.EntityState, error) {
	panic("GetEntitiesByType not implemented in mockQueryClient")
}
func (m *mockQueryClient) GetEntitiesBatch(_ context.Context, _ []string) ([]*gtypes.EntityState, error) {
	panic("GetEntitiesBatch not implemented in mockQueryClient")
}
func (m *mockQueryClient) ListEntities(_ context.Context) ([]string, error) {
	panic("ListEntities not implemented in mockQueryClient")
}
func (m *mockQueryClient) CountEntities(_ context.Context) (int, error) {
	panic("CountEntities not implemented in mockQueryClient")
}
func (m *mockQueryClient) GetIncomingRelationships(_ context.Context, _ string) ([]string, error) {
	panic("GetIncomingRelationships not implemented in mockQueryClient")
}
func (m *mockQueryClient) GetEntityConnections(_ context.Context, _ string) ([]*gtypes.EntityState, error) {
	panic("GetEntityConnections not implemented in mockQueryClient")
}
func (m *mockQueryClient) VerifyRelationship(_ context.Context, _, _, _ string) (bool, error) {
	panic("VerifyRelationship not implemented in mockQueryClient")
}
func (m *mockQueryClient) CountIncomingRelationships(_ context.Context, _ string) (int, error) {
	panic("CountIncomingRelationships not implemented in mockQueryClient")
}
func (m *mockQueryClient) QueryEntities(_ context.Context, _ map[string]any) ([]*gtypes.EntityState, error) {
	panic("QueryEntities not implemented in mockQueryClient")
}
func (m *mockQueryClient) GetEntitiesInRegion(_ context.Context, _ string) ([]*gtypes.EntityState, error) {
	panic("GetEntitiesInRegion not implemented in mockQueryClient")
}
func (m *mockQueryClient) GetEntitiesByPredicate(_ context.Context, _ string) ([]string, error) {
	panic("GetEntitiesByPredicate not implemented in mockQueryClient")
}
func (m *mockQueryClient) ListPredicates(_ context.Context) ([]gtypes.PredicateSummary, error) {
	panic("ListPredicates not implemented in mockQueryClient")
}
func (m *mockQueryClient) GetPredicateStats(_ context.Context, _ string, _ int) (*gtypes.PredicateStatsData, error) {
	panic("GetPredicateStats not implemented in mockQueryClient")
}
func (m *mockQueryClient) QueryCompoundPredicates(_ context.Context, _ gtypes.CompoundPredicateQuery) ([]string, error) {
	panic("QueryCompoundPredicates not implemented in mockQueryClient")
}
func (m *mockQueryClient) GetCacheStats() query.CacheStats { return query.CacheStats{} }
func (m *mockQueryClient) Clear() error                    { return nil }
func (m *mockQueryClient) Close() error                    { return nil }

// -- helpers --

func tripleByPredicate(triples []message.Triple, predicate string) *message.Triple {
	for i := range triples {
		if triples[i].Predicate == predicate {
			return &triples[i]
		}
	}
	return nil
}

// -- tests --

func TestHelper_RecordLoopCreated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		loopID    string
		role      string
		model     string
		upsertErr error
		wantErr   bool
	}{
		{
			name:    "creates entity with role model status triples",
			loopID:  "loop-1",
			role:    "planner",
			model:   "gpt-4o",
			wantErr: false,
		},
		{
			name:      "propagates upsert error",
			loopID:    "loop-err",
			role:      "executor",
			model:     "gpt-4o-mini",
			upsertErr: errors.New("nats: bucket unavailable"),
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			em := newMockEntityManager()
			em.upsertErr = tc.upsertErr
			h := agentgraph.NewHelper(em, newMockQueryClient())

			err := h.RecordLoopCreated(context.Background(), tc.loopID, tc.role, tc.model)

			if (err != nil) != tc.wantErr {
				t.Fatalf("RecordLoopCreated() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}

			if len(em.upserted) != 1 {
				t.Fatalf("expected 1 upserted entity, got %d", len(em.upserted))
			}

			entity := em.upserted[0]
			wantID := agentgraph.LoopEntityID(tc.loopID)
			if entity.ID != wantID {
				t.Errorf("entity ID = %q, want %q", entity.ID, wantID)
			}

			// Verify all three property triples are present.
			roleT := tripleByPredicate(entity.Triples, agentgraph.PredicateRole)
			if roleT == nil {
				t.Error("missing role triple")
			} else if roleT.Object != tc.role {
				t.Errorf("role triple object = %v, want %q", roleT.Object, tc.role)
			}

			modelT := tripleByPredicate(entity.Triples, agentgraph.PredicateModel)
			if modelT == nil {
				t.Error("missing model triple")
			} else if modelT.Object != tc.model {
				t.Errorf("model triple object = %v, want %q", modelT.Object, tc.model)
			}

			statusT := tripleByPredicate(entity.Triples, agentgraph.PredicateStatus)
			if statusT == nil {
				t.Error("missing status triple")
			} else if statusT.Object != "created" {
				t.Errorf("status triple object = %v, want \"created\"", statusT.Object)
			}

			// All triples must carry the Semspec source.
			for _, triple := range entity.Triples {
				if triple.Source != agentgraph.SourceSemspec {
					t.Errorf("triple %q has source %q, want %q", triple.Predicate, triple.Source, agentgraph.SourceSemspec)
				}
			}
		})
	}
}

func TestHelper_RecordSpawn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		parentLoopID string
		childLoopID  string
		role         string
		model        string
		upsertErr    error
		relErr       error
		wantErr      bool
		wantRel      bool
	}{
		{
			name:         "creates child entity and relationship",
			parentLoopID: "parent-1",
			childLoopID:  "child-1",
			role:         "executor",
			model:        "gpt-4o-mini",
			wantErr:      false,
			wantRel:      true,
		},
		{
			name:         "fails when child entity creation fails",
			parentLoopID: "parent-2",
			childLoopID:  "child-2",
			role:         "executor",
			model:        "gpt-4o-mini",
			upsertErr:    errors.New("storage error"),
			wantErr:      true,
			wantRel:      false,
		},
		{
			name:         "fails when relationship creation fails",
			parentLoopID: "parent-3",
			childLoopID:  "child-3",
			role:         "executor",
			model:        "gpt-4o-mini",
			relErr:       errors.New("relationship error"),
			wantErr:      true,
			wantRel:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			em := newMockEntityManager()
			em.upsertErr = tc.upsertErr
			em.relErr = tc.relErr
			h := agentgraph.NewHelper(em, newMockQueryClient())

			err := h.RecordSpawn(context.Background(), tc.parentLoopID, tc.childLoopID, tc.role, tc.model)

			if (err != nil) != tc.wantErr {
				t.Fatalf("RecordSpawn() error = %v, wantErr %v", err, tc.wantErr)
			}

			if tc.wantRel {
				if len(em.relationships) != 1 {
					t.Fatalf("expected 1 relationship, got %d", len(em.relationships))
				}
				rel := em.relationships[0]
				wantFrom := agentgraph.LoopEntityID(tc.parentLoopID)
				wantTo := agentgraph.LoopEntityID(tc.childLoopID)
				if rel.from != wantFrom {
					t.Errorf("relationship from = %q, want %q", rel.from, wantFrom)
				}
				if rel.to != wantTo {
					t.Errorf("relationship to = %q, want %q", rel.to, wantTo)
				}
				if rel.predicate != agentgraph.PredicateSpawned {
					t.Errorf("relationship predicate = %q, want %q", rel.predicate, agentgraph.PredicateSpawned)
				}
			}
		})
	}
}

func TestHelper_RecordLoopStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		loopID    string
		status    string
		getErr    error
		updateErr error
		wantErr   bool
	}{
		{
			name:    "updates status triple successfully",
			loopID:  "loop-1",
			status:  "running",
			wantErr: false,
		},
		{
			name:    "propagates get error",
			loopID:  "loop-bad",
			status:  "running",
			getErr:  errors.New("entity not found"),
			wantErr: true,
		},
		{
			name:      "propagates update error",
			loopID:    "loop-upd-err",
			status:    "failed",
			updateErr: errors.New("CAS conflict"),
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			em := newMockEntityManager()
			em.getErr = tc.getErr
			em.updateErr = tc.updateErr

			// Pre-populate so GetEntity returns something sensible when no error.
			if tc.getErr == nil {
				em.entities[agentgraph.LoopEntityID(tc.loopID)] = &gtypes.EntityState{
					ID:      agentgraph.LoopEntityID(tc.loopID),
					Triples: []message.Triple{},
				}
			}

			h := agentgraph.NewHelper(em, newMockQueryClient())

			err := h.RecordLoopStatus(context.Background(), tc.loopID, tc.status)

			if (err != nil) != tc.wantErr {
				t.Fatalf("RecordLoopStatus() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}

			// Verify that an UpdateEntityWithTriples call occurred.
			if len(em.updated) == 0 {
				t.Fatal("expected UpdateEntityWithTriples to be called")
			}

			// After the update, the stored entity should contain the status triple.
			stored := em.entities[agentgraph.LoopEntityID(tc.loopID)]
			statusT := tripleByPredicate(stored.Triples, agentgraph.PredicateStatus)
			if statusT == nil {
				t.Error("status triple not found after update")
			} else if statusT.Object != tc.status {
				t.Errorf("status triple object = %v, want %q", statusT.Object, tc.status)
			}
		})
	}
}

func TestHelper_GetChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		loopID        string
		childEntities []string // entity IDs returned by mock
		outgoingErr   error
		wantChildren  []string
		wantErr       bool
	}{
		{
			name:         "returns empty slice when loop has no children",
			loopID:       "root",
			wantChildren: []string{},
		},
		{
			name:   "returns loop IDs extracted from entity IDs",
			loopID: "parent",
			childEntities: []string{
				agentgraph.LoopEntityID("child-a"),
				agentgraph.LoopEntityID("child-b"),
			},
			wantChildren: []string{"child-a", "child-b"},
		},
		{
			name:        "propagates query error",
			loopID:      "err-loop",
			outgoingErr: errors.New("nats timeout"),
			wantErr:     true,
		},
		{
			name:   "skips malformed entity IDs",
			loopID: "parent-skip",
			childEntities: []string{
				agentgraph.LoopEntityID("valid-child"),
				"not-a-valid-entity-id",
			},
			wantChildren: []string{"valid-child"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			qc := newMockQueryClient()
			qc.outgoingErr = tc.outgoingErr
			if tc.childEntities != nil {
				qc.outgoing[agentgraph.LoopEntityID(tc.loopID)] = tc.childEntities
			}

			h := agentgraph.NewHelper(newMockEntityManager(), qc)

			children, err := h.GetChildren(context.Background(), tc.loopID)

			if (err != nil) != tc.wantErr {
				t.Fatalf("GetChildren() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}

			if len(children) != len(tc.wantChildren) {
				t.Fatalf("GetChildren() returned %d children, want %d: %v", len(children), len(tc.wantChildren), children)
			}
			for i, want := range tc.wantChildren {
				if children[i] != want {
					t.Errorf("children[%d] = %q, want %q", i, children[i], want)
				}
			}
		})
	}
}

func TestHelper_GetTree(t *testing.T) {
	t.Parallel()

	root := agentgraph.LoopEntityID("root")
	child1 := agentgraph.LoopEntityID("child-1")
	child2 := agentgraph.LoopEntityID("child-2")

	now := time.Now()

	tests := []struct {
		name         string
		rootLoopID   string
		maxDepth     int
		pathEntities []*gtypes.EntityState
		pathErr      error
		wantIDs      []string
		wantErr      bool
	}{
		{
			name:       "returns all traversed entity IDs",
			rootLoopID: "root",
			maxDepth:   5,
			pathEntities: []*gtypes.EntityState{
				{ID: root, UpdatedAt: now},
				{ID: child1, UpdatedAt: now},
				{ID: child2, UpdatedAt: now},
			},
			wantIDs: []string{root, child1, child2},
		},
		{
			name:         "returns empty slice when no entities visited",
			rootLoopID:   "lonely-root",
			maxDepth:     3,
			pathEntities: []*gtypes.EntityState{},
			wantIDs:      []string{},
		},
		{
			name:       "propagates path query error",
			rootLoopID: "err-root",
			maxDepth:   2,
			pathErr:    errors.New("query timeout"),
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			qc := newMockQueryClient()
			qc.pathEntities = tc.pathEntities
			qc.pathErr = tc.pathErr

			h := agentgraph.NewHelper(newMockEntityManager(), qc)

			ids, err := h.GetTree(context.Background(), tc.rootLoopID, tc.maxDepth)

			if (err != nil) != tc.wantErr {
				t.Fatalf("GetTree() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}

			if len(ids) != len(tc.wantIDs) {
				t.Fatalf("GetTree() returned %d IDs, want %d: %v", len(ids), len(tc.wantIDs), ids)
			}
			got := make(map[string]bool, len(ids))
			for _, id := range ids {
				got[id] = true
			}
			for _, wantID := range tc.wantIDs {
				if !got[wantID] {
					t.Errorf("GetTree() missing expected entity ID %q", wantID)
				}
			}
		})
	}
}

func TestHelper_GetStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		loopID     string
		triples    []message.Triple
		getErr     error
		wantStatus string
		wantErr    bool
	}{
		{
			name:   "returns status when triple is present",
			loopID: "loop-1",
			triples: []message.Triple{
				{Predicate: agentgraph.PredicateStatus, Object: "running"},
			},
			wantStatus: "running",
		},
		{
			name:       "returns empty string when no status triple",
			loopID:     "loop-nostatus",
			triples:    []message.Triple{},
			wantStatus: "",
		},
		{
			name:    "propagates get error",
			loopID:  "loop-err",
			getErr:  errors.New("entity not found"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			em := newMockEntityManager()
			em.getErr = tc.getErr
			if tc.getErr == nil {
				em.entities[agentgraph.LoopEntityID(tc.loopID)] = &gtypes.EntityState{
					ID:      agentgraph.LoopEntityID(tc.loopID),
					Triples: tc.triples,
				}
			}

			h := agentgraph.NewHelper(em, newMockQueryClient())

			status, err := h.GetStatus(context.Background(), tc.loopID)

			if (err != nil) != tc.wantErr {
				t.Fatalf("GetStatus() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if status != tc.wantStatus {
				t.Errorf("GetStatus() = %q, want %q", status, tc.wantStatus)
			}
		})
	}
}

// TestPredicateAlignment asserts that the convenience predicate constants in
// agentgraph/graph.go carry identical string values to the canonical constants
// registered in vocabulary/semspec/predicates.go. This catches accidental
// divergence between the two sets of constants (Phase 1 review recommendation #1).
func TestPredicateAlignment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		agentgraphConst string
		vocabConst      string
		name            string
	}{
		{agentgraph.PredicateSpawned, semspecvocab.PredicateLoopSpawned, "Spawned"},
		{agentgraph.PredicateLoopTask, semspecvocab.PredicateLoopTaskLink, "LoopTask"},
		{agentgraph.PredicateDependsOn, semspecvocab.PredicateTaskDependsOn, "DependsOn"},
		{agentgraph.PredicateRole, semspecvocab.PredicateAgenticLoopRole, "Role"},
		{agentgraph.PredicateModel, semspecvocab.PredicateAgenticLoopModel, "Model"},
		{agentgraph.PredicateStatus, semspecvocab.PredicateAgenticLoopStatus, "Status"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.agentgraphConst != tc.vocabConst {
				t.Errorf("agentgraph predicate %q != vocabulary predicate %q", tc.agentgraphConst, tc.vocabConst)
			}
		})
	}
}
