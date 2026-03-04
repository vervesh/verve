CREATE TABLE conversation (
    id                TEXT PRIMARY KEY,
    repo_id           TEXT NOT NULL REFERENCES repo(id) ON DELETE CASCADE,
    title             TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    messages          JSONB NOT NULL DEFAULT '[]',
    model             TEXT,
    claimed_at        BIGINT,
    last_heartbeat_at BIGINT,
    pending_message   TEXT,
    epic_id           TEXT REFERENCES epic(id) ON DELETE SET NULL,
    created_at        BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
    updated_at        BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT
);
CREATE INDEX idx_conversation_repo_id ON conversation(repo_id);
CREATE INDEX idx_conversation_status ON conversation(status);
