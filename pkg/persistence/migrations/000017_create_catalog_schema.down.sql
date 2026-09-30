-- Drop Schema Isolation for catalog
DROP SCHEMA IF EXISTS catalog CASCADE;

DROP TABLE IF EXISTS applications;
DROP TABLE IF EXISTS variants;
DROP TABLE IF EXISTS item_categories;
DROP TABLE IF EXISTS items;
DROP TABLE IF EXISTS categories;