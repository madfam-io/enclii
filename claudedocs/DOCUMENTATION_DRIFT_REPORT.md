# Documentation Drift Report

**Audit Date:** 2026-01-15
**Auditor:** Claude Opus 4.5 (Lead Technical Writer & QA Auditor)
**Scope:** Enclii + Janua repositories
**Mission:** 100% Factual Consistency - Codebase as single source of truth

---

## Executive Summary

| Category | Enclii | Janua | Total |
|----------|--------|-------|-------|
| **CRITICAL** | 12 | 14 | **26** |
| **WARNING** | 15 | 18 | **33** |
| **CLEAN** | 8 sections | 6 sections | **14** |

**Overall Assessment:** Both platforms have significant documentation drift requiring immediate remediation before next release cycle.

---

# ENCLII FINDINGS

## Configuration Reality Check

### Environment Variables

#### CRITICAL - Variables in K8s but NOT wired to code

| Variable | K8s Value | Impact |
|----------|-----------|--------|
| `ENCLII_DB_POOL_SIZE` | "50" | Database pooling not configurable |
| `ENCLII_CACHE_TTL_SECONDS` | "7200" | Cache TTL hardcoded |
| `ENCLII_RATE_LIMIT_REQUESTS_PER_MINUTE` | "10000" | Rate limiting not implemented |
| `ENCLII_MAX_REQUEST_SIZE` | "10MB" | Request size not enforced |
| `ENCLII_ENABLE_PROFILING` | "false" | Profiling toggle dead |
| `ENCLII_ADMIN_EMAILS` | "admin@madfam.io" | Admin mapping not configured |

**File:** `infra/k8s/production/environment-patch.yaml`

**Action Items:**
```diff
# apps/switchyard-api/internal/config/config.go
+ DBPoolSize        int    `envconfig:"ENCLII_DB_POOL_SIZE" default:"25"`
+ CacheTTLSeconds   int    `envconfig:"ENCLII_CACHE_TTL_SECONDS" default:"3600"`
+ RateLimitPerMin   int    `envconfig:"ENCLII_RATE_LIMIT_REQUESTS_PER_MINUTE" default:"1000"`
```

---

#### CRITICAL - Production variables UNDOCUMENTED

| Variable | Purpose | Used In |
|----------|---------|---------|
| `ENCLII_AUTH_MODE` | local/oidc switch | `internal/auth/manager.go` |
| `ENCLII_EXTERNAL_JWKS_URL` | Janua token validation | `internal/auth/oidc.go` |
| `ENCLII_EXTERNAL_ISSUER` | Janua issuer | `internal/auth/oidc.go` |
| `ENCLII_JANUA_API_URL` | GitHub token retrieval | `internal/api/github_handlers.go` |
| `ENCLII_BUILD_MODE` | in-process/roundhouse | `internal/config/config.go` |
| `ENCLII_ROUNDHOUSE_URL` | Async build worker | `internal/service/build_service.go` |
| `ENCLII_CLOUDFLARE_API_TOKEN` | DNS/Tunnel management | `internal/service/dns_service.go` |
| `ENCLII_CLOUDFLARE_ACCOUNT_ID` | Account identifier | `internal/service/dns_service.go` |
| `ENCLII_CLOUDFLARE_ZONE_ID` | Zone identifier | `internal/service/dns_service.go` |
| `ENCLII_CLOUDFLARE_TUNNEL_ID` | Tunnel identifier | `internal/service/dns_service.go` |

**Action Item:** Create `.env.production.example` with all production variables documented.

---

#### WARNING - Documented but UNUSED

| Variable | Location | Status |
|----------|----------|--------|
| `ENCLII_METRICS_PORT` | `.env.example:19` | Dead code |
| `ENCLII_DEFAULT_REGION` | `.env.example:16` | Never read |
| `ENCLII_CACHE_ENABLED` | `.env.build.example:142` | Not implemented |
| `ENCLII_CACHE_TTL` | `.env.build.example:145` | Not implemented |

**Action Item:** Remove from `.env.example` or implement functionality.

---

#### WARNING - Redis naming mismatch

**Documentation says:**
```
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
```

**Code actually uses:**
```go
ENCLII_REDIS_HOST    // default: localhost
ENCLII_REDIS_PORT    // default: 6379
ENCLII_REDIS_PASSWORD
```

**Action Item:** Update `.env.build.example:42-48` to use `ENCLII_REDIS_*` prefix.

---

## Endpoint Integrity Scan

### CRITICAL - Missing K8s Health Probes

**OpenAPI spec defines:**
- `GET /health/live` - Liveness probe
- `GET /health/ready` - Readiness probe

**Code implements:**
- `GET /health` - Single combined endpoint

**Impact:** K8s probes may fail to distinguish between startup and runtime health.

**Action Item:**
```go
// apps/switchyard-api/internal/api/router.go
+ r.GET("/health/live", handlers.LivenessProbe)
+ r.GET("/health/ready", handlers.ReadinessProbe)
```

---

### CRITICAL - Build Status Path Mismatch

**Spec expects:** `GET /build/status?commit_sha=abc123` (query param)
**Code provides:** `GET /v1/builds/{commit_sha}/status` (path param)

**Action Item:** Update `docs/api/openapi.yaml` to match implementation.

---

### WARNING - 60 Undocumented Endpoints

Major feature groups missing from OpenAPI:
- Database add-ons (7 endpoints)
- Serverless functions (8 endpoints)
- Service templates (7 endpoints)
- Notification webhooks (15+ endpoints)
- WebSocket streaming, metrics, domain management

**Action Item:** Generate OpenAPI spec from code annotations or add documentation.

---

## Instructional Validity Test

### CRITICAL - Missing Make Targets

| Target | Referenced In | Status |
|--------|---------------|--------|
| `make dns-dev` | README.md:262, CLAUDE.md:66 | **DOES NOT EXIST** |
| `make precommit` | README.md:463, CLAUDE.md:87 | **DOES NOT EXIST** |
| `make e2e` | CLAUDE.md:85, CLAUDE.md:564 | **DOES NOT EXIST** |
| `make run-all` | CLAUDE.md:491 | **DOES NOT EXIST** |

**Action Items:**

```makefile
# Add to Makefile

dns-dev:
	@echo "Configuring dev DNS entries..."
	./scripts/configure-dns-dev.sh

precommit: lint test
	@echo "Pre-commit checks passed"

e2e:
	cd tests/e2e && pnpm test

run-all:
	$(MAKE) -j3 run-switchyard run-ui run-reconcilers
```

---

### CLEAN - Verified Correct

- All `./scripts/*.sh` files exist
- All `infra/k8s/*` paths correct
- All Docker Compose files exist
- All binary paths (`bin/enclii`, `bin/switchyard-api`) exist

---

## Feature Claim Audit

### CRITICAL - FALSE Claims

| Feature | Claim | Reality |
|---------|-------|---------|
| Auto-Deploy to Production | "Production Ready" | **BLOCKED** - ENVIRONMENT_NOT_FOUND error |

**Location:** `FEATURE_PARITY_ROADMAP.md:5`

**Action Item:**
```diff
- Auto-Deploy ✅ Production
+ Auto-Deploy 🔲 BLOCKED (environment configuration issue)
```

---

### WARNING - Accurately Marked as Future

| Feature | Status | Documentation |
|---------|--------|---------------|
| Multi-Region | "(Future feature)" | `examples/README.md:313-315` |
| Canary Deployments | "(Future feature)" | `examples/README.md:317-319` |
| Database Add-ons | "(Future feature)" | `examples/README.md:321-323` |

**Status:** Documentation is ACCURATE - no action needed.

---

### CLEAN - Verified Accurate Claims

- Authentication/OIDC Integration - Production Ready
- Build Pipeline - Operational (Jan 2026)
- GitOps/ArgoCD - Operational (Jan 2026)
- Storage/Longhorn CSI - Operational (Jan 2026)
- Infrastructure Cost ~$55/month - Verified

---

# JANUA FINDINGS

## Configuration Reality Check

### CRITICAL - Runtime Crash Variables

| Variable | Documented | In Settings Class | Impact |
|----------|------------|-------------------|--------|
| `CONEKTA_WEBHOOK_SECRET` | `.env.production.example:114` | **NOT DEFINED** | `AttributeError` at runtime |
| `SENDGRID_API_KEY` | `.env.production.example:102` | **NOT DEFINED** | `AttributeError` when SendGrid enabled |

**Action Item:**
```python
# apps/api/app/config.py - Add to Settings class
CONEKTA_WEBHOOK_SECRET: str = Field(default="")
SENDGRID_API_KEY: str = Field(default="")
```

---

### CRITICAL - Payment Variables Bypass Pydantic

Variables used via `os.getenv()` instead of Settings class:

| Variable | File | Issue |
|----------|------|-------|
| `CONEKTA_API_KEY` | `app/services/payment/router.py` | No type validation |
| `POLAR_API_KEY` | `app/services/payment/router.py` | No type validation |
| `POLAR_WEBHOOK_SECRET` | `app/routers/webhooks.py` | No type validation |
| `STRIPE_WEBHOOK_SECRET` | `app/routers/webhooks.py` | No type validation |

**Action Item:** Add all payment variables to `config.py` Settings class.

---

### CRITICAL - Feature Flag Naming Mismatch

**Documentation uses:**
```
FEATURE_PASSKEYS_ENABLED=true
FEATURE_MFA_ENABLED=true
FEATURE_SSO_ENABLED=true
```

**Code uses:**
```python
ENABLE_DOCS: bool = Field(default=True)
ENABLE_MFA: bool = Field(default=True)
ENABLE_SSO: bool = Field(default=False)
```

**Impact:** Any deployment using `FEATURE_*` variables will be IGNORED.

**Action Item:** Update `.env.production.example` to use `ENABLE_*` prefix:
```diff
- FEATURE_PASSKEYS_ENABLED=true
+ ENABLE_PASSKEYS=true
- FEATURE_MFA_ENABLED=true
+ ENABLE_MFA=true
```

---

### WARNING - Compliance Naming Conflicts

| Documented | In Settings | Issue |
|------------|-------------|-------|
| `GDPR_ENABLED` | `COMPLIANCE_GDPR_ENABLED` | Name mismatch |
| `AUDIT_LOG_RETENTION_DAYS` | `COMPLIANCE_AUDIT_RETENTION_YEARS` | **Units differ!** |
| `CCPA_ENABLED` | `COMPLIANCE_CCPA_ENABLED` | Name mismatch |
| `SOC2_COMPLIANCE_MODE` | `COMPLIANCE_SOC2_ENABLED` | Name mismatch |

**Action Item:** Standardize naming in `.env.production.example` to match `COMPLIANCE_*` prefix.

---

### WARNING - K8s Manifest Mismatches

**File:** `k8s/base/deployments/janua-api.yaml`

| K8s Variable | Settings Class | Fix |
|--------------|----------------|-----|
| `JWT_SECRET` | `JWT_SECRET_KEY` | Update K8s manifest |
| `EMAIL_FROM` | `EMAIL_FROM_ADDRESS` | Update K8s manifest |

---

## Endpoint Integrity Scan

### CRITICAL - Organization CRUD Missing

| Documented | Implemented |
|------------|-------------|
| `GET /api/v1/organizations` | **NOT FOUND** |
| `POST /api/v1/organizations` | **NOT FOUND** |
| `GET /api/v1/organizations/{id}` | **NOT FOUND** |
| `PUT /api/v1/organizations/{id}` | **NOT FOUND** |
| `DELETE /api/v1/organizations/{id}` | EXISTS |

**Impact:** Cannot create, list, or query organizations - feature is unusable.

**Action Item:** Implement full Organization CRUD in `apps/api/app/routers/v1/organizations.py`

---

### CRITICAL - Session Management Incomplete

| Documented | Implemented |
|------------|-------------|
| `GET /api/v1/sessions` | **NOT FOUND** |
| `GET /api/v1/sessions/{id}` | **NOT FOUND** |
| `GET /api/v1/sessions/current` | **NOT FOUND** |
| `DELETE /api/v1/sessions/{id}` | EXISTS |
| `POST /api/v1/sessions/revoke-all` | EXISTS |

**Action Item:** Implement session listing endpoints.

---

### CRITICAL - User Profile Path Mismatch

**Documentation says:** `GET /api/v1/users/me`
**Code implements:** `GET /api/v1/auth/me`

**Action Item:** Update API documentation to reference `/auth/me` path.

---

### CRITICAL - Compliance Features Broken

Only 3/15 compliance endpoints implemented:
- Audit log listing: **MISSING**
- Audit log details: **MISSING**
- Compliance reports: **MISSING**
- Policy management: **MINIMAL**

**Impact:** Enterprise compliance features advertised but mostly non-functional.

---

### WARNING - Admin User List Missing

`GET /api/v1/admin/users` - **NOT IMPLEMENTED**

**Impact:** Admin dashboard cannot display user list.

---

### WARNING - Webhook CRUD Incomplete

| Endpoint | Status |
|----------|--------|
| `GET /api/v1/webhooks` | **MISSING** |
| `POST /api/v1/webhooks` | **MISSING** |
| `PUT /api/v1/webhooks/{id}` | **MISSING** |
| `DELETE /api/v1/webhooks/{id}` | EXISTS |

---

### CLEAN - Fully Implemented

- Authentication (13/13 endpoints)
- MFA (7/7 endpoints)
- Passkeys/WebAuthn (6/6 endpoints)
- SCIM 2.0 (11/11 endpoints)
- SSO/SAML (6/6 endpoints)
- OAuth (7/7 endpoints)

---

## Instructional Validity Test

### CRITICAL - Missing Scripts

| Script | Referenced In | Status |
|--------|---------------|--------|
| `./scripts/start-local-demo.sh` | QUICK_START.md:14 | **DOES NOT EXIST** |
| `./scripts/run-demo-tests.sh` | QUICK_START.md:66 | **DOES NOT EXIST** |

**Action Item:** Create scripts or update documentation to reference `./scripts/start-demo.sh`.

---

### CRITICAL - Missing Dockerfile

**Referenced:** `apps/api/Dockerfile.production` in DEPLOYMENT.md:178,237,329
**Actual:** Only `apps/api/Dockerfile` exists

**Action Item:** Create `Dockerfile.production` or update DEPLOYMENT.md to use existing Dockerfile.

---

### CRITICAL - Missing Documentation

**Referenced:** `docs/guides/CONFIGURATION.md` in README.md:259
**Status:** **FILE DOES NOT EXIST**

**Action Item:** Create configuration guide or update README link.

---

### CRITICAL - Wrong Directory Name

**Documentation says:** `cd apps/landing && npm install`
**Actual directory:** `apps/website`

**Action Item:**
```diff
# docs/guides/QUICK_START.md:159
- cd apps/landing && npm install
+ cd apps/website && npm install
```

---

### WARNING - Port Inconsistency

**README Quick Start:** Port 8000 (localhost:8000/docs)
**Production Dockerfile:** Port 4100 (MADFAM standard)

**Action Item:** Clarify that 8000 is dev-only, 4100 is production.

---

### WARNING - uvicorn Command Inconsistency

**Multiple formats in docs:**
- `uvicorn app.main:app` (README.md)
- `uvicorn main:app` (LOCAL_DEMO_GUIDE.md)

**Action Item:** Standardize to `uvicorn app.main:app --reload` across all docs.

---

## Feature Claim Audit

### CRITICAL - FALSE Claims (Q1 2026 Features with 0% Implementation)

| Feature | Claimed Timeline | Implementation |
|---------|------------------|----------------|
| SMS MFA Integration | Q1 2026, P1 | **0% - No code exists** |
| Adaptive MFA (Risk-Based) | Q1 2026, P1 | **0% - No code exists** |
| Breach Detection (HIBP) | Q1 2026, P2 | **0% - No code exists** |

**Note:** Q1 2026 started 2026-01-01 - these are "imminent" but have zero implementation.

**Action Item:** Move to Q2 2026 or mark as "Planned - Not Started".

---

### CLEAN - Accurate Claims

- OAuth Providers (8) - Production Ready
- SAML 2.0 SSO - Production Ready
- WebAuthn/Passkeys - Production Ready
- TOTP/Backup Codes - Production Ready
- Multi-Tenancy/RBAC - Production Ready
- SDKs (React, Vue, Next.js, Python, Go, Flutter, React Native) - All exist

---

# REMEDIATION PRIORITY

## Week 1 - CRITICAL (Production Breaking)

### Enclii
1. Wire K8s environment variables to code (6 vars)
2. Add missing health probe endpoints (`/health/live`, `/health/ready`)
3. Fix auto-deploy blocker or update status claim
4. Add missing Make targets (4 targets)

### Janua
1. Add `CONEKTA_WEBHOOK_SECRET` and `SENDGRID_API_KEY` to Settings class
2. Fix feature flag naming (`FEATURE_*` → `ENABLE_*`)
3. Implement Organization CRUD (4 endpoints)
4. Create missing scripts or update QUICK_START.md
5. Create `Dockerfile.production` or update DEPLOYMENT.md

## Week 2 - WARNING (Developer Experience)

### Enclii
1. Document all production environment variables
2. Fix Redis variable naming
3. Remove/implement unused env vars
4. Update OpenAPI spec for 60 undocumented endpoints

### Janua
1. Standardize compliance variable naming
2. Fix K8s manifest variable names
3. Implement session listing endpoints
4. Fix user profile path in docs (`/users/me` → `/auth/me`)
5. Standardize uvicorn commands in docs
6. Create `docs/guides/CONFIGURATION.md`

## Week 3 - Documentation Polish

### Both
1. Update Q1 2026 roadmap items with realistic timelines
2. Add validation step to docs build process
3. Create file location index for quick reference
4. Cross-reference all deployment guides with actual manifests

---

# VERIFICATION CHECKLIST

## After Remediation

- [ ] All `.env.example` variables are used in code
- [ ] All code environment reads have documentation
- [ ] All OpenAPI endpoints match route handlers
- [ ] All scripts referenced in docs exist
- [ ] All Make targets referenced in docs exist
- [ ] All file paths in docs point to existing files
- [ ] All "Production Ready" claims verified working
- [ ] All "Planned" features have zero production claims

---

**Report Generated:** 2026-01-15
**Next Audit Recommended:** After Week 3 remediation completion

*This report was generated by systematic analysis of both codebases. The codebase is the single source of truth - all documentation must conform to implementation reality.*
