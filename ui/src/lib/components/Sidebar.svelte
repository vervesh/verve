<script lang="ts">
	import { page } from '$app/stores';
	import { repoStore } from '$lib/stores/repos.svelte';
	import { ListTodo, Layers, Activity, BookOpen, MessageSquare } from 'lucide-svelte';
	import RepoSettingsDialog from './RepoSettingsDialog.svelte';

	const currentPath = $derived($page.url.pathname);
	const isTasksActive = $derived(currentPath === '/' || currentPath.startsWith('/tasks'));
	const isEpicsActive = $derived(currentPath.startsWith('/epics'));
	const isConversationsActive = $derived(currentPath.startsWith('/conversations'));
	const isMetricsActive = $derived(currentPath.startsWith('/agents'));

	let repoSettingsOpen = $state(false);
	const hasRepo = $derived(!!repoStore.selectedRepoId);
</script>

<!-- Desktop sidebar (hidden on mobile) -->
<nav class="hidden sm:flex w-44 border-r bg-card/50 flex-col shrink-0">
	<div class="flex flex-col gap-1 p-2">
		<a
			href="/"
			class="flex items-center gap-2.5 px-2.5 py-2 rounded-lg text-sm font-medium transition-colors
				{isTasksActive
				? 'bg-primary/10 text-primary'
				: 'text-muted-foreground hover:text-foreground hover:bg-accent'}"
		>
			<ListTodo class="w-4 h-4 shrink-0" />
			<span>Tasks</span>
		</a>
		<a
			href="/epics"
			class="flex items-center gap-2.5 px-2.5 py-2 rounded-lg text-sm font-medium transition-colors
				{isEpicsActive
				? 'bg-primary/10 text-primary'
				: 'text-muted-foreground hover:text-foreground hover:bg-accent'}"
		>
			<Layers class="w-4 h-4 shrink-0" />
			<span>Epics</span>
		</a>
		<a
			href="/conversations"
			class="flex items-center gap-2.5 px-2.5 py-2 rounded-lg text-sm font-medium transition-colors
				{isConversationsActive
				? 'bg-primary/10 text-primary'
				: 'text-muted-foreground hover:text-foreground hover:bg-accent'}"
		>
			<MessageSquare class="w-4 h-4 shrink-0" />
			<span>Conversations</span>
		</a>
		<a
			href="/agents"
			class="flex items-center gap-2.5 px-2.5 py-2 rounded-lg text-sm font-medium transition-colors
				{isMetricsActive
				? 'bg-primary/10 text-primary'
				: 'text-muted-foreground hover:text-foreground hover:bg-accent'}"
		>
			<Activity class="w-4 h-4 shrink-0" />
			<span>Metrics</span>
		</a>
		{#if hasRepo}
			<button
				type="button"
				onclick={() => (repoSettingsOpen = true)}
				class="flex items-center gap-2.5 px-2.5 py-2 rounded-lg text-sm font-medium transition-colors text-muted-foreground hover:text-foreground hover:bg-accent text-left"
			>
				<BookOpen class="w-4 h-4 shrink-0" />
				<span>Repo Settings</span>
			</button>
		{/if}
	</div>
</nav>

<!-- Mobile bottom bar (hidden on desktop) -->
<nav class="sm:hidden fixed bottom-0 left-0 right-0 z-50 border-t bg-card/95 backdrop-blur-sm">
	<div class="flex items-center justify-around px-1 py-1.5">
		<a
			href="/"
			class="flex flex-col items-center gap-0.5 px-2 py-1.5 rounded-lg text-[10px] font-medium transition-colors min-w-0 flex-1
				{isTasksActive
				? 'text-primary'
				: 'text-muted-foreground'}"
		>
			<ListTodo class="w-5 h-5" />
			<span>Tasks</span>
		</a>
		<a
			href="/epics"
			class="flex flex-col items-center gap-0.5 px-2 py-1.5 rounded-lg text-[10px] font-medium transition-colors min-w-0 flex-1
				{isEpicsActive
				? 'text-primary'
				: 'text-muted-foreground'}"
		>
			<Layers class="w-5 h-5" />
			<span>Epics</span>
		</a>
		<a
			href="/conversations"
			class="flex flex-col items-center gap-0.5 px-2 py-1.5 rounded-lg text-[10px] font-medium transition-colors min-w-0 flex-1
				{isConversationsActive
				? 'text-primary'
				: 'text-muted-foreground'}"
		>
			<MessageSquare class="w-5 h-5" />
			<span>Chats</span>
		</a>
		<a
			href="/agents"
			class="flex flex-col items-center gap-0.5 px-2 py-1.5 rounded-lg text-[10px] font-medium transition-colors min-w-0 flex-1
				{isMetricsActive
				? 'text-primary'
				: 'text-muted-foreground'}"
		>
			<Activity class="w-5 h-5" />
			<span>Metrics</span>
		</a>
		{#if hasRepo}
			<button
				type="button"
				onclick={() => (repoSettingsOpen = true)}
				class="flex flex-col items-center gap-0.5 px-2 py-1.5 rounded-lg text-[10px] font-medium transition-colors text-muted-foreground min-w-0 flex-1"
			>
				<BookOpen class="w-5 h-5" />
				<span>Repo</span>
			</button>
		{/if}
	</div>
</nav>

<RepoSettingsDialog bind:open={repoSettingsOpen} />
