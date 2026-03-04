<script lang="ts">
	import { page } from '$app/stores';
	import { repoStore } from '$lib/stores/repos.svelte';
	import { ListTodo, Layers, Activity, BookOpen } from 'lucide-svelte';
	import RepoSettingsDialog from './RepoSettingsDialog.svelte';

	const currentPath = $derived($page.url.pathname);
	const isTasksActive = $derived(currentPath === '/' || currentPath.startsWith('/tasks'));
	const isEpicsActive = $derived(currentPath.startsWith('/epics'));
	const isMetricsActive = $derived(currentPath.startsWith('/agents'));

	let repoSettingsOpen = $state(false);
	const hasRepo = $derived(!!repoStore.selectedRepoId);
</script>

<nav class="w-12 sm:w-44 border-r bg-card/50 flex flex-col shrink-0">
	<div class="flex flex-col gap-1 p-2">
		<a
			href="/"
			class="flex items-center gap-2.5 px-2.5 py-2 rounded-lg text-sm font-medium transition-colors
				{isTasksActive
				? 'bg-primary/10 text-primary'
				: 'text-muted-foreground hover:text-foreground hover:bg-accent'}"
		>
			<ListTodo class="w-4 h-4 shrink-0" />
			<span class="hidden sm:inline">Tasks</span>
		</a>
		<a
			href="/epics"
			class="flex items-center gap-2.5 px-2.5 py-2 rounded-lg text-sm font-medium transition-colors
				{isEpicsActive
				? 'bg-primary/10 text-primary'
				: 'text-muted-foreground hover:text-foreground hover:bg-accent'}"
		>
			<Layers class="w-4 h-4 shrink-0" />
			<span class="hidden sm:inline">Epics</span>
		</a>
		<a
			href="/agents"
			class="flex items-center gap-2.5 px-2.5 py-2 rounded-lg text-sm font-medium transition-colors
				{isMetricsActive
				? 'bg-primary/10 text-primary'
				: 'text-muted-foreground hover:text-foreground hover:bg-accent'}"
		>
			<Activity class="w-4 h-4 shrink-0" />
			<span class="hidden sm:inline">Metrics</span>
		</a>
		{#if hasRepo}
			<button
				type="button"
				onclick={() => (repoSettingsOpen = true)}
				class="flex items-center gap-2.5 px-2.5 py-2 rounded-lg text-sm font-medium transition-colors text-muted-foreground hover:text-foreground hover:bg-accent text-left"
			>
				<BookOpen class="w-4 h-4 shrink-0" />
				<span class="hidden sm:inline">Repo Settings</span>
			</button>
		{/if}
	</div>
</nav>

<RepoSettingsDialog bind:open={repoSettingsOpen} />
