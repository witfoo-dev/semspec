// Package tools registers agent tools with the semstreams agentic-tools component.
// Follows the bash-first approach: bash is the universal tool, specialized tools
// only for things bash can't do (graph queries, terminal signals, DAG decomposition).
//
// All registration happens in RegisterAgenticTools, called once during component
// startup. There are no init() registrations — semspec always runs with NATS.
package tools

import (
	"context"
	"os"
	"path/filepath"

	agentictools "github.com/c360studio/semstreams/processor/agentic-tools"

	"github.com/c360studio/semspec/tools/bash"
	"github.com/c360studio/semspec/tools/decompose"
	"github.com/c360studio/semspec/tools/httptool"
	"github.com/c360studio/semspec/tools/question"
	"github.com/c360studio/semspec/tools/review"
	"github.com/c360studio/semspec/tools/spawn"
	"github.com/c360studio/semspec/tools/terminal"
	"github.com/c360studio/semspec/tools/websearch"
	"github.com/c360studio/semspec/tools/workflow"
	wf "github.com/c360studio/semspec/workflow"
	"github.com/c360studio/semstreams/natsclient"
	nats "github.com/nats-io/nats.go"
)

// AgenticToolDeps carries the infrastructure dependencies required by tools.
type AgenticToolDeps struct {
	// NATSClient is the concrete NATS client.
	NATSClient *natsclient.Client

	// GraphHelper is required by spawn_agent for recording spawn relationships.
	GraphHelper spawn.GraphHelper

	// DefaultModel is the fallback LLM model for spawned agents.
	DefaultModel string

	// MaxDepth overrides the default spawn depth limit (5). Zero uses default.
	MaxDepth int

	// ErrorCategoryRegistry is required by review_scenario for category validation.
	// If nil, the review tool is not registered.
	ErrorCategoryRegistry *wf.ErrorCategoryRegistry
}

// RegisterAgenticTools registers all agent tools. Call once during component startup.
func RegisterAgenticTools(deps AgenticToolDeps) {
	// --- Stateless tools ---

	// bash — universal shell access (sandbox or local).
	repoRoot := resolveRepoRoot()
	bashExec := bash.NewExecutor(repoRoot, os.Getenv("SANDBOX_URL"))
	_ = agentictools.RegisterTool("bash", bashExec)

	// Terminal tools (StopLoop=true).
	termExec := terminal.NewExecutor()
	_ = agentictools.RegisterTool("submit_work", termExec)
	_ = agentictools.RegisterTool("submit_review", termExec)

	// decompose_task — validates LLM-provided TaskDAG.
	decomposeExec := decompose.NewExecutor()
	_ = agentictools.RegisterTool("decompose_task", decomposeExec)

	// http_request — with NATS for graph persistence when available.
	httptool.Register(deps.NATSClient)

	// graph tools (graph_search, graph_query, graph_summary).
	workflow.Register()

	// web_search — only active when BRAVE_SEARCH_API_KEY is set.
	websearch.Register()

	// --- Infrastructure-dependent tools ---

	// spawn_agent — requires NATS + graph.
	if deps.NATSClient != nil && deps.GraphHelper != nil {
		spawnNC := &spawnNATSAdapter{client: deps.NATSClient}
		spawnOpts := []spawn.Option{}
		if deps.DefaultModel != "" {
			spawnOpts = append(spawnOpts, spawn.WithDefaultModel(deps.DefaultModel))
		}
		if deps.MaxDepth > 0 {
			spawnOpts = append(spawnOpts, spawn.WithMaxDepth(deps.MaxDepth))
		}
		spawnExec := spawn.NewExecutor(spawnNC, deps.GraphHelper, spawnOpts...)
		_ = agentictools.RegisterTool("spawn_agent", spawnExec)
	}

	// review_scenario — requires graph helper + error category registry.
	if deps.GraphHelper != nil {
		if rg, ok := deps.GraphHelper.(review.GraphHelper); ok && deps.ErrorCategoryRegistry != nil {
			reviewExec := review.NewExecutor(rg, deps.ErrorCategoryRegistry)
			_ = agentictools.RegisterTool("review_scenario", reviewExec)
		}
	}

	// ask_question — blocks until answer arrives via NATS.
	if deps.NATSClient != nil {
		questionExec := question.NewExecutor(deps.NATSClient, nil)
		_ = agentictools.RegisterTool("ask_question", questionExec)
	}
}

// RegisterAgenticToolsWithContext is RegisterAgenticTools with a context for
// future use (e.g. graceful shutdown). Currently a pass-through.
func RegisterAgenticToolsWithContext(_ context.Context, deps AgenticToolDeps) {
	RegisterAgenticTools(deps)
}

// resolveRepoRoot determines the workspace root from env or cwd.
func resolveRepoRoot() string {
	repoRoot := os.Getenv("SEMSPEC_REPO_PATH")
	if repoRoot == "" {
		var err error
		repoRoot, err = os.Getwd()
		if err != nil {
			repoRoot = "."
		}
	}
	absRepoRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return repoRoot
	}
	return absRepoRoot
}

// spawnNATSAdapter adapts *natsclient.Client to spawn.NATSClient.
type spawnNATSAdapter struct {
	client *natsclient.Client
}

func (a *spawnNATSAdapter) PublishToStream(ctx context.Context, subject string, data []byte) error {
	return a.client.PublishToStream(ctx, subject, data)
}

func (a *spawnNATSAdapter) Subscribe(ctx context.Context, subject string, handler func(msg []byte)) (spawn.Subscription, error) {
	sub, err := a.client.Subscribe(ctx, subject, func(_ context.Context, msg *nats.Msg) {
		handler(msg.Data)
	})
	if err != nil {
		return nil, err
	}
	return sub, nil
}
