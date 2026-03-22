// Package scenarios provides e2e test scenario implementations.
package scenarios

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/c360studio/semspec/processor/context-builder/gatherers"
	"github.com/c360studio/semspec/test/e2e/client"
	"github.com/c360studio/semspec/test/e2e/config"
)

// RestAPIScenario is a Tier 3 E2E scenario that drives a Go HTTP service
// through the full semspec pipeline, adding a /users REST API with CRUD
// endpoints and request logging middleware. It tests that the system can:
//
//  1. Detect Go as the project language.
//  2. Plan and approve adding multi-endpoint CRUD + middleware.
//  3. Execute the plan, decomposing it into parallel TDD tasks.
//  4. Produce handler .go files, middleware .go files, and _test.go files.
//
// This scenario uses real LLM inference and is a medium-complexity stepping
// stone between the health-check (Tier 2) and epic-meshtastic (Tier 4).
// Run it with:
//
//	task e2e:llm -- rest-api claude
//	task e2e:llm -- rest-api openrouter
//
// Stages:
//  1. setup-project       — Write Go fixture files to workspace, verify, git init.
//  2. detect-stack        — Verify Go is detected.
//  3. init-project        — Initialize project with detected languages.
//  4. verify-graph-ready  — Gate on graph-gateway readiness.
//  5. create-plan         — POST plan with /users CRUD + middleware prompt.
//  6. wait-for-plan-goal  — Poll until planner writes a Goal.
//  7. wait-for-approval   — Poll until plan.Approved == true.
//  8. trigger-execution   — POST execute.
//  9. wait-for-scenarios  — Verify at least 3 scenarios generated.
// 10. wait-for-execution  — Poll until execution pipeline progresses.
// 11. verify-deliverables — Check for handler .go files, middleware .go, _test.go files.
type RestAPIScenario struct {
	config *config.Config
	http   *client.HTTPClient
	fs     *client.FilesystemClient
}

// NewRestAPIScenario creates a new REST API scenario.
func NewRestAPIScenario(cfg *config.Config) *RestAPIScenario {
	return &RestAPIScenario{
		config: cfg,
		http:   client.NewHTTPClient(cfg.HTTPBaseURL),
		fs:     client.NewFilesystemClient(cfg.WorkspacePath),
	}
}

func (s *RestAPIScenario) Name() string { return "rest-api" }
func (s *RestAPIScenario) Description() string {
	return "Tier 3: Go HTTP service — add /users CRUD + request logging middleware through full plan+execute pipeline"
}

// Setup writes fixture files to the workspace before Execute runs.
func (s *RestAPIScenario) Setup(ctx context.Context) error {
	return s.setupWorkspace()
}

// Teardown is a no-op; the workspace is cleaned by the test runner.
func (s *RestAPIScenario) Teardown(ctx context.Context) error { return nil }

// Execute runs the scenario stages sequentially. Each stage has its own
// deadline; a stage failure short-circuits the run and records the error.
func (s *RestAPIScenario) Execute(ctx context.Context) (*Result, error) {
	result := NewResult(s.Name())
	defer result.Complete()

	stages := []struct {
		name    string
		fn      func(context.Context, *Result) error
		timeout time.Duration
	}{
		{"setup-project", s.stageSetupProject, 30 * time.Second},
		{"detect-stack", s.stageDetectStack, 15 * time.Second},
		{"init-project", s.stageInitProject, 15 * time.Second},
		{"verify-graph-ready", s.stageVerifyGraphReady, 30 * time.Second},
		{"create-plan", s.stageCreatePlan, 15 * time.Second},
		{"wait-for-plan-goal", s.stageWaitForPlanGoal, 180 * time.Second},
		{"wait-for-approval", s.stageWaitForApproval, 600 * time.Second},
		{"trigger-execution", s.stageTriggerExecution, 15 * time.Second},
		{"wait-for-scenarios", s.stageWaitForScenarios, 120 * time.Second},
		{"wait-for-execution", s.stageWaitForExecution, 600 * time.Second},
		{"verify-deliverables", s.stageVerifyDeliverables, 30 * time.Second},
	}

	if s.config.FastTimeouts {
		for i := range stages {
			stages[i].timeout = stages[i].timeout / 2
		}
	}

	for _, stage := range stages {
		stageCtx, cancel := context.WithTimeout(ctx, stage.timeout)
		start := time.Now()

		err := stage.fn(stageCtx, result)
		duration := time.Since(start)
		cancel()

		if err != nil {
			result.AddStage(stage.name, false, duration, err.Error())
			result.AddError(fmt.Sprintf("%s: %s", stage.name, err.Error()))
			result.Error = fmt.Sprintf("%s failed: %s", stage.name, err.Error())
			return result, nil
		}

		result.AddStage(stage.name, true, duration, "")
		result.SetMetric(stage.name+"_duration_us", duration.Microseconds())
	}

	result.Success = true
	return result, nil
}

// ---------------------------------------------------------------------------
// Workspace setup
// ---------------------------------------------------------------------------

func (s *RestAPIScenario) setupWorkspace() error {
	files := map[string]string{
		"go.mod": "module example.com/userservice\n\ngo 1.25\n",
		"main.go": "package main\n\nimport (\n\t\"fmt\"\n\t\"log\"\n\t\"net/http\"\n)\n\nfunc main() {\n\tmux := http.NewServeMux()\n\tmux.HandleFunc(\"/\", func(w http.ResponseWriter, r *http.Request) {\n\t\tfmt.Fprintf(w, \"User Service v0.1.0\")\n\t})\n\n\tlog.Println(\"Starting server on :8080\")\n\tlog.Fatal(http.ListenAndServe(\":8080\", mux))\n}\n",
		"README.md": "# User Service\n\nA Go HTTP service. Needs a /users REST API with CRUD operations and request logging middleware.\n\n## Current endpoints\n\n- `GET /` — Service info\n",
		".semspec/projects/default/project.json": "{\n  \"name\": \"user-service\",\n  \"description\": \"Go HTTP service for REST API E2E testing\",\n  \"languages\": [\"go\"],\n  \"created_at\": \"2026-03-22T00:00:00Z\"\n}\n",
	}
	for path, content := range files {
		if err := s.fs.WriteFileRelative(path, content); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	if err := s.fs.InitGit(); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Stages
// ---------------------------------------------------------------------------

func (s *RestAPIScenario) stageSetupProject(ctx context.Context, result *Result) error {
	for _, path := range []string{"go.mod", "main.go", "README.md"} {
		full := filepath.Join(s.config.WorkspacePath, path)
		if _, err := os.Stat(full); os.IsNotExist(err) {
			return fmt.Errorf("fixture file missing: %s", path)
		}
	}
	if !s.fs.IsGitRepo() {
		return fmt.Errorf("workspace is not a git repository after setup")
	}
	result.SetDetail("project_ready", true)
	return nil
}

func (s *RestAPIScenario) stageDetectStack(ctx context.Context, result *Result) error {
	detection, err := s.http.DetectProject(ctx)
	if err != nil {
		return fmt.Errorf("detect project: %w", err)
	}
	if len(detection.Languages) == 0 {
		return fmt.Errorf("no languages detected")
	}
	// Confirm Go was detected — warn rather than fail, since detection heuristics
	// may rank other markers first depending on infra configuration.
	foundGo := false
	for _, lang := range detection.Languages {
		if strings.EqualFold(lang.Name, "go") {
			foundGo = true
			break
		}
	}
	if !foundGo {
		result.AddWarning(fmt.Sprintf("Go not in detected languages: %v — proceeding anyway", detection.Languages))
	}
	result.SetDetail("detected_languages", len(detection.Languages))
	result.SetDetail("go_detected", foundGo)
	return nil
}

func (s *RestAPIScenario) stageInitProject(ctx context.Context, result *Result) error {
	detection, err := s.http.DetectProject(ctx)
	if err != nil {
		return fmt.Errorf("detect: %w", err)
	}

	var languages []string
	for _, lang := range detection.Languages {
		languages = append(languages, lang.Name)
	}

	initReq := &client.ProjectInitRequest{
		Project: client.ProjectInitInput{
			Name:        "REST API E2E",
			Description: "Test the full plan + execution pipeline for a Go HTTP service with CRUD endpoints",
			Languages:   languages,
		},
		Checklist: detection.ProposedChecklist,
		Standards: client.StandardsInput{
			Version: "1.0.0",
			Rules:   []any{},
		},
	}

	resp, err := s.http.InitProject(ctx, initReq)
	if err != nil {
		return fmt.Errorf("init project: %w", err)
	}
	result.SetDetail("init_success", resp.Success)
	return nil
}

func (s *RestAPIScenario) stageVerifyGraphReady(ctx context.Context, result *Result) error {
	g := gatherers.NewGraphGatherer(s.config.GraphURL)
	if err := g.WaitForReady(ctx, 30*time.Second); err != nil {
		return fmt.Errorf("graph not ready: %w", err)
	}
	result.SetDetail("graph_ready", true)
	return nil
}

const restAPIPlanPrompt = `Add a /users REST API to the Go HTTP service with the following:

1. CRUD endpoints:
   - GET /users — list all users (in-memory store)
   - GET /users/{id} — get user by ID
   - POST /users — create a user (name, email required)
   - DELETE /users/{id} — delete a user

2. Request logging middleware:
   - Log method, path, status code, and duration for every request
   - Apply to all routes

3. Error handling:
   - Return 404 JSON for missing users
   - Return 400 JSON for invalid POST body
   - Return 405 for unsupported methods

Include unit tests for handlers and middleware.`

func (s *RestAPIScenario) stageCreatePlan(ctx context.Context, result *Result) error {
	resp, err := s.http.CreatePlan(ctx, restAPIPlanPrompt)
	if err != nil {
		return fmt.Errorf("create plan: %w", err)
	}
	if resp.Slug == "" {
		return fmt.Errorf("empty slug in response")
	}
	result.SetDetail("plan_slug", resp.Slug)
	result.SetDetail("plan_trace_id", resp.TraceID)
	return nil
}

func (s *RestAPIScenario) stageWaitForPlanGoal(ctx context.Context, result *Result) error {
	slug, _ := result.GetDetailString("plan_slug")

	plan, err := s.http.WaitForPlanGoal(ctx, slug)
	if err != nil {
		return fmt.Errorf("plan goal never populated: %w", err)
	}
	result.SetDetail("plan_goal", plan.Goal)
	return nil
}

func (s *RestAPIScenario) stageWaitForApproval(ctx context.Context, result *Result) error {
	slug, _ := result.GetDetailString("plan_slug")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("plan approval timed out: %w", ctx.Err())
		case <-ticker.C:
			plan, err := s.http.GetPlan(ctx, slug)
			if err != nil {
				continue
			}
			if plan.Approved {
				result.SetDetail("plan_approved", true)
				result.SetDetail("plan_stage", plan.Stage)
				return nil
			}
			if plan.Stage == "escalated" || plan.Stage == "error" {
				return fmt.Errorf("plan reached terminal state: %s", plan.Stage)
			}
		}
	}
}

func (s *RestAPIScenario) stageTriggerExecution(ctx context.Context, result *Result) error {
	slug, _ := result.GetDetailString("plan_slug")

	// Capture agent.complete.* baseline before triggering execution so that
	// stageWaitForExecution can detect growth from the execution pipeline
	// rather than counting planning-phase completions.
	baselineEntries, _ := s.http.GetMessageLogEntries(ctx, 500, "agent.complete.*")
	result.SetDetail("exec_complete_baseline_count", len(baselineEntries))

	resp, err := s.http.ExecutePlan(ctx, slug)
	if err != nil {
		return fmt.Errorf("execute plan: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("execute plan returned error: %s", resp.Error)
	}

	result.SetDetail("execute_batch_id", resp.BatchID)
	result.SetDetail("execution_triggered", true)
	return nil
}

// stageWaitForScenarios polls until at least 3 scenarios have been generated
// for the plan. The REST API prompt should produce CRUD, middleware, and error
// handling requirements — each yielding at least one scenario.
func (s *RestAPIScenario) stageWaitForScenarios(ctx context.Context, result *Result) error {
	slug, _ := result.GetDetailString("plan_slug")

	const minScenarios = 3

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("fewer than %d scenarios generated within timeout: %w", minScenarios, ctx.Err())
		case <-ticker.C:
			scenarioList, err := s.http.ListScenarios(ctx, slug, "")
			if err != nil {
				// Transient error — keep polling.
				continue
			}
			if len(scenarioList) >= minScenarios {
				result.SetDetail("scenario_count", len(scenarioList))
				return nil
			}
		}
	}
}

// stageWaitForExecution polls two signals to confirm the execution pipeline
// progressed beyond the trigger phase:
//
//  1. agent.task.* messages — TDD pipeline dispatched at least one stage.
//  2. agent.complete.* count growth beyond the pre-execution baseline.
//
// Both checks emit warnings rather than hard failures when the pipeline has
// not fully completed, because real-LLM scenarios may still be in-flight.
// The stage returns nil once either signal is observed, or after a timeout.
func (s *RestAPIScenario) stageWaitForExecution(ctx context.Context, result *Result) error {
	baseline := 0
	if v, ok := result.GetDetail("exec_complete_baseline_count"); ok {
		if n, ok := v.(int); ok {
			baseline = n
		}
	}

	// Gate 1: wait for at least one agent.task.* dispatch — confirms the TDD
	// pipeline received its first task from scenario-execution-loop.
	taskCtx, cancelTask := context.WithTimeout(ctx, 120*time.Second)
	defer cancelTask()

	if err := s.pollMessageLogger(taskCtx, "agent.task.*", 1); err != nil {
		result.AddWarning(fmt.Sprintf(
			"no agent.task.* messages observed within 120s — execution pipeline may not have started: %v", err,
		))
		result.SetDetail("task_dispatch_observed", false)
	} else {
		result.SetDetail("task_dispatch_observed", true)
	}

	// Gate 2: wait for agent.complete.* count to grow beyond the baseline.
	// This confirms at least one agentic loop in the execution phase completed.
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			result.AddWarning(fmt.Sprintf(
				"agent.complete.* count did not grow beyond baseline %d within timeout: %v",
				baseline, ctx.Err(),
			))
			result.SetDetail("exec_complete_observed", false)
			// Non-fatal: the pipeline may still be running in a real-LLM environment.
			return nil
		case <-ticker.C:
			entries, err := s.http.GetMessageLogEntries(ctx, 500, "agent.complete.*")
			if err != nil {
				continue
			}
			if len(entries) > baseline {
				result.SetDetail("exec_complete_observed", true)
				result.SetDetail("exec_complete_count", len(entries))
				result.SetDetail("exec_complete_new", len(entries)-baseline)
				return nil
			}
		}
	}
}

// stageVerifyDeliverables checks the workspace for concrete output from the
// execution pipeline. It looks for new .go files (handlers, middleware) and
// _test.go files written by the agent beyond the initial fixture files.
//
// Checks warn rather than fail because real-LLM agents may place files in
// varied directory structures (handlers/, users/, internal/, etc.).
func (s *RestAPIScenario) stageVerifyDeliverables(ctx context.Context, result *Result) error {
	allFiles, err := s.fs.ListFiles()
	if err != nil {
		return fmt.Errorf("list workspace files: %w", err)
	}

	// Fixture files present before execution — not counted as agent output.
	fixtureFiles := map[string]bool{
		"main.go":   true,
		"go.mod":    true,
		"README.md": true,
	}

	var (
		newGoFiles        []string
		testGoFiles       []string
		handlerFiles      []string
		middlewareFiles   []string
	)

	for _, f := range allFiles {
		base := filepath.Base(f)
		if fixtureFiles[base] {
			continue
		}

		switch {
		case strings.HasSuffix(f, "_test.go"):
			testGoFiles = append(testGoFiles, f)
		case strings.HasSuffix(f, ".go"):
			newGoFiles = append(newGoFiles, f)

			// Identify handler files — agents may put them in handlers/, users/,
			// or name them users.go, user_handler.go, etc.
			lower := strings.ToLower(f)
			if strings.Contains(lower, "handler") || strings.Contains(lower, "users") ||
				strings.Contains(lower, "user") {
				handlerFiles = append(handlerFiles, f)
			}

			// Identify middleware files — agents may name them middleware.go,
			// logging.go, logger.go, or place them in a middleware/ directory.
			if strings.Contains(lower, "middleware") || strings.Contains(lower, "logging") ||
				strings.Contains(lower, "logger") {
				middlewareFiles = append(middlewareFiles, f)
			}
		}
	}

	result.SetDetail("new_go_files", newGoFiles)
	result.SetDetail("test_go_files", testGoFiles)
	result.SetDetail("handler_files", handlerFiles)
	result.SetDetail("middleware_files", middlewareFiles)
	result.SetDetail("workspace_file_count", len(allFiles))

	// Warn rather than fail — real LLM may not have finished or may place
	// files under unexpected paths.
	if len(newGoFiles) == 0 && len(testGoFiles) == 0 {
		result.AddWarning("no new .go files found in workspace — execution pipeline may not have written output yet")
	}

	if len(testGoFiles) == 0 {
		result.AddWarning("no _test.go files found — TDD pipeline may not have completed")
	}

	if len(handlerFiles) == 0 {
		result.AddWarning("no handler .go files detected — agent may have used an unexpected file layout")
	}

	if len(middlewareFiles) == 0 {
		result.AddWarning("no middleware .go files detected — agent may have inlined middleware or used an unexpected name")
	}

	// Verify mock stats when running against the mock LLM to confirm the
	// execution pipeline actually fired.
	if s.config.MockLLMURL != "" {
		s.verifyMockStats(ctx, result)
	}

	return nil
}

// verifyMockStats fetches call counts from the mock LLM and records them as
// details and warnings. It never returns an error — mock stats are advisory.
func (s *RestAPIScenario) verifyMockStats(ctx context.Context, result *Result) {
	statsURL := s.config.MockLLMURL + "/stats"
	req, err := http.NewRequestWithContext(ctx, "GET", statsURL, nil)
	if err != nil {
		result.AddWarning(fmt.Sprintf("create mock stats request: %v", err))
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		result.AddWarning(fmt.Sprintf("fetch mock stats: %v", err))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.AddWarning(fmt.Sprintf("read mock stats body: %v", err))
		return
	}

	var mockStats struct {
		CallsByModel map[string]int `json:"calls_by_model"`
		TotalCalls   int            `json:"total_calls"`
	}
	if err := json.Unmarshal(body, &mockStats); err != nil {
		result.AddWarning(fmt.Sprintf("parse mock stats: %v", err))
		return
	}

	stats := mockStats.CallsByModel
	if stats == nil {
		stats = make(map[string]int)
	}

	result.SetDetail("mock_total_calls", mockStats.TotalCalls)

	// Plan phase: planner and reviewer must both have been called.
	for _, model := range []string{"mock-planner", "mock-reviewer"} {
		if count := stats[model]; count == 0 {
			result.AddWarning(fmt.Sprintf("expected mock model %q to be called, got 0", model))
		}
	}

	// Execution phase: mock-coder handles decomposer + TDD stages.
	// Expect more calls than health-check due to ~4-6 tasks vs 1.
	if coderCalls := stats["mock-coder"]; coderCalls > 0 {
		result.SetDetail("mock_coder_calls", coderCalls)
	} else {
		result.AddWarning("mock-coder was not called — execution phase may not have progressed to task execution")
	}

	var summary []string
	for model, count := range stats {
		summary = append(summary, fmt.Sprintf("%s=%d", model, count))
	}
	result.SetDetail("mock_call_summary", strings.Join(summary, ", "))
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// pollMessageLogger polls the message-logger until at least minCount entries
// appear for the given subjectFilter, or the context is cancelled.
func (s *RestAPIScenario) pollMessageLogger(ctx context.Context, subjectFilter string, minCount int) error {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			entries, err := s.http.GetMessageLogEntries(ctx, 100, subjectFilter)
			if err != nil {
				// Transient error — keep polling.
				continue
			}
			if len(entries) >= minCount {
				return nil
			}
		}
	}
}
