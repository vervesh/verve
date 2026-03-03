<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { client } from '$lib/api-client';
	import type { Metrics, ActiveAgent, CompletedAgent, WorkerInfo } from '$lib/models/metrics';
	import {
		Activity,
		Clock,
		DollarSign,
		CheckCircle2,
		XCircle,
		Play,
		Eye,
		AlertCircle,
		RefreshCw,
		Cpu,
		Server,
		Layers
	} from 'lucide-svelte';

	let metrics = $state<Metrics | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let pollInterval: ReturnType<typeof setInterval> | null = null;

	async function loadMetrics() {
		try {
			metrics = await client.getMetrics();
			error = null;
		} catch (e) {
			error = (e as Error).message;
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		loadMetrics();
		// Auto-refresh every 5 seconds
		pollInterval = setInterval(loadMetrics, 5000);
	});

	onDestroy(() => {
		if (pollInterval) {
			clearInterval(pollInterval);
		}
	});

	function formatDuration(ms: number): string {
		if (ms < 1000) return `${ms}ms`;
		const seconds = Math.floor(ms / 1000);
		if (seconds < 60) return `${seconds}s`;
		const minutes = Math.floor(seconds / 60);
		const remainingSeconds = seconds % 60;
		if (minutes < 60) return `${minutes}m ${remainingSeconds}s`;
		const hours = Math.floor(minutes / 60);
		const remainingMinutes = minutes % 60;
		return `${hours}h ${remainingMinutes}m`;
	}

	function formatTimeAgo(isoString: string): string {
		const date = new Date(isoString);
		const now = new Date();
		const ms = now.getTime() - date.getTime();
		return formatDuration(ms) + ' ago';
	}

	function statusColor(status: string): string {
		switch (status) {
			case 'merged':
				return 'text-green-400';
			case 'closed':
				return 'text-gray-400';
			case 'failed':
				return 'text-red-400';
			default:
				return 'text-muted-foreground';
		}
	}

	function statusBg(status: string): string {
		switch (status) {
			case 'merged':
				return 'bg-green-500/10';
			case 'closed':
				return 'bg-gray-500/10';
			case 'failed':
				return 'bg-red-500/10';
			default:
				return 'bg-muted';
		}
	}

	const successRate = $derived(() => {
		if (!metrics) return 0;
		const total = metrics.completed_tasks + metrics.failed_tasks;
		if (total === 0) return 0;
		return Math.round((metrics.completed_tasks / total) * 100);
	});
</script>

<div class="p-4 sm:p-6 flex-1 min-h-0 flex flex-col">
	<header class="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3 mb-4 sm:mb-6">
		<div>
			<div class="flex items-center gap-3">
				<h1 class="text-xl sm:text-2xl font-bold">Metrics</h1>
				{#if metrics}
					<span
						class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {metrics.running_agents > 0 ? 'bg-green-500/15 text-green-400' : 'bg-muted text-muted-foreground'}"
					>
						{metrics.running_agents} running
					</span>
					<span
						class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {metrics.workers.length > 0 ? 'bg-blue-500/15 text-blue-400' : 'bg-muted text-muted-foreground'}"
					>
						{metrics.workers.length} {metrics.workers.length === 1 ? 'worker' : 'workers'}
					</span>
				{/if}
			</div>
			<p class="text-muted-foreground text-sm mt-1 hidden sm:block">
				Monitor running agents, connected workers, and track performance metrics
			</p>
		</div>
		<div class="flex items-center gap-2 sm:gap-3">
			<button
				onclick={loadMetrics}
				class="inline-flex items-center gap-2 px-3 py-2 rounded-md text-sm font-medium border border-border bg-card hover:bg-accent transition-colors"
			>
				<RefreshCw class="w-4 h-4" />
				<span class="hidden sm:inline">Refresh</span>
			</button>
		</div>
	</header>

	{#if error}
		<div
			class="bg-destructive/10 text-destructive p-4 rounded-lg mb-4 flex items-center gap-3 border border-destructive/20"
		>
			<AlertCircle class="w-5 h-5 flex-shrink-0" />
			<span>{error}</span>
		</div>
	{/if}

	{#if loading && !metrics}
		<div class="flex-1 flex items-center justify-center">
			<div class="text-muted-foreground text-sm">Loading metrics...</div>
		</div>
	{:else if metrics}
		<!-- Tasks -->
		<div class="mb-6">
			<h2 class="text-sm font-semibold text-muted-foreground mb-3 flex items-center gap-2">
				<Activity class="w-4 h-4" />
				Tasks
			</h2>
		<!-- Summary Cards -->
		<div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3 mb-6">
			<div class="bg-card border border-border rounded-lg p-4">
				<div class="flex items-center gap-2 mb-2">
					<Play class="w-4 h-4 text-green-400" />
					<span class="text-xs font-medium text-muted-foreground">Running</span>
				</div>
				<div class="text-2xl font-bold">{metrics.running_agents}</div>
			</div>
			<div class="bg-card border border-border rounded-lg p-4">
				<div class="flex items-center gap-2 mb-2">
					<Clock class="w-4 h-4 text-amber-400" />
					<span class="text-xs font-medium text-muted-foreground">Pending</span>
				</div>
				<div class="text-2xl font-bold">{metrics.pending_tasks}</div>
			</div>
			<div class="bg-card border border-border rounded-lg p-4">
				<div class="flex items-center gap-2 mb-2">
					<Eye class="w-4 h-4 text-purple-400" />
					<span class="text-xs font-medium text-muted-foreground">In Review</span>
				</div>
				<div class="text-2xl font-bold">{metrics.review_tasks}</div>
			</div>
			<div class="bg-card border border-border rounded-lg p-4">
				<div class="flex items-center gap-2 mb-2">
					<CheckCircle2 class="w-4 h-4 text-green-400" />
					<span class="text-xs font-medium text-muted-foreground">Completed</span>
				</div>
				<div class="text-2xl font-bold">{metrics.completed_tasks}</div>
			</div>
			<div class="bg-card border border-border rounded-lg p-4">
				<div class="flex items-center gap-2 mb-2">
					<XCircle class="w-4 h-4 text-red-400" />
					<span class="text-xs font-medium text-muted-foreground">Failed</span>
				</div>
				<div class="text-2xl font-bold">{metrics.failed_tasks}</div>
			</div>
			<div class="bg-card border border-border rounded-lg p-4">
				<div class="flex items-center gap-2 mb-2">
					<DollarSign class="w-4 h-4 text-yellow-400" />
					<span class="text-xs font-medium text-muted-foreground">Total Cost</span>
				</div>
				<div class="text-2xl font-bold">${metrics.total_cost_usd.toFixed(2)}</div>
			</div>
		</div>

		<!-- Success Rate Bar -->
		{#if metrics.completed_tasks + metrics.failed_tasks > 0}
			<div class="bg-card border border-border rounded-lg p-4 mb-6">
				<div class="flex items-center justify-between mb-2">
					<span class="text-sm font-medium">Success Rate</span>
					<span class="text-sm font-bold">{successRate()}%</span>
				</div>
				<div class="w-full bg-muted rounded-full h-2 overflow-hidden">
					<div
						class="h-full rounded-full transition-all duration-500 {successRate() >= 80 ? 'bg-green-500' : successRate() >= 50 ? 'bg-amber-500' : 'bg-red-500'}"
						style="width: {successRate()}%"
					></div>
				</div>
				<div class="flex justify-between mt-1 text-xs text-muted-foreground">
					<span>{metrics.completed_tasks} completed</span>
					<span>{metrics.failed_tasks} failed</span>
				</div>
			</div>
		{/if}
		</div>

		<!-- Connected Workers -->
		<div class="mb-6">
			<h2 class="text-sm font-semibold text-muted-foreground mb-3 flex items-center gap-2">
				<Server class="w-4 h-4" />
				Connected Workers
				{#if metrics.workers.length > 0}
					<span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-blue-500/15 text-blue-400">
						{metrics.workers.length}
					</span>
				{/if}
			</h2>
			{#if metrics.workers.length === 0}
				<div class="bg-card border border-border rounded-lg p-8 text-center">
					<Server class="w-8 h-8 text-muted-foreground mx-auto mb-3" />
					<p class="text-muted-foreground text-sm">No workers connected</p>
					<p class="text-muted-foreground text-xs mt-1">Workers will appear here when they start polling for tasks</p>
				</div>
			{:else}
				<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
					{#each metrics.workers as worker (worker.worker_id)}
						<div class="bg-card border border-border rounded-lg p-4">
							<div class="flex items-center justify-between mb-3">
								<div class="flex items-center gap-2">
									<span class="relative flex h-2 w-2 shrink-0">
										{#if worker.polling}
											<span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75"></span>
											<span class="relative inline-flex rounded-full h-2 w-2 bg-blue-500"></span>
										{:else}
											<span class="relative inline-flex rounded-full h-2 w-2 bg-green-500"></span>
										{/if}
									</span>
									<span class="text-xs font-mono text-muted-foreground">{worker.worker_id.slice(0, 8)}</span>
								</div>
								<span class="inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium {worker.polling ? 'bg-blue-500/10 text-blue-400' : 'bg-green-500/10 text-green-400'}">
									{worker.polling ? 'Polling' : 'Active'}
								</span>
							</div>
							<div class="space-y-2">
								<div class="flex items-center justify-between text-xs">
									<span class="text-muted-foreground flex items-center gap-1">
										<Layers class="w-3 h-3" />
										Agent capacity
									</span>
									<span class="font-medium">
										{worker.active_tasks} / {worker.max_concurrent_tasks}
									</span>
								</div>
								<div class="w-full bg-muted rounded-full h-1.5 overflow-hidden">
									<div
										class="h-full rounded-full transition-all duration-500 {worker.active_tasks >= worker.max_concurrent_tasks ? 'bg-amber-500' : 'bg-blue-500'}"
										style="width: {worker.max_concurrent_tasks > 0 ? Math.min((worker.active_tasks / worker.max_concurrent_tasks) * 100, 100) : 0}%"
									></div>
								</div>
								<div class="flex items-center justify-between text-xs">
									<span class="text-muted-foreground flex items-center gap-1">
										<Clock class="w-3 h-3" />
										Uptime
									</span>
									<span class="font-medium">{formatDuration(worker.uptime_ms)}</span>
								</div>
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</div>

		<!-- Active Agents -->
		<div class="mb-6">
			<h2 class="text-sm font-semibold text-muted-foreground mb-3 flex items-center gap-2">
				<Activity class="w-4 h-4" />
				Active Agents
				{#if metrics.running_agents > 0}
					<span class="relative flex h-2 w-2">
						<span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75"></span>
						<span class="relative inline-flex rounded-full h-2 w-2 bg-green-500"></span>
					</span>
				{/if}
			</h2>
			{#if metrics.active_agents.length === 0}
				<div class="bg-card border border-border rounded-lg p-8 text-center">
					<Cpu class="w-8 h-8 text-muted-foreground mx-auto mb-3" />
					<p class="text-muted-foreground text-sm">No agents currently running</p>
					<p class="text-muted-foreground text-xs mt-1">Agents will appear here when tasks are being processed</p>
				</div>
			{:else}
				<div class="space-y-2">
					{#each metrics.active_agents as agent (agent.task_id)}
						<a
							href={agent.is_planning ? `/epics/${agent.epic_id}` : `/tasks/${agent.task_id}`}
							class="block bg-card border border-border rounded-lg p-4 hover:border-primary/30 transition-colors"
						>
							<div class="flex items-start justify-between gap-3">
								<div class="min-w-0 flex-1">
									<div class="flex items-center gap-2 mb-1">
										<span class="relative flex h-2 w-2 shrink-0">
											{#if agent.is_planning}
												<span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-violet-400 opacity-75"></span>
												<span class="relative inline-flex rounded-full h-2 w-2 bg-violet-500"></span>
											{:else}
												<span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75"></span>
												<span class="relative inline-flex rounded-full h-2 w-2 bg-green-500"></span>
											{/if}
										</span>
										<span class="font-medium text-sm truncate">{agent.task_title || agent.task_id}</span>
										{#if agent.is_planning}
											<span class="inline-flex items-center px-1.5 py-0.5 rounded text-xs bg-violet-500/10 text-violet-400">
												Planning
											</span>
										{/if}
									</div>
									<div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground mt-2">
										<span class="flex items-center gap-1">
											<Clock class="w-3 h-3" />
											{agent.is_planning ? 'Planning for' : 'Running for'} {formatDuration(agent.running_for_ms)}
										</span>
										{#if agent.cost_usd > 0}
											<span class="flex items-center gap-1">
												<DollarSign class="w-3 h-3" />
												${agent.cost_usd.toFixed(2)}
											</span>
										{/if}
										{#if agent.model}
											<span class="inline-flex items-center px-1.5 py-0.5 rounded text-xs bg-muted">
												{agent.model}
											</span>
										{/if}
										{#if agent.attempt > 1}
											<span class="inline-flex items-center px-1.5 py-0.5 rounded text-xs bg-amber-500/10 text-amber-400">
												Attempt {agent.attempt}
											</span>
										{/if}
									</div>
								</div>
								<div class="text-xs text-muted-foreground shrink-0">
									{agent.task_id.slice(0, 12)}...
								</div>
							</div>
						</a>
					{/each}
				</div>
			{/if}
		</div>

		<!-- Recent Completions -->
		{#if metrics.recent_completions.length > 0}
			<div>
				<h2 class="text-sm font-semibold text-muted-foreground mb-3">Recent Completions</h2>
				<div class="bg-card border border-border rounded-lg overflow-hidden">
					<table class="w-full text-sm">
						<thead>
							<tr class="border-b border-border">
								<th class="text-left p-3 text-xs font-medium text-muted-foreground">Task</th>
								<th class="text-left p-3 text-xs font-medium text-muted-foreground hidden sm:table-cell">Status</th>
								<th class="text-left p-3 text-xs font-medium text-muted-foreground hidden md:table-cell">Duration</th>
								<th class="text-left p-3 text-xs font-medium text-muted-foreground hidden lg:table-cell">Cost</th>
								<th class="text-right p-3 text-xs font-medium text-muted-foreground">Finished</th>
							</tr>
						</thead>
						<tbody>
							{#each metrics.recent_completions as completion (completion.task_id + completion.finished_at)}
								<tr class="border-b border-border last:border-0 hover:bg-accent/50 transition-colors">
									<td class="p-3">
										<a href="/tasks/{completion.task_id}" class="hover:text-primary transition-colors">
											<div class="font-medium truncate max-w-[200px] sm:max-w-[300px]">
												{completion.task_title || completion.task_id}
											</div>
										</a>
									</td>
									<td class="p-3 hidden sm:table-cell">
										<span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium {statusBg(completion.status)} {statusColor(completion.status)}">
											{completion.status}
										</span>
									</td>
									<td class="p-3 hidden md:table-cell text-muted-foreground">
										{completion.duration_ms ? formatDuration(completion.duration_ms) : '-'}
									</td>
									<td class="p-3 hidden lg:table-cell text-muted-foreground">
										{completion.cost_usd > 0 ? `$${completion.cost_usd.toFixed(2)}` : '-'}
									</td>
									<td class="p-3 text-right text-muted-foreground text-xs">
										{formatTimeAgo(completion.finished_at)}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</div>
		{/if}
	{/if}
</div>
