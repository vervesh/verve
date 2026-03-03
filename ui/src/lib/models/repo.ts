export type SetupStatus = 'pending' | 'scanning' | 'needs_setup' | 'ready';

export interface Repo {
	id: string;
	owner: string;
	name: string;
	full_name: string;
	summary: string;
	tech_stack: string[];
	setup_status: SetupStatus;
	has_code: boolean;
	has_claude_md: boolean;
	has_readme: boolean;
	expectations: string;
	setup_completed_at?: string;
	created_at: string;
}

export interface GitHubRepo {
	full_name: string;
	owner_login: string;
	name: string;
	description: string;
	private: boolean;
	html_url: string;
}
