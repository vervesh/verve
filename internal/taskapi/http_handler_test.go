package taskapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/joshjon/kit/server"
	"github.com/joshjon/kit/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/task"
	"github.com/joshjon/verve/internal/taskapi"
)

// --- CreateTask ---

func TestCreateTask_Success(t *testing.T) {
	f := newFixture(t)

	req := taskapi.CreateTaskRequest{
		Title:       "Fix bug",
		Description: "Fix the login bug",
	}
	res := testutil.Post[server.Response[task.Task]](t, f.repoTasksURL(), req)
	assert.Equal(t, "Fix bug", res.Data.Title)
	assert.Equal(t, task.StatusPending, res.Data.Status)
	assert.Equal(t, "sonnet", res.Data.Model)
}

func TestCreateTask_EmptyTitle(t *testing.T) {
	f := newFixture(t)

	req := taskapi.CreateTaskRequest{
		Title:       "",
		Description: "some desc",
	}
	httpRes := doJSON(t, http.MethodPost, f.repoTasksURL(), req)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode, "expected validation error for empty title")
}

func TestCreateTask_TitleTooLong(t *testing.T) {
	f := newFixture(t)

	req := taskapi.CreateTaskRequest{
		Title:       strings.Repeat("a", 151),
		Description: "desc",
	}
	httpRes := doJSON(t, http.MethodPost, f.repoTasksURL(), req)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode, "expected validation error for long title")
}

func TestCreateTask_InvalidRepoID(t *testing.T) {
	f := newFixture(t)

	req := taskapi.CreateTaskRequest{Title: "Fix bug"}
	// Post to an invalid repo ID URL.
	url := f.Server.Address() + "/api/v1/repos/invalid/tasks"
	httpRes := doJSON(t, http.MethodPost, url, req)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode, "expected validation error for invalid repo ID")
}

func TestCreateTask_WithModel(t *testing.T) {
	f := newFixture(t)

	req := taskapi.CreateTaskRequest{
		Title:       "Fix bug",
		Description: "desc",
		Model:       "opus",
	}
	res := testutil.Post[server.Response[task.Task]](t, f.repoTasksURL(), req)
	assert.Equal(t, "opus", res.Data.Model)
}

// --- GetTask ---

func TestGetTask_Success(t *testing.T) {
	f := newFixture(t)
	tsk := f.seedTask("title", "desc")

	res := testutil.Get[server.Response[task.Task]](t, f.taskURL(tsk.ID))
	assert.Equal(t, "title", res.Data.Title)
}

func TestGetTask_InvalidID(t *testing.T) {
	f := newFixture(t)

	url := f.Server.Address() + "/api/v1/tasks/invalid"
	httpRes, err := testutil.DefaultClient.Get(url)
	require.NoError(t, err)
	defer httpRes.Body.Close()

	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode, "expected validation error for invalid task ID")
}

// --- AppendLogs ---

func TestAppendLogs_Success(t *testing.T) {
	f := newFixture(t)
	tsk := f.seedTask("title", "desc")

	req := taskapi.LogsRequest{
		Logs:    []string{"line 1", "line 2"},
		Attempt: 1,
	}
	postNoContent(t, f.taskActionURL(tsk.ID, "logs"), req)
}

func TestAppendLogs_DefaultAttempt(t *testing.T) {
	f := newFixture(t)
	tsk := f.seedTask("title", "desc")

	req := taskapi.LogsRequest{
		Logs: []string{"line 1"},
	}
	postNoContent(t, f.taskActionURL(tsk.ID, "logs"), req)
}

// --- CompleteTask ---

func TestCompleteTask_Failure(t *testing.T) {
	f := newFixture(t)
	tsk := f.seedRunningTask("title", "desc")

	req := taskapi.CompleteRequest{
		Success: false,
		Error:   "exit code 1",
	}
	postNoContent(t, f.taskActionURL(tsk.ID, "complete"), req)

	updated := f.readTask(tsk.ID)
	assert.Equal(t, task.StatusFailed, updated.Status)
}

func TestCompleteTask_FailureWithExistingPR_FailedNotReview(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	tsk := f.seedTask("title", "desc")
	require.NoError(t, f.TaskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/10", 10))
	require.NoError(t, f.TaskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))

	req := taskapi.CompleteRequest{
		Success: false,
		Error:   "exit code 1",
	}
	postNoContent(t, f.taskActionURL(tsk.ID, "complete"), req)

	updated := f.readTask(tsk.ID)
	assert.Equal(t, task.StatusFailed, updated.Status, "expected failed even when task has existing PR")
}

func TestCompleteTask_FailureWithExistingBranch_FailedNotReview(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	tsk := f.seedTask("title", "desc")
	require.NoError(t, f.TaskRepo.SetBranchName(ctx, tsk.ID, "verve/task-tsk_123"))
	require.NoError(t, f.TaskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))

	req := taskapi.CompleteRequest{
		Success: false,
		Error:   "exit code 1",
	}
	postNoContent(t, f.taskActionURL(tsk.ID, "complete"), req)

	updated := f.readTask(tsk.ID)
	assert.Equal(t, task.StatusFailed, updated.Status, "expected failed even when task has existing branch")
}

func TestCompleteTask_FailureWithPrereqFailed_FailedEvenWithPR(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	tsk := f.seedTask("title", "desc")
	require.NoError(t, f.TaskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/10", 10))
	require.NoError(t, f.TaskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))

	req := taskapi.CompleteRequest{
		Success:      false,
		PrereqFailed: "missing deps",
	}
	postNoContent(t, f.taskActionURL(tsk.ID, "complete"), req)

	updated := f.readTask(tsk.ID)
	assert.Equal(t, task.StatusFailed, updated.Status, "expected failed when prereq_failed is set")
}

func TestCompleteTask_SuccessWithPR(t *testing.T) {
	f := newFixture(t)
	tsk := f.seedRunningTask("title", "desc")

	req := taskapi.CompleteRequest{
		Success:        true,
		PullRequestURL: "https://github.com/org/repo/pull/42",
		PRNumber:       42,
	}
	postNoContent(t, f.taskActionURL(tsk.ID, "complete"), req)

	updated := f.readTask(tsk.ID)
	assert.Equal(t, task.StatusReview, updated.Status)
	assert.Equal(t, 42, updated.PRNumber)
}

func TestCompleteTask_SuccessWithBranch(t *testing.T) {
	f := newFixture(t)
	tsk := f.seedRunningTask("title", "desc")

	req := taskapi.CompleteRequest{
		Success:    true,
		BranchName: "verve/task-tsk_123",
	}
	postNoContent(t, f.taskActionURL(tsk.ID, "complete"), req)

	updated := f.readTask(tsk.ID)
	assert.Equal(t, "verve/task-tsk_123", updated.BranchName)
}

func TestCompleteTask_SuccessNoPR_ClosedIfNoExistingPR(t *testing.T) {
	f := newFixture(t)
	tsk := f.seedRunningTask("title", "desc")

	req := taskapi.CompleteRequest{Success: true}
	postNoContent(t, f.taskActionURL(tsk.ID, "complete"), req)

	updated := f.readTask(tsk.ID)
	assert.Equal(t, task.StatusClosed, updated.Status, "expected status closed (no PR)")
}

func TestCompleteTask_SuccessNoPR_ReviewIfExistingPR(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	tsk := f.seedTask("title", "desc")
	require.NoError(t, f.TaskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/10", 10))
	require.NoError(t, f.TaskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))

	req := taskapi.CompleteRequest{Success: true}
	postNoContent(t, f.taskActionURL(tsk.ID, "complete"), req)

	updated := f.readTask(tsk.ID)
	assert.Equal(t, task.StatusReview, updated.Status, "expected status review (existing PR)")
}

func TestCompleteTask_SuccessNoChanges_ClosedWithReason(t *testing.T) {
	f := newFixture(t)
	tsk := f.seedRunningTask("title", "desc")

	req := taskapi.CompleteRequest{
		Success:   true,
		NoChanges: true,
	}
	postNoContent(t, f.taskActionURL(tsk.ID, "complete"), req)

	updated := f.readTask(tsk.ID)
	assert.Equal(t, task.StatusClosed, updated.Status, "expected status closed (no changes needed)")
	assert.Contains(t, updated.CloseReason, "No changes needed", "expected close reason to mention no changes")
}

func TestCompleteTask_WithAgentStatus(t *testing.T) {
	f := newFixture(t)
	tsk := f.seedRunningTask("title", "desc")

	req := taskapi.CompleteRequest{
		Success:     false,
		AgentStatus: `{"confidence":"high"}`,
		CostUSD:     1.5,
	}
	postNoContent(t, f.taskActionURL(tsk.ID, "complete"), req)
}

func TestCompleteTask_RetryableFailure_SchedulesRetry(t *testing.T) {
	f := newFixture(t)
	tsk := f.seedRunningTask("title", "desc")

	req := taskapi.CompleteRequest{
		Success:   false,
		Error:     "Claude max usage exceeded",
		Retryable: true,
	}
	postNoContent(t, f.taskActionURL(tsk.ID, "complete"), req)

	updated := f.readTask(tsk.ID)
	assert.Equal(t, task.StatusPending, updated.Status, "expected task to be scheduled for retry")
	assert.Equal(t, 2, updated.Attempt, "expected attempt to be incremented")
}

func TestCompleteTask_RetryableFailure_MaxAttemptsReached(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	tsk := task.NewTask(f.Repo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet", true)
	tsk.Attempt = 5
	tsk.MaxAttempts = 5
	require.NoError(t, f.TaskRepo.CreateTask(ctx, tsk))
	require.NoError(t, f.TaskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))

	req := taskapi.CompleteRequest{
		Success:   false,
		Error:     "Claude rate limit exceeded",
		Retryable: true,
	}
	postNoContent(t, f.taskActionURL(tsk.ID, "complete"), req)

	updated := f.readTask(tsk.ID)
	assert.Equal(t, task.StatusFailed, updated.Status, "expected task to fail when max attempts reached")
}

func TestCompleteTask_RetryableWithPrereqFailed_NotRetried(t *testing.T) {
	f := newFixture(t)
	tsk := f.seedRunningTask("title", "desc")

	req := taskapi.CompleteRequest{
		Success:      false,
		Error:        "prereq issue",
		Retryable:    true,
		PrereqFailed: "missing deps",
	}
	postNoContent(t, f.taskActionURL(tsk.ID, "complete"), req)

	updated := f.readTask(tsk.ID)
	assert.Equal(t, task.StatusFailed, updated.Status, "prereq failures should not be retried even if retryable flag is set")
}

func TestCompleteTask_TransientFailure_SchedulesRetry(t *testing.T) {
	f := newFixture(t)
	tsk := f.seedRunningTask("title", "desc")

	req := taskapi.CompleteRequest{
		Success:   false,
		Error:     "failed to create container verve-task-tsk_123: connection refused",
		Retryable: true,
	}
	postNoContent(t, f.taskActionURL(tsk.ID, "complete"), req)

	updated := f.readTask(tsk.ID)
	assert.Equal(t, task.StatusPending, updated.Status, "expected task to be scheduled for retry on transient error")
	assert.Equal(t, 2, updated.Attempt, "expected attempt to be incremented")
	assert.Contains(t, updated.RetryReason, "transient:", "expected retry reason to have transient category prefix")
}

func TestCompleteTask_NetworkFailure_SchedulesRetry(t *testing.T) {
	f := newFixture(t)
	tsk := f.seedRunningTask("title", "desc")

	req := taskapi.CompleteRequest{
		Success:   false,
		Error:     "fatal: unable to access 'https://github.com/org/repo.git/': Could not resolve host: github.com",
		Retryable: true,
	}
	postNoContent(t, f.taskActionURL(tsk.ID, "complete"), req)

	updated := f.readTask(tsk.ID)
	assert.Equal(t, task.StatusPending, updated.Status, "expected task to be scheduled for retry on network error")
	assert.Equal(t, 2, updated.Attempt, "expected attempt to be incremented")
	assert.Contains(t, updated.RetryReason, "transient:", "expected retry reason to have transient category prefix")
}

// --- CloseTask ---

func TestCloseTask_Success(t *testing.T) {
	f := newFixture(t)
	tsk := f.seedRunningTask("title", "desc")

	req := taskapi.CloseRequest{Reason: "no longer needed"}
	res := testutil.Post[server.Response[task.Task]](t, f.taskActionURL(tsk.ID, "close"), req)
	assert.Equal(t, task.StatusClosed, res.Data.Status)
}

// --- MoveToReview ---

func TestMoveToReview_FailedWithPR(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	tsk := f.seedTask("title", "desc")
	require.NoError(t, f.TaskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/10", 10))
	require.NoError(t, f.TaskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusFailed))

	res := testutil.Post[server.Response[task.Task]](t, f.taskActionURL(tsk.ID, "move-to-review"), nil)
	assert.Equal(t, task.StatusReview, res.Data.Status)
}

func TestMoveToReview_FailedWithBranch(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	tsk := f.seedTask("title", "desc")
	require.NoError(t, f.TaskRepo.SetBranchName(ctx, tsk.ID, "verve/task-tsk_123"))
	require.NoError(t, f.TaskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusFailed))

	res := testutil.Post[server.Response[task.Task]](t, f.taskActionURL(tsk.ID, "move-to-review"), nil)
	assert.Equal(t, task.StatusReview, res.Data.Status)
}

func TestMoveToReview_FailedNoPR_Rejected(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	tsk := f.seedTask("title", "desc")
	require.NoError(t, f.TaskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusFailed))

	httpRes := doJSON(t, http.MethodPost, f.taskActionURL(tsk.ID, "move-to-review"), nil)
	defer httpRes.Body.Close()
	assert.True(t, httpRes.StatusCode >= 400, "expected error when task has no PR or branch")

	updated := f.readTask(tsk.ID)
	assert.Equal(t, task.StatusFailed, updated.Status, "status should remain failed")
}

func TestMoveToReview_NotFailed_Rejected(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	tsk := f.seedTask("title", "desc")
	require.NoError(t, f.TaskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/org/repo/pull/10", 10))
	require.NoError(t, f.TaskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))

	httpRes := doJSON(t, http.MethodPost, f.taskActionURL(tsk.ID, "move-to-review"), nil)
	defer httpRes.Body.Close()
	assert.True(t, httpRes.StatusCode >= 400, "expected error when task is not in failed status")

	updated := f.readTask(tsk.ID)
	assert.Equal(t, task.StatusRunning, updated.Status, "status should remain running")
}

// --- ListTasksByRepo ---

func TestListTasksByRepo_Success(t *testing.T) {
	f := newFixture(t)

	f.seedTask("task 1", "desc")
	f.seedTask("task 2", "desc")

	res := testutil.Get[server.ResponseList[task.Task]](t, f.repoTasksURL())
	assert.Len(t, res.Data, 2)
}

// --- GetTaskChecks ---

func TestGetTaskChecks_NoPR(t *testing.T) {
	f := newFixture(t)
	tsk := f.seedTask("title", "desc")

	res := testutil.Get[server.Response[taskapi.CheckStatusResponse]](t, f.taskActionURL(tsk.ID, "checks"))
	assert.Equal(t, "success", res.Data.Status, "expected status 'success' for no CI")
}

// --- FeedbackTask ---

func TestFeedbackTask_EmptyFeedback(t *testing.T) {
	f := newFixture(t)
	tsk := f.seedTask("title", "desc")

	req := taskapi.FeedbackRequest{Feedback: ""}
	httpRes := doJSON(t, http.MethodPost, f.taskActionURL(tsk.ID, "feedback"), req)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode, "expected validation error for empty feedback")
}

// --- RemoveDependency ---

func TestRemoveDependency_Success(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	dep := f.seedTask("dep", "dep desc")

	tsk := task.NewTask(f.Repo.ID.String(), "title", "desc", []string{dep.ID.String()}, nil, 0, false, "sonnet", true)
	require.NoError(t, f.TaskRepo.CreateTask(ctx, tsk))

	req := taskapi.RemoveDependencyRequest{DependsOn: dep.ID.String()}
	// RemoveDependency uses DELETE with a JSON body, so use doJSON.
	httpRes := doJSON(t, http.MethodDelete, f.taskActionURL(tsk.ID, "dependency"), req)
	defer httpRes.Body.Close()
	assert.True(t, httpRes.StatusCode >= 200 && httpRes.StatusCode < 300, "expected success")

	updated := f.readTask(tsk.ID)
	assert.Empty(t, updated.DependsOn)
}

func TestRemoveDependency_InvalidTaskID(t *testing.T) {
	f := newFixture(t)

	req := taskapi.RemoveDependencyRequest{DependsOn: "tsk_abc"}
	url := f.Server.Address() + "/api/v1/tasks/invalid/dependency"
	httpRes := doJSON(t, http.MethodDelete, url, req)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode, "expected validation error for invalid task ID")
}

func TestRemoveDependency_EmptyDependsOn(t *testing.T) {
	f := newFixture(t)
	tsk := f.seedTask("title", "desc")

	req := taskapi.RemoveDependencyRequest{DependsOn: ""}
	httpRes := doJSON(t, http.MethodDelete, f.taskActionURL(tsk.ID, "dependency"), req)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode, "expected validation error for empty depends_on")
}

// --- DeleteTask ---

func TestDeleteTask_FailedWithLogs(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	tsk := f.seedTask("title", "desc")
	require.NoError(t, f.TaskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusFailed))
	require.NoError(t, f.TaskRepo.AppendTaskLogs(ctx, tsk.ID, 1, []string{"error log line 1", "error log line 2"}))

	testutil.Delete(t, f.taskURL(tsk.ID))

	// Verify task was deleted.
	_, readErr := f.TaskRepo.ReadTask(ctx, tsk.ID)
	assert.Error(t, readErr, "expected task to be deleted")

	// Verify logs were deleted.
	logs, logsErr := f.TaskRepo.ReadTaskLogs(ctx, tsk.ID)
	assert.NoError(t, logsErr)
	assert.Empty(t, logs, "expected task logs to be deleted")
}

func TestDeleteTask_InvalidID(t *testing.T) {
	f := newFixture(t)

	url := f.Server.Address() + "/api/v1/tasks/invalid"
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	require.NoError(t, err)

	httpRes, err := testutil.DefaultClient.Do(req)
	require.NoError(t, err)
	defer httpRes.Body.Close()

	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode, "expected validation error for invalid task ID")
}

// --- classifyRetryReason (internal unit test) ---

func TestClassifyRetryReason(t *testing.T) {
	tests := []struct {
		name       string
		errMsg     string
		wantPrefix string
	}{
		{"rate limit", "Claude rate limit exceeded", "rate_limit: "},
		{"max usage", "Claude max usage exceeded", "rate_limit: "},
		{"too many requests", "API returned Too many requests", "rate_limit: "},
		{"overloaded error", "overloaded_error from API", "rate_limit: "},
		{"network error", "Could not resolve host: github.com", "transient: "},
		{"connection refused", "connection refused", "transient: "},
		{"connection timeout", "connection timed out", "transient: "},
		{"DNS failure", "temporary failure in name resolution", "transient: "},
		{"docker create error", "failed to create container: OCI error", "transient: "},
		{"docker start error", "failed to start container: no space left", "transient: "},
		{"unknown retryable", "some unknown error", "transient: "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := taskapi.ClassifyRetryReason(tt.errMsg)
			assert.True(t, strings.HasPrefix(result, tt.wantPrefix),
				"expected prefix %q, got %q", tt.wantPrefix, result)
			assert.Contains(t, result, tt.errMsg, "expected result to contain original error message")
		})
	}
}
