<script lang="ts">
	/**
	 * ScenarioDetail - Displays a single Given/When/Then behavioral contract.
	 *
	 * Compact enough to be embedded inline within RequirementPanel.
	 * Shows BDD clauses and status badge clearly.
	 */

	import Icon from '$lib/components/shared/Icon.svelte';
	import type { Scenario } from '$lib/types/scenario';
	import { getScenarioStatusInfo } from '$lib/types/scenario';

	interface Props {
		scenario: Scenario;
		requirementTitle?: string;
	}

	let { scenario, requirementTitle }: Props = $props();

	const statusInfo = $derived(getScenarioStatusInfo(scenario.status));

	function statusClass(color: string): string {
		switch (color) {
			case 'green':
				return 'badge-success';
			case 'red':
				return 'badge-error';
			case 'orange':
				return 'badge-warning';
			default:
				return 'badge-neutral';
		}
	}
</script>

<div class="scenario-detail" data-status={scenario.status}>
	<div class="scenario-header">
		{#if requirementTitle}
			<div class="scenario-meta">
				<span class="requirement-link">
					<Icon name="link" size={12} />
					{requirementTitle}
				</span>
			</div>
		{/if}
		<span class="status-badge {statusClass(statusInfo.color)}">
			{statusInfo.label}
		</span>
	</div>

	<div class="bdd-clauses">
		<div class="clause">
			<span class="clause-keyword given">Given</span>
			<span class="clause-text">{scenario.given}</span>
		</div>
		<div class="clause">
			<span class="clause-keyword when">When</span>
			<span class="clause-text">{scenario.when}</span>
		</div>
		<div class="clause then-clause">
			<span class="clause-keyword then">Then</span>
			<div class="then-list">
				{#each scenario.then as outcome, i}
					<div class="then-item">
						{#if scenario.then.length > 1}
							<span class="then-index">{i + 1}.</span>
						{/if}
						<span class="clause-text">{outcome}</span>
					</div>
				{/each}
			</div>
		</div>
	</div>
</div>

<style>
	.scenario-detail {
		padding: var(--space-3);
		background: var(--color-bg-tertiary);
		border-radius: var(--radius-md);
		border: 1px solid var(--color-border);
	}

	.scenario-detail[data-status='failing'] {
		border-color: var(--color-error);
		background: var(--color-error-muted, rgba(239, 68, 68, 0.05));
	}

	.scenario-detail[data-status='passing'] {
		border-color: var(--color-success);
	}

	.scenario-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: var(--space-3);
	}

	.scenario-meta {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.requirement-link {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		font-size: var(--font-size-xs);
		color: var(--color-text-muted);
	}

	.status-badge {
		display: inline-flex;
		align-items: center;
		padding: 2px var(--space-2);
		border-radius: var(--radius-full);
		font-size: 10px;
		font-weight: var(--font-weight-medium);
	}

	.badge-success {
		background: var(--color-success-muted);
		color: var(--color-success);
	}

	.badge-error {
		background: var(--color-error-muted);
		color: var(--color-error);
	}

	.badge-warning {
		background: var(--color-warning-muted);
		color: var(--color-warning);
	}

	.badge-neutral {
		background: var(--color-bg-secondary);
		color: var(--color-text-muted);
	}

	.bdd-clauses {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}

	.clause {
		display: flex;
		gap: var(--space-2);
		align-items: baseline;
	}

	.clause-keyword {
		flex-shrink: 0;
		width: 44px;
		font-size: var(--font-size-xs);
		font-weight: var(--font-weight-semibold);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		font-family: var(--font-family-mono);
	}

	.clause-keyword.given {
		color: var(--color-accent);
	}

	.clause-keyword.when {
		color: var(--color-warning);
	}

	.clause-keyword.then {
		color: var(--color-success);
	}

	.clause-text {
		font-size: var(--font-size-sm);
		color: var(--color-text-primary);
		line-height: var(--line-height-relaxed);
	}

	.then-clause {
		align-items: flex-start;
	}

	.then-list {
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
	}

	.then-item {
		display: flex;
		gap: var(--space-1);
	}

	.then-index {
		font-size: var(--font-size-xs);
		color: var(--color-text-muted);
		font-family: var(--font-family-mono);
		flex-shrink: 0;
	}
</style>
