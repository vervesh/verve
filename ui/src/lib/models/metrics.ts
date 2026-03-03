export interface ActiveAgent {
	task_id: string;
	task_title: string;
	repo_id: string;
	started_at: string;
	running_for_ms: number;
	attempt: number;
	cost_usd: number;
	model?: string;
	epic_id?: string;
	is_planning?: boolean;
	epic_title?: string;
}

export interface CompletedAgent {
	task_id: string;
	task_title: string;
	repo_id: string;
	status: string;
	duration_ms?: number;
	cost_usd: number;
	attempt: number;
	finished_at: string;
}

export interface WorkerInfo {
	worker_id: string;
	max_concurrent_tasks: number;
	active_tasks: number;
	connected_at: string;
	last_poll_at: string;
	uptime_ms: number;
	polling: boolean;
}

export interface Metrics {
	running_agents: number;
	pending_tasks: number;
	review_tasks: number;
	total_tasks: number;
	completed_tasks: number;
	failed_tasks: number;
	total_cost_usd: number;
	active_agents: ActiveAgent[];
	recent_completions: CompletedAgent[];
	workers: WorkerInfo[];
}
