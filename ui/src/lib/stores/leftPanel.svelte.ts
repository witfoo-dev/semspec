/**
 * Left Panel Store — tracks mode (activity feed vs plans navigator).
 *
 * Auto-switches to feed mode when loops are executing,
 * back to plans mode when all loops complete. User can override.
 */

type LeftPanelMode = 'feed' | 'plans';
type PlanFilter = 'all' | 'active' | 'draft' | 'complete';

class LeftPanelStore {
	mode = $state<LeftPanelMode>('plans');
	planFilter = $state<PlanFilter>('all');

	/** Whether auto-switching is enabled (user can disable) */
	autoSwitch = $state(true);

	/** User manually overrode the mode — suppresses auto-switch until next idle→executing transition */
	private manualOverride = $state(false);

	setMode(mode: LeftPanelMode): void {
		this.mode = mode;
		this.manualOverride = true;
	}

	setPlanFilter(filter: PlanFilter): void {
		this.planFilter = filter;
	}

	/**
	 * Called when active loop count changes. Auto-switches mode
	 * unless the user has manually overridden.
	 */
	onLoopCountChange(activeCount: number): void {
		if (!this.autoSwitch) return;

		if (activeCount > 0 && !this.manualOverride) {
			this.mode = 'feed';
		} else if (activeCount === 0) {
			this.mode = 'plans';
			this.manualOverride = false; // Reset override when idle
		}
	}
}

export const leftPanelStore = new LeftPanelStore();
