CREATE TABLE task (
    id                       TEXT PRIMARY KEY,
    repo_id                  TEXT    NOT NULL REFERENCES repo(id),
    title                    TEXT    NOT NULL DEFAULT '',
    description              TEXT    NOT NULL,
    status                   TEXT    NOT NULL DEFAULT 'pending'
                             CHECK(status IN ('pending', 'running', 'review', 'merged', 'closed', 'failed')),
    pull_request_url         TEXT,
    pr_number                INTEGER,
    depends_on               TEXT    NOT NULL DEFAULT '[]',
    close_reason             TEXT,
    attempt                  INTEGER NOT NULL DEFAULT 1,
    max_attempts             INTEGER NOT NULL DEFAULT 5,
    retry_reason             TEXT,
    acceptance_criteria_list TEXT    NOT NULL DEFAULT '[]',
    agent_status             TEXT,
    retry_context            TEXT,
    consecutive_failures     INTEGER NOT NULL DEFAULT 0,
    cost_usd                 REAL    NOT NULL DEFAULT 0,
    max_cost_usd             REAL,
    skip_pr                  INTEGER NOT NULL DEFAULT 0,
    branch_name              TEXT,
    model                    TEXT,
    started_at               INTEGER,
    ready                    INTEGER NOT NULL DEFAULT 1,
    last_heartbeat_at        INTEGER,
    epic_id                  TEXT    REFERENCES epic(id),
    created_at               INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at               INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX idx_task_repo_id ON task(repo_id);
CREATE INDEX idx_task_status ON task(status);
CREATE INDEX idx_task_status_pr ON task(status, pr_number) WHERE pr_number IS NOT NULL;
CREATE INDEX idx_task_epic_id ON task(epic_id) WHERE epic_id IS NOT NULL;

CREATE TABLE task_log (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id    TEXT    NOT NULL REFERENCES task(id),
    lines      TEXT    NOT NULL DEFAULT '[]',
    attempt    INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX idx_task_log_task_id ON task_log(task_id);
