package agentapi_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/joshjon/kit/server"
	"github.com/joshjon/kit/testutil"
	"github.com/stretchr/testify/assert"

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
