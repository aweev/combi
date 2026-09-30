-- Drop Schema Isolation for auth
DROP SCHEMA IF EXISTS auth CASCADE;

-- 1. Drop triggers first (no dependencies)
DROP TRIGGER IF EXISTS set_tokens_updated_at ON tokens;
DROP TRIGGER IF EXISTS set_sessions_updated_at ON sessions;
DROP TRIGGER IF EXISTS set_auth_methods_updated_at ON auth_methods;
DROP TRIGGER IF EXISTS set_accounts_updated_at ON accounts;

-- 2. Drop tables (now safe, triggers are gone)
DROP TABLE IF EXISTS tokens;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS auth_methods;
DROP TABLE IF EXISTS accounts;

-- 3. Leave shared resources alone (commented out)
-- DROP FUNCTION IF EXISTS update_updated_at_column();
-- DROP EXTENSION IF EXISTS "pgcrypto";