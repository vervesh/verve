package metricapi_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/joshjon/kit/server"
	"github.com/joshjon/kit/testutil"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/metricapi"
	"github.com/joshjon/verve/internal/repo"
	"github.com/joshjon/verve/internal/sqlite"
	"github.com/joshjon/verve/internal/task"
)

type fixture struct {
	Server   *server.Server
	TaskRepo task.Repository
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

	handler := metricapi.NewHTTPHandler(taskStore, nil, nil)

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
		Repo:     r,
		t:        t,
	}
}

func (f *fixture) metricsURL() string {
	return fmt.Sprintf("%s/api/v1/metrics", f.Server.Address())
}

func (f *fixture) seedTask(title string, status task.Status) *task.Task {
	f.t.Helper()
	ctx := context.Background()
	tsk := task.NewTask(f.Repo.ID.String(), title, "desc", nil, nil, 0, false, "sonnet", true)
	require.NoError(f.t, f.TaskRepo.CreateTask(ctx, tsk))
	if status != task.StatusPending {
		require.NoError(f.t, f.TaskRepo.UpdateTaskStatus(ctx, tsk.ID, status))
	}
	return tsk
}
