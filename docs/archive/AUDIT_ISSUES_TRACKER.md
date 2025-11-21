# ENCLII AUDIT - ISSUES TRACKER
## Prioritized Action Items

**Audit Date:** November 19, 2025
**Total Issues:** 327
**Status:** NOT PRODUCTION READY

---

## 🔴 CRITICAL PRIORITY (35 issues) - FIX IMMEDIATELY

### Security (5 issues)

| ID | Issue | Location | Effort | Owner |
|----|-------|----------|--------|-------|
| SEC-001 | Hardcoded database credentials | `/apps/switchyard-api/internal/backup/postgres.go:381-396` | 2h | - |
| SEC-002 | Database SSL/TLS disabled | `/apps/switchyard-api/internal/config/config.go:59` | 1h | - |
| SEC-003 | CORS allows all origins | `/apps/switchyard-api/internal/middleware/security.go:428` | 1h | - |
| SEC-004 | Hardcoded OIDC secret | `/apps/switchyard-api/internal/config/config.go:64` | 1h | - |
| SEC-005 | Auth handler runtime error | `/apps/switchyard-api/internal/api/auth_handlers.go:202` | 2h | - |

### Infrastructure (7 issues)

| ID | Issue | Location | Effort | Owner |
|----|-------|----------|--------|-------|
| INFRA-001 | Secrets in Git repository | `/infra/k8s/base/secrets.yaml` | 8h | - |
| INFRA-002 | Database password hardcoded | `/infra/k8s/base/postgres.yaml` | 2h | - |
| INFRA-003 | PostgreSQL no resource limits | `/infra/k8s/base/postgres.yaml:22-45` | 2h | - |
| INFRA-004 | PostgreSQL no persistent storage | `/infra/k8s/base/postgres.yaml:39-42` | 4h | - |
| INFRA-005 | RBAC overprivileged | `/infra/k8s/base/rbac.yaml` | 3h | - |
| INFRA-006 | No default-deny NetworkPolicies | `/infra/k8s/base/network-policies.yaml` | 6h | - |
| INFRA-007 | No TLS/HTTPS on ingress | `/infra/k8s/base/ingress-nginx.yaml` | 6h | - |

### API (5 issues)

| ID | Issue | Location | Effort | Owner |
|----|-------|----------|--------|-------|
| API-001 | JWT keys not shared across replicas | `/apps/switchyard-api/internal/auth/jwt.go` | 8h | - |
| API-002 | Missing resource cleanup | `/apps/switchyard-api/cmd/api/main.go:70-90` | 2h | - |
| API-003 | No token revocation | `/apps/switchyard-api/internal/auth/jwt.go` | 12h | - |
| API-004 | Secret rotation not atomic | `/apps/switchyard-api/internal/rotation/controller.go` | 8h | - |
| API-005 | No webhook signature verification | `/apps/switchyard-api/internal/compliance/vanta.go` | 4h | - |

### UI (3 issues)

| ID | Issue | Location | Effort | Owner |
|----|-------|----------|--------|-------|
| UI-001 | Hardcoded auth tokens | `/apps/switchyard-ui/app/projects/page.tsx:41,58,87` | 8h | - |
| UI-002 | No authentication middleware | `/apps/switchyard-ui/app/layout.tsx` | 6h | - |
| UI-003 | No CSRF protection | All forms | 4h | - |

### Test Coverage (6 issues)

| ID | Issue | Location | Effort | Owner |
|----|-------|----------|--------|-------|
| TEST-001 | Compilation error: db package | `/apps/switchyard-api/internal/db/connection.go:98-106` | 30min | - |
| TEST-002 | Compilation error: k8s package | `/apps/switchyard-api/internal/k8s/client.go` | 1h | - |
| TEST-003 | Compilation error: backup package | `/apps/switchyard-api/internal/backup/postgres.go` | 1h | - |
| TEST-004 | Compilation error: builder package | `/apps/switchyard-api/internal/builder/git.go` | 1h | - |
| TEST-005 | Compilation error: cache package | `/apps/switchyard-api/internal/cache/redis.go` | 30min | - |
| TEST-006 | Compilation error: provenance package | `/apps/switchyard-api/internal/provenance/checker.go` | 30min | - |

### Code Quality (8 issues)

| ID | Issue | Location | Effort | Owner |
|----|-------|----------|--------|-------|
| QUAL-001 | Auth ProjectIDs not populated | `/apps/switchyard-api/internal/api/auth_handlers.go` | 6h | - |
| QUAL-002 | Session revocation not implemented | `/apps/switchyard-api/internal/api/auth_handlers.go` | 12h | - |
| QUAL-003 | Image rollback broken | `/apps/switchyard-api/internal/reconciler/service.go` | 8h | - |
| QUAL-004 | Log streaming dummy implementation | `/packages/cli/internal/cmd/logs.go:134` | 12h | - |
| QUAL-005 | Rotation audit logging incomplete | `/apps/switchyard-api/internal/rotation/controller.go` | 4h | - |
| QUAL-006 | Handler god object | `/apps/switchyard-api/internal/api/handlers.go` | 16h | - |
| QUAL-007 | RBAC enforcement incomplete | `/apps/switchyard-api/internal/api/handlers.go` | 16h | - |
| QUAL-008 | context.Background() misuse (42 instances) | Multiple files | 6h | - |

### CLI (1 issue)

| ID | Issue | Location | Effort | Owner |
|----|-------|----------|--------|-------|
| CLI-001 | API token exposed in error messages | `/packages/cli/internal/client/api.go` | 3h | - |

**CRITICAL SUBTOTAL: 35 issues, ~170 hours**

---

## 🟠 HIGH PRIORITY (65 issues) - FIX BEFORE PRODUCTION

### Security (8 issues)

| ID | Issue | Effort |
|----|-------|--------|
| SEC-006 | Missing session revocation | 12h |
| SEC-007 | Project-level authorization gaps | 16h |
| SEC-008 | No CSRF protection | 6h |
| SEC-009 | Vault token in environment variables | 4h |
| SEC-010 | GitHub token rotation missing | 8h |
| SEC-011 | Type casting issues in logout handler | 2h |
| SEC-012 | Secrets in logs | 8h |
| SEC-013 | JWT key management issues | 8h |

### Infrastructure (12 issues)

| ID | Issue | Effort |
|----|-------|--------|
| INFRA-008 | No high availability (single replica DB) | 8h |
| INFRA-009 | No Pod Disruption Budgets | 2h |
| INFRA-010 | No backups configured | 8h |
| INFRA-011 | Missing image pull secrets | 2h |
| INFRA-012 | No admission control (Kyverno/OPA) | 12h |
| INFRA-013 | No Pod Security Standards | 6h |
| INFRA-014 | Jaeger no security context | 2h |
| INFRA-015 | Redis no persistent storage | 3h |
| INFRA-016 | No resource quotas | 3h |
| INFRA-017 | No limit ranges | 2h |
| INFRA-018 | Missing Horizontal Pod Autoscaler | 4h |
| INFRA-019 | No monitoring/alerting configured | 10h |

### API (9 issues)

| ID | Issue | Effort |
|----|-------|--------|
| API-006 | Database connection pool not configured | 2h |
| API-007 | Silent error swallowing in audit logger | 3h |
| API-008 | No rate limiting on auth endpoints | 6h |
| API-009 | SQL injection in dynamic queries | 4h |
| API-010 | Incomplete RBAC checks | 12h |
| API-011 | N+1 query patterns | 8h |
| API-012 | Goroutine leak in cleanup | 3h |
| API-013 | Missing indexes on foreign keys | 2h |
| API-014 | Context timeout reuse issues | 4h |

### CLI (4 issues)

| ID | Issue | Effort |
|----|-------|--------|
| CLI-002 | No retry logic | 6h |
| CLI-003 | Command injection risk in git commands | 4h |
| CLI-004 | No config file reading | 8h |
| CLI-005 | No integration tests for deployment flow | 16h |

### UI (2 issues)

| ID | Issue | Effort |
|----|-------|--------|
| UI-004 | No input validation | 6h |
| UI-005 | All components marked 'use client' | 8h |

### Test Coverage (12 issues)

| ID | Issue | Effort |
|----|-------|--------|
| TEST-007 | No authentication tests | 10h |
| TEST-008 | No validation tests | 10h |
| TEST-009 | No database integration tests | 12h |
| TEST-010 | No Kubernetes client tests | 8h |
| TEST-011 | No API handlers tests (expanded) | 12h |
| TEST-012 | No builder service tests | 10h |
| TEST-013 | No cache service tests | 8h |
| TEST-014 | No CLI command tests | 8h |
| TEST-015 | No E2E deployment tests | 15h |
| TEST-016 | No security/compliance tests | 12h |
| TEST-017 | No audit logging tests | 8h |
| TEST-018 | No Vault integration tests | 5h |

### Code Quality (18 issues)

| ID | Issue | Effort |
|----|-------|--------|
| QUAL-009 | handlers.go monolith (1,082 lines) | 16h |
| QUAL-010 | repositories.go monolith (936 lines) | 12h |
| QUAL-011 | No service layer | 20h |
| QUAL-012 | Hardcoded localhost (7 instances) | 2h |
| QUAL-013 | No configuration validation | 4h |
| QUAL-014 | Low test coverage (26%) | 100h |
| QUAL-015 | Mixed logging frameworks | 6h |
| QUAL-016 | Tight coupling to database | 16h |
| QUAL-017 | Missing error context | 4h |
| QUAL-018 | Incomplete error messages | 6h |
| QUAL-019 | No graceful degradation | 8h |
| QUAL-020 | Poor separation of concerns | 12h |
| QUAL-021 | Unsafe type assertions | 4h |
| QUAL-022 | Inefficient database queries | 8h |
| QUAL-023 | Missing transaction boundaries | 6h |
| QUAL-024 | Inconsistent naming conventions | 4h |
| QUAL-025 | Code duplication (DatabaseManager) | 3h |
| QUAL-026 | Large functions (>50 lines, 23 instances) | 12h |

**HIGH SUBTOTAL: 65 issues, ~410 hours**

---

## 🟡 MEDIUM PRIORITY (123 issues) - FIX BEFORE GA

### Sampling of Medium Priority Issues

| ID | Category | Issue | Effort |
|----|----------|-------|--------|
| MED-001 | Security | YAML injection risks in CLI | 4h |
| MED-002 | Security | Missing rate limiting enforcement | 6h |
| MED-003 | Security | Container runs as root | 2h |
| MED-004 | Infrastructure | Image tags use :latest | 1h |
| MED-005 | Infrastructure | No image digest pinning | 2h |
| MED-006 | API | Cache fallback without metrics | 3h |
| MED-007 | API | Context leaks in background workers | 4h |
| MED-008 | API | Missing URL validation for Git repos | 2h |
| MED-009 | CLI | Inconsistent error field names | 2h |
| MED-010 | CLI | Hardcoded default config values | 3h |
| MED-011 | UI | No proper TypeScript strict mode | 4h |
| MED-012 | UI | Sequential API calls (should be parallel) | 3h |
| MED-013 | Code | Magic numbers (42 instances) | 6h |
| MED-014 | Code | TODO comments (37 instances) | 20h |
| MED-015 | Test | No frontend tests (0%) | 25h |

*... (108 more medium priority issues)*

**MEDIUM SUBTOTAL: 123 issues, ~350 hours**

---

## 🟢 LOW PRIORITY (104 issues) - POLISH & ENHANCEMENT

### Sampling of Low Priority Issues

| ID | Category | Issue | Effort |
|----|----------|-------|--------|
| LOW-001 | CLI | Bcrypt cost not configurable | 1h |
| LOW-002 | CLI | No cache metrics | 2h |
| LOW-003 | CLI | Emoji in help text (accessibility) | 1h |
| LOW-004 | UI | Missing loading spinners | 2h |
| LOW-005 | UI | No keyboard shortcuts | 4h |
| LOW-006 | Code | Inconsistent comment style | 2h |
| LOW-007 | Code | Missing package documentation | 8h |
| LOW-008 | Test | No benchmark tests | 6h |
| LOW-009 | Docs | Incomplete API documentation | 6h |
| LOW-010 | Docs | Missing architecture diagrams | 4h |

*... (94 more low priority issues)*

**LOW SUBTOTAL: 104 issues, ~180 hours**

---

## 📊 SUMMARY STATISTICS

| Priority | Count | Estimated Hours | % of Total |
|----------|-------|-----------------|------------|
| 🔴 CRITICAL | 35 | 170 | 11% |
| 🟠 HIGH | 65 | 410 | 20% |
| 🟡 MEDIUM | 123 | 350 | 38% |
| 🟢 LOW | 104 | 180 | 32% |
| **TOTAL** | **327** | **~1,110** | **100%** |

---

## 🎯 PHASED IMPLEMENTATION

### Phase 1: Critical Blockers (Weeks 1-2)
- Fix: SEC-001 through SEC-005 (Security)
- Fix: INFRA-001 through INFRA-007 (Infrastructure)
- Fix: API-001 through API-005 (API Critical)
- Fix: UI-001 through UI-003 (UI Authentication)
- Fix: TEST-001 through TEST-006 (Compilation)
- Fix: QUAL-001 through QUAL-008 (Code Critical)
- Fix: CLI-001 (CLI Critical)
- **Total: 35 issues, ~170 hours**

### Phase 2: High Priority (Weeks 3-5)
- Fix: All 65 HIGH priority issues
- Focus areas: Testing, RBAC, monitoring
- **Total: 65 issues, ~410 hours**

### Phase 3: Production Ready (Weeks 6-8)
- Fix: Top 50 MEDIUM priority issues
- Focus areas: Refactoring, performance, polish
- **Total: 50 issues, ~180 hours**

### Phase 4: GA Ready (Weeks 9-10)
- Fix: Remaining MEDIUM issues
- Fix: Critical LOW issues (documentation)
- **Total: 73 issues, ~200 hours**

---

## 📝 ISSUE TEMPLATE

Use this template when creating tickets:

```markdown
## [ID] Issue Title

**Priority:** 🔴 CRITICAL / 🟠 HIGH / 🟡 MEDIUM / 🟢 LOW
**Category:** Security / Infrastructure / API / CLI / UI / Test / Code Quality
**Effort:** Xh
**Phase:** 1 / 2 / 3 / 4

### Description
[Clear description of the issue]

### Location
- File: `/path/to/file.go`
- Lines: 123-456

### Current Behavior
[What happens now]

### Expected Behavior
[What should happen]

### Acceptance Criteria
- [ ] Criterion 1
- [ ] Criterion 2
- [ ] Tests added
- [ ] Documentation updated

### Implementation Notes
[Technical details, code snippets, references]

### Dependencies
- Blocks: [Other issue IDs]
- Depends on: [Other issue IDs]
```

---

## 🔄 PROGRESS TRACKING

### Week 1 (Target: 20 issues)
- [ ] SEC-001 - Hardcoded credentials
- [ ] SEC-002 - Database SSL
- [ ] SEC-003 - CORS config
- [ ] SEC-004 - OIDC secret
- [ ] SEC-005 - Auth handler error
- [ ] INFRA-001 - Secrets in Git
- [ ] INFRA-002 - DB password
- [ ] INFRA-003 - PostgreSQL limits
- [ ] INFRA-004 - PostgreSQL storage
- [ ] INFRA-005 - RBAC overpriv
- [ ] INFRA-006 - Network policies
- [ ] INFRA-007 - TLS ingress
- [ ] TEST-001 - DB compilation
- [ ] TEST-002 - K8s compilation
- [ ] TEST-003 - Backup compilation
- [ ] TEST-004 - Builder compilation
- [ ] TEST-005 - Cache compilation
- [ ] TEST-006 - Provenance compilation
- [ ] API-002 - Resource cleanup
- [ ] UI-003 - CSRF protection

### Week 2 (Target: 15 issues)
- [ ] API-001 - JWT keys
- [ ] API-003 - Token revocation
- [ ] API-004 - Secret rotation
- [ ] API-005 - Webhook signature
- [ ] UI-001 - Hardcoded tokens
- [ ] UI-002 - Auth middleware
- [ ] CLI-001 - Token exposure
- [ ] QUAL-001 - Auth ProjectIDs
- [ ] QUAL-002 - Session revocation
- [ ] QUAL-003 - Image rollback
- [ ] QUAL-004 - Log streaming
- [ ] QUAL-005 - Rotation audit
- [ ] QUAL-007 - RBAC enforcement
- [ ] QUAL-008 - context.Background
- [ ] Test infrastructure setup

---

**Last Updated:** November 19, 2025
**Next Review:** Weekly on Mondays
**Owner:** Engineering Team Lead
