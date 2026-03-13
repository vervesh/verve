<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { onDestroy } from 'svelte';
	import { client } from '$lib/api-client';
	import { epicStore } from '$lib/stores/epics.svelte';
	import { repoStore } from '$lib/stores/repos.svelte';
	import { taskUrl, epicUrl } from '$lib/utils';
	import { renderMarkdown } from '$lib/markdown';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import type { Epic, ProposedTask } from '$lib/models/epic';
	import ProposedTaskPreviewDialog from '$lib/components/ProposedTaskPreviewDialog.svelte';
	import EditProposedTaskDialog from '$lib/components/EditProposedTaskDialog.svelte';
	import {
		ArrowLeft,
		Layers,
		Loader2,
		Plus,
		Trash2,
		Check,
		X,
		Edit3,
		Link2,
		MessageSquare,
		PauseCircle,
		StopCircle,
		CheckCircle2,
		AlertCircle,
		AlertTriangle,
		RefreshCw,
		Clock,
		CircleDot,
		GitMerge,
		XCircle,
		Play,
		Eye,
		Terminal,
		Send
	} from 'lucide-svelte';

	let epic = $state<Epic | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let epicLoaded = $state(false);

	// Epic task statuses
	type EpicTask = { id: string; number: number; title: string; status: string };
	let epicTasks = $state<EpicTask[]>([]);
	let failedTasks = $derived(epicTasks.filter((t) => t.status === 'failed'));
	let isActive = $derived(epic?.status === 'active');
	let isCompleted = $derived(epic?.status === 'completed');
	let hasCreatedTasks = $derived(epic != null && epic.task_ids.length > 0);

	// Grouped epic tasks by status
	let pendingTasks = $derived(epicTasks.filter((t) => t.status === 'pending'));
	let runningTasks = $derived(epicTasks.filter((t) => t.status === 'running'));
	let reviewTasks = $derived(epicTasks.filter((t) => t.status === 'review'));
	let doneTasks = $derived(epicTasks.filter((t) => t.status === 'merged' || t.status === 'closed'));
	let epicFailedTasks = $derived(epicTasks.filter((t) => t.status === 'failed'));

	// Planning state
	let isPlanning = $derived(epic?.status === 'planning');
	let isDraft = $derived(epic?.status === 'draft' || epic?.status === 'ready');
	let isEditable = $derived(isDraft);
	let isClaimed = $derived(!!epic?.claimed_at);
	let canStopPlanning = $derived(isPlanning && isClaimed);
	let stopping = $state(false);

	// Change request state
	let changeMessage = $state('');
	let sendingChange = $state(false);

	// Task preview/edit dialog state
	let previewDialogOpen = $state(false);
	let editDialogOpen = $state(false);
	let selectedTaskIdx = $state<number>(0);
	let selectedTask = $derived<ProposedTask | null>(
		epic && selectedTaskIdx >= 0 && selectedTaskIdx < (epic?.proposed_tasks?.length ?? 0)
			? epic.proposed_tasks[selectedTaskIdx]
			: null
	);

	// Log auto-scroll
	let logContainer: HTMLDivElement | null = $state(null);
	let logAutoScroll = $state(true);
	let lastLogCount = $state(0);

	// Confirmation state
	let confirming = $state(false);
	let notReady = $state(false);
	let closing = $state(false);
	let showDeleteConfirm = $state(false);
	let deleting = $state(false);

	// Polling
	let pollTimer: ReturnType<typeof setInterval> | null = null;
	let taskPollTimer: ReturnType<typeof setInterval> | null = null;

	const ownerParam = $derived($page.params.owner as string);
	const nameParam = $derived($page.params.name as string);
	const numberParam = $derived(Number($page.params.number));

	// Resolve the repo from the store by owner/name
	const repo = $derived(repoStore.repos.find((r) => r.owner === ownerParam && r.name === nameParam) ?? null);

	// Use $effect to wait for repo store to be populated before loading.
	// The layout loads repos asynchronously, so repo may be null on first render.
	$effect(() => {
		if (repo && !epicLoaded) {
			epicLoaded = true;
			loadEpic();
		}
	});

	onDestroy(() => {
		stopPolling();
		stopTaskPolling();
	});

	function startPolling() {
		if (pollTimer) return;
		pollTimer = setInterval(async () => {
			if (!epic) return;
			try {
				const updated = await client.getEpic(epic.id);
				epic = updated;
				if (updated.status !== 'planning') {
					stopPolling();
				}
			} catch {
				// Ignore polling errors silently
			}
		}, 2000);
	}

	function stopPolling() {
		if (pollTimer) {
			clearInterval(pollTimer);
			pollTimer = null;
		}
	}

	function startTaskPolling() {
		if (taskPollTimer) return;
		taskPollTimer = setInterval(async () => {
			if (!epic) return;
			try {
				epicTasks = await client.getEpicTasks(epic.id);
			} catch {
				// Ignore polling errors silently
			}
		}, 5000);
	}

	function stopTaskPolling() {
		if (taskPollTimer) {
			clearInterval(taskPollTimer);
			taskPollTimer = null;
		}
	}

	async function loadEpic() {
		loading = true;
		error = null;
		try {
			if (!repo) {
				error = 'Repository not found';
				return;
			}
			epic = await client.getEpicByNumber(repo.id, numberParam);
			if (epic.status === 'planning') {
				startPolling();
			}
			if (epic.task_ids.length > 0) {
				await loadEpicTasks();
				if (epic.status === 'active') {
					startTaskPolling();
				}
			}
		} catch (err) {
			error = (err as Error).message;
		} finally {
			loading = false;
		}
	}

	async function loadEpicTasks() {
		if (!epic) return;
		try {
			epicTasks = await client.getEpicTasks(epic.id);
		} catch {
			// Non-critical
		}
	}

	async function handleRequestChanges() {
		if (!changeMessage.trim() || !epic) return;
		sendingChange = true;
		error = null;
		try {
			epic = await client.sendSessionMessage(epic.id, changeMessage);
			changeMessage = '';
			// Epic transitions to planning — start polling
			startPolling();
		} catch (err) {
			error = (err as Error).message;
		} finally {
			sendingChange = false;
		}
	}

	function openPreviewTask(idx: number) {
		if (!epic) return;
		selectedTaskIdx = idx;
		previewDialogOpen = true;
	}

	function openEditTask(idx: number) {
		if (!epic || isPlanning) return;
		selectedTaskIdx = idx;
		editDialogOpen = true;
	}

	async function handleSaveProposedTask(updated: ProposedTask) {
		if (!epic) return;
		const tasks = [...epic.proposed_tasks];
		const idx = tasks.findIndex((t) => t.temp_id === updated.temp_id);
		if (idx === -1) return;
		tasks[idx] = updated;
		try {
			epic = await client.updateProposedTasks(epic.id, tasks);
		} catch (err) {
			error = (err as Error).message;
		}
	}

	async function addNewTask() {
		if (!epic) return;
		const newTask: ProposedTask = {
			temp_id: `temp_${Date.now()}`,
			title: 'New task',
			description: '',
			depends_on_temp_ids: [],
			acceptance_criteria: []
		};
		const tasks = [...epic.proposed_tasks, newTask];
		try {
			epic = await client.updateProposedTasks(epic.id, tasks);
			openEditTask(tasks.length - 1);
		} catch (err) {
			error = (err as Error).message;
		}
	}

	async function removeTask(idx: number) {
		if (!epic) return;
		const removedId = epic.proposed_tasks[idx].temp_id;
		const tasks = epic.proposed_tasks
			.filter((_, i) => i !== idx)
			.map((t) => ({
				...t,
				depends_on_temp_ids: (t.depends_on_temp_ids ?? []).filter((id) => id !== removedId)
			}));
		try {
			epic = await client.updateProposedTasks(epic.id, tasks);
		} catch (err) {
			error = (err as Error).message;
		}
	}

	async function handleConfirm() {
		if (!epic) return;
		confirming = true;
		error = null;
		try {
			epic = await client.confirmEpic(epic.id, notReady);
			epicStore.updateEpic(epic);
			if (epic.task_ids.length > 0) {
				await loadEpicTasks();
				if (epic.status === 'active') {
					startTaskPolling();
				}
			}
		} catch (err) {
			error = (err as Error).message;
		} finally {
			confirming = false;
		}
	}

	async function handleStopPlanning() {
		if (!epic) return;
		stopping = true;
		error = null;
		try {
			epic = await client.stopEpic(epic.id);
			epicStore.updateEpic(epic);
			stopPolling();
		} catch (err) {
			error = (err as Error).message;
		} finally {
			stopping = false;
		}
	}

	async function handleClose() {
		if (!epic) return;
		closing = true;
		error = null;
		try {
			epic = await client.closeEpic(epic.id);
			epicStore.updateEpic(epic);
		} catch (err) {
			error = (err as Error).message;
		} finally {
			closing = false;
		}
	}

	async function handleDelete() {
		if (!epic) return;
		deleting = true;
		error = null;
		try {
			await client.deleteEpic(epic.id);
			epicStore.removeEpic(epic.id);
			goto('/epics');
		} catch (err) {
			error = (err as Error).message;
		} finally {
			deleting = false;
			showDeleteConfirm = false;
		}
	}

	function getStatusColor(status: string) {
		switch (status) {
			case 'draft':
				return 'bg-gray-500/15 text-gray-400';
			case 'planning':
				return 'bg-violet-500/15 text-violet-400';
			case 'ready':
				return 'bg-amber-500/15 text-amber-400';
			case 'active':
				return 'bg-blue-500/15 text-blue-400';
			case 'completed':
				return 'bg-green-500/15 text-green-400';
			case 'closed':
				return 'bg-red-500/15 text-red-400';
			default:
				return 'bg-gray-500/15 text-gray-400';
		}
	}

	function getDependencyLabel(tempId: string): string {
		if (!epic) return tempId;
		const t = epic.proposed_tasks.find((pt) => pt.temp_id === tempId);
		return t ? t.title : tempId;
	}

	function getTaskStatusBadge(status: string): { bg: string; text: string; label: string } {
		switch (status) {
			case 'pending':
				return { bg: 'bg-amber-500/15 border-amber-500/20', text: 'text-amber-400', label: 'Pending' };
			case 'running':
				return { bg: 'bg-blue-500/15 border-blue-500/20', text: 'text-blue-400', label: 'Running' };
			case 'review':
				return { bg: 'bg-purple-500/15 border-purple-500/20', text: 'text-purple-400', label: 'In Review' };
			case 'merged':
				return { bg: 'bg-green-500/15 border-green-500/20', text: 'text-green-400', label: 'Merged' };
			case 'closed':
				return { bg: 'bg-gray-500/15 border-gray-500/20', text: 'text-gray-400', label: 'Closed' };
			case 'failed':
				return { bg: 'bg-red-500/15 border-red-500/20', text: 'text-red-400', label: 'Failed' };
			default:
				return { bg: 'bg-gray-500/15 border-gray-500/20', text: 'text-gray-400', label: status };
		}
	}

	// Auto-scroll log when new entries arrive
	$effect(() => {
		const logCount = epic?.session_log?.length ?? 0;
		if (logCount > lastLogCount) {
			lastLogCount = logCount;
			if (logAutoScroll && logContainer) {
				requestAnimationFrame(() => {
					if (logContainer) {
						logContainer.scrollTop = logContainer.scrollHeight;
					}
				});
			}
		}
	});

	function handleLogScroll(e: Event) {
		const el = e.target as HTMLDivElement;
		const isNearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 50;
		logAutoScroll = isNearBottom;
	}

	function handleLogWheel(e: WheelEvent) {
		const el = e.currentTarget as HTMLDivElement;
		const atTop = el.scrollTop <= 0;
		const atBottom = el.scrollTop + el.clientHeight >= el.scrollHeight - 1;
		if ((e.deltaY < 0 && atTop) || (e.deltaY > 0 && atBottom)) {
			e.preventDefault();
		}
	}

	function getLogLineClass(line: string): string {
		if (line.startsWith('user:')) return 'text-blue-400';
		if (line.startsWith('system:')) return 'text-violet-400';
		if (line.startsWith('error:') || line.startsWith('[ERROR]')) return 'text-red-400';
		return 'text-muted-foreground';
	}
</script>

<div class="p-4 sm:p-6 flex-1 min-h-0 flex flex-col">
	<div class="mb-4">
		<button
			onclick={() => goto('/epics')}
			class="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
		>
			<ArrowLeft class="w-4 h-4" />
			Back to Epics
		</button>
	</div>

	{#if loading}
		<div class="flex items-center justify-center py-20">
			<Loader2 class="w-8 h-8 animate-spin text-muted-foreground" />
		</div>
	{:else if error && !epic}
		<div class="bg-destructive/10 text-destructive p-4 rounded-lg flex items-center gap-3">
			<AlertCircle class="w-5 h-5" />
			{error}
		</div>
	{:else if epic}
		<!-- Header -->
		<header class="flex flex-col sm:flex-row sm:items-center gap-3 mb-6">
			<div class="flex items-center gap-3 flex-1 min-w-0">
				<div class="w-10 h-10 rounded-lg bg-violet-500/10 flex items-center justify-center shrink-0">
					<Layers class="w-5 h-5 text-violet-500" />
				</div>
				<div class="min-w-0">
					<h1 class="text-xl font-bold truncate">{epic.title}</h1>
					<div class="flex items-center gap-2 mt-0.5">
						<span class="text-xs font-mono text-muted-foreground">#{epic.number}</span>
						<span class="px-2 py-0.5 rounded-full text-[11px] font-semibold {getStatusColor(epic.status)}">
							{epic.status}
						</span>
						{#if isPlanning}
							{#if isClaimed}
								<Loader2 class="w-3 h-3 animate-spin text-violet-400" />
							{:else}
								<Clock class="w-3 h-3 text-muted-foreground" />
							{/if}
						{/if}
					</div>
				</div>
			</div>
			<div class="flex items-center gap-2 shrink-0">
				{#if isDraft}
					<Button variant="outline" size="sm" onclick={handleClose} disabled={closing} class="gap-1.5 text-red-400 border-red-500/30 hover:bg-red-500/10">
						{#if closing}
							<Loader2 class="w-3.5 h-3.5 animate-spin" />
						{:else}
							<X class="w-3.5 h-3.5" />
						{/if}
						Close Epic
					</Button>
				{/if}
				<Button variant="outline" size="sm" onclick={() => (showDeleteConfirm = true)} class="gap-1.5 text-red-400 border-red-500/30 hover:bg-red-500/10">
					<Trash2 class="w-3.5 h-3.5" />
					Delete
				</Button>
			</div>
		</header>

		{#if error}
			<div class="bg-destructive/10 text-destructive p-3 rounded-lg text-sm mb-4 flex items-center gap-2">
				<AlertCircle class="w-4 h-4 shrink-0" />
				{error}
			</div>
		{/if}

		<!-- Planning status banner -->
		{#if isPlanning}
			<Card.Root class="mb-6 border-violet-500/20 bg-violet-500/5">
				<Card.Content class="p-4">
					<div class="flex items-center justify-between">
						<div class="flex items-center gap-3">
							{#if isClaimed}
								<Loader2 class="w-5 h-5 animate-spin text-violet-400 shrink-0" />
								<div>
									<p class="text-sm font-medium text-violet-400">Agent is planning...</p>
									<p class="text-xs text-muted-foreground mt-0.5">
										The AI agent is analyzing the codebase and generating a task breakdown. The agent will propose tasks when done.
									</p>
								</div>
							{:else}
								<Clock class="w-5 h-5 text-muted-foreground shrink-0" />
								<div>
									<p class="text-sm font-medium text-muted-foreground">Queued for planning...</p>
									<p class="text-xs text-muted-foreground mt-0.5">
										This epic is in the planning queue and will be picked up by the next available worker.
									</p>
								</div>
							{/if}
						</div>
						{#if canStopPlanning}
							<Button variant="outline" size="sm" onclick={handleStopPlanning} disabled={stopping} class="gap-1.5 text-red-400 border-red-500/30 hover:bg-red-500/10 shrink-0">
								{#if stopping}
									<Loader2 class="w-3.5 h-3.5 animate-spin" />
								{:else}
									<StopCircle class="w-3.5 h-3.5" />
								{/if}
								Stop Planning
							</Button>
						{/if}
					</div>
				</Card.Content>
			</Card.Root>
		{/if}

		<!-- Failed tasks warning banner -->
		{#if isActive && failedTasks.length > 0}
			<Card.Root class="mb-6 border-red-500/20 bg-red-500/5">
				<Card.Content class="p-4">
					<div class="flex items-start gap-3">
						<AlertTriangle class="w-5 h-5 text-red-400 shrink-0 mt-0.5" />
						<div>
							<p class="text-sm font-medium text-red-400">
								{failedTasks.length} failed task{failedTasks.length !== 1 ? 's' : ''} preventing epic completion
							</p>
							<div class="mt-2 space-y-1">
								{#each failedTasks as ft}
									<a
										href={repo ? taskUrl(repo.owner, repo.name, ft.number) : '#'}
										class="flex items-center gap-1.5 text-xs text-red-400/80 hover:text-red-400 hover:underline"
									>
										<XCircle class="w-3 h-3" />
										{ft.title}
									</a>
								{/each}
							</div>
						</div>
					</div>
				</Card.Content>
			</Card.Root>
		{/if}

		{#if epic.description}
			<Card.Root class="mb-6 bg-[oklch(0.18_0.005_285.823)]">
				<Card.Content class="p-4 max-h-48 overflow-y-auto overscroll-contain">
					<div class="prose prose-sm dark:prose-invert max-w-none">
						{@html renderMarkdown(epic.description)}
					</div>
				</Card.Content>
			</Card.Root>
		{/if}

		{#if hasCreatedTasks}
			<!-- Active/Completed Epic: Show actual tasks grouped by status -->
			<div class="flex-1 min-h-0 flex flex-col">
				<div class="flex items-center justify-between mb-3">
					<h2 class="text-sm font-semibold flex items-center gap-2">
						Epic Tasks
						<span class="px-2 py-0.5 rounded-full text-xs bg-muted text-muted-foreground">
							{epicTasks.length}
						</span>
					</h2>
					{#if isActive}
						<div class="flex items-center gap-1.5 text-xs text-muted-foreground">
							<Loader2 class="w-3 h-3 animate-spin" />
							Auto-refreshing
						</div>
					{/if}
				</div>

				<!-- Task status summary bar -->
				{#if epicTasks.length > 0}
					<div class="flex items-center gap-3 mb-4 flex-wrap">
						{#if pendingTasks.length > 0}
							<span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-amber-500/10 text-amber-400 border border-amber-500/20">
								<Clock class="w-3 h-3" />
								{pendingTasks.length} Pending
							</span>
						{/if}
						{#if runningTasks.length > 0}
							<span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-blue-500/10 text-blue-400 border border-blue-500/20">
								<Play class="w-3 h-3" />
								{runningTasks.length} Running
							</span>
						{/if}
						{#if reviewTasks.length > 0}
							<span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-purple-500/10 text-purple-400 border border-purple-500/20">
								<Eye class="w-3 h-3" />
								{reviewTasks.length} In Review
							</span>
						{/if}
						{#if doneTasks.length > 0}
							<span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-green-500/10 text-green-400 border border-green-500/20">
								<CheckCircle2 class="w-3 h-3" />
								{doneTasks.length} Done
							</span>
						{/if}
						{#if epicFailedTasks.length > 0}
							<span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-red-500/10 text-red-400 border border-red-500/20">
								<XCircle class="w-3 h-3" />
								{epicFailedTasks.length} Failed
							</span>
						{/if}
					</div>

					<!-- Progress bar -->
					{@const totalCount = epicTasks.length}
					{@const mergedCount = epicTasks.filter(t => t.status === 'merged').length}
					{@const closedCount = epicTasks.filter(t => t.status === 'closed').length}
					{@const failedCount = epicFailedTasks.length}
					{@const reviewCount = reviewTasks.length}
					{@const runningCount = runningTasks.length}
					<div class="w-full h-2 rounded-full bg-muted overflow-hidden flex mb-4">
						{#if mergedCount > 0}
							<div class="bg-green-500 h-full" style="width: {(mergedCount / totalCount) * 100}%"></div>
						{/if}
						{#if closedCount > 0}
							<div class="bg-gray-500 h-full" style="width: {(closedCount / totalCount) * 100}%"></div>
						{/if}
						{#if reviewCount > 0}
							<div class="bg-purple-500 h-full" style="width: {(reviewCount / totalCount) * 100}%"></div>
						{/if}
						{#if runningCount > 0}
							<div class="bg-blue-500 h-full" style="width: {(runningCount / totalCount) * 100}%"></div>
						{/if}
						{#if failedCount > 0}
							<div class="bg-red-500 h-full" style="width: {(failedCount / totalCount) * 100}%"></div>
						{/if}
					</div>
				{/if}

				<!-- Task list -->
				<div class="space-y-2 overflow-y-auto overscroll-contain flex-1 min-h-0">
					{#each epicTasks as epicTask (epicTask.id)}
						{@const badge = getTaskStatusBadge(epicTask.status)}
						<a
							href={repo ? taskUrl(repo.owner, repo.name, epicTask.number) : '#'}
							class="block"
						>
							<Card.Root class="bg-[oklch(0.18_0.005_285.823)] shadow-sm hover:bg-accent/50 hover:border-accent transition-all duration-200 hover:shadow-md cursor-pointer">
								<Card.Content class="p-3">
									<div class="flex items-start justify-between gap-3">
										<div class="flex items-start gap-3 flex-1 min-w-0">
											<div class="mt-0.5 shrink-0">
												{#if epicTask.status === 'running'}
													<Loader2 class="w-4 h-4 text-blue-400 animate-spin" />
												{:else if epicTask.status === 'pending'}
													<Clock class="w-4 h-4 text-amber-400" />
												{:else if epicTask.status === 'review'}
													<Eye class="w-4 h-4 text-purple-400" />
												{:else if epicTask.status === 'merged'}
													<GitMerge class="w-4 h-4 text-green-400" />
												{:else if epicTask.status === 'closed'}
													<XCircle class="w-4 h-4 text-gray-400" />
												{:else if epicTask.status === 'failed'}
													<AlertCircle class="w-4 h-4 text-red-400" />
												{:else}
													<CircleDot class="w-4 h-4 text-muted-foreground" />
												{/if}
											</div>
											<div class="flex-1 min-w-0">
												<p class="text-sm font-medium truncate">{epicTask.title}</p>
												<span class="text-[10px] text-muted-foreground font-mono">#{epicTask.number}</span>
											</div>
										</div>
										<span class="inline-flex items-center gap-1 text-[11px] font-semibold {badge.text} {badge.bg} px-2 py-0.5 rounded-full border shrink-0">
											{badge.label}
										</span>
									</div>
								</Card.Content>
							</Card.Root>
						</a>
					{/each}
					{#if epicTasks.length === 0}
						<Card.Root class="bg-[oklch(0.18_0.005_285.823)]">
							<Card.Content class="p-8 text-center">
								<div class="w-12 h-12 rounded-xl bg-muted flex items-center justify-center mx-auto mb-3">
									<Loader2 class="w-6 h-6 text-muted-foreground animate-spin" />
								</div>
								<p class="text-sm text-muted-foreground">Loading tasks...</p>
							</Card.Content>
						</Card.Root>
					{/if}
				</div>
			</div>
		{:else}
			<!-- Draft/Planning: Show proposed tasks + session terminal -->
			<div class="grid grid-cols-1 lg:grid-cols-3 gap-6 flex-1">
				<!-- Left: Proposed Tasks -->
				<div class="lg:col-span-2 flex flex-col min-h-0">
					<div class="flex items-center justify-between mb-3">
						<h2 class="text-md font-semibold flex items-center gap-2">
							Proposed Tasks
							{#if epic.proposed_tasks.length > 0}
								<span class="px-2 py-0.5 rounded-full text-xs bg-muted text-muted-foreground">
									{epic.proposed_tasks.length}
								</span>
							{/if}
						</h2>
						{#if isEditable}
							<Button variant="outline" size="sm" onclick={addNewTask} class="gap-1.5 text-xs" disabled={isPlanning}>
								<Plus class="w-3.5 h-3.5" />
								Add Task
							</Button>
						{/if}
					</div>

					{#if epic.proposed_tasks.length === 0}
						<Card.Root class="bg-[oklch(0.18_0.005_285.823)] flex-1">
							<Card.Content class="p-8 text-center">
								<div class="w-12 h-12 rounded-xl bg-muted flex items-center justify-center mx-auto mb-3">
									{#if isPlanning}
										<Loader2 class="w-6 h-6 text-violet-400 animate-spin" />
									{:else}
										<Layers class="w-6 h-6 text-muted-foreground" />
									{/if}
								</div>
								<p class="text-sm text-muted-foreground">
									{#if isPlanning && isClaimed}
										Agent is analyzing the epic and generating tasks...
									{:else if isPlanning}
										Waiting for an agent to start planning...
									{:else}
										No tasks have been proposed yet.
									{/if}
								</p>
							</Card.Content>
						</Card.Root>
					{:else}
						<div class="space-y-2 overflow-y-auto overscroll-contain flex-1 min-h-0 max-h-[60vh]">
							{#each epic.proposed_tasks as task, idx (task.temp_id)}
								<button
									type="button"
									class="w-full text-left"
									onclick={() => openPreviewTask(idx)}
								>
									<Card.Root class="relative bg-[oklch(0.18_0.005_285.823)] {isPlanning ? 'opacity-60' : ''} hover:bg-accent/50 hover:border-accent transition-all duration-200 hover:shadow-md cursor-pointer">
										{#if isEditable && !isPlanning}
											<div class="absolute top-2 right-2 flex items-center gap-0.5">
												<button
													class="p-1.5 hover:bg-accent rounded-md transition-colors text-muted-foreground hover:text-foreground"
													onclick={(e) => { e.stopPropagation(); e.preventDefault(); openEditTask(idx); }}
													title="Edit task"
												>
													<Edit3 class="w-4 h-4" />
												</button>
												<button
													class="p-1.5 hover:bg-destructive/10 hover:text-destructive rounded-md transition-colors text-muted-foreground"
													onclick={(e) => { e.stopPropagation(); e.preventDefault(); removeTask(idx); }}
													title="Remove task"
												>
													<Trash2 class="w-4 h-4" />
												</button>
											</div>
										{/if}
										<Card.Content class="p-3">
											<div class="flex items-start gap-2">
												<span class="text-xs text-muted-foreground font-mono mt-0.5 shrink-0">{idx + 1}.</span>
												<div class="flex-1 min-w-0 {isEditable && !isPlanning ? 'pr-16' : ''}">
													<p class="text-sm font-medium">{task.title}</p>
													{#if task.description}
														<div class="text-xs text-muted-foreground mt-1 line-clamp-2 prose prose-sm dark:prose-invert max-w-none [&>*]:m-0 [&>*]:text-xs [&>*]:text-muted-foreground">
															{@html renderMarkdown(task.description)}
														</div>
													{/if}
													<div class="flex items-center gap-3 mt-2 flex-wrap">
														{#if task.depends_on_temp_ids && task.depends_on_temp_ids.length > 0}
															<span class="text-[10px] text-muted-foreground flex items-center gap-0.5">
																<Link2 class="w-3 h-3" />
																{task.depends_on_temp_ids.map((id) => getDependencyLabel(id)).join(', ')}
															</span>
														{/if}
														{#if task.acceptance_criteria && task.acceptance_criteria.length > 0}
															<span class="text-[10px] text-muted-foreground flex items-center gap-0.5">
																<CheckCircle2 class="w-3 h-3" />
																{task.acceptance_criteria.length} criteria
															</span>
														{/if}
													</div>
												</div>
											</div>
										</Card.Content>
									</Card.Root>
								</button>
							{/each}
						</div>
					{/if}

					<!-- Confirm / Ready section -->
					{#if isEditable && epic.proposed_tasks.length > 0}
						<Card.Root class="mt-4 bg-[oklch(0.18_0.005_285.823)] border-green-500/20">
							<Card.Content class="p-4">
								<div class="flex flex-col sm:flex-row items-start sm:items-center gap-3">
									<div class="flex-1">
										<p class="text-sm font-medium">Ready to create tasks?</p>
										<p class="text-xs text-muted-foreground mt-0.5">
											This will create {epic.proposed_tasks.length} task{epic.proposed_tasks.length !== 1 ? 's' : ''} from the proposed plan.
										</p>
									</div>
									<div class="flex items-center gap-3">
										<label class="flex items-center gap-2 cursor-pointer">
											<input
												type="checkbox"
												bind:checked={notReady}
												class="w-3.5 h-3.5 rounded border-input accent-primary"
											/>
											<span class="text-xs flex items-center gap-1">
												<PauseCircle class="w-3 h-3" />
												Hold tasks
											</span>
										</label>
										<Button onclick={handleConfirm} disabled={confirming} class="gap-1.5 bg-green-600 hover:bg-green-700">
											{#if confirming}
												<Loader2 class="w-4 h-4 animate-spin" />
												Confirming...
											{:else}
												<Check class="w-4 h-4" />
												Confirm Epic
											{/if}
										</Button>
									</div>
								</div>
							</Card.Content>
						</Card.Root>
					{/if}
				</div>

				<!-- Right: Session Terminal -->
				<div class="flex flex-col min-h-0">
					<h2 class="text-md font-semibold mt-1 mb-3 flex items-center gap-2">
						<Terminal class="w-4 h-4 text-violet-400" />
						Planning Log
						{#if isPlanning && isClaimed}
							<Loader2 class="w-3 h-3 animate-spin text-violet-400" />
						{/if}
					</h2>

					<!-- Terminal-style log view -->
					<Card.Root class="mt-1 bg-[oklch(0.13_0.005_285.823)] flex flex-col min-h-[300px] max-h-[60vh] border-border/50">
						<Card.Content class="p-0 flex-1 flex flex-col min-h-0 overflow-hidden">
							{#if epic.planning_prompt}
								<div class="text-xs text-muted-foreground px-3 py-2 border-b border-border/30">
									<span class="font-medium text-violet-400">Planning prompt:</span>
									<p class="mt-1 line-clamp-3">{epic.planning_prompt}</p>
								</div>
							{/if}

							<!-- Log output -->
							<div
								bind:this={logContainer}
								onscroll={handleLogScroll}
								onwheel={handleLogWheel}
								class="flex-1 overflow-y-auto overscroll-contain min-h-0 p-3 font-mono text-xs space-y-0.5"
							>
								{#each epic.session_log as line, i}
									<div class="{getLogLineClass(line)} leading-relaxed">
										<span class="text-muted-foreground/40 select-none mr-2">{String(i + 1).padStart(3, ' ')}</span>{line}
									</div>
								{/each}
								{#if epic.session_log.length === 0 && !isPlanning}
									<p class="text-muted-foreground/50 text-center py-8">
										No log output yet.
									</p>
								{/if}
								{#if isPlanning}
									<div class="flex items-center gap-2 text-violet-400 pt-2">
										<Loader2 class="w-3 h-3 animate-spin" />
										{#if isClaimed}
											Agent is working...
										{:else}
											Waiting for worker...
										{/if}
									</div>
								{/if}
							</div>

							<!-- Request changes input (when in draft/ready state) -->
							{#if isDraft && epic.proposed_tasks.length > 0}
								<div class="mt-auto border-t border-border/30 p-3">
									<p class="text-[10px] text-muted-foreground mb-2">
										Not happy with the plan? Describe what should change and the agent will re-plan.
									</p>
									<div class="flex items-center gap-2">
										<input
											type="text"
											bind:value={changeMessage}
											class="flex-1 border border-border/50 rounded-lg px-3 py-2 bg-background/50 text-foreground text-sm focus:outline-none focus:ring-2 focus:ring-ring"
											placeholder="Describe changes needed..."
											disabled={sendingChange}
											onkeydown={(e) => {
												if (e.key === 'Enter' && !e.shiftKey) {
													e.preventDefault();
													handleRequestChanges();
												}
											}}
										/>
										<Button
											size="sm"
											variant="outline"
											onclick={handleRequestChanges}
											disabled={!changeMessage.trim() || sendingChange}
											class="shrink-0 gap-1.5"
										>
											{#if sendingChange}
												<Loader2 class="w-4 h-4 animate-spin" />
											{:else}
												<Send class="w-4 h-4" />
											{/if}
											Re-plan
										</Button>
									</div>
								</div>
							{/if}
						</Card.Content>
					</Card.Root>
				</div>
			</div>
		{/if}
	{/if}
</div>

<!-- Task preview dialog -->
{#if epic}
	<ProposedTaskPreviewDialog
		bind:open={previewDialogOpen}
		task={selectedTask}
		taskIndex={selectedTaskIdx}
		allTasks={epic.proposed_tasks}
		onEdit={isEditable && !isPlanning ? () => openEditTask(selectedTaskIdx) : undefined}
	/>

	<!-- Task edit dialog -->
	<EditProposedTaskDialog
		bind:open={editDialogOpen}
		task={selectedTask}
		taskIndex={selectedTaskIdx}
		allTasks={epic.proposed_tasks}
		onSave={handleSaveProposedTask}
	/>
{/if}

<!-- Delete confirmation dialog -->
{#if showDeleteConfirm}
	<div class="fixed inset-0 bg-black/60 flex items-center justify-center z-50" role="dialog">
		<div class="bg-background border rounded-xl p-6 max-w-md w-full mx-4 shadow-xl">
			<div class="flex items-center gap-3 mb-4">
				<div class="w-10 h-10 rounded-full bg-red-500/15 flex items-center justify-center shrink-0">
					<Trash2 class="w-5 h-5 text-red-400" />
				</div>
				<div>
					<h3 class="font-semibold">Delete Epic</h3>
					<p class="text-sm text-muted-foreground">This action cannot be undone.</p>
				</div>
			</div>
			<p class="text-sm text-muted-foreground mb-1">
				This will permanently delete the epic <strong class="text-foreground">{epic?.title}</strong> and delete all of its {epic?.task_ids?.length ?? 0} child task{(epic?.task_ids?.length ?? 0) !== 1 ? 's' : ''}.
			</p>
			<div class="flex items-center gap-2 justify-end mt-6">
				<Button variant="ghost" size="sm" onclick={() => (showDeleteConfirm = false)} disabled={deleting}>
					Cancel
				</Button>
				<Button variant="destructive" size="sm" onclick={handleDelete} disabled={deleting} class="gap-1.5">
					{#if deleting}
						<Loader2 class="w-3.5 h-3.5 animate-spin" />
						Deleting...
					{:else}
						<Trash2 class="w-3.5 h-3.5" />
						Delete Epic
					{/if}
				</Button>
			</div>
		</div>
	</div>
{/if}
