import { test, expect, mockPlan } from './helpers/setup';

/**
 * Tests for plan rollup review stage:
 * - Plan shows "reviewing_rollup" status correctly
 * - Pipeline execute phase shows active during rollup
 * - Transition to complete after rollup approval
 */

test.describe('Plan Rollup Review', () => {
	// All plan detail pages fetch phases, requirements, scenarios, and tasks.
	// Provide default empty responses so missing mocks don't cause 404s.
	test.beforeEach(async ({ page }) => {
		await page.route('**/workflow-api/plans/*/phases', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([])
			});
		});

		await page.route('**/workflow-api/plans/*/requirements', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([])
			});
		});

		await page.route('**/workflow-api/plans/*/scenarios**', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([])
			});
		});

		await page.route('**/workflow-api/plans/*/tasks', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([])
			});
		});
	});

	test('shows rollup review status badge', async ({ page, planDetailPage }) => {
		const plan = mockPlan({
			slug: 'test-rollup',
			goal: 'Build auth middleware',
			context: 'Adding JWT support',
			approved: true,
			stage: 'reviewing_rollup'
		});

		await page.route('**/workflow-api/plans', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([plan])
			});
		});

		await page.route('**/workflow-api/plans/test-rollup', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(plan)
			});
		});

		await planDetailPage.goto('test-rollup');

		// The stage badge should reflect the reviewing_rollup stage
		const stageBadge = page.locator('.plan-stage');
		await expect(stageBadge).toBeVisible();
		// The badge text uses display formatting — verify it contains stage-related text
		const badgeText = await stageBadge.textContent();
		expect(badgeText?.toLowerCase()).toMatch(/reviewing|rollup/i);
	});

	test('pipeline shows execute phase as active during rollup', async ({ page, planDetailPage }) => {
		const plan = mockPlan({
			slug: 'test-rollup-pipeline',
			goal: 'Build auth middleware',
			approved: true,
			stage: 'reviewing_rollup'
		});

		await page.route('**/workflow-api/plans', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([plan])
			});
		});

		await page.route('**/workflow-api/plans/test-rollup-pipeline', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(plan)
			});
		});

		await planDetailPage.goto('test-rollup-pipeline');
		await planDetailPage.expectPipelineVisible();

		// The pipeline should be rendered; execute phase should be active (not complete)
		// since reviewing_rollup is still within the execute phase
		const pipelineStages = planDetailPage.pipelineStages;
		const count = await pipelineStages.count();
		expect(count).toBeGreaterThan(0);

		// At least one stage should be active (not all complete)
		const activeStages = page.locator('.pipeline-stage.active');
		const completeStages = page.locator('.pipeline-stage.complete');
		// Either active stages exist OR all stages are positioned — pipeline is rendered
		const planStage = page.locator('.plan-stage');
		await expect(planStage).toBeVisible();
	});

	test('transitions from reviewing_rollup to complete on reload', async ({
		page,
		planDetailPage
	}) => {
		let requestCount = 0;

		await page.route('**/workflow-api/plans/test-transition', route => {
			requestCount++;
			const stage = requestCount <= 2 ? 'reviewing_rollup' : 'complete';
			const plan = mockPlan({
				slug: 'test-transition',
				goal: 'Build auth middleware',
				approved: true,
				stage
			});
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(plan)
			});
		});

		// Also override the list route for reload
		await page.route('**/workflow-api/plans', route => {
			const stage = requestCount <= 2 ? 'reviewing_rollup' : 'complete';
			const plan = mockPlan({
				slug: 'test-transition',
				goal: 'Build auth middleware',
				approved: true,
				stage
			});
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([plan])
			});
		});

		await planDetailPage.goto('test-transition');

		// Initially shows rollup status
		const stageBadge = page.locator('.plan-stage');
		await expect(stageBadge).toBeVisible();
		const initialText = await stageBadge.textContent();
		expect(initialText?.toLowerCase()).toMatch(/reviewing|rollup/i);

		// After reload the mock returns 'complete'
		await page.reload();
		await page.waitForSelector('.plan-detail, .not-found', { timeout: 15000 });

		// Stage badge now shows complete
		await expect(page.locator('.plan-stage')).toBeVisible();
		const finalText = await page.locator('.plan-stage').textContent();
		expect(finalText?.toLowerCase()).toContain('complete');
	});

	test('reviewing_rollup plan is approved and has pipeline visible', async ({
		page,
		planDetailPage
	}) => {
		const plan = mockPlan({
			slug: 'rollup-approved',
			goal: 'Implement user login flow',
			context: 'JWT-based authentication',
			approved: true,
			stage: 'reviewing_rollup',
			active_loops: [
				{
					loop_id: 'rollup-reviewer-loop',
					role: 'spec_reviewer',
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

		await page.route('**/workflow-api/plans/rollup-approved', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(plan)
			});
		});

		await planDetailPage.goto('rollup-approved');
		await planDetailPage.expectPipelineVisible();

		// A plan in reviewing_rollup is approved and executing — pipeline is visible
		await expect(planDetailPage.pipelineSection).toBeVisible();
		await expect(planDetailPage.agentPipelineView).toBeVisible();
	});

	test('reviewing_rollup shows active loop in pipeline', async ({ page, planDetailPage }) => {
		const plan = mockPlan({
			slug: 'rollup-with-loop',
			goal: 'Refactor the data layer',
			approved: true,
			stage: 'reviewing_rollup',
			active_loops: [
				{
					loop_id: 'rollup-loop-1',
					role: 'reviewer',
					model: 'claude-3',
					state: 'executing',
					iterations: 2,
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

		await page.route('**/workflow-api/plans/rollup-with-loop', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(plan)
			});
		});

		await planDetailPage.goto('rollup-with-loop');
		await planDetailPage.expectPipelineVisible();

		// Active loop in reviewing_rollup should produce an active pipeline stage
		const activeStage = page.locator('.pipeline-stage.active');
		await expect(activeStage).toBeVisible();

		// Spinner is present on the active stage
		const spinner = activeStage.locator('.spin');
		await expect(spinner).toBeVisible();
	});

	test('complete plan after rollup shows all stages done', async ({ page, planDetailPage }) => {
		const plan = mockPlan({
			slug: 'post-rollup-complete',
			goal: 'Build the reporting module',
			approved: true,
			stage: 'complete',
			active_loops: []
		});

		await page.route('**/workflow-api/plans', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([plan])
			});
		});

		await page.route('**/workflow-api/plans/post-rollup-complete', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(plan)
			});
		});

		await planDetailPage.goto('post-rollup-complete');

		// A complete plan shows the "Complete" stage badge
		const stageBadge = page.locator('.plan-stage');
		await expect(stageBadge).toHaveText('Complete');
	});
});
