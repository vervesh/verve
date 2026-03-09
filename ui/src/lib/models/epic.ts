export type EpicStatus = 'draft' | 'planning' | 'ready' | 'active' | 'completed' | 'closed';

export interface ProposedTask {
	temp_id: string;
	title: string;
	description: string;
	depends_on_temp_ids?: string[];
	acceptance_criteria?: string[];
}

export interface Epic {
	id: string;
	repo_id: string;
	number: number;
	title: string;
	description: string;
	status: EpicStatus;
	proposed_tasks: ProposedTask[];
	task_ids: string[];
	planning_prompt?: string;
	session_log: string[];
	not_ready: boolean;
	model?: string;
	claimed_at?: string;
	created_at: string;
	updated_at: string;
}
