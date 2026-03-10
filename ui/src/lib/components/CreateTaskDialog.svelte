<script lang="ts">
	import { client } from '$lib/api-client';
	import { taskStore } from '$lib/stores/tasks.svelte';
	import { repoStore } from '$lib/stores/repos.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Badge } from '$lib/components/ui/badge';
	import { FileText, Link2, Search, X, Loader2, Sparkles, ChevronDown, ChevronRight, Target, DollarSign, GitBranch, GitPullRequestDraft, Plus, Type, Cpu } from 'lucide-svelte';

	let {
		open = $bindable(false),
		onCreated
	}: { open: boolean; onCreated: () => void } = $props();

	let title = $state('');
	let description = $state('');
	let loading = $state(false);
	let error = $state<string | null>(null);
	let selectedDeps = $state<string[]>([]);
	let searchQuery = $state('');
	let acceptanceCriteria = $state<string[]>([]);
	let maxCostUsd = $state<number | undefined>(undefined);
	let skipPr = $state(false);
	let draftPr = $state(false);
	let notReady = $state(false);
	let showAdvanced = $state(false);
	let selectedModel = $state('');
	let defaultModel = $state('');
	let availableModels = $state<{ value: string; label: string }[]>([]);

	// Fetch default model and available models when dialog opens
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

	// Filter available tasks (exclude closed/failed and already selected)
	const availableTasks = $derived(
		taskStore.tasks.filter(
			(t) =>
				!['closed', 'failed'].includes(t.status) &&
				!selectedDeps.includes(t.id) &&
				(searchQuery === '' ||
					`#${t.number}`.includes(searchQuery) ||
					t.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
					t.description.toLowerCase().includes(searchQuery.toLowerCase()))
		)
	);

	// Lookup map from task ID to task number for dependency display
	const taskNumberMap = $derived(
		Object.fromEntries(taskStore.tasks.map((t) => [t.id, t.number]))
	);

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		if (!title.trim()) return;

		loading = true;
		error = null;

		try {
			const repoId = repoStore.selectedRepoId;
			if (!repoId) throw new Error('No repository selected');
			const filteredCriteria = acceptanceCriteria.filter((c) => c.trim() !== '');
			await client.createTaskInRepo(
				repoId,
				title,
				description,
				selectedDeps.length > 0 ? selectedDeps : undefined,
				filteredCriteria.length > 0 ? filteredCriteria : undefined,
				maxCostUsd,
				skipPr || undefined,
				draftPr || undefined,
				selectedModel || undefined,
				notReady || undefined
			);
			title = '';
			description = '';
			selectedDeps = [];
			acceptanceCriteria = [];
			maxCostUsd = undefined;
			skipPr = false;
			draftPr = false;
			notReady = false;
			selectedModel = '';
			showAdvanced = false;
			open = false;
			onCreated();
		} catch (err) {
			error = (err as Error).message;
		} finally {
			loading = false;
		}
	}

	function handleClose() {
		open = false;
		title = '';
		description = '';
		selectedDeps = [];
		acceptanceCriteria = [];
		maxCostUsd = undefined;
		skipPr = false;
		draftPr = false;
		notReady = false;
		selectedModel = '';
		showAdvanced = false;
		error = null;
		searchQuery = '';
	}

	function addDependency(taskId: string) {
		selectedDeps = [...selectedDeps, taskId];
		searchQuery = '';
	}

	function removeDependency(taskId: string) {
		selectedDeps = selectedDeps.filter((id) => id !== taskId);
	}

	function addCriterion() {
		acceptanceCriteria = [...acceptanceCriteria, ''];
	}

	function removeCriterion(index: number) {
		acceptanceCriteria = acceptanceCriteria.filter((_, i) => i !== index);
	}

	function updateCriterion(index: number, value: string) {
		acceptanceCriteria = acceptanceCriteria.map((c, i) => (i === index ? value : c));
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-[900px] max-h-[85vh] sm:max-h-[90vh] overflow-y-auto">
		<Dialog.Header>
			<Dialog.Title class="flex items-center gap-2">
				<div class="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center">
					<Sparkles class="w-4 h-4 text-primary" />
				</div>
				Create New Task
			</Dialog.Title>
			<Dialog.Description>
				Describe the task you want the AI agent to complete. Be specific for best results.
			</Dialog.Description>
		</Dialog.Header>
		<form onsubmit={handleSubmit}>
			<div class="py-4 space-y-7">
				<div class="space-y-3">
					<div>
						<label for="title" class="text-sm font-medium mb-2 flex items-center gap-2">
							<Type class="w-4 h-4 text-muted-foreground" />
							Title
						</label>
						<input
							id="title"
							type="text"
							bind:value={title}
							autofocus
							maxlength={150}
							class="w-full border rounded-lg p-3 bg-background text-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-shadow"
							placeholder="e.g., Add Fibonacci function with unit tests"
							disabled={loading}
						/>
						<div class="flex items-center justify-between mt-1">
							<label
								for="not-ready"
								class="flex items-start gap-2 cursor-pointer"
							>
								<input
									id="not-ready"
									type="checkbox"
									bind:checked={notReady}
									class="w-3.5 h-3.5 rounded border-input accent-primary mt-0.5"
									disabled={loading}
								/>
								<div>
									<span class="text-sm">Mark as not ready</span>
									<span class="block text-xs text-muted-foreground">Agent will not receive task until marked as ready</span>
								</div>
							</label>
							<p class="text-xs text-muted-foreground text-right self-start">{title.length}/150</p>
						</div>
					</div>

					<div>
						<label for="description" class="text-sm font-medium mb-2 flex items-center gap-2">
							<FileText class="w-4 h-4 text-muted-foreground" />
							Description
							<span class="text-xs text-muted-foreground font-normal">(optional)</span>
						</label>
						<textarea
							id="description"
							bind:value={description}
							class="w-full border rounded-lg p-3 min-h-[120px] sm:min-h-[240px] bg-background text-foreground resize-y focus:outline-none focus:ring-2 focus:ring-ring transition-shadow"
							placeholder="Detailed description of what needs to be done..."
							disabled={loading}
						></textarea>
					</div>
				</div>

				<hr class="border-border" />

				<div>
					<label class="text-sm font-medium mb-2 flex items-center gap-2">
						<Target class="w-4 h-4 text-muted-foreground" />
						Acceptance Criteria
						<span class="text-xs text-muted-foreground font-normal">(optional)</span>
					</label>
					{#if acceptanceCriteria.length > 0}
						<div class="space-y-2 mb-2">
							{#each acceptanceCriteria as criterion, i}
								<div class="flex items-center gap-2">
									<span class="text-xs text-muted-foreground font-mono w-5 shrink-0 text-right">{i + 1}.</span>
									<input
										type="text"
										value={criterion}
										oninput={(e) => updateCriterion(i, (e.target as HTMLInputElement).value)}
										class="flex-1 border rounded-lg px-3 py-2 bg-background text-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-shadow text-sm"
										placeholder="e.g., All tests pass"
										disabled={loading}
									/>
									<button
										type="button"
										class="p-1.5 hover:bg-destructive/10 hover:text-destructive rounded transition-colors shrink-0"
										onclick={() => removeCriterion(i)}
									>
										<X class="w-3.5 h-3.5" />
									</button>
								</div>
							{/each}
						</div>
					{/if}
					<Button
						type="button"
						variant="outline"
						size="sm"
						onclick={addCriterion}
						disabled={loading}
						class="gap-1.5 text-xs"
					>
						<Plus class="w-3.5 h-3.5" />
						Add criterion
					</Button>
				</div>

				<hr class="border-border" />

				<div>
					<label for="dep-search" class="text-sm font-medium mb-2 flex items-center gap-2">
						<Link2 class="w-4 h-4 text-muted-foreground" />
						Dependencies
						<span class="text-xs text-muted-foreground font-normal">(optional)</span>
					</label>

					{#if selectedDeps.length > 0}
						<div class="flex flex-wrap gap-1.5 mb-3 max-h-20 overflow-y-auto">
							{#each selectedDeps as depId}
								<Badge variant="secondary" class="gap-1 pl-2 pr-1 py-1 max-w-48">
									<span class="font-mono text-xs truncate">{taskNumberMap[depId] ? `#${taskNumberMap[depId]}` : '(loading...)'}</span>
									<button
										type="button"
										class="ml-1 hover:bg-destructive/20 hover:text-destructive rounded p-0.5 transition-colors shrink-0"
										onclick={() => removeDependency(depId)}
									>
										<X class="w-3 h-3" />
									</button>
								</Badge>
							{/each}
						</div>
					{/if}

					<div class="relative">
						<Search class="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
						<input
							id="dep-search"
							type="text"
							bind:value={searchQuery}
							class="w-full border rounded-lg pl-9 pr-3 py-2 bg-background text-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-shadow"
							placeholder="Search tasks to add as dependency..."
							disabled={loading}
							autocomplete="off"
						/>
					</div>

					<div class="mt-2 border rounded-lg max-h-36 overflow-y-auto bg-muted/20">
						{#if availableTasks.length > 0}
							{#each availableTasks as task (task.id)}
								<button
									type="button"
									class="w-full text-left px-3 py-2.5 hover:bg-accent cursor-pointer border-b last:border-b-0 transition-colors overflow-hidden"
									onclick={() => addDependency(task.id)}
								>
									<div class="flex items-center gap-2">
										<span class="font-mono text-xs text-muted-foreground bg-background px-1.5 py-0.5 rounded shrink-0">
											#{task.number}
										</span>
										<span class="text-sm truncate">{task.title || task.description}</span>
									</div>
								</button>
							{/each}
						{:else if searchQuery}
							<div class="p-4 text-sm text-muted-foreground text-center">
								<Search class="w-5 h-5 mx-auto mb-2 opacity-40" />
								No matching tasks found
							</div>
						{:else}
							<div class="p-4 text-sm text-muted-foreground text-center">
								<Link2 class="w-5 h-5 mx-auto mb-2 opacity-40" />
								No tasks available as dependencies
							</div>
						{/if}
					</div>
				</div>

				<div>
					<button
						type="button"
						class="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
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
								<label for="model-select" class="text-sm font-medium mb-2 flex items-center gap-2">
									<Cpu class="w-4 h-4 text-muted-foreground" />
									Model
								</label>
								<select
									id="model-select"
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
							<div>
								<label for="max-cost" class="text-sm font-medium mb-2 flex items-center gap-2">
									<DollarSign class="w-4 h-4 text-muted-foreground" />
									Max Cost (USD)
									<span class="text-xs text-muted-foreground font-normal">(optional)</span>
								</label>
								<input
									id="max-cost"
									type="number"
									step="0.01"
									min="0"
									bind:value={maxCostUsd}
									class="w-full border rounded-lg p-2 bg-background text-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-shadow text-sm"
									placeholder="e.g., 5.00"
									disabled={loading}
								/>
							</div>
							<label
								for="skip-pr"
								class="flex items-center gap-3 p-3 rounded-lg border cursor-pointer hover:bg-accent/50 transition-colors {draftPr ? 'opacity-50' : ''}"
							>
								<input
									id="skip-pr"
									type="checkbox"
									bind:checked={skipPr}
									onchange={() => { if (skipPr) draftPr = false; }}
									class="w-4 h-4 rounded border-input accent-primary"
									disabled={loading || draftPr}
								/>
								<div class="flex-1">
									<div class="text-sm font-medium flex items-center gap-1.5">
										<GitBranch class="w-3.5 h-3.5 text-muted-foreground" />
										Push to branch only
									</div>
									<p class="text-xs text-muted-foreground mt-0.5">
										Skip PR creation. You can create the PR manually and it will be detected on sync.
									</p>
								</div>
							</label>
							<label
								for="draft-pr"
								class="flex items-center gap-3 p-3 rounded-lg border cursor-pointer hover:bg-accent/50 transition-colors {skipPr ? 'opacity-50' : ''}"
							>
								<input
									id="draft-pr"
									type="checkbox"
									bind:checked={draftPr}
									onchange={() => { if (draftPr) skipPr = false; }}
									class="w-4 h-4 rounded border-input accent-primary"
									disabled={loading || skipPr}
								/>
								<div class="flex-1">
									<div class="text-sm font-medium flex items-center gap-1.5">
										<GitPullRequestDraft class="w-3.5 h-3.5 text-muted-foreground" />
										Create as draft PR
									</div>
									<p class="text-xs text-muted-foreground mt-0.5">
										Open the pull request as a draft. Useful for work-in-progress or early review.
									</p>
								</div>
							</label>
						</div>
					{/if}
				</div>

				{#if error}
					<div class="bg-destructive/10 text-destructive text-sm p-3 rounded-lg flex items-center gap-2">
						<X class="w-4 h-4 flex-shrink-0" />
						{error}
					</div>
				{/if}
			</div>
			<Dialog.Footer>
				<div class="flex flex-col-reverse sm:flex-row justify-end gap-2 w-full">
					<Button type="button" variant="outline" onclick={handleClose} disabled={loading}>
						Cancel
					</Button>
					<Button type="submit" disabled={loading || !title.trim()} class="gap-2">
						{#if loading}
							<Loader2 class="w-4 h-4 animate-spin" />
							Creating...
						{:else}
							<Sparkles class="w-4 h-4" />
							Create Task
						{/if}
					</Button>
				</div>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
