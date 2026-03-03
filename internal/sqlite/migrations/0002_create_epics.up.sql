CREATE TABLE epic (
    id                TEXT PRIMARY KEY,
    repo_id           TEXT    NOT NULL REFERENCES repo(id),
    title             TEXT    NOT NULL,
    description       TEXT    NOT NULL DEFAULT '',
    status            TEXT    NOT NULL DEFAULT 'draft'
                      CHECK(status IN ('draft', 'planning', 'ready', 'active', 'completed', 'closed')),
    proposed_tasks    TEXT    NOT NULL DEFAULT '[]',
    task_ids          TEXT    NOT NULL DEFAULT '[]',
    planning_prompt   TEXT,
    session_log       TEXT    NOT NULL DEFAULT '[]',
    not_ready         INTEGER NOT NULL DEFAULT 0,
    claimed_at        INTEGER,
    last_heartbeat_at INTEGER,
    feedback          TEXT,
    feedback_type     TEXT    CHECK(feedback_type IN ('message', 'confirmed', 'closed')),
    model             TEXT,
    created_at        INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at        INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX idx_epic_repo_id ON epic(repo_id);
CREATE INDEX idx_epic_status ON epic(status);
