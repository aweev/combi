-- Auto-Generated Schema Isolation for catalog
CREATE SCHEMA IF NOT EXISTS catalog;
SET search_path TO catalog, public;

-- Drop old tables if they exist (cleanup from previous iterations)
DROP TABLE IF EXISTS application_schemas;
DROP TABLE IF EXISTS offerings;
DROP TABLE IF EXISTS programs;
DROP TABLE IF EXISTS applications;
DROP TABLE IF EXISTS variants;
DROP TABLE IF EXISTS item_categories;
DROP TABLE IF EXISTS items;
DROP TABLE IF EXISTS categories;

-- 1. Categories
CREATE TABLE categories (
    id UUID PRIMARY KEY,
    parent_id UUID REFERENCES categories(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_categories_parent ON categories(parent_id);

-- 2. Items
CREATE TABLE items (
    id UUID PRIMARY KEY,
    type VARCHAR(50) NOT NULL, -- PROGRAM, VOLUNTEER, PRODUCT
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status VARCHAR(50) NOT NULL, -- DRAFT, AVAILABLE, ARCHIVED
    
    properties JSONB NOT NULL DEFAULT '{}',
    tags JSONB NOT NULL DEFAULT '[]',
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_items_type ON items(type);
CREATE INDEX idx_items_status ON items(status);
CREATE INDEX idx_items_tags ON items USING gin (tags);

-- 3. Item <-> Category (Many-to-Many)
CREATE TABLE item_categories (
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    PRIMARY KEY (item_id, category_id)
);

CREATE INDEX idx_item_categories_cat ON item_categories(category_id);

-- 4. Variants
CREATE TABLE variants (
    id UUID PRIMARY KEY,
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    sku VARCHAR(100),
    name VARCHAR(255) NOT NULL,
    price_amount BIGINT NOT NULL DEFAULT 0,
    price_currency CHAR(3) NOT NULL DEFAULT 'USD',
    stock INTEGER, -- NULL = Infinite
    
    attributes JSONB NOT NULL DEFAULT '{}',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_variants_item ON variants(item_id);

-- 5. Applications
CREATE TABLE applications (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL, -- External User ID
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    variant_id UUID REFERENCES variants(id) ON DELETE SET NULL,
    
    status VARCHAR(50) NOT NULL, -- PENDING, ACCEPTED, REJECTED
    answers JSONB NOT NULL DEFAULT '{}',
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Business Rule: One active application per user per item
    UNIQUE (user_id, item_id) 
);

CREATE INDEX idx_applications_user ON applications(user_id);
CREATE INDEX idx_applications_item ON applications(item_id);
CREATE INDEX idx_applications_status ON applications(status);