import { API_BASE_URL } from './config/api';
import type { Task } from './models/task';
import type { Repo, GitHubRepo } from './models/repo';
import type { Epic, ProposedTask } from './models/epic';
import type { Conversation } from './models/conversation';
import type { Metrics } from './models/metrics';

export class VerveClient {
	private baseUrl: string;

	constructor() {
		this.baseUrl = API_BASE_URL + '/api/v1';
	}

	private async request<T>(res: Response, fallbackError: string): Promise<T> {
		if (!res.ok) {
			const body = await res.json().catch(() => null);
			throw new Error(body?.error?.message || fallbackError);
		}
		const json = await res.json();
		return json.data;
	}

	private async requestVoid(res: Response, fallbackError: string): Promise<void> {
		if (!res.ok) {
			const body = await res.json().catch(() => null);
			throw new Error(body?.error?.message || fallbackError);
		}
	}

	// --- Repo APIs ---

	async listRepos(): Promise<Repo[]> {
		const res = await fetch(`${this.baseUrl}/repos`);
		return this.request<Repo[]>(res, 'Failed to fetch repos');
	}

	async addRepo(fullName: string): Promise<Repo> {
		const res = await fetch(`${this.baseUrl}/repos`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ full_name: fullName })
		});
		return this.request<Repo>(res, 'Failed to add repo');
	}

	async removeRepo(repoId: string): Promise<void> {
		const res = await fetch(`${this.baseUrl}/repos/${repoId}`, {
			method: 'DELETE'
		});
		return this.requestVoid(res, 'Failed to remove repo');
	}

	async listAvailableRepos(): Promise<GitHubRepo[]> {
		const res = await fetch(`${this.baseUrl}/repos/available`);
		return this.request<GitHubRepo[]>(res, 'Failed to list available repos');
	}

	// --- Repo Setup APIs ---

	async getRepoSetup(repoId: string): Promise<Repo> {
		const res = await fetch(`${this.baseUrl}/repos/${repoId}/setup`);
		return this.request<Repo>(res, 'Failed to fetch repo setup');
	}

	async updateRepoSetup(
		repoId: string,
		updates: {
			summary?: string;
			expectations?: string;
			tech_stack?: string[];
			mark_ready?: boolean;
		}
	): Promise<Repo> {
		const res = await fetch(`${this.baseUrl}/repos/${repoId}/setup`, {
			method: 'PATCH',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(updates)
		});
		return this.request<Repo>(res, 'Failed to update repo setup');
	}

	async rescanRepo(repoId: string): Promise<Repo> {
		const res = await fetch(`${this.baseUrl}/repos/${repoId}/setup/rescan`, {
			method: 'POST'
		});
		return this.request<Repo>(res, 'Failed to trigger rescan');
	}

	async skipRepoSetup(repoId: string): Promise<Repo> {
		const res = await fetch(`${this.baseUrl}/repos/${repoId}/setup/skip`, {
			method: 'POST'
		});
		return this.request<Repo>(res, 'Failed to skip setup');
	}

	async submitRepoSetup(
		repoId: string,
		updates: {
			summary?: string;
			expectations?: string;
			tech_stack?: string[];
		}
	): Promise<Repo> {
		const res = await fetch(`${this.baseUrl}/repos/${repoId}/setup/submit`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(updates)
		});
		return this.request<Repo>(res, 'Failed to submit setup for review');
	}

	async confirmRepoSetup(repoId: string): Promise<Repo> {
		const res = await fetch(`${this.baseUrl}/repos/${repoId}/setup/confirm`, {
			method: 'POST'
		});
		return this.request<Repo>(res, 'Failed to confirm repo setup');
	}

	// --- Repo-scoped Task APIs ---

	async listTasksByRepo(repoId: string): Promise<Task[]> {
		const res = await fetch(`${this.baseUrl}/repos/${repoId}/tasks`);
		return this.request<Task[]>(res, 'Failed to fetch tasks');
	}

	async createTaskInRepo(
		repoId: string,
		title: string,
		description?: string,
		dependsOn?: string[],
		acceptanceCriteria?: string[],
		maxCostUsd?: number,
		skipPr?: boolean,
		draftPr?: boolean,
		model?: string,
		notReady?: boolean
	): Promise<Task> {
		const body: Record<string, unknown> = { title, description, depends_on: dependsOn };
		if (acceptanceCriteria && acceptanceCriteria.length > 0)
			body.acceptance_criteria = acceptanceCriteria;
		if (maxCostUsd && maxCostUsd > 0) body.max_cost_usd = maxCostUsd;
		if (skipPr) body.skip_pr = true;
		if (draftPr) body.draft_pr = true;
		if (model) body.model = model;
		if (notReady) body.not_ready = true;
		const res = await fetch(`${this.baseUrl}/repos/${repoId}/tasks`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(body)
		});
		return this.request<Task>(res, 'Failed to create task');
	}

	async syncRepoTasks(repoId: string): Promise<{ synced: number; merged: number }> {
		const res = await fetch(`${this.baseUrl}/repos/${repoId}/tasks/sync`, {
			method: 'POST'
		});
		return this.request<{ synced: number; merged: number }>(res, 'Failed to sync tasks');
	}

	// --- Task APIs (global by ID) ---

	async updateTask(
		id: string,
		updates: {
			title?: string;
			description?: string;
			depends_on?: string[];
			acceptance_criteria?: string[];
			max_cost_usd?: number;
			skip_pr?: boolean;
			draft_pr?: boolean;
			model?: string;
			not_ready?: boolean;
		}
	): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}`, {
			method: 'PATCH',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(updates)
		});
		return this.request<Task>(res, 'Failed to update task');
	}

	async getTask(id: string): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}`);
		return this.request<Task>(res, 'Task not found');
	}

	async getTaskByNumber(repoId: string, number: number): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/repos/${repoId}/tasks/${number}`);
		return this.request<Task>(res, 'Task not found');
	}

	async syncTask(id: string): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/sync`, {
			method: 'POST'
		});
		return this.request<Task>(res, 'Failed to sync task');
	}

	async stopTask(id: string): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/stop`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' }
		});
		return this.request<Task>(res, 'Failed to stop task');
	}

	async closeTask(id: string, reason?: string): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/close`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ reason })
		});
		return this.request<Task>(res, 'Failed to close task');
	}

	async getTaskChecks(id: string): Promise<{
		status: 'pending' | 'success' | 'failure' | 'error';
		summary?: string;
		failed_names?: string[];
		check_runs_skipped?: boolean;
		checks?: { name: string; status: string; conclusion: string; url: string }[];
	}> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/checks`);
		return this.request(res, 'Failed to fetch check status');
	}

	async getTaskDiff(id: string): Promise<{ diff: string }> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/diff`);
		return this.request<{ diff: string }>(res, 'Failed to fetch task diff');
	}

	async retryTask(id: string, instructions?: string): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/retry`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ instructions })
		});
		return this.request<Task>(res, 'Failed to retry task');
	}

	async feedbackTask(id: string, feedback: string): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/feedback`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ feedback })
		});
		return this.request<Task>(res, 'Failed to submit feedback');
	}

	async moveToReview(id: string): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/move-to-review`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' }
		});
		return this.request<Task>(res, 'Failed to move task to review');
	}

	async startOverTask(
		id: string,
		updates?: { title?: string; description?: string; acceptance_criteria?: string[] }
	): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/start-over`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(updates ?? {})
		});
		return this.request<Task>(res, 'Failed to start over');
	}

	async setReady(id: string, ready: boolean): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/ready`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ ready })
		});
		return this.request<Task>(res, 'Failed to update task ready state');
	}

	async removeDependency(id: string, dependsOn: string): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/dependency`, {
			method: 'DELETE',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ depends_on: dependsOn })
		});
		return this.request<Task>(res, 'Failed to remove dependency');
	}

	async deleteTask(id: string): Promise<void> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}`, {
			method: 'DELETE'
		});
		return this.requestVoid(res, 'Failed to delete task');
	}

	async bulkDeleteTasks(taskIds: string[]): Promise<void> {
		const res = await fetch(`${this.baseUrl}/tasks/bulk-delete`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ task_ids: taskIds })
		});
		return this.requestVoid(res, 'Failed to bulk delete tasks');
	}

	// --- Agent Observability APIs ---

	async getMetrics(): Promise<Metrics> {
		const res = await fetch(`${this.baseUrl}/metrics`);
		return this.request<Metrics>(res, 'Failed to fetch metrics');
	}

	// --- Settings APIs ---

	async getGitHubTokenStatus(): Promise<{ configured: boolean; fine_grained?: boolean }> {
		const res = await fetch(`${this.baseUrl}/settings/github-token`);
		return this.request(res, 'Failed to check GitHub token status');
	}

	async saveGitHubToken(token: string): Promise<void> {
		const res = await fetch(`${this.baseUrl}/settings/github-token`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ token })
		});
		return this.requestVoid(res, 'Failed to save GitHub token');
	}

	async deleteGitHubToken(): Promise<void> {
		const res = await fetch(`${this.baseUrl}/settings/github-token`, {
			method: 'DELETE'
		});
		return this.requestVoid(res, 'Failed to delete GitHub token');
	}

	async getDefaultModel(): Promise<{ model: string; configured: boolean }> {
		const res = await fetch(`${this.baseUrl}/settings/default-model`);
		return this.request(res, 'Failed to get default model');
	}

	async listModels(): Promise<{ value: string; label: string }[]> {
		const res = await fetch(`${this.baseUrl}/settings/models`);
		return this.request(res, 'Failed to list models');
	}

	async saveDefaultModel(model: string): Promise<void> {
		const res = await fetch(`${this.baseUrl}/settings/default-model`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ model })
		});
		return this.requestVoid(res, 'Failed to save default model');
	}

	async deleteDefaultModel(): Promise<void> {
		const res = await fetch(`${this.baseUrl}/settings/default-model`, {
			method: 'DELETE'
		});
		return this.requestVoid(res, 'Failed to delete default model');
	}

	// --- Epic APIs ---

	async listEpicsByRepo(repoId: string): Promise<Epic[]> {
		const res = await fetch(`${this.baseUrl}/repos/${repoId}/epics`);
		return this.request<Epic[]>(res, 'Failed to fetch epics');
	}

	async createEpic(
		repoId: string,
		title: string,
		description: string,
		planningPrompt?: string,
		model?: string
	): Promise<Epic> {
		const body: Record<string, unknown> = { title, description };
		if (planningPrompt) body.planning_prompt = planningPrompt;
		if (model) body.model = model;
		const res = await fetch(`${this.baseUrl}/repos/${repoId}/epics`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(body)
		});
		return this.request<Epic>(res, 'Failed to create epic');
	}

	async getEpic(id: string): Promise<Epic> {
		const res = await fetch(`${this.baseUrl}/epics/${id}`);
		return this.request<Epic>(res, 'Epic not found');
	}

	async getEpicByNumber(repoId: string, number: number): Promise<Epic> {
		const res = await fetch(`${this.baseUrl}/repos/${repoId}/epics/${number}`);
		return this.request<Epic>(res, 'Epic not found');
	}

	async getEpicTasks(id: string): Promise<{ id: string; number: number; title: string; status: string }[]> {
		const res = await fetch(`${this.baseUrl}/epics/${id}/tasks`);
		return this.request(res, 'Failed to fetch epic tasks');
	}

	async deleteEpic(id: string): Promise<void> {
		const res = await fetch(`${this.baseUrl}/epics/${id}`, {
			method: 'DELETE'
		});
		return this.requestVoid(res, 'Failed to delete epic');
	}

	async startPlanning(id: string, prompt: string): Promise<Epic> {
		const res = await fetch(`${this.baseUrl}/epics/${id}/plan`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ prompt })
		});
		return this.request<Epic>(res, 'Failed to start planning');
	}

	async updateProposedTasks(id: string, tasks: ProposedTask[]): Promise<Epic> {
		const res = await fetch(`${this.baseUrl}/epics/${id}/proposed-tasks`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ tasks })
		});
		return this.request<Epic>(res, 'Failed to update proposed tasks');
	}

	async sendSessionMessage(id: string, message: string): Promise<Epic> {
		const res = await fetch(`${this.baseUrl}/epics/${id}/session-message`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ message })
		});
		return this.request<Epic>(res, 'Failed to send message');
	}

	async confirmEpic(id: string, notReady?: boolean): Promise<Epic> {
		const res = await fetch(`${this.baseUrl}/epics/${id}/confirm`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ not_ready: notReady ?? false })
		});
		return this.request<Epic>(res, 'Failed to confirm epic');
	}

	async closeEpic(id: string): Promise<Epic> {
		const res = await fetch(`${this.baseUrl}/epics/${id}/close`, {
			method: 'POST'
		});
		return this.request<Epic>(res, 'Failed to close epic');
	}

	// --- Conversation APIs ---

	async listConversationsByRepo(repoId: string, status?: string): Promise<Conversation[]> {
		const params = status ? `?status=${encodeURIComponent(status)}` : '';
		const res = await fetch(`${this.baseUrl}/repos/${repoId}/conversations${params}`);
		return this.request<Conversation[]>(res, 'Failed to fetch conversations');
	}

	async createConversation(
		repoId: string,
		title: string,
		initialMessage?: string,
		model?: string
	): Promise<Conversation> {
		const body: Record<string, unknown> = { title };
		if (initialMessage) body.initial_message = initialMessage;
		if (model) body.model = model;
		const res = await fetch(`${this.baseUrl}/repos/${repoId}/conversations`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(body)
		});
		return this.request<Conversation>(res, 'Failed to create conversation');
	}

	async getConversation(id: string): Promise<Conversation> {
		const res = await fetch(`${this.baseUrl}/conversations/${id}`);
		return this.request<Conversation>(res, 'Conversation not found');
	}

	async deleteConversation(id: string): Promise<void> {
		const res = await fetch(`${this.baseUrl}/conversations/${id}`, {
			method: 'DELETE'
		});
		return this.requestVoid(res, 'Failed to delete conversation');
	}

	async sendConversationMessage(id: string, message: string): Promise<Conversation> {
		const res = await fetch(`${this.baseUrl}/conversations/${id}/messages`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ message })
		});
		return this.request<Conversation>(res, 'Failed to send message');
	}

	async archiveConversation(id: string): Promise<Conversation> {
		const res = await fetch(`${this.baseUrl}/conversations/${id}/archive`, {
			method: 'POST'
		});
		return this.request<Conversation>(res, 'Failed to archive conversation');
	}

	async generateTasksFromConversation(
		id: string,
		title: string,
		planningPrompt?: string
	): Promise<{ epic_id: string }> {
		const body: Record<string, unknown> = { title };
		if (planningPrompt) body.planning_prompt = planningPrompt;
		const res = await fetch(`${this.baseUrl}/conversations/${id}/generate-tasks`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(body)
		});
		return this.request<{ epic_id: string }>(res, 'Failed to generate tasks');
	}

	// --- SSE URLs ---

	eventsURL(repoId?: string): string {
		if (repoId) {
			return `${this.baseUrl}/events?repo_id=${repoId}`;
		}
		return `${this.baseUrl}/events`;
	}

	taskLogsURL(id: string): string {
		return `${this.baseUrl}/tasks/${id}/logs`;
	}
}

export const client = new VerveClient();
