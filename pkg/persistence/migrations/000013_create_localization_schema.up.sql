-- Auto-Generated Schema Isolation for localization
CREATE SCHEMA IF NOT EXISTS localization;
SET search_path TO localization, public;


CREATE TABLE locales (
    code VARCHAR(10) PRIMARY KEY, -- e.g. "en-US"
    name VARCHAR(100) NOT NULL,   -- e.g. "English (US)"
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    direction VARCHAR(10) NOT NULL DEFAULT 'ltr',
    date_format VARCHAR(50) NOT NULL DEFAULT 'DD/MM/YYYY',
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
);

CREATE TABLE translation_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    namespace VARCHAR(100) NOT NULL, -- e.g. "auth", "home"
    key VARCHAR(255) NOT NULL,       -- e.g. "welcome_message"
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    
    UNIQUE (namespace, key)
);

CREATE TABLE translation_values (
    key_id UUID NOT NULL REFERENCES translation_keys(id) ON DELETE CASCADE,
    locale VARCHAR(10) NOT NULL REFERENCES locales(code) ON DELETE CASCADE,
    value TEXT NOT NULL,
    is_verified BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    
    PRIMARY KEY (key_id, locale)
);

-- Seed Default Locales
INSERT INTO locales (code, name, is_default, is_active, direction, date_format) VALUES 
('en-US', 'English (US)', true, true, 'ltr', 'MM/DD/YYYY'),
('de-DE', 'German (Germany)', false, true, 'ltr', 'DD.MM.YYYY'),
('fr-FR', 'French (France)', false, true, 'ltr', 'DD/MM/YYYY'),
('es-ES', 'Spanish (Spain)', false, true, 'ltr', 'DD/MM/YYYY'),
('ar-SA', 'Arabic (Saudi Arabia)', false, true, 'rtl', 'DD/MM/YYYY');