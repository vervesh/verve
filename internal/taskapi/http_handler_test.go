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

	"github.com/joshjon/verve/internal/repo"
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

func TestCreateTask_WithDraftPR(t *testing.T) {
	f := newFixture(t)

	req := taskapi.CreateTaskRequest{
		Title:       "Fix bug",
		Description: "desc",
		DraftPR:     true,
	}
	res := testutil.Post[server.Response[task.Task]](t, f.repoTasksURL(), req)
	assert.Equal(t, true, res.Data.DraftPR)
	assert.Equal(t, false, res.Data.SkipPR)
}

func TestCreateTask_SkipPRAndDraftPR_MutuallyExclusive(t *testing.T) {
	f := newFixture(t)

	req := taskapi.CreateTaskRequest{
		Title:       "Fix bug",
		Description: "desc",
		SkipPR:      true,
		DraftPR:     true,
	}
	httpRes := doJSON(t, http.MethodPost, f.repoTasksURL(), req)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode, "expected validation error for mutually exclusive skip_pr and draft_pr")
}

func TestCreateTask_WithSkipPR(t *testing.T) {
	f := newFixture(t)

	req := taskapi.CreateTaskRequest{
		Title:       "Fix bug",
		Description: "desc",
		SkipPR:      true,
	}
	res := testutil.Post[server.Response[task.Task]](t, f.repoTasksURL(), req)
	assert.Equal(t, true, res.Data.SkipPR)
	assert.Equal(t, false, res.Data.DraftPR)
}

func TestCreateTask_BlockedWhenRepoNotReady(t *testing.T) {
	f := newFixture(t)

	// Set repo to scanning (not ready)
	ctx := context.Background()
	require.NoError(t, f.RepoStore.UpdateRepoSetupStatus(ctx, f.Repo.ID, repo.SetupStatusScanning))

	req := taskapi.CreateTaskRequest{
		Title:       "Fix bug",
		Description: "desc",
	}
	httpRes := doJSON(t, http.MethodPost, f.repoTasksURL(), req)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusConflict, httpRes.StatusCode, "expected 409 when repo setup is not complete")
}

// --- UpdateTask draft_pr ---

func TestUpdateTask_SetDraftPR(t *testing.T) {
	f := newFixture(t)
	tsk := f.seedTask("title", "desc")

	draftPR := true
	req := taskapi.UpdateTaskRequest{DraftPR: &draftPR}
	httpRes := doJSON(t, http.MethodPatch, f.taskURL(tsk.ID), req)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusOK, httpRes.StatusCode)

	updated := f.readTask(tsk.ID)
	assert.Equal(t, true, updated.DraftPR)
	assert.Equal(t, false, updated.SkipPR)
}

func TestUpdateTask_SkipPRAndDraftPR_MutuallyExclusive(t *testing.T) {
	f := newFixture(t)
	tsk := f.seedTask("title", "desc")

	skipPR := true
	draftPR := true
	req := taskapi.UpdateTaskRequest{SkipPR: &skipPR, DraftPR: &draftPR}
	httpRes := doJSON(t, http.MethodPatch, f.taskURL(tsk.ID), req)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode, "expected error for mutually exclusive skip_pr and draft_pr")
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

	tsk := task.NewTask(f.Repo.ID.String(), "title", "desc", []string{dep.ID.String()}, nil, 0, false, false, "sonnet", true)
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

func TestDeleteTask_WithPullRequest(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	tsk := f.seedTask("title", "desc")
	require.NoError(t, f.TaskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	require.NoError(t, f.TaskRepo.SetTaskPullRequest(ctx, tsk.ID, "https://github.com/owner/test-repo/pull/42", 42))

	// Verify PR info was set.
	tsk = f.readTask(tsk.ID)
	assert.Equal(t, 42, tsk.PRNumber)

	testutil.Delete(t, f.taskURL(tsk.ID))

	// Verify task was deleted even though no GitHub client is configured (graceful no-op).
	_, readErr := f.TaskRepo.ReadTask(ctx, tsk.ID)
	assert.Error(t, readErr, "expected task to be deleted")
}

func TestBulkDeleteTasks_WithPullRequests(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	tsk1 := f.seedTask("task1", "desc1")
	require.NoError(t, f.TaskRepo.UpdateTaskStatus(ctx, tsk1.ID, task.StatusRunning))
	require.NoError(t, f.TaskRepo.SetTaskPullRequest(ctx, tsk1.ID, "https://github.com/owner/test-repo/pull/10", 10))

	tsk2 := f.seedTask("task2", "desc2")

	req := taskapi.BulkDeleteTasksRequest{
		TaskIDs: []string{tsk1.ID.String(), tsk2.ID.String()},
	}
	postNoContent(t, f.Server.Address()+"/api/v1/tasks/bulk-delete", req)

	// Verify both tasks were deleted.
	_, err := f.TaskRepo.ReadTask(ctx, tsk1.ID)
	assert.Error(t, err, "expected task1 to be deleted")
	_, err = f.TaskRepo.ReadTask(ctx, tsk2.ID)
	assert.Error(t, err, "expected task2 to be deleted")
}

// --- GetTaskByNumber ---

func TestGetTaskByNumber_Success(t *testing.T) {
	f := newFixture(t)

	// Create a task via the API so it gets a number assigned.
	createReq := taskapi.CreateTaskRequest{
		Title:       "By number test",
		Description: "desc",
	}
	createRes := testutil.Post[server.Response[task.Task]](t, f.repoTasksURL(), createReq)
	require.True(t, createRes.Data.Number > 0, "expected task to have a number assigned")

	// Look it up by number.
	res := testutil.Get[server.Response[task.Task]](t, f.taskByNumberURL(createRes.Data.Number))
	assert.Equal(t, createRes.Data.ID, res.Data.ID)
	assert.Equal(t, createRes.Data.Number, res.Data.Number)
	assert.Equal(t, "By number test", res.Data.Title)
}

func TestGetTaskByNumber_NotFound(t *testing.T) {
	f := newFixture(t)

	httpRes, err := testutil.DefaultClient.Get(f.taskByNumberURL(9999))
	require.NoError(t, err)
	defer httpRes.Body.Close()

	assert.Equal(t, http.StatusNotFound, httpRes.StatusCode, "expected 404 for nonexistent number")
}

func TestGetTaskByNumber_InvalidNumber(t *testing.T) {
	tests := []struct {
		name   string
		number string
	}{
		{"non-numeric", "abc"},
		{"zero", "0"},
		{"negative", "-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)

			httpRes, err := testutil.DefaultClient.Get(f.taskByNumberRawURL(f.Repo.ID.String(), tt.number))
			require.NoError(t, err)
			defer httpRes.Body.Close()

			assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode, "expected validation error for invalid number format")
		})
	}
}

func TestGetTaskByNumber_InvalidRepoID(t *testing.T) {
	f := newFixture(t)

	httpRes, err := testutil.DefaultClient.Get(f.taskByNumberRawURL("invalid", "1"))
	require.NoError(t, err)
	defer httpRes.Body.Close()

	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode, "expected validation error for invalid repo ID")
}

// --- Task number in API responses ---

func TestCreateTask_HasNumber(t *testing.T) {
	f := newFixture(t)

	req := taskapi.CreateTaskRequest{
		Title:       "First task",
		Description: "desc",
	}
	res := testutil.Post[server.Response[task.Task]](t, f.repoTasksURL(), req)
	assert.True(t, res.Data.Number > 0, "created task should have a number")
}

func TestListTasksByRepo_TasksHaveNumbers(t *testing.T) {
	f := newFixture(t)

	f.seedTask("task 1", "desc")
	f.seedTask("task 2", "desc")

	res := testutil.Get[server.ResponseList[task.Task]](t, f.repoTasksURL())
	require.Len(t, res.Data, 2)
	for _, tsk := range res.Data {
		assert.True(t, tsk.Number > 0, "each task should have a number")
	}
}

