<script lang="ts">
	import type { Epic } from '$lib/models/epic';
	import type { Repo } from '$lib/models/repo';
	import * as Card from '$lib/components/ui/card';
	import { goto } from '$app/navigation';
	import { epicUrl } from '$lib/utils';
	import { Layers, ChevronRight, CheckCircle2, ListTodo } from 'lucide-svelte';

	let { epic, repo }: { epic: Epic; repo: Repo } = $props();

	function handleClick() {
		goto(epicUrl(repo.owner, repo.name, epic.number));
	}

	function getStatusColor(status: string) {
		switch (status) {
			case 'draft':
				return 'bg-gray-500/15 text-gray-400 border-gray-500/20';
			case 'planning':
				return 'bg-violet-500/15 text-violet-400 border-violet-500/20';
			case 'ready':
				return 'bg-amber-500/15 text-amber-400 border-amber-500/20';
			case 'active':
				return 'bg-blue-500/15 text-blue-400 border-blue-500/20';
			case 'completed':
				return 'bg-green-500/15 text-green-400 border-green-500/20';
			case 'closed':
				return 'bg-red-500/15 text-red-400 border-red-500/20';
			default:
				return 'bg-gray-500/15 text-gray-400 border-gray-500/20';
		}
	}
</script>

<Card.Root
	class="group p-3 cursor-pointer bg-[oklch(0.18_0.005_285.823)] shadow-sm hover:bg-accent/50 hover:border-violet-500/30 transition-all duration-200 hover:shadow-md border-violet-500/10"
	onclick={handleClick}
	role="button"
	tabindex={0}
>
	<div class="flex items-start justify-between gap-2">
		<div class="flex items-center gap-2 min-w-0">
			<Layers class="w-4 h-4 text-violet-400 shrink-0" />
			<p class="font-medium text-sm line-clamp-1 flex-1">{epic.title}</p>
		</div>
		<span class="inline-flex items-center text-[11px] font-semibold px-2 py-0.5 rounded-full border shrink-0 {getStatusColor(epic.status)}">
			{epic.status}
		</span>
	</div>
	<div class="flex items-center gap-3 mt-2">
		<span class="text-[10px] text-muted-foreground font-mono bg-muted px-1.5 py-0.5 rounded">
			#{epic.number}
		</span>
		{#if epic.proposed_tasks.length > 0}
			<span class="text-[10px] text-muted-foreground flex items-center gap-0.5">
				<ListTodo class="w-3 h-3" />
				{epic.proposed_tasks.length} proposed
			</span>
		{/if}
		{#if epic.task_ids.length > 0}
			<span class="text-[10px] text-muted-foreground flex items-center gap-0.5">
				<CheckCircle2 class="w-3 h-3" />
				{epic.task_ids.length} tasks
			</span>
		{/if}
	</div>
</Card.Root>
