-- 001_init.sql — Snow Radar control plane schema
-- Users: 128-bit random IDs (never sequential — audit finding #7).

CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS subscriptions (
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider   TEXT NOT NULL,
    external_id TEXT NOT NULL,
    state      TEXT NOT NULL CHECK (state IN ('trialing','active','past_due','cancelled')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, provider)
);

-- One tunnel address per peer; the UNIQUE constraint on address is the
-- DB-level backstop against duplicate lease assignment (defense in depth
-- behind the allocator).
CREATE TABLE IF NOT EXISTS peers (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    node_id    TEXT NOT NULL CHECK (node_id IN ('sgp','fsn')),
    stealth    BOOLEAN NOT NULL,
    address    CIDR NOT NULL UNIQUE,
    public_key TEXT NOT NULL UNIQUE,
    created_at BIGINT NOT NULL,
    active     BOOLEAN NOT NULL DEFAULT TRUE
);
CREATE INDEX IF NOT EXISTS idx_peers_user ON peers(user_id) WHERE active;

-- Refresh token JTIs for rotation/revoke (audit finding #2).
CREATE TABLE IF NOT EXISTS refresh_tokens (
    jti      TEXT PRIMARY KEY,
    user_id  TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    consumed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_refresh_user ON refresh_tokens(user_id);
