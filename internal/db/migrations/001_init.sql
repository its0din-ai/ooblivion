PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS scopes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    match_on    TEXT NOT NULL,
    match_type  TEXT NOT NULL,
    pattern     TEXT NOT NULL,
    header_name TEXT,
    enabled     INTEGER NOT NULL DEFAULT 1,
    priority    INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS requests (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    method          TEXT NOT NULL,
    scheme          TEXT,
    host            TEXT,
    path            TEXT NOT NULL,
    query           TEXT,
    http_version    TEXT,
    request_headers TEXT,
    body            TEXT,
    body_truncated  INTEGER NOT NULL DEFAULT 0,
    source_ip       TEXT,
    remote_addr     TEXT,
    forwarded_for   TEXT,
    user_agent      TEXT,
    content_type    TEXT,
    saved           INTEGER NOT NULL DEFAULT 0,
    scope_id        INTEGER REFERENCES scopes(id),
    notified        INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_requests_saved_created ON requests (saved, created_at);
CREATE INDEX IF NOT EXISTS idx_requests_method ON requests (method);
CREATE INDEX IF NOT EXISTS idx_requests_path ON requests (path);
CREATE INDEX IF NOT EXISTS idx_requests_host ON requests (host);
CREATE INDEX IF NOT EXISTS idx_requests_scope ON requests (scope_id);
CREATE INDEX IF NOT EXISTS idx_requests_created ON requests (created_at);

CREATE TABLE IF NOT EXISTS notification_rules (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    match_on    TEXT NOT NULL,
    match_type  TEXT NOT NULL,
    pattern     TEXT NOT NULL,
    header_name TEXT,
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS notification_log (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id    INTEGER REFERENCES notification_rules(id),
    request_id INTEGER REFERENCES requests(id),
    ok         INTEGER NOT NULL,
    response   TEXT,
    sent_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS auth (
    id            INTEGER PRIMARY KEY CHECK (id = 1),
    password_hash TEXT NOT NULL,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS jwt_denylist (
    jti        TEXT PRIMARY KEY,
    expires_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_log (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    actor      TEXT NOT NULL,
    action     TEXT NOT NULL,
    detail     TEXT,
    ip         TEXT,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_audit_log_created ON audit_log (created_at);
CREATE INDEX IF NOT EXISTS idx_notification_log_sent ON notification_log (sent_at);
