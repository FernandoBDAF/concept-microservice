-- Auth Service Database Schema
-- Migration: 001_create_users_table.sql

-- Create users table with salt support
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    hashed_password VARCHAR(255) NOT NULL,
    salt VARCHAR(64) NOT NULL,
    role VARCHAR(50) DEFAULT 'user',
    is_active BOOLEAN DEFAULT true,
    failed_attempts INTEGER DEFAULT 0,
    locked_until TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create audit logs table
CREATE TABLE IF NOT EXISTS auth_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID,
    action VARCHAR(100) NOT NULL,
    ip_address INET,
    user_agent TEXT,
    success BOOLEAN NOT NULL,
    details JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for users table (create after table for better performance)
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
CREATE INDEX IF NOT EXISTS idx_users_is_active ON users(is_active);

-- Create indexes for audit logs (only essential ones to avoid timeout)
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON auth_audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON auth_audit_logs(created_at);

-- Add foreign key constraint after indexes are created
ALTER TABLE auth_audit_logs ADD CONSTRAINT fk_audit_user_id 
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;

-- Add essential constraints only
ALTER TABLE users ADD CONSTRAINT chk_users_role CHECK (role IN ('user', 'admin'));

-- Add comments for documentation
COMMENT ON TABLE users IS 'User accounts with authentication information';
COMMENT ON COLUMN users.id IS 'Unique user identifier (UUID)';
COMMENT ON COLUMN users.email IS 'User email address (unique, lowercase)';
COMMENT ON COLUMN users.hashed_password IS 'Bcrypt hashed password - never expose in API';
COMMENT ON COLUMN users.salt IS 'Password salt - never expose in API';
COMMENT ON COLUMN users.failed_attempts IS 'Number of consecutive failed login attempts';
COMMENT ON COLUMN users.locked_until IS 'Account lock expiration timestamp'; 