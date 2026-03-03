package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/joshjon/kit/errtag"
	"github.com/joshjon/kit/tx"

	"github.com/joshjon/verve/internal/sqlite/sqlc"
	"github.com/joshjon/verve/internal/task"
)

var _ task.Repository = (*TaskRepository)(nil)

// TaskRepository implements task.Repository using SQLite.
type TaskRepository struct {
	dbtx sqlc.DBTX
	db   *sqlc.Queries
	txer *tx.SQLiteRepositoryTxer[task.Repository]
}

// TaskRepoOption configures a TaskRepository.
type TaskRepoOption func(*tx.SQLiteRepositoryTxerConfig[task.Repository])

// WithNoPragma disables PRAGMA statements inside transactions. Use this for
// SQLite drivers or backends that do not support PRAGMAs (e.g. Turso/libSQL).
func WithNoPragma() TaskRepoOption {
	return func(cfg *tx.SQLiteRepositoryTxerConfig[task.Repository]) {
		cfg.NoPragma = true
	}
}

// NewTaskRepository creates a new TaskRepository backed by the given SQLite DB.
func NewTaskRepository(db DB, opts ...TaskRepoOption) *TaskRepository {
	cfg := tx.SQLiteRepositoryTxerConfig[task.Repository]{
		Timeout: tx.DefaultTimeout,
		WithTxFunc: func(repo task.Repository, txer *tx.SQLiteRepositoryTxer[task.Repository], sqlTx *sql.Tx) task.Repository {
			cpy := *repo.(*TaskRepository)
			cpy.dbtx = sqlTx
			cpy.db = cpy.db.WithTx(sqlTx)
			cpy.txer = txer
			return task.Repository(&cpy)
		},
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &TaskRepository{
		dbtx: db,
		db:   sqlc.New(db),
		txer: tx.NewSQLiteRepositoryTxer(db, cfg),
	}
}

func (r *TaskRepository) CreateTask(ctx context.Context, t *task.Task) error {
	var maxCostUSD *float64
	if t.MaxCostUSD > 0 {
		maxCostUSD = &t.MaxCostUSD
	}
	var skipPR int64
	if t.SkipPR {
		skipPR = 1
	}
	var draftPR int64
	if t.DraftPR {
		draftPR = 1
	}
	var ready int64
	if t.Ready {
		ready = 1
	}
	var model *string
	if t.Model != "" {
		model = &t.Model
	}
	var epicID *string
	if t.EpicID != "" {
		epicID = &t.EpicID
	}
	err := r.db.CreateTask(ctx, sqlc.CreateTaskParams{
		ID:                    t.ID.String(),
		RepoID:                t.RepoID,
		Title:                 t.Title,
		Description:           t.Description,
		Status:                string(t.Status),
		DependsOn:             marshalJSONStrings(t.DependsOn),
		Attempt:               int64(t.Attempt),
		MaxAttempts:           int64(t.MaxAttempts),
		AcceptanceCriteriaList: marshalJSONStrings(t.AcceptanceCriteria),
		MaxCostUsd:            maxCostUSD,
		SkipPr:                skipPR,
		DraftPr:               draftPR,
		Model:                 model,
		Ready:                 ready,
		EpicID:                epicID,
		CreatedAt:             t.CreatedAt.Unix(),
		UpdatedAt:             t.UpdatedAt.Unix(),
	})
	return tagTaskErr(err)
}

func (r *TaskRepository) ReadTask(ctx context.Context, id task.TaskID) (*task.Task, error) {
	row, err := r.db.ReadTask(ctx, id.String())
	if err != nil {
		return nil, tagTaskErr(err)
	}
	return unmarshalTask(row), nil
}

func (r *TaskRepository) ListTasks(ctx context.Context) ([]*task.Task, error) {
	rows, err := r.db.ListTasks(ctx)
	if err != nil {
		return nil, err
	}
	return unmarshalTaskList(rows), nil
}

func (r *TaskRepository) ListTasksByRepo(ctx context.Context, repoID string) ([]*task.Task, error) {
	rows, err := r.db.ListTasksByRepo(ctx, repoID)
	if err != nil {
		return nil, err
	}
	return unmarshalTaskList(rows), nil
}

func (r *TaskRepository) ListPendingTasks(ctx context.Context) ([]*task.Task, error) {
	rows, err := r.db.ListPendingTasks(ctx)
	if err != nil {
		return nil, err
	}
	return unmarshalTaskList(rows), nil
}

// ListPendingTasksByRepos returns pending tasks filtered by repo IDs.
// SQLite doesn't support ANY($1::text[]), so we build the query dynamically.
func (r *TaskRepository) ListPendingTasksByRepos(ctx context.Context, repoIDs []string) ([]*task.Task, error) {
	if len(repoIDs) == 0 {
		return nil, nil
	}
	query := "SELECT id, repo_id, title, description, status, pull_request_url, pr_number, depends_on, close_reason, attempt, max_attempts, retry_reason, acceptance_criteria_list, agent_status, retry_context, consecutive_failures, cost_usd, max_cost_usd, skip_pr, draft_pr, branch_name, model, started_at, ready, last_heartbeat_at, epic_id, created_at, updated_at FROM task WHERE status = 'pending' AND ready = 1 AND repo_id IN (?" + strings.Repeat(",?", len(repoIDs)-1) + ") ORDER BY created_at ASC"
	args := make([]any, len(repoIDs))
	for i, id := range repoIDs {
		args[i] = id
	}
	rows, err := r.dbtx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var tasks []*task.Task
	for rows.Next() {
		var t sqlc.Task
		if err := rows.Scan(&t.ID, &t.RepoID, &t.Title, &t.Description, &t.Status, &t.PullRequestUrl, &t.PrNumber, &t.DependsOn, &t.CloseReason, &t.Attempt, &t.MaxAttempts, &t.RetryReason, &t.AcceptanceCriteriaList, &t.AgentStatus, &t.RetryContext, &t.ConsecutiveFailures, &t.CostUsd, &t.MaxCostUsd, &t.SkipPr, &t.DraftPr, &t.BranchName, &t.Model, &t.StartedAt, &t.Ready, &t.LastHeartbeatAt, &t.EpicID, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, unmarshalTask(&t))
	}
	return tasks, rows.Err()
}

func (r *TaskRepository) AppendTaskLogs(ctx context.Context, id task.TaskID, attempt int, logs []string) error {
	return tagTaskErr(r.db.AppendTaskLogs(ctx, sqlc.AppendTaskLogsParams{
		TaskID:  id.String(),
		Attempt: int64(attempt),
		Lines:   marshalJSONStrings(logs),
	}))
}

func (r *TaskRepository) ReadTaskLogs(ctx context.Context, id task.TaskID) ([]string, error) {
	batches, err := r.db.ReadTaskLogs(ctx, id.String())
	if err != nil {
		return nil, tagTaskErr(err)
	}
	var logs []string
	for _, batch := range batches {
		logs = append(logs, unmarshalJSONStrings(batch.Lines)...)
	}
	if logs == nil {
		logs = []string{}
	}
	return logs, nil
}

func (r *TaskRepository) StreamTaskLogs(ctx context.Context, id task.TaskID, fn func(attempt int, lines []string) error) error {
	rows, err := r.dbtx.QueryContext(ctx, "SELECT attempt, lines FROM task_log WHERE task_id = ? ORDER BY id", id.String())
	if err != nil {
		return tagTaskErr(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var attempt int
		var linesJSON string
		if err := rows.Scan(&attempt, &linesJSON); err != nil {
			return err
		}
		if err := fn(attempt, unmarshalJSONStrings(linesJSON)); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (r *TaskRepository) UpdateTaskStatus(ctx context.Context, id task.TaskID, status task.Status) error {
	return tagTaskErr(r.db.UpdateTaskStatus(ctx, sqlc.UpdateTaskStatusParams{
		ID:     id.String(),
		Status: string(status),
	}))
}

func (r *TaskRepository) SetTaskPullRequest(ctx context.Context, id task.TaskID, prURL string, prNumber int) error {
	return tagTaskErr(r.db.SetTaskPullRequest(ctx, sqlc.SetTaskPullRequestParams{
		ID:             id.String(),
		PullRequestUrl: &prURL,
		PrNumber:       ptr(int64(prNumber)),
	}))
}

func (r *TaskRepository) ListTasksInReview(ctx context.Context) ([]*task.Task, error) {
	rows, err := r.db.ListTasksInReview(ctx)
	if err != nil {
		return nil, err
	}
	return unmarshalTaskList(rows), nil
}

func (r *TaskRepository) ListTasksInReviewByRepo(ctx context.Context, repoID string) ([]*task.Task, error) {
	rows, err := r.db.ListTasksInReviewByRepo(ctx, repoID)
	if err != nil {
		return nil, err
	}
	return unmarshalTaskList(rows), nil
}

func (r *TaskRepository) HasTasksForRepo(ctx context.Context, repoID string) (bool, error) {
	result, err := r.db.HasTasksForRepo(ctx, repoID)
	if err != nil {
		return false, err
	}
	return result != 0, nil
}

func (r *TaskRepository) CloseTask(ctx context.Context, id task.TaskID, reason string) error {
	return tagTaskErr(r.db.CloseTask(ctx, sqlc.CloseTaskParams{
		ID:          id.String(),
		CloseReason: &reason,
	}))
}

func (r *TaskRepository) TaskExists(ctx context.Context, id task.TaskID) (bool, error) {
	result, err := r.db.TaskExists(ctx, id.String())
	if err != nil {
		return false, err
	}
	return result != 0, nil
}

func (r *TaskRepository) ReadTaskStatus(ctx context.Context, id task.TaskID) (task.Status, error) {
	status, err := r.db.ReadTaskStatus(ctx, id.String())
	if err != nil {
		return "", tagTaskErr(err)
	}
	return task.Status(status), nil
}

func (r *TaskRepository) ClaimTask(ctx context.Context, id task.TaskID) (bool, error) {
	rows, err := r.db.ClaimTask(ctx, id.String())
	return rows > 0, err
}

func (r *TaskRepository) RetryTask(ctx context.Context, id task.TaskID, reason string) (bool, error) {
	rows, err := r.db.RetryTask(ctx, sqlc.RetryTaskParams{
		RetryReason: &reason,
		ID:          id.String(),
	})
	return rows > 0, tagTaskErr(err)
}

func (r *TaskRepository) ScheduleRetryFromRunning(ctx context.Context, id task.TaskID, reason string) (bool, error) {
	rows, err := r.db.ScheduleRetryFromRunning(ctx, sqlc.ScheduleRetryFromRunningParams{
		RetryReason: &reason,
		ID:          id.String(),
	})
	return rows > 0, tagTaskErr(err)
}

func (r *TaskRepository) SetAgentStatus(ctx context.Context, id task.TaskID, status string) error {
	return tagTaskErr(r.db.SetAgentStatus(ctx, sqlc.SetAgentStatusParams{
		AgentStatus: &status,
		ID:          id.String(),
	}))
}

func (r *TaskRepository) SetRetryContext(ctx context.Context, id task.TaskID, retryCtx string) error {
	return tagTaskErr(r.db.SetRetryContext(ctx, sqlc.SetRetryContextParams{
		RetryContext: &retryCtx,
		ID:           id.String(),
	}))
}

func (r *TaskRepository) AddCost(ctx context.Context, id task.TaskID, costUSD float64) error {
	return tagTaskErr(r.db.AddTaskCost(ctx, sqlc.AddTaskCostParams{
		CostUsd: costUSD,
		ID:      id.String(),
	}))
}

func (r *TaskRepository) SetConsecutiveFailures(ctx context.Context, id task.TaskID, count int) error {
	return tagTaskErr(r.db.SetConsecutiveFailures(ctx, sqlc.SetConsecutiveFailuresParams{
		ConsecutiveFailures: int64(count),
		ID:                  id.String(),
	}))
}

func (r *TaskRepository) SetCloseReason(ctx context.Context, id task.TaskID, reason string) error {
	return tagTaskErr(r.db.SetCloseReason(ctx, sqlc.SetCloseReasonParams{
		CloseReason: &reason,
		ID:          id.String(),
	}))
}

func (r *TaskRepository) SetBranchName(ctx context.Context, id task.TaskID, branchName string) error {
	return tagTaskErr(r.db.SetBranchName(ctx, sqlc.SetBranchNameParams{
		BranchName: &branchName,
		ID:         id.String(),
	}))
}

func (r *TaskRepository) ManualRetryTask(ctx context.Context, id task.TaskID, instructions string) (bool, error) {
	var reason *string
	if instructions != "" {
		reason = &instructions
	}
	rows, err := r.db.ManualRetryTask(ctx, sqlc.ManualRetryTaskParams{
		RetryReason: reason,
		ID:          id.String(),
	})
	return rows > 0, tagTaskErr(err)
}

func (r *TaskRepository) FeedbackRetryTask(ctx context.Context, id task.TaskID, feedback string) (bool, error) {
	var reason *string
	if feedback != "" {
		reason = &feedback
	}
	rows, err := r.db.FeedbackRetryTask(ctx, sqlc.FeedbackRetryTaskParams{
		RetryReason: reason,
		ID:          id.String(),
	})
	return rows > 0, tagTaskErr(err)
}

func (r *TaskRepository) DeleteTaskLogs(ctx context.Context, id task.TaskID) error {
	return tagTaskErr(r.db.DeleteTaskLogs(ctx, id.String()))
}

func (r *TaskRepository) RemoveDependency(ctx context.Context, id task.TaskID, depID string) error {
	t, err := r.ReadTask(ctx, id)
	if err != nil {
		return err
	}
	filtered := make([]string, 0, len(t.DependsOn))
	for _, d := range t.DependsOn {
		if d != depID {
			filtered = append(filtered, d)
		}
	}
	return tagTaskErr(r.db.SetDependsOn(ctx, sqlc.SetDependsOnParams{
		DependsOn: marshalJSONStrings(filtered),
		ID:        id.String(),
	}))
}

func (r *TaskRepository) SetReady(ctx context.Context, id task.TaskID, ready bool) error {
	var readyInt int64
	if ready {
		readyInt = 1
	}
	return tagTaskErr(r.db.SetReady(ctx, sqlc.SetReadyParams{
		Ready: readyInt,
		ID:    id.String(),
	}))
}

func (r *TaskRepository) UpdatePendingTask(ctx context.Context, id task.TaskID, params task.UpdatePendingTaskParams) (bool, error) {
	var maxCostUSD *float64
	if params.MaxCostUSD > 0 {
		maxCostUSD = &params.MaxCostUSD
	}
	var skipPR int64
	if params.SkipPR {
		skipPR = 1
	}
	var draftPR int64
	if params.DraftPR {
		draftPR = 1
	}
	var ready int64
	if params.Ready {
		ready = 1
	}
	var model *string
	if params.Model != "" {
		model = &params.Model
	}
	rows, err := r.db.UpdatePendingTask(ctx, sqlc.UpdatePendingTaskParams{
		Title:                  params.Title,
		Description:            params.Description,
		DependsOn:              marshalJSONStrings(params.DependsOn),
		AcceptanceCriteriaList: marshalJSONStrings(params.AcceptanceCriteria),
		MaxCostUsd:             maxCostUSD,
		SkipPr:                 skipPR,
		DraftPr:                draftPR,
		Model:                  model,
		Ready:                  ready,
		ID:                     id.String(),
	})
	return rows > 0, tagTaskErr(err)
}

func (r *TaskRepository) StartOverTask(ctx context.Context, id task.TaskID, params task.StartOverTaskParams) (bool, error) {
	rows, err := r.db.StartOverTask(ctx, sqlc.StartOverTaskParams{
		Title:                  params.Title,
		Description:            params.Description,
		AcceptanceCriteriaList: marshalJSONStrings(params.AcceptanceCriteria),
		ID:                     id.String(),
	})
	return rows > 0, tagTaskErr(err)
}

func (r *TaskRepository) StopTask(ctx context.Context, id task.TaskID, reason string) (bool, error) {
	rows, err := r.db.StopTask(ctx, sqlc.StopTaskParams{
		CloseReason: &reason,
		ID:          id.String(),
	})
	return rows > 0, tagTaskErr(err)
}

func (r *TaskRepository) Heartbeat(ctx context.Context, id task.TaskID) (bool, error) {
	rows, err := r.db.Heartbeat(ctx, id.String())
	return rows > 0, tagTaskErr(err)
}

func (r *TaskRepository) ListStaleTasks(ctx context.Context, before time.Time) ([]*task.Task, error) {
	beforeUnix := before.Unix()
	rows, err := r.db.ListStaleTasks(ctx, &beforeUnix)
	if err != nil {
		return nil, err
	}
	return unmarshalTaskList(rows), nil
}

func (r *TaskRepository) DeleteTask(ctx context.Context, id task.TaskID) error {
	return tagTaskErr(r.db.DeleteTask(ctx, id.String()))
}

func (r *TaskRepository) ListTasksByEpic(ctx context.Context, epicID string) ([]*task.Task, error) {
	rows, err := r.db.ListTasksByEpic(ctx, &epicID)
	if err != nil {
		return nil, err
	}
	return unmarshalTaskList(rows), nil
}

func (r *TaskRepository) BulkCloseTasksByEpic(ctx context.Context, epicID, reason string) error {
	return tagTaskErr(r.db.BulkCloseTasksByEpic(ctx, sqlc.BulkCloseTasksByEpicParams{
		CloseReason: &reason,
		EpicID:      &epicID,
	}))
}

func (r *TaskRepository) ClearEpicIDForTasks(ctx context.Context, epicID string) error {
	return tagTaskErr(r.db.ClearEpicIDForTasks(ctx, &epicID))
}

func (r *TaskRepository) BulkDeleteTasksByEpic(ctx context.Context, epicID string) error {
	// Delete logs first (FK constraint)
	if err := r.db.BulkDeleteTaskLogsByEpic(ctx, &epicID); err != nil {
		return tagTaskErr(err)
	}
	return tagTaskErr(r.db.BulkDeleteTasksByEpic(ctx, &epicID))
}

func (r *TaskRepository) BulkDeleteTasksByIDs(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	// SQLite doesn't support ANY($1::text[]), so we build the query dynamically.
	placeholders := "?" + strings.Repeat(",?", len(ids)-1)
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	// Delete logs first (FK constraint)
	if _, err := r.dbtx.ExecContext(ctx, "DELETE FROM task_log WHERE task_id IN ("+placeholders+")", args...); err != nil {
		return tagTaskErr(err)
	}
	if _, err := r.dbtx.ExecContext(ctx, "DELETE FROM task WHERE id IN ("+placeholders+")", args...); err != nil {
		return tagTaskErr(err)
	}
	return nil
}

func (r *TaskRepository) DeleteExpiredLogs(ctx context.Context, before time.Time) (int64, error) {
	n, err := r.db.DeleteExpiredLogs(ctx, before.Unix())
	return n, tagTaskErr(err)
}

func (r *TaskRepository) ListTasksInReviewNoPR(ctx context.Context) ([]*task.Task, error) {
	rows, err := r.db.ListTasksInReviewNoPR(ctx)
	if err != nil {
		return nil, err
	}
	return unmarshalTaskList(rows), nil
}

func (r *TaskRepository) WithTx(txn tx.Tx) task.Repository {
	return r.txer.WithTx(r, txn)
}

func (r *TaskRepository) BeginTxFunc(ctx context.Context, fn func(ctx context.Context, txn tx.Tx, repo task.Repository) error) error {
	return r.txer.BeginTxFunc(ctx, r, fn)
}

func tagTaskErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return errtag.Tag[task.ErrTagTaskNotFound](err)
	}
	if isSQLiteErrCode(err, sqliteConstraint, sqliteConstraintUnique, sqliteConstraintPrimaryKey) {
		return errtag.Tag[task.ErrTagTaskConflict](err)
	}
	return tx.TagSQLiteTimeoutErr(err)
}
