package eventapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/joshjon/verve/internal/repo"
	"github.com/joshjon/verve/internal/task"
)

// HTTPHandler handles SSE event HTTP requests.
type HTTPHandler struct {
	taskStore *task.Store
	repoStore *repo.Store
}

// NewHTTPHandler creates a new HTTPHandler.
func NewHTTPHandler(taskStore *task.Store, repoStore *repo.Store) *HTTPHandler {
	return &HTTPHandler{taskStore: taskStore, repoStore: repoStore}
}

// Register adds the endpoints to the provided Echo router group.
func (h *HTTPHandler) Register(g *echo.Group) {
	g.GET("/events", h.Events)
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
		tasks, err = h.taskStore.ListTasksByRepo(ctx, repoIDFilter)
	} else {
		tasks, err = h.taskStore.ListTasks(ctx)
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
	ch := h.taskStore.Subscribe()
	defer h.taskStore.Unsubscribe(ch)

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
