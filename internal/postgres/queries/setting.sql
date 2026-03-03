-- name: UpsertSetting :exec
INSERT INTO setting (key, value, updated_at) VALUES ($1, $2, EXTRACT(EPOCH FROM NOW())::BIGINT)
ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT;

-- name: ReadSetting :one
SELECT value FROM setting WHERE key = $1;

-- name: DeleteSetting :exec
DELETE FROM setting WHERE key = $1;

-- name: ListSettings :many
SELECT key, value FROM setting;
