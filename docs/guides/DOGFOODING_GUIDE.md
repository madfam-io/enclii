# Enclii + Janua Dogfooding Strategy

> ⚠️ **IMPLEMENTATION PLAN** - This document describes the **future state** (Weeks 5-6 of roadmap).
> **Current Status:** Service specs created, awaiting infrastructure setup (Weeks 1-2) and Janua integration (Weeks 3-4).
> **Not Yet Implemented:** Enclii is NOT yet self-hosting. Janua is NOT yet integrated.

---

> **Goal (Weeks 5-6):** "We'll run our entire platform on Enclii, authenticated by Janua. We'll be our own most demanding customer."

This document outlines the **plan** for how Enclii will deploy **itself** using its own platform, and how we'll use **Janua** (our own auth solution) to authenticate the Enclii control plane. This will be critical for product quality, customer confidence, and sales credibility.

---

## Table of Contents

1. [Why Dogfooding Matters](#why-dogfooding-matters)
2. [Current State](#current-state)
3. [Dogfooding Architecture](#dogfooding-architecture)
4. [Deployment Strategy](#deployment-strategy)
5. [Repository Structure](#repository-structure)
6. [Step-by-Step Implementation](#step-by-step-implementation)
7. [The Confidence Signal](#the-confidence-signal)
8. [Troubleshooting](#troubleshooting)

---

## Why Dogfooding Matters

### The Problem We're Solving

**Before Dogfooding:**
- ❌ Enclii deployed via raw Kubernetes manifests (`kubectl apply -k infra/k8s/base`)
- ❌ Not using our own platform (can't validate our own product)
- ❌ Missing customer pain points (we don't experience what they do)
- ❌ No confidence signal ("If they don't use it, why should we?")
- ❌ Janua built but unused (we don't authenticate with our own solution)

**After Dogfooding:**
- ✅ Enclii deploys Enclii (using `enclii deploy` commands)
- ✅ Janua authenticates Enclii (OAuth/OIDC flows battle-tested daily)
- ✅ We experience every customer pain point first
- ✅ Powerful sales narrative: "We run production on Enclii + Janua"
- ✅ Product quality improves (we fix issues before customers see them)

### Business Impact

**Customer Confidence:**
- "If Enclii trusts Enclii for their own production, so can we"
- Removes #1 objection: "Is this actually production-ready?"

**Sales Credibility:**
- Authentic testimonials: "We've deployed 50+ times this month using Enclii"
- Technical demos show real production usage, not toy examples

**Product Quality:**
- Engineering team uses Enclii daily (bugs found and fixed faster)
- Edge cases discovered organically (complex auth flows, networking, etc.)

**Team Alignment:**
- Everyone experiences the developer experience daily
- Product decisions informed by real usage, not assumptions

---

## Current State

### What We Have

**Enclii Repository:** https://github.com/madfam-io/enclii
- ✅ Control plane API (Switchyard)
- ✅ Web UI (Next.js dashboard)
- ✅ CLI (`enclii` command)
- ✅ Kubernetes reconcilers
- ✅ Infrastructure manifests (`infra/k8s/`)

**Janua Repository:** https://github.com/madfam-io/janua
- ✅ OAuth 2.0 / OIDC provider
- ✅ RS256 JWT signing
- ✅ Multi-tenant organization support
- ✅ Password + SSO authentication

### What's Missing

**Dogfooding Gap:**
- ❌ No service specs for Enclii components (`dogfooding/*.yaml`)
- ❌ Enclii deployed manually, not via `enclii deploy`
- ❌ Janua not deployed on Enclii
- ❌ Enclii not authenticated by Janua (using standalone JWT)
- ❌ No internal services (landing page, docs, status) on Enclii

---

## Dogfooding Architecture

### Service Topology

```
┌─────────────────────────────────────────────────────────────────┐
│                       Enclii Platform                           │
│                (Deployed on Hetzner + Cloudflare)               │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  Public Internet                                                │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │  enclii.io   │  │ app.enclii.io│  │auth.enclii.io│         │
│  │ (Landing)    │  │   (Web UI)   │  │   (Janua)   │         │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘         │
│         │                 │                  │                  │
└─────────┼─────────────────┼──────────────────┼──────────────────┘
          │                 │                  │
          │                 │                  │
┌─────────┼─────────────────┼──────────────────┼──────────────────┐
│         ▼                 ▼                  ▼                  │
│  ┌────────────────────────────────────────────────────┐        │
│  │         Cloudflare Tunnel (Replaces LB)            │        │
│  └────────────────────────────────────────────────────┘        │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  Kubernetes Cluster (Hetzner 3x CPX31)                   │  │
│  │                                                           │  │
│  │  Namespace: enclii-platform                              │  │
│  │  ┌─────────────────────────────────────────────────┐    │  │
│  │  │  Switchyard API (3 replicas)                    │    │  │
│  │  │  └─> api.enclii.io                              │    │  │
│  │  │  └─> Built from: github.com/madfam-io/enclii   │    │  │
│  │  │  └─> Deployed via: enclii deploy                │    │  │
│  │  └─────────────────────────────────────────────────┘    │  │
│  │                                                           │  │
│  │  ┌─────────────────────────────────────────────────┐    │  │
│  │  │  Switchyard UI (2 replicas)                     │    │  │
│  │  │  └─> app.enclii.io                              │    │  │
│  │  │  └─> Built from: github.com/madfam-io/enclii   │    │  │
│  │  │  └─> Deployed via: enclii deploy                │    │  │
│  │  └─────────────────────────────────────────────────┘    │  │
│  │                                                           │  │
│  │  ┌─────────────────────────────────────────────────┐    │  │
│  │  │  Janua (3 replicas)                            │    │  │
│  │  │  └─> auth.enclii.io                             │    │  │
│  │  │  └─> Built from: github.com/madfam-io/janua   │    │  │
│  │  │  └─> Deployed via: enclii deploy                │    │  │
│  │  │  └─> Authenticates: Enclii itself!              │    │  │
│  │  └─────────────────────────────────────────────────┘    │  │
│  │                                                           │  │
│  │  ┌─────────────────────────────────────────────────┐    │  │
│  │  │  Landing Page (2 replicas)                      │    │  │
│  │  │  └─> enclii.io                                  │    │  │
│  │  │  └─> Deployed via: enclii deploy                │    │  │
│  │  └─────────────────────────────────────────────────┘    │  │
│  │                                                           │  │
│  │  ┌─────────────────────────────────────────────────┐    │  │
│  │  │  Docs Site (2 replicas)                         │    │  │
│  │  │  └─> docs.enclii.io                             │    │  │
│  │  │  └─> Deployed via: enclii deploy                │    │  │
│  │  └─────────────────────────────────────────────────┘    │  │
│  │                                                           │  │
│  │  ┌─────────────────────────────────────────────────┐    │  │
│  │  │  Status Page (2 replicas)                       │    │  │
│  │  │  └─> status.enclii.io                           │    │  │
│  │  │  └─> Deployed via: enclii deploy                │    │  │
│  │  └─────────────────────────────────────────────────┘    │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  Shared Infrastructure                                   │  │
│  │  ┌────────────────────────────────────────────────┐     │  │
│  │  │  Ubicloud PostgreSQL (managed, HA)            │     │  │
│  │  │  └─> Used by: Enclii + Janua                 │     │  │
│  │  └────────────────────────────────────────────────┘     │  │
│  │                                                           │  │
│  │  ┌────────────────────────────────────────────────┐     │  │
│  │  │  Redis Sentinel (self-hosted, 3 nodes)        │     │  │
│  │  │  └─> Used by: Enclii + Janua                 │     │  │
│  │  └────────────────────────────────────────────────┘     │  │
│  │                                                           │  │
│  │  ┌────────────────────────────────────────────────┐     │  │
│  │  │  Cloudflare R2 (object storage)                │     │  │
│  │  │  └─> Used for: SBOMs, artifacts, build cache   │     │  │
│  │  └────────────────────────────────────────────────┘     │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### Authentication Flow

```
User visits app.enclii.io
    │
    ├─> Redirected to auth.enclii.io (Janua)
    │       │
    │       ├─> User logs in (password or SSO)
    │       │
    │       └─> Janua issues ID token (RS256 JWT)
    │
    ├─> Redirect back to app.enclii.io/callback
    │       │
    │       ├─> Exchange code for tokens
    │       │
    │       └─> Store tokens in browser
    │
    ├─> User makes API request to api.enclii.io
    │       │
    │       ├─> Include ID token in Authorization header
    │       │
    │       ├─> Switchyard API validates token via Janua JWKS
    │       │
    │       └─> Request succeeds (user authenticated!)
```

**Key Point:** Enclii authenticates its own users via Janua. We eat our own dog food.

---

## Deployment Strategy

### Phase 1: Bootstrap (One-Time Setup)

The **first deployment** of Enclii must be manual (chicken-and-egg problem). After that, Enclii deploys itself forever.

**Bootstrap Steps:**

1. **Deploy Infrastructure** (Ubicloud PostgreSQL, Redis Sentinel, R2)
2. **Deploy Enclii Control Plane Manually** (using `kubectl apply -k infra/k8s/base`)
3. **Deploy Janua Manually** (using `kubectl apply -f dogfooding/janua.yaml`)
4. **Configure Janua** (create OAuth clients for Enclii)
5. **Switch to Self-Service** (all future deploys via `enclii deploy`)

### Phase 2: Dogfooding (Forever After)

Once bootstrapped, **all deployments** happen via Enclii itself:

```bash
# Deploy Switchyard API (from GitHub)
./bin/enclii deploy --service switchyard-api --env production

# Deploy Switchyard UI (from GitHub)
./bin/enclii deploy --service switchyard-ui --env production

# Deploy Janua (from separate repo!)
./bin/enclii deploy --service janua --env production

# Deploy landing page
./bin/enclii deploy --service landing-page --env production

# Deploy docs
./bin/enclii deploy --service docs-site --env production

# Deploy status page
./bin/enclii deploy --service status-page --env production
```

**Result:** Enclii deploys Enclii. We're our own customer.

---

## Repository Structure

### Enclii Repository (`github.com/madfam-io/enclii`)

```
enclii/
├── apps/
│   ├── switchyard-api/          # Control plane API (Go)
│   ├── switchyard-ui/           # Web dashboard (Next.js)
│   ├── landing/                 # Marketing site (Next.js)
│   ├── status/                  # Status page
│   └── ...
├── packages/
│   └── cli/                     # enclii CLI
├── infra/
│   ├── k8s/
│   │   ├── base/                # Raw Kubernetes manifests (bootstrap only)
│   │   ├── staging/
│   │   └── production/
│   └── terraform/               # Infrastructure as code (Hetzner, Cloudflare)
├── dogfooding/                  # ⭐ Service specs for self-hosting
│   ├── switchyard-api.yaml      # Enclii API spec
│   ├── switchyard-ui.yaml       # Enclii UI spec
│   ├── janua.yaml              # Janua spec (separate repo!)
│   ├── landing-page.yaml        # Landing page spec
│   ├── docs-site.yaml           # Docs spec
│   └── status-page.yaml         # Status page spec
└── DOGFOODING_GUIDE.md          # This file
```

### Janua Repository (`github.com/madfam-io/janua`)

```
janua/
├── src/                         # Janua source code
├── Dockerfile                   # Container build
├── docker-compose.yml           # Local dev
└── README.md
```

**Key Insight:** Janua lives in a **separate repository**, but is deployed on Enclii via the `dogfooding/janua.yaml` spec. This demonstrates Enclii's ability to build from any GitHub repository.

---

## Step-by-Step Implementation

### Prerequisites

- Hetzner account with 3x CPX31 nodes (Kubernetes cluster)
- Cloudflare account with Tunnel configured
- Ubicloud account with managed PostgreSQL
- GitHub accounts with access to `madfam-io/enclii` and `madfam-io/janua`

### Step 1: Bootstrap Infrastructure (Week 1)

Follow the [PRODUCTION_DEPLOYMENT_ROADMAP.md](./PRODUCTION_DEPLOYMENT_ROADMAP.md) to set up:

1. **Hetzner Kubernetes cluster** (3x CPX31 nodes)
2. **Cloudflare Tunnel** (replaces LoadBalancer)
3. **Cloudflare for SaaS** (100 free custom domains)
4. **Ubicloud PostgreSQL** (managed, HA)
5. **Redis Sentinel** (self-hosted, 3 nodes)
6. **Cloudflare R2** (object storage)

**Result:** Infrastructure ready, but Enclii not deployed yet.

### Step 2: Bootstrap Enclii Control Plane (Week 2)

Deploy Enclii manually **one time** using raw Kubernetes manifests:

```bash
# Clone Enclii repository
git clone https://github.com/madfam-io/enclii
cd enclii

# Configure secrets
kubectl create secret generic enclii-secrets \
  --from-literal=database-url="postgres://..." \
  --from-literal=redis-url="redis://..." \
  --from-literal=r2-endpoint="https://..." \
  --from-literal=r2-access-key-id="..." \
  --from-literal=r2-secret-access-key="..." \
  -n enclii-platform

kubectl create secret generic jwt-secrets \
  --from-file=private-key=keys/rsa-private.pem \
  --from-file=public-key=keys/rsa-public.pem \
  -n enclii-platform

# Deploy control plane
kubectl apply -k infra/k8s/production

# Wait for readiness
kubectl wait --for=condition=ready pod -l app=switchyard-api -n enclii-platform --timeout=300s

# Verify
curl https://api.enclii.io/health
# {"status": "ok"}
```

**Result:** Enclii control plane running, but not self-hosted yet.

### Step 3: Bootstrap Janua (Week 3)

Deploy Janua manually **one time**:

```bash
# Clone Janua repository
git clone https://github.com/madfam-io/janua
cd janua

# Configure secrets
kubectl create secret generic janua-secrets \
  --from-literal=database-url="postgres://..." \
  --from-literal=redis-url="redis://..." \
  --from-literal=session-secret="$(openssl rand -base64 32)" \
  --from-literal=smtp-host="smtp.sendgrid.net" \
  --from-literal=smtp-port="587" \
  --from-literal=smtp-user="apikey" \
  --from-literal=smtp-password="SG...." \
  -n enclii-platform

# Deploy Janua
kubectl apply -f ../enclii/dogfooding/janua.yaml

# Wait for readiness
kubectl wait --for=condition=ready pod -l app=janua -n enclii-platform --timeout=300s

# Verify
curl https://auth.enclii.io/health
# {"status": "ok"}
```

**Result:** Janua running on Enclii infrastructure.

### Step 4: Configure Janua OAuth Clients (Week 3)

Create OAuth clients in Janua for Enclii:

```bash
# Create Enclii Web UI client (public)
curl -X POST https://auth.enclii.io/v1/clients \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JANUA_ADMIN_TOKEN" \
  -d '{
    "client_id": "enclii-web-ui",
    "client_name": "Enclii Web Dashboard",
    "redirect_uris": [
      "https://app.enclii.io/callback",
      "https://dashboard.enclii.io/callback",
      "http://localhost:3000/callback"
    ],
    "grant_types": ["authorization_code", "refresh_token"],
    "response_types": ["code"],
    "scope": "openid profile email",
    "token_endpoint_auth_method": "none",
    "application_type": "web"
  }'

# Create Enclii API client (confidential)
curl -X POST https://auth.enclii.io/v1/clients \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JANUA_ADMIN_TOKEN" \
  -d '{
    "client_id": "enclii-api",
    "client_name": "Enclii Control Plane API",
    "client_secret": "<generated-secret>",
    "grant_types": ["client_credentials"],
    "scope": "api:read api:write",
    "token_endpoint_auth_method": "client_secret_basic",
    "application_type": "service"
  }'
```

**Result:** Janua configured to authenticate Enclii.

### Step 5: Update Enclii to Use Janua (Week 4)

Update Switchyard API to validate Janua tokens:

```bash
# apps/switchyard-api/main.go
jwksProvider, _ := auth.NewJWKSProvider("https://auth.enclii.io/.well-known/jwks.json")
jwtManager := auth.NewJWTManager(jwksProvider)

r.Use(jwtManager.AuthMiddleware())
```

Update Switchyard UI to use Janua OAuth:

```bash
# apps/switchyard-ui/lib/auth-config.ts
export const authConfig = {
  authority: 'https://auth.enclii.io',
  client_id: 'enclii-web-ui',
  redirect_uri: 'https://app.enclii.io/callback',
  scope: 'openid profile email',
  response_type: 'code',
}
```

**Result:** Enclii authenticates via Janua (but still deployed manually).

### Step 6: Migrate to Self-Service Deployment (Week 5)

Now the **critical transition**: Deploy Enclii components using **Enclii itself**.

```bash
cd enclii

# Create project in Enclii
./bin/enclii project create enclii-platform

# Import service specs
./bin/enclii service create --file dogfooding/switchyard-api.yaml
./bin/enclii service create --file dogfooding/switchyard-ui.yaml
./bin/enclii service create --file dogfooding/janua.yaml
./bin/enclii service create --file dogfooding/landing-page.yaml
./bin/enclii service create --file dogfooding/docs-site.yaml
./bin/enclii service create --file dogfooding/status-page.yaml

# Deploy everything via Enclii
./bin/enclii deploy --service switchyard-api --env production
./bin/enclii deploy --service switchyard-ui --env production
./bin/enclii deploy --service janua --env production
./bin/enclii deploy --service landing-page --env production
./bin/enclii deploy --service docs-site --env production
./bin/enclii deploy --service status-page --env production

# Verify all services
./bin/enclii services list
# NAME              STATUS     REPLICAS  AGE
# switchyard-api    Running    3/3       5m
# switchyard-ui     Running    2/2       5m
# janua            Running    3/3       5m
# landing-page      Running    2/2       5m
# docs-site         Running    2/2       5m
# status-page       Running    2/2       5m
```

**Result:** ✅ **Enclii deploys Enclii. Dogfooding complete!**

### Step 7: Enable Continuous Deployment (Week 5)

Configure GitHub webhooks so that **every push to main** triggers a deploy:

```yaml
# dogfooding/switchyard-api.yaml
spec:
  build:
    source:
      git:
        repository: https://github.com/madfam-io/enclii
        branch: main
        autoDeploy: true  # ⭐ Auto-deploy on push
```

**Workflow:**
1. Developer pushes to `main` branch
2. GitHub webhook notifies Enclii control plane
3. Enclii builds new image (with provenance)
4. Enclii creates new release (with SBOM)
5. Enclii deploys with canary strategy
6. If healthy after 5 minutes, promotes to 100%
7. If unhealthy, automatic rollback

**Result:** ✅ **Continuous deployment for Enclii itself.**

---

## The Confidence Signal

### What We Can Now Say

**To Customers:**
> "Enclii's entire production infrastructure runs on Enclii itself. Our control plane, web dashboard, authentication service, landing page, documentation, and status page are all deployed via `enclii deploy`. We've performed 200+ production deployments using our own platform. We're our own most demanding customer."

**To Investors:**
> "We dogfood our own product ruthlessly. Every feature we ship is battle-tested in our own production environment before customers see it. This ensures product quality and reduces support burden."

**To Engineering Candidates:**
> "You'll use Enclii every day to deploy your own work. It's not a side project—it's how we run our entire company."

### Sales Narrative

**Before Dogfooding:**
- Sales call: "Can Enclii handle production workloads?"
- Us: "Uh... we think so? Our test suite passes..."
- Customer: 😬

**After Dogfooding:**
- Sales call: "Can Enclii handle production workloads?"
- Us: "We run our entire production on Enclii. Here's our status page showing 99.95% uptime. We deploy 10-20 times per day with zero downtime. Want to see our deployment logs?"
- Customer: 🤝

### Authenticity Matters

Customers can **verify** our claims:

```bash
# Customer checks our public API
curl https://api.enclii.io/health

# Customer checks Janua JWKS endpoint
curl https://auth.enclii.io/.well-known/jwks.json

# Customer checks status page
curl https://status.enclii.io
# Shows real uptime data for Enclii services
```

They can see we're not lying. We really do run on Enclii.

---

## Troubleshooting

### Issue: "Enclii API won't start after Janua integration"

**Symptoms:**
- Switchyard API returns 401 Unauthorized
- Logs show: "failed to fetch JWKS from Janua"

**Root Cause:**
- Janua not accessible from Switchyard API pods
- NetworkPolicy blocking traffic

**Fix:**
```bash
# Check NetworkPolicy
kubectl get netpol -n enclii-platform

# Verify Janua is reachable
kubectl exec -it -n enclii-platform deployment/switchyard-api -- \
  curl http://janua.enclii-platform.svc.cluster.local:8000/.well-known/jwks.json

# If blocked, update NetworkPolicy to allow egress to Janua
```

### Issue: "Circular dependency during bootstrap"

**Symptoms:**
- Can't deploy Enclii via Enclii (chicken-and-egg)

**Root Cause:**
- First deployment must be manual

**Fix:**
- Follow **Step 2: Bootstrap Enclii Control Plane** exactly
- Deploy manually **once**, then migrate to self-service
- Don't try to skip the bootstrap phase

### Issue: "Auto-deploy triggers too frequently"

**Symptoms:**
- Every commit triggers a deploy (even docs changes)
- Deploys happen during business hours (risky)

**Fix:**
```yaml
# dogfooding/switchyard-api.yaml
spec:
  build:
    source:
      git:
        autoDeploy: true
        deployFilter:
          paths:
            - "apps/switchyard-api/**"  # Only deploy on API changes
          excludePaths:
            - "**/*.md"  # Ignore docs
        deploySchedule:
          onlyAfter: "22:00 UTC"  # Only deploy after 10pm UTC
          onlyBefore: "06:00 UTC"  # Only deploy before 6am UTC
```

### Issue: "Janua tokens not validating"

**Symptoms:**
- User logs into Janua successfully
- Switchyard API rejects tokens with "invalid signature"

**Root Cause:**
- JWKS cache stale
- Clock skew between services

**Fix:**
```bash
# Check JWKS cache age
curl https://api.enclii.io/debug/jwks/cache
# {"last_refresh": "2025-11-20T10:30:00Z", "next_refresh": "2025-11-20T10:45:00Z"}

# Force JWKS refresh
curl -X POST https://api.enclii.io/debug/jwks/refresh \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Check clock skew
kubectl exec -it -n enclii-platform deployment/switchyard-api -- date
kubectl exec -it -n enclii-platform deployment/janua -- date
# Should be within 1-2 seconds
```

---

## Next Steps

### Week 1-2: Infrastructure Setup
- [ ] Provision Hetzner cluster (3x CPX31)
- [ ] Deploy Cloudflare Tunnel
- [ ] Set up Ubicloud PostgreSQL
- [ ] Deploy Redis Sentinel
- [ ] Configure Cloudflare R2

### Week 3: Bootstrap Enclii
- [ ] Deploy Switchyard API manually
- [ ] Deploy Switchyard UI manually
- [ ] Configure secrets and networking
- [ ] Verify control plane health

### Week 4: Bootstrap Janua
- [ ] Deploy Janua manually
- [ ] Create OAuth clients for Enclii
- [ ] Update Enclii to use Janua auth
- [ ] Test full OAuth flow

### Week 5: Migrate to Dogfooding
- [ ] Import service specs into Enclii
- [ ] Redeploy Switchyard API via `enclii deploy`
- [ ] Redeploy Switchyard UI via `enclii deploy`
- [ ] Redeploy Janua via `enclii deploy`
- [ ] Deploy landing page, docs, status page
- [ ] Enable continuous deployment

### Week 6: Validation & Polish
- [ ] Perform 10+ test deployments
- [ ] Verify canary deployments work
- [ ] Test automatic rollbacks
- [ ] Load test to 1000 RPS
- [ ] Update sales materials with dogfooding narrative

---

## Conclusion

Dogfooding is **not optional**—it's a critical competitive advantage. By running Enclii on Enclii and authenticating with Janua, we:

1. **Validate our product** before customers do
2. **Build customer confidence** through authentic usage
3. **Improve product quality** by experiencing pain points first
4. **Enable powerful sales narratives** with real production metrics
5. **Align the team** around a shared experience

The service specs in `dogfooding/` are not toy examples—they're **production-ready configurations** that deploy our entire platform. Follow this guide to make Enclii its own best customer.

---

**Questions?** Open an issue or ask in #engineering on Slack.

**Ready to dogfood?** Start with [Step 1: Bootstrap Infrastructure](#step-1-bootstrap-infrastructure-week-1).
