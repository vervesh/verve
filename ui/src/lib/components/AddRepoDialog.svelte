<script lang="ts">
	import { client } from '$lib/api-client';
	import { repoStore } from '$lib/stores/repos.svelte';
	import type { GitHubRepo } from '$lib/models/repo';
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { GitBranch, Search, Loader2, Lock, Globe, X } from 'lucide-svelte';

	let { open = $bindable(false) }: { open: boolean } = $props();

	let availableRepos = $state<GitHubRepo[]>([]);
	let loadingRepos = $state(false);
	let adding = $state<string | null>(null);
	let error = $state<string | null>(null);
	let searchQuery = $state('');

	// Track already-added repos by full name
	const addedNames = $derived(new Set(repoStore.repos.map((r) => r.full_name)));

	const filteredRepos = $derived(
		availableRepos.filter(
			(r) =>
				!addedNames.has(r.full_name) &&
				(searchQuery === '' || r.full_name.toLowerCase().includes(searchQuery.toLowerCase()))
		)
	);

	// Load repos when dialog opens (via prop change or user interaction).
	$effect(() => {
		if (open) {
			loadAvailableRepos();
		} else {
			searchQuery = '';
			error = null;
		}
	});

	async function loadAvailableRepos() {
		if (availableRepos.length > 0) return;
		loadingRepos = true;
		error = null;
		try {
			availableRepos = await client.listAvailableRepos();
		} catch (e) {
			error = (e as Error).message;
		} finally {
			loadingRepos = false;
		}
	}

	async function addRepo(fullName: string) {
		adding = fullName;
		error = null;
		try {
			const repo = await client.addRepo(fullName);
			repoStore.addRepo(repo);
			open = false;
		} catch (e) {
			error = (e as Error).message;
		} finally {
			adding = null;
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-[625px]">
		<Dialog.Header>
			<Dialog.Title class="flex items-center gap-2">
				<div class="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center">
					<GitBranch class="w-4 h-4 text-primary" />
				</div>
				Add Repository
			</Dialog.Title>
			<Dialog.Description>
				Select a GitHub repository to add to Verve. Tasks will be scoped to individual repos.
			</Dialog.Description>
		</Dialog.Header>

		<div class="py-4 space-y-4">
			<div class="relative">
				<Search class="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
				<input
					type="text"
					bind:value={searchQuery}
					class="w-full border rounded-lg pl-9 pr-3 py-2 bg-background text-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-shadow"
					placeholder="Search repositories..."
					autocomplete="off"
				/>
			</div>

			{#if error}
				<div
					class="bg-destructive/10 text-destructive text-sm p-3 rounded-lg flex items-center gap-2"
				>
					<X class="w-4 h-4 flex-shrink-0" />
					{error}
				</div>
			{/if}

			<div class="border rounded-lg max-h-72 overflow-y-auto bg-muted/20">
				{#if loadingRepos}
					<div class="flex items-center justify-center py-8 gap-2 text-muted-foreground">
						<Loader2 class="w-4 h-4 animate-spin" />
						<span class="text-sm">Loading repositories...</span>
					</div>
				{:else if filteredRepos.length > 0}
					{#each filteredRepos as repo (repo.full_name)}
						<button
							class="w-full text-left px-3 py-2.5 hover:bg-accent cursor-pointer border-b last:border-b-0 transition-colors flex items-center justify-between gap-2"
							onclick={() => addRepo(repo.full_name)}
							disabled={adding === repo.full_name}
						>
							<div class="flex-1 min-w-0">
								<div class="flex items-center gap-2">
									{#if repo.private}
										<Lock class="w-3 h-3 text-muted-foreground flex-shrink-0" />
									{:else}
										<Globe class="w-3 h-3 text-muted-foreground flex-shrink-0" />
									{/if}
									<span class="font-medium text-sm break-all">{repo.full_name}</span>
								</div>
								{#if repo.description}
									<p class="text-xs text-muted-foreground mt-0.5 line-clamp-2 pl-5">
										{repo.description}
									</p>
								{/if}
							</div>
							{#if adding === repo.full_name}
								<Loader2 class="w-4 h-4 animate-spin text-muted-foreground flex-shrink-0" />
							{/if}
						</button>
					{/each}
				{:else if searchQuery}
					<div class="p-6 text-sm text-muted-foreground text-center">
						<Search class="w-5 h-5 mx-auto mb-2 opacity-40" />
						No matching repositories found
					</div>
				{:else}
					<div class="p-6 text-sm text-muted-foreground text-center">
						<GitBranch class="w-5 h-5 mx-auto mb-2 opacity-40" />
						All available repositories have been added
					</div>
				{/if}
			</div>
		</div>

		<Dialog.Footer>
			<Button variant="outline" onclick={() => (open = false)}>Done</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
