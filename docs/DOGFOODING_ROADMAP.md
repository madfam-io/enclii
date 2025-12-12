# MADFAM Ecosystem Full Dogfooding Roadmap

**Goal**: Achieve complete self-hosting where Enclii manages ALL DevOps for the entire MADFAM ecosystem, including auto-deploying both `janua` and `enclii` repositories.

**Date**: 2025-12-12
**Status**: ~70% Complete

---

## Current State Summary

### What's Working ✅

| Component | Status | Notes |
|-----------|--------|-------|
| **Enclii Platform** | ✅ Running | api.enclii.dev, app.enclii.dev |
| **Janua SSO** | ✅ Running | auth.madfam.io (OIDC provider) |
| **GitHub OAuth** | ✅ Working | Repo import via Janua-linked GitHub |
| **Service Import UI** | ✅ Complete | Monorepo path detection working |
| **Reconciler** | ✅ Fixed | Lifecycle + namespace handling |
| **Build Pipeline** | ✅ Working | Buildpacks/Dockerfile detection |
| **K8s Deployments** | ✅ Working | Reconciler creates pods correctly |

### Projects in Enclii Database

| Project | Services Registered | Auto-Deploy | Webhooks |
|---------|---------------------|-------------|----------|
| **enclii** | switchyard-api, switchyard-ui | ❌ None | ❌ None |
| **Janua** | janua-api, janua-admin, janua-dashboard | ❌ None | ❌ None |

---

## Remaining Work

### Phase A: Complete Service Registration (1-2 hours)

**Goal**: Register ALL services from both repos in Enclii.

#### Enclii Repo (`madfam-io/enclii`)
Currently registered: `switchyard-api`, `switchyard-ui`

**Still need to import**:
1. `docs-site` (apps/docs-site) - Documentation portal
2. `landing-page` (apps/landing-page) - Marketing site
3. `status-page` (apps/status-page) - Status monitoring

#### Janua Repo (`madfam-io/janua`)
Currently registered: `janua-api`, `janua-admin`, `janua-dashboard`

**Complete** - All Janua services registered.

**Action Items**:
```bash
# Via UI: app.enclii.dev → enclii project → Import from GitHub
# Select: docs-site, landing-page, status-page (if they exist)
```

---

### Phase B: Enable Auto-Deploy for All Services (1 hour)

**Goal**: Configure each service for automatic deployment on git push.

**Per-Service Configuration Needed**:
1. Set `auto_deploy = true`
2. Set `auto_deploy_branch = "main"`
3. Set `auto_deploy_env = "production"` (or appropriate environment)
4. Configure build settings (Dockerfile path, build args)

**Services to Configure**:

| Service | Repo | Root Path | Branch |
|---------|------|-----------|--------|
| switchyard-api | enclii | apps/switchyard-api | main |
| switchyard-ui | enclii | apps/switchyard-ui | main |
| docs-site | enclii | apps/docs-site | main |
| janua-api | janua | apps/api | main |
| janua-admin | janua | apps/admin | main |
| janua-dashboard | janua | apps/dashboard | main |

**Action Items**:
```bash
# Via UI: app.enclii.dev → Service → Settings → Enable Auto-Deploy
# Or via API:
curl -X PATCH https://api.enclii.dev/v1/services/{id} \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"auto_deploy": true, "auto_deploy_branch": "main", "auto_deploy_env": "production"}'
```

---

### Phase C: Configure GitHub Webhooks (1-2 hours)

**Goal**: Set up webhooks so GitHub notifies Enclii on every push.

**Current State**: Webhooks table exists but no webhooks configured.

**Webhook Requirements**:

1. **enclii repo webhook**:
   - URL: `https://api.enclii.dev/v1/webhooks/github`
   - Events: `push`, `pull_request`
   - Secret: HMAC signature verification

2. **janua repo webhook**:
   - URL: `https://api.enclii.dev/v1/webhooks/github`
   - Events: `push`, `pull_request`
   - Secret: HMAC signature verification

**Action Items**:
```bash
# Option 1: Manual via GitHub UI
# Go to: github.com/madfam-io/enclii/settings/hooks
# Add webhook with:
#   Payload URL: https://api.enclii.dev/v1/webhooks/github
#   Content type: application/json
#   Secret: [generate and store in ENCLII_WEBHOOK_SECRET]

# Option 2: Via GitHub CLI (if admin:repo_hook scope granted)
gh api repos/madfam-io/enclii/hooks -X POST \
  -f name=web \
  -f config[url]=https://api.enclii.dev/v1/webhooks/github \
  -f config[content_type]=json \
  -f config[secret]=$WEBHOOK_SECRET \
  -f events[]=push \
  -f active=true
```

**API Endpoint Status**:
- `POST /v1/webhooks/github` - ✅ Exists and handles webhook payloads
- Webhook secret verification - ✅ HMAC-SHA256 implemented

---

### Phase D: Environment Variables Management (2-3 hours)

**Goal**: Each service needs proper env vars configured in Enclii.

**Current State**: Services deployed but env vars may be hardcoded or missing.

**Required Env Vars by Service**:

#### switchyard-api
```
ENCLII_DATABASE_URL=postgres://...
ENCLII_REDIS_HOST=...
ENCLII_AUTH_MODE=oidc
ENCLII_OIDC_ISSUER=https://auth.madfam.io
ENCLII_OIDC_CLIENT_ID=...
ENCLII_REGISTRY=ghcr.io/madfam-io
ENCLII_GITHUB_TOKEN=...
```

#### switchyard-ui
```
NEXT_PUBLIC_API_URL=https://api.enclii.dev
NEXT_PUBLIC_AUTH_MODE=oidc
NEXT_PUBLIC_JANUA_URL=https://auth.madfam.io
```

#### janua-api
```
DATABASE_URL=postgres://...
REDIS_URL=redis://...
JWT_PRIVATE_KEY=...
GITHUB_CLIENT_ID=...
GITHUB_CLIENT_SECRET=...
```

**Action Items**:
1. Implement environment variables UI in switchyard-ui
2. Create API endpoints for env var CRUD
3. Store secrets securely (encrypted at rest)
4. Inject env vars during deployment

---

### Phase E: Custom Domains (1-2 hours)

**Goal**: Configure proper domain routing for all services.

**Domain Mapping**:
| Service | Domain | Status |
|---------|--------|--------|
| switchyard-api | api.enclii.dev | ✅ Working |
| switchyard-ui | app.enclii.dev | ✅ Working |
| docs-site | docs.enclii.dev | ✅ Working |
| janua-api | api.janua.dev / auth.madfam.io | ✅ Working |
| janua-admin | admin.janua.dev | ⏳ Pending |
| janua-dashboard | app.janua.dev | ⏳ Pending |

**Action Items**:
1. Implement custom domains UI
2. Configure Cloudflare DNS + tunnel routes
3. SSL certificate management (Let's Encrypt via cert-manager)

---

### Phase F: Database Management (2-3 hours)

**Goal**: Provision and manage PostgreSQL databases per-service.

**Current State**: Using shared Ubicloud PostgreSQL instance.

**Target Architecture**:
- Shared instance OK for cost optimization
- Per-service database separation
- Connection pooling via PgBouncer
- Automated backups

**Services Needing Database**:
- switchyard-api (enclii_prod database)
- janua-api (janua_prod database)

---

### Phase G: End-to-End Validation (1 hour)

**Goal**: Verify full auto-deploy pipeline works.

**Test Scenario**:
1. Make code change in `janua` repo
2. Push to `main` branch
3. Webhook triggers Enclii build
4. Build completes, pushes to ghcr.io
5. Reconciler picks up new release
6. Kubernetes deployment updates
7. Service running with new code
8. Health checks pass

**Validation Checklist**:
- [ ] Git push triggers webhook
- [ ] Webhook creates new build job
- [ ] Build completes successfully
- [ ] Image pushed to registry
- [ ] Release created in database
- [ ] Deployment created with status=pending
- [ ] Reconciler processes deployment
- [ ] Kubernetes pods updated
- [ ] Health checks passing
- [ ] Service accessible at domain

---

## Success Criteria

**Full Dogfooding Achieved When**:

1. **Zero Manual Deployments**: All services deploy automatically on git push
2. **Self-Managing**: Enclii deploys Enclii (recursive dogfooding)
3. **Complete Coverage**: ALL MADFAM services managed by Enclii
4. **Production-Ready**: High availability, monitoring, rollback capability

---

## Priority Order

| Priority | Phase | Effort | Impact |
|----------|-------|--------|--------|
| 🔴 P0 | B: Auto-Deploy Config | 1h | Enables automation |
| 🔴 P0 | C: GitHub Webhooks | 2h | Triggers deployments |
| 🟡 P1 | D: Env Variables | 3h | Proper configuration |
| 🟡 P1 | A: Service Registration | 1h | Complete coverage |
| 🟢 P2 | E: Custom Domains | 2h | Professional URLs |
| 🟢 P2 | F: Database Management | 3h | Data isolation |
| 🟢 P2 | G: Validation | 1h | Confidence |

---

## Quick Start for Next Session

```bash
# 1. Get fresh auth token
PASSWORD='YS9V9CK!qmR2s&' curl -s -X POST https://auth.madfam.io/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@madfam.io","password":"'"$PASSWORD"'"}' | jq -r .access_token

# 2. Check current services
curl -s https://api.enclii.dev/v1/projects -H "Authorization: Bearer $TOKEN" | jq

# 3. Enable auto-deploy on a service
curl -X PATCH https://api.enclii.dev/v1/services/{service_id} \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"auto_deploy": true, "auto_deploy_branch": "main"}'

# 4. Configure GitHub webhook manually via GitHub UI
# github.com/madfam-io/enclii/settings/hooks → Add webhook
```

---

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                     MADFAM Ecosystem                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐      ┌─────────────┐      ┌─────────────┐    │
│  │   GitHub    │──────▶   Enclii    │──────▶ Kubernetes  │    │
│  │  Webhooks   │      │   API       │      │   Cluster   │    │
│  └─────────────┘      └─────────────┘      └─────────────┘    │
│         │                   │                     │            │
│         │                   ▼                     │            │
│         │            ┌─────────────┐              │            │
│         │            │  Buildpacks │              │            │
│         │            │   Builder   │              │            │
│         │            └─────────────┘              │            │
│         │                   │                     │            │
│         │                   ▼                     │            │
│         │            ┌─────────────┐              │            │
│         │            │   ghcr.io   │              │            │
│         │            │  Registry   │              │            │
│         │            └─────────────┘              │            │
│         │                   │                     │            │
│         ▼                   ▼                     ▼            │
│  ┌─────────────────────────────────────────────────────┐      │
│  │              Deployed Services                       │      │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐            │      │
│  │  │  Enclii  │ │  Janua   │ │   Docs   │   ...      │      │
│  │  │   API    │ │   SSO    │ │   Site   │            │      │
│  │  └──────────┘ └──────────┘ └──────────┘            │      │
│  └─────────────────────────────────────────────────────┘      │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Git Commits Made This Session

1. **janua repo** (pushed):
   - `df041d7 fix(reconciler): implement proper reconciler lifecycle and namespace handling`

2. **enclii repo** (pushed):
   - `443a2fc fix(reconciler): implement proper lifecycle and namespace handling`

---

*Last Updated: 2025-12-12 by Claude Code*
