-- name: CreateTask :exec
INSERT INTO task (id, repo_id, title, description, status, depends_on, attempt, max_attempts, acceptance_criteria_list, max_cost_usd, skip_pr, draft_pr, model, ready, epic_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17);

-- name: ReadTask :one
SELECT * FROM task WHERE id = $1;

-- name: ListTasks :many
SELECT * FROM task ORDER BY created_at DESC;

-- name: ListTasksByRepo :many
SELECT * FROM task WHERE repo_id = $1 ORDER BY created_at DESC;

-- name: ListPendingTasks :many
SELECT * FROM task WHERE status = 'pending' AND ready = true ORDER BY created_at ASC;

-- name: ListPendingTasksByRepos :many
SELECT * FROM task WHERE status = 'pending' AND ready = true AND repo_id = ANY($1::text[]) ORDER BY created_at ASC;

-- name: AppendTaskLogs :exec
INSERT INTO task_log (task_id, attempt, lines) VALUES (@id, @attempt, @lines);

-- name: ReadTaskLogs :many
SELECT attempt, lines FROM task_log WHERE task_id = @id ORDER BY id;

-- name: UpdateTaskStatus :exec
UPDATE task SET status = $2, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1;

-- name: SetTaskPullRequest :exec
UPDATE task SET pull_request_url = $2, pr_number = $3, status = 'review', updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1;

-- name: ListTasksInReview :many
SELECT * FROM task WHERE status = 'review';

-- name: ListTasksInReviewByRepo :many
SELECT * FROM task WHERE repo_id = $1 AND status = 'review';

-- name: CloseTask :exec
UPDATE task SET status = 'closed', close_reason = $2, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1;

-- name: TaskExists :one
SELECT EXISTS(SELECT 1 FROM task WHERE id = $1);

-- name: ReadTaskStatus :one
SELECT status FROM task WHERE id = $1;

-- name: ClaimTask :execrows
UPDATE task SET status = 'running', started_at = EXTRACT(EPOCH FROM NOW())::BIGINT, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1 AND status = 'pending' AND ready = true;

-- name: HasTasksForRepo :one
SELECT EXISTS(SELECT 1 FROM task WHERE repo_id = $1);

-- name: RetryTask :execrows
UPDATE task SET status = 'pending', attempt = attempt + 1, retry_reason = $2, started_at = NULL, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1 AND status = 'review';

-- name: SetAgentStatus :exec
UPDATE task SET agent_status = $2, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT WHERE id = $1;

-- name: SetRetryContext :exec
UPDATE task SET retry_context = $2, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT WHERE id = $1;

-- name: AddTaskCost :exec
UPDATE task SET cost_usd = cost_usd + $2, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT WHERE id = $1;

-- name: SetConsecutiveFailures :exec
UPDATE task SET consecutive_failures = $2, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT WHERE id = $1;

-- name: SetCloseReason :exec
UPDATE task SET close_reason = $2, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT WHERE id = $1;

-- name: SetBranchName :exec
UPDATE task SET branch_name = $2, status = 'review', updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT WHERE id = $1;

-- name: ListTasksInReviewNoPR :many
SELECT * FROM task WHERE status = 'review' AND branch_name IS NOT NULL AND pr_number IS NULL;

-- name: ManualRetryTask :execrows
UPDATE task SET status = 'pending', attempt = attempt + 1,
  retry_reason = $2, retry_context = NULL,
  close_reason = NULL, consecutive_failures = 0,
  started_at = NULL, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1 AND status = 'failed';

-- name: FeedbackRetryTask :execrows
UPDATE task SET status = 'pending', attempt = attempt + 1,
  max_attempts = max_attempts + 1,
  retry_reason = $2, retry_context = NULL,
  consecutive_failures = 0,
  started_at = NULL, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1 AND status = 'review';

-- name: DeleteTaskLogs :exec
DELETE FROM task_log WHERE task_id = $1;

-- name: DeleteTask :exec
DELETE FROM task WHERE id = $1;

-- name: RemoveDependency :exec
UPDATE task SET depends_on = array_remove(depends_on, $2), updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1;

-- name: SetReady :exec
UPDATE task SET ready = $2, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT WHERE id = $1;

-- name: UpdatePendingTask :execrows
UPDATE task SET
  title = $2,
  description = $3,
  depends_on = $4,
  acceptance_criteria_list = $5,
  max_cost_usd = $6,
  skip_pr = $7,
  draft_pr = $8,
  model = $9,
  ready = $10,
  updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1 AND status = 'pending';

-- name: ScheduleRetryFromRunning :execrows
UPDATE task SET status = 'pending', attempt = attempt + 1, retry_reason = $2, started_at = NULL, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1 AND status = 'running';

-- name: StartOverTask :execrows
UPDATE task SET
  status = 'pending',
  title = $2,
  description = $3,
  acceptance_criteria_list = $4,
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
  updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1 AND status IN ('review', 'failed', 'closed');

-- name: StopTask :execrows
UPDATE task SET status = 'pending', ready = false, close_reason = $2,
  started_at = NULL, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1 AND status = 'running';

-- name: Heartbeat :execrows
UPDATE task SET last_heartbeat_at = EXTRACT(EPOCH FROM NOW())::BIGINT WHERE id = $1 AND status = 'running';

-- name: ListStaleTasks :many
SELECT * FROM task WHERE status = 'running' AND last_heartbeat_at IS NOT NULL AND last_heartbeat_at < $1 ORDER BY started_at;

-- name: ListTasksByEpic :many
SELECT * FROM task WHERE epic_id = $1 ORDER BY created_at ASC;

-- name: BulkCloseTasksByEpic :exec
UPDATE task SET status = 'closed', close_reason = $2, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE epic_id = $1 AND status NOT IN ('closed', 'merged');

-- name: ClearEpicIDForTasks :exec
UPDATE task SET epic_id = NULL, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE epic_id = $1;

-- name: BulkDeleteTaskLogsByEpic :exec
DELETE FROM task_log WHERE task_id IN (SELECT id FROM task WHERE epic_id = $1);

-- name: BulkDeleteTasksByEpic :exec
DELETE FROM task WHERE epic_id = $1;

-- name: BulkDeleteTaskLogsByIDs :exec
DELETE FROM task_log WHERE task_id = ANY($1::text[]);

-- name: BulkDeleteTasksByIDs :exec
DELETE FROM task WHERE id = ANY($1::text[]);

-- name: DeleteExpiredLogs :execrows
DELETE FROM task_log WHERE created_at < $1;
