<script lang="ts">
	import type { Repo } from '$lib/models/repo';
	import { Badge } from '$lib/components/ui/badge';
	import {
		FileText,
		FileCode,
		BookOpen,
		Check,
		X
	} from 'lucide-svelte';

	let { repo }: { repo: Repo } = $props();
</script>

<div class="space-y-4">
	{#if repo.summary}
		<div>
			<h3 class="text-sm font-medium mb-2">Summary</h3>
			<p class="text-sm text-muted-foreground whitespace-pre-wrap">{repo.summary}</p>
		</div>
	{/if}

	{#if repo.tech_stack && repo.tech_stack.length > 0}
		<div>
			<h3 class="text-sm font-medium mb-2">Tech Stack</h3>
			<div class="flex flex-wrap gap-1.5">
				{#each repo.tech_stack as tech}
					<Badge variant="secondary" class="text-xs">{tech}</Badge>
				{/each}
			</div>
		</div>
	{/if}

	<div>
		<h3 class="text-sm font-medium mb-2">Detected Files</h3>
		<div class="flex flex-wrap gap-3">
			<span class="inline-flex items-center gap-1.5 text-xs {repo.has_code ? 'text-green-500' : 'text-muted-foreground'}">
				{#if repo.has_code}
					<Check class="w-3.5 h-3.5" />
				{:else}
					<X class="w-3.5 h-3.5" />
				{/if}
				<FileCode class="w-3.5 h-3.5" />
				Source Code
			</span>
			<span class="inline-flex items-center gap-1.5 text-xs {repo.has_claude_md ? 'text-green-500' : 'text-muted-foreground'}">
				{#if repo.has_claude_md}
					<Check class="w-3.5 h-3.5" />
				{:else}
					<X class="w-3.5 h-3.5" />
				{/if}
				<FileText class="w-3.5 h-3.5" />
				CLAUDE.md
			</span>
			<span class="inline-flex items-center gap-1.5 text-xs {repo.has_readme ? 'text-green-500' : 'text-muted-foreground'}">
				{#if repo.has_readme}
					<Check class="w-3.5 h-3.5" />
				{:else}
					<X class="w-3.5 h-3.5" />
				{/if}
				<BookOpen class="w-3.5 h-3.5" />
				README
			</span>
		</div>
	</div>
</div>
