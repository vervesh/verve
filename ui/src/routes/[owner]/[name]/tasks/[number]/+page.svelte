<script lang="ts">
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { client } from '$lib/api-client';
	import type { Task, TaskStatus } from '$lib/models/task';
	import type { Epic } from '$lib/models/epic';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import * as Card from '$lib/components/ui/card';
	import * as Dialog from '$lib/components/ui/dialog';
	import { goto } from '$app/navigation';
	import { repoStore } from '$lib/stores/repos.svelte';
	import { taskStore } from '$lib/stores/tasks.svelte';
	import { taskUrl } from '$lib/utils';
	import { renderMarkdown } from '$lib/markdown';
	import EditTaskDialog from '$lib/components/EditTaskDialog.svelte';
	import {
		ArrowLeft,
		Clock,
		Play,
		Eye,
		GitMerge,
		CheckCircle,
		XCircle,
		FileText,
		GitPullRequest,
		GitBranch,
		Link2,
		Terminal,
		Sparkles,
		RefreshCw,
		X,
		Loader2,
		Target,
		DollarSign,
		AlertTriangle,
		ChevronDown,
		ExternalLink,
		CircleDot,
		MinusCircle,
		MessageSquare,
		Send,
		Timer,
		PauseCircle,
		PlayCircle,
		RotateCcw,
		Pencil,
		Trash2,
		StopCircle,
		Filter,
		Layers
	} from 'lucide-svelte';
	import type { ComponentType } from 'svelte';
	import type { Icon } from 'lucide-svelte';
	import { AnsiUp } from 'ansi_up';

	// Initial log lines that should always be shown (before any bracket-prefixed log appears)
	const INITIAL_LOG_MARKERS = [
		'=== Verve Agent Starting ===',
		'Task ID:',
		'Repository:',
		'Title:',
		'Description:'
	];

	function isInitialLogLine(line: string): boolean {
		return INITIAL_LOG_MARKERS.some((marker) => line.startsWith(marker));
	}

	function isClaudeLogLine(line: string): boolean {
		return line.trimStart().startsWith('[claude]');
	}

	function filterClaudeLogs(lines: string[]): string[] {
		const filtered: string[] = [];
		let pastInitialBlock = false;
		let inClaudeBlock = false;

		for (const line of lines) {
			if (!pastInitialBlock) {
				// Keep all lines until we see the first bracket-prefixed line
				if (/^\[[\w_-]+\]/.test(line.trimStart())) {
					pastInitialBlock = true;
				} else if (isInitialLogLine(line) || line.trim() === '') {
					filtered.push(line);
					continue;
				} else {
					// Non-marker, non-empty line before any bracket prefix — keep it as initial
					filtered.push(line);
					continue;
				}
			}

			if (pastInitialBlock) {
				if (isClaudeLogLine(line)) {
					inClaudeBlock = true;
					filtered.push(line);
				} else if (inClaudeBlock && !/^\[[\w_-]+\]/.test(line.trimStart())) {
					// Continuation of claude block (no bracket prefix)
					filtered.push(line);
				} else {
					inClaudeBlock = false;
				}
			}
		}
		return filtered;
	}

	function colorizeLineHtml(html: string): string {
		// Colorize a single line's HTML by only modifying text nodes, not tag attributes
		return html.replace(/([^<]+)|(<[^>]*>)/g, (match, text, tag) => {
			if (tag) return tag;
			return text
				// Bracketed prefixes: [agent], [error], [info], [warn], [system], etc.
				.replace(/\[([a-zA-Z_-]+)\]/g, '<span class=log-bracket>[$1]</span>')
				// Lines starting with $ or > (command prompts)
				.replace(/^([\$>]\s)(.*)$/, '<span class=log-prompt>$1</span><span class=log-cmd>$2</span>')
				// File paths (word with slashes and optional extension)
				.replace(/(?<!\w)((?:\.{0,2}\/)?(?:[\w.-]+\/)+[\w.-]+)(?!\w)/g, '<span class=log-path>$1</span>')
				// URLs
				.replace(/(https?:\/\/[^\s<]+)/g, '<span class=log-url>$1</span>')
				// Numbers (standalone)
				.replace(/(?<=\s|^|\(|:)(\d+\.?\d*)(?=\s|$|\)|,|;)/g, '<span class=log-num>$1</span>')
				// Quoted strings
				.replace(/("|')(.*?)(\1)/g, '<span class=log-str>$1$2$3</span>');
		});
	}

	function colorizeLogs(lines: string[], highlightClaude: boolean): string {
		// Process each log line individually through AnsiUp and colorization.
		// This prevents ANSI span tags from crossing line boundaries, which
		// would break the regex-based syntax highlighting for multiline output.
		// A fresh AnsiUp instance is created per render to avoid stale state
		// when the full log set is re-derived after new lines are appended.
		const ansi = new AnsiUp();
		let inClaudeBlock = false;
		const colorizedLines: string[] = [];

		for (const line of lines) {
			const lineHtml = colorizeLineHtml(ansi.ansi_to_html(line));

			if (highlightClaude) {
				// Detect [claude] lines and continuation lines (lines without a bracket prefix)
				const isClaudeLine = lineHtml.includes('<span class=log-bracket>[claude]</span>');
				const hasBracketPrefix = lineHtml.includes('<span class=log-bracket>');

				if (isClaudeLine) {
					inClaudeBlock = true;
					colorizedLines.push('<span class=log-claude-line>' + lineHtml + '</span>');
				} else if (inClaudeBlock && !hasBracketPrefix) {
					// Continuation of a claude block (no new bracket prefix)
					colorizedLines.push('<span class=log-claude-line>' + lineHtml + '</span>');
				} else {
					inClaudeBlock = false;
					colorizedLines.push(lineHtml);
				}
			} else {
				colorizedLines.push(lineHtml);
			}
		}

		return colorizedLines.join('\n');
	}

	let task = $state<Task | null>(null);
	let logsByAttempt = $state<Record<number, string[]>>({});
	let activeAttemptTab = $state(1);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let syncing = $state(false);
	let closing = $state(false);
	let showCloseForm = $state(false);
	let closeReason = $state('');
	let retrying = $state(false);
	let showRetryForm = $state(false);
	let retryInstructions = $state('');
	let sendingFeedback = $state(false);
	let showFeedbackForm = $state(false);
	let feedbackText = $state('');
	let togglingReady = $state(false);
	let movingToReview = $state(false);
	let startingOver = $state(false);
	let showStartOverForm = $state(false);
	let startOverTitle = $state('');
	let startOverDescription = $state('');
	let startOverCriteria = $state('');
	let removingDep = $state<string | null>(null);
	let showEditDialog = $state(false);
	let showDeleteDialog = $state(false);
	let deleting = $state(false);
	let stopping = $state(false);
	let logsContainer: HTMLDivElement | null = $state(null);
	let autoScroll = $state(true);
	let lastLogCount = $state(0);
	let claudeOnly = $state(false);
	let showRetryContext = $state(false);
	let epic = $state<Epic | null>(null);
	let depTaskNumbers = $state<Record<string, number>>({});
	let checkStatus = $state<{
		status: 'pending' | 'success' | 'failure' | 'error';
		summary?: string;
		failed_names?: string[];
		check_runs_skipped?: boolean;
		checks?: { name: string; status: string; conclusion: string; url: string }[];
	} | null>(null);
	let checkStatusLoading = $state(false);
	let checkPollTimer = $state<ReturnType<typeof setTimeout> | null>(null);
	let forceCheckPolls = $state(0);

	interface AgentStatusParsed {
		files_modified?: string[];
		tests_status?: string;
		confidence?: string;
		blockers?: string[];
		criteria_met?: string[];
		notes?: string;
	}

	const parsedAgentStatus = $derived.by(() => {
		if (!task?.agent_status) return null;
		try {
			return JSON.parse(task.agent_status) as AgentStatusParsed;
		} catch {
			return null;
		}
	});

	const confidenceColor = $derived.by(() => {
		if (!parsedAgentStatus?.confidence) return '';
		switch (parsedAgentStatus.confidence) {
			case 'high':
				return 'bg-green-500/20 text-green-400';
			case 'medium':
				return 'bg-amber-500/20 text-amber-400';
			case 'low':
				return 'bg-red-500/20 text-red-400';
			default:
				return 'bg-gray-500/20 text-gray-400';
		}
	});

	function formatCost(cost: number): string {
		return `$${cost.toFixed(2)}`;
	}

	const ownerParam = $derived($page.params.owner as string);
	const nameParam = $derived($page.params.name as string);
	const numberParam = $derived(Number($page.params.number));
	const taskId = $derived(task?.id ?? '');

	// Resolve the repo from the store by owner/name
	const repo = $derived(repoStore.repos.find((r) => r.owner === ownerParam && r.name === nameParam) ?? null);

	const statusConfig: Record<
		TaskStatus,
		{ label: string; icon: ComponentType<Icon>; bgClass: string; textClass: string }
	> = {
		pending: {
			label: 'Pending',
			icon: Clock,
			bgClass: 'bg-amber-500/20 text-amber-400',
			textClass: 'text-amber-600 dark:text-amber-400'
		},
		running: {
			label: 'Running',
			icon: Play,
			bgClass: 'bg-blue-500/20 text-blue-400',
			textClass: 'text-blue-600 dark:text-blue-400'
		},
		review: {
			label: 'In Review',
			icon: Eye,
			bgClass: 'bg-purple-500/20 text-purple-400',
			textClass: 'text-purple-600 dark:text-purple-400'
		},
		merged: {
			label: 'Merged',
			icon: GitMerge,
			bgClass: 'bg-green-500/20 text-green-400',
			textClass: 'text-green-600 dark:text-green-400'
		},
		closed: {
			label: 'Closed',
			icon: CheckCircle,
			bgClass: 'bg-gray-500/20 text-gray-400',
			textClass: 'text-gray-600 dark:text-gray-400'
		},
		failed: {
			label: 'Failed',
			icon: XCircle,
			bgClass: 'bg-red-500/20 text-red-400',
			textClass: 'text-red-600 dark:text-red-400'
		}
	};

	const branchURL = $derived.by(() => {
		if (!task?.branch_name) return null;
		const r = repoStore.repos.find((r) => r.id === task!.repo_id);
		if (!r) return null;
		return `https://github.com/${r.full_name}/tree/${task.branch_name}`;
	});

	const isStopped = $derived(task && !task.ready && task.status === 'pending' && task.close_reason === 'Stopped by user');
	const canStop = $derived(task?.status === 'running');
	const canClose = $derived(task && !['closed', 'merged', 'failed'].includes(task.status));
	const canStartOver = $derived(task?.status === 'review' || task?.status === 'failed' || task?.status === 'closed');
	const canRetry = $derived(task?.status === 'failed');
	const canProvideFeedback = $derived(task?.status === 'review');
	const isRetrying = $derived(task?.pull_request_url && (task?.status === 'running' || task?.status === 'pending'));

	const currentStatusConfig = $derived(task ? statusConfig[task.status] : null);
	const StatusIcon = $derived(currentStatusConfig?.icon ?? Clock);

	// Render description as markdown
	const renderedDescription = $derived(task && task.description.trim() ? renderMarkdown(task.description) : '');

	// Per-attempt log tracking
	const logs = $derived(logsByAttempt[activeAttemptTab] ?? []);
	const displayLogs = $derived(claudeOnly ? filterClaudeLogs(logs) : logs);
	// When claudeOnly filter is active, don't highlight claude lines (they're all claude).
	// When filter is off, highlight claude lines to make them stand out.
	const renderedLogs = $derived(displayLogs.length > 0 ? colorizeLogs(displayLogs, !claudeOnly) : '');
	const attemptNumbers = $derived.by(() => {
		const nums = new Set(Object.keys(logsByAttempt).map(Number));
		if (task) {
			for (let i = 1; i <= task.attempt; i++) nums.add(i);
		}
		return [...nums].sort((a, b) => a - b);
	});
	const showAttemptTabs = $derived(attemptNumbers.length > 1);

	function switchAttemptTab(attempt: number) {
		activeAttemptTab = attempt;
		lastLogCount = 0;
		autoScroll = true;
		requestAnimationFrame(() => {
			if (logsContainer) logsContainer.scrollTop = logsContainer.scrollHeight;
		});
	}

	// Auto-scroll logs when new logs arrive
	$effect(() => {
		if (logs.length > lastLogCount) {
			lastLogCount = logs.length;
			if (autoScroll && logsContainer) {
				requestAnimationFrame(() => {
					if (logsContainer) {
						logsContainer.scrollTop = logsContainer.scrollHeight;
					}
				});
			}
		}
	});

	function handleLogsScroll(e: Event) {
		const el = e.target as HTMLDivElement;
		// Check if user is near bottom (within 50px)
		const isNearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 50;
		autoScroll = isNearBottom;
	}

	// Prevent scroll events from propagating to parent when at scroll boundaries
	function handleLogsWheel(e: WheelEvent) {
		const el = e.currentTarget as HTMLDivElement;
		const atTop = el.scrollTop <= 0;
		const atBottom = el.scrollTop + el.clientHeight >= el.scrollHeight - 1;

		// If scrolling up at top or scrolling down at bottom, prevent parent scroll
		if ((e.deltaY < 0 && atTop) || (e.deltaY > 0 && atBottom)) {
			e.preventDefault();
		}
	}

	let es: EventSource | null = null;
	let logsES: EventSource | null = null;

	let taskLoaded = $state(false);

	// Use $effect to wait for repo store to be populated before loading.
	// The layout loads repos asynchronously, so repo may be null on first render.
	$effect(() => {
		if (repo && !taskLoaded) {
			taskLoaded = true;
			loadTask();
		}
	});

	onMount(() => {
		return () => {
			es?.close();
			logsES?.close();
			stopCheckPolling();
		};
	});

	function connectSSE(resolvedTaskId: string) {
		// Task metadata updates via global SSE.
		es = new EventSource(client.eventsURL());

		es.addEventListener('task_updated', (e) => {
			const event = JSON.parse(e.data);
			if (event.task?.id === resolvedTaskId && task) {
				const prev = task.status;
				const updated = { ...event.task, logs: task.logs };
				task = updated;
				// Refresh check status when task enters review
				if (updated.status === 'review' && updated.pr_number && prev !== 'review') {
					checkStatus = null;
					stopCheckPolling();
					forceCheckPolls = 3;
					// Delay first fetch to let GitHub process the new commit
					checkPollTimer = setTimeout(loadCheckStatus, 5000);
				}
			}
		});

		// Log streaming via dedicated SSE endpoint.
		// Uses double-buffering so reconnects replace logs without flashing.
		// Logs are grouped by attempt number for tabbed display.
		let logBufferMap: Record<number, string[]> = {};
		let historicalDone = false;

		logsES = new EventSource(client.taskLogsURL(resolvedTaskId));

		logsES.addEventListener('open', () => {
			logBufferMap = {};
			historicalDone = false;
		});

		logsES.addEventListener('logs_appended', (e) => {
			const event = JSON.parse(e.data);
			const attempt: number = event.attempt || 1;
			if (historicalDone) {
				logsByAttempt[attempt] = [...(logsByAttempt[attempt] ?? []), ...event.logs];
				// Auto-switch to latest attempt on first log of new attempt
				if (attempt > activeAttemptTab) {
					activeAttemptTab = attempt;
					lastLogCount = 0;
				}
			} else {
				logBufferMap[attempt] = [...(logBufferMap[attempt] ?? []), ...event.logs];
			}
		});

		logsES.addEventListener('logs_done', () => {
			logsByAttempt = logBufferMap;
			logBufferMap = {};
			historicalDone = true;
			// Default to latest attempt and auto-scroll to bottom
			const keys = Object.keys(logsByAttempt).map(Number);
			if (keys.length > 0) {
				activeAttemptTab = Math.max(...keys);
			}
			lastLogCount = 0;
			autoScroll = true;
		});
	}

	async function loadTask() {
		try {
			if (!repo) {
				error = 'Repository not found';
				loading = false;
				return;
			}
			task = await client.getTaskByNumber(repo.id, numberParam);
			error = null;
			connectSSE(task.id);
			if (task.status === 'review' && task.pr_number) {
				loadCheckStatus();
			}
			if (task.epic_id) {
				loadEpic(task.epic_id);
			}
			if (task.depends_on && task.depends_on.length > 0) {
				loadDepNumbers(task.depends_on);
			}
		} catch (e) {
			error = (e as Error).message;
		} finally {
			loading = false;
		}
	}

	async function loadEpic(epicId: string) {
		try {
			epic = await client.getEpic(epicId);
		} catch {
			// Epic may have been deleted; ignore errors
			epic = null;
		}
	}

	async function loadDepNumbers(depIds: string[]) {
		const numbers: Record<string, number> = {};
		for (const depId of depIds) {
			try {
				const depTask = await client.getTask(depId);
				numbers[depId] = depTask.number;
			} catch {
				// Dep may have been deleted; leave out
			}
		}
		depTaskNumbers = numbers;
	}

	function stopCheckPolling() {
		if (checkPollTimer) {
			clearTimeout(checkPollTimer);
			checkPollTimer = null;
		}
	}

	async function loadCheckStatus() {
		if (!task) return;
		checkStatusLoading = true;
		stopCheckPolling();
		try {
			checkStatus = await client.getTaskChecks(task.id);
			// Keep polling while checks are pending, or during forced
			// polls after a status transition (handles stale GitHub data).
			const shouldPoll =
				checkStatus.status === 'pending' || forceCheckPolls > 0;
			if (forceCheckPolls > 0) forceCheckPolls--;
			if (shouldPoll && task?.status === 'review') {
				checkPollTimer = setTimeout(loadCheckStatus, 10000);
			}
		} catch {
			checkStatus = { status: 'error', summary: 'Failed to fetch check status' };
		} finally {
			checkStatusLoading = false;
		}
	}

	async function syncStatus() {
		if (!task || syncing) return;
		syncing = true;
		try {
			task = await client.syncTask(task.id);
		} catch (e) {
			error = (e as Error).message;
		} finally {
			syncing = false;
		}
	}

	async function handleStop() {
		if (!task || stopping) return;
		stopping = true;
		try {
			task = await client.stopTask(task.id);
		} catch (e) {
			error = (e as Error).message;
		} finally {
			stopping = false;
		}
	}

	async function handleClose() {
		if (!task || closing) return;
		closing = true;
		try {
			task = await client.closeTask(task.id, closeReason || undefined);
			showCloseForm = false;
			closeReason = '';
		} catch (e) {
			error = (e as Error).message;
		} finally {
			closing = false;
		}
	}

	async function handleRetry() {
		if (!task || retrying) return;
		retrying = true;
		try {
			task = await client.retryTask(task.id, retryInstructions || undefined);
			showRetryForm = false;
			retryInstructions = '';
		} catch (e) {
			error = (e as Error).message;
		} finally {
			retrying = false;
		}
	}

	async function handleFeedback() {
		if (!task || sendingFeedback || !feedbackText.trim()) return;
		sendingFeedback = true;
		try {
			task = await client.feedbackTask(task.id, feedbackText.trim());
			showFeedbackForm = false;
			feedbackText = '';
		} catch (e) {
			error = (e as Error).message;
		} finally {
			sendingFeedback = false;
		}
	}

	async function handleMoveToReview() {
		if (!task || movingToReview) return;
		movingToReview = true;
		try {
			task = await client.moveToReview(task.id);
		} catch (e) {
			error = (e as Error).message;
		} finally {
			movingToReview = false;
		}
	}

	async function handleRemoveDependency(depId: string) {
		if (!task || removingDep) return;
		removingDep = depId;
		try {
			task = await client.removeDependency(task.id, depId);
		} catch (e) {
			error = (e as Error).message;
		} finally {
			removingDep = null;
		}
	}

	const canEdit = $derived(task?.status === 'pending');

	function openEditDialog() {
		if (!task) return;
		showCloseForm = false;
		showStartOverForm = false;
		showEditDialog = true;
	}

	function handleEditUpdated(updated: Task) {
		task = updated;
	}

	async function handleToggleReady() {
		if (!task || togglingReady) return;
		togglingReady = true;
		try {
			task = await client.setReady(task.id, !task.ready);
		} catch (e) {
			error = (e as Error).message;
		} finally {
			togglingReady = false;
		}
	}

	function openStartOverForm() {
		if (!task) return;
		startOverTitle = task.title;
		startOverDescription = task.description;
		startOverCriteria = (task.acceptance_criteria ?? []).join('\n');
		showStartOverForm = true;
		showCloseForm = false;
	}

	async function handleStartOver() {
		if (!task || startingOver) return;
		startingOver = true;
		try {
			const updates: { title?: string; description?: string; acceptance_criteria?: string[] } = {};
			if (startOverTitle !== task.title) updates.title = startOverTitle;
			if (startOverDescription !== task.description) updates.description = startOverDescription;
			const newCriteria = startOverCriteria.split('\n').map((s) => s.trim()).filter(Boolean);
			const oldCriteria = task.acceptance_criteria ?? [];
			if (JSON.stringify(newCriteria) !== JSON.stringify(oldCriteria)) updates.acceptance_criteria = newCriteria;
			task = await client.startOverTask(task.id, Object.keys(updates).length > 0 ? updates : undefined);
			showStartOverForm = false;
			logsByAttempt = {};
			activeAttemptTab = 1;
			lastLogCount = 0;
		} catch (e) {
			error = (e as Error).message;
		} finally {
			startingOver = false;
		}
	}

	function formatDuration(ms: number): string {
		const seconds = Math.floor(ms / 1000);
		if (seconds < 60) return `${seconds}s`;
		const minutes = Math.floor(seconds / 60);
		const remainSeconds = seconds % 60;
		if (minutes < 60) return `${minutes}m ${remainSeconds}s`;
		const hours = Math.floor(minutes / 60);
		const remainMinutes = minutes % 60;
		return `${hours}h ${remainMinutes}m`;
	}

	function formatDate(dateStr: string): string {
		const d = new Date(dateStr);
		return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' }) +
			' ' + d.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' });
	}

	function formatRelativeTime(dateStr: string): string {
		const date = new Date(dateStr);
		const now = new Date();
		const diffMs = now.getTime() - date.getTime();
		const diffMins = Math.floor(diffMs / 60000);
		const diffHours = Math.floor(diffMs / 3600000);
		const diffDays = Math.floor(diffMs / 86400000);

		if (diffMins < 1) return 'Just now';
		if (diffMins < 60) return `${diffMins}m ago`;
		if (diffHours < 24) return `${diffHours}h ago`;
		return `${diffDays}d ago`;
	}

	async function handleDelete() {
		if (!task || deleting) return;
		deleting = true;
		try {
			await client.deleteTask(task.id);
			showDeleteDialog = false;
			await goto('/');
		} catch (e) {
			error = (e as Error).message;
		} finally {
			deleting = false;
		}
	}
</script>

<div class="p-4 sm:p-6">
	<Button variant="ghost" onclick={() => goto('/')} class="mb-4 sm:mb-6 gap-2 -ml-2">
		<ArrowLeft class="w-4 h-4" />
		<span class="hidden sm:inline">Back to Tasks</span>
		<span class="sm:hidden">Back</span>
	</Button>

	{#if loading}
		<div class="flex flex-col items-center justify-center py-16">
			<Loader2 class="w-8 h-8 animate-spin text-primary mb-4" />
			<p class="text-muted-foreground">Loading task...</p>
		</div>
	{:else if error && !task}
		<div
			class="bg-destructive/10 text-destructive p-4 rounded-lg flex items-center gap-3 border border-destructive/20"
		>
			<XCircle class="w-5 h-5 flex-shrink-0" />
			<span>{error}</span>
		</div>
	{:else if task}
		<!-- Header: Title + Metadata (full width) -->
		<div class="space-y-4 mb-6">
			{#if task.title}
				<h1 class="text-xl sm:text-2xl font-semibold">{task.title}</h1>
			{/if}

			<div class="flex items-center gap-2 sm:gap-3 flex-wrap pb-4 sm:pb-5 border-b">
				<span class="font-mono text-xs sm:text-sm text-muted-foreground bg-muted px-2 py-0.5 rounded">
					#{task.number}
				</span>
				<Badge class="{currentStatusConfig?.bgClass} gap-1">
					<StatusIcon class="w-3 h-3" />
					{currentStatusConfig?.label}
				</Badge>
				{#if epic}
					<button
						class="inline-flex items-center gap-1.5 text-xs bg-indigo-500/15 text-indigo-600 dark:text-indigo-400 px-2.5 py-1 rounded-md hover:bg-indigo-500/25 transition-colors cursor-pointer border border-indigo-500/20"
						onclick={() => goto(`/epics/${epic!.id}`)}
						title="Part of epic: {epic!.title}"
					>
						<Layers class="w-3 h-3" />
						<span class="max-w-[200px] truncate">{epic!.title}</span>
					</button>
				{/if}
				{#if task.duration_ms}
					<span class="text-xs text-muted-foreground flex items-center gap-1.5">
						<Timer class="w-3.5 h-3.5" />
						{formatDuration(task.duration_ms)}
					</span>
				{/if}
				{#if task.cost_usd > 0}
					<span class="text-xs text-muted-foreground flex items-center gap-1.5">
						<DollarSign class="w-3.5 h-3.5" />
						{formatCost(task.cost_usd)}
						{#if task.max_cost_usd}
							<span class="text-muted-foreground/60">/ {formatCost(task.max_cost_usd)}</span>
						{/if}
					</span>
				{/if}

				<div class="ml-auto flex items-center gap-2 flex-wrap">
					<span class="text-xs text-muted-foreground bg-muted px-2 py-0.5 rounded">
						<span class="text-muted-foreground/60">Created</span> {formatDate(task.created_at)}
					</span>
					<span class="text-xs text-muted-foreground bg-muted px-2 py-0.5 rounded">
						<span class="text-muted-foreground/60">Updated</span> {formatDate(task.updated_at)}
					</span>
					{#if canEdit}
						<Button size="sm" variant="outline" onclick={openEditDialog} class="gap-1">
							<Pencil class="w-4 h-4" />
							<span class="hidden sm:inline">Edit</span>
						</Button>
					{/if}
					{#if task.ready && task.status === 'pending'}
						<Button
							size="sm"
							variant="outline"
							onclick={handleToggleReady}
							disabled={togglingReady}
							class="gap-1"
							title="Mark this task as not ready so agents won't pick it up"
						>
							{#if togglingReady}
								<Loader2 class="w-4 h-4 animate-spin" />
							{:else}
								<PauseCircle class="w-4 h-4" />
							{/if}
							<span class="hidden sm:inline">Mark Not Ready</span>
						</Button>
					{/if}
					{#if canStartOver}
						{#if showStartOverForm}
							<Button size="sm" variant="ghost" onclick={() => (showStartOverForm = false)} class="gap-1">
								<X class="w-4 h-4" />
								Cancel
							</Button>
						{:else}
							<Button size="sm" variant="outline" onclick={openStartOverForm} class="gap-1">
								<RotateCcw class="w-4 h-4" />
								Start Over
							</Button>
						{/if}
					{/if}
					{#if canClose}
						{#if showCloseForm}
							<Button size="sm" variant="ghost" onclick={() => (showCloseForm = false)} class="gap-1">
								<X class="w-4 h-4" />
								Cancel
							</Button>
						{:else}
							<Button size="sm" variant="outline" onclick={() => (showCloseForm = true)} class="gap-1">
								<XCircle class="w-4 h-4" />
								<span class="hidden sm:inline">Close Task</span>
								<span class="sm:hidden">Close</span>
							</Button>
						{/if}
					{/if}
					<Button size="sm" variant="destructive" onclick={() => (showDeleteDialog = true)} class="gap-1">
						<Trash2 class="w-4 h-4" />
						<span class="hidden sm:inline">Delete</span>
					</Button>
				</div>
			</div>

			<!-- Close Form (full width, above columns) -->
			{#if showCloseForm}
				<Card.Root class="border-destructive/30 bg-destructive/5">
					<Card.Header class="pb-0 gap-0">
						<Card.Title class="text-base flex items-center gap-2">
							<XCircle class="w-4 h-4 text-destructive" />
							Close Task
						</Card.Title>
					</Card.Header>
					<Card.Content>
						<div class="space-y-4">
							<div>
								<label for="close-reason" class="text-sm font-medium mb-2 block">
									Reason (optional)
								</label>
								<textarea
									id="close-reason"
									bind:value={closeReason}
									class="w-full border rounded-lg p-3 min-h-[80px] bg-background text-foreground resize-none focus:outline-none focus:ring-2 focus:ring-ring"
									placeholder="Why is this task being closed?"
									disabled={closing}
								></textarea>
							</div>
							<div class="flex justify-end gap-2">
								<Button variant="outline" onclick={() => (showCloseForm = false)} disabled={closing}>
									Cancel
								</Button>
								<Button variant="destructive" onclick={handleClose} disabled={closing} class="gap-2">
									{#if closing}
										<Loader2 class="w-4 h-4 animate-spin" />
										Closing...
									{:else}
										<XCircle class="w-4 h-4" />
										Close Task
									{/if}
								</Button>
							</div>
						</div>
					</Card.Content>
				</Card.Root>
			{/if}

			<!-- Start Over Form (full width, above columns) -->
			{#if showStartOverForm}
				<Card.Root class="border-amber-500/30 bg-amber-500/5">
					<Card.Header class="pb-0 gap-0">
						<Card.Title class="text-base flex items-center gap-2">
							<RotateCcw class="w-4 h-4 text-amber-500" />
							Start Over
						</Card.Title>
						<p class="text-sm text-muted-foreground mt-1">
							This will clear all logs, agent data, cost, and close the PR if one exists. The task will be placed back to pending for a fresh attempt.
						</p>
					</Card.Header>
					<Card.Content>
						<div class="space-y-4">
							<div>
								<label for="start-over-title" class="text-sm font-medium mb-2 block">
									Title
								</label>
								<input
									id="start-over-title"
									type="text"
									bind:value={startOverTitle}
									class="w-full border rounded-lg p-3 bg-background text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
									placeholder="Task title"
									disabled={startingOver}
									maxlength={150}
								/>
							</div>
							<div>
								<label for="start-over-description" class="text-sm font-medium mb-2 block">
									Description
								</label>
								<textarea
									id="start-over-description"
									bind:value={startOverDescription}
									class="w-full border rounded-lg p-3 min-h-[120px] bg-background text-foreground resize-none focus:outline-none focus:ring-2 focus:ring-ring"
									placeholder="Describe what the agent should do..."
									disabled={startingOver}
								></textarea>
							</div>
							<div>
								<label for="start-over-criteria" class="text-sm font-medium mb-2 block">
									Acceptance Criteria <span class="text-muted-foreground font-normal">(one per line)</span>
								</label>
								<textarea
									id="start-over-criteria"
									bind:value={startOverCriteria}
									class="w-full border rounded-lg p-3 min-h-[80px] bg-background text-foreground resize-none focus:outline-none focus:ring-2 focus:ring-ring"
									placeholder="Each line becomes a criterion..."
									disabled={startingOver}
								></textarea>
							</div>
							<div class="flex justify-end gap-2">
								<Button variant="outline" onclick={() => (showStartOverForm = false)} disabled={startingOver}>
									Cancel
								</Button>
								<Button onclick={handleStartOver} disabled={startingOver || !startOverTitle.trim()} class="gap-2 bg-amber-600 hover:bg-amber-700 text-white">
									{#if startingOver}
										<Loader2 class="w-4 h-4 animate-spin" />
										Restarting...
									{:else}
										<RotateCcw class="w-4 h-4" />
										Confirm Start Over
									{/if}
								</Button>
							</div>
						</div>
					</Card.Content>
				</Card.Root>
			{/if}

			</div>

		<!-- Two-column layout -->
		<div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
			<!-- Left column: Task details -->
			<div class="space-y-6">
				<!-- Stopped Banner -->
				{#if isStopped}
					<div class="rounded-lg border border-red-500/30 bg-red-500/5 px-5 py-4 flex items-center justify-between gap-4">
						<div class="space-y-1.5">
							<div class="flex items-center gap-2.5">
								<StopCircle class="w-5 h-5 text-red-500 shrink-0" />
								<span class="text-sm font-medium text-red-600 dark:text-red-400">Stopped</span>
							</div>
							<p class="text-xs text-muted-foreground">This task was manually stopped. The agent has been interrupted. Click retry to run the task again.</p>
						</div>
						<Button
							size="sm"
							onclick={handleToggleReady}
							disabled={togglingReady}
							class="gap-1.5 shrink-0 bg-green-700 hover:bg-green-800 dark:bg-green-800 dark:hover:bg-green-900 text-white"
						>
							{#if togglingReady}
								<Loader2 class="w-4 h-4 animate-spin" />
								Retrying...
							{:else}
								<RefreshCw class="w-4 h-4" />
								Retry
							{/if}
						</Button>
					</div>
				<!-- Not Ready Banner -->
				{:else if !task.ready && task.status === 'pending'}
					<div class="rounded-lg border border-orange-500/30 bg-orange-500/5 px-5 py-4 flex items-center justify-between gap-4">
						<div class="space-y-1.5">
							<div class="flex items-center gap-2.5">
								<PauseCircle class="w-5 h-5 text-orange-500 shrink-0" />
								<span class="text-sm font-medium text-orange-600 dark:text-orange-400">Not Ready</span>
							</div>
							<p class="text-xs text-muted-foreground">This task is paused for tracking only. Agents will not pick it up until it is marked as ready.</p>
						</div>
						<Button
							size="sm"
							onclick={handleToggleReady}
							disabled={togglingReady}
							class="gap-1.5 shrink-0 bg-green-700 hover:bg-green-800 dark:bg-green-800 dark:hover:bg-green-900 text-white"
						>
							{#if togglingReady}
								<Loader2 class="w-4 h-4 animate-spin" />
								Updating...
							{:else}
								<PlayCircle class="w-4 h-4" />
								Mark as Ready
							{/if}
						</Button>
					</div>
				{/if}
				<!-- Task Details (unified card) -->
				<div class="rounded-xl border bg-card shadow-sm overflow-hidden">
					<!-- Header -->
					<div class="flex items-center gap-2 px-5 py-3 border-b">
						<FileText class="w-4 h-4 text-muted-foreground" />
						<span class="font-semibold text-sm">Task Details</span>
					</div>

					<!-- Description -->
					<div class="px-5 py-4 {(task.acceptance_criteria && task.acceptance_criteria.length > 0) || (task.depends_on && task.depends_on.length > 0) ? 'border-b' : ''}">
						{#if renderedDescription}
							<div class="max-h-96 overflow-y-auto">
								<div class="prose prose-sm dark:prose-invert max-w-none">
									{@html renderedDescription}
								</div>
							</div>
						{:else}
							<p class="text-sm text-muted-foreground italic">No description provided</p>
						{/if}
					</div>

					<!-- Acceptance Criteria -->
					{#if task.acceptance_criteria && task.acceptance_criteria.length > 0}
						<div class="px-5 py-4 {task.depends_on && task.depends_on.length > 0 ? 'border-b' : ''}">
							<div class="flex items-center gap-2 mb-3">
								<Target class="w-3.5 h-3.5 text-muted-foreground" />
								<span class="text-sm font-medium">Acceptance Criteria</span>
								<span class="text-xs text-muted-foreground">
									({task.acceptance_criteria.length})
								</span>
							</div>
							<ol class="space-y-2 max-h-48 overflow-y-auto overscroll-contain">
								{#each task.acceptance_criteria as criterion, i}
									<li class="flex items-start gap-2.5 text-sm">
										<span class="text-xs text-muted-foreground font-mono mt-0.5 w-5 shrink-0 text-right">{i + 1}.</span>
										<span class="text-foreground/80">{criterion}</span>
									</li>
								{/each}
							</ol>
						</div>
					{/if}

					<!-- Dependencies -->
					{#if task.depends_on && task.depends_on.length > 0}
						<div class="px-5 py-4">
							<div class="flex items-center gap-2 mb-3">
								<Link2 class="w-3.5 h-3.5 text-muted-foreground" />
								<span class="text-sm font-medium">Dependencies</span>
								<span class="text-xs text-muted-foreground">
									({task.depends_on.length})
								</span>
							</div>
							<div class="flex flex-wrap gap-2">
								{#each task.depends_on as depId}
									<div class="inline-flex items-center rounded-md bg-muted text-sm font-mono transition-colors">
										<button
											class="inline-flex items-center gap-1 px-3 py-1.5 hover:bg-accent rounded-l-md transition-colors"
											onclick={() => {
												const depNum = depTaskNumbers[depId];
												if (depNum && repo) {
													goto(taskUrl(ownerParam, nameParam, depNum));
												}
											}}
										>
											<Link2 class="w-3 h-3" />
											{depTaskNumbers[depId] ? `#${depTaskNumbers[depId]}` : depId.slice(0, 12) + '...'}
										</button>
										<button
											class="inline-flex items-center px-1.5 py-1.5 hover:bg-destructive/20 hover:text-destructive rounded-r-md transition-colors border-l border-border"
											onclick={() => handleRemoveDependency(depId)}
											disabled={removingDep === depId}
											title="Remove dependency"
										>
											{#if removingDep === depId}
												<Loader2 class="w-3 h-3 animate-spin" />
											{:else}
												<X class="w-3 h-3" />
											{/if}
										</button>
									</div>
								{/each}
							</div>
						</div>
					{/if}
				</div>

				<!-- Pull Request -->
				{#if task.pull_request_url}
					<div class="rounded-xl border shadow-sm overflow-hidden {task.status === 'merged' ? 'border-green-500/30 bg-green-500/10' : task.status === 'closed' || task.status === 'failed' ? 'border-gray-500/30 bg-gray-500/5' : isRetrying ? 'border-blue-500/30 bg-blue-500/[0.08]' : 'border-purple-500/30 bg-purple-500/[0.08]'}">
						<!-- Header -->
						<div class="flex items-center gap-2 px-5 py-3 {task.status === 'review' || isRetrying ? 'border-b' : ''}">
							{#if task.status === 'merged'}
								<GitMerge class="w-4 h-4 text-green-500" />
								<span class="font-semibold text-sm">Pull Request (Merged)</span>
							{:else if task.status === 'closed' || task.status === 'failed'}
								<GitPullRequest class="w-4 h-4 text-gray-500" />
								<span class="font-semibold text-sm">Pull Request (Closed)</span>
							{:else if isRetrying}
								<GitPullRequest class="w-4 h-4 text-blue-500" />
								<span class="font-semibold text-sm">Pull Request (Updating)</span>
							{:else}
								<GitPullRequest class="w-4 h-4 text-purple-500" />
								<span class="font-semibold text-sm">Pull Request</span>
							{/if}
							<div class="ml-auto flex items-center gap-2">
								<a
									href={task.pull_request_url}
									class="text-primary hover:underline font-medium flex items-center gap-2 text-sm"
									target="_blank"
									rel="noopener noreferrer"
								>
									<GitPullRequest class="w-4 h-4" />
									PR #{task.pr_number}
								</a>
								{#if task.status === 'review'}
									<Button
										size="sm"
										variant="ghost"
										class="h-7 w-7 p-0 text-muted-foreground"
										onclick={() => { syncStatus(); loadCheckStatus(); }}
										disabled={checkStatusLoading || syncing}
										title="Sync PR status and refresh checks"
									>
										<RefreshCw class="w-3.5 h-3.5 {checkStatusLoading || syncing ? 'animate-spin' : ''}" />
									</Button>
								{/if}
							</div>
						</div>
						<!-- Active Retry Note -->
						{#if isRetrying}
							<div class="px-5 py-3 flex items-center gap-3 bg-blue-500/5">
								<Loader2 class="w-4 h-4 animate-spin text-blue-500 shrink-0" />
								<span class="text-sm text-blue-600 dark:text-blue-400">
									{task.status === 'running' ? 'Agent is working on changes — the PR will be updated soon.' : 'Waiting for agent to pick up the task — the PR will be updated soon.'}
								</span>
							</div>
						{/if}
						<!-- CI Checks -->
						{#if task.status === 'review'}
							<div class="px-5 py-3 space-y-2">
								<div class="flex items-center gap-2">
									{#if checkStatusLoading && !checkStatus}
										<Loader2 class="w-3.5 h-3.5 animate-spin text-muted-foreground" />
										<span class="text-sm text-muted-foreground">Checking CI status...</span>
									{:else if checkStatus?.check_runs_skipped}
										<AlertTriangle class="w-3.5 h-3.5 text-amber-500" />
										<span class="text-sm text-muted-foreground">CI checks skipped — fine-grained tokens do not support this. Use a classic token for CI visibility.</span>
									{:else if checkStatus?.status === 'success'}
										<CheckCircle class="w-3.5 h-3.5 text-green-600 dark:text-green-400" />
										<span class="text-sm text-green-600 dark:text-green-400">All checks passed</span>
									{:else if checkStatus?.status === 'pending'}
										<Loader2 class="w-3.5 h-3.5 animate-spin text-amber-600 dark:text-amber-400" />
										<span class="text-sm text-amber-600 dark:text-amber-400">Checks in progress</span>
									{:else if checkStatus?.status === 'failure'}
										<XCircle class="w-3.5 h-3.5 text-red-600 dark:text-red-400" />
										<span class="text-sm text-red-600 dark:text-red-400">Checks failed</span>
									{:else if checkStatus?.status === 'error'}
										<AlertTriangle class="w-3.5 h-3.5 text-amber-500" />
										<span class="text-sm text-muted-foreground">{checkStatus.summary}</span>
									{/if}
								</div>
								{#if checkStatus?.checks && checkStatus.checks.length > 0}
									<div class="space-y-1">
										{#each checkStatus.checks as check}
											<div class="flex items-center gap-2 text-sm pl-1">
												{#if check.status !== 'completed'}
													<Loader2 class="w-3.5 h-3.5 animate-spin text-amber-600 dark:text-amber-400 shrink-0" />
												{:else if check.conclusion === 'success'}
													<CheckCircle class="w-3.5 h-3.5 text-green-600 dark:text-green-400 shrink-0" />
												{:else if check.conclusion === 'failure'}
													<XCircle class="w-3.5 h-3.5 text-red-600 dark:text-red-400 shrink-0" />
												{:else if check.conclusion === 'skipped'}
													<MinusCircle class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
												{:else if check.conclusion === 'cancelled'}
													<XCircle class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
												{:else}
													<CircleDot class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
												{/if}
												{#if check.url}
													<a
														href={check.url}
														class="text-muted-foreground hover:text-foreground hover:underline truncate flex items-center gap-1"
														target="_blank"
														rel="noopener noreferrer"
													>
														{check.name}
														<ExternalLink class="w-3 h-3 shrink-0 opacity-50" />
													</a>
												{:else}
													<span class="text-muted-foreground truncate">{check.name}</span>
												{/if}
											</div>
										{/each}
									</div>
								{/if}
							</div>
							<!-- Request Agent Changes -->
							{#if canProvideFeedback}
								<div class="px-5 py-3 border-t">
									{#if showFeedbackForm}
										<div class="space-y-3">
											<label for="feedback-text" class="text-sm font-medium block">
												Describe what you'd like changed
											</label>
											<textarea
												id="feedback-text"
												bind:value={feedbackText}
												class="w-full border rounded-lg p-3 min-h-[100px] bg-background text-foreground resize-none focus:outline-none focus:ring-2 focus:ring-purple-500/40"
												placeholder="e.g. &quot;Add error handling to the new endpoint&quot; or &quot;Use a map instead of a slice for lookups&quot;..."
												disabled={sendingFeedback}
											></textarea>
											<div class="flex justify-end gap-2">
												<Button variant="outline" size="sm" onclick={() => (showFeedbackForm = false)} disabled={sendingFeedback}>
													Cancel
												</Button>
												<Button size="sm" onclick={handleFeedback} disabled={sendingFeedback || !feedbackText.trim()} class="gap-2 bg-purple-600 hover:bg-purple-700 text-white">
													{#if sendingFeedback}
														<Loader2 class="w-4 h-4 animate-spin" />
														Sending...
													{:else}
														<Send class="w-4 h-4" />
														Send to Agent
													{/if}
												</Button>
											</div>
										</div>
									{:else}
										<div class="flex items-center gap-3">
											<Button size="sm" variant="outline" onclick={() => (showFeedbackForm = true)} class="gap-2 border-purple-500/40 text-purple-600 dark:text-purple-400 hover:bg-purple-500/10">
												<MessageSquare class="w-4 h-4" />
												Request Agent Changes
											</Button>
											<span class="text-xs text-muted-foreground">The agent will update this branch and PR based on your instructions.</span>
										</div>
									{/if}
								</div>
							{/if}
						{/if}
						<!-- Move to Review (failed tasks with PR) -->
						{#if task.status === 'failed'}
							<div class="px-5 py-3 border-t">
								<div class="flex items-center gap-3">
									<Button size="sm" variant="outline" onclick={handleMoveToReview} disabled={movingToReview} class="gap-2">
										{#if movingToReview}
											<Loader2 class="w-4 h-4 animate-spin" />
											Moving...
										{:else}
											<GitPullRequest class="w-4 h-4" />
											Move to Review
										{/if}
									</Button>
									<span class="text-xs text-muted-foreground">Mark this task as in review to track the existing PR.</span>
								</div>
							</div>
						{/if}
					</div>
				{/if}

				<!-- View Full PR -->
				{#if task.pull_request_url && (task.status === 'review' || task.status === 'merged' || task.status === 'closed' || task.status === 'failed')}
					<Button
						variant="outline"
						onclick={() => goto(`/${ownerParam}/${nameParam}/tasks/${numberParam}/pr`)}
						class="w-full gap-2 py-5 border-dashed"
					>
						<Eye class="w-4 h-4" />
						View Pull Request &amp; Changes
					</Button>
				{/if}

				<!-- Branch (skip-PR mode) -->
				{#if task.branch_name && !task.pull_request_url}
					<Card.Root class="border-cyan-500/30 bg-cyan-500/5">
						<Card.Header class="pb-0 gap-0">
							<Card.Title class="text-base flex items-center gap-2">
								<GitBranch class="w-4 h-4 text-cyan-500" />
								Branch
							</Card.Title>
						</Card.Header>
						<Card.Content class="space-y-3">
							<div class="flex items-center gap-4">
								{#if branchURL}
									<a
										href={branchURL}
										class="text-primary hover:underline font-medium flex items-center gap-2"
										target="_blank"
										rel="noopener noreferrer"
									>
										<GitBranch class="w-4 h-4" />
										{task.branch_name}
									</a>
								{:else}
									<span class="text-sm font-mono text-muted-foreground flex items-center gap-2">
										<GitBranch class="w-4 h-4" />
										{task.branch_name}
									</span>
								{/if}
								{#if task.status === 'review'}
									<Button
										size="sm"
										variant="outline"
										onclick={syncStatus}
										disabled={syncing}
										class="gap-2 shrink-0"
									>
										<RefreshCw class="w-4 h-4 {syncing ? 'animate-spin' : ''}" />
										{syncing ? 'Syncing...' : 'Sync PR'}
									</Button>
								{/if}
							</div>
							{#if task.status === 'review'}
								<p class="text-sm text-muted-foreground">No PR linked yet. Create one from this branch and sync to detect it.</p>
							{/if}
							{#if !task.pull_request_url && task.branch_name && (task.status === 'running' || task.status === 'pending') && task.attempt > 1}
								<div class="flex items-center gap-3 pt-2">
									<Loader2 class="w-4 h-4 animate-spin text-blue-500 shrink-0" />
									<span class="text-sm text-blue-600 dark:text-blue-400">
										{task.status === 'running' ? 'Agent is working on changes — the branch will be updated soon.' : 'Waiting for agent to pick up the task — the branch will be updated soon.'}
									</span>
								</div>
							{/if}
							{#if canProvideFeedback}
								<div class="pt-3 border-t mt-3">
									{#if showFeedbackForm}
										<div class="space-y-3">
											<label for="feedback-text-branch" class="text-sm font-medium block">
												Describe what you'd like changed
											</label>
											<textarea
												id="feedback-text-branch"
												bind:value={feedbackText}
												class="w-full border rounded-lg p-3 min-h-[100px] bg-background text-foreground resize-none focus:outline-none focus:ring-2 focus:ring-purple-500/40"
												placeholder="e.g. &quot;Add error handling to the new endpoint&quot; or &quot;Use a map instead of a slice for lookups&quot;..."
												disabled={sendingFeedback}
											></textarea>
											<div class="flex justify-end gap-2">
												<Button variant="outline" size="sm" onclick={() => (showFeedbackForm = false)} disabled={sendingFeedback}>
													Cancel
												</Button>
												<Button size="sm" onclick={handleFeedback} disabled={sendingFeedback || !feedbackText.trim()} class="gap-2 bg-purple-600 hover:bg-purple-700 text-white">
													{#if sendingFeedback}
														<Loader2 class="w-4 h-4 animate-spin" />
														Sending...
													{:else}
														<Send class="w-4 h-4" />
														Send to Agent
													{/if}
												</Button>
											</div>
										</div>
									{:else}
										<div class="flex items-center gap-3">
											<Button size="sm" variant="outline" onclick={() => (showFeedbackForm = true)} class="gap-2 border-purple-500/40 text-purple-600 dark:text-purple-400 hover:bg-purple-500/10">
												<MessageSquare class="w-4 h-4" />
												Request Agent Changes
											</Button>
											<span class="text-xs text-muted-foreground">The agent will update this branch based on your instructions.</span>
										</div>
									{/if}
								</div>
							{/if}
							{#if task.status === 'failed'}
								<div class="pt-3 border-t mt-3">
									<div class="flex items-center gap-3">
										<Button size="sm" variant="outline" onclick={handleMoveToReview} disabled={movingToReview} class="gap-2">
											{#if movingToReview}
												<Loader2 class="w-4 h-4 animate-spin" />
												Moving...
											{:else}
												<GitBranch class="w-4 h-4" />
												Move to Review
											{/if}
										</Button>
										<span class="text-xs text-muted-foreground">Mark this task as in review to track the existing branch.</span>
									</div>
								</div>
							{/if}
						</Card.Content>
					</Card.Root>
				{/if}

				<!-- Close Reason (don't show for stopped tasks since the banner handles that) -->
				{#if task.close_reason && !isStopped}
					<Card.Root class="border-gray-500/30">
						<Card.Header class="pb-0 gap-0">
							<Card.Title class="text-base flex items-center gap-2">
								<CheckCircle class="w-4 h-4 text-gray-500" />
								Close Reason
							</Card.Title>
						</Card.Header>
						<Card.Content>
							<p class="whitespace-pre-wrap text-muted-foreground">{task.close_reason}</p>
						</Card.Content>
					</Card.Root>
				{/if}
			</div>

			<!-- Right column: Agent Session Pane -->
			<div class="rounded-xl border bg-card shadow-sm overflow-hidden self-start">
				<!-- Header Bar -->
				<div class="flex items-center gap-2 px-5 py-3 border-b">
					<Sparkles class="w-4 h-4 text-muted-foreground" />
					<span class="font-semibold text-sm">Agent</span>
					{#if task.model}
						<span class="text-xs text-muted-foreground bg-muted px-2 py-0.5 rounded capitalize">{task.model}</span>
					{/if}
					{#if task.status === 'running'}
						<span class="flex items-center gap-1 text-xs text-blue-500">
							<span class="relative flex h-2 w-2">
								<span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75"></span>
								<span class="relative inline-flex rounded-full h-2 w-2 bg-blue-500"></span>
							</span>
							Live
						</span>
					{/if}
					{#if parsedAgentStatus?.tests_status}
						<Badge class="text-xs gap-1 {parsedAgentStatus.tests_status === 'pass' ? 'bg-green-500/20 text-green-400' : parsedAgentStatus.tests_status === 'skip' ? 'bg-secondary text-secondary-foreground' : 'bg-red-500/20 text-red-400'}">
							{#if parsedAgentStatus.tests_status === 'pass'}
								<CheckCircle class="w-3 h-3" />
							{:else if parsedAgentStatus.tests_status === 'fail'}
								<XCircle class="w-3 h-3" />
							{:else if parsedAgentStatus.tests_status === 'skip'}
								<MinusCircle class="w-3 h-3" />
							{/if}
							Tests: {parsedAgentStatus.tests_status}
						</Badge>
					{/if}
					{#if parsedAgentStatus?.confidence}
						<Badge class="{confidenceColor} text-xs">
							{parsedAgentStatus.confidence} confidence
						</Badge>
					{/if}
					{#if canStop || canRetry}
						<div class="ml-auto flex items-center gap-2">
							{#if canStop}
								<Button
									size="sm"
									variant="outline"
									onclick={handleStop}
									disabled={stopping}
									class="gap-1 border-red-500/40 text-red-600 dark:text-red-400 hover:bg-red-500/10"
								>
									{#if stopping}
										<Loader2 class="w-4 h-4 animate-spin" />
									{:else}
										<StopCircle class="w-4 h-4" />
									{/if}
									Stop Agent
								</Button>
							{/if}
							{#if canRetry}
								{#if showRetryForm}
									<Button size="sm" variant="ghost" onclick={() => (showRetryForm = false)} class="gap-1">
										<X class="w-4 h-4" />
										Cancel
									</Button>
								{:else}
									<Button size="sm" variant="outline" onclick={() => (showRetryForm = true)} class="gap-1">
										<RefreshCw class="w-4 h-4" />
										Retry
									</Button>
								{/if}
							{/if}
						</div>
					{/if}
				</div>

			<!-- Agent Insights -->
				{#if parsedAgentStatus || task.attempt > 1}
					<div class="px-5 py-4 border-b space-y-4">
						{#if task.attempt > 1}
							<div class="space-y-2">
								<div class="flex items-center gap-2">
									<span class="text-sm font-medium">Last Retry</span>
									<Badge variant="outline" class="text-xs {['review', 'merged'].includes(task.status) ? 'border-green-500/50 text-green-600 dark:text-green-400' : ''}">
										Attempt {task.attempt}/{task.max_attempts}
									</Badge>
								</div>
								{#if task.consecutive_failures >= 2 && !['review', 'merged'].includes(task.status)}
									<Badge class="bg-red-500 text-white gap-1 text-xs">
										<AlertTriangle class="w-3 h-3" />
										{task.consecutive_failures} consecutive failures
									</Badge>
								{/if}
								{#if task.retry_reason}
									<div class="flex items-start gap-2 text-sm">
										<span class="text-muted-foreground shrink-0">Reason:</span>
										<span class="text-foreground/80">{task.retry_reason}</span>
									</div>
								{/if}
								{#if task.retry_context}
									<div>
										<button
											type="button"
											class="inline-flex items-center gap-1.5 text-xs text-amber-700 dark:text-amber-400 hover:text-amber-800 dark:hover:text-amber-300 transition-colors bg-amber-500/10 hover:bg-amber-500/20 px-2 py-1.5 rounded-md"
											onclick={() => (showRetryContext = !showRetryContext)}
										>
											CI Failure Logs
											<ChevronDown class="w-3.5 h-3.5 transition-transform {showRetryContext ? 'rotate-180' : ''}" />
										</button>
										{#if showRetryContext}
											<pre class="mt-2 text-xs font-mono bg-zinc-900/50 text-white rounded-lg p-3 max-h-48 overflow-y-auto overscroll-contain whitespace-pre-wrap border border-border">{task.retry_context}</pre>
										{/if}
									</div>
								{/if}
							</div>
						{/if}
						{#if parsedAgentStatus?.files_modified && parsedAgentStatus.files_modified.length > 0}
							<div class="space-y-1.5">
								<span class="text-sm font-medium">Files Changed</span>
								<div class="flex flex-wrap gap-1.5">
									{#each parsedAgentStatus.files_modified as file}
										<span class="text-xs font-mono bg-indigo-500/10 text-indigo-700 dark:text-indigo-400 px-2 py-0.5 rounded-md">{file}</span>
									{/each}
								</div>
							</div>
						{/if}
						{#if parsedAgentStatus?.criteria_met && parsedAgentStatus.criteria_met.length > 0 && task.acceptance_criteria && task.acceptance_criteria.length > 0}
							<div class="space-y-1.5">
								<span class="text-sm font-medium">Criteria</span>
								<div class="flex flex-wrap gap-1.5">
									{#each parsedAgentStatus.criteria_met as criterion}
										<span class="inline-flex items-center gap-1 text-xs bg-green-500/10 text-green-700 dark:text-green-400 px-2 py-0.5 rounded-md">
											<CheckCircle class="w-3 h-3" />
											{criterion}
										</span>
									{/each}
								</div>
							</div>
						{/if}
						{#if parsedAgentStatus?.blockers && parsedAgentStatus.blockers.length > 0}
							<div class="space-y-1.5">
								<span class="text-sm font-medium">Blockers</span>
								<div class="flex flex-wrap gap-1.5">
									{#each parsedAgentStatus.blockers as blocker}
										<Badge variant="destructive" class="gap-1 text-xs">
											<AlertTriangle class="w-3 h-3" />
											{blocker}
										</Badge>
									{/each}
								</div>
							</div>
						{/if}
						{#if parsedAgentStatus?.notes}
							<div class="space-y-1.5">
								<span class="text-sm font-medium">Notes</span>
								<p class="text-sm text-muted-foreground">{parsedAgentStatus.notes}</p>
							</div>
						{/if}
					</div>
				{/if}

				<!-- Retry Form -->
				{#if showRetryForm}
					<div class="px-5 py-4 border-b bg-blue-500/5">
						<div class="space-y-4">
							<div>
								<label for="retry-instructions" class="text-sm font-medium mb-2 block">
									Instructions (optional)
								</label>
								<textarea
									id="retry-instructions"
									bind:value={retryInstructions}
									class="w-full border rounded-lg p-3 min-h-[80px] bg-background text-foreground resize-none focus:outline-none focus:ring-2 focus:ring-ring"
									placeholder="What should the agent do differently this time?"
									disabled={retrying}
								></textarea>
							</div>
							<div class="flex justify-end gap-2">
								<Button variant="outline" onclick={() => (showRetryForm = false)} disabled={retrying}>
									Cancel
								</Button>
								<Button onclick={handleRetry} disabled={retrying} class="gap-2">
									{#if retrying}
										<Loader2 class="w-4 h-4 animate-spin" />
										Retrying...
									{:else}
										<RefreshCw class="w-4 h-4" />
										Retry Task
									{/if}
								</Button>
							</div>
						</div>
					</div>
				{/if}

				<!-- Logs -->
				<div class="p-4">
					<div class="rounded-lg border border-zinc-800 overflow-hidden">
						<div class="flex items-center px-3 py-2 bg-zinc-950 border-b border-zinc-800">
							{#if showAttemptTabs}
								<div class="flex items-center gap-1">
									{#each attemptNumbers as num}
										<button
											type="button"
											class="px-3 py-1 text-xs font-medium rounded-md transition-all {activeAttemptTab === num ? 'bg-white/10 text-white' : 'text-zinc-600 hover:text-zinc-400 hover:bg-white/5'}"
											onclick={() => switchAttemptTab(num)}
										>
											Run {num}
											{#if task.status === 'running' && num === task.attempt}
												<span class="inline-flex ml-1 h-1.5 w-1.5 rounded-full bg-blue-500 animate-pulse"></span>
											{/if}
										</button>
									{/each}
								</div>
							{/if}
							<div class="ml-auto">
								<button
									type="button"
									class="inline-flex items-center gap-1.5 px-2 py-1 text-xs font-medium rounded-md transition-all {claudeOnly ? 'bg-white/10 text-zinc-300' : 'text-zinc-500 hover:text-zinc-300 hover:bg-white/5'}"
									onclick={() => (claudeOnly = !claudeOnly)}
									title={claudeOnly ? 'Showing Claude logs only — click to show all logs' : 'Showing all logs — click to filter to Claude only'}
								>
									<Filter class="w-3 h-3" />
									{claudeOnly ? 'Claude Only' : 'All Logs'}
								</button>
							</div>
						</div>
						<div
							bind:this={logsContainer}
							onscroll={handleLogsScroll}
							onwheel={handleLogsWheel}
							class="terminal-container h-[250px] sm:h-[400px] lg:h-[500px] w-full bg-zinc-950 p-3 sm:p-4 overflow-y-auto overscroll-contain"
						>
						{#if displayLogs.length > 0}
							<pre class="log-output text-xs font-mono whitespace-pre-wrap leading-relaxed">{@html renderedLogs}</pre>
						{:else if logs.length > 0}
							<div class="flex flex-col items-center justify-center h-full text-muted-foreground">
								<Filter class="w-8 h-8 opacity-20 mb-2" />
								<p class="text-sm">No Claude logs yet</p>
								<button
									type="button"
									class="text-xs text-zinc-400 hover:text-zinc-300 mt-1 underline underline-offset-2"
									onclick={() => (claudeOnly = false)}
								>
									Show all logs
								</button>
							</div>
						{:else}
							<div class="flex flex-col items-center justify-center h-full text-muted-foreground">
								<Terminal class="w-8 h-8 opacity-20 mb-2" />
								<p class="text-sm">No logs available yet</p>
							</div>
						{/if}
					</div>
					</div>
				</div>
			</div>
		</div>
		{#if task}
			<EditTaskDialog bind:open={showEditDialog} {task} onUpdated={handleEditUpdated} />
		{/if}
	{/if}
</div>

<!-- Delete Confirmation Dialog -->
<Dialog.Root bind:open={showDeleteDialog}>
	<Dialog.Content class="sm:max-w-[450px]">
		<Dialog.Header>
			<Dialog.Title class="flex items-center gap-2">
				<div class="w-8 h-8 rounded-lg bg-destructive/10 flex items-center justify-center">
					<Trash2 class="w-4 h-4 text-destructive" />
				</div>
				Delete Task
			</Dialog.Title>
			<Dialog.Description>
				Are you sure you want to delete this task? This action cannot be undone. All logs, agent data, and related information will be permanently removed.
			</Dialog.Description>
		</Dialog.Header>
		{#if error}
			<div class="bg-destructive/10 text-destructive text-sm p-3 rounded-lg flex items-center gap-2">
				<XCircle class="w-4 h-4 flex-shrink-0" />
				{error}
			</div>
		{/if}
		<Dialog.Footer>
			<div class="flex justify-end gap-2 w-full">
				<Button type="button" variant="outline" onclick={() => (showDeleteDialog = false)} disabled={deleting}>
					Cancel
				</Button>
				<Button type="button" variant="destructive" onclick={handleDelete} disabled={deleting} class="gap-2">
					{#if deleting}
						<Loader2 class="w-4 h-4 animate-spin" />
						Deleting...
					{:else}
						<Trash2 class="w-4 h-4" />
						Delete Task
					{/if}
				</Button>
			</div>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<style>
	:global(.terminal-container) {
		box-shadow: inset 0 2px 4px rgba(0, 0, 0, 0.3);
	}
	:global(.log-output) {
		color: #bac2cd;
	}
	:global(.log-bracket) {
		color: #79c0ff;
	}
	:global(.log-prompt) {
		color: #7ee787;
	}
	:global(.log-cmd) {
		color: #e2e8f0;
	}
	:global(.log-path) {
		color: #d2a8ff;
	}
	:global(.log-url) {
		color: #79c0ff;
		text-decoration: underline;
		text-decoration-color: #79c0ff40;
	}
	:global(.log-num) {
		color: #ffa657;
	}
	:global(.log-str) {
		color: #a5d6ff;
	}
	:global(.log-claude-line) {
		display: inline-block;
		width: 100%;
		background: rgba(245, 166, 35, 0.07);
		border-left: 2px solid #e8a33d;
		padding-left: 8px;
		margin-left: -8px;
	}
	:global(.log-claude-line .log-bracket) {
		color: #e8a33d;
		font-weight: 600;
	}
</style>

