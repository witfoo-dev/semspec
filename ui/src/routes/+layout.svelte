<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import Sidebar from '$lib/components/shared/Sidebar.svelte';
	import Header from '$lib/components/shared/Header.svelte';
	import BottomChatBar from '$lib/components/chat/BottomChatBar.svelte';
	import Toast from '$lib/components/shared/Toast.svelte';
	import Icon from '$lib/components/shared/Icon.svelte';
	import { activityStore } from '$lib/stores/activity.svelte';
	import { loopsStore } from '$lib/stores/loops.svelte';
	import { systemStore } from '$lib/stores/system.svelte';
	import { messagesStore } from '$lib/stores/messages.svelte';
	import { plansStore } from '$lib/stores/plans.svelte';
	import { questionsStore } from '$lib/stores/questions.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { chatBarStore } from '$lib/stores/chatDrawer.svelte';
	import { setupStore } from '$lib/stores/setup.svelte';
	import { sidebarStore } from '$lib/stores/sidebar.svelte';
	import '../app.css';

	import type { Snippet } from 'svelte';

	interface Props {
		children: Snippet;
	}

	let { children }: Props = $props();

	/**
	 * Global keyboard shortcuts.
	 */
	function handleKeydown(e: KeyboardEvent): void {
		// Cmd+K (Mac) or Ctrl+K (Windows/Linux) - Toggle chat drawer
		if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
			e.preventDefault();
			chatBarStore.toggle();
		}
	}

	// Check if project config is missing (warn, don't block)
	const configWarning = $derived(
		setupStore.step === 'scaffold' ||
			setupStore.step === 'detection' ||
			setupStore.step === 'error'
	);

	// Mark hydration complete for e2e tests and initialize connections
	onMount(() => {
		document.body.classList.add('hydrated');
		setupStore.checkStatus();

		// Initialize SSE and data connections (runs once)
		activityStore.connect();
		questionsStore.connect();
		loopsStore.fetch();
		systemStore.fetch();
		plansStore.fetch();

		const unsubscribe = activityStore.onEvent((event) => {
			messagesStore.handleActivityEvent(event);
		});

		// Periodic refresh for non-SSE data
		const interval = setInterval(() => {
			loopsStore.fetch();
			systemStore.fetch();
			plansStore.fetch();
		}, 30000);

		return () => {
			activityStore.disconnect();
			questionsStore.disconnect();
			unsubscribe();
			clearInterval(interval);
		};
	});

	// Apply reduced motion setting (reactive — responds to preference changes)
	$effect(() => {
		if (settingsStore.reducedMotion) {
			document.documentElement.classList.add('reduced-motion');
		} else {
			document.documentElement.classList.remove('reduced-motion');
		}
	});
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="app-layout">
	<Sidebar currentPath={page.url.pathname} />

	<!-- Mobile sidebar backdrop -->
	{#if sidebarStore.isOpen}
		<button
			class="sidebar-backdrop"
			onclick={() => sidebarStore.close()}
			aria-label="Close navigation"
		></button>
	{/if}

	<div class="main-area">
		<!-- Mobile hamburger button -->
		<button
			class="hamburger-btn"
			onclick={() => sidebarStore.open()}
			aria-label="Open navigation"
			aria-expanded={sidebarStore.isOpen}
		>
			<Icon name="menu" size={24} />
		</button>

		<Header />

		<!-- Config warning banner (non-blocking) -->
		{#if configWarning}
			<div class="config-warning" role="alert">
				<Icon name="alert-triangle" size={16} />
				<span>Project not fully configured — checklist or standards missing. Some features may be limited.</span>
			</div>
		{/if}

		<main class="content">
			{@render children()}
		</main>

		<!-- Persistent bottom chat bar -->
		<BottomChatBar />
		<Toast />
	</div>
</div>

<style>
	:global(.spin) {
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		from {
			transform: rotate(0deg);
		}
		to {
			transform: rotate(360deg);
		}
	}

	.app-layout {
		display: flex;
		height: 100vh;
		overflow: hidden;
	}

	.main-area {
		flex: 1;
		display: flex;
		flex-direction: column;
		overflow: hidden;
	}

	.content {
		flex: 1;
		overflow: auto;
	}

	.config-warning {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-4);
		background: var(--color-warning-muted);
		color: var(--color-warning);
		font-size: var(--font-size-xs);
		border-bottom: 1px solid var(--color-warning);
	}

	/* Mobile hamburger button - hidden on desktop */
	.hamburger-btn {
		display: none;
		position: fixed;
		top: var(--space-3);
		left: var(--space-3);
		z-index: 50;
		width: 40px;
		height: 40px;
		padding: 0;
		border: none;
		background: var(--color-bg-secondary);
		color: var(--color-text-primary);
		border-radius: var(--radius-md);
		box-shadow: var(--shadow-md);
		cursor: pointer;
		align-items: center;
		justify-content: center;
	}

	.hamburger-btn:hover {
		background: var(--color-bg-tertiary);
	}

	/* Mobile sidebar backdrop */
	.sidebar-backdrop {
		display: none;
		position: fixed;
		inset: 0;
		z-index: 99;
		background: rgba(0, 0, 0, 0.5);
		border: none;
		cursor: pointer;
	}

	@media (max-width: 768px) {
		.hamburger-btn {
			display: flex;
		}

		.sidebar-backdrop {
			display: block;
		}

		.main-area {
			/* Add top padding for hamburger button */
			padding-top: calc(40px + var(--space-3) * 2);
		}
	}
</style>
