-- Auto-Generated Schema Isolation for 000039
CREATE SCHEMA IF NOT EXISTS 000039;
SET search_path TO 000039, public;

-- Add organization_id to campaign and financial schemas for multi-tenancy

-- Campaigns
ALTER TABLE campaigns ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE;
ALTER TABLE campaigns ADD COLUMN image_url TEXT;
CREATE INDEX idx_campaigns_org ON campaigns(organization_id);

-- The slug should be unique per-organization, not globally
ALTER TABLE campaigns DROP CONSTRAINT campaigns_slug_key;
ALTER TABLE campaigns ADD CONSTRAINT campaigns_org_slug_key UNIQUE (organization_id, slug);

-- Fundraisers
ALTER TABLE fundraisers ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE;
ALTER TABLE fundraisers ADD COLUMN image_url TEXT;
CREATE INDEX idx_fundraisers_org ON fundraisers(organization_id);

ALTER TABLE fundraisers DROP CONSTRAINT fundraisers_slug_key;
ALTER TABLE fundraisers ADD CONSTRAINT fundraisers_org_slug_key UNIQUE (organization_id, slug);

-- Financial: Transactions
ALTER TABLE transactions ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE;
CREATE INDEX idx_transactions_org ON transactions(organization_id);

-- Financial: Products
ALTER TABLE products ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE;
CREATE INDEX idx_products_org ON products(organization_id);

-- Financial: Ledger Accounts
ALTER TABLE ledger_accounts ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE;
CREATE INDEX idx_ledger_accounts_org ON ledger_accounts(organization_id);

-- Financial: Customers
ALTER TABLE customers ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE;
CREATE INDEX idx_customers_org ON customers(organization_id);

-- Financial: Subscriptions
ALTER TABLE subscriptions ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE;
CREATE INDEX idx_subscriptions_org ON subscriptions(organization_id);

-- Financial: Prices
ALTER TABLE prices ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE;
CREATE INDEX idx_prices_org ON prices(organization_id);

-- Financial: Payment Methods
ALTER TABLE payment_methods ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE;
CREATE INDEX idx_payment_methods_org ON payment_methods(organization_id);

-- Financial: Journal Entries
ALTER TABLE journal_entries ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE;
CREATE INDEX idx_journal_entries_org ON journal_entries(organization_id);

-- Financial: Ledger Entries
ALTER TABLE ledger_entries ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE;
CREATE INDEX idx_ledger_entries_org ON ledger_entries(organization_id);

-- Financial: Invoices
ALTER TABLE invoices ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE;
CREATE INDEX idx_invoices_org ON invoices(organization_id);

-- Campaign: Donation Attributions
ALTER TABLE donation_attributions ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE;
CREATE INDEX idx_attributions_org ON donation_attributions(organization_id);
