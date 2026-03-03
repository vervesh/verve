CREATE TABLE epic
(
    id                TEXT PRIMARY KEY,
    repo_id           TEXT    NOT NULL REFERENCES repo (id),
    title             TEXT    NOT NULL,
    description       TEXT    NOT NULL DEFAULT '',
    status            TEXT    NOT NULL DEFAULT 'draft'
                      CHECK (status IN ('draft', 'planning', 'ready', 'active', 'completed', 'closed')),
    proposed_tasks    JSONB   NOT NULL DEFAULT '[]',
    task_ids          TEXT[]  NOT NULL DEFAULT '{}',
    planning_prompt   TEXT,
    session_log       TEXT[]  NOT NULL DEFAULT '{}',
    not_ready         BOOLEAN NOT NULL DEFAULT false,
    claimed_at        BIGINT,
    last_heartbeat_at BIGINT,
    feedback          TEXT,
    feedback_type     TEXT    CHECK (feedback_type IN ('message', 'confirmed', 'closed')),
    model             TEXT,
    created_at        BIGINT  NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
    updated_at        BIGINT  NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT
);

CREATE INDEX idx_epic_repo_id ON epic (repo_id);
CREATE INDEX idx_epic_status ON epic (status);
