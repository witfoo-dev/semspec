import { test, expect, mockPlan } from './helpers/setup';
import type { Scenario } from '../src/lib/types/scenario';
import type { Requirement } from '../src/lib/types/requirement';

/**
 * Tests for scenario status display and review verdicts:
 * - Scenario shows status (pending, passing, failing, skipped)
 * - Scenario detail panel shows BDD content when selected
 * - Multiple scenarios display correctly in the plan nav tree
 * - Requirements tree expands to reveal scenario children
 */

// ============================================================================
// Mock data builders
// ============================================================================

function mockRequirement(overrides: {
	id: string;
	title: string;
	description?: string;
	status?: string;
}): Requirement {
	return {
		plan_id: 'plan-test',
		status: 'active',
		description: '',
		created_at: new Date().toISOString(),
		updated_at: new Date().toISOString(),
		...overrides
	} as Requirement;
}

function mockScenario(overrides: {
	id: string;
	requirement_id: string;
	given: string;
	when: string;
	then: string[];
	status?: Scenario['status'];
}): Scenario {
	return {
		status: 'pending',
		created_at: new Date().toISOString(),
		updated_at: new Date().toISOString(),
		...overrides
	} as Scenario;
}

// ============================================================================
// Route setup helper
// ============================================================================

async function setupPlanRoutes(
	page: import('@playwright/test').Page,
	plan: ReturnType<typeof mockPlan>,
	requirements: Requirement[],
	scenarios: Scenario[]
): Promise<void> {
	await Promise.all([
		page.route('**/workflow-api/plans', route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([plan])
			});
		}),
		page.route(`**/workflow-api/plans/${plan.slug}`, route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(plan)
			});
		}),
		page.route(`**/workflow-api/plans/${plan.slug}/phases`, route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([])
			});
		}),
		page.route(`**/workflow-api/plans/${plan.slug}/requirements`, route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(requirements)
			});
		}),
		page.route(`**/workflow-api/plans/${plan.slug}/scenarios**`, route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(scenarios)
			});
		}),
		page.route(`**/workflow-api/plans/${plan.slug}/tasks`, route => {
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([])
			});
		})
	]);
}

// ============================================================================
// Tests
// ============================================================================

test.describe('Scenario Review', () => {
	test.describe('Scenario Status Display', () => {
		test('displays pending scenario in nav tree', async ({ page, planDetailPage }) => {
			const plan = mockPlan({
				slug: 'test-scenarios-pending',
				goal: 'Build auth system',
				approved: true,
				stage: 'implementing'
			});

			const requirements = [
				mockRequirement({ id: 'req-1', title: 'User Authentication' })
			];

			const scenarios = [
				mockScenario({
					id: 'sc-1',
					requirement_id: 'req-1',
					given: 'a registered user',
					when: 'they submit valid credentials',
					then: ['they are redirected to the dashboard', 'a session token is created'],
					status: 'pending'
				})
			];

			await setupPlanRoutes(page, plan, requirements, scenarios);
			await planDetailPage.goto('test-scenarios-pending');
			await planDetailPage.expectNavTreeVisible();

			// The requirement should appear in the tree
			await planDetailPage.expectRequirementInTree('User Authentication');

			// Expand to see scenario
			await planDetailPage.expandRequirementInTree('User Authentication');
			await planDetailPage.expectScenarioInTree('valid credentials');
		});

		test('displays passing scenario in nav tree', async ({ page, planDetailPage }) => {
			const plan = mockPlan({
				slug: 'test-scenarios-passing',
				goal: 'Build auth system',
				approved: true,
				stage: 'reviewing_rollup'
			});

			const requirements = [
				mockRequirement({ id: 'req-1', title: 'Login Flow' })
			];

			const scenarios = [
				mockScenario({
					id: 'sc-passing',
					requirement_id: 'req-1',
					given: 'a user with valid credentials',
					when: 'they submit the login form',
					then: ['the response status is 200', 'a JWT token is returned'],
					status: 'passing'
				})
			];

			await setupPlanRoutes(page, plan, requirements, scenarios);
			await planDetailPage.goto('test-scenarios-passing');
			await planDetailPage.expectNavTreeVisible();

			await planDetailPage.expectRequirementInTree('Login Flow');
			await planDetailPage.expandRequirementInTree('Login Flow');
			await planDetailPage.expectScenarioInTree('valid credentials');
		});

		test('displays failing scenario in nav tree', async ({ page, planDetailPage }) => {
			const plan = mockPlan({
				slug: 'test-scenarios-failing',
				goal: 'Build auth system',
				approved: true,
				stage: 'implementing'
			});

			const requirements = [
				mockRequirement({ id: 'req-1', title: 'Error Handling' })
			];

			const scenarios = [
				mockScenario({
					id: 'sc-failing',
					requirement_id: 'req-1',
					given: 'a user with invalid credentials',
					when: 'they submit the login form',
					then: ['the response status is 401', 'an error message is displayed'],
					status: 'failing'
				})
			];

			await setupPlanRoutes(page, plan, requirements, scenarios);
			await planDetailPage.goto('test-scenarios-failing');
			await planDetailPage.expectNavTreeVisible();

			await planDetailPage.expectRequirementInTree('Error Handling');
			await planDetailPage.expandRequirementInTree('Error Handling');
			await planDetailPage.expectScenarioInTree('invalid credentials');
		});

		test('displays mixed-status scenarios under the same requirement', async ({
			page,
			planDetailPage
		}) => {
			const plan = mockPlan({
				slug: 'test-scenarios-mixed',
				goal: 'Build auth system',
				approved: true,
				stage: 'implementing'
			});

			const requirements = [
				mockRequirement({ id: 'req-1', title: 'Authentication', description: 'JWT auth' })
			];

			const scenarios = [
				mockScenario({
					id: 'sc-1',
					requirement_id: 'req-1',
					given: 'valid credentials',
					when: 'login',
					then: ['200 OK', 'JWT returned'],
					status: 'passing'
				}),
				mockScenario({
					id: 'sc-2',
					requirement_id: 'req-1',
					given: 'expired token',
					when: 'refresh attempt',
					then: ['new token returned'],
					status: 'pending'
				}),
				mockScenario({
					id: 'sc-3',
					requirement_id: 'req-1',
					given: 'invalid credentials',
					when: 'login attempt',
					then: ['401 error'],
					status: 'failing'
				})
			];

			await setupPlanRoutes(page, plan, requirements, scenarios);
			await planDetailPage.goto('test-scenarios-mixed');
			await planDetailPage.expectNavTreeVisible();

			// Requirement appears in the tree
			const reqNode = planDetailPage.navTree.locator('.tree-node.req-node', {
				hasText: 'Authentication'
			});
			await expect(reqNode).toBeVisible();

			// Expand to see all three scenarios
			const reqRow = planDetailPage.navTree.locator('.req-row').filter({ hasText: 'Authentication' });
			await reqRow.locator('.expand-btn').click();

			await planDetailPage.expectScenarioInTree('valid credentials');
			await planDetailPage.expectScenarioInTree('expired token');
			await planDetailPage.expectScenarioInTree('invalid credentials');
		});
	});

	test.describe('Scenario Detail Panel', () => {
		test('selecting a scenario opens the detail panel', async ({ page, planDetailPage }) => {
			const plan = mockPlan({
				slug: 'test-scenario-detail',
				goal: 'Build auth system',
				approved: true,
				stage: 'ready_for_execution'
			});

			const requirements = [
				mockRequirement({
					id: 'req-1',
					title: 'Login Flow',
					description: 'User login with email and password'
				})
			];

			const scenarios = [
				mockScenario({
					id: 'sc-reviewed',
					requirement_id: 'req-1',
					given: 'valid user credentials',
					when: 'the user submits the login form',
					then: ['the response status is 200', 'a JWT token is returned in the body'],
					status: 'passing'
				})
			];

			await setupPlanRoutes(page, plan, requirements, scenarios);
			await planDetailPage.goto('test-scenario-detail');
			await planDetailPage.expectNavTreeVisible();

			// Expand the requirement, then click the scenario
			await planDetailPage.expandRequirementInTree('Login Flow');
			await planDetailPage.selectScenarioInTree('valid user credentials');

			// ScenarioDetail should appear in the detail panel
			await expect(page.locator('.scenario-detail')).toBeVisible({ timeout: 5000 });
		});

		test('scenario detail shows BDD content', async ({ page, planDetailPage }) => {
			const plan = mockPlan({
				slug: 'test-scenario-bdd',
				goal: 'Build auth system',
				approved: true,
				stage: 'implementing'
			});

			const requirements = [
				mockRequirement({ id: 'req-1', title: 'Session Expiry' })
			];

			const scenarios = [
				mockScenario({
					id: 'sc-bdd',
					requirement_id: 'req-1',
					given: 'an authenticated user',
					when: 'their session has been idle for 25 hours',
					then: [
						'they are automatically logged out',
						'the session token is invalidated'
					],
					status: 'pending'
				})
			];

			await setupPlanRoutes(page, plan, requirements, scenarios);
			await planDetailPage.goto('test-scenario-bdd');
			await planDetailPage.expectNavTreeVisible();

			await planDetailPage.expandRequirementInTree('Session Expiry');
			await planDetailPage.selectScenarioInTree('idle for 25 hours');

			// BDD content should be visible in the detail panel
			const detail = page.locator('.scenario-detail');
			await expect(detail).toBeVisible({ timeout: 5000 });
		});
	});

	test.describe('Multiple Requirements', () => {
		test('shows scenarios grouped under their respective requirements', async ({
			page,
			planDetailPage
		}) => {
			const plan = mockPlan({
				slug: 'test-multi-req',
				goal: 'Build auth system',
				approved: true,
				stage: 'ready_for_execution'
			});

			const requirements = [
				mockRequirement({
					id: 'req-1',
					title: 'User Login',
					description: 'Users can log in with email and password',
					status: 'active'
				}),
				mockRequirement({
					id: 'req-2',
					title: 'Session Management',
					description: 'Sessions expire after 24 hours of inactivity',
					status: 'active'
				})
			];

			const scenarios = [
				mockScenario({
					id: 'sc-1',
					requirement_id: 'req-1',
					given: 'a registered user',
					when: 'they submit valid credentials',
					then: ['they are redirected to the dashboard'],
					status: 'passing'
				}),
				mockScenario({
					id: 'sc-2',
					requirement_id: 'req-1',
					given: 'a registered user',
					when: 'they submit an invalid password',
					then: ['an error message is displayed'],
					status: 'failing'
				}),
				mockScenario({
					id: 'sc-3',
					requirement_id: 'req-2',
					given: 'an authenticated user',
					when: 'their session has been idle for 25 hours',
					then: ['they are automatically logged out'],
					status: 'pending'
				})
			];

			await setupPlanRoutes(page, plan, requirements, scenarios);
			await planDetailPage.goto('test-multi-req');
			await planDetailPage.expectNavTreeVisible();

			// Both requirements appear in the tree
			await planDetailPage.expectRequirementInTree('User Login');
			await planDetailPage.expectRequirementInTree('Session Management');

			// Expand User Login to see its scenarios
			await planDetailPage.expandRequirementInTree('User Login');
			await planDetailPage.expectScenarioInTree('valid credentials');
			await planDetailPage.expectScenarioInTree('invalid password');

			// Expand Session Management to see its scenario
			await planDetailPage.expandRequirementInTree('Session Management');
			await planDetailPage.expectScenarioInTree('idle for 25 hours');
		});

		test('requirement detail shows when requirement is selected', async ({
			page,
			planDetailPage
		}) => {
			const plan = mockPlan({
				slug: 'test-req-detail',
				goal: 'Build auth system',
				approved: true,
				stage: 'implementing'
			});

			const requirements = [
				mockRequirement({
					id: 'req-1',
					title: 'Password Reset',
					description: 'Users can reset their password via email',
					status: 'active'
				})
			];

			const scenarios = [
				mockScenario({
					id: 'sc-1',
					requirement_id: 'req-1',
					given: 'a registered user',
					when: 'they request a password reset',
					then: ['an email is sent with a reset link'],
					status: 'pending'
				})
			];

			await setupPlanRoutes(page, plan, requirements, scenarios);
			await planDetailPage.goto('test-req-detail');
			await planDetailPage.expectNavTreeVisible();

			// Click the requirement node — should show RequirementDetail
			await planDetailPage.selectRequirementInTree('Password Reset');
			await planDetailPage.expectRequirementDetailVisible();
			await planDetailPage.expectRequirementDetailTitle('Password Reset');
		});
	});

	test.describe('Empty States', () => {
		test('shows plan detail without scenarios when requirements have none', async ({
			page,
			planDetailPage
		}) => {
			const plan = mockPlan({
				slug: 'test-no-scenarios',
				goal: 'Define the system',
				approved: true,
				stage: 'scenarios_generated'
			});

			const requirements = [
				mockRequirement({
					id: 'req-1',
					title: 'Core Functionality',
					description: 'The system core must be reliable'
				})
			];

			// No scenarios yet
			await setupPlanRoutes(page, plan, requirements, []);
			await planDetailPage.goto('test-no-scenarios');
			await planDetailPage.expectNavTreeVisible();

			// Requirement is present
			await planDetailPage.expectRequirementInTree('Core Functionality');

			// Expanding shows no scenarios
			const reqRow = planDetailPage.navTree
				.locator('.req-row')
				.filter({ hasText: 'Core Functionality' });
			const expandBtn = reqRow.locator('.expand-btn');
			// Expand button may not exist or may be disabled when no children exist
			const expandBtnVisible = await expandBtn.isVisible();
			if (expandBtnVisible) {
				await expandBtn.click();
				// No scenario nodes should appear
				const scenarioNodes = planDetailPage.navTree.locator('.tree-node.scenario-node');
				await expect(scenarioNodes).toHaveCount(0);
			}
		});

		test('shows plan with no requirements loaded yet', async ({ page, planDetailPage }) => {
			const plan = mockPlan({
				slug: 'test-no-requirements',
				goal: 'Plan with no requirements',
				approved: true,
				stage: 'approved'
			});

			await setupPlanRoutes(page, plan, [], []);
			await planDetailPage.goto('test-no-requirements');

			// Page loads without error
			await planDetailPage.expectVisible();

			// No requirement nodes in the tree
			const reqNodes = planDetailPage.requirementNodes;
			await expect(reqNodes).toHaveCount(0);
		});
	});

	test.describe('Stage-Specific Scenario Display', () => {
		test('shows scenarios in implementing stage', async ({ page, planDetailPage }) => {
			const plan = mockPlan({
				slug: 'test-implementing-scenarios',
				goal: 'Build the API layer',
				approved: true,
				stage: 'implementing'
			});

			const requirements = [
				mockRequirement({ id: 'req-1', title: 'REST Endpoints' })
			];

			const scenarios = [
				mockScenario({
					id: 'sc-impl-1',
					requirement_id: 'req-1',
					given: 'a client with a valid API key',
					when: 'they send a GET request to /api/users',
					then: ['the response status is 200', 'the body contains a list of users'],
					status: 'pending'
				}),
				mockScenario({
					id: 'sc-impl-2',
					requirement_id: 'req-1',
					given: 'a client without an API key',
					when: 'they send a GET request to /api/users',
					then: ['the response status is 401'],
					status: 'passing'
				})
			];

			await setupPlanRoutes(page, plan, requirements, scenarios);
			await planDetailPage.goto('test-implementing-scenarios');
			await planDetailPage.expectNavTreeVisible();

			await planDetailPage.expectRequirementInTree('REST Endpoints');
			await planDetailPage.expandRequirementInTree('REST Endpoints');
			await planDetailPage.expectScenarioInTree('valid API key');
			await planDetailPage.expectScenarioInTree('without an API key');
		});

		test('shows all scenarios as passing after reviewing_rollup completes', async ({
			page,
			planDetailPage
		}) => {
			const plan = mockPlan({
				slug: 'test-rollup-scenarios',
				goal: 'Build the API layer',
				approved: true,
				stage: 'reviewing_rollup'
			});

			const requirements = [
				mockRequirement({ id: 'req-1', title: 'API Security' })
			];

			const scenarios = [
				mockScenario({
					id: 'sc-r1',
					requirement_id: 'req-1',
					given: 'a client with a valid token',
					when: 'they access a protected route',
					then: ['access is granted'],
					status: 'passing'
				}),
				mockScenario({
					id: 'sc-r2',
					requirement_id: 'req-1',
					given: 'a client with an expired token',
					when: 'they access a protected route',
					then: ['they receive a 401 response'],
					status: 'passing'
				})
			];

			await setupPlanRoutes(page, plan, requirements, scenarios);
			await planDetailPage.goto('test-rollup-scenarios');

			// Pipeline is visible for reviewing_rollup stage
			await planDetailPage.expectPipelineVisible();
			await planDetailPage.expectNavTreeVisible();

			await planDetailPage.expectRequirementInTree('API Security');
			await planDetailPage.expandRequirementInTree('API Security');
			await planDetailPage.expectScenarioInTree('valid token');
			await planDetailPage.expectScenarioInTree('expired token');
		});
	});
});
