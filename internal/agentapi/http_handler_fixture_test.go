package agentapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/joshjon/kit/log"
	"github.com/joshjon/kit/server"
	"github.com/joshjon/kit/testutil"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/agentapi"
	"github.com/joshjon/verve/internal/epic"
	"github.com/joshjon/verve/internal/repo"
	"github.com/joshjon/verve/internal/sqlite"
	"github.com/joshjon/verve/internal/task"
	"github.com/joshjon/verve/internal/workertracker"
)

type fixture struct {
	Server         *server.Server
	TaskStore      *task.Store
	EpicStore      *epic.Store
	RepoStore      *repo.Store
	WorkerRegistry *workertracker.Registry
	Repo           *repo.Repo
	t              *testing.T

	taskRepo task.Repository
	epicRepo epic.Repository
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	db := sqlite.NewTestDB(t)
	taskRepo := sqlite.NewTaskRepository(db)
	broker := task.NewBroker(nil)
	taskStore := task.NewStore(taskRepo, broker)

	repoRepo := sqlite.NewRepoRepository(db)
	repoStore := repo.NewStore(repoRepo, taskRepo)

	epicRepo := sqlite.NewEpicRepository(db)
	logger := log.NewLogger(log.WithNop())
	epicStore := epic.NewStore(epicRepo, nil, logger)

	registry := workertracker.New()

	handler := agentapi.NewHTTPHandler(taskStore, epicStore, repoStore, nil, registry)

	srv, err := server.NewServer(testutil.GetFreePort(t))
	require.NoError(t, err)
	srv.Register("/api/v1/agent", handler)

	go srv.Start()
	err = srv.WaitHealthy(10, 100*time.Millisecond)
	require.NoError(t, err)

	// Pre-create a repo for use in tests.
	r, _ := repo.NewRepo("owner/test-repo")
	require.NoError(t, repoStore.CreateRepo(context.Background(), r))

	t.Cleanup(func() { srv.Stop(context.Background()) })

	return &fixture{
		Server:         srv,
		TaskStore:      taskStore,
		EpicStore:      epicStore,
		RepoStore:      repoStore,
		WorkerRegistry: registry,
		Repo:           r,
		t:              t,
		taskRepo:       taskRepo,
		epicRepo:       epicRepo,
	}
}

// --- URL helpers ---

func (f *fixture) taskLogsURL(id task.TaskID) string {
	return fmt.Sprintf("%s/api/v1/agent/tasks/%s/logs", f.Server.Address(), id)
}

func (f *fixture) taskHeartbeatURL(id task.TaskID) string {
	return fmt.Sprintf("%s/api/v1/agent/tasks/%s/heartbeat", f.Server.Address(), id)
}

func (f *fixture) taskCompleteURL(id task.TaskID) string {
	return fmt.Sprintf("%s/api/v1/agent/tasks/%s/complete", f.Server.Address(), id)
}

func (f *fixture) epicProposeURL(id epic.EpicID) string {
	return fmt.Sprintf("%s/api/v1/agent/epics/%s/propose", f.Server.Address(), id)
}

func (f *fixture) epicHeartbeatURL(id epic.EpicID) string {
	return fmt.Sprintf("%s/api/v1/agent/epics/%s/heartbeat", f.Server.Address(), id)
}

func (f *fixture) epicLogsURL(id epic.EpicID) string {
	return fmt.Sprintf("%s/api/v1/agent/epics/%s/logs", f.Server.Address(), id)
}

func (f *fixture) workersURL() string {
	return fmt.Sprintf("%s/api/v1/agent/workers", f.Server.Address())
}

// --- Seed helpers ---

func (f *fixture) seedRunningTask() *task.Task {
	f.t.Helper()
	ctx := context.Background()
	tsk := task.NewTask(f.Repo.ID.String(), "Test Task", "description", nil, nil, 0, false, "sonnet", true)
	require.NoError(f.t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(f.t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	return tsk
}

func (f *fixture) seedPlanningEpic() *epic.Epic {
	f.t.Helper()
	ctx := context.Background()
	e := epic.NewEpic(f.Repo.ID.String(), "Test Epic", "description")
	require.NoError(f.t, f.EpicStore.CreateEpic(ctx, e))
	return e
}

// --- HTTP helpers ---

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
