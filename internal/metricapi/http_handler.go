package metricapi

import (
	"net/http"
	"time"

	"github.com/joshjon/kit/server"
	"github.com/labstack/echo/v4"

	"github.com/joshjon/verve/internal/metric"
	"github.com/joshjon/verve/internal/task"
	"github.com/joshjon/verve/internal/workertracker"
)

// HTTPHandler handles metrics HTTP requests.
type HTTPHandler struct {
	store          *task.Store
	epicLister     metric.PlanningEpicLister
	workerRegistry *workertracker.Registry
}

// NewHTTPHandler creates a new HTTPHandler.
func NewHTTPHandler(store *task.Store, epicLister metric.PlanningEpicLister, workerRegistry *workertracker.Registry) *HTTPHandler {
	return &HTTPHandler{store: store, epicLister: epicLister, workerRegistry: workerRegistry}
}

// Register adds the endpoints to the provided Echo router group.
func (h *HTTPHandler) Register(g *echo.Group) {
	g.GET("/metrics", h.GetMetrics)
}

// GetMetrics handles GET /metrics
// Returns a snapshot of current agent activity and performance metrics.
func (h *HTTPHandler) GetMetrics(c echo.Context) error {
	var workers []workertracker.WorkerInfo
	if h.workerRegistry != nil {
		workers = h.workerRegistry.ListWorkers(2 * time.Minute)
	}

	metrics, err := metric.Compute(c.Request().Context(), h.store, h.epicLister, workers)
	if err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, metrics)
}
