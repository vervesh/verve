package conversationapi_test

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

	"github.com/joshjon/verve/internal/conversation"
	"github.com/joshjon/verve/internal/conversationapi"
	"github.com/joshjon/verve/internal/epic"
	"github.com/joshjon/verve/internal/repo"
	"github.com/joshjon/verve/internal/sqlite"
	"github.com/joshjon/verve/internal/task"
)

type fixture struct {
	Server            *server.Server
	ConversationStore *conversation.Store
	EpicStore         *epic.Store
	RepoStore         *repo.Store
	Repo              *repo.Repo
	t                 *testing.T

	convRepo conversation.Repository
	epicRepo epic.Repository
}

// stubTaskCreator implements epic.TaskCreator for tests.
type stubTaskCreator struct {
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
	logger := log.NewLogger(log.WithNop())

	taskRepo := sqlite.NewTaskRepository(db)
	broker := task.NewBroker(nil)
	taskStore := task.NewStore(taskRepo, broker)

	repoRepo := sqlite.NewRepoRepository(db)
	repoStore := repo.NewStore(repoRepo)

	epicRepo := sqlite.NewEpicRepository(db)
	tc := &stubTaskCreator{taskStore: taskStore}
	epicStore := epic.NewStore(epicRepo, tc, logger)

	convRepo := sqlite.NewConversationRepository(db)
	convStore := conversation.NewStore(convRepo, logger)

	handler := conversationapi.NewHTTPHandler(convStore, repoStore, epicStore, nil)

	srv, err := server.NewServer(testutil.GetFreePort(t))
	require.NoError(t, err)
	srv.Register("/api/v1", handler)

	go srv.Start()
	err = srv.WaitHealthy(10, 100*time.Millisecond)
	require.NoError(t, err)

	// Pre-create a repo for use in tests and mark it ready.
	r, _ := repo.NewRepo("owner/test-repo")
	ctx := context.Background()
	require.NoError(t, repoStore.CreateRepo(ctx, r))
	require.NoError(t, repoStore.UpdateRepoSetupStatus(ctx, r.ID, repo.SetupStatusReady))

	t.Cleanup(func() { srv.Stop(context.Background()) })

	return &fixture{
		Server:            srv,
		ConversationStore: convStore,
		EpicStore:         epicStore,
		RepoStore:         repoStore,
		Repo:              r,
		t:                 t,
		convRepo:          convRepo,
		epicRepo:          epicRepo,
	}
}

// --- URL helpers ---

func (f *fixture) repoConversationsURL() string {
	return fmt.Sprintf("%s/api/v1/repos/%s/conversations", f.Server.Address(), f.Repo.ID)
}

func (f *fixture) conversationURL(id conversation.ConversationID) string {
	return fmt.Sprintf("%s/api/v1/conversations/%s", f.Server.Address(), id)
}

func (f *fixture) conversationActionURL(id conversation.ConversationID, action string) string {
	return fmt.Sprintf("%s/%s", f.conversationURL(id), action)
}

// --- Seed helpers ---

func (f *fixture) seedConversation(title string) *conversation.Conversation {
	f.t.Helper()
	conv := conversation.NewConversation(f.Repo.ID.String(), title, "sonnet")
	require.NoError(f.t, f.ConversationStore.CreateConversation(context.Background(), conv))
	return conv
}

func (f *fixture) seedConversationWithMessages(title string) *conversation.Conversation {
	f.t.Helper()
	ctx := context.Background()
	conv := f.seedConversation(title)
	require.NoError(f.t, f.ConversationStore.SendMessage(ctx, conv.ID, "Hello, what should we build?"))
	require.NoError(f.t, f.ConversationStore.CompleteResponse(ctx, conv.ID, "I suggest we build a REST API."))
	require.NoError(f.t, f.ConversationStore.SendMessage(ctx, conv.ID, "Great idea, let's add auth too."))
	require.NoError(f.t, f.ConversationStore.CompleteResponse(ctx, conv.ID, "We can use JWT-based authentication."))
	// Re-read to get updated state.
	updated, err := f.ConversationStore.ReadConversation(ctx, conv.ID)
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
