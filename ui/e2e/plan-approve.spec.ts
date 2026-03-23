import { test, expect } from '@playwright/test';
import { waitForHydration } from './helpers/hydration';
import { createPlan, deletePlan, getPlan } from './helpers/api';
import { MockLLMClient } from './helpers/mock-llm';
import { startExecutionButton } from './helpers/selectors';

/**
 * Plan approval + cascade flow.
 * Uses mock LLM (hello-world scenario).
 *
 * Serial: the cascade consumes mock LLM fixtures, so tests must
 * share a single plan and run in order.
 */
test.describe('@mock @happy-path plan-approve', () => {
	let slug: string;

	test.describe.configure({ mode: 'serial' });

	test.beforeAll(async () => {
		const mockLLM = new MockLLMClient();
		await mockLLM.resetScenario('hello-world');
		const plan = await createPlan(`Approve flow test ${Date.now()}`);
		slug = plan.slug;
	});

	test.afterAll(async () => {
		if (slug) await deletePlan(slug).catch(() => {});
	});

	test('shows Approve Plan button for draft plan', async ({ page }) => {
		await page.goto(`/plans/${slug}`);
		await waitForHydration(page);

		await expect(page.getByRole('button', { name: /Approve Plan/i }).first()).toBeVisible();
	});

	test('approving triggers cascade and reaches ready state', async ({ page }) => {
		await page.goto(`/plans/${slug}`);
		await waitForHydration(page);

		await page.getByRole('button', { name: /Approve Plan/i }).first().click();

		// Approve button should disappear after click
		await expect(page.getByRole('button', { name: /Approve Plan/i })).not.toBeVisible({
			timeout: 10000
		});

		// The cascade runs: planning → review → requirements → scenarios
		// "Start Execution" appears when cascade completes
		await expect(startExecutionButton(page)).toBeVisible({ timeout: 60000 });
	});

	test('backend confirms approved state', async () => {
		// The UI shows "Start Execution" based on data presence (hasScenarios),
		// but the backend stage field may still be catching up.
		// Poll until the stage settles to ready_for_execution.
		let plan = await getPlan(slug);
		const start = Date.now();
		while (plan.stage !== 'ready_for_execution' && Date.now() - start < 30000) {
			await new Promise((r) => setTimeout(r, 1000));
			plan = await getPlan(slug);
		}
		expect(plan.approved).toBe(true);
		expect(plan.stage).toBe('ready_for_execution');
	});

	test('stage badge shows ready to execute', async ({ page }) => {
		await page.goto(`/plans/${slug}`);
		await waitForHydration(page);

		await expect(page.locator('[data-stage="ready_for_execution"]').first()).toBeVisible();
		await expect(startExecutionButton(page)).toBeVisible();
	});
});
