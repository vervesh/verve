-- name: CreateRepo :exec
INSERT INTO repo (id, owner, name, full_name, created_at)
VALUES (?, ?, ?, ?, ?);

-- name: ReadRepo :one
SELECT * FROM repo WHERE id = ?;

-- name: ReadRepoByFullName :one
SELECT * FROM repo WHERE full_name = ?;

-- name: ListRepos :many
SELECT * FROM repo ORDER BY created_at DESC;

-- name: DeleteRepo :exec
DELETE FROM repo WHERE id = ?;

-- name: UpdateRepoSetupScan :exec
UPDATE repo
SET summary = ?,
    tech_stack = ?,
    has_code = ?,
    has_claude_md = ?,
    has_readme = ?,
    setup_status = ?
WHERE id = ?;

-- name: UpdateRepoSetupStatus :exec
UPDATE repo
SET setup_status = ?
WHERE id = ?;

-- name: UpdateRepoExpectations :exec
UPDATE repo
SET expectations = ?,
    setup_completed_at = ?
WHERE id = ?;

-- name: UpdateRepoSummary :exec
UPDATE repo
SET summary = ?
WHERE id = ?;

-- name: UpdateRepoTechStack :exec
UPDATE repo
SET tech_stack = ?
WHERE id = ?;

-- name: ListReposBySetupStatus :many
SELECT * FROM repo WHERE setup_status = ? ORDER BY created_at DESC;
