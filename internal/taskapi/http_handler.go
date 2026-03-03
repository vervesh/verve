package taskapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/joshjon/kit/server"
	"github.com/labstack/echo/v4"

	"github.com/joshjon/verve/internal/epic"
	"github.com/joshjon/verve/internal/github"
	"github.com/joshjon/verve/internal/githubtoken"
	"github.com/joshjon/verve/internal/logkey"
	"github.com/joshjon/verve/internal/repo"
	"github.com/joshjon/verve/internal/setting"
	"github.com/joshjon/verve/internal/task"
)

// HTTPHandler handles task HTTP requests.
type HTTPHandler struct {
	store              *task.Store
	repoStore          *repo.Store
	epicStore          *epic.Store
	githubTokenService *githubtoken.Service
	settingService     *setting.Service
}

// NewHTTPHandler creates a new HTTPHandler.
func NewHTTPHandler(store *task.Store, repoStore *repo.Store, epicStore *epic.Store, githubTokenService *githubtoken.Service, settingService *setting.Service) *HTTPHandler {
	return &HTTPHandler{store: store, repoStore: repoStore, epicStore: epicStore, githubTokenService: githubTokenService, settingService: settingService}
}

// Register adds the endpoints to the provided Echo router group.
func (h *HTTPHandler) Register(g *echo.Group) {
	// Repo-scoped task operations
	g.GET("/repos/:repo_id/tasks", h.ListTasksByRepo)
	g.POST("/repos/:repo_id/tasks", h.CreateTask)
	g.POST("/repos/:repo_id/tasks/sync", h.SyncRepoTasks)

	// Task operations (globally unique IDs)
	g.GET("/tasks/:id", h.GetTask)
	g.GET("/tasks/:id/logs", h.StreamLogs)
	g.POST("/tasks/:id/logs", h.AppendLogs)
	g.POST("/tasks/:id/heartbeat", h.Heartbeat)
	g.POST("/tasks/:id/complete", h.CompleteTask)
	g.POST("/tasks/:id/close", h.CloseTask)
	g.POST("/tasks/:id/stop", h.StopTask)
	g.POST("/tasks/:id/retry", h.RetryTask)
	g.POST("/tasks/:id/start-over", h.StartOverTask)
	g.POST("/tasks/:id/feedback", h.FeedbackTask)
	g.POST("/tasks/:id/move-to-review", h.MoveToReview)
	g.POST("/tasks/:id/sync", h.SyncTaskStatus)
	g.GET("/tasks/:id/checks", h.GetTaskChecks)
	g.GET("/tasks/:id/diff", h.GetTaskDiff)
	g.DELETE("/tasks/:id/dependency", h.RemoveDependency)
	g.PUT("/tasks/:id/ready", h.SetReady)
	g.PATCH("/tasks/:id", h.UpdateTask)
	g.DELETE("/tasks/:id", h.DeleteTask)
	g.POST("/tasks/bulk-delete", h.BulkDeleteTasks)

	// Worker polling
	g.GET("/tasks/poll", h.PollTask)
}

// --- Task Handlers ---

// ListTasksByRepo handles GET /repos/:repo_id/tasks
func (h *HTTPHandler) ListTasksByRepo(c echo.Context) error {
	req, err := server.BindRequest[RepoIDRequest](c)
	if err != nil {
		return err
	}
	id := repo.MustParseRepoID(req.RepoID)
	c.Set(logkey.RepoID, id.String())

	tasks, err := h.store.ListTasksByRepo(c.Request().Context(), id.String())
	if err != nil {
		return err
	}
	return server.SetResponseList(c, http.StatusOK, tasks, "")
}

// CreateTask handles POST /repos/:repo_id/tasks
func (h *HTTPHandler) CreateTask(c echo.Context) error {
	req, err := server.BindRequest[CreateTaskRequest](c)
	if err != nil {
		return err
	}
	repoID := repo.MustParseRepoID(req.RepoID)
	c.Set(logkey.RepoID, repoID.String())

	model := req.Model
	if model == "" && h.settingService != nil {
		model = h.settingService.Get(setting.KeyDefaultModel)
	}
	if model == "" {
		model = "sonnet"
	}
	t := task.NewTask(repoID.String(), req.Title, req.Description, req.DependsOn, req.AcceptanceCriteria, req.MaxCostUSD, req.SkipPR, model, !req.NotReady)
	if err := h.store.CreateTask(c.Request().Context(), t); err != nil {
		return err
	}
	c.Set(logkey.TaskID, t.ID.String())
	return server.SetResponse(c, http.StatusCreated, t)
}

// GetTask handles GET /tasks/:id
func (h *HTTPHandler) GetTask(c echo.Context) error {
	req, err := server.BindRequest[TaskIDRequest](c)
	if err != nil {
		return err
	}
	id := task.MustParseTaskID(req.ID)
	c.Set(logkey.TaskID, id.String())

	t, err := h.store.ReadTask(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, t)
}

// UpdateTask handles PATCH /tasks/:id
func (h *HTTPHandler) UpdateTask(c echo.Context) error {
	req, err := server.BindRequest[UpdateTaskRequest](c)
	if err != nil {
		return err
	}
	id := task.MustParseTaskID(req.ID)
	c.Set(logkey.TaskID, id.String())

	ctx := c.Request().Context()

	existing, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return err
	}

	params := task.UpdatePendingTaskParams{
		Title:              existing.Title,
		Description:        existing.Description,
		DependsOn:          existing.DependsOn,
		AcceptanceCriteria: existing.AcceptanceCriteria,
		MaxCostUSD:         existing.MaxCostUSD,
		SkipPR:             existing.SkipPR,
		Model:              existing.Model,
		Ready:              existing.Ready,
	}

	if req.Title != nil {
		params.Title = *req.Title
	}
	if req.Description != nil {
		params.Description = *req.Description
	}
	if req.DependsOn != nil {
		params.DependsOn = req.DependsOn
	}
	if req.AcceptanceCriteria != nil {
		params.AcceptanceCriteria = req.AcceptanceCriteria
	}
	if req.MaxCostUSD != nil {
		params.MaxCostUSD = *req.MaxCostUSD
	}
	if req.SkipPR != nil {
		params.SkipPR = *req.SkipPR
	}
	if req.Model != nil {
		params.Model = *req.Model
	}
	if req.NotReady != nil {
		params.Ready = !*req.NotReady
	}

	if params.DependsOn == nil {
		params.DependsOn = []string{}
	}
	if params.AcceptanceCriteria == nil {
		params.AcceptanceCriteria = []string{}
	}

	if err := h.store.UpdatePendingTask(ctx, id, params); err != nil {
		return err
	}

	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, t)
}

// PollTask handles GET /tasks/poll
// Long-polls for a pending task, claiming it atomically.
func (h *HTTPHandler) PollTask(c echo.Context) error {
	timeout := 30 * time.Second
	deadline := time.Now().Add(timeout)

	ctx := c.Request().Context()

	for {
		t, err := h.store.ClaimPendingTask(ctx, nil)
		if err != nil {
			return err
		}
		if t != nil {
			repoID, parseErr := repo.ParseRepoID(t.RepoID)
			if parseErr != nil {
				return parseErr
			}
			r, readErr := h.repoStore.ReadRepo(ctx, repoID)
			if readErr != nil {
				return readErr
			}

			var token string
			if h.githubTokenService != nil {
				token = h.githubTokenService.GetToken()
			}

			return server.SetResponse(c, http.StatusOK, PollTaskResponse{
				Task:         t,
				GitHubToken:  token,
				RepoFullName: r.FullName,
			})
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return c.NoContent(http.StatusNoContent)
		}

		select {
		case <-h.store.WaitForPending():
		case <-time.After(remaining):
			return c.NoContent(http.StatusNoContent)
		case <-ctx.Done():
			return c.NoContent(http.StatusNoContent)
		}
	}
}

// AppendLogs handles POST /tasks/:id/logs
func (h *HTTPHandler) AppendLogs(c echo.Context) error {
	req, err := server.BindRequest[LogsRequest](c)
	if err != nil {
		return err
	}
	id := task.MustParseTaskID(req.ID)
	c.Set(logkey.TaskID, id.String())

	attempt := req.Attempt
	if attempt == 0 {
		attempt = 1
	}

	if err := h.store.AppendTaskLogs(c.Request().Context(), id, attempt, req.Logs); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// Heartbeat handles POST /tasks/:id/heartbeat.
func (h *HTTPHandler) Heartbeat(c echo.Context) error {
	req, err := server.BindRequest[HeartbeatRequest](c)
	if err != nil {
		return err
	}
	id := task.MustParseTaskID(req.ID)
	c.Set(logkey.TaskID, id.String())

	stillRunning, err := h.store.Heartbeat(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, map[string]bool{"stopped": !stillRunning})
}

// CompleteTask handles POST /tasks/:id/complete
func (h *HTTPHandler) CompleteTask(c echo.Context) error {
	req, err := server.BindRequest[CompleteRequest](c)
	if err != nil {
		return err
	}
	id := task.MustParseTaskID(req.ID)
	c.Set(logkey.TaskID, id.String())

	ctx := c.Request().Context()

	if req.AgentStatus != "" {
		if err := h.store.SetAgentStatus(ctx, id, req.AgentStatus); err != nil {
			return err
		}
	}
	if req.CostUSD > 0 {
		if err := h.store.AddCost(ctx, id, req.CostUSD); err != nil {
			return err
		}
	}

	switch {
	case !req.Success:
		if req.PrereqFailed != "" {
			if err := h.store.SetCloseReason(ctx, id, req.PrereqFailed); err != nil {
				return err
			}
		}

		if req.Retryable && req.PrereqFailed == "" {
			reason := ClassifyRetryReason(req.Error)
			if err := h.store.ScheduleRetry(ctx, id, reason); err != nil {
				return err
			}
			return c.NoContent(http.StatusNoContent)
		}

		if err := h.store.UpdateTaskStatus(ctx, id, task.StatusFailed); err != nil {
			return err
		}
	case req.PullRequestURL != "":
		if err := h.store.SetTaskPullRequest(ctx, id, req.PullRequestURL, req.PRNumber); err != nil {
			return err
		}
	case req.BranchName != "":
		if err := h.store.SetTaskBranch(ctx, id, req.BranchName); err != nil {
			return err
		}
	default:
		t, readErr := h.store.ReadTask(ctx, id)
		if readErr != nil {
			return readErr
		}
		if t.PRNumber > 0 || t.BranchName != "" {
			if err := h.store.UpdateTaskStatus(ctx, id, task.StatusReview); err != nil {
				return err
			}
		} else {
			if req.NoChanges {
				if err := h.store.SetCloseReason(ctx, id, "No changes needed — the codebase already meets the required criteria"); err != nil {
					return err
				}
			}
			if err := h.store.UpdateTaskStatus(ctx, id, task.StatusClosed); err != nil {
				return err
			}
		}
	}

	return c.NoContent(http.StatusNoContent)
}

// SyncTaskStatus handles POST /tasks/:id/sync
func (h *HTTPHandler) SyncTaskStatus(c echo.Context) error {
	req, err := server.BindRequest[TaskIDRequest](c)
	if err != nil {
		return err
	}
	id := task.MustParseTaskID(req.ID)
	c.Set(logkey.TaskID, id.String())

	ctx := c.Request().Context()

	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return err
	}

	gh := h.githubClient()
	if t.Status == task.StatusReview && gh != nil && t.PRNumber > 0 {
		repoID, parseErr := repo.ParseRepoID(t.RepoID)
		if parseErr != nil {
			return parseErr
		}
		r, readErr := h.repoStore.ReadRepo(ctx, repoID)
		if readErr != nil {
			return readErr
		}

		merged, ghErr := gh.IsPRMerged(ctx, r.Owner, r.Name, t.PRNumber)
		if ghErr != nil {
			return ghErr
		}
		if merged {
			if err := h.store.UpdateTaskStatus(ctx, id, task.StatusMerged); err != nil {
				return err
			}
			t, err = h.store.ReadTask(ctx, id)
			if err != nil {
				return err
			}
		}
	}

	return server.SetResponse(c, http.StatusOK, t)
}

// SyncRepoTasks handles POST /repos/:repo_id/tasks/sync
func (h *HTTPHandler) SyncRepoTasks(c echo.Context) error {
	req, err := server.BindRequest[SyncRepoTasksRequest](c)
	if err != nil {
		return err
	}
	repoID := repo.MustParseRepoID(req.RepoID)
	c.Set(logkey.RepoID, repoID.String())

	gh := h.githubClient()
	if gh == nil {
		return server.SetResponse(c, http.StatusOK, map[string]int{"synced": 0, "merged": 0})
	}

	ctx := c.Request().Context()

	r, err := h.repoStore.ReadRepo(ctx, repoID)
	if err != nil {
		return err
	}

	tasks, err := h.store.ListTasksInReviewByRepo(ctx, repoID.String())
	if err != nil {
		return err
	}

	synced := 0
	merged := 0
	for _, t := range tasks {
		if t.PRNumber > 0 {
			synced++
			isMerged, err := gh.IsPRMerged(ctx, r.Owner, r.Name, t.PRNumber)
			if err != nil {
				continue
			}
			if isMerged {
				_ = h.store.UpdateTaskStatus(ctx, t.ID, task.StatusMerged)
				merged++
			}
		}
	}

	return server.SetResponse(c, http.StatusOK, map[string]int{"synced": synced, "merged": merged})
}

// CloseTask handles POST /tasks/:id/close
func (h *HTTPHandler) CloseTask(c echo.Context) error {
	req, err := server.BindRequest[CloseRequest](c)
	if err != nil {
		return err
	}
	id := task.MustParseTaskID(req.ID)
	c.Set(logkey.TaskID, id.String())

	ctx := c.Request().Context()

	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return err
	}

	if err := h.store.CloseTask(ctx, id, req.Reason); err != nil {
		return err
	}

	// Close the corresponding GitHub PR and delete its branch if unmerged.
	if t.PRNumber > 0 && t.Status != task.StatusMerged {
		if gh := h.githubClient(); gh != nil {
			repoID, parseErr := repo.ParseRepoID(t.RepoID)
			if parseErr == nil {
				r, readErr := h.repoStore.ReadRepo(ctx, repoID)
				if readErr == nil {
					branch, closeErr := gh.ClosePR(ctx, r.Owner, r.Name, t.PRNumber)
					if closeErr == nil && branch != "" {
						_ = gh.DeleteBranch(ctx, r.Owner, r.Name, branch)
					}
				}
			}
		}
	}

	t, err = h.store.ReadTask(ctx, id)
	if err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, t)
}

// StopTask handles POST /tasks/:id/stop
func (h *HTTPHandler) StopTask(c echo.Context) error {
	req, err := server.BindRequest[TaskIDRequest](c)
	if err != nil {
		return err
	}
	id := task.MustParseTaskID(req.ID)
	c.Set(logkey.TaskID, id.String())

	ctx := c.Request().Context()

	if err := h.store.StopTask(ctx, id, "Stopped by user"); err != nil {
		return err
	}

	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, t)
}

// RetryTask handles POST /tasks/:id/retry
func (h *HTTPHandler) RetryTask(c echo.Context) error {
	req, err := server.BindRequest[RetryTaskRequest](c)
	if err != nil {
		return err
	}
	id := task.MustParseTaskID(req.ID)
	c.Set(logkey.TaskID, id.String())

	ctx := c.Request().Context()

	if err := h.store.ManualRetryTask(ctx, id, req.Instructions); err != nil {
		return err
	}

	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, t)
}

// MoveToReview handles POST /tasks/:id/move-to-review
func (h *HTTPHandler) MoveToReview(c echo.Context) error {
	req, err := server.BindRequest[TaskIDRequest](c)
	if err != nil {
		return err
	}
	id := task.MustParseTaskID(req.ID)
	c.Set(logkey.TaskID, id.String())

	ctx := c.Request().Context()

	if err := h.store.MoveToReview(ctx, id); err != nil {
		return err
	}

	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, t)
}

// StartOverTask handles POST /tasks/:id/start-over
func (h *HTTPHandler) StartOverTask(c echo.Context) error {
	req, err := server.BindRequest[StartOverRequest](c)
	if err != nil {
		return err
	}
	id := task.MustParseTaskID(req.ID)
	c.Set(logkey.TaskID, id.String())

	ctx := c.Request().Context()

	existing, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return err
	}

	params := task.StartOverTaskParams{
		Title:              existing.Title,
		Description:        existing.Description,
		AcceptanceCriteria: existing.AcceptanceCriteria,
	}
	if req.Title != nil {
		params.Title = *req.Title
	}
	if req.Description != nil {
		params.Description = *req.Description
	}
	if req.AcceptanceCriteria != nil {
		params.AcceptanceCriteria = req.AcceptanceCriteria
	}
	if params.AcceptanceCriteria == nil {
		params.AcceptanceCriteria = []string{}
	}

	prev, err := h.store.StartOverTask(ctx, id, params)
	if err != nil {
		return err
	}
	if prev == nil {
		return echo.NewHTTPError(http.StatusConflict, "task is not in review or failed status")
	}

	// Close the corresponding GitHub PR and delete its branch if it had one.
	if prev.PRNumber > 0 && prev.Status != task.StatusMerged {
		if gh := h.githubClient(); gh != nil {
			repoID, parseErr := repo.ParseRepoID(prev.RepoID)
			if parseErr == nil {
				r, readErr := h.repoStore.ReadRepo(ctx, repoID)
				if readErr == nil {
					branch, closeErr := gh.ClosePR(ctx, r.Owner, r.Name, prev.PRNumber)
					if closeErr == nil && branch != "" {
						_ = gh.DeleteBranch(ctx, r.Owner, r.Name, branch)
					}
				}
			}
		}
	}

	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, t)
}

// FeedbackTask handles POST /tasks/:id/feedback
func (h *HTTPHandler) FeedbackTask(c echo.Context) error {
	req, err := server.BindRequest[FeedbackRequest](c)
	if err != nil {
		return err
	}
	id := task.MustParseTaskID(req.ID)
	c.Set(logkey.TaskID, id.String())

	ctx := c.Request().Context()

	if err := h.store.FeedbackRetryTask(ctx, id, req.Feedback); err != nil {
		return err
	}

	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, t)
}

// RemoveDependency handles DELETE /tasks/:id/dependency
func (h *HTTPHandler) RemoveDependency(c echo.Context) error {
	req, err := server.BindRequest[RemoveDependencyRequest](c)
	if err != nil {
		return err
	}
	id := task.MustParseTaskID(req.ID)
	c.Set(logkey.TaskID, id.String())

	ctx := c.Request().Context()

	if err := h.store.RemoveDependency(ctx, id, req.DependsOn); err != nil {
		return err
	}

	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, t)
}

// SetReady handles PUT /tasks/:id/ready
func (h *HTTPHandler) SetReady(c echo.Context) error {
	req, err := server.BindRequest[SetReadyRequest](c)
	if err != nil {
		return err
	}
	id := task.MustParseTaskID(req.ID)
	c.Set(logkey.TaskID, id.String())

	ctx := c.Request().Context()

	if err := h.store.SetReady(ctx, id, req.Ready); err != nil {
		return err
	}

	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, t)
}

// DeleteTask handles DELETE /tasks/:id
func (h *HTTPHandler) DeleteTask(c echo.Context) error {
	req, err := server.BindRequest[TaskIDRequest](c)
	if err != nil {
		return err
	}
	id := task.MustParseTaskID(req.ID)
	c.Set(logkey.TaskID, id.String())

	ctx := c.Request().Context()

	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return err
	}

	if err := h.store.DeleteTask(ctx, id); err != nil {
		return err
	}

	if t.EpicID != "" && h.epicStore != nil {
		epicID, parseErr := epic.ParseEpicID(t.EpicID)
		if parseErr == nil {
			if err := h.epicStore.RemoveTaskAndCheck(ctx, epicID, id.String()); err != nil {
				c.Logger().Errorf("failed to update epic after task deletion: %v", err)
			}
		}
	}

	return c.NoContent(http.StatusNoContent)
}

// BulkDeleteTasks handles POST /tasks/bulk-delete
func (h *HTTPHandler) BulkDeleteTasks(c echo.Context) error {
	req, err := server.BindRequest[BulkDeleteTasksRequest](c)
	if err != nil {
		return err
	}

	ctx := c.Request().Context()

	type epicRef struct {
		epicID string
		taskID string
	}
	var epicRefs []epicRef
	for _, idStr := range req.TaskIDs {
		id, parseErr := task.ParseTaskID(idStr)
		if parseErr != nil {
			continue
		}
		t, readErr := h.store.ReadTask(ctx, id)
		if readErr != nil {
			continue
		}
		if t.EpicID != "" {
			epicRefs = append(epicRefs, epicRef{epicID: t.EpicID, taskID: idStr})
		}
	}

	if err := h.store.BulkDeleteTasksByIDs(ctx, req.TaskIDs); err != nil {
		return err
	}

	if h.epicStore != nil {
		for _, ref := range epicRefs {
			epicID, parseErr := epic.ParseEpicID(ref.epicID)
			if parseErr == nil {
				if err := h.epicStore.RemoveTaskAndCheck(ctx, epicID, ref.taskID); err != nil {
					c.Logger().Errorf("failed to update epic after bulk task deletion: %v", err)
				}
			}
		}
	}

	return c.NoContent(http.StatusNoContent)
}

// GetTaskChecks handles GET /tasks/:id/checks
func (h *HTTPHandler) GetTaskChecks(c echo.Context) error {
	req, err := server.BindRequest[TaskIDRequest](c)
	if err != nil {
		return err
	}
	id := task.MustParseTaskID(req.ID)
	c.Set(logkey.TaskID, id.String())

	ctx := c.Request().Context()

	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return err
	}

	if t.PRNumber <= 0 {
		return server.SetResponse(c, http.StatusOK, CheckStatusResponse{Status: "success", Summary: "No CI checks configured"})
	}

	gh := h.githubClient()
	if gh == nil {
		return server.SetResponse(c, http.StatusOK, CheckStatusResponse{Status: "error", Summary: "GitHub token not configured"})
	}

	if h.githubTokenService.IsFineGrained() {
		return server.SetResponse(c, http.StatusOK, CheckStatusResponse{Status: "success", CheckRunsSkipped: true})
	}

	repoID, parseErr := repo.ParseRepoID(t.RepoID)
	if parseErr != nil {
		return parseErr
	}
	r, readErr := h.repoStore.ReadRepo(ctx, repoID)
	if readErr != nil {
		return readErr
	}

	result, err := gh.GetPRCheckStatus(ctx, r.Owner, r.Name, t.PRNumber)
	if err != nil {
		return server.SetResponse(c, http.StatusOK, CheckStatusResponse{Status: "error", Summary: "Failed to fetch check status"})
	}

	return server.SetResponse(c, http.StatusOK, CheckStatusResponse{
		Status:           string(result.Status),
		Summary:          result.Summary,
		FailedNames:      result.FailedNames,
		CheckRunsSkipped: result.CheckRunsSkipped,
		Checks:           result.Checks,
	})
}

// GetTaskDiff handles GET /tasks/:id/diff
func (h *HTTPHandler) GetTaskDiff(c echo.Context) error {
	req, err := server.BindRequest[TaskIDRequest](c)
	if err != nil {
		return err
	}
	id := task.MustParseTaskID(req.ID)
	c.Set(logkey.TaskID, id.String())

	ctx := c.Request().Context()

	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return err
	}

	if t.PRNumber <= 0 {
		return server.SetResponse(c, http.StatusOK, DiffResponse{Diff: ""})
	}

	gh := h.githubClient()
	if gh == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "GitHub token not configured")
	}

	repoID, parseErr := repo.ParseRepoID(t.RepoID)
	if parseErr != nil {
		return parseErr
	}
	r, readErr := h.repoStore.ReadRepo(ctx, repoID)
	if readErr != nil {
		return readErr
	}

	diff, err := gh.GetPRDiff(ctx, r.Owner, r.Name, t.PRNumber)
	if err != nil {
		return err
	}

	return server.SetResponse(c, http.StatusOK, DiffResponse{Diff: diff})
}

// StreamLogs handles GET /tasks/:id/logs as a Server-Sent Events stream.
func (h *HTTPHandler) StreamLogs(c echo.Context) error {
	req, err := server.BindRequest[TaskIDRequest](c)
	if err != nil {
		return err
	}
	id := task.MustParseTaskID(req.ID)
	c.Set(logkey.TaskID, id.String())

	w := c.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	ctx := c.Request().Context()

	ch := h.store.Subscribe()
	defer h.store.Unsubscribe(ch)

	err = h.store.StreamTaskLogs(ctx, id, func(attempt int, lines []string) error {
		return writeSSE(w, task.EventLogsAppended, task.Event{
			Type:    task.EventLogsAppended,
			TaskID:  id,
			Attempt: attempt,
			Logs:    lines,
		})
	})
	if err != nil {
		return nil
	}

	if err := writeSSE(w, "logs_done", map[string]any{}); err != nil {
		return nil
	}

	for {
		select {
		case event := <-ch:
			if event.Type == task.EventLogsAppended && event.TaskID == id {
				if err := writeSSE(w, event.Type, event); err != nil {
					return nil
				}
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// githubClient returns the current GitHub client, or nil if no token is configured.
func (h *HTTPHandler) githubClient() *github.Client {
	if h.githubTokenService == nil {
		return nil
	}
	return h.githubTokenService.GetClient()
}

func writeSSE(w *echo.Response, event string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b); err != nil {
		return err
	}
	w.Flush()
	return nil
}

// ClassifyRetryReason categorizes a retryable error message into a reason
// string with a category prefix.
func ClassifyRetryReason(errMsg string) string {
	lower := strings.ToLower(errMsg)

	rateLimitPatterns := []string{"rate limit", "rate_limit", "too many requests", "max usage", "overloaded_error"}
	for _, p := range rateLimitPatterns {
		if strings.Contains(lower, p) {
			return "rate_limit: " + errMsg
		}
	}

	infraPatterns := []string{
		"could not resolve host", "unable to access", "unable to look up",
		"connection refused", "connection timed out", "connection reset",
		"no such host", "network is unreachable", "temporary failure in name resolution",
		"tls handshake timeout", "i/o timeout", "unexpected disconnect",
		"the remote end hung up unexpectedly", "early eof",
		"ssl_error", "gnutls_handshake", "failed to connect",
		"failed to create container", "failed to start container",
		"error waiting for container",
	}
	for _, p := range infraPatterns {
		if strings.Contains(lower, p) {
			return "transient: " + errMsg
		}
	}

	return "transient: " + errMsg
}
