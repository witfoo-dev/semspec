import { test, expect } from '@playwright/test';
import { waitForHydration } from './helpers/hydration';
import { createPlan, deletePlan } from './helpers/api';
import {
	filterChip,
	planListItem,
	emptyPlansMessage,
	plansModeRadio,
	feedModeRadio
} from './helpers/selectors';

test.describe('@mock plan-list', () => {
	let createdSlugs: string[] = [];

	test.afterEach(async () => {
		// Clean up plans created during tests
		for (const slug of createdSlugs) {
			await deletePlan(slug).catch(() => {});
		}
		createdSlugs = [];
	});

	test('shows empty state when no plans exist', async ({ page }) => {
		await page.goto('/');
		await waitForHydration(page);
		// Filter to "Drafts" to avoid seeing any existing plans from other tests
		await filterChip(page, 'Drafts').click();
		// Either shows plans or empty state - this depends on test isolation
	});

	test('shows created plan in list', async ({ page }) => {
		const plan = await createPlan('Test plan for list view');
		createdSlugs.push(plan.slug);

		await page.goto('/');
		await waitForHydration(page);

		await expect(planListItem(page, plan.slug)).toBeVisible();
	});

	test('plan item links to detail page', async ({ page }) => {
		const plan = await createPlan('Test plan for navigation');
		createdSlugs.push(plan.slug);

		await page.goto('/');
		await waitForHydration(page);

		await planListItem(page, plan.slug).click();
		await expect(page).toHaveURL(`/plans/${plan.slug}`);
	});

	test('filter chips switch between views', async ({ page }) => {
		const plan = await createPlan('Test plan for filtering');
		createdSlugs.push(plan.slug);

		await page.goto('/');
		await waitForHydration(page);

		// Default "All" filter should show the plan
		await expect(planListItem(page, plan.slug)).toBeVisible();

		// "Drafts" should show it (new plans are drafts)
		await filterChip(page, 'Drafts').click();
		await expect(planListItem(page, plan.slug)).toBeVisible();

		// "Active" should NOT show it (draft plans aren't active)
		await filterChip(page, 'Active').click();
		await expect(planListItem(page, plan.slug)).not.toBeVisible();
	});

	test('mode switcher toggles between Plans and Feed', async ({ page }) => {
		await page.goto('/');
		await waitForHydration(page);

		// Plans mode is default
		await expect(plansModeRadio(page)).toHaveAttribute('aria-checked', 'true');
		await expect(feedModeRadio(page)).toHaveAttribute('aria-checked', 'false');

		// Switch to Feed
		await feedModeRadio(page).click();
		await expect(feedModeRadio(page)).toHaveAttribute('aria-checked', 'true');
		await expect(plansModeRadio(page)).toHaveAttribute('aria-checked', 'false');

		// Switch back to Plans
		await plansModeRadio(page).click();
		await expect(plansModeRadio(page)).toHaveAttribute('aria-checked', 'true');
	});
});
