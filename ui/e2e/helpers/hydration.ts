import type { Page } from '@playwright/test';

/**
 * Wait for Svelte 5 hydration to complete.
 * The +layout.svelte adds 'hydrated' class to <body> in onMount().
 * Without this, $state/$derived/$effect are not wired up and the UI is non-interactive.
 */
export async function waitForHydration(page: Page): Promise<void> {
	await page.locator('body.hydrated').waitFor({ timeout: 15000 });
}
