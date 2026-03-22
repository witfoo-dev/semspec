import { type Page, type Locator, expect } from '@playwright/test';

/**
 * Page Object Model for Questions in the Chat interface.
 *
 * Questions are rendered as QuestionMessage components (.question-message)
 * within the chat message log ([role="log"]). There is no dedicated
 * question panel — questions appear inline in the chat.
 *
 * Provides methods to interact with and verify:
 * - Individual question messages in the chat log
 * - Question status (pending, answered, timeout)
 * - Answer form within question messages
 */
export class QuestionPanelPage {
	readonly page: Page;
	readonly panel: Locator;
	readonly questionCards: Locator;
	readonly emptyState: Locator;

	constructor(page: Page) {
		this.page = page;
		this.panel = page.locator('[role="log"][aria-label="Chat messages"]');
		this.questionCards = page.locator('.question-message');
		this.emptyState = this.panel.locator('.empty-state');
	}

	async expectVisible(): Promise<void> {
		await expect(this.questionCards.first()).toBeVisible();
	}

	async expectQuestionCards(count: number): Promise<void> {
		await expect(this.questionCards).toHaveCount(count);
	}

	async expectEmptyState(): Promise<void> {
		await expect(this.emptyState).toBeVisible();
	}

	async expectNoEmptyState(): Promise<void> {
		await expect(this.emptyState).not.toBeVisible();
	}

	async getQuestionCard(questionId: string): Promise<Locator> {
		return this.questionCards.filter({ hasText: questionId.slice(0, 10) });
	}

	async expectQuestionStatus(questionId: string, status: string): Promise<void> {
		const card = await this.getQuestionCard(questionId);
		await expect(card).toHaveClass(new RegExp(status));
	}

	async expectQuestionTopic(questionId: string, topic: string): Promise<void> {
		const card = await this.getQuestionCard(questionId);
		const topicElement = card.locator('.topic');
		await expect(topicElement).toContainText(topic);
	}

	async expectQuestionText(questionId: string, text: string): Promise<void> {
		const card = await this.getQuestionCard(questionId);
		const questionText = card.locator('.question-text');
		await expect(questionText).toContainText(text);
	}

	async expectQuestionUrgency(questionId: string, urgency: string): Promise<void> {
		const card = await this.getQuestionCard(questionId);
		await expect(card).toHaveClass(new RegExp(urgency));
	}

	async openAnswerForm(questionId: string): Promise<void> {
		const card = await this.getQuestionCard(questionId);
		const replyButton = card.locator('.action-btn.reply');
		await replyButton.click();
	}

	async submitAnswer(questionId: string, answer: string): Promise<void> {
		const card = await this.getQuestionCard(questionId);
		const textarea = card.locator('.reply-form textarea');
		const submitButton = card.locator('.btn-submit');
		await textarea.fill(answer);
		await submitButton.click();
	}

	async cancelAnswer(questionId: string): Promise<void> {
		const card = await this.getQuestionCard(questionId);
		const cancelButton = card.locator('.btn-cancel');
		await cancelButton.click();
	}

	async expectAnswerFormVisible(questionId: string): Promise<void> {
		const card = await this.getQuestionCard(questionId);
		const answerForm = card.locator('.reply-form');
		await expect(answerForm).toBeVisible();
	}

	async expectAnswerFormHidden(questionId: string): Promise<void> {
		const card = await this.getQuestionCard(questionId);
		const answerForm = card.locator('.reply-form');
		await expect(answerForm).not.toBeVisible();
	}
}
