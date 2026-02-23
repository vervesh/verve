<script lang="ts">
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { client } from '$lib/api-client';
	import type { Task } from '$lib/models/task';
	import { Button } from '$lib/components/ui/button';
	import { goto } from '$app/navigation';
	import DiffViewer from '$lib/components/DiffViewer.svelte';
	import {
		ArrowLeft,
		GitPullRequest,
		GitMerge,
		ExternalLink,
		Loader2,
		XCircle,
		CheckCircle,
		AlertTriangle,
		RefreshCw,
		CircleDot,
		MinusCircle,
		MessageSquare,
		Send,
		GitBranch
	} from 'lucide-svelte';

	let task = $state<Task | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let syncing = $state(false);
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
	let sendingFeedback = $state(false);
	let showFeedbackForm = $state(false);
	let feedbackText = $state('');

	const taskId = $derived($page.params.id as string);

	const isRetrying = $derived(task?.pull_request_url && (task?.status === 'running' || task?.status === 'pending'));
	const canProvideFeedback = $derived(task?.status === 'review');

	const prStatusLabel = $derived.by(() => {
		if (!task) return '';
		if (task.status === 'merged') return 'Merged';
		if (task.status === 'closed' || task.status === 'failed') return 'Closed';
		if (isRetrying) return 'Updating';
		return 'Open';
	});

	const prStatusColor = $derived.by(() => {
		if (!task) return '';
		if (task.status === 'merged') return 'text-green-600 dark:text-green-400';
		if (task.status === 'closed' || task.status === 'failed') return 'text-gray-500';
		if (isRetrying) return 'text-blue-500';
		return 'text-purple-600 dark:text-purple-400';
	});

	const prBorderColor = $derived.by(() => {
		if (!task) return 'border-border';
		if (task.status === 'merged') return 'border-green-500/30';
		if (task.status === 'closed' || task.status === 'failed') return 'border-gray-500/30';
		if (isRetrying) return 'border-blue-500/30';
		return 'border-purple-500/30';
	});

	onMount(() => {
		loadTask();

		const es = new EventSource(client.eventsURL());
		es.addEventListener('task_updated', (e) => {
			const event = JSON.parse(e.data);
			if (event.task?.id === taskId && task) {
				const prev = task.status;
				const updated: Task = { ...event.task, logs: task.logs };
				task = updated;
				if (updated.status === 'review' && updated.pr_number && prev !== 'review') {
					checkStatus = null;
					stopCheckPolling();
					forceCheckPolls = 3;
					checkPollTimer = setTimeout(loadCheckStatus, 5000);
				}
			}
		});

		return () => {
			es.close();
			stopCheckPolling();
		};
	});

	async function loadTask() {
		try {
			task = await client.getTask(taskId);
			error = null;
			if (task.status === 'review' && task.pr_number) {
				loadCheckStatus();
			}
		} catch (e) {
			error = (e as Error).message;
		} finally {
			loading = false;
		}
	}

	function stopCheckPolling() {
		if (checkPollTimer) {
			clearTimeout(checkPollTimer);
			checkPollTimer = null;
		}
	}

	async function loadCheckStatus() {
		checkStatusLoading = true;
		stopCheckPolling();
		try {
			checkStatus = await client.getTaskChecks(taskId);
			const shouldPoll = checkStatus.status === 'pending' || forceCheckPolls > 0;
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
</script>

<div class="flex flex-col min-h-0">
	<!-- Header section with padding -->
	<div class="p-4 sm:p-6 pb-0 sm:pb-0">
		<!-- Back Navigation -->
		<Button variant="ghost" onclick={() => goto(`/tasks/${taskId}`)} class="mb-4 sm:mb-6 gap-2 -ml-2">
			<ArrowLeft class="w-4 h-4" />
			Back to Task
		</Button>
	</div>

	{#if loading}
		<div class="flex flex-col items-center justify-center py-16">
			<Loader2 class="w-8 h-8 animate-spin text-primary mb-4" />
			<p class="text-muted-foreground">Loading pull request...</p>
		</div>
	{:else if error && !task}
		<div class="px-4 sm:px-6">
			<div class="bg-destructive/10 text-destructive p-4 rounded-lg flex items-center gap-3 border border-destructive/20">
				<XCircle class="w-5 h-5 flex-shrink-0" />
				<span>{error}</span>
			</div>
		</div>
	{:else if task && !task.pull_request_url}
		<div class="px-4 sm:px-6">
			<div class="bg-muted/50 text-muted-foreground p-6 rounded-lg flex flex-col items-center gap-3">
				<GitPullRequest class="w-8 h-8 opacity-40" />
				<p>No pull request associated with this task.</p>
				<Button variant="outline" onclick={() => goto(`/tasks/${taskId}`)}>Back to Task</Button>
			</div>
		</div>
	{:else if task && task.pull_request_url}
		<!-- PR Header with padding -->
		<div class="px-4 sm:px-6 space-y-4 pb-4">
			<div class="flex items-start gap-4 flex-wrap">
				<div class="flex items-center gap-3">
					{#if task.status === 'merged'}
						<div class="w-10 h-10 rounded-lg bg-green-500/10 flex items-center justify-center shrink-0">
							<GitMerge class="w-5 h-5 text-green-500" />
						</div>
					{:else}
						<div class="w-10 h-10 rounded-lg bg-purple-500/10 flex items-center justify-center shrink-0">
							<GitPullRequest class="w-5 h-5 text-purple-500" />
						</div>
					{/if}
					<div>
						<h1 class="text-xl sm:text-2xl font-semibold">{task.title}</h1>
						<div class="flex items-center gap-2 mt-1 text-sm">
							<span class="{prStatusColor} font-medium">{prStatusLabel}</span>
							<span class="text-muted-foreground">·</span>
							<a
								href={task.pull_request_url}
								class="text-primary hover:underline flex items-center gap-1"
								target="_blank"
								rel="noopener noreferrer"
							>
								PR #{task.pr_number}
								<ExternalLink class="w-3 h-3" />
							</a>
							{#if task.branch_name}
								<span class="text-muted-foreground">·</span>
								<span class="text-xs font-mono text-muted-foreground flex items-center gap-1">
									<GitBranch class="w-3 h-3" />
									{task.branch_name}
								</span>
							{/if}
						</div>
					</div>
				</div>
				<div class="ml-auto flex items-center gap-2">
					{#if task.status === 'review'}
						<Button
							size="sm"
							variant="outline"
							onclick={() => { syncStatus(); loadCheckStatus(); }}
							disabled={checkStatusLoading || syncing}
							class="gap-1.5"
						>
							<RefreshCw class="w-3.5 h-3.5 {checkStatusLoading || syncing ? 'animate-spin' : ''}" />
							Sync
						</Button>
					{/if}
					<Button
						size="sm"
						variant="outline"
						onclick={() => window.open(task!.pull_request_url, '_blank')}
						class="gap-1.5"
					>
						<ExternalLink class="w-3.5 h-3.5" />
						View on GitHub
					</Button>
				</div>
			</div>

			<!-- Active Retry Banner -->
			{#if isRetrying}
				<div class="rounded-lg border border-blue-500/30 bg-blue-500/[0.08] px-5 py-3 flex items-center gap-3">
					<Loader2 class="w-4 h-4 animate-spin text-blue-500 shrink-0" />
					<span class="text-sm text-blue-600 dark:text-blue-400">
						{task.status === 'running' ? 'Agent is working on changes — the PR will be updated soon.' : 'Waiting for agent to pick up the task — the PR will be updated soon.'}
					</span>
				</div>
			{/if}

			<!-- CI Checks -->
			{#if task.status === 'review' || task.status === 'merged' || checkStatus}
				<div class="rounded-xl border {prBorderColor} shadow-sm overflow-hidden">
					<div class="flex items-center gap-2 px-5 py-3 border-b">
						<CheckCircle class="w-4 h-4 text-muted-foreground" />
						<span class="font-semibold text-sm">CI Checks</span>
					</div>
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
							{:else}
								<CircleDot class="w-3.5 h-3.5 text-muted-foreground" />
								<span class="text-sm text-muted-foreground">No check data available</span>
							{/if}
						</div>
						{#if checkStatus?.checks && checkStatus.checks.length > 0}
							<div class="space-y-1 mt-1">
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
										placeholder='e.g. "Add error handling to the new endpoint" or "Use a map instead of a slice for lookups"...'
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
				</div>
			{/if}
		</div>

		<!-- Diff Viewer with same padding as above sections -->
		<div class="px-4 sm:px-6 pb-4 sm:pb-6">
			<DiffViewer taskId={task.id} hasPR={true} prUrl={task.pull_request_url} autoExpand={true} />
		</div>
	{/if}
</div>
