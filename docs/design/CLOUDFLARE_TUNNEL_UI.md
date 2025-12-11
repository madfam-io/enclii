# Cloudflare Tunnel Configuration UI Design

**Status**: Draft
**Author**: Claude (AI Assistant)
**Created**: 2025-12-11
**Related**: [MONOREPO_PROJECT_MODEL.md](./MONOREPO_PROJECT_MODEL.md)

## Overview

This document describes the UI design for configuring Cloudflare tunnels within Enclii. The goal is to abstract away Cloudflare's complexity while giving users full control over their service networking, custom domains, and Zero Trust protection.

### Design Principles

1. **Hide Infrastructure Complexity**: Users manage "domains" not "tunnels"
2. **Zero-Config Defaults**: Platform domains work instantly
3. **Progressive Disclosure**: Advanced options (Zero Trust, custom TLS) available but not required
4. **Real-Time Feedback**: Live status indicators for tunnel health and domain verification
5. **Monorepo-Aware**: Domain patterns that work with multi-service projects

## Current State Analysis

### Existing Infrastructure

From `infra/terraform/cloudflare.tf`:
- Single tunnel per environment defined statically in Terraform
- Ingress rules map hostnames to internal Kubernetes services
- DNS records created via Cloudflare provider
- Zero Trust Access policies for protected routes

### Limitations of Current Approach

1. **Static Configuration**: Adding new services requires Terraform changes
2. **No Self-Service**: Users cannot add custom domains without admin intervention
3. **Hidden Status**: No visibility into tunnel health or domain verification state
4. **No Protection UI**: Zero Trust policies require direct Cloudflare dashboard access

## Data Model Extensions

### New Tables

```sql
-- Platform-level Cloudflare account configuration (admin only)
CREATE TABLE cloudflare_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    account_id TEXT NOT NULL UNIQUE,
    api_token_encrypted TEXT NOT NULL,
    zone_id TEXT,  -- Primary zone for platform domains
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Environment-scoped Cloudflare tunnel
CREATE TABLE cloudflare_tunnels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cloudflare_account_id UUID NOT NULL REFERENCES cloudflare_accounts(id),
    environment_id UUID NOT NULL REFERENCES environments(id),
    tunnel_id TEXT NOT NULL,  -- Cloudflare tunnel ID
    tunnel_name TEXT NOT NULL,
    tunnel_token_encrypted TEXT NOT NULL,
    cname TEXT NOT NULL,  -- e.g., abc123.cfargotunnel.com
    status TEXT NOT NULL DEFAULT 'active',  -- active, degraded, offline
    last_health_check TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(environment_id)  -- One tunnel per environment
);

-- Tunnel ingress rules (dynamically managed)
CREATE TABLE tunnel_ingress_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tunnel_id UUID NOT NULL REFERENCES cloudflare_tunnels(id) ON DELETE CASCADE,
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    hostname TEXT NOT NULL,
    path TEXT DEFAULT '/*',
    origin_service TEXT NOT NULL,  -- e.g., http://api-svc:8080
    priority INT NOT NULL DEFAULT 100,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tunnel_id, hostname, path)
);
```

### Extended Tables

```sql
-- Extend custom_domains table
ALTER TABLE custom_domains ADD COLUMN cloudflare_tunnel_id UUID REFERENCES cloudflare_tunnels(id);
ALTER TABLE custom_domains ADD COLUMN zero_trust_enabled BOOLEAN DEFAULT false;
ALTER TABLE custom_domains ADD COLUMN access_policy_id TEXT;  -- Cloudflare Access policy ID
ALTER TABLE custom_domains ADD COLUMN tls_provider TEXT DEFAULT 'cert-manager';  -- cert-manager, cloudflare-for-saas
ALTER TABLE custom_domains ADD COLUMN dns_verified_at TIMESTAMPTZ;
```

### Go Type Definitions

```go
// pkg/types/networking.go

type CloudflareAccount struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    AccountID string    `json:"account_id"`
    ZoneID    string    `json:"zone_id,omitempty"`
    CreatedAt time.Time `json:"created_at"`
}

type CloudflareTunnel struct {
    ID                   string    `json:"id"`
    CloudflareAccountID  string    `json:"cloudflare_account_id"`
    EnvironmentID        string    `json:"environment_id"`
    TunnelID             string    `json:"tunnel_id"`
    TunnelName           string    `json:"tunnel_name"`
    CNAME                string    `json:"cname"`
    Status               string    `json:"status"`
    LastHealthCheck      time.Time `json:"last_health_check,omitempty"`
    CreatedAt            time.Time `json:"created_at"`
}

type TunnelIngressRule struct {
    ID            string `json:"id"`
    TunnelID      string `json:"tunnel_id"`
    ServiceID     string `json:"service_id"`
    Hostname      string `json:"hostname"`
    Path          string `json:"path"`
    OriginService string `json:"origin_service"`
    Priority      int    `json:"priority"`
}

type ServiceNetworking struct {
    ServiceID      string              `json:"service_id"`
    ServiceName    string              `json:"service_name"`
    Domains        []CustomDomainInfo  `json:"domains"`
    InternalRoutes []InternalRoute     `json:"internal_routes"`
    TunnelStatus   *TunnelStatusInfo   `json:"tunnel_status,omitempty"`
}

type CustomDomainInfo struct {
    ID               string    `json:"id"`
    Domain           string    `json:"domain"`
    Environment      string    `json:"environment"`
    IsPlatformDomain bool      `json:"is_platform_domain"`
    Status           string    `json:"status"`  // pending, verifying, active, error
    TLSStatus        string    `json:"tls_status"`  // pending, provisioning, active
    TLSProvider      string    `json:"tls_provider"`
    ZeroTrustEnabled bool      `json:"zero_trust_enabled"`
    DNSVerifiedAt    time.Time `json:"dns_verified_at,omitempty"`
    VerificationTXT  string    `json:"verification_txt,omitempty"`  // For custom domains
    CreatedAt        time.Time `json:"created_at"`
}

type TunnelStatusInfo struct {
    TunnelID     string    `json:"tunnel_id"`
    Status       string    `json:"status"`
    CNAME        string    `json:"cname"`
    Connectors   int       `json:"connectors"`  // Number of cloudflared instances
    LastHealthy  time.Time `json:"last_healthy"`
}

type InternalRoute struct {
    Path          string `json:"path"`
    TargetService string `json:"target_service"`
    TargetPort    int    `json:"target_port"`
}
```

## API Design

### Networking Endpoints

```
# Get combined networking info for a service
GET /api/v1/services/:service_id/networking
Response: ServiceNetworking

# List domains for a service
GET /api/v1/services/:service_id/domains
Response: { domains: CustomDomainInfo[] }

# Add domain to service
POST /api/v1/services/:service_id/domains
Body: {
    domain: string,          // e.g., "api.mycompany.com" or auto-generated
    environment_id: string,
    is_platform_domain: boolean,
    tls_provider?: "cert-manager" | "cloudflare-for-saas",
    zero_trust_enabled?: boolean
}
Response: CustomDomainInfo

# Verify custom domain DNS
POST /api/v1/domains/:domain_id/verify
Response: { verified: boolean, error?: string }

# Toggle Zero Trust protection
PUT /api/v1/domains/:domain_id/protection
Body: { zero_trust_enabled: boolean }
Response: CustomDomainInfo

# Delete domain
DELETE /api/v1/domains/:domain_id
Response: 204 No Content
```

### Admin Endpoints (Platform Operators)

```
# List Cloudflare accounts
GET /api/v1/admin/cloudflare/accounts
Response: { accounts: CloudflareAccount[] }

# Add Cloudflare account
POST /api/v1/admin/cloudflare/accounts
Body: {
    name: string,
    account_id: string,
    api_token: string,
    zone_id?: string
}
Response: CloudflareAccount

# List tunnels
GET /api/v1/admin/cloudflare/tunnels
Response: { tunnels: CloudflareTunnel[] }

# Create tunnel for environment
POST /api/v1/admin/cloudflare/tunnels
Body: {
    cloudflare_account_id: string,
    environment_id: string,
    tunnel_name: string
}
Response: CloudflareTunnel

# Get tunnel health
GET /api/v1/admin/cloudflare/tunnels/:tunnel_id/health
Response: TunnelStatusInfo
```

## UI Component Design

### Service Detail Page - Networking Tab

```
┌─────────────────────────────────────────────────────────────────────┐
│ Service: switchyard-api                                    [tabs]   │
│ ┌──────────┬────────────┬───────────────┬──────────┐              │
│ │ Overview │ Networking │ Configuration │ Metrics  │              │
│ └──────────┴────────────┴───────────────┴──────────┘              │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  DOMAINS                                           [+ Add Domain]   │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │ Domain                    Env    Status   TLS    Protection │   │
│  ├─────────────────────────────────────────────────────────────┤   │
│  │ api.enclii.dev           prod   ● Active  ✓ TLS  🔒 On     │   │
│  │ api-staging.enclii.dev   stage  ● Active  ✓ TLS  ○ Off     │   │
│  │ api.mycompany.com        prod   ◐ Verify  ⏳...   ○ Off     │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐ │
│  │ 📋 DNS Instructions for api.mycompany.com                     │ │
│  │                                                               │ │
│  │ Add these DNS records to verify ownership:                    │ │
│  │                                                               │ │
│  │ Type: TXT                                                     │ │
│  │ Name: _enclii-verification.api                                │ │
│  │ Value: enclii-verify=abc123def456                 [Copy]      │ │
│  │                                                               │ │
│  │ Then add a CNAME to route traffic:                           │ │
│  │                                                               │ │
│  │ Type: CNAME                                                   │ │
│  │ Name: api                                                     │ │
│  │ Value: tunnel-xyz.cfargotunnel.com               [Copy]      │ │
│  │                                                               │ │
│  │                              [Check Verification Status]      │ │
│  └───────────────────────────────────────────────────────────────┘ │
│                                                                     │
│  INTERNAL ROUTING                                                   │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │ Cluster Service: switchyard-api.enclii.svc.cluster.local    │   │
│  │ Container Port: 8080                                         │   │
│  │ Tunnel Status: ● Connected (3 connectors)                    │   │
│  │ Tunnel CNAME: abc123.cfargotunnel.com                       │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Add Domain Modal

```
┌─────────────────────────────────────────────────────────────────┐
│ Add Domain                                               [X]    │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Domain Type                                                    │
│  ┌─────────────────────┐  ┌─────────────────────┐              │
│  │ ● Platform Domain   │  │ ○ Custom Domain     │              │
│  │   *.enclii.dev      │  │   Your own domain   │              │
│  └─────────────────────┘  └─────────────────────┘              │
│                                                                 │
│  Environment                                                    │
│  ┌─────────────────────────────────────────────────┐           │
│  │ Production                                   ▼  │           │
│  └─────────────────────────────────────────────────┘           │
│                                                                 │
│  Subdomain                                                      │
│  ┌──────────────────────────┐                                  │
│  │ api                      │ .enclii.dev                      │
│  └──────────────────────────┘                                  │
│  💡 Suggested: api (based on service name)                      │
│                                                                 │
│  Options                                                        │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ ☐ Enable Zero Trust Protection                          │   │
│  │   Require authentication to access this domain          │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│                              [Cancel]  [Add Domain]             │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Custom Domain Flow

```
┌─────────────────────────────────────────────────────────────────┐
│ Add Domain                                               [X]    │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Domain Type                                                    │
│  ┌─────────────────────┐  ┌─────────────────────┐              │
│  │ ○ Platform Domain   │  │ ● Custom Domain     │              │
│  │   *.enclii.dev      │  │   Your own domain   │              │
│  └─────────────────────┘  └─────────────────────┘              │
│                                                                 │
│  Environment                                                    │
│  ┌─────────────────────────────────────────────────┐           │
│  │ Production                                   ▼  │           │
│  └─────────────────────────────────────────────────┘           │
│                                                                 │
│  Your Domain                                                    │
│  ┌─────────────────────────────────────────────────┐           │
│  │ api.mycompany.com                               │           │
│  └─────────────────────────────────────────────────┘           │
│                                                                 │
│  TLS Certificate                                                │
│  ┌─────────────────────┐  ┌─────────────────────┐              │
│  │ ● Let's Encrypt     │  │ ○ Cloudflare SaaS   │              │
│  │   Auto-renewed      │  │   Instant issuance  │              │
│  └─────────────────────┘  └─────────────────────┘              │
│                                                                 │
│  Options                                                        │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ ☐ Enable Zero Trust Protection                          │   │
│  │   Require authentication to access this domain          │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│                              [Cancel]  [Add Domain]             │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Status Indicators

| Status | Icon | Color | Meaning |
|--------|------|-------|---------|
| Active | ● | Green | Domain is live and serving traffic |
| Verifying | ◐ | Yellow | Waiting for DNS verification |
| Pending | ○ | Gray | Domain created but not configured |
| Error | ● | Red | Configuration or verification error |
| TLS Active | ✓ | Green | Certificate provisioned |
| TLS Pending | ⏳ | Yellow | Certificate being issued |
| Protected | 🔒 | Blue | Zero Trust enabled |
| Unprotected | ○ | Gray | No Zero Trust |

## Reconciliation Architecture

### Domain Reconciler

```go
// internal/reconcilers/domain_reconciler.go

func (r *DomainReconciler) Reconcile(ctx context.Context, domain *types.CustomDomain) error {
    // 1. Verify DNS if custom domain
    if !domain.IsPlatformDomain && domain.DNSVerifiedAt.IsZero() {
        verified, err := r.verifyDNS(ctx, domain)
        if err != nil || !verified {
            return r.updateStatus(ctx, domain, "verifying")
        }
        domain.DNSVerifiedAt = time.Now()
    }

    // 2. Create/update tunnel ingress rule
    if err := r.ensureIngressRule(ctx, domain); err != nil {
        return err
    }

    // 3. Provision TLS certificate
    if err := r.ensureTLS(ctx, domain); err != nil {
        return r.updateStatus(ctx, domain, "tls_pending")
    }

    // 4. Configure Zero Trust if enabled
    if domain.ZeroTrustEnabled {
        if err := r.ensureAccessPolicy(ctx, domain); err != nil {
            return err
        }
    }

    return r.updateStatus(ctx, domain, "active")
}
```

### Tunnel Health Monitor

```go
// internal/reconcilers/tunnel_health.go

func (m *TunnelHealthMonitor) CheckHealth(ctx context.Context, tunnel *types.CloudflareTunnel) error {
    // Call Cloudflare API to get tunnel status
    status, err := m.cfClient.GetTunnelStatus(ctx, tunnel.TunnelID)
    if err != nil {
        return err
    }

    // Update tunnel status in database
    tunnel.Status = status.Health  // "healthy", "degraded", "down"
    tunnel.LastHealthCheck = time.Now()

    return m.db.UpdateTunnel(ctx, tunnel)
}
```

## Integration with Monorepo Model

When importing a monorepo project with multiple services:

### Automatic Domain Suggestions

```
Project: janua (monorepo)
├── apps/api      → api.janua.enclii.dev
├── apps/dashboard → app.janua.enclii.dev
└── apps/docs     → docs.janua.enclii.dev
```

### Deployment Group Domain Activation

When deploying a monorepo with deployment groups:

1. **Pre-deployment**: Verify all custom domains have valid DNS
2. **During deployment**: Keep old ingress rules active
3. **Post-deployment**: Atomically switch ingress rules to new services
4. **Rollback**: Revert ingress rules to previous service versions

### Service Dependency Routing

For services with dependencies:

```yaml
# If auth-service is a dependency of api-service
dependencies:
  - auth-service (must route before api-service)

# Routing priority:
# 1. auth.project.enclii.dev → auth-service (priority: 100)
# 2. api.project.enclii.dev → api-service (priority: 200)
```

## Implementation Phases

### Phase 1: Backend Foundation (Week 1)

1. Add database migrations for new tables
2. Implement Cloudflare API client
3. Add domain CRUD endpoints
4. Basic domain verification flow

**Deliverables:**
- `POST /api/v1/services/:id/domains`
- `GET /api/v1/services/:id/networking`
- `POST /api/v1/domains/:id/verify`
- Database migrations

### Phase 2: UI Components (Week 2)

1. NetworkingTab component
2. DomainsList with status badges
3. AddDomainModal (platform and custom)
4. DNS instructions card
5. Tunnel status display

**Deliverables:**
- `components/networking/NetworkingTab.tsx`
- `components/networking/DomainsList.tsx`
- `components/networking/AddDomainModal.tsx`
- `components/networking/DNSInstructions.tsx`

### Phase 3: Reconciliation (Week 3)

1. Domain reconciler for DNS verification
2. Ingress rule management
3. TLS certificate automation
4. Tunnel health monitoring

**Deliverables:**
- `reconcilers/domain_reconciler.go`
- `reconcilers/tunnel_health.go`
- Cloudflare API integration

### Phase 4: Zero Trust Integration (Week 4)

1. Access policy creation API
2. Zero Trust toggle UI
3. Policy templates (public, authenticated, admin-only)
4. Session management integration

**Deliverables:**
- `PUT /api/v1/domains/:id/protection`
- Zero Trust policy templates
- Integration with Janua authentication

## Security Considerations

### API Token Storage

- Cloudflare API tokens encrypted at rest using AES-256-GCM
- Tokens never returned in API responses
- Separate tokens per environment with least-privilege scopes

### Domain Verification

- TXT record verification prevents domain hijacking
- 24-hour verification window before cleanup
- Rate limiting on verification attempts

### Zero Trust Policies

- Default policy: Allow authenticated users
- Custom policies for admin-only endpoints
- Integration with Janua for SSO

## Appendix: Cloudflare API Integration

### Required API Scopes

```
Zone:Read
Zone:Edit
DNS:Read
DNS:Edit
Cloudflare Tunnel:Read
Cloudflare Tunnel:Edit
Access: Apps and Policies:Edit
SSL and Certificates:Read
SSL and Certificates:Edit
```

### Tunnel Configuration Example

```json
{
  "config": {
    "ingress": [
      {
        "hostname": "api.enclii.dev",
        "service": "http://switchyard-api:8080",
        "originRequest": {
          "noTLSVerify": true
        }
      },
      {
        "hostname": "app.enclii.dev",
        "service": "http://switchyard-ui:3000"
      },
      {
        "service": "http_status:404"
      }
    ]
  }
}
```

### Zero Trust Access Policy Example

```json
{
  "name": "api.enclii.dev",
  "decision": "allow",
  "include": [
    {
      "email_domain": {
        "domain": "madfam.io"
      }
    }
  ],
  "require": [
    {
      "login_method": {
        "id": "janua-oidc-connector-id"
      }
    }
  ]
}
```
