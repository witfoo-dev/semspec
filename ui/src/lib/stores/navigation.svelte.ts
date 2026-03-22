/**
 * Navigation Store — tracks the user's current focus within the app shell.
 *
 * Drives the right panel content and header breadcrumbs.
 */

type RightTab = 'trajectory' | 'reviews' | 'agents' | 'files';

class NavigationStore {
	/** Currently viewed plan slug (set when navigating to /plans/[slug]) */
	activePlanSlug = $state<string | null>(null);

	/** Currently selected loop ID (drives trajectory panel) */
	activeLoopId = $state<string | null>(null);

	/** Active tab in the right panel */
	rightTab = $state<RightTab>('trajectory');

	/** Whether the right panel should auto-open */
	rightAutoOpen = $state(true);

	selectPlan(slug: string | null): void {
		this.activePlanSlug = slug;
		if (!slug) {
			this.activeLoopId = null;
		}
	}

	selectLoop(loopId: string | null): void {
		this.activeLoopId = loopId;
		if (loopId) {
			this.rightTab = 'trajectory';
		}
	}

	setRightTab(tab: RightTab): void {
		this.rightTab = tab;
	}
}

export const navigationStore = new NavigationStore();
