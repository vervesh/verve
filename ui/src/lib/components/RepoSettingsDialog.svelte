<script lang="ts">
	import { client } from '$lib/api-client';
	import { repoStore } from '$lib/stores/repos.svelte';
	import type { Repo } from '$lib/models/repo';
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import RepoSummary from './RepoSummary.svelte';
	import RepoSetupWizard from './RepoSetupWizard.svelte';
	import {
		Settings,
		Loader2,
		Check,
		RefreshCw,
		Pencil,
		X,
		BookOpen
	} from 'lucide-svelte';

	let {
		open = $bindable(false)
	}: { open: boolean } = $props();

	let editingSummary = $state(false);
	let summaryText = $state('');
	let savingSummary = $state(false);
	let rescanning = $state(false);
	let error = $state<string | null>(null);
	let wizardOpen = $state(false);

	const repo = $derived(repoStore.selectedRepo);

	// Sync summary text when dialog opens
	$effect(() => {
		if (open && repo) {
			summaryText = repo.summary || '';
			editingSummary = false;
			error = null;
		}
	});

	async function handleSaveSummary() {
		if (!repo) return;
		savingSummary = true;
		error = null;
		try {
			const updated = await client.updateRepoSummary(repo.id, summaryText);
			repoStore.updateRepo(updated);
			editingSummary = false;
		} catch (err) {
			error = (err as Error).message;
		} finally {
			savingSummary = false;
		}
	}

	async function handleRescan() {
		if (!repo) return;
		rescanning = true;
		error = null;
		try {
			const updated = await client.rescanRepo(repo.id);
			repoStore.updateRepo(updated);
			open = false;
		} catch (err) {
			error = (err as Error).message;
		} finally {
			rescanning = false;
		}
	}

	function handleSetupComplete(updated: Repo) {
		repoStore.updateRepo(updated);
	}

	function statusLabel(status: string): string {
		switch (status) {
			case 'pending':
				return 'Pending';
			case 'scanning':
				return 'Scanning';
			case 'needs_setup':
				return 'Needs Configuration';
			case 'ready':
				return 'Ready';
			default:
				return status;
		}
	}

	function statusColor(status: string): string {
		switch (status) {
			case 'pending':
				return 'bg-blue-500/15 text-blue-400';
			case 'scanning':
				return 'bg-violet-500/15 text-violet-400';
			case 'needs_setup':
				return 'bg-amber-500/15 text-amber-400';
			case 'ready':
				return 'bg-green-500/15 text-green-400';
			default:
				return 'bg-muted text-muted-foreground';
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-[700px] max-h-[90vh] overflow-y-auto">
		<Dialog.Header>
			<Dialog.Title class="flex items-center gap-2">
				<div class="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center">
					<BookOpen class="w-4 h-4 text-primary" />
				</div>
				Repository Settings
			</Dialog.Title>
			<Dialog.Description>
				View and manage repository scan results, summary, and expectations.
			</Dialog.Description>
		</Dialog.Header>

		{#if repo}
			<div class="py-4 space-y-6">
				<!-- Status -->
				<div class="flex items-center gap-3">
					<span class="text-sm font-medium">Setup Status</span>
					<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {statusColor(repo.setup_status)}">
						{statusLabel(repo.setup_status)}
					</span>
				</div>

				<!-- Scan Results -->
				{#if repo.summary || (repo.tech_stack && repo.tech_stack.length > 0)}
					<div class="bg-muted/30 rounded-lg p-4 border">
						{#if editingSummary}
							<div>
								<label for="summary-edit" class="text-sm font-medium mb-2 block">Edit Summary</label>
								<textarea
									id="summary-edit"
									bind:value={summaryText}
									class="w-full border rounded-lg p-3 min-h-[120px] bg-background text-foreground resize-y focus:outline-none focus:ring-2 focus:ring-ring transition-shadow text-sm"
									disabled={savingSummary}
								></textarea>
								<div class="flex items-center gap-2 mt-3">
									<Button size="sm" onclick={handleSaveSummary} disabled={savingSummary} class="gap-1.5">
										{#if savingSummary}
											<Loader2 class="w-3.5 h-3.5 animate-spin" />
											Saving...
										{:else}
											<Check class="w-3.5 h-3.5" />
											Save
										{/if}
									</Button>
									<Button size="sm" variant="outline" onclick={() => { editingSummary = false; summaryText = repo?.summary || ''; }}>
										Cancel
									</Button>
								</div>
							</div>
						{:else}
							<div class="flex items-start justify-between gap-2">
								<RepoSummary {repo} />
								<Button size="sm" variant="ghost" onclick={() => { editingSummary = true; summaryText = repo?.summary || ''; }} class="gap-1.5 shrink-0">
									<Pencil class="w-3.5 h-3.5" />
									Edit
								</Button>
							</div>
						{/if}
					</div>
				{:else}
					<div class="bg-muted/30 rounded-lg p-4 border text-center">
						<p class="text-sm text-muted-foreground">
							No scan results available. Run a scan to analyze the repository.
						</p>
					</div>
				{/if}

				<!-- Expectations -->
				{#if repo.expectations}
					<div>
						<div class="flex items-center justify-between mb-2">
							<h3 class="text-sm font-medium">Expectations</h3>
							<Button size="sm" variant="ghost" onclick={() => (wizardOpen = true)} class="gap-1.5">
								<Pencil class="w-3.5 h-3.5" />
								Edit
							</Button>
						</div>
						<div class="bg-muted/30 rounded-lg p-4 border">
							<p class="text-sm text-muted-foreground whitespace-pre-wrap">{repo.expectations}</p>
						</div>
					</div>
				{:else if repo.setup_status === 'ready'}
					<div>
						<div class="flex items-center justify-between mb-2">
							<h3 class="text-sm font-medium">Expectations</h3>
						</div>
						<div class="bg-muted/30 rounded-lg p-4 border text-center">
							<p class="text-sm text-muted-foreground mb-3">
								No expectations configured. Add expectations to guide the AI agent.
							</p>
							<Button size="sm" variant="outline" onclick={() => (wizardOpen = true)} class="gap-1.5">
								<Settings class="w-3.5 h-3.5" />
								Configure Expectations
							</Button>
						</div>
					</div>
				{/if}

				{#if error}
					<div class="bg-destructive/10 text-destructive text-sm p-3 rounded-lg flex items-center gap-2">
						<X class="w-4 h-4 flex-shrink-0" />
						{error}
					</div>
				{/if}
			</div>

			<Dialog.Footer>
				<div class="flex items-center justify-between w-full gap-2">
					<Button
						type="button"
						variant="outline"
						onclick={handleRescan}
						disabled={rescanning || repo.setup_status === 'scanning'}
						class="gap-1.5"
					>
						<RefreshCw class="w-4 h-4 {rescanning ? 'animate-spin' : ''}" />
						{repo.setup_status === 'scanning' ? 'Scanning...' : 'Rescan Repository'}
					</Button>
					<Button type="button" variant="ghost" onclick={() => (open = false)}>
						Close
					</Button>
				</div>
			</Dialog.Footer>
		{/if}
	</Dialog.Content>
</Dialog.Root>

{#if repo && wizardOpen}
	<RepoSetupWizard bind:open={wizardOpen} {repo} onComplete={handleSetupComplete} />
{/if}
