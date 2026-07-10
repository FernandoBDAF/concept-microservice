-- Clean up storage service database for new architecture
-- Remove auth-related tables, keep only profile tables

-- Drop auth-related tables
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS user_sessions CASCADE;
DROP TABLE IF EXISTS app_data CASCADE;

-- Verify only profile tables remain
\echo 'Tables remaining after cleanup:'
\dt 