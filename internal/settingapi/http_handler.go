package settingapi

import (
	"net/http"

	"github.com/joshjon/kit/server"
	"github.com/labstack/echo/v4"

	"github.com/joshjon/verve/internal/githubtoken"
	"github.com/joshjon/verve/internal/setting"
)

// HTTPHandler handles settings HTTP requests.
type HTTPHandler struct {
	githubTokenService *githubtoken.Service
	settingService     *setting.Service
	models             []setting.ModelOption
}

// NewHTTPHandler creates a new HTTPHandler.
func NewHTTPHandler(githubTokenService *githubtoken.Service, settingService *setting.Service, models []setting.ModelOption) *HTTPHandler {
	if len(models) == 0 {
		models = setting.DefaultModels
	}
	return &HTTPHandler{githubTokenService: githubTokenService, settingService: settingService, models: models}
}

// Register adds the endpoints to the provided Echo router group.
func (h *HTTPHandler) Register(g *echo.Group) {
	g.PUT("/settings/github-token", h.SaveGitHubToken)
	g.GET("/settings/github-token", h.GetGitHubTokenStatus)
	g.DELETE("/settings/github-token", h.DeleteGitHubToken)
	g.PUT("/settings/default-model", h.SaveDefaultModel)
	g.GET("/settings/default-model", h.GetDefaultModel)
	g.DELETE("/settings/default-model", h.DeleteDefaultModel)
	g.GET("/settings/models", h.ListModels)
}

// SaveGitHubToken handles PUT /settings/github-token
func (h *HTTPHandler) SaveGitHubToken(c echo.Context) error {
	if h.githubTokenService == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "encryption key not configured")
	}

	req, err := server.BindRequest[SaveGitHubTokenRequest](c)
	if err != nil {
		return err
	}

	if err := h.githubTokenService.SaveToken(c.Request().Context(), req.Token); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// GetGitHubTokenStatus handles GET /settings/github-token
func (h *HTTPHandler) GetGitHubTokenStatus(c echo.Context) error {
	configured := h.githubTokenService != nil && h.githubTokenService.HasToken()
	fineGrained := h.githubTokenService != nil && h.githubTokenService.IsFineGrained()
	return server.SetResponse(c, http.StatusOK, GitHubTokenStatusResponse{Configured: configured, FineGrained: fineGrained})
}

// DeleteGitHubToken handles DELETE /settings/github-token
func (h *HTTPHandler) DeleteGitHubToken(c echo.Context) error {
	if h.githubTokenService == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "encryption key not configured")
	}

	if err := h.githubTokenService.DeleteToken(c.Request().Context()); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// SaveDefaultModel handles PUT /settings/default-model
func (h *HTTPHandler) SaveDefaultModel(c echo.Context) error {
	if h.settingService == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "settings not available")
	}

	req, err := server.BindRequest[DefaultModelRequest](c)
	if err != nil {
		return err
	}

	if err := h.settingService.Set(c.Request().Context(), setting.KeyDefaultModel, req.Model); err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, DefaultModelResponse{Model: req.Model, Configured: true})
}

// GetDefaultModel handles GET /settings/default-model
func (h *HTTPHandler) GetDefaultModel(c echo.Context) error {
	var model string
	if h.settingService != nil {
		model = h.settingService.Get(setting.KeyDefaultModel)
	}
	return server.SetResponse(c, http.StatusOK, DefaultModelResponse{Model: model, Configured: model != ""})
}

// DeleteDefaultModel handles DELETE /settings/default-model
func (h *HTTPHandler) DeleteDefaultModel(c echo.Context) error {
	if h.settingService == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "settings not available")
	}

	if err := h.settingService.Delete(c.Request().Context(), setting.KeyDefaultModel); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// ListModels handles GET /settings/models
func (h *HTTPHandler) ListModels(c echo.Context) error {
	return server.SetResponseList(c, http.StatusOK, h.models, "")
}
