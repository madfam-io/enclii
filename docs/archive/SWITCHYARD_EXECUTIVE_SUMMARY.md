# SWITCHYARD PIVOT - EXECUTIVE SUMMARY
## From Generic PaaS → Compliance & Operations Engine

**Date**: 2025-11-19
**Current State**: 90% deployment tool, 12% compliance engine
**Target State**: 85% SOC 2 compliant in 8 weeks
**Market**: "Compliance Panic" - Series B scale-ups needing SOC 2

---

## 🎯 THE OPPORTUNITY

**Market Gap**: Competitors (Qovery, Porter, Flightcontrol) gate compliance features behind "Enterprise" pricing or "Contact Sales."

**Our Strategy**: **Ungated Compliance** - Give Series B companies SSO, audit logs, and RBAC in the base product.

**First-Mover Advantage**: No competitor has PR approval tracking or Vanta/Drata integration.

---

## 🔴 CRITICAL FINDINGS (Must Fix Week 1)

### 1. Authentication is Broken
**Location**: `apps/switchyard-api/cmd/api/main.go:62-66`
- OIDC initialization will panic at runtime
- Role constants (`RoleAdmin`, `RoleDeveloper`) are undefined
- No login endpoint exists
- **Impact**: Platform cannot authenticate users

### 2. No Compliance Database Schema
- Missing 8 tables: `users`, `teams`, `roles`, `permissions`, `project_access`, `audit_logs`, `sessions`, `approval_records`
- Current: 5 tables (projects, environments, services, releases, deployments)
- **Impact**: Cannot store audit data or manage access

### 3. RBAC is Binary, Not Granular
- Current: Single role per user (Admin OR Developer)
- Required: Environment-specific (Admin in Prod, Developer in Staging)
- **Impact**: Fails SOC 2 CC6.1 "Logical access controls"

### 4. Security Vulnerability: No Project Isolation
**Location**: `apps/switchyard-api/internal/auth/jwt.go:295-296`
```go
// TODO: In production, implement proper project-level authorization
// For now, we allow access if user has valid token
```
- **Impact**: Any authenticated user can access ANY project

---

## ✅ GOOD NEWS

### Zero "Taxes" to Kill
✅ No user count limits
✅ No seat-based pricing
✅ No gated features
✅ No "Contact Sales" triggers

**Conclusion**: Already ungated by design. Market this aggressively.

### Strong Technical Foundation
✅ Build pipeline functional (Git clone → Buildpacks → Deploy)
✅ CLI commands work end-to-end
✅ Kubernetes integration solid
✅ Monorepo structure clean

---

## 📊 GAP SUMMARY

### Critical Refactors (16-23 days)
| Gap | Effort | Impact |
|-----|--------|--------|
| Fix authentication system | L (5-7d) | Platform non-functional without this |
| Create compliance database schema | M (3-5d) | Blocks audit logging |
| Implement granular RBAC | L (5-7d) | Blocks SOC 2 compliance |
| Enforce project authorization | M (3-4d) | Critical security vulnerability |

### Feature Gaps (25-36 days)
| Feature | Effort | Competitive Advantage |
|---------|--------|----------------------|
| Immutable audit log system | M (4-5d) | SOC 2 requirement |
| GitHub PR approval tracking | M (4-5d) | **First-mover** |
| Vanta/Drata webhooks | S (2-3d) | **First-mover** |
| Zero-downtime secret rotation | M (3-4d) | SOC 2 best practice |
| SBOM generation | S (2d) | Supply chain security |
| Image signing (cosign) | S (2-3d) | Supply chain security |
| Subway map topology view | M (4-5d) | Visual differentiation |
| RBAC admin UI | M (4-5d) | Usability |

**Total**: 41-59 days (~8-12 weeks with 1 engineer, ~4-6 weeks with 2 engineers)

---

## 🗓️ RECOMMENDED ROADMAP

### Sprint 0: Emergency Fixes (Week 1)
**Goal**: Make auth functional
- Fix OIDC bug, define role constants
- Create login/logout endpoints
- Create users table
- **Deliverable**: Users can authenticate

### Sprint 1: Compliance Foundation (Weeks 2-3)
**Goal**: SOC 2 baseline (50% compliance)
- Create 7 remaining compliance tables
- Implement audit logging
- Granular RBAC + project authorization
- SBOM + image signing
- **Deliverable**: Platform is SOC 2 certifiable

### Sprint 2: Provenance Engine (Weeks 4-5)
**Goal**: Competitive differentiation
- GitHub PR approval tracking
- Vanta/Drata webhooks
- Zero-downtime secret rotation
- **Deliverable**: "Roundhouse" provenance complete

### Sprint 3: Switchyard Aesthetic (Weeks 6-7)
**Goal**: Visual identity
- Subway map topology view
- Railroad theme CSS
- RBAC admin UI
- **Deliverable**: Distinctive "Switchyard" brand

### Sprint 4: Launch (Week 8)
- Integration tests, docs, launch

---

## 🎯 COMPETITIVE POSITIONING

### Feature Matrix

| Feature | Enclii Switchyard | Qovery | Porter | Flightcontrol |
|---------|-------------------|--------|--------|---------------|
| **SSO (SAML/OIDC)** | ✅ All tiers | 💰 Enterprise | 💰 Enterprise | 💰 Contact Sales |
| **Granular RBAC** | ✅ Environment-level | 💰 Enterprise | ❌ Basic | 💰 Enterprise |
| **Audit Logging** | ✅ Immutable | 💰 Enterprise | ❌ Basic | 💰 Contact Sales |
| **PR Approval Tracking** | ✅ All tiers | ❌ None | ❌ None | ❌ None |
| **Compliance Webhooks** | ✅ Vanta/Drata | ❌ None | ❌ None | ❌ None |
| **Image Signing** | ✅ Cosign | ❌ None | ❌ None | ❌ None |

**Legend**: ✅ Included, ❌ Not available, 💰 Enterprise only

---

## 💰 GO-TO-MARKET MESSAGING

### Before (Generic)
> "Enclii is a deployment platform that makes it easy to ship your code."

**Problem**: Commodity positioning

### After (Switchyard)
> "Enclii Switchyard is the Compliance & Operations Engine for Series B scale-ups. We give you SSO, audit logs, and deployment provenance—ungated and included—so you can pass SOC 2 without begging for enterprise pricing."

**Why this wins**: Targets "Compliance Panic" moment

### Key Messages
1. **"No Enterprise Tax"** - All compliance features included
2. **"Provenance, Not Just Logs"** - Track who approved what, when, why
3. **"Built for Auditors"** - One-click export to Vanta/Drata
4. **"Operational Sovereignty"** - Control your infrastructure destiny

---

## 📈 SUCCESS METRICS

### Technical
- SOC 2 Readiness: **12% → 85%** (Target: 8 weeks)
- Audit Coverage: **0% → 100%** of API calls
- RBAC: **Binary → Environment-specific**
- Provenance: **0% → 100%** deployments linked to PRs

### Business
- **ICP**: Series B scale-ups (50-200 employees)
- **Win Rate**: Track vs Qovery/Porter on "ungated compliance"
- **Time-to-SOC-2**: Measure customer compliance speed
- **Pricing**: No per-seat tax vs competitors

---

## ⚠️ RISKS

| Risk | Impact | Mitigation |
|------|--------|------------|
| 8-week timeline too aggressive | Schedule slip | Ship incrementally (Sprint 0+1 first) |
| SOC 2 requirements change | Rework | Validate with compliance consultant upfront |
| GitHub rate limits | PR checking fails | Cache, batch, use GitHub App auth |
| Vanta/Drata APIs change | Integration breaks | Build generic webhook system |
| Topology view doesn't scale | UI unusable at 100+ services | Implement filtering, zoom, groups |

---

## 🚀 IMMEDIATE NEXT STEPS

### Today
1. ✅ Review gap report with team
2. [ ] Decision: Full 8-week roadmap or critical path only?
3. [ ] Assign engineer(s) to Sprint 0
4. [ ] Schedule weekly compliance review

### Week 1 (Sprint 0)
- [ ] Fix OIDC initialization bug
- [ ] Define role constants
- [ ] Create users table migration
- [ ] Add login/logout endpoints

---

## 📚 SUPPORTING DOCUMENTS

Full audit reports available:
1. **SWITCHYARD_GAP_REPORT.md** - Complete technical roadmap (140+ pages)
2. **AUTH_AUDIT_REPORT.md** - Authentication system analysis (652 lines)
3. **AUDIT_LOGGING_PROVENANCE.md** - Logging & compliance audit (509 lines)
4. **audit_findings.md** - UI/branding audit (switchyard-ui/)

---

## 🎯 THE ASK

**Decision Needed**: Proceed with Sprint 0 (Emergency Fixes) starting Monday?

**Resources**: 1-2 engineers for 8 weeks

**Expected Outcome**: Distinctive platform that wins "Compliance Panic" market by ungating features competitors gate.

---

**Bottom Line**: We're 8 weeks from a defensible competitive position in a blue ocean market. The foundation is solid, we just need to bolt on compliance and provenance tracking.

**Ready to build Switchyard? 🚂**
