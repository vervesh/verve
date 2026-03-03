package repoapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/joshjon/kit/server"
	"github.com/joshjon/kit/testutil"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/repo"
	"github.com/joshjon/verve/internal/repoapi"
	"github.com/joshjon/verve/internal/sqlite"
	"github.com/joshjon/verve/internal/task"
)

type fixture struct {
	Server    *server.Server
	RepoStore *repo.Store
	TaskStore *task.Store
	t         *testing.T
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	db := sqlite.NewTestDB(t)
	repoRepo := sqlite.NewRepoRepository(db)
	repoStore := repo.NewStore(repoRepo)

	broker := task.NewBroker(nil)
	taskRepo := sqlite.NewTaskRepository(db)
	taskStore := task.NewStore(taskRepo, broker)

	handler := repoapi.NewHTTPHandler(repoStore, taskStore, nil)

	srv, err := server.NewServer(testutil.GetFreePort(t))
	require.NoError(t, err)
	srv.Register("/api/v1", handler)

	go srv.Start()
	err = srv.WaitHealthy(10, 100*time.Millisecond)
	require.NoError(t, err)

	t.Cleanup(func() { srv.Stop(context.Background()) })

	return &fixture{
		Server:    srv,
		RepoStore: repoStore,
		TaskStore: taskStore,
		t:         t,
	}
}

func (f *fixture) addRepo(fullName string) *repo.Repo {
	f.t.Helper()
	r, err := repo.NewRepo(fullName)
	require.NoError(f.t, err)
	require.NoError(f.t, f.RepoStore.CreateRepo(context.Background(), r))
	return r
}

func (f *fixture) reposURL() string {
	return fmt.Sprintf("%s/api/v1/repos", f.Server.Address())
}

func (f *fixture) repoURL(id repo.RepoID) string {
	return fmt.Sprintf("%s/%s", f.reposURL(), id)
}

func (f *fixture) repoSetupURL(id repo.RepoID) string {
	return fmt.Sprintf("%s/api/v1/repos/%s/setup", f.Server.Address(), id)
}

func (f *fixture) repoExpectationsURL(id repo.RepoID) string {
	return fmt.Sprintf("%s/api/v1/repos/%s/setup/expectations", f.Server.Address(), id)
}

func (f *fixture) repoRescanURL(id repo.RepoID) string {
	return fmt.Sprintf("%s/api/v1/repos/%s/setup/rescan", f.Server.Address(), id)
}

func (f *fixture) availableReposURL() string {
	return fmt.Sprintf("%s/api/v1/repos/available", f.Server.Address())
}

func mustJSONReader(v any) io.Reader {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return bytes.NewReader(b)
}
