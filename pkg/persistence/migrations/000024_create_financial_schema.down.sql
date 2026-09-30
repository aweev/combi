-- Drop Schema Isolation for financial
DROP SCHEMA IF EXISTS financial CASCADE;

-- Drop tables in reverse order of dependency to avoid Foreign Key errors

-- 1. Invoices depends on Subscriptions & Transactions
DROP TABLE IF EXISTS invoices;

-- 2. Subscriptions depends on Prices
DROP TABLE IF EXISTS subscriptions;

-- 3. Payment Methods & Customers depend on nothing internal (just users)
DROP TABLE IF EXISTS payment_methods;
DROP TABLE IF EXISTS customers;

-- 4. Prices depends on Products
DROP TABLE IF EXISTS prices;
DROP TABLE IF EXISTS products;

-- 5. Ledger Entries depends on Journal Entries & Accounts
DROP TABLE IF EXISTS ledger_entries;

-- 6. Journal Entries depends on Transactions
DROP TABLE IF EXISTS journal_entries;

-- 7. Transactions (central financial table)
DROP TABLE IF EXISTS transactions;

-- 8. Ledger Accounts
DROP TABLE IF EXISTS ledger_accounts;

-- 9. Donation Attributions depends on Campaigns & Fundraisers
DROP TABLE IF EXISTS donation_attributions;

-- 10. Fundraisers depends on Campaigns
DROP TABLE IF EXISTS fundraisers;

-- 11. Campaigns (Root table)
DROP TABLE IF EXISTS campaigns;