CREATE TABLE setting
(
    key        TEXT PRIMARY KEY,
    value      TEXT   NOT NULL,
    updated_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT
);
