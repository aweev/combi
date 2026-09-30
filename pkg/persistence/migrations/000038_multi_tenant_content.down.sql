-- Drop Schema Isolation for 000038
DROP SCHEMA IF EXISTS 000038 CASCADE;

ALTER TABLE models DROP CONSTRAINT models_space_org_api_key_key;
ALTER TABLE models ADD CONSTRAINT models_space_id_api_key_key UNIQUE (api_key, space_id);

DROP INDEX idx_entries_org;
DROP INDEX idx_models_org;
DROP INDEX idx_spaces_org;

ALTER TABLE entries DROP COLUMN organization_id;
ALTER TABLE models DROP COLUMN organization_id;
ALTER TABLE spaces DROP COLUMN organization_id;
