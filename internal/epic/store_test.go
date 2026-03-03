package epic

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/joshjon/kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock Repository ---

type mockRepository struct {
	mu    sync.Mutex
	epics map[string]*Epic

	createEpicErr      error
	readEpicErr        error
	updateEpicErr      error
	updateStatusErr    error
	deleteEpicErr      error
	claimResult        bool
	claimErr           error
	setFeedbackErr     error
	clearFeedbackErr   error
	releaseClaimErr    error
	updateProposedErr  error
	setTaskIDsErr      error
	appendLogErr       error
	heartbeatErr       error
	removeTaskIDErr    error
}

func newMockRepo() *mockRepository {
	return &mockRepository{
		epics:       make(map[string]*Epic),
		claimResult: true,
	}
}

func (m *mockRepository) CreateEpic(_ context.Context, e *Epic) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createEpicErr != nil {
		return m.createEpicErr
	}
	m.epics[e.ID.String()] = e
	return nil
}

func (m *mockRepository) ReadEpic(_ context.Context, id EpicID) (*Epic, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readEpicErr != nil {
		return nil, m.readEpicErr
	}
	e, ok := m.epics[id.String()]
	if !ok {
		return nil, errors.New("epic not found")
	}
	return e, nil
}

func (m *mockRepository) ListEpics(_ context.Context) ([]*Epic, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*Epic
	for _, e := range m.epics {
		result = append(result, e)
	}
	return result, nil
}

func (m *mockRepository) ListEpicsByRepo(_ context.Context, repoID string) ([]*Epic, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*Epic
	for _, e := range m.epics {
		if e.RepoID == repoID {
			result = append(result, e)
		}
	}
	return result, nil
}

func (m *mockRepository) UpdateEpic(_ context.Context, e *Epic) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateEpicErr != nil {
		return m.updateEpicErr
	}
	m.epics[e.ID.String()] = e
	return nil
}

func (m *mockRepository) UpdateEpicStatus(_ context.Context, id EpicID, status Status) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateStatusErr != nil {
		return m.updateStatusErr
	}
	if e, ok := m.epics[id.String()]; ok {
		e.Status = status
	}
	return nil
}

func (m *mockRepository) UpdateProposedTasks(_ context.Context, id EpicID, tasks []ProposedTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateProposedErr != nil {
		return m.updateProposedErr
	}
	if e, ok := m.epics[id.String()]; ok {
		e.ProposedTasks = tasks
	}
	return nil
}

func (m *mockRepository) SetTaskIDs(_ context.Context, id EpicID, taskIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setTaskIDsErr != nil {
		return m.setTaskIDsErr
	}
	if e, ok := m.epics[id.String()]; ok {
		e.TaskIDs = taskIDs
	}
	return nil
}

func (m *mockRepository) AppendSessionLog(_ context.Context, id EpicID, lines []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.appendLogErr != nil {
		return m.appendLogErr
	}
	if e, ok := m.epics[id.String()]; ok {
		e.SessionLog = append(e.SessionLog, lines...)
	}
	return nil
}

func (m *mockRepository) DeleteEpic(_ context.Context, id EpicID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.deleteEpicErr != nil {
		return m.deleteEpicErr
	}
	delete(m.epics, id.String())
	return nil
}

func (m *mockRepository) ListPlanningEpics(_ context.Context) ([]*Epic, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*Epic
	for _, e := range m.epics {
		if e.Status == StatusPlanning && e.ClaimedAt == nil {
			result = append(result, e)
		}
	}
	return result, nil
}

func (m *mockRepository) ClaimEpic(_ context.Context, id EpicID) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.claimErr != nil {
		return false, m.claimErr
	}
	if !m.claimResult {
		return false, nil
	}
	if e, ok := m.epics[id.String()]; ok {
		now := time.Now()
		e.ClaimedAt = &now
	}
	return true, nil
}

func (m *mockRepository) EpicHeartbeat(_ context.Context, id EpicID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.heartbeatErr != nil {
		return m.heartbeatErr
	}
	if e, ok := m.epics[id.String()]; ok {
		now := time.Now()
		e.LastHeartbeatAt = &now
	}
	return nil
}

func (m *mockRepository) SetEpicFeedback(_ context.Context, id EpicID, feedback, feedbackType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setFeedbackErr != nil {
		return m.setFeedbackErr
	}
	if e, ok := m.epics[id.String()]; ok {
		e.Feedback = &feedback
		e.FeedbackType = &feedbackType
	}
	return nil
}

func (m *mockRepository) ClearEpicFeedback(_ context.Context, id EpicID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.clearFeedbackErr != nil {
		return m.clearFeedbackErr
	}
	if e, ok := m.epics[id.String()]; ok {
		e.Feedback = nil
		e.FeedbackType = nil
	}
	return nil
}

func (m *mockRepository) ReleaseEpicClaim(_ context.Context, id EpicID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.releaseClaimErr != nil {
		return m.releaseClaimErr
	}
	if e, ok := m.epics[id.String()]; ok {
		e.ClaimedAt = nil
	}
	return nil
}

func (m *mockRepository) ListStaleEpics(_ context.Context, threshold time.Time) ([]*Epic, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*Epic
	for _, e := range m.epics {
		if e.ClaimedAt != nil && e.Status == StatusPlanning {
			hb := e.ClaimedAt
			if e.LastHeartbeatAt != nil {
				hb = e.LastHeartbeatAt
			}
			if hb.Before(threshold) {
				result = append(result, e)
			}
		}
	}
	return result, nil
}

func (m *mockRepository) ListActiveEpics(_ context.Context) ([]*Epic, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*Epic
	for _, e := range m.epics {
		if e.Status == StatusActive {
			result = append(result, e)
		}
	}
	return result, nil
}

func (m *mockRepository) RemoveTaskID(_ context.Context, id EpicID, taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.removeTaskIDErr != nil {
		return m.removeTaskIDErr
	}
	if e, ok := m.epics[id.String()]; ok {
		filtered := make([]string, 0, len(e.TaskIDs))
		for _, tid := range e.TaskIDs {
			if tid != taskID {
				filtered = append(filtered, tid)
			}
		}
		e.TaskIDs = filtered
	}
	return nil
}

// --- Mock TaskCreator ---

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

// --- Mock TaskStatusReader ---

type mockTaskStatusReader struct {
	statuses map[string]string
	err      error
}

func (m *mockTaskStatusReader) ReadTaskStatus(_ context.Context, taskID string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	status, ok := m.statuses[taskID]
	if !ok {
		return "", errors.New("task not found")
	}
	return status, nil
}

// --- Helper ---

func newTestStore(repo Repository, tc TaskCreator) *Store {
	logger := log.NewLogger(log.WithNop())
	s := NewStore(repo, tc, logger)
	return s
}

// --- Store Tests ---

func TestStore_CreateEpic(t *testing.T) {
	repo := newMockRepo()
	store := newTestStore(repo, nil)

	e := NewEpic("repo_123", "Test Epic", "description")
	err := store.CreateEpic(context.Background(), e)
	require.NoError(t, err)

	// Should be stored in repo
	stored, err := repo.ReadEpic(context.Background(), e.ID)
	require.NoError(t, err)
	assert.Equal(t, e.Title, stored.Title)

	// Should notify pending
	select {
	case <-store.WaitForPending():
		// Good
	default:
		assert.Fail(t, "expected pending notification")
	}
}

func TestStore_RequestChanges(t *testing.T) {
	t.Run("stores feedback and transitions to planning", func(t *testing.T) {
		repo := newMockRepo()
		store := newTestStore(repo, nil)

		e := NewEpic("repo_123", "Epic", "desc")
		e.Status = StatusDraft
		require.NoError(t, repo.CreateEpic(context.Background(), e))

		err := store.RequestChanges(context.Background(), e.ID, "please change X")
		require.NoError(t, err)

		stored, err := repo.ReadEpic(context.Background(), e.ID)
		require.NoError(t, err)
		require.NotNil(t, stored.Feedback)
		assert.Equal(t, "please change X", *stored.Feedback)
		assert.Equal(t, StatusPlanning, stored.Status)

		// Should notify pending for workers to pick up
		select {
		case <-store.WaitForPending():
		default:
			assert.Fail(t, "expected pending notification")
		}
	})

	t.Run("rejects non-draft status", func(t *testing.T) {
		repo := newMockRepo()
		store := newTestStore(repo, nil)

		e := NewEpic("repo_123", "Epic", "desc")
		e.Status = StatusActive
		require.NoError(t, repo.CreateEpic(context.Background(), e))

		err := store.RequestChanges(context.Background(), e.ID, "please change X")
		assert.Error(t, err)
	})
}

func TestStore_CompletePlanning(t *testing.T) {
	repo := newMockRepo()
	store := newTestStore(repo, nil)

	e := NewEpic("repo_123", "Epic", "desc")
	e.Status = StatusPlanning
	now := time.Now()
	e.ClaimedAt = &now
	fb := "some feedback"
	ft := string(FeedbackMessage)
	e.Feedback = &fb
	e.FeedbackType = &ft
	require.NoError(t, repo.CreateEpic(context.Background(), e))

	tasks := []ProposedTask{
		{TempID: "t1", Title: "Task 1", Description: "desc 1"},
		{TempID: "t2", Title: "Task 2", Description: "desc 2"},
	}
	err := store.CompletePlanning(context.Background(), e.ID, tasks)
	require.NoError(t, err)

	stored, _ := repo.ReadEpic(context.Background(), e.ID)
	assert.Len(t, stored.ProposedTasks, 2)
	assert.Equal(t, StatusDraft, stored.Status)
	assert.Nil(t, stored.ClaimedAt, "claim should be released")
	assert.Nil(t, stored.Feedback, "feedback should be cleared")
	assert.Nil(t, stored.FeedbackType, "feedback type should be cleared")
}

func TestStore_FailPlanning(t *testing.T) {
	t.Run("with existing proposals goes to draft", func(t *testing.T) {
		repo := newMockRepo()
		store := newTestStore(repo, nil)

		e := NewEpic("repo_123", "Epic", "desc")
		e.Status = StatusPlanning
		now := time.Now()
		e.ClaimedAt = &now
		e.ProposedTasks = []ProposedTask{{TempID: "t1", Title: "Task 1"}}
		require.NoError(t, repo.CreateEpic(context.Background(), e))

		err := store.FailPlanning(context.Background(), e.ID)
		require.NoError(t, err)

		stored, _ := repo.ReadEpic(context.Background(), e.ID)
		assert.Nil(t, stored.ClaimedAt, "claim should be released")
		assert.Equal(t, StatusDraft, stored.Status)
	})

	t.Run("without proposals stays in planning", func(t *testing.T) {
		repo := newMockRepo()
		store := newTestStore(repo, nil)

		e := NewEpic("repo_123", "Epic", "desc")
		e.Status = StatusPlanning
		now := time.Now()
		e.ClaimedAt = &now
		require.NoError(t, repo.CreateEpic(context.Background(), e))

		err := store.FailPlanning(context.Background(), e.ID)
		require.NoError(t, err)

		stored, _ := repo.ReadEpic(context.Background(), e.ID)
		assert.Nil(t, stored.ClaimedAt, "claim should be released")
		assert.Equal(t, StatusPlanning, stored.Status)

		// Should notify pending for retry
		select {
		case <-store.WaitForPending():
		default:
			assert.Fail(t, "expected pending notification for retry")
		}
	})
}

func TestStore_UpdateProposedTasks(t *testing.T) {
	repo := newMockRepo()
	store := newTestStore(repo, nil)

	e := NewEpic("repo_123", "Epic", "desc")
	e.Status = StatusDraft
	require.NoError(t, repo.CreateEpic(context.Background(), e))

	tasks := []ProposedTask{
		{TempID: "t1", Title: "Task 1", Description: "desc 1"},
		{TempID: "t2", Title: "Task 2", Description: "desc 2"},
	}
	err := store.UpdateProposedTasks(context.Background(), e.ID, tasks)
	require.NoError(t, err)

	stored, _ := repo.ReadEpic(context.Background(), e.ID)
	assert.Len(t, stored.ProposedTasks, 2)
	// UpdateProposedTasks no longer changes status (that's CompletePlanning)
	assert.Equal(t, StatusDraft, stored.Status)
}

func TestStore_StartPlanning(t *testing.T) {
	tests := []struct {
		name    string
		status  Status
		wantErr bool
	}{
		{"from draft", StatusDraft, false},
		{"from ready", StatusReady, false},
		{"from active (invalid)", StatusActive, true},
		{"from completed (invalid)", StatusCompleted, true},
		{"from closed (invalid)", StatusClosed, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepo()
			store := newTestStore(repo, nil)

			e := NewEpic("repo_123", "Epic", "desc")
			e.Status = tt.status
			require.NoError(t, repo.CreateEpic(context.Background(), e))

			err := store.StartPlanning(context.Background(), e.ID, "new prompt")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				stored, _ := repo.ReadEpic(context.Background(), e.ID)
				assert.Equal(t, StatusPlanning, stored.Status)
				assert.Equal(t, "new prompt", stored.PlanningPrompt)
			}
		})
	}
}

func TestStore_ConfirmEpic(t *testing.T) {
	t.Run("creates tasks from proposed and activates", func(t *testing.T) {
		repo := newMockRepo()
		tc := &mockTaskCreator{idPrefix: "tsk"}
		store := newTestStore(repo, tc)

		e := NewEpic("repo_123", "Epic", "desc")
		e.Status = StatusDraft
		e.Model = "sonnet"
		e.ProposedTasks = []ProposedTask{
			{TempID: "t1", Title: "Task 1", Description: "desc 1"},
			{TempID: "t2", Title: "Task 2", Description: "desc 2", DependsOnTempIDs: []string{"t1"}},
		}
		require.NoError(t, repo.CreateEpic(context.Background(), e))

		err := store.ConfirmEpic(context.Background(), e.ID, false)
		require.NoError(t, err)

		stored, _ := repo.ReadEpic(context.Background(), e.ID)
		assert.Equal(t, StatusActive, stored.Status)
		assert.Len(t, stored.TaskIDs, 2)

		// Verify task creator was called with correct dep mapping
		require.Len(t, tc.calls, 2)
		assert.Equal(t, "Task 1", tc.calls[0].title)
		assert.Empty(t, tc.calls[0].dependsOn)
		assert.Equal(t, "Task 2", tc.calls[1].title)
		assert.Contains(t, tc.calls[1].dependsOn, "tsk_Task 1")
	})

	t.Run("wrong status", func(t *testing.T) {
		repo := newMockRepo()
		store := newTestStore(repo, nil)

		e := NewEpic("repo_123", "Epic", "desc")
		e.Status = StatusActive
		require.NoError(t, repo.CreateEpic(context.Background(), e))

		err := store.ConfirmEpic(context.Background(), e.ID, false)
		assert.Error(t, err)
	})

	t.Run("no proposed tasks", func(t *testing.T) {
		repo := newMockRepo()
		store := newTestStore(repo, nil)

		e := NewEpic("repo_123", "Epic", "desc")
		e.Status = StatusDraft
		require.NoError(t, repo.CreateEpic(context.Background(), e))

		err := store.ConfirmEpic(context.Background(), e.ID, false)
		assert.Error(t, err)
	})

	t.Run("not ready sets status to ready", func(t *testing.T) {
		repo := newMockRepo()
		tc := &mockTaskCreator{idPrefix: "tsk"}
		store := newTestStore(repo, tc)

		e := NewEpic("repo_123", "Epic", "desc")
		e.Status = StatusDraft
		e.ProposedTasks = []ProposedTask{
			{TempID: "t1", Title: "Task 1", Description: "desc 1"},
		}
		require.NoError(t, repo.CreateEpic(context.Background(), e))

		err := store.ConfirmEpic(context.Background(), e.ID, true)
		require.NoError(t, err)

		stored, _ := repo.ReadEpic(context.Background(), e.ID)
		assert.Equal(t, StatusReady, stored.Status)
	})
}

func TestStore_CloseEpic(t *testing.T) {
	repo := newMockRepo()
	store := newTestStore(repo, nil)

	e := NewEpic("repo_123", "Epic", "desc")
	require.NoError(t, repo.CreateEpic(context.Background(), e))

	err := store.CloseEpic(context.Background(), e.ID)
	require.NoError(t, err)

	stored, _ := repo.ReadEpic(context.Background(), e.ID)
	assert.Equal(t, StatusClosed, stored.Status)
}

func TestStore_DeleteEpic(t *testing.T) {
	repo := newMockRepo()
	store := newTestStore(repo, nil)

	e := NewEpic("repo_123", "Epic", "desc")
	e.Status = StatusActive
	require.NoError(t, repo.CreateEpic(context.Background(), e))

	err := store.DeleteEpic(context.Background(), e.ID)
	require.NoError(t, err)

	_, err = repo.ReadEpic(context.Background(), e.ID)
	assert.Error(t, err)
}

func TestStore_CheckAndCompleteEpic(t *testing.T) {
	t.Run("all merged/closed completes epic", func(t *testing.T) {
		repo := newMockRepo()
		tsr := &mockTaskStatusReader{statuses: map[string]string{
			"task-1": "merged",
			"task-2": "closed",
		}}
		store := newTestStore(repo, nil)
		store.SetTaskStatusReader(tsr)

		e := NewEpic("repo_123", "Epic", "desc")
		e.Status = StatusActive
		e.TaskIDs = []string{"task-1", "task-2"}
		require.NoError(t, repo.CreateEpic(context.Background(), e))

		err := store.CheckAndCompleteEpic(context.Background(), e.ID)
		require.NoError(t, err)

		stored, _ := repo.ReadEpic(context.Background(), e.ID)
		assert.Equal(t, StatusCompleted, stored.Status)
	})

	t.Run("failed task blocks completion", func(t *testing.T) {
		repo := newMockRepo()
		tsr := &mockTaskStatusReader{statuses: map[string]string{
			"task-1": "merged",
			"task-2": "failed",
		}}
		store := newTestStore(repo, nil)
		store.SetTaskStatusReader(tsr)

		e := NewEpic("repo_123", "Epic", "desc")
		e.Status = StatusActive
		e.TaskIDs = []string{"task-1", "task-2"}
		require.NoError(t, repo.CreateEpic(context.Background(), e))

		err := store.CheckAndCompleteEpic(context.Background(), e.ID)
		require.NoError(t, err)

		stored, _ := repo.ReadEpic(context.Background(), e.ID)
		assert.Equal(t, StatusActive, stored.Status)
	})

	t.Run("no taskStatusReader is a noop", func(t *testing.T) {
		repo := newMockRepo()
		store := newTestStore(repo, nil)

		e := NewEpic("repo_123", "Epic", "desc")
		e.Status = StatusActive
		e.TaskIDs = []string{"task-1"}
		require.NoError(t, repo.CreateEpic(context.Background(), e))

		err := store.CheckAndCompleteEpic(context.Background(), e.ID)
		require.NoError(t, err)

		stored, _ := repo.ReadEpic(context.Background(), e.ID)
		assert.Equal(t, StatusActive, stored.Status)
	})

	t.Run("non-active epic is a noop", func(t *testing.T) {
		repo := newMockRepo()
		tsr := &mockTaskStatusReader{statuses: map[string]string{"task-1": "merged"}}
		store := newTestStore(repo, nil)
		store.SetTaskStatusReader(tsr)

		e := NewEpic("repo_123", "Epic", "desc")
		e.Status = StatusDraft
		e.TaskIDs = []string{"task-1"}
		require.NoError(t, repo.CreateEpic(context.Background(), e))

		err := store.CheckAndCompleteEpic(context.Background(), e.ID)
		require.NoError(t, err)

		stored, _ := repo.ReadEpic(context.Background(), e.ID)
		assert.Equal(t, StatusDraft, stored.Status)
	})

	t.Run("empty task IDs does not auto-complete", func(t *testing.T) {
		repo := newMockRepo()
		tsr := &mockTaskStatusReader{statuses: map[string]string{}}
		store := newTestStore(repo, nil)
		store.SetTaskStatusReader(tsr)

		e := NewEpic("repo_123", "Epic", "desc")
		e.Status = StatusActive
		e.TaskIDs = []string{}
		require.NoError(t, repo.CreateEpic(context.Background(), e))

		err := store.CheckAndCompleteEpic(context.Background(), e.ID)
		require.NoError(t, err)

		stored, _ := repo.ReadEpic(context.Background(), e.ID)
		assert.Equal(t, StatusActive, stored.Status)
	})
}

func TestStore_TimeoutStaleEpics(t *testing.T) {
	repo := newMockRepo()
	store := newTestStore(repo, nil)

	e := NewEpic("repo_123", "Epic", "desc")
	e.Status = StatusPlanning
	staleTime := time.Now().Add(-10 * time.Minute)
	e.ClaimedAt = &staleTime
	require.NoError(t, repo.CreateEpic(context.Background(), e))

	count, err := store.TimeoutStaleEpics(context.Background(), 5*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	stored, _ := repo.ReadEpic(context.Background(), e.ID)
	assert.Nil(t, stored.ClaimedAt, "claim should be released")
	assert.Contains(t, stored.SessionLog, "system: Planning session timed out due to inactivity.")
}

func TestStore_ClaimPendingEpic(t *testing.T) {
	t.Run("claims first available", func(t *testing.T) {
		repo := newMockRepo()
		store := newTestStore(repo, nil)

		e := NewEpic("repo_123", "Epic", "desc")
		e.Status = StatusPlanning
		require.NoError(t, repo.CreateEpic(context.Background(), e))

		claimed, err := store.ClaimPendingEpic(context.Background())
		require.NoError(t, err)
		require.NotNil(t, claimed)
		assert.Equal(t, e.ID.String(), claimed.ID.String())
		assert.NotNil(t, claimed.ClaimedAt)
	})

	t.Run("returns nil when none available", func(t *testing.T) {
		repo := newMockRepo()
		store := newTestStore(repo, nil)

		claimed, err := store.ClaimPendingEpic(context.Background())
		require.NoError(t, err)
		assert.Nil(t, claimed)
	})
}
