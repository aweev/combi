-- Auto-Generated Schema Isolation for notification
CREATE SCHEMA IF NOT EXISTS notification;
SET search_path TO notification, public;

-- NOTIFICATION_PROVIDERS
CREATE TABLE notification_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) UNIQUE NOT NULL,
    type VARCHAR(50) NOT NULL, -- SMTP, TWILIO, etc.
    channel VARCHAR(50) NOT NULL, -- EMAIL, SMS, etc.
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    config_encrypted BYTEA NOT NULL,
    encryption_context JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
);
CREATE INDEX idx_notif_providers_channel ON notification_providers(channel) WHERE is_active = TRUE;
CREATE TRIGGER set_notif_providers_updated_at BEFORE UPDATE ON notification_providers FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- NOTIFICATION_TEMPLATES
CREATE TABLE notification_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    description TEXT,
    type VARCHAR(50) NOT NULL, -- TRANSACTIONAL, MARKETING
    category VARCHAR(100) NOT NULL,
    channel VARCHAR(50) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    locale VARCHAR(10) NOT NULL DEFAULT 'en-US',
    subject TEXT,
    body TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    created_by UUID NOT NULL, -- Assuming reference to a user/admin
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),

    UNIQUE(slug, channel)
);
CREATE TRIGGER set_notif_templates_updated_at BEFORE UPDATE ON notification_templates FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- TEMPLATE_TRANSLATIONS
CREATE TABLE template_translations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id UUID NOT NULL REFERENCES notification_templates(id) ON DELETE CASCADE,
    locale VARCHAR(10) NOT NULL,
    subject TEXT,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    UNIQUE(template_id, locale)
);
CREATE TRIGGER set_template_translations_updated_at BEFORE UPDATE ON template_translations FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- NOTIFICATION_PREFERENCES
CREATE TABLE notification_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL, -- Weak reference to users table (or strong if strictly coupled)
    channel VARCHAR(50) NOT NULL,
    scope VARCHAR(50) NOT NULL, -- GLOBAL, CATEGORY, NOTIFICATION
    category VARCHAR(100),
    template_id UUID REFERENCES notification_templates(id) ON DELETE CASCADE,
    is_enabled BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    -- Add unique constraint logic depending on scope via application or partial indexes
    UNIQUE(user_id, channel, scope, category, template_id)
);
CREATE TRIGGER set_notif_prefs_updated_at BEFORE UPDATE ON notification_preferences FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- NOTIFICATIONS
CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recipient_id UUID NOT NULL,
    channel VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    template_id UUID REFERENCES notification_templates(id) ON DELETE SET NULL,
    subject TEXT,
    body TEXT NOT NULL,
    data JSONB NOT NULL DEFAULT '{}',
    to_address VARCHAR(255) NOT NULL,
    category VARCHAR(100) NOT NULL,
    reference_entity_type VARCHAR(100),
    reference_entity_id UUID,
    sent_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    read_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    provider_name VARCHAR(100),
    provider_id VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
);
CREATE INDEX idx_notifications_user_status ON notifications(recipient_id, status);
CREATE INDEX idx_notifications_provider_id ON notifications(provider_name, provider_id);
CREATE TRIGGER set_notifications_updated_at BEFORE UPDATE ON notifications FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- NOTIFICATION_EVENTS
CREATE TABLE notification_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id UUID NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    source VARCHAR(50) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    message TEXT,
    details JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
);
CREATE INDEX idx_notif_events_notification_id ON notification_events(notification_id);
