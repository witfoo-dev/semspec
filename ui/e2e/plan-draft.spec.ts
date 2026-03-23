import { test, expect, waitForHydration, mockPlan } from './helpers/setup';

test.describe('Plan Detail Draft', () => {
	const slug = 'add-user-auth';

	test.beforeEach(async ({ page }) => {
		const plan = mockPlan({
			slug,
			title: 'Add User Authentication',
			goal: 'Implement JWT-based user authentication with login and signup flows',
			context: 'Existing Express.js API with PostgreSQL database',
			approved: false,
			stage: 'draft'
		});

		await Promise.all([
			page.route('**/plan-api/plans', (route) => {
				if (route.request().method() === 'GET') {
					route.fulfill({
						status: 200,
						contentType: 'application/json',
						body: JSON.stringify([plan])
					});
				} else {
					route.continue();
				}
			}),
			page.route('**/plan-api/plans/add-user-auth', (route) => {
				route.fulfill({
					status: 200,
					contentType: 'application/json',
					body: JSON.stringify(plan)
				});
			}),
			page.route('**/plan-api/plans/add-user-auth/tasks', (route) => {
				route.fulfill({ status: 200, contentType: 'application/json', body: '[]' });
			}),
			page.route('**/plan-api/plans/add-user-auth/phases', (route) => {
				route.fulfill({ status: 200, contentType: 'application/json', body: '[]' });
			}),
			page.route('**/plan-api/plans/add-user-auth/requirements', (route) => {
				route.fulfill({ status: 200, contentType: 'application/json', body: '[]' });
			}),
			page.route('**/plan-api/plans/add-user-auth/scenarios**', (route) => {
				route.fulfill({ status: 200, contentType: 'application/json', body: '[]' });
			})
		]);

		await page.goto('/plans/add-user-auth');
		await waitForHydration(page);
	});

	test('shows plan title', async ({ page }) => {
		await expect(page.locator('.plan-title')).toHaveText('Add User Authentication');
	});

	test('shows draft stage badge', async ({ page }) => {
		await expect(page.locator('.plan-stage')).toHaveText('Draft');
	});

	test('shows back link', async ({ page }) => {
		const backLink = page.getByRole('link', { name: 'Back to Plans' }).first();
		await expect(backLink).toBeVisible();
	});

	test('shows goal text', async ({ page }) => {
		await expect(page.getByText('JWT-based user authentication')).toBeVisible();
	});

	test('shows context text', async ({ page }) => {
		await expect(page.getByText('Express.js API with PostgreSQL')).toBeVisible();
	});

	test('shows approve plan button', async ({ page }) => {
		await expect(page.getByRole('button', { name: /Approve Plan/ }).first()).toBeVisible();
	});

	test('edit button is visible', async ({ page }) => {
		await expect(page.getByRole('button', { name: 'Edit' })).toBeVisible();
	});

	test('clicking edit shows goal and context textareas', async ({ page }) => {
		await page.getByRole('button', { name: 'Edit' }).click();
		await expect(page.locator('#edit-goal')).toBeVisible();
		await expect(page.locator('#edit-context')).toBeVisible();
	});

	test('cancel exits edit mode', async ({ page }) => {
		await page.getByRole('button', { name: 'Edit' }).click();
		await page.locator('#edit-goal').fill('Changed goal');
		await page.getByRole('button', { name: 'Cancel' }).click();
		await expect(page.locator('#edit-goal')).not.toBeVisible();
		await expect(page.getByText('JWT-based user authentication')).toBeVisible();
	});
});

test.describe('Plan Not Found', () => {
	test('shows not found for invalid slug', async ({ page }) => {
		await page.route('**/plan-api/plans/nonexistent', (route) => {
			route.fulfill({
				status: 404,
				contentType: 'application/json',
				body: JSON.stringify({ error: 'Not found' })
			});
		});
		await page.route('**/plan-api/plans', (route) => {
			route.fulfill({ status: 200, contentType: 'application/json', body: '[]' });
		});

		await page.goto('/plans/nonexistent');
		await waitForHydration(page);
		await expect(page.getByRole('heading', { name: 'Plan not found' })).toBeVisible();
	});
});
