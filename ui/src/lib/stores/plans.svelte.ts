import { api } from '$lib/api/client';
import type { PlanWithStatus, PlanStage } from '$lib/types/plan';
import type { Task } from '$lib/types/task';

/**
 * Store for ADR-003 Plan + Tasks workflow.
 *
 * All computed properties use $derived to avoid creating new array references
 * on every read — raw class getters would cause cascading re-renders.
 */
class PlansStore {
	all = $state<PlanWithStatus[]>([]);
	tasksByPlan = $state<Record<string, Task[]>>({});
	loading = $state(false);
	error = $state<string | null>(null);
	selectedSlug = $state<string | null>(null);

	/** Draft plans (not yet approved) */
	drafts = $derived(this.all.filter((p) => !p.approved));

	/** Approved plans */
	approved = $derived(this.all.filter((p) => p.approved));

	/** Active plans (not complete or failed) */
	active = $derived(this.all.filter((p) => !['complete', 'failed'].includes(p.stage)));

	/** Plans currently executing */
	executing = $derived(
		this.all.filter((p) => p.stage === 'implementing' || p.stage === 'executing')
	);

	/** Plans with active loops */
	withActiveLoops = $derived(this.all.filter((p) => (p.active_loops?.length ?? 0) > 0));

	/** Plans grouped by stage */
	byStage = $derived.by((): Record<PlanStage, PlanWithStatus[]> => {
		const grouped: Record<PlanStage, PlanWithStatus[]> = {
			draft: [],
			drafting: [],
			ready_for_approval: [],
			reviewed: [],
			needs_changes: [],
			planning: [],
			approved: [],
			rejected: [],
			requirements_generated: [],
			scenarios_generated: [],
			ready_for_execution: [],
			phases_generated: [],
			phases_approved: [],
			tasks_generated: [],
			tasks_approved: [],
			tasks: [],
			implementing: [],
			executing: [],
			complete: [],
			archived: [],
			failed: []
		};

		for (const plan of this.all) {
			if (plan.stage in grouped) {
				grouped[plan.stage].push(plan);
			}
		}

		return grouped;
	});

	/**
	 * Get a single plan by slug
	 */
	getBySlug(slug: string): PlanWithStatus | undefined {
		return this.all.find((p) => p.slug === slug);
	}

	/**
	 * Fetch all plans with in-place reconciliation to avoid unnecessary re-renders.
	 */
	async fetch(): Promise<void> {
		this.loading = true;
		this.error = null;

		try {
			const fetched = await api.plans.list();
			if (!Array.isArray(fetched)) return;

			const fetchedBySlug = new Map(fetched.map((p) => [p.slug, p]));
			const existingSlugs = new Set(this.all.map((p) => p.slug));

			// Remove deleted plans
			const filtered = this.all.filter((p) => fetchedBySlug.has(p.slug));

			// Update existing plans in-place (preserves Svelte proxy references)
			for (const existing of filtered) {
				const updated = fetchedBySlug.get(existing.slug);
				if (updated) Object.assign(existing, updated);
			}

			// Add new plans
			let added = false;
			for (const plan of fetched) {
				if (!existingSlugs.has(plan.slug)) {
					filtered.push(plan);
					added = true;
				}
			}

			// Only replace the array if items were added or removed
			if (added || filtered.length !== this.all.length) {
				this.all = filtered;
			}
		} catch (err) {
			this.error = err instanceof Error ? err.message : 'Failed to fetch plans';
		} finally {
			this.loading = false;
		}
	}

	/**
	 * Fetch tasks for a specific plan
	 */
	async fetchTasks(slug: string): Promise<Task[]> {
		try {
			const tasks = await api.plans.getTasks(slug);
			this.tasksByPlan[slug] = tasks;
			return tasks;
		} catch (err) {
			console.error('Failed to fetch tasks:', err);
			return [];
		}
	}

	/**
	 * Get cached tasks for a plan
	 */
	getTasks(slug: string): Task[] {
		return this.tasksByPlan[slug] || [];
	}

	/**
	 * Approve a draft plan
	 */
	async promote(slug: string): Promise<void> {
		const plan = this.getBySlug(slug);
		if (!plan) return;

		try {
			const updated = await api.plans.promote(slug);
			Object.assign(plan, updated);
		} catch (err) {
			this.error = err instanceof Error ? err.message : 'Failed to approve plan';
		}
	}

	/**
	 * Generate tasks for an approved plan
	 */
	async generateTasks(slug: string): Promise<void> {
		const plan = this.getBySlug(slug);
		if (!plan || !plan.approved) return;

		try {
			const tasks = await api.plans.generateTasks(slug);
			this.tasksByPlan[slug] = tasks;
			plan.stage = 'tasks_generated';
		} catch (err) {
			this.error = err instanceof Error ? err.message : 'Failed to generate tasks';
		}
	}

	/**
	 * Start executing tasks for a plan
	 */
	async execute(slug: string): Promise<void> {
		const plan = this.getBySlug(slug);
		if (!plan) return;

		try {
			const updated = await api.plans.execute(slug);
			Object.assign(plan, updated);
		} catch (err) {
			this.error = err instanceof Error ? err.message : 'Failed to start execution';
		}
	}
}

export const plansStore = new PlansStore();
