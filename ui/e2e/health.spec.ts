import { test, expect } from '@playwright/test';
import { waitForHydration } from './helpers/hydration';
import { connectionStatus } from './helpers/selectors';

test.describe('@mock @smoke health check', () => {
	test('page loads and hydrates', async ({ page }) => {
		await page.goto('/');
		await waitForHydration(page);
	});

	test('shows connected status', async ({ page }) => {
		await page.goto('/');
		await waitForHydration(page);
		await expect(connectionStatus(page, 'Connected')).toBeVisible();
	});

	test('left panel shows Plans mode by default', async ({ page }) => {
		await page.goto('/');
		await waitForHydration(page);
		const plansRadio = page.getByRole('radio', { name: 'Plans' });
		await expect(plansRadio).toBeVisible();
		await expect(plansRadio).toHaveAttribute('aria-checked', 'true');
	});

	test('navigates to new plan form', async ({ page }) => {
		await page.goto('/');
		await waitForHydration(page);
		await page.getByTitle('New Plan').click();
		await expect(page).toHaveURL('/plans/new');
		await expect(page.getByLabel('What do you want to build?')).toBeVisible();
	});
});
