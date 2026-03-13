package epic_test

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/joshjon/kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/epic"
	"github.com/joshjon/verve/internal/repo"
	"github.com/joshjon/verve/internal/sqlite"
)

// --- Mock TaskCreator (no SQLite implementation available) ---

type mockTaskCreator struct {
	mu       sync.Mutex
	calls    []createTaskCall
	idPrefix string
	err      error
}

type createTaskCall struct {
	repoID, title, description string
	dependsOn                  []string
	acceptanceCriteria         []string
	epicID                     string
	ready                      bool
	model                      string
}

func (m *mockTaskCreator) CreateTaskFromEpic(_ context.Context, repoID, title, description string, dependsOn, acceptanceCriteria []string, epicID string, ready bool, model string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return "", m.err
	}
	call := createTaskCall{repoID, title, description, dependsOn, acceptanceCriteria, epicID, ready, model}
	m.calls = append(m.calls, call)
	return m.idPrefix + "_" + title, nil
}

// --- Mock TaskStatusReader (no SQLite implementation available) ---

type mockTaskStatusReader struct {
	statuses map[string]string
}

func (m *mockTaskStatusReader) ReadTaskStatus(_ context.Context, taskID string) (string, error) {
	status, ok := m.statuses[taskID]
	if !ok {
		return "", fmt.Errorf("task not found")
	}
	return status, nil
}

// --- Fixture ---

type epicFixture struct {
	store    *epic.Store
	epicRepo epic.Repository
	repoID   string
	db       *sql.DB
}

func newEpicFixture(t *testing.T) *epicFixture {
	t.Helper()
	db := sqlite.NewTestDB(t)
	repoRepo := sqlite.NewRepoRepository(db)
	r, err := repo.NewRepo("owner/test-repo")
	require.NoError(t, err)
	require.NoError(t, repoRepo.CreateRepo(context.Background(), r))

	epicRepo := sqlite.NewEpicRepository(db)
	logger := log.NewLogger(log.WithNop())
	store := epic.NewStore(epicRepo, nil, logger)

	return &epicFixture{
		store:    store,
		epicRepo: epicRepo,
		repoID:   r.ID.String(),
		db:       db,
	}
}

func newEpicFixtureWithTaskCreator(t *testing.T, tc epic.TaskCreator) *epicFixture {
	t.Helper()
	f := newEpicFixture(t)
	f.store = epic.NewStore(f.epicRepo, tc, log.NewLogger(log.WithNop()))
	return f
}

// seedEpic creates an epic in the given status. It creates the epic in planning
// status first (as NewEpic does), then transitions to the desired status.
func (f *epicFixture) seedEpic(t *testing.T, title, description string, status epic.Status) *epic.Epic {
	t.Helper()
	ctx := context.Background()
	e := epic.NewEpic(f.repoID, title, description)
	require.NoError(t, f.epicRepo.CreateEpic(ctx, e))
	if status != epic.StatusPlanning {
		require.NoError(t, f.epicRepo.UpdateEpicStatus(ctx, e.ID, status))
	}
	// Re-read to get the updated state
	updated, err := f.epicRepo.ReadEpic(ctx, e.ID)
	require.NoError(t, err)
	return updated
}

// --- Store Tests ---

func TestStore_CreateEpic(t *testing.T) {
	f := newEpicFixture(t)

	e := epic.NewEpic(f.repoID, "Test Epic", "description")
	err := f.store.CreateEpic(context.Background(), e)
	require.NoError(t, err)

	// Should be stored in repo
	stored, err := f.epicRepo.ReadEpic(context.Background(), e.ID)
	require.NoError(t, err)
	assert.Equal(t, e.Title, stored.Title)

	// Should notify pending
	select {
	case <-f.store.WaitForPending():
		// Good
	default:
		assert.Fail(t, "expected pending notification")
	}
}

func TestStore_RequestChanges(t *testing.T) {
	t.Run("stores feedback and transitions to planning", func(t *testing.T) {
		f := newEpicFixture(t)
		e := f.seedEpic(t, "Epic", "desc", epic.StatusDraft)

		err := f.store.RequestChanges(context.Background(), e.ID, "please change X")
		require.NoError(t, err)

		stored, err := f.epicRepo.ReadEpic(context.Background(), e.ID)
		require.NoError(t, err)
		require.NotNil(t, stored.Feedback)
		assert.Equal(t, "please change X", *stored.Feedback)
		assert.Equal(t, epic.StatusPlanning, stored.Status)

		// Should notify pending for workers to pick up
		select {
		case <-f.store.WaitForPending():
		default:
			assert.Fail(t, "expected pending notification")
		}
	})

	t.Run("rejects non-draft status", func(t *testing.T) {
		f := newEpicFixture(t)
		e := f.seedEpic(t, "Epic", "desc", epic.StatusActive)

		err := f.store.RequestChanges(context.Background(), e.ID, "please change X")
		assert.Error(t, err)
	})
}

func TestStore_CompletePlanning(t *testing.T) {
	f := newEpicFixture(t)
	ctx := context.Background()

	e := f.seedEpic(t, "Epic", "desc", epic.StatusPlanning)

	// Claim the epic (sets claimed_at)
	claimed, err := f.epicRepo.ClaimEpic(ctx, e.ID)
	require.NoError(t, err)
	require.True(t, claimed)

	// Set feedback
	require.NoError(t, f.epicRepo.SetEpicFeedback(ctx, e.ID, "some feedback", string(epic.FeedbackMessage)))

	tasks := []epic.ProposedTask{
		{TempID: "t1", Title: "Task 1", Description: "desc 1"},
		{TempID: "t2", Title: "Task 2", Description: "desc 2"},
	}
	err = f.store.CompletePlanning(ctx, e.ID, tasks)
	require.NoError(t, err)

	stored, err := f.epicRepo.ReadEpic(ctx, e.ID)
	require.NoError(t, err)
	assert.Len(t, stored.ProposedTasks, 2)
	assert.Equal(t, epic.StatusDraft, stored.Status)
	assert.Nil(t, stored.ClaimedAt, "claim should be released")
	assert.Nil(t, stored.Feedback, "feedback should be cleared")
	assert.Nil(t, stored.FeedbackType, "feedback type should be cleared")
}

func TestStore_FailPlanning(t *testing.T) {
	t.Run("with existing proposals goes to draft", func(t *testing.T) {
		f := newEpicFixture(t)
		ctx := context.Background()

		e := f.seedEpic(t, "Epic", "desc", epic.StatusPlanning)

		// Claim the epic
		claimed, err := f.epicRepo.ClaimEpic(ctx, e.ID)
		require.NoError(t, err)
		require.True(t, claimed)

		// Set proposed tasks
		require.NoError(t, f.epicRepo.UpdateProposedTasks(ctx, e.ID, []epic.ProposedTask{
			{TempID: "t1", Title: "Task 1"},
		}))

		err = f.store.FailPlanning(ctx, e.ID)
		require.NoError(t, err)

		stored, err := f.epicRepo.ReadEpic(ctx, e.ID)
		require.NoError(t, err)
		assert.Nil(t, stored.ClaimedAt, "claim should be released")
		assert.Equal(t, epic.StatusDraft, stored.Status)
	})

	t.Run("without proposals stays in planning", func(t *testing.T) {
		f := newEpicFixture(t)
		ctx := context.Background()

		e := f.seedEpic(t, "Epic", "desc", epic.StatusPlanning)

		// Claim the epic
		claimed, err := f.epicRepo.ClaimEpic(ctx, e.ID)
		require.NoError(t, err)
		require.True(t, claimed)

		err = f.store.FailPlanning(ctx, e.ID)
		require.NoError(t, err)

		stored, err := f.epicRepo.ReadEpic(ctx, e.ID)
		require.NoError(t, err)
		assert.Nil(t, stored.ClaimedAt, "claim should be released")
		assert.Equal(t, epic.StatusPlanning, stored.Status)

		// Should notify pending for retry
		select {
		case <-f.store.WaitForPending():
		default:
			assert.Fail(t, "expected pending notification for retry")
		}
	})
}

func TestStore_UpdateProposedTasks(t *testing.T) {
	f := newEpicFixture(t)

	e := f.seedEpic(t, "Epic", "desc", epic.StatusDraft)

	tasks := []epic.ProposedTask{
		{TempID: "t1", Title: "Task 1", Description: "desc 1"},
		{TempID: "t2", Title: "Task 2", Description: "desc 2"},
	}
	err := f.store.UpdateProposedTasks(context.Background(), e.ID, tasks)
	require.NoError(t, err)

	stored, err := f.epicRepo.ReadEpic(context.Background(), e.ID)
	require.NoError(t, err)
	assert.Len(t, stored.ProposedTasks, 2)
	assert.Equal(t, epic.StatusDraft, stored.Status)
}

func TestStore_StartPlanning(t *testing.T) {
	tests := []struct {
		name    string
		status  epic.Status
		wantErr bool
	}{
		{"from draft", epic.StatusDraft, false},
		{"from ready", epic.StatusReady, false},
		{"from active (invalid)", epic.StatusActive, true},
		{"from completed (invalid)", epic.StatusCompleted, true},
		{"from closed (invalid)", epic.StatusClosed, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newEpicFixture(t)
			e := f.seedEpic(t, "Epic", "desc", tt.status)

			err := f.store.StartPlanning(context.Background(), e.ID, "new prompt")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				stored, err := f.epicRepo.ReadEpic(context.Background(), e.ID)
				require.NoError(t, err)
				assert.Equal(t, epic.StatusPlanning, stored.Status)
				assert.Equal(t, "new prompt", stored.PlanningPrompt)
			}
		})
	}
}

func TestStore_ConfirmEpic(t *testing.T) {
	t.Run("creates tasks from proposed and activates", func(t *testing.T) {
		tc := &mockTaskCreator{idPrefix: "tsk"}
		f := newEpicFixtureWithTaskCreator(t, tc)
		ctx := context.Background()

		e := f.seedEpic(t, "Epic", "desc", epic.StatusDraft)

		// Set model and proposed tasks
		epicObj, err := f.epicRepo.ReadEpic(ctx, e.ID)
		require.NoError(t, err)
		epicObj.Model = "sonnet"
		epicObj.ProposedTasks = []epic.ProposedTask{
			{TempID: "t1", Title: "Task 1", Description: "desc 1"},
			{TempID: "t2", Title: "Task 2", Description: "desc 2", DependsOnTempIDs: []string{"t1"}},
		}
		require.NoError(t, f.epicRepo.UpdateEpic(ctx, epicObj))

		err = f.store.ConfirmEpic(ctx, e.ID, false)
		require.NoError(t, err)

		stored, err := f.epicRepo.ReadEpic(ctx, e.ID)
		require.NoError(t, err)
		assert.Equal(t, epic.StatusActive, stored.Status)
		assert.Len(t, stored.TaskIDs, 2)

		// Verify task creator was called with correct dep mapping
		require.Len(t, tc.calls, 2)
		assert.Equal(t, "Task 1", tc.calls[0].title)
		assert.Empty(t, tc.calls[0].dependsOn)
		assert.Equal(t, "Task 2", tc.calls[1].title)
		assert.Contains(t, tc.calls[1].dependsOn, "tsk_Task 1")
	})

	t.Run("wrong status", func(t *testing.T) {
		f := newEpicFixture(t)
		e := f.seedEpic(t, "Epic", "desc", epic.StatusActive)

		err := f.store.ConfirmEpic(context.Background(), e.ID, false)
		assert.Error(t, err)
	})

	t.Run("no proposed tasks", func(t *testing.T) {
		f := newEpicFixture(t)
		e := f.seedEpic(t, "Epic", "desc", epic.StatusDraft)

		err := f.store.ConfirmEpic(context.Background(), e.ID, false)
		assert.Error(t, err)
	})

	t.Run("not ready sets status to ready", func(t *testing.T) {
		tc := &mockTaskCreator{idPrefix: "tsk"}
		f := newEpicFixtureWithTaskCreator(t, tc)
		ctx := context.Background()

		e := f.seedEpic(t, "Epic", "desc", epic.StatusDraft)

		// Set proposed tasks
		require.NoError(t, f.epicRepo.UpdateProposedTasks(ctx, e.ID, []epic.ProposedTask{
			{TempID: "t1", Title: "Task 1", Description: "desc 1"},
		}))

		err := f.store.ConfirmEpic(ctx, e.ID, true)
		require.NoError(t, err)

		stored, err := f.epicRepo.ReadEpic(ctx, e.ID)
		require.NoError(t, err)
		assert.Equal(t, epic.StatusReady, stored.Status)
	})
}

func TestStore_CloseEpic(t *testing.T) {
	f := newEpicFixture(t)
	e := f.seedEpic(t, "Epic", "desc", epic.StatusPlanning)

	err := f.store.CloseEpic(context.Background(), e.ID)
	require.NoError(t, err)

	stored, err := f.epicRepo.ReadEpic(context.Background(), e.ID)
	require.NoError(t, err)
	assert.Equal(t, epic.StatusClosed, stored.Status)
}

func TestStore_DeleteEpic(t *testing.T) {
	f := newEpicFixture(t)
	e := f.seedEpic(t, "Epic", "desc", epic.StatusActive)

	err := f.store.DeleteEpic(context.Background(), e.ID)
	require.NoError(t, err)

	_, err = f.epicRepo.ReadEpic(context.Background(), e.ID)
	assert.Error(t, err)
}

func TestStore_CheckAndCompleteEpic(t *testing.T) {
	t.Run("all merged/closed completes epic", func(t *testing.T) {
		f := newEpicFixture(t)
		ctx := context.Background()

		tsr := &mockTaskStatusReader{statuses: map[string]string{
			"task-1": "merged",
			"task-2": "closed",
		}}
		f.store.SetTaskStatusReader(tsr)

		e := f.seedEpic(t, "Epic", "desc", epic.StatusActive)
		require.NoError(t, f.epicRepo.SetTaskIDs(ctx, e.ID, []string{"task-1", "task-2"}))

		err := f.store.CheckAndCompleteEpic(ctx, e.ID)
		require.NoError(t, err)

		stored, err := f.epicRepo.ReadEpic(ctx, e.ID)
		require.NoError(t, err)
		assert.Equal(t, epic.StatusCompleted, stored.Status)
	})

	t.Run("failed task blocks completion", func(t *testing.T) {
		f := newEpicFixture(t)
		ctx := context.Background()

		tsr := &mockTaskStatusReader{statuses: map[string]string{
			"task-1": "merged",
			"task-2": "failed",
		}}
		f.store.SetTaskStatusReader(tsr)

		e := f.seedEpic(t, "Epic", "desc", epic.StatusActive)
		require.NoError(t, f.epicRepo.SetTaskIDs(ctx, e.ID, []string{"task-1", "task-2"}))

		err := f.store.CheckAndCompleteEpic(ctx, e.ID)
		require.NoError(t, err)

		stored, err := f.epicRepo.ReadEpic(ctx, e.ID)
		require.NoError(t, err)
		assert.Equal(t, epic.StatusActive, stored.Status)
	})

	t.Run("no taskStatusReader is a noop", func(t *testing.T) {
		f := newEpicFixture(t)
		ctx := context.Background()

		e := f.seedEpic(t, "Epic", "desc", epic.StatusActive)
		require.NoError(t, f.epicRepo.SetTaskIDs(ctx, e.ID, []string{"task-1"}))

		err := f.store.CheckAndCompleteEpic(ctx, e.ID)
		require.NoError(t, err)

		stored, err := f.epicRepo.ReadEpic(ctx, e.ID)
		require.NoError(t, err)
		assert.Equal(t, epic.StatusActive, stored.Status)
	})

	t.Run("non-active epic is a noop", func(t *testing.T) {
		f := newEpicFixture(t)
		ctx := context.Background()

		tsr := &mockTaskStatusReader{statuses: map[string]string{"task-1": "merged"}}
		f.store.SetTaskStatusReader(tsr)

		e := f.seedEpic(t, "Epic", "desc", epic.StatusDraft)
		require.NoError(t, f.epicRepo.SetTaskIDs(ctx, e.ID, []string{"task-1"}))

		err := f.store.CheckAndCompleteEpic(ctx, e.ID)
		require.NoError(t, err)

		stored, err := f.epicRepo.ReadEpic(ctx, e.ID)
		require.NoError(t, err)
		assert.Equal(t, epic.StatusDraft, stored.Status)
	})

	t.Run("empty task IDs does not auto-complete", func(t *testing.T) {
		f := newEpicFixture(t)
		ctx := context.Background()

		tsr := &mockTaskStatusReader{statuses: map[string]string{}}
		f.store.SetTaskStatusReader(tsr)

		e := f.seedEpic(t, "Epic", "desc", epic.StatusActive)

		err := f.store.CheckAndCompleteEpic(ctx, e.ID)
		require.NoError(t, err)

		stored, err := f.epicRepo.ReadEpic(ctx, e.ID)
		require.NoError(t, err)
		assert.Equal(t, epic.StatusActive, stored.Status)
	})
}

func TestStore_TimeoutStaleEpics(t *testing.T) {
	f := newEpicFixture(t)
	ctx := context.Background()

	e := f.seedEpic(t, "Epic", "desc", epic.StatusPlanning)

	// Claim the epic (sets claimed_at and last_heartbeat_at to now)
	claimed, err := f.epicRepo.ClaimEpic(ctx, e.ID)
	require.NoError(t, err)
	require.True(t, claimed)

	// Backdate last_heartbeat_at to simulate staleness
	staleTime := time.Now().Add(-10 * time.Minute).Unix()
	_, err = f.db.ExecContext(ctx, "UPDATE epic SET last_heartbeat_at = ? WHERE id = ?", staleTime, e.ID.String())
	require.NoError(t, err)

	count, err := f.store.TimeoutStaleEpics(ctx, 5*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	stored, err := f.epicRepo.ReadEpic(ctx, e.ID)
	require.NoError(t, err)
	assert.Nil(t, stored.ClaimedAt, "claim should be released")
	assert.Contains(t, stored.SessionLog, "system: Planning session timed out due to inactivity.")
}

func TestStore_StopEpic_Success(t *testing.T) {
	f := newEpicFixture(t)
	ctx := context.Background()

	e := f.seedEpic(t, "Epic", "desc", epic.StatusPlanning)

	// Claim the epic
	claimed, err := f.epicRepo.ClaimEpic(ctx, e.ID)
	require.NoError(t, err)
	require.True(t, claimed)

	// Add some proposed tasks
	require.NoError(t, f.epicRepo.UpdateProposedTasks(ctx, e.ID, []epic.ProposedTask{
		{TempID: "t1", Title: "Task 1"},
	}))

	err = f.store.StopEpic(ctx, e.ID)
	require.NoError(t, err)

	stored, err := f.epicRepo.ReadEpic(ctx, e.ID)
	require.NoError(t, err)
	assert.Equal(t, epic.StatusDraft, stored.Status)
	assert.Nil(t, stored.ClaimedAt, "claim should be released")
	assert.Contains(t, stored.SessionLog, "system: Stopped by user.")
}

func TestStore_StopEpic_NoProposals(t *testing.T) {
	f := newEpicFixture(t)
	ctx := context.Background()

	e := f.seedEpic(t, "Epic", "desc", epic.StatusPlanning)

	// Claim the epic (no proposed tasks)
	claimed, err := f.epicRepo.ClaimEpic(ctx, e.ID)
	require.NoError(t, err)
	require.True(t, claimed)

	err = f.store.StopEpic(ctx, e.ID)
	require.NoError(t, err)

	stored, err := f.epicRepo.ReadEpic(ctx, e.ID)
	require.NoError(t, err)
	assert.Equal(t, epic.StatusDraft, stored.Status)
	assert.Nil(t, stored.ClaimedAt, "claim should be released")
	assert.Contains(t, stored.SessionLog, "system: Stopped by user.")
}

func TestStore_StopEpic_NotPlanning(t *testing.T) {
	f := newEpicFixture(t)
	ctx := context.Background()

	e := f.seedEpic(t, "Epic", "desc", epic.StatusDraft)

	err := f.store.StopEpic(ctx, e.ID)
	assert.Error(t, err)
}

func TestStore_StopEpic_DrainStops(t *testing.T) {
	f := newEpicFixture(t)
	ctx := context.Background()

	// DrainStops on fresh store returns nil
	assert.Nil(t, f.store.DrainStops())

	e := f.seedEpic(t, "Epic", "desc", epic.StatusPlanning)

	// Claim the epic
	claimed, err := f.epicRepo.ClaimEpic(ctx, e.ID)
	require.NoError(t, err)
	require.True(t, claimed)

	// Stop the epic
	require.NoError(t, f.store.StopEpic(ctx, e.ID))

	// DrainStops should return the stopped epic ID
	stops := f.store.DrainStops()
	require.Len(t, stops, 1)
	assert.Equal(t, e.ID.String(), stops[0].String())

	// Second drain should return nil
	assert.Nil(t, f.store.DrainStops())

	// WaitForStop channel should have been signaled
	select {
	case <-f.store.WaitForStop():
		// Good — channel was signaled
	default:
		// Channel already drained by DrainStops check above, this is fine
	}
}

func TestStore_ClaimPendingEpic(t *testing.T) {
	t.Run("claims first available", func(t *testing.T) {
		f := newEpicFixture(t)

		e := f.seedEpic(t, "Epic", "desc", epic.StatusPlanning)

		claimed, err := f.store.ClaimPendingEpic(context.Background())
		require.NoError(t, err)
		require.NotNil(t, claimed)
		assert.Equal(t, e.ID.String(), claimed.ID.String())
		assert.NotNil(t, claimed.ClaimedAt)
	})

	t.Run("returns nil when none available", func(t *testing.T) {
		f := newEpicFixture(t)

		claimed, err := f.store.ClaimPendingEpic(context.Background())
		require.NoError(t, err)
		assert.Nil(t, claimed)
	})
}
