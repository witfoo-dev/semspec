<script lang="ts">
	import Icon from './Icon.svelte';
	import { computeAttentionItems } from '$lib/stores/attention.svelte';
	import { sidebarStore } from '$lib/stores/sidebar.svelte';
	import type { PlanWithStatus } from '$lib/types/plan';
	import type { Loop } from '$lib/types';

	interface Props {
		currentPath: string;
		plans?: PlanWithStatus[];
		loops?: Loop[];
		activeLoopCount: number;
	}

	let { currentPath, plans = [], loops = [], activeLoopCount }: Props = $props();

	// Close sidebar on navigation (mobile)
	$effect(() => {
		currentPath;
		sidebarStore.close();
	});

	const navItems = [
		{ path: '/board', icon: 'layout-grid', label: 'Board' },
		{ path: '/plans', icon: 'git-pull-request', label: 'Plans' },
		{ path: '/activity', icon: 'activity', label: 'Activity' },
		{ path: '/trajectories', icon: 'git-branch', label: 'Trajectories' },
		{ path: '/workspace', icon: 'folder', label: 'Workspace' },
		{ path: '/settings', icon: 'settings', label: 'Settings' }
	];

	const attentionCount = $derived(computeAttentionItems(plans, loops).length);

	function isActive(path: string): boolean {
		// Board is homepage, so highlight it for root path too
		if (path === '/board') return currentPath === '/' || currentPath.startsWith('/board');
		return currentPath.startsWith(path);
	}
</script>

<aside class="sidebar" class:open={sidebarStore.isOpen}>
	<div class="sidebar-header">
		<span class="logo">SemSpec</span>
		<button
			class="close-btn"
			onclick={() => sidebarStore.close()}
			aria-label="Close navigation"
		>
			<Icon name="x" size={20} />
		</button>
	</div>

	<nav class="sidebar-nav" aria-label="Main navigation">
		{#each navItems as item}
			<a
				href={item.path}
				class="nav-item"
				class:active={isActive(item.path)}
				aria-current={isActive(item.path) ? 'page' : undefined}
			>
				<Icon name={item.icon} size={20} />
				<span>{item.label}</span>

				{#if item.path === '/board' && attentionCount > 0}
					<span class="badge" aria-label="{attentionCount} items need attention">
						{attentionCount}
					</span>
				{/if}

				{#if item.path === '/activity' && activeLoopCount > 0}
					<span class="badge badge-muted" aria-label="{activeLoopCount} active loops">
						{activeLoopCount}
					</span>
				{/if}
			</a>
		{/each}
	</nav>
</aside>

<style>
	.sidebar {
		width: var(--sidebar-width);
		height: 100%;
		background: var(--color-bg-secondary);
		border-right: 1px solid var(--color-border);
		display: flex;
		flex-direction: column;
		flex-shrink: 0;
	}

	.sidebar-header {
		padding: var(--space-4);
		border-bottom: 1px solid var(--color-border);
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.logo {
		font-size: var(--font-size-xl);
		font-weight: var(--font-weight-semibold);
		color: var(--color-text-primary);
	}

	.close-btn {
		display: none;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		padding: 0;
		border: none;
		background: transparent;
		color: var(--color-text-muted);
		border-radius: var(--radius-sm);
		cursor: pointer;
	}

	.close-btn:hover {
		background: var(--color-bg-tertiary);
		color: var(--color-text-primary);
	}

	/* Mobile: sidebar is hidden by default, slides in when open */
	@media (max-width: 768px) {
		.sidebar {
			position: fixed;
			top: 0;
			left: 0;
			z-index: 100;
			transform: translateX(-100%);
			transition: transform 0.2s ease;
			box-shadow: var(--shadow-lg);
		}

		.sidebar.open {
			transform: translateX(0);
		}

		.close-btn {
			display: flex;
		}
	}

	.sidebar-nav {
		flex: 1;
		padding: var(--space-2);
	}

	.nav-item {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-3) var(--space-3);
		color: var(--color-text-secondary);
		border-radius: var(--radius-md);
		text-decoration: none;
		transition: all var(--transition-fast);
	}

	.nav-item:hover {
		background: var(--color-bg-tertiary);
		color: var(--color-text-primary);
		text-decoration: none;
	}

	.nav-item.active {
		background: var(--color-accent-muted);
		color: var(--color-accent);
	}

	.nav-item .badge {
		margin-left: auto;
		background: var(--color-warning);
		color: var(--color-bg-primary);
		font-size: var(--font-size-xs);
		font-weight: var(--font-weight-semibold);
		padding: 2px 6px;
		border-radius: var(--radius-full);
	}

	.badge-muted {
		background: var(--color-bg-tertiary);
		color: var(--color-text-muted);
	}
</style>
