import type { Repo } from '$lib/models/repo';

const STORAGE_KEY = 'verve_selected_repo_id';

class RepoStore {
	repos = $state<Repo[]>([]);
	selectedRepoId = $state<string | null>(null);
	loading = $state(false);

	constructor() {
		if (typeof window !== 'undefined') {
			this.selectedRepoId = localStorage.getItem(STORAGE_KEY);
		}
	}

	get selectedRepo(): Repo | null {
		return this.repos.find((r) => r.id === this.selectedRepoId) ?? null;
	}

	setRepos(repos: Repo[]) {
		this.repos = repos;
		// If selected repo no longer exists, auto-select first
		if (this.selectedRepoId && !repos.find((r) => r.id === this.selectedRepoId)) {
			this.selectRepo(repos.length > 0 ? repos[0].id : null);
		}
		// If nothing selected and repos exist, select first
		if (!this.selectedRepoId && repos.length > 0) {
			this.selectRepo(repos[0].id);
		}
	}

	selectRepo(id: string | null) {
		this.selectedRepoId = id;
		if (typeof window !== 'undefined') {
			if (id) {
				localStorage.setItem(STORAGE_KEY, id);
			} else {
				localStorage.removeItem(STORAGE_KEY);
			}
		}
	}

	addRepo(repo: Repo) {
		this.repos = [...this.repos, repo];
		this.selectRepo(repo.id);
	}

	removeRepo(id: string) {
		this.repos = this.repos.filter((r) => r.id !== id);
		if (this.selectedRepoId === id) {
			this.selectRepo(this.repos.length > 0 ? this.repos[0].id : null);
		}
	}

	updateRepo(updated: Repo) {
		this.repos = this.repos.map((r) => (r.id === updated.id ? updated : r));
	}
}

export const repoStore = new RepoStore();
