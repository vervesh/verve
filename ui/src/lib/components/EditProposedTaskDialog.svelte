<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import type { ProposedTask } from '$lib/models/epic';
	import {
		FileText,
		Link2,
		X,
		Loader2,
		Target,
		Plus,
		Type,
		Pencil,
		Check
	} from 'lucide-svelte';

	let {
		open = $bindable(false),
		task,
		taskIndex,
		allTasks = [],
		onSave
	}: {
		open: boolean;
		task: ProposedTask | null;
		taskIndex: number;
		allTasks: ProposedTask[];
		onSave: (updated: ProposedTask) => void;
	} = $props();

	let editTitle = $state('');
	let editDescription = $state('');
	let editCriteria = $state<string[]>([]);
	let editDeps = $state<string[]>([]);
	let loading = $state(false);
	let error = $state<string | null>(null);

	// Populate form when dialog opens
	$effect(() => {
		if (open && task) {
			editTitle = task.title;
			editDescription = task.description;
			editCriteria = [...(task.acceptance_criteria ?? [])];
			editDeps = [...(task.depends_on_temp_ids ?? [])];
			error = null;
		}
	});

	// Available tasks for dependency selection (exclude current task)
	const availableDeps = $derived(
		allTasks.filter(
			(t) => t.temp_id !== task?.temp_id && !editDeps.includes(t.temp_id)
		)
	);

	function getDependencyLabel(tempId: string): string {
		const t = allTasks.find((pt) => pt.temp_id === tempId);
		return t ? t.title : tempId;
	}

	function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		if (!task || !editTitle.trim()) return;

		const updated: ProposedTask = {
			...task,
			title: editTitle.trim(),
			description: editDescription,
			acceptance_criteria: editCriteria.filter((c) => c.trim() !== ''),
			depends_on_temp_ids: editDeps
		};

		onSave(updated);
		open = false;
	}

	function handleClose() {
		open = false;
		error = null;
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

	function addDependency(tempId: string) {
		editDeps = [...editDeps, tempId];
	}

	function removeDependency(tempId: string) {
		editDeps = editDeps.filter((id) => id !== tempId);
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-[900px] max-h-[85vh] sm:max-h-[90vh] overflow-y-auto">
		<Dialog.Header>
			<Dialog.Title class="flex items-center gap-2">
				<div class="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center">
					<Pencil class="w-4 h-4 text-primary" />
				</div>
				Edit Proposed Task
			</Dialog.Title>
			<Dialog.Description>
				Edit this proposed task before confirming the epic. Changes are saved to the draft plan.
			</Dialog.Description>
		</Dialog.Header>
		<form onsubmit={handleSubmit}>
			<div class="py-4 space-y-7">
				<div class="space-y-3">
					<div>
						<label for="edit-proposed-title" class="text-sm font-medium mb-2 flex items-center gap-2">
							<Type class="w-4 h-4 text-muted-foreground" />
							Title
						</label>
						<input
							id="edit-proposed-title"
							type="text"
							bind:value={editTitle}
							maxlength={150}
							class="w-full border rounded-lg p-3 bg-background text-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-shadow"
							placeholder="Task title"
						/>
						<p class="text-xs text-muted-foreground text-right mt-1">{editTitle.length}/150</p>
					</div>

					<div>
						<label for="edit-proposed-description" class="text-sm font-medium mb-2 flex items-center gap-2">
							<FileText class="w-4 h-4 text-muted-foreground" />
							Description
						</label>
						<textarea
							id="edit-proposed-description"
							bind:value={editDescription}
							class="w-full border rounded-lg p-3 min-h-[120px] sm:min-h-[240px] bg-background text-foreground resize-y focus:outline-none focus:ring-2 focus:ring-ring transition-shadow"
							placeholder="Detailed description of what the agent should do..."
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
						class="gap-1.5 text-xs"
					>
						<Plus class="w-3.5 h-3.5" />
						Add criterion
					</Button>
				</div>

				<hr class="border-border" />

				<!-- Dependencies (other proposed tasks in the same epic) -->
				<div>
					<label class="text-sm font-medium mb-2 flex items-center gap-2">
						<Link2 class="w-4 h-4 text-muted-foreground" />
						Dependencies
						<span class="text-xs text-muted-foreground font-normal">(other tasks in this epic)</span>
					</label>

					{#if editDeps.length > 0}
						<div class="space-y-1.5 mb-3">
							{#each editDeps as depId}
								<div class="flex items-center gap-2 py-1.5 px-3 rounded-lg bg-muted/20 border">
									<Link2 class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
									<span class="text-sm flex-1 min-w-0 truncate">{getDependencyLabel(depId)}</span>
									<button
										type="button"
										class="p-1 hover:bg-destructive/10 hover:text-destructive rounded transition-colors shrink-0"
										onclick={() => removeDependency(depId)}
									>
										<X class="w-3 h-3" />
									</button>
								</div>
							{/each}
						</div>
					{/if}

					{#if availableDeps.length > 0}
						<div class="border rounded-lg max-h-36 overflow-y-auto bg-muted/20">
							{#each availableDeps as dep (dep.temp_id)}
								<button
									type="button"
									class="w-full text-left px-3 py-2 hover:bg-accent cursor-pointer border-b last:border-b-0 transition-colors"
									onclick={() => addDependency(dep.temp_id)}
								>
									<p class="text-sm truncate">{dep.title}</p>
								</button>
							{/each}
						</div>
					{:else if editDeps.length === allTasks.length - 1}
						<p class="text-xs text-muted-foreground">All other tasks are already dependencies.</p>
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
					<Button type="button" variant="outline" onclick={handleClose}>
						Cancel
					</Button>
					<Button type="submit" disabled={!editTitle.trim()} class="gap-2">
						<Check class="w-4 h-4" />
						Save Changes
					</Button>
				</div>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
