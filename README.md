# Enclii

> **The Railway-style platform with $100/month production infrastructure.**
> *Production-grade Kubernetes orchestration on Hetzner + Cloudflare.*

[![Production Readiness](https://img.shields.io/badge/production%20ready-70%25-yellow)](./docs/production/PRODUCTION_READINESS_AUDIT.md)
[![Infrastructure](https://img.shields.io/badge/infrastructure-Hetzner%20%2B%20Cloudflare-blue)](./docs/production/PRODUCTION_DEPLOYMENT_ROADMAP.md)
[![Auth](https://img.shields.io/badge/auth-JWT%20(RS256)-orange)](./docs/production/PRODUCTION_READINESS_AUDIT.md)
[![Cost](https://img.shields.io/badge/monthly%20cost-%24100-success)](./docs/production/PRODUCTION_DEPLOYMENT_ROADMAP.md)

**Status:** Alpha (70% production-ready) | [Production Roadmap →](./docs/production/PRODUCTION_DEPLOYMENT_ROADMAP.md)
**Authentication:** JWT (RS256) - Janua integration planned for Weeks 3-4
**Infrastructure:** Hetzner + Cloudflare + Ubicloud (~$100/month)

---

## What is Enclii?

Enclii is a **Railway-style Platform-as-a-Service** that runs on cost-effective infrastructure ($100/month vs $2,220 for Railway + Auth0). It deploys containerized services with enterprise-grade security, auto-scaling, and zero vendor lock-in.

### The Dogfooding Strategy (Planned)

> **Goal:** "We'll run our entire platform on Enclii, authenticated by Janua. We'll be our own most demanding customer."

**Planned Services** (Weeks 5-6 of roadmap):
- 🔲 **Control Plane API** (`api.enclii.io`) → Deploy via Enclii itself
- 🔲 **Web Dashboard** (`app.enclii.io`) → Deploy via Enclii itself
- 🔲 **Authentication** (`auth.enclii.io`) → Janua (from [separate repo](https://github.com/madfam-io/janua))
- 🔲 **Landing Page** (`enclii.io`) → Deploy via Enclii itself
- 🔲 **Documentation** (`docs.enclii.io`) → Deploy via Enclii itself
- 🔲 **Status Page** (`status.enclii.io`) → Deploy via Enclii itself

**Current Status:** Service specs ready in `dogfooding/` directory. Implementation scheduled for Weeks 5-6 after Janua integration (Weeks 3-4). [See dogfooding plan →](./docs/guides/DOGFOODING_GUIDE.md)

---

## Key Features

### 🏗️ Production-Ready Infrastructure

**Cost-Optimized Stack** (~$100/month):
- **Hetzner Cloud** (3x CPX31) - AMD EPYC, NVMe SSD - $45/mo
- **Cloudflare Tunnel** - Replaces expensive load balancers - $0
- **Cloudflare for SaaS** - 100 custom domains FREE - $0
- **Cloudflare R2** - Zero-egress object storage - $5/mo
- **Ubicloud PostgreSQL** - Managed DB on Hetzner infra - $50/mo
- **Redis Sentinel** - Self-hosted HA caching - $0

**vs Traditional Stack** ($2,220/month):
- Railway: $2,000+/month
- Auth0: $220+/month
- **5-Year Savings: $127,200** 💰

[View infrastructure details →](./docs/production/PRODUCTION_DEPLOYMENT_ROADMAP.md)

### 🔐 Authentication & Security

**Current Implementation:**
- **JWT Authentication** with RSA signing (RS256)
- **RBAC** with admin/developer/viewer roles
- **Secure session management** with Redis
- **API key support** for CI/CD integration

**Planned (Weeks 3-4): Janua Integration**
- Self-hosted OAuth 2.0 / OIDC provider
- Multi-tenant organization support
- Replace Auth0/Clerk dependency
- Built from: [github.com/madfam-io/janua](https://github.com/madfam-io/janua)
- Deploy via Enclii itself (dogfooding)

**Why Janua (when integrated):**
- ✅ No Auth0/Clerk vendor lock-in
- ✅ No per-MAU costs ($0 vs $220+/month)
- ✅ Full control over auth flows
- ✅ Multi-tenant ready out of the box

[View Janua integration plan →](./docs/production/PRODUCTION_READINESS_AUDIT.md)

### 🚀 Multi-Tenant SaaS Ready

**Cloudflare for SaaS** enables unlimited custom domains:
- ✅ First **100 domains FREE**
- ✅ $0.10/domain after that
- ✅ Auto-provisioned SSL in ~30 seconds
- ✅ No cert-manager rate limits
- ✅ No Kubernetes overhead

**Perfect for:** SaaS platforms serving multiple customers with custom domains.

### 📦 Complete Feature Set

**Developer Experience:**
- Railway-style CLI (`enclii init`, `enclii up`, `enclii deploy`)
- Auto-detect buildpacks (Nixpacks, Buildpacks, Dockerfile)
- Preview environments on every PR
- Real-time log streaming

**Security & Compliance:**
- RS256 JWT authentication with RSA signing
- RBAC with admin/developer/viewer roles
- Rate limiting (1,000-10,000 req/min)
- Security headers (HSTS, CSP, X-Frame-Options)
- Audit logging with immutable trail
- Image signing (Cosign) + SBOM (CycloneDX)

**Operations:**
- Canary deployments with auto-rollback
- Blue-green deployment strategy
- Horizontal pod autoscaling (HPA)
- Redis caching with tag-based invalidation
- PgBouncer connection pooling
- Prometheus + Grafana monitoring

**Multi-Tenancy:**
- NetworkPolicies (zero-trust networking)
- ResourceQuotas per tenant
- Per-tenant metrics and logging
- Cost tracking and showback

---

## Architecture

### Repository Structure (Monorepo)

```
enclii/
├── apps/
│   ├── switchyard-api/        # Control plane API (Go)
│   ├── switchyard-ui/         # Web dashboard (Next.js)
│   ├── roundhouse/            # Build workers (Go)
│   └── reconcilers/           # Kubernetes controllers (Go)
├── packages/
│   └── cli/                   # `enclii` CLI (Go)
├── infra/
│   ├── k8s/                   # Kubernetes manifests
│   │   ├── base/              # Core infrastructure
│   │   ├── staging/           # Staging overlays
│   │   └── production/        # Production overlays
│   └── terraform/             # Infrastructure as Code
├── dogfooding/                # ⭐ Service specs for self-hosting
│   ├── switchyard-api.yaml    # Control plane (from this repo)
│   ├── switchyard-ui.yaml     # Web UI (from this repo)
│   ├── janua.yaml             # Auth (from github.com/madfam-io/janua)
│   ├── landing-page.yaml      # Marketing site
│   ├── docs-site.yaml         # Documentation
│   └── status-page.yaml       # Status monitoring
├── docs/                      # Documentation
├── examples/                  # Sample service specs
└── DOGFOODING_GUIDE.md        # Self-hosting strategy
```

### Component Names

**Production Names** (all railroad-themed 🚂):
- **Switchyard** - Control plane API
- **Conductor** - CLI (`enclii` command)
- **Roundhouse** - Build/provenance/signing workers
- **Junctions** - Ingress/routing/DNS/TLS
- **Timetable** - Cron jobs and scheduled tasks
- **Lockbox** - Secrets management
- **Signal** - Observability (logs/metrics/traces)
- **Waybill** - Cost tracking and showback

---

## Production Readiness

### Current Status: 70% Ready

From [PRODUCTION_READINESS_AUDIT.md](./PRODUCTION_READINESS_AUDIT.md):

**Infrastructure Compatibility: 75%**
- ✅ Cloud-agnostic (no vendor lock-in)
- ✅ PgBouncer-compatible database pooling
- ⚠️ Cloudflare Tunnel integration needed (3 days)
- ⚠️ R2 object storage for SBOMs needed (2 days)
- ⚠️ Redis Sentinel HA needed (1 day)

**Janua Integration: 65%**
- ✅ Already using RS256 JWT (perfect compatibility!)
- ✅ Database has `oidc_sub` field ready
- ❌ JWKS provider not implemented
- ❌ OAuth handlers missing
- ❌ Frontend needs oidc-client-ts rewrite

### Timeline to Production: 6-8 Weeks

**Week 1-2:** Infrastructure (Hetzner + Cloudflare + Ubicloud)
**Week 3-4:** Security hardening (NetworkPolicies, admission control)
**Week 5-6:** Janua integration + Dogfooding setup
**Week 7-8:** Load testing + Security audit + **GO LIVE** 🚀

[View detailed roadmap →](./docs/production/PRODUCTION_DEPLOYMENT_ROADMAP.md)

---

## Quick Start

### Prerequisites

**Core:**
- Docker ≥ 24
- kubectl ≥ 1.29
- kind ≥ 0.23 (for local dev)
- Helm ≥ 3.14

**Languages:**
- Go ≥ 1.22
- Node.js ≥ 20
- pnpm ≥ 9

**macOS:**
```bash
brew install go node pnpm kind helm kubectl docker
```

### NPM Registry Configuration

Enclii uses MADFAM's private npm registry for internal packages. Configure your `.npmrc`:

```bash
# Add to your project's .npmrc or ~/.npmrc
@madfam:registry=https://npm.madfam.io
@enclii:registry=https://npm.madfam.io
@janua:registry=https://npm.madfam.io
//npm.madfam.io/:_authToken=${NPM_MADFAM_TOKEN}
```

Set the `NPM_MADFAM_TOKEN` environment variable with your registry token.

**Note:** Enclii also hosts the npm.madfam.io registry via Verdaccio. See [NPM Registry Implementation](./docs/NPM_REGISTRY_IMPLEMENTATION.md) for details.

### Local Development (10 minutes)

```bash
# 1. Clone and bootstrap
git clone https://github.com/madfam-io/enclii
cd enclii
make bootstrap  # Install dependencies

# 2. Start local Kubernetes
make kind-up         # Create kind cluster
make infra-dev       # Install NGINX Ingress, cert-manager, Prometheus
make dns-dev         # Configure dev DNS

# 3. Run the platform
make run-switchyard  # Control plane API on :8001
make run-ui          # Web UI on http://localhost:8030
make run-reconcilers # Kubernetes controllers

# 4. Try the CLI
make build-cli
./bin/enclii init                  # Scaffold a service
./bin/enclii up                    # Deploy preview environment
./bin/enclii deploy --env prod     # Deploy to production
./bin/enclii logs api -f           # Tail logs
```

[View detailed setup →](./docs/getting-started/QUICKSTART.md)

### Production Deployment

See [Production Deployment Roadmap](./docs/production/PRODUCTION_DEPLOYMENT_ROADMAP.md) for the complete 8-week implementation plan.

**Bootstrap (Week 1-2):**
```bash
# Provision Hetzner cluster
hcloud server create --name enclii-node-{1,2,3} --type cpx31

# Configure Cloudflare Tunnel
cloudflared tunnel create enclii-production

# Deploy infrastructure
kubectl apply -k infra/k8s/production
```

**Dogfooding (Week 5-6):**
```bash
# Import service specs
./bin/enclii service create --file dogfooding/switchyard-api.yaml
./bin/enclii service create --file dogfooding/janua.yaml

# Deploy via Enclii itself
./bin/enclii deploy --service switchyard-api --env production
./bin/enclii deploy --service janua --env production

# ✅ Enclii now deploys Enclii!
```

---

## CLI Reference

```bash
enclii init              # Scaffold a new service from template
enclii up                # Build & deploy current branch (preview)
enclii deploy            # Deploy to production with canary
enclii logs <service>    # Stream logs
enclii ps                # List services, versions, health
enclii scale             # Configure autoscaling
enclii secrets set       # Manage secrets
enclii rollback          # Revert to previous release
enclii auth login        # Authenticate via Janua OAuth
```

**Common workflows:**

```bash
# Deploy with canary strategy
enclii deploy --env prod --strategy canary --wait

# Set secrets
enclii secrets set DATABASE_URL=postgres://... --env prod

# Custom domain
enclii routes add --host api.example.com --service api --env prod

# Scale to 5 replicas
enclii scale --min 5 --max 10 --service api --env prod
```

---

## Documentation

**📚 [Complete Documentation Index →](./docs/README.md)**

**Getting Started:**
- [Production Deployment Roadmap](./docs/production/PRODUCTION_DEPLOYMENT_ROADMAP.md) - 8-week plan
- [Production Readiness Audit](./docs/production/PRODUCTION_READINESS_AUDIT.md) - Current state
- [Dogfooding Guide](./docs/guides/DOGFOODING_GUIDE.md) - Self-hosting strategy
- [Quick Start](./docs/getting-started/QUICKSTART.md) - Local dev in 10 minutes

**Architecture:**
- [Architecture Overview](./docs/architecture/ARCHITECTURE.md) - System design
- [API Documentation](./docs/architecture/API.md) - REST API reference
- [Development Guide](./docs/getting-started/DEVELOPMENT.md) - Contributing guide

**Audits & Reports:**
- [Audit Navigation](./docs/audits/README.md) - Browse all audit reports
- [Master Audit Report](./docs/audits/MASTER_REPORT.md) - Comprehensive overview

**Operations:**
- [Deployment Guide](./infra/DEPLOYMENT.md) - Production ops
- [Secrets Management](./infra/SECRETS_MANAGEMENT.md) - Lockbox integration

---

## Key Differentiators

### vs Railway ($2,000+/month)

| Feature | Railway | Enclii |
|---------|---------|--------|
| **Cost** | $2,000+/mo | **$100/mo** 💰 |
| **Custom Domains** | Limited, expensive | **100 FREE** (Cloudflare for SaaS) |
| **Vendor Lock-In** | Full lock-in | **None** (portable Kubernetes) |
| **Auth** | Bring your own ($220/mo for Auth0) | **Janua included** ($0) |
| **Bandwidth** | Expensive egress | **Zero egress** (Cloudflare R2) |
| **Multi-Tenancy** | Not designed for it | **Built-in** (NetworkPolicies, quotas) |
| **Self-Hosting** | Impossible | **Fully self-hosted** |

### vs Vercel + Clerk

| Feature | Vercel + Clerk | Enclii |
|---------|----------------|--------|
| **Cost** | $2,500/mo | **$100/mo** 💰 |
| **Backend Support** | Limited (Functions) | **Full container support** |
| **Database** | Bring your own | **Managed PostgreSQL included** |
| **Auth** | Clerk ($300+/mo) | **Janua included** ($0) |
| **Control** | SaaS (no control) | **Full control** (self-hosted) |

### The Self-Hosted Advantage

**Why self-hosted infrastructure matters:**

1. **Cost Control** - $100/month vs $2,220 (95% savings)
2. **No Vendor Lock-In** - Portable Kubernetes, standard tools
3. **Data Sovereignty** - Your infrastructure, your rules
4. **Unlimited Scale** - No artificial SaaS limits
5. **Self-Hosted Auth** - No Auth0/Clerk dependency
6. **Custom Compliance** - Meet any regulatory requirement

---

## Roadmap

### Phase 1: Alpha (Current - 70% Complete)

- ✅ Control plane API (Switchyard)
- ✅ CLI (`enclii init/up/deploy/logs`)
- ✅ Web UI (Next.js dashboard)
- ✅ JWT authentication (RS256)
- ✅ RBAC (admin/developer/viewer)
- ✅ Preview environments
- ✅ Kubernetes reconcilers
- ⚠️ Cloudflare Tunnel (3 days)
- ⚠️ R2 object storage (2 days)
- ⚠️ Redis Sentinel HA (1 day)

### Phase 2: Janua Integration (Weeks 3-4)

- ❌ JWKS provider for Janua
- ❌ OAuth 2.0 handlers
- ❌ Frontend oidc-client-ts integration
- ❌ Janua deployment on Enclii
- ❌ Multi-tenant organization support

### Phase 3: Production (Weeks 5-8)

- ❌ Dogfooding (Enclii deploys itself)
- ❌ Load testing (1,000 RPS)
- ❌ Security audit ($2,000 third-party)
- ❌ Canary deployments with auto-rollback
- ❌ Blue-green deployment strategy
- ❌ Disaster recovery runbooks

### Phase 4: GA (Post-Launch)

- Multi-region deployments
- KEDA autoscaling (custom metrics)
- Cost showback and budget alerts
- Policy-as-code gates (OPA)
- Cron jobs and scheduled tasks
- SOC 2 compliance documentation

[View detailed roadmap →](./docs/production/PRODUCTION_DEPLOYMENT_ROADMAP.md)

---

## Contributing

**Internal only** for now. Before contributing:

1. Read [CLAUDE.md](./CLAUDE.md) for project conventions
2. Run `make precommit` before pushing
3. Use conventional commits for changelog
4. Open draft PR early for feedback

---

## Security

**Supply Chain Security:**
- SBOM generation (CycloneDX format)
- Image signing (Cosign with RSA keys)
- Base image rotation every 30 days
- Vulnerability scanning (Trivy)

**Runtime Security:**
- Zero-trust networking (NetworkPolicies)
- Non-root containers (UID 65532)
- Read-only root filesystem
- Dropped Linux capabilities
- Seccomp profiles enabled

**Responsible Disclosure:**
Email: [security@enclii.dev](mailto:security@enclii.dev)

---

## The Vision: Dogfooding as Competitive Advantage

**Goal (Weeks 5-8):** Run our entire production infrastructure on Enclii, authenticated by Janua.

When we launch, prospects will ask **"Can Enclii handle production?"**

We'll answer with verifiable proof:
> "We run our entire production on Enclii. Here's our status page showing 99.95% uptime. We deploy 10-20 times per day with zero downtime using our own platform."

**What we're building (service specs ready in `dogfooding/`):**
- Control Plane API at api.enclii.io
- Web Dashboard at app.enclii.io
- Janua Auth at auth.enclii.io
- Public status page at status.enclii.io

**Why this matters:**
- Customer confidence: "If they trust it, we can too"
- Product quality: We'll find bugs before customers do
- Sales credibility: Authentic production usage metrics

[See complete dogfooding plan →](./docs/guides/DOGFOODING_GUIDE.md)

---

## License

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

This project is licensed under the **GNU Affero General Public License v3.0** (AGPL-3.0) to protect the sovereignty of the infrastructure and ensure that all modifications remain open source when deployed as a network service.

**Copyright (C) 2025 Innovaciones MADFAM SAS de CV**

This program is free software: you can redistribute it and/or modify it under the terms of the GNU Affero General Public License as published by the Free Software Foundation, either version 3 of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License along with this program. If not, see [LICENSE](./LICENSE) or visit https://www.gnu.org/licenses/agpl-3.0.html.

### Why AGPL-3.0?

The AGPL-3.0 license ensures that:

- **Network Copyleft**: Anyone running a modified version of Enclii as a network service must provide the source code to users
- **Infrastructure Sovereignty**: No vendor can take this code, modify it, and offer it as a proprietary service without sharing improvements
- **Community Protection**: All improvements and modifications must be contributed back to the community
- **Freedom Preservation**: Users retain the freedom to study, modify, and distribute the software

This aligns with the **MADFAM Manifesto Section IV**: protecting open infrastructure from proprietary capture.

---

## Links

- **Documentation:** [docs.enclii.io](https://docs.enclii.io)
- **Status Page:** [status.enclii.io](https://status.enclii.io)
- **Janua (Auth):** [github.com/madfam-io/janua](https://github.com/madfam-io/janua)
- **Production Roadmap:** [PRODUCTION_DEPLOYMENT_ROADMAP.md](./docs/production/PRODUCTION_DEPLOYMENT_ROADMAP.md)
- **Dogfooding Guide:** [DOGFOODING_GUIDE.md](./docs/guides/DOGFOODING_GUIDE.md)

---

**Questions?** Open an issue or contact the team at [engineering@enclii.io](mailto:engineering@enclii.io)

**Ready to deploy?** Start with [PRODUCTION_DEPLOYMENT_ROADMAP.md](./docs/production/PRODUCTION_DEPLOYMENT_ROADMAP.md) 🚀
