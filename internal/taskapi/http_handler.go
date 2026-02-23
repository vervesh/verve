package taskapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/joshjon/kit/errtag"
	"github.com/labstack/echo/v4"

	"verve/internal/epic"
	"verve/internal/github"
	"verve/internal/githubtoken"
	"verve/internal/repo"
	"verve/internal/setting"
	"verve/internal/task"
	"verve/internal/workertracker"
)

// HTTPHandler handles task and repo HTTP requests.
type HTTPHandler struct {
	store              *task.Store
	repoStore          *repo.Store
	epicStore          *epic.Store
	githubTokenService *githubtoken.Service
	settingService     *setting.Service
	workerRegistry     *workertracker.Registry
	models             []setting.ModelOption
}

// NewHTTPHandler creates a new HTTPHandler.
func NewHTTPHandler(store *task.Store, repoStore *repo.Store, epicStore *epic.Store, githubTokenService *githubtoken.Service, settingService *setting.Service, workerRegistry *workertracker.Registry, models []setting.ModelOption) *HTTPHandler {
	if len(models) == 0 {
		models = setting.DefaultModels
	}
	return &HTTPHandler{store: store, repoStore: repoStore, epicStore: epicStore, githubTokenService: githubTokenService, settingService: settingService, workerRegistry: workerRegistry, models: models}
}

// Register adds the endpoints to the provided Echo router group.
func (h *HTTPHandler) Register(g *echo.Group) {
	g.GET("/events", h.Events)

	// Repo management
	g.GET("/repos", h.ListRepos)
	g.POST("/repos", h.AddRepo)
	g.DELETE("/repos/:repo_id", h.RemoveRepo)
	g.GET("/repos/available", h.ListAvailableRepos)

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
	g.POST("/tasks/:id/sync", h.SyncTaskStatus)
	g.GET("/tasks/:id/checks", h.GetTaskChecks)
	g.GET("/tasks/:id/diff", h.GetTaskDiff)
	g.DELETE("/tasks/:id/dependency", h.RemoveDependency)
	g.PUT("/tasks/:id/ready", h.SetReady)
	g.PATCH("/tasks/:id", h.UpdateTask)
	g.DELETE("/tasks/:id", h.DeleteTask)
	g.POST("/tasks/bulk-delete", h.BulkDeleteTasks)

	// Agent observability
	g.GET("/agents/metrics", h.GetAgentMetrics)

	// Worker polling
	g.GET("/tasks/poll", h.PollTask)

	// Settings
	g.PUT("/settings/github-token", h.SaveGitHubToken)
	g.GET("/settings/github-token", h.GetGitHubTokenStatus)
	g.DELETE("/settings/github-token", h.DeleteGitHubToken)
	g.PUT("/settings/default-model", h.SaveDefaultModel)
	g.GET("/settings/default-model", h.GetDefaultModel)
	g.DELETE("/settings/default-model", h.DeleteDefaultModel)
	g.GET("/settings/models", h.ListModels)
}

// --- Agent Observability Handlers ---

// GetAgentMetrics handles GET /agents/metrics
// Returns a snapshot of current agent activity and performance metrics.
func (h *HTTPHandler) GetAgentMetrics(c echo.Context) error {
	metrics, err := h.store.GetAgentMetrics(c.Request().Context())
	if err != nil {
		return jsonError(c, err)
	}
	// Attach worker info from the registry
	if h.workerRegistry != nil {
		metrics.Workers = h.workerRegistry.ListWorkers(2 * time.Minute)
	}
	if metrics.Workers == nil {
		metrics.Workers = []workertracker.WorkerInfo{}
	}
	return c.JSON(http.StatusOK, metrics)
}

// --- Settings Handlers ---

// SaveGitHubToken handles PUT /settings/github-token
func (h *HTTPHandler) SaveGitHubToken(c echo.Context) error {
	if h.githubTokenService == nil {
		return c.JSON(http.StatusServiceUnavailable, errorResponse("encryption key not configured"))
	}

	var req SaveGitHubTokenRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}
	if req.Token == "" {
		return c.JSON(http.StatusBadRequest, errorResponse("token required"))
	}
	if !githubtoken.IsValidTokenPrefix(req.Token) {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid token format — expected a GitHub personal access token starting with ghp_ or github_pat_"))
	}

	if err := h.githubTokenService.SaveToken(c.Request().Context(), req.Token); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResponse("failed to save token: "+err.Error()))
	}
	return c.JSON(http.StatusOK, statusOK())
}

// GetGitHubTokenStatus handles GET /settings/github-token
func (h *HTTPHandler) GetGitHubTokenStatus(c echo.Context) error {
	configured := h.githubTokenService != nil && h.githubTokenService.HasToken()
	fineGrained := h.githubTokenService != nil && h.githubTokenService.IsFineGrained()
	return c.JSON(http.StatusOK, GitHubTokenStatusResponse{Configured: configured, FineGrained: fineGrained})
}

// DeleteGitHubToken handles DELETE /settings/github-token
func (h *HTTPHandler) DeleteGitHubToken(c echo.Context) error {
	if h.githubTokenService == nil {
		return c.JSON(http.StatusServiceUnavailable, errorResponse("encryption key not configured"))
	}

	if err := h.githubTokenService.DeleteToken(c.Request().Context()); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResponse("failed to delete token: "+err.Error()))
	}
	return c.JSON(http.StatusOK, statusOK())
}

// --- Default Model Settings Handlers ---

// SaveDefaultModel handles PUT /settings/default-model
func (h *HTTPHandler) SaveDefaultModel(c echo.Context) error {
	if h.settingService == nil {
		return c.JSON(http.StatusServiceUnavailable, errorResponse("settings not available"))
	}

	var req DefaultModelRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}
	if req.Model == "" {
		return c.JSON(http.StatusBadRequest, errorResponse("model required"))
	}

	if err := h.settingService.Set(c.Request().Context(), setting.KeyDefaultModel, req.Model); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResponse("failed to save default model: "+err.Error()))
	}
	return c.JSON(http.StatusOK, DefaultModelResponse{Model: req.Model, Configured: true})
}

// GetDefaultModel handles GET /settings/default-model
func (h *HTTPHandler) GetDefaultModel(c echo.Context) error {
	var model string
	if h.settingService != nil {
		model = h.settingService.Get(setting.KeyDefaultModel)
	}
	return c.JSON(http.StatusOK, DefaultModelResponse{Model: model, Configured: model != ""})
}

// ListModels handles GET /settings/models
func (h *HTTPHandler) ListModels(c echo.Context) error {
	return c.JSON(http.StatusOK, h.models)
}

// DeleteDefaultModel handles DELETE /settings/default-model
func (h *HTTPHandler) DeleteDefaultModel(c echo.Context) error {
	if h.settingService == nil {
		return c.JSON(http.StatusServiceUnavailable, errorResponse("settings not available"))
	}

	if err := h.settingService.Delete(c.Request().Context(), setting.KeyDefaultModel); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResponse("failed to delete default model: "+err.Error()))
	}
	return c.JSON(http.StatusOK, statusOK())
}

// --- Repo Handlers ---

// ListRepos handles GET /repos
func (h *HTTPHandler) ListRepos(c echo.Context) error {
	repos, err := h.repoStore.ListRepos(c.Request().Context())
	if err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, repos)
}

// AddRepo handles POST /repos
func (h *HTTPHandler) AddRepo(c echo.Context) error {
	var req AddRepoRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}
	if req.FullName == "" {
		return c.JSON(http.StatusBadRequest, errorResponse("full_name required"))
	}

	r, err := repo.NewRepo(req.FullName)
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse(err.Error()))
	}

	if err := h.repoStore.CreateRepo(c.Request().Context(), r); err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusCreated, r)
}

// RemoveRepo handles DELETE /repos/:repo_id
func (h *HTTPHandler) RemoveRepo(c echo.Context) error {
	id, err := repo.ParseRepoID(c.Param("repo_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid repo ID"))
	}

	if err := h.repoStore.DeleteRepo(c.Request().Context(), id); err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, statusOK())
}

// ListAvailableRepos handles GET /repos/available
func (h *HTTPHandler) ListAvailableRepos(c echo.Context) error {
	gh := h.githubClient()
	if gh == nil {
		return c.JSON(http.StatusServiceUnavailable, errorResponse("GitHub token not configured"))
	}

	repos, err := gh.ListAccessibleRepos(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResponse("failed to list GitHub repos: "+err.Error()))
	}
	return c.JSON(http.StatusOK, repos)
}

// --- Task Handlers ---

// ListTasksByRepo handles GET /repos/:repo_id/tasks
func (h *HTTPHandler) ListTasksByRepo(c echo.Context) error {
	id, err := repo.ParseRepoID(c.Param("repo_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid repo ID"))
	}

	tasks, err := h.store.ListTasksByRepo(c.Request().Context(), id.String())
	if err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, tasks)
}

// CreateTask handles POST /repos/:repo_id/tasks
func (h *HTTPHandler) CreateTask(c echo.Context) error {
	repoID, err := repo.ParseRepoID(c.Param("repo_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid repo ID"))
	}

	var req CreateTaskRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}
	if req.Title == "" {
		return c.JSON(http.StatusBadRequest, errorResponse("title required"))
	}
	if len(req.Title) > 150 {
		return c.JSON(http.StatusBadRequest, errorResponse("title must be 150 characters or less"))
	}
	model := req.Model
	if model == "" && h.settingService != nil {
		model = h.settingService.Get(setting.KeyDefaultModel)
	}
	if model == "" {
		model = "sonnet"
	}
	t := task.NewTask(repoID.String(), req.Title, req.Description, req.DependsOn, req.AcceptanceCriteria, req.MaxCostUSD, req.SkipPR, model, !req.NotReady)
	if err := h.store.CreateTask(c.Request().Context(), t); err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusCreated, t)
}

// GetTask handles GET /tasks/:id
func (h *HTTPHandler) GetTask(c echo.Context) error {
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}

	t, err := h.store.ReadTask(c.Request().Context(), id)
	if err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, t)
}

// UpdateTask handles PATCH /tasks/:id
// Updates a task that is still in pending status. Rejects updates if the
// task has transitioned to any other status.
func (h *HTTPHandler) UpdateTask(c echo.Context) error {
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}

	var req UpdateTaskRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}

	ctx := c.Request().Context()

	// Read current task to merge with provided fields
	existing, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return jsonError(c, err)
	}

	// Build update params by merging request with current values
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
		if *req.Title == "" {
			return c.JSON(http.StatusBadRequest, errorResponse("title required"))
		}
		if len(*req.Title) > 150 {
			return c.JSON(http.StatusBadRequest, errorResponse("title must be 150 characters or less"))
		}
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
		return jsonError(c, err)
	}

	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, t)
}

// PollTask handles GET /tasks/poll
// Long-polls for a pending task, claiming it atomically.
// Returns a PollTaskResponse with the task, GitHub token, and repo full name.
func (h *HTTPHandler) PollTask(c echo.Context) error {
	timeout := 30 * time.Second
	deadline := time.Now().Add(timeout)

	ctx := c.Request().Context()

	for {
		t, err := h.store.ClaimPendingTask(ctx, nil)
		if err != nil {
			return jsonError(c, err)
		}
		if t != nil {
			// Look up repo to get full name for the worker
			repoID, parseErr := repo.ParseRepoID(t.RepoID)
			if parseErr != nil {
				return c.JSON(http.StatusInternalServerError, errorResponse("invalid repo ID on task"))
			}
			r, readErr := h.repoStore.ReadRepo(ctx, repoID)
			if readErr != nil {
				return jsonError(c, readErr)
			}

			var token string
			if h.githubTokenService != nil {
				token = h.githubTokenService.GetToken()
			}

			return c.JSON(http.StatusOK, PollTaskResponse{
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
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}

	var req LogsRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}

	attempt := req.Attempt
	if attempt == 0 {
		attempt = 1 // default for backward compat with old workers
	}

	if err := h.store.AppendTaskLogs(c.Request().Context(), id, attempt, req.Logs); err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, statusOK())
}

// Heartbeat handles POST /tasks/:id/heartbeat.
// Returns {"stopped": true} when the task is no longer running — either because
// it was explicitly stopped, closed, or deleted — so the worker can cancel the
// agent container immediately and avoid wasting resources.
func (h *HTTPHandler) Heartbeat(c echo.Context) error {
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}
	stillRunning, err := h.store.Heartbeat(c.Request().Context(), id)
	if err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]bool{"stopped": !stillRunning})
}

// CompleteTask handles POST /tasks/:id/complete
func (h *HTTPHandler) CompleteTask(c echo.Context) error {
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}

	var req CompleteRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}

	ctx := c.Request().Context()

	// Store agent status and cost before updating task status
	if req.AgentStatus != "" {
		if err := h.store.SetAgentStatus(ctx, id, req.AgentStatus); err != nil {
			return jsonError(c, err)
		}
	}
	if req.CostUSD > 0 {
		if err := h.store.AddCost(ctx, id, req.CostUSD); err != nil {
			return jsonError(c, err)
		}
	}

	switch {
	case !req.Success:
		if req.PrereqFailed != "" {
			if err := h.store.SetCloseReason(ctx, id, req.PrereqFailed); err != nil {
				return jsonError(c, err)
			}
		}

		// Retryable failure (e.g. Claude rate limit, network error, or other
		// transient issue): schedule a retry back to pending instead of marking
		// as failed.
		if req.Retryable && req.PrereqFailed == "" {
			reason := classifyRetryReason(req.Error)
			if err := h.store.ScheduleRetry(ctx, id, reason); err != nil {
				return jsonError(c, err)
			}
			return c.JSON(http.StatusOK, statusOK())
		}

		// Non-retryable failure: mark as failed. Even if a PR or branch
		// exists from a previous attempt, the agent explicitly failed so
		// the status should reflect that. The PR/branch data is preserved
		// on the task record for the user to inspect.
		if err := h.store.UpdateTaskStatus(ctx, id, task.StatusFailed); err != nil {
			return jsonError(c, err)
		}
	case req.PullRequestURL != "":
		if err := h.store.SetTaskPullRequest(ctx, id, req.PullRequestURL, req.PRNumber); err != nil {
			return jsonError(c, err)
		}
	case req.BranchName != "":
		if err := h.store.SetTaskBranch(ctx, id, req.BranchName); err != nil {
			return jsonError(c, err)
		}
	default:
		// Check if this task already has a PR (retry scenario — agent pushed
		// fixes to the existing branch without emitting a new PR marker).
		// Return it to review rather than closing.
		t, readErr := h.store.ReadTask(ctx, id)
		if readErr != nil {
			return jsonError(c, readErr)
		}
		if t.PRNumber > 0 || t.BranchName != "" {
			if err := h.store.UpdateTaskStatus(ctx, id, task.StatusReview); err != nil {
				return jsonError(c, err)
			}
		} else {
			if req.NoChanges {
				if err := h.store.SetCloseReason(ctx, id, "No changes needed — the codebase already meets the required criteria"); err != nil {
					return jsonError(c, err)
				}
			}
			if err := h.store.UpdateTaskStatus(ctx, id, task.StatusClosed); err != nil {
				return jsonError(c, err)
			}
		}
	}

	return c.JSON(http.StatusOK, statusOK())
}

// SyncTaskStatus handles POST /tasks/:id/sync
func (h *HTTPHandler) SyncTaskStatus(c echo.Context) error {
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}

	ctx := c.Request().Context()

	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return jsonError(c, err)
	}

	gh := h.githubClient()
	if t.Status == task.StatusReview && gh != nil && t.PRNumber > 0 {
		// Look up repo to get owner/name for the GitHub API call.
		repoID, parseErr := repo.ParseRepoID(t.RepoID)
		if parseErr != nil {
			return c.JSON(http.StatusInternalServerError, errorResponse("invalid repo ID on task"))
		}
		r, readErr := h.repoStore.ReadRepo(ctx, repoID)
		if readErr != nil {
			return jsonError(c, readErr)
		}

		merged, ghErr := gh.IsPRMerged(ctx, r.Owner, r.Name, t.PRNumber)
		if ghErr != nil {
			return c.JSON(http.StatusInternalServerError, errorResponse("failed to check PR status: "+ghErr.Error()))
		}
		if merged {
			if err := h.store.UpdateTaskStatus(ctx, id, task.StatusMerged); err != nil {
				return jsonError(c, err)
			}
			t, err = h.store.ReadTask(ctx, id)
			if err != nil {
				return jsonError(c, err)
			}
		}
	}

	return c.JSON(http.StatusOK, t)
}

// SyncRepoTasks handles POST /repos/:repo_id/tasks/sync
func (h *HTTPHandler) SyncRepoTasks(c echo.Context) error {
	repoID, err := repo.ParseRepoID(c.Param("repo_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid repo ID"))
	}

	gh := h.githubClient()
	if gh == nil {
		return c.JSON(http.StatusOK, map[string]int{"synced": 0, "merged": 0})
	}

	ctx := c.Request().Context()

	r, err := h.repoStore.ReadRepo(ctx, repoID)
	if err != nil {
		return jsonError(c, err)
	}

	tasks, err := h.store.ListTasksInReviewByRepo(ctx, repoID.String())
	if err != nil {
		return jsonError(c, err)
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

	return c.JSON(http.StatusOK, map[string]int{"synced": synced, "merged": merged})
}

// CloseTask handles POST /tasks/:id/close
func (h *HTTPHandler) CloseTask(c echo.Context) error {
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}

	var req CloseRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}

	ctx := c.Request().Context()

	// Read task before closing to check for open PR.
	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return jsonError(c, err)
	}

	if err := h.store.CloseTask(ctx, id, req.Reason); err != nil {
		return jsonError(c, err)
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
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, t)
}

// StopTask handles POST /tasks/:id/stop — interrupts a running task.
// The task transitions from running → pending with ready=false so the worker
// stops execution and the task won't be picked up until manually retried.
func (h *HTTPHandler) StopTask(c echo.Context) error {
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}

	ctx := c.Request().Context()

	if err := h.store.StopTask(ctx, id, "Stopped by user"); err != nil {
		return jsonError(c, err)
	}

	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, t)
}

// RetryTask handles POST /tasks/:id/retry
func (h *HTTPHandler) RetryTask(c echo.Context) error {
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}

	var req RetryTaskRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}

	ctx := c.Request().Context()

	if err := h.store.ManualRetryTask(ctx, id, req.Instructions); err != nil {
		return jsonError(c, err)
	}

	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, t)
}

// StartOverTask handles POST /tasks/:id/start-over
// Resets a task in review or failed status back to pending with fresh metadata.
// Clears all logs, PR info, agent status, cost, and retry state.
// Optionally updates title, description, and acceptance criteria.
// If the task had an open PR, it is closed on GitHub and the branch deleted.
func (h *HTTPHandler) StartOverTask(c echo.Context) error {
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}

	var req StartOverRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}

	ctx := c.Request().Context()

	// Read existing task to merge fields
	existing, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return jsonError(c, err)
	}

	params := task.StartOverTaskParams{
		Title:              existing.Title,
		Description:        existing.Description,
		AcceptanceCriteria: existing.AcceptanceCriteria,
	}
	if req.Title != nil {
		if *req.Title == "" {
			return c.JSON(http.StatusBadRequest, errorResponse("title required"))
		}
		if len(*req.Title) > 150 {
			return c.JSON(http.StatusBadRequest, errorResponse("title must be 150 characters or less"))
		}
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
		return jsonError(c, err)
	}
	if prev == nil {
		return c.JSON(http.StatusConflict, errorResponse("task is not in review or failed status"))
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
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, t)
}

// FeedbackTask handles POST /tasks/:id/feedback
// Re-prompts the agent to iterate on a task in review based on user feedback.
func (h *HTTPHandler) FeedbackTask(c echo.Context) error {
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}

	var req FeedbackRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}
	if req.Feedback == "" {
		return c.JSON(http.StatusBadRequest, errorResponse("feedback is required"))
	}

	ctx := c.Request().Context()

	if err := h.store.FeedbackRetryTask(ctx, id, req.Feedback); err != nil {
		return jsonError(c, err)
	}

	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, t)
}

// RemoveDependency handles DELETE /tasks/:id/dependency
func (h *HTTPHandler) RemoveDependency(c echo.Context) error {
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}

	var req RemoveDependencyRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}
	if req.DependsOn == "" {
		return c.JSON(http.StatusBadRequest, errorResponse("depends_on is required"))
	}

	ctx := c.Request().Context()

	if err := h.store.RemoveDependency(ctx, id, req.DependsOn); err != nil {
		return jsonError(c, err)
	}

	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, t)
}

// SetReady handles PUT /tasks/:id/ready
// Toggles whether a task is ready to be picked up by workers.
func (h *HTTPHandler) SetReady(c echo.Context) error {
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}

	var req SetReadyRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}

	ctx := c.Request().Context()

	if err := h.store.SetReady(ctx, id, req.Ready); err != nil {
		return jsonError(c, err)
	}

	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, t)
}

// DeleteTask handles DELETE /tasks/:id
func (h *HTTPHandler) DeleteTask(c echo.Context) error {
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}

	ctx := c.Request().Context()

	// Read task before deletion to check epic association
	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return jsonError(c, err)
	}

	if err := h.store.DeleteTask(ctx, id); err != nil {
		return jsonError(c, err)
	}

	// If the task belonged to an epic, remove it from the epic's task_ids
	// and check if the epic should be marked as completed.
	if t.EpicID != "" && h.epicStore != nil {
		epicID, parseErr := epic.ParseEpicID(t.EpicID)
		if parseErr == nil {
			if err := h.epicStore.RemoveTaskAndCheck(ctx, epicID, id.String()); err != nil {
				c.Logger().Errorf("failed to update epic after task deletion: %v", err)
			}
		}
	}

	return c.JSON(http.StatusOK, statusOK())
}

// BulkDeleteTasks handles POST /tasks/bulk-delete
// Deletes multiple tasks by their IDs in a single operation.
func (h *HTTPHandler) BulkDeleteTasks(c echo.Context) error {
	var req BulkDeleteTasksRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}
	if len(req.TaskIDs) == 0 {
		return c.JSON(http.StatusBadRequest, errorResponse("task_ids required"))
	}

	ctx := c.Request().Context()

	// Read tasks before deletion to check epic associations.
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
		return jsonError(c, err)
	}

	// Update epic task_ids for any deleted tasks that belonged to epics.
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

	return c.JSON(http.StatusOK, statusOK())
}

// GetTaskChecks handles GET /tasks/:id/checks
// Returns the CI check status for a task's PR from GitHub.
func (h *HTTPHandler) GetTaskChecks(c echo.Context) error {
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}

	ctx := c.Request().Context()

	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return jsonError(c, err)
	}

	if t.PRNumber <= 0 {
		return c.JSON(http.StatusOK, CheckStatusResponse{Status: "success", Summary: "No CI checks configured"})
	}

	gh := h.githubClient()
	if gh == nil {
		return c.JSON(http.StatusOK, CheckStatusResponse{Status: "error", Summary: "GitHub token not configured"})
	}

	// Fine-grained tokens cannot access CI check APIs — skip entirely.
	if h.githubTokenService.IsFineGrained() {
		return c.JSON(http.StatusOK, CheckStatusResponse{Status: "success", CheckRunsSkipped: true})
	}

	repoID, parseErr := repo.ParseRepoID(t.RepoID)
	if parseErr != nil {
		return c.JSON(http.StatusInternalServerError, errorResponse("invalid repo ID on task"))
	}
	r, readErr := h.repoStore.ReadRepo(ctx, repoID)
	if readErr != nil {
		return jsonError(c, readErr)
	}

	result, err := gh.GetPRCheckStatus(ctx, r.Owner, r.Name, t.PRNumber)
	if err != nil {
		return c.JSON(http.StatusOK, CheckStatusResponse{Status: "error", Summary: "Failed to fetch check status"})
	}

	return c.JSON(http.StatusOK, CheckStatusResponse{
		Status:           string(result.Status),
		Summary:          result.Summary,
		FailedNames:      result.FailedNames,
		CheckRunsSkipped: result.CheckRunsSkipped,
		Checks:           result.Checks,
	})
}

// GetTaskDiff handles GET /tasks/:id/diff
// Returns the PR diff for a task from GitHub.
func (h *HTTPHandler) GetTaskDiff(c echo.Context) error {
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}

	ctx := c.Request().Context()

	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return jsonError(c, err)
	}

	if t.PRNumber <= 0 {
		return c.JSON(http.StatusOK, DiffResponse{Diff: ""})
	}

	gh := h.githubClient()
	if gh == nil {
		return c.JSON(http.StatusServiceUnavailable, errorResponse("GitHub token not configured"))
	}

	repoID, parseErr := repo.ParseRepoID(t.RepoID)
	if parseErr != nil {
		return c.JSON(http.StatusInternalServerError, errorResponse("invalid repo ID on task"))
	}
	r, readErr := h.repoStore.ReadRepo(ctx, repoID)
	if readErr != nil {
		return jsonError(c, readErr)
	}

	diff, err := gh.GetPRDiff(ctx, r.Owner, r.Name, t.PRNumber)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResponse("failed to fetch PR diff: "+err.Error()))
	}

	return c.JSON(http.StatusOK, DiffResponse{Diff: diff})
}

// StreamLogs handles GET /tasks/:id/logs as a Server-Sent Events stream.
// It streams historical log batches from the database one at a time, then
// subscribes to the broker for live log events.
func (h *HTTPHandler) StreamLogs(c echo.Context) error {
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}

	w := c.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	ctx := c.Request().Context()

	// Subscribe to broker BEFORE reading historical logs to avoid gaps.
	ch := h.store.Subscribe()
	defer h.store.Unsubscribe(ch)

	// Stream existing log batches from the database one row at a time.
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

	// Signal that all historical logs have been sent.
	if err := writeSSE(w, "logs_done", map[string]any{}); err != nil {
		return nil
	}

	// Stream live log events from the broker.
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

// Events handles GET /events as a Server-Sent Events stream.
// Optionally filtered by ?repo_id=xxx.
func (h *HTTPHandler) Events(c echo.Context) error {
	repoIDFilter := c.QueryParam("repo_id")

	w := c.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	ctx := c.Request().Context()

	// Send init event with task list (logs nil'd), filtered by repo if specified.
	var tasks []*task.Task
	var err error
	if repoIDFilter != "" {
		tasks, err = h.store.ListTasksByRepo(ctx, repoIDFilter)
	} else {
		tasks, err = h.store.ListTasks(ctx)
	}
	if err != nil {
		return err
	}
	for _, t := range tasks {
		t.Logs = nil
	}
	if err := writeSSE(w, "init", tasks); err != nil {
		return err
	}

	// Subscribe to broker and stream events.
	ch := h.store.Subscribe()
	defer h.store.Unsubscribe(ch)

	for {
		select {
		case event := <-ch:
			// Filter by repo if specified.
			if repoIDFilter != "" && event.RepoID != repoIDFilter {
				continue
			}
			if err := writeSSE(w, event.Type, event); err != nil {
				return nil
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

// jsonError maps errtag-tagged errors to appropriate HTTP status codes.
// For 5xx errors, the underlying error details are logged to aid debugging.
func jsonError(c echo.Context, err error) error {
	code := http.StatusInternalServerError
	msg := "internal server error"

	var tagger errtag.Tagger
	if errors.As(err, &tagger) {
		code = tagger.Code()
		msg = tagger.Msg()
	}

	if code >= 500 {
		c.Logger().Errorf("handler error: method=%s path=%s status=%d error=%v", c.Request().Method, c.Path(), code, err)
	}

	return c.JSON(code, errorResponse(msg))
}

func errorResponse(msg string) map[string]string {
	return map[string]string{"error": msg}
}

func statusOK() map[string]string {
	return map[string]string{"status": "ok"}
}

// classifyRetryReason categorizes a retryable error message into a reason
// string with a category prefix. The category is used by the circuit breaker
// in Store.ScheduleRetry to detect consecutive same-type failures.
func classifyRetryReason(errMsg string) string {
	lower := strings.ToLower(errMsg)

	// Check for rate limit / max usage errors
	rateLimitPatterns := []string{"rate limit", "rate_limit", "too many requests", "max usage", "overloaded_error"}
	for _, p := range rateLimitPatterns {
		if strings.Contains(lower, p) {
			return "rate_limit: " + errMsg
		}
	}

	// Check for transient infrastructure errors (network, DNS, timeouts)
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

	// Default: use a generic retryable category
	return "transient: " + errMsg
}
