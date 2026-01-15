-- Migration 020: Notification Webhooks
-- Enables Slack, Discord, and Telegram notifications for deployment events
-- Matches Vercel/Railway webhook functionality

-- ============================================================================
-- WEBHOOK DESTINATIONS TABLE
-- Stores webhook endpoints for notifications
-- ============================================================================
CREATE TABLE IF NOT EXISTS webhook_destinations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,

    -- Webhook identification
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL, -- 'slack', 'discord', 'telegram', 'custom'

    -- Webhook configuration
    webhook_url TEXT NOT NULL, -- The webhook URL (Slack/Discord) or empty for Telegram

    -- Telegram-specific fields
    telegram_bot_token TEXT, -- Encrypted bot token for Telegram
    telegram_chat_id VARCHAR(255), -- Chat/channel ID for Telegram

    -- Custom webhook fields
    custom_headers JSONB DEFAULT '{}', -- Custom headers for webhook requests

    -- Secret for webhook signature verification (optional, for custom webhooks)
    signing_secret TEXT,

    -- Event subscriptions (which events trigger this webhook)
    -- Stored as JSONB array: ["deployment.started", "deployment.succeeded", "deployment.failed"]
    events JSONB NOT NULL DEFAULT '["deployment.succeeded", "deployment.failed"]',

    -- Status
    enabled BOOLEAN NOT NULL DEFAULT true,

    -- Last delivery info
    last_delivery_at TIMESTAMP WITH TIME ZONE,
    last_delivery_status VARCHAR(50), -- 'success', 'failed'
    last_delivery_error TEXT,
    consecutive_failures INTEGER DEFAULT 0,

    -- Auto-disable after too many failures
    auto_disabled_at TIMESTAMP WITH TIME ZONE,

    -- Audit fields
    created_by UUID REFERENCES users(id),
    created_by_email VARCHAR(255),

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Constraints
    CONSTRAINT valid_webhook_type CHECK (type IN ('slack', 'discord', 'telegram', 'custom')),
    CONSTRAINT valid_last_delivery_status CHECK (last_delivery_status IS NULL OR last_delivery_status IN ('success', 'failed')),
    CONSTRAINT unique_webhook_name_per_project UNIQUE (project_id, name),
    CONSTRAINT telegram_requires_bot_token CHECK (type != 'telegram' OR (telegram_bot_token IS NOT NULL AND telegram_chat_id IS NOT NULL)),
    CONSTRAINT non_telegram_requires_url CHECK (type = 'telegram' OR webhook_url != '')
);

-- ============================================================================
-- WEBHOOK DELIVERY LOG TABLE
-- Tracks delivery attempts for debugging and analytics
-- ============================================================================
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id UUID NOT NULL REFERENCES webhook_destinations(id) ON DELETE CASCADE,

    -- Event that triggered the delivery
    event_type VARCHAR(100) NOT NULL,
    event_id UUID, -- Reference to the source event (e.g., deployment_id)

    -- Delivery details
    payload JSONB NOT NULL, -- The payload sent to the webhook

    -- Response info
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- 'pending', 'success', 'failed'
    status_code INTEGER, -- HTTP status code from webhook endpoint
    response_body TEXT, -- Response body (truncated)
    error_message TEXT,

    -- Timing
    attempted_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    duration_ms INTEGER, -- How long the request took

    -- Retry info
    attempt_number INTEGER DEFAULT 1,

    -- Constraints
    CONSTRAINT valid_delivery_status CHECK (status IN ('pending', 'success', 'failed'))
);

-- ============================================================================
-- INDEXES FOR PERFORMANCE
-- ============================================================================
CREATE INDEX IF NOT EXISTS idx_webhook_destinations_project_id ON webhook_destinations(project_id);
CREATE INDEX IF NOT EXISTS idx_webhook_destinations_type ON webhook_destinations(type);
CREATE INDEX IF NOT EXISTS idx_webhook_destinations_enabled ON webhook_destinations(enabled) WHERE enabled = true;

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_webhook_id ON webhook_deliveries(webhook_id);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_status ON webhook_deliveries(status);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_event_type ON webhook_deliveries(event_type);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_attempted_at ON webhook_deliveries(attempted_at);

-- ============================================================================
-- TRIGGER FOR UPDATED_AT
-- ============================================================================
CREATE OR REPLACE FUNCTION update_webhook_destinations_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_update_webhook_destinations_updated_at ON webhook_destinations;
CREATE TRIGGER trigger_update_webhook_destinations_updated_at
    BEFORE UPDATE ON webhook_destinations
    FOR EACH ROW
    EXECUTE FUNCTION update_webhook_destinations_updated_at();

-- ============================================================================
-- COMMENTS FOR DOCUMENTATION
-- ============================================================================
COMMENT ON TABLE webhook_destinations IS 'Webhook endpoints for Slack, Discord, Telegram notifications';
COMMENT ON COLUMN webhook_destinations.type IS 'Webhook type: slack, discord, telegram, custom';
COMMENT ON COLUMN webhook_destinations.events IS 'JSON array of subscribed events';
COMMENT ON COLUMN webhook_destinations.telegram_bot_token IS 'Encrypted Telegram bot token';
COMMENT ON COLUMN webhook_destinations.consecutive_failures IS 'Number of consecutive delivery failures';
COMMENT ON COLUMN webhook_destinations.auto_disabled_at IS 'When webhook was auto-disabled due to failures';

COMMENT ON TABLE webhook_deliveries IS 'Log of webhook delivery attempts';
COMMENT ON COLUMN webhook_deliveries.event_type IS 'Event that triggered delivery (e.g., deployment.succeeded)';
COMMENT ON COLUMN webhook_deliveries.duration_ms IS 'Request duration in milliseconds';
