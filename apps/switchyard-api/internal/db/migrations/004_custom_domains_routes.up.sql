-- Migration 004: Add custom domains and routes tables
-- This adds support for custom domain mapping and HTTP route configuration

-- Custom domains table
CREATE TABLE IF NOT EXISTS custom_domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    domain VARCHAR(255) NOT NULL,
    verified BOOLEAN NOT NULL DEFAULT FALSE,
    tls_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    tls_issuer VARCHAR(100) DEFAULT 'letsencrypt-prod',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    verified_at TIMESTAMP,
    UNIQUE(service_id, environment_id, domain)
);

CREATE INDEX idx_custom_domains_service_id ON custom_domains(service_id);
CREATE INDEX idx_custom_domains_environment_id ON custom_domains(environment_id);
CREATE INDEX idx_custom_domains_domain ON custom_domains(domain);

-- Routes table for custom HTTP path routing
CREATE TABLE IF NOT EXISTS routes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    path VARCHAR(500) NOT NULL,
    path_type VARCHAR(50) NOT NULL DEFAULT 'Prefix', -- 'Prefix', 'Exact', 'ImplementationSpecific'
    port INTEGER NOT NULL DEFAULT 80,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(service_id, environment_id, path)
);

CREATE INDEX idx_routes_service_id ON routes(service_id);
CREATE INDEX idx_routes_environment_id ON routes(environment_id);

-- Add volumes column to services table (JSON array)
ALTER TABLE services ADD COLUMN IF NOT EXISTS volumes JSONB DEFAULT '[]'::jsonb;
