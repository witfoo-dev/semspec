import { chatBarStore } from './chatDrawer.svelte';

export interface ToastItem {
	id: string;
	message: string;
	questionId: string;
	urgency: string;
	createdAt: number;
}

class ToastStore {
	items = $state<ToastItem[]>([]);
	private timers = new Map<string, ReturnType<typeof setTimeout>>();

	show(opts: { message: string; questionId: string; urgency: string }): void {
		const id = crypto.randomUUID();
		const item: ToastItem = {
			id,
			message: opts.message,
			questionId: opts.questionId,
			urgency: opts.urgency,
			createdAt: Date.now()
		};
		this.items = [...this.items, item];

		const timer = setTimeout(() => this.dismiss(id), 6000);
		this.timers.set(id, timer);
	}

	dismiss(id: string): void {
		this.items = this.items.filter((t) => t.id !== id);
		const timer = this.timers.get(id);
		if (timer) {
			clearTimeout(timer);
			this.timers.delete(id);
		}
	}

	/** Open chat bar and dismiss the toast. */
	view(id: string): void {
		chatBarStore.expand();
		this.dismiss(id);
	}
}

export const toastStore = new ToastStore();
