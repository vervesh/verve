-- name: CreateConversation :exec
INSERT INTO conversation (id, repo_id, title, status, messages, model, pending_message, epic_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: ReadConversation :one
SELECT * FROM conversation WHERE id = $1;

-- name: ListConversationsByRepo :many
SELECT * FROM conversation WHERE repo_id = $1 ORDER BY created_at DESC;

-- name: UpdateConversationStatus :exec
UPDATE conversation SET status = $2, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1;

-- name: AppendConversationMessage :exec
UPDATE conversation SET messages = messages || $2::jsonb, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1;

-- name: SetConversationMessages :exec
UPDATE conversation SET messages = $2, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1;

-- name: SetPendingMessage :exec
UPDATE conversation SET pending_message = $2, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1;

-- name: SetConversationEpicID :exec
UPDATE conversation SET epic_id = $2, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1;

-- name: DeleteConversation :exec
DELETE FROM conversation WHERE id = $1;

-- name: ListPendingConversations :many
SELECT * FROM conversation
WHERE status = 'active' AND pending_message IS NOT NULL AND claimed_at IS NULL
ORDER BY created_at ASC;

-- name: ClaimConversation :execrows
UPDATE conversation SET
  claimed_at = EXTRACT(EPOCH FROM NOW())::BIGINT,
  last_heartbeat_at = EXTRACT(EPOCH FROM NOW())::BIGINT,
  updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1 AND status = 'active' AND pending_message IS NOT NULL AND claimed_at IS NULL;

-- name: ConversationHeartbeat :exec
UPDATE conversation SET
  last_heartbeat_at = EXTRACT(EPOCH FROM NOW())::BIGINT,
  updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1;

-- name: ReleaseConversationClaim :exec
UPDATE conversation SET
  claimed_at = NULL,
  last_heartbeat_at = NULL,
  updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1;

-- name: ListStaleConversations :many
SELECT * FROM conversation
WHERE claimed_at IS NOT NULL
  AND last_heartbeat_at < $1
  AND status = 'active'
ORDER BY last_heartbeat_at ASC;

-- name: ListActiveConversations :many
SELECT * FROM conversation
WHERE status = 'active'
ORDER BY created_at ASC;
