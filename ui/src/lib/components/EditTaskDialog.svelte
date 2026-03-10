<script lang="ts">
	import { client } from '$lib/api-client';
	import { taskStore } from '$lib/stores/tasks.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Badge } from '$lib/components/ui/badge';
	import type { Task } from '$lib/models/task';
	import { FileText, Link2, Search, X, Loader2, ChevronDown, ChevronRight, Target, DollarSign, GitBranch, GitPullRequestDraft, Plus, Type, Cpu, Pencil, Check } from 'lucide-svelte';

	let {
		open = $bindable(false),
		task,
		onUpdated
	}: { open: boolean; task: Task; onUpdated: (updated: Task) => void } = $props();

	let editTitle = $state('');
	let editDescription = $state('');
	let loading = $state(false);
	let error = $state<string | null>(null);
	let editDeps = $state<string[]>([]);
	let editDepSearch = $state('');
	let editCriteria = $state<string[]>([]);
	let editMaxCostUsd = $state<number | undefined>(undefined);
	let editSkipPr = $state(false);
	let editDraftPr = $state(false);
	let editNotReady = $state(false);
	let editShowAdvanced = $state(false);
	let editModel = $state('');

	// Populate form when dialog opens
	$effect(() => {
		if (open && task) {
			editTitle = task.title;
			editDescription = task.description;
			editCriteria = [...(task.acceptance_criteria ?? [])];
			editDeps = [...(task.depends_on ?? [])];
			editMaxCostUsd = task.max_cost_usd;
			editSkipPr = task.skip_pr;
			editDraftPr = task.draft_pr;
			editModel = task.model ?? '';
			editNotReady = !task.ready;
			editShowAdvanced = !!(task.model || task.max_cost_usd || task.skip_pr || task.draft_pr);
			editDepSearch = '';
			error = null;
		}
	});

	let availableModels = $state<{ value: string; label: string }[]>([]);

	// Fetch available models when dialog opens
	$effect(() => {
		if (open) {
			client.listModels().then((models) => {
				availableModels = models;
			}).catch(() => {});
		}
	});

	const modelOptions = $derived([
		{ value: '', label: 'Default' },
		...availableModels
	]);

	// Filter available tasks (exclude current task, closed/failed, and already selected)
	const editAvailableTasks = $derived(
		taskStore.tasks.filter(
			(t) =>
				t.id !== task?.id &&
				!['closed', 'failed'].includes(t.status) &&
				!editDeps.includes(t.id) &&
				(editDepSearch === '' ||
					`#${t.number}`.includes(editDepSearch) ||
					t.title.toLowerCase().includes(editDepSearch.toLowerCase()) ||
					t.description.toLowerCase().includes(editDepSearch.toLowerCase()))
		)
	);

	// Lookup map from task ID to task number for dependency display
	const taskNumberMap = $derived(
		Object.fromEntries(taskStore.tasks.map((t) => [t.id, t.number]))
	);

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		if (!task || !editTitle.trim()) return;

		loading = true;
		error = null;

		try {
			const updates: Record<string, unknown> = {};
			if (editTitle !== task.title) updates.title = editTitle;
			if (editDescription !== task.description) updates.description = editDescription;

			const filteredCriteria = editCriteria.filter((c) => c.trim() !== '');
			const oldCriteria = task.acceptance_criteria ?? [];
			if (JSON.stringify(filteredCriteria) !== JSON.stringify(oldCriteria)) {
				updates.acceptance_criteria = filteredCriteria;
			}

			const oldDeps = task.depends_on ?? [];
			if (JSON.stringify(editDeps) !== JSON.stringify(oldDeps)) {
				updates.depends_on = editDeps;
			}

			const oldMaxCost = task.max_cost_usd ?? undefined;
			if (editMaxCostUsd !== oldMaxCost) {
				updates.max_cost_usd = editMaxCostUsd ?? 0;
			}

			if (editSkipPr !== task.skip_pr) {
				updates.skip_pr = editSkipPr;
			}

			if (editDraftPr !== task.draft_pr) {
				updates.draft_pr = editDraftPr;
			}

			const oldModel = task.model ?? '';
			if (editModel !== oldModel) {
				updates.model = editModel;
			}

			if (editNotReady !== !task.ready) {
				updates.not_ready = editNotReady;
			}

			let updated = task;
			if (Object.keys(updates).length > 0) {
				updated = await client.updateTask(task.id, updates);
			}
			open = false;
			onUpdated(updated);
		} catch (err) {
			error = (err as Error).message;
		} finally {
			loading = false;
		}
	}

	function handleClose() {
		open = false;
		error = null;
	}

	function addDependency(taskId: string) {
		editDeps = [...editDeps, taskId];
		editDepSearch = '';
	}

	function removeDependency(taskId: string) {
		editDeps = editDeps.filter((id) => id !== taskId);
	}

	function addCriterion() {
		editCriteria = [...editCriteria, ''];
	}

	function removeCriterion(index: number) {
		editCriteria = editCriteria.filter((_, i) => i !== index);
	}

	function updateCriterion(index: number, value: string) {
		editCriteria = editCriteria.map((c, i) => (i === index ? value : c));
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-[750px] max-h-[85vh] sm:max-h-[90vh] overflow-y-auto">
		<Dialog.Header>
			<Dialog.Title class="flex items-center gap-2">
				<div class="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center">
					<Pencil class="w-4 h-4 text-primary" />
				</div>
				Edit Task
			</Dialog.Title>
			<Dialog.Description>
				Update this pending task. Changes are saved without resetting logs or agent data.
			</Dialog.Description>
		</Dialog.Header>
		<form onsubmit={handleSubmit}>
			<div class="py-4 space-y-7">
				<div class="space-y-3">
					<div>
						<label for="edit-title" class="text-sm font-medium mb-2 flex items-center gap-2">
							<Type class="w-4 h-4 text-muted-foreground" />
							Title
						</label>
						<input
							id="edit-title"
							type="text"
							bind:value={editTitle}
							maxlength={150}
							class="w-full border rounded-lg p-3 bg-background text-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-shadow"
							placeholder="Task title"
							disabled={loading}
						/>
						<div class="flex items-center justify-between mt-1">
							<label
								for="edit-not-ready"
								class="flex items-start gap-2 cursor-pointer"
							>
								<input
									id="edit-not-ready"
									type="checkbox"
									bind:checked={editNotReady}
									class="w-3.5 h-3.5 rounded border-input accent-primary mt-0.5"
									disabled={loading}
								/>
								<div>
									<span class="text-sm">Mark as not ready</span>
									<span class="block text-xs text-muted-foreground">Agent will not receive task until marked as ready</span>
								</div>
							</label>
							<p class="text-xs text-muted-foreground text-right self-start">{editTitle.length}/150</p>
						</div>
					</div>

					<div>
						<label for="edit-description" class="text-sm font-medium mb-2 flex items-center gap-2">
							<FileText class="w-4 h-4 text-muted-foreground" />
							Description
							<span class="text-xs text-muted-foreground font-normal">(optional)</span>
						</label>
						<textarea
							id="edit-description"
							bind:value={editDescription}
							class="w-full border rounded-lg p-3 min-h-[120px] sm:min-h-[240px] bg-background text-foreground resize-y focus:outline-none focus:ring-2 focus:ring-ring transition-shadow"
							placeholder="Describe what the agent should do..."
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
					{#if editCriteria.length > 0}
						<div class="space-y-2 mb-2">
							{#each editCriteria as criterion, i}
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
					<label for="edit-dep-search" class="text-sm font-medium mb-2 flex items-center gap-2">
						<Link2 class="w-4 h-4 text-muted-foreground" />
						Dependencies
						<span class="text-xs text-muted-foreground font-normal">(optional)</span>
					</label>

					{#if editDeps.length > 0}
						<div class="flex flex-wrap gap-1.5 mb-3 max-h-20 overflow-y-auto">
							{#each editDeps as depId}
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
							id="edit-dep-search"
							type="text"
							bind:value={editDepSearch}
							class="w-full border rounded-lg pl-9 pr-3 py-2 bg-background text-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-shadow"
							placeholder="Search tasks to add as dependency..."
							disabled={loading}
							autocomplete="off"
						/>
					</div>

					<div class="mt-2 border rounded-lg max-h-36 overflow-y-auto bg-muted/20">
						{#if editAvailableTasks.length > 0}
							{#each editAvailableTasks as t (t.id)}
								<button
									type="button"
									class="w-full text-left px-3 py-2.5 hover:bg-accent cursor-pointer border-b last:border-b-0 transition-colors overflow-hidden"
									onclick={() => addDependency(t.id)}
								>
									<div class="flex items-center gap-2">
										<span class="font-mono text-xs text-muted-foreground bg-background px-1.5 py-0.5 rounded shrink-0">
											#{t.number}
										</span>
										<span class="text-sm truncate">{t.title || t.description}</span>
									</div>
								</button>
							{/each}
						{:else if editDepSearch}
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
						onclick={() => (editShowAdvanced = !editShowAdvanced)}
					>
						{#if editShowAdvanced}
							<ChevronDown class="w-4 h-4" />
						{:else}
							<ChevronRight class="w-4 h-4" />
						{/if}
						Advanced Options
					</button>

					{#if editShowAdvanced}
						<div class="mt-3 space-y-4 pl-1">
							<div>
								<label for="edit-model-select" class="text-sm font-medium mb-2 flex items-center gap-2">
									<Cpu class="w-4 h-4 text-muted-foreground" />
									Model
								</label>
								<select
									id="edit-model-select"
									bind:value={editModel}
									class="w-full border rounded-lg p-2 bg-background text-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-shadow text-sm"
									disabled={loading}
								>
									{#each modelOptions as option}
										<option value={option.value}>{option.label}</option>
									{/each}
								</select>
							</div>
							<div>
								<label for="edit-max-cost" class="text-sm font-medium mb-2 flex items-center gap-2">
									<DollarSign class="w-4 h-4 text-muted-foreground" />
									Max Cost (USD)
									<span class="text-xs text-muted-foreground font-normal">(optional)</span>
								</label>
								<input
									id="edit-max-cost"
									type="number"
									step="0.01"
									min="0"
									bind:value={editMaxCostUsd}
									class="w-full border rounded-lg p-2 bg-background text-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-shadow text-sm"
									placeholder="e.g., 5.00"
									disabled={loading}
								/>
							</div>
							<label
								for="edit-skip-pr"
								class="flex items-center gap-3 p-3 rounded-lg border cursor-pointer hover:bg-accent/50 transition-colors {editDraftPr ? 'opacity-50' : ''}"
							>
								<input
									id="edit-skip-pr"
									type="checkbox"
									bind:checked={editSkipPr}
									onchange={() => { if (editSkipPr) editDraftPr = false; }}
									class="w-4 h-4 rounded border-input accent-primary"
									disabled={loading || editDraftPr}
								/>
								<div class="flex-1">
									<div class="text-sm font-medium flex items-center gap-1.5">
										<GitBranch class="w-3.5 h-3.5 text-muted-foreground" />
										Push to branch only
									</div>
									<p class="text-xs text-muted-foreground mt-0.5">
										Skip PR creation. You can create the PR manually.
									</p>
								</div>
							</label>
							<label
								for="edit-draft-pr"
								class="flex items-center gap-3 p-3 rounded-lg border cursor-pointer hover:bg-accent/50 transition-colors {editSkipPr ? 'opacity-50' : ''}"
							>
								<input
									id="edit-draft-pr"
									type="checkbox"
									bind:checked={editDraftPr}
									onchange={() => { if (editDraftPr) editSkipPr = false; }}
									class="w-4 h-4 rounded border-input accent-primary"
									disabled={loading || editSkipPr}
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
				<Button type="button" variant="outline" onclick={handleClose} disabled={loading}>
					Cancel
				</Button>
				<Button type="submit" disabled={loading || !editTitle.trim()} class="gap-2">
					{#if loading}
						<Loader2 class="w-4 h-4 animate-spin" />
						Saving...
					{:else}
						<Check class="w-4 h-4" />
						Save Changes
					{/if}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
