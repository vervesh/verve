<script lang="ts">
	import { client } from '$lib/api-client';
	import { repoStore } from '$lib/stores/repos.svelte';
	import type { Repo } from '$lib/models/repo';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import * as Dialog from '$lib/components/ui/dialog';
	import RepoSummary from './RepoSummary.svelte';
	import {
		Settings,
		Loader2,
		X,
		Check,
		RefreshCw,
		Code,
		Building2,
		TestTube,
		GitBranch,
		Package,
		FileText,
		ChevronDown,
		ChevronRight,
		SkipForward,
		Plus,
		Layers,
		Sparkles,
		Send
	} from 'lucide-svelte';

	let {
		open = $bindable(false),
		repo,
		onComplete
	}: { open: boolean; repo: Repo; onComplete: (updated: Repo) => void } = $props();

	let expectations = $state('');
	let techStack = $state<string[]>([]);
	let techStackInput = $state('');
	let saving = $state(false);
	let skipping = $state(false);
	let rescanning = $state(false);
	let submitting = $state(false);
	let confirming = $state(false);
	let error = $state<string | null>(null);

	// Section expansion state
	let expandedSections = $state<Set<string>>(new Set(['code_quality']));

	const sections = [
		{
			id: 'code_quality',
			label: 'Code Quality',
			icon: Code,
			placeholder:
				'e.g., Follow ESLint rules, use Prettier for formatting, prefer const over let...',
			tip: 'Define coding standards, linting rules, and formatting preferences.'
		},
		{
			id: 'architecture',
			label: 'Architecture',
			icon: Building2,
			placeholder:
				'e.g., Follow hexagonal architecture, keep business logic in domain layer...',
			tip: 'Describe project structure, design patterns, and conventions to follow.'
		},
		{
			id: 'testing',
			label: 'Testing',
			icon: TestTube,
			placeholder:
				'e.g., Write unit tests for all new functions, use Jest, aim for 80% coverage...',
			tip: 'Specify testing frameworks, coverage requirements, and testing patterns.'
		},
		{
			id: 'git_conventions',
			label: 'Git/PR Conventions',
			icon: GitBranch,
			placeholder:
				'e.g., Use conventional commits, branch naming: feature/*, squash merge PRs...',
			tip: 'Define branch naming, commit message format, and PR description standards.'
		},
		{
			id: 'dependencies',
			label: 'Dependencies',
			icon: Package,
			placeholder:
				'e.g., Prefer stdlib over third-party, pin dependency versions, no leftpad...',
			tip: 'Set policies for package management and dependency decisions.'
		},
		{
			id: 'documentation',
			label: 'Documentation',
			icon: FileText,
			placeholder:
				'e.g., Add JSDoc for public APIs, update README for new features, inline comments for complex logic...',
			tip: 'Describe documentation requirements and comment style preferences.'
		}
	];

	// Initialize state from repo when dialog opens
	$effect(() => {
		if (open) {
			expectations = repo.expectations || '';
			techStack = [...(repo.tech_stack || [])];
			techStackInput = '';
			error = null;
		}
	});

	function toggleSection(id: string) {
		if (expandedSections.has(id)) {
			expandedSections.delete(id);
		} else {
			expandedSections.add(id);
		}
		expandedSections = new Set(expandedSections);
	}

	function insertSectionTemplate(section: typeof sections[number]) {
		const header = `## ${section.label}\n\n`;
		if (expectations.includes(`## ${section.label}`)) return;
		expectations = expectations ? expectations.trimEnd() + '\n\n' + header : header;
	}

	function addTechStackItem() {
		const item = techStackInput.trim();
		if (item && !techStack.some((t) => t.toLowerCase() === item.toLowerCase())) {
			techStack = [...techStack, item];
		}
		techStackInput = '';
	}

	function removeTechStackItem(index: number) {
		techStack = techStack.filter((_, i) => i !== index);
	}

	function handleTechStackKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			addTechStackItem();
		}
	}

	// Check if tech stack was modified
	function techStackChanged(): boolean {
		const original = repo.tech_stack || [];
		if (original.length !== techStack.length) return true;
		return original.some((item, i) => item !== techStack[i]);
	}

	// Submit for AI review — saves user input and triggers AI to flesh it out
	async function handleSubmitForReview() {
		submitting = true;
		error = null;
		try {
			const updates: { expectations?: string; tech_stack?: string[]; summary?: string } = {};
			updates.expectations = expectations;
			if (techStackChanged()) {
				updates.tech_stack = techStack;
			}
			const updated = await client.submitRepoSetup(repo.id, updates);
			repoStore.updateRepo(updated);
			open = false;
			onComplete(updated);
		} catch (err) {
			error = (err as Error).message;
		} finally {
			submitting = false;
		}
	}

	// Confirm the AI-reviewed configuration
	async function handleConfirm() {
		confirming = true;
		error = null;
		try {
			const updated = await client.confirmRepoSetup(repo.id);
			repoStore.updateRepo(updated);
			open = false;
			onComplete(updated);
		} catch (err) {
			error = (err as Error).message;
		} finally {
			confirming = false;
		}
	}

	// Save directly without AI review (mark ready immediately)
	async function handleSave() {
		saving = true;
		error = null;
		try {
			const updates: { expectations: string; mark_ready: boolean; tech_stack?: string[] } = {
				expectations,
				mark_ready: true
			};
			if (techStackChanged()) {
				updates.tech_stack = techStack;
			}
			const updated = await client.updateRepoSetup(repo.id, updates);
			repoStore.updateRepo(updated);
			open = false;
			onComplete(updated);
		} catch (err) {
			error = (err as Error).message;
		} finally {
			saving = false;
		}
	}

	async function handleSkip() {
		skipping = true;
		error = null;
		try {
			const updates: { expectations: string; mark_ready: boolean; tech_stack?: string[] } = {
				expectations: '',
				mark_ready: true
			};
			if (techStackChanged()) {
				updates.tech_stack = techStack;
			}
			const updated = await client.updateRepoSetup(repo.id, updates);
			repoStore.updateRepo(updated);
			open = false;
			onComplete(updated);
		} catch (err) {
			error = (err as Error).message;
		} finally {
			skipping = false;
		}
	}

	async function handleRescan() {
		rescanning = true;
		error = null;
		try {
			const updated = await client.rescanRepo(repo.id);
			repoStore.updateRepo(updated);
			open = false;
			onComplete(updated);
		} catch (err) {
			error = (err as Error).message;
		} finally {
			rescanning = false;
		}
	}

	function handleClose() {
		open = false;
		error = null;
	}

	const isDisabled = $derived(saving || skipping || submitting || confirming);
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-[800px] max-h-[90vh] overflow-y-auto">
		<Dialog.Header>
			<Dialog.Title class="flex items-center gap-2">
				<div class="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center">
					<Settings class="w-4 h-4 text-primary" />
				</div>
				Configure Repository
			</Dialog.Title>
			<Dialog.Description>
				Set up the tech stack and expectations for how the AI agent should work with this repository.
			</Dialog.Description>
		</Dialog.Header>

		<div class="py-4 space-y-6">
			<!-- Repo Summary -->
			<div class="bg-muted/30 rounded-lg p-4 border">
				<RepoSummary {repo} />
			</div>

			<!-- Tech Stack Editor -->
			<div>
				<label for="tech-stack-input" class="text-sm font-medium mb-2 flex items-center gap-2">
					<Layers class="w-4 h-4 text-muted-foreground" />
					Tech Stack
				</label>
				<p class="text-xs text-muted-foreground mb-3">
					Specify the technologies used in this repository. This helps the AI agent understand the project context.
				</p>
				{#if techStack.length > 0}
					<div class="flex flex-wrap gap-1.5 mb-3">
						{#each techStack as tech, i}
							<Badge variant="secondary" class="text-xs gap-1 pr-1">
								{tech}
								<button
									type="button"
									class="ml-0.5 rounded-full hover:bg-muted-foreground/20 p-0.5 transition-colors"
									onclick={() => removeTechStackItem(i)}
									disabled={isDisabled}
									aria-label="Remove {tech}"
								>
									<X class="w-3 h-3" />
								</button>
							</Badge>
						{/each}
					</div>
				{/if}
				<div class="flex gap-2">
					<input
						id="tech-stack-input"
						type="text"
						bind:value={techStackInput}
						onkeydown={handleTechStackKeydown}
						class="flex-1 border rounded-lg px-3 py-2 bg-background text-foreground text-sm focus:outline-none focus:ring-2 focus:ring-ring transition-shadow"
						placeholder="e.g., TypeScript, React, PostgreSQL..."
						disabled={isDisabled}
					/>
					<Button
						type="button"
						variant="outline"
						size="sm"
						onclick={addTechStackItem}
						disabled={isDisabled || !techStackInput.trim()}
						class="gap-1 shrink-0"
					>
						<Plus class="w-4 h-4" />
						Add
					</Button>
				</div>
			</div>

			<!-- Section shortcuts -->
			<div>
				<h3 class="text-sm font-medium mb-3">Quick-add sections</h3>
				<p class="text-xs text-muted-foreground mb-3">
					Click a category to add its section header to the expectations below. Use these as a guide for what to configure.
				</p>
				<div class="grid grid-cols-2 sm:grid-cols-3 gap-2">
					{#each sections as section}
						<button
							type="button"
							class="flex items-center gap-2 px-3 py-2 rounded-lg border text-sm text-left hover:bg-accent/50 transition-colors {expectations.includes(`## ${section.label}`) ? 'border-primary/30 bg-primary/5' : ''}"
							onclick={() => insertSectionTemplate(section)}
							title={section.tip}
						>
							<section.icon class="w-4 h-4 text-muted-foreground shrink-0" />
							<span class="truncate">{section.label}</span>
							{#if expectations.includes(`## ${section.label}`)}
								<Check class="w-3 h-3 text-primary ml-auto shrink-0" />
							{/if}
						</button>
					{/each}
				</div>
			</div>

			<!-- Expectations Editor -->
			<div>
				<label for="expectations" class="text-sm font-medium mb-2 flex items-center gap-2">
					<FileText class="w-4 h-4 text-muted-foreground" />
					Expectations
					<span class="text-xs text-muted-foreground font-normal">(markdown supported)</span>
				</label>
				<textarea
					id="expectations"
					bind:value={expectations}
					class="w-full border rounded-lg p-3 min-h-[280px] bg-background text-foreground resize-y focus:outline-none focus:ring-2 focus:ring-ring transition-shadow font-mono text-sm"
					placeholder="Describe how the AI agent should work with this repository...&#10;&#10;You can use markdown formatting. Use the section buttons above to add category headers."
					disabled={isDisabled}
				></textarea>
			</div>

			<!-- Section reference (collapsible) -->
			<div>
				<button
					type="button"
					class="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
					onclick={() => toggleSection('tips')}
				>
					{#if expandedSections.has('tips')}
						<ChevronDown class="w-4 h-4" />
					{:else}
						<ChevronRight class="w-4 h-4" />
					{/if}
					Tips & Examples
				</button>
				{#if expandedSections.has('tips')}
					<div class="mt-3 space-y-2 pl-1">
						{#each sections as section}
							<div class="text-xs text-muted-foreground">
								<span class="font-medium text-foreground">{section.label}:</span>
								{section.tip}
							</div>
						{/each}
					</div>
				{/if}
			</div>

			{#if error}
				<div class="bg-destructive/10 text-destructive text-sm p-3 rounded-lg flex items-center gap-2">
					<X class="w-4 h-4 flex-shrink-0" />
					{error}
				</div>
			{/if}
		</div>

		<Dialog.Footer>
			<div class="flex items-center justify-between w-full gap-2">
				<div class="flex items-center gap-2">
					<Button
						type="button"
						variant="outline"
						onclick={handleRescan}
						disabled={isDisabled || rescanning}
						class="gap-1.5"
					>
						<RefreshCw class="w-4 h-4 {rescanning ? 'animate-spin' : ''}" />
						Rescan
					</Button>
				</div>
				<div class="flex items-center gap-2">
					<Button type="button" variant="ghost" onclick={handleClose} disabled={isDisabled}>
						Cancel
					</Button>
					<Button
						type="button"
						variant="outline"
						onclick={handleSkip}
						disabled={isDisabled}
						class="gap-1.5"
					>
						{#if skipping}
							<Loader2 class="w-4 h-4 animate-spin" />
							Skipping...
						{:else}
							<SkipForward class="w-4 h-4" />
							Skip
						{/if}
					</Button>
					<Button
						type="button"
						variant="outline"
						onclick={handleSubmitForReview}
						disabled={isDisabled}
						class="gap-1.5"
					>
						{#if submitting}
							<Loader2 class="w-4 h-4 animate-spin" />
							Submitting...
						{:else}
							<Sparkles class="w-4 h-4" />
							Submit for AI Review
						{/if}
					</Button>
					<Button
						type="button"
						onclick={handleSave}
						disabled={isDisabled}
						class="gap-1.5"
					>
						{#if saving}
							<Loader2 class="w-4 h-4 animate-spin" />
							Saving...
						{:else}
							<Check class="w-4 h-4" />
							Save & Complete
						{/if}
					</Button>
				</div>
			</div>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
