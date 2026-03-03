-- name: CreateEpic :exec
INSERT INTO epic (id, repo_id, title, description, status, proposed_tasks, task_ids, planning_prompt, session_log, not_ready, model, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);

-- name: ReadEpic :one
SELECT * FROM epic WHERE id = $1;

-- name: ListEpics :many
SELECT * FROM epic ORDER BY created_at DESC;

-- name: ListEpicsByRepo :many
SELECT * FROM epic WHERE repo_id = $1 ORDER BY created_at DESC;

-- name: UpdateEpic :exec
UPDATE epic SET
  title = $2,
  description = $3,
  status = $4,
  proposed_tasks = $5,
  task_ids = $6,
  planning_prompt = $7,
  session_log = $8,
  not_ready = $9,
  model = $10,
  updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1;

-- name: UpdateEpicStatus :exec
UPDATE epic SET status = $2, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1;

-- name: UpdateProposedTasks :exec
UPDATE epic SET proposed_tasks = $2, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1;

-- name: SetEpicTaskIDs :exec
UPDATE epic SET task_ids = $2, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1;

-- name: AppendSessionLog :exec
UPDATE epic SET session_log = session_log || $2, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1;

-- name: DeleteEpic :exec
DELETE FROM epic WHERE id = $1;

-- name: ListPlanningEpics :many
SELECT * FROM epic
WHERE status = 'planning' AND claimed_at IS NULL
ORDER BY created_at ASC;

-- name: ClaimEpic :execrows
UPDATE epic SET
  claimed_at = EXTRACT(EPOCH FROM NOW())::BIGINT,
  last_heartbeat_at = EXTRACT(EPOCH FROM NOW())::BIGINT,
  updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1 AND status = 'planning' AND claimed_at IS NULL;

-- name: EpicHeartbeat :exec
UPDATE epic SET
  last_heartbeat_at = EXTRACT(EPOCH FROM NOW())::BIGINT,
  updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1;

-- name: SetEpicFeedback :exec
UPDATE epic SET
  feedback = $2,
  feedback_type = $3,
  updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1;

-- name: ClearEpicFeedback :exec
UPDATE epic SET
  feedback = NULL,
  feedback_type = NULL,
  updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1;

-- name: ReleaseEpicClaim :exec
UPDATE epic SET
  claimed_at = NULL,
  last_heartbeat_at = NULL,
  status = 'planning',
  updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1;

-- name: ListStaleEpics :many
SELECT * FROM epic
WHERE claimed_at IS NOT NULL
  AND last_heartbeat_at < $1
  AND status IN ('planning', 'draft')
ORDER BY last_heartbeat_at ASC;

-- name: ListActiveEpics :many
SELECT * FROM epic
WHERE status = 'active'
ORDER BY created_at ASC;

-- name: RemoveEpicTaskID :exec
UPDATE epic SET
  task_ids = array_remove(task_ids, $2),
  updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1;
