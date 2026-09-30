-- Reverts the schema created for the User service.

-- Drop triggers FIRST (before tables)
DROP TRIGGER IF EXISTS set_connections_updated_at ON connections;
DROP TRIGGER IF EXISTS set_profiles_updated_at ON profiles;
DROP TRIGGER IF EXISTS set_users_updated_at ON users;

-- Drop tables in reverse order
DROP TABLE IF EXISTS connections;
DROP TABLE IF EXISTS profiles;
DROP TABLE IF EXISTS users;

-- DO NOT drop the shared function (other services use it)
-- DROP FUNCTION IF EXISTS update_updated_at_column();

-- DO NOT drop the extension (other services use it)
-- DROP EXTENSION IF EXISTS "pgcrypto";