package repoapi

import (
	"net/http"

	"github.com/joshjon/kit/server"
	"github.com/labstack/echo/v4"

	"github.com/joshjon/verve/internal/github"
	"github.com/joshjon/verve/internal/githubtoken"
	"github.com/joshjon/verve/internal/logkey"
	"github.com/joshjon/verve/internal/repo"
)

// HTTPHandler handles repo HTTP requests.
type HTTPHandler struct {
	repoStore          *repo.Store
	githubTokenService *githubtoken.Service
}

// NewHTTPHandler creates a new HTTPHandler.
func NewHTTPHandler(repoStore *repo.Store, githubTokenService *githubtoken.Service) *HTTPHandler {
	return &HTTPHandler{repoStore: repoStore, githubTokenService: githubTokenService}
}

// Register adds the endpoints to the provided Echo router group.
func (h *HTTPHandler) Register(g *echo.Group) {
	g.GET("/repos", h.ListRepos)
	g.POST("/repos", h.AddRepo)
	g.DELETE("/repos/:repo_id", h.RemoveRepo)
	g.GET("/repos/available", h.ListAvailableRepos)
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

	if err := h.repoStore.CreateRepo(c.Request().Context(), r); err != nil {
		return err
	}
	c.Set(logkey.RepoID, r.ID.String())
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

// githubClient returns the current GitHub client, or nil if no token is configured.
func (h *HTTPHandler) githubClient() *github.Client {
	if h.githubTokenService == nil {
		return nil
	}
	return h.githubTokenService.GetClient()
}
