<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { invalidate } from '$app/navigation';
	import Icon from '$lib/components/shared/Icon.svelte';
	// ThreePanelLayout is now at the root layout level — plan detail renders directly in the center panel
	import ModeIndicator from '$lib/components/board/ModeIndicator.svelte';
	import PipelineIndicator from '$lib/components/board/PipelineIndicator.svelte';
	import PlanDetailPanel from '$lib/components/plan/PlanDetailPanel.svelte';
	import RejectionBanner from '$lib/components/plan/RejectionBanner.svelte';
	import ActionBar from '$lib/components/plan/ActionBar.svelte';
	import { AgentPipelineView } from '$lib/components/pipeline';
	// ReviewDashboard and TrajectoryPanel are now rendered in the layout-level RightPanel
	import { planSelectionStore, type PlanSelection } from '$lib/stores/planSelection.svelte';
	import { api } from '$lib/api/client';
	import { promotePlan, executePlan } from '$lib/actions/plans';
	import { derivePlanPipeline, type PlanStage } from '$lib/types/plan';
	import type { Task } from '$lib/types/task';
	import type { Phase } from '$lib/types/phase';
	import type { Requirement } from '$lib/types/requirement';
	import type { Scenario } from '$lib/types/scenario';
	import type { PageData } from './$types';

	interface Props {
		data: PageData;
	}

	let { data }: Props = $props();

	const slug = $derived(page.params.slug);
	const plan = $derived(data.plan);
	const pipeline = $derived(plan ? derivePlanPipeline(plan) : null);

	// Mutable local state — synced from load data, updated by mutations
	let tasks = $state<Task[]>([]);
	let phases = $state<Phase[]>([]);
	let requirements = $state<Requirement[]>([]);
	let scenariosByReq = $state<Record<string, Scenario[]>>({});

	// Sync from load data on initial render and when SvelteKit re-runs the load
	$effect(() => {
		tasks = data.tasks;
		requirements = data.requirements;
		scenariosByReq = data.scenariosByReq;
	});

	// Group tasks by phase ID for legacy nav tree support
	const tasksByPhase = $derived.by(() => {
		const grouped: Record<string, Task[]> = {};
		for (const task of tasks) {
			const phaseId = task.phase_id ?? '__unassigned__';
			if (!grouped[phaseId]) {
				grouped[phaseId] = [];
			}
			grouped[phaseId].push(task);
		}
		return grouped;
	});

	// Update label cache when data changes.
	// labelCache is a plain Map (not reactive), so these writes don't trigger re-renders.
	function updateLabels(
		_plan: typeof plan,
		_reqs: typeof requirements,
		_scenarios: typeof scenariosByReq,
		_tasks: typeof tasks
	) {
		if (plan) {
			planSelectionStore.setLabel(`plan:${plan.slug}`, plan.title || plan.slug);
		}
		for (const req of requirements) {
			planSelectionStore.setLabel(`requirement:${req.id}`, req.title);
		}
		for (const [, scenarios] of Object.entries(scenariosByReq)) {
			for (const scenario of scenarios) {
				planSelectionStore.setLabel(`scenario:${scenario.id}`, `When ${scenario.when}`.slice(0, 30));
			}
		}
		for (const task of tasks) {
			const label = task.description.length > 30 ? task.description.slice(0, 30) + '...' : task.description;
			planSelectionStore.setLabel(`task:${task.id}`, label);
		}
	}

	// Run label update as a side effect — safe because labelCache is not $state.
	// Dependencies are passed as args so Svelte tracks them explicitly.
	$effect(() => {
		updateLabels(plan, requirements, scenariosByReq, tasks);
	});

	// Browser-only: selection init, chat context, periodic refresh for auto-cascade stages
	onMount(() => {
		if (slug) {
			planSelectionStore.selectPlan(slug);
		}

		// Periodically refresh requirements during auto-cascade stages
		const interval = setInterval(() => {
			if (plan && ['approved', 'requirements_generated', 'scenarios_generated'].includes(plan.stage)) {
				fetchRequirements();
			}
		}, 5000);

		return () => {
			clearInterval(interval);
			planSelectionStore.clear();
		};
	});

	// Find any task with an active rejection
	const activeRejection = $derived.by(() => {
		const rejectedTask = tasks.find((t) => t.rejection && t.status === 'in_progress');
		return rejectedTask ? { task: rejectedTask, rejection: rejectedTask.rejection! } : null;
	});

	function handleSelect(selection: PlanSelection): void {
		planSelectionStore.selection = selection;
	}

	async function fetchRequirements(): Promise<void> {
		if (!slug) return;
		try {
			requirements = await api.requirements.list(slug);
			for (const req of requirements) {
				fetchScenariosForReq(req.id);
			}
		} catch {
			requirements = [];
		}
	}

	async function fetchScenariosForReq(reqId: string): Promise<void> {
		if (!slug) return;
		try {
			const scenarios = await api.scenarios.listByRequirement(slug, reqId);
			scenariosByReq = { ...scenariosByReq, [reqId]: scenarios };
		} catch {
			scenariosByReq = { ...scenariosByReq, [reqId]: [] };
		}
	}

	// Action handlers — call API + invalidate for fresh data via SvelteKit cascade
	let actionError = $state<string | null>(null);

	async function handlePromote() {
		if (!plan) return;
		actionError = null;
		try { await promotePlan(plan.slug); } catch (e) {
			actionError = e instanceof Error ? e.message : 'Failed to promote plan';
		}
	}

	async function handleExecute() {
		if (!plan) return;
		actionError = null;
		try { await executePlan(plan.slug); } catch (e) {
			actionError = e instanceof Error ? e.message : 'Failed to start execution';
		}
	}

	async function handleReplay() {
		if (!plan) return;
		actionError = null;
		try { await executePlan(plan.slug); } catch (e) {
			actionError = e instanceof Error ? e.message : 'Failed to replay';
		}
	}

	async function handleRefreshPlan() {
		await invalidate('app:plans');
	}

	async function handleRefreshRequirements() {
		await fetchRequirements();
	}

	async function handleRefreshScenarios(reqId: string) {
		await fetchScenariosForReq(reqId);
	}

	async function handleDeleteRequirement(reqId: string) {
		if (!slug) return;
		await api.requirements.delete(slug, reqId);
		await fetchRequirements();
		// Clear scenarios cache
		const { [reqId]: _, ...rest } = scenariosByReq;
		scenariosByReq = rest;
		// Navigate back to plan
		planSelectionStore.selectPlan(slug);
	}

	async function handleExpandRequirement(reqId: string) {
		if (!scenariosByReq[reqId]) {
			await fetchScenariosForReq(reqId);
		}
	}

	// Legacy handlers for phases/tasks
	async function handleRefreshTasks() {
		if (slug) {
			tasks = await api.plans.getTasks(slug);
		}
	}

	async function handleRefreshPhases() {
		if (slug) {
			try {
				phases = await api.phases.list(slug);
			} catch {
				// ignore
			}
		}
	}

	async function handleApprovePhase(phaseId: string) {
		if (!slug) return;
		await api.phases.approve(slug, phaseId);
		await handleRefreshPhases();
	}

	async function handleRejectPhase(phaseId: string, reason: string) {
		if (!slug) return;
		await api.phases.reject(slug, phaseId, reason);
		await handleRefreshPhases();
	}

	async function handleApproveTask(taskId: string) {
		if (!slug) return;
		const updated = await api.tasks.approve(slug, taskId);
		const index = tasks.findIndex((t) => t.id === taskId);
		if (index !== -1) {
			tasks[index] = updated;
			tasks = [...tasks];
		}
	}

	async function handleRejectTask(taskId: string, reason: string) {
		if (!slug) return;
		const updated = await api.tasks.reject(slug, taskId, reason);
		const index = tasks.findIndex((t) => t.id === taskId);
		if (index !== -1) {
			tasks[index] = updated;
			tasks = [...tasks];
		}
	}

	function getStageLabel(stage: PlanStage): string {
		switch (stage) {
			case 'draft':
			case 'drafting':
				return 'Draft';
			case 'ready_for_approval':
				return 'Ready for Approval';
			case 'reviewed':
				return 'Reviewed';
			case 'needs_changes':
				return 'Needs Changes';
			case 'planning':
				return 'Planning';
			case 'approved':
				return 'Approved';
			case 'rejected':
				return 'Rejected';
			case 'requirements_generated':
				return 'Requirements Generated';
			case 'scenarios_generated':
				return 'Scenarios Generated';
			case 'ready_for_execution':
				return 'Ready to Execute';
			case 'phases_generated':
				return 'Phases Generated';
			case 'phases_approved':
				return 'Phases Approved';
			case 'tasks_generated':
				return 'Tasks Generated';
			case 'tasks_approved':
			case 'tasks':
				return 'Ready to Execute';
			case 'implementing':
			case 'executing':
				return 'Executing';
			case 'complete':
				return 'Complete';
			case 'archived':
				return 'Archived';
			case 'failed':
				return 'Failed';
			default:
				return stage;
		}
	}
</script>

<svelte:head>
	<title>{plan?.slug || 'Plan'} - Semspec</title>
</svelte:head>

<div class="plan-detail">
	<header class="detail-header">
		<div class="header-left">
			<a href="/plans" class="back-link">
				<Icon name="chevron-left" size={16} />
				Back to Plans
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
			<a href="/plans" class="btn btn-primary">Back to Plans</a>
		</div>
	{:else}
		<!-- Progressive plan view — content adapts to plan stage -->
		<div class="plan-content">
			<!-- Pipeline + Action bar (always visible when plan has content) -->
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

			<!-- Agent pipeline (visible during execution) -->
			{#if plan.active_loops && plan.active_loops.length > 0}
				<div class="agent-pipeline-section">
					<AgentPipelineView slug={plan.slug} loops={plan.active_loops} />
				</div>
			{/if}

			<!-- Rejection banner -->
			{#if activeRejection}
				<RejectionBanner
					rejection={activeRejection.rejection}
					taskDescription={activeRejection.task.description}
				/>
			{/if}

			<!-- Stage-driven content -->
			<div class="stage-content">
				<PlanDetailPanel
					selection={planSelectionStore.selection}
					{plan}
					{phases}
					{tasksByPhase}
					{requirements}
					{scenariosByReq}
					onRefreshPlan={handleRefreshPlan}
					onRefreshPhases={handleRefreshPhases}
					onRefreshTasks={handleRefreshTasks}
					onRefreshRequirements={handleRefreshRequirements}
					onRefreshScenarios={handleRefreshScenarios}
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
		min-width: 150px;
	}

	.header-right {
		flex-shrink: 0;
		min-width: 150px;
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

	/* Progressive plan content — single scrollable column */
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

	/* Responsive: mobile */
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
