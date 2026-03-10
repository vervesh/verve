<script lang="ts">
	import { client } from '$lib/api-client';
	import { conversationStore } from '$lib/stores/conversations.svelte';
	import { repoStore } from '$lib/stores/repos.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Loader2, MessageSquare, Type, FileText, ChevronDown, ChevronRight, Cpu } from 'lucide-svelte';

	let {
		open = $bindable(false),
		onCreated
	}: { open: boolean; onCreated: (conversationId: string) => void } = $props();

	let title = $state('');
	let initialMessage = $state('');
	let loading = $state(false);
	let error = $state<string | null>(null);
	let showAdvanced = $state(false);
	let selectedModel = $state('');
	let defaultModel = $state('');
	let availableModels = $state<{ value: string; label: string }[]>([]);

	$effect(() => {
		if (open) {
			client.getDefaultModel().then((res) => {
				defaultModel = res.model;
			}).catch(() => {});
			client.listModels().then((models) => {
				availableModels = models;
			}).catch(() => {});
		}
	});

	const modelOptions = $derived([
		{ value: '', label: 'Default' },
		...availableModels
	]);

	const defaultModelLabel = $derived(
		availableModels.find((m) => m.value === defaultModel)?.label || defaultModel
	);

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		if (!title.trim()) return;

		loading = true;
		error = null;

		try {
			const repoId = repoStore.selectedRepoId;
			if (!repoId) throw new Error('No repository selected');
			const conversation = await client.createConversation(
				repoId,
				title,
				initialMessage || undefined,
				selectedModel || undefined
			);
			conversationStore.addConversation(conversation);
			title = '';
			initialMessage = '';
			selectedModel = '';
			open = false;
			onCreated(conversation.id);
		} catch (err) {
			error = (err as Error).message;
		} finally {
			loading = false;
		}
	}

	function handleClose() {
		open = false;
		title = '';
		initialMessage = '';
		selectedModel = '';
		error = null;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-[650px] max-h-[90vh] overflow-y-auto">
		<Dialog.Header>
			<Dialog.Title class="flex items-center gap-2">
				<div class="w-8 h-8 rounded-lg bg-blue-500/10 flex items-center justify-center">
					<MessageSquare class="w-4 h-4 text-blue-500" />
				</div>
				New Conversation
			</Dialog.Title>
			<Dialog.Description>
				Start a conversation with an AI agent about your codebase. You can discuss ideas, plan features, and generate tasks.
			</Dialog.Description>
		</Dialog.Header>
		<form onsubmit={handleSubmit}>
			<div class="py-4 space-y-5">
				<div>
					<label for="conv-title" class="text-sm font-medium mb-2 flex items-center gap-2">
						<Type class="w-4 h-4 text-muted-foreground" />
						Title
					</label>
					<input
						id="conv-title"
						type="text"
						bind:value={title}
						maxlength={200}
						class="w-full border rounded-lg p-3 bg-background text-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-shadow"
						placeholder="e.g., Plan authentication system"
						disabled={loading}
					/>
					<p class="text-xs text-muted-foreground text-right mt-1">{title.length}/200</p>
				</div>

				<div>
					<label for="conv-message" class="text-sm font-medium mb-2 flex items-center gap-2">
						<FileText class="w-4 h-4 text-muted-foreground" />
						Initial Message
						<span class="text-xs text-muted-foreground font-normal">(optional)</span>
					</label>
					<textarea
						id="conv-message"
						bind:value={initialMessage}
						class="w-full border rounded-lg p-3 min-h-[120px] bg-background text-foreground resize-none focus:outline-none focus:ring-2 focus:ring-ring transition-shadow"
						placeholder="Start the conversation with a question or topic..."
						disabled={loading}
					></textarea>
				</div>

				<div>
					<button
						type="button"
						class="text-sm text-muted-foreground hover:text-foreground flex items-center gap-1 transition-colors"
						onclick={() => (showAdvanced = !showAdvanced)}
					>
						{#if showAdvanced}
							<ChevronDown class="w-4 h-4" />
						{:else}
							<ChevronRight class="w-4 h-4" />
						{/if}
						Advanced Options
					</button>

					{#if showAdvanced}
						<div class="mt-3 space-y-4 pl-1">
							<div>
								<label for="conv-model-select" class="text-sm font-medium mb-2 flex items-center gap-2">
									<Cpu class="w-4 h-4 text-muted-foreground" />
									Model
								</label>
								<select
									id="conv-model-select"
									bind:value={selectedModel}
									class="w-full border rounded-lg p-2 bg-background text-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-shadow text-sm"
									disabled={loading}
								>
									{#each modelOptions as option}
										<option value={option.value}>
											{option.label}{option.value === '' ? ` (${defaultModelLabel})` : ''}
										</option>
									{/each}
								</select>
							</div>
						</div>
					{/if}
				</div>

				{#if error}
					<div class="bg-destructive/10 text-destructive text-sm p-3 rounded-lg">
						{error}
					</div>
				{/if}
			</div>
			<Dialog.Footer>
				<Button type="button" variant="outline" onclick={handleClose} disabled={loading}>
					Cancel
				</Button>
				<Button type="submit" disabled={loading || !title.trim()} class="gap-2 bg-blue-600 hover:bg-blue-700">
					{#if loading}
						<Loader2 class="w-4 h-4 animate-spin" />
						Creating...
					{:else}
						<MessageSquare class="w-4 h-4" />
						Start Conversation
					{/if}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
