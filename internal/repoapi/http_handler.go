package repoapi

import (
	"net/http"
	"time"

	"github.com/joshjon/kit/server"
	"github.com/labstack/echo/v4"

	"github.com/joshjon/verve/internal/github"
	"github.com/joshjon/verve/internal/githubtoken"
	"github.com/joshjon/verve/internal/logkey"
	"github.com/joshjon/verve/internal/repo"
	"github.com/joshjon/verve/internal/task"
)

// HTTPHandler handles repo HTTP requests.
type HTTPHandler struct {
	repoStore          *repo.Store
	taskStore          *task.Store
	githubTokenService *githubtoken.Service
}

// NewHTTPHandler creates a new HTTPHandler.
func NewHTTPHandler(repoStore *repo.Store, taskStore *task.Store, githubTokenService *githubtoken.Service) *HTTPHandler {
	return &HTTPHandler{repoStore: repoStore, taskStore: taskStore, githubTokenService: githubTokenService}
}

// Register adds the endpoints to the provided Echo router group.
func (h *HTTPHandler) Register(g *echo.Group) {
	g.GET("/repos", h.ListRepos)
	g.POST("/repos", h.AddRepo)
	g.DELETE("/repos/:repo_id", h.RemoveRepo)
	g.GET("/repos/available", h.ListAvailableRepos)

	// Setup endpoints
	g.GET("/repos/:repo_id/setup", h.GetSetup)
	g.PUT("/repos/:repo_id/setup/expectations", h.UpdateExpectations)
	g.PUT("/repos/:repo_id/setup/summary", h.UpdateSummary)
	g.POST("/repos/:repo_id/setup/rescan", h.Rescan)
	g.POST("/repos/:repo_id/setup/skip", h.SkipSetup)
}

// ListRepos handles GET /repos
func (h *HTTPHandler) ListRepos(c echo.Context) error {
	repos, err := h.repoStore.ListRepos(c.Request().Context())
	if err != nil {
		return err
	}
	return server.SetResponseList(c, http.StatusOK, repos, "")
}

// AddRepo handles POST /repos
func (h *HTTPHandler) AddRepo(c echo.Context) error {
	req, err := server.BindRequest[AddRepoRequest](c)
	if err != nil {
		return err
	}

	r, err := repo.NewRepo(req.FullName)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	ctx := c.Request().Context()

	if err := h.repoStore.CreateRepo(ctx, r); err != nil {
		return err
	}
	c.Set(logkey.RepoID, r.ID.String())

	// Auto-trigger setup scan: create an internal setup task and set status to scanning.
	setupTask := task.NewSetupTask(r.ID.String())
	if err := h.taskStore.CreateTask(ctx, setupTask); err != nil {
		return err
	}
	if err := h.repoStore.UpdateRepoSetupStatus(ctx, r.ID, repo.SetupStatusScanning); err != nil {
		return err
	}
	r.SetupStatus = repo.SetupStatusScanning

	return server.SetResponse(c, http.StatusCreated, r)
}

// RemoveRepo handles DELETE /repos/:repo_id
func (h *HTTPHandler) RemoveRepo(c echo.Context) error {
	req, err := server.BindRequest[RemoveRepoRequest](c)
	if err != nil {
		return err
	}

	id := repo.MustParseRepoID(req.RepoID)
	c.Set(logkey.RepoID, id.String())

	if err := h.repoStore.DeleteRepo(c.Request().Context(), id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// ListAvailableRepos handles GET /repos/available
func (h *HTTPHandler) ListAvailableRepos(c echo.Context) error {
	gh := h.githubClient()
	if gh == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "GitHub token not configured")
	}

	repos, err := gh.ListAccessibleRepos(c.Request().Context())
	if err != nil {
		return err
	}
	return server.SetResponseList(c, http.StatusOK, repos, "")
}

// GetSetup handles GET /repos/:repo_id/setup
func (h *HTTPHandler) GetSetup(c echo.Context) error {
	req, err := server.BindRequest[RepoIDRequest](c)
	if err != nil {
		return err
	}

	id := repo.MustParseRepoID(req.RepoID)
	c.Set(logkey.RepoID, id.String())

	r, err := h.repoStore.ReadRepo(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, r)
}

// UpdateExpectations handles PUT /repos/:repo_id/setup/expectations
func (h *HTTPHandler) UpdateExpectations(c echo.Context) error {
	req, err := server.BindRequest[UpdateExpectationsRequest](c)
	if err != nil {
		return err
	}

	id := repo.MustParseRepoID(req.RepoID)
	c.Set(logkey.RepoID, id.String())

	ctx := c.Request().Context()

	update := repo.ExpectationsUpdate{
		Expectations: req.Expectations,
	}
	if req.MarkReady {
		now := time.Now()
		update.SetupCompletedAt = &now
	}

	if err := h.repoStore.UpdateRepoExpectations(ctx, id, update); err != nil {
		return err
	}

	if req.MarkReady {
		if err := h.repoStore.UpdateRepoSetupStatus(ctx, id, repo.SetupStatusReady); err != nil {
			return err
		}
	}

	r, err := h.repoStore.ReadRepo(ctx, id)
	if err != nil {
		return err
	}

	// Publish SSE event for setup status change.
	h.taskStore.PublishRepoEvent(ctx, id.String(), r)

	return server.SetResponse(c, http.StatusOK, r)
}

// Rescan handles POST /repos/:repo_id/setup/rescan
func (h *HTTPHandler) Rescan(c echo.Context) error {
	req, err := server.BindRequest[RepoIDRequest](c)
	if err != nil {
		return err
	}

	id := repo.MustParseRepoID(req.RepoID)
	c.Set(logkey.RepoID, id.String())

	ctx := c.Request().Context()

	// Transition to scanning.
	if err := h.repoStore.UpdateRepoSetupStatus(ctx, id, repo.SetupStatusScanning); err != nil {
		return err
	}

	// Create a new setup scan task.
	setupTask := task.NewSetupTask(id.String())
	if err := h.taskStore.CreateTask(ctx, setupTask); err != nil {
		return err
	}

	r, err := h.repoStore.ReadRepo(ctx, id)
	if err != nil {
		return err
	}

	// Publish SSE event for status change.
	h.taskStore.PublishRepoEvent(ctx, id.String(), r)

	return server.SetResponse(c, http.StatusOK, r)
}

// UpdateSummary handles PUT /repos/:repo_id/setup/summary
func (h *HTTPHandler) UpdateSummary(c echo.Context) error {
	req, err := server.BindRequest[UpdateSummaryRequest](c)
	if err != nil {
		return err
	}

	id := repo.MustParseRepoID(req.RepoID)
	c.Set(logkey.RepoID, id.String())

	ctx := c.Request().Context()

	if err := h.repoStore.UpdateRepoSummary(ctx, id, req.Summary); err != nil {
		return err
	}

	r, err := h.repoStore.ReadRepo(ctx, id)
	if err != nil {
		return err
	}

	h.taskStore.PublishRepoEvent(ctx, id.String(), r)

	return server.SetResponse(c, http.StatusOK, r)
}

// SkipSetup handles POST /repos/:repo_id/setup/skip — marks a repo as ready
// without requiring a scan. Useful for pre-existing repos that were added
// before the setup scan feature.
func (h *HTTPHandler) SkipSetup(c echo.Context) error {
	req, err := server.BindRequest[RepoIDRequest](c)
	if err != nil {
		return err
	}

	id := repo.MustParseRepoID(req.RepoID)
	c.Set(logkey.RepoID, id.String())

	ctx := c.Request().Context()

	now := time.Now()
	update := repo.ExpectationsUpdate{
		SetupCompletedAt: &now,
	}
	if err := h.repoStore.UpdateRepoExpectations(ctx, id, update); err != nil {
		return err
	}

	if err := h.repoStore.UpdateRepoSetupStatus(ctx, id, repo.SetupStatusReady); err != nil {
		return err
	}

	r, err := h.repoStore.ReadRepo(ctx, id)
	if err != nil {
		return err
	}

	h.taskStore.PublishRepoEvent(ctx, id.String(), r)

	return server.SetResponse(c, http.StatusOK, r)
}

// githubClient returns the current GitHub client, or nil if no token is configured.
func (h *HTTPHandler) githubClient() *github.Client {
	if h.githubTokenService == nil {
		return nil
	}
	return h.githubTokenService.GetClient()
}
