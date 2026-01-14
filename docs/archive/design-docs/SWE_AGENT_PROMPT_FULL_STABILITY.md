# Enclii Full Stability & Vision Feature Parity: SWE Agent Prompt

> ⚠️ **Archive Notice (Jan 2026):** This document was written during planning (Dec 2025). Current infrastructure is a single Hetzner AX41-NVME dedicated server (~$55/mo) with self-hosted PostgreSQL/Redis. Ubicloud was evaluated but not used.

**Date:** December 2025
**Target:** Complete Enclii from 70% to 100% production readiness
**Timeline:** 4-6 weeks of focused development
**Infrastructure:** Hetzner AX41-NVME + Cloudflare (~$55/month)

---

## EXECUTIVE SUMMARY

You are an expert SWE agent tasked with completing Enclii, a Railway-style Platform-as-a-Service. The platform is 70% complete with a working GitHub webhook handler, authentication system, and Kubernetes integration. Your mission is to achieve full stability and vision feature parity by completing the remaining deployment pipeline, reconciler verification, and production hardening phases.

### Vision Feature Parity Goals

Enclii must deliver:
1. **Zero-touch deployments** - Push to main → automatic build → deploy
2. **SSO via Janua** - OIDC authentication with RS256 JWT
3. **Multi-tenant architecture** - 100+ custom domains via Cloudflare for SaaS
4. **Dogfooding complete** - Enclii runs on Enclii, authenticated by Janua
5. **99.95% uptime SLO** - Enterprise-grade reliability
6. **<200ms P95 latency** - Performance at scale (1000 RPS)

---

## CURRENT STATE (December 2025)

### Completed ✅

**Phase 1: GitHub Webhook Integration**
- [x] `webhook_handlers.go` with HMAC SHA-256 signature verification
- [x] `GitHubWebhookSecret` in config.go
- [x] `GetByGitRepo` method in ServiceRepository
- [x] Webhook route registered at `/v1/webhooks/github`
- [x] Async build triggering via goroutines
- [x] Branch filtering (main/master only)

**Authentication Infrastructure**
- [x] JWT with RS256 (local mode)
- [x] OIDC mode support (Janua integration ready)
- [x] External JWKS validation for SSO tokens
- [x] RBAC with admin/developer/viewer roles
- [x] Rate limiting with memory-bounded LRU cache
- [x] CSRF protection middleware
- [x] Session management via Redis

**API Endpoints**
- [x] Projects CRUD operations
- [x] Services CRUD operations
- [x] Environments management
- [x] Deployments with rollback
- [x] Build & release management
- [x] Custom domain management
- [x] Topology/dependency graph
- [x] Audit logging infrastructure

**Infrastructure**
- [x] Kubernetes manifests for all services
- [x] PostgreSQL deployment with health checks
- [x] Redis deployment with persistence
- [x] NetworkPolicies for isolation
- [x] Pod security contexts
- [x] Prometheus ServiceMonitor definitions
- [x] Jaeger tracing deployment

---

## REMAINING PHASES

### PHASE 2: Production Build Environment (Priority: 🔴 CRITICAL)

**Objective:** Complete container build pipeline with provenance and signing

#### Tasks

##### 2.1 BuildKit Integration (4-6 hours)
```
Location: apps/switchyard-api/internal/builder/
Status: Scaffolded but incomplete

Required Implementation:
1. Complete BuildService.Build() method to actually execute builds
2. Implement Nixpacks/Buildpacks detection and execution
3. Handle Dockerfile builds when detected
4. Stream build logs to database for UI display
5. Implement build cancellation mechanism
6. Add build timeout handling (30-minute default)

Files to modify:
- internal/builder/service.go (complete Build method)
- internal/builder/nixpacks.go (implement Nixpacks execution)
- internal/builder/dockerfile.go (implement Dockerfile execution)

Verification:
- Unit tests for each build type
- Integration test with real BuildKit
- E2E test: push to repo → build completes → image in registry
```

##### 2.2 Container Registry Push (2-3 hours)
```
Location: apps/switchyard-api/internal/builder/

Required Implementation:
1. Push built images to ghcr.io/madfam registry
2. Implement image tagging strategy (git SHA, semver, latest)
3. Add registry authentication via secrets
4. Implement retry logic for network failures

Files to modify:
- internal/builder/registry.go (new file)
- internal/config/config.go (add registry credentials config)

Verification:
- Image appears in GitHub Container Registry
- Multiple tags applied correctly
- Registry authentication works
```

##### 2.3 SBOM Generation (2-3 hours)
```
Location: apps/switchyard-api/internal/sbom/

Required Implementation:
1. Generate SBOM using Syft during build
2. Store SBOM in Cloudflare R2 for compliance
3. Link SBOM to release record in database
4. Add SBOM download endpoint

Files to modify:
- internal/sbom/generator.go (implement Syft integration)
- internal/api/release_handlers.go (add SBOM download)
- internal/storage/r2.go (R2 upload for SBOMs)

Verification:
- SBOM generated for each build
- SBOM stored in R2 and retrievable
- SBOM format is valid (CycloneDX or SPDX)
```

##### 2.4 Container Signing (3-4 hours)
```
Location: apps/switchyard-api/internal/signing/

Required Implementation:
1. Sign images with cosign using keyless (Fulcio/Rekor)
2. Store signature attestation with release
3. Verify signatures before deployment (admission controller)
4. Add signature verification endpoint

Files to modify:
- internal/signing/cosign.go (implement signing)
- internal/provenance/checker.go (verify signatures)

Verification:
- All images signed with valid attestation
- Unsigned images rejected at deploy time
- Signature chain verifiable via cosign verify
```

---

### PHASE 3: Reconciler Verification (Priority: 🔴 CRITICAL)

**Objective:** Ensure Kubernetes reconcilers correctly sync deployments

#### Tasks

##### 3.1 Deployment Reconciler (4-6 hours)
```
Location: apps/switchyard-api/internal/reconciler/

Required Implementation:
1. Watch for Deployment status changes
2. Update deployment status in database (pending → running → healthy/failed)
3. Handle rollback on failed health checks
4. Implement canary deployment logic
5. Track replica count changes

Files to modify:
- internal/reconciler/controller.go (complete reconcile loop)
- internal/reconciler/deployment.go (deployment sync logic)
- internal/db/repositories.go (add UpdateDeploymentStatus method)

Verification:
- Deployment status syncs to database within 5 seconds
- Failed deployments trigger automatic rollback
- Canary deployments route 10% → 50% → 100%
```

##### 3.2 Service Reconciler (3-4 hours)
```
Location: apps/switchyard-api/internal/reconciler/

Required Implementation:
1. Create/update Kubernetes Services for each Enclii service
2. Manage service ports and selectors
3. Handle service deletion cleanup
4. Sync external DNS/ingress configuration

Files to modify:
- internal/reconciler/service_reconciler.go (complete implementation)

Verification:
- K8s Service created for each Enclii service
- Service ports match container ports
- DNS updated when service changes
```

##### 3.3 Ingress/Domain Reconciler (3-4 hours)
```
Location: apps/switchyard-api/internal/reconciler/

Required Implementation:
1. Create Ingress resources for custom domains
2. Integrate with Cloudflare for SaaS API for SSL
3. Handle domain verification workflow
4. Update domain status based on SSL provisioning

New files needed:
- internal/reconciler/ingress_reconciler.go
- internal/cloudflare/sas_client.go (Cloudflare for SaaS API)

Verification:
- Custom domain → Ingress created → SSL provisioned
- Domain status reflects actual SSL state
- Verification CNAME checked periodically
```

---

### PHASE 4: End-to-End Integration (Priority: 🟠 HIGH)

**Objective:** Complete the push-to-deploy workflow end-to-end

#### Tasks

##### 4.1 Webhook → Build → Deploy Pipeline (4-6 hours)
```
Required Implementation:
1. Wire webhook handler to BuildService
2. On build success, automatically create deployment
3. Track deployment through reconciler to completion
4. Emit events for observability

Integration flow:
GitHub Push → Webhook → Release Created → Build Started →
Build Success → Deployment Created → Reconciler Syncs →
Pods Running → Health Check Passed → Deployment Complete

Files to modify:
- internal/api/webhook_handlers.go (wire to BuildService)
- internal/builder/service.go (emit events on completion)
- internal/services/deployment_service.go (auto-deploy on build success)

Verification:
- Push to main → service running in <10 minutes
- Failed builds don't trigger deployment
- Build logs visible in real-time via API
```

##### 4.2 UI Integration (4-6 hours)
```
Location: apps/switchyard-ui/

Required Implementation:
1. Display deployment status in real-time
2. Show build logs streaming
3. Add rollback button functionality
4. Display service topology graph
5. Implement custom domain management UI

Files to modify:
- apps/switchyard-ui/components/DeploymentStatus.tsx
- apps/switchyard-ui/components/BuildLogs.tsx
- apps/switchyard-ui/app/projects/[slug]/services/[id]/page.tsx

Verification:
- UI reflects deployment status changes in <5 seconds
- Build logs stream without page refresh
- Rollback button works end-to-end
```

##### 4.3 CLI Integration (3-4 hours)
```
Location: packages/cli/

Required Implementation:
1. `enclii deploy` triggers build and waits for completion
2. `enclii logs -f` streams live logs
3. `enclii rollback` executes rollback
4. `enclii status` shows service health

Files to modify:
- packages/cli/cmd/deploy.go
- packages/cli/cmd/logs.go
- packages/cli/cmd/rollback.go
- packages/cli/cmd/status.go

Verification:
- CLI commands work end-to-end
- Proper exit codes for success/failure
- Human-friendly output with progress indicators
```

---

### PHASE 5: Production Hardening (Priority: 🟠 HIGH)

**Objective:** Enterprise-grade reliability and security

#### Tasks

##### 5.1 Observability Stack (4-6 hours)
```
Required Implementation:
1. Deploy Prometheus with Enclii-specific dashboards
2. Add custom metrics for build/deploy pipeline
3. Configure alerting for SLO violations
4. Implement structured logging with correlation IDs

Metrics to track:
- enclii_builds_total (counter by status)
- enclii_build_duration_seconds (histogram)
- enclii_deployments_total (counter by status)
- enclii_deployment_duration_seconds (histogram)
- enclii_api_latency_seconds (histogram by endpoint)
- enclii_active_services_total (gauge)

Files to modify:
- internal/monitoring/metrics.go (add metrics)
- internal/middleware/metrics.go (instrument API)
- internal/builder/service.go (instrument builds)
- infra/k8s/base/prometheus/ (deployment manifests)

Verification:
- Grafana dashboards show all metrics
- Alerts fire when SLO violated (>0.5% errors, >200ms P95)
- Correlation ID links logs across services
```

##### 5.2 Security Hardening (4-6 hours)
```
Required Implementation:
1. Implement Sealed Secrets for all credentials
2. Add Pod Security Admission (restricted mode)
3. Configure Kyverno admission policies
4. Enable Kubernetes audit logging
5. Add Trivy container scanning in CI

Policies to enforce:
- No privileged containers
- No hostPath volumes
- Required resource limits
- No latest tag in production
- Signed images only

Files to create/modify:
- infra/k8s/base/sealed-secrets/
- infra/k8s/base/kyverno/policies/
- .github/workflows/security-scan.yaml

Verification:
- Secrets encrypted at rest
- Policy violations block deployments
- All containers scanned before push
```

##### 5.3 Connection Pooling & HA (2-3 hours)
```
Required Implementation:
1. Deploy PgBouncer for PostgreSQL connection pooling
2. Verify Redis Sentinel failover works
3. Implement graceful shutdown in API
4. Add Pod Disruption Budgets

Files to create/modify:
- infra/k8s/base/pgbouncer/deployment.yaml
- apps/switchyard-api/cmd/api/main.go (graceful shutdown)
- infra/k8s/base/*/pdb.yaml (disruption budgets)

Verification:
- 1000 concurrent API connections work
- Redis failover completes in <20 seconds
- Rolling deploys have zero downtime
```

---

### PHASE 6: Janua SSO Integration (Priority: 🟠 HIGH)

**Objective:** Full OIDC authentication via Janua

#### Tasks

##### 6.1 OIDC Flow Completion (3-4 hours)
```
Location: apps/switchyard-api/internal/auth/

Required Implementation:
1. Complete OIDC callback handling
2. Exchange auth code for tokens
3. Validate ID token and extract claims
4. Create or update user in Enclii database
5. Issue Enclii session cookie

Files to modify:
- internal/auth/oidc_manager.go (complete callback)
- internal/api/auth_handlers.go (OIDCCallback handler)
- internal/services/auth_service.go (user upsert logic)

Verification:
- Login via Janua works end-to-end
- User roles sync from Janua claims
- Session cookie set correctly
```

##### 6.2 Token Validation (2-3 hours)
```
Required Implementation:
1. Validate access tokens from Janua via JWKS
2. Cache JWKS with configurable TTL
3. Handle token expiration gracefully
4. Support both Janua tokens and API keys

Files to modify:
- internal/auth/oidc_manager.go (JWKS validation)
- internal/middleware/auth.go (token validation)

Verification:
- Janua tokens accepted by API
- JWKS cached (not fetched every request)
- Expired tokens rejected with 401
```

##### 6.3 Frontend SSO (3-4 hours)
```
Location: apps/switchyard-ui/

Required Implementation:
1. Add "Login with Janua" button
2. Handle OIDC redirect flow
3. Store tokens in httpOnly cookie or secure storage
4. Add logout functionality

Files to modify:
- apps/switchyard-ui/app/auth/login/page.tsx
- apps/switchyard-ui/lib/auth.ts
- apps/switchyard-ui/contexts/AuthContext.tsx

Verification:
- Login button redirects to Janua
- Callback handles tokens correctly
- User profile displayed after login
```

---

### PHASE 7: Dogfooding Deployment (Priority: 🟡 MEDIUM)

**Objective:** Deploy Enclii on Enclii, authenticated by Janua

#### Tasks

##### 7.1 Self-Deployment Setup (4-6 hours)
```
Service specs already exist in dogfooding/:
- switchyard-api.yaml
- switchyard-ui.yaml
- janua-api.yaml
- janua-dashboard.yaml
- docs-site.yaml
- landing-page.yaml
- status-page.yaml

Required Implementation:
1. Configure Cloudflare Tunnel routing
2. Deploy Enclii services via Enclii CLI
3. Configure inter-service networking
4. Set up continuous deployment from GitHub

Commands to run:
$ enclii project create enclii-platform
$ enclii service create -f dogfooding/switchyard-api.yaml
$ enclii service create -f dogfooding/switchyard-ui.yaml
$ enclii deploy --all

Verification:
- All services running via Enclii deployment
- api.enclii.dev returns 200
- app.enclii.dev loads UI
- auth.enclii.io handles authentication
```

##### 7.2 Status Page Integration (2-3 hours)
```
Required Implementation:
1. Deploy Cachet or similar status page
2. Configure health checks for all services
3. Display public uptime metrics
4. Integrate with alerting

Files to create:
- dogfooding/status-page.yaml (already exists, deploy it)
- Configure monitors for each endpoint

Verification:
- status.enclii.io displays system status
- Incidents auto-created on outages
- Historical uptime visible
```

---

### PHASE 8: Testing & Validation (Priority: 🟡 MEDIUM)

**Objective:** Comprehensive test coverage and validation

#### Tasks

##### 8.1 Unit Test Coverage (6-8 hours)
```
Target: 80%+ code coverage

Priority areas:
1. Authentication middleware (100% coverage)
2. Build service logic (90% coverage)
3. Reconciler sync logic (90% coverage)
4. API handlers (80% coverage)

Files to test:
- internal/auth/*.go
- internal/builder/*.go
- internal/reconciler/*.go
- internal/api/*.go
- internal/services/*.go

Verification:
- go test -cover reports 80%+
- Critical paths have 100% coverage
- All tests pass in CI
```

##### 8.2 Integration Tests (4-6 hours)
```
Required tests:
1. Full build pipeline (webhook → build → push)
2. Deployment pipeline (create → reconcile → healthy)
3. Rollback workflow
4. Custom domain provisioning
5. OIDC authentication flow

Test infrastructure:
- TestContainers for PostgreSQL and Redis
- Mock BuildKit for fast tests
- K8s envtest for reconciler tests

Verification:
- All integration tests pass
- Tests run in <10 minutes
- Tests isolated (no shared state)
```

##### 8.3 E2E Tests with Playwright (4-6 hours)
```
Required tests:
1. User login via Janua
2. Create project and service
3. Trigger deployment
4. View deployment logs
5. Add custom domain
6. Execute rollback

Files to create:
- e2e/tests/auth.spec.ts
- e2e/tests/deployment.spec.ts
- e2e/tests/domains.spec.ts

Verification:
- All E2E tests pass in CI
- Tests run against staging environment
- Screenshot comparisons for UI regressions
```

##### 8.4 Load Testing (2-3 hours)
```
Required tests with k6:
1. API latency under load (1000 RPS)
2. Concurrent deployments (10 simultaneous)
3. Build queue saturation (20 builds)

Performance targets:
- P95 latency < 200ms at 1000 RPS
- Zero errors under sustained load
- Graceful degradation at 2000 RPS

Files to create:
- e2e/load/api-latency.js
- e2e/load/concurrent-deploys.js

Verification:
- P95 < 200ms confirmed
- No OOM errors under load
- Error rate < 0.1% at target load
```

---

## SUCCESS CRITERIA

### Quantitative Metrics

| Metric | Current | Target | Measurement |
|--------|---------|--------|-------------|
| Production Readiness | 70% | 95%+ | Checklist completion |
| Test Coverage | ~40% | 80%+ | go test -cover |
| P95 API Latency | N/A | <200ms | Prometheus |
| Error Rate | N/A | <0.1% | Prometheus |
| Uptime SLO | N/A | 99.95% | Status page |
| Build Time | N/A | <5 min P95 | Prometheus |
| Deploy Time | N/A | <3 min P95 | Prometheus |

### Qualitative Milestones

- [ ] Push to main triggers automatic deployment (zero-touch)
- [ ] All team members can deploy via `enclii deploy`
- [ ] Login works exclusively via Janua SSO
- [ ] Custom domains provision SSL automatically
- [ ] Rollback takes <1 minute
- [ ] All services run on Enclii (dogfooding complete)
- [ ] Status page shows 99.95%+ uptime

---

## REPOSITORY STRUCTURE

```
enclii/
├── apps/
│   ├── switchyard-api/          # Go API (control plane)
│   │   ├── cmd/api/             # Entry point
│   │   └── internal/
│   │       ├── api/             # HTTP handlers
│   │       ├── auth/            # Authentication (JWT/OIDC)
│   │       ├── builder/         # Container builds
│   │       ├── cache/           # Redis caching
│   │       ├── compliance/      # SBOM, compliance exports
│   │       ├── config/          # Viper configuration
│   │       ├── db/              # Database repositories
│   │       ├── k8s/             # Kubernetes client
│   │       ├── logging/         # Structured logging
│   │       ├── middleware/      # Rate limiting, auth
│   │       ├── monitoring/      # Prometheus metrics
│   │       ├── provenance/      # Build provenance
│   │       ├── reconciler/      # K8s reconciliation
│   │       ├── services/        # Business logic
│   │       ├── signing/         # Container signing
│   │       ├── storage/         # R2 object storage
│   │       └── validation/      # Input validation
│   │
│   └── switchyard-ui/           # Next.js frontend
│       ├── app/                 # Pages (App Router)
│       ├── components/          # React components
│       ├── contexts/            # Auth context
│       └── lib/                 # API client
│
├── packages/
│   ├── cli/                     # Go CLI (`enclii` command)
│   └── sdk-go/                  # Go SDK with types
│
├── infra/
│   ├── k8s/                     # Kubernetes manifests
│   │   ├── base/                # Base configurations
│   │   └── overlays/            # Environment overlays
│   └── terraform/               # Hetzner Cloud IaC
│
├── dogfooding/                  # Self-deployment specs
│   ├── switchyard-api.yaml
│   ├── switchyard-ui.yaml
│   ├── janua-api.yaml
│   └── ...
│
├── docs/                        # Documentation
│   ├── architecture/
│   └── production/
│
└── e2e/                         # E2E and load tests
```

---

## KEY CONFIGURATION

### Environment Variables (Production)

```bash
# Database (Ubicloud managed PostgreSQL)
ENCLII_DATABASE_URL=postgres://user:pass@db.ubicloud.com:5432/enclii_prod

# Redis (Sentinel)
ENCLII_REDIS_HOST=redis-sentinel.enclii.svc
ENCLII_REDIS_PORT=26379

# Authentication
ENCLII_AUTH_MODE=oidc
ENCLII_OIDC_ISSUER=https://auth.madfam.io
ENCLII_OIDC_CLIENT_ID=enclii
ENCLII_OIDC_CLIENT_SECRET=<secret>
ENCLII_EXTERNAL_JWKS_URL=https://auth.madfam.io/.well-known/jwks.json

# Container Registry
ENCLII_REGISTRY=ghcr.io/madfam

# Build
ENCLII_BUILDKIT_ADDR=tcp://buildkit.enclii.svc:1234
ENCLII_BUILD_TIMEOUT=1800

# Object Storage (Cloudflare R2)
ENCLII_R2_ENDPOINT=https://<account>.r2.cloudflarestorage.com
ENCLII_R2_ACCESS_KEY_ID=<key>
ENCLII_R2_SECRET_ACCESS_KEY=<secret>
ENCLII_R2_BUCKET=enclii-production

# GitHub Webhook
ENCLII_GITHUB_WEBHOOK_SECRET=<webhook-secret>
ENCLII_GITHUB_TOKEN=<pat-for-api>
```

### Port Allocation

| Service | Port | Domain |
|---------|------|--------|
| Enclii API | 4200 | api.enclii.dev |
| Enclii UI | 4201 | app.enclii.dev |
| Janua API | 4100 | api.janua.dev / auth.madfam.io |
| Janua Dashboard | 4101 | dashboard.janua.dev |

---

## EXECUTION INSTRUCTIONS

### For Each Phase

1. **Read relevant code** before implementing
2. **Write tests first** (TDD approach)
3. **Implement feature** following existing patterns
4. **Run `go build ./...`** to verify compilation
5. **Run tests** to verify functionality
6. **Update CLAUDE.md** if patterns change

### Build & Test Commands

```bash
# Build all Go code
cd apps/switchyard-api && GOWORK=$PWD/../../go.work go build ./...

# Run unit tests
go test ./... -v

# Run with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run integration tests (requires Docker)
docker-compose -f docker-compose.test.yaml up -d
go test ./... -tags=integration

# Run E2E tests
cd e2e && npx playwright test
```

### Deployment Commands

```bash
# Build Docker image
docker build -t enclii-api:latest -f apps/switchyard-api/Dockerfile .

# Deploy to k8s
kubectl apply -k infra/k8s/overlays/production

# Check deployment
kubectl -n enclii get pods
kubectl -n enclii logs -f deployment/switchyard-api
```

---

## CRITICAL SUCCESS FACTORS

1. **Don't break existing functionality** - Run tests before and after changes
2. **Follow existing patterns** - Check similar code before implementing
3. **Complete features end-to-end** - Don't leave partial implementations
4. **Write tests for critical paths** - Especially auth and deployments
5. **Document significant changes** - Update CLAUDE.md as needed
6. **Verify in staging before production** - Never skip validation

---

## EXPECTED OUTCOMES

Upon completing all phases:

1. **Developers** can push to main and see their service running in <10 minutes
2. **Platform team** has observability into all deployments and builds
3. **Security** is enterprise-grade with signed images and audit trails
4. **Reliability** hits 99.95% uptime through HA and auto-rollback
5. **Dogfooding** proves the platform by running itself on itself

---

*This prompt provides a comprehensive roadmap for completing Enclii to full production readiness. Execute phases in order, verify each phase before moving on, and maintain the quality bar throughout.*
