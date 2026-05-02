PRAGMA journal_mode = WAL;
PRAGMA synchronous  = NORMAL;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS snapshots (
    source     TEXT    NOT NULL,
    fetched_at INTEGER NOT NULL, -- unix milliseconds UTC
    etag       TEXT,
    payload    BLOB    NOT NULL, -- gzipped JSON
    PRIMARY KEY (source, fetched_at)
);
