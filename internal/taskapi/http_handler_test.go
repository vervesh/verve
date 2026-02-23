package taskapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/joshjon/kit/tx"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"verve/internal/repo"
	"verve/internal/setting"
	"verve/internal/task"
)

// --- Mock task repository ---

type mockTaskRepo struct {
	tasks              map[string]*task.Task
	taskStatuses       map[string]task.Status
	logs               map[string][]string
	mu                 sync.Mutex

	createTaskErr     error
	readTaskErr       error
	taskExistsResult  bool
	taskExistsErr     error
	updateStatusErr   error
	claimTaskResult   bool
	claimTaskErr      error
	closeTaskErr      error
	appendLogsErr     error
	setPullRequestErr error
	manualRetryResult bool
	manualRetryErr    error
	feedbackResult    bool
	feedbackErr       error
	hasTasksResult    bool
	hasTasksErr       error
}

func newMockTaskRepo() *mockTaskRepo {
	return &mockTaskRepo{
		tasks:        make(map[string]*task.Task),
		taskStatuses: make(map[string]task.Status),
		logs:         make(map[string][]string),
	}
}

func (m *mockTaskRepo) CreateTask(_ context.Context, t *task.Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createTaskErr != nil {
		return m.createTaskErr
	}
	m.tasks[t.ID.String()] = t
	m.taskStatuses[t.ID.String()] = t.Status
	return nil
}

func (m *mockTaskRepo) ReadTask(_ context.Context, id task.TaskID) (*task.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readTaskErr != nil {
		return nil, m.readTaskErr
	}
	t, ok := m.tasks[id.String()]
	if !ok {
		return nil, errors.New("not found")
	}
	return t, nil
}

func (m *mockTaskRepo) ListTasks(_ context.Context) ([]*task.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*task.Task
	for _, t := range m.tasks {
		result = append(result, t)
	}
	return result, nil
}

func (m *mockTaskRepo) ListTasksByRepo(_ context.Context, repoID string) ([]*task.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*task.Task
	for _, t := range m.tasks {
		if t.RepoID == repoID {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockTaskRepo) ListPendingTasks(_ context.Context) ([]*task.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*task.Task
	for _, t := range m.tasks {
		if t.Status == task.StatusPending {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockTaskRepo) ListPendingTasksByRepos(_ context.Context, repoIDs []string) ([]*task.Task, error) {
	return nil, nil
}

func (m *mockTaskRepo) AppendTaskLogs(_ context.Context, id task.TaskID, _ int, logs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.appendLogsErr != nil {
		return m.appendLogsErr
	}
	m.logs[id.String()] = append(m.logs[id.String()], logs...)
	return nil
}

func (m *mockTaskRepo) ReadTaskLogs(_ context.Context, id task.TaskID) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.logs[id.String()], nil
}

func (m *mockTaskRepo) StreamTaskLogs(_ context.Context, id task.TaskID, fn func(int, []string) error) error {
	return nil
}

func (m *mockTaskRepo) UpdateTaskStatus(_ context.Context, id task.TaskID, status task.Status) error {
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

func (m *mockTaskRepo) SetTaskPullRequest(_ context.Context, id task.TaskID, prURL string, prNumber int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setPullRequestErr != nil {
		return m.setPullRequestErr
	}
	if t, ok := m.tasks[id.String()]; ok {
		t.PullRequestURL = prURL
		t.PRNumber = prNumber
		t.Status = task.StatusReview
	}
	return nil
}

func (m *mockTaskRepo) ListTasksInReview(_ context.Context) ([]*task.Task, error) {
	return nil, nil
}

func (m *mockTaskRepo) ListTasksInReviewByRepo(_ context.Context, _ string) ([]*task.Task, error) {
	return nil, nil
}

func (m *mockTaskRepo) CloseTask(_ context.Context, id task.TaskID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closeTaskErr != nil {
		return m.closeTaskErr
	}
	if t, ok := m.tasks[id.String()]; ok {
		t.Status = task.StatusClosed
		t.CloseReason = reason
	}
	return nil
}

func (m *mockTaskRepo) TaskExists(_ context.Context, _ task.TaskID) (bool, error) {
	return m.taskExistsResult, m.taskExistsErr
}

func (m *mockTaskRepo) ReadTaskStatus(_ context.Context, id task.TaskID) (task.Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.taskStatuses[id.String()]
	if !ok {
		return "", errors.New("not found")
	}
	return s, nil
}

func (m *mockTaskRepo) ClaimTask(_ context.Context, id task.TaskID) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.claimTaskErr != nil {
		return false, m.claimTaskErr
	}
	if !m.claimTaskResult {
		return false, nil
	}
	if t, ok := m.tasks[id.String()]; ok {
		t.Status = task.StatusRunning
	}
	return true, nil
}

func (m *mockTaskRepo) HasTasksForRepo(_ context.Context, _ string) (bool, error) {
	return m.hasTasksResult, m.hasTasksErr
}

func (m *mockTaskRepo) RetryTask(_ context.Context, _ task.TaskID, _ string) (bool, error) {
	return false, nil
}

func (m *mockTaskRepo) ScheduleRetryFromRunning(_ context.Context, id task.TaskID, reason string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id.String()]
	if !ok || t.Status != task.StatusRunning {
		return false, nil
	}
	t.Status = task.StatusPending
	t.Attempt++
	t.RetryReason = reason
	t.StartedAt = nil
	return true, nil
}

func (m *mockTaskRepo) SetAgentStatus(_ context.Context, _ task.TaskID, _ string) error {
	return nil
}

func (m *mockTaskRepo) SetRetryContext(_ context.Context, _ task.TaskID, _ string) error {
	return nil
}

func (m *mockTaskRepo) AddCost(_ context.Context, _ task.TaskID, _ float64) error {
	return nil
}

func (m *mockTaskRepo) SetConsecutiveFailures(_ context.Context, _ task.TaskID, _ int) error {
	return nil
}

func (m *mockTaskRepo) SetCloseReason(_ context.Context, id task.TaskID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.tasks[id.String()]; ok {
		t.CloseReason = reason
	}
	return nil
}

func (m *mockTaskRepo) SetBranchName(_ context.Context, id task.TaskID, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.tasks[id.String()]; ok {
		t.BranchName = name
	}
	return nil
}

func (m *mockTaskRepo) ListTasksInReviewNoPR(_ context.Context) ([]*task.Task, error) {
	return nil, nil
}

func (m *mockTaskRepo) ManualRetryTask(_ context.Context, _ task.TaskID, _ string) (bool, error) {
	return m.manualRetryResult, m.manualRetryErr
}

func (m *mockTaskRepo) FeedbackRetryTask(_ context.Context, _ task.TaskID, _ string) (bool, error) {
	return m.feedbackResult, m.feedbackErr
}

func (m *mockTaskRepo) DeleteTaskLogs(_ context.Context, id task.TaskID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.logs, id.String())
	return nil
}

func (m *mockTaskRepo) RemoveDependency(_ context.Context, id task.TaskID, depID string) error {
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

func (m *mockTaskRepo) SetReady(_ context.Context, id task.TaskID, ready bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.tasks[id.String()]; ok {
		t.Ready = ready
	}
	return nil
}

func (m *mockTaskRepo) UpdatePendingTask(_ context.Context, id task.TaskID, params task.UpdatePendingTaskParams) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id.String()]
	if !ok {
		return false, errors.New("task not found")
	}
	if t.Status != task.StatusPending {
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

func (m *mockTaskRepo) StartOverTask(_ context.Context, id task.TaskID, params task.StartOverTaskParams) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id.String()]
	if !ok {
		return false, errors.New("task not found")
	}
	if t.Status != task.StatusReview && t.Status != task.StatusFailed {
		return false, nil
	}
	t.Status = task.StatusPending
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

func (m *mockTaskRepo) BeginTxFunc(ctx context.Context, fn func(context.Context, tx.Tx, task.Repository) error) error {
	return fn(ctx, nil, m)
}

func (m *mockTaskRepo) WithTx(_ tx.Tx) task.Repository {
	return m
}

func (m *mockTaskRepo) StopTask(_ context.Context, id task.TaskID, reason string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id.String()]
	if !ok || t.Status != task.StatusRunning {
		return false, nil
	}
	t.Status = task.StatusPending
	t.Ready = false
	t.CloseReason = reason
	m.taskStatuses[id.String()] = task.StatusPending
	return true, nil
}

func (m *mockTaskRepo) Heartbeat(_ context.Context, _ task.TaskID) (bool, error) {
	return true, nil
}

func (m *mockTaskRepo) ListStaleTasks(_ context.Context, _ time.Time) ([]*task.Task, error) {
	return nil, nil
}

func (m *mockTaskRepo) DeleteTask(_ context.Context, id task.TaskID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tasks, id.String())
	delete(m.taskStatuses, id.String())
	delete(m.logs, id.String())
	return nil
}

func (m *mockTaskRepo) ListTasksByEpic(_ context.Context, _ string) ([]*task.Task, error) {
	return nil, nil
}

func (m *mockTaskRepo) BulkCloseTasksByEpic(_ context.Context, _ string, _ string) error {
	return nil
}

func (m *mockTaskRepo) ClearEpicIDForTasks(_ context.Context, _ string) error {
	return nil
}

func (m *mockTaskRepo) BulkDeleteTasksByEpic(_ context.Context, _ string) error {
	return nil
}

func (m *mockTaskRepo) BulkDeleteTasksByIDs(_ context.Context, _ []string) error {
	return nil
}

func (m *mockTaskRepo) DeleteExpiredLogs(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

// --- Mock repo repository ---

type mockRepoRepo struct {
	repos map[string]*repo.Repo
}

func newMockRepoRepo() *mockRepoRepo {
	return &mockRepoRepo{repos: make(map[string]*repo.Repo)}
}

func (m *mockRepoRepo) CreateRepo(_ context.Context, r *repo.Repo) error {
	m.repos[r.ID.String()] = r
	return nil
}

func (m *mockRepoRepo) ReadRepo(_ context.Context, id repo.RepoID) (*repo.Repo, error) {
	r, ok := m.repos[id.String()]
	if !ok {
		return nil, errors.New("not found")
	}
	return r, nil
}

func (m *mockRepoRepo) ReadRepoByFullName(_ context.Context, fullName string) (*repo.Repo, error) {
	for _, r := range m.repos {
		if r.FullName == fullName {
			return r, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockRepoRepo) ListRepos(_ context.Context) ([]*repo.Repo, error) {
	var result []*repo.Repo
	for _, r := range m.repos {
		result = append(result, r)
	}
	return result, nil
}

func (m *mockRepoRepo) DeleteRepo(_ context.Context, id repo.RepoID) error {
	delete(m.repos, id.String())
	return nil
}

// --- Mock setting repository ---

type mockSettingRepo struct {
	settings map[string]string
}

func newMockSettingRepo() *mockSettingRepo {
	return &mockSettingRepo{settings: make(map[string]string)}
}

func (m *mockSettingRepo) UpsertSetting(_ context.Context, key, value string) error {
	m.settings[key] = value
	return nil
}

func (m *mockSettingRepo) ReadSetting(_ context.Context, key string) (string, error) {
	v, ok := m.settings[key]
	if !ok {
		return "", setting.ErrNotFound
	}
	return v, nil
}

func (m *mockSettingRepo) DeleteSetting(_ context.Context, key string) error {
	delete(m.settings, key)
	return nil
}

func (m *mockSettingRepo) ListSettings(_ context.Context) (map[string]string, error) {
	result := make(map[string]string)
	for k, v := range m.settings {
		result[k] = v
	}
	return result, nil
}

// --- Mock task checker for repo store ---

type mockTaskChecker struct {
	hasTasks bool
}

func (m *mockTaskChecker) HasTasksForRepo(_ context.Context, _ string) (bool, error) {
	return m.hasTasks, nil
}

// --- Test helpers ---

func setupHandler() (*HTTPHandler, *mockTaskRepo, *mockRepoRepo, *repo.Repo) {
	taskRepo := newMockTaskRepo()
	broker := task.NewBroker(nil)
	taskStore := task.NewStore(taskRepo, broker)

	repoRepo := newMockRepoRepo()
	checker := &mockTaskChecker{}
	repoStore := repo.NewStore(repoRepo, checker)

	settingRepo := newMockSettingRepo()
	settingService := setting.NewService(settingRepo)

	handler := NewHTTPHandler(taskStore, repoStore, nil, nil, settingService, nil, nil)

	// Pre-create a repo for use in tests
	r, _ := repo.NewRepo("owner/test-repo")
	_ = repoStore.CreateRepo(context.Background(), r)

	return handler, taskRepo, repoRepo, r
}

func newContext(e *echo.Echo, method, path, body string) (echo.Context, *httptest.ResponseRecorder) {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	return c, rec
}

// --- Tests ---

func TestCreateTask_Success(t *testing.T) {
	handler, _, _, testRepo := setupHandler()
	e := echo.New()

	body := `{"title":"Fix bug","description":"Fix the login bug"}`
	c, rec := newContext(e, http.MethodPost, "/repos/"+testRepo.ID.String()+"/tasks", body)
	c.SetParamNames("repo_id")
	c.SetParamValues(testRepo.ID.String())

	err := handler.CreateTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	var created task.Task
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created), "decode response")
	assert.Equal(t, "Fix bug", created.Title)
	assert.Equal(t, task.StatusPending, created.Status)
	assert.Equal(t, "sonnet", created.Model)
}

func TestCreateTask_EmptyTitle(t *testing.T) {
	handler, _, _, testRepo := setupHandler()
	e := echo.New()

	body := `{"title":"","description":"some desc"}`
	c, rec := newContext(e, http.MethodPost, "/repos/"+testRepo.ID.String()+"/tasks", body)
	c.SetParamNames("repo_id")
	c.SetParamValues(testRepo.ID.String())

	err := handler.CreateTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateTask_TitleTooLong(t *testing.T) {
	handler, _, _, testRepo := setupHandler()
	e := echo.New()

	longTitle := strings.Repeat("a", 151)
	body := `{"title":"` + longTitle + `","description":"desc"}`
	c, rec := newContext(e, http.MethodPost, "/repos/"+testRepo.ID.String()+"/tasks", body)
	c.SetParamNames("repo_id")
	c.SetParamValues(testRepo.ID.String())

	err := handler.CreateTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "expected status 400 for long title")
}

func TestCreateTask_InvalidRepoID(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	body := `{"title":"Fix bug"}`
	c, rec := newContext(e, http.MethodPost, "/repos/invalid/tasks", body)
	c.SetParamNames("repo_id")
	c.SetParamValues("invalid")

	err := handler.CreateTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateTask_WithModel(t *testing.T) {
	handler, _, _, testRepo := setupHandler()
	e := echo.New()

	body := `{"title":"Fix bug","description":"desc","model":"opus"}`
	c, rec := newContext(e, http.MethodPost, "/repos/"+testRepo.ID.String()+"/tasks", body)
	c.SetParamNames("repo_id")
	c.SetParamValues(testRepo.ID.String())

	err := handler.CreateTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	var created task.Task
	json.Unmarshal(rec.Body.Bytes(), &created)
	assert.Equal(t, "opus", created.Model)
}

func TestGetTask_Success(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet", true)
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	c, rec := newContext(e, http.MethodGet, "/tasks/"+tsk.ID.String(), "")
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.GetTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var result task.Task
	json.Unmarshal(rec.Body.Bytes(), &result)
	assert.Equal(t, "title", result.Title)
}

func TestGetTask_InvalidID(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	c, rec := newContext(e, http.MethodGet, "/tasks/invalid", "")
	c.SetParamNames("id")
	c.SetParamValues("invalid")

	err := handler.GetTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAppendLogs_Success(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet", true)
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"logs":["line 1","line 2"],"attempt":1}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/logs", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.AppendLogs(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAppendLogs_DefaultAttempt(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet", true)
	taskRepo.tasks[tsk.ID.String()] = tsk

	body := `{"logs":["line 1"]}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/logs", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.AppendLogs(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCompleteTask_Failure(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet", true)
	tsk.Status = task.StatusRunning
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"success":false,"error":"exit code 1"}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/complete", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.CompleteTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, task.StatusFailed, tsk.Status)
}

func TestCompleteTask_FailureWithExistingPR_FailedNotReview(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet", true)
	tsk.Status = task.StatusRunning
	tsk.PRNumber = 10
	tsk.PullRequestURL = "https://github.com/org/repo/pull/10"
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"success":false,"error":"exit code 1"}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/complete", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.CompleteTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, task.StatusFailed, tsk.Status, "expected failed even when task has existing PR")
}

func TestCompleteTask_FailureWithExistingBranch_FailedNotReview(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet", true)
	tsk.Status = task.StatusRunning
	tsk.BranchName = "verve/task-tsk_123"
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"success":false,"error":"exit code 1"}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/complete", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.CompleteTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, task.StatusFailed, tsk.Status, "expected failed even when task has existing branch")
}

func TestCompleteTask_FailureWithPrereqFailed_FailedEvenWithPR(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet", true)
	tsk.Status = task.StatusRunning
	tsk.PRNumber = 10
	tsk.PullRequestURL = "https://github.com/org/repo/pull/10"
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"success":false,"prereq_failed":"missing deps"}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/complete", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.CompleteTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, task.StatusFailed, tsk.Status, "expected failed when prereq_failed is set")
}

func TestCompleteTask_SuccessWithPR(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet", true)
	tsk.Status = task.StatusRunning
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"success":true,"pull_request_url":"https://github.com/org/repo/pull/42","pr_number":42}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/complete", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.CompleteTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, task.StatusReview, tsk.Status)
	assert.Equal(t, 42, tsk.PRNumber)
}

func TestCompleteTask_SuccessWithBranch(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet", true)
	tsk.Status = task.StatusRunning
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"success":true,"branch_name":"verve/task-tsk_123"}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/complete", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.CompleteTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "verve/task-tsk_123", tsk.BranchName)
}

func TestCompleteTask_SuccessNoPR_ClosedIfNoExistingPR(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet", true)
	tsk.Status = task.StatusRunning
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"success":true}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/complete", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.CompleteTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, task.StatusClosed, tsk.Status, "expected status closed (no PR)")
}

func TestCompleteTask_SuccessNoPR_ReviewIfExistingPR(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet", true)
	tsk.Status = task.StatusRunning
	tsk.PRNumber = 10
	tsk.PullRequestURL = "https://github.com/org/repo/pull/10"
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"success":true}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/complete", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.CompleteTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, task.StatusReview, tsk.Status, "expected status review (existing PR)")
}

func TestCloseTask_Success(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet", true)
	tsk.Status = task.StatusRunning
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"reason":"no longer needed"}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/close", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.CloseTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, task.StatusClosed, tsk.Status)
}

func TestListTasksByRepo_Success(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk1 := task.NewTask(testRepo.ID.String(), "task 1", "desc", nil, nil, 0, false, "sonnet", true)
	tsk2 := task.NewTask(testRepo.ID.String(), "task 2", "desc", nil, nil, 0, false, "sonnet", true)
	taskRepo.tasks[tsk1.ID.String()] = tsk1
	taskRepo.tasks[tsk2.ID.String()] = tsk2

	c, rec := newContext(e, http.MethodGet, "/repos/"+testRepo.ID.String()+"/tasks", "")
	c.SetParamNames("repo_id")
	c.SetParamValues(testRepo.ID.String())

	err := handler.ListTasksByRepo(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var tasks []*task.Task
	json.Unmarshal(rec.Body.Bytes(), &tasks)
	assert.Len(t, tasks, 2)
}

func TestAddRepo_Success(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	body := `{"full_name":"newowner/newrepo"}`
	c, rec := newContext(e, http.MethodPost, "/repos", body)

	err := handler.AddRepo(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	var r repo.Repo
	json.Unmarshal(rec.Body.Bytes(), &r)
	assert.Equal(t, "newowner/newrepo", r.FullName)
}

func TestAddRepo_EmptyFullName(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	body := `{"full_name":""}`
	c, rec := newContext(e, http.MethodPost, "/repos", body)

	err := handler.AddRepo(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAddRepo_InvalidFullName(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	body := `{"full_name":"noslash"}`
	c, rec := newContext(e, http.MethodPost, "/repos", body)

	err := handler.AddRepo(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestListRepos(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	c, rec := newContext(e, http.MethodGet, "/repos", "")

	err := handler.ListRepos(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestGetDefaultModel_Default(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	c, rec := newContext(e, http.MethodGet, "/settings/default-model", "")

	err := handler.GetDefaultModel(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp DefaultModelResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.Equal(t, "", resp.Model, "expected empty model when no default explicitly set")
}

func TestSaveDefaultModel(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	body := `{"model":"opus"}`
	c, rec := newContext(e, http.MethodPut, "/settings/default-model", body)

	err := handler.SaveDefaultModel(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestSaveDefaultModel_EmptyModel(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	body := `{"model":""}`
	c, rec := newContext(e, http.MethodPut, "/settings/default-model", body)

	err := handler.SaveDefaultModel(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetGitHubTokenStatus_NotConfigured(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	c, rec := newContext(e, http.MethodGet, "/settings/github-token", "")

	err := handler.GetGitHubTokenStatus(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp GitHubTokenStatusResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.False(t, resp.Configured, "expected configured=false when no token service")
}

func TestSaveGitHubToken_NoService(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	body := `{"token":"ghp_test"}`
	c, rec := newContext(e, http.MethodPut, "/settings/github-token", body)

	err := handler.SaveGitHubToken(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestGetTaskChecks_NoPR(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet", true)
	tsk.PRNumber = 0
	taskRepo.tasks[tsk.ID.String()] = tsk

	c, rec := newContext(e, http.MethodGet, "/tasks/"+tsk.ID.String()+"/checks", "")
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.GetTaskChecks(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp CheckStatusResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.Equal(t, "success", resp.Status, "expected status 'success' for no CI")
}

func TestFeedbackTask_EmptyFeedback(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet", true)
	tsk.Status = task.StatusReview
	taskRepo.tasks[tsk.ID.String()] = tsk

	body := `{"feedback":""}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/feedback", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.FeedbackTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "expected status 400 for empty feedback")
}

func TestRemoveRepo_InvalidID(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	c, rec := newContext(e, http.MethodDelete, "/repos/invalid", "")
	c.SetParamNames("repo_id")
	c.SetParamValues("invalid")

	err := handler.RemoveRepo(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestErrorResponse(t *testing.T) {
	resp := errorResponse("test error")
	assert.Equal(t, "test error", resp["error"])
}

func TestStatusOK(t *testing.T) {
	resp := statusOK()
	assert.Equal(t, "ok", resp["status"])
}

func TestListAvailableRepos_NoGitHubClient(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	c, rec := newContext(e, http.MethodGet, "/repos/available", "")

	err := handler.ListAvailableRepos(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestCompleteTask_WithAgentStatus(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet", true)
	tsk.Status = task.StatusRunning
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"success":false,"agent_status":"{\"confidence\":\"high\"}","cost_usd":1.5}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/complete", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.CompleteTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRemoveDependency_Success(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	dep := task.NewTask(testRepo.ID.String(), "dep", "dep desc", nil, nil, 0, false, "sonnet", true)
	taskRepo.tasks[dep.ID.String()] = dep

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", []string{dep.ID.String()}, nil, 0, false, "sonnet", true)
	taskRepo.tasks[tsk.ID.String()] = tsk

	body := `{"depends_on":"` + dep.ID.String() + `"}`
	c, rec := newContext(e, http.MethodDelete, "/tasks/"+tsk.ID.String()+"/dependency", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.RemoveDependency(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var result task.Task
	json.Unmarshal(rec.Body.Bytes(), &result)
	assert.Empty(t, result.DependsOn)
}

func TestRemoveDependency_InvalidTaskID(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	body := `{"depends_on":"tsk_abc"}`
	c, rec := newContext(e, http.MethodDelete, "/tasks/invalid/dependency", body)
	c.SetParamNames("id")
	c.SetParamValues("invalid")

	err := handler.RemoveDependency(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCompleteTask_SuccessNoChanges_ClosedWithReason(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet", true)
	tsk.Status = task.StatusRunning
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"success":true,"no_changes":true}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/complete", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.CompleteTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, task.StatusClosed, tsk.Status, "expected status closed (no changes needed)")
	assert.Contains(t, tsk.CloseReason, "No changes needed", "expected close reason to mention no changes")
}

func TestRemoveDependency_EmptyDependsOn(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet", true)
	taskRepo.tasks[tsk.ID.String()] = tsk

	body := `{"depends_on":""}`
	c, rec := newContext(e, http.MethodDelete, "/tasks/"+tsk.ID.String()+"/dependency", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.RemoveDependency(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCompleteTask_RetryableFailure_SchedulesRetry(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet", true)
	tsk.Status = task.StatusRunning
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"success":false,"error":"Claude max usage exceeded","retryable":true}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/complete", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.CompleteTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, task.StatusPending, tsk.Status, "expected task to be scheduled for retry")
	assert.Equal(t, 2, tsk.Attempt, "expected attempt to be incremented")
}

func TestCompleteTask_RetryableFailure_MaxAttemptsReached(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet", true)
	tsk.Status = task.StatusRunning
	tsk.Attempt = 5
	tsk.MaxAttempts = 5
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"success":false,"error":"Claude rate limit exceeded","retryable":true}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/complete", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.CompleteTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, task.StatusFailed, tsk.Status, "expected task to fail when max attempts reached")
}

func TestCompleteTask_RetryableWithPrereqFailed_NotRetried(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet", true)
	tsk.Status = task.StatusRunning
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"success":false,"error":"prereq issue","retryable":true,"prereq_failed":"missing deps"}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/complete", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.CompleteTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, task.StatusFailed, tsk.Status, "prereq failures should not be retried even if retryable flag is set")
}

func TestCompleteTask_TransientFailure_SchedulesRetry(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet", true)
	tsk.Status = task.StatusRunning
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"success":false,"error":"failed to create container verve-agent-tsk_123: connection refused","retryable":true}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/complete", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.CompleteTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, task.StatusPending, tsk.Status, "expected task to be scheduled for retry on transient error")
	assert.Equal(t, 2, tsk.Attempt, "expected attempt to be incremented")
	assert.Contains(t, tsk.RetryReason, "transient:", "expected retry reason to have transient category prefix")
}

func TestCompleteTask_NetworkFailure_SchedulesRetry(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet", true)
	tsk.Status = task.StatusRunning
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"success":false,"error":"fatal: unable to access 'https://github.com/org/repo.git/': Could not resolve host: github.com","retryable":true}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/complete", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.CompleteTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, task.StatusPending, tsk.Status, "expected task to be scheduled for retry on network error")
	assert.Equal(t, 2, tsk.Attempt, "expected attempt to be incremented")
	assert.Contains(t, tsk.RetryReason, "transient:", "expected retry reason to have transient category prefix")
}

func TestClassifyRetryReason(t *testing.T) {
	tests := []struct {
		name       string
		errMsg     string
		wantPrefix string
	}{
		{"rate limit", "Claude rate limit exceeded", "rate_limit: "},
		{"max usage", "Claude max usage exceeded", "rate_limit: "},
		{"too many requests", "API returned Too many requests", "rate_limit: "},
		{"overloaded error", "overloaded_error from API", "rate_limit: "},
		{"network error", "Could not resolve host: github.com", "transient: "},
		{"connection refused", "connection refused", "transient: "},
		{"connection timeout", "connection timed out", "transient: "},
		{"DNS failure", "temporary failure in name resolution", "transient: "},
		{"docker create error", "failed to create container: OCI error", "transient: "},
		{"docker start error", "failed to start container: no space left", "transient: "},
		{"unknown retryable", "some unknown error", "transient: "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyRetryReason(tt.errMsg)
			assert.True(t, strings.HasPrefix(result, tt.wantPrefix),
				"expected prefix %q, got %q", tt.wantPrefix, result)
			assert.Contains(t, result, tt.errMsg, "expected result to contain original error message")
		})
	}
}

func TestDeleteTask_FailedWithLogs(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet", true)
	tsk.Status = task.StatusFailed
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status
	taskRepo.logs[tsk.ID.String()] = []string{"error log line 1", "error log line 2"}

	c, rec := newContext(e, http.MethodDelete, "/tasks/"+tsk.ID.String(), "")
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.DeleteTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Verify task was deleted
	_, exists := taskRepo.tasks[tsk.ID.String()]
	assert.False(t, exists, "expected task to be deleted")

	// Verify logs were deleted
	_, logsExist := taskRepo.logs[tsk.ID.String()]
	assert.False(t, logsExist, "expected task logs to be deleted")
}

func TestDeleteTask_InvalidID(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	c, rec := newContext(e, http.MethodDelete, "/tasks/invalid", "")
	c.SetParamNames("id")
	c.SetParamValues("invalid")

	err := handler.DeleteTask(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
