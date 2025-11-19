# Secret Management Audit - Quick Summary

## Current Status: CRITICAL GAPS - 15% Production Ready

### Key Findings

#### 1. Secret Storage & Injection (0% Complete)
- **Current**: Plaintext environment variables passed to pods
- **Missing**: 
  - No Lockbox/Vault integration
  - No secrets table in database
  - No encryption at rest
  - No secret versioning

**Code Locations**:
- API: `/home/user/enclii/apps/switchyard-api/internal/api/handlers.go:388`
- Injection: `/home/user/enclii/apps/switchyard-api/internal/reconciler/service.go:167`
- Database: `/home/user/enclii/apps/switchyard-api/internal/db/migrations/001_initial_schema.up.sql`

#### 2. Rotation Capabilities (0% Complete)
- **Current**: Manual rotation only - requires pod restart
- **Missing**:
  - No rotation handlers
  - No zero-downtime rolling restart coordination
  - No health monitoring during rotation
  - No automatic rollback on failure

**Would Need**:
- New: `apps/switchyard-api/internal/reconciler/rotation.go`
- New: `apps/switchyard-api/internal/lockbox/` package
- Update: `apps/switchyard-api/internal/reconciler/controller.go`

**Test Case**: TC-03 in SOFTWARE_SPEC.md (not implemented)

#### 3. Access Control (0% Complete)
- **Current**: API-level RBAC only
- **Missing**:
  - No secret-level RBAC
  - No audit logging for secret access
  - No service account isolation
  - No scope-based access (project/env/service)

#### 4. Type Definition Mismatches (CRITICAL BUG)
- **File**: `packages/sdk-go/pkg/types/types.go`
- **Issue**: Release struct missing fields used in code:
  ```go
  // Struct doesn't have these but code uses them:
  Release.Environment      // Line in reconciler/service.go:167
  Release.BuildID         // Line in reconciler/service.go:190
  Release.ImageURL        // Should be ImageURI (line 220)
  ```
- **Risk**: Runtime panics possible

---

## Work Required by Phase

### Phase 1: Foundation (3-4 weeks) - CRITICAL
1. Fix type definitions
2. Create secrets database tables
3. Implement SecretRepository
4. Create Lockbox interface

**Files to Create**:
- `apps/switchyard-api/internal/db/migrations/002_add_secrets.up.sql` (NEW)
- `apps/switchyard-api/internal/lockbox/client.go` (NEW)
- `apps/switchyard-api/internal/db/secret_repository.go` (NEW)

**Files to Update**:
- `packages/sdk-go/pkg/types/types.go` (add Secret, SecretScope types)
- `apps/switchyard-api/internal/db/repositories.go` (add SecretRepository)

### Phase 2: Implementation (2-3 weeks)
1. Secret CRUD API endpoints
2. CLI secrets command
3. Audit logging

**Files to Create**:
- `packages/cli/internal/cmd/secrets.go` (NEW)

**Files to Update**:
- `apps/switchyard-api/internal/api/handlers.go` (add secret endpoints)
- `packages/cli/internal/cmd/root.go` (register secrets command)

### Phase 3: Zero-Downtime Rotation (2-3 weeks)
1. Rotation orchestrator
2. Health monitoring during rotation
3. Auto-rollback capability

**Files to Create**:
- `apps/switchyard-api/internal/reconciler/rotation.go` (NEW)

**Files to Update**:
- `apps/switchyard-api/internal/reconciler/controller.go` (add rotation handler)
- `apps/switchyard-api/internal/k8s/client.go` (add rolling restart methods)

### Phase 4: Compliance (1-2 weeks)
1. Comprehensive audit logging
2. Secret-level RBAC
3. Documentation & runbooks

---

## Critical Issues Blocking Production

| Issue | Impact | Effort |
|-------|--------|--------|
| Plaintext secrets in pods | Security breach risk | Phase 1 |
| No audit trail | Compliance violation | Phase 1 |
| Type mismatches | Runtime failures | Phase 1 |
| Manual rotation | Downtime risk | Phase 3 |
| No secret scoping | Lateral movement risk | Phase 1-2 |

---

## Testing Gaps

Currently Missing:
- Unit tests for secret operations
- Integration tests for rotation
- E2E test TC-03 (secret rotation with zero downtime)
- No secret repository tests
- No Lockbox mock/stub tests

**Files to Create**:
- `apps/switchyard-api/internal/db/secret_repository_test.go`
- `apps/switchyard-api/internal/lockbox/client_test.go`
- `apps/switchyard-api/internal/reconciler/rotation_test.go`
- `packages/cli/internal/cmd/secrets_test.go`

---

## Quick Reference: Absolute File Paths

### Current Implementation (Incomplete)
- `/home/user/enclii/apps/switchyard-api/internal/api/handlers.go` - Secret handling in API
- `/home/user/enclii/apps/switchyard-api/internal/reconciler/service.go` - Pod injection
- `/home/user/enclii/apps/switchyard-api/internal/config/config.go` - Config (no Lockbox config)
- `/home/user/enclii/apps/switchyard-api/internal/db/migrations/001_initial_schema.up.sql` - Schema (no secrets table)
- `/home/user/enclii/packages/sdk-go/pkg/types/types.go` - Types (incomplete)

### CLI (Not Yet Implemented)
- `/home/user/enclii/packages/cli/internal/cmd/root.go` - Missing secrets command
- `/home/user/enclii/packages/cli/internal/cmd/deploy.go` - Doesn't handle secret references

### Infrastructure (Development Only)
- `/home/user/enclii/infra/k8s/base/secrets.yaml` - Dev secrets (hardcoded)

---

## Recommended Next Steps

1. **This Week**:
   - Review this audit with team
   - Identify priority (compliance vs features)
   - Plan Phase 1 sprint

2. **Next Sprint**:
   - Fix type definitions
   - Create secrets database schema
   - Implement SecretRepository
   - Create Lockbox interface with stub

3. **Following Sprint**:
   - Implement API endpoints
   - Add CLI commands
   - Basic audit logging
   - Integration tests

4. **Thereafter**:
   - Rotation orchestrator
   - Zero-downtime rotation
   - Production Vault/1Password integration
   - Compliance features (SIEM export, etc.)

---

## References

- **Full Audit**: `/home/user/enclii/docs/SECRET_MANAGEMENT_AUDIT.md`
- **Architecture Spec**: `/home/user/enclii/SOFTWARE_SPEC.md` (sections 3, 10, 15)
- **Current API Code**: `/home/user/enclii/apps/switchyard-api/`
- **CLI Code**: `/home/user/enclii/packages/cli/`
- **Database**: `/home/user/enclii/apps/switchyard-api/internal/db/`

---

## Questions for Team Review

1. Should we implement stub Lockbox first or integrate real Vault immediately?
2. What's the compliance deadline that drives this work?
3. Should we prioritize zero-downtime rotation or other features?
4. Do we need SIEM integration for audit logs in Phase 1?

