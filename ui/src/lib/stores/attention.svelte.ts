import { plansStore } from './plans.svelte';
import { loopsStore } from './loops.svelte';
import type { AttentionItem } from '$lib/api/mock-plans';

/**
 * Attention item types
 */
export type AttentionType =
	| 'approval_needed'
	| 'task_failed'
	| 'task_blocked'
	| 'rejection';

/**
 * Store for items requiring human attention.
 * Derives attention items from plans, loops, and questions stores.
 *
 * All computed properties use $derived to cache results and avoid
 * creating new references on every template read.
 */
class AttentionStore {
	items = $derived.by((): AttentionItem[] => {
		const result: AttentionItem[] = [];

		// Plans ready to execute (tasks approved)
		for (const plan of plansStore.all.filter((p) => p.stage === 'tasks_approved')) {
			result.push({
				type: 'approval_needed',
				plan_slug: plan.slug,
				title: `Ready to execute "${plan.slug}"`,
				description: 'Tasks have been generated. Approve to begin execution.',
				action_url: `/plans/${plan.slug}`,
				created_at: plan.approved_at || plan.created_at
			});
		}

		// Plans with active rejections
		for (const plan of plansStore.all) {
			const tasks = plansStore.getTasks(plan.slug);
			const rejectedTask = tasks.find((t) => t.rejection && t.status === 'in_progress');
			if (rejectedTask && rejectedTask.rejection) {
				result.push({
					type: 'rejection',
					plan_slug: plan.slug,
					title: `Task rejected in "${plan.slug}"`,
					description: rejectedTask.rejection.reason,
					action_url: `/plans/${plan.slug}`,
					created_at: rejectedTask.rejection.rejected_at
				});
			}
		}

		// Failed loops
		for (const loop of loopsStore.all.filter((l) => l.state === 'failed')) {
			const plan = plansStore.all.find((p) =>
				p.active_loops?.some((al) => al.loop_id === loop.loop_id)
			);

			result.push({
				type: 'task_failed',
				loop_id: loop.loop_id,
				plan_slug: plan?.slug,
				title: `Task failed in loop ${loop.loop_id.slice(-6)}`,
				description: `Loop failed after ${loop.iterations} iterations`,
				action_url: plan ? `/plans/${plan.slug}` : '/activity',
				created_at: loop.created_at || new Date().toISOString()
			});
		}

		return result.sort(
			(a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
		);
	});

	count = $derived(this.items.length);

	byType = $derived.by((): Record<AttentionType, AttentionItem[]> => {
		const grouped: Record<AttentionType, AttentionItem[]> = {
			approval_needed: [],
			task_failed: [],
			task_blocked: [],
			rejection: []
		};

		for (const item of this.items) {
			grouped[item.type].push(item);
		}

		return grouped;
	});

	hasType(type: AttentionType): boolean {
		return this.items.some((i) => i.type === type);
	}

	forPlan(slug: string): AttentionItem[] {
		return this.items.filter((i) => i.plan_slug === slug);
	}

	forChange(slug: string): AttentionItem[] {
		return this.forPlan(slug);
	}
}

export const attentionStore = new AttentionStore();
