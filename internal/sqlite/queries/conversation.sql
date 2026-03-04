-- name: CreateConversation :exec
INSERT INTO conversation (id, repo_id, title, status, messages, model, pending_message, epic_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ReadConversation :one
SELECT * FROM conversation WHERE id = ?;

-- name: ListConversationsByRepo :many
SELECT * FROM conversation WHERE repo_id = ? ORDER BY created_at DESC;

-- name: UpdateConversationStatus :exec
UPDATE conversation SET status = ?, updated_at = unixepoch()
WHERE id = ?;

-- name: SetConversationMessages :exec
UPDATE conversation SET messages = ?, updated_at = unixepoch()
WHERE id = ?;

-- name: SetPendingMessage :exec
UPDATE conversation SET pending_message = ?, updated_at = unixepoch()
WHERE id = ?;

-- name: SetConversationEpicID :exec
UPDATE conversation SET epic_id = ?, updated_at = unixepoch()
WHERE id = ?;

-- name: DeleteConversation :exec
DELETE FROM conversation WHERE id = ?;

-- name: ListPendingConversations :many
SELECT * FROM conversation
WHERE status = 'active' AND pending_message IS NOT NULL AND claimed_at IS NULL
ORDER BY created_at ASC;

-- name: ClaimConversation :execrows
UPDATE conversation SET
  claimed_at = unixepoch(),
  last_heartbeat_at = unixepoch(),
  updated_at = unixepoch()
WHERE id = ? AND status = 'active' AND pending_message IS NOT NULL AND claimed_at IS NULL;

-- name: ConversationHeartbeat :exec
UPDATE conversation SET
  last_heartbeat_at = unixepoch(),
  updated_at = unixepoch()
WHERE id = ?;

-- name: ReleaseConversationClaim :exec
UPDATE conversation SET
  claimed_at = NULL,
  last_heartbeat_at = NULL,
  updated_at = unixepoch()
WHERE id = ?;

-- name: ListStaleConversations :many
SELECT * FROM conversation
WHERE claimed_at IS NOT NULL
  AND last_heartbeat_at < ?
  AND status = 'active'
ORDER BY last_heartbeat_at ASC;

-- name: ListActiveConversations :many
SELECT * FROM conversation
WHERE status = 'active'
ORDER BY created_at ASC;
