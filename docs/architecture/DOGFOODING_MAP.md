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

## 📦 Complete Repository Inventory (17 Actual Repos)

### 🏛️ Tier 0: Platform Infrastructure (Deploy First)

| Repo | Purpose | Tech Stack |
|------|---------|------------|
| **enclii** | PaaS Control Plane | Go, React, PostgreSQL, Redis |
| **janua** | Self-hosted Auth (OIDC/OAuth2) | Python/FastAPI, PostgreSQL |

### 💼 Tier 1: Production SaaS Products

| Repo | Purpose | Tech Stack |
|------|---------|------------|
| **forgesight** | Global Fabrication Pricing Intelligence | Python/FastAPI, React, PostgreSQL |
| **dhanam** | Budget & Wealth Tracking (LATAM-first) | TypeScript, React Native, PostgreSQL |
| **fortuna** | Problem Intelligence Platform | Python, React |
| **digifab-quoting** (cotiza.studio) | Digital Manufacturing Quoting | Node.js, React |
| **coforma-studio** | Customer Advisory Boards SaaS | Node.js, Next.js |

### 🔬 Tier 2: Specialized Apps

| Repo | Purpose | Tech Stack |
|------|---------|------------|
| **electrochem-sim** (Galvana) | Electrochemistry Simulation Platform | Python, React, Redis |
| **sim4d** | Web-first Parametric CAD | TypeScript, WASM/OCCT |
| **bloom-scroll** | Anti-doomscroll Content Aggregator | FastAPI, Flutter |
| **avala** | Learning & Competency Cloud (MX compliance) | TBD |
| **blueprint-harvester** | 3D Printable Blueprint Discovery Engine | Python, OpenSearch, MinIO |
| **forj** | Decentralized Fabrication Storefront Builder | Three.js, Blockchain |

### 🌐 Tier 3: Business Sites

| Repo | Purpose | Tech Stack |
|------|---------|------------|
| **madfam-site** | MADFAM Corporate Website | Next.js 14, TypeScript |
| **aureo-labs** | Aureo Labs Website | Next.js |
| **primavera3d** | 3D Modeling/Fabrication Portfolio | Next.js, Turbo |

### 🔧 Tier 4: Libraries & Infrastructure

| Repo | Purpose | Used By |
|------|---------|---------|
| **geom-core** | C++ Geometry Engine (Python/WASM bindings) | sim4d, forgesight, digifab-quoting |
| **solarpunk-foundry** | Ops, Scripts, Shared Infra | All repos |

---

## 🔗 Interconnectivity Map

### Authentication Flow (Janua Hub)

```
                              ┌─────────────────┐
                              │      JANUA      │
                              │ (auth.janua.dev)│
                              └────────┬────────┘
                                       │
              ┌────────────────────────┼────────────────────────┐
              │                        │                        │
              ▼                        ▼                        ▼
    ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
    │  Platform       │    │  SaaS Products  │    │  Specialized    │
    │                 │    │                 │    │                 │
    │ • enclii UI     │    │ • forgesight    │    │ • galvana       │
    │ • admin panels  │    │ • dhanam        │    │ • sim4d         │
    │                 │    │ • fortuna       │    │ • bloom-scroll  │
    │                 │    │ • cotiza.studio │    │ • avala         │
    │                 │    │ • coforma       │    │ • forj          │
    └─────────────────┘    └─────────────────┘    └─────────────────┘
```

**OAuth 2.0 / OIDC Flow**:
1. User visits `app.forgesight.quest`
2. Redirect to `auth.janua.dev/authorize`
3. User logs in (password/SSO/social)
4. Janua issues RS256 JWT
5. Redirect back with token
6. App validates JWT via Janua JWKS

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
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐           │    │
│  │  │forgesight│ │  dhanam  │ │ galvana  │ │  sim4d   │  ...      │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘           │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                           │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## 🚀 Deployment Order (Critical Path)

```
Week 1-2: Infrastructure Bootstrap
├── Hetzner Server (K3s cluster)
├── Cloudflare Tunnel (ingress)
├── PostgreSQL (Ubicloud managed)
├── Redis Sentinel
└── Container Registry (GHCR)

Week 2-3: Platform Core
├── 1. Janua (CRITICAL - all auth depends on this)
│   └── Domain: auth.janua.dev
│
├── 2. Enclii Core
│   ├── switchyard-api
│   ├── switchyard-ui
│   └── Domain: app.enclii.dev
│
├── 3. Roundhouse (build pipeline)
└── 4. Waybill (usage/billing)

Week 4-5: SaaS Products
├── forgesight (forgesight.quest)
├── dhanam (dhanam.app)
├── fortuna (fortuna.tube)
├── digifab-quoting (cotiza.studio)
└── coforma-studio (coforma.studio)

Week 6-7: Specialized Apps
├── electrochem-sim / Galvana
├── sim4d (sim4d.com)
├── bloom-scroll
├── avala
├── blueprint-harvester
└── forj (forj.design)

Week 8+: Business Sites
├── madfam-site (madfam.io)
└── aureo-labs (aureolabs.dev)
```

---

## 🔐 Namespace Isolation

```yaml
namespaces:
  - enclii-platform      # Core platform (switchyard, roundhouse, waybill)
  - enclii-janua         # Identity provider (isolated)
  - enclii-forgesight    # Forgesight
  - enclii-dhanam        # Dhanam
  - enclii-fortuna       # Fortuna
  - enclii-cotiza        # Cotiza Studio (digifab-quoting)
  - enclii-coforma       # Coforma Studio
  - enclii-galvana       # Electrochem-sim
  - enclii-sim4d         # Sim4D
  - enclii-bloomscroll   # Bloom Scroll
  - enclii-avala         # Avala
  - enclii-blueprint     # Blueprint Harvester
  - enclii-forj          # Forj
```

---

## 💰 Resource Allocation

### Per-App Resource Budgets

| App | CPU Request | Memory Request | Notes |
|-----|-------------|----------------|-------|
| janua | 200m | 256Mi | Auth - always on |
| switchyard-api | 250m | 512Mi | Platform core |
| roundhouse-worker | 500m | 1Gi | Build jobs |
| forgesight-api | 200m | 512Mi | Pricing engine |
| galvana-worker | 1000m | 2Gi | Simulation heavy |
| sim4d-api | 500m | 1Gi | CAD processing |
| geom-core (WASM) | - | - | Client-side |

### Cluster Resources (Hetzner CPX31 x3)

```
Total: 12 vCPU, 24GB RAM, 480GB SSD
Platform overhead: ~30%
Available for apps: ~8 vCPU, 17GB RAM
```

---

## 🔄 CI/CD Flow

```
┌──────────────────────────────────────────────────────────────────┐
│  1. git push main                                                │
│         │                                                        │
│         ▼                                                        │
│  2. GitHub webhook ──► Roundhouse                               │
│         │                                                        │
│         ▼                                                        │
│  3. BuildKit ──► SBOM (Syft) ──► Sign (Cosign) ──► GHCR        │
│         │                                                        │
│         ▼                                                        │
│  4. Callback ──► Switchyard ──► K8s Deploy                      │
│         │                                                        │
│         ▼                                                        │
│  5. Waybill records usage ──► Stripe (if billable)              │
└──────────────────────────────────────────────────────────────────┘
```

---

## 🎯 Success Criteria

### Platform
- [ ] Enclii deploys itself
- [ ] Janua authenticates all services
- [ ] Roundhouse builds from webhooks
- [ ] Waybill tracks usage accurately

### Apps
- [ ] All 12 deployable apps running via Enclii
- [ ] All apps authenticate via Janua
- [ ] Custom domains working
- [ ] Autoscaling responding to load

### Business
- [ ] Platform cost < $150/month
- [ ] 99.9% uptime for core services
- [ ] Build time < 5 minutes
- [ ] Zero security incidents

---

## 📚 App Quick Reference

| App | Domain | What It Does |
|-----|--------|--------------|
| **enclii** | enclii.dev | Railway-style PaaS (~$100/mo) |
| **janua** | janua.dev | Self-hosted Auth0 alternative |
| **forgesight** | forgesight.quest | Fabrication pricing intelligence |
| **dhanam** | dhanam.app | Budget/wealth tracking (LATAM) |
| **fortuna** | fortuna.tube | Problem discovery platform |
| **cotiza.studio** | cotiza.studio | Manufacturing quoting |
| **coforma** | coforma.studio | Customer advisory boards |
| **galvana** | galvana.io | Electrochemistry simulation |
| **sim4d** | sim4d.com | Browser-based parametric CAD |
| **bloom-scroll** | bloomscroll.app | Anti-doomscroll content |
| **avala** | avala.mx | Learning/competency (MX) |
| **blueprint-harvester** | - | 3D blueprint discovery |
| **forj** | forj.design | Decentralized fab storefronts |
| **madfam-site** | madfam.io | Corporate website |
| **aureo-labs** | aureolabs.dev | Aureo Labs website |
| **primavera3d** | primavera3d.com | 3D modeling/fab portfolio |
| **geom-core** | (library) | Geometry engine |
| **solarpunk-foundry** | (ops) | Shared infrastructure |

---

*Last Updated: 2025-11-27*
*Total Repos: 18*
