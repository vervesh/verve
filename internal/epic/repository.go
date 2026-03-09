package epic

import (
	"context"
	"time"
)

// Repository is the interface for performing CRUD operations on Epics.
type Repository interface {
	CreateEpic(ctx context.Context, epic *Epic) error
	ReadEpic(ctx context.Context, id EpicID) (*Epic, error)
	ReadEpicByNumber(ctx context.Context, repoID string, number int) (*Epic, error)
	ListEpics(ctx context.Context) ([]*Epic, error)
	ListEpicsByRepo(ctx context.Context, repoID string) ([]*Epic, error)
	UpdateEpic(ctx context.Context, epic *Epic) error
	UpdateEpicStatus(ctx context.Context, id EpicID, status Status) error
	UpdateProposedTasks(ctx context.Context, id EpicID, tasks []ProposedTask) error
	SetTaskIDs(ctx context.Context, id EpicID, taskIDs []string) error
	AppendSessionLog(ctx context.Context, id EpicID, lines []string) error
	DeleteEpic(ctx context.Context, id EpicID) error

	// Worker support
	ListPlanningEpics(ctx context.Context) ([]*Epic, error)
	ClaimEpic(ctx context.Context, id EpicID) (bool, error)
	EpicHeartbeat(ctx context.Context, id EpicID) error
	SetEpicFeedback(ctx context.Context, id EpicID, feedback, feedbackType string) error
	ClearEpicFeedback(ctx context.Context, id EpicID) error
	ReleaseEpicClaim(ctx context.Context, id EpicID) error
	ListStaleEpics(ctx context.Context, threshold time.Time) ([]*Epic, error)

	// ListActiveEpics returns all epics in active status.
	ListActiveEpics(ctx context.Context) ([]*Epic, error)
	// RemoveTaskID removes a task ID from an epic's task_ids array.
	RemoveTaskID(ctx context.Context, id EpicID, taskID string) error
}
