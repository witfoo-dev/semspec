<script lang="ts">
	import { page } from '$app/state';
	import { invalidate } from '$app/navigation';
	import Icon from '$lib/components/shared/Icon.svelte';
	import ModeIndicator from '$lib/components/board/ModeIndicator.svelte';
	import PipelineIndicator from '$lib/components/board/PipelineIndicator.svelte';
	import PlanDetailPanel from '$lib/components/plan/PlanDetailPanel.svelte';
	import RejectionBanner from '$lib/components/plan/RejectionBanner.svelte';
	import ActionBar from '$lib/components/plan/ActionBar.svelte';
	import { AgentPipelineView } from '$lib/components/pipeline';
	import { planSelectionStore } from '$lib/stores/planSelection.svelte';
	import { api } from '$lib/api/client';
	import { promotePlan, executePlan } from '$lib/actions/plans';
	import { derivePlanPipeline, getStageLabel } from '$lib/types/plan';
	import type { PageData } from './$types';

	interface Props {
		data: PageData;
	}

	let { data }: Props = $props();

	// All data comes from SvelteKit load function — no local $state copies
	const slug = $derived(page.params.slug);
	const plan = $derived(data.plan);
	const pipeline = $derived(plan ? derivePlanPipeline(plan) : null);
	const requirements = $derived(data.requirements);
	const scenariosByReq = $derived(data.scenariosByReq);
	const tasks = $derived(data.tasks);

	// Derived from load data — no local state needed
	const tasksByPhase = $derived.by(() => {
		const grouped: Record<string, typeof tasks> = {};
		for (const task of tasks) {
			const phaseId = task.phase_id ?? '__unassigned__';
			(grouped[phaseId] ??= []).push(task);
		}
		return grouped;
	});

	const activeRejection = $derived.by(() => {
		const rejectedTask = tasks.find((t) => t.rejection && t.status === 'in_progress');
		return rejectedTask ? { task: rejectedTask, rejection: rejectedTask.rejection! } : null;
	});

	// Cascade stages that need periodic refresh
	const isCascading = $derived(
		plan !== null && ['approved', 'requirements_generated', 'scenarios_generated'].includes(plan.stage)
	);

	// Auto-refresh during cascade — $effect recreates interval when isCascading changes
	$effect(() => {
		if (!isCascading) return;
		const interval = setInterval(() => invalidate('app:plans'), 5000);
		return () => clearInterval(interval);
	});

	// Sync plan selection store for child components that read it
	$effect(() => {
		if (slug) planSelectionStore.selectPlan(slug);
		return () => planSelectionStore.clear();
	});

	// Action handlers — all mutations invalidate to let the load function re-fetch
	let actionError = $state<string | null>(null);

	async function handlePromote() {
		if (!plan) return;
		actionError = null;
		try {
			await promotePlan(plan.slug);
		} catch (e) {
			actionError = e instanceof Error ? e.message : 'Failed to promote plan';
		}
	}

	async function handleExecute() {
		if (!plan) return;
		actionError = null;
		try {
			await executePlan(plan.slug);
		} catch (e) {
			actionError = e instanceof Error ? e.message : 'Failed to start execution';
		}
	}

	async function handleReplay() {
		if (!plan) return;
		actionError = null;
		try {
			await executePlan(plan.slug);
		} catch (e) {
			actionError = e instanceof Error ? e.message : 'Failed to replay';
		}
	}

	// All refresh handlers use invalidate — SvelteKit re-runs the load function
	async function handleRefresh() {
		await invalidate('app:plans');
	}

	// Mutation handlers that call API then invalidate
	async function handleDeleteRequirement(reqId: string) {
		if (!slug) return;
		await api.requirements.delete(slug, reqId);
		await invalidate('app:plans');
		planSelectionStore.selectPlan(slug);
	}

	async function handleApprovePhase(phaseId: string) {
		if (!slug) return;
		await api.phases.approve(slug, phaseId);
		await invalidate('app:plans');
	}

	async function handleRejectPhase(phaseId: string, reason: string) {
		if (!slug) return;
		await api.phases.reject(slug, phaseId, reason);
		await invalidate('app:plans');
	}

	async function handleApproveTask(taskId: string) {
		if (!slug) return;
		await api.tasks.approve(slug, taskId);
		await invalidate('app:plans');
	}

	async function handleRejectTask(taskId: string, reason: string) {
		if (!slug) return;
		await api.tasks.reject(slug, taskId, reason);
		await invalidate('app:plans');
	}
</script>

<svelte:head>
	<title>{plan?.slug || 'Plan'} - Semspec</title>
</svelte:head>

<div class="plan-detail">
	<header class="detail-header">
		<div class="header-left">
			<a href="/" class="back-link">
				<Icon name="chevron-left" size={16} />
				Back
			</a>
		</div>
		<div class="header-center">
			{#if plan}
				<h1 class="plan-title">{plan.title || plan.slug}</h1>
				<div class="plan-meta">
					<ModeIndicator approved={plan.approved} />
					<span class="plan-stage" data-stage={plan.stage}>
						{getStageLabel(plan.stage)}
					</span>
					{#if plan.github}
						<span class="separator">|</span>
						<a
							href={plan.github.epic_url}
							target="_blank"
							rel="noopener noreferrer"
							class="github-link"
						>
							<Icon name="external-link" size={14} />
							GH #{plan.github.epic_number}
						</a>
					{/if}
				</div>
			{/if}
		</div>
		<div class="header-right"></div>
	</header>

	{#if !plan}
		<div class="not-found">
			<Icon name="alert-circle" size={48} />
			<h2>Plan not found</h2>
			<p>The plan "{slug}" could not be found.</p>
			<a href="/" class="btn btn-primary">Back to Board</a>
		</div>
	{:else}
		<div class="plan-content">
			{#if plan.approved}
				<div class="pipeline-bar">
					<div class="pipeline-left">
						{#if pipeline}
							<PipelineIndicator
								plan={pipeline.plan}
								requirements={pipeline.requirements}
								execute={pipeline.execute}
							/>
						{/if}
					</div>
					<div class="pipeline-right">
						<ActionBar
							{plan}
							onPromote={handlePromote}
							onExecute={handleExecute}
							onReplay={handleReplay}
						/>
					</div>
				</div>
			{:else if !plan.approved && plan.goal}
				<div class="pipeline-bar">
					<div class="pipeline-left"></div>
					<div class="pipeline-right">
						<ActionBar
							{plan}
							onPromote={handlePromote}
							onExecute={handleExecute}
							onReplay={handleReplay}
						/>
					</div>
				</div>
			{/if}

			{#if plan.active_loops && plan.active_loops.length > 0}
				<div class="agent-pipeline-section">
					<AgentPipelineView slug={plan.slug} loops={plan.active_loops} />
				</div>
			{/if}

			{#if activeRejection}
				<RejectionBanner
					rejection={activeRejection.rejection}
					taskDescription={activeRejection.task.description}
				/>
			{/if}

			<div class="stage-content">
				<PlanDetailPanel
					selection={planSelectionStore.selection}
					{plan}
					phases={[]}
					{tasksByPhase}
					{requirements}
					{scenariosByReq}
					onRefreshPlan={handleRefresh}
					onRefreshPhases={handleRefresh}
					onRefreshTasks={handleRefresh}
					onRefreshRequirements={handleRefresh}
					onRefreshScenarios={handleRefresh}
					onDeleteRequirement={handleDeleteRequirement}
					onApprovePhase={handleApprovePhase}
					onRejectPhase={handleRejectPhase}
					onApproveTask={handleApproveTask}
					onRejectTask={handleRejectTask}
				/>
			</div>
		</div>
	{/if}
</div>

<style>
	.plan-detail {
		padding: var(--space-4);
		max-width: 1800px;
		margin: 0 auto;
		height: 100%;
		display: flex;
		flex-direction: column;
	}

	.detail-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		margin-bottom: var(--space-4);
		gap: var(--space-4);
		flex-shrink: 0;
	}

	.header-left {
		flex-shrink: 0;
		min-width: 80px;
	}

	.header-right {
		flex-shrink: 0;
		min-width: 80px;
	}

	.header-center {
		flex: 1;
		text-align: center;
	}

	.back-link {
		display: inline-flex;
		align-items: center;
		gap: var(--space-1);
		color: var(--color-text-muted);
		text-decoration: none;
		font-size: var(--font-size-sm);
	}

	.back-link:hover {
		color: var(--color-text-primary);
	}

	.plan-title {
		font-size: var(--font-size-xl);
		font-weight: var(--font-weight-semibold);
		color: var(--color-text-primary);
		margin: 0 0 var(--space-1);
	}

	.plan-meta {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: var(--space-3);
		font-size: var(--font-size-sm);
		color: var(--color-text-muted);
	}

	.plan-stage {
		font-size: var(--font-size-xs);
		padding: 2px var(--space-2);
		border-radius: var(--radius-sm);
		background: var(--color-bg-tertiary);
		color: var(--color-text-muted);
	}

	.plan-stage[data-stage='implementing'],
	.plan-stage[data-stage='executing'] {
		background: var(--color-accent-muted);
		color: var(--color-accent);
	}

	.plan-stage[data-stage='requirements_generated'],
	.plan-stage[data-stage='scenarios_generated'] {
		background: var(--color-accent-muted);
		color: var(--color-accent);
	}

	.plan-stage[data-stage='ready_for_execution'] {
		background: var(--color-success-muted, rgba(34, 197, 94, 0.15));
		color: var(--color-success);
	}

	.plan-stage[data-stage='needs_changes'],
	.plan-stage[data-stage='rejected'] {
		background: var(--color-warning-muted, rgba(234, 179, 8, 0.15));
		color: var(--color-warning, #eab308);
	}

	.plan-stage[data-stage='complete'] {
		background: var(--color-success-muted, rgba(34, 197, 94, 0.15));
		color: var(--color-success);
	}

	.plan-stage[data-stage='failed'] {
		background: var(--color-error-muted, rgba(239, 68, 68, 0.15));
		color: var(--color-error);
	}

	.separator {
		color: var(--color-border);
	}

	.github-link {
		display: inline-flex;
		align-items: center;
		gap: var(--space-1);
		color: var(--color-accent);
	}

	.not-found {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: var(--space-3);
		padding: var(--space-12) 0;
		color: var(--color-text-muted);
		text-align: center;
	}

	.not-found h2 {
		margin: 0;
		color: var(--color-text-primary);
	}

	.plan-content {
		flex: 1;
		min-height: 0;
		overflow-y: auto;
		padding: 0 var(--space-4) var(--space-4);
		max-width: 900px;
	}

	.stage-content {
		flex: 1;
		min-height: 0;
	}

	.pipeline-bar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-4);
		padding: var(--space-3) var(--space-4);
		background: var(--color-bg-tertiary);
		border-radius: var(--radius-lg);
		margin-bottom: var(--space-4);
		flex-shrink: 0;
	}

	.pipeline-left {
		flex: 1;
		min-width: 0;
	}

	.pipeline-right {
		flex-shrink: 0;
	}

	.agent-pipeline-section {
		margin-top: var(--space-4);
		padding-top: var(--space-4);
		border-top: 1px solid var(--color-border);
	}

	@media (max-width: 900px) {
		.detail-header {
			flex-direction: column;
			align-items: flex-start;
			gap: var(--space-2);
		}

		.header-center {
			text-align: left;
		}

		.plan-meta {
			justify-content: flex-start;
		}
	}

	.btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-4);
		border: none;
		border-radius: var(--radius-md);
		font-size: var(--font-size-sm);
		font-weight: var(--font-weight-medium);
		text-decoration: none;
		cursor: pointer;
		transition: all var(--transition-fast);
	}

	.btn-primary {
		background: var(--color-accent);
		color: white;
	}

	.btn-primary:hover {
		background: var(--color-accent-hover);
	}
</style>
