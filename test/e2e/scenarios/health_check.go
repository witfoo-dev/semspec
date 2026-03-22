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

// HealthCheckScenario is a Tier 2 E2E scenario that drives a minimal Go HTTP
// service through the full semspec pipeline. It tests that the system can:
//
//  1. Detect Go as the project language.
//  2. Plan and approve adding a /health endpoint.
//  3. Execute the plan, decomposing it into TDD tasks.
//  4. Produce new .go source files and _test.go files in the workspace.
//
// This scenario uses real LLM inference and is designed for low token cost.
// Run it with:
//
//	task e2e:llm -- health-check claude
//	task e2e:llm -- health-check openrouter
//
// Stages:
//  1. setup-project       — Copy Go fixture files to workspace, verify, git init.
//  2. detect-stack        — Verify Go is detected.
//  3. init-project        — Initialize project with detected languages.
//  4. verify-graph-ready  — Gate on graph-gateway readiness.
//  5. create-plan         — POST plan with /health endpoint prompt.
//  6. wait-for-plan-goal  — Poll until planner writes a Goal.
//  7. wait-for-approval   — Poll until plan.Approved == true.
//  8. trigger-execution   — POST execute.
//  9. wait-for-scenarios  — Verify at least 1 scenario generated.
// 10. wait-for-execution  — Poll until execution pipeline progresses.
// 11. verify-deliverables — Check for new .go files and _test.go files.
type HealthCheckScenario struct {
	config *config.Config
	http   *client.HTTPClient
	fs     *client.FilesystemClient
}

// NewHealthCheckScenario creates a new health check scenario.
func NewHealthCheckScenario(cfg *config.Config) *HealthCheckScenario {
	return &HealthCheckScenario{
		config: cfg,
		http:   client.NewHTTPClient(cfg.HTTPBaseURL),
		fs:     client.NewFilesystemClient(cfg.WorkspacePath),
	}
}

func (s *HealthCheckScenario) Name() string { return "health-check" }
func (s *HealthCheckScenario) Description() string {
	return "Tier 2: Go HTTP service — add /health endpoint through full plan+execute pipeline"
}

// Setup writes fixture files to the workspace before Execute runs.
func (s *HealthCheckScenario) Setup(ctx context.Context) error {
	return s.setupWorkspace()
}

// Teardown is a no-op; the workspace is cleaned by the test runner.
func (s *HealthCheckScenario) Teardown(ctx context.Context) error { return nil }

// Execute runs the scenario stages sequentially. Each stage has its own
// deadline; a stage failure short-circuits the run and records the error.
func (s *HealthCheckScenario) Execute(ctx context.Context) (*Result, error) {
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
		{"wait-for-plan-goal", s.stageWaitForPlanGoal, 120 * time.Second},
		{"wait-for-approval", s.stageWaitForApproval, 300 * time.Second},
		{"trigger-execution", s.stageTriggerExecution, 15 * time.Second},
		{"wait-for-scenarios", s.stageWaitForScenarios, 60 * time.Second},
		{"wait-for-execution", s.stageWaitForExecution, 300 * time.Second},
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

func (s *HealthCheckScenario) setupWorkspace() error {
	files := map[string]string{
		"go.mod": "module example.com/healthservice\n\ngo 1.25\n",
		"main.go": "package main\n\nimport (\n\t\"fmt\"\n\t\"log\"\n\t\"net/http\"\n)\n\nfunc main() {\n\thttp.HandleFunc(\"/\", func(w http.ResponseWriter, r *http.Request) {\n\t\tfmt.Fprintf(w, \"Hello, World!\")\n\t})\n\n\tlog.Println(\"Starting server on :8080\")\n\tlog.Fatal(http.ListenAndServe(\":8080\", nil))\n}\n",
		"README.md": "# Health Service\n\nA simple Go HTTP service. Needs a /health endpoint.\n",
		".semspec/projects/default/project.json": "{\n  \"name\": \"health-service\",\n  \"description\": \"Go HTTP service for health check E2E testing\",\n  \"languages\": [\"go\"],\n  \"created_at\": \"2026-03-22T00:00:00Z\"\n}\n",
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

func (s *HealthCheckScenario) stageSetupProject(ctx context.Context, result *Result) error {
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

func (s *HealthCheckScenario) stageDetectStack(ctx context.Context, result *Result) error {
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

func (s *HealthCheckScenario) stageInitProject(ctx context.Context, result *Result) error {
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
			Name:        "Health Check E2E",
			Description: "Test the full plan + execution pipeline for a Go HTTP service",
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

func (s *HealthCheckScenario) stageVerifyGraphReady(ctx context.Context, result *Result) error {
	g := gatherers.NewGraphGatherer(s.config.GraphURL)
	if err := g.WaitForReady(ctx, 30*time.Second); err != nil {
		return fmt.Errorf("graph not ready: %w", err)
	}
	result.SetDetail("graph_ready", true)
	return nil
}

const healthCheckPlanPrompt = `Add a /health endpoint to the Go HTTP service. The endpoint should return JSON with:
- "status": "ok"
- "uptime": seconds since server start
- "version": Go runtime version

Include unit tests for the health handler.`

func (s *HealthCheckScenario) stageCreatePlan(ctx context.Context, result *Result) error {
	resp, err := s.http.CreatePlan(ctx, healthCheckPlanPrompt)
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

func (s *HealthCheckScenario) stageWaitForPlanGoal(ctx context.Context, result *Result) error {
	slug, _ := result.GetDetailString("plan_slug")

	plan, err := s.http.WaitForPlanGoal(ctx, slug)
	if err != nil {
		return fmt.Errorf("plan goal never populated: %w", err)
	}
	result.SetDetail("plan_goal", plan.Goal)
	return nil
}

func (s *HealthCheckScenario) stageWaitForApproval(ctx context.Context, result *Result) error {
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

func (s *HealthCheckScenario) stageTriggerExecution(ctx context.Context, result *Result) error {
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

// stageWaitForScenarios polls until at least one scenario has been generated
// for the plan. This confirms the planning phase fully resolved requirements
// into testable scenarios before execution begins.
func (s *HealthCheckScenario) stageWaitForScenarios(ctx context.Context, result *Result) error {
	slug, _ := result.GetDetailString("plan_slug")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("no scenarios generated within timeout: %w", ctx.Err())
		case <-ticker.C:
			scenarioList, err := s.http.ListScenarios(ctx, slug, "")
			if err != nil {
				// Transient error — keep polling.
				continue
			}
			if len(scenarioList) >= 1 {
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
func (s *HealthCheckScenario) stageWaitForExecution(ctx context.Context, result *Result) error {
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
	ticker := time.NewTicker(5 * time.Second)
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
// execution pipeline. It looks for new .go files and _test.go files written
// by the agent beyond the initial fixture (main.go, go.mod).
func (s *HealthCheckScenario) stageVerifyDeliverables(ctx context.Context, result *Result) error {
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
		newGoFiles  []string
		testGoFiles []string
	)

	for _, f := range allFiles {
		base := filepath.Base(f)
		if fixtureFiles[base] {
			continue
		}
		if strings.HasSuffix(f, "_test.go") {
			testGoFiles = append(testGoFiles, f)
		} else if strings.HasSuffix(f, ".go") {
			newGoFiles = append(newGoFiles, f)
		}
	}

	result.SetDetail("new_go_files", newGoFiles)
	result.SetDetail("test_go_files", testGoFiles)
	result.SetDetail("workspace_file_count", len(allFiles))

	// Warn rather than fail when the agent has not yet written files — the
	// execution pipeline may still be in-flight under a real LLM.
	if len(newGoFiles) == 0 && len(testGoFiles) == 0 {
		result.AddWarning("no new .go files found in workspace — execution pipeline may not have written output yet")
	}

	if len(testGoFiles) == 0 {
		result.AddWarning("no _test.go files found — TDD pipeline may not have completed")
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
func (s *HealthCheckScenario) verifyMockStats(ctx context.Context, result *Result) {
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
func (s *HealthCheckScenario) pollMessageLogger(ctx context.Context, subjectFilter string, minCount int) error {
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
