CREATE TABLE IF NOT EXISTS settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    password_hash TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS api_keys (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    key_prefix TEXT NOT NULL,
    key_hash TEXT NOT NULL,
    created_at TEXT NOT NULL,
    last_used_at TEXT,
    revoked_at TEXT
);

CREATE TABLE IF NOT EXISTS items (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    source_kind TEXT NOT NULL,
    original_filename TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    expires_at TEXT,
    trashed_at TEXT,
    screenshot_status TEXT NOT NULL DEFAULT 'pending',
    screenshot_error TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_items_status_updated_at ON items (status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_items_status_expires_at ON items (status, expires_at);
CREATE INDEX IF NOT EXISTS idx_items_status_trashed_at ON items (status, trashed_at);
