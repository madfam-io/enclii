# Enclii Dogfooding Map

> **Complete ecosystem overview: How Enclii hosts everything, and how everything connects.**

---

## 🎯 The Dogfooding Vision

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           ENCLII PLATFORM                                   │
│                    (Self-hosted on Hetzner + Cloudflare)                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐ │
│   │   Janua     │    │  Enclii     │    │ Roundhouse  │    │  Waybill    │ │
│   │   (Auth)    │◄───│   (PaaS)    │───►│  (Builds)   │    │ (Billing)   │ │
│   └──────┬──────┘    └──────┬──────┘    └─────────────┘    └─────────────┘ │
│          │                  │                                               │
│          │    ┌─────────────┴─────────────┐                                │
│          │    │                           │                                │
│          ▼    ▼                           ▼                                │
│   ┌─────────────────────────────────────────────────────────────────────┐  │
│   │                     ALL MADFAM APPS                                  │  │
│   │  forgesight │ dhanam │ fortuna │ electrochem-sim │ bloom-scroll │...│  │
│   └─────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Key Principle**: Enclii deploys Enclii. Janua authenticates Janua. We are our own most demanding customer.

---

## 📦 Complete Repository Inventory

### Tier 1: Platform Infrastructure (Deploy First)

| Repo | Purpose | Components | Dependencies |
|------|---------|------------|--------------|
| **enclii** | PaaS Control Plane | switchyard-api, switchyard-ui, roundhouse, waybill, reconcilers | PostgreSQL, Redis, Janua |
| **janua** | Identity/Auth (OIDC) | api, admin-ui | PostgreSQL, Redis, SMTP |

### Tier 2: Production SaaS Apps (Deploy After Platform)

| Repo | Purpose | Components | Auth | Database |
|------|---------|------------|------|----------|
| **forgesight** | Fabrication Pricing | api, web, worker | Janua | PostgreSQL |
| **dhanam** | Financial Wellness | api, web | Janua | PostgreSQL |
| **fortuna** | Portfolio Tracker | api, web | Janua | PostgreSQL |
| **electrochem-sim** | Electrochemistry | api, web, simulation-worker | Janua | PostgreSQL, Redis |

### Tier 3: Platform Apps (Deploy with Platform)

| Repo | Purpose | Components | Auth | Database |
|------|---------|------------|------|----------|
| **bloom-scroll** | Content Curation | api (FastAPI), web (Flutter) | Janua | PostgreSQL, Redis |
| **coforma-studio** | Feedback Management | api (Node), web (Next.js) | Janua | PostgreSQL |
| **avala** | Project Management | api, web | Janua | PostgreSQL |
| **blueprint-harvester** | Code Extraction | api, processing-worker | Janua | PostgreSQL, MinIO, OpenSearch |
| **cotiza-studio** | Quotation Management | api, web | Janua | PostgreSQL |
| **forj** | Forge Operations | api, web | Janua | PostgreSQL |

### Tier 4: Business Sites (Static/Simple)

| Repo | Purpose | Type | Hosting |
|------|---------|------|---------|
| **madfam-site** | Company Website | Static Next.js | Enclii (or Cloudflare Pages) |
| **solarpunk-studio** | Design Studio | Static | Enclii |
| **coforma-ai** | AI Product Site | Static | Enclii |

### Tier 5: Libraries (Not Deployed)

| Repo | Purpose | Used By |
|------|---------|---------|
| **geom-core** | Geometry Library | electrochem-sim, forgesight |
| **solarpunk-foundry** | UI Components | All web apps |

---

## 🔗 Interconnectivity Map

### Authentication Flow (Janua Hub)

```
                              ┌─────────────────┐
                              │      Janua      │
                              │  (auth.janua.dev)│
                              └────────┬────────┘
                                       │
              ┌────────────────────────┼────────────────────────┐
              │                        │                        │
              ▼                        ▼                        ▼
    ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
    │  Enclii Apps    │    │  SaaS Products  │    │  Internal Tools │
    │                 │    │                 │    │                 │
    │ • switchyard-ui │    │ • forgesight    │    │ • avala         │
    │ • admin panels  │    │ • dhanam        │    │ • blueprint     │
    │                 │    │ • fortuna       │    │ • coforma       │
    └─────────────────┘    │ • bloom-scroll  │    └─────────────────┘
                           │ • electrochem   │
                           └─────────────────┘
```

**OAuth 2.0 / OIDC Flow**:
1. User visits `app.forgesight.quest`
2. Redirect to `auth.janua.dev/authorize`
3. User logs in (password/SSO/social)
4. Janua issues RS256 JWT
5. Redirect to `app.forgesight.quest/callback`
6. App validates JWT via Janua JWKS (`auth.janua.dev/.well-known/jwks.json`)

### Platform Service Flow

```
┌──────────────────────────────────────────────────────────────────────────┐
│                          ENCLII PLATFORM                                  │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                           │
│  ┌─────────────┐         ┌─────────────┐         ┌─────────────┐        │
│  │ Switchyard  │◄───────►│ Roundhouse  │────────►│  Registry   │        │
│  │    API      │ enqueue │  (builds)   │  push   │   (GHCR)    │        │
│  └──────┬──────┘         └──────┬──────┘         └─────────────┘        │
│         │                       │                                        │
│         │ deploy                │ callback                               │
│         ▼                       ▼                                        │
│  ┌─────────────┐         ┌─────────────┐                                │
│  │ Reconcilers │         │  Waybill    │                                │
│  │ (K8s ops)   │────────►│ (billing)   │                                │
│  └──────┬──────┘  events └─────────────┘                                │
│         │                                                                │
│         ▼                                                                │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                    KUBERNETES CLUSTER                            │    │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐   │    │
│  │  │forgesight│ │ dhanam │ │ fortuna │ │bloom-scr│ │ ...etc  │   │    │
│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘   │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                           │
└──────────────────────────────────────────────────────────────────────────┘
```

### Data Flow Between Apps

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         SHARED INFRASTRUCTURE                            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   ┌──────────────────┐    ┌──────────────────┐    ┌──────────────────┐  │
│   │   PostgreSQL     │    │      Redis       │    │   MinIO/R2       │  │
│   │   (Ubicloud)     │    │   (Sentinel)     │    │  (Object Store)  │  │
│   └────────┬─────────┘    └────────┬─────────┘    └────────┬─────────┘  │
│            │                       │                       │            │
│   ┌────────┴─────────┐    ┌────────┴─────────┐    ┌────────┴─────────┐  │
│   │ Per-app databases│    │ Session/cache    │    │ Files/artifacts  │  │
│   │                  │    │                  │    │                  │  │
│   │ • janua_prod     │    │ • janua sessions │    │ • SBOMs          │  │
│   │ • enclii_prod    │    │ • app caches     │    │ • Build logs     │  │
│   │ • forgesight_prod│    │ • rate limiting  │    │ • User uploads   │  │
│   │ • dhanam_prod    │    │ • job queues     │    │ • Exports        │  │
│   │ • fortuna_prod   │    │                  │    │                  │  │
│   │ • bloomscroll_prod    │                  │    │                  │  │
│   │ • ...            │    │                  │    │                  │  │
│   └──────────────────┘    └──────────────────┘    └──────────────────┘  │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 🚀 Deployment Order (Critical Path)

```
Week 1-2: Infrastructure Bootstrap
├── Hetzner Server (K3s cluster)
├── Cloudflare Tunnel (ingress)
├── PostgreSQL (Ubicloud managed)
├── Redis Sentinel (self-hosted)
└── Container Registry (GHCR)

Week 3-4: Platform Core
├── 1. Janua (CRITICAL - all auth depends on this)
│   ├── PostgreSQL database: janua_prod
│   ├── Redis: sessions, rate limiting
│   └── Domain: auth.janua.dev
│
├── 2. Enclii Core
│   ├── switchyard-api (api.enclii.dev)
│   ├── switchyard-ui (app.enclii.dev)
│   ├── PostgreSQL database: enclii_prod
│   └── Secrets: JWT keys, Janua client credentials
│
├── 3. Roundhouse (builds)
│   ├── roundhouse-api
│   ├── roundhouse-worker(s)
│   └── Redis: job queue
│
└── 4. Waybill (billing)
    ├── waybill-api
    ├── waybill-aggregator
    └── Stripe integration

Week 5-6: App Deployments (via Enclii!)
├── Tier 1 (Ready Now)
│   ├── forgesight (api + web)
│   ├── dhanam (api + web)
│   ├── electrochem-sim (api + web + worker)
│   ├── cotiza-studio (api + web)
│   └── forj (api + web)
│
├── Tier 2 (Needs Polish)
│   ├── fortuna (api + web)
│   ├── avala (api + web)
│   └── blueprint-harvester (api + worker)
│
└── Tier 3 (Active Dev)
    ├── bloom-scroll (api + web)
    └── coforma-studio (api + web)

Week 7+: Business Sites & Extras
├── madfam-site
├── solarpunk-studio
├── coforma-ai
├── enclii landing page
├── enclii docs site
└── status page
```

---

## 📋 Dogfooding Specs Summary

### Platform Services

| Service | Spec File | Domain | Replicas | Autoscale |
|---------|-----------|--------|----------|-----------|
| switchyard-api | `switchyard-api.yaml` | api.enclii.dev | 3 | 3-10 |
| switchyard-ui | `switchyard-ui.yaml` | app.enclii.dev | 2 | 2-8 |
| janua | `janua.yaml` | auth.janua.dev | 3 | 3-10 |
| roundhouse-api | (new) | builds.enclii.dev | 2 | 2-5 |
| roundhouse-worker | (new) | - | 2 | 2-10 |
| waybill-api | (new) | billing.enclii.dev | 2 | 2-5 |

### SaaS Products

| Service | Spec File | Domain | Replicas |
|---------|-----------|--------|----------|
| forgesight-api | `forgesight.yaml` | api.forgesight.quest | 2-10 |
| forgesight-web | `forgesight.yaml` | forgesight.quest | 2-5 |
| dhanam-api | `dhanam.yaml` | api.dhanam.app | 2-6 |
| dhanam-web | `dhanam.yaml` | dhanam.app | 2-4 |
| fortuna-api | `fortuna.yaml` | api.fortuna.app | 2-6 |
| fortuna-web | `fortuna.yaml` | fortuna.app | 2-4 |
| electrochem-sim-api | `electrochem-sim.yaml` | api.electrochem.sim | 2-6 |
| electrochem-sim-web | `electrochem-sim.yaml` | electrochem.sim | 2-4 |
| electrochem-sim-worker | `electrochem-sim.yaml` | - | 2-8 |
| bloom-scroll-api | `bloom-scroll.yaml` | api.bloomscroll.app | 2-6 |
| bloom-scroll-web | `bloom-scroll.yaml` | bloomscroll.app | 2-4 |

---

## 🔐 Security & Network Policies

### Namespace Isolation

```yaml
# Each app gets its own namespace
namespaces:
  - enclii-platform      # Core platform services
  - enclii-janua         # Identity provider (isolated)
  - enclii-forgesight    # Forgesight app
  - enclii-dhanam        # Dhanam app
  - enclii-fortuna       # Fortuna app
  - enclii-electrochem   # Electrochem-sim app
  - enclii-bloomscroll   # Bloom-scroll app
  - enclii-coforma       # Coforma-studio app
  - enclii-avala         # Avala app
  - enclii-blueprint     # Blueprint-harvester
  # ... etc
```

### Network Policies

```
┌─────────────────────────────────────────────────────────────────┐
│                    ALLOWED TRAFFIC FLOWS                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Internet ──► Cloudflare Tunnel ──► Ingress Controller          │
│                                           │                      │
│                                           ▼                      │
│                              ┌────────────────────┐              │
│                              │   Web Services     │              │
│                              │ (port 80/443 only) │              │
│                              └─────────┬──────────┘              │
│                                        │                         │
│  ┌─────────────────────────────────────┼─────────────────────┐  │
│  │                                     ▼                      │  │
│  │  ALL APPS ─────────────────────► JANUA (auth)             │  │
│  │     │                              │                       │  │
│  │     │                              │                       │  │
│  │     ▼                              ▼                       │  │
│  │  Own PostgreSQL DB            Redis (sessions)            │  │
│  │  Own Redis (cache)                                        │  │
│  │                                                            │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                  │
│  DENIED:                                                        │
│  • App A cannot access App B's database                         │
│  • Apps cannot access platform internals (except Janua)        │
│  • Direct pod-to-pod across namespaces (except allowed)        │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 💰 Resource Allocation

### Per-App Resource Budgets

| App | CPU Request | Memory Request | CPU Limit | Memory Limit |
|-----|-------------|----------------|-----------|--------------|
| janua | 200m | 256Mi | 1000m | 1Gi |
| switchyard-api | 250m | 512Mi | 2000m | 2Gi |
| switchyard-ui | 100m | 256Mi | 500m | 512Mi |
| roundhouse-worker | 500m | 1Gi | 2000m | 4Gi |
| forgesight-api | 200m | 512Mi | 1000m | 2Gi |
| bloom-scroll-api | 500m | 1Gi | 2000m | 4Gi |
| electrochem-worker | 1000m | 2Gi | 4000m | 8Gi |

### Total Cluster Resources (Hetzner CPX31 x3)

```
Total Available:
├── vCPU: 12 cores (4 per node)
├── RAM: 24 GB (8 per node)
└── Storage: 480 GB SSD (160 per node)

Platform Overhead (~30%):
├── System pods (kube-system)
├── Ingress controller
├── Monitoring stack
└── Redis Sentinel

Available for Apps (~70%):
├── vCPU: ~8 cores
├── RAM: ~17 GB
└── Storage: ~300 GB
```

---

## 📊 Monitoring & Observability

### Metrics Collection

```
┌─────────────────────────────────────────────────────────────────┐
│                    MONITORING STACK                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   All Services ──► Prometheus ──► Grafana                       │
│       │               │              │                          │
│       │               │              └──► Dashboards            │
│       │               │                   • Platform health     │
│       │               │                   • App metrics         │
│       │               │                   • Usage/billing       │
│       │               │                                         │
│       │               └──► AlertManager ──► Slack/PagerDuty    │
│       │                                                         │
│       └──► Loki (logs) ──► Grafana Log Explorer                │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Key Dashboards

1. **Platform Health** - Enclii core services status
2. **Janua Auth** - Login rates, token issuance, failures
3. **Build Pipeline** - Roundhouse queue, build times
4. **Usage Metrics** - Waybill data, per-project usage
5. **Per-App Dashboards** - Individual app health

---

## 🔄 CI/CD Flow

### Automatic Deployment Pipeline

```
┌─────────────────────────────────────────────────────────────────┐
│                    DEPLOYMENT PIPELINE                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   1. Developer pushes to main                                   │
│          │                                                       │
│          ▼                                                       │
│   2. GitHub webhook ──► Roundhouse API                          │
│          │                                                       │
│          ▼                                                       │
│   3. Roundhouse Worker                                          │
│      ├── Clone repo                                             │
│      ├── Build image (BuildKit)                                 │
│      ├── Generate SBOM (Syft)                                   │
│      ├── Sign image (Cosign)                                    │
│      └── Push to GHCR                                           │
│          │                                                       │
│          ▼                                                       │
│   4. Callback to Switchyard                                     │
│          │                                                       │
│          ▼                                                       │
│   5. Canary Deployment (if configured)                          │
│      ├── 10% traffic ──► 5 min analysis                        │
│      ├── 50% traffic ──► 5 min analysis                        │
│      └── 100% traffic (or rollback)                            │
│          │                                                       │
│          ▼                                                       │
│   6. Waybill records usage event                                │
│          │                                                       │
│          ▼                                                       │
│   7. Slack notification                                         │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 🎯 Success Criteria

### Platform Health
- [ ] Enclii deploys itself successfully
- [ ] Janua authenticates all platform services
- [ ] Roundhouse builds from GitHub webhooks
- [ ] Waybill tracks usage accurately
- [ ] Canary deployments work with auto-rollback

### App Health
- [ ] All Tier 1 apps deployed via Enclii
- [ ] All apps authenticate via Janua
- [ ] Custom domains working (Cloudflare)
- [ ] Autoscaling responding to load
- [ ] Monitoring dashboards populated

### Business Metrics
- [ ] Platform cost < $150/month
- [ ] 99.9% uptime for core services
- [ ] Build time < 5 minutes average
- [ ] Deployment time < 2 minutes
- [ ] Zero security incidents

---

## 📚 Related Documentation

- [DOGFOODING_GUIDE.md](./DOGFOODING_GUIDE.md) - Step-by-step deployment guide
- [Platform Components](./platform_components_implementation_2025_11_27.md) - Roundhouse/Waybill implementation
- [ENCLII_CAPABILITY_MATRIX.md](./ENCLII_CAPABILITY_MATRIX.md) - Feature completeness
- [ENCLII_EXECUTIVE_SUMMARY.md](./ENCLII_EXECUTIVE_SUMMARY.md) - Business case

---

*Last Updated: 2025-11-27*
