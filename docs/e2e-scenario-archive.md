# E2E Scenario Archive — Playwright Migration Reference

This document is the authoritative reference for four real-LLM E2E scenarios being migrated from
the Go backend test runner to Playwright. Each entry contains everything the Playwright team needs
to recreate the scenario: plan prompt, workspace setup, stage sequence with timeouts, key
assertions, config requirements, and non-obvious implementation details.

See [docs/11-execution-pipeline.md](11-execution-pipeline.md) for the pipeline stages these
scenarios exercise. See [docs/12-plan-api.md](12-plan-api.md) for the REST API calls used during
verification stages.

---

## Scenario Index

| # | Name | Tier | Description |
|---|------|------|-------------|
| 1 | [health-check](#1-health-check) | 2 | Go HTTP service — add /health endpoint |
| 2 | [rest-api](#2-rest-api) | 3 | Go HTTP service — CRUD + middleware |
| 3 | [todo-app](#3-todo-app) | Brownfield | Go+Svelte — add due dates with SOP enforcement |
| 4 | [epic-meshtastic](#4-epic-meshtastic) | 4 — Epic | Federated graph → Meshtastic OSH driver |

---

## Shared Patterns

All four scenarios share these implementation patterns. Read this section before implementing any
individual scenario.

### Stage Runner

Scenarios execute as a sequential list of named stages. Each stage has an individual timeout. The
runner short-circuits on the first stage failure — subsequent stages do not run. This makes timeout
attribution straightforward: the failing stage name identifies where the pipeline stalled.

The `FastTimeouts` flag (set by mock/CI environments) halves all stage timeouts. Account for this
when choosing timeout values — the halved value must still be generous enough for a healthy run.

### Polling Pattern

Stages that wait for async pipeline progress use a ticker-based polling loop:

- Tick interval: 3–5 seconds
- Loop exits when the expected condition is met or the stage context is cancelled
- Always respect context cancellation — do not use `time.Sleep`

Example condition: poll `GET /plan-api/plans/{slug}` until `status` equals `approved` or until the
stage deadline expires.

### Graph Readiness Gate

All scenarios call a graph readiness check before creating the plan. This polls the graph-gateway
until the workspace has been indexed. The gate prevents the planner from receiving an empty graph
context.

- Use `gatherers.NewGraphGatherer().WaitForReady()` (Go runner) or poll
  `GET /graph-gateway/status` in Playwright
- Timeout: 30 seconds for Tier 2–3 scenarios; up to 10 minutes for the epic scenario

### Message Logger Assertions

Two message-logger subjects are checked during execution stages:

- `agent.task.*` — confirms the execution pipeline dispatched at least one task
- `agent.complete.*` — count grows as agentic loops finish; verify the count increases beyond the
  baseline captured before execution was triggered

Query endpoint: `GET /message-logger/entries?subject=agent.task.*&limit=100`

Message-logger returns entries **newest first**. Sort by timestamp for chronological analysis.

### Deliverable Warnings vs Failures

In real-LLM mode the pipeline may still be running when the final verification stage fires.
Deliverable checks (new files in workspace, file content assertions) should emit warnings rather
than hard failures, because a slightly slow model will produce correct output after the stage
deadline. Treat missing deliverables as a signal to investigate logs, not as an automatic test
failure.

### Mock Stats Verification

When `MockLLMURL` is set, fetch `GET {MockLLMURL}/stats` after execution completes to assert
per-model call counts. This confirms the pipeline exercised the correct agent roles. Skip this
assertion when running against a real LLM provider.

---

## 1. health-check

**Tier**: 2
**Run command**: `task e2e:llm -- health-check claude` or `task e2e:llm -- health-check openrouter`

### Description

A minimal Go HTTP service receives a plan to add a `/health` endpoint. The scenario exercises the
complete plan-plus-execute pipeline in the simplest possible form: one language (Go), one new
endpoint, one requirement, and a small set of unit tests. Use this scenario to verify the pipeline
is healthy before running heavier scenarios.

### Plan Prompt

```text
Add a /health endpoint to the Go HTTP service. The endpoint should return JSON with:
"status": "ok", "uptime": seconds since server start, "version": Go runtime version.
Include unit tests for the health handler.
```

### Workspace Setup

Create these files in a fresh temp directory before the test run. The directory becomes the
semspec workspace (`--repo` flag or `SEMSPEC_REPO` env var).

**`go.mod`**

```go
module example.com/healthservice

go 1.25
```

**`main.go`**

```go
package main

import (
    "fmt"
    "net/http"
)

func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintln(w, "Hello, World!")
    })
    http.ListenAndServe(":8080", nil)
}
```

**`README.md`**

```markdown
# Health Service

A basic Go HTTP service.
```

**`.semspec/projects/default/project.json`**

```json
{
  "id": "semspec.local.project.default",
  "name": "default",
  "slug": "default"
}
```

The `.semspec/projects/default/project.json` file must exist before plan creation. The plan-api
uses it to associate the plan with a project.

### Stage Sequence

| # | Stage | Timeout | What It Does |
|---|-------|---------|--------------|
| 1 | setup-project | 30s | Write workspace files, verify directory structure |
| 2 | detect-stack | 15s | Assert Go is detected from `go.mod` |
| 3 | init-project | 15s | `POST /plan-api/projects` or verify default project exists |
| 4 | verify-graph-ready | 30s | Poll graph-gateway until workspace is indexed |
| 5 | create-plan | 15s | `POST /plan-api/plans` with the prompt above |
| 6 | wait-for-plan-goal | 120s | Poll plan until `goal` field is non-empty |
| 7 | wait-for-approval | 300s | Poll plan until `status` is `approved` |
| 8 | trigger-execution | 15s | `POST /plan-api/plans/{slug}/execute` |
| 9 | wait-for-scenarios | 60s | Poll until at least 1 scenario has `status != pending` |
| 10 | wait-for-execution | 300s | Poll until `agent.complete.*` count grows beyond baseline |
| 11 | verify-deliverables | 30s | Assert new `.go` and `_test.go` files exist in workspace |

### Key Assertions

- **Stack detection**: `go.mod` presence causes Go to be detected; assert this before plan
  creation to catch workspace setup errors early
- **Plan goal**: the `goal` field must be non-empty before proceeding — an empty goal means the
  planner has not completed its first pass
- **Approval gate**: if plan `status` reaches `escalated` or `error`, fail immediately rather than
  waiting for the full timeout
- **Scenario count**: at least 1 scenario must be generated; 0 scenarios after the timeout means
  the planner did not produce requirements
- **Task dispatch**: `GET /message-logger/entries?subject=agent.task.*` must return at least 1
  entry; absence means the scenario-orchestrator did not fire
- **Agent completion**: `agent.complete.*` entry count must be strictly greater than the baseline
  captured at the end of `trigger-execution`
- **Deliverables**: at least one new `.go` file and at least one `_test.go` file must appear under
  the workspace root (non-fatal warning in real-LLM mode — see Shared Patterns)

### Config Requirements

| Variable | Value |
|----------|-------|
| `E2E_CONFIG` | `e2e-claude.json` or `e2e-openrouter.json` |
| Compose overlay | `docker/compose/e2e-llm.yml` |
| `SEMSPEC_REPO` | Path to the temp workspace |
| `auto_approve` | Must be `false` in test config so the approval gate is meaningful |

The `auto_approve: false` setting is critical. If it is `true`, the plan skips human approval and
the `wait-for-approval` stage never exercises the approval path.

### Non-Obvious Details

- The `/health` endpoint must record server start time at `main()` entry, not at handler
  registration. The LLM sometimes places the `startTime` variable inside the handler function,
  which resets it on every request. Validate that the generated code initialises `startTime`
  outside the handler.
- `runtime.Version()` is the correct Go call for the version string. Plans that produce
  `os.Getenv("GO_VERSION")` or similar are incorrect and will fail validation.
- Unit tests for the health handler require `net/http/httptest`. The LLM occasionally omits this
  import — the build step catches it, but it triggers a builder-retry cycle that adds 60–90 seconds
  to total execution time.

---

## 2. rest-api

**Tier**: 3
**Run command**: `task e2e:llm -- rest-api claude`

### Description

A Go HTTP service receives a plan for a full `/users` CRUD API plus request logging middleware.
This scenario tests multi-requirement planning (3+ requirements), middleware generation, and error
handling conventions. Longer timeouts are required because the plan is more complex and the
execution pipeline dispatches more tasks.

### Plan Prompt

```text
Add a /users REST API to the Go HTTP service with the following:
1. CRUD endpoints: GET /users — list all users (in-memory store), GET /users/{id} — get user by
   ID, POST /users — create a user (name, email required), DELETE /users/{id} — delete a user.
2. Request logging middleware: Log method, path, status code, and duration for every request.
   Apply to all routes.
3. Error handling: Return 404 JSON for missing users, Return 400 JSON for invalid POST body,
   Return 405 for unsupported methods.
Include unit tests for handlers and middleware.
```

### Workspace Setup

**`go.mod`**

```go
module example.com/userservice

go 1.25
```

**`main.go`**

```go
package main

import (
    "fmt"
    "net/http"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintln(w, "User Service")
    })
    http.ListenAndServe(":8080", mux)
}
```

**`README.md`**

```markdown
# User Service

A Go HTTP service with CRUD API.
```

**`.semspec/projects/default/project.json`**

Same content as health-check.

### Stage Sequence

| # | Stage | Timeout | What It Does |
|---|-------|---------|--------------|
| 1 | setup-project | 30s | Write workspace files |
| 2 | detect-stack | 15s | Assert Go detected from `go.mod` |
| 3 | init-project | 15s | Verify default project exists |
| 4 | verify-graph-ready | 30s | Poll graph-gateway until indexed |
| 5 | create-plan | 15s | `POST /plan-api/plans` with the prompt above |
| 6 | wait-for-plan-goal | 180s | Poll until `goal` field is non-empty (extra 60s vs health-check) |
| 7 | wait-for-approval | 600s | Poll until `status` is `approved` |
| 8 | trigger-execution | 15s | `POST /plan-api/plans/{slug}/execute` |
| 9 | wait-for-scenarios | 120s | Poll until at least 3 scenarios are non-pending |
| 10 | wait-for-execution | 600s | Poll until `agent.complete.*` count exceeds baseline |
| 11 | verify-deliverables | 30s | Assert handler and middleware files exist |

### Key Assertions

All assertions from health-check apply, plus:

- **Scenario count**: at least 3 scenarios required (one each for CRUD endpoints, middleware, and
  error handling). Fewer than 3 means the planner collapsed the requirements.
- **Handler files**: at least one `.go` file in the workspace whose path or content contains
  `handler` or `user` (case-insensitive). This confirms the CRUD implementation was generated as a
  separate file rather than inlined into `main.go`.
- **Middleware files**: at least one `.go` file whose path or content contains `middleware`,
  `logging`, or `logger` (case-insensitive). Middleware inlined into handlers satisfies neither the
  structural requirement nor testability.
- **Test coverage**: `_test.go` files must exist for both handler and middleware packages.

### Config Requirements

Same as health-check. The longer timeouts in stages 6–10 are inherent to the scenario complexity —
they are not configurable.

### Non-Obvious Details

- The prompt intentionally omits `PUT /users/{id}` (update). The LLM sometimes adds it
  anyway. This is acceptable — do not fail the test because extra endpoints were generated.
- In-memory store implementations that use a plain `map[string]User` without a mutex will fail
  the validator's concurrency check. The builder typically adds `sync.RWMutex` on its first retry.
  Factor an extra 90–120 seconds into execution time estimates for this retry cycle.
- `http.ServeMux` in Go 1.22+ supports method-based routing (`GET /users`, `DELETE /users/{id}`).
  Earlier patterns using manual `r.Method` switches are also valid. Both pass the validator.
- The `405 Method Not Allowed` requirement is the most commonly missed. Verify the deliverable
  files explicitly handle unsupported methods rather than silently returning `200 OK`.

---

## 3. todo-app

**Tier**: Brownfield
**Run command**: `task e2e:llm -- todo-app [provider]`

### Description

A brownfield Go+Svelte todo application receives a terse plan prompt to add due dates. This
scenario's purpose is to validate that the planner correctly reads and references existing source
files rather than hallucinating a greenfield design. An SOP is injected before planning to test
SOP enforcement. CRUD operations on requirements and scenarios are verified against the plan-api.

### Plan Prompt

```text
add due dates to todos — backend field, API update, UI date picker
```

The prompt is intentionally brief. The planner must infer scope from the existing codebase context
assembled by the context-builder.

### Workspace Setup

The workspace is a full Go+Svelte monorepo (~200 lines total). **Initialize as a git repository
with an initial commit** before the test run — the context-builder's git strategy depends on
commit history.

**`api/go.mod`**

```go
module example.com/todo

go 1.25
```

**`api/main.go`** — HTTP entry point with router setup and CORS headers.

**`api/handlers.go`** — `GET /todos`, `POST /todos`, `DELETE /todos/{id}` handlers using an
in-memory store. Each handler reads/writes a `Todo` struct.

**`api/models.go`** — `Todo` struct: `ID string`, `Title string`, `Done bool`. This is the struct
the plan must extend with a `DueDate` field.

**`ui/src/routes/+page.svelte`** — Svelte 5 page component. Fetches todos from the API, renders
a list, and includes an add-todo form.

**`ui/src/lib/api.ts`** — TypeScript API client. Functions: `fetchTodos()`, `createTodo(title)`,
`deleteTodo(id)`.

**`ui/src/lib/types.ts`** — TypeScript types. `Todo` interface matching the Go struct.

**`ui/package.json`** — Standard SvelteKit package descriptor.

**`ui/svelte.config.js`** — Standard SvelteKit config with `adapter-auto`.

**`README.md`**

```markdown
# Todo App

A simple todo application with a Go backend and Svelte frontend.
```

**`.semspec/projects/default/project.json`** — Same as other scenarios.

**SOP file** — Inject before the `init-project` stage by writing:

```
sources/model-change-sop.md
```

with YAML frontmatter:

```yaml
---
category: sop
scope: all
severity: error
---
```

followed by a brief SOP body describing the model-change procedure (e.g., "When modifying a
shared model struct, update all dependent types, API response shapes, and frontend type
definitions in the same plan.").

The SOP must land in `sources/` so semsource ingests it and it becomes available to the
context-builder's SOP strategy.

**Git initialization**

```bash
git init
git add .
git commit -m "initial commit"
```

### Stage Sequence

| # | Stage | Timeout | What It Does |
|---|-------|---------|--------------|
| 1 | setup-project | 30s | Write workspace files, git init, initial commit |
| 2 | check-not-initialized | 10s | Assert no existing plan for this slug |
| 3 | detect-stack | 30s | Assert both Go (`api/go.mod`) and Svelte (`ui/svelte.config.js`) detected |
| 4 | init-project | 30s | `POST /plan-api/projects` |
| 5 | verify-initialized | 10s | `GET /plan-api/projects/{id}` returns 200 |
| 6 | ingest-sop | 30s | Verify `sources/model-change-sop.md` exists; poll semsource ingest |
| 7 | verify-sop-ingested | 60s | Poll graph-gateway for entity with `source.doc` predicate and `category=sop` |
| 8 | verify-standards-populated | 30s | Poll until `standards.json` contains at least one rule |
| 9 | verify-graph-ready | 30s | Poll graph-gateway until workspace is indexed |
| 10 | create-plan | 30s | `POST /plan-api/plans` with the prompt above |
| 11 | wait-for-plan | 300s | Poll until `status` is `approved` |
| 12 | verify-plan-semantics | 10s | Inline assertions against the plan JSON |
| 13 | approve-plan | 240s | If `auto_approve=false`, call `POST /plans/{slug}/promote`; wait for `ready_for_execution` |
| 14 | verify-requirements | 10s | `GET /plan-api/plans/{slug}/requirements` — assert count > 0 |
| 15 | verify-scenarios | 10s | `GET /plan-api/plans/{slug}/requirements/{id}/scenarios` — assert count > 0 |
| 16 | requirement-crud | 30s | Exercise create/get/update/deprecate/delete on a test requirement |
| 17 | scenario-crud | 30s | Exercise create/get/update/list/delete on a test scenario |
| 18 | capture-trajectory | 30s | `GET /trajectory-api/loops` — capture loop list for report |
| 19 | generate-report | 10s | Emit timing, scope hallucination rate, and requirement/scenario counts |

### Key Assertions

**Semantic plan validation** (stage 12 — inline assertions against plan JSON):

- `goal` mentions "due date" or "dueDate" (case-insensitive)
- `context` or `goal` references at least 3 of: `handlers.go`, `models.go`, `+page.svelte`,
  `api.ts`, `types.ts`
- Plan text references both `api/` and `ui/` directory prefixes, confirming the planner
  understood the monorepo structure
- Plan context mentions "existing" code at least once, confirming brownfield awareness

**Scope hallucination rate** (stage 12):

Count paths mentioned in the plan that do not exist in the workspace. Divide by total mentioned
paths. Assert rate is below 50%. A rate above 50% means the planner ignored the codebase context
and hallucinated a greenfield design.

**SOP enforcement** (stages 6–8):

The SOP must be ingested and appear in the graph before `create-plan`. If `verify-sop-ingested`
times out, the plan will lack SOP context and semantic validation will likely fail. Treat SOP
ingest failure as a blocking error.

**CRUD operations** (stages 16–17):

These stages call the plan-api directly to verify the CRUD endpoints are functional. They do not
test the LLM output — they test the API itself using the plan created in stage 10 as the parent
entity. Run these even if earlier stages show warnings.

### Config Requirements

| Variable | Value |
|----------|-------|
| `E2E_CONFIG` | Provider-specific config (claude, openrouter, etc.) |
| Compose overlay | `docker/compose/e2e-llm.yml` |
| `SEMSPEC_REPO` | Path to the temp workspace |
| `auto_approve` | `false` recommended; stage 13 handles both cases |

### Non-Obvious Details

- The `sources/` directory must exist before semsource starts watching the workspace. Create it as
  part of the workspace setup in stage 1, even if the SOP file itself is written in stage 6.
  Semsource may miss files written to a directory it has not yet seen.
- `standards.json` is written by the plan-coordinator after SOP ingest. It lives at
  `.semspec/standards.json` in the workspace. Its presence is necessary for the plan-reviewer to
  apply SOP rules. If `verify-standards-populated` times out, check whether semsource is reporting
  the SOP entity correctly.
- The brownfield context-builder strategy requires a git history. A workspace without commits
  causes the git gatherer to return empty context, which degrades plan quality significantly.
  Always commit the initial workspace before running this scenario.
- The `todo-app` scenario does **not** run execution (no `trigger-execution` stage). It validates
  planning quality only. This is intentional — the scenario focuses on SOP enforcement and
  brownfield context assembly, not on the TDD pipeline.
- Svelte 5 type detection: the stack detector checks for `$state`, `$derived`, or `$props` in
  `.svelte` files to distinguish Svelte 5 from Svelte 4. The workspace must use Svelte 5 runes in
  the starter `+page.svelte` file, or the model configuration will select the wrong prompt tier.

---

## 4. epic-meshtastic

**Tier**: 4 — Epic
**Run command**: `task e2e:epic -- claude` or `task e2e:epic -- openrouter`

### Description

The most complex scenario in the suite. Three external semsource instances (OSH, Meshtastic, OGC)
are started alongside semspec. Their knowledge graphs are federated into a single queryable graph.
The planner must draw on all three knowledge domains to design a Meshtastic driver for
OpenSensorHub using the Connected Systems API. This scenario validates the full alpha pipeline:
federated knowledge graph assembly, multi-domain planning, execution to deliverables, and plan
rollup review.

### Plan Prompt

```text
Design and implement a Meshtastic driver for OpenSensorHub (OSH). The driver must use the
Connected Systems API to send and receive messages over the Meshtastic mesh network. Deliver
working Java source files, unit tests, and a README with usage examples.
```

### Workspace Setup

A Maven/Java project scaffold. Initialize as a git repository with an initial commit.

**`pom.xml`**

```xml
<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>org.sensorhub</groupId>
  <artifactId>osh-driver-meshtastic</artifactId>
  <version>0.1.0-SNAPSHOT</version>
</project>
```

**`src/main/java/.gitkeep`** — Empty placeholder so the source tree exists.

**`src/test/java/.gitkeep`** — Empty placeholder.

**`README.md`**

```markdown
# OSH Meshtastic Driver

Driver for integrating Meshtastic mesh networking with OpenSensorHub.
```

**Git initialization**

```bash
git init
git add .
git commit -m "initial scaffold"
```

### Infrastructure Requirements

This scenario requires three additional semsource containers. These are started by
`task e2e:epic` via the `docker/compose/e2e-epic.yml` overlay.

| Service | Config File | Knowledge Domain |
|---------|------------|-----------------|
| `semsource-osh` | `configs/semsource/osh.json` | OpenSensorHub API, Connected Systems |
| `semsource-meshtastic` | `configs/semsource/meshtastic.json` | Meshtastic protocol, mesh networking |
| `semsource-ogc` | `configs/semsource/ogc.json` | OGC standards, Connected Systems API spec |

All three semsource instances must reach `phase: ready` before planning begins. Poll
`GET /source-manifest/status` on each instance. Allow up to 10 minutes total — indexing a large
knowledge domain can take several minutes on first run.

The `GRAPH_SOURCES` environment variable must be set on the semspec container to point at all
three instances:

```json
[
  {"name": "osh",          "url": "http://semsource-osh:8080",        "type": "semsource"},
  {"name": "meshtastic",   "url": "http://semsource-meshtastic:8080", "type": "semsource"},
  {"name": "ogc",          "url": "http://semsource-ogc:8080",        "type": "semsource"}
]
```

### Stage Sequence

| # | Stage | Timeout | What It Does |
|---|-------|---------|--------------|
| 1 | verify-service-health | 30s | Assert NATS, semspec HTTP, and all 3 semsource instances respond |
| 2 | verify-graph-manifest | 60s | Poll `GET /graph-gateway/manifest` until total entity count > 100 |
| 3 | setup-workspace | 30s | Write workspace files, git init, initial commit |
| 4 | init-project | 15s | `POST /plan-api/projects` or verify default project |
| 5 | create-plan | 15s | `POST /plan-api/plans` with the prompt above |
| 6 | wait-for-plan-goal | 180s | Poll until plan `goal` is non-empty |
| 7 | wait-for-approval | 600s | Poll until `status` is `approved` |
| 8 | trigger-execution | 15s | `POST /plan-api/plans/{slug}/execute` |
| 9 | wait-for-scenarios | 120s | Poll until at least 3 scenarios are non-pending |
| 10 | wait-for-execution | 900s | Poll until `status` is `reviewing_rollup` or `complete` |
| 11 | wait-for-rollup | 120s | If status is `reviewing_rollup`, poll until `status` is `complete` |
| 12 | verify-deliverables | 30s | Assert Java source files and README content |

### Key Assertions

- **Federated graph**: `verify-graph-manifest` must see more than 100 entities in the combined
  graph before planning starts. Fewer than 100 means at least one semsource instance has not
  finished indexing its knowledge domain.
- **Java detection**: stack detector must identify Java from `pom.xml`. If detection fails,
  assert that the plan-coordinator added Java to the detected stacks manually before dispatching
  the planner.
- **Plan approval**: if `status` reaches `rejected` or `escalated` the test fails immediately.
  A rejected plan at this tier indicates the planner could not reconcile the three knowledge
  domains — investigate the plan-reviewer feedback in the plan JSON's `reviews` array.
- **Scenario count**: at least 3 scenarios required (driver implementation, unit tests, README /
  integration documentation is a reasonable minimum decomposition).
- **Rollup review**: the plan must pass through `reviewing_rollup` status before reaching
  `complete`. If the status jumps directly from `implementing` to `complete`, the rollup reviewer
  was bypassed — this is a pipeline bug, not a test pass.
- **Java deliverables**: at least one `.java` file must exist under `src/main/java/`. An empty
  directory means the execution pipeline ran but no code was generated.
- **README content**: `README.md` must be at least 200 bytes after execution. The original
  placeholder is 71 bytes — a larger file confirms the agent updated it.

### Config Requirements

| Variable | Value |
|----------|-------|
| `E2E_CONFIG` | `e2e-claude.json` or `e2e-openrouter.json` |
| Compose overlay | `docker/compose/e2e-epic.yml` |
| `GRAPH_SOURCES` | JSON array pointing at all 3 semsource instances (see above) |
| `SEMSPEC_REPO` | Path to the temp workspace |
| `auto_approve` | `false` — rollup review path requires human-like approval gating |

### Non-Obvious Details

- Semsource indexing order matters. The OGC instance must finish before the planner runs because
  the Connected Systems API spec is the primary integration contract. If `ogc` is still in
  `phase: indexing` when the plan is created, the planner will produce a weaker design that may
  not reference the correct API endpoints. `verify-graph-manifest` gates on entity count, but a
  count check alone does not guarantee the OGC entities are present. Consider adding a predicate
  check: poll until at least one entity with `source.doc.domain=ogc` exists.
- The 900-second `wait-for-execution` timeout is not a mistake. Java compilation, test execution,
  and multi-file generation across 3+ scenarios takes significantly longer than Go scenarios. A
  real LLM provider with a loaded inference cluster can take 12–15 minutes for this scenario.
- `reviewing_rollup` is a distinct plan status introduced in the reactive execution model
  (ADR-025). If the plan-api version predates this status, the `wait-for-execution` stage will
  never exit via the rollup path — it will poll until `complete` directly. Check the plan-api
  version before asserting rollup behavior.
- The epic scenario does not use a mock LLM. There are no fixture files for it. Running
  `task e2e:mock -- epic-meshtastic` will fail — use `task e2e:epic` exclusively.
- Token costs: this scenario typically consumes 150–300k tokens depending on provider and model.
  Do not run it in cost-sensitive CI without a spending cap.
