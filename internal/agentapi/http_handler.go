package agentapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/joshjon/kit/server"
	"github.com/labstack/echo/v4"

	"github.com/joshjon/verve/internal/epic"
	"github.com/joshjon/verve/internal/githubtoken"
	"github.com/joshjon/verve/internal/logkey"
	"github.com/joshjon/verve/internal/repo"
	"github.com/joshjon/verve/internal/task"
	"github.com/joshjon/verve/internal/workertracker"
)

// HTTPHandler handles agent-facing API requests.
type HTTPHandler struct {
	taskStore      *task.Store
	epicStore      *epic.Store
	repoStore      *repo.Store
	githubToken    *githubtoken.Service
	workerRegistry *workertracker.Registry
}

// NewHTTPHandler creates a new HTTPHandler.
func NewHTTPHandler(taskStore *task.Store, epicStore *epic.Store, repoStore *repo.Store, githubToken *githubtoken.Service, workerRegistry *workertracker.Registry) *HTTPHandler {
	return &HTTPHandler{
		taskStore:      taskStore,
		epicStore:      epicStore,
		repoStore:      repoStore,
		githubToken:    githubToken,
		workerRegistry: workerRegistry,
	}
}

// Register adds the agent endpoints to the provided Echo router group.
func (h *HTTPHandler) Register(g *echo.Group) {
	// Unified poll (epics first, then tasks)
	g.GET("/poll", h.Poll)

	// Worker observability
	g.GET("/workers", h.ListWorkers)

	// Task agent endpoints
	g.POST("/tasks/:id/logs", h.TaskAppendLogs)
	g.POST("/tasks/:id/heartbeat", h.TaskHeartbeat)
	g.POST("/tasks/:id/complete", h.TaskComplete)

	// Epic agent endpoints
	g.POST("/epics/:id/propose", h.EpicPropose)
	g.GET("/epics/:id/poll-feedback", h.EpicPollFeedback)
	g.POST("/epics/:id/heartbeat", h.EpicHeartbeat)
	g.POST("/epics/:id/logs", h.EpicAppendLogs)
}

// Poll handles GET /poll — unified long-poll for available work.
func (h *HTTPHandler) Poll(c echo.Context) error {
	timeout := 30 * time.Second
	deadline := time.Now().Add(timeout)
	ctx := c.Request().Context()

	workerID := c.QueryParam("worker_id")
	if workerID != "" && h.workerRegistry != nil {
		maxConcurrent, _ := strconv.Atoi(c.QueryParam("max_concurrent"))
		activeTasks, _ := strconv.Atoi(c.QueryParam("active_tasks"))
		if maxConcurrent <= 0 {
			maxConcurrent = 1
		}
		h.workerRegistry.RecordPollStart(workerID, maxConcurrent, activeTasks)
		defer h.workerRegistry.RecordPollEnd(workerID)
	}

	for {
		e, err := h.epicStore.ClaimPendingEpic(ctx)
		if err != nil {
			return err
		}
		if e != nil {
			resp, err := h.buildEpicPollResponse(c, e)
			if err != nil {
				return err
			}
			return server.SetResponse(c, http.StatusOK, resp)
		}

		t, err := h.taskStore.ClaimPendingTask(ctx, nil)
		if err != nil {
			return err
		}
		if t != nil {
			resp, err := h.buildTaskPollResponse(c, t)
			if err != nil {
				return err
			}
			return server.SetResponse(c, http.StatusOK, resp)
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return c.NoContent(http.StatusNoContent)
		}

		select {
		case <-h.epicStore.WaitForPending():
		case <-h.taskStore.WaitForPending():
		case <-time.After(remaining):
			return c.NoContent(http.StatusNoContent)
		case <-ctx.Done():
			return c.NoContent(http.StatusNoContent)
		}
	}
}

func (h *HTTPHandler) buildEpicPollResponse(c echo.Context, e *epic.Epic) (*PollResponse, error) {
	repoID, err := repo.ParseRepoID(e.RepoID)
	if err != nil {
		return nil, err
	}
	r, err := h.repoStore.ReadRepo(c.Request().Context(), repoID)
	if err != nil {
		return nil, err
	}
	var token string
	if h.githubToken != nil {
		token = h.githubToken.GetToken()
	}
	return &PollResponse{
		Type:         "epic",
		Epic:         e,
		GitHubToken:  token,
		RepoFullName: r.FullName,
	}, nil
}

func (h *HTTPHandler) buildTaskPollResponse(c echo.Context, t *task.Task) (*PollResponse, error) {
	repoID, err := repo.ParseRepoID(t.RepoID)
	if err != nil {
		return nil, err
	}
	r, err := h.repoStore.ReadRepo(c.Request().Context(), repoID)
	if err != nil {
		return nil, err
	}
	var token string
	if h.githubToken != nil {
		token = h.githubToken.GetToken()
	}
	return &PollResponse{
		Type:         "task",
		Task:         t,
		GitHubToken:  token,
		RepoFullName: r.FullName,
	}, nil
}

// --- Task Agent Endpoints ---

// TaskAppendLogs handles POST /tasks/:id/logs
func (h *HTTPHandler) TaskAppendLogs(c echo.Context) error {
	req, err := server.BindRequest[TaskLogsRequest](c)
	if err != nil {
		return err
	}
	id := task.MustParseTaskID(req.ID)
	c.Set(logkey.TaskID, id.String())

	attempt := req.Attempt
	if attempt == 0 {
		attempt = 1
	}
	if err := h.taskStore.AppendTaskLogs(c.Request().Context(), id, attempt, req.Logs); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// TaskHeartbeat handles POST /tasks/:id/heartbeat.
func (h *HTTPHandler) TaskHeartbeat(c echo.Context) error {
	req, err := server.BindRequest[TaskIDRequest](c)
	if err != nil {
		return err
	}
	id := task.MustParseTaskID(req.ID)
	c.Set(logkey.TaskID, id.String())

	ctx := c.Request().Context()
	stillRunning, err := h.taskStore.Heartbeat(ctx, id)
	if err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"stopped": !stillRunning,
	})
}

// TaskComplete handles POST /tasks/:id/complete
func (h *HTTPHandler) TaskComplete(c echo.Context) error {
	req, err := server.BindRequest[TaskCompleteRequest](c)
	if err != nil {
		return err
	}
	id := task.MustParseTaskID(req.ID)
	c.Set(logkey.TaskID, id.String())

	ctx := c.Request().Context()

	if req.AgentStatus != "" {
		if err := h.taskStore.SetAgentStatus(ctx, id, req.AgentStatus); err != nil {
			return err
		}
	}
	if req.CostUSD > 0 {
		if err := h.taskStore.AddCost(ctx, id, req.CostUSD); err != nil {
			return err
		}
	}

	switch {
	case !req.Success:
		if req.PrereqFailed != "" {
			if err := h.taskStore.SetCloseReason(ctx, id, req.PrereqFailed); err != nil {
				return err
			}
		}
		if req.Retryable && req.PrereqFailed == "" {
			reason := "rate_limit: " + req.Error
			if err := h.taskStore.ScheduleRetry(ctx, id, reason); err != nil {
				return err
			}
			return c.NoContent(http.StatusNoContent)
		}
		t, readErr := h.taskStore.ReadTask(ctx, id)
		if readErr != nil {
			return readErr
		}
		if req.PrereqFailed == "" && (t.PRNumber > 0 || t.BranchName != "") {
			if err := h.taskStore.UpdateTaskStatus(ctx, id, task.StatusReview); err != nil {
				return err
			}
		} else {
			if err := h.taskStore.UpdateTaskStatus(ctx, id, task.StatusFailed); err != nil {
				return err
			}
		}
	case req.PullRequestURL != "":
		if err := h.taskStore.SetTaskPullRequest(ctx, id, req.PullRequestURL, req.PRNumber); err != nil {
			return err
		}
	case req.BranchName != "":
		if err := h.taskStore.SetTaskBranch(ctx, id, req.BranchName); err != nil {
			return err
		}
	default:
		t, readErr := h.taskStore.ReadTask(ctx, id)
		if readErr != nil {
			return readErr
		}
		if t.PRNumber > 0 || t.BranchName != "" {
			if err := h.taskStore.UpdateTaskStatus(ctx, id, task.StatusReview); err != nil {
				return err
			}
		} else {
			if req.NoChanges {
				if err := h.taskStore.SetCloseReason(ctx, id, "No changes needed — the codebase already meets the required criteria"); err != nil {
					return err
				}
			}
			if err := h.taskStore.UpdateTaskStatus(ctx, id, task.StatusClosed); err != nil {
				return err
			}
		}
	}

	return c.NoContent(http.StatusNoContent)
}

// --- Epic Agent Endpoints ---

// EpicPropose handles POST /epics/:id/propose
func (h *HTTPHandler) EpicPropose(c echo.Context) error {
	req, err := server.BindRequest[ProposeTasksRequest](c)
	if err != nil {
		return err
	}
	id := epic.MustParseEpicID(req.ID)
	c.Set(logkey.EpicID, id.String())

	ctx := c.Request().Context()
	if err := h.epicStore.UpdateProposedTasks(ctx, id, req.Tasks); err != nil {
		return err
	}

	e, err := h.epicStore.ReadEpic(ctx, id)
	if err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, e)
}

// EpicPollFeedback handles GET /epics/:id/poll-feedback
func (h *HTTPHandler) EpicPollFeedback(c echo.Context) error {
	req, err := server.BindRequest[EpicIDRequest](c)
	if err != nil {
		return err
	}
	id := epic.MustParseEpicID(req.ID)
	c.Set(logkey.EpicID, id.String())

	timeout := 30 * time.Second
	deadline := time.Now().Add(timeout)
	ctx := c.Request().Context()

	for {
		feedback, feedbackType, err := h.epicStore.PollFeedback(ctx, id)
		if err != nil {
			return err
		}
		if feedbackType != nil {
			resp := FeedbackResponse{Type: *feedbackType}
			if feedback != nil {
				resp.Feedback = *feedback
			}
			return server.SetResponse(c, http.StatusOK, resp)
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return server.SetResponse(c, http.StatusOK, FeedbackResponse{Type: "timeout"})
		}

		select {
		case <-h.epicStore.WaitForFeedback(id.String()):
		case <-time.After(remaining):
			return server.SetResponse(c, http.StatusOK, FeedbackResponse{Type: "timeout"})
		case <-ctx.Done():
			return server.SetResponse(c, http.StatusOK, FeedbackResponse{Type: "timeout"})
		}
	}
}

// EpicHeartbeat handles POST /epics/:id/heartbeat
func (h *HTTPHandler) EpicHeartbeat(c echo.Context) error {
	req, err := server.BindRequest[EpicIDRequest](c)
	if err != nil {
		return err
	}
	id := epic.MustParseEpicID(req.ID)
	c.Set(logkey.EpicID, id.String())

	if err := h.epicStore.EpicHeartbeat(c.Request().Context(), id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// EpicAppendLogs handles POST /epics/:id/logs
func (h *HTTPHandler) EpicAppendLogs(c echo.Context) error {
	req, err := server.BindRequest[SessionLogRequest](c)
	if err != nil {
		return err
	}
	id := epic.MustParseEpicID(req.ID)
	c.Set(logkey.EpicID, id.String())

	if err := h.epicStore.AppendSessionLog(c.Request().Context(), id, req.Lines); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// --- Worker Observability ---

// ListWorkers handles GET /workers
func (h *HTTPHandler) ListWorkers(c echo.Context) error {
	if h.workerRegistry == nil {
		return server.SetResponseList(c, http.StatusOK, []workertracker.WorkerInfo{}, "")
	}
	workers := h.workerRegistry.ListWorkers(2 * time.Minute)
	if workers == nil {
		workers = []workertracker.WorkerInfo{}
	}
	return server.SetResponseList(c, http.StatusOK, workers, "")
}
