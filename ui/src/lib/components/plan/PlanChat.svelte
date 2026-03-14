<script lang="ts">
	import ChatPanel from '$lib/components/activity/ChatPanel.svelte';
	import ChatDropZone from '$lib/components/chat/ChatDropZone.svelte';
	import { plansStore } from '$lib/stores/plans.svelte';

	interface Props {
		planSlug: string;
	}

	let { planSlug }: Props = $props();

	// Get project_id from plan
	const projectId = $derived.by(() => {
		const plan = plansStore.getBySlug(planSlug);
		return plan?.project_id ?? 'default';
	});
</script>

<div class="plan-chat">
	<div class="chat-section">
		<ChatDropZone {projectId}>
			<ChatPanel title="Plan Chat" {planSlug} />
		</ChatDropZone>
	</div>
</div>

<style>
	.plan-chat {
		display: flex;
		flex-direction: column;
		height: 100%;
	}

	.chat-section {
		flex: 1;
		min-height: 0;
		display: flex;
		flex-direction: column;
	}
</style>
