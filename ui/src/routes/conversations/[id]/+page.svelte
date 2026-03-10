<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { onMount, onDestroy } from 'svelte';
	import { client } from '$lib/api-client';
	import { repoStore } from '$lib/stores/repos.svelte';
	import { conversationStore } from '$lib/stores/conversations.svelte';
	import { epicUrl } from '$lib/utils';
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import type { Conversation } from '$lib/models/conversation';
	import { renderMarkdown } from '$lib/markdown';
	import {
		ArrowLeft,
		MessageSquare,
		Loader2,
		Trash2,
		Archive,
		Send,
		AlertCircle,
		Layers,
		ExternalLink,
		User,
		Bot
	} from 'lucide-svelte';

	let conversation = $state<Conversation | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Message input
	let messageInput = $state('');
	let sendingMessage = $state(false);

	// Actions
	let showDeleteConfirm = $state(false);
	let deleting = $state(false);
	let archiving = $state(false);

	// Generate tasks dialog
	let showGenerateDialog = $state(false);
	let generateTitle = $state('');
	let generatePrompt = $state('');
	let generating = $state(false);

	// Polling
	let pollTimer: ReturnType<typeof setInterval> | null = null;

	// Auto-scroll
	let messagesContainer: HTMLDivElement | null = $state(null);
	let autoScroll = $state(true);
	let lastMessageCount = $state(0);

	const conversationId = $derived($page.params.id);
	const isPending = $derived(!!conversation?.pending_message);
	const hasEpic = $derived(!!conversation?.epic_id);

	async function navigateToEpic() {
		if (!conversation?.epic_id) return;
		try {
			const epic = await client.getEpic(conversation.epic_id);
			const repo = repoStore.repos.find((r) => r.id === conversation!.repo_id);
			if (repo) {
				goto(epicUrl(repo.owner, repo.name, epic.number));
			}
		} catch {
			// Ignore navigation errors
		}
	}

	onMount(async () => {
		await loadConversation();
	});

	onDestroy(() => {
		stopPolling();
	});

	function startPolling() {
		if (pollTimer) return;
		pollTimer = setInterval(async () => {
			if (!conversation) return;
			try {
				const updated = await client.getConversation(conversation.id);
				conversation = updated;
				conversationStore.updateConversation(updated);
				if (!updated.pending_message) {
					stopPolling();
					sendingMessage = false;
				}
			} catch {
				// Ignore polling errors
			}
		}, 2000);
	}

	function stopPolling() {
		if (pollTimer) {
			clearInterval(pollTimer);
			pollTimer = null;
		}
	}

	async function loadConversation() {
		loading = true;
		error = null;
		try {
			const id = conversationId;
			if (!id) {
				error = 'Conversation ID is required';
				return;
			}
			conversation = await client.getConversation(id);
			if (conversation.pending_message) {
				sendingMessage = true;
				startPolling();
			}
		} catch (err) {
			error = (err as Error).message;
		} finally {
			loading = false;
		}
	}

	async function handleSendMessage() {
		if (!messageInput.trim() || !conversation || sendingMessage) return;
		const message = messageInput.trim();
		messageInput = '';
		sendingMessage = true;
		error = null;

		try {
			const updated = await client.sendConversationMessage(conversation.id, message);
			conversation = updated;
			conversationStore.updateConversation(updated);
			if (updated.pending_message) {
				startPolling();
			}
		} catch (err) {
			error = (err as Error).message;
			sendingMessage = false;
		}
	}

	async function handleArchive() {
		if (!conversation) return;
		archiving = true;
		error = null;
		try {
			const updated = await client.archiveConversation(conversation.id);
			conversation = updated;
			conversationStore.updateConversation(updated);
		} catch (err) {
			error = (err as Error).message;
		} finally {
			archiving = false;
		}
	}

	async function handleDelete() {
		if (!conversation) return;
		deleting = true;
		error = null;
		try {
			await client.deleteConversation(conversation.id);
			conversationStore.removeConversation(conversation.id);
			goto('/conversations');
		} catch (err) {
			error = (err as Error).message;
		} finally {
			deleting = false;
			showDeleteConfirm = false;
		}
	}

	async function handleGenerateTasks() {
		if (!conversation || !generateTitle.trim()) return;
		generating = true;
		error = null;
		try {
			const createdEpic = await client.generateTasksFromConversation(
				conversation.id,
				generateTitle,
				generatePrompt || undefined
			);
			showGenerateDialog = false;
			generateTitle = '';
			generatePrompt = '';
			// Reload to get the updated epic_id
			conversation = await client.getConversation(conversation.id);
			conversationStore.updateConversation(conversation);
			const repo = repoStore.repos.find((r) => r.id === conversation!.repo_id);
			if (repo) {
				goto(epicUrl(repo.owner, repo.name, createdEpic.number));
			}
		} catch (err) {
			error = (err as Error).message;
		} finally {
			generating = false;
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			handleSendMessage();
		}
	}

	function formatTimestamp(ts: number): string {
		const date = new Date(ts * 1000);
		return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
	}

	function handleScroll(e: Event) {
		const el = e.target as HTMLDivElement;
		const isNearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 50;
		autoScroll = isNearBottom;
	}

	// Auto-scroll when new messages arrive
	$effect(() => {
		const msgCount = conversation?.messages?.length ?? 0;
		if (msgCount > lastMessageCount) {
			lastMessageCount = msgCount;
			if (autoScroll && messagesContainer) {
				requestAnimationFrame(() => {
					if (messagesContainer) {
						messagesContainer.scrollTop = messagesContainer.scrollHeight;
					}
				});
			}
		}
	});
</script>

<div class="flex-1 min-h-0 flex flex-col">
	{#if loading}
		<div class="flex items-center justify-center py-20">
			<Loader2 class="w-8 h-8 animate-spin text-muted-foreground" />
		</div>
	{:else if error && !conversation}
		<div class="p-4 sm:p-6">
			<div class="bg-destructive/10 text-destructive p-4 rounded-lg flex items-center gap-3">
				<AlertCircle class="w-5 h-5" />
				{error}
			</div>
		</div>
	{:else if conversation}
		<!-- Header -->
		<header class="border-b px-4 sm:px-6 py-3 shrink-0">
			<div class="flex items-center gap-3">
				<button
					onclick={() => goto('/conversations')}
					class="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors shrink-0"
				>
					<ArrowLeft class="w-4 h-4" />
				</button>
				<div class="flex items-center gap-2 flex-1 min-w-0">
					<MessageSquare class="w-5 h-5 text-blue-400 shrink-0" />
					<h1 class="text-lg font-semibold truncate">{conversation.title}</h1>
					{#if conversation.status === 'archived'}
						<span class="inline-flex items-center text-[11px] font-semibold px-2 py-0.5 rounded-full border bg-gray-500/15 text-gray-400 border-gray-500/20 shrink-0">
							archived
						</span>
					{/if}
				</div>
				<div class="hidden sm:flex items-center gap-2 shrink-0">
					{#if hasEpic}
						<Button variant="outline" size="sm" onclick={navigateToEpic} class="gap-1.5 text-xs">
							<ExternalLink class="w-3.5 h-3.5" />
							View Generated Tasks
						</Button>
					{:else}
						<Button variant="outline" size="sm" onclick={() => (showGenerateDialog = true)} class="gap-1.5 text-xs">
							<Layers class="w-3.5 h-3.5" />
							Generate Tasks
						</Button>
					{/if}
					{#if conversation.status === 'active'}
						<Button variant="outline" size="sm" onclick={handleArchive} disabled={archiving} class="gap-1.5 text-xs text-muted-foreground">
							{#if archiving}
								<Loader2 class="w-3.5 h-3.5 animate-spin" />
							{:else}
								<Archive class="w-3.5 h-3.5" />
							{/if}
							Archive
						</Button>
					{/if}
					<Button variant="outline" size="sm" onclick={() => (showDeleteConfirm = true)} class="gap-1.5 text-xs text-red-400 border-red-500/30 hover:bg-red-500/10">
						<Trash2 class="w-3.5 h-3.5" />
						Delete
					</Button>
				</div>
			</div>
			<!-- Mobile action buttons -->
			<div class="flex sm:hidden items-center gap-2 mt-2">
				{#if hasEpic}
					<Button variant="outline" size="sm" onclick={navigateToEpic} class="gap-1.5 text-xs shrink-0">
						<ExternalLink class="w-3.5 h-3.5" />
						View Tasks
					</Button>
				{:else}
					<Button variant="outline" size="sm" onclick={() => (showGenerateDialog = true)} class="gap-1.5 text-xs shrink-0">
						<Layers class="w-3.5 h-3.5" />
						Generate
					</Button>
				{/if}
				{#if conversation.status === 'active'}
					<Button variant="outline" size="sm" onclick={handleArchive} disabled={archiving} class="gap-1.5 text-xs text-muted-foreground shrink-0">
						{#if archiving}
							<Loader2 class="w-3.5 h-3.5 animate-spin" />
						{:else}
							<Archive class="w-3.5 h-3.5" />
						{/if}
						Archive
					</Button>
				{/if}
				<Button variant="outline" size="sm" onclick={() => (showDeleteConfirm = true)} class="gap-1.5 text-xs text-red-400 border-red-500/30 hover:bg-red-500/10 shrink-0">
					<Trash2 class="w-3.5 h-3.5" />
					Delete
				</Button>
			</div>
		</header>

		{#if error}
			<div class="px-4 sm:px-6 pt-3">
				<div class="bg-destructive/10 text-destructive p-3 rounded-lg text-sm flex items-center gap-2">
					<AlertCircle class="w-4 h-4 shrink-0" />
					{error}
				</div>
			</div>
		{/if}

		<!-- Messages -->
		<div
			bind:this={messagesContainer}
			onscroll={handleScroll}
			class="flex-1 overflow-y-auto p-4 sm:p-6 space-y-4"
		>
			{#if conversation.messages.length === 0 && !isPending}
				<div class="flex flex-col items-center justify-center text-center py-12">
					<div class="w-12 h-12 rounded-xl bg-blue-500/10 flex items-center justify-center mb-3">
						<MessageSquare class="w-6 h-6 text-blue-400" />
					</div>
					<p class="text-sm text-muted-foreground">
						No messages yet. Send a message to start the conversation.
					</p>
				</div>
			{/if}

			{#each conversation.messages as message, i (i)}
				{#if message.role === 'user'}
					<!-- User message - right aligned -->
					<div class="flex justify-end">
						<div class="max-w-[80%] flex flex-col items-end gap-1">
							<div class="flex items-center gap-1.5 text-xs text-muted-foreground">
								<User class="w-3 h-3" />
								You
								{#if message.timestamp}
									<span class="text-muted-foreground/50">{formatTimestamp(message.timestamp)}</span>
								{/if}
							</div>
							<div class="bg-blue-600 text-white rounded-2xl rounded-tr-sm px-4 py-2.5">
								<p class="text-sm whitespace-pre-wrap">{message.content}</p>
							</div>
						</div>
					</div>
				{:else}
					<!-- Assistant message - left aligned -->
					<div class="flex justify-start">
						<div class="max-w-[80%] flex flex-col items-start gap-1">
							<div class="flex items-center gap-1.5 text-xs text-muted-foreground">
								<Bot class="w-3 h-3" />
								Assistant
								{#if message.timestamp}
									<span class="text-muted-foreground/50">{formatTimestamp(message.timestamp)}</span>
								{/if}
							</div>
							<div class="bg-[oklch(0.18_0.005_285.823)] border rounded-2xl rounded-tl-sm px-4 py-2.5">
								<div class="prose prose-sm prose-invert max-w-none prose-headings:text-zinc-100 prose-p:text-zinc-300 prose-strong:text-zinc-200 prose-a:text-blue-400 prose-code:text-zinc-300 prose-code:bg-zinc-900 prose-code:px-1 prose-code:py-0.5 prose-code:rounded prose-code:before:content-none prose-code:after:content-none prose-pre:bg-zinc-900 prose-pre:border prose-pre:border-zinc-700 prose-blockquote:border-zinc-600 prose-li:text-zinc-300 [&_pre_code]:bg-transparent [&_pre_code]:p-0 [&_pre]:overflow-x-auto">
									{@html renderMarkdown(message.content)}
								</div>
							</div>
						</div>
					</div>
				{/if}
			{/each}

			<!-- Typing indicator when pending -->
			{#if isPending}
				<div class="flex justify-start">
					<div class="max-w-[80%] flex flex-col items-start gap-1">
						<div class="flex items-center gap-1.5 text-xs text-muted-foreground">
							<Bot class="w-3 h-3" />
							Assistant
						</div>
						<div class="bg-[oklch(0.18_0.005_285.823)] border rounded-2xl rounded-tl-sm px-4 py-3">
							<div class="flex items-center gap-1.5">
								<span class="w-2 h-2 rounded-full bg-blue-400 animate-pulse"></span>
								<span class="w-2 h-2 rounded-full bg-blue-400 animate-pulse" style="animation-delay: 0.2s"></span>
								<span class="w-2 h-2 rounded-full bg-blue-400 animate-pulse" style="animation-delay: 0.4s"></span>
							</div>
						</div>
					</div>
				</div>
			{/if}
		</div>

		<!-- Input area -->
		<div class="border-t px-4 sm:px-6 py-3 shrink-0">
			<div class="flex items-end gap-2">
				<textarea
					bind:value={messageInput}
					onkeydown={handleKeydown}
					placeholder={isPending ? 'Waiting for response...' : 'Type a message...'}
					disabled={sendingMessage || conversation.status === 'archived'}
					rows={1}
					class="flex-1 border rounded-lg px-3 py-2.5 bg-background text-foreground text-sm resize-none focus:outline-none focus:ring-2 focus:ring-ring transition-shadow min-h-[42px] max-h-[120px] disabled:opacity-50"
				></textarea>
				<Button
					onclick={handleSendMessage}
					disabled={!messageInput.trim() || sendingMessage || conversation.status === 'archived'}
					class="shrink-0 gap-1.5 bg-blue-600 hover:bg-blue-700 h-[42px]"
				>
					{#if sendingMessage}
						<Loader2 class="w-4 h-4 animate-spin" />
					{:else}
						<Send class="w-4 h-4" />
					{/if}
				</Button>
			</div>
		</div>
	{/if}
</div>

<!-- Delete confirmation dialog -->
{#if showDeleteConfirm}
	<div class="fixed inset-0 bg-black/60 flex items-center justify-center z-50" role="dialog">
		<div class="bg-background border rounded-xl p-6 max-w-md w-full mx-4 shadow-xl">
			<div class="flex items-center gap-3 mb-4">
				<div class="w-10 h-10 rounded-full bg-red-500/15 flex items-center justify-center shrink-0">
					<Trash2 class="w-5 h-5 text-red-400" />
				</div>
				<div>
					<h3 class="font-semibold">Delete Conversation</h3>
					<p class="text-sm text-muted-foreground">This action cannot be undone.</p>
				</div>
			</div>
			<p class="text-sm text-muted-foreground mb-1">
				This will permanently delete the conversation <strong class="text-foreground">{conversation?.title}</strong> and all its messages.
			</p>
			<div class="flex items-center gap-2 justify-end mt-6">
				<Button variant="ghost" size="sm" onclick={() => (showDeleteConfirm = false)} disabled={deleting}>
					Cancel
				</Button>
				<Button variant="destructive" size="sm" onclick={handleDelete} disabled={deleting} class="gap-1.5">
					{#if deleting}
						<Loader2 class="w-3.5 h-3.5 animate-spin" />
						Deleting...
					{:else}
						<Trash2 class="w-3.5 h-3.5" />
						Delete
					{/if}
				</Button>
			</div>
		</div>
	</div>
{/if}

<!-- Generate Tasks dialog -->
<Dialog.Root bind:open={showGenerateDialog}>
	<Dialog.Content class="sm:max-w-[500px]">
		<Dialog.Header>
			<Dialog.Title class="flex items-center gap-2">
				<div class="w-8 h-8 rounded-lg bg-violet-500/10 flex items-center justify-center">
					<Layers class="w-4 h-4 text-violet-500" />
				</div>
				Generate Tasks
			</Dialog.Title>
			<Dialog.Description>
				Create an epic with tasks based on this conversation. An AI agent will analyze the discussion and propose a task breakdown.
			</Dialog.Description>
		</Dialog.Header>
		<div class="py-4 space-y-4">
			<div>
				<label for="gen-title" class="text-sm font-medium mb-2 block">Epic Title</label>
				<input
					id="gen-title"
					type="text"
					bind:value={generateTitle}
					class="w-full border rounded-lg p-3 bg-background text-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-shadow"
					placeholder="e.g., Implement authentication system"
					disabled={generating}
				/>
			</div>
			<div>
				<label for="gen-prompt" class="text-sm font-medium mb-2 block">
					Planning Instructions
					<span class="text-xs text-muted-foreground font-normal">(optional)</span>
				</label>
				<textarea
					id="gen-prompt"
					bind:value={generatePrompt}
					class="w-full border rounded-lg p-3 min-h-[80px] bg-background text-foreground resize-none focus:outline-none focus:ring-2 focus:ring-ring transition-shadow"
					placeholder="e.g., Break into small tasks, prioritize backend first..."
					disabled={generating}
				></textarea>
			</div>
			{#if error}
				<div class="bg-destructive/10 text-destructive text-sm p-3 rounded-lg">
					{error}
				</div>
			{/if}
		</div>
		<Dialog.Footer>
			<div class="flex justify-end gap-2 w-full">
				<Button type="button" variant="outline" onclick={() => (showGenerateDialog = false)} disabled={generating}>
					Cancel
				</Button>
				<Button onclick={handleGenerateTasks} disabled={generating || !generateTitle.trim()} class="gap-2 bg-violet-600 hover:bg-violet-700">
					{#if generating}
						<Loader2 class="w-4 h-4 animate-spin" />
						Generating...
					{:else}
						<Layers class="w-4 h-4" />
						Generate
					{/if}
				</Button>
			</div>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
