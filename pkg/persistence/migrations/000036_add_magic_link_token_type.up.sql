-- Auto-Generated Schema Isolation for 000036
CREATE SCHEMA IF NOT EXISTS 000036;
SET search_path TO 000036, public;

ALTER TABLE tokens DROP CONSTRAINT tokens_type_check;
ALTER TABLE tokens ADD CONSTRAINT tokens_type_check CHECK (type IN ('EMAIL_VERIFICATION', 'PASSWORD_RESET', 'MFA_CHALLENGE', 'MAGIC_LINK_LOGIN'));
