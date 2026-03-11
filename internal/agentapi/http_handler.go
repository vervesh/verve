package agentapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/joshjon/kit/server"
	"github.com/labstack/echo/v4"

	"github.com/joshjon/verve/internal/conversation"
	"github.com/joshjon/verve/internal/redact"
	"github.com/joshjon/verve/internal/epic"
	"github.com/joshjon/verve/internal/githubtoken"
	"github.com/joshjon/verve/internal/logkey"
	"github.com/joshjon/verve/internal/repo"
	"github.com/joshjon/verve/internal/task"
	"github.com/joshjon/verve/internal/workertracker"
)

// HTTPHandler handles agent-facing API requests.
type HTTPHandler struct {
	taskStore         *task.Store
	epicStore         *epic.Store
	repoStore         *repo.Store
	conversationStore *conversation.Store
	githubToken       *githubtoken.Service
	workerRegistry    *workertracker.Registry
}

// NewHTTPHandler creates a new HTTPHandler.
func NewHTTPHandler(taskStore *task.Store, epicStore *epic.Store, repoStore *repo.Store, conversationStore *conversation.Store, githubToken *githubtoken.Service, workerRegistry *workertracker.Registry) *HTTPHandler {
	return &HTTPHandler{
		taskStore:         taskStore,
		epicStore:         epicStore,
		repoStore:         repoStore,
		conversationStore: conversationStore,
		githubToken:       githubToken,
		workerRegistry:    workerRegistry,
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
	g.POST("/epics/:id/complete", h.EpicComplete)
	g.POST("/epics/:id/heartbeat", h.EpicHeartbeat)
	g.POST("/epics/:id/logs", h.EpicAppendLogs)

	// Conversation agent endpoints
	g.POST("/conversations/:id/complete", h.ConversationComplete)
	g.POST("/conversations/:id/heartbeat", h.ConversationHeartbeat)
	g.POST("/conversations/:id/logs", h.ConversationAppendLogs)

	// Repo setup agent endpoints
	g.POST("/repos/:repo_id/setup-complete", h.RepoSetupComplete)
	g.POST("/repos/:repo_id/setup-heartbeat", h.RepoSetupHeartbeat)
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

		if h.conversationStore != nil {
			conv, err := h.conversationStore.ClaimPendingConversation(ctx)
			if err != nil {
				return err
			}
			if conv != nil {
				resp, err := h.buildConversationPollResponse(c, conv)
				if err != nil {
					return err
				}
				return server.SetResponse(c, http.StatusOK, resp)
			}
		}

		t, err := h.taskStore.ClaimPendingTask(ctx, nil)
		if err != nil {
			return err
		}
		if t != nil {
			if t.Type == task.TaskTypeSetup || t.Type == task.TaskTypeSetupReview {
				resp, err := h.buildSetupPollResponse(c, t)
				if err != nil {
					return err
				}
				return server.SetResponse(c, http.StatusOK, resp)
			}
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

		var convPending <-chan struct{}
		if h.conversationStore != nil {
			convPending = h.conversationStore.WaitForPending()
		}

		select {
		case <-h.epicStore.WaitForPending():
		case <-convPending:
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
		Type:             "epic",
		Epic:             e,
		GitHubToken:      token,
		RepoFullName:     r.FullName,
		RepoSummary:      r.Summary,
		RepoExpectations: r.Expectations,
		RepoTechStack:    strings.Join(r.TechStack, ", "),
	}, nil
}

func (h *HTTPHandler) buildSetupPollResponse(c echo.Context, t *task.Task) (*PollResponse, error) {
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

	workType := "setup"
	if t.Type == task.TaskTypeSetupReview {
		workType = "setup-review"
	}

	return &PollResponse{
		Type: workType,
		Setup: &Setup{
			TaskID:   t.ID.String(),
			RepoID:   t.RepoID,
			FullName: r.FullName,
		},
		GitHubToken:      token,
		RepoFullName:     r.FullName,
		RepoSummary:      r.Summary,
		RepoExpectations: r.Expectations,
		RepoTechStack:    strings.Join(r.TechStack, ", "),
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
		Type:             "task",
		Task:             t,
		GitHubToken:      token,
		RepoFullName:     r.FullName,
		RepoSummary:      r.Summary,
		RepoExpectations: r.Expectations,
		RepoTechStack:    strings.Join(r.TechStack, ", "),
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
	if err := h.taskStore.AppendTaskLogs(c.Request().Context(), id, attempt, redact.Lines(req.Logs)); err != nil {
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
		if req.Retryable {
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
		if t.PRNumber > 0 || t.BranchName != "" {
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

// EpicComplete handles POST /epics/:id/complete — agent reports planning result.
func (h *HTTPHandler) EpicComplete(c echo.Context) error {
	req, err := server.BindRequest[EpicCompleteRequest](c)
	if err != nil {
		return err
	}
	id := epic.MustParseEpicID(req.ID)
	c.Set(logkey.EpicID, id.String())

	ctx := c.Request().Context()
	if req.Success && len(req.Tasks) > 0 {
		if err := h.epicStore.CompletePlanning(ctx, id, req.Tasks); err != nil {
			return err
		}
	} else {
		if err := h.epicStore.FailPlanning(ctx, id); err != nil {
			return err
		}
	}
	return c.NoContent(http.StatusNoContent)
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

	if err := h.epicStore.AppendSessionLog(c.Request().Context(), id, redact.Lines(req.Lines)); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// --- Repo Setup Agent Endpoint ---

// RepoSetupComplete handles POST /repos/:repo_id/setup-complete
func (h *HTTPHandler) RepoSetupComplete(c echo.Context) error {
	req, err := server.BindRequest[RepoSetupCompleteRequest](c)
	if err != nil {
		return err
	}

	repoID := repo.MustParseRepoID(req.RepoID)
	c.Set(logkey.RepoID, repoID.String())

	ctx := c.Request().Context()

	if !req.Success {
		// On failure, leave status as scanning so the user can rescan later.
		return c.NoContent(http.StatusNoContent)
	}

	// Determine the target setup status.
	setupStatus := repo.SetupStatusReady
	if req.NeedsSetup {
		setupStatus = repo.SetupStatusNeedsSetup
	}

	result := repo.SetupScanResult{
		Summary:     req.Summary,
		TechStack:   req.TechStack,
		HasCode:     req.HasCode,
		HasCLAUDEMD: req.HasClaudeMD,
		HasREADME:   req.HasREADME,
		SetupStatus: setupStatus,
	}

	if err := h.repoStore.UpdateRepoSetupScan(ctx, repoID, result); err != nil {
		return err
	}

	// If the agent provided enhanced expectations (from setup-review), update them.
	if req.Expectations != "" {
		update := repo.ExpectationsUpdate{
			Expectations: req.Expectations,
		}
		if err := h.repoStore.UpdateRepoExpectations(ctx, repoID, update); err != nil {
			return err
		}
	}

	// Read updated repo and publish SSE event.
	r, err := h.repoStore.ReadRepo(ctx, repoID)
	if err != nil {
		return err
	}
	h.taskStore.PublishRepoEvent(ctx, repoID.String(), r)

	return c.NoContent(http.StatusNoContent)
}

// RepoSetupHeartbeat handles POST /repos/:repo_id/setup-heartbeat.
// Uses the underlying task heartbeat mechanism since setup scans run as tasks.
func (h *HTTPHandler) RepoSetupHeartbeat(c echo.Context) error {
	req, err := server.BindRequest[RepoSetupHeartbeatRequest](c)
	if err != nil {
		return err
	}
	_ = repo.MustParseRepoID(req.RepoID)
	c.Set(logkey.RepoID, req.RepoID)
	return c.NoContent(http.StatusNoContent)
}

// --- Conversation Agent Endpoints ---

// ConversationComplete handles POST /conversations/:id/complete
func (h *HTTPHandler) ConversationComplete(c echo.Context) error {
	req, err := server.BindRequest[ConversationCompleteRequest](c)
	if err != nil {
		return err
	}
	id := conversation.MustParseConversationID(req.ID)
	c.Set(logkey.ConversationID, id.String())

	ctx := c.Request().Context()
	if req.Success {
		if err := h.conversationStore.CompleteResponse(ctx, id, req.Response); err != nil {
			return err
		}
	} else {
		if err := h.conversationStore.FailResponse(ctx, id); err != nil {
			return err
		}
	}
	return c.NoContent(http.StatusNoContent)
}

// ConversationHeartbeat handles POST /conversations/:id/heartbeat
func (h *HTTPHandler) ConversationHeartbeat(c echo.Context) error {
	req, err := server.BindRequest[ConversationIDRequest](c)
	if err != nil {
		return err
	}
	id := conversation.MustParseConversationID(req.ID)
	c.Set(logkey.ConversationID, id.String())

	if err := h.conversationStore.ConversationHeartbeat(c.Request().Context(), id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// ConversationAppendLogs handles POST /conversations/:id/logs
func (h *HTTPHandler) ConversationAppendLogs(c echo.Context) error {
	req, err := server.BindRequest[ConversationLogsRequest](c)
	if err != nil {
		return err
	}
	id := conversation.MustParseConversationID(req.ID)
	c.Set(logkey.ConversationID, id.String())

	// Conversation logs are acknowledged but not persisted separately.
	// The primary conversation content flows through the complete endpoint.
	_ = id
	return c.NoContent(http.StatusNoContent)
}

func (h *HTTPHandler) buildConversationPollResponse(c echo.Context, conv *conversation.Conversation) (*PollResponse, error) {
	repoID, err := repo.ParseRepoID(conv.RepoID)
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
		Type:             "conversation",
		Conversation:     conv,
		GitHubToken:      token,
		RepoFullName:     r.FullName,
		RepoSummary:      r.Summary,
		RepoExpectations: r.Expectations,
		RepoTechStack:    strings.Join(r.TechStack, ", "),
	}, nil
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
