package agentapi_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/joshjon/kit/server"
	"github.com/joshjon/kit/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/agentapi"
	"github.com/joshjon/verve/internal/epic"
	"github.com/joshjon/verve/internal/task"
	"github.com/joshjon/verve/internal/workertracker"
)

// --- Task Agent Endpoints ---

func TestTaskAppendLogs(t *testing.T) {
	f := newFixture(t)
	tsk := f.seedRunningTask()

	req := agentapi.TaskLogsRequest{
		Logs:    []string{"line 1", "line 2"},
		Attempt: 1,
	}
	postNoContent(t, f.taskLogsURL(tsk.ID), req)

	// Verify logs were stored
	stored, err := f.taskRepo.ReadTaskLogs(context.Background(), tsk.ID)
	assert.NoError(t, err)
	assert.Contains(t, stored, "line 1")
	assert.Contains(t, stored, "line 2")
}

func TestTaskAppendLogs_InvalidID(t *testing.T) {
	f := newFixture(t)

	url := f.Server.Address() + "/api/v1/agent/tasks/bad-id/logs"
	req := agentapi.TaskLogsRequest{Logs: []string{"line"}}
	httpRes := doJSON(t, http.MethodPost, url, req)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode)
}

func TestTaskHeartbeat(t *testing.T) {
	f := newFixture(t)
	tsk := f.seedRunningTask()

	res := testutil.Post[server.Response[map[string]interface{}]](t, f.taskHeartbeatURL(tsk.ID), nil)
	assert.Equal(t, "ok", res.Data["status"])
	assert.Equal(t, false, res.Data["stopped"])
}

func TestTaskComplete_Success(t *testing.T) {
	f := newFixture(t)
	tsk := f.seedRunningTask()

	req := agentapi.TaskCompleteRequest{
		Success:        true,
		PullRequestURL: "https://github.com/owner/repo/pull/42",
		PRNumber:       42,
	}
	postNoContent(t, f.taskCompleteURL(tsk.ID), req)

	// Verify task transitioned to review
	stored, err := f.taskRepo.ReadTask(context.Background(), tsk.ID)
	assert.NoError(t, err)
	assert.Equal(t, task.StatusReview, stored.Status)
	assert.Equal(t, "https://github.com/owner/repo/pull/42", stored.PullRequestURL)
}

func TestTaskComplete_Failure(t *testing.T) {
	f := newFixture(t)
	tsk := f.seedRunningTask()

	req := agentapi.TaskCompleteRequest{
		Success: false,
		Error:   "something went wrong",
	}
	postNoContent(t, f.taskCompleteURL(tsk.ID), req)

	stored, err := f.taskRepo.ReadTask(context.Background(), tsk.ID)
	assert.NoError(t, err)
	assert.Equal(t, task.StatusFailed, stored.Status)
}

func TestTaskComplete_MergesFilesModifiedAcrossRetries(t *testing.T) {
	f := newFixture(t)
	tsk := f.seedRunningTask()

	// Simulate a previous attempt having set agent_status with files_modified
	err := f.taskRepo.SetAgentStatus(context.Background(), tsk.ID,
		`{"files_modified":["main.go","config.go"],"tests_status":"fail","confidence":"medium"}`)
	assert.NoError(t, err)

	// Complete task with new agent status (retry attempt only reports its own files)
	req := agentapi.TaskCompleteRequest{
		Success:        true,
		PullRequestURL: "https://github.com/owner/repo/pull/42",
		PRNumber:       42,
		AgentStatus:    `{"files_modified":["main.go","handler.go"],"tests_status":"pass","confidence":"high"}`,
	}
	postNoContent(t, f.taskCompleteURL(tsk.ID), req)

	// Verify merged agent_status includes files from both attempts
	stored, err := f.taskRepo.ReadTask(context.Background(), tsk.ID)
	assert.NoError(t, err)
	assert.Contains(t, stored.AgentStatus, `main.go`)
	assert.Contains(t, stored.AgentStatus, `handler.go`)
	assert.Contains(t, stored.AgentStatus, `config.go`)
	// New status fields should take precedence
	assert.Contains(t, stored.AgentStatus, `pass`)
	assert.Contains(t, stored.AgentStatus, `high`)
}

// --- Epic Agent Endpoints ---

func TestEpicComplete_Success(t *testing.T) {
	f := newFixture(t)
	e := f.seedPlanningEpic()

	req := agentapi.EpicCompleteRequest{
		Success: true,
		Tasks: []epic.ProposedTask{
			{TempID: "t1", Title: "Task 1", Description: "desc 1"},
		},
	}
	postNoContent(t, f.epicCompleteURL(e.ID), req)

	// Verify epic transitioned to draft with proposed tasks
	stored, err := f.EpicStore.ReadEpic(context.Background(), e.ID)
	assert.NoError(t, err)
	assert.Len(t, stored.ProposedTasks, 1)
	assert.Equal(t, epic.StatusDraft, stored.Status)
	assert.Nil(t, stored.ClaimedAt, "claim should be released")
}

func TestEpicComplete_Failure(t *testing.T) {
	f := newFixture(t)
	e := f.seedPlanningEpic()

	req := agentapi.EpicCompleteRequest{
		Success: false,
		Error:   "planning failed",
	}
	postNoContent(t, f.epicCompleteURL(e.ID), req)

	// With no previous proposals, should stay in planning for retry
	stored, err := f.EpicStore.ReadEpic(context.Background(), e.ID)
	assert.NoError(t, err)
	assert.Equal(t, epic.StatusPlanning, stored.Status)
	assert.Nil(t, stored.ClaimedAt, "claim should be released")
}

func TestEpicHeartbeat(t *testing.T) {
	f := newFixture(t)
	e := f.seedPlanningEpic()

	postNoContent(t, f.epicHeartbeatURL(e.ID), nil)

	// Verify heartbeat updated
	stored, err := f.EpicStore.ReadEpic(context.Background(), e.ID)
	assert.NoError(t, err)
	assert.NotNil(t, stored.LastHeartbeatAt)
}

func TestEpicAppendLogs(t *testing.T) {
	f := newFixture(t)
	e := f.seedPlanningEpic()

	req := agentapi.SessionLogRequest{
		Lines: []string{"agent: analyzing repo", "agent: proposing tasks"},
	}
	postNoContent(t, f.epicLogsURL(e.ID), req)

	stored, err := f.EpicStore.ReadEpic(context.Background(), e.ID)
	assert.NoError(t, err)
	assert.Contains(t, stored.SessionLog, "agent: analyzing repo")
}

// --- Repo Setup Agent Endpoint ---

func TestRepoSetupComplete_Success(t *testing.T) {
	f := newFixture(t)

	// Set repo to scanning status first (pending → scanning)
	ctx := context.Background()
	require.NoError(t, f.RepoStore.UpdateRepoSetupStatus(ctx, f.Repo.ID, "scanning"))

	req := agentapi.RepoSetupCompleteRequest{
		Success:     true,
		Summary:     "Go web application using Echo",
		TechStack:   []string{"Go", "Echo v4", "PostgreSQL"},
		HasCode:     true,
		HasClaudeMD: true,
		HasREADME:   true,
		NeedsSetup:  false,
	}
	postNoContent(t, f.repoSetupCompleteURL(f.Repo.ID), req)

	// Verify scan results were stored
	r, err := f.RepoStore.ReadRepo(ctx, f.Repo.ID)
	require.NoError(t, err)
	assert.Equal(t, "Go web application using Echo", r.Summary)
	assert.Equal(t, []string{"Go", "Echo v4", "PostgreSQL"}, r.TechStack)
	assert.True(t, r.HasCode)
	assert.True(t, r.HasCLAUDEMD)
	assert.True(t, r.HasREADME)
	assert.Equal(t, "ready", r.SetupStatus, "should be ready when needs_setup=false")
}

func TestRepoSetupComplete_NeedsSetup(t *testing.T) {
	f := newFixture(t)

	ctx := context.Background()
	require.NoError(t, f.RepoStore.UpdateRepoSetupStatus(ctx, f.Repo.ID, "scanning"))

	req := agentapi.RepoSetupCompleteRequest{
		Success:     true,
		Summary:     "Empty repo",
		TechStack:   []string{},
		HasCode:     false,
		HasClaudeMD: false,
		HasREADME:   false,
		NeedsSetup:  true,
	}
	postNoContent(t, f.repoSetupCompleteURL(f.Repo.ID), req)

	r, err := f.RepoStore.ReadRepo(ctx, f.Repo.ID)
	require.NoError(t, err)
	assert.Equal(t, "needs_setup", r.SetupStatus, "should be needs_setup when needs_setup=true")
}

func TestRepoSetupComplete_Failure(t *testing.T) {
	f := newFixture(t)

	ctx := context.Background()
	require.NoError(t, f.RepoStore.UpdateRepoSetupStatus(ctx, f.Repo.ID, "scanning"))

	req := agentapi.RepoSetupCompleteRequest{
		Success: false,
	}
	postNoContent(t, f.repoSetupCompleteURL(f.Repo.ID), req)

	// Should remain in scanning status
	r, err := f.RepoStore.ReadRepo(ctx, f.Repo.ID)
	require.NoError(t, err)
	assert.Equal(t, "scanning", r.SetupStatus, "should remain scanning on failure")
}

func TestRepoSetupComplete_InvalidID(t *testing.T) {
	f := newFixture(t)

	url := f.Server.Address() + "/api/v1/agent/repos/bad-id/setup-complete"
	req := agentapi.RepoSetupCompleteRequest{Success: true}
	httpRes := doJSON(t, http.MethodPost, url, req)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode)
}

// --- Worker Observability ---

func TestListWorkers_Empty(t *testing.T) {
	f := newFixture(t)

	res := testutil.Get[server.ResponseList[workertracker.WorkerInfo]](t, f.workersURL())
	assert.Empty(t, res.Data)
}

func TestListWorkers_WithWorkers(t *testing.T) {
	f := newFixture(t)

	// Register a worker via the registry
	f.WorkerRegistry.RecordPollStart("worker-1", 4, 1)

	res := testutil.Get[server.ResponseList[workertracker.WorkerInfo]](t, f.workersURL())
	assert.Len(t, res.Data, 1)
	assert.Equal(t, "worker-1", res.Data[0].WorkerID)
	assert.Equal(t, 4, res.Data[0].MaxConcurrentTasks)
}

// --- Conversation Agent Endpoints ---

func TestConversationComplete_Success(t *testing.T) {
	f := newFixture(t)
	conv := f.seedClaimedConversation()

	req := agentapi.ConversationCompleteRequest{
		Success:  true,
		Response: "Here is my analysis of the codebase.",
	}
	postNoContent(t, f.conversationCompleteURL(conv.ID), req)

	// Verify assistant message was appended and claim was released
	stored, err := f.ConversationStore.ReadConversation(context.Background(), conv.ID)
	assert.NoError(t, err)
	// Should have user message + assistant response
	assert.Len(t, stored.Messages, 2)
	assert.Equal(t, "assistant", stored.Messages[1].Role)
	assert.Equal(t, "Here is my analysis of the codebase.", stored.Messages[1].Content)
	assert.Nil(t, stored.PendingMessage, "pending message should be cleared")
	assert.Nil(t, stored.ClaimedAt, "claim should be released")
}

func TestConversationComplete_Failure(t *testing.T) {
	f := newFixture(t)
	conv := f.seedClaimedConversation()

	req := agentapi.ConversationCompleteRequest{
		Success: false,
		Error:   "something went wrong",
	}
	postNoContent(t, f.conversationCompleteURL(conv.ID), req)

	// Verify claim released and pending cleared, no assistant message added
	stored, err := f.ConversationStore.ReadConversation(context.Background(), conv.ID)
	assert.NoError(t, err)
	assert.Len(t, stored.Messages, 1) // only the user message
	assert.Nil(t, stored.PendingMessage, "pending message should be cleared")
	assert.Nil(t, stored.ClaimedAt, "claim should be released")
}

func TestConversationComplete_InvalidID(t *testing.T) {
	f := newFixture(t)

	url := f.Server.Address() + "/api/v1/agent/conversations/bad-id/complete"
	req := agentapi.ConversationCompleteRequest{Success: true, Response: "test"}
	httpRes := doJSON(t, http.MethodPost, url, req)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode)
}

func TestConversationHeartbeat(t *testing.T) {
	f := newFixture(t)
	conv := f.seedClaimedConversation()

	postNoContent(t, f.conversationHeartbeatURL(conv.ID), nil)

	// Verify heartbeat updated
	stored, err := f.ConversationStore.ReadConversation(context.Background(), conv.ID)
	assert.NoError(t, err)
	assert.NotNil(t, stored.LastHeartbeatAt)
}

func TestConversationAppendLogs(t *testing.T) {
	f := newFixture(t)
	conv := f.seedClaimedConversation()

	req := agentapi.ConversationLogsRequest{
		Lines: []string{"agent: processing message", "agent: generating response"},
	}
	postNoContent(t, f.conversationLogsURL(conv.ID), req)
}

func TestPoll_ReturnsConversation(t *testing.T) {
	f := newFixture(t)
	// Need to mark the repo as ready for setup tasks
	require.NoError(t, f.RepoStore.UpdateRepoSetupStatus(context.Background(), f.Repo.ID, "ready"))

	conv := f.seedPendingConversation()

	// Poll should return the conversation
	res := testutil.Get[server.Response[agentapi.PollResponse]](t, f.pollURL())
	assert.Equal(t, "conversation", res.Data.Type)
	assert.NotNil(t, res.Data.Conversation)
	assert.Equal(t, conv.ID.String(), res.Data.Conversation.ID.String())
	assert.Equal(t, "owner/test-repo", res.Data.RepoFullName)

	// After claiming, verify conversation is claimed
	stored, err := f.ConversationStore.ReadConversation(context.Background(), conv.ID)
	assert.NoError(t, err)
	assert.NotNil(t, stored.ClaimedAt, "conversation should be claimed after poll")
}
