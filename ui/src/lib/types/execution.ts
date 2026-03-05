/**
 * Types for reactive execution model visualization — ADR-025.
 *
 * Covers agent hierarchies, DAG execution graphs, and retrospective phase views.
 */

// ============================================================================
// Agent Loop Hierarchy
// ============================================================================

/**
 * AgentLoop represents a single agent loop in the execution hierarchy.
 * Loops can spawn child loops, forming a tree rooted at the orchestrator.
 */
export interface AgentLoop {
	loopId: string;
	parentLoopId?: string;
	role: string;
	model: string;
	status: 'running' | 'completed' | 'failed';
	depth: number;
	children: AgentLoop[];
	taskId?: string;
	startedAt?: string;
	completedAt?: string;
}

// ============================================================================
// DAG Execution
// ============================================================================

/**
 * DAGExecution represents the full execution graph for a scenario.
 * Nodes are tasks with explicit dependency edges.
 */
export interface DAGExecution {
	executionId: string;
	scenarioId: string;
	status: 'executing' | 'complete' | 'failed';
	nodes: DAGNode[];
}

/**
 * DAGNode represents a single task vertex in the execution graph.
 */
export interface DAGNode {
	id: string;
	prompt: string;
	role: string;
	dependsOn: string[];
	status: 'pending' | 'running' | 'completed' | 'failed';
	loopId?: string;
}

// ============================================================================
// Retrospective
// ============================================================================

/**
 * RetrospectivePhase groups completed work by Requirement for the retrospective view.
 */
export interface RetrospectivePhase {
	requirementId: string;
	requirementTitle: string;
	scenarios: RetrospectiveScenario[];
}

/**
 * RetrospectiveScenario groups completed tasks under a single scenario.
 */
export interface RetrospectiveScenario {
	scenarioId: string;
	scenarioTitle: string;
	completedTasks: RetrospectiveTask[];
}

/**
 * RetrospectiveTask is a completed task entry in the retrospective view.
 */
export interface RetrospectiveTask {
	taskId: string;
	prompt: string;
	completedAt?: string;
}

// ============================================================================
// Helper Functions
// ============================================================================

/**
 * Get the CSS class name for an AgentLoop status.
 */
export function getAgentLoopStatusClass(status: AgentLoop['status']): string {
	switch (status) {
		case 'running':
			return 'warning';
		case 'completed':
			return 'success';
		case 'failed':
			return 'error';
	}
}

/**
 * Get the CSS class name for a DAGNode status.
 */
export function getDAGNodeStatusClass(status: DAGNode['status']): string {
	switch (status) {
		case 'pending':
			return 'neutral';
		case 'running':
			return 'warning';
		case 'completed':
			return 'success';
		case 'failed':
			return 'error';
	}
}

/**
 * Compute summary statistics for a retrospective.
 */
export function computeRetrospectiveStats(phases: RetrospectivePhase[]): {
	totalRequirements: number;
	totalScenarios: number;
	totalTasks: number;
} {
	const totalRequirements = phases.length;
	const totalScenarios = phases.reduce((sum, p) => sum + p.scenarios.length, 0);
	const totalTasks = phases.reduce(
		(sum, p) => sum + p.scenarios.reduce((s2, sc) => s2 + sc.completedTasks.length, 0),
		0
	);
	return { totalRequirements, totalScenarios, totalTasks };
}
