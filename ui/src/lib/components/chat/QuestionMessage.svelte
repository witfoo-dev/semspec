<script lang="ts">
	import Icon from '$lib/components/shared/Icon.svelte';
	import { questionsStore } from '$lib/stores/questions.svelte';
	import type { Question } from '$lib/types';

	interface Props {
		question: Question;
	}

	let { question }: Props = $props();

	let showReplyForm = $state(false);
	let answerText = $state('');
	let submitting = $state(false);
	let submitError = $state<string | null>(null);

	const isPending = $derived(question.status === 'pending');
	const isAnswered = $derived(question.status === 'answered');
	const isTimeout = $derived(question.status === 'timeout');
	const isBlocking = $derived(question.urgency === 'blocking');
	const isEnvironment = $derived(question.category === 'environment');
	const hasInstallAction = $derived(
		question.action?.type === 'install_package' ||
		!!question.metadata?.suggested_packages
	);

	function getUrgencyIcon(): string {
		switch (question.urgency) {
			case 'blocking': return 'alert-circle';
			case 'high': return 'alert-triangle';
			default: return 'help-circle';
		}
	}

	function formatRelativeTime(dateStr: string): string {
		const date = new Date(dateStr);
		const now = new Date();
		const diffMs = now.getTime() - date.getTime();
		const diffMins = Math.floor(diffMs / 60000);
		const diffHours = Math.floor(diffMins / 60);
		const diffDays = Math.floor(diffHours / 24);

		if (diffMs < 0 || diffMins < 1) return 'just now';
		if (diffMins < 60) return `${diffMins}m ago`;
		if (diffHours < 24) return `${diffHours}h ago`;
		return `${diffDays}d ago`;
	}

	async function handleAnswer(text: string) {
		submitting = true;
		submitError = null;
		try {
			await questionsStore.answer(question.id, text);
			answerText = '';
			showReplyForm = false;
		} catch (err) {
			submitError = err instanceof Error ? err.message : 'Failed to send answer';
		} finally {
			submitting = false;
		}
	}

	async function handleInstall() {
		const packages = question.metadata?.suggested_packages ??
			question.action?.parameters?.packages ?? '';
		await handleAnswer(`Install: ${packages}`);
	}

	async function handleSkip() {
		await handleAnswer('Skip — find an alternative approach.');
	}
</script>

<div
	class="question-message"
	class:pending={isPending}
	class:answered={isAnswered}
	class:timeout={isTimeout}
	class:blocking={isBlocking}
>
	<div class="question-header">
		<div class="header-left">
			<Icon name={getUrgencyIcon()} size={16} />
			<span class="label">
				{isBlocking ? 'BLOCKING' : question.urgency === 'high' ? 'URGENT' : 'QUESTION'}
			</span>
			{#if question.topic}
				<span class="topic">{question.topic}</span>
			{/if}
		</div>
		<span class="time">{formatRelativeTime(question.created_at)}</span>
	</div>

	{#if question.from_agent && question.from_agent !== 'unknown'}
		<div class="agent-name">
			<Icon name="user" size={12} />
			{question.from_agent}
		</div>
	{/if}

	<div class="question-text">
		{question.question}
	</div>

	{#if question.context}
		<div class="question-context">{question.context}</div>
	{/if}

	{#if isPending}
		<div class="actions">
			{#if isEnvironment && hasInstallAction}
				<button class="action-btn install" onclick={handleInstall} disabled={submitting}>
					<Icon name="download" size={14} />
					Install
				</button>
				<button class="action-btn skip" onclick={handleSkip} disabled={submitting}>
					Skip
				</button>
			{/if}
			{#if showReplyForm}
				<div class="reply-form">
					<textarea
						bind:value={answerText}
						aria-label="Your answer"
						placeholder="Type your answer..."
						rows="2"
						disabled={submitting}
					></textarea>
					<div class="form-actions">
						<button class="btn-cancel" onclick={() => { showReplyForm = false; answerText = ''; submitError = null; }} disabled={submitting}>
							Cancel
						</button>
						<button class="btn-submit" onclick={() => handleAnswer(answerText.trim())} disabled={!answerText.trim() || submitting}>
							{submitting ? 'Sending...' : 'Send'}
						</button>
					</div>
					{#if submitError}
						<div class="submit-error">{submitError}</div>
					{/if}
				</div>
			{:else}
				<button class="action-btn reply" onclick={() => showReplyForm = true}>
					<Icon name="message-square" size={14} />
					Reply
				</button>
			{/if}
		</div>
	{/if}

	{#if isAnswered && question.answer}
		<div class="answer-section">
			<div class="answer-label">
				<Icon name="check-circle" size={12} />
				Answered{#if question.answered_by} by {question.answered_by}{/if}
			</div>
			<div class="answer-text">{question.answer}</div>
		</div>
	{/if}

	{#if isTimeout}
		<div class="timeout-notice">
			<Icon name="clock" size={12} />
			Timed out
		</div>
	{/if}
</div>

<style>
	.question-message {
		background: var(--color-bg-tertiary);
		border: 1px solid var(--color-border);
		border-left: 3px solid var(--color-warning);
		border-radius: var(--radius-lg);
		padding: var(--space-3);
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
		margin: var(--space-2) 0;
	}

	.question-message.blocking {
		border-left-color: var(--color-error);
		border-color: var(--color-error);
		background: color-mix(in srgb, var(--color-error) 5%, var(--color-bg-tertiary));
	}

	.question-message.answered {
		border-left-color: var(--color-success);
		opacity: 0.85;
	}

	.question-message.timeout {
		border-left-color: var(--color-text-muted);
		opacity: 0.6;
	}

	.question-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
	}

	.header-left {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		color: var(--color-warning);
	}

	.question-message.blocking .header-left {
		color: var(--color-error);
	}

	.question-message.answered .header-left {
		color: var(--color-success);
	}

	.label {
		font-size: var(--font-size-xs);
		font-weight: var(--font-weight-semibold);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.topic {
		font-size: var(--font-size-xs);
		color: var(--color-accent);
		padding: 1px var(--space-2);
		background: var(--color-accent-muted);
		border-radius: var(--radius-full);
	}

	.time {
		font-size: var(--font-size-xs);
		color: var(--color-text-muted);
	}

	.agent-name {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		font-size: var(--font-size-xs);
		color: var(--color-text-secondary);
	}

	.question-text {
		font-size: var(--font-size-sm);
		color: var(--color-text-primary);
		line-height: 1.5;
	}

	.question-context {
		font-size: var(--font-size-xs);
		color: var(--color-text-muted);
		padding: var(--space-2);
		background: var(--color-bg-elevated);
		border-radius: var(--radius-md);
		white-space: pre-wrap;
	}

	.actions {
		display: flex;
		gap: var(--space-2);
		align-items: flex-start;
		flex-wrap: wrap;
	}

	.action-btn {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		padding: var(--space-1) var(--space-2);
		border: none;
		border-radius: var(--radius-md);
		cursor: pointer;
		font-size: var(--font-size-xs);
		transition: all var(--transition-fast);
	}

	.action-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.action-btn.install {
		background: var(--color-success-muted);
		color: var(--color-success);
	}

	.action-btn.skip {
		background: var(--color-bg-elevated);
		color: var(--color-text-secondary);
	}

	.action-btn.reply {
		background: var(--color-accent-muted);
		color: var(--color-accent);
	}

	.action-btn:hover:not(:disabled) {
		filter: brightness(1.2);
	}

	.reply-form {
		flex: 1;
		min-width: 200px;
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}

	.reply-form textarea {
		width: 100%;
		padding: var(--space-2);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-md);
		background: var(--color-bg-primary);
		color: var(--color-text-primary);
		font-size: var(--font-size-sm);
		resize: vertical;
	}

	.reply-form textarea:focus {
		outline: none;
		border-color: var(--color-accent);
	}

	.form-actions {
		display: flex;
		gap: var(--space-2);
		justify-content: flex-end;
	}

	.btn-cancel,
	.btn-submit {
		padding: var(--space-1) var(--space-2);
		border: none;
		border-radius: var(--radius-md);
		cursor: pointer;
		font-size: var(--font-size-xs);
	}

	.btn-cancel {
		background: var(--color-bg-elevated);
		color: var(--color-text-secondary);
	}

	.btn-submit {
		background: var(--color-accent);
		color: white;
	}

	.btn-submit:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.submit-error {
		font-size: var(--font-size-xs);
		color: var(--color-error);
	}

	.answer-section {
		padding: var(--space-2);
		background: var(--color-bg-elevated);
		border-radius: var(--radius-md);
	}

	.answer-label {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		font-size: var(--font-size-xs);
		color: var(--color-success);
		margin-bottom: var(--space-1);
	}

	.answer-text {
		font-size: var(--font-size-sm);
		color: var(--color-text-primary);
	}

	.timeout-notice {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		font-size: var(--font-size-xs);
		color: var(--color-text-muted);
	}
</style>
