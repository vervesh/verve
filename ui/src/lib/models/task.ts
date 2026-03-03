export type TaskStatus = 'pending' | 'running' | 'review' | 'merged' | 'closed' | 'failed';
export type TaskType = 'task' | 'setup';

export interface Task {
	id: string;
	repo_id: string;
	type: TaskType;
	title: string;
	description: string;
	status: TaskStatus;
	logs: string[];
	pull_request_url?: string;
	pr_number?: number;
	depends_on?: string[];
	close_reason?: string;
	attempt: number;
	max_attempts: number;
	retry_reason?: string;
	acceptance_criteria: string[];
	agent_status?: string;
	retry_context?: string;
	consecutive_failures: number;
	cost_usd: number;
	max_cost_usd?: number;
	skip_pr: boolean;
	draft_pr: boolean;
	ready: boolean;
	epic_id?: string;
	model?: string;
	branch_name?: string;
	started_at?: string;
	duration_ms?: number;
	created_at: string;
	updated_at: string;
}
