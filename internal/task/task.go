package task

import "time"

// Status represents the lifecycle state of a Task.
type Status string

const (
	StatusPending Status = "pending"
	StatusRunning Status = "running"
	StatusReview  Status = "review" // PR created, awaiting review/merge
	StatusMerged  Status = "merged" // PR has been merged
	StatusClosed  Status = "closed" // Manually closed by user
	StatusFailed  Status = "failed"
)

// TaskType distinguishes regular coding tasks from internal system tasks.
const (
	TaskTypeTask        = "task"         // Regular coding task
	TaskTypeSetup       = "setup"        // Internal repo setup scan
	TaskTypeSetupReview = "setup-review" // Internal repo setup review (AI refines user config)
)

// Task represents a unit of work dispatched to an AI coding agent.
type Task struct {
	ID                  TaskID    `json:"id"`
	RepoID              string    `json:"repo_id"`
	Type                string    `json:"type"`
	Title               string    `json:"title"`
	Description         string    `json:"description"`
	Status              Status    `json:"status"`
	Logs                []string  `json:"logs"`
	PullRequestURL      string    `json:"pull_request_url,omitempty"`
	PRNumber            int       `json:"pr_number,omitempty"`
	DependsOn           []string  `json:"depends_on,omitempty"`
	CloseReason         string    `json:"close_reason,omitempty"`
	Attempt             int       `json:"attempt"`
	MaxAttempts         int       `json:"max_attempts"`
	RetryReason         string    `json:"retry_reason,omitempty"`
	AcceptanceCriteria  []string  `json:"acceptance_criteria"`
	AgentStatus         string    `json:"agent_status,omitempty"`
	RetryContext        string    `json:"retry_context,omitempty"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
	CostUSD             float64   `json:"cost_usd"`
	MaxCostUSD          float64   `json:"max_cost_usd,omitempty"`
	SkipPR              bool      `json:"skip_pr"`
	DraftPR             bool      `json:"draft_pr"`
	Ready               bool      `json:"ready"`
	EpicID              string     `json:"epic_id,omitempty"`
	Model               string     `json:"model,omitempty"`
	BranchName          string     `json:"branch_name,omitempty"`
	StartedAt           *time.Time `json:"started_at,omitempty"`
	DurationMs          *int64     `json:"duration_ms,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// ComputeDuration calculates the run duration from StartedAt to UpdatedAt
// for tasks that have finished running (review, merged, closed, failed),
// or from StartedAt to now for tasks that are currently running.
func (t *Task) ComputeDuration() {
	if t.StartedAt == nil {
		return
	}
	var end time.Time
	switch t.Status {
	case StatusRunning:
		end = time.Now()
	case StatusReview, StatusMerged, StatusClosed, StatusFailed:
		end = t.UpdatedAt
	default:
		return
	}
	ms := end.Sub(*t.StartedAt).Milliseconds()
	t.DurationMs = &ms
}

// UpdatePendingTaskParams holds the fields that can be updated on a pending task.
// All fields are required — the caller should merge with current values before calling.
type UpdatePendingTaskParams struct {
	Title              string
	Description        string
	DependsOn          []string
	AcceptanceCriteria []string
	MaxCostUSD         float64
	SkipPR             bool
	DraftPR            bool
	Model              string
	Ready              bool
}

// StartOverTaskParams holds the fields that can be updated when starting a task over.
type StartOverTaskParams struct {
	Title              string
	Description        string
	AcceptanceCriteria []string
}

// NewSetupTask creates a new internal setup scan task for a repo.
func NewSetupTask(repoID string) *Task {
	now := time.Now()
	return &Task{
		ID:                 NewTaskID(),
		RepoID:             repoID,
		Type:               TaskTypeSetup,
		Title:              "Repository setup scan",
		Description:        "Scan the repository to detect tech stack, configuration files, and setup requirements.",
		Status:             StatusPending,
		DependsOn:          []string{},
		Attempt:            1,
		MaxAttempts:        3,
		AcceptanceCriteria: []string{},
		SkipPR:             true,
		Ready:              true,
		Model:              "sonnet",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

// NewSetupReviewTask creates a new internal setup review task. The AI agent
// reviews and fleshes out the user's raw configuration input.
func NewSetupReviewTask(repoID string) *Task {
	now := time.Now()
	return &Task{
		ID:                 NewTaskID(),
		RepoID:             repoID,
		Type:               TaskTypeSetupReview,
		Title:              "Repository setup review",
		Description:        "Review and flesh out the user's repository configuration input.",
		Status:             StatusPending,
		DependsOn:          []string{},
		Attempt:            1,
		MaxAttempts:        3,
		AcceptanceCriteria: []string{},
		SkipPR:             true,
		Ready:              true,
		Model:              "sonnet",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

// NewTask creates a new Task with a generated TaskID and pending status.
func NewTask(repoID, title, description string, dependsOn, acceptanceCriteria []string, maxCostUSD float64, skipPR, draftPR bool, model string, ready bool) *Task {
	now := time.Now()
	if dependsOn == nil {
		dependsOn = []string{}
	}
	if acceptanceCriteria == nil {
		acceptanceCriteria = []string{}
	}
	return &Task{
		ID:                 NewTaskID(),
		RepoID:             repoID,
		Type:               TaskTypeTask,
		Title:              title,
		Description:        description,
		Status:             StatusPending,
		DependsOn:          dependsOn,
		Attempt:            1,
		MaxAttempts:        5,
		AcceptanceCriteria: acceptanceCriteria,
		MaxCostUSD:         maxCostUSD,
		SkipPR:             skipPR,
		DraftPR:            draftPR,
		Ready:              ready,
		Model:              model,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}
