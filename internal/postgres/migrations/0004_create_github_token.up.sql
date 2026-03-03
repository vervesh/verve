CREATE TABLE github_token
(
    id              TEXT PRIMARY KEY DEFAULT 'default' CHECK (id = 'default'),
    encrypted_token TEXT   NOT NULL,
    created_at      BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
    updated_at      BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT
);
