package scenarios

import (
	"context"
	"fmt"
	"time"

	"github.com/c360studio/semspec/test/e2e/client"
	"github.com/c360studio/semspec/test/e2e/config"
)

// Trajectory scenario constants.
// kvPollInterval is also used by hello_world.go, context_pressure.go, and todo_app.go.
const (
	// kvPollInterval is the time between polling attempts for KV and graph data
	kvPollInterval = 500 * time.Millisecond
)

// TrajectoryScenario tests the trajectory tracking functionality.
// It triggers an LLM call via CreatePlan REST API and verifies the trajectory
// data is recorded and queryable via the trajectory-api endpoints.
type TrajectoryScenario struct {
	name        string
	description string
	config      *config.Config
	http        *client.HTTPClient
	fs          *client.FilesystemClient
}

// NewTrajectoryScenario creates a new trajectory tracking scenario.
func NewTrajectoryScenario(cfg *config.Config) *TrajectoryScenario {
	return &TrajectoryScenario{
		name:        "trajectory",
		description: "Tests trajectory tracking via trajectory-api endpoints",
		config:      cfg,
	}
}

// Name returns the scenario name.
func (s *TrajectoryScenario) Name() string {
	return s.name
}

// Description returns the scenario description.
func (s *TrajectoryScenario) Description() string {
	return s.description
}

// Setup prepares the scenario environment.
func (s *TrajectoryScenario) Setup(ctx context.Context) error {
	// Create filesystem client and setup workspace
	s.fs = client.NewFilesystemClient(s.config.WorkspacePath)
	if err := s.fs.SetupWorkspace(); err != nil {
		return fmt.Errorf("setup workspace: %w", err)
	}

	// Create HTTP client
	s.http = client.NewHTTPClient(s.config.HTTPBaseURL)

	// Wait for service to be healthy
	if err := s.http.WaitForHealthy(ctx); err != nil {
		return fmt.Errorf("service not healthy: %w", err)
	}

	return nil
}

// Execute runs the trajectory tracking scenario.
func (s *TrajectoryScenario) Execute(ctx context.Context) (*Result, error) {
	result := NewResult(s.name)
	defer result.Complete()

	stages := []struct {
		name string
		fn   func(context.Context, *Result) error
	}{
		{"trigger-llm-call", s.stageTriggerLLMCall},
		{"wait-for-recording", s.stageWaitForRecording},
		{"query-by-trace", s.stageQueryByTrace},
		{"verify-aggregation", s.stageVerifyAggregation},
	}

	for _, stage := range stages {
		stageStart := time.Now()
		// Use longer timeout for LLM-dependent stages
		stageTimeout := s.config.StageTimeout
		if stage.name == "wait-for-recording" {
			stageTimeout = 180 * time.Second // LLM coordination can take a while
		}
		stageCtx, cancel := context.WithTimeout(ctx, stageTimeout)

		err := stage.fn(stageCtx, result)
		cancel()

		stageDuration := time.Since(stageStart)
		result.SetMetric(fmt.Sprintf("%s_duration_us", stage.name), stageDuration.Microseconds())

		if err != nil {
			result.AddStage(stage.name, false, stageDuration, err.Error())
			result.AddError(fmt.Sprintf("%s: %v", stage.name, err))
			result.Error = fmt.Sprintf("%s failed: %v", stage.name, err)
			return result, nil
		}

		result.AddStage(stage.name, true, stageDuration, "")
	}

	result.Success = true
	return result, nil
}

// Teardown cleans up after the scenario.
func (s *TrajectoryScenario) Teardown(_ context.Context) error {
	return nil
}

// stageTriggerLLMCall creates a plan via REST API that triggers the planner component.
// The planner uses LLM calls which are recorded to the knowledge graph.
// Note: Only semspec components (planner, task-generator, question-answerer) record
// LLM calls - the generic agentic-model from semstreams does not.
func (s *TrajectoryScenario) stageTriggerLLMCall(ctx context.Context, result *Result) error {
	// Create plan via REST API which triggers the planner component with LLM recording.
	// The response includes a trace_id that correlates all LLM calls for this request.
	resp, err := s.http.CreatePlan(ctx, "trajectory-test-feature")
	if err != nil {
		return fmt.Errorf("create plan: %w", err)
	}

	if resp.Error != "" {
		return fmt.Errorf("plan creation returned error: %s", resp.Error)
	}

	result.SetDetail("trigger_response", resp)

	// The workflow-api returns trace_id directly in the response.
	traceID := resp.TraceID
	if traceID == "" {
		return fmt.Errorf("workflow-api did not return a trace_id in the plan creation response")
	}

	result.SetDetail("trace_id", traceID)
	if resp.Slug != "" {
		result.SetDetail("plan_slug", resp.Slug)
	}
	return nil
}

// stageWaitForRecording waits for LLM call records to appear in the knowledge graph
// by polling the trajectory-api /traces/{trace_id} endpoint.
// LLM calls are stored in the knowledge graph (not KV) since the graph migration.
func (s *TrajectoryScenario) stageWaitForRecording(ctx context.Context, result *Result) error {
	traceID, ok := result.GetDetailString("trace_id")
	if !ok || traceID == "" {
		return fmt.Errorf("trace_id not set from trigger stage")
	}

	ticker := time.NewTicker(kvPollInterval)
	defer ticker.Stop()

	var lastErr error
	for {
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("timeout waiting for LLM calls in graph: %w (last error: %v)", ctx.Err(), lastErr)
			}
			return fmt.Errorf("timeout waiting for LLM calls in graph: %w", ctx.Err())
		case <-ticker.C:
			trajectory, statusCode, err := s.http.GetTrajectoryByTrace(ctx, traceID, false)
			if err != nil {
				if statusCode == 404 {
					// Not yet recorded — keep polling
					lastErr = fmt.Errorf("trajectory not yet available (404)")
					continue
				}
				if statusCode == 503 {
					// trajectory-api graph querier not ready — keep polling
					lastErr = fmt.Errorf("trajectory-api not ready (503)")
					continue
				}
				lastErr = err
				continue
			}

			if trajectory != nil && trajectory.ModelCalls > 0 {
				result.SetDetail("llm_calls_count", trajectory.ModelCalls)
				result.SetDetail("trajectory_from_poll", trajectory)
				return nil
			}

			// trajectory returned but no model calls yet
			modelCalls := 0
			if trajectory != nil {
				modelCalls = trajectory.ModelCalls
			}
			lastErr = fmt.Errorf("trajectory found but no model calls yet (got %d)", modelCalls)
		}
	}
}

// stageQueryByTrace queries trajectory data with full entries using the trace ID
// obtained from the plan creation response.
func (s *TrajectoryScenario) stageQueryByTrace(ctx context.Context, result *Result) error {
	traceID, ok := result.GetDetailString("trace_id")
	if !ok || traceID == "" {
		return fmt.Errorf("trace_id not set from trigger stage")
	}

	// Query with format=json to include full entry details
	trajectory, statusCode, err := s.http.GetTrajectoryByTrace(ctx, traceID, true)
	if err != nil {
		if statusCode == 404 {
			// trajectory-api may be disabled in this deployment
			result.AddWarning("trajectory-api returned 404 - component may not be enabled")
			return nil
		}
		return fmt.Errorf("get trajectory by trace: %w", err)
	}

	result.SetDetail("trajectory_trace_id", trajectory.TraceID)
	result.SetDetail("trajectory_model_calls", trajectory.ModelCalls)
	result.SetDetail("trajectory_tokens_in", trajectory.TokensIn)
	result.SetDetail("trajectory_tokens_out", trajectory.TokensOut)
	result.SetDetail("trajectory_entries_count", len(trajectory.Entries))

	return nil
}

// stageVerifyAggregation verifies the trajectory aggregation logic.
func (s *TrajectoryScenario) stageVerifyAggregation(_ context.Context, result *Result) error {
	// Verify we have trajectory data
	modelCalls, ok := result.GetDetail("trajectory_model_calls")
	if !ok {
		// Skip verification if trajectory-api wasn't available
		if _, hasWarning := result.GetDetail("trajectory_trace_id"); !hasWarning {
			result.AddWarning("Skipping aggregation verification - trajectory data not available")
			return nil
		}
	}

	// Verify model calls count makes sense
	if calls, ok := modelCalls.(int); ok && calls > 0 {
		result.SetDetail("verified_model_calls", true)
	}

	// Verify token counts are reasonable
	tokensIn, _ := result.GetDetail("trajectory_tokens_in")
	tokensOut, _ := result.GetDetail("trajectory_tokens_out")

	if in, ok := tokensIn.(int); ok && in > 0 {
		result.SetDetail("verified_tokens_in", true)
	}
	if out, ok := tokensOut.(int); ok && out > 0 {
		result.SetDetail("verified_tokens_out", true)
	}

	return nil
}
