package task_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/repo"
	"github.com/joshjon/verve/internal/sqlite"
	"github.com/joshjon/verve/internal/task"
)

type taskFixture struct {
	store    *task.Store
	taskRepo task.Repository
	repoID   string
}

// newTestTaskStore creates a Store backed by a real in-memory SQLite database.
func newTestTaskFixture(t *testing.T) *taskFixture {
	t.Helper()
	db := sqlite.NewTestDB(t)

	// Create a repo first since tasks have a FK to repo.
	repoRepo := sqlite.NewRepoRepository(db)
	r, err := repo.NewRepo("owner/test-repo")
	require.NoError(t, err)
	require.NoError(t, repoRepo.CreateRepo(context.Background(), r))

	taskRepo := sqlite.NewTaskRepository(db)
	broker := task.NewBroker(nil)
	store := task.NewStore(taskRepo, broker)

	return &taskFixture{
		store:    store,
		taskRepo: taskRepo,
		repoID:   r.ID.String(),
	}
}

// newTask creates a new task using the fixture's repo ID.
func (f *taskFixture) newTask(title, desc string, ready bool) *task.Task {
	return task.NewTask(f.repoID, title, desc, nil, nil, 0, false, false, "", ready)
}

// newTaskWithBudget creates a new task with a cost budget.
func (f *taskFixture) newTaskWithBudget(title, desc string, maxCostUSD float64) *task.Task {
	return task.NewTask(f.repoID, title, desc, nil, nil, maxCostUSD, false, false, "", true)
}

// newTaskWithDeps creates a new task with dependencies.
func (f *taskFixture) newTaskWithDeps(title, desc string, deps []string) *task.Task {
	return task.NewTask(f.repoID, title, desc, deps, nil, 0, false, false, "", true)
}

// --- Store tests ---

func TestStore_CreateTask_Success(t *testing.T) {
	f := newTestTaskFixture(t)

	tsk := f.newTask("title", "desc", true)
	err := f.store.CreateTask(context.Background(), tsk)
	require.NoError(t, err)

	read, err := f.store.ReadTask(context.Background(), tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, "title", read.Title)
}

func TestStore_CreateTask_InvalidDependencyID(t *testing.T) {
	f := newTestTaskFixture(t)

	tsk := task.NewTask(f.repoID, "title", "desc", []string{"not-a-valid-id"}, nil, 0, false, false, "", true)
	err := f.store.CreateTask(context.Background(), tsk)
	assert.Error(t, err, "expected error for invalid dependency ID")
}

func TestStore_CreateTask_DependencyNotFound(t *testing.T) {
	f := newTestTaskFixture(t)

	depID := task.NewTaskID()
	tsk := task.NewTask(f.repoID, "title", "desc", []string{depID.String()}, nil, 0, false, false, "", true)
	err := f.store.CreateTask(context.Background(), tsk)
	assert.Error(t, err, "expected error for missing dependency")
}

func TestStore_CreateTask_NotifiesPending(t *testing.T) {
	f := newTestTaskFixture(t)

	tsk := f.newTask("title", "desc", true)
	err := f.store.CreateTask(context.Background(), tsk)
	require.NoError(t, err)

	select {
	case <-f.store.WaitForPending():
		// Good
	default:
		assert.Fail(t, "expected pending notification")
	}
}

func TestStore_CreateTask_PublishesEvent(t *testing.T) {
	f := newTestTaskFixture(t)

	ch := f.store.Subscribe()
	defer f.store.Unsubscribe(ch)

	tsk := f.newTask("title", "desc", true)
	err := f.store.CreateTask(context.Background(), tsk)
	require.NoError(t, err)

	select {
	case event := <-ch:
		assert.Equal(t, task.EventTaskCreated, event.Type)
		assert.Equal(t, f.repoID, event.RepoID)
		assert.NotNil(t, event.Task, "expected non-nil task in event")
		assert.Nil(t, event.Task.Logs, "expected nil logs in published event")
	default:
		assert.Fail(t, "expected event to be published")
	}
}

func TestStore_RetryTask_MaxAttempts(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	require.NoError(t, f.taskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/1", 1))
	// Task starts at attempt=1, maxAttempts=5.
	// Simulate 4 retries to reach attempt=5 (max).
	for i := 0; i < 4; i++ {
		ok, err := f.taskRepo.RetryTask(ctx, tsk.ID, "ci_failure: attempt")
		require.NoError(t, err)
		require.True(t, ok)
		require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
		require.NoError(t, f.taskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/1", 1))
	}
	// Now attempt=5, maxAttempts=5. Store should fail the task.
	err := f.store.RetryTask(ctx, tsk.ID, "ci_failure:tests", "CI tests failed")
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, task.StatusFailed, read.Status)
}

func TestStore_RetryTask_BudgetExceeded(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTaskWithBudget("title", "desc", 5.0)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	require.NoError(t, f.taskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/1", 1))
	require.NoError(t, f.taskRepo.AddCost(ctx, tsk.ID, 6.0))

	err := f.store.RetryTask(ctx, tsk.ID, "", "some reason")
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, task.StatusFailed, read.Status, "expected status failed due to budget exceeded")
}

func TestStore_RetryTask_CircuitBreaker(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	require.NoError(t, f.taskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/1", 1))
	// Set up consecutive failures at 2 with matching retry reason.
	require.NoError(t, f.taskRepo.SetConsecutiveFailures(ctx, tsk.ID, 2))
	ok, err := f.taskRepo.RetryTask(ctx, tsk.ID, "ci_failure:tests: CI tests failed")
	require.NoError(t, err)
	require.True(t, ok)
	// Reset to review for the next store retry call.
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	require.NoError(t, f.taskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/1", 1))
	require.NoError(t, f.taskRepo.SetConsecutiveFailures(ctx, tsk.ID, 2))

	err = f.store.RetryTask(ctx, tsk.ID, "ci_failure:tests", "CI tests failed again")
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, task.StatusFailed, read.Status, "expected status failed due to circuit breaker")
}

func TestStore_RetryTask_CircuitBreakerAllowsSecondRetry(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	require.NoError(t, f.taskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/1", 1))
	require.NoError(t, f.taskRepo.SetConsecutiveFailures(ctx, tsk.ID, 1))
	ok, err := f.taskRepo.RetryTask(ctx, tsk.ID, "ci_failure:tests: CI tests failed")
	require.NoError(t, err)
	require.True(t, ok)
	// Reset to review.
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	require.NoError(t, f.taskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/1", 1))
	require.NoError(t, f.taskRepo.SetConsecutiveFailures(ctx, tsk.ID, 1))

	err = f.store.RetryTask(ctx, tsk.ID, "ci_failure:tests", "CI tests failed again")
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	// Circuit breaker should NOT trigger: same category only twice (threshold is 3)
	assert.NotEqual(t, task.StatusFailed, read.Status, "second consecutive failure should still allow retry")
}

func TestStore_RetryTask_DifferentCategory(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	require.NoError(t, f.taskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/1", 1))
	require.NoError(t, f.taskRepo.SetConsecutiveFailures(ctx, tsk.ID, 1))
	ok, err := f.taskRepo.RetryTask(ctx, tsk.ID, "ci_failure:tests: CI tests failed")
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	require.NoError(t, f.taskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/1", 1))
	require.NoError(t, f.taskRepo.SetConsecutiveFailures(ctx, tsk.ID, 1))

	// Different category should reset consecutive failures
	err = f.store.RetryTask(ctx, tsk.ID, "ci_failure:changelog", "changelog check failed")
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, read.ConsecutiveFailures, "different category should reset consecutive failures to 1")
}

func TestStore_RetryTask_MergeConflictIgnoresMaxAttempts(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	require.NoError(t, f.taskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/1", 1))
	// Simulate being at max attempts
	for i := 0; i < 4; i++ {
		ok, err := f.taskRepo.RetryTask(ctx, tsk.ID, "ci_failure: attempt")
		require.NoError(t, err)
		require.True(t, ok)
		require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
		require.NoError(t, f.taskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/1", 1))
	}

	// Merge conflict retries should NOT be blocked by max attempts
	err := f.store.RetryTask(ctx, tsk.ID, "merge_conflict", "merge_conflict: PR has conflicts with base branch")
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.NotEqual(t, task.StatusFailed, read.Status,
		"merge conflict retry should not fail task at max attempts")
}

func TestStore_RetryTask_MergeConflictRespectsMaxBudget(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTaskWithBudget("title", "desc", 5.0)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	require.NoError(t, f.taskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/1", 1))
	require.NoError(t, f.taskRepo.AddCost(ctx, tsk.ID, 6.0))

	err := f.store.RetryTask(ctx, tsk.ID, "merge_conflict", "merge_conflict: PR has conflicts with base branch")
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, task.StatusFailed, read.Status,
		"merge conflict retry should still fail when budget exceeded")
}

func TestStore_ManualRetryTask(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusFailed))

	err := f.store.ManualRetryTask(ctx, tsk.ID, "try again please")
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, task.StatusPending, read.Status)
}

func TestStore_ManualRetryTask_NotFailed(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))

	err := f.store.ManualRetryTask(ctx, tsk.ID, "")
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, task.StatusRunning, read.Status)
}

func TestStore_FeedbackRetryTask_BudgetExceeded(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTaskWithBudget("title", "desc", 5.0)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	require.NoError(t, f.taskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/1", 1))
	require.NoError(t, f.taskRepo.AddCost(ctx, tsk.ID, 6.0))

	err := f.store.FeedbackRetryTask(ctx, tsk.ID, "fix the tests")
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, task.StatusFailed, read.Status, "expected status failed due to budget exceeded")
}

func TestStore_FeedbackRetryTask_IgnoresMaxAttempts(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	require.NoError(t, f.taskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/1", 1))
	for i := 0; i < 4; i++ {
		ok, err := f.taskRepo.RetryTask(ctx, tsk.ID, "ci_failure: attempt")
		require.NoError(t, err)
		require.True(t, ok)
		require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
		require.NoError(t, f.taskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/1", 1))
	}

	// Feedback retries should NOT be blocked by max attempts
	err := f.store.FeedbackRetryTask(ctx, tsk.ID, "fix the tests")
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.NotEqual(t, task.StatusFailed, read.Status, "feedback should not fail task at max attempts")
}

func TestStore_FeedbackRetryTask_IncrementsAttemptAndMaxAttempts(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	require.NoError(t, f.taskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/1", 1))
	for i := 0; i < 3; i++ {
		ok, err := f.taskRepo.RetryTask(ctx, tsk.ID, "ci_failure: attempt")
		require.NoError(t, err)
		require.True(t, ok)
		require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
		require.NoError(t, f.taskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/1", 1))
	}

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, 4, read.Attempt)

	err = f.store.FeedbackRetryTask(ctx, tsk.ID, "please update the error messages")
	require.NoError(t, err)

	read, err = f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, 5, read.Attempt, "attempt should be incremented after feedback retry")
	assert.Equal(t, 6, read.MaxAttempts, "max_attempts should be incremented to preserve retry budget")
	assert.Equal(t, 0, read.ConsecutiveFailures, "consecutive failures should be reset after feedback retry")
}

func TestStore_FeedbackRetryTask_ClearsRetryContext(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	require.NoError(t, f.taskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/1", 1))
	require.NoError(t, f.taskRepo.SetRetryContext(ctx, tsk.ID, "CI failure logs from previous attempt..."))

	err := f.store.FeedbackRetryTask(ctx, tsk.ID, "please fix the formatting")
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Empty(t, read.RetryContext, "retry context should be cleared after feedback retry")
}

func TestStore_FeedbackRetryTask_ThenAutomatedRetryGetsFullBudget(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	require.NoError(t, f.taskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/1", 1))
	for i := 0; i < 3; i++ {
		ok, err := f.taskRepo.RetryTask(ctx, tsk.ID, "ci_failure: attempt")
		require.NoError(t, err)
		require.True(t, ok)
		require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
		require.NoError(t, f.taskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/1", 1))
	}

	// Step 1: Feedback retry increments both attempt and max_attempts
	err := f.store.FeedbackRetryTask(ctx, tsk.ID, "update error handling")
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, 5, read.Attempt, "attempt should be incremented after feedback")
	assert.Equal(t, 6, read.MaxAttempts, "max_attempts should be incremented to preserve budget")

	// Simulate: agent runs again and ends up in review with attempt=5
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	require.NoError(t, f.taskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/1", 1))

	// Step 2: Automated retry after CI failure should NOT be blocked
	err = f.store.RetryTask(ctx, tsk.ID, "ci_failure:tests", "CI tests failed")
	require.NoError(t, err)

	read, err = f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.NotEqual(t, task.StatusFailed, read.Status,
		"automated retry should succeed after feedback incremented the budget")
}

func TestStore_ClaimPendingTask_NoPending(t *testing.T) {
	f := newTestTaskFixture(t)

	claimed, err := f.store.ClaimPendingTask(context.Background(), nil)
	require.NoError(t, err)
	assert.Nil(t, claimed, "expected nil claimed task when no pending tasks")
}

func TestStore_ClaimPendingTask_Success(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))

	claimed, err := f.store.ClaimPendingTask(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, claimed, "expected non-nil claimed task")
	assert.Equal(t, task.StatusRunning, claimed.Status)
}

func TestStore_ClaimPendingTask_WithRepoFilter(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))

	claimed, err := f.store.ClaimPendingTask(ctx, []string{f.repoID})
	require.NoError(t, err)
	require.NotNil(t, claimed, "expected non-nil claimed task")
}

func TestStore_AppendTaskLogs(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))

	err := f.store.AppendTaskLogs(ctx, tsk.ID, 1, []string{"line 1", "line 2"})
	require.NoError(t, err)

	logs, err := f.store.ReadTaskLogs(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Len(t, logs, 2)
}

func TestStore_WaitForPending(t *testing.T) {
	f := newTestTaskFixture(t)

	ch := f.store.WaitForPending()
	select {
	case <-ch:
		assert.Fail(t, "expected no pending notification initially")
	default:
	}

	tsk := f.newTask("title", "desc", true)
	_ = f.store.CreateTask(context.Background(), tsk)

	select {
	case <-ch:
		// Good
	default:
		assert.Fail(t, "expected pending notification after create")
	}
}

func TestStore_CloseTask(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))

	err := f.store.CloseTask(ctx, tsk.ID, "no longer needed")
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, task.StatusClosed, read.Status)
	assert.Equal(t, "no longer needed", read.CloseReason)
}

func TestStore_UpdateTaskStatus(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))

	err := f.store.UpdateTaskStatus(ctx, tsk.ID, task.StatusFailed)
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, task.StatusFailed, read.Status)
}

func TestStore_SetTaskPullRequest(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))

	err := f.store.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/42", 42)
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/org/repo/pull/42", read.PullRequestURL)
	assert.Equal(t, 42, read.PRNumber)
}

func TestStore_RemoveDependency_Success(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	dep := f.newTask("dep", "dep desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, dep))

	tsk := f.newTaskWithDeps("title", "desc", []string{dep.ID.String()})
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))

	err := f.store.RemoveDependency(ctx, tsk.ID, dep.ID.String())
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Empty(t, read.DependsOn)
}

func TestStore_RemoveDependency_InvalidDepID(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))

	err := f.store.RemoveDependency(ctx, tsk.ID, "not-a-valid-id")
	assert.Error(t, err, "expected error for invalid dependency ID")
}

func TestStore_RemoveDependency_NotifiesPending(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	dep := f.newTask("dep", "dep desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, dep))

	tsk := f.newTaskWithDeps("title", "desc", []string{dep.ID.String()})
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))

	select {
	case <-f.store.WaitForPending():
	default:
	}

	err := f.store.RemoveDependency(ctx, tsk.ID, dep.ID.String())
	require.NoError(t, err)

	select {
	case <-f.store.WaitForPending():
		// Good
	default:
		assert.Fail(t, "expected pending notification after removing dependency")
	}
}

func TestStore_ScheduleRetry_Success(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))

	err := f.store.ScheduleRetry(ctx, tsk.ID, "rate_limit: Claude max usage exceeded")
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, task.StatusPending, read.Status, "expected task to transition to pending for retry")
}

func TestStore_ScheduleRetry_MaxAttempts(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	for i := 0; i < 4; i++ {
		ok, err := f.taskRepo.ScheduleRetryFromRunning(ctx, tsk.ID, "rate_limit: max usage")
		require.NoError(t, err)
		require.True(t, ok)
		require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	}

	err := f.store.ScheduleRetry(ctx, tsk.ID, "rate_limit: max usage")
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, task.StatusFailed, read.Status, "expected task to fail when max attempts reached")
}

func TestStore_ScheduleRetry_BudgetExceeded(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTaskWithBudget("title", "desc", 5.0)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	require.NoError(t, f.taskRepo.AddCost(ctx, tsk.ID, 6.0))

	err := f.store.ScheduleRetry(ctx, tsk.ID, "rate_limit: max usage")
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, task.StatusFailed, read.Status, "expected task to fail when budget exceeded")
}

func TestStore_ScheduleRetry_CircuitBreaker(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	require.NoError(t, f.taskRepo.SetConsecutiveFailures(ctx, tsk.ID, 2))
	ok, err := f.taskRepo.ScheduleRetryFromRunning(ctx, tsk.ID, "rate_limit: Claude max usage exceeded")
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	require.NoError(t, f.taskRepo.SetConsecutiveFailures(ctx, tsk.ID, 2))

	err = f.store.ScheduleRetry(ctx, tsk.ID, "rate_limit: Claude max usage exceeded")
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, task.StatusFailed, read.Status, "expected task to fail due to circuit breaker")
}

func TestStore_DeleteTask_WithLogs(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusFailed))
	require.NoError(t, f.taskRepo.AppendTaskLogs(ctx, tsk.ID, 1, []string{"log line 1", "log line 2"}))

	err := f.store.DeleteTask(ctx, tsk.ID)
	require.NoError(t, err)

	_, err = f.taskRepo.ReadTask(ctx, tsk.ID)
	assert.Error(t, err, "expected task to be deleted")
}

func TestStore_DeleteTask_RemovesDependencies(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusFailed))

	dependent := f.newTaskWithDeps("dependent", "desc", []string{tsk.ID.String()})
	require.NoError(t, f.taskRepo.CreateTask(ctx, dependent))

	err := f.store.DeleteTask(ctx, tsk.ID)
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, dependent.ID)
	require.NoError(t, err)
	assert.Empty(t, read.DependsOn, "expected dependency to be removed")
}

func TestStore_SetAgentStatus_MergesFilesAcrossRetries(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	require.NoError(t, f.taskRepo.SetAgentStatus(ctx, tsk.ID, `{"files_modified":["main.go","config.go"],"tests_status":"fail","confidence":"medium"}`))

	newStatus := `{"files_modified":["main.go","handler.go"],"tests_status":"pass","confidence":"high"}`
	err := f.store.SetAgentStatus(ctx, tsk.ID, newStatus)
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Contains(t, read.AgentStatus, `"main.go"`)
	assert.Contains(t, read.AgentStatus, `"handler.go"`)
	assert.Contains(t, read.AgentStatus, `"config.go"`)
	assert.Contains(t, read.AgentStatus, `"pass"`)
	assert.Contains(t, read.AgentStatus, `"high"`)
}

func TestStore_SetAgentStatus_NoPreviousStatus(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))

	newStatus := `{"files_modified":["main.go"],"tests_status":"pass","confidence":"high"}`
	err := f.store.SetAgentStatus(ctx, tsk.ID, newStatus)
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, newStatus, read.AgentStatus)
}

func TestStore_SetAgentStatus_EmptyNewFiles(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	require.NoError(t, f.taskRepo.SetAgentStatus(ctx, tsk.ID, `{"files_modified":["main.go","config.go"],"tests_status":"fail"}`))

	newStatus := `{"files_modified":[],"tests_status":"pass","confidence":"high"}`
	err := f.store.SetAgentStatus(ctx, tsk.ID, newStatus)
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Contains(t, read.AgentStatus, `"main.go"`)
	assert.Contains(t, read.AgentStatus, `"config.go"`)
}

func TestStore_SetAgentStatus_InvalidJSON(t *testing.T) {
	f := newTestTaskFixture(t)
	ctx := context.Background()

	tsk := f.newTask("title", "desc", true)
	require.NoError(t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	require.NoError(t, f.taskRepo.SetAgentStatus(ctx, tsk.ID, `not valid json`))

	newStatus := `{"files_modified":["main.go"],"tests_status":"pass"}`
	err := f.store.SetAgentStatus(ctx, tsk.ID, newStatus)
	require.NoError(t, err)

	read, err := f.taskRepo.ReadTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Equal(t, newStatus, read.AgentStatus)
}

func TestMergeAgentStatusFiles(t *testing.T) {
	db := sqlite.NewTestDB(t)
	repoRepo := sqlite.NewRepoRepository(db)
	r, err := repo.NewRepo("owner/test-repo")
	require.NoError(t, err)
	require.NoError(t, repoRepo.CreateRepo(context.Background(), r))
	taskRepo := sqlite.NewTaskRepository(db)
	repoID := r.ID.String()

	tests := []struct {
		name      string
		oldStatus string
		newStatus string
		wantFiles []string
	}{
		{
			name:      "merges unique files from old and new",
			oldStatus: `{"files_modified":["a.go","b.go"],"tests_status":"fail"}`,
			newStatus: `{"files_modified":["b.go","c.go"],"tests_status":"pass"}`,
			wantFiles: []string{"b.go", "c.go", "a.go"},
		},
		{
			name:      "no old files",
			oldStatus: `{"files_modified":[],"tests_status":"fail"}`,
			newStatus: `{"files_modified":["a.go"],"tests_status":"pass"}`,
			wantFiles: []string{"a.go"},
		},
		{
			name:      "no old status",
			oldStatus: "",
			newStatus: `{"files_modified":["a.go"],"tests_status":"pass"}`,
			wantFiles: []string{"a.go"},
		},
		{
			name:      "identical files",
			oldStatus: `{"files_modified":["a.go","b.go"]}`,
			newStatus: `{"files_modified":["a.go","b.go"]}`,
			wantFiles: []string{"a.go", "b.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tsk := task.NewTask(repoID, "title", "desc", nil, nil, 0, false, false, "", true)
			require.NoError(t, taskRepo.CreateTask(ctx, tsk))
			if tt.oldStatus != "" {
				require.NoError(t, taskRepo.SetAgentStatus(ctx, tsk.ID, tt.oldStatus))
			}

			broker := task.NewBroker(nil)
			store := task.NewStore(taskRepo, broker)
			require.NoError(t, store.SetAgentStatus(ctx, tsk.ID, tt.newStatus))

			read, err := taskRepo.ReadTask(ctx, tsk.ID)
			require.NoError(t, err)

			var parsed map[string]interface{}
			err = json.Unmarshal([]byte(read.AgentStatus), &parsed)
			require.NoError(t, err)

			filesRaw, ok := parsed["files_modified"].([]interface{})
			require.True(t, ok)
			var files []string
			for _, f := range filesRaw {
				files = append(files, f.(string))
			}
			assert.Equal(t, tt.wantFiles, files)
		})
	}
}
