<script lang="ts">
	import { goto } from '$app/navigation';
	import { client } from '$lib/api-client';
	import { repoStore } from '$lib/stores/repos.svelte';
	import { epicStore } from '$lib/stores/epics.svelte';
	import EpicCard from '$lib/components/EpicCard.svelte';
	import CreateEpicDialog from '$lib/components/CreateEpicDialog.svelte';
	import RepoSetupBanner from '$lib/components/RepoSetupBanner.svelte';
	import { Button } from '$lib/components/ui/button';
	import {
		Plus,
		Layers,
		GitBranch,
		AlertCircle
	} from 'lucide-svelte';

	let openCreateEpic = $state(false);

	// Load epics when selected repo changes.
	$effect(() => {
		const repoId = repoStore.selectedRepoId;
		if (repoId) {
			loadEpics(repoId);
		} else {
			epicStore.clear();
		}
	});

	async function loadEpics(repoId: string) {
		epicStore.loading = true;
		try {
			const epics = await client.listEpicsByRepo(repoId);
			epicStore.setEpics(epics);
		} catch {
			epicStore.error = 'Failed to load epics';
		} finally {
			epicStore.loading = false;
		}
	}

	const hasRepo = $derived(!!repoStore.selectedRepoId);
	const repoReady = $derived(repoStore.selectedRepo?.setup_status === 'ready');
	const totalEpics = $derived(epicStore.epics.length);

	const activeEpics = $derived(
		epicStore.epics.filter((e) => !['completed', 'closed'].includes(e.status))
	);
	const completedEpics = $derived(
		epicStore.epics.filter((e) => ['completed', 'closed'].includes(e.status))
	);
</script>

<div class="p-4 sm:p-6 flex-1 min-h-0 flex flex-col">
	{#if hasRepo}
		<header class="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3 mb-4 sm:mb-6">
			<div>
				<div class="flex items-center gap-3">
					<h1 class="text-xl sm:text-2xl font-bold">Epics</h1>
					{#if repoReady && totalEpics > 0}
						<span
							class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-violet-500/15 text-violet-400"
						>
							{totalEpics} total
						</span>
					{/if}
				</div>
				<p class="text-muted-foreground text-sm mt-1 hidden sm:block">
					{repoReady ? 'Plan and manage multi-task epics with AI-powered task breakdown' : 'Complete repository setup to start creating epics'}
				</p>
			</div>
			{#if repoReady}
				<div class="flex items-center gap-2 sm:gap-3">
					<Button onclick={() => (openCreateEpic = true)} class="gap-2 bg-violet-600 hover:bg-violet-700 text-white">
						<Plus class="w-4 h-4" />
						New Epic
					</Button>
				</div>
			{/if}
		</header>

		{#if !repoReady}
			<RepoSetupBanner />
		{:else}
			{#if epicStore.error}
				<div
					class="bg-destructive/10 text-destructive p-4 rounded-lg mb-4 flex items-center gap-3 border border-destructive/20"
				>
					<AlertCircle class="w-5 h-5 flex-shrink-0" />
					<span>{epicStore.error}</span>
				</div>
			{/if}

			{#if epicStore.epics.length === 0}
				<div class="flex-1 flex flex-col items-center justify-center text-center">
					<div class="w-16 h-16 rounded-2xl bg-violet-500/10 flex items-center justify-center mb-4">
						<Layers class="w-8 h-8 text-violet-400" />
					</div>
					<h2 class="text-xl font-semibold mb-2">No epics yet</h2>
					<p class="text-muted-foreground text-sm max-w-md mb-4">
						Create an epic to break down large features into AI-planned tasks.
					</p>
					<Button onclick={() => (openCreateEpic = true)} class="gap-2 bg-violet-600 hover:bg-violet-700 text-white">
						<Plus class="w-4 h-4" />
						Create Your First Epic
					</Button>
				</div>
			{:else}
				{#if activeEpics.length > 0}
					<div class="mb-6">
						<h2 class="text-sm font-semibold text-muted-foreground mb-3">Active</h2>
						<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">
							{#each activeEpics as epic (epic.id)}
								<EpicCard {epic} />
							{/each}
						</div>
					</div>
				{/if}

				{#if completedEpics.length > 0}
					<div>
						<h2 class="text-sm font-semibold text-muted-foreground mb-3">Completed</h2>
						<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">
							{#each completedEpics as epic (epic.id)}
								<EpicCard {epic} />
							{/each}
						</div>
					</div>
				{/if}
			{/if}
		{/if}
	{:else}
		<div class="flex-1 flex flex-col items-center justify-center text-center">
			<div class="w-16 h-16 rounded-2xl bg-muted flex items-center justify-center mb-4">
				<GitBranch class="w-8 h-8 text-muted-foreground" />
			</div>
			<h2 class="text-xl font-semibold mb-2">No repository selected</h2>
			<p class="text-muted-foreground text-sm max-w-md">
				Add a GitHub repository to get started. Epics are scoped to individual repositories.
			</p>
		</div>
	{/if}
</div>

{#if hasRepo && repoReady}
	<CreateEpicDialog bind:open={openCreateEpic} onCreated={(id) => goto(`/epics/${id}`)} />
{/if}
