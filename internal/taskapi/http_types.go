package taskapi

import (
	"github.com/cohesivestack/valgo"

	"github.com/joshjon/verve/internal/github"
	"github.com/joshjon/verve/internal/repo"
	"github.com/joshjon/verve/internal/task"
)

// --- Param-only request types ---

// TaskIDRequest captures the :id path parameter for task endpoints.
type TaskIDRequest struct {
	ID string `param:"id" json:"-"`
}

func (r TaskIDRequest) Validate() error {
	return valgo.In("params", valgo.Is(task.TaskIDValidator(r.ID, "id"))).ToError()
}

// RepoIDRequest captures the :repo_id path parameter for repo-scoped endpoints.
type RepoIDRequest struct {
	RepoID string `param:"repo_id" json:"-"`
}

func (r RepoIDRequest) Validate() error {
	return valgo.In("params", valgo.Is(repo.RepoIDValidator(r.RepoID, "repo_id"))).ToError()
}

// --- Param+body request types ---

// CreateTaskRequest is the request body for creating a task.
type CreateTaskRequest struct {
	RepoID             string   `param:"repo_id" json:"-"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	DependsOn          []string `json:"depends_on,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
	MaxCostUSD         float64  `json:"max_cost_usd,omitempty"`
	SkipPR             bool     `json:"skip_pr,omitempty"`
	Model              string   `json:"model,omitempty"`
	NotReady           bool     `json:"not_ready,omitempty"`
}

func (r CreateTaskRequest) Validate() error {
	return valgo.
		In("params", valgo.Is(repo.RepoIDValidator(r.RepoID, "repo_id"))).
		Is(
			valgo.String(r.Title, "title").Not().Blank().MaxLength(150),
		).ToError()
}

// UpdateTaskRequest is the request body for updating a pending task.
// All fields are optional — only provided fields are updated.
type UpdateTaskRequest struct {
	ID                 string   `param:"id" json:"-"`
	Title              *string  `json:"title,omitempty"`
	Description        *string  `json:"description,omitempty"`
	DependsOn          []string `json:"depends_on,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
	MaxCostUSD         *float64 `json:"max_cost_usd,omitempty"`
	SkipPR             *bool    `json:"skip_pr,omitempty"`
	Model              *string  `json:"model,omitempty"`
	NotReady           *bool    `json:"not_ready,omitempty"`
}

func (r UpdateTaskRequest) Validate() error {
	v := valgo.In("params", valgo.Is(task.TaskIDValidator(r.ID, "id")))
	if r.Title != nil {
		v = v.Is(valgo.String(*r.Title, "title").Not().Blank().MaxLength(150))
	}
	return v.ToError()
}

// CloseRequest is the request body for closing a task.
type CloseRequest struct {
	ID     string `param:"id" json:"-"`
	Reason string `json:"reason,omitempty"`
}

func (r CloseRequest) Validate() error {
	return valgo.In("params", valgo.Is(task.TaskIDValidator(r.ID, "id"))).ToError()
}

// RetryTaskRequest is the request body for retrying a failed task.
type RetryTaskRequest struct {
	ID           string `param:"id" json:"-"`
	Instructions string `json:"instructions,omitempty"`
}

func (r RetryTaskRequest) Validate() error {
	return valgo.In("params", valgo.Is(task.TaskIDValidator(r.ID, "id"))).ToError()
}

// FeedbackRequest is the request body for providing feedback on a task in review.
type FeedbackRequest struct {
	ID       string `param:"id" json:"-"`
	Feedback string `json:"feedback"`
}

func (r FeedbackRequest) Validate() error {
	return valgo.In("params", valgo.Is(task.TaskIDValidator(r.ID, "id"))).
		Is(valgo.String(r.Feedback, "feedback").Not().Blank()).
		ToError()
}

// StartOverRequest is the request body for starting a task over from scratch.
type StartOverRequest struct {
	ID                 string   `param:"id" json:"-"`
	Title              *string  `json:"title,omitempty"`
	Description        *string  `json:"description,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
}

func (r StartOverRequest) Validate() error {
	v := valgo.In("params", valgo.Is(task.TaskIDValidator(r.ID, "id")))
	if r.Title != nil {
		v = v.Is(valgo.String(*r.Title, "title").Not().Blank().MaxLength(150))
	}
	return v.ToError()
}

// RemoveDependencyRequest is the request body for removing a dependency from a task.
type RemoveDependencyRequest struct {
	ID        string `param:"id" json:"-"`
	DependsOn string `json:"depends_on"`
}

func (r RemoveDependencyRequest) Validate() error {
	return valgo.In("params", valgo.Is(task.TaskIDValidator(r.ID, "id"))).
		Is(valgo.String(r.DependsOn, "depends_on").Not().Blank()).
		ToError()
}

// SetReadyRequest is the request body for toggling a task's ready state.
type SetReadyRequest struct {
	ID    string `param:"id" json:"-"`
	Ready bool   `json:"ready"`
}

func (r SetReadyRequest) Validate() error {
	return valgo.In("params", valgo.Is(task.TaskIDValidator(r.ID, "id"))).ToError()
}

// LogsRequest is the request body for appending logs.
type LogsRequest struct {
	ID      string   `param:"id" json:"-"`
	Logs    []string `json:"logs"`
	Attempt int      `json:"attempt,omitempty"`
}

func (r LogsRequest) Validate() error {
	return valgo.In("params", valgo.Is(task.TaskIDValidator(r.ID, "id"))).ToError()
}

// CompleteRequest is the request body for completing a task.
type CompleteRequest struct {
	ID             string  `param:"id" json:"-"`
	Success        bool    `json:"success"`
	Error          string  `json:"error,omitempty"`
	PullRequestURL string  `json:"pull_request_url,omitempty"`
	PRNumber       int     `json:"pr_number,omitempty"`
	AgentStatus    string  `json:"agent_status,omitempty"`
	CostUSD        float64 `json:"cost_usd,omitempty"`
	PrereqFailed   string  `json:"prereq_failed,omitempty"`
	BranchName     string  `json:"branch_name,omitempty"`
	NoChanges      bool    `json:"no_changes,omitempty"`
	Retryable      bool    `json:"retryable,omitempty"`
}

func (r CompleteRequest) Validate() error {
	return valgo.In("params", valgo.Is(task.TaskIDValidator(r.ID, "id"))).ToError()
}

// HeartbeatRequest captures the :id path parameter for the heartbeat endpoint.
type HeartbeatRequest struct {
	ID string `param:"id" json:"-"`
}

func (r HeartbeatRequest) Validate() error {
	return valgo.In("params", valgo.Is(task.TaskIDValidator(r.ID, "id"))).ToError()
}

// BulkDeleteTasksRequest is the request body for bulk-deleting tasks.
type BulkDeleteTasksRequest struct {
	TaskIDs []string `json:"task_ids"`
}

func (r BulkDeleteTasksRequest) Validate() error {
	if len(r.TaskIDs) == 0 {
		return valgo.AddErrorMessage("task_ids", "task_ids required").ToError()
	}
	return nil
}

// SyncRepoTasksRequest captures the :repo_id path parameter.
type SyncRepoTasksRequest struct {
	RepoID string `param:"repo_id" json:"-"`
}

func (r SyncRepoTasksRequest) Validate() error {
	return valgo.In("params", valgo.Is(repo.RepoIDValidator(r.RepoID, "repo_id"))).ToError()
}

// --- Response types ---

// CheckStatusResponse is the response body for the task check status endpoint.
type CheckStatusResponse struct {
	Status           string                   `json:"status"`                       // "pending", "success", "failure", "error"
	Summary          string                   `json:"summary,omitempty"`
	FailedNames      []string                 `json:"failed_names,omitempty"`
	CheckRunsSkipped bool                     `json:"check_runs_skipped,omitempty"` // True when GitHub Actions checks couldn't be read (fine-grained PAT)
	Checks           []github.IndividualCheck `json:"checks,omitempty"`
}

// DiffResponse is the response body for the task diff endpoint.
type DiffResponse struct {
	Diff string `json:"diff"`
}

// PollTaskResponse wraps a claimed task with the credentials and repo info
// needed by the worker to execute it. The GitHub token is included so that
// workers don't need their own token configuration.
type PollTaskResponse struct {
	Task         *task.Task `json:"task"`
	GitHubToken  string     `json:"github_token"`
	RepoFullName string     `json:"repo_full_name"`
}
