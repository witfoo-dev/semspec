import { type Page, type Locator, expect } from '@playwright/test';

type NavItem = 'Board' | 'Plans' | 'Activity' | 'Trajectories' | 'Workspace' | 'Settings';

/**
 * Page Object Model for the Sidebar navigation.
 *
 * Provides methods to interact with and verify:
 * - Navigation items (Board, Plans, Activity, Trajectories, Workspace, Settings)
 * - Logo
 */
export class SidebarPage {
	readonly page: Page;
	readonly sidebar: Locator;
	readonly logo: Locator;
	readonly navigation: Locator;

	constructor(page: Page) {
		this.page = page;
		this.sidebar = page.locator('aside.sidebar');
		this.logo = this.sidebar.locator('.logo');
		this.navigation = this.sidebar.locator('nav[aria-label="Main navigation"]');
	}

	async expectVisible(): Promise<void> {
		await expect(this.sidebar).toBeVisible();
	}

	async expectLogo(text = 'SemSpec'): Promise<void> {
		await expect(this.logo).toHaveText(text);
	}

	async navigateTo(path: NavItem): Promise<void> {
		const navItem = this.navigation.locator(`a:has-text("${path}")`);
		await navItem.click();
	}

	async expectActivePage(path: NavItem): Promise<void> {
		const navItem = this.navigation.locator(`a:has-text("${path}")`);
		await expect(navItem).toHaveClass(/active/);
	}

	async getNavItems(): Promise<string[]> {
		const items = await this.navigation.locator('.nav-item span').allTextContents();
		return items;
	}

	// Connection status is now in the Header, not the sidebar.
	// These methods check the Header's connection-status element.
	async expectHealthy(): Promise<void> {
		const header = this.page.locator('.header .connection-status');
		await expect(header).toHaveClass(/connected/);
	}

	async expectUnhealthy(): Promise<void> {
		const header = this.page.locator('.header .connection-status');
		await expect(header).not.toHaveClass(/connected/);
	}
}
