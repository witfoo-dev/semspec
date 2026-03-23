import { test, expect, waitForHydration, mockPlan } from './helpers/setup';

function mockPlanRoutes(page: any, plan: any, extra?: { requirements?: any[]; scenarios?: any[] }) {
	return Promise.all([
		page.route('**/workflow-api/plans', (r: any) =>
			r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([plan]) })
		),
		page.route(`**/workflow-api/plans/${plan.slug}`, (r: any) =>
			r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(plan) })
		),
		page.route(`**/workflow-api/plans/${plan.slug}/tasks`, (r: any) =>
			r.fulfill({ status: 200, contentType: 'application/json', body: '[]' })
		),
		page.route(`**/workflow-api/plans/${plan.slug}/requirements`, (r: any) =>
			r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(extra?.requirements ?? []) })
		),
		page.route(`**/workflow-api/plans/${plan.slug}/scenarios**`, (r: any) =>
			r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(extra?.scenarios ?? []) })
		)
	]);
}

test.describe('Plan Cascade', () => {
	test('approved shows generating requirements', async ({ page }) => {
		const plan = mockPlan({ slug: 'c1', goal: 'Build auth', approved: true, stage: 'approved' });
		await mockPlanRoutes(page, plan);
		await page.goto('/plans/c1');
		await waitForHydration(page);
		await expect(page.locator('.cascade-status')).toContainText('Generating requirements');
	});

	test('requirements_generated shows generating scenarios', async ({ page }) => {
		const plan = mockPlan({ slug: 'c2', goal: 'Build auth', approved: true, stage: 'requirements_generated' });
		const reqs = [{ id: 'r1', plan_id: 'x', title: 'User Login', description: '', status: 'active', created_at: new Date().toISOString(), updated_at: new Date().toISOString() }];
		await mockPlanRoutes(page, plan, { requirements: reqs });
		await page.goto('/plans/c2');
		await waitForHydration(page);
		await expect(page.locator('.cascade-status')).toContainText('Generating scenarios');
		await expect(page.locator('.plan-stage')).toHaveText('Requirements Generated');
	});
});

test.describe('Plan Ready', () => {
	test('ready_for_execution shows Start Execution', async ({ page }) => {
		const plan = mockPlan({ slug: 'r1', goal: 'Build auth', approved: true, stage: 'ready_for_execution' });
		const reqs = [
			{ id: 'r1', plan_id: 'x', title: 'User Login', description: '', status: 'active', created_at: new Date().toISOString(), updated_at: new Date().toISOString() },
			{ id: 'r2', plan_id: 'x', title: 'Sessions', description: '', status: 'active', created_at: new Date().toISOString(), updated_at: new Date().toISOString() }
		];
		await mockPlanRoutes(page, plan, { requirements: reqs });
		await page.goto('/plans/r1');
		await waitForHydration(page);
		await expect(page.getByRole('button', { name: /Start Execution/ })).toBeVisible();
		await expect(page.locator('.plan-stage')).toHaveText('Ready to Execute');
	});
});

test.describe('Plan Executing', () => {
	test('executing shows status and pipeline', async ({ page }) => {
		const plan = mockPlan({
			slug: 'e1', goal: 'Build auth', approved: true, stage: 'executing',
			active_loops: [{ loop_id: 'l1', role: 'developer', state: 'executing', model: 'claude-3', iterations: 2, max_iterations: 10 }]
		});
		await mockPlanRoutes(page, plan);
		await page.goto('/plans/e1');
		await waitForHydration(page);
		await expect(page.locator('.execution-status')).toContainText('Executing');
		await expect(page.locator('.agent-pipeline-section')).toBeVisible();
	});
});

test.describe('Plan Terminal', () => {
	test('complete shows success', async ({ page }) => {
		const plan = mockPlan({ slug: 'd1', goal: 'Build auth', approved: true, stage: 'complete' });
		await mockPlanRoutes(page, plan);
		await page.goto('/plans/d1');
		await waitForHydration(page);
		await expect(page.locator('.complete-status')).toContainText('Complete');
		await expect(page.locator('.plan-stage')).toHaveText('Complete');
	});

	test('failed shows replay', async ({ page }) => {
		const plan = mockPlan({ slug: 'f1', goal: 'Build auth', approved: true, stage: 'failed' });
		await mockPlanRoutes(page, plan);
		await page.goto('/plans/f1');
		await waitForHydration(page);
		await expect(page.getByRole('button', { name: /Replay/ })).toBeVisible();
		await expect(page.locator('.plan-stage')).toHaveText('Failed');
	});
});
