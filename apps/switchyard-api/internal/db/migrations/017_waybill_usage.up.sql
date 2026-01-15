-- Waybill usage tracking and billing tables

-- Usage events (append-only, high volume)
CREATE TABLE usage_events (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    team_id UUID REFERENCES teams(id) ON DELETE SET NULL,
    event_type VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id UUID NOT NULL,
    resource_name VARCHAR(255),
    metrics JSONB NOT NULL DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for event queries
CREATE INDEX idx_usage_events_project ON usage_events(project_id);
CREATE INDEX idx_usage_events_timestamp ON usage_events(timestamp DESC);
CREATE INDEX idx_usage_events_type ON usage_events(event_type);
CREATE INDEX idx_usage_events_unprocessed ON usage_events(timestamp) WHERE processed_at IS NULL;
CREATE INDEX idx_usage_events_resource ON usage_events(resource_type, resource_id);

-- Hourly aggregated usage
CREATE TABLE hourly_usage (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    metric_type VARCHAR(50) NOT NULL,
    value DECIMAL(20, 6) NOT NULL DEFAULT 0,
    hour TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    UNIQUE(project_id, metric_type, hour)
);

CREATE INDEX idx_hourly_usage_project ON hourly_usage(project_id);
CREATE INDEX idx_hourly_usage_hour ON hourly_usage(hour DESC);
CREATE INDEX idx_hourly_usage_lookup ON hourly_usage(project_id, hour, metric_type);

-- Daily aggregated usage
CREATE TABLE daily_usage (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    metric_type VARCHAR(50) NOT NULL,
    value DECIMAL(20, 6) NOT NULL DEFAULT 0,
    date DATE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    UNIQUE(project_id, metric_type, date)
);

CREATE INDEX idx_daily_usage_project ON daily_usage(project_id);
CREATE INDEX idx_daily_usage_date ON daily_usage(date DESC);

-- Pricing plans
CREATE TABLE pricing_plans (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    price_monthly DECIMAL(10, 2) NOT NULL,
    includes JSONB NOT NULL DEFAULT '{}',
    overage_rates JSONB NOT NULL DEFAULT '{}',
    features JSONB DEFAULT '[]',
    is_active BOOLEAN DEFAULT true,
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Insert default plans
INSERT INTO pricing_plans (id, name, description, price_monthly, includes, overage_rates, features, sort_order) VALUES
('hobby', 'Hobby', 'For personal projects and experiments', 5.00,
    '{"compute_gb_hours": 500, "build_minutes": 500, "storage_gb": 1, "bandwidth_gb": 100, "custom_domains": 1}',
    '{"compute_per_gb_hour": 0.000463, "build_per_minute": 0.01, "storage_per_gb": 0.25, "bandwidth_per_gb": 0.10}',
    '[]', 1),
('pro', 'Pro', 'For production applications', 20.00,
    '{"compute_gb_hours": 2000, "build_minutes": 2000, "storage_gb": 10, "bandwidth_gb": 500, "custom_domains": -1}',
    '{"compute_per_gb_hour": 0.000463, "build_per_minute": 0.01, "storage_per_gb": 0.20, "bandwidth_per_gb": 0.08}',
    '["Priority support", "Team collaboration", "Advanced metrics", "Custom health checks"]', 2),
('team', 'Team', 'For teams and organizations', 50.00,
    '{"compute_gb_hours": 5000, "build_minutes": 5000, "storage_gb": 50, "bandwidth_gb": 1000, "custom_domains": -1, "team_members": 10}',
    '{"compute_per_gb_hour": 0.0004, "build_per_minute": 0.008, "storage_per_gb": 0.15, "bandwidth_per_gb": 0.05}',
    '["Everything in Pro", "SSO/SAML", "Audit logs", "SLA guarantee", "Dedicated support"]', 3);

-- Subscriptions
CREATE TABLE subscriptions (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    team_id UUID REFERENCES teams(id) ON DELETE SET NULL,
    plan_id VARCHAR(50) NOT NULL REFERENCES pricing_plans(id),
    stripe_customer_id VARCHAR(255),
    stripe_subscription_id VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    current_period_start TIMESTAMP WITH TIME ZONE,
    current_period_end TIMESTAMP WITH TIME ZONE,
    cancel_at TIMESTAMP WITH TIME ZONE,
    cancelled_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    UNIQUE(project_id)
);

CREATE INDEX idx_subscriptions_stripe ON subscriptions(stripe_customer_id);
CREATE INDEX idx_subscriptions_status ON subscriptions(status);

-- Billing records (monthly invoices)
CREATE TABLE billing_records (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    subscription_id UUID REFERENCES subscriptions(id) ON DELETE SET NULL,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,

    -- Base plan
    plan_id VARCHAR(50) REFERENCES pricing_plans(id),
    plan_amount DECIMAL(10, 2) NOT NULL DEFAULT 0,

    -- Usage metrics
    compute_gb_hours DECIMAL(20, 6) DEFAULT 0,
    build_minutes DECIMAL(20, 6) DEFAULT 0,
    storage_gb_hours DECIMAL(20, 6) DEFAULT 0,
    bandwidth_gb DECIMAL(20, 6) DEFAULT 0,
    custom_domains INTEGER DEFAULT 0,

    -- Costs
    compute_cost DECIMAL(10, 2) DEFAULT 0,
    build_cost DECIMAL(10, 2) DEFAULT 0,
    storage_cost DECIMAL(10, 2) DEFAULT 0,
    bandwidth_cost DECIMAL(10, 2) DEFAULT 0,

    -- Totals
    subtotal DECIMAL(10, 2) NOT NULL DEFAULT 0,
    credits_applied DECIMAL(10, 2) DEFAULT 0,
    total_amount DECIMAL(10, 2) NOT NULL DEFAULT 0,
    currency VARCHAR(3) DEFAULT 'USD',

    -- Payment
    stripe_invoice_id VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'draft',
    paid_at TIMESTAMP WITH TIME ZONE,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    UNIQUE(project_id, period_start, period_end)
);

CREATE INDEX idx_billing_records_project ON billing_records(project_id);
CREATE INDEX idx_billing_records_period ON billing_records(period_start, period_end);
CREATE INDEX idx_billing_records_status ON billing_records(status);
CREATE INDEX idx_billing_records_stripe ON billing_records(stripe_invoice_id);

-- Credits/promotional balance
CREATE TABLE credits (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    amount DECIMAL(10, 2) NOT NULL,
    description TEXT,
    expires_at TIMESTAMP WITH TIME ZONE,
    used_amount DECIMAL(10, 2) DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_credits_project ON credits(project_id);
CREATE INDEX idx_credits_active ON credits(project_id, expires_at) WHERE used_amount < amount;

-- Usage alerts
CREATE TABLE usage_alerts (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    metric_type VARCHAR(50) NOT NULL,
    threshold_value DECIMAL(20, 6) NOT NULL,
    threshold_percent INTEGER,
    notification_sent_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    UNIQUE(project_id, metric_type, threshold_value)
);

-- Trigger to update updated_at
CREATE OR REPLACE FUNCTION update_waybill_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER subscriptions_updated_at
    BEFORE UPDATE ON subscriptions
    FOR EACH ROW
    EXECUTE FUNCTION update_waybill_updated_at();

CREATE TRIGGER billing_records_updated_at
    BEFORE UPDATE ON billing_records
    FOR EACH ROW
    EXECUTE FUNCTION update_waybill_updated_at();

CREATE TRIGGER pricing_plans_updated_at
    BEFORE UPDATE ON pricing_plans
    FOR EACH ROW
    EXECUTE FUNCTION update_waybill_updated_at();

-- Usage summary view
CREATE VIEW project_usage_summary AS
SELECT
    p.id AS project_id,
    p.name AS project_name,
    s.plan_id,
    s.status AS subscription_status,
    COALESCE(SUM(hu.value) FILTER (WHERE hu.metric_type = 'compute_gb_hours'), 0) AS compute_gb_hours,
    COALESCE(SUM(hu.value) FILTER (WHERE hu.metric_type = 'build_minutes'), 0) AS build_minutes,
    COALESCE(SUM(hu.value) FILTER (WHERE hu.metric_type = 'storage_gb_hours'), 0) AS storage_gb_hours,
    COALESCE(SUM(hu.value) FILTER (WHERE hu.metric_type = 'bandwidth_gb'), 0) AS bandwidth_gb
FROM projects p
LEFT JOIN subscriptions s ON p.id = s.project_id
LEFT JOIN hourly_usage hu ON p.id = hu.project_id
    AND hu.hour >= DATE_TRUNC('month', CURRENT_TIMESTAMP)
GROUP BY p.id, p.name, s.plan_id, s.status;

COMMENT ON TABLE usage_events IS 'Raw usage events from platform services';
COMMENT ON TABLE hourly_usage IS 'Hourly aggregated usage metrics';
COMMENT ON TABLE daily_usage IS 'Daily aggregated usage metrics';
COMMENT ON TABLE pricing_plans IS 'Available subscription plans';
COMMENT ON TABLE subscriptions IS 'Project subscriptions to pricing plans';
COMMENT ON TABLE billing_records IS 'Monthly billing records and invoices';
COMMENT ON TABLE credits IS 'Promotional credits and balances';
COMMENT ON TABLE usage_alerts IS 'Usage threshold alerts configuration';
