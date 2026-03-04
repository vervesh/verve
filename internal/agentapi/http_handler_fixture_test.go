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
	"github.com/joshjon/verve/internal/conversation"
	"github.com/joshjon/verve/internal/epic"
	"github.com/joshjon/verve/internal/repo"
	"github.com/joshjon/verve/internal/sqlite"
	"github.com/joshjon/verve/internal/task"
	"github.com/joshjon/verve/internal/workertracker"
)

type fixture struct {
	Server            *server.Server
	TaskStore         *task.Store
	EpicStore         *epic.Store
	RepoStore         *repo.Store
	ConversationStore *conversation.Store
	WorkerRegistry    *workertracker.Registry
	Repo              *repo.Repo
	t                 *testing.T

	taskRepo task.Repository
	epicRepo epic.Repository
	convRepo conversation.Repository
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
	epicStore := epic.NewStore(epicRepo, nil, logger)

	registry := workertracker.New()

	convRepo := sqlite.NewConversationRepository(db)
	convStore := conversation.NewStore(convRepo, logger)

	handler := agentapi.NewHTTPHandler(taskStore, epicStore, repoStore, convStore, nil, registry)

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
		Server:            srv,
		TaskStore:         taskStore,
		EpicStore:         epicStore,
		RepoStore:         repoStore,
		ConversationStore: convStore,
		WorkerRegistry:    registry,
		Repo:              r,
		t:                 t,
		taskRepo:          taskRepo,
		epicRepo:          epicRepo,
		convRepo:          convRepo,
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

func (f *fixture) epicCompleteURL(id epic.EpicID) string {
	return fmt.Sprintf("%s/api/v1/agent/epics/%s/complete", f.Server.Address(), id)
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

func (f *fixture) repoSetupCompleteURL(repoID repo.RepoID) string {
	return fmt.Sprintf("%s/api/v1/agent/repos/%s/setup-complete", f.Server.Address(), repoID)
}

func (f *fixture) conversationCompleteURL(id conversation.ConversationID) string {
	return fmt.Sprintf("%s/api/v1/agent/conversations/%s/complete", f.Server.Address(), id)
}

func (f *fixture) conversationHeartbeatURL(id conversation.ConversationID) string {
	return fmt.Sprintf("%s/api/v1/agent/conversations/%s/heartbeat", f.Server.Address(), id)
}

func (f *fixture) conversationLogsURL(id conversation.ConversationID) string {
	return fmt.Sprintf("%s/api/v1/agent/conversations/%s/logs", f.Server.Address(), id)
}

func (f *fixture) pollURL() string {
	return fmt.Sprintf("%s/api/v1/agent/poll", f.Server.Address())
}

// --- Seed helpers ---

func (f *fixture) seedRunningTask() *task.Task {
	f.t.Helper()
	ctx := context.Background()
	tsk := task.NewTask(f.Repo.ID.String(), "Test Task", "description", nil, nil, 0, false, false, "sonnet", true)
	require.NoError(f.t, f.taskRepo.CreateTask(ctx, tsk))
	require.NoError(f.t, f.taskRepo.UpdateTaskStatus(ctx, tsk.ID, task.StatusRunning))
	return tsk
}

func (f *fixture) seedPendingConversation() *conversation.Conversation {
	f.t.Helper()
	ctx := context.Background()
	conv := conversation.NewConversation(f.Repo.ID.String(), "Test Conv", "sonnet")
	require.NoError(f.t, f.ConversationStore.CreateConversation(ctx, conv))
	require.NoError(f.t, f.ConversationStore.SendMessage(ctx, conv.ID, "Hello"))
	updated, err := f.ConversationStore.ReadConversation(ctx, conv.ID)
	require.NoError(f.t, err)
	return updated
}

func (f *fixture) seedClaimedConversation() *conversation.Conversation {
	f.t.Helper()
	ctx := context.Background()
	conv := f.seedPendingConversation()
	claimed, err := f.ConversationStore.ClaimPendingConversation(ctx)
	require.NoError(f.t, err)
	require.NotNil(f.t, claimed)
	require.Equal(f.t, conv.ID.String(), claimed.ID.String())
	return claimed
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
