package taskapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/joshjon/kit/server"
	"github.com/joshjon/kit/testutil"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/repo"
	"github.com/joshjon/verve/internal/sqlite"
	"github.com/joshjon/verve/internal/task"
	"github.com/joshjon/verve/internal/taskapi"
)

type fixture struct {
	Server   *server.Server
	TaskRepo task.Repository
	RepoStore *repo.Store
	Repo     *repo.Repo
	t        *testing.T
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	db := sqlite.NewTestDB(t)
	taskRepo := sqlite.NewTaskRepository(db)
	broker := task.NewBroker(nil)
	taskStore := task.NewStore(taskRepo, broker)

	repoRepo := sqlite.NewRepoRepository(db)
	repoStore := repo.NewStore(repoRepo, taskRepo)

	handler := taskapi.NewHTTPHandler(taskStore, repoStore, nil, nil, nil)

	srv, err := server.NewServer(testutil.GetFreePort(t))
	require.NoError(t, err)
	srv.Register("/api/v1", handler)

	go srv.Start()
	err = srv.WaitHealthy(10, 100*time.Millisecond)
	require.NoError(t, err)

	// Pre-create a repo for use in tests.
	r, _ := repo.NewRepo("owner/test-repo")
	require.NoError(t, repoStore.CreateRepo(context.Background(), r))

	t.Cleanup(func() { srv.Stop(context.Background()) })

	return &fixture{
		Server:   srv,
		TaskRepo: taskRepo,
		RepoStore: repoStore,
		Repo:     r,
		t:        t,
	}
}

// --- URL helpers ---

func (f *fixture) repoTasksURL() string {
	return fmt.Sprintf("%s/api/v1/repos/%s/tasks", f.Server.Address(), f.Repo.ID)
}

func (f *fixture) taskURL(id task.TaskID) string {
	return fmt.Sprintf("%s/api/v1/tasks/%s", f.Server.Address(), id)
}

func (f *fixture) taskActionURL(id task.TaskID, action string) string {
	return fmt.Sprintf("%s/%s", f.taskURL(id), action)
}

// --- Seed helpers ---

func (f *fixture) seedTask(title, description string) *task.Task {
	f.t.Helper()
	ctx := context.Background()
	tsk := task.NewTask(f.Repo.ID.String(), title, description, nil, nil, 0, false, "sonnet", true)
	require.NoError(f.t, f.TaskRepo.CreateTask(ctx, tsk))
	return tsk
}

func (f *fixture) seedRunningTask(title, description string) *task.Task {
	f.t.Helper()
	ctx := context.Background()
	tsk := f.seedTask(title, description)
	require.NoError(f.t, f.TaskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	return tsk
}

func (f *fixture) readTask(id task.TaskID) *task.Task {
	f.t.Helper()
	tsk, err := f.TaskRepo.ReadTask(context.Background(), id)
	require.NoError(f.t, err)
	return tsk
}

// --- HTTP helpers for non-standard cases ---

// postNoContent sends a POST request and asserts 204 No Content.
func postNoContent(t *testing.T, url string, body any) {
	t.Helper()
	httpRes, err := testutil.DefaultClient.Post(url, "application/json", mustJSONReader(body))
	require.NoError(t, err)
	defer httpRes.Body.Close()
	if httpRes.StatusCode < 200 || httpRes.StatusCode >= 300 {
		respBody, _ := io.ReadAll(httpRes.Body)
		require.Failf(t, "http error", "POST %s\nStatus: %s\nBody: %s", url, httpRes.Status, string(respBody))
	}
}

// doJSON sends an HTTP request with a JSON body and returns the raw response (for error tests).
func doJSON(t *testing.T, method, url string, body any) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, mustJSONReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	httpRes, err := testutil.DefaultClient.Do(req)
	require.NoError(t, err)
	return httpRes
}

func mustJSONReader(v any) io.Reader {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return bytes.NewReader(b)
}
