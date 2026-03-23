import { test, expect, waitForHydration } from './helpers/setup';
import { ExecutionPage } from './pages/ExecutionPage';
import type { AgentLoop, DAGExecution, RetrospectivePhase } from '../src/lib/types/execution';

// ============================================================================
// Shared mock data
// ============================================================================

const mockLoops: AgentLoop[] = [
	{
		loopId: 'loop-root-1',
		role: 'orchestrator',
		model: 'mock-planner',
		status: 'completed',
		depth: 0,
		children: [
			{
				loopId: 'loop-child-1',
				parentLoopId: 'loop-root-1',
				role: 'developer',
				model: 'mock-coder',
				status: 'completed',
				depth: 1,
				children: [],
				taskId: 'task-impl-1',
				startedAt: '2026-03-15T10:00:00Z',
				completedAt: '2026-03-15T10:05:00Z'
			},
			{
				loopId: 'loop-child-2',
				parentLoopId: 'loop-root-1',
				role: 'reviewer',
				model: 'mock-reviewer',
				status: 'running',
				depth: 1,
				children: [],
				startedAt: '2026-03-15T10:05:00Z'
			}
		],
		startedAt: '2026-03-15T10:00:00Z',
		completedAt: '2026-03-15T10:10:00Z'
	}
];

const mockDAG: DAGExecution = {
	executionId: 'exec-abc123',
	scenarioId: 'scenario-xyz',
	status: 'executing',
	nodes: [
		{
			id: 'node-1',
			prompt: 'Implement health check endpoint',
			role: 'developer',
			dependsOn: [],
			status: 'completed',
			loopId: 'loop-child-1'
		},
		{
			id: 'node-2',
			prompt: 'Add tests for health check',
			role: 'developer',
			dependsOn: ['node-1'],
			status: 'running',
			loopId: 'loop-child-2'
		},
		{
			id: 'node-3',
			prompt: 'Review implementation',
			role: 'reviewer',
			dependsOn: ['node-1', 'node-2'],
			status: 'pending'
		}
	]
};

const mockPhases: RetrospectivePhase[] = [
	{
		requirementId: 'req-1',
		requirementTitle: 'Health check endpoint returns status',
		scenarios: [
			{
				scenarioId: 'sc-1',
				scenarioTitle: 'Health check returns 200 when service is healthy',
				completedTasks: [
					{
						taskId: 'task-impl-1',
						prompt: 'Implement health check handler',
						completedAt: '2026-03-15T10:05:00Z'
					}
				]
			}
		]
	},
	{
		requirementId: 'req-2',
		requirementTitle: 'Monitoring integration',
		scenarios: [
			{
				scenarioId: 'sc-2',
				scenarioTitle: 'Metrics are exposed on /metrics endpoint',
				completedTasks: [
					{
						taskId: 'task-metrics-1',
						prompt: 'Add Prometheus metrics endpoint',
						completedAt: '2026-03-15T10:10:00Z'
					},
					{
						taskId: 'task-metrics-2',
						prompt: 'Add request duration histogram',
						completedAt: '2026-03-15T10:15:00Z'
					}
				]
			}
		]
	}
];

// ============================================================================
// Helpers
// ============================================================================

/**
 * Set up route mocks for the agent-tree API returning mockLoops.
 */
async function mockAgentTreeAPI(
	page: import('@playwright/test').Page,
	loops: AgentLoop[] = mockLoops
): Promise<void> {
	await page.route('**/plan-api/plans/test-plan/agent-tree', (route) => {
		route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify(loops)
		});
	});
}

/**
 * Set up route mocks for the DAG execution API.
 */
async function mockDAGExecutionAPI(
	page: import('@playwright/test').Page,
	execution: DAGExecution = mockDAG
): Promise<void> {
	await page.route('**/plan-api/executions/exec-test', (route) => {
		route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify(execution)
		});
	});
}

/**
 * Set up route mocks for the retrospective API.
 */
async function mockRetrospectiveAPI(
	page: import('@playwright/test').Page,
	phases: RetrospectivePhase[] = mockPhases
): Promise<void> {
	await page.route('**/plan-api/plans/test-plan/phases/retrospective', (route) => {
		route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify(phases)
		});
	});
}

/**
 * Mock the trajectory API so LoopDetail does not make real calls.
 * Returns an empty trajectory to keep the panel functional.
 */
async function mockTrajectoryAPI(page: import('@playwright/test').Page): Promise<void> {
	await page.route('**/trajectory-api/loops/*', (route) => {
		route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({
				loop_id: 'loop-root-1',
				trace_id: 'trace-test',
				entries: [],
				status: 'completed'
			})
		});
	});
}

// ============================================================================
// Tests
// ============================================================================

test.describe('Execution Components', () => {
	let executionPage: ExecutionPage;

	test.beforeEach(async ({ page }) => {
		executionPage = new ExecutionPage(page);
	});

	// --------------------------------------------------------------------------
	// AgentTree
	// --------------------------------------------------------------------------

	test.describe('AgentTree', () => {
		test('renders root node with correct role', async ({ page }) => {
			await mockAgentTreeAPI(page);
			await executionPage.goto('agent-tree');
			await waitForHydration(page);

			// Root orchestrator node should appear
			await expect(page.locator('.node-role').first()).toContainText('Orchestrator');
		});

		test('renders hierarchy with correct node count at root', async ({ page }) => {
			await mockAgentTreeAPI(page);
			await executionPage.goto('agent-tree');
			await waitForHydration(page);

			// Only root-level nodes in the outermost <ul> — running nodes expand by default
			// The orchestrator is 'completed', so its children start collapsed
			const rootList = page.locator('.harness-agent-tree > ul.tree-list');
			const rootNodes = rootList.locator(':scope > li.tree-node');
			await expect(rootNodes).toHaveCount(1);
		});

		test('shows completed status badge on finished loop', async ({ page }) => {
			await mockAgentTreeAPI(page);
			await executionPage.goto('agent-tree');
			await waitForHydration(page);

			const rootNode = page.locator('li.tree-node').first();
			const badge = rootNode.locator('.status-badge').first();
			await expect(badge).toContainText('completed');
		});

		test('shows expand button when node has children', async ({ page }) => {
			await mockAgentTreeAPI(page);
			await executionPage.goto('agent-tree');
			await waitForHydration(page);

			// Root node has children so its expand-btn should have .visible class
			const rootNode = page.locator('li.tree-node').first();
			await expect(rootNode.locator('.expand-btn.visible')).toBeVisible();
		});

		test('expand/collapse children works on click', async ({ page }) => {
			await mockAgentTreeAPI(page);
			await executionPage.goto('agent-tree');
			await waitForHydration(page);

			// Root is completed → children start collapsed; click expand btn
			const expandBtn = page.locator('.expand-btn.visible').first();
			await expandBtn.click();

			// After expanding, children container should appear
			await expect(page.locator('.children-container')).toBeVisible();

			// Child nodes should now be visible (developer + reviewer)
			const childNodes = page.locator('.children-container li.tree-node');
			await expect(childNodes).toHaveCount(2);
		});

		test('node click selects the loop and highlights it', async ({ page }) => {
			await mockAgentTreeAPI(page);
			await executionPage.goto('agent-tree');
			await waitForHydration(page);

			// Click the node-content button on the root node
			const rootContent = page.locator('li.tree-node .node-content').first();
			await rootContent.click();

			// The node-row should gain .selected class
			const rootRow = page.locator('li.tree-node .node-row').first();
			await expect(rootRow).toHaveClass(/selected/);
		});

		test('shows model name in node', async ({ page }) => {
			await mockAgentTreeAPI(page);
			await executionPage.goto('agent-tree');
			await waitForHydration(page);

			// The root loop's model is 'mock-planner' — shown as last segment after '/'
			await expect(page.locator('.node-model').first()).toContainText('mock-planner');
		});

		test('shows empty state when no agent loops', async ({ page }) => {
			await mockAgentTreeAPI(page, []);
			await executionPage.goto('agent-tree-empty');
			await waitForHydration(page);

			await expect(executionPage.treeEmptyState).toBeVisible();
			await expect(executionPage.treeEmptyState).toContainText('No agent loops');
		});

		test('running loop nodes start expanded by default', async ({ page }) => {
			// Provide a tree where the root is 'running' with a child
			const runningRoot: AgentLoop[] = [
				{
					loopId: 'loop-running-root',
					role: 'orchestrator',
					model: 'mock-planner',
					status: 'running',
					depth: 0,
					children: [
						{
							loopId: 'loop-running-child',
							parentLoopId: 'loop-running-root',
							role: 'developer',
							model: 'mock-coder',
							status: 'running',
							depth: 1,
							children: []
						}
					],
					startedAt: '2026-03-15T10:00:00Z'
				}
			];

			await mockAgentTreeAPI(page, runningRoot);
			await executionPage.goto('agent-tree');
			await waitForHydration(page);

			// Running root → children container should be visible without any click
			await expect(page.locator('.children-container')).toBeVisible();
		});
	});

	// --------------------------------------------------------------------------
	// DAGView
	// --------------------------------------------------------------------------

	test.describe('DAGView', () => {
		test('renders header with execution ID', async ({ page }) => {
			await mockDAGExecutionAPI(page);
			await executionPage.goto('dag-view');
			await waitForHydration(page);

			// Execution ID is truncated to 8 chars: 'exec-abc' → 'exec-abc' (exactly 8)
			await expect(executionPage.dagHeader).toBeVisible();
			await expect(page.locator('.execution-id')).toContainText('exec-abc');
		});

		test('shows execution status badge in header', async ({ page }) => {
			await mockDAGExecutionAPI(page);
			await executionPage.goto('dag-view');
			await waitForHydration(page);

			const badge = executionPage.dagHeader.locator('.status-badge');
			await expect(badge).toContainText('executing');
		});

		test('renders correct number of DAG nodes', async ({ page }) => {
			await mockDAGExecutionAPI(page);
			await executionPage.goto('dag-view');
			await waitForHydration(page);

			await expect(executionPage.dagNodes).toHaveCount(3);
		});

		test('renders nodes in topological layers', async ({ page }) => {
			await mockDAGExecutionAPI(page);
			await executionPage.goto('dag-view');
			await waitForHydration(page);

			// node-1 has no deps → layer 0; node-2 depends on node-1 → layer 1;
			// node-3 depends on node-1 + node-2 → layer 2
			// Each layer is a .dag-layer div
			const layers = page.locator('.dag-layer');
			await expect(layers).toHaveCount(3);
		});

		test('shows node prompt text', async ({ page }) => {
			await mockDAGExecutionAPI(page);
			await executionPage.goto('dag-view');
			await waitForHydration(page);

			await expect(executionPage.dagNodes.first()).toContainText(
				'Implement health check endpoint'
			);
		});

		test('shows node role labels', async ({ page }) => {
			await mockDAGExecutionAPI(page);
			await executionPage.goto('dag-view');
			await waitForHydration(page);

			// First node is role 'developer' → displayed as 'Developer'
			const roleLabel = executionPage.dagNodes.first().locator('.node-role');
			await expect(roleLabel).toContainText('Developer');
		});

		test('DAG node click selects node', async ({ page }) => {
			await mockDAGExecutionAPI(page);
			await executionPage.goto('dag-view');
			await waitForHydration(page);

			const firstNode = executionPage.dagNodes.first();
			await firstNode.click();

			// Selected node gains .selected class and aria-pressed=true
			await expect(firstNode).toHaveClass(/selected/);
			await expect(firstNode).toHaveAttribute('aria-pressed', 'true');
		});

		test('shows status summary bar with counts', async ({ page }) => {
			await mockDAGExecutionAPI(page);
			await executionPage.goto('dag-view');
			await waitForHydration(page);

			await expect(executionPage.dagSummary).toBeVisible();
			// 1 completed, 1 running, 1 pending
			await expect(executionPage.dagSummary).toContainText('1 completed');
			await expect(executionPage.dagSummary).toContainText('1 running');
			await expect(executionPage.dagSummary).toContainText('1 pending');
		});

		test('shows empty state when DAG has no nodes', async ({ page }) => {
			const emptyDAG: DAGExecution = {
				executionId: 'exec-empty',
				scenarioId: 'scenario-empty',
				status: 'executing',
				nodes: []
			};
			await mockDAGExecutionAPI(page, emptyDAG);
			await executionPage.goto('dag-view-empty');
			await waitForHydration(page);

			await expect(executionPage.dagEmptyState).toBeVisible();
			await expect(executionPage.dagEmptyState).toContainText('No execution nodes');
		});

		test('applies status-colored border class to nodes', async ({ page }) => {
			await mockDAGExecutionAPI(page);
			await executionPage.goto('dag-view');
			await waitForHydration(page);

			// node-1 is 'completed' → status-success class
			const completedNode = executionPage.dagNodes.first();
			await expect(completedNode).toHaveClass(/status-success/);

			// node-2 is 'running' → status-warning class
			const runningNode = executionPage.dagNodes.nth(1);
			await expect(runningNode).toHaveClass(/status-warning/);

			// node-3 is 'pending' → status-neutral/status-pending class
			const pendingNode = executionPage.dagNodes.nth(2);
			await expect(pendingNode).toHaveClass(/status-neutral|status-pending/);
		});
	});

	// --------------------------------------------------------------------------
	// LoopDetail
	// --------------------------------------------------------------------------

	test.describe('LoopDetail', () => {
		test('detail panel is visible when loop is selected', async ({ page }) => {
			await mockAgentTreeAPI(page);
			await mockTrajectoryAPI(page);
			await executionPage.goto('loop-detail');
			await waitForHydration(page);

			await expect(executionPage.loopDetailPanel).toBeVisible();
		});

		test('shows correct role in panel title', async ({ page }) => {
			await mockAgentTreeAPI(page);
			await mockTrajectoryAPI(page);
			await executionPage.goto('loop-detail');
			await waitForHydration(page);

			const title = await executionPage.getDetailRole();
			expect(title).toBe('Orchestrator');
		});

		test('shows status badge in panel header', async ({ page }) => {
			await mockAgentTreeAPI(page);
			await mockTrajectoryAPI(page);
			await executionPage.goto('loop-detail');
			await waitForHydration(page);

			const status = await executionPage.getDetailStatus();
			expect(status).toBe('completed');
		});

		test('displays loop metadata in meta-grid', async ({ page }) => {
			await mockAgentTreeAPI(page);
			await mockTrajectoryAPI(page);
			await executionPage.goto('loop-detail');
			await waitForHydration(page);

			await expect(executionPage.metaGrid).toBeVisible();

			// Loop ID row should contain the actual loop ID value
			const loopIdValue = await executionPage.getDetailMetaValue('Loop ID');
			expect(loopIdValue).toContain('loop-root-1');

			// Model row should show the model name
			const modelValue = await executionPage.getDetailMetaValue('Model');
			expect(modelValue).toBe('mock-planner');

			// Depth row should show depth 0
			const depthValue = await executionPage.getDetailMetaValue('Depth');
			expect(depthValue).toBe('0');
		});

		test('closes panel on close button click', async ({ page }) => {
			await mockAgentTreeAPI(page);
			await mockTrajectoryAPI(page);
			await executionPage.goto('loop-detail');
			await waitForHydration(page);

			await expect(executionPage.loopDetailPanel).toBeVisible();
			await executionPage.closeDetailPanel();

			await expect(executionPage.loopDetailPanel).not.toBeVisible();
		});

		test('closes panel on Escape key', async ({ page }) => {
			await mockAgentTreeAPI(page);
			await mockTrajectoryAPI(page);
			await executionPage.goto('loop-detail');
			await waitForHydration(page);

			await expect(executionPage.loopDetailPanel).toBeVisible();
			await page.keyboard.press('Escape');

			await expect(executionPage.loopDetailPanel).not.toBeVisible();
		});

		test('closes panel on backdrop click', async ({ page }) => {
			await mockAgentTreeAPI(page);
			await mockTrajectoryAPI(page);
			await executionPage.goto('loop-detail');
			await waitForHydration(page);

			await expect(executionPage.loopDetailPanel).toBeVisible();
			await executionPage.loopDetailBackdrop.click({ position: { x: 10, y: 10 } });

			await expect(executionPage.loopDetailPanel).not.toBeVisible();
		});

		test('shows spawned children section when loop has children', async ({ page }) => {
			await mockAgentTreeAPI(page);
			await mockTrajectoryAPI(page);
			await executionPage.goto('loop-detail');
			await waitForHydration(page);

			// Root loop has 2 children → .children-section should appear
			const childrenSection = page.locator('.children-section');
			await expect(childrenSection).toBeVisible();
			await expect(childrenSection).toContainText('Spawned Loops (2)');
		});

		test('shows empty trajectory state when no entries', async ({ page }) => {
			await mockAgentTreeAPI(page);
			await mockTrajectoryAPI(page);
			await executionPage.goto('loop-detail');
			await waitForHydration(page);

			// Trajectory returns empty entries → empty-state inside panel body
			const emptyState = executionPage.loopDetailPanel.locator('.empty-state');
			await expect(emptyState).toBeVisible();
			await expect(emptyState).toContainText('No trajectory data');
		});
	});

	// --------------------------------------------------------------------------
	// RetrospectiveView
	// --------------------------------------------------------------------------

	test.describe('RetrospectiveView', () => {
		test('renders stats bar with correct counts', async ({ page }) => {
			await mockRetrospectiveAPI(page);
			await executionPage.goto('retro-view');
			await waitForHydration(page);

			const stats = await executionPage.getRetroStats();
			expect(stats.requirements).toBe('2');
			expect(stats.scenarios).toBe('2');
			expect(stats.tasks).toBe('3');
		});

		test('renders correct number of requirement headers', async ({ page }) => {
			await mockRetrospectiveAPI(page);
			await executionPage.goto('retro-view');
			await waitForHydration(page);

			const count = await executionPage.getRequirementCount();
			expect(count).toBe(2);
		});

		test('first requirement is expanded by default on mount', async ({ page }) => {
			await mockRetrospectiveAPI(page);
			await executionPage.goto('retro-view');
			await waitForHydration(page);

			// onMount expands the first requirement
			const firstHeader = executionPage.requirementHeaders.first();
			await expect(firstHeader).toHaveAttribute('aria-expanded', 'true');
		});

		test('second requirement starts collapsed', async ({ page }) => {
			await mockRetrospectiveAPI(page);
			await executionPage.goto('retro-view');
			await waitForHydration(page);

			const secondHeader = executionPage.requirementHeaders.nth(1);
			await expect(secondHeader).toHaveAttribute('aria-expanded', 'false');
		});

		test('clicking requirement header toggles expansion', async ({ page }) => {
			await mockRetrospectiveAPI(page);
			await executionPage.goto('retro-view');
			await waitForHydration(page);

			// First requirement starts expanded; clicking should collapse it
			const firstHeader = executionPage.requirementHeaders.first();
			await firstHeader.click();
			await expect(firstHeader).toHaveAttribute('aria-expanded', 'false');

			// Click again to re-expand
			await firstHeader.click();
			await expect(firstHeader).toHaveAttribute('aria-expanded', 'true');
		});

		test('requirement title is rendered in header', async ({ page }) => {
			await mockRetrospectiveAPI(page);
			await executionPage.goto('retro-view');
			await waitForHydration(page);

			const firstHeader = executionPage.requirementHeaders.first();
			await expect(firstHeader.locator('.req-title')).toContainText(
				'Health check endpoint returns status'
			);
		});

		test('expanded requirement shows scenario headers', async ({ page }) => {
			await mockRetrospectiveAPI(page);
			await executionPage.goto('retro-view');
			await waitForHydration(page);

			// First requirement is auto-expanded; its scenario should be visible
			const scenarioHeader = page.locator('button.scenario-header').first();
			await expect(scenarioHeader).toBeVisible();
			await expect(scenarioHeader).toContainText(
				'Health check returns 200 when service is healthy'
			);
		});

		test('expanding scenario shows task items', async ({ page }) => {
			await mockRetrospectiveAPI(page);
			await executionPage.goto('retro-view');
			await waitForHydration(page);

			// First requirement is auto-expanded; click its scenario to expand tasks
			const scenarioHeader = page.locator('button.scenario-header').first();
			await scenarioHeader.click();

			// Task items should now be visible
			const taskItems = page.locator('.task-item');
			await expect(taskItems).toHaveCount(1);
			await expect(taskItems.first()).toContainText('Implement health check handler');
		});

		test('requirement meta badges show scenario and task counts', async ({ page }) => {
			await mockRetrospectiveAPI(page);
			await executionPage.goto('retro-view');
			await waitForHydration(page);

			const firstHeader = executionPage.requirementHeaders.first();
			// req-1 has 1 scenario and 1 task
			await expect(firstHeader).toContainText('1 scenario');
			await expect(firstHeader).toContainText('1 task');
		});

		test('shows empty state when no phases', async ({ page }) => {
			await mockRetrospectiveAPI(page, []);
			await executionPage.goto('retro-view-empty');
			await waitForHydration(page);

			await expect(executionPage.retroEmptyState).toBeVisible();
			await expect(executionPage.retroEmptyState).toContainText('No completed work yet');
		});

		test('empty state still shows stats bar with zero counts', async ({ page }) => {
			await mockRetrospectiveAPI(page, []);
			await executionPage.goto('retro-view-empty');
			await waitForHydration(page);

			// Stats bar is always rendered; zeros when no phases
			await expect(executionPage.retroStatsBar).toBeVisible();
			const stats = await executionPage.getRetroStats();
			expect(stats.requirements).toBe('0');
			expect(stats.scenarios).toBe('0');
			expect(stats.tasks).toBe('0');
		});
	});
});
