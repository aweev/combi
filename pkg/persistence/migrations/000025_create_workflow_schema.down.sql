-- Drop Schema Isolation for workflow
DROP SCHEMA IF EXISTS workflow CASCADE;

-- Reverse order of creation to handle Foreign Key constraints

-- 1. Drop Approvals (Depends on Instances)
DROP TABLE IF EXISTS approval_requests;

-- 2. Drop Instances (Depends on Definitions)
DROP TABLE IF EXISTS workflow_instances;

-- 3. Drop Definitions
DROP TABLE IF EXISTS workflow_definitions;

-- Note: We generally do NOT drop the "uuid-ossp" extension here, 
-- as it is likely shared by other schemas (Auth, Automation, etc.) in the database.