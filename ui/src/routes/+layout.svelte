<script lang="ts">
	import './layout.css';
	import { onMount } from 'svelte';
	import { Settings, DollarSign } from 'lucide-svelte';
	import { client } from '$lib/api-client';
	import { repoStore } from '$lib/stores/repos.svelte';
	import { taskStore } from '$lib/stores/tasks.svelte';
	import { Button } from '$lib/components/ui/button';
	import RepoSelector from '$lib/components/RepoSelector.svelte';
	import VerveLogo from '$lib/components/VerveLogo.svelte';
	import GitHubTokenDialog from '$lib/components/GitHubTokenDialog.svelte';
	import Sidebar from '$lib/components/Sidebar.svelte';

	let { children } = $props();
	let openSettingsDialog = $state(false);
	let tokenConfigured = $state<boolean | null>(null);
	let modelConfigured = $state<boolean | null>(null);

	// All required settings are configured
	const allConfigured = $derived(tokenConfigured === true && modelConfigured === true);
	const settingsRequired = $derived(tokenConfigured === false || modelConfigured === false);

	onMount(async () => {
		try {
			const [tokenStatus, modelStatus] = await Promise.all([
				client.getGitHubTokenStatus(),
				client.getDefaultModel()
			]);
			tokenConfigured = tokenStatus.configured;
			modelConfigured = modelStatus.configured;
			if (!tokenStatus.configured || !modelStatus.configured) {
				openSettingsDialog = true;
			}
		} catch {
			tokenConfigured = false;
			modelConfigured = false;
			openSettingsDialog = true;
		}

		if (tokenConfigured && modelConfigured) {
			await loadRepos();
		}
	});

	async function loadRepos() {
		repoStore.loading = true;
		try {
			const repos = await client.listRepos();
			repoStore.setRepos(repos);
		} catch {
			// Ignore errors on initial load
		} finally {
			repoStore.loading = false;
		}
	}

	function handleConfigured() {
		tokenConfigured = true;
		modelConfigured = true;
		openSettingsDialog = false;
		loadRepos();
	}
</script>

<svelte:head>
	<title>Verve - AI Task Orchestrator</title>
</svelte:head>

<div class="min-h-screen bg-background flex flex-col">
	<header class="border-b bg-card/50 backdrop-blur-sm sticky top-0 z-50">
		<div class="px-4 sm:px-6 h-14 flex items-center justify-between gap-2">
			<div class="flex items-center gap-2 sm:gap-4 min-w-0">
				<a href="/" class="flex items-center gap-2 hover:opacity-80 transition-opacity shrink-0">
					<VerveLogo size={32} />
					<span class="font-bold text-xl tracking-tight hidden sm:inline">Verve</span>
				</a>
				{#if allConfigured}
					<RepoSelector />
				{/if}
				{#if allConfigured && taskStore.totalCost > 0}
					<span
						class="hidden sm:inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-muted text-muted-foreground border border-border"
						title="Total cost for this repository"
					>
						<DollarSign class="w-3 h-3" />
						{taskStore.totalCost.toFixed(2)} spent
					</span>
				{/if}
			</div>
			<div class="flex items-center shrink-0 gap-1">
				<Button
					variant="ghost"
					size="icon"
					onclick={() => (openSettingsDialog = true)}
					title="Settings"
				>
					<Settings class="w-5 h-5 text-muted-foreground" />
				</Button>
			</div>
		</div>
	</header>
	<div class="flex-1 min-h-0 flex">
		{#if allConfigured}
			<Sidebar />
			<main class="flex-1 min-h-0 flex flex-col overflow-auto">
				{@render children()}
			</main>
		{:else if settingsRequired}
			<main class="flex-1 min-h-0 flex flex-col">
				<div class="flex items-center justify-center h-[60vh] text-muted-foreground text-sm">
					Configure your settings to get started.
				</div>
			</main>
		{/if}
	</div>
</div>

<GitHubTokenDialog
	bind:open={openSettingsDialog}
	required={settingsRequired}
	onconfigured={handleConfigured}
/>
