-- name: CreateRepo :exec
INSERT INTO repo (id, owner, name, full_name, created_at)
VALUES ($1, $2, $3, $4, $5);

-- name: ReadRepo :one
SELECT * FROM repo WHERE id = $1;

-- name: ReadRepoByFullName :one
SELECT * FROM repo WHERE full_name = $1;

-- name: ListRepos :many
SELECT * FROM repo ORDER BY created_at DESC;

-- name: DeleteRepo :exec
DELETE FROM repo WHERE id = $1;

-- name: UpdateRepoSetupScan :exec
UPDATE repo
SET summary = $2,
    tech_stack = $3,
    has_code = $4,
    has_claude_md = $5,
    has_readme = $6,
    setup_status = $7
WHERE id = $1;

-- name: UpdateRepoSetupStatus :exec
UPDATE repo
SET setup_status = $2
WHERE id = $1;

-- name: UpdateRepoExpectations :exec
UPDATE repo
SET expectations = $2,
    setup_completed_at = $3
WHERE id = $1;

-- name: UpdateRepoSummary :exec
UPDATE repo
SET summary = $2
WHERE id = $1;

-- name: ListReposBySetupStatus :many
SELECT * FROM repo WHERE setup_status = $1 ORDER BY created_at DESC;
