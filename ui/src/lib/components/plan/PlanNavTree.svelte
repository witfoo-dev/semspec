<script lang="ts">
	/**
	 * PlanNavTree - Progressive disclosure navigation tree for plans.
	 *
	 * Shows plan → requirements → scenarios hierarchy with:
	 * - Expand/collapse for requirements
	 * - Scenario nodes within expanded requirements
	 * - Status indicators and selection highlighting
	 * - Keyboard navigation support
	 */

	import Icon from '$lib/components/shared/Icon.svelte';
	import type { PlanWithStatus } from '$lib/types/plan';
	import type { Requirement, RequirementStatus } from '$lib/types/requirement';
	import type { Scenario, ScenarioStatus } from '$lib/types/scenario';
	import type { PlanSelection, SelectionType } from '$lib/stores/planSelection.svelte';

	interface Props {
		plan: PlanWithStatus;
		requirements: Requirement[];
		scenariosByReq: Record<string, Scenario[]>;
		selection: PlanSelection | null;
		onSelect: (selection: PlanSelection) => void;
		onExpandRequirement?: (reqId: string) => void;
	}

	let { plan, requirements, scenariosByReq, selection, onSelect, onExpandRequirement }: Props = $props();

	// Track which requirements are manually expanded
	let manuallyExpanded = $state<Set<string>>(new Set());

	function isExpanded(reqId: string): boolean {
		if (manuallyExpanded.has(reqId)) return true;
		if (selection?.requirementId === reqId) return true;
		return false;
	}

	function toggleRequirement(reqId: string): void {
		const newExpanded = new Set(manuallyExpanded);
		if (newExpanded.has(reqId)) {
			newExpanded.delete(reqId);
		} else {
			newExpanded.add(reqId);
			onExpandRequirement?.(reqId);
		}
		manuallyExpanded = newExpanded;
	}

	function handleSelectPlan(): void {
		onSelect({ type: 'plan', planSlug: plan.slug });
	}

	function handleSelectRequirement(reqId: string): void {
		onSelect({ type: 'requirement', planSlug: plan.slug, requirementId: reqId });
		// Auto-expand when selected
		if (!manuallyExpanded.has(reqId)) {
			const newExpanded = new Set(manuallyExpanded);
			newExpanded.add(reqId);
			manuallyExpanded = newExpanded;
			onExpandRequirement?.(reqId);
		}
	}

	function handleSelectScenario(reqId: string, scenarioId: string): void {
		onSelect({ type: 'scenario', planSlug: plan.slug, requirementId: reqId, scenarioId });
	}

	function isSelected(type: SelectionType, id?: string): boolean {
		if (!selection) return false;
		if (type === 'plan') return selection.type === 'plan';
		if (type === 'requirement') return (selection.type === 'requirement' || selection.type === 'scenario') && selection.requirementId === id;
		if (type === 'scenario') return selection.type === 'scenario' && selection.scenarioId === id;
		return false;
	}

	function getReqStatusIcon(status: RequirementStatus): string {
		switch (status) {
			case 'active':
				return 'check-circle';
			case 'deprecated':
				return 'archive';
			case 'superseded':
				return 'arrow-right-circle';
			default:
				return 'circle';
		}
	}

	function getScenarioStatusIcon(status: ScenarioStatus): string {
		switch (status) {
			case 'passing':
				return 'check';
			case 'failing':
				return 'x';
			case 'skipped':
				return 'skip-forward';
			default:
				return 'circle';
		}
	}

	function getScenarioCount(reqId: string): number {
		return scenariosByReq[reqId]?.length ?? 0;
	}

	function getPassingCount(reqId: string): number {
		return scenariosByReq[reqId]?.filter((s) => s.status === 'passing').length ?? 0;
	}

	const showRequirements = $derived(plan.approved && requirements.length > 0);

	// Cascade status text when requirements are being generated
	const cascadeStatus = $derived.by(() => {
		if (!plan.approved) return '';
		if (plan.stage === 'approved' && requirements.length === 0) return 'Generating requirements...';
		if (plan.stage === 'requirements_generated') return 'Generating scenarios...';
		return '';
	});

	function handleKeyDown(e: KeyboardEvent, action: () => void): void {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			action();
		}
	}

	function formatScenarioLabel(scenario: Scenario): string {
		const text = `When ${scenario.when}`;
		return text.length > 40 ? text.slice(0, 40) + '...' : text;
	}
</script>

<div class="plan-nav-tree" role="tree" aria-label="Plan structure">
	<!-- Plan root node -->
	<button
		class="tree-node plan-node"
		class:selected={isSelected('plan')}
		onclick={handleSelectPlan}
		onkeydown={(e) => handleKeyDown(e, handleSelectPlan)}
		role="treeitem"
		aria-selected={isSelected('plan')}
		aria-level={1}
	>
		<span class="node-icon plan-icon">
			<Icon name="file-text" size={16} />
		</span>
		<span class="node-label">{plan.title || plan.slug}</span>
		{#if !plan.approved}
			<span class="node-badge draft">Draft</span>
		{/if}
	</button>

	<!-- Requirement nodes -->
	{#if showRequirements}
		<div class="tree-children" role="group">
			{#each requirements as req (req.id)}
				{@const expanded = isExpanded(req.id)}
				{@const scenarios = scenariosByReq[req.id] ?? []}
				{@const hasScenarios = scenarios.length > 0}

				<div class="tree-branch">
					<div class="req-row">
						{#if hasScenarios || expanded}
							<button
								type="button"
								class="expand-btn"
								onclick={() => toggleRequirement(req.id)}
								aria-expanded={expanded}
								aria-label={expanded ? 'Collapse requirement' : 'Expand requirement'}
							>
								<Icon name={expanded ? 'chevron-down' : 'chevron-right'} size={14} />
							</button>
						{:else}
							<button
								type="button"
								class="expand-btn"
								onclick={() => toggleRequirement(req.id)}
								aria-label="Expand requirement"
							>
								<Icon name="chevron-right" size={14} />
							</button>
						{/if}

						<button
							class="tree-node req-node"
							class:selected={isSelected('requirement', req.id)}
							onclick={() => handleSelectRequirement(req.id)}
							onkeydown={(e) => handleKeyDown(e, () => handleSelectRequirement(req.id))}
							role="treeitem"
							aria-selected={isSelected('requirement', req.id)}
							aria-level={2}
							aria-expanded={expanded}
						>
							<span class="node-icon req-status" data-status={req.status}>
								<Icon name={getReqStatusIcon(req.status)} size={14} />
							</span>
							<span class="node-label">{req.title}</span>
							{#if hasScenarios}
								<span class="scenario-count">
									{getPassingCount(req.id)}/{getScenarioCount(req.id)}
								</span>
							{/if}
						</button>
					</div>

					<!-- Scenario nodes (if expanded) -->
					{#if expanded && hasScenarios}
						<div class="tree-children scenarios" role="group">
							{#each scenarios as scenario (scenario.id)}
								<button
									class="tree-node scenario-node"
									class:selected={isSelected('scenario', scenario.id)}
									onclick={() => handleSelectScenario(req.id, scenario.id)}
									onkeydown={(e) => handleKeyDown(e, () => handleSelectScenario(req.id, scenario.id))}
									role="treeitem"
									aria-selected={isSelected('scenario', scenario.id)}
									aria-level={3}
								>
									<span class="node-icon scenario-status" data-status={scenario.status}>
										<Icon name={getScenarioStatusIcon(scenario.status)} size={12} />
									</span>
									<span class="node-label">{formatScenarioLabel(scenario)}</span>
								</button>
							{/each}
						</div>
					{/if}
				</div>
			{/each}
		</div>
	{:else if cascadeStatus}
		<div class="tree-empty">
			<Icon name="loader" size={14} />
			<span>{cascadeStatus}</span>
		</div>
	{:else if plan.approved && requirements.length === 0}
		<div class="tree-empty">
			<Icon name="list-checks" size={14} />
			<span>No requirements yet</span>
		</div>
	{/if}
</div>

<style>
	.plan-nav-tree {
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
		padding: var(--space-2);
	}

	.tree-node {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		width: 100%;
		padding: var(--space-2) var(--space-3);
		background: transparent;
		border: 1px solid transparent;
		border-radius: var(--radius-md);
		text-align: left;
		font-size: var(--font-size-sm);
		color: var(--color-text-primary);
		cursor: pointer;
		transition: all var(--transition-fast);
	}

	.tree-node:hover {
		background: var(--color-bg-tertiary);
	}

	.tree-node:focus-visible {
		outline: 2px solid var(--color-accent);
		outline-offset: -2px;
	}

	.tree-node.selected {
		background: var(--color-accent-muted);
		border-color: var(--color-accent);
	}

	.plan-node {
		font-weight: var(--font-weight-semibold);
	}

	.plan-icon {
		color: var(--color-accent);
	}

	/* Requirement row with expand button */
	.req-row {
		display: flex;
		align-items: center;
		gap: var(--space-1);
	}

	.expand-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 20px;
		height: 20px;
		padding: 0;
		background: transparent;
		border: none;
		border-radius: var(--radius-sm);
		color: var(--color-text-muted);
		cursor: pointer;
		flex-shrink: 0;
	}

	.expand-btn:hover {
		background: var(--color-bg-tertiary);
		color: var(--color-text-primary);
	}

	.req-node {
		flex: 1;
	}

	/* Tree structure */
	.tree-children {
		margin-left: var(--space-4);
		padding-left: var(--space-2);
		border-left: 1px solid var(--color-border);
	}

	.tree-children.scenarios {
		margin-left: calc(20px + var(--space-1));
	}

	.tree-branch {
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
	}

	.scenario-node {
		padding: var(--space-1) var(--space-2);
		font-size: var(--font-size-xs);
	}

	.scenario-node .node-label {
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	/* Node icon styling */
	.node-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}

	.node-label {
		flex: 1;
		min-width: 0;
	}

	/* Status colors */
	.req-status[data-status='active'] {
		color: var(--color-success);
	}

	.req-status[data-status='deprecated'] {
		color: var(--color-text-muted);
	}

	.req-status[data-status='superseded'] {
		color: var(--color-warning);
	}

	.scenario-status[data-status='passing'] {
		color: var(--color-success);
	}

	.scenario-status[data-status='failing'] {
		color: var(--color-error);
	}

	.scenario-status[data-status='skipped'] {
		color: var(--color-warning);
	}

	.scenario-status[data-status='pending'] {
		color: var(--color-text-muted);
	}

	/* Badges and counts */
	.node-badge {
		font-size: var(--font-size-xs);
		padding: 1px var(--space-1);
		border-radius: var(--radius-sm);
		flex-shrink: 0;
	}

	.node-badge.draft {
		background: var(--color-warning-muted, rgba(245, 158, 11, 0.1));
		color: var(--color-warning);
	}

	.scenario-count {
		font-size: var(--font-size-xs);
		color: var(--color-text-muted);
		flex-shrink: 0;
	}

	/* Empty state */
	.tree-empty {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-3);
		margin-left: var(--space-4);
		font-size: var(--font-size-xs);
		color: var(--color-text-muted);
	}

	/* Loader animation */
	.tree-empty :global(svg) {
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
</style>
