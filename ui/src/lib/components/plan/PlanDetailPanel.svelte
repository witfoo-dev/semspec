<script lang="ts">
	/**
	 * PlanDetailPanel - Switches between detail views based on selection.
	 *
	 * Renders the appropriate detail component based on the current selection:
	 * - plan → PlanDetail (with RequirementPanel inline)
	 * - requirement → RequirementDetail (with scenarios)
	 * - scenario → ScenarioDetail (standalone view)
	 * - phase → PhaseDetail (legacy)
	 * - task → TaskDetail (legacy)
	 */

	import PlanDetail from './PlanDetail.svelte';
	import PhaseDetail from './PhaseDetail.svelte';
	import TaskDetail from './TaskDetail.svelte';
	import RequirementDetail from './RequirementDetail.svelte';
	import ScenarioDetail from './ScenarioDetail.svelte';
	import type { PlanWithStatus } from '$lib/types/plan';
	import type { Phase } from '$lib/types/phase';
	import type { Task } from '$lib/types/task';
	import type { Requirement } from '$lib/types/requirement';
	import type { Scenario } from '$lib/types/scenario';
	import type { PlanSelection } from '$lib/stores/planSelection.svelte';

	interface Props {
		selection: PlanSelection | null;
		plan: PlanWithStatus;
		phases: Phase[];
		tasksByPhase: Record<string, Task[]>;
		requirements: Requirement[];
		scenariosByReq: Record<string, Scenario[]>;
		onRefreshPlan?: () => Promise<void>;
		onRefreshPhases?: () => Promise<void>;
		onRefreshTasks?: () => Promise<void>;
		onRefreshRequirements?: () => Promise<void>;
		onRefreshScenarios?: (reqId: string) => Promise<void>;
		onDeleteRequirement?: (reqId: string) => Promise<void>;
		onApprovePhase?: (phaseId: string) => Promise<void>;
		onRejectPhase?: (phaseId: string, reason: string) => Promise<void>;
		onApproveTask?: (taskId: string) => Promise<void>;
		onRejectTask?: (taskId: string, reason: string) => Promise<void>;
	}

	let {
		selection,
		plan,
		phases,
		tasksByPhase,
		requirements,
		scenariosByReq,
		onRefreshPlan,
		onRefreshPhases,
		onRefreshTasks,
		onRefreshRequirements,
		onRefreshScenarios,
		onDeleteRequirement,
		onApprovePhase,
		onRejectPhase,
		onApproveTask,
		onRejectTask
	}: Props = $props();

	// Find selected requirement
	const selectedRequirement = $derived.by(() => {
		if (!selection?.requirementId) return undefined;
		return requirements.find((r) => r.id === selection.requirementId);
	});

	// Find selected scenario
	const selectedScenario = $derived.by(() => {
		if (!selection?.scenarioId || !selection?.requirementId) return undefined;
		const scenarios = scenariosByReq[selection.requirementId] ?? [];
		return scenarios.find((s) => s.id === selection.scenarioId);
	});

	// Find selected phase (legacy)
	const selectedPhase = $derived.by(() => {
		if (!selection?.phaseId) return undefined;
		return phases.find((p) => p.id === selection.phaseId);
	});

	// Find selected task (legacy)
	const selectedTask = $derived.by(() => {
		if (!selection?.taskId || !selection?.phaseId) return undefined;
		const phaseTasks = tasksByPhase[selection.phaseId] ?? [];
		return phaseTasks.find((t) => t.id === selection.taskId);
	});

	// Scenarios for the selected requirement
	const selectedReqScenarios = $derived.by(() => {
		if (!selection?.requirementId) return [];
		return scenariosByReq[selection.requirementId] ?? [];
	});

	// Requirement title for standalone scenario view
	const selectedReqTitle = $derived.by(() => {
		if (!selection?.requirementId) return undefined;
		return requirements.find((r) => r.id === selection.requirementId)?.title;
	});
</script>

<div class="detail-panel-container">
	{#if !selection || selection.type === 'plan'}
		<PlanDetail
			{plan}
			{phases}
			{requirements}
			onRefresh={onRefreshPlan}
		/>
	{:else if selection.type === 'requirement' && selectedRequirement}
		{@const reqId = selectedRequirement.id}
		<RequirementDetail
			requirement={selectedRequirement}
			scenarios={selectedReqScenarios}
			{plan}
			onRefresh={onRefreshRequirements}
			onRefreshScenarios={onRefreshScenarios ? () => onRefreshScenarios(reqId) : undefined}
			onDelete={onDeleteRequirement}
		/>
	{:else if selection.type === 'scenario' && selectedScenario}
		<div class="scenario-standalone">
			<ScenarioDetail scenario={selectedScenario} requirementTitle={selectedReqTitle} />
		</div>
	{:else if selection.type === 'phase' && selectedPhase}
		<PhaseDetail
			phase={selectedPhase}
			{plan}
			tasks={tasksByPhase[selectedPhase.id] ?? []}
			allPhases={phases}
			onRefresh={onRefreshPhases}
			onApprove={onApprovePhase}
			onReject={onRejectPhase}
			onRefreshTasks={onRefreshTasks}
		/>
	{:else if selection.type === 'task' && selectedTask && selectedPhase}
		<TaskDetail
			task={selectedTask}
			phase={selectedPhase}
			{plan}
			onRefresh={onRefreshTasks}
			onApprove={onApproveTask}
			onReject={onRejectTask}
		/>
	{:else}
		<PlanDetail
			{plan}
			{phases}
			{requirements}
			onRefresh={onRefreshPlan}
		/>
	{/if}
</div>

<style>
	.detail-panel-container {
		display: flex;
		flex-direction: column;
		height: 100%;
		overflow: auto;
	}

	.scenario-standalone {
		padding: var(--space-4);
	}
</style>
