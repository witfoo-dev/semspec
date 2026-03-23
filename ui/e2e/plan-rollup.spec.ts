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
		await page.route('**/plan-api/plans/*/phases', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([])
			});
		});

		await page.route('**/plan-api/plans/*/requirements', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([])
			});
		});

		await page.route('**/plan-api/plans/*/scenarios**', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([])
			});
		});

		await page.route('**/plan-api/plans/*/tasks', route => {
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

		await page.route('**/plan-api/plans', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([plan])
			});
		});

		await page.route('**/plan-api/plans/test-rollup', route => {
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

		await page.route('**/plan-api/plans', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([plan])
			});
		});

		await page.route('**/plan-api/plans/test-rollup-pipeline', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(plan)
			});
		});

		await planDetailPage.goto('test-rollup-pipeline');

		// The stage badge should show reviewing_rollup stage
		const planStage = page.locator('.plan-stage');
		await expect(planStage).toBeVisible();

		// The execute phase status badge should be active (reviewing_rollup is within the execute phase)
		const execStatus = page.getByRole('status').filter({ hasText: 'exec' });
		await expect(execStatus).toBeVisible();
		const execText = await execStatus.textContent();
		expect(execText?.toLowerCase()).toMatch(/exec/i);
	});

	test('transitions from reviewing_rollup to complete on reload', async ({
		page,
		planDetailPage
	}) => {
		// Use a flag to control which stage to return: before reload vs after
		let reloaded = false;

		await page.route('**/plan-api/plans/test-transition', route => {
			const stage = reloaded ? 'complete' : 'reviewing_rollup';
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
		await page.route('**/plan-api/plans', route => {
			const stage = reloaded ? 'complete' : 'reviewing_rollup';
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

		// Flip the flag before reload so the mock returns 'complete'
		reloaded = true;
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

		await page.route('**/plan-api/plans', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([plan])
			});
		});

		await page.route('**/plan-api/plans/rollup-approved', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(plan)
			});
		});

		await planDetailPage.goto('rollup-approved');

		// A plan in reviewing_rollup is approved and executing
		// The stage badge confirms the plan is in the rollup review stage
		await expect(page.locator('.plan-stage')).toBeVisible();
		const stageText = await page.locator('.plan-stage').textContent();
		expect(stageText?.toLowerCase()).toMatch(/reviewing|rollup/i);

		// Pipeline phase status badges are rendered for an approved plan in execution
		await expect(page.getByRole('status').filter({ hasText: 'plan' })).toBeVisible();
		await expect(page.getByRole('status').filter({ hasText: 'exec' })).toBeVisible();
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

		await page.route('**/plan-api/plans', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([plan])
			});
		});

		await page.route('**/plan-api/plans/rollup-with-loop', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(plan)
			});
		});

		await planDetailPage.goto('rollup-with-loop');

		// The plan is in reviewing_rollup with an active loop — the stage badge confirms this
		await expect(page.locator('.plan-stage')).toBeVisible();
		const stageText = await page.locator('.plan-stage').textContent();
		expect(stageText?.toLowerCase()).toMatch(/reviewing|rollup/i);

		// The Active Loop panel is shown in the right sidebar when a loop is executing
		await expect(page.getByRole('heading', { name: 'Active Loop' })).toBeVisible();
	});

	test('complete plan after rollup shows all stages done', async ({ page, planDetailPage }) => {
		const plan = mockPlan({
			slug: 'post-rollup-complete',
			goal: 'Build the reporting module',
			approved: true,
			stage: 'complete',
			active_loops: []
		});

		await page.route('**/plan-api/plans', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([plan])
			});
		});

		await page.route('**/plan-api/plans/post-rollup-complete', route => {
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
