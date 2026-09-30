-- Drop Schema Isolation for 000039
DROP SCHEMA IF EXISTS 000039 CASCADE;

-- Revert campaign/financial multi-tenancy

ALTER TABLE donation_attributions DROP COLUMN organization_id;
ALTER TABLE invoices DROP COLUMN organization_id;
ALTER TABLE ledger_entries DROP COLUMN organization_id;
ALTER TABLE journal_entries DROP COLUMN organization_id;
ALTER TABLE payment_methods DROP COLUMN organization_id;
ALTER TABLE prices DROP COLUMN organization_id;
ALTER TABLE subscriptions DROP COLUMN organization_id;
ALTER TABLE customers DROP COLUMN organization_id;
ALTER TABLE ledger_accounts DROP COLUMN organization_id;
ALTER TABLE products DROP COLUMN organization_id;
ALTER TABLE transactions DROP COLUMN organization_id;

ALTER TABLE fundraisers DROP CONSTRAINT fundraisers_org_slug_key;
ALTER TABLE fundraisers ADD CONSTRAINT fundraisers_slug_key UNIQUE (slug);
ALTER TABLE fundraisers DROP COLUMN image_url;
ALTER TABLE fundraisers DROP COLUMN organization_id;

ALTER TABLE campaigns DROP CONSTRAINT campaigns_org_slug_key;
ALTER TABLE campaigns ADD CONSTRAINT campaigns_slug_key UNIQUE (slug);
ALTER TABLE campaigns DROP COLUMN image_url;
ALTER TABLE campaigns DROP COLUMN organization_id;
