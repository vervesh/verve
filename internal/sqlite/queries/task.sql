-- name: CreateTask :exec
INSERT INTO task (id, repo_id, type, title, description, status, depends_on, attempt, max_attempts, acceptance_criteria_list, max_cost_usd, skip_pr, draft_pr, model, ready, epic_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ReadTask :one
SELECT * FROM task WHERE id = ?;

-- name: ListTasks :many
SELECT * FROM task WHERE type = 'task' ORDER BY created_at DESC;

-- name: ListTasksByRepo :many
SELECT * FROM task WHERE repo_id = ? AND type = 'task' ORDER BY created_at DESC;

-- name: ListPendingTasks :many
SELECT * FROM task WHERE status = 'pending' AND ready = 1 ORDER BY created_at ASC;

-- name: AppendTaskLogs :exec
INSERT INTO task_log (task_id, attempt, lines) VALUES (?, ?, ?);

-- name: ReadTaskLogs :many
SELECT attempt, lines FROM task_log WHERE task_id = ? ORDER BY id;

-- name: UpdateTaskStatus :exec
UPDATE task SET status = ?, updated_at = unixepoch()
WHERE id = ?;

-- name: SetTaskPullRequest :exec
UPDATE task SET pull_request_url = ?, pr_number = ?, status = 'review', updated_at = unixepoch()
WHERE id = ?;

-- name: ListTasksInReview :many
SELECT * FROM task WHERE status = 'review';

-- name: ListTasksInReviewByRepo :many
SELECT * FROM task WHERE repo_id = ? AND status = 'review';

-- name: CloseTask :exec
UPDATE task SET status = 'closed', close_reason = ?, updated_at = unixepoch()
WHERE id = ?;

-- name: TaskExists :one
SELECT EXISTS(SELECT 1 FROM task WHERE id = ?);

-- name: ReadTaskStatus :one
SELECT status FROM task WHERE id = ?;

-- name: ClaimTask :execrows
UPDATE task SET status = 'running', started_at = unixepoch(), updated_at = unixepoch()
WHERE id = ? AND status = 'pending' AND ready = 1;

-- name: HasTasksForRepo :one
SELECT EXISTS(SELECT 1 FROM task WHERE repo_id = ?);

-- name: RetryTask :execrows
UPDATE task SET status = 'pending', attempt = attempt + 1, retry_reason = ?, started_at = NULL, updated_at = unixepoch()
WHERE id = ? AND status = 'review';

-- name: SetAgentStatus :exec
UPDATE task SET agent_status = ?, updated_at = unixepoch() WHERE id = ?;

-- name: SetRetryContext :exec
UPDATE task SET retry_context = ?, updated_at = unixepoch() WHERE id = ?;

-- name: AddTaskCost :exec
UPDATE task SET cost_usd = cost_usd + ?, updated_at = unixepoch() WHERE id = ?;

-- name: SetConsecutiveFailures :exec
UPDATE task SET consecutive_failures = ?, updated_at = unixepoch() WHERE id = ?;

-- name: SetCloseReason :exec
UPDATE task SET close_reason = ?, updated_at = unixepoch() WHERE id = ?;

-- name: SetBranchName :exec
UPDATE task SET branch_name = ?, status = 'review', updated_at = unixepoch() WHERE id = ?;

-- name: ListTasksInReviewNoPR :many
SELECT * FROM task WHERE status = 'review' AND branch_name IS NOT NULL AND pr_number IS NULL;

-- name: ManualRetryTask :execrows
UPDATE task SET status = 'pending', attempt = attempt + 1,
  retry_reason = ?, retry_context = NULL,
  close_reason = NULL, consecutive_failures = 0,
  started_at = NULL, updated_at = unixepoch()
WHERE id = ? AND status = 'failed';

-- name: FeedbackRetryTask :execrows
UPDATE task SET status = 'pending', attempt = attempt + 1,
  max_attempts = max_attempts + 1,
  retry_reason = ?, retry_context = NULL,
  consecutive_failures = 0,
  started_at = NULL, updated_at = unixepoch()
WHERE id = ? AND status = 'review';

-- name: DeleteTaskLogs :exec
DELETE FROM task_log WHERE task_id = ?;

-- name: DeleteTask :exec
DELETE FROM task WHERE id = ?;

-- name: SetDependsOn :exec
UPDATE task SET depends_on = ?, updated_at = unixepoch()
WHERE id = ?;

-- name: SetReady :exec
UPDATE task SET ready = ?, updated_at = unixepoch()
WHERE id = ?;

-- name: UpdatePendingTask :execrows
UPDATE task SET
  title = ?,
  description = ?,
  depends_on = ?,
  acceptance_criteria_list = ?,
  max_cost_usd = ?,
  skip_pr = ?,
  draft_pr = ?,
  model = ?,
  ready = ?,
  updated_at = unixepoch()
WHERE id = ? AND status = 'pending';

-- name: ScheduleRetryFromRunning :execrows
UPDATE task SET status = 'pending', attempt = attempt + 1, retry_reason = ?, started_at = NULL, updated_at = unixepoch()
WHERE id = ? AND status = 'running';

-- name: StartOverTask :execrows
UPDATE task SET
  status = 'pending',
  title = ?,
  description = ?,
  acceptance_criteria_list = ?,
  attempt = 1,
  max_attempts = 5,
  retry_reason = NULL,
  retry_context = NULL,
  close_reason = NULL,
  agent_status = NULL,
  consecutive_failures = 0,
  cost_usd = 0,
  pull_request_url = NULL,
  pr_number = NULL,
  branch_name = NULL,
  started_at = NULL,
  updated_at = unixepoch()
WHERE id = ? AND status IN ('review', 'failed', 'closed');

-- name: StopTask :execrows
UPDATE task SET status = 'pending', ready = 0, close_reason = ?,
  started_at = NULL, updated_at = unixepoch()
WHERE id = ? AND status = 'running';

-- name: Heartbeat :execrows
UPDATE task SET last_heartbeat_at = unixepoch() WHERE id = ? AND status = 'running';

-- name: ListStaleTasks :many
SELECT * FROM task WHERE status = 'running' AND last_heartbeat_at IS NOT NULL AND last_heartbeat_at < ? ORDER BY started_at;

-- name: ListTasksByEpic :many
SELECT * FROM task WHERE epic_id = ? ORDER BY created_at ASC;

-- name: BulkCloseTasksByEpic :exec
UPDATE task SET status = 'closed', close_reason = ?, updated_at = unixepoch()
WHERE epic_id = ? AND status NOT IN ('closed', 'merged');

-- name: ClearEpicIDForTasks :exec
UPDATE task SET epic_id = NULL, updated_at = unixepoch()
WHERE epic_id = ?;

-- name: BulkDeleteTaskLogsByEpic :exec
DELETE FROM task_log WHERE task_id IN (SELECT id FROM task WHERE epic_id = ?);

-- name: BulkDeleteTasksByEpic :exec
DELETE FROM task WHERE epic_id = ?;

-- name: AssignTaskNumber :one
UPDATE task SET number = (SELECT COALESCE(MAX(t2.number), 0) + 1 FROM task t2 WHERE t2.repo_id = sqlc.arg(repo_id)) WHERE task.id = sqlc.arg(id) RETURNING number;

-- name: ReadTaskByNumber :one
SELECT * FROM task WHERE repo_id = ? AND number = ?;

-- name: DeleteExpiredLogs :execrows
DELETE FROM task_log WHERE created_at < ?;
