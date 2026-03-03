-- name: CreateEpic :exec
INSERT INTO epic (id, repo_id, title, description, status, proposed_tasks, task_ids, planning_prompt, session_log, not_ready, model, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ReadEpic :one
SELECT * FROM epic WHERE id = ?;

-- name: ListEpics :many
SELECT * FROM epic ORDER BY created_at DESC;

-- name: ListEpicsByRepo :many
SELECT * FROM epic WHERE repo_id = ? ORDER BY created_at DESC;

-- name: UpdateEpic :exec
UPDATE epic SET
  title = ?,
  description = ?,
  status = ?,
  proposed_tasks = ?,
  task_ids = ?,
  planning_prompt = ?,
  session_log = ?,
  not_ready = ?,
  model = ?,
  updated_at = unixepoch()
WHERE id = ?;

-- name: UpdateEpicStatus :exec
UPDATE epic SET status = ?, updated_at = unixepoch()
WHERE id = ?;

-- name: UpdateProposedTasks :exec
UPDATE epic SET proposed_tasks = ?, updated_at = unixepoch()
WHERE id = ?;

-- name: SetEpicTaskIDs :exec
UPDATE epic SET task_ids = ?, updated_at = unixepoch()
WHERE id = ?;

-- name: AppendSessionLog :exec
UPDATE epic SET session_log = ?, updated_at = unixepoch()
WHERE id = ?;

-- name: DeleteEpic :exec
DELETE FROM epic WHERE id = ?;

-- name: ListPlanningEpics :many
SELECT * FROM epic
WHERE status = 'planning' AND claimed_at IS NULL
ORDER BY created_at ASC;

-- name: ClaimEpic :execrows
UPDATE epic SET
  claimed_at = unixepoch(),
  last_heartbeat_at = unixepoch(),
  updated_at = unixepoch()
WHERE id = ? AND status = 'planning' AND claimed_at IS NULL;

-- name: EpicHeartbeat :exec
UPDATE epic SET
  last_heartbeat_at = unixepoch(),
  updated_at = unixepoch()
WHERE id = ?;

-- name: SetEpicFeedback :exec
UPDATE epic SET
  feedback = ?,
  feedback_type = ?,
  updated_at = unixepoch()
WHERE id = ?;

-- name: ClearEpicFeedback :exec
UPDATE epic SET
  feedback = NULL,
  feedback_type = NULL,
  updated_at = unixepoch()
WHERE id = ?;

-- name: ReleaseEpicClaim :exec
UPDATE epic SET
  claimed_at = NULL,
  last_heartbeat_at = NULL,
  status = 'planning',
  updated_at = unixepoch()
WHERE id = ?;

-- name: ListStaleEpics :many
SELECT * FROM epic
WHERE claimed_at IS NOT NULL
  AND last_heartbeat_at < ?
  AND status IN ('planning', 'draft')
ORDER BY last_heartbeat_at ASC;

-- name: ListActiveEpics :many
SELECT * FROM epic
WHERE status = 'active'
ORDER BY created_at ASC;
