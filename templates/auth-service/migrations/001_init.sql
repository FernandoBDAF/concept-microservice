-- auth-service template — baseline schema.
-- Idempotent (IF NOT EXISTS / guarded constraints): safe to re-run, so both the
-- in-app migration runner (src/infrastructure/database/migrations.ts) and a raw
-- `psql -f` loop (the compose/k8s one-shot migrate job) can apply it.
--
-- NOTE ON SALT: there is no `salt` column. bcrypt salts internally; the lab's
-- history added then dropped a separate salt column (ADR-009.6) — a fresh copy
-- starts from the settled shape.

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    hashed_password VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'user',
    is_active BOOLEAN NOT NULL DEFAULT true,
    failed_attempts INTEGER NOT NULL DEFAULT 0,   -- account lockout counter
    locked_until TIMESTAMP,                        -- lockout expiry (null = not locked)
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_is_active ON users(is_active);

-- Role whitelist. Postgres has no "ADD CONSTRAINT IF NOT EXISTS", so guard it.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_users_role') THEN
        ALTER TABLE users ADD CONSTRAINT chk_users_role CHECK (role IN ('user', 'admin'));
    END IF;
END $$;

-- Minimal security audit trail: the template writes exactly one event kind,
-- REFRESH_TOKEN_REUSE (rotation theft signal, ADR-009.2). Keep it lean.
CREATE TABLE IF NOT EXISTS auth_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID,
    action VARCHAR(100) NOT NULL,
    success BOOLEAN NOT NULL,
    details JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON auth_audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON auth_audit_logs(created_at);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_audit_user_id') THEN
        ALTER TABLE auth_audit_logs ADD CONSTRAINT fk_audit_user_id
            FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;
    END IF;
END $$;
