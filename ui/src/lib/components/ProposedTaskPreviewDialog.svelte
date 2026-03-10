<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { renderMarkdown } from '$lib/markdown';
	import type { ProposedTask } from '$lib/models/epic';
	import {
		Eye,
		FileText,
		Target,
		Link2,
		CheckCircle2,
		Edit3,
		X
	} from 'lucide-svelte';

	let {
		open = $bindable(false),
		task,
		taskIndex,
		allTasks = [],
		onEdit
	}: {
		open: boolean;
		task: ProposedTask | null;
		taskIndex: number;
		allTasks: ProposedTask[];
		onEdit?: () => void;
	} = $props();

	function getDependencyLabel(tempId: string): string {
		const t = allTasks.find((pt) => pt.temp_id === tempId);
		return t ? t.title : tempId;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-[900px] max-h-[85vh] sm:max-h-[90vh] overflow-y-auto">
		<Dialog.Header>
			<Dialog.Title class="flex items-center gap-2">
				<div class="w-8 h-8 rounded-lg bg-violet-500/10 flex items-center justify-center">
					<Eye class="w-4 h-4 text-violet-500" />
				</div>
				<span class="flex-1 min-w-0">
					<span class="block truncate">Task {taskIndex + 1}: {task?.title ?? ''}</span>
				</span>
			</Dialog.Title>
			<Dialog.Description>
				Preview of the proposed task from the epic plan.
			</Dialog.Description>
		</Dialog.Header>

		{#if task}
			<div class="py-4 space-y-6">
				<!-- Title -->
				<div>
					<h3 class="text-sm font-medium mb-1.5 flex items-center gap-2 text-muted-foreground">
						Title
					</h3>
					<p class="text-sm font-medium">{task.title}</p>
				</div>

				<!-- Description -->
				{#if task.description}
					<div>
						<h3 class="text-sm font-medium mb-1.5 flex items-center gap-2 text-muted-foreground">
							<FileText class="w-4 h-4" />
							Description
						</h3>
						<div class="border rounded-lg p-4 bg-muted/20 max-h-[40vh] overflow-y-auto overscroll-contain">
							<div class="prose prose-sm dark:prose-invert max-w-none">
								{@html renderMarkdown(task.description)}
							</div>
						</div>
					</div>
				{/if}

				<!-- Acceptance Criteria -->
				{#if task.acceptance_criteria && task.acceptance_criteria.length > 0}
					<div>
						<h3 class="text-sm font-medium mb-2 flex items-center gap-2 text-muted-foreground">
							<Target class="w-4 h-4" />
							Acceptance Criteria
							<span class="px-1.5 py-0.5 rounded-full text-[10px] bg-muted">{task.acceptance_criteria.length}</span>
						</h3>
						<div class="space-y-1.5 max-h-48 overflow-y-auto overscroll-contain">
							{#each task.acceptance_criteria as criterion, i}
								<div class="flex items-start gap-2.5 py-1.5 px-3 rounded-lg bg-muted/20">
									<CheckCircle2 class="w-3.5 h-3.5 text-muted-foreground mt-0.5 shrink-0" />
									<span class="text-sm">{criterion}</span>
								</div>
							{/each}
						</div>
					</div>
				{/if}

				<!-- Dependencies -->
				{#if task.depends_on_temp_ids && task.depends_on_temp_ids.length > 0}
					<div>
						<h3 class="text-sm font-medium mb-2 flex items-center gap-2 text-muted-foreground">
							<Link2 class="w-4 h-4" />
							Dependencies
							<span class="px-1.5 py-0.5 rounded-full text-[10px] bg-muted">{task.depends_on_temp_ids.length}</span>
						</h3>
						<div class="space-y-1.5">
							{#each task.depends_on_temp_ids as depId}
								<div class="flex items-center gap-2 py-1.5 px-3 rounded-lg bg-muted/20">
									<Link2 class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
									<span class="text-sm">{getDependencyLabel(depId)}</span>
								</div>
							{/each}
						</div>
					</div>
				{/if}
			</div>

			<Dialog.Footer>
				<div class="flex justify-end gap-2 w-full">
					<Button type="button" variant="outline" onclick={() => (open = false)}>
						Close
					</Button>
					{#if onEdit}
						<Button
							type="button"
							onclick={() => {
								open = false;
								onEdit?.();
							}}
							class="gap-2"
						>
							<Edit3 class="w-4 h-4" />
							Edit Task
						</Button>
					{/if}
				</div>
			</Dialog.Footer>
		{/if}
	</Dialog.Content>
</Dialog.Root>
