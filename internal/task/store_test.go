package task

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/joshjon/kit/tx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRepository implements the Repository interface for testing.
type mockRepository struct {
	tasks              map[string]*Task
	taskStatuses       map[string]Status
	logs               map[string][]string
	consecutiveFailMap map[string]int
	mu                 sync.Mutex

	createTaskErr                    error
	readTaskErr                      error
	taskExistsResult                 bool
	taskExistsErr                    error
	updateStatusErr                  error
	retryTaskResult                  bool
	retryTaskErr                     error
	scheduleRetryFromRunningResult   bool
	scheduleRetryFromRunningErr      error
	manualRetryTaskResult            bool
	manualRetryTaskErr               error
	feedbackRetryResult              bool
	feedbackRetryErr                 error
	claimTaskResult                  bool
	claimTaskErr                     error
	closeTaskErr                     error
	setAgentStatusErr                error
	setRetryContextErr               error
	addCostErr                       error
	setConsFailErr                   error
	setCloseReasonErr                error
	setBranchNameErr                 error
	appendLogsErr                    error
	setPullRequestErr                error
	hasTasksForRepoResult            bool
	hasTasksForRepoErr               error

	// Track calls
	createCalls                      int
	retryTaskCalls                   int
	scheduleRetryFromRunningCalls    int
	setConsFails                     []int
}

func newMockRepo() *mockRepository {
	return &mockRepository{
		tasks:              make(map[string]*Task),
		taskStatuses:       make(map[string]Status),
		logs:               make(map[string][]string),
		consecutiveFailMap: make(map[string]int),
	}
}

func (m *mockRepository) CreateTask(_ context.Context, task *Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createCalls++
	if m.createTaskErr != nil {
		return m.createTaskErr
	}
	m.tasks[task.ID.String()] = task
	m.taskStatuses[task.ID.String()] = task.Status
	return nil
}

func (m *mockRepository) ReadTask(_ context.Context, id TaskID) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readTaskErr != nil {
		return nil, m.readTaskErr
	}
	t, ok := m.tasks[id.String()]
	if !ok {
		return nil, errors.New("task not found")
	}
	return t, nil
}

func (m *mockRepository) ListTasks(_ context.Context) ([]*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*Task
	for _, t := range m.tasks {
		result = append(result, t)
	}
	return result, nil
}

func (m *mockRepository) ListTasksByRepo(_ context.Context, repoID string) ([]*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*Task
	for _, t := range m.tasks {
		if t.RepoID == repoID {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockRepository) ListPendingTasks(_ context.Context) ([]*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*Task
	for _, t := range m.tasks {
		if t.Status == StatusPending {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockRepository) ListPendingTasksByRepos(_ context.Context, repoIDs []string) ([]*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	repoSet := make(map[string]bool)
	for _, id := range repoIDs {
		repoSet[id] = true
	}
	var result []*Task
	for _, t := range m.tasks {
		if t.Status == StatusPending && repoSet[t.RepoID] {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockRepository) AppendTaskLogs(_ context.Context, id TaskID, _ int, logs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.appendLogsErr != nil {
		return m.appendLogsErr
	}
	m.logs[id.String()] = append(m.logs[id.String()], logs...)
	return nil
}

func (m *mockRepository) ReadTaskLogs(_ context.Context, id TaskID) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.logs[id.String()], nil
}

func (m *mockRepository) StreamTaskLogs(_ context.Context, id TaskID, fn func(attempt int, lines []string) error) error {
	m.mu.Lock()
	logs := m.logs[id.String()]
	m.mu.Unlock()
	if len(logs) > 0 {
		return fn(1, logs)
	}
	return nil
}

func (m *mockRepository) UpdateTaskStatus(_ context.Context, id TaskID, status Status) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateStatusErr != nil {
		return m.updateStatusErr
	}
	if t, ok := m.tasks[id.String()]; ok {
		t.Status = status
	}
	m.taskStatuses[id.String()] = status
	return nil
}

func (m *mockRepository) SetTaskPullRequest(_ context.Context, id TaskID, prURL string, prNumber int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setPullRequestErr != nil {
		return m.setPullRequestErr
	}
	if t, ok := m.tasks[id.String()]; ok {
		t.PullRequestURL = prURL
		t.PRNumber = prNumber
		t.Status = StatusReview
	}
	return nil
}

func (m *mockRepository) ListTasksInReview(_ context.Context) ([]*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*Task
	for _, t := range m.tasks {
		if t.Status == StatusReview {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockRepository) ListTasksInReviewByRepo(_ context.Context, repoID string) ([]*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*Task
	for _, t := range m.tasks {
		if t.Status == StatusReview && t.RepoID == repoID {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockRepository) CloseTask(_ context.Context, id TaskID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closeTaskErr != nil {
		return m.closeTaskErr
	}
	if t, ok := m.tasks[id.String()]; ok {
		t.Status = StatusClosed
		t.CloseReason = reason
	}
	return nil
}

func (m *mockRepository) TaskExists(_ context.Context, _ TaskID) (bool, error) {
	return m.taskExistsResult, m.taskExistsErr
}

func (m *mockRepository) ReadTaskStatus(_ context.Context, id TaskID) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	status, ok := m.taskStatuses[id.String()]
	if !ok {
		return "", errors.New("not found")
	}
	return status, nil
}

func (m *mockRepository) ClaimTask(_ context.Context, id TaskID) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.claimTaskErr != nil {
		return false, m.claimTaskErr
	}
	if !m.claimTaskResult {
		return false, nil
	}
	if t, ok := m.tasks[id.String()]; ok {
		t.Status = StatusRunning
	}
	return true, nil
}

func (m *mockRepository) HasTasksForRepo(_ context.Context, _ string) (bool, error) {
	return m.hasTasksForRepoResult, m.hasTasksForRepoErr
}

func (m *mockRepository) RetryTask(_ context.Context, _ TaskID, _ string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retryTaskCalls++
	return m.retryTaskResult, m.retryTaskErr
}

func (m *mockRepository) ScheduleRetryFromRunning(_ context.Context, id TaskID, reason string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scheduleRetryFromRunningCalls++
	if m.scheduleRetryFromRunningErr != nil {
		return false, m.scheduleRetryFromRunningErr
	}
	if !m.scheduleRetryFromRunningResult {
		return false, nil
	}
	if t, ok := m.tasks[id.String()]; ok {
		t.Status = StatusPending
		t.Attempt++
		t.RetryReason = reason
		t.StartedAt = nil
	}
	return true, nil
}

func (m *mockRepository) SetAgentStatus(_ context.Context, id TaskID, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setAgentStatusErr != nil {
		return m.setAgentStatusErr
	}
	if t, ok := m.tasks[id.String()]; ok {
		t.AgentStatus = status
	}
	return nil
}

func (m *mockRepository) SetRetryContext(_ context.Context, _ TaskID, _ string) error {
	return m.setRetryContextErr
}

func (m *mockRepository) AddCost(_ context.Context, _ TaskID, _ float64) error {
	return m.addCostErr
}

func (m *mockRepository) SetConsecutiveFailures(_ context.Context, _ TaskID, count int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setConsFails = append(m.setConsFails, count)
	return m.setConsFailErr
}

func (m *mockRepository) SetCloseReason(_ context.Context, id TaskID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setCloseReasonErr != nil {
		return m.setCloseReasonErr
	}
	if t, ok := m.tasks[id.String()]; ok {
		t.CloseReason = reason
	}
	return nil
}

func (m *mockRepository) SetBranchName(_ context.Context, id TaskID, branchName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setBranchNameErr != nil {
		return m.setBranchNameErr
	}
	if t, ok := m.tasks[id.String()]; ok {
		t.BranchName = branchName
	}
	return nil
}

func (m *mockRepository) ListTasksInReviewNoPR(_ context.Context) ([]*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*Task
	for _, t := range m.tasks {
		if t.Status == StatusReview && t.PRNumber == 0 && t.BranchName != "" {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockRepository) ManualRetryTask(_ context.Context, _ TaskID, _ string) (bool, error) {
	return m.manualRetryTaskResult, m.manualRetryTaskErr
}

func (m *mockRepository) FeedbackRetryTask(_ context.Context, id TaskID, _ string) (bool, error) {
	if m.feedbackRetryResult {
		m.mu.Lock()
		if t, ok := m.tasks[id.String()]; ok {
			t.Attempt++
			t.MaxAttempts++
			t.ConsecutiveFailures = 0
			t.RetryContext = ""
		}
		m.mu.Unlock()
	}
	return m.feedbackRetryResult, m.feedbackRetryErr
}

func (m *mockRepository) DeleteTaskLogs(_ context.Context, id TaskID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.logs, id.String())
	return nil
}

func (m *mockRepository) RemoveDependency(_ context.Context, id TaskID, depID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id.String()]
	if !ok {
		return errors.New("task not found")
	}
	filtered := make([]string, 0, len(t.DependsOn))
	for _, d := range t.DependsOn {
		if d != depID {
			filtered = append(filtered, d)
		}
	}
	t.DependsOn = filtered
	return nil
}

func (m *mockRepository) SetReady(_ context.Context, id TaskID, ready bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.tasks[id.String()]; ok {
		t.Ready = ready
	}
	return nil
}

func (m *mockRepository) UpdatePendingTask(_ context.Context, id TaskID, params UpdatePendingTaskParams) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id.String()]
	if !ok {
		return false, errors.New("task not found")
	}
	if t.Status != StatusPending {
		return false, nil
	}
	t.Title = params.Title
	t.Description = params.Description
	t.DependsOn = params.DependsOn
	t.AcceptanceCriteria = params.AcceptanceCriteria
	t.MaxCostUSD = params.MaxCostUSD
	t.SkipPR = params.SkipPR
	t.Model = params.Model
	t.Ready = params.Ready
	return true, nil
}

func (m *mockRepository) StartOverTask(_ context.Context, id TaskID, params StartOverTaskParams) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id.String()]
	if !ok {
		return false, errors.New("task not found")
	}
	if t.Status != StatusReview && t.Status != StatusFailed {
		return false, nil
	}
	t.Status = StatusPending
	t.Title = params.Title
	t.Description = params.Description
	t.AcceptanceCriteria = params.AcceptanceCriteria
	t.Attempt = 1
	t.MaxAttempts = 5
	t.RetryReason = ""
	t.RetryContext = ""
	t.CloseReason = ""
	t.AgentStatus = ""
	t.ConsecutiveFailures = 0
	t.CostUSD = 0
	t.PullRequestURL = ""
	t.PRNumber = 0
	t.BranchName = ""
	t.StartedAt = nil
	return true, nil
}

func (m *mockRepository) BeginTxFunc(ctx context.Context, fn func(context.Context, tx.Tx, Repository) error) error {
	return fn(ctx, nil, m)
}

func (m *mockRepository) WithTx(_ tx.Tx) Repository {
	return m
}

func (m *mockRepository) StopTask(_ context.Context, id TaskID, reason string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id.String()]
	if !ok || t.Status != StatusRunning {
		return false, nil
	}
	t.Status = StatusPending
	t.Ready = false
	t.CloseReason = reason
	m.taskStatuses[id.String()] = StatusPending
	return true, nil
}

func (m *mockRepository) Heartbeat(_ context.Context, _ TaskID) (bool, error) {
	return true, nil
}

func (m *mockRepository) ListStaleTasks(_ context.Context, _ time.Time) ([]*Task, error) {
	return nil, nil
}

func (m *mockRepository) DeleteTask(_ context.Context, id TaskID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tasks, id.String())
	delete(m.taskStatuses, id.String())
	delete(m.logs, id.String())
	delete(m.consecutiveFailMap, id.String())
	return nil
}

func (m *mockRepository) ListTasksByEpic(_ context.Context, _ string) ([]*Task, error) {
	return nil, nil
}

func (m *mockRepository) BulkCloseTasksByEpic(_ context.Context, _ string, _ string) error {
	return nil
}

func (m *mockRepository) ClearEpicIDForTasks(_ context.Context, _ string) error {
	return nil
}

func (m *mockRepository) BulkDeleteTasksByEpic(_ context.Context, _ string) error {
	return nil
}

func (m *mockRepository) BulkDeleteTasksByIDs(_ context.Context, _ []string) error {
	return nil
}

func (m *mockRepository) DeleteExpiredLogs(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

// --- Store tests ---

func TestStore_CreateTask_Success(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "sonnet", true)
	err := store.CreateTask(context.Background(), tsk)
	require.NoError(t, err)
	assert.Equal(t, 1, repo.createCalls)
}

func TestStore_CreateTask_InvalidDependencyID(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", []string{"not-a-valid-id"}, nil, 0, false, false, "", true)
	err := store.CreateTask(context.Background(), tsk)
	assert.Error(t, err, "expected error for invalid dependency ID")
}

func TestStore_CreateTask_DependencyNotFound(t *testing.T) {
	repo := newMockRepo()
	repo.taskExistsResult = false
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	depID := NewTaskID()
	tsk := NewTask("repo_123", "title", "desc", []string{depID.String()}, nil, 0, false, false, "", true)
	err := store.CreateTask(context.Background(), tsk)
	assert.Error(t, err, "expected error for missing dependency")
}

func TestStore_CreateTask_NotifiesPending(t *testing.T) {
	repo := newMockRepo()
	repo.taskExistsResult = true
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	err := store.CreateTask(context.Background(), tsk)
	require.NoError(t, err)

	// The pending channel should have a notification
	select {
	case <-store.WaitForPending():
		// Good
	default:
		assert.Fail(t, "expected pending notification")
	}
}

func TestStore_CreateTask_PublishesEvent(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	ch := broker.Subscribe()
	defer broker.Unsubscribe(ch)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	err := store.CreateTask(context.Background(), tsk)
	require.NoError(t, err)

	select {
	case event := <-ch:
		assert.Equal(t, EventTaskCreated, event.Type)
		assert.Equal(t, "repo_123", event.RepoID)
		assert.NotNil(t, event.Task, "expected non-nil task in event")
		assert.Nil(t, event.Task.Logs, "expected nil logs in published event")
	default:
		assert.Fail(t, "expected event to be published")
	}
}

func TestStore_RetryTask_MaxAttempts(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	tsk.Attempt = 5
	tsk.MaxAttempts = 5
	tsk.Status = StatusReview
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusReview

	err := store.RetryTask(context.Background(), tsk.ID, "ci_failure:tests", "CI tests failed")
	require.NoError(t, err)

	// Task should be failed because max attempts reached
	assert.Equal(t, StatusFailed, tsk.Status)
}

func TestStore_RetryTask_BudgetExceeded(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 5.0, false, false, "", true)
	tsk.CostUSD = 6.0
	tsk.Status = StatusReview
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusReview

	err := store.RetryTask(context.Background(), tsk.ID, "", "some reason")
	require.NoError(t, err)

	assert.Equal(t, StatusFailed, tsk.Status, "expected status failed due to budget exceeded")
}

func TestStore_RetryTask_CircuitBreaker(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	tsk.Status = StatusReview
	tsk.ConsecutiveFailures = 2
	tsk.RetryReason = "ci_failure:tests: CI tests failed"
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusReview

	err := store.RetryTask(context.Background(), tsk.ID, "ci_failure:tests", "CI tests failed again")
	require.NoError(t, err)

	// Circuit breaker should trigger: same category three times
	assert.Equal(t, StatusFailed, tsk.Status, "expected status failed due to circuit breaker")
}

func TestStore_RetryTask_CircuitBreakerAllowsSecondRetry(t *testing.T) {
	repo := newMockRepo()
	repo.retryTaskResult = true
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	tsk.Status = StatusReview
	tsk.ConsecutiveFailures = 1
	tsk.RetryReason = "ci_failure:tests: CI tests failed"
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusReview

	err := store.RetryTask(context.Background(), tsk.ID, "ci_failure:tests", "CI tests failed again")
	require.NoError(t, err)

	// Circuit breaker should NOT trigger: same category only twice (threshold is 3)
	assert.NotEqual(t, StatusFailed, tsk.Status, "second consecutive failure should still allow retry")
	require.Len(t, repo.setConsFails, 1)
	assert.Equal(t, 2, repo.setConsFails[0])
}

func TestStore_RetryTask_DifferentCategory(t *testing.T) {
	repo := newMockRepo()
	repo.retryTaskResult = true
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	tsk.Status = StatusReview
	tsk.ConsecutiveFailures = 1
	tsk.RetryReason = "ci_failure:tests: CI tests failed"
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusReview

	// Different non-conflict category should NOT trip circuit breaker
	err := store.RetryTask(context.Background(), tsk.ID, "ci_failure:changelog", "changelog check failed")
	require.NoError(t, err)

	// Should have set consecutive failures to 1 (reset for new category)
	require.Len(t, repo.setConsFails, 1)
	assert.Equal(t, 1, repo.setConsFails[0])
}

func TestStore_RetryTask_MergeConflictIgnoresMaxAttempts(t *testing.T) {
	repo := newMockRepo()
	repo.feedbackRetryResult = true
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	tsk.Attempt = 5
	tsk.MaxAttempts = 5
	tsk.Status = StatusReview
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusReview

	// Merge conflict retries should NOT be blocked by max attempts
	err := store.RetryTask(context.Background(), tsk.ID, "merge_conflict", "merge_conflict: PR has conflicts with base branch")
	require.NoError(t, err)

	assert.NotEqual(t, StatusFailed, tsk.Status,
		"merge conflict retry should not fail task at max attempts")
	// Both attempt and max_attempts should be incremented
	assert.Equal(t, 6, tsk.Attempt, "attempt should be incremented")
	assert.Equal(t, 6, tsk.MaxAttempts, "max_attempts should be incremented to preserve budget")
}

func TestStore_RetryTask_MergeConflictIgnoresCircuitBreaker(t *testing.T) {
	repo := newMockRepo()
	repo.feedbackRetryResult = true
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	tsk.Status = StatusReview
	tsk.ConsecutiveFailures = 5
	tsk.RetryReason = "merge_conflict: PR has conflicts with base branch"
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusReview

	// Merge conflict retries should NOT be blocked by circuit breaker
	// even with many consecutive failures
	err := store.RetryTask(context.Background(), tsk.ID, "merge_conflict", "merge_conflict: PR has conflicts with base branch")
	require.NoError(t, err)

	assert.NotEqual(t, StatusFailed, tsk.Status,
		"merge conflict retry should not fail task due to circuit breaker")
}

func TestStore_RetryTask_MergeConflictRespectsMaxBudget(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 5.0, false, false, "", true)
	tsk.CostUSD = 6.0
	tsk.Status = StatusReview
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusReview

	// Even merge conflict retries should respect the cost budget
	err := store.RetryTask(context.Background(), tsk.ID, "merge_conflict", "merge_conflict: PR has conflicts with base branch")
	require.NoError(t, err)

	assert.Equal(t, StatusFailed, tsk.Status,
		"merge conflict retry should still fail when budget exceeded")
}

func TestStore_ManualRetryTask(t *testing.T) {
	repo := newMockRepo()
	repo.manualRetryTaskResult = true
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	tsk.Status = StatusFailed
	repo.tasks[tsk.ID.String()] = tsk

	err := store.ManualRetryTask(context.Background(), tsk.ID, "try again please")
	require.NoError(t, err)
}

func TestStore_ManualRetryTask_NotFailed(t *testing.T) {
	repo := newMockRepo()
	repo.manualRetryTaskResult = false
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	tsk.Status = StatusRunning
	repo.tasks[tsk.ID.String()] = tsk

	err := store.ManualRetryTask(context.Background(), tsk.ID, "")
	require.NoError(t, err)
	// Should be a no-op
}

func TestStore_FeedbackRetryTask_BudgetExceeded(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 5.0, false, false, "", true)
	tsk.CostUSD = 6.0
	tsk.Status = StatusReview
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusReview

	err := store.FeedbackRetryTask(context.Background(), tsk.ID, "fix the tests")
	require.NoError(t, err)

	assert.Equal(t, StatusFailed, tsk.Status, "expected status failed due to budget exceeded")
}

func TestStore_FeedbackRetryTask_IgnoresMaxAttempts(t *testing.T) {
	repo := newMockRepo()
	repo.feedbackRetryResult = true
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	tsk.Attempt = 5
	tsk.MaxAttempts = 5
	tsk.Status = StatusReview
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusReview

	err := store.FeedbackRetryTask(context.Background(), tsk.ID, "fix the tests")
	require.NoError(t, err)

	// Feedback retries should NOT be blocked by max attempts since they
	// represent user-driven iteration, not failure recovery.
	assert.NotEqual(t, StatusFailed, tsk.Status, "feedback should not fail task at max attempts")
}

func TestStore_FeedbackRetryTask_IncrementsAttemptAndMaxAttempts(t *testing.T) {
	repo := newMockRepo()
	repo.feedbackRetryResult = true
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	tsk.Attempt = 4
	tsk.MaxAttempts = 5
	tsk.Status = StatusReview
	tsk.ConsecutiveFailures = 1
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusReview

	err := store.FeedbackRetryTask(context.Background(), tsk.ID, "please update the error messages")
	require.NoError(t, err)

	// After feedback retry, both attempt and max_attempts are incremented so
	// the new attempt gets a unique number for log tabbing while the retry
	// budget remains unchanged.
	assert.Equal(t, 5, tsk.Attempt, "attempt should be incremented after feedback retry")
	assert.Equal(t, 6, tsk.MaxAttempts, "max_attempts should be incremented to preserve retry budget")
	assert.Equal(t, 0, tsk.ConsecutiveFailures, "consecutive failures should be reset after feedback retry")
}

func TestStore_FeedbackRetryTask_ClearsRetryContext(t *testing.T) {
	repo := newMockRepo()
	repo.feedbackRetryResult = true
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	tsk.Status = StatusReview
	tsk.RetryContext = "CI failure logs from previous attempt..."
	tsk.RetryReason = "ci_failure:tests: Tests failed"
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusReview

	err := store.FeedbackRetryTask(context.Background(), tsk.ID, "please fix the formatting")
	require.NoError(t, err)

	// After feedback retry, the CI failure context should be cleared because
	// the user's change request supersedes the previous CI failure info.
	assert.Empty(t, tsk.RetryContext, "retry context should be cleared after feedback retry")
}

func TestStore_FeedbackRetryTask_ThenAutomatedRetryGetsFullBudget(t *testing.T) {
	// Scenario:
	// 1. Task has used several retry attempts (attempt=4, maxAttempts=5)
	// 2. Human requests changes (feedback retry) → attempt=5, maxAttempts=6
	// 3. Agent updates code → CI fails → automated retry should succeed
	//    because max_attempts was also incremented
	repo := newMockRepo()
	repo.feedbackRetryResult = true
	repo.retryTaskResult = true
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	tsk.Attempt = 4
	tsk.MaxAttempts = 5
	tsk.Status = StatusReview
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusReview

	// Step 1: Feedback retry increments both attempt and max_attempts
	err := store.FeedbackRetryTask(context.Background(), tsk.ID, "update error handling")
	require.NoError(t, err)
	assert.Equal(t, 5, tsk.Attempt, "attempt should be incremented after feedback")
	assert.Equal(t, 6, tsk.MaxAttempts, "max_attempts should be incremented to preserve budget")

	// Simulate: agent runs again and ends up in review with attempt=5
	tsk.Status = StatusReview

	// Step 2: Automated retry after CI failure should NOT be blocked
	// because max_attempts was also incremented (attempt=5, maxAttempts=6 → room for retry)
	err = store.RetryTask(context.Background(), tsk.ID, "ci_failure:tests", "CI tests failed")
	require.NoError(t, err)
	assert.NotEqual(t, StatusFailed, tsk.Status,
		"automated retry should succeed after feedback incremented the budget")
}

func TestStore_ClaimPendingTask_NoPending(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	claimed, err := store.ClaimPendingTask(context.Background(), nil)
	require.NoError(t, err)
	assert.Nil(t, claimed, "expected nil claimed task when no pending tasks")
}

func TestStore_ClaimPendingTask_Success(t *testing.T) {
	repo := newMockRepo()
	repo.claimTaskResult = true
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusPending

	claimed, err := store.ClaimPendingTask(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, claimed, "expected non-nil claimed task")
	assert.Equal(t, StatusRunning, claimed.Status)
}

func TestStore_ClaimPendingTask_WithRepoFilter(t *testing.T) {
	repo := newMockRepo()
	repo.claimTaskResult = true
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusPending

	claimed, err := store.ClaimPendingTask(context.Background(), []string{"repo_123"})
	require.NoError(t, err)
	require.NotNil(t, claimed, "expected non-nil claimed task")
}

func TestStore_AppendTaskLogs(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	ch := broker.Subscribe()
	defer broker.Unsubscribe(ch)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	repo.tasks[tsk.ID.String()] = tsk

	err := store.AppendTaskLogs(context.Background(), tsk.ID, 1, []string{"line 1", "line 2"})
	require.NoError(t, err)

	// Check event published
	select {
	case event := <-ch:
		assert.Equal(t, EventLogsAppended, event.Type)
		assert.Len(t, event.Logs, 2)
	default:
		assert.Fail(t, "expected log event to be published")
	}
}

func TestStore_WaitForPending(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	// Initially no notification
	ch := store.WaitForPending()
	select {
	case <-ch:
		assert.Fail(t, "expected no pending notification initially")
	default:
		// Good
	}

	// Create a task to trigger notification
	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	_ = store.CreateTask(context.Background(), tsk)

	select {
	case <-ch:
		// Good
	default:
		assert.Fail(t, "expected pending notification after create")
	}
}

func TestStore_CloseTask(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	repo.tasks[tsk.ID.String()] = tsk

	err := store.CloseTask(context.Background(), tsk.ID, "no longer needed")
	require.NoError(t, err)

	assert.Equal(t, StatusClosed, tsk.Status)
	assert.Equal(t, "no longer needed", tsk.CloseReason)
}

func TestStore_UpdateTaskStatus(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	repo.tasks[tsk.ID.String()] = tsk

	err := store.UpdateTaskStatus(context.Background(), tsk.ID, StatusFailed)
	require.NoError(t, err)

	assert.Equal(t, StatusFailed, tsk.Status)
}

func TestStore_SetTaskPullRequest(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	tsk.Status = StatusRunning
	repo.tasks[tsk.ID.String()] = tsk

	err := store.SetTaskPullRequest(context.Background(), tsk.ID, "https://github.com/org/repo/pull/42", 42)
	require.NoError(t, err)

	assert.Equal(t, "https://github.com/org/repo/pull/42", tsk.PullRequestURL)
	assert.Equal(t, 42, tsk.PRNumber)
}

func TestStore_RemoveDependency_Success(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	dep := NewTask("repo_123", "dep", "dep desc", nil, nil, 0, false, false, "", true)
	repo.tasks[dep.ID.String()] = dep

	tsk := NewTask("repo_123", "title", "desc", []string{dep.ID.String()}, nil, 0, false, false, "", true)
	repo.tasks[tsk.ID.String()] = tsk

	err := store.RemoveDependency(context.Background(), tsk.ID, dep.ID.String())
	require.NoError(t, err)
	assert.Empty(t, tsk.DependsOn)
}

func TestStore_RemoveDependency_InvalidDepID(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	repo.tasks[tsk.ID.String()] = tsk

	err := store.RemoveDependency(context.Background(), tsk.ID, "not-a-valid-id")
	assert.Error(t, err, "expected error for invalid dependency ID")
}

func TestStore_RemoveDependency_NotifiesPending(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	dep := NewTask("repo_123", "dep", "dep desc", nil, nil, 0, false, false, "", true)
	repo.tasks[dep.ID.String()] = dep

	tsk := NewTask("repo_123", "title", "desc", []string{dep.ID.String()}, nil, 0, false, false, "", true)
	repo.tasks[tsk.ID.String()] = tsk

	// Drain any existing notification
	select {
	case <-store.WaitForPending():
	default:
	}

	err := store.RemoveDependency(context.Background(), tsk.ID, dep.ID.String())
	require.NoError(t, err)

	select {
	case <-store.WaitForPending():
		// Good — removing a dependency may unblock the task
	default:
		assert.Fail(t, "expected pending notification after removing dependency")
	}
}

func TestStore_ScheduleRetry_Success(t *testing.T) {
	repo := newMockRepo()
	repo.scheduleRetryFromRunningResult = true
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	tsk.Status = StatusRunning
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusRunning

	err := store.ScheduleRetry(context.Background(), tsk.ID, "rate_limit: Claude max usage exceeded")
	require.NoError(t, err)

	assert.Equal(t, StatusPending, tsk.Status, "expected task to transition to pending for retry")
	assert.Equal(t, 1, repo.scheduleRetryFromRunningCalls)
}

func TestStore_ScheduleRetry_MaxAttempts(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	tsk.Status = StatusRunning
	tsk.Attempt = 5
	tsk.MaxAttempts = 5
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusRunning

	err := store.ScheduleRetry(context.Background(), tsk.ID, "rate_limit: max usage")
	require.NoError(t, err)

	assert.Equal(t, StatusFailed, tsk.Status, "expected task to fail when max attempts reached")
	assert.Equal(t, 0, repo.scheduleRetryFromRunningCalls)
}

func TestStore_ScheduleRetry_BudgetExceeded(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 5.0, false, false, "", true)
	tsk.CostUSD = 6.0
	tsk.Status = StatusRunning
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusRunning

	err := store.ScheduleRetry(context.Background(), tsk.ID, "rate_limit: max usage")
	require.NoError(t, err)

	assert.Equal(t, StatusFailed, tsk.Status, "expected task to fail when budget exceeded")
}

func TestStore_ScheduleRetry_CircuitBreaker(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	tsk.Status = StatusRunning
	tsk.ConsecutiveFailures = 2
	tsk.RetryReason = "rate_limit: Claude max usage exceeded"
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusRunning

	err := store.ScheduleRetry(context.Background(), tsk.ID, "rate_limit: Claude max usage exceeded")
	require.NoError(t, err)

	// Circuit breaker should trigger: 3 consecutive same failures
	assert.Equal(t, StatusFailed, tsk.Status, "expected task to fail due to circuit breaker")
}

func TestStore_DeleteTask_WithLogs(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	tsk.Status = StatusFailed
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusFailed
	repo.logs[tsk.ID.String()] = []string{"log line 1", "log line 2"}

	err := store.DeleteTask(context.Background(), tsk.ID)
	require.NoError(t, err)

	// Verify task was deleted
	_, ok := repo.tasks[tsk.ID.String()]
	assert.False(t, ok, "expected task to be deleted")

	// Verify logs were deleted
	_, ok = repo.logs[tsk.ID.String()]
	assert.False(t, ok, "expected task logs to be deleted")
}

func TestStore_DeleteTask_RemovesDependencies(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	// Create a task that will be deleted
	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	tsk.Status = StatusFailed
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusFailed

	// Create another task that depends on the first
	dependent := NewTask("repo_123", "dependent", "desc", []string{tsk.ID.String()}, nil, 0, false, false, "", true)
	repo.tasks[dependent.ID.String()] = dependent
	repo.taskStatuses[dependent.ID.String()] = StatusPending

	err := store.DeleteTask(context.Background(), tsk.ID)
	require.NoError(t, err)

	// Verify the dependency was removed from the dependent task
	assert.Empty(t, dependent.DependsOn, "expected dependency to be removed")
}

func TestStore_SetAgentStatus_MergesFilesAcrossRetries(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	tsk.Status = StatusRunning
	tsk.AgentStatus = `{"files_modified":["main.go","config.go"],"tests_status":"fail","confidence":"medium"}`
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusRunning

	// Simulate retry attempt reporting new agent status with only the files it changed
	newStatus := `{"files_modified":["main.go","handler.go"],"tests_status":"pass","confidence":"high"}`
	err := store.SetAgentStatus(context.Background(), tsk.ID, newStatus)
	require.NoError(t, err)

	// The stored status should have merged files from both attempts
	assert.Contains(t, tsk.AgentStatus, `"main.go"`)
	assert.Contains(t, tsk.AgentStatus, `"handler.go"`)
	assert.Contains(t, tsk.AgentStatus, `"config.go"`)
	// New status fields should take precedence
	assert.Contains(t, tsk.AgentStatus, `"pass"`)
	assert.Contains(t, tsk.AgentStatus, `"high"`)
}

func TestStore_SetAgentStatus_NoPreviousStatus(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	tsk.Status = StatusRunning
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusRunning

	newStatus := `{"files_modified":["main.go"],"tests_status":"pass","confidence":"high"}`
	err := store.SetAgentStatus(context.Background(), tsk.ID, newStatus)
	require.NoError(t, err)

	assert.Equal(t, newStatus, tsk.AgentStatus)
}

func TestStore_SetAgentStatus_EmptyNewFiles(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	tsk.Status = StatusRunning
	tsk.AgentStatus = `{"files_modified":["main.go","config.go"],"tests_status":"fail"}`
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusRunning

	// New status has empty files_modified — old files should still be merged in
	newStatus := `{"files_modified":[],"tests_status":"pass","confidence":"high"}`
	err := store.SetAgentStatus(context.Background(), tsk.ID, newStatus)
	require.NoError(t, err)

	assert.Contains(t, tsk.AgentStatus, `"main.go"`)
	assert.Contains(t, tsk.AgentStatus, `"config.go"`)
}

func TestStore_SetAgentStatus_InvalidJSON(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
	tsk.Status = StatusRunning
	tsk.AgentStatus = `not valid json`
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusRunning

	// Invalid old JSON should not break — new status used as-is
	newStatus := `{"files_modified":["main.go"],"tests_status":"pass"}`
	err := store.SetAgentStatus(context.Background(), tsk.ID, newStatus)
	require.NoError(t, err)

	assert.Equal(t, newStatus, tsk.AgentStatus)
}

func TestMergeAgentStatusFiles(t *testing.T) {
	tests := []struct {
		name      string
		oldStatus string
		newStatus string
		wantFiles []string
	}{
		{
			name:      "merges unique files from old and new",
			oldStatus: `{"files_modified":["a.go","b.go"],"tests_status":"fail"}`,
			newStatus: `{"files_modified":["b.go","c.go"],"tests_status":"pass"}`,
			wantFiles: []string{"b.go", "c.go", "a.go"},
		},
		{
			name:      "no old files",
			oldStatus: `{"files_modified":[],"tests_status":"fail"}`,
			newStatus: `{"files_modified":["a.go"],"tests_status":"pass"}`,
			wantFiles: []string{"a.go"},
		},
		{
			name:      "no old status",
			oldStatus: "",
			newStatus: `{"files_modified":["a.go"],"tests_status":"pass"}`,
			wantFiles: []string{"a.go"},
		},
		{
			name:      "identical files",
			oldStatus: `{"files_modified":["a.go","b.go"]}`,
			newStatus: `{"files_modified":["a.go","b.go"]}`,
			wantFiles: []string{"a.go", "b.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepo()
			tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, false, "", true)
			tsk.AgentStatus = tt.oldStatus
			repo.tasks[tsk.ID.String()] = tsk

			result := mergeAgentStatusFiles(context.Background(), repo, tsk.ID, tt.newStatus)

			// Parse result and check files_modified
			var parsed map[string]interface{}
			err := json.Unmarshal([]byte(result), &parsed)
			require.NoError(t, err)

			filesRaw, ok := parsed["files_modified"].([]interface{})
			require.True(t, ok)
			var files []string
			for _, f := range filesRaw {
				files = append(files, f.(string))
			}
			assert.Equal(t, tt.wantFiles, files)
		})
	}
}
