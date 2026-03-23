<script lang="ts">
	/**
	 * E2E test harness for execution components.
	 *
	 * This page is only used by Playwright tests. It renders AgentTree, DAGView,
	 * LoopDetail, and RetrospectiveView with data loaded from the execution APIs
	 * so that each component can be exercised in isolation with mocked responses.
	 *
	 * Navigation:
	 *   GET /e2e-test/execution?scenario=<name>
	 *
	 * Scenarios:
	 *   agent-tree          — AgentTree with 3-node hierarchy
	 *   agent-tree-empty    — AgentTree with no loops
	 *   dag-view            — DAGView with 3-node DAG in executing state
	 *   dag-view-empty      — DAGView with zero nodes
	 *   loop-detail         — LoopDetail panel open for orchestrator loop
	 *   retro-view          — RetrospectiveView with 2 requirements/3 tasks
	 *   retro-view-empty    — RetrospectiveView with no phases
	 */

	import { page } from '$app/state';
	import { onMount } from 'svelte';

	// No dev guard — this route is only used by Playwright tests and is not
	// linked from the main navigation. Safe to render in production builds.
	import AgentTree from '$lib/components/execution/AgentTree.svelte';
	import DAGView from '$lib/components/execution/DAGView.svelte';
	import LoopDetail from '$lib/components/execution/LoopDetail.svelte';
	import RetrospectiveView from '$lib/components/execution/RetrospectiveView.svelte';
	import { fetchAgentTree, fetchDAGExecution, fetchRetrospective } from '$lib/api/execution';
	import type { AgentLoop, DAGExecution, RetrospectivePhase } from '$lib/types/execution';

	const scenario = $derived(page.url.searchParams.get('scenario') ?? '');

	// ---- AgentTree state ----
	let agentLoops = $state<AgentLoop[]>([]);
	let selectedLoopId = $state<string | undefined>(undefined);
	let selectedLoop = $state<AgentLoop | undefined>(undefined);

	// ---- DAGView state ----
	let dagExecution = $state<DAGExecution | null>(null);
	let selectedNodeId = $state<string | undefined>(undefined);

	// ---- RetrospectiveView state ----
	let retroPhases = $state<RetrospectivePhase[]>([]);

	// ---- Loading/error state ----
	let loading = $state(true);
	let error = $state<string | null>(null);

	onMount(async () => {
		try {
			if (scenario.startsWith('agent-tree')) {
				const loops = await fetchAgentTree('test-plan');
				agentLoops = loops;
			} else if (scenario.startsWith('dag-view')) {
				const exec = await fetchDAGExecution('exec-test');
				dagExecution = exec;
			} else if (scenario === 'loop-detail') {
				const loops = await fetchAgentTree('test-plan');
				agentLoops = loops;
				// Auto-open the first loop
				if (loops.length > 0) {
					selectedLoop = loops[0];
					selectedLoopId = loops[0].loopId;
				}
			} else if (scenario.startsWith('retro-view')) {
				const phases = await fetchRetrospective('test-plan');
				retroPhases = phases;
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load data';
		} finally {
			loading = false;
		}
	});

	function handleLoopSelect(loop: AgentLoop) {
		selectedLoopId = loop.loopId;
		selectedLoop = loop;
	}

	function handleCloseDetail() {
		selectedLoop = undefined;
		selectedLoopId = undefined;
	}

	function handleNodeSelect(node: import('$lib/types/execution').DAGNode) {
		selectedNodeId = node.id;
	}
</script>

<div class="test-harness" data-scenario={scenario} data-loading={loading}>
	{#if loading}
		<div class="harness-loading">Loading...</div>
	{:else if error}
		<div class="harness-error">{error}</div>
	{:else if scenario.startsWith('agent-tree')}
		<div class="harness-agent-tree">
			<AgentTree
				loops={agentLoops}
				{selectedLoopId}
				onLoopSelect={handleLoopSelect}
			/>
		</div>
	{:else if scenario.startsWith('dag-view') && dagExecution}
		<div class="harness-dag-view">
			<DAGView
				execution={dagExecution}
				{selectedNodeId}
				onNodeSelect={handleNodeSelect}
			/>
		</div>
	{:else if scenario === 'loop-detail' && selectedLoop}
		<div class="harness-loop-detail">
			<AgentTree
				loops={agentLoops}
				{selectedLoopId}
				onLoopSelect={handleLoopSelect}
			/>
			<LoopDetail loop={selectedLoop} onClose={handleCloseDetail} />
		</div>
	{:else if scenario.startsWith('retro-view')}
		<div class="harness-retro-view">
			<RetrospectiveView phases={retroPhases} />
		</div>
	{:else}
		<div class="harness-unknown">Unknown scenario: {scenario}</div>
	{/if}
</div>

<style>
	.test-harness {
		padding: 16px;
		min-height: 100vh;
		background: var(--color-bg-primary);
	}

	.harness-loading,
	.harness-error,
	.harness-unknown {
		padding: 32px;
		text-align: center;
		color: var(--color-text-muted);
	}

	.harness-error {
		color: var(--color-error);
	}

	.harness-agent-tree,
	.harness-dag-view,
	.harness-retro-view {
		max-width: 900px;
		margin: 0 auto;
	}

	.harness-loop-detail {
		max-width: 600px;
		margin: 0 auto;
	}
</style>
