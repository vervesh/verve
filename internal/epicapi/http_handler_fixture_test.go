package epicapi_test

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

	"github.com/vervesh/verve/internal/epic"
	"github.com/vervesh/verve/internal/epicapi"
	"github.com/vervesh/verve/internal/repo"
	"github.com/vervesh/verve/internal/sqlite"
	"github.com/vervesh/verve/internal/task"
)

type fixture struct {
	Server    *server.Server
	EpicStore *epic.Store
	TaskStore *task.Store
	RepoStore *repo.Store
	Repo      *repo.Repo
	t         *testing.T

	epicRepo epic.Repository
	taskRepo task.Repository
}

// stubTaskCreator implements epic.TaskCreator for tests — creates real tasks via taskStore.
type stubTaskCreator struct {
	taskRepo  task.Repository
	taskStore *task.Store
}

func (s *stubTaskCreator) CreateTaskFromEpic(ctx context.Context, repoID, title, description string, dependsOn, acceptanceCriteria []string, epicID string, ready bool, model string) (string, error) {
	tsk := task.NewTask(repoID, title, description, dependsOn, acceptanceCriteria, 0, false, false, model, ready)
	tsk.EpicID = epicID
	if err := s.taskStore.CreateTask(ctx, tsk); err != nil {
		return "", err
	}
	return tsk.ID.String(), nil
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	db := sqlite.NewTestDB(t)
	taskRepo := sqlite.NewTaskRepository(db)
	broker := task.NewBroker(nil)
	taskStore := task.NewStore(taskRepo, broker)

	repoRepo := sqlite.NewRepoRepository(db)
	repoStore := repo.NewStore(repoRepo)

	epicRepo := sqlite.NewEpicRepository(db)
	logger := log.NewLogger(log.WithNop())
	tc := &stubTaskCreator{taskRepo: taskRepo, taskStore: taskStore}
	epicStore := epic.NewStore(epicRepo, tc, logger)

	handler := epicapi.NewHTTPHandler(epicStore, repoStore, taskStore, nil)

	srv, err := server.NewServer(testutil.GetFreePort(t))
	require.NoError(t, err)
	srv.Register("/api/v1", handler)

	go srv.Start()
	err = srv.WaitHealthy(10, 100*time.Millisecond)
	require.NoError(t, err)

	// Pre-create a repo for use in tests and mark it ready so epics can be created.
	r, _ := repo.NewRepo("owner/test-repo")
	ctx := context.Background()
	require.NoError(t, repoStore.CreateRepo(ctx, r))
	require.NoError(t, repoStore.UpdateRepoSetupStatus(ctx, r.ID, repo.SetupStatusReady))

	t.Cleanup(func() { srv.Stop(context.Background()) })

	return &fixture{
		Server:    srv,
		EpicStore: epicStore,
		TaskStore: taskStore,
		RepoStore: repoStore,
		Repo:      r,
		t:         t,
		epicRepo:  epicRepo,
		taskRepo:  taskRepo,
	}
}

// --- URL helpers ---

func (f *fixture) repoEpicsURL() string {
	return fmt.Sprintf("%s/api/v1/repos/%s/epics", f.Server.Address(), f.Repo.ID)
}

func (f *fixture) epicURL(id epic.EpicID) string {
	return fmt.Sprintf("%s/api/v1/epics/%s", f.Server.Address(), id)
}

func (f *fixture) epicActionURL(id epic.EpicID, action string) string {
	return fmt.Sprintf("%s/%s", f.epicURL(id), action)
}

func (f *fixture) epicByNumberURL(number int) string {
	return fmt.Sprintf("%s/api/v1/repos/%s/epics/%d", f.Server.Address(), f.Repo.ID, number)
}

// --- Seed helpers ---

func (f *fixture) seedEpic(title, desc string) *epic.Epic {
	f.t.Helper()
	e := epic.NewEpic(f.Repo.ID.String(), title, desc)
	require.NoError(f.t, f.EpicStore.CreateEpic(context.Background(), e))
	return e
}

func (f *fixture) seedClaimedPlanningEpic(title, desc string) *epic.Epic {
	f.t.Helper()
	ctx := context.Background()
	e := f.seedEpic(title, desc)
	// Claim the epic so it's in planning+claimed state
	claimed, err := f.epicRepo.ClaimEpic(ctx, e.ID)
	require.NoError(f.t, err)
	require.True(f.t, claimed)
	updated, err := f.EpicStore.ReadEpic(ctx, e.ID)
	require.NoError(f.t, err)
	return updated
}

func (f *fixture) seedDraftEpic(title, desc string) *epic.Epic {
	f.t.Helper()
	ctx := context.Background()
	e := f.seedEpic(title, desc)
	// Use CompletePlanning to transition to draft with proposed tasks
	tasks := []epic.ProposedTask{
		{TempID: "t1", Title: "Sub-task 1", Description: "desc"},
	}
	require.NoError(f.t, f.EpicStore.CompletePlanning(ctx, e.ID, tasks))
	// Re-read to get updated state
	updated, err := f.EpicStore.ReadEpic(ctx, e.ID)
	require.NoError(f.t, err)
	return updated
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
