# Enclii Logging and Provenance Tracking Audit Report

## Executive Summary

Enclii has established a **structured logging foundation** but lacks critical compliance infrastructure for SOC 2 Type II audit requirements. Key findings:

- ✓ Structured logging implemented with OpenTelemetry tracing
- ✗ No immutable audit log database tables
- ✗ No deployment provenance metadata (SBOM, signatures, approvals)
- ✗ No compliance tool integrations (Vanta, Drata)
- ✗ Missing actor identity tracking in deployments

---

## 1. AUDIT LOGGING ARCHITECTURE

### Current Implementation

**Location:** `/home/user/enclii/apps/switchyard-api/internal/logging/structured.go`

**Type:** Structured JSON logging using Logrus + OpenTelemetry

```go
// Structured logger with fields:
- timestamp (RFC3339 nano precision)
- level (debug/info/warn/error/fatal)
- message
- service (enclii-switchyard)
- version
- environment
- request_id (UUID, auto-generated if missing)
- user_id (from JWT context)
- trace_id (from OpenTelemetry span)
- span_id (from OpenTelemetry span)
- caller (file:line)
- function (function name)
+ custom fields per log call
```

**Output Destinations:**
- stdout (default for development)
- stderr (configurable)
- File path (configurable)
- JSON format (timestamps, field mapping configured)

**Middleware Coverage:**
- `RequestLoggingMiddleware()` - logs HTTP requests (method, path, status, latency, user_agent, client_ip)
- `RequestIDMiddleware()` - generates/propagates X-Request-ID header
- `TracingMiddleware()` - creates OpenTelemetry spans with semantic attributes
- `SecurityMiddleware` - logs rate limit breaches, IP blocks, suspicious user agents

### Critical Gaps

**1. NO DATABASE AUDIT TRAIL**
- All logs stream to stdout/files only
- No immutable audit log table exists in database schema
- Cannot query audit history for compliance reports
- Logs can be modified/deleted (not tamper-evident)

**Database Schema Analysis:**
```sql
-- Current tables in 001_initial_schema.up.sql:
- projects
- environments
- services
- releases
- deployments
-- MISSING: audit_logs, audit_events, or change_history table
```

**2. NO ACTOR TRACKING IN API HANDLERS**
```go
// Example from DeployService handler (line 377-452):
deployment := &types.Deployment{
    ID:          uuid.New().String(),
    ServiceID:   serviceID.String(),
    ReleaseID:   req.ReleaseID,
    Environment: req.Environment,
    Replicas:    req.Replicas,
    Status:      types.DeploymentStatusPending,
    CreatedAt:   time.Now(),
    UpdatedAt:   time.Now(),
    // MISSING: ActorID, ApprovedBy, ApprovalTime, ChangeReason
}
```

**3. UNSTRUCTURED OPERATION TRACKING**
- Logs are the only source of "who did what"
- No correlation between deployment records and approval/authorization
- Build process logs (line 307-356) don't capture triggering user
- Rollback handler (line 580-656) doesn't record who requested rollback or when it was approved

---

## 2. DEPLOYMENT PROVENANCE TRACKING

### Current Status

**Implemented:**
- ✓ Git SHA captured in Release records (`git_sha VARCHAR(255)`)
- ✓ Release version with timestamp (`version VARCHAR(255)`)
- ✓ Image URI stored (`image_uri VARCHAR(500)`)
- ✓ Release status tracked (building → ready → failed)
- ✓ Build logs captured in handler (line 348-351) but not persisted

**Not Implemented:**
- ✗ SBOM (Software Bill of Materials) generation
- ✗ Image signature verification (cosign)
- ✗ Build provenance object
- ✗ Git commit details (author, message, branch)
- ✗ GitHub PR/review approval status

### Code Evidence

**Release Table Schema (Incomplete):**
```sql
CREATE TABLE releases (
    id UUID PRIMARY KEY,
    service_id UUID NOT NULL,
    version VARCHAR(255) NOT NULL,
    image_uri VARCHAR(500) NOT NULL,
    git_sha VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'building',
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

-- MISSING fields per SOFTWARE_SPEC.md line 149:
-- sbomURI, signature, buildId
```

**Build Process (apps/switchyard-api/internal/builder/service.go):**
```go
type CompleteBuildResult struct {
    ImageURI  string        // Only this persists to Release
    GitSHA    string
    Success   bool
    Error     error
    Logs      []string      // Captured in handler log() but not saved to DB
    Duration  time.Duration
    ClonePath string
}
```

**Deployment Record (Types Definition):**
```go
type Deployment struct {
    ID            uuid.UUID
    ReleaseID     uuid.UUID
    EnvironmentID uuid.UUID
    Replicas      int
    Status        DeploymentStatus
    Health        HealthStatus
    CreatedAt     time.Time
    UpdatedAt     time.Time
    // MISSING: ApprovedBy, ApprovalTime, ApprovalSource (CI|Manual|Auto)
    // MISSING: DeployedBy, DeploymentStrategy (canary|blue_green|rolling)
}
```

### Deployment Trigger Flow (No Approval Capture)

**From deploy.go (CLI):**
```go
func deployService(cfg *Config, environment string, wait bool) error {
    gitSHA, err := getCurrentGitSHA()           // Gets current commit
    // NO: Check GitHub PR approval status
    // NO: Verify manual approval for prod
    // NO: Log approver identity
    
    release, err := apiClient.BuildService(ctx, service.ID, gitSHA)  // Triggers build
    deployment, err := apiClient.DeployService(ctx, service.ID, deployReq)  // Creates deployment
    // NO: Record who triggered this deployment
}
```

**From handlers.go (API):**
```go
func (h *Handler) DeployService(c *gin.Context) {
    // Can extract user from JWT claims:
    // c.Get("user_id"), c.Get("user_email"), c.Get("user_role")
    
    // BUT: Not stored in deployment record
    deployment := &types.Deployment{
        // Missing: ActorID, ActorEmail, RequestedAt
    }
    
    // Can access JWT claims:
    // claims, _ := auth.GetClaimsFromContext(c)
    // But doesn't use them for audit trail
}
```

---

## 3. COMPLIANCE INTEGRATION STATUS

### Missing: Vanta, Drata, or Similar Webhook

**Search Results:**
```bash
$ grep -r "vanta\|drata\|webhook\|compliance" /home/user/enclii --include="*.go" --include="*.yaml"
# No matches for vanta or drata
# No webhook implementations for audit events
```

### Missing: Deployment Receipt/Certificate

**Required for:** SOC 2 Type II evidence of approval chain

**Current:** No deployment approval records, no receipts/certificates generated

### Missing: Compliance Event Emitter

**Required for:** Real-time audit log streaming to compliance platforms

**Current:** Logs only go to stdout/files, no event bus or webhook publisher

---

## 4. SECURITY AND IDENTITY GAPS

### Actor Identity Not Captured

**JWT Claims Available (auth/jwt.go):**
```go
type Claims struct {
    UserID      uuid.UUID
    Email       string
    Role        string
    ProjectIDs  []string
    TokenType   string
    RegisteredClaims jwt.RegisteredClaims
}
```

**BUT:**
- User identity NOT stored with deployment records
- User identity NOT stored with build records
- User identity NOT stored with release records
- Only available in request logs (lost if logs are rotated)

### Role-Based Authorization Gap

**Code References Undefined Constants:**
```go
// handlers.go line 76, 81, 86, 88, 96:
v1.POST("/projects", h.auth.RequireRole(types.RoleAdmin), ...)
v1.POST("/services/:id/deploy", h.auth.RequireRole(types.RoleDeveloper), ...)

// But types.RoleAdmin, types.RoleDeveloper NOT DEFINED in types.go
// Only 134 lines in types.go, ends with EnvVar struct
```

**Impact:** Auth middleware functions exist but reference undefined constants (compilation error expected)

---

## 5. LOGGING MIDDLEWARE ANALYSIS

### HTTP Request Logging

**Captured:**
- client_ip
- method
- path
- status_code
- latency
- user_agent
- request_size
- response_size
- error (if any)

**Missing:**
- user_id / actor_id
- request body summary (for audit context)
- authorization scope

### Security Event Logging

**Structure Defined (middleware/security.go line 437-446):**
```go
type SecurityEvent struct {
    Timestamp   time.Time
    EventType   string
    ClientIP    string
    UserAgent   string
    Path        string
    Method      string
    StatusCode  int
    Message     string
}
```

**Logged Events:**
- Rate limit exceeded (line 86-90)
- IP blocked (line 192, 210)
- Suspicious user agent (line 355-359)

**NOT Logged:**
- Authorization failures (only at WARN level with timestamp)
- Deployment approvals/rejections
- Secrets rotation events
- Policy violations

---

## 6. OPENTELEMETRY TRACING

### Implemented

**Jaeger Integration:**
```go
// Configured in LogConfig:
TracingEnabled: true
JaegerEndpoint: "http://localhost:14268/api/traces"
TracingSampler: 0.1  // 10% sampling

// Spans created with semantic attributes:
- HTTPMethodKey
- HTTPURLKey
- HTTPUserAgentKey
- HTTPClientIPKey
- HTTPStatusCodeKey
- HTTPResponseSizeKey
```

**Capabilities:**
- Request tracing across services
- Span context propagation (trace_id, span_id in logs)
- Error status attribution to spans
- But: No business event spans (deployment, release, etc.)

### Gap

- Tracing useful for performance debugging
- NOT useful for compliance audit trails (10% sampling, not 100%)
- No custom spans for business-critical operations (deployment, approval)

---

## 7. DATABASE SCHEMA vs COMPLIANCE REQUIREMENTS

### SOFTWARE_SPEC.md Requirements (Line 155)

```
* **AuditEvent** {id, actor, action, entityRef, timestamp, payload}
```

### Current Implementation

**NO audit event table exists**

### Comparison Matrix

| Field | Database? | Request Logs? | Note |
|-------|-----------|---------------|------|
| id (audit_id) | NO | NO | Would need UUID |
| actor (user_id) | NO | YES (in JWT) | Available but not persisted |
| action (deploy, rollback, etc) | NO | YES | Only in log messages |
| entityRef (service_id, release_id) | YES | YES | Stored separately in deployment table |
| timestamp | NO | YES (RFC3339) | Created_at on records, not audit |
| payload (approvals, changes) | NO | NO | Not captured anywhere |

---

## 8. IDENTIFIED GAPS FOR SOC 2 AUDIT

### Critical (Audit Committee Blockers)

1. **No immutable audit log table**
   - Requirement: SOC 2 CC7.2 "monitor system activity"
   - Current: Logs to files, can be deleted
   - Impact: Cannot prove change history or access control

2. **No deployment approval tracking**
   - Requirement: SOC 2 CC6.1 "authorize and approve information processing"
   - Current: Deployments created without approval record
   - Impact: Cannot show who approved production deployments

3. **No policy/compliance event webhook**
   - Requirement: SOC 2 SI1.1 "boundary protection and monitoring"
   - Current: No real-time event stream to compliance tools
   - Impact: Manual log analysis required for evidence gathering

### High Priority (Risk Management)

4. **No SBOM or image signing integration**
   - Requirement: SOC 2 CC7.4 "supply chain security"
   - Current: Not implemented (noted in docs as TODO)
   - Impact: Cannot verify image provenance or detect supply chain compromise

5. **No GitHub PR approval status storage**
   - Requirement: SOC 2 CC6.2 "change management"
   - Current: Only git SHA stored, not PR/review status
   - Impact: Cannot prove PR review requirements were met for production changes

6. **Role constants undefined**
   - Requirement: SOC 2 CC6.1 "least privilege"
   - Current: handlers.go references undefined types.RoleAdmin
   - Impact: RBAC enforcement may not work correctly

### Medium Priority (Operational)

7. **Build logs not persisted to database**
   - Current: Captured in handler log() but only sent to stdout
   - Impact: Build failure investigations limited to available log files

8. **Canary deployment progress not tracked**
   - Current: No deployment strategy or approval checkpoint recorded
   - Impact: Cannot demonstrate automated rollback compliance

---

## 9. RECOMMENDATIONS

### Immediate (Sprint 1)

- [ ] Create `audit_events` table with (id, actor_id, action, entity_type, entity_id, timestamp, details_json, source)
- [ ] Create `audit_log_entries` table with (event_id, timestamp, level, message, context_json) for immutable audit storage
- [ ] Update `deployments` table to add columns: `created_by`, `approved_by`, `approval_timestamp`, `deployment_strategy`, `approval_notes`
- [ ] Update `releases` table to add columns: `sbom_uri`, `signature_hash`, `build_initiator`
- [ ] Implement `AuditLog` service that writes all critical operations (deploy, rollback, release creation) to audit tables

### Short Term (Sprint 2)

- [ ] Add SBOM generation to build pipeline (syft or cyclonedx)
- [ ] Implement image signature verification (cosign integration)
- [ ] Add GitHub PR status lookup to deployment flow
- [ ] Create deployment approval workflow for production environments
- [ ] Implement webhook publisher for compliance event streaming (Vanta, Drata format)

### Configuration

- [ ] Define role constants in types.go (RoleAdmin, RoleDeveloper, RoleOwner, RoleReadOnly)
- [ ] Add audit log persistence configuration to LogConfig
- [ ] Create ComplianceEventConfig for webhook destinations

---

## 10. FILES ANALYZED

**Core Logging:**
- `/home/user/enclii/apps/switchyard-api/internal/logging/structured.go` (454 lines)
- `/home/user/enclii/apps/switchyard-api/internal/middleware/security.go` (462 lines)

**API & Deployment:**
- `/home/user/enclii/apps/switchyard-api/internal/api/handlers.go` (753 lines)
- `/home/user/enclii/packages/cli/internal/cmd/deploy.go` (244 lines)

**Builders & Reconciliation:**
- `/home/user/enclii/apps/switchyard-api/internal/builder/service.go` (144 lines)
- `/home/user/enclii/apps/switchyard-api/internal/builder/git.go` (154 lines)
- `/home/user/enclii/apps/switchyard-api/internal/reconciler/service.go` (439 lines)

**Authentication & Types:**
- `/home/user/enclii/apps/switchyard-api/internal/auth/jwt.go` (366 lines)
- `/home/user/enclii/packages/sdk-go/pkg/types/types.go` (134 lines)

**Database:**
- `/home/user/enclii/apps/switchyard-api/internal/db/repositories.go` (363 lines)
- `/home/user/enclii/apps/switchyard-api/internal/db/migrations/001_initial_schema.up.sql` (80 lines)

**Documentation:**
- `/home/user/enclii/SOFTWARE_SPEC.md` (lines 95-157)
- `/home/user/enclii/CLAUDE.md` (deployment requirements)
- `/home/user/enclii/README.md` (deployment workflow)

---

## Appendix: Code Excerpts

### Missing Audit Event Entity (Should be in types.go)

```go
// Proposed addition to types.go:

type AuditAction string

const (
    ActionDeploymentCreated AuditAction = "deployment.created"
    ActionDeploymentApproved AuditAction = "deployment.approved"
    ActionDeploymentRolledBack AuditAction = "deployment.rolled_back"
    ActionReleaseCreated AuditAction = "release.created"
    ActionSecretRotated AuditAction = "secret.rotated"
    ActionPolicyViolation AuditAction = "policy.violation"
)

type AuditEvent struct {
    ID          uuid.UUID        `json:"id" db:"id"`
    ActorID     uuid.UUID        `json:"actor_id" db:"actor_id"`
    ActorEmail  string           `json:"actor_email" db:"actor_email"`
    Action      AuditAction      `json:"action" db:"action"`
    EntityType  string           `json:"entity_type" db:"entity_type"`        // deployment, release, secret, etc
    EntityID    uuid.UUID        `json:"entity_id" db:"entity_id"`
    Details     json.RawMessage  `json:"details" db:"details"`               // approval_id, strategy, etc
    Timestamp   time.Time        `json:"timestamp" db:"timestamp"`
    Source      string           `json:"source" db:"source"`                 // api, cli, webhook
    IPAddress   string           `json:"ip_address" db:"ip_address"`
    UserAgent   string           `json:"user_agent" db:"user_agent"`
    CreatedAt   time.Time        `json:"created_at" db:"created_at"`
}
```

---

**Report Generated:** 2025-11-19
**Audit Scope:** Logging and Provenance Tracking
**Status:** GAP ANALYSIS - Ready for Implementation Planning
