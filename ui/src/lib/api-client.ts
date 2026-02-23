import { API_BASE_URL } from './config/api';
import type { Task } from './models/task';
import type { Repo, GitHubRepo } from './models/repo';
import type { Epic, ProposedTask } from './models/epic';
import type { AgentMetrics } from './models/agent-metrics';

export class VerveClient {
	private baseUrl: string;

	constructor() {
		this.baseUrl = API_BASE_URL + '/api/v1';
	}

	// --- Repo APIs ---

	async listRepos(): Promise<Repo[]> {
		const res = await fetch(`${this.baseUrl}/repos`);
		if (!res.ok) {
			throw new Error('Failed to fetch repos');
		}
		return res.json();
	}

	async addRepo(fullName: string): Promise<Repo> {
		const res = await fetch(`${this.baseUrl}/repos`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ full_name: fullName })
		});
		if (!res.ok) {
			throw new Error('Failed to add repo');
		}
		return res.json();
	}

	async removeRepo(repoId: string): Promise<void> {
		const res = await fetch(`${this.baseUrl}/repos/${repoId}`, {
			method: 'DELETE'
		});
		if (!res.ok) {
			throw new Error('Failed to remove repo');
		}
	}

	async listAvailableRepos(): Promise<GitHubRepo[]> {
		const res = await fetch(`${this.baseUrl}/repos/available`);
		if (!res.ok) {
			throw new Error('Failed to list available repos');
		}
		return res.json();
	}

	// --- Repo-scoped Task APIs ---

	async listTasksByRepo(repoId: string): Promise<Task[]> {
		const res = await fetch(`${this.baseUrl}/repos/${repoId}/tasks`);
		if (!res.ok) {
			throw new Error('Failed to fetch tasks');
		}
		return res.json();
	}

	async createTaskInRepo(
		repoId: string,
		title: string,
		description?: string,
		dependsOn?: string[],
		acceptanceCriteria?: string[],
		maxCostUsd?: number,
		skipPr?: boolean,
		model?: string,
		notReady?: boolean
	): Promise<Task> {
		const body: Record<string, unknown> = { title, description, depends_on: dependsOn };
		if (acceptanceCriteria && acceptanceCriteria.length > 0)
			body.acceptance_criteria = acceptanceCriteria;
		if (maxCostUsd && maxCostUsd > 0) body.max_cost_usd = maxCostUsd;
		if (skipPr) body.skip_pr = true;
		if (model) body.model = model;
		if (notReady) body.not_ready = true;
		const res = await fetch(`${this.baseUrl}/repos/${repoId}/tasks`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(body)
		});
		if (!res.ok) {
			throw new Error('Failed to create task');
		}
		return res.json();
	}

	async syncRepoTasks(repoId: string): Promise<{ synced: number; merged: number }> {
		const res = await fetch(`${this.baseUrl}/repos/${repoId}/tasks/sync`, {
			method: 'POST'
		});
		if (!res.ok) {
			throw new Error('Failed to sync tasks');
		}
		return res.json();
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
			model?: string;
			not_ready?: boolean;
		}
	): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}`, {
			method: 'PATCH',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(updates)
		});
		if (!res.ok) {
			const body = await res.json().catch(() => null);
			throw new Error(body?.error || 'Failed to update task');
		}
		return res.json();
	}

	async getTask(id: string): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}`);
		if (!res.ok) {
			throw new Error('Task not found');
		}
		return res.json();
	}

	async syncTask(id: string): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/sync`, {
			method: 'POST'
		});
		if (!res.ok) {
			throw new Error('Failed to sync task');
		}
		return res.json();
	}

	async stopTask(id: string): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/stop`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' }
		});
		if (!res.ok) {
			throw new Error('Failed to stop task');
		}
		return res.json();
	}

	async closeTask(id: string, reason?: string): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/close`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ reason })
		});
		if (!res.ok) {
			throw new Error('Failed to close task');
		}
		return res.json();
	}

	async getTaskChecks(id: string): Promise<{
		status: 'pending' | 'success' | 'failure' | 'error';
		summary?: string;
		failed_names?: string[];
		check_runs_skipped?: boolean;
		checks?: { name: string; status: string; conclusion: string; url: string }[];
	}> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/checks`);
		if (!res.ok) {
			throw new Error('Failed to fetch check status');
		}
		return res.json();
	}

	async getTaskDiff(id: string): Promise<{ diff: string }> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/diff`);
		if (!res.ok) {
			throw new Error('Failed to fetch task diff');
		}
		return res.json();
	}

	async retryTask(id: string, instructions?: string): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/retry`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ instructions })
		});
		if (!res.ok) {
			throw new Error('Failed to retry task');
		}
		return res.json();
	}

	async feedbackTask(id: string, feedback: string): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/feedback`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ feedback })
		});
		if (!res.ok) {
			throw new Error('Failed to submit feedback');
		}
		return res.json();
	}

	async moveToReview(id: string): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/move-to-review`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' }
		});
		if (!res.ok) {
			const body = await res.json().catch(() => null);
			throw new Error(body?.error || 'Failed to move task to review');
		}
		return res.json();
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
		if (!res.ok) {
			const body = await res.json().catch(() => null);
			throw new Error(body?.error || 'Failed to start over');
		}
		return res.json();
	}

	async setReady(id: string, ready: boolean): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/ready`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ ready })
		});
		if (!res.ok) {
			throw new Error('Failed to update task ready state');
		}
		return res.json();
	}

	async removeDependency(id: string, dependsOn: string): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/dependency`, {
			method: 'DELETE',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ depends_on: dependsOn })
		});
		if (!res.ok) {
			throw new Error('Failed to remove dependency');
		}
		return res.json();
	}

	async deleteTask(id: string): Promise<void> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}`, {
			method: 'DELETE'
		});
		if (!res.ok) {
			throw new Error('Failed to delete task');
		}
	}

	async bulkDeleteTasks(taskIds: string[]): Promise<void> {
		const res = await fetch(`${this.baseUrl}/tasks/bulk-delete`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ task_ids: taskIds })
		});
		if (!res.ok) {
			throw new Error('Failed to bulk delete tasks');
		}
	}

	// --- Agent Observability APIs ---

	async getAgentMetrics(): Promise<AgentMetrics> {
		const res = await fetch(`${this.baseUrl}/agents/metrics`);
		if (!res.ok) {
			throw new Error('Failed to fetch agent metrics');
		}
		return res.json();
	}

	// --- Settings APIs ---

	async getGitHubTokenStatus(): Promise<{ configured: boolean; fine_grained?: boolean }> {
		const res = await fetch(`${this.baseUrl}/settings/github-token`);
		if (!res.ok) {
			throw new Error('Failed to check GitHub token status');
		}
		return res.json();
	}

	async saveGitHubToken(token: string): Promise<void> {
		const res = await fetch(`${this.baseUrl}/settings/github-token`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ token })
		});
		if (!res.ok) {
			const body = await res.json().catch(() => null);
			throw new Error(body?.error || 'Failed to save GitHub token');
		}
	}

	async deleteGitHubToken(): Promise<void> {
		const res = await fetch(`${this.baseUrl}/settings/github-token`, {
			method: 'DELETE'
		});
		if (!res.ok) {
			throw new Error('Failed to delete GitHub token');
		}
	}

	async getDefaultModel(): Promise<{ model: string; configured: boolean }> {
		const res = await fetch(`${this.baseUrl}/settings/default-model`);
		if (!res.ok) {
			throw new Error('Failed to get default model');
		}
		return res.json();
	}

	async listModels(): Promise<{ value: string; label: string }[]> {
		const res = await fetch(`${this.baseUrl}/settings/models`);
		if (!res.ok) {
			throw new Error('Failed to list models');
		}
		return res.json();
	}

	async saveDefaultModel(model: string): Promise<void> {
		const res = await fetch(`${this.baseUrl}/settings/default-model`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ model })
		});
		if (!res.ok) {
			throw new Error('Failed to save default model');
		}
	}

	async deleteDefaultModel(): Promise<void> {
		const res = await fetch(`${this.baseUrl}/settings/default-model`, {
			method: 'DELETE'
		});
		if (!res.ok) {
			throw new Error('Failed to delete default model');
		}
	}

	// --- Epic APIs ---

	async listEpicsByRepo(repoId: string): Promise<Epic[]> {
		const res = await fetch(`${this.baseUrl}/repos/${repoId}/epics`);
		if (!res.ok) {
			throw new Error('Failed to fetch epics');
		}
		return res.json();
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
		if (!res.ok) {
			throw new Error('Failed to create epic');
		}
		return res.json();
	}

	async getEpic(id: string): Promise<Epic> {
		const res = await fetch(`${this.baseUrl}/epics/${id}`);
		if (!res.ok) {
			throw new Error('Epic not found');
		}
		return res.json();
	}

	async getEpicTasks(id: string): Promise<{ id: string; title: string; status: string }[]> {
		const res = await fetch(`${this.baseUrl}/epics/${id}/tasks`);
		if (!res.ok) {
			throw new Error('Failed to fetch epic tasks');
		}
		return res.json();
	}

	async deleteEpic(id: string): Promise<void> {
		const res = await fetch(`${this.baseUrl}/epics/${id}`, {
			method: 'DELETE'
		});
		if (!res.ok) {
			throw new Error('Failed to delete epic');
		}
	}

	async startPlanning(id: string, prompt: string): Promise<Epic> {
		const res = await fetch(`${this.baseUrl}/epics/${id}/plan`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ prompt })
		});
		if (!res.ok) {
			throw new Error('Failed to start planning');
		}
		return res.json();
	}

	async updateProposedTasks(id: string, tasks: ProposedTask[]): Promise<Epic> {
		const res = await fetch(`${this.baseUrl}/epics/${id}/proposed-tasks`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ tasks })
		});
		if (!res.ok) {
			throw new Error('Failed to update proposed tasks');
		}
		return res.json();
	}

	async sendSessionMessage(id: string, message: string): Promise<Epic> {
		const res = await fetch(`${this.baseUrl}/epics/${id}/session-message`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ message })
		});
		if (!res.ok) {
			throw new Error('Failed to send message');
		}
		return res.json();
	}

	async finishPlanning(id: string): Promise<Epic> {
		const res = await fetch(`${this.baseUrl}/epics/${id}/finish-planning`, {
			method: 'POST'
		});
		if (!res.ok) {
			throw new Error('Failed to finish planning');
		}
		return res.json();
	}

	async confirmEpic(id: string, notReady?: boolean): Promise<Epic> {
		const res = await fetch(`${this.baseUrl}/epics/${id}/confirm`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ not_ready: notReady ?? false })
		});
		if (!res.ok) {
			const body = await res.json().catch(() => null);
			throw new Error(body?.error || 'Failed to confirm epic');
		}
		return res.json();
	}

	async closeEpic(id: string): Promise<Epic> {
		const res = await fetch(`${this.baseUrl}/epics/${id}/close`, {
			method: 'POST'
		});
		if (!res.ok) {
			throw new Error('Failed to close epic');
		}
		return res.json();
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
