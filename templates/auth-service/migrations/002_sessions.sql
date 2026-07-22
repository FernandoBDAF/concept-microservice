-- Refresh-token sessions (ADR-009.2). One row per active refresh chain.
--   refresh_token_id  = jti of the CURRENT refresh token
--   previous_token_id = the immediately-rotated-out jti — replaying it is the
--                       reuse (theft) signal that revokes the whole chain
-- Idempotent so the raw-psql migrate path can re-run it.

CREATE TABLE IF NOT EXISTS sessions (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_id  UUID NOT NULL UNIQUE,
    previous_token_id UUID,
    expires_at        TIMESTAMP NOT NULL,
    created_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at        TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_previous_token ON sessions(previous_token_id);
