CREATE TABLE github_token (
    id              TEXT    PRIMARY KEY DEFAULT 'default' CHECK (id = 'default'),
    encrypted_token TEXT    NOT NULL,
    created_at      INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at      INTEGER NOT NULL DEFAULT (unixepoch())
);
