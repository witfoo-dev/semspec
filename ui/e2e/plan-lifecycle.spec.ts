import { test, expect } from '@playwright/test';
import { waitForHydration } from './helpers/hydration';
import { createPlan, deletePlan, getPlan } from './helpers/api';
import { startExecutionButton, planListItem } from './helpers/selectors';

/**
 * Full plan lifecycle: create → approve → cascade → execute → complete.
 * Requires mock LLM (hello-world scenario).
 *
 * Serial: each step depends on the previous, and mock LLM fixtures are consumed sequentially.
 * Plan created via API to keep focus on the UI flow being tested.
 */
test.describe('@mock @happy-path plan-lifecycle', () => {
	let slug: string;

	test.describe.configure({ mode: 'serial' });

	test.beforeAll(async () => {
		const plan = await createPlan(`Lifecycle test ${Date.now()}`);
		slug = plan.slug;
	});

	test.afterAll(async () => {
		if (slug) await deletePlan(slug).catch(() => {});
	});

	test('plan detail shows approve button', async ({ page }) => {
		await page.goto(`/plans/${slug}`);
		await waitForHydration(page);

		await expect(page.getByRole('button', { name: /Approve Plan/i }).first()).toBeVisible();
	});

	test('approve triggers cascade and reaches ready state', async ({ page }) => {
		await page.goto(`/plans/${slug}`);
		await waitForHydration(page);

		await page.getByRole('button', { name: /Approve Plan/i }).first().click();

		await expect(page.getByRole('button', { name: /Approve Plan/i })).not.toBeVisible({
			timeout: 10000
		});

		// Wait for cascade to complete — "Start Execution" appears
		await expect(startExecutionButton(page)).toBeVisible({ timeout: 60000 });
	});

	test('execute plan triggers execution pipeline', async ({ page }) => {
		// Wait for backend to reach ready_for_execution (UI may show button before stage settles)
		const preStart = Date.now();
		let plan = await getPlan(slug);
		while (plan.stage !== 'ready_for_execution' && Date.now() - preStart < 30000) {
			await new Promise((r) => setTimeout(r, 1000));
			plan = await getPlan(slug);
		}
		expect(plan.stage).toBe('ready_for_execution');

		await page.goto(`/plans/${slug}`);
		await waitForHydration(page);

		await expect(startExecutionButton(page)).toBeVisible();
		await startExecutionButton(page).click();

		// Verify the pipeline advances past ready_for_execution
		const start = Date.now();
		plan = await getPlan(slug);
		while (plan.stage === 'ready_for_execution' && Date.now() - start < 30000) {
			await new Promise((r) => setTimeout(r, 1000));
			plan = await getPlan(slug);
		}
		expect(['implementing', 'executing', 'reviewing_rollup', 'complete']).toContain(plan.stage);
	});

	// TODO: Execution stalls at reviewing_rollup with mock LLM — rollup review
	// fixtures or component config needed. Un-skip when mock supports full cycle.
	test.skip('execution reaches complete', async ({ page }) => {
		await page.goto(`/plans/${slug}`);
		await waitForHydration(page);

		await expect(
			page.getByTestId('panel-center').locator('[data-stage="complete"]')
		).toBeVisible({ timeout: 90000 });

		const plan = await getPlan(slug);
		expect(plan.stage).toBe('complete');
	});

	test.skip('completed plan shows in Done filter', async ({ page }) => {
		await page.goto('/');
		await waitForHydration(page);

		await page.getByRole('radio', { name: 'Done' }).click();
		await expect(planListItem(page, slug)).toBeVisible();
	});
});
