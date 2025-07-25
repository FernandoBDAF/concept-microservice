-- Migration: Create auth tables for auth-service integration
-- Version: 003
-- Description: Creates auth_users, auth_audit_logs, auth_roles, and auth_permissions tables

-- Create auth_users table
CREATE TABLE IF NOT EXISTS auth_users (
    id VARCHAR(36) PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    hashed_password VARCHAR(255) NOT NULL,
    salt VARCHAR(255) NOT NULL,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'user',
    is_active BOOLEAN NOT NULL DEFAULT true,
    is_verified BOOLEAN NOT NULL DEFAULT false,
    last_login_at TIMESTAMP,
    failed_attempts INTEGER NOT NULL DEFAULT 0,
    locked_until TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for auth_users
CREATE INDEX IF NOT EXISTS idx_auth_users_email ON auth_users(email);
CREATE INDEX IF NOT EXISTS idx_auth_users_role ON auth_users(role);
CREATE INDEX IF NOT EXISTS idx_auth_users_is_active ON auth_users(is_active);
CREATE INDEX IF NOT EXISTS idx_auth_users_created_at ON auth_users(created_at);

-- Create auth_audit_logs table
CREATE TABLE IF NOT EXISTS auth_audit_logs (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36),
    action VARCHAR(100) NOT NULL,
    resource VARCHAR(255) NOT NULL,
    ip_address VARCHAR(45) NOT NULL,
    user_agent VARCHAR(1000),
    success BOOLEAN NOT NULL,
    details TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Foreign key constraint
    CONSTRAINT fk_audit_user FOREIGN KEY (user_id) REFERENCES auth_users(id) ON DELETE SET NULL
);

-- Create indexes for auth_audit_logs
CREATE INDEX IF NOT EXISTS idx_auth_audit_logs_user_id ON auth_audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_auth_audit_logs_action ON auth_audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_auth_audit_logs_resource ON auth_audit_logs(resource);
CREATE INDEX IF NOT EXISTS idx_auth_audit_logs_ip_address ON auth_audit_logs(ip_address);
CREATE INDEX IF NOT EXISTS idx_auth_audit_logs_success ON auth_audit_logs(success);
CREATE INDEX IF NOT EXISTS idx_auth_audit_logs_created_at ON auth_audit_logs(created_at);

-- Create auth_roles table
CREATE TABLE IF NOT EXISTS auth_roles (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    description VARCHAR(500),
    is_system BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for auth_roles
CREATE INDEX IF NOT EXISTS idx_auth_roles_name ON auth_roles(name);
CREATE INDEX IF NOT EXISTS idx_auth_roles_is_system ON auth_roles(is_system);

-- Create auth_permissions table
CREATE TABLE IF NOT EXISTS auth_permissions (
    id VARCHAR(36) PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id VARCHAR(36) NOT NULL,
    permission VARCHAR(100) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Foreign key constraint
    CONSTRAINT fk_permission_role FOREIGN KEY (role_id) REFERENCES auth_roles(id) ON DELETE CASCADE,
    
    -- Unique constraint to prevent duplicate permissions per role
    CONSTRAINT uk_role_permission UNIQUE (role_id, permission)
);

-- Create indexes for auth_permissions
CREATE INDEX IF NOT EXISTS idx_auth_permissions_role_id ON auth_permissions(role_id);
CREATE INDEX IF NOT EXISTS idx_auth_permissions_permission ON auth_permissions(permission);

-- Insert default system roles
INSERT INTO auth_roles (id, name, description, is_system, created_at, updated_at) VALUES
    ('admin', 'Administrator', 'Full system access with all permissions', true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('user', 'User', 'Standard user with basic profile access', true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('moderator', 'Moderator', 'Moderation access with user and profile management', true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO NOTHING;

-- Insert default permissions for admin role
INSERT INTO auth_permissions (role_id, permission, created_at) VALUES
    ('admin', 'users:read', CURRENT_TIMESTAMP),
    ('admin', 'users:write', CURRENT_TIMESTAMP),
    ('admin', 'users:delete', CURRENT_TIMESTAMP),
    ('admin', 'profiles:read', CURRENT_TIMESTAMP),
    ('admin', 'profiles:write', CURRENT_TIMESTAMP),
    ('admin', 'profiles:delete', CURRENT_TIMESTAMP),
    ('admin', 'audit:read', CURRENT_TIMESTAMP),
    ('admin', 'roles:read', CURRENT_TIMESTAMP),
    ('admin', 'roles:write', CURRENT_TIMESTAMP),
    ('admin', 'roles:delete', CURRENT_TIMESTAMP),
    ('admin', 'system:admin', CURRENT_TIMESTAMP)
ON CONFLICT (role_id, permission) DO NOTHING;

-- Insert default permissions for user role
INSERT INTO auth_permissions (role_id, permission, created_at) VALUES
    ('user', 'profiles:read', CURRENT_TIMESTAMP),
    ('user', 'profiles:write', CURRENT_TIMESTAMP)
ON CONFLICT (role_id, permission) DO NOTHING;

-- Insert default permissions for moderator role
INSERT INTO auth_permissions (role_id, permission, created_at) VALUES
    ('moderator', 'users:read', CURRENT_TIMESTAMP),
    ('moderator', 'users:write', CURRENT_TIMESTAMP),
    ('moderator', 'profiles:read', CURRENT_TIMESTAMP),
    ('moderator', 'profiles:write', CURRENT_TIMESTAMP),
    ('moderator', 'profiles:delete', CURRENT_TIMESTAMP),
    ('moderator', 'audit:read', CURRENT_TIMESTAMP)
ON CONFLICT (role_id, permission) DO NOTHING;

-- Add updated_at trigger for auth_users
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_auth_users_updated_at BEFORE UPDATE ON auth_users FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_auth_roles_updated_at BEFORE UPDATE ON auth_roles FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Add constraints for better data integrity
ALTER TABLE auth_users ADD CONSTRAINT check_email_format CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$');
ALTER TABLE auth_users ADD CONSTRAINT check_role_valid CHECK (role IN ('admin', 'user', 'moderator'));
ALTER TABLE auth_users ADD CONSTRAINT check_failed_attempts_positive CHECK (failed_attempts >= 0);

-- Add comments for documentation
COMMENT ON TABLE auth_users IS 'User accounts for authentication system';
COMMENT ON TABLE auth_audit_logs IS 'Security audit logs for tracking user actions';
COMMENT ON TABLE auth_roles IS 'Role definitions for role-based access control';
COMMENT ON TABLE auth_permissions IS 'Permissions assigned to roles';

COMMENT ON COLUMN auth_users.hashed_password IS 'Bcrypt hashed password - never expose in API';
COMMENT ON COLUMN auth_users.salt IS 'Password salt - never expose in API';
COMMENT ON COLUMN auth_users.failed_attempts IS 'Number of consecutive failed login attempts';
COMMENT ON COLUMN auth_users.locked_until IS 'Account lock expiration timestamp';
COMMENT ON COLUMN auth_audit_logs.details IS 'JSON string with additional context for audit event';
COMMENT ON COLUMN auth_roles.is_system IS 'System roles cannot be deleted or modified'; 