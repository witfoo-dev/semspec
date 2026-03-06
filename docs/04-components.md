# Semspec Components

> **When to use components vs workflows?** See [Architecture: Components vs Workflows](03-architecture.md#components-vs-workflows)
> for the decision framework.

---

## Indexing

### ast-indexer

**Purpose**: Extracts code entities from Go source files and publishes them to the graph.

**Location**: `processor/ast-indexer/`

#### Configuration

```json
{
  "repo_path": ".",
  "org": "semspec",
  "project": "myproject",
  "watch_enabled": true,
  "index_interval": "5m"
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `repo_path` | string | `.` | Repository path to index |
| `org` | string | required | Organization for entity IDs |
| `project` | string | required | Project name for entity IDs |
| `watch_enabled` | bool | `true` | Enable file watcher for real-time updates |
| `index_interval` | string | `5m` | Periodic full reindex interval |

#### Behavior

1. **Startup**: Performs full index of all `.go` files in `repo_path`
2. **Watch mode**: If enabled, watches for file changes via fsnotify
3. **Periodic reindex**: If interval set, performs full reindex on schedule
4. **Output**: Publishes entities to `graph.ingest.entity` subject

#### Entity Types Extracted

| Type | Description |
|------|-------------|
| `file` | Go source files |
| `function` | Standalone functions |
| `method` | Methods with receivers |
| `struct` | Struct types |
| `interface` | Interface types |
| `const` | Constants |
| `var` | Variables |

#### Entity ID Format

```
{org}.semspec.code.{type}.{project}.{instance}
```

Example: `acme.semspec.code.function.myproject.cmd-main-go-main`

#### Dependencies

Uses `processor/ast/` package:

- `parser.go` - Go AST parsing
- `entities.go` - CodeEntity type and serialization
- `watcher.go` - File system watcher with debouncing
- `predicates.go` - Vocabulary predicate constants

---

## Planning

### plan-coordinator

**Purpose**: Orchestrates parallel planners with focus area decomposition. This is the primary entry
point for `/plan` commands.

**Location**: `processor/plan-coordinator/`

#### Configuration

```json
{
  "stream_name": "WORKFLOWS",
  "consumer_name": "plan-coordinator",
  "trigger_subject": "workflow.trigger.plan-coordinator",
  "sessions_bucket": "PLAN_SESSIONS",
  "max_concurrent_planners": 3,
  "planner_timeout": "120s",
  "context_timeout": "30s",
  "default_capability": "planning"
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `stream_name` | string | `WORKFLOWS` | JetStream stream for workflow triggers |
| `consumer_name` | string | `plan-coordinator` | Durable consumer name |
| `trigger_subject` | string | `workflow.trigger.plan-coordinator` | Subject for coordinator triggers |
| `sessions_bucket` | string | `PLAN_SESSIONS` | KV bucket for plan sessions |
| `max_concurrent_planners` | int | `3` | Maximum concurrent planners (1–3) |
| `planner_timeout` | string | `120s` | Timeout for each planner to complete |
| `context_timeout` | string | `30s` | Timeout for context building |
| `default_capability` | string | `planning` | Default model capability for coordination |
| `context_subject_prefix` | string | `context.build` | Subject prefix for context build requests |
| `context_response_bucket` | string | `CONTEXT_RESPONSES` | KV bucket for context responses |

#### Behavior

1. **Determines focus areas**: Calls context-builder for project context, then calls the LLM to
   decompose the plan title into 1–3 distinct focus areas.
2. **Runs planners in parallel**: Each focus area gets its own goroutine that calls the LLM with a
   focused system prompt and its portion of the project context.
3. **Synthesizes results**: If multiple planners ran, calls the LLM to merge their outputs into a
   single coherent plan; falls back to simple concatenation on merge failure.
4. **Saves plan**: Writes the final Goal/Context/Scope to `.semspec/plans/<slug>/plan.json`.

> **Design note**: plan-coordinator calls the LLM directly in goroutines for each focus area. It
> does **not** delegate to the `planner` component.

#### NATS Subjects

| Subject | Transport | Direction | Description |
|---------|-----------|-----------|-------------|
| `workflow.trigger.plan-coordinator` | JetStream (WORKFLOWS) | Input | Plan coordinator triggers |
| `workflow.result.plan-coordinator.<slug>` | Core NATS | Output | Completion notifications |

---

### planner

**Purpose**: Generates Goal/Context/Scope for plans using LLM. This is the simple single-planner
path; `plan-coordinator` is the primary orchestrator for most `/plan` commands.

**Location**: `processor/planner/`

#### Configuration

```json
{
  "stream_name": "WORKFLOWS",
  "consumer_name": "planner",
  "trigger_subject": "workflow.trigger.planner",
  "default_capability": "planning"
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `stream_name` | string | `WORKFLOWS` | JetStream stream name |
| `consumer_name` | string | `planner` | Durable consumer name |
| `trigger_subject` | string | `workflow.trigger.planner` | Subject to consume triggers from |
| `default_capability` | string | `planning` | Default model capability |

#### Behavior

1. **Subscribes**: Consumes from `workflow.trigger.planner` on the WORKFLOWS stream
2. **Loads Plan**: Reads existing plan from `.semspec/plans/{slug}/plan.json`
3. **Generates Content**: Calls LLM with planner system prompt
4. **Parses Response**: Extracts JSON for Goal/Context/Scope from LLM output
5. **Saves Plan**: Updates `plan.json` with generated content
6. **Publishes Result**: Sends completion to `workflow.result.planner.{slug}`

#### LLM Response Format

The component expects the LLM to return JSON, optionally wrapped in markdown code fences:

```json
{
  "goal": "Clear statement of what the plan accomplishes",
  "context": "Current state and relevant background",
  "scope": {
    "include": ["files/areas to modify"],
    "exclude": ["files/areas to avoid"],
    "do_not_touch": ["critical files to preserve"]
  }
}
```

#### NATS Subjects

| Subject | Transport | Direction | Description |
|---------|-----------|-----------|-------------|
| `workflow.trigger.planner` | JetStream (WORKFLOWS) | Input | Plan generation triggers |
| `workflow.result.planner.<slug>` | Core NATS | Output | Completion notifications |

---

### plan-reviewer

**Purpose**: SOP-aware plan review before approval. Validates plans against project SOPs and flags
scope hallucination.

**Location**: `processor/plan-reviewer/`

#### Configuration

```json
{
  "stream_name": "WORKFLOWS",
  "consumer_name": "plan-reviewer",
  "trigger_subject": "workflow.trigger.plan-reviewer",
  "result_subject_prefix": "workflow.result.plan-reviewer",
  "graph_gateway_url": "http://localhost:8082",
  "context_token_budget": 4000,
  "default_capability": "reviewing",
  "llm_timeout": "120s"
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `stream_name` | string | `WORKFLOWS` | JetStream stream for workflow triggers |
| `consumer_name` | string | `plan-reviewer` | Durable consumer name |
| `trigger_subject` | string | `workflow.trigger.plan-reviewer` | Subject for plan review triggers |
| `result_subject_prefix` | string | `workflow.result.plan-reviewer` | Subject prefix for review results |
| `graph_gateway_url` | string | `http://localhost:8082` | Graph gateway URL for context queries |
| `context_token_budget` | int | `4000` | Token budget for additional graph context |
| `default_capability` | string | `reviewing` | Default model capability for plan review |
| `llm_timeout` | string | `120s` | Timeout for LLM calls |
| `context_build_timeout` | string | `30s` | Timeout for context building requests |

#### Trigger Payload

```json
{
  "request_id": "...",
  "slug": "add-auth-refresh",
  "project_id": "myproject",
  "plan_content": "{ ... }",
  "scope_patterns": ["processor/", "workflow/"],
  "sop_context": "..."
}
```

#### Behavior

1. **Enriches context**: Queries graph for related plans and code patterns.
2. **Auto-approves**: If no SOP context and no graph context are available, returns `approved`
   immediately.
3. **Validates**: Calls LLM (temperature 0.3) to verify the plan against each SOP requirement.
4. **Checks scope**: Compares scope paths against the actual project file tree to detect
   hallucinated paths.
5. **Returns verdict**: `approved` or `needs_changes` with a `findings` array.

Each finding has the shape:

```json
{
  "sop_id": "SOP-001",
  "sop_title": "Testing Standards",
  "severity": "error",
  "status": "violation",
  "issue": "No test tasks included",
  "suggestion": "Add unit test tasks for new functions",
  "evidence": "scope includes processor/ but tasks.json has no test entries"
}
```

#### NATS Subjects

| Subject | Transport | Direction | Description |
|---------|-----------|-----------|-------------|
| `workflow.trigger.plan-reviewer` | JetStream (WORKFLOWS) | Input | Plan review triggers |
| `workflow.result.plan-reviewer.<slug>` | JetStream | Output | Review results (ordering guarantee) |

---

## Context

### context-builder

**Purpose**: Assembles curated LLM context from the knowledge graph, filesystem, and SOPs. Shared
service used by `plan-coordinator`, `planner`, `task-generator`, and `task-dispatcher`.

**Location**: `processor/context-builder/`

#### Configuration

```json
{
  "stream_name": "AGENT",
  "consumer_name": "context-builder",
  "input_subject_pattern": "context.build.>",
  "output_subject_prefix": "context.built",
  "default_token_budget": 32000,
  "headroom_tokens": 6400,
  "graph_gateway_url": "http://localhost:8082",
  "default_capability": "reviewing",
  "sop_entity_prefix": "source.doc",
  "response_bucket_name": "CONTEXT_RESPONSES",
  "graph_readiness_budget": "15s"
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `stream_name` | string | `AGENT` | JetStream stream for context requests |
| `consumer_name` | string | `context-builder` | Durable consumer name |
| `input_subject_pattern` | string | `context.build.>` | Subject pattern for context build requests |
| `output_subject_prefix` | string | `context.built` | Subject prefix for context responses |
| `default_token_budget` | int | `32000` | Default token budget when no model specified |
| `headroom_tokens` | int | `6400` | Safety buffer tokens reserved for model response |
| `graph_gateway_url` | string | `http://localhost:8082` | Graph gateway URL for entity queries |
| `default_capability` | string | `reviewing` | Default model capability |
| `sop_entity_prefix` | string | `source.doc` | Predicate prefix for finding SOP entities |
| `response_bucket_name` | string | `CONTEXT_RESPONSES` | KV bucket for context responses |
| `graph_readiness_budget` | string | `15s` | Max time to wait for graph readiness on first request |
| `allow_blocking` | bool | `true` | Enable blocking to wait for Q&A answers |
| `blocking_timeout_seconds` | int | `300` | Max seconds to wait for Q&A answers |

#### Behavior

1. **Graph readiness probe**: On the first request, probes the full NATS request-reply path to the
   graph pipeline. Result is cached via `atomic.Bool` + `sync.Once` to avoid repeated probes.
2. **Strategy selection**: Chooses one of six assembly strategies based on the task type field of
   the incoming request.
3. **Prioritized assembly**: Executes ordered steps (file tree, summaries, docs, SOPs, code
   patterns) until the token budget is consumed.
4. **Stores response**: Writes the assembled context to the `CONTEXT_RESPONSES` KV bucket so
   callers can watch reactively.

#### Strategies

Each strategy prioritizes a different content mix:

| Strategy | Priority Order |
|----------|----------------|
| `planning` | file tree → codebase summary → arch docs → specs → code patterns → requested files → SOPs |
| `plan-review` | SOPs (all-or-nothing) → plan content → file tree → arch docs |
| `implementation` | spec entity → target files → related patterns → conventions |
| `review` | SOPs (all-or-nothing) → changed file contents → test files → conventions |
| `exploration` | codebase summary → matching entities → related docs → requested files |
| `question` | matching entities → source docs → codebase summary → relevant docs |

> **Graph readiness**: Uses `WaitForReady()` with exponential backoff (250ms → 2s, capped by
> `graph_readiness_budget`). When the graph is not ready, strategies skip graph steps cleanly.

#### NATS Subjects

| Subject | Transport | Direction | Description |
|---------|-----------|-----------|-------------|
| `context.build.>` | JetStream (AGENT) | Input | Context build requests |
| `context.built.<request_id>` | Core NATS | Output | Context responses |
| CONTEXT_RESPONSES KV | JetStream KV | Output | Stored responses for reactive watchers |

---

## Sources

### source-ingester

**Purpose**: Ingests documents (SOPs, specs, references) with YAML frontmatter parsing and publishes
to the knowledge graph.

**Location**: `processor/source-ingester/`

#### Configuration

```json
{
  "stream_name": "SOURCES",
  "consumer_name": "source-ingester",
  "sources_dir": ".semspec/sources/docs",
  "analysis_timeout": "30s",
  "chunk_config": {
    "target_tokens": 1000,
    "max_tokens": 1500,
    "min_tokens": 200
  }
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `stream_name` | string | `SOURCES` | JetStream stream name |
| `consumer_name` | string | `source-ingester` | Durable consumer name |
| `sources_dir` | string | `.semspec/sources/docs` | Base directory for document sources |
| `analysis_timeout` | string | `30s` | LLM analysis timeout |
| `chunk_config.target_tokens` | int | `1000` | Ideal chunk size in tokens |
| `chunk_config.max_tokens` | int | `1500` | Maximum chunk size in tokens |
| `chunk_config.min_tokens` | int | `200` | Minimum chunk size (smaller chunks are merged) |

#### Behavior

1. **Reads file**: Retrieves document from disk at the path specified in the ingest message.
2. **Parses frontmatter**: If the file has a YAML frontmatter block with a `category` field,
   extracts metadata directly (fast path — no LLM call).
3. **LLM fallback**: If no frontmatter is present, calls the LLM to classify and summarize the
   document.
4. **Chunks document**: Splits content into token-bounded chunks for graph storage.
5. **Builds graph entities**: Applies `source.doc.*` vocabulary predicates.
6. **Publishes to graph**: Publishes chunk entities first, then the parent document entity to
   `graph.ingest.entity`.

#### Key Vocabularies

- `source.doc.category` — Document category (SOP, spec, reference, etc.)
- `source.doc.scope` — Applicable scope or subsystem
- `source.doc.severity` — Severity level for SOP findings
- `source.doc.applies_to` — Target components or paths
- `source.doc.requirements` — Extracted requirements text
- `source.doc.content` — Full document content
- `source.meta.name` — Human-readable document name
- `source.meta.status` — Ingestion status

#### NATS Subjects

| Subject | Transport | Direction | Description |
|---------|-----------|-----------|-------------|
| `source.ingest.>` | JetStream (SOURCES) | Input | Document ingestion requests |
| `graph.ingest.entity` | JetStream (GRAPH) | Output | Entity state updates |

---

## Execution

### task-generator

**Purpose**: Runs the multi-step planning pipeline (Requirements → Scenarios → Phases → Tasks)
from an approved plan when `reactive_mode=false`. When `reactive_mode=true` (default), it skips
task generation entirely and advances the plan status to `ready_for_execution` so the
scenario-orchestrator can decompose work at runtime.

See [Workflow System: Planning Pipeline](05-workflow-system.md#planning-pipeline-adr-024) for the
full pipeline description.

**Location**: `processor/task-generator/`

#### Configuration

```json
{
  "stream_name": "WORKFLOWS",
  "consumer_name": "task-generator",
  "trigger_subject": "workflow.trigger.task-generator",
  "default_capability": "planning",
  "pipeline_mode": "pipeline",
  "reactive_mode": true
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `stream_name` | string | `WORKFLOWS` | JetStream stream name |
| `consumer_name` | string | `task-generator` | Durable consumer name |
| `trigger_subject` | string | `workflow.trigger.task-generator` | Subject to consume triggers from |
| `default_capability` | string | `planning` | Default model capability |
| `pipeline_mode` | string | `pipeline` | `pipeline` (default) or `single_shot` |
| `reactive_mode` | bool | `true` | Skip task generation; advance plan to `ready_for_execution` |

#### Behavior

**Reactive mode** (default, `reactive_mode=true`) — defers task decomposition to execution time:

1. After `scenarios_generated`, advances plan status directly to `ready_for_execution`
1. Does **not** generate Phases or Tasks upfront; does **not** write `tasks.json`
1. Publishes result to `workflow.result.tasks.{slug}` to signal completion
1. The `scenario-orchestrator` then decomposes each Scenario into a TaskDAG via LLM
   at execution time, when the agent can inspect the live codebase

**Pipeline mode** (`reactive_mode=false`) — four focused LLM calls:

1. **Subscribes**: Consumes from `workflow.trigger.task-generator` on the WORKFLOWS stream
1. **Loads Plan**: Reads plan from `.semspec/plans/{slug}/plan.json`
1. **Generates Requirements**: LLM call with requirement-focused prompt; publishes
   `requirement.created` events; advances plan status to `requirements_generated`
1. **Generates Scenarios**: LLM call per Requirement producing Given/When/Then triples; publishes
   `scenario.created` events; advances plan status to `scenarios_generated`
1. **Generates Phases**: LLM call for scheduling containers (unchanged from pre-ADR-024)
1. **Generates Tasks**: LLM call using Scenarios as input; tasks carry `ScenarioIDs` (many-to-many)
   instead of embedded `AcceptanceCriteria`
1. **Saves Tasks**: Writes to `.semspec/plans/{slug}/tasks.json`
1. **Publishes Result**: Sends completion to `workflow.result.tasks.{slug}`

**Single-shot mode** (`pipeline_mode=single_shot`) — one LLM call producing all tasks directly.
Use when pipeline latency is unacceptable (e.g., local development with small models).

#### Task JSON Format

Tasks produced by the pipeline reference Scenarios by ID rather than embedding acceptance criteria:

```json
{
  "tasks": [
    {
      "id": "1",
      "title": "Task title",
      "description": "What needs to be done",
      "scenarioIDs": ["scenario.add-auth.1.1", "scenario.add-auth.1.2"],
      "dependencies": []
    }
  ]
}
```

#### NATS Subjects

| Subject | Transport | Direction | Description |
|---------|-----------|-----------|-------------|
| `workflow.trigger.task-generator` | JetStream (WORKFLOWS) | Input | Task generation triggers |
| `requirement.created` | JetStream (WORKFLOWS) | Output | New requirement published |
| `scenario.created` | JetStream (WORKFLOWS) | Output | New scenario published |
| `workflow.result.tasks.<slug>` | Core NATS | Output | Completion notifications |

---

### task-dispatcher

**Purpose**: Dependency-aware task dispatch with parallel context building. Reads `tasks.json` and
dispatches each task to an agentic development loop.

**Location**: `processor/task-dispatcher/`

#### Configuration

```json
{
  "stream_name": "WORKFLOWS",
  "consumer_name": "task-dispatcher",
  "trigger_subject": "workflow.trigger.task-dispatcher",
  "max_concurrent": 3,
  "context_timeout": "30s",
  "execution_timeout": "300s",
  "context_subject_prefix": "context.build",
  "context_response_bucket": "CONTEXT_RESPONSES",
  "agent_task_subject": "agent.task.development"
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `stream_name` | string | `WORKFLOWS` | JetStream stream for workflow triggers |
| `consumer_name` | string | `task-dispatcher` | Durable consumer name |
| `trigger_subject` | string | `workflow.trigger.task-dispatcher` | Subject for batch task triggers |
| `max_concurrent` | int | `3` | Maximum parallel task executions (1–10) |
| `context_timeout` | string | `30s` | Timeout for context building per task |
| `execution_timeout` | string | `300s` | Timeout for task execution |
| `context_subject_prefix` | string | `context.build` | Subject prefix for context build requests |
| `context_response_bucket` | string | `CONTEXT_RESPONSES` | KV bucket for context responses |
| `agent_task_subject` | string | `agent.task.development` | Subject for publishing agent tasks |

#### Behavior (3 Phases)

1. **Parallel context building**: Fires ALL context build requests simultaneously for every task
   in the batch — no concurrency limit at this phase.
2. **Dependency-aware dispatch**: Builds an in-memory dependency graph; dispatches tasks as their
   dependencies complete. A semaphore enforces `max_concurrent` during execution.
3. **Individual dispatch**: Wraps each task as an `agentic.TaskMessage` and publishes to
   `agent.task.development` via JetStream (ordering guarantee ensures dependent tasks see their
   predecessors' dispatch messages before they are dispatched themselves).

#### NATS Subjects

| Subject | Transport | Direction | Description |
|---------|-----------|-----------|-------------|
| `workflow.trigger.task-dispatcher` | JetStream (WORKFLOWS) | Input | Batch dispatch triggers |
| `context.build.implementation` | Core NATS | Output | Context build requests |
| `agent.task.development` | JetStream | Output | Agent task messages |
| `workflow.result.task-dispatcher.<slug>` | Core NATS | Output | Batch completion notifications |

---

### scenario-orchestrator

**Purpose**: Entry point for reactive execution (ADR-025). Receives an orchestration trigger for
a plan, and fires a `scenario-execution-loop` workflow for each pending or dirty Scenario. Only
active when `reactive_mode=true` on `task-generator`.

**Location**: `processor/scenario-orchestrator/`

#### Configuration

```json
{
  "stream_name": "WORKFLOW",
  "consumer_name": "scenario-orchestrator",
  "trigger_subject": "scenario.orchestrate.*",
  "workflow_trigger_subject": "workflow.trigger.scenario-execution-loop",
  "execution_timeout": "120s",
  "max_concurrent": 5
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `stream_name` | string | `WORKFLOW` | JetStream stream for orchestration triggers |
| `consumer_name` | string | `scenario-orchestrator` | Durable consumer name |
| `trigger_subject` | string | `scenario.orchestrate.*` | Pattern for per-plan triggers |
| `workflow_trigger_subject` | string | `workflow.trigger.scenario-execution-loop` | Subject for per-scenario triggers |
| `execution_timeout` | string | `120s` | Maximum time for a single orchestration cycle |
| `max_concurrent` | int | `5` | Maximum parallel scenario executions triggered per cycle (1–20) |

#### Trigger Payload

```json
{
  "plan_slug": "add-user-authentication",
  "scenarios": [
    {
      "scenario_id": "scenario.add-user-authentication.1.1",
      "prompt": "Given the user is logged out ...",
      "role": "developer",
      "model": "qwen"
    }
  ],
  "trace_id": "abc123"
}
```

#### Behavior

1. **Receives trigger**: Consumes `OrchestratorTrigger` from `scenario.orchestrate.<planSlug>`
1. **Dispatches concurrently**: Fires one `ScenarioExecutionTriggerPayload` per Scenario, bounded
   by `max_concurrent`
1. **ACKs on success**: NAKs on any dispatch failure (message will be redelivered, max 3 attempts)

The orchestrator does not track execution results. Once triggers are dispatched it is done.
The `scenario-execution-loop` and `dag-execution-loop` reactive workflows handle the rest.

#### NATS Subjects

| Subject | Transport | Direction | Description |
|---------|-----------|-----------|-------------|
| `scenario.orchestrate.*` | JetStream (WORKFLOW) | Input | Per-plan orchestration triggers |
| `workflow.trigger.scenario-execution-loop` | JetStream (WORKFLOW) | Output | Per-scenario execution triggers |

---

## Support

### trajectory-api

**Purpose**: Provides trajectory and LLM call query endpoints for debugging and analysis.

**Location**: `processor/trajectory-api/`

#### Configuration

```json
{
  "llm_calls_bucket": "LLM_CALLS",
  "tool_calls_bucket": "TOOL_CALLS",
  "loops_bucket": "AGENT_LOOPS"
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `llm_calls_bucket` | string | `LLM_CALLS` | KV bucket for LLM call records |
| `tool_calls_bucket` | string | `TOOL_CALLS` | KV bucket for tool call records |
| `loops_bucket` | string | `AGENT_LOOPS` | KV bucket for agent loop state |

#### Behavior

Exposes HTTP endpoints for querying LLM call history and agent loop trajectories. Buckets are
accessed lazily — if a bucket does not exist at startup, the component retries on the first query.
Used by E2E tests to capture trajectory data for correctness verification.

No NATS subjects are consumed or published directly; all access is via JetStream KV.

---

### workflow-api

**Purpose**: Provides workflow execution query endpoints.

**Location**: `processor/workflow-api/`

#### Configuration

```json
{
  "execution_bucket_name": "WORKFLOW_EXECUTIONS"
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `execution_bucket_name` | string | `WORKFLOW_EXECUTIONS` | KV bucket for workflow executions |

#### Behavior

Exposes HTTP endpoints for querying workflow execution state. Also registers Q&A HTTP endpoints via
`workflow.QuestionHTTPHandler` when the question store is available. The execution bucket is
accessed lazily on the first query if it does not exist at startup.

No NATS subjects are consumed or published directly; all access is via JetStream KV.

---

### rdf-export

**Purpose**: Streaming output component that subscribes to graph entity ingestion messages and
serializes them to RDF formats.

**Location**: `processor/rdf-export/`

#### Configuration

```json
{
  "format": "turtle",
  "profile": "minimal",
  "base_iri": "https://semspec.dev"
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `format` | string | `turtle` | RDF format: `turtle`, `ntriples`, `jsonld` |
| `profile` | string | `minimal` | Ontology profile: `minimal`, `bfo`, `cco` |
| `base_iri` | string | `https://semspec.dev` | Base IRI for entity URIs |

#### Profiles

| Profile | Description |
|---------|-------------|
| `minimal` | PROV-O only — basic provenance |
| `bfo` | Adds BFO (Basic Formal Ontology) types |
| `cco` | Adds CCO (Common Core Ontologies) types |

#### Behavior

1. **Subscribes**: Consumes from `graph.ingest.entity`
2. **Infers Types**: Adds `rdf:type` triples based on entity ID pattern
3. **Serializes**: Converts triples to the requested RDF format
4. **Publishes**: Outputs to `graph.export.rdf`

#### NATS Subjects

| Subject | Transport | Direction | Description |
|---------|-----------|-----------|-------------|
| `graph.ingest.entity` | JetStream | Input | Entity ingest messages |
| `graph.export.rdf` | Core NATS | Output | Serialized RDF output |

#### Entity Type Inference

| Pattern | RDF Type |
|---------|----------|
| `*.code.function.*` | `semspec:Function` |
| `*.code.struct.*` | `semspec:Struct` |
| `*.plan.*` | `semspec:Plan` |

---

### workflow-validator

**Purpose**: Request/reply service for validating workflow documents against their type requirements.
Ensures plans and tasks meet content requirements before workflow progression.

**Location**: `processor/workflow-validator/`

#### Configuration

```json
{
  "base_dir": ".",
  "timeout_secs": 30
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `base_dir` | string | `SEMSPEC_REPO_PATH` or cwd | Base directory for document paths |
| `timeout_secs` | int | `30` | Request timeout in seconds |

#### Request Format

```json
{
  "slug": "add-auth-refresh",
  "document": "plan",
  "content": "...",
  "path": ".semspec/plans/add-auth-refresh/plan.json"
}
```

Either `content` or `path` must be provided.

#### Response Format

```json
{
  "valid": true,
  "document": "plan",
  "errors": [],
  "warnings": ["Consider adding acceptance criteria"]
}
```

#### Behavior

1. **Receives Request**: Via NATS request/reply on `workflow.validate.*`
2. **Resolves Content**: From `content` field or reads from `path`
3. **Validates Structure**: Checks document against type-specific requirements
4. **Returns Result**: Synchronous response with validation status

#### NATS Subjects

| Subject | Transport | Direction | Description |
|---------|-----------|-----------|-------------|
| `workflow.validate.*` | Core NATS | Input | Validation requests (wildcard for document type) |
| `workflow.validation.events` | Core NATS | Output | Optional validation event notifications |

#### Security

- **Path validation**: Document paths validated to stay within `base_dir`
- **Path traversal protection**: Blocks attempts to read outside the repository

---

### workflow-documents

**Purpose**: Output component that subscribes to workflow document messages and writes them as files
to the `.semspec/plans/{slug}/` directory.

**Location**: `output/workflow-documents/`

#### Configuration

```json
{
  "base_dir": ".",
  "ports": {
    "inputs": [{
      "name": "documents_in",
      "type": "jetstream",
      "subject": "output.workflow.documents",
      "stream_name": "WORKFLOWS"
    }],
    "outputs": [{
      "name": "documents_written",
      "type": "nats",
      "subject": "workflow.documents.written"
    }]
  }
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `base_dir` | string | `SEMSPEC_REPO_PATH` or cwd | Base directory for document output |
| `ports` | PortConfig | (see above) | Input/output port configuration |

#### Behavior

1. **Consumes Messages**: From `output.workflow.documents` JetStream subject
2. **Transforms Content**: Converts JSON content to the target format based on document type
3. **Writes File**: Creates `.semspec/plans/{slug}/{document}.json`
4. **Publishes Notification**: Sends `workflow.documents.written` event

#### Document Types

| Type | Output File | Content |
|------|-------------|---------|
| `plan` | `plan.json` | Goal/context/scope |
| `tasks` | `tasks.json` | BDD task list with acceptance criteria |

#### NATS Subjects

| Subject | Transport | Direction | Description |
|---------|-----------|-----------|-------------|
| `output.workflow.documents` | JetStream (WORKFLOWS) | Input | Document output messages |
| `workflow.documents.written` | Core NATS | Output | File written notifications |

#### File Structure

```
.semspec/
└── plans/
    └── {slug}/
        ├── plan.json
        ├── metadata.json
        └── tasks.json
```

---

### question-answerer

**Purpose**: Answers questions using LLM agents based on topic and capability routing. Part of the
knowledge gap resolution protocol.

**Location**: `processor/question-answerer/`

#### Configuration

```json
{
  "stream_name": "AGENT",
  "consumer_name": "question-answerer",
  "task_subject": "agent.task.question-answerer",
  "default_capability": "reviewing"
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `stream_name` | string | `AGENT` | JetStream stream name |
| `consumer_name` | string | `question-answerer` | Durable consumer name |
| `task_subject` | string | `agent.task.question-answerer` | Subject to consume tasks from |
| `default_capability` | string | `reviewing` | Default model capability |

#### Behavior

1. **Consumes Tasks**: Listens on `agent.task.question-answerer` for question-answering tasks
2. **Resolves Model**: Uses capability-based model selection (planning, reviewing, coding, etc.)
3. **Generates Answer**: Calls LLM with question context and topic
4. **Publishes Answer**: Sends answer to `question.answer.<id>`
5. **Updates Store**: Marks question as answered in the QUESTIONS KV bucket

#### NATS Subjects

| Subject | Transport | Direction | Description |
|---------|-----------|-----------|-------------|
| `agent.task.question-answerer` | JetStream (AGENT) | Input | Question-answering tasks from router |
| `question.answer.<id>` | JetStream | Output | Answer payloads |

#### Dependencies

- `workflow/answerer/` — Task types and routing
- `workflow/question.go` — Question store
- `model/` — Capability-based model selection

---

### question-timeout

**Purpose**: Monitors question SLAs and triggers escalation when questions are not answered in time.
Disabled by default — enable by adding an instance to `configs/semspec.json`.

**Location**: `processor/question-timeout/`

#### Configuration

```json
{
  "check_interval": "1m",
  "default_sla": "24h",
  "answerer_config_path": "configs/answerers.yaml"
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `check_interval` | duration | `1m` | How often to check for timed-out questions |
| `default_sla` | duration | `24h` | Default SLA when not specified in route config |
| `answerer_config_path` | string | (auto-detected) | Path to `answerers.yaml` |

#### Behavior

1. **Periodic Check**: Runs on `check_interval` to find overdue questions
2. **SLA Evaluation**: Compares question age against the route SLA (or default)
3. **Timeout Events**: Publishes `question.timeout.<id>` when SLA is exceeded
4. **Escalation**: If `escalate_to` is configured, reassigns question and publishes
   `question.escalate.<id>`
5. **Notifications**: Can trigger notifications via configured channels

#### NATS Subjects

| Subject | Transport | Direction | Description |
|---------|-----------|-----------|-------------|
| `question.timeout.<id>` | JetStream | Output | Timeout events |
| `question.escalate.<id>` | JetStream | Output | Escalation events |

#### Escalation Flow

When a question's SLA is exceeded:

1. Timeout event published
2. Question reassigned to `escalate_to` answerer
3. Escalation event published
4. Notifications sent (if configured)

#### Dependencies

- `workflow/answerer/registry.go` — Route configuration with SLAs
- `workflow/question.go` — Question store

---

## ChangeProposal Reactive Workflow (ADR-024)

The ChangeProposal lifecycle is implemented as a reactive workflow, not as a standalone processor
component. It follows the same OODA-loop pattern as other reactive workflows (see ADR-005).

### Implementation Files

| File | Purpose |
|------|---------|
| `workflow/reactive/change_proposal.go` | Reactive rules: accept-trigger, dispatch-review, handle-accepted, handle-rejected |
| `workflow/reactive/change_proposal_actions.go` | Cascade logic: graph traversal, dirty marking, event publishing |
| `processor/workflow-api/http_change_proposal.go` | HTTP handlers for proposal CRUD and accept/reject actions |

### Future Components

Two components are planned to support automated ChangeProposal creation. Their reactive workflow
rules are already defined in `workflow/reactive/change_proposal.go` but the components themselves
are future work:

| Component | Purpose | Status |
|-----------|---------|--------|
| `change-proposal-cascade` | Executes dirty cascade after proposal acceptance | Planned (Phase 5) |
| `change-proposal-reviewer` | LLM-based review gate for incoming proposals | Planned (Phase 5) |

See [Workflow System: ChangeProposal Lifecycle](05-workflow-system.md#changeproposal-lifecycle-adr-024)
for the full lifecycle description including the OODA loop and cascade logic.

## Creating New Components

### Directory Structure

```
processor/<name>/
├── component.go   # Discoverable + lifecycle implementation
├── config.go      # Configuration schema
└── factory.go     # Component registration
```

### Required Interface

```go
// Must implement component.Discoverable
type Component struct { ... }

func (c *Component) Meta() component.Metadata
func (c *Component) InputPorts() []component.Port
func (c *Component) OutputPorts() []component.Port
func (c *Component) ConfigSchema() component.ConfigSchema
func (c *Component) Health() component.HealthStatus
func (c *Component) DataFlow() component.FlowMetrics

// Optional lifecycle methods
func (c *Component) Initialize() error
func (c *Component) Start(ctx context.Context) error
func (c *Component) Stop(timeout time.Duration) error
```

### Registration

```go
// factory.go
func Register(registry RegistryInterface) error {
    return registry.RegisterWithConfig(component.RegistrationConfig{
        Name:        "my-component",
        Factory:     NewComponent,
        Schema:      mySchema,
        Type:        "processor",
        Protocol:    "custom",
        Domain:      "semantic",
        Description: "My custom component",
        Version:     "0.1.0",
    })
}
```

### Wiring

1. Import in `cmd/semspec/main.go`
2. Call `mycomponent.Register(registry)`
3. Add instance config to `configs/semspec.json`
