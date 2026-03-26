package requirementexecutor

// exec_mutations.go — helpers for sending mutations to execution-manager.
// Requirement-executor is NOT a writer to EXECUTION_STATES. All persistent
// state changes go through execution-manager via request/reply mutations.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/c360studio/semspec/workflow"
	"github.com/c360studio/semspec/workflow/payloads"
	"github.com/c360studio/semstreams/natsclient"
)

// Mutation subjects — must match execution-manager/mutations.go constants.
const (
	mutReqCreate = "execution.mutation.req.create"
	mutReqPhase  = "execution.mutation.req.phase"
	mutReqNode   = "execution.mutation.req.node"
)

// execMutationResponse mirrors ExecMutationResponse from execution-manager.
type execMutationResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Key     string `json:"key,omitempty"`
}

// sendReqCreate sends a requirement execution creation mutation to execution-manager.
// Returns the store key on success.
func (c *Component) sendReqCreate(ctx context.Context, exec *requirementExecution, trigger *payloads.RequirementExecutionRequest) (string, error) {
	req := map[string]any{
		"slug":           trigger.Slug,
		"requirement_id": trigger.RequirementID,
		"title":          trigger.Title,
		"description":    trigger.Description,
		"project_id":     trigger.ProjectID,
		"trace_id":       trigger.TraceID,
		"loop_id":        trigger.LoopID,
		"request_id":     trigger.RequestID,
		"model":          trigger.Model,
		"scenarios":      exec.Scenarios,
	}
	if exec.BlueTeamID != "" {
		req["blue_team_id"] = exec.BlueTeamID
	}
	if exec.RedTeamID != "" {
		req["red_team_id"] = exec.RedTeamID
	}

	resp, err := c.sendMutation(ctx, mutReqCreate, req)
	if err != nil {
		return "", err
	}
	return resp.Key, nil
}

// sendReqPhase sends a phase transition mutation to execution-manager.
func (c *Component) sendReqPhase(ctx context.Context, key, stage string, fields map[string]any) error {
	req := map[string]any{
		"key":   key,
		"stage": stage,
	}
	for k, v := range fields {
		req[k] = v
	}
	_, err := c.sendMutation(ctx, mutReqPhase, req)
	return err
}

// sendReqNode sends a DAG node update mutation to execution-manager.
func (c *Component) sendReqNode(ctx context.Context, key string, nodeIdx int, nodeTaskID string, result *workflow.NodeResult) error {
	req := map[string]any{
		"key":               key,
		"current_node_idx":  nodeIdx,
		"current_node_task_id": nodeTaskID,
	}
	if result != nil {
		req["node_result"] = result
	}
	_, err := c.sendMutation(ctx, mutReqNode, req)
	return err
}

// sendMutation sends a mutation request/reply to execution-manager and parses the response.
func (c *Component) sendMutation(ctx context.Context, subject string, payload any) (*execMutationResponse, error) {
	if c.natsClient == nil {
		return nil, fmt.Errorf("%s: nats client not available", subject)
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", subject, err)
	}

	respData, err := c.natsClient.RequestWithRetry(ctx, subject, data, 5*time.Second, natsclient.DefaultRetryConfig())
	if err != nil {
		return nil, fmt.Errorf("%s request failed: %w", subject, err)
	}

	var resp execMutationResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal %s response: %w", subject, err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("%s rejected: %s", subject, resp.Error)
	}

	return &resp, nil
}
