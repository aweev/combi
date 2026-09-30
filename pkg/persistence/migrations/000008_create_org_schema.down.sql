-- Drop Schema Isolation for org
DROP SCHEMA IF EXISTS org CASCADE;

DROP TABLE IF EXISTS org_invitations;
DROP TABLE IF EXISTS org_members;
DROP TABLE IF EXISTS org_roles;
DROP TABLE IF EXISTS organizations;