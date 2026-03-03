-- name: UpsertSetting :exec
INSERT INTO setting (key, value, updated_at) VALUES (?, ?, unixepoch())
ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = unixepoch();

-- name: ReadSetting :one
SELECT value FROM setting WHERE key = ?;

-- name: DeleteSetting :exec
DELETE FROM setting WHERE key = ?;

-- name: ListSettings :many
SELECT key, value FROM setting;
