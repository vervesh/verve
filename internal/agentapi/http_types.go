package agentapi

import (
	"github.com/cohesivestack/valgo"

	"github.com/joshjon/verve/internal/epic"
	"github.com/joshjon/verve/internal/repo"
	"github.com/joshjon/verve/internal/task"
)

// PollResponse is the discriminated union returned by the unified poll endpoint.
type PollResponse struct {
	Type string `json:"type"` // "task" or "epic"

	// Task fields (present when Type == "task")
	Task *task.Task `json:"task,omitempty"`

	// Epic fields (present when Type == "epic")
	Epic *epic.Epic `json:"epic,omitempty"`

	// Common fields
	GitHubToken  string `json:"github_token,omitempty"`
	RepoFullName string `json:"repo_full_name"`

	// Repo setup data (injected into agent prompts)
	RepoSummary      string `json:"repo_summary,omitempty"`
	RepoExpectations string `json:"repo_expectations,omitempty"`
	RepoTechStack    string `json:"repo_tech_stack,omitempty"`
}

// TaskIDRequest captures the :id path parameter for task agent endpoints.
type TaskIDRequest struct {
	ID string `param:"id" json:"-"`
}

func (r TaskIDRequest) Validate() error {
	return valgo.In("params", valgo.Is(task.TaskIDValidator(r.ID, "id"))).ToError()
}

// TaskLogsRequest is the request for appending task logs.
type TaskLogsRequest struct {
	ID      string   `param:"id" json:"-"`
	Logs    []string `json:"logs"`
	Attempt int      `json:"attempt"`
}

func (r TaskLogsRequest) Validate() error {
	return valgo.In("params", valgo.Is(task.TaskIDValidator(r.ID, "id"))).ToError()
}

// TaskCompleteRequest is the request for completing a task.
type TaskCompleteRequest struct {
	ID             string  `param:"id" json:"-"`
	Success        bool    `json:"success"`
	PullRequestURL string  `json:"pull_request_url"`
	PRNumber       int     `json:"pr_number"`
	BranchName     string  `json:"branch_name"`
	Error       string  `json:"error"`
	AgentStatus string  `json:"agent_status"`
	CostUSD     float64 `json:"cost_usd"`
	NoChanges   bool    `json:"no_changes"`
	Retryable   bool    `json:"retryable"`
}

func (r TaskCompleteRequest) Validate() error {
	return valgo.In("params", valgo.Is(task.TaskIDValidator(r.ID, "id"))).ToError()
}

// EpicIDRequest captures the :id path parameter for epic agent endpoints.
type EpicIDRequest struct {
	ID string `param:"id" json:"-"`
}

func (r EpicIDRequest) Validate() error {
	return valgo.In("params", valgo.Is(epic.EpicIDValidator(r.ID, "id"))).ToError()
}

// EpicCompleteRequest is the request for completing epic planning.
type EpicCompleteRequest struct {
	ID      string              `param:"id" json:"-"`
	Success bool                `json:"success"`
	Tasks   []epic.ProposedTask `json:"tasks,omitempty"`
	Error   string              `json:"error,omitempty"`
}

func (r EpicCompleteRequest) Validate() error {
	return valgo.In("params", valgo.Is(epic.EpicIDValidator(r.ID, "id"))).ToError()
}

// SessionLogRequest is the request body for appending session log entries.
type SessionLogRequest struct {
	ID    string   `param:"id" json:"-"`
	Lines []string `json:"lines"`
}

func (r SessionLogRequest) Validate() error {
	return valgo.In("params", valgo.Is(epic.EpicIDValidator(r.ID, "id"))).ToError()
}

// RepoSetupCompleteRequest is the request for completing a repo setup scan.
type RepoSetupCompleteRequest struct {
	RepoID     string   `param:"repo_id" json:"-"`
	Success    bool     `json:"success"`
	Summary    string   `json:"summary"`
	TechStack  []string `json:"tech_stack"`
	HasCode    bool     `json:"has_code"`
	HasClaudeMD bool    `json:"has_claude_md"`
	HasREADME  bool     `json:"has_readme"`
	NeedsSetup bool     `json:"needs_setup"`
}

func (r RepoSetupCompleteRequest) Validate() error {
	return valgo.In("params", valgo.Is(repo.RepoIDValidator(r.RepoID, "repo_id"))).ToError()
}
