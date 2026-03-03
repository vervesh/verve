CREATE TABLE repo (
    id         TEXT PRIMARY KEY,
    owner      TEXT    NOT NULL,
    name       TEXT    NOT NULL,
    full_name  TEXT    NOT NULL UNIQUE,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
