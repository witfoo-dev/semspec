import { test, expect } from '@playwright/test';
import { waitForHydration } from './helpers/hydration';
import { createPlan, deletePlan, getPlan } from './helpers/api';
import { MockLLMClient } from './helpers/mock-llm';
import { startExecutionButton } from './helpers/selectors';

/**
 * Plan rejection flow: plan is rejected by reviewer once, then approved on retry.
 * Switches mock LLM to hello-world-plan-rejection scenario before running.
 */
test.describe('@mock @rejection plan-rejection', () => {
	const mockLLM = new MockLLMClient();
	let slug: string;

	test.describe.configure({ mode: 'serial' });

	test.beforeAll(async () => {
		await mockLLM.resetScenario('hello-world-plan-rejection');
		const plan = await createPlan('Build a rejection recovery test feature');
		slug = plan.slug;
	});

	test.afterAll(async () => {
		// Restore default scenario for other tests
		await mockLLM.resetScenario('hello-world');
		if (slug) await deletePlan(slug).catch(() => {});
	});

	test('plan recovers from rejection and reaches ready state', async ({ page }) => {
		await page.goto(`/plans/${slug}`);
		await waitForHydration(page);

		await page.getByRole('button', { name: /Approve Plan/i }).first().click();
		await expect(page.getByRole('button', { name: /Approve Plan/i })).not.toBeVisible({
			timeout: 10000
		});

		// The mock reviewer rejects first (mock-reviewer.1.json), then approves (mock-reviewer.json).
		// After the full cycle: plan → reject → re-plan → approve → cascade
		await expect(startExecutionButton(page)).toBeVisible({ timeout: 90000 });

		const plan = await getPlan(slug);
		expect(plan.approved).toBe(true);
	});
});
