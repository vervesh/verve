package epicapi

import (
	"net/http"

	"github.com/joshjon/kit/server"
	"github.com/labstack/echo/v4"

	"github.com/joshjon/verve/internal/epic"
	"github.com/joshjon/verve/internal/logkey"
	"github.com/joshjon/verve/internal/repo"
	"github.com/joshjon/verve/internal/setting"
	"github.com/joshjon/verve/internal/task"
)

// HTTPHandler handles epic HTTP requests.
type HTTPHandler struct {
	store          *epic.Store
	repoStore      *repo.Store
	taskStore      *task.Store
	settingService *setting.Service
}

// NewHTTPHandler creates a new HTTPHandler.
func NewHTTPHandler(store *epic.Store, repoStore *repo.Store, taskStore *task.Store, settingService *setting.Service) *HTTPHandler {
	return &HTTPHandler{store: store, repoStore: repoStore, taskStore: taskStore, settingService: settingService}
}

// Register adds the epic endpoints to the provided Echo router group.
func (h *HTTPHandler) Register(g *echo.Group) {
	// Epic CRUD (repo-scoped)
	g.POST("/repos/:repo_id/epics", h.CreateEpic)
	g.GET("/repos/:repo_id/epics", h.ListEpicsByRepo)

	// Epic operations (globally unique IDs)
	g.GET("/epics/:id", h.GetEpic)
	g.GET("/epics/:id/tasks", h.GetEpicTasks)
	g.DELETE("/epics/:id", h.DeleteEpic)

	// Planning session
	g.POST("/epics/:id/plan", h.StartPlanning)
	g.PUT("/epics/:id/proposed-tasks", h.UpdateProposedTasks)
	g.POST("/epics/:id/session-message", h.SendSessionMessage)

	// Confirmation
	g.POST("/epics/:id/confirm", h.ConfirmEpic)
	g.POST("/epics/:id/close", h.CloseEpic)
}

// CreateEpic handles POST /repos/:repo_id/epics
func (h *HTTPHandler) CreateEpic(c echo.Context) error {
	req, err := server.BindRequest[CreateEpicRequest](c)
	if err != nil {
		return err
	}
	repoID := repo.MustParseRepoID(req.RepoID)
	c.Set(logkey.RepoID, repoID.String())

	// Block epic creation until repo setup is complete.
	r, err := h.repoStore.ReadRepo(c.Request().Context(), repoID)
	if err != nil {
		return err
	}
	if r.SetupStatus != repo.SetupStatusReady {
		return echo.NewHTTPError(http.StatusConflict, "repository setup is not complete — finish setup before adding epics")
	}

	e := epic.NewEpic(repoID.String(), req.Title, req.Description)
	e.PlanningPrompt = req.PlanningPrompt

	model := req.Model
	if model == "" && h.settingService != nil {
		model = h.settingService.Get(setting.KeyDefaultModel)
	}
	if model == "" {
		model = "sonnet"
	}
	e.Model = model

	if err := h.store.CreateEpic(c.Request().Context(), e); err != nil {
		return err
	}

	c.Set(logkey.EpicID, e.ID.String())
	return server.SetResponse(c, http.StatusCreated, e)
}

// ListEpicsByRepo handles GET /repos/:repo_id/epics
func (h *HTTPHandler) ListEpicsByRepo(c echo.Context) error {
	req, err := server.BindRequest[RepoIDRequest](c)
	if err != nil {
		return err
	}
	repoID := repo.MustParseRepoID(req.RepoID)
	c.Set(logkey.RepoID, repoID.String())

	epics, err := h.store.ListEpicsByRepo(c.Request().Context(), repoID.String())
	if err != nil {
		return err
	}
	return server.SetResponseList(c, http.StatusOK, epics, "")
}

// GetEpic handles GET /epics/:id
func (h *HTTPHandler) GetEpic(c echo.Context) error {
	req, err := server.BindRequest[EpicIDRequest](c)
	if err != nil {
		return err
	}
	id := epic.MustParseEpicID(req.ID)
	c.Set(logkey.EpicID, id.String())

	e, err := h.store.ReadEpic(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, e)
}

// DeleteEpic handles DELETE /epics/:id
func (h *HTTPHandler) DeleteEpic(c echo.Context) error {
	req, err := server.BindRequest[EpicIDRequest](c)
	if err != nil {
		return err
	}
	id := epic.MustParseEpicID(req.ID)
	c.Set(logkey.EpicID, id.String())

	ctx := c.Request().Context()

	if _, err := h.store.ReadEpic(ctx, id); err != nil {
		return err
	}

	if h.taskStore != nil {
		if err := h.taskStore.BulkDeleteTasksByEpic(ctx, id.String()); err != nil {
			c.Logger().Errorf("failed to bulk delete tasks for epic %s: %v", id, err)
		}
	}

	if err := h.store.DeleteEpic(ctx, id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// StartPlanning handles POST /epics/:id/plan
func (h *HTTPHandler) StartPlanning(c echo.Context) error {
	req, err := server.BindRequest[StartPlanningRequest](c)
	if err != nil {
		return err
	}
	id := epic.MustParseEpicID(req.ID)
	c.Set(logkey.EpicID, id.String())

	ctx := c.Request().Context()
	if err := h.store.StartPlanning(ctx, id, req.Prompt); err != nil {
		return err
	}

	e, err := h.store.ReadEpic(ctx, id)
	if err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, e)
}

// UpdateProposedTasks handles PUT /epics/:id/proposed-tasks
func (h *HTTPHandler) UpdateProposedTasks(c echo.Context) error {
	req, err := server.BindRequest[UpdateProposedTasksRequest](c)
	if err != nil {
		return err
	}
	id := epic.MustParseEpicID(req.ID)
	c.Set(logkey.EpicID, id.String())

	ctx := c.Request().Context()
	if err := h.store.UpdateProposedTasks(ctx, id, req.Tasks); err != nil {
		return err
	}

	e, err := h.store.ReadEpic(ctx, id)
	if err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, e)
}

// SendSessionMessage handles POST /epics/:id/session-message — queues a change
// request for the epic plan. The epic transitions back to planning status and
// a worker will pick it up with the feedback as context.
func (h *HTTPHandler) SendSessionMessage(c echo.Context) error {
	req, err := server.BindRequest[SessionMessageRequest](c)
	if err != nil {
		return err
	}
	id := epic.MustParseEpicID(req.ID)
	c.Set(logkey.EpicID, id.String())

	ctx := c.Request().Context()
	if err := h.store.AppendSessionLog(ctx, id, []string{"user: " + req.Message}); err != nil {
		return err
	}

	if err := h.store.RequestChanges(ctx, id, req.Message); err != nil {
		return err
	}

	e, err := h.store.ReadEpic(ctx, id)
	if err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, e)
}

// ConfirmEpic handles POST /epics/:id/confirm
func (h *HTTPHandler) ConfirmEpic(c echo.Context) error {
	req, err := server.BindRequest[ConfirmEpicRequest](c)
	if err != nil {
		return err
	}
	id := epic.MustParseEpicID(req.ID)
	c.Set(logkey.EpicID, id.String())

	ctx := c.Request().Context()
	if err := h.store.ConfirmEpic(ctx, id, req.NotReady); err != nil {
		return err
	}

	e, err := h.store.ReadEpic(ctx, id)
	if err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, e)
}

// CloseEpic handles POST /epics/:id/close
func (h *HTTPHandler) CloseEpic(c echo.Context) error {
	req, err := server.BindRequest[EpicIDRequest](c)
	if err != nil {
		return err
	}
	id := epic.MustParseEpicID(req.ID)
	c.Set(logkey.EpicID, id.String())

	ctx := c.Request().Context()
	if err := h.store.CloseEpic(ctx, id); err != nil {
		return err
	}

	if h.taskStore != nil {
		if err := h.taskStore.BulkCloseTasksByEpic(ctx, id.String(), "Epic closed"); err != nil {
			c.Logger().Errorf("failed to bulk close tasks for epic %s: %v", id, err)
		}
	}

	e, err := h.store.ReadEpic(ctx, id)
	if err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, e)
}

// EpicTaskSummary contains the status summary for a task in an epic.
type EpicTaskSummary struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// GetEpicTasks handles GET /epics/:id/tasks
func (h *HTTPHandler) GetEpicTasks(c echo.Context) error {
	req, err := server.BindRequest[EpicIDRequest](c)
	if err != nil {
		return err
	}
	id := epic.MustParseEpicID(req.ID)
	c.Set(logkey.EpicID, id.String())

	ctx := c.Request().Context()
	e, err := h.store.ReadEpic(ctx, id)
	if err != nil {
		return err
	}

	summaries := make([]EpicTaskSummary, 0, len(e.TaskIDs))
	for _, taskIDStr := range e.TaskIDs {
		taskID, parseErr := task.ParseTaskID(taskIDStr)
		if parseErr != nil {
			continue
		}
		t, readErr := h.taskStore.ReadTask(ctx, taskID)
		if readErr != nil {
			continue
		}
		summaries = append(summaries, EpicTaskSummary{
			ID:     t.ID.String(),
			Title:  t.Title,
			Status: string(t.Status),
		})
	}

	return server.SetResponseList(c, http.StatusOK, summaries, "")
}
