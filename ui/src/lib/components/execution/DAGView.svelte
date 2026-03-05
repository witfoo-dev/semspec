<script lang="ts">
	/**
	 * DAGView — DAG execution graph visualization.
	 *
	 * Renders task nodes as cards arranged by dependency level (topological layers).
	 * Shows dependency edges via SVG arrows. Nodes are color-coded by status.
	 * Optionally shows which agent loop is executing each node.
	 */

	import Icon from '../shared/Icon.svelte';
	import type { DAGExecution, DAGNode } from '$lib/types/execution';
	import { getDAGNodeStatusClass } from '$lib/types/execution';

	interface Props {
		execution: DAGExecution;
		/** Callback when a node is clicked */
		onNodeSelect?: (node: DAGNode) => void;
		/** Currently selected node ID */
		selectedNodeId?: string;
	}

	let { execution, onNodeSelect, selectedNodeId }: Props = $props();

	// Compute topological layers for layout
	const layers = $derived.by(() => {
		const nodes = execution.nodes;
		if (nodes.length === 0) return [];

		// Assign each node to a layer based on longest dependency chain
		const depthMap = new Map<string, number>();

		function computeDepth(nodeId: string, visited = new Set<string>()): number {
			if (depthMap.has(nodeId)) return depthMap.get(nodeId)!;
			if (visited.has(nodeId)) return 0; // cycle guard

			const node = nodes.find((n) => n.id === nodeId);
			if (!node || node.dependsOn.length === 0) {
				depthMap.set(nodeId, 0);
				return 0;
			}

			visited.add(nodeId);
			const maxParentDepth = Math.max(
				...node.dependsOn.map((dep) => computeDepth(dep, new Set(visited)))
			);
			const depth = maxParentDepth + 1;
			depthMap.set(nodeId, depth);
			return depth;
		}

		nodes.forEach((n) => computeDepth(n.id));

		// Group nodes by layer
		const maxLayer = Math.max(...nodes.map((n) => depthMap.get(n.id) ?? 0));
		const result: DAGNode[][] = Array.from({ length: maxLayer + 1 }, () => []);
		nodes.forEach((n) => {
			const layer = depthMap.get(n.id) ?? 0;
			result[layer].push(n);
		});

		return result;
	});

	function getStatusIcon(status: DAGNode['status']): string {
		switch (status) {
			case 'pending':
				return 'circle';
			case 'running':
				return 'loader';
			case 'completed':
				return 'check-circle';
			case 'failed':
				return 'x-circle';
		}
	}

	function truncatePrompt(prompt: string, maxLen = 80): string {
		return prompt.length > maxLen ? prompt.slice(0, maxLen) + '...' : prompt;
	}

	function formatRole(role: string): string {
		return role
			.split(/[-_]/)
			.map((w) => w.charAt(0).toUpperCase() + w.slice(1))
			.join(' ');
	}

	const overallStatusClass = $derived.by(() => {
		switch (execution.status) {
			case 'executing': return 'warning';
			case 'complete': return 'success';
			case 'failed': return 'error';
		}
	});
</script>

<div class="dag-view">
	<div class="dag-header">
		<div class="dag-title">
			<Icon name="git-branch" size={16} />
			<span>Execution Graph</span>
			<span class="execution-id">{execution.executionId.slice(0, 8)}</span>
		</div>
		<span class="status-badge badge-{overallStatusClass}">{execution.status}</span>
	</div>

	{#if execution.nodes.length === 0}
		<div class="empty-state">
			<Icon name="git-branch" size={24} />
			<span>No execution nodes</span>
			<p class="empty-hint">Tasks will appear here once execution begins</p>
		</div>
	{:else}
		<div class="dag-canvas" aria-label="Execution DAG nodes">
			{#each layers as layer, layerIndex}
				<div class="dag-layer">
					{#if layerIndex > 0}
						<div class="layer-connector" aria-hidden="true">
							<Icon name="chevron-right" size={16} />
						</div>
					{/if}

					<div class="layer-nodes">
						{#each layer as node (node.id)}
							{@const statusClass = getDAGNodeStatusClass(node.status)}
							{@const isSelected = selectedNodeId === node.id}
							<button
								class="dag-node status-{statusClass}"
								class:selected={isSelected}
								onclick={() => onNodeSelect?.(node)}
								aria-label="{node.role}: {node.prompt}"
								aria-pressed={isSelected}
							>
								<div class="node-header">
									<span class="node-status-icon status-{statusClass}">
										<Icon
											name={getStatusIcon(node.status)}
											size={14}
											class={node.status === 'running' ? 'spin' : ''}
										/>
									</span>
									<span class="node-role">{formatRole(node.role)}</span>
									<span class="node-id">{node.id.slice(0, 6)}</span>
								</div>

								<p class="node-prompt">{truncatePrompt(node.prompt)}</p>

								{#if node.dependsOn.length > 0}
									<div class="node-deps">
										<Icon name="git-branch" size={10} />
										<span>depends on {node.dependsOn.length}</span>
									</div>
								{/if}

								{#if node.loopId}
									<div class="node-loop">
										<Icon name="activity" size={10} />
										<span class="loop-ref">{node.loopId.slice(0, 8)}</span>
									</div>
								{/if}
							</button>
						{/each}
					</div>
				</div>
			{/each}
		</div>

		<!-- Summary bar -->
		<div class="dag-summary">
			{#each (['pending', 'running', 'completed', 'failed'] as const) as status}
				{@const count = execution.nodes.filter((n) => n.status === status).length}
				{#if count > 0}
					{@const cls = getDAGNodeStatusClass(status)}
					<span class="summary-item">
						<span class="summary-dot dot-{cls}"></span>
						<span>{count} {status}</span>
					</span>
				{/if}
			{/each}
		</div>
	{/if}
</div>

<style>
	.dag-view {
		display: flex;
		flex-direction: column;
		background: var(--color-bg-secondary);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-lg);
		overflow: hidden;
	}

	.dag-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: var(--space-3) var(--space-4);
		background: var(--color-bg-tertiary);
		border-bottom: 1px solid var(--color-border);
	}

	.dag-title {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		font-size: var(--font-size-sm);
		font-weight: var(--font-weight-semibold);
		color: var(--color-text-primary);
	}

	.execution-id {
		font-family: var(--font-family-mono);
		font-size: var(--font-size-xs);
		color: var(--color-text-muted);
		font-weight: var(--font-weight-normal);
	}

	.status-badge {
		font-size: var(--font-size-xs);
		font-weight: var(--font-weight-medium);
		padding: 2px 8px;
		border-radius: var(--radius-full);
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
		background: var(--color-bg-elevated);
		color: var(--color-text-secondary);
	}

	.empty-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-8);
		color: var(--color-text-muted);
		text-align: center;
	}

	.empty-hint {
		font-size: var(--font-size-xs);
		margin: 0;
	}

	.dag-canvas {
		display: flex;
		align-items: flex-start;
		gap: 0;
		padding: var(--space-4);
		overflow-x: auto;
		overflow-y: auto;
		min-height: 200px;
	}

	.dag-layer {
		display: flex;
		align-items: center;
		gap: 0;
		flex-shrink: 0;
	}

	.layer-connector {
		display: flex;
		align-items: center;
		padding: 0 var(--space-2);
		color: var(--color-text-muted);
	}

	.layer-nodes {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}

	.dag-node {
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
		width: 200px;
		padding: var(--space-3);
		background: var(--color-bg-primary);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-md);
		cursor: pointer;
		text-align: left;
		transition: all var(--transition-fast);
	}

	.dag-node:hover {
		border-color: var(--color-accent);
		background: var(--color-bg-tertiary);
	}

	.dag-node.selected {
		border-color: var(--color-accent);
		background: var(--color-accent-muted);
	}

	/* Status-colored left border */
	.dag-node.status-pending {
		border-left: 3px solid var(--color-text-muted);
	}

	.dag-node.status-warning {
		border-left: 3px solid var(--color-warning);
	}

	.dag-node.status-success {
		border-left: 3px solid var(--color-success);
	}

	.dag-node.status-error {
		border-left: 3px solid var(--color-error);
	}

	.node-header {
		display: flex;
		align-items: center;
		gap: var(--space-1);
	}

	.node-status-icon {
		display: flex;
		align-items: center;
		flex-shrink: 0;
	}

	.node-status-icon.status-pending { color: var(--color-text-muted); }
	.node-status-icon.status-warning { color: var(--color-warning); }
	.node-status-icon.status-success { color: var(--color-success); }
	.node-status-icon.status-error { color: var(--color-error); }

	.node-role {
		flex: 1;
		font-size: var(--font-size-xs);
		font-weight: var(--font-weight-semibold);
		color: var(--color-text-primary);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.node-id {
		font-family: var(--font-family-mono);
		font-size: 10px;
		color: var(--color-text-muted);
		flex-shrink: 0;
	}

	.node-prompt {
		font-size: var(--font-size-xs);
		color: var(--color-text-secondary);
		line-height: 1.4;
		margin: 0;
		display: -webkit-box;
		-webkit-line-clamp: 3;
		line-clamp: 3;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}

	.node-deps,
	.node-loop {
		display: flex;
		align-items: center;
		gap: 3px;
		font-size: 10px;
		color: var(--color-text-muted);
	}

	.loop-ref {
		font-family: var(--font-family-mono);
	}

	.dag-summary {
		display: flex;
		align-items: center;
		gap: var(--space-4);
		padding: var(--space-2) var(--space-4);
		border-top: 1px solid var(--color-border);
		background: var(--color-bg-tertiary);
		flex-wrap: wrap;
	}

	.summary-item {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		font-size: var(--font-size-xs);
		color: var(--color-text-secondary);
	}

	.summary-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		flex-shrink: 0;
	}

	.dot-neutral { background: var(--color-text-muted); }
	.dot-warning { background: var(--color-warning); }
	.dot-success { background: var(--color-success); }
	.dot-error { background: var(--color-error); }

</style>
