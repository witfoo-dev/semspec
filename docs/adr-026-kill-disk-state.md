# ADR-026: Kill Disk State — KV Twofer Is Truth

**Status**: Approved
**Date**: 2026-03-24
**Author**: Coby + Claude

## Context

Workflow state (plans, requirements, scenarios, change proposals) lives in `.semspec/*.json` files on disk. This reimplements what NATS KV already provides. A single KV put to ENTITY_STATES gives us state, events (via KV Watch), and history — for free. The disk JSON files bypass the entire semstreams platform.

**Semstreams principles (from concept docs):**
- **KV Twofer** (02-kv-twofer): The write IS the event. One KV put = state + notification + history.
- **Facts → KV, Requests → JetStream** (03-streams-vs-kv-watches): Entity status is a fact. "Generate requirements" is a request.
- **Rules trigger, workflows coordinate, components execute** (14-orchestration-layers): State transitions watched by rules, not polled by components reading files.
- **Use natsclient**: `natsclient.KVStore` has `Get`, `Put`, `UpdateWithRetry`, `KeysByPrefix`, `Watch` — no custom utilities needed.
- **Payload registry**: `component.CreatePayload(domain, category, version)` provides runtime type safety — returns nil for unregistered types. EntityPayload already registered for all 9 entity types via `init()`.

**What's wrong today**: Plan-api publishes entities to graph-ingest as a side effect, but all components read from disk. The graph is decoration; the filesystem is truth. This inverts the architecture.

**Anti-patterns identified:**
- `graphutil` package wraps NATS request/reply instead of using natsclient directly
- Components poll disk files for state instead of using KV Watch
- Dual-write (disk + graph publish) with reads only from disk
- Mutex maps for concurrency instead of KV CAS
- Custom unmarshal code instead of using payload registry for type safety

## Decision

Replace all disk-based workflow state with direct `natsclient.KVStore` operations on the ENTITY_STATES bucket. Manager holds a `*natsclient.KVStore` directly — no intermediate StateStore interface. Use semstreams-native types: `graph.EntityState` for storage, `graph.GetPropertyValue` / `graph.GetPropertyValueTyped[T]` for reads, `graph.MergeTriples` for CAS mutations, payload registry for runtime type validation.

---

## Design: Direct KV on ENTITY_STATES

### Storage format

ENTITY_STATES KV values are `graph.EntityState` JSON:

```go
type EntityState struct {
    ID          string           // 6-part entity ID (= KV key)
    Triples     []message.Triple // All semantic facts
    MessageType message.Type     // Provenance: "plan.entity.v1" etc.
    Version     uint64           // Optimistic concurrency
    UpdatedAt   time.Time
}
```

`MessageType` maps to the payload registry. `component.CreatePayload(type.Domain, type.Category, type.Version)` returns nil for unregistered types — runtime safety without custom validation code.

### Write pattern — the write IS the event

```go
entity := graph.EntityState{
    ID:          workflow.PlanEntityID(plan.Slug),
    Triples:     workflow.PlanTriples(plan),
    MessageType: workflow.EntityType,  // registered in init()
    UpdatedAt:   time.Now(),
}
data, _ := json.Marshal(entity)
kv.Put(ctx, entity.ID, data)
// Done. KV Watch notifies rules automatically.
```

### Read pattern — KV Get + payload registry validation

```go
entry, err := kv.Get(ctx, workflow.PlanEntityID(slug))
var entity graph.EntityState
json.Unmarshal(entry.Value, &entity)

// Runtime safety via payload registry
if component.CreatePayload(entity.MessageType.Domain, entity.MessageType.Category, entity.MessageType.Version) == nil {
    return nil, fmt.Errorf("unregistered entity type: %s", entity.MessageType)
}

plan, err := workflow.PlanFromTriples(entity.ID, entity.Triples)
```

### Mutation pattern — CAS with auto-retry

```go
kv.UpdateWithRetry(ctx, entityID, func(current []byte) ([]byte, error) {
    var entity graph.EntityState
    json.Unmarshal(current, &entity)
    entity.Triples = graph.MergeTriples(entity.Triples, []message.Triple{
        {Subject: entityID, Predicate: semspec.PredicatePlanStatus, Object: string(newStatus)},
    })
    entity.UpdatedAt = time.Now()
    return json.Marshal(entity)
})
```

### Collection pattern — prefix scan

```go
keys, _ := kv.KeysByPrefix(ctx, workflow.EntityPrefix()+".wf.plan.req.")
for _, key := range keys {
    entry, _ := kv.Get(ctx, key)
    var entity graph.EntityState
    json.Unmarshal(entry.Value, &entity)
    // filter by plan predicate, unmarshal
}
```

### Semstreams tools used

| Tool | Package | Purpose |
|------|---------|---------|
| `natsclient.KVStore` | `natsclient` | All KV ops: Get, Put, UpdateWithRetry, KeysByPrefix, Watch |
| `graph.EntityState` | `graph` | KV value struct (ID, Triples, MessageType, Version, UpdatedAt) |
| `graph.MergeTriples` | `graph` | Triple merge for CAS mutations (newer wins by predicate) |
| `graph.GetPropertyValue` | `graph` | Extract single property from EntityState by predicate |
| `graph.GetPropertyValueTyped[T]` | `graph` | Typed property extraction (generic) |
| `component.CreatePayload` | `component` | Payload registry lookup — runtime type validation |
| `message.Triple` | `message` | Triple struct (Subject, Predicate, Object, Source, Timestamp) |
| Vocabulary predicates | `vocabulary/semspec` | All predicate constants (already defined) |
| Entity ID helpers | `workflow/entity.go` | PlanEntityID, RequirementEntityID, etc. (already defined) |
| Entity message types | `workflow/entity.go` | EntityType, RequirementEntityType, etc. (already registered) |

### What stays on disk (legitimate filesystem use)

- Config: `project.json`, `checklist.json`, `standards.json`
- Constitution: `constitution.md`
- Artifact exports: spec markdown, archive summaries

---

## Implementation

### Phase 1: Triple Marshal/Unmarshal Helpers

#### 1a. Triple marshal functions (extract from plan-api/graph.go)

**New file**: `workflow/graph_marshal.go`

Extract triple-building logic from `processor/plan-api/graph.go` into standalone functions. The code already exists — we're moving it to `workflow/` so Manager can use it.

```go
func PlanTriples(plan *Plan) []message.Triple           // from publishPlanEntity
func RequirementTriples(planSlug string, req *Requirement) []message.Triple  // from publishRequirementEntity
func ScenarioTriples(planSlug string, s *Scenario) []message.Triple         // from publishScenarioEntity
func ChangeProposalTriples(planSlug string, p *ChangeProposal) []message.Triple  // from publishChangeProposalEntity
```

Uses existing predicate constants from `vocabulary/semspec/predicates.go` and entity ID helpers from `workflow/entity.go`.

#### 1b. Triple unmarshal functions

**New file**: `workflow/graph_unmarshal.go`

```go
func PlanFromTriples(entityID string, triples []message.Triple) (*Plan, error)
func RequirementFromTriples(entityID string, triples []message.Triple) (*Requirement, error)
func ScenarioFromTriples(entityID string, triples []message.Triple) (*Scenario, error)
func ChangeProposalFromTriples(entityID string, triples []message.Triple) (*ChangeProposal, error)
```

Use `graph.GetPropertyValue` / `graph.GetPropertyValueTyped[T]` for single-valued properties. For multi-valued predicates (DependsOn, Then, AffectedReqIDs), iterate `triples` and collect all matches for the predicate.

#### 1c. Tests — pure function round-trips

**New file**: `workflow/graph_marshal_test.go`

Table-driven: struct → XxxTriples → XxxFromTriples → assert equality. Cover multi-valued fields (DependsOn with 3 deps, Then with 5 outcomes). No NATS needed — pure function tests.

---

### Phase 2: Manager Gets KVStore, Drops Disk State

#### 2a. Manager holds KVStore directly

**File**: `workflow/structure.go`

```go
type Manager struct {
    repoRoot string              // artifact paths + config reads
    kv       *natsclient.KVStore // ENTITY_STATES bucket — nil only in migrate.go
}

func NewManager(repoRoot string, kv *natsclient.KVStore) *Manager {
    return &Manager{repoRoot: repoRoot, kv: kv}
}
```

No interface. No abstraction layer. Components pass their KVStore directly.

#### 2b. Manager state methods use KV + helpers

**Files**: `workflow/plan.go`, `workflow/plan_requirement.go`, `workflow/plan_scenario.go`, `workflow/plan_change_proposal.go`

Each method follows the same pattern:

```go
func (m *Manager) SavePlan(ctx context.Context, plan *Plan) error {
    entity := graph.EntityState{
        ID:          PlanEntityID(plan.Slug),
        Triples:     PlanTriples(plan),
        MessageType: EntityType,
        UpdatedAt:   time.Now(),
    }
    data, err := json.Marshal(entity)
    if err != nil { return fmt.Errorf("marshal plan entity: %w", err) }
    _, err = m.kv.Put(ctx, entity.ID, data)
    return err
}

func (m *Manager) LoadPlan(ctx context.Context, slug string) (*Plan, error) {
    entry, err := m.kv.Get(ctx, PlanEntityID(slug))
    if err != nil {
        if natsclient.IsKVNotFoundError(err) { return nil, ErrPlanNotFound }
        return nil, err
    }
    var entity graph.EntityState
    if err := json.Unmarshal(entry.Value, &entity); err != nil { return nil, err }
    // Payload registry runtime validation
    if component.CreatePayload(entity.MessageType.Domain, entity.MessageType.Category, entity.MessageType.Version) == nil {
        return nil, fmt.Errorf("unregistered entity type: %s", entity.MessageType)
    }
    return PlanFromTriples(entity.ID, entity.Triples)
}

func (m *Manager) SetPlanStatus(ctx context.Context, slug string, status Status) error {
    entityID := PlanEntityID(slug)
    return m.kv.UpdateWithRetry(ctx, entityID, func(current []byte) ([]byte, error) {
        var entity graph.EntityState
        if err := json.Unmarshal(current, &entity); err != nil { return nil, err }
        entity.Triples = graph.MergeTriples(entity.Triples, []message.Triple{
            {Subject: entityID, Predicate: semspec.PredicatePlanStatus, Object: string(status)},
        })
        entity.UpdatedAt = time.Now()
        return json.Marshal(entity)
    })
}

func (m *Manager) LoadRequirements(ctx context.Context, planSlug string) ([]Requirement, error) {
    keys, err := m.kv.KeysByPrefix(ctx, EntityPrefix()+".wf.plan.req.")
    if err != nil { return nil, err }
    planEntityID := PlanEntityID(planSlug)
    var reqs []Requirement
    for _, key := range keys {
        entry, err := m.kv.Get(ctx, key)
        if err != nil { continue }
        var entity graph.EntityState
        if err := json.Unmarshal(entry.Value, &entity); err != nil { continue }
        req, err := RequirementFromTriples(entity.ID, entity.Triples)
        if err != nil { continue }
        // Filter: only requirements belonging to this plan
        if req.PlanID == planEntityID {
            reqs = append(reqs, *req)
        }
    }
    return reqs, nil
}
```

All `os.ReadFile`/`os.WriteFile`/`json.MarshalIndent` for state files deleted. Mutex maps (`taskLocks`, `projectLocks`) deleted — KV CAS via `UpdateWithRetry` handles concurrency.

#### 2c. Clean up plan-api/graph.go

**File**: `processor/plan-api/graph.go`

Delete `publishPlanEntity`, `publishRequirementEntity`, `publishScenarioEntity`, `publishChangeProposalEntity` — replaced by Manager's KV writes. Keep:
- `publishGraphEntity` helper (used by non-workflow entity publishes)
- `publishQuestionEntity`, `publishApprovalEntity`, `publishDAGNodeEntity`

---

### Phase 3: Rewire All Callers

Every `workflow.NewManager(repoRoot)` → `workflow.NewManager(repoRoot, kvStore)`.

#### Component wiring pattern

```go
// In component Start():
bucket, err := c.natsClient.CreateKeyValueBucket(ctx, jetstream.KeyValueConfig{
    Bucket: "ENTITY_STATES",
})
if err != nil { return err }
kv := c.natsClient.NewKVStore(bucket)
c.manager = workflow.NewManager(repoRoot, kv)
```

#### Callers to update

| Component | File | Sites |
|-----------|------|-------|
| plan-api HTTP | `processor/plan-api/http.go:204` | `getManager()` |
| plan-api events | `processor/plan-api/events.go:1127` | `getManager()` |
| plan-api coordinator | `processor/plan-api/coordinator.go:867,1066,1324` | 3 inline → 1 field |
| requirement-generator | `processor/requirement-generator/component.go:343,550` | 2 inline → 1 field |
| scenario-generator | `processor/scenario-generator/component.go:358,647` | 2 inline → 1 field |
| change-proposal-handler | `processor/change-proposal-handler/component.go:238` | 1 site |
| scenario-orchestrator | `processor/scenario-orchestrator/component.go:287` | 1 site |
| trajectory-api | `processor/trajectory-api/component.go:161` | stored as field |
| document tool | `tools/workflow/document.go:141,223,276,305` | Wire NATS through |
| migrate.go | `cmd/semspec/migrate.go:77` | Reads old disk format (that's its job) |

Components creating Manager multiple times should create it once in `Start()` and store as a field.

---

### Phase 4: Update Tests

#### Marshal/unmarshal tests — pure functions, no NATS

`workflow/graph_marshal_test.go` — table-driven round-trip tests. No KV needed.

#### Manager tests — embedded NATS

Tests that need a Manager with KV use an embedded NATS test server:

```go
func setupTestManager(t *testing.T) *workflow.Manager {
    // Use nats-server/v2/server for embedded test server
    ns, _ := server.NewServer(&server.Options{JetStream: true})
    ns.Start()
    t.Cleanup(ns.Shutdown)

    nc, _ := natsclient.Connect(ns.ClientURL())
    bucket, _ := nc.CreateKeyValueBucket(ctx, jetstream.KeyValueConfig{Bucket: "ENTITY_STATES"})
    kv := nc.NewKVStore(bucket)
    return workflow.NewManager(t.TempDir(), kv)
}
```

This tests the real KV path — no mocks, no fake implementations.

#### Files to update

- `workflow/plan_test.go`, `workflow/project_test.go`
- `processor/plan-api/coordinator_test.go`, `coordinator_entity_test.go`
- `processor/execution-orchestrator/*_test.go`
- `processor/requirement-executor/*_test.go`
- `processor/change-proposal-handler/*_test.go`

---

### Phase 5: Clean Up

- Delete disk state constants (`PlanFile`, `RequirementsFile`, etc.)
- Delete `workflow/plan_task.go` disk operations (tasks already graph-based)
- Delete mutex maps (`taskLocks`, `projectLocks`) — KV CAS replaces them
- `PortSubject` in graphutil → move to component utils or inline (not graph-related)
- Note: graphutil.WriteTriple (114 operational state calls) flagged as separate cleanup

---

## Files

### New (3)

| File | Purpose |
|------|---------|
| `workflow/graph_marshal.go` | Struct → triples (extract from plan-api/graph.go) |
| `workflow/graph_unmarshal.go` | Triples → struct (using graph.GetPropertyValue + payload registry) |
| `workflow/graph_marshal_test.go` | Round-trip tests (pure functions, no NATS) |

### Modified — core (7)

| File | Change |
|------|--------|
| `workflow/structure.go` | Manager gets `*natsclient.KVStore` field, new constructor |
| `workflow/plan.go` | KV reads/writes via Manager.kv, delete disk I/O |
| `workflow/plan_requirement.go` | KV reads/writes, delete disk I/O |
| `workflow/plan_scenario.go` | KV reads/writes, delete disk I/O |
| `workflow/plan_change_proposal.go` | KV reads/writes, delete disk I/O |
| `workflow/project.go` | Delete disk state, keep config reads |
| `processor/plan-api/graph.go` | Delete redundant entity publish methods |

### Modified — callers (10)

| File | Change |
|------|--------|
| `processor/plan-api/http.go` | Wire KVStore from natsClient |
| `processor/plan-api/events.go` | Wire KVStore from natsClient |
| `processor/plan-api/coordinator.go` | Wire KVStore, Manager as field |
| `processor/requirement-generator/component.go` | Wire KVStore |
| `processor/scenario-generator/component.go` | Wire KVStore |
| `processor/change-proposal-handler/component.go` | Wire KVStore |
| `processor/scenario-orchestrator/component.go` | Wire KVStore |
| `processor/trajectory-api/component.go` | Wire KVStore |
| `tools/workflow/document.go` | Wire NATS for plan reads |
| `cmd/semspec/migrate.go` | Update output target (optional) |

### Modified — tests (8+)

All tests using Manager get embedded NATS test server or pure function tests.

---

## Verification

1. **Unit**: `go test ./workflow/...` — round-trip marshal tests
2. **Build**: `go build ./...` — all callers compile
3. **E2E**: `task e2e:run -- plan-workflow` — REST API CRUD via KV
4. **E2E**: `task e2e:run -- scenario-execution` — DAG gating from KV
5. **E2E**: `task e2e:mock -- hello-world` — full lifecycle
6. **KV check**: After plan creation, verify entity in ENTITY_STATES:
   ```bash
   curl http://localhost:8180/message-logger/kv/ENTITY_STATES | \
     jq 'to_entries[] | select(.key | contains("wf.plan")) | .key'
   ```
7. **Rules**: Confirm rules still fire on state changes (check message-logger for `workflow.events.plan.approved` after approval)

## Consequences

- All workflow state queries go through NATS KV — requires NATS to be running
- Tests use embedded NATS server (real KV path, no mocks)
- Marshal/unmarshal tests are pure functions (no NATS)
- Payload registry provides runtime type safety on read — unregistered types fail fast
- KV Watch enables future reactive patterns (scenario-orchestrator watching for status changes)
- graphutil.WriteTriple (114 operational state calls) flagged for separate cleanup
- migrate.go CLI retains disk-read capability for migrating existing plans
