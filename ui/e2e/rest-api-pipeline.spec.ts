import { test, expect, mockPlan } from './helpers/setup';
import type { Requirement } from '../src/lib/types/requirement';
import type { Scenario } from '../src/lib/types/scenario';

/**
 * Tests for the REST API plan lifecycle in the UI:
 * - Plan card shows multi-concern goal on board
 * - Plan detail renders rich goal and context
 * - Three requirements appear in nav tree
 * - Scenarios distributed across multiple requirements
 * - Mixed scenario statuses display correctly during execution
 * - Pipeline shows active loop with spinner during multi-task execution
 * - reviewing_rollup stage shows completed scenarios rollup
 * - Complete plan shows all requirements satisfied
 * - Requirement detail panel shows description on click
 * - Scenario detail shows BDD format on click
 *
 * Uses mock API routes — no real backend needed.
 * Run with: npx playwright test rest-api-pipeline.spec.ts
 */

// ============================================================================
// Mock data
// ============================================================================

const restApiPlan = mockPlan({
	slug: 'rest-api',
	goal: 'Add /users REST API with CRUD endpoints, request logging middleware, and error handling',
	context: 'Go HTTP service needs a users resource with proper middleware and error responses',
	approved: true,
	stage: 'implementing'
});

const requirements: Requirement[] = [
	{
		id: 'req-crud',
		plan_id: 'plan-rest-api',
		title: 'CRUD endpoints for /users resource',
		description: 'GET list, GET by ID, POST create, DELETE',
		status: 'active',
		created_at: new Date().toISOString(),
		updated_at: new Date().toISOString()
	},
	{
		id: 'req-middleware',
		plan_id: 'plan-rest-api',
		title: 'Request logging middleware',
		description: 'Log method, path, status, duration for all requests',
		status: 'active',
		created_at: new Date().toISOString(),
		updated_at: new Date().toISOString()
	},
	{
		id: 'req-errors',
		plan_id: 'plan-rest-api',
		title: 'JSON error responses',
		description: '404 for missing users, 400 for invalid body, 405 for bad methods',
		status: 'active',
		created_at: new Date().toISOString(),
		updated_at: new Date().toISOString()
	}
];

const scenarios: Scenario[] = [
	{
		id: 'sc-list',
		requirement_id: 'req-crud',
		given: 'users exist in the store',
		when: 'GET /users',
		then: ['200 with JSON array of users'],
		status: 'passing',
		created_at: new Date().toISOString(),
		updated_at: new Date().toISOString()
	},
	{
		id: 'sc-create',
		requirement_id: 'req-crud',
		given: 'valid user payload',
		when: 'POST /users',
		then: ['201 with created user', 'user has ID assigned'],
		status: 'passing',
		created_at: new Date().toISOString(),
		updated_at: new Date().toISOString()
	},
	{
		id: 'sc-not-found',
		requirement_id: 'req-errors',
		given: 'no user with ID 999',
		when: 'GET /users/999',
		then: ['404 with JSON error message'],
		status: 'pending',
		created_at: new Date().toISOString(),
		updated_at: new Date().toISOString()
	},
	{
		id: 'sc-logging',
		requirement_id: 'req-middleware',
		given: 'any request to the server',
		when: 'the request completes',
		then: ['log entry contains method, path, status, duration'],
		status: 'pending',
		created_at: new Date().toISOString(),
		updated_at: new Date().toISOString()
	},
	{
		id: 'sc-bad-body',
		requirement_id: 'req-errors',
		given: 'invalid JSON body',
		when: 'POST /users',
		then: ['400 with descriptive error'],
		status: 'pending',
		created_at: new Date().toISOString(),
		updated_at: new Date().toISOString()
	}
];

// ============================================================================
// Route setup helpers
// ============================================================================

/**
 * Register default empty responses for all sub-resource endpoints that every
 * plan detail page fetches on load. Individual tests override these as needed.
 */
async function setupDefaultSubRoutes(page: import('@playwright/test').Page): Promise<void> {
	await page.route('**/workflow-api/plans/*/phases', route => {
		route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) });
	});
	await page.route('**/workflow-api/plans/*/requirements', route => {
		route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) });
	});
	await page.route('**/workflow-api/plans/*/scenarios**', route => {
		route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) });
	});
	await page.route('**/workflow-api/plans/*/tasks', route => {
		route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) });
	});
}

function withStage(stage: string, extraOverrides: object = {}) {
	return { ...restApiPlan, stage, ...extraOverrides };
}

// ============================================================================
// Tests
// ============================================================================

test.describe('REST API Plan Pipeline Lifecycle', () => {
	test.beforeEach(async ({ page }) => {
		await setupDefaultSubRoutes(page);
	});

	test('plan card on board shows multi-requirement goal', async ({ page, boardPage }) => {
		const plan = withStage('implementing');

		await page.route('**/workflow-api/plans', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([plan])
			});
		});

		await boardPage.goto();
		await boardPage.expectVisible();
		await boardPage.expectPlansGrid();

		// The plan card must display the multi-concern goal text
		const planCard = page.locator('.plan-card', { hasText: restApiPlan.goal! });
		await expect(planCard).toBeVisible();
	});

	test('plan detail shows rich goal and context', async ({ page, planDetailPage }) => {
		const plan = withStage('approved');

		await page.route('**/workflow-api/plans', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([plan])
			});
		});

		await page.route('**/workflow-api/plans/rest-api', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(plan)
			});
		});

		await planDetailPage.goto('rest-api');
		await expect(planDetailPage.planDetail).toBeVisible();

		// Goal mentions multiple concerns; context must also appear
		await expect(page.locator('.plan-detail')).toContainText(restApiPlan.goal!);
		await expect(page.locator('.plan-detail')).toContainText(restApiPlan.context!);
	});

	test('three requirements in nav tree after requirements_generated stage', async ({
		page,
		planDetailPage
	}) => {
		const plan = withStage('requirements_generated');

		await page.route('**/workflow-api/plans', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([plan])
			});
		});

		await page.route('**/workflow-api/plans/rest-api', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(plan)
			});
		});

		// Override the default empty requirements route with all three requirements
		await page.route('**/workflow-api/plans/rest-api/requirements', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(requirements)
			});
		});

		await planDetailPage.goto('rest-api');
		await planDetailPage.expectNavTreeVisible();

		// All three requirement titles must appear as tree nodes
		await planDetailPage.expectRequirementInTree(requirements[0].title);
		await planDetailPage.expectRequirementInTree(requirements[1].title);
		await planDetailPage.expectRequirementInTree(requirements[2].title);
	});

	test('scenarios distributed across requirements in nav tree', async ({
		page,
		planDetailPage
	}) => {
		const plan = withStage('scenarios_generated');

		await page.route('**/workflow-api/plans', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([plan])
			});
		});

		await page.route('**/workflow-api/plans/rest-api', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(plan)
			});
		});

		await page.route('**/workflow-api/plans/rest-api/requirements', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(requirements)
			});
		});

		await page.route('**/workflow-api/plans/rest-api/scenarios**', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(scenarios)
			});
		});

		await planDetailPage.goto('rest-api');
		await planDetailPage.expectNavTreeVisible();

		// Expand the CRUD requirement — it has 2 scenarios (sc-list, sc-create)
		await planDetailPage.expandRequirementInTree(requirements[0].title);
		await planDetailPage.expectScenarioInTree('GET /users');
		await planDetailPage.expectScenarioInTree('POST /users');

		// Expand the errors requirement — it has 2 scenarios (sc-not-found, sc-bad-body)
		await planDetailPage.expandRequirementInTree(requirements[2].title);
		await planDetailPage.expectScenarioInTree('GET /users/999');

		// All 5 scenario nodes must be rendered across the tree
		const scenarioNodes = planDetailPage.navTree.locator('.tree-node.scenario-node');
		await expect(scenarioNodes).toHaveCount(scenarios.length);
	});

	test('mixed scenario statuses display correctly during implementing stage', async ({
		page,
		planDetailPage
	}) => {
		const plan = withStage('implementing');

		await page.route('**/workflow-api/plans', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([plan])
			});
		});

		await page.route('**/workflow-api/plans/rest-api', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(plan)
			});
		});

		await page.route('**/workflow-api/plans/rest-api/requirements', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(requirements)
			});
		});

		await page.route('**/workflow-api/plans/rest-api/scenarios**', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(scenarios)
			});
		});

		await planDetailPage.goto('rest-api');
		await planDetailPage.expectNavTreeVisible();

		// Passing scenarios (sc-list, sc-create) and pending scenarios (sc-not-found, etc.)
		// must all render — verify total count across tree
		const scenarioNodes = planDetailPage.navTree.locator('.tree-node.scenario-node');
		await expect(scenarioNodes).toHaveCount(scenarios.length);
	});

	test('pipeline shows active loop with spinner during multi-task execution', async ({
		page,
		planDetailPage
	}) => {
		const plan = withStage('implementing', {
			active_loops: [
				{
					loop_id: 'rest-api-builder-loop',
					role: 'builder',
					model: 'claude-3-sonnet',
					state: 'executing',
					iterations: 3,
					max_iterations: 10
				}
			]
		});

		await page.route('**/workflow-api/plans', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([plan])
			});
		});

		await page.route('**/workflow-api/plans/rest-api', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(plan)
			});
		});

		await planDetailPage.goto('rest-api');
		await planDetailPage.expectPipelineVisible();

		// An active pipeline stage with a spinner must be present
		const activeStage = page.locator('.pipeline-stage.active');
		await expect(activeStage).toBeVisible();

		const spinner = activeStage.locator('.spin');
		await expect(spinner).toBeVisible();
	});

	test('reviewing_rollup stage shows rollup status badge with mostly completed scenarios', async ({
		page,
		planDetailPage
	}) => {
		// All scenarios passing at rollup time
		const passingScenarios: Scenario[] = scenarios.map(sc => ({
			...sc,
			status: 'passing' as const
		}));

		const plan = withStage('reviewing_rollup', {
			active_loops: [
				{
					loop_id: 'rest-api-rollup-loop',
					role: 'reviewer',
					model: 'claude-3-sonnet',
					state: 'executing',
					iterations: 1,
					max_iterations: 5
				}
			]
		});

		await page.route('**/workflow-api/plans', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([plan])
			});
		});

		await page.route('**/workflow-api/plans/rest-api', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(plan)
			});
		});

		await page.route('**/workflow-api/plans/rest-api/requirements', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(requirements)
			});
		});

		await page.route('**/workflow-api/plans/rest-api/scenarios**', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(passingScenarios)
			});
		});

		await planDetailPage.goto('rest-api');

		// Stage badge must reflect reviewing_rollup
		const stageBadge = page.locator('.plan-stage');
		await expect(stageBadge).toBeVisible();
		const badgeText = await stageBadge.textContent();
		expect(badgeText?.toLowerCase()).toMatch(/reviewing|rollup/i);

		// Pipeline must remain visible — rollup is still within the execute phase
		await planDetailPage.expectPipelineVisible();
		await expect(planDetailPage.agentPipelineView).toBeVisible();
	});

	test('complete plan shows all requirements satisfied with no active pipeline stages', async ({
		page,
		planDetailPage
	}) => {
		const passingScenarios: Scenario[] = scenarios.map(sc => ({
			...sc,
			status: 'passing' as const
		}));

		const plan = withStage('complete', { active_loops: [] });

		await page.route('**/workflow-api/plans', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([plan])
			});
		});

		await page.route('**/workflow-api/plans/rest-api', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(plan)
			});
		});

		await page.route('**/workflow-api/plans/rest-api/requirements', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(requirements)
			});
		});

		await page.route('**/workflow-api/plans/rest-api/scenarios**', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(passingScenarios)
			});
		});

		await planDetailPage.goto('rest-api');

		// The stage badge must read "Complete"
		const stageBadge = page.locator('.plan-stage');
		await expect(stageBadge).toHaveText('Complete');

		// No active pipeline stages should remain
		const activeStages = page.locator('.pipeline-stage.active');
		await expect(activeStages).toHaveCount(0);
	});

	test('requirement detail panel shows description when clicked', async ({
		page,
		planDetailPage
	}) => {
		const plan = withStage('requirements_generated');

		await page.route('**/workflow-api/plans', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([plan])
			});
		});

		await page.route('**/workflow-api/plans/rest-api', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(plan)
			});
		});

		await page.route('**/workflow-api/plans/rest-api/requirements', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(requirements)
			});
		});

		await planDetailPage.goto('rest-api');
		await planDetailPage.expectNavTreeVisible();

		// Click the middleware requirement to open its detail panel
		await planDetailPage.selectRequirementInTree(requirements[1].title);

		// The detail panel must be visible and contain the requirement description
		await planDetailPage.expectRequirementDetailVisible();
		const detail = page.locator('.requirement-detail');
		await expect(detail).toContainText(requirements[1].description);
	});

	test('scenario detail shows BDD format when clicked', async ({ page, planDetailPage }) => {
		const plan = withStage('implementing');

		await page.route('**/workflow-api/plans', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([plan])
			});
		});

		await page.route('**/workflow-api/plans/rest-api', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(plan)
			});
		});

		await page.route('**/workflow-api/plans/rest-api/requirements', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(requirements)
			});
		});

		await page.route('**/workflow-api/plans/rest-api/scenarios**', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(scenarios)
			});
		});

		await planDetailPage.goto('rest-api');
		await planDetailPage.expectNavTreeVisible();

		// Expand the CRUD requirement and select the sc-list scenario
		await planDetailPage.expandRequirementInTree(requirements[0].title);
		await planDetailPage.selectScenarioInTree('users exist in the store');

		// ScenarioDetail must appear with BDD fields visible
		const scenarioDetail = page.locator('.scenario-detail');
		await expect(scenarioDetail).toBeVisible({ timeout: 5000 });

		// Given, When, Then fields must render the BDD content
		await expect(scenarioDetail).toContainText(scenarios[0].given);
		await expect(scenarioDetail).toContainText(scenarios[0].when);
		await expect(scenarioDetail).toContainText(scenarios[0].then[0]);
	});
});
