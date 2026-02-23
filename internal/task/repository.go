package task

import (
	"context"
	"time"

	"github.com/joshjon/kit/tx"
)

// Repository is the interface for performing CRUD operations on Tasks.
//
// Implementations must handle all database-specific concerns (error mapping,
// marshalling) and return domain types.
type Repository interface {
	TaskRepository
	tx.Repository[Repository]
}

// TaskRepository defines the data access methods for Tasks.
type TaskRepository interface {
	CreateTask(ctx context.Context, task *Task) error
	ReadTask(ctx context.Context, id TaskID) (*Task, error)
	ListTasks(ctx context.Context) ([]*Task, error)
	ListTasksByRepo(ctx context.Context, repoID string) ([]*Task, error)
	ListPendingTasks(ctx context.Context) ([]*Task, error)
	ListPendingTasksByRepos(ctx context.Context, repoIDs []string) ([]*Task, error)
	AppendTaskLogs(ctx context.Context, id TaskID, attempt int, logs []string) error
	ReadTaskLogs(ctx context.Context, id TaskID) ([]string, error)
	StreamTaskLogs(ctx context.Context, id TaskID, fn func(attempt int, lines []string) error) error
	UpdateTaskStatus(ctx context.Context, id TaskID, status Status) error
	SetTaskPullRequest(ctx context.Context, id TaskID, prURL string, prNumber int) error
	ListTasksInReview(ctx context.Context) ([]*Task, error)
	ListTasksInReviewByRepo(ctx context.Context, repoID string) ([]*Task, error)
	CloseTask(ctx context.Context, id TaskID, reason string) error
	TaskExists(ctx context.Context, id TaskID) (bool, error)
	ReadTaskStatus(ctx context.Context, id TaskID) (Status, error)
	ClaimTask(ctx context.Context, id TaskID) (bool, error)
	HasTasksForRepo(ctx context.Context, repoID string) (bool, error)
	// RetryTask atomically transitions a task from review → pending, increments
	// attempt, and records the retry reason. Returns false if the task was not
	// in review status (already retried or status changed).
	RetryTask(ctx context.Context, id TaskID, reason string) (bool, error)
	// ScheduleRetryFromRunning atomically transitions a task from running → pending,
	// increments attempt, and records the retry reason. Used when the agent hits a
	// retryable error (e.g. Claude rate limit or session max usage exceeded).
	// Returns false if the task was not in running status.
	ScheduleRetryFromRunning(ctx context.Context, id TaskID, reason string) (bool, error)
	SetAgentStatus(ctx context.Context, id TaskID, status string) error
	SetRetryContext(ctx context.Context, id TaskID, retryCtx string) error
	AddCost(ctx context.Context, id TaskID, costUSD float64) error
	SetConsecutiveFailures(ctx context.Context, id TaskID, count int) error
	SetCloseReason(ctx context.Context, id TaskID, reason string) error
	SetBranchName(ctx context.Context, id TaskID, branchName string) error
	ListTasksInReviewNoPR(ctx context.Context) ([]*Task, error)
	ManualRetryTask(ctx context.Context, id TaskID, instructions string) (bool, error)
	// FeedbackRetryTask transitions a task from review → pending and records
	// the user's feedback. Unlike ManualRetryTask, it preserves the existing
	// PR/branch so the agent pushes fixes to the same branch. The attempt
	// counter is reset to 1 so that subsequent automated failure retries get
	// a fresh retry budget — failures after user-requested changes are
	// caused by those changes, not by the original code.
	FeedbackRetryTask(ctx context.Context, id TaskID, feedback string) (bool, error)
	DeleteTaskLogs(ctx context.Context, id TaskID) error
	RemoveDependency(ctx context.Context, id TaskID, depID string) error
	SetReady(ctx context.Context, id TaskID, ready bool) error
	// UpdatePendingTask atomically updates a pending task's editable fields.
	// Returns false if the task was not in pending status.
	UpdatePendingTask(ctx context.Context, id TaskID, params UpdatePendingTaskParams) (bool, error)
	// StartOverTask resets a task from review, failed, or closed back to pending with
	// fresh metadata. Clears logs, PR, branch, agent status, cost, and retry
	// state. Optionally updates title, description, and acceptance criteria.
	// Returns false if the task was not in review, failed, or closed status.
	StartOverTask(ctx context.Context, id TaskID, params StartOverTaskParams) (bool, error)
	// StopTask atomically transitions a task from running → pending with ready=false,
	// recording the stop reason. Returns false if the task was not in running status.
	StopTask(ctx context.Context, id TaskID, reason string) (bool, error)
	// Heartbeat updates the last heartbeat time for a running task.
	// Returns true if the task is still running (row was updated), false if the
	// task no longer exists or is no longer in running status (e.g. stopped,
	// closed, or deleted).
	Heartbeat(ctx context.Context, id TaskID) (bool, error)
	// ListStaleTasks returns running tasks whose last heartbeat is before the given time.
	ListStaleTasks(ctx context.Context, before time.Time) ([]*Task, error)
	DeleteTask(ctx context.Context, id TaskID) error
	// ListTasksByEpic returns all tasks belonging to a given epic.
	ListTasksByEpic(ctx context.Context, epicID string) ([]*Task, error)
	// BulkCloseTasksByEpic closes all non-terminal tasks for an epic.
	BulkCloseTasksByEpic(ctx context.Context, epicID, reason string) error
	// ClearEpicIDForTasks removes the epic_id FK from all tasks for a given epic.
	ClearEpicIDForTasks(ctx context.Context, epicID string) error
	// BulkDeleteTasksByEpic deletes all tasks (and their logs) for a given epic.
	BulkDeleteTasksByEpic(ctx context.Context, epicID string) error
	// BulkDeleteTasksByIDs deletes tasks (and their logs) by their IDs.
	BulkDeleteTasksByIDs(ctx context.Context, ids []string) error
	// DeleteExpiredLogs deletes all log entries older than the given time.
	// Returns the number of log batches deleted.
	DeleteExpiredLogs(ctx context.Context, before time.Time) (int64, error)
}
