<script lang="ts">
	import Icon from './Icon.svelte';
	import { toastStore } from '$lib/stores/toast.svelte';

	const toasts = $derived(toastStore.items);
</script>

{#if toasts.length > 0}
	<div class="toast-container" aria-live="polite">
		{#each toasts as toast (toast.id)}
			<div class="toast" class:blocking={toast.urgency === 'blocking'} class:high={toast.urgency === 'high'}>
				<Icon name="help-circle" size={16} />
				<span class="toast-message">{toast.message}</span>
				<button class="toast-view" onclick={() => toastStore.view(toast.id)} aria-label="View question from {toast.message}">
					View
				</button>
				<button class="toast-close" onclick={() => toastStore.dismiss(toast.id)} aria-label="Dismiss">
					<Icon name="x" size={14} />
				</button>
			</div>
		{/each}
	</div>
{/if}

<style>
	.toast-container {
		position: fixed;
		bottom: 52px;
		right: 16px;
		z-index: 60;
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
		pointer-events: none;
	}

	.toast {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		background: var(--color-bg-elevated);
		border: 1px solid var(--color-warning);
		border-radius: var(--radius-lg);
		box-shadow: var(--shadow-lg);
		color: var(--color-text-primary);
		font-size: var(--font-size-sm);
		pointer-events: auto;
		animation: slide-up 0.2s ease-out;
	}

	.toast.blocking {
		border-color: var(--color-error);
		background: color-mix(in srgb, var(--color-error) 8%, var(--color-bg-elevated));
	}

	.toast.high {
		border-color: var(--color-warning);
	}

	.toast-message {
		flex: 1;
		min-width: 0;
	}

	.toast-view {
		padding: var(--space-1) var(--space-2);
		border: none;
		border-radius: var(--radius-md);
		background: var(--color-accent-muted);
		color: var(--color-accent);
		font-size: var(--font-size-xs);
		cursor: pointer;
		white-space: nowrap;
	}

	.toast-view:hover {
		filter: brightness(1.2);
	}

	.toast-close {
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 2px;
		border: none;
		border-radius: var(--radius-sm);
		background: transparent;
		color: var(--color-text-muted);
		cursor: pointer;
	}

	.toast-close:hover {
		color: var(--color-text-primary);
		background: var(--color-bg-tertiary);
	}

	@keyframes slide-up {
		from {
			opacity: 0;
			transform: translateY(8px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}
</style>
