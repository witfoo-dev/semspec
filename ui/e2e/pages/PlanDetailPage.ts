import { type Page, type Locator, expect } from '@playwright/test';

/**
 * Page Object Model for the Plan Detail page.
 *
 * Provides methods to interact with and verify:
 * - Plan information and metadata
 * - Agent Pipeline View (stages, progress, parallel branches)
 * - Review Dashboard (spec gate, reviewer cards, findings)
 * - Requirements/Scenarios tree navigation
 * - ThreePanelLayout (left nav, center detail, right reviews)
 */
export class PlanDetailPage {
	readonly page: Page;
	readonly planDetail: Locator;
	readonly backLink: Locator;
	readonly planTitle: Locator;
	readonly planStage: Locator;
	readonly notFound: Locator;

	// Pipeline section
	readonly pipelineSection: Locator;
	readonly pipelineIndicator: Locator;
	readonly agentPipelineView: Locator;
	readonly pipelineStages: Locator;
	readonly mainPipeline: Locator;
	readonly reviewBranch: Locator;

	// Reviews section
	readonly reviewsSection: Locator;
	readonly reviewsToggle: Locator;
	readonly reviewsContent: Locator;
	readonly reviewDashboard: Locator;
	readonly specGate: Locator;
	readonly reviewerCards: Locator;
	readonly findingsList: Locator;
	readonly findingsRows: Locator;

	// ActionBar
	readonly actionBar: Locator;
	readonly approvePlanBtn: Locator;
	readonly executeBtn: Locator;
	readonly cascadeStatus: Locator;

	// ThreePanelLayout
	readonly threePanel: Locator;
	readonly panelLeft: Locator;
	readonly panelCenter: Locator;
	readonly panelRight: Locator;
	readonly toggleLeft: Locator;
	readonly toggleRight: Locator;

	// DataTable (tasks)
	readonly taskTable: Locator;
	readonly taskTableFilter: Locator;
	readonly taskTableStatusFilter: Locator;
	readonly taskTableRows: Locator;
	readonly taskTablePagination: Locator;
	readonly addTaskBtn: Locator;

	// Plan editing
	readonly planEditBtn: Locator;
	readonly planGoalTextarea: Locator;
	readonly planContextTextarea: Locator;
	readonly planSaveBtn: Locator;
	readonly planCancelBtn: Locator;

	// Task edit modal
	readonly taskEditModal: Locator;
	readonly taskDescriptionInput: Locator;
	readonly taskTypeSelect: Locator;
	readonly taskFilesTextarea: Locator;
	readonly taskModalSaveBtn: Locator;
	readonly taskModalCancelBtn: Locator;

	// Navigation tree
	readonly navTree: Locator;
	readonly treeNodes: Locator;
	readonly requirementNodes: Locator;
	readonly scenarioNodes: Locator;
	readonly requirementDetail: Locator;
	readonly requirementDetailTitle: Locator;

	// Task detail panel
	readonly taskDetail: Locator;
	readonly taskDetailTitle: Locator;
	readonly taskApproveBtn: Locator;
	readonly taskRejectBtn: Locator;
	readonly taskRejectReason: Locator;
	readonly taskRejectConfirm: Locator;

	constructor(page: Page) {
		this.page = page;
		this.planDetail = page.locator('.plan-detail').first();
		this.backLink = page.locator('.back-link');
		this.planTitle = page.locator('.detail-title');
		this.planStage = page.locator('.plan-stage');
		this.notFound = page.locator('.not-found');

		// Pipeline section
		this.pipelineSection = page.locator('.agent-pipeline-section');
		this.pipelineIndicator = page.locator('.pipeline');
		this.agentPipelineView = page.locator('.pipeline-view');
		this.pipelineStages = page.locator('.pipeline-stage');
		this.mainPipeline = page.locator('.main-pipeline');
		this.reviewBranch = page.locator('.review-branch');

		// Reviews section — rendered in the right panel as ReviewDashboard
		this.reviewsSection = page.locator('[data-testid="panel-right"]').filter({ has: page.locator('.review-dashboard') });
		this.reviewsToggle = page.locator('[data-testid="toggle-right"]');
		this.reviewsContent = page.locator('.review-dashboard');
		this.reviewDashboard = page.locator('.review-dashboard');
		this.specGate = page.locator('.spec-gate');
		this.reviewerCards = page.locator('.reviewer-card');
		this.findingsList = page.locator('.findings-list');
		this.findingsRows = page.locator('.finding-row');

		// ActionBar
		this.actionBar = page.locator('.action-bar');
		this.approvePlanBtn = this.actionBar.locator('button', { hasText: 'Approve Plan' });
		this.executeBtn = this.actionBar.locator('button', { hasText: /Start Execution/ });
		this.cascadeStatus = page.locator('.cascade-status');

		// ThreePanelLayout (replaces old ResizableSplit)
		this.threePanel = page.locator('[data-testid="three-panel-layout"]');
		this.panelLeft = page.locator('[data-testid="panel-left"]');
		this.panelCenter = page.locator('[data-testid="panel-center"]');
		this.panelRight = page.locator('[data-testid="panel-right"]');
		this.toggleLeft = page.locator('[data-testid="toggle-left"]');
		this.toggleRight = page.locator('[data-testid="toggle-right"]');

		// DataTable (tasks)
		this.taskTable = page.locator('[data-testid="task-list"]');
		this.taskTableFilter = page.locator('[data-testid="task-list-filter"]');
		this.taskTableStatusFilter = page.locator('[data-testid="task-list-status-filter"]');
		this.taskTableRows = page.locator('[data-testid="task-list-row"]');
		this.taskTablePagination = page.locator('[data-testid="task-list-pagination"]');
		this.addTaskBtn = page.locator('.add-task-btn');

		// Plan editing
		const detailPanel = page.locator('.detail-panel-container');
		this.planEditBtn = detailPanel.locator('.detail-header button', { hasText: 'Edit' });
		this.planGoalTextarea = page.locator('#edit-goal');
		this.planContextTextarea = page.locator('#edit-context');
		this.planSaveBtn = page.locator('.edit-actions .btn-primary');
		this.planCancelBtn = page.locator('.edit-actions .btn-ghost');

		// Task edit modal
		this.taskEditModal = page.locator('.modal');
		this.taskDescriptionInput = page.locator('#task-description');
		this.taskTypeSelect = page.locator('#task-type');
		this.taskFilesTextarea = page.locator('#task-files');
		this.taskModalSaveBtn = this.taskEditModal.locator('button[type="submit"]');
		this.taskModalCancelBtn = this.taskEditModal.locator('button', { hasText: 'Cancel' });

		// Navigation tree (PlanNavTree)
		this.navTree = page.locator('.plan-nav-tree');
		this.treeNodes = page.locator('.tree-node');
		this.requirementNodes = this.navTree.locator('.tree-node.req-node');
		this.scenarioNodes = this.navTree.locator('.tree-node.scenario-node');
		this.requirementDetail = page.locator('.requirement-detail');
		this.requirementDetailTitle = this.requirementDetail.locator('.detail-title');

		// Task detail panel
		this.taskDetail = page.locator('.task-detail');
		this.taskDetailTitle = this.taskDetail.locator('.detail-title');
		this.taskApproveBtn = this.taskDetail.locator('button', { hasText: 'Approve Task' });
		this.taskRejectBtn = this.taskDetail.locator('button', { hasText: 'Reject' });
		this.taskRejectReason = this.taskDetail.locator('#reject-reason');
		this.taskRejectConfirm = this.taskDetail.locator('button', { hasText: 'Confirm Rejection' });
	}

	async goto(slug: string): Promise<void> {
		await this.page.goto(`/plans/${slug}`);
		await this.page.waitForSelector('.plan-detail, .not-found', { timeout: 15000 });
	}

	async expectVisible(): Promise<void> {
		await expect(this.planDetail).toBeVisible();
	}

	async expectNotFound(): Promise<void> {
		await expect(this.notFound).toBeVisible();
	}

	async expectPlanTitle(title: string): Promise<void> {
		await expect(this.planTitle).toHaveText(title);
	}

	async expectPlanStage(stage: string): Promise<void> {
		await expect(this.planStage).toHaveText(stage);
	}

	// Pipeline methods
	async expectPipelineVisible(): Promise<void> {
		await expect(this.pipelineSection).toBeVisible();
		await expect(this.agentPipelineView).toBeVisible();
	}

	async expectPipelineStageCount(count: number): Promise<void> {
		await expect(this.pipelineStages).toHaveCount(count);
	}

	async getStage(stageId: string): Promise<Locator> {
		return this.pipelineStages.filter({ hasText: stageId });
	}

	async expectStageState(stageId: string, state: 'pending' | 'active' | 'complete' | 'failed'): Promise<void> {
		const stage = await this.getStage(stageId);
		await expect(stage).toHaveClass(new RegExp(state));
	}

	async expectActiveStageSpinner(stageId: string): Promise<void> {
		const stage = await this.getStage(stageId);
		const spinner = stage.locator('.spin');
		await expect(spinner).toBeVisible();
	}

	async expectStageProgress(stageId: string, current: number, max: number): Promise<void> {
		const stage = await this.getStage(stageId);
		const progress = stage.locator('.stage-progress');
		await expect(progress).toHaveText(`${current}/${max}`);
	}

	async expectReviewBranchVisible(): Promise<void> {
		await expect(this.reviewBranch).toBeVisible();
	}

	async expectReviewBranchHidden(): Promise<void> {
		await expect(this.reviewBranch).not.toBeVisible();
	}

	// Reviews methods
	async expectReviewsSectionVisible(): Promise<void> {
		await expect(this.reviewsSection).toBeVisible();
	}

	async expectReviewsSectionHidden(): Promise<void> {
		await expect(this.reviewsSection).not.toBeVisible();
	}

	async expandReviews(): Promise<void> {
		const isExpanded = await this.reviewsContent.isVisible();
		if (!isExpanded && await this.reviewsToggle.isVisible()) {
			await this.reviewsToggle.click();
		}
	}

	async collapseReviews(): Promise<void> {
		const isExpanded = await this.reviewsContent.isVisible();
		if (isExpanded && await this.reviewsToggle.isVisible()) {
			await this.reviewsToggle.click();
		}
	}

	async expectReviewsExpanded(): Promise<void> {
		await expect(this.reviewsContent).toBeVisible();
		await expect(this.reviewDashboard).toBeVisible();
	}

	async expectReviewsCollapsed(): Promise<void> {
		await expect(this.reviewsContent).not.toBeVisible();
	}

	async expectSpecGateVisible(): Promise<void> {
		await expect(this.specGate).toBeVisible();
	}

	async expectSpecGatePassed(): Promise<void> {
		await expect(this.specGate).toHaveClass(/passed/);
	}

	async expectSpecGateFailed(): Promise<void> {
		await expect(this.specGate).toHaveClass(/failed/);
	}

	async expectSpecGateVerdict(verdict: string): Promise<void> {
		const badge = this.specGate.locator('.badge');
		await expect(badge).toContainText(verdict);
	}

	async expectSpecGateStatus(status: 'Gate Passed' | 'Gate Failed' | 'Awaiting review'): Promise<void> {
		const statusEl = this.specGate.locator('.gate-status');
		await expect(statusEl).toContainText(status);
	}

	async expectReviewerCount(count: number): Promise<void> {
		await expect(this.reviewerCards).toHaveCount(count);
	}

	async getReviewerCard(role: string): Promise<Locator> {
		return this.reviewerCards.filter({ hasText: role });
	}

	async expectReviewerPassed(role: string): Promise<void> {
		const card = await this.getReviewerCard(role);
		await expect(card).toHaveClass(/passed/);
	}

	async expectReviewerFailed(role: string): Promise<void> {
		const card = await this.getReviewerCard(role);
		await expect(card).toHaveClass(/failed/);
	}

	async expectFindingsCount(count: number): Promise<void> {
		await expect(this.findingsRows).toHaveCount(count);
	}

	async expectFindingsListVisible(): Promise<void> {
		await expect(this.findingsList).toBeVisible();
	}

	async expectFindingSeverity(index: number, severity: string): Promise<void> {
		const finding = this.findingsRows.nth(index);
		const severityBadge = finding.locator('.severity-badge');
		await expect(severityBadge).toHaveText(severity);
	}

	async expectFindingFile(index: number, file: string): Promise<void> {
		const finding = this.findingsRows.nth(index);
		const fileEl = finding.locator('.finding-file');
		await expect(fileEl).toContainText(file);
	}

	async expectEmptyReviews(): Promise<void> {
		const emptyState = this.reviewDashboard.locator('.empty-state');
		await expect(emptyState).toBeVisible();
		await expect(emptyState).toContainText('No review results available');
	}

	async expectLoadingReviews(): Promise<void> {
		const loadingState = this.reviewDashboard.locator('.loading-state');
		await expect(loadingState).toBeVisible();
	}

	async expectReviewError(): Promise<void> {
		const errorState = this.reviewDashboard.locator('.error-state');
		await expect(errorState).toBeVisible();
	}

	// Dashboard stats
	async expectReviewerStats(passed: number, total: number): Promise<void> {
		const passCount = this.reviewDashboard.locator('.pass-count');
		await expect(passCount).toHaveText(`${passed}/${total} passed`);
	}

	async expectVerdictBadge(verdict: string): Promise<void> {
		const badge = this.reviewDashboard.locator('.dashboard-header .badge');
		await expect(badge).toContainText(verdict);
	}

	// ActionBar methods
	async expectActionBarVisible(): Promise<void> {
		await expect(this.actionBar).toBeVisible();
	}

	async expectApprovePlanBtnVisible(): Promise<void> {
		await expect(this.approvePlanBtn).toBeVisible();
	}

	async expectExecuteBtnVisible(): Promise<void> {
		await expect(this.executeBtn).toBeVisible();
	}

	async expectCascadeStatusVisible(): Promise<void> {
		await expect(this.cascadeStatus).toBeVisible();
	}

	async expectCascadeStatusText(text: string): Promise<void> {
		await expect(this.cascadeStatus).toContainText(text);
	}

	async clickApprovePlan(): Promise<void> {
		await this.approvePlanBtn.click();
	}

	async clickExecute(): Promise<void> {
		await this.executeBtn.click();
	}

	async goBack(): Promise<void> {
		await this.backLink.click();
	}

	// ThreePanelLayout methods
	async expectThreePanelVisible(): Promise<void> {
		await expect(this.threePanel).toBeVisible();
	}

	async expectPanelLeftVisible(): Promise<void> {
		await expect(this.panelLeft).toBeVisible();
	}

	async expectPanelCenterVisible(): Promise<void> {
		await expect(this.panelCenter).toBeVisible();
	}

	async expectPanelRightVisible(): Promise<void> {
		await expect(this.panelRight).toBeVisible();
	}

	// Backwards-compat aliases for tests using old ResizableSplit names
	async expectResizableSplitVisible(): Promise<void> {
		await this.expectThreePanelVisible();
	}

	async expectPlanPanelVisible(): Promise<void> {
		await this.expectPanelLeftVisible();
	}

	async expectTasksPanelVisible(): Promise<void> {
		await this.expectPanelCenterVisible();
	}

	async expectResizeDividerVisible(): Promise<void> {
		// ThreePanelLayout uses toggle buttons instead of a drag divider
		await expect(this.toggleLeft.or(this.toggleRight)).toBeVisible();
	}

	// DataTable methods
	async expectTaskTableVisible(): Promise<void> {
		await expect(this.taskTable).toBeVisible();
	}

	async filterTasks(text: string): Promise<void> {
		await this.taskTableFilter.fill(text);
	}

	async filterTasksByStatus(status: string): Promise<void> {
		await this.taskTableStatusFilter.selectOption(status);
	}

	async expectTaskCount(count: number): Promise<void> {
		await expect(this.taskTableRows).toHaveCount(count);
	}

	async expectTaskTableCount(text: string): Promise<void> {
		const countLabel = this.taskTable.locator('[data-testid="task-list-count"]');
		await expect(countLabel).toContainText(text);
	}

	async clickTaskRow(index: number): Promise<void> {
		await this.taskTableRows.nth(index).click();
	}

	async expandTaskRow(index: number): Promise<void> {
		const expandBtns = this.taskTable.locator('button[aria-label="Expand row"]');
		const btn = expandBtns.nth(index);
		await btn.scrollIntoViewIfNeeded();
		await btn.click();
		// Wait for the expanded content to appear
		await expect(this.taskTable.locator('button[aria-label="Collapse row"]').first()).toBeVisible();
	}

	async expectTaskRowExpanded(index: number): Promise<void> {
		const expandBtns = this.taskTable.locator('button[aria-label="Collapse row"]');
		await expect(expandBtns.first()).toBeVisible();
	}

	async approveTask(index: number): Promise<void> {
		const row = this.taskTableRows.nth(index);
		await row.locator('.btn-success').click();
	}

	async rejectTask(index: number, reason: string): Promise<void> {
		const row = this.taskTableRows.nth(index);
		await row.locator('.btn-outline').click();
		await row.locator('.reject-reason-input').fill(reason);
		await row.locator('.btn-danger').click();
	}

	async goToPage(pageNum: number): Promise<void> {
		if (pageNum === 1) {
			await this.taskTablePagination.locator('button').first().click();
		} else {
			const nextBtn = this.taskTablePagination.locator('button').nth(3);
			for (let i = 1; i < pageNum; i++) {
				await nextBtn.click();
			}
		}
	}

	async expectCurrentPage(pageNum: number, totalPages: number): Promise<void> {
		const pageInfo = this.taskTablePagination.locator('.page-info');
		await expect(pageInfo).toHaveText(`Page ${pageNum} of ${totalPages}`);
	}

	// Plan editing methods
	async expectPlanEditBtnVisible(): Promise<void> {
		await expect(this.planEditBtn).toBeVisible();
	}

	async expectPlanEditBtnHidden(): Promise<void> {
		await expect(this.planEditBtn).not.toBeVisible();
	}

	async clickPlanEdit(): Promise<void> {
		await this.planEditBtn.click();
	}

	async expectPlanEditMode(): Promise<void> {
		await expect(this.planGoalTextarea).toBeVisible();
		await expect(this.planContextTextarea).toBeVisible();
		await expect(this.planSaveBtn).toBeVisible();
		await expect(this.planCancelBtn).toBeVisible();
	}

	async expectPlanViewMode(): Promise<void> {
		await expect(this.planGoalTextarea).not.toBeVisible();
		await expect(this.planSaveBtn).not.toBeVisible();
	}

	async editPlanGoal(text: string): Promise<void> {
		await this.planGoalTextarea.fill(text);
	}

	async editPlanContext(text: string): Promise<void> {
		await this.planContextTextarea.fill(text);
	}

	async savePlanEdit(): Promise<void> {
		await this.planSaveBtn.click();
	}

	async cancelPlanEdit(): Promise<void> {
		await this.planCancelBtn.click();
	}

	// Task modal methods
	async expectAddTaskBtnVisible(): Promise<void> {
		await expect(this.addTaskBtn).toBeVisible();
	}

	async expectAddTaskBtnHidden(): Promise<void> {
		await expect(this.addTaskBtn).not.toBeVisible();
	}

	async clickAddTask(): Promise<void> {
		await this.addTaskBtn.click();
	}

	async expectTaskModalVisible(): Promise<void> {
		await expect(this.taskEditModal).toBeVisible();
	}

	async expectTaskModalHidden(): Promise<void> {
		await expect(this.taskEditModal).not.toBeVisible();
	}

	async fillTaskDescription(text: string): Promise<void> {
		await this.taskDescriptionInput.fill(text);
	}

	async selectTaskType(type: string): Promise<void> {
		await this.taskTypeSelect.selectOption(type);
	}

	async fillTaskFiles(files: string): Promise<void> {
		await this.taskFilesTextarea.fill(files);
	}

	async saveTaskModal(): Promise<void> {
		await this.taskModalSaveBtn.click();
	}

	async cancelTaskModal(): Promise<void> {
		await this.taskModalCancelBtn.click();
	}

	async editTask(index: number): Promise<void> {
		const row = this.taskTableRows.nth(index);
		await row.locator('button[title="Edit task"]').click();
	}

	// Requirement/scenario tree navigation methods
	async selectRequirementInTree(title: string): Promise<void> {
		const reqNode = this.navTree.locator('.tree-node.req-node', { hasText: title });
		await reqNode.click();
		await expect(this.requirementDetail).toBeVisible({ timeout: 5000 });
	}

	async expandRequirementInTree(title: string): Promise<void> {
		const reqRow = this.navTree.locator('.req-row').filter({ hasText: title });
		const expandBtn = reqRow.locator('.expand-btn');
		const isExpanded = await expandBtn.getAttribute('aria-expanded');
		if (isExpanded !== 'true') {
			await expandBtn.click();
		}
	}

	async selectScenarioInTree(scenarioText: string): Promise<void> {
		const scenarioNode = this.navTree.locator('.tree-node.scenario-node', { hasText: scenarioText }).first();
		await scenarioNode.click();
	}

	async expectRequirementInTree(title: string): Promise<void> {
		const reqNode = this.navTree.locator('.tree-node.req-node', { hasText: title });
		await expect(reqNode).toBeVisible();
	}

	async expectScenarioInTree(scenarioText: string): Promise<void> {
		const scenarioNode = this.navTree.locator('.tree-node.scenario-node', { hasText: scenarioText }).first();
		await expect(scenarioNode).toBeVisible();
	}

	// Requirement detail methods
	async expectRequirementDetailVisible(): Promise<void> {
		await expect(this.requirementDetail).toBeVisible();
	}

	async expectRequirementDetailTitle(title: string): Promise<void> {
		await expect(this.requirementDetailTitle).toContainText(title);
	}

	async expectNavTreeVisible(): Promise<void> {
		await expect(this.navTree).toBeVisible();
	}

	// Task detail methods
	async expectTaskDetailVisible(): Promise<void> {
		await expect(this.taskDetail).toBeVisible();
	}

	async expectTaskDetailTitle(title: string): Promise<void> {
		await expect(this.taskDetailTitle).toContainText(title);
	}

	async expectTaskApproveVisible(): Promise<void> {
		await expect(this.taskApproveBtn).toBeVisible();
	}

	async clickTaskApprove(): Promise<void> {
		await this.taskApproveBtn.click();
	}

	async clickTaskReject(): Promise<void> {
		await this.taskRejectBtn.click();
	}

	async fillTaskRejectReason(reason: string): Promise<void> {
		await this.taskRejectReason.fill(reason);
	}

	async confirmTaskReject(): Promise<void> {
		await this.taskRejectConfirm.click();
	}

	async expectAcceptanceCriteria(): Promise<void> {
		const criteria = this.taskDetail.locator('.criteria-list');
		await expect(criteria).toBeVisible();
	}
}
