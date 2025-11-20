# Enclii Dogfooding Service Specs

> ⚠️ **PLANNED IMPLEMENTATION** - These are service specifications for **future deployment** (Weeks 5-6).
> **Current Status:** Specs are ready. Awaiting infrastructure (Weeks 1-2) and Plinto integration (Weeks 3-4).
> **Not Yet Deployed:** These services are NOT yet running in production.

This directory contains **production-ready service specifications** for deploying Enclii's own infrastructure using Enclii itself.

## Why This Will Matter

> **Goal (Weeks 5-6):** "We'll run our entire platform on Enclii, authenticated by Plinto. We'll be our own most demanding customer."

These service specs will demonstrate:
- 🔲 Enclii deploys Enclii (self-hosting) - PLANNED
- 🔲 Plinto authenticates Enclii (eating our own auth solution) - PLANNED
- ✅ Multi-repo support (Enclii + Plinto from separate GitHub repos) - SPECS READY
- ✅ Full production features (HA, autoscaling, monitoring, custom domains) - SPECS READY

## Service Specs

### Core Platform

- **`switchyard-api.yaml`** - Control plane REST API
  - Built from: `github.com/madfam-io/enclii`
  - Exposed at: `api.enclii.io`
  - 3 replicas (HA)
  - Autoscaling: 3-10 pods based on CPU/memory

- **`switchyard-ui.yaml`** - Web dashboard (Next.js)
  - Built from: `github.com/madfam-io/enclii`
  - Exposed at: `app.enclii.io`
  - 2 replicas
  - Autoscaling: 2-8 pods

- **`plinto.yaml`** - Authentication service (OAuth/OIDC)
  - Built from: `github.com/madfam-io/plinto` ⭐ **Separate repository**
  - Exposed at: `auth.enclii.io`
  - 3 replicas (auth is critical)
  - Autoscaling: 3-10 pods

### Public Services

- **`landing-page.yaml`** - Marketing site
  - Exposed at: `enclii.io`
  - Static Next.js export
  - Aggressive caching (24 hours)

- **`docs-site.yaml`** - Documentation
  - Exposed at: `docs.enclii.io`
  - Docusaurus or similar
  - Caching: 1 hour

- **`status-page.yaml`** - Public status page
  - Exposed at: `status.enclii.io`
  - Monitors all Enclii services
  - Connected to Prometheus

## How to Use

### Prerequisites

1. **Bootstrap infrastructure** (Hetzner + Cloudflare + Ubicloud)
2. **Deploy Enclii control plane manually** (one time)
3. **Configure secrets** in Kubernetes

See [DOGFOODING_GUIDE.md](../DOGFOODING_GUIDE.md) for full instructions.

### Deploy Services

```bash
# Create project
./bin/enclii project create enclii-platform

# Import service specs
./bin/enclii service create --file dogfooding/switchyard-api.yaml
./bin/enclii service create --file dogfooding/switchyard-ui.yaml
./bin/enclii service create --file dogfooding/plinto.yaml
./bin/enclii service create --file dogfooding/landing-page.yaml
./bin/enclii service create --file dogfooding/docs-site.yaml
./bin/enclii service create --file dogfooding/status-page.yaml

# Deploy to production
./bin/enclii deploy --service switchyard-api --env production
./bin/enclii deploy --service switchyard-ui --env production
./bin/enclii deploy --service plinto --env production
./bin/enclii deploy --service landing-page --env production
./bin/enclii deploy --service docs-site --env production
./bin/enclii deploy --service status-page --env production

# Check status
./bin/enclii services list
```

### Continuous Deployment

All services have `autoDeploy: true`, which means:

1. **Developer pushes to `main`** (either Enclii or Plinto repo)
2. **GitHub webhook triggers Enclii**
3. **Enclii builds new image** (with provenance + SBOM)
4. **Enclii deploys with canary** (10% → 50% → 100%)
5. **Automatic rollback on failure** (if error rate > 2%)

## Architecture Highlights

### Multi-Repository Support

```yaml
# Enclii API (from Enclii repo)
source:
  git:
    repository: https://github.com/madfam-io/enclii
    branch: main

# Plinto (from separate Plinto repo)
source:
  git:
    repository: https://github.com/madfam-io/plinto
    branch: main
```

This demonstrates Enclii can build from **any GitHub repository**, not just monorepos.

### Authentication Flow

```
User → app.enclii.io
  ↓
Redirect to auth.enclii.io (Plinto)
  ↓
Login with password/SSO
  ↓
Plinto issues RS256 JWT
  ↓
Redirect to app.enclii.io/callback
  ↓
Store tokens
  ↓
API requests to api.enclii.io
  ↓
Switchyard validates JWT via Plinto JWKS
  ↓
✅ Authenticated
```

**Key point:** Enclii authenticates its own users with Plinto. Total dogfooding.

### Infrastructure

- **Kubernetes:** Hetzner Cloud (3x CPX31 nodes)
- **Ingress:** Cloudflare Tunnel (replaces LoadBalancer)
- **Database:** Ubicloud PostgreSQL (managed, HA)
- **Cache:** Redis Sentinel (self-hosted, 3 nodes)
- **Storage:** Cloudflare R2 (SBOMs, artifacts)
- **DNS:** Cloudflare for SaaS (100 free domains)

**Cost:** ~$100/month (vs $2,220 for Railway + Auth0)

## Secrets Required

Before deploying, create these secrets:

```bash
# Enclii secrets
kubectl create secret generic enclii-secrets \
  --from-literal=database-url="postgres://..." \
  --from-literal=redis-url="redis://..." \
  --from-literal=r2-endpoint="https://..." \
  --from-literal=r2-access-key-id="..." \
  --from-literal=r2-secret-access-key="..." \
  --from-literal=prometheus-url="http://prometheus.monitoring:9090" \
  -n enclii-platform

# JWT signing keys (RS256)
kubectl create secret generic jwt-secrets \
  --from-file=private-key=keys/rsa-private.pem \
  --from-file=public-key=keys/rsa-public.pem \
  -n enclii-platform

# Plinto secrets
kubectl create secret generic plinto-secrets \
  --from-literal=database-url="postgres://..." \
  --from-literal=redis-url="redis://..." \
  --from-literal=session-secret="$(openssl rand -base64 32)" \
  --from-literal=smtp-host="smtp.sendgrid.net" \
  --from-literal=smtp-port="587" \
  --from-literal=smtp-user="apikey" \
  --from-literal=smtp-password="SG...." \
  -n enclii-platform
```

## Monitoring

All services emit metrics to Prometheus:

- **Control Plane API:** `/metrics` endpoint
- **Web UI:** `/api/metrics` endpoint
- **Plinto:** `/metrics` endpoint

Grafana dashboards available at: `grafana.enclii.io`

## Status Page

Public status page monitors:
- ✅ Control Plane API (`api.enclii.io/health`)
- ✅ Web Dashboard (`app.enclii.io/api/health`)
- ✅ Authentication (`auth.enclii.io/health`)
- ✅ Documentation (`docs.enclii.io`)

View at: https://status.enclii.io

## Next Steps

1. **Read [DOGFOODING_GUIDE.md](../DOGFOODING_GUIDE.md)** for full implementation guide
2. **Bootstrap infrastructure** (Week 1-2)
3. **Deploy Enclii manually** (Week 3)
4. **Deploy Plinto** (Week 4)
5. **Migrate to self-service** (Week 5)

---

**Questions?** See [DOGFOODING_GUIDE.md](../DOGFOODING_GUIDE.md) or open an issue.
