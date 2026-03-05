/**
 * API client methods for reactive execution model — ADR-025.
 *
 * Covers agent tree queries, DAG execution fetching, and retrospective data.
 */

import { request } from './client';
import type { AgentLoop, DAGExecution, RetrospectivePhase } from '$lib/types/execution';

/**
 * Fetch the agent hierarchy tree for a plan.
 * Returns a forest of AgentLoop nodes rooted at top-level orchestrators.
 *
 * GET /workflow-api/plans/{slug}/agent-tree
 */
export async function fetchAgentTree(planSlug: string): Promise<AgentLoop[]> {
	return request<AgentLoop[]>(`/workflow-api/plans/${planSlug}/agent-tree`);
}

/**
 * Fetch a DAG execution graph by execution ID.
 * Returns the full graph with nodes, edges (via dependsOn), and statuses.
 *
 * GET /workflow-api/executions/{executionId}
 */
export async function fetchDAGExecution(executionId: string): Promise<DAGExecution> {
	return request<DAGExecution>(`/workflow-api/executions/${executionId}`);
}

/**
 * Fetch the retrospective view for a plan.
 * Returns completed work grouped by Requirement → Scenario → Task.
 *
 * GET /workflow-api/plans/{slug}/phases/retrospective
 */
export async function fetchRetrospective(planSlug: string): Promise<RetrospectivePhase[]> {
	return request<RetrospectivePhase[]>(`/workflow-api/plans/${planSlug}/phases/retrospective`);
}
