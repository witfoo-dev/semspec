<script lang="ts">
	/**
	 * AgentTree — Recursive agent hierarchy visualization.
	 *
	 * Renders a tree of AgentLoop nodes showing parent→child relationships.
	 * Each node shows role, model, status badge, and depth indicator.
	 * Nodes are expandable/collapsible; clicking a node emits onLoopSelect.
	 */

	import Icon from '../shared/Icon.svelte';
	import AgentTree from './AgentTree.svelte';
	import type { AgentLoop } from '$lib/types/execution';
	import { getAgentLoopStatusClass } from '$lib/types/execution';

	interface Props {
		loops: AgentLoop[];
		/** Callback when a loop node is clicked for detail view */
		onLoopSelect?: (loop: AgentLoop) => void;
		/** Currently selected loop ID */
		selectedLoopId?: string;
		/** Depth offset for nested rendering (internal use) */
		depthOffset?: number;
	}

	let { loops, onLoopSelect, selectedLoopId, depthOffset = 0 }: Props = $props();

	// Track which nodes are expanded (keyed by loopId)
	let expanded = $state<Record<string, boolean>>({});

	function toggleExpand(loopId: string) {
		expanded[loopId] = !expanded[loopId];
	}

	function isExpanded(loopId: string): boolean {
		if (loopId in expanded) return expanded[loopId];
		// Default: expanded for running loops, collapsed for completed/failed
		const loop = loops.find((l) => l.loopId === loopId);
		return loop?.status === 'running';
	}

	function getStatusIcon(status: AgentLoop['status']): string {
		switch (status) {
			case 'running':
				return 'loader';
			case 'completed':
				return 'check-circle';
			case 'failed':
				return 'x-circle';
		}
	}

	function formatRole(role: string): string {
		return role
			.split(/[-_]/)
			.map((w) => w.charAt(0).toUpperCase() + w.slice(1))
			.join(' ');
	}

	function formatModel(model: string): string {
		// Trim long model names: show last segment after '/'
		const parts = model.split('/');
		return parts[parts.length - 1];
	}

	function formatDuration(startedAt?: string, completedAt?: string): string | undefined {
		if (!startedAt) return undefined;
		const end = completedAt ? new Date(completedAt) : new Date();
		const start = new Date(startedAt);
		const ms = end.getTime() - start.getTime();
		if (ms < 1000) return `${ms}ms`;
		if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
		return `${Math.floor(ms / 60000)}m ${Math.floor((ms % 60000) / 1000)}s`;
	}
</script>

{#if loops.length === 0}
	<div class="empty-state">
		<Icon name="git-branch" size={24} />
		<span>No agent loops</span>
	</div>
{:else}
	<ul class="tree-list" role="tree" aria-label="Agent loop hierarchy">
		{#each loops as loop (loop.loopId)}
			{@const statusClass = getAgentLoopStatusClass(loop.status)}
			{@const hasChildren = loop.children.length > 0}
			{@const nodeExpanded = isExpanded(loop.loopId)}
			{@const isSelected = selectedLoopId === loop.loopId}
			{@const duration = formatDuration(loop.startedAt, loop.completedAt)}
			<li class="tree-node" role="treeitem" aria-expanded={hasChildren ? nodeExpanded : undefined} aria-selected={isSelected}>
				<div
					class="node-row"
					class:selected={isSelected}
					class:has-children={hasChildren}
					style:--depth={loop.depth + depthOffset}
				>
					<!-- Expand/collapse toggle -->
					<button
						class="expand-btn"
						class:visible={hasChildren}
						onclick={() => hasChildren && toggleExpand(loop.loopId)}
						aria-label={nodeExpanded ? 'Collapse children' : 'Expand children'}
						tabindex={hasChildren ? 0 : -1}
					>
						{#if hasChildren}
							<Icon name={nodeExpanded ? 'chevron-down' : 'chevron-right'} size={12} />
						{:else}
							<span class="leaf-dot" aria-hidden="true"></span>
						{/if}
					</button>

					<!-- Status indicator -->
					<span class="status-icon status-{statusClass}" title={loop.status}>
						<Icon name={getStatusIcon(loop.status)} size={14} class={loop.status === 'running' ? 'spin' : ''} />
					</span>

					<!-- Node content — clicking opens detail -->
					<button
						class="node-content"
						onclick={() => onLoopSelect?.(loop)}
						aria-label="View details for {formatRole(loop.role)}"
					>
						<span class="node-role">{formatRole(loop.role)}</span>
						<span class="node-model">{formatModel(loop.model)}</span>
						{#if loop.taskId}
							<span class="node-task" title="Task: {loop.taskId}">
								<Icon name="list-checks" size={10} />
								<span class="task-id">{loop.taskId.slice(0, 8)}</span>
							</span>
						{/if}
						{#if duration}
							<span class="node-duration">
								<Icon name="clock" size={10} />
								{duration}
							</span>
						{/if}
					</button>

					<!-- Status badge -->
					<span class="status-badge badge-{statusClass}">{loop.status}</span>
				</div>

				<!-- Recursive children -->
				{#if hasChildren && nodeExpanded}
					<div class="children-container">
						<AgentTree
							loops={loop.children}
							{onLoopSelect}
							{selectedLoopId}
							depthOffset={loop.depth + depthOffset + 1}
						/>
					</div>
				{/if}
			</li>
		{/each}
	</ul>
{/if}

<style>
	.empty-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-6);
		color: var(--color-text-muted);
		font-size: var(--font-size-sm);
		text-align: center;
	}

	.tree-list {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
	}

	.tree-node {
		display: flex;
		flex-direction: column;
	}

	.node-row {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		padding: var(--space-2) var(--space-3);
		padding-left: calc(var(--space-3) + var(--depth, 0) * 20px);
		border-radius: var(--radius-md);
		border: 1px solid transparent;
		transition: background-color var(--transition-fast);
	}

	.node-row:hover {
		background: var(--color-bg-tertiary);
	}

	.node-row.selected {
		background: var(--color-accent-muted);
		border-color: var(--color-accent);
	}

	.expand-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 18px;
		height: 18px;
		background: transparent;
		border: none;
		padding: 0;
		cursor: default;
		color: var(--color-text-muted);
		flex-shrink: 0;
	}

	.expand-btn.visible {
		cursor: pointer;
	}

	.expand-btn.visible:hover {
		color: var(--color-text-primary);
	}

	.leaf-dot {
		display: block;
		width: 4px;
		height: 4px;
		border-radius: 50%;
		background: var(--color-border);
		margin: auto;
	}

	.status-icon {
		display: flex;
		align-items: center;
		flex-shrink: 0;
	}

	.status-warning { color: var(--color-warning); }
	.status-success { color: var(--color-success); }
	.status-error { color: var(--color-error); }
	.status-neutral { color: var(--color-text-muted); }

	.node-content {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		flex: 1;
		min-width: 0;
		background: transparent;
		border: none;
		padding: 0;
		cursor: pointer;
		text-align: left;
		color: inherit;
	}

	.node-content:hover .node-role {
		color: var(--color-accent);
	}

	.node-role {
		font-size: var(--font-size-sm);
		font-weight: var(--font-weight-medium);
		color: var(--color-text-primary);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		transition: color var(--transition-fast);
	}

	.node-model {
		font-size: var(--font-size-xs);
		font-family: var(--font-family-mono);
		color: var(--color-text-muted);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		flex-shrink: 1;
	}

	.node-task,
	.node-duration {
		display: flex;
		align-items: center;
		gap: 2px;
		font-size: var(--font-size-xs);
		color: var(--color-text-muted);
		white-space: nowrap;
		flex-shrink: 0;
	}

	.task-id {
		font-family: var(--font-family-mono);
	}

	.status-badge {
		font-size: 10px;
		font-weight: var(--font-weight-medium);
		padding: 1px 6px;
		border-radius: var(--radius-full);
		white-space: nowrap;
		flex-shrink: 0;
	}

	.badge-warning {
		background: var(--color-warning-muted);
		color: var(--color-warning);
	}

	.badge-success {
		background: var(--color-success-muted);
		color: var(--color-success);
	}

	.badge-error {
		background: var(--color-error-muted);
		color: var(--color-error);
	}

	.badge-neutral {
		background: var(--color-bg-tertiary);
		color: var(--color-text-secondary);
	}

	.children-container {
		/* Indent line using border-left for visual hierarchy */
		margin-left: calc(var(--space-3) + 9px);
		padding-left: var(--space-3);
		border-left: 1px solid var(--color-border);
	}

</style>
