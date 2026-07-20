-- Auth Service Database Schema
-- Migration: 002_sessions_roles.sql (ADR-009.2 / .6 / .7)
--
-- Refresh-token sessions with rotation + reuse detection, and the salt drop.
--
-- ⚠️ ADR-009.6: dropping users.salt invalidates every existing dev credential,
-- because old hashes were computed over `password + salt`. This is accepted for
-- the lab — re-seed users (SEED_ADMIN_* bootstrap / POST /v1/users) after this
-- migration. bcrypt already salts internally, so the column added nothing.

CREATE TABLE sessions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_id UUID NOT NULL UNIQUE,        -- jti of the CURRENT token
    previous_token_id UUID,                       -- rotation chain, reuse detection
    expires_at       TIMESTAMP NOT NULL,
    created_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at       TIMESTAMP
);
CREATE INDEX idx_sessions_user ON sessions(user_id);
ALTER TABLE users DROP COLUMN salt;               -- ADR-009.6
