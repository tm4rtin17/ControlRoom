-- 001_initial.sql — auth, sessions, audit, setup token.

CREATE TABLE users (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    username        TEXT    NOT NULL UNIQUE,
    password_hash   BLOB    NOT NULL,
    totp_secret     TEXT,
    totp_enabled    INTEGER NOT NULL DEFAULT 0,
    role            TEXT    NOT NULL DEFAULT 'admin',
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
);

-- session.id is a random 32-char hex string; refresh_hash is sha256 of the
-- opaque refresh secret half of the cookie. family_id groups rotated tokens
-- so reuse of a revoked token can revoke the whole chain.
CREATE TABLE sessions (
    id              TEXT    PRIMARY KEY,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_hash    BLOB    NOT NULL,
    family_id       TEXT    NOT NULL,
    parent_id       TEXT    REFERENCES sessions(id) ON DELETE SET NULL,
    ip              TEXT    NOT NULL,
    user_agent      TEXT,
    created_at      INTEGER NOT NULL,
    expires_at      INTEGER NOT NULL,
    revoked_at      INTEGER
);

CREATE INDEX idx_sessions_user_id   ON sessions(user_id);
CREATE INDEX idx_sessions_family_id ON sessions(family_id);

CREATE TABLE audit_log (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    ts              INTEGER NOT NULL,
    user_id         INTEGER REFERENCES users(id) ON DELETE SET NULL,
    ip              TEXT,
    action          TEXT    NOT NULL,
    target          TEXT,
    outcome         TEXT    NOT NULL,
    detail          TEXT
);

CREATE INDEX idx_audit_log_ts      ON audit_log(ts);
CREATE INDEX idx_audit_log_user_id ON audit_log(user_id);
CREATE INDEX idx_audit_log_action  ON audit_log(action);

-- One row only; CHECK enforces it.
CREATE TABLE setup_token (
    id              INTEGER PRIMARY KEY CHECK (id = 1),
    token_hash      BLOB    NOT NULL,
    created_at      INTEGER NOT NULL,
    used_at         INTEGER
);
