package task_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/repo"
	"github.com/joshjon/verve/internal/sqlite"
	"github.com/joshjon/verve/internal/task"
)

// TestManualRetryTask_PreservesPRInfo tests that retrying a failed task
// preserves the PR association (URL, number, and branch name).
func TestManualRetryTask_PreservesPRInfo(t *testing.T) {
	db := sqlite.NewTestDB(t)

	repoRepo := sqlite.NewRepoRepository(db)
	r, err := repo.NewRepo("owner/test-repo")
	require.NoError(t, err)
	require.NoError(t, repoRepo.CreateRepo(context.Background(), r))
	repoID := r.ID.String()

	taskRepo := sqlite.NewTaskRepository(db)
	broker := task.NewBroker(nil)
	store := task.NewStore(taskRepo, broker)
	ctx := context.Background()

	// Create a task and set it up as failed with PR info
	tsk := task.NewTask(repoID, "Fix bug", "desc", nil, nil, 0, false, false, "", true)
	require.NoError(t, taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	require.NoError(t, taskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/owner/repo/pull/42", 42))
	require.NoError(t, taskRepo.SetBranchName(ctx, tsk.ID, "verve/task-tsk_123"))
	require.NoError(t, taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusFailed))

	// Retry the task
	err = store.ManualRetryTask(ctx, tsk.ID, "please try again")
	require.NoError(t, err)

	// Verify PR info is preserved
	updatedTask, err := store.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, task.StatusPending, updatedTask.Status)
	assert.Equal(t, "https://github.com/owner/repo/pull/42", updatedTask.PullRequestURL, "PR URL should be preserved")
	assert.Equal(t, 42, updatedTask.PRNumber, "PR number should be preserved")
	assert.Equal(t, "verve/task-tsk_123", updatedTask.BranchName, "Branch name should be preserved")
}
