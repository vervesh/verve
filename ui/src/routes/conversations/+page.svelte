<script lang="ts">
	import { goto } from '$app/navigation';
	import { client } from '$lib/api-client';
	import { repoStore } from '$lib/stores/repos.svelte';
	import { conversationStore } from '$lib/stores/conversations.svelte';
	import ConversationCard from '$lib/components/ConversationCard.svelte';
	import CreateConversationDialog from '$lib/components/CreateConversationDialog.svelte';
	import RepoSetupBanner from '$lib/components/RepoSetupBanner.svelte';
	import { Button } from '$lib/components/ui/button';
	import {
		Plus,
		MessageSquare,
		GitBranch,
		AlertCircle
	} from 'lucide-svelte';

	let openCreateConversation = $state(false);

	$effect(() => {
		const repoId = repoStore.selectedRepoId;
		if (repoId) {
			loadConversations(repoId);
		} else {
			conversationStore.clear();
		}
	});

	async function loadConversations(repoId: string) {
		conversationStore.loading = true;
		try {
			const conversations = await client.listConversationsByRepo(repoId, 'all');
			conversationStore.setConversations(conversations);
		} catch {
			conversationStore.error = 'Failed to load conversations';
		} finally {
			conversationStore.loading = false;
		}
	}

	const hasRepo = $derived(!!repoStore.selectedRepoId);
	const repoReady = $derived(repoStore.selectedRepo?.setup_status === 'ready');
	const totalConversations = $derived(conversationStore.conversations.length);

	const activeConversations = $derived(conversationStore.activeConversations);
	const archivedConversations = $derived(conversationStore.archivedConversations);
</script>

<div class="p-4 sm:p-6 flex-1 min-h-0 flex flex-col">
	{#if hasRepo}
		<header class="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3 mb-4 sm:mb-6">
			<div>
				<div class="flex items-center gap-3">
					<h1 class="text-xl sm:text-2xl font-bold">Conversations</h1>
					{#if repoReady && totalConversations > 0}
						<span
							class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-500/15 text-blue-400"
						>
							{totalConversations} total
						</span>
					{/if}
				</div>
				<p class="text-muted-foreground text-sm mt-1 hidden sm:block">
					{repoReady ? 'Chat with AI agents about your codebase, plan features, and generate tasks' : 'Complete repository setup to start conversations'}
				</p>
			</div>
			{#if repoReady}
				<div class="flex items-center gap-2 sm:gap-3">
					<Button onclick={() => (openCreateConversation = true)} class="gap-2 bg-blue-600 hover:bg-blue-700 text-white">
						<Plus class="w-4 h-4" />
						New Conversation
					</Button>
				</div>
			{/if}
		</header>

		{#if !repoReady}
			<RepoSetupBanner />
		{:else}
			{#if conversationStore.error}
				<div
					class="bg-destructive/10 text-destructive p-4 rounded-lg mb-4 flex items-center gap-3 border border-destructive/20"
				>
					<AlertCircle class="w-5 h-5 flex-shrink-0" />
					<span>{conversationStore.error}</span>
				</div>
			{/if}

			{#if conversationStore.conversations.length === 0}
				<div class="flex-1 flex flex-col items-center justify-center text-center">
					<div class="w-16 h-16 rounded-2xl bg-blue-500/10 flex items-center justify-center mb-4">
						<MessageSquare class="w-8 h-8 text-blue-400" />
					</div>
					<h2 class="text-xl font-semibold mb-2">No conversations yet</h2>
					<p class="text-muted-foreground text-sm max-w-md mb-4">
						Start a conversation to discuss ideas, plan features, or explore your codebase with an AI agent.
					</p>
					<Button onclick={() => (openCreateConversation = true)} class="gap-2 bg-blue-600 hover:bg-blue-700 text-white">
						<Plus class="w-4 h-4" />
						Start Your First Conversation
					</Button>
				</div>
			{:else}
				{#if activeConversations.length > 0}
					<div class="mb-6">
						<h2 class="text-sm font-semibold text-muted-foreground mb-3">Active</h2>
						<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">
							{#each activeConversations as conversation (conversation.id)}
								<ConversationCard {conversation} />
							{/each}
						</div>
					</div>
				{/if}

				{#if archivedConversations.length > 0}
					<div>
						<h2 class="text-sm font-semibold text-muted-foreground mb-3">Archived</h2>
						<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">
							{#each archivedConversations as conversation (conversation.id)}
								<ConversationCard {conversation} />
							{/each}
						</div>
					</div>
				{/if}
			{/if}
		{/if}
	{:else}
		<div class="flex-1 flex flex-col items-center justify-center text-center">
			<div class="w-16 h-16 rounded-2xl bg-muted flex items-center justify-center mb-4">
				<GitBranch class="w-8 h-8 text-muted-foreground" />
			</div>
			<h2 class="text-xl font-semibold mb-2">No repository selected</h2>
			<p class="text-muted-foreground text-sm max-w-md">
				Add a GitHub repository to get started. Conversations are scoped to individual repositories.
			</p>
		</div>
	{/if}
</div>

{#if hasRepo && repoReady}
	<CreateConversationDialog bind:open={openCreateConversation} onCreated={(id) => goto(`/conversations/${id}`)} />
{/if}
