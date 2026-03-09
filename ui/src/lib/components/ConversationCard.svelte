<script lang="ts">
	import type { Conversation } from '$lib/models/conversation';
	import * as Card from '$lib/components/ui/card';
	import { goto } from '$app/navigation';
	import { MessageSquare } from 'lucide-svelte';
	import { stripMarkdown } from '$lib/markdown';

	let { conversation }: { conversation: Conversation } = $props();

	function handleClick() {
		goto(`/conversations/${conversation.id}`);
	}

	function getStatusColor(status: string) {
		switch (status) {
			case 'active':
				return 'bg-green-500/15 text-green-400 border-green-500/20';
			case 'archived':
				return 'bg-gray-500/15 text-gray-400 border-gray-500/20';
			default:
				return 'bg-gray-500/15 text-gray-400 border-gray-500/20';
		}
	}

	const lastMessage = $derived(
		conversation.messages.length > 0
			? conversation.messages[conversation.messages.length - 1]
			: null
	);

	const lastMessagePreview = $derived.by(() => {
		if (!lastMessage) return 'No messages yet';
		const stripped = stripMarkdown(lastMessage.content);
		return stripped.slice(0, 100) + (stripped.length > 100 ? '...' : '');
	});

	const timeAgo = $derived.by(() => {
		const date = new Date(conversation.updated_at);
		const now = new Date();
		const diffMs = now.getTime() - date.getTime();
		const diffMins = Math.floor(diffMs / 60000);
		if (diffMins < 1) return 'just now';
		if (diffMins < 60) return `${diffMins}m ago`;
		const diffHours = Math.floor(diffMins / 60);
		if (diffHours < 24) return `${diffHours}h ago`;
		const diffDays = Math.floor(diffHours / 24);
		return `${diffDays}d ago`;
	});
</script>

<Card.Root
	class="group p-3 cursor-pointer bg-[oklch(0.18_0.005_285.823)] shadow-sm hover:bg-accent/50 hover:border-blue-500/30 transition-all duration-200 hover:shadow-md border-blue-500/10"
	onclick={handleClick}
	role="button"
	tabindex={0}
>
	<div class="flex items-start justify-between gap-2">
		<div class="flex items-center gap-2 min-w-0">
			<MessageSquare class="w-4 h-4 text-blue-400 shrink-0" />
			<p class="font-medium text-sm line-clamp-1 flex-1">{conversation.title}</p>
		</div>
		<span class="inline-flex items-center text-[11px] font-semibold px-2 py-0.5 rounded-full border shrink-0 {getStatusColor(conversation.status)}">
			{conversation.status}
		</span>
	</div>
	<p class="text-xs text-muted-foreground mt-2 line-clamp-2 ml-6">{lastMessagePreview}</p>
	<div class="flex items-center gap-3 mt-2 ml-6">
		<span class="text-[10px] text-muted-foreground">
			{conversation.messages.length} message{conversation.messages.length !== 1 ? 's' : ''}
		</span>
		<span class="text-[10px] text-muted-foreground">{timeAgo}</span>
		{#if conversation.pending_message}
			<span class="text-[10px] text-blue-400 flex items-center gap-1">
				<span class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-pulse"></span>
				Processing
			</span>
		{/if}
	</div>
</Card.Root>
