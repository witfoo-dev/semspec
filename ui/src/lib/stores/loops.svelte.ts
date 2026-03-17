import { api } from '$lib/api/client';
import type { Loop } from '$lib/types';

class LoopsStore {
	all = $state<Loop[]>([]);
	loading = $state(false);
	error = $state<string | null>(null);

	active = $derived(
		this.all.filter((l) => ['pending', 'executing', 'paused'].includes(l.state))
	);

	paused = $derived(this.all.filter((l) => l.state === 'paused'));

	async fetch(): Promise<void> {
		this.loading = true;
		this.error = null;

		try {
			const fetched = await api.router.getLoops();
			if (!Array.isArray(fetched)) return;

			// Reconcile in-place to avoid unnecessary re-renders
			const fetchedById = new Map(fetched.map((l) => [l.loop_id, l]));
			const existingIds = new Set(this.all.map((l) => l.loop_id));

			const filtered = this.all.filter((l) => fetchedById.has(l.loop_id));
			for (const existing of filtered) {
				const updated = fetchedById.get(existing.loop_id);
				if (updated) Object.assign(existing, updated);
			}

			let added = false;
			for (const loop of fetched) {
				if (!existingIds.has(loop.loop_id)) {
					filtered.push(loop);
					added = true;
				}
			}

			if (added || filtered.length !== this.all.length) {
				this.all = filtered;
			}
		} catch (err) {
			this.error = err instanceof Error ? err.message : 'Failed to fetch loops';
		} finally {
			this.loading = false;
		}
	}

	async sendSignal(loopId: string, type: 'pause' | 'resume' | 'cancel', reason?: string): Promise<void> {
		await api.router.sendSignal(loopId, type, reason);
	}
}

export const loopsStore = new LoopsStore();
