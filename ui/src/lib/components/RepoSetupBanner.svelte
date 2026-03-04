<script lang="ts">
	import { onDestroy } from 'svelte';
	import { client } from '$lib/api-client';
	import { repoStore } from '$lib/stores/repos.svelte';
	import type { Repo } from '$lib/models/repo';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import RepoSummary from './RepoSummary.svelte';
	import RepoSetupWizard from './RepoSetupWizard.svelte';
	import {
		Loader2,
		Settings,
		ChevronDown,
		ChevronRight,
		ScanSearch,
		SkipForward
	} from 'lucide-svelte';

	let wizardOpen = $state(false);
	let showDetails = $state(false);
	let startingScan = $state(false);
	let skippingSetup = $state(false);

	// Polling state
	let pollTimer: ReturnType<typeof setInterval> | null = null;

	const repo = $derived(repoStore.selectedRepo);
	const setupStatus = $derived(repo?.setup_status ?? 'ready');
	const isPending = $derived(setupStatus === 'pending');
	const isScanning = $derived(setupStatus === 'scanning');
	const needsSetup = $derived(setupStatus === 'needs_setup');
	const showBanner = $derived(setupStatus !== 'ready' && repo != null);

	// Start/stop polling based on scanning state
	$effect(() => {
		if (isScanning && repo) {
			startPolling(repo.id);
		} else {
			stopPolling();
		}
	});

	onDestroy(() => {
		stopPolling();
	});

	function startPolling(repoId: string) {
		if (pollTimer) return;
		pollTimer = setInterval(async () => {
			try {
				const updated = await client.getRepoSetup(repoId);
				repoStore.updateRepo(updated);
				if (updated.setup_status !== 'scanning') {
					stopPolling();
				}
			} catch {
				// Ignore polling errors
			}
		}, 4000);
	}

	function stopPolling() {
		if (pollTimer) {
			clearInterval(pollTimer);
			pollTimer = null;
		}
	}

	function handleSetupComplete(updated: Repo) {
		repoStore.updateRepo(updated);
	}

	async function handleStartScan() {
		if (!repo) return;
		startingScan = true;
		try {
			const updated = await client.rescanRepo(repo.id);
			repoStore.updateRepo(updated);
		} catch {
			// Ignore errors
		} finally {
			startingScan = false;
		}
	}

	async function handleSkipSetup() {
		if (!repo) return;
		skippingSetup = true;
		try {
			const updated = await client.skipRepoSetup(repo.id);
			repoStore.updateRepo(updated);
		} catch {
			// Ignore errors
		} finally {
			skippingSetup = false;
		}
	}

</script>

{#if showBanner && repo}
	<Card.Root class="mb-4 {isScanning ? 'border-violet-500/20 bg-violet-500/5' : isPending ? 'border-blue-500/20 bg-blue-500/5' : 'border-amber-500/20 bg-amber-500/5'}">
		<Card.Content class="p-4">
			<div class="flex items-start gap-3">
				{#if isPending}
					<ScanSearch class="w-5 h-5 text-blue-400 shrink-0 mt-0.5" />
					<div class="flex-1 min-w-0">
						<p class="text-sm font-medium text-blue-400">Repository setup required</p>
						<p class="text-xs text-muted-foreground mt-0.5">
							This repository needs to be set up before tasks or epics can be created. Run a scan to analyze the repository, or skip to start immediately.
						</p>
						<div class="mt-3 flex items-center gap-2">
							<Button size="sm" onclick={handleStartScan} disabled={startingScan || skippingSetup} class="gap-1.5">
								{#if startingScan}
									<Loader2 class="w-3.5 h-3.5 animate-spin" />
									Starting...
								{:else}
									<ScanSearch class="w-3.5 h-3.5" />
									Start Scan
								{/if}
							</Button>
							<Button size="sm" variant="outline" onclick={handleSkipSetup} disabled={startingScan || skippingSetup} class="gap-1.5">
								{#if skippingSetup}
									<Loader2 class="w-3.5 h-3.5 animate-spin" />
									Skipping...
								{:else}
									<SkipForward class="w-3.5 h-3.5" />
									Skip Setup
								{/if}
							</Button>
						</div>
					</div>
				{:else if isScanning}
					<Loader2 class="w-5 h-5 animate-spin text-violet-400 shrink-0 mt-0.5" />
					<div class="flex-1 min-w-0">
						<p class="text-sm font-medium text-violet-400">Scanning repository...</p>
						<p class="text-xs text-muted-foreground mt-0.5">
							An agent is analyzing your repository structure, tech stack, and configuration files.
						</p>
					</div>
				{:else if needsSetup}
					<Settings class="w-5 h-5 text-amber-400 shrink-0 mt-0.5" />
					<div class="flex-1 min-w-0">
						<p class="text-sm font-medium text-amber-400">Repository needs configuration</p>
						<p class="text-xs text-muted-foreground mt-0.5">
							The scan is complete. Configure expectations to guide the AI agent when working on tasks.
						</p>

						<!-- Inline repo summary for needs_setup -->
						<div class="mt-3">
							<button
								type="button"
								class="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
								onclick={() => (showDetails = !showDetails)}
							>
								{#if showDetails}
									<ChevronDown class="w-3.5 h-3.5" />
								{:else}
									<ChevronRight class="w-3.5 h-3.5" />
								{/if}
								Scan Results
							</button>
							{#if showDetails}
								<div class="mt-2 p-3 bg-background/50 rounded-lg border">
									<RepoSummary {repo} />
								</div>
							{/if}
						</div>

						<div class="mt-3 flex items-center gap-2">
							<Button size="sm" onclick={() => (wizardOpen = true)} class="gap-1.5">
								<Settings class="w-3.5 h-3.5" />
								Configure
							</Button>
						</div>
					</div>
				{/if}
			</div>
		</Card.Content>
	</Card.Root>
{/if}

{#if repo && needsSetup}
	<RepoSetupWizard bind:open={wizardOpen} {repo} onComplete={handleSetupComplete} />
{/if}
