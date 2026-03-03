CREATE TYPE task_status AS ENUM (
    'pending',
    'running',
    'review',
    'merged',
    'closed',
    'failed'
    );

CREATE TABLE task
(
    id                       TEXT PRIMARY KEY,
    repo_id                  TEXT             NOT NULL REFERENCES repo (id),
    title                    TEXT             NOT NULL DEFAULT '',
    description              TEXT             NOT NULL,
    status                   task_status      NOT NULL DEFAULT 'pending',
    pull_request_url         TEXT,
    pr_number                INTEGER,
    depends_on               TEXT[]           NOT NULL DEFAULT '{}',
    close_reason             TEXT,
    attempt                  INTEGER          NOT NULL DEFAULT 1,
    max_attempts             INTEGER          NOT NULL DEFAULT 5,
    retry_reason             TEXT,
    acceptance_criteria_list TEXT[]           NOT NULL DEFAULT '{}',
    agent_status             TEXT,
    retry_context            TEXT,
    consecutive_failures     INTEGER          NOT NULL DEFAULT 0,
    cost_usd                 DOUBLE PRECISION NOT NULL DEFAULT 0,
    max_cost_usd             DOUBLE PRECISION,
    skip_pr                  BOOLEAN          NOT NULL DEFAULT false,
    branch_name              TEXT,
    model                    TEXT,
    started_at               BIGINT,
    ready                    BOOLEAN          NOT NULL DEFAULT true,
    last_heartbeat_at        BIGINT,
    epic_id                  TEXT             REFERENCES epic (id),
    created_at               BIGINT           NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
    updated_at               BIGINT           NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT
);

CREATE INDEX idx_task_repo_id ON task (repo_id);
CREATE INDEX idx_task_status ON task (status);
CREATE INDEX idx_task_status_pr ON task (status, pr_number) WHERE pr_number IS NOT NULL;
CREATE INDEX idx_task_epic_id ON task (epic_id) WHERE epic_id IS NOT NULL;

CREATE TABLE task_log
(
    id         BIGSERIAL PRIMARY KEY,
    task_id    TEXT    NOT NULL REFERENCES task (id),
    lines      TEXT[]  NOT NULL,
    attempt    INTEGER NOT NULL DEFAULT 1,
    created_at BIGINT  NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT
);

CREATE INDEX idx_task_log_task_id ON task_log (task_id);
