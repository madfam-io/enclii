# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Enclii is a Railway-style Platform-as-a-Service that runs on cost-effective infrastructure ($100/month vs $2,220 for Railway + Auth0). It deploys containerized services with enterprise-grade security, auto-scaling, and zero vendor lock-in.

**Current Status:** 🟢 v0.1.0 - Production Beta (85% ready) ([checklist](./docs/production/PRODUCTION_CHECKLIST.md))
**Infrastructure:** Hetzner Cloud + Cloudflare + Ubicloud (~$100/month) - **Running**
**Authentication:** OIDC via Janua SSO (RS256 JWT) - **Integrated**
**Dogfooding:** Core services deployed ([api.enclii.dev](https://api.enclii.dev), [app.enclii.dev](https://app.enclii.dev))
**Build Pipeline:** GitHub webhook CI/CD with Buildpacks - **Operational**

### Port Allocation

Per [PORT_ALLOCATION.md](https://github.com/madfam-io/solarpunk-foundry/blob/main/docs/PORT_ALLOCATION.md), Enclii uses the 4200-4299 block.

| Service | Port | Container | Public Domain |
|---------|------|-----------|---------------|
| Switchyard API | 4200 | enclii-api | api.enclii.dev |
| Web UI | 4201 | enclii-ui | app.enclii.dev |
| Agent | 4202 | enclii-agent | - |
| Metrics | 4290 | enclii-metrics | - |

### Quick Start (Production Deployment)
```bash
# 1. Configure credentials
cp infra/terraform/terraform.tfvars.example infra/terraform/terraform.tfvars
# Edit terraform.tfvars with your Hetzner/Cloudflare credentials

# 2. Deploy infrastructure
./scripts/deploy-production.sh check    # Validate config
./scripts/deploy-production.sh init     # Initialize Terraform
./scripts/deploy-production.sh plan     # Review changes
./scripts/deploy-production.sh apply    # Create infrastructure
./scripts/deploy-production.sh kubeconfig    # Get cluster access
./scripts/deploy-production.sh post-deploy   # Setup tunnel & namespaces
./scripts/deploy-production.sh status   # Verify deployment
```

## Architecture

The project follows a monorepo structure with these key components:

- **Switchyard**: Control plane API (Go) - manages projects, environments, services, deployments
- **Conductor (CLI)**: Developer interface (`enclii` command) (Go)
- **Roundhouse**: Build/provenance/signing workers (Go)
- **Reconcilers**: Kubernetes operators/controllers (Go)
- **UI**: Web interface (Next.js)
- **Junctions**: Routing/ingress + certs + DNS
- **Timetable**: Cron and one-off jobs
- **Lockbox**: Secrets management (Vault/1Password)
- **Signal**: Observability stack (logs/metrics/traces)
- **Waybill**: Cost tracking and budget alerts

## Common Development Commands

### Setup & Bootstrap
```bash
make bootstrap          # Install hooks, dependencies, and configure workspaces
make kind-up           # Create local kind cluster
make infra-dev         # Install ingress, cert-manager, observability stack
make dns-dev           # Configure dev DNS entries
```

### Running Services Locally
```bash
make run-switchyard    # Start control plane API on :8080
make run-ui            # Start web UI on :3000
make run-reconcilers   # Start Kubernetes controllers
```

### Building
```bash
make build-all         # Build all components
make build-cli         # Build CLI only
```

### Testing & Quality
```bash
make test              # Run unit tests
make e2e               # Run end-to-end tests
make lint              # Run linters (golangci-lint, eslint, prettier)
make precommit         # Run all checks before committing
```

### CLI Operations
```bash
./bin/enclii init                  # Scaffold a new service
./bin/enclii up                     # Deploy preview environment
./bin/enclii deploy --env prod     # Deploy to production
./bin/enclii logs <service> -f     # Tail service logs
./bin/enclii rollback <service>    # Rollback to previous release
```

## Key Technical Details

### Service Configuration
Services are defined using YAML specs (stored versioned in the control plane):
- Located at: Service spec embedded in control plane DB
- Format: `apiVersion: enclii.dev/v1`
- Includes: runtime config, health checks, routes, secrets, volumes, jobs, autoscaling

### Deployment Flow
1. Build via Nixpacks/Buildpacks or Dockerfile
2. Create immutable Release with provenance (git SHA, SBOM, signature)
3. Deploy with canary/blue-green strategies
4. Automatic rollback on failure based on SLO metrics

### Environment Variables
Key vars for local development (set in `.env`):
- `ENCLII_DB_URL`: Control plane database URL
- `ENCLII_REGISTRY`: Container registry
- `ENCLII_OIDC_ISSUER`: Auth provider URL
- `ENCLII_DEFAULT_REGION`: Default deployment region
- `ENCLII_LOG_LEVEL`: Logging verbosity

### Testing Strategy
- **Unit tests**: Test control plane validation, CLI arguments, reconciler idempotency
- **Integration tests**: Test build→release→deploy pipeline, secret injection, TLS issuance
- **E2E tests**: Test preview environment creation, canary deployments, rollbacks, cron jobs

### Platform SLOs
- Control plane API availability: 99.95% monthly
- Build subsystem availability: 99.9% monthly
- Preview environment provisioning: P95 < 3 minutes
- Deploy to stage: P95 ≤ 8 minutes for typical Node/Go services

## Important Conventions

### Security
- Never commit secrets - use Lockbox/Vault
- All images must be signed with cosign
- SBOM required for all releases
- Admission policies enforced via Kyverno/OPA

### Git Workflow
- Trunk-based development on `main`
- Conventional commits for changelog generation
- Preview environments created automatically for PRs
- Canary deploys to stage, then manual approval to prod

### Error Handling
- Exit codes: 0 (success), 10 (validation), 20 (build failed), 30 (deploy failed), 40 (timeout), 50 (auth)
- Precise error messages with actionable context
- Automatic rollback triggers when error rate > 2% for 2 minutes

### Cost Management
- Resource usage tracked per project/environment/service
- Budget alerts at 80% threshold
- Hard throttle at 100% for non-production environments

## Production Infrastructure

### Research-Validated Stack (~$100/month)

Enclii runs on cost-optimized infrastructure validated through independent research:

**Compute & Kubernetes:**
- **Hetzner Cloud** (3x CPX31) - AMD EPYC, NVMe SSD - $45/month
- **k3s** - Lightweight Kubernetes distribution
- **Cloudflare Tunnel** - Replaces LoadBalancer (saves $108/year) - $0

**Database & Caching:**
- **Ubicloud PostgreSQL** - Managed DB on Hetzner infrastructure - $50/month
- **Redis Sentinel** - Self-hosted HA (3 nodes, automatic failover) - $0

**Storage & Networking:**
- **Cloudflare R2** - Zero-egress object storage (SBOMs, artifacts) - $5/month
- **Cloudflare for SaaS** - First 100 custom domains FREE - $0

**vs Traditional SaaS Stack:** $2,220/month (Railway $2,000 + Auth0 $220)
**5-Year Savings:** $127,200

See [PRODUCTION_DEPLOYMENT_ROADMAP.md](./docs/production/PRODUCTION_DEPLOYMENT_ROADMAP.md) for details.

### Authentication (Production)

**Current Implementation:**
- ✅ **OIDC/OAuth 2.0** via Janua SSO (auth.madfam.io)
- ✅ **External JWKS validation** for federated identity
- ✅ **GitHub OAuth integration** for repo imports
- ✅ **RBAC** with admin/developer/viewer roles
- ✅ **Session Management** via Redis
- ✅ **API Keys** for CI/CD integration

**Janua Integration (Complete):**
- **Repository:** [github.com/madfam-io/janua](https://github.com/madfam-io/janua)
- **Production URL:** https://auth.madfam.io
- **Protocol:** OAuth 2.0 / OIDC with RS256 JWT
- **Features:** Multi-tenant orgs, GitHub OAuth, JWKS rotation

### Dogfooding Status (In Progress)

**Goal:** Run our entire platform on Enclii, authenticated by Janua.

> **Current State:** "We run our core production services on Enclii. We are our own most demanding customer."

**Production Services** (running at enclii.dev):
- ✅ `switchyard-api` → api.enclii.dev (control plane)
- ✅ `switchyard-ui` → app.enclii.dev (web dashboard)
- ✅ `janua` → auth.madfam.io (SSO authentication)
- ✅ `docs-site` → docs.enclii.dev (documentation)
- ✅ `landing-page` → enclii.dev (deployed)
- 🔲 `status-page` → status.enclii.dev (pending)

**Build Pipeline Status:**
- ✅ GitHub webhook configured with HMAC verification
- ✅ Real build pipeline (Buildpacks/Dockerfile detection)
- ✅ Container registry push (ghcr.io/madfam-io)
- ✅ Kubernetes reconciler for deployments

See [DOGFOODING_GUIDE.md](./docs/guides/DOGFOODING_GUIDE.md) for complete implementation plan.

**Why This Matters:**
- **Customer Confidence:** "If they trust it, we can too"
- **Product Quality:** We find bugs before customers do
- **Sales Credibility:** Authentic production usage metrics