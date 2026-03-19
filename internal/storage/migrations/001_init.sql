-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS files (
    slug        TEXT PRIMARY KEY,
    filename    TEXT NOT NULL,
    size        INTEGER NOT NULL,
    mime_type   TEXT NOT NULL DEFAULT 'application/octet-stream',
    password    TEXT,           -- bcrypt hash if password-protected, NULL otherwise
    one_use     INTEGER NOT NULL DEFAULT 0,  -- 0=false, 1=true
    downloaded_at INTEGER,      -- unix epoch; NULL = not yet downloaded (used for one-use atomicity)
    expires_at  INTEGER,        -- unix epoch; NULL = never
    created_at  INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX IF NOT EXISTS idx_files_expires ON files(expires_at) WHERE expires_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS buckets (
    slug        TEXT PRIMARY KEY,
    password    TEXT,
    one_use     INTEGER NOT NULL DEFAULT 0,
    downloaded_at INTEGER,
    expires_at  INTEGER,
    created_at  INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX IF NOT EXISTS idx_buckets_expires ON buckets(expires_at) WHERE expires_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS bucket_files (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    bucket_slug TEXT NOT NULL REFERENCES buckets(slug) ON DELETE CASCADE,
    filename    TEXT NOT NULL,
    size        INTEGER NOT NULL,
    mime_type   TEXT NOT NULL DEFAULT 'application/octet-stream',
    storage_key TEXT NOT NULL   -- slug used as on-disk key (never user-supplied filename)
);

CREATE TABLE IF NOT EXISTS pastes (
    slug        TEXT PRIMARY KEY,
    title       TEXT NOT NULL DEFAULT '',
    content     TEXT NOT NULL,
    lang        TEXT NOT NULL DEFAULT 'text',
    password    TEXT,
    one_use     INTEGER NOT NULL DEFAULT 0,
    viewed_at   INTEGER,        -- unix epoch; NULL = not yet viewed (one-use atomicity)
    expires_at  INTEGER,
    created_at  INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX IF NOT EXISTS idx_pastes_expires ON pastes(expires_at) WHERE expires_at IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS pastes;
DROP TABLE IF EXISTS bucket_files;
DROP TABLE IF EXISTS buckets;
DROP TABLE IF EXISTS files;
