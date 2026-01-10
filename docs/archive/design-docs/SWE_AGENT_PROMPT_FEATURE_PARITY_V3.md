# Enclii Feature Parity SWE Agent Prompt v3.0

**Generated:** December 11, 2025
**Target:** Complete Vercel/Railway feature parity for production internal + external deployments
**Current State:** 85% → Target: 100%

---

## 🚨 CRITICAL CONTEXT

You are implementing the remaining features for **Enclii**, a Railway-style PaaS running on Hetzner Cloud with Cloudflare networking. The codebase is a monorepo with:

- **Control Plane API:** Go (Gin) at `apps/switchyard-api/`
- **Web UI:** Next.js at `apps/switchyard-ui/`
- **CLI:** Go at `packages/cli/`
- **Infrastructure:** K3s on Hetzner, Cloudflare Tunnel at `infra/`

### Current Production Environment
- **API:** https://api.enclii.dev (running but DB not initialized)
- **UI:** https://app.enclii.dev (running)
- **Auth:** Janua SSO at https://auth.madfam.io (OIDC/OAuth 2.0)
- **Registry:** ghcr.io/madfam-org

---

## PHASE 0: CRITICAL BLOCKERS (Fix First - Estimated: 1-2 Days)

These must be resolved before ANY other work proceeds.

### 0.1 Database Migrations Not Running
**Severity:** 🔴 P0 CRITICAL - Platform is non-functional without this

**Current State:**
- Database `enclii` exists but has **ZERO TABLES**
- 14 migration files exist at `apps/switchyard-api/internal/db/migrations/`
- API starts but all DB-dependent features silently fail

**Problem Location:**
- `apps/switchyard-api/cmd/server/main.go` - No migration runner on startup
- No init container or migration job in Kubernetes

**Required Implementation:**
```go
// Option A: Add auto-migrate on startup (recommended for single-instance)
// In main.go or db/init.go:
func RunMigrations(db *sql.DB, migrationsPath string) error {
    driver, err := postgres.WithInstance(db, &postgres.Config{})
    m, err := migrate.NewWithDatabaseInstance(
        "file://"+migrationsPath,
        "postgres", driver)
    return m.Up()
}

// Option B: Add migrate subcommand to binary
// enclii-api migrate up
```

**Kubernetes Changes:**
```yaml
# infra/k8s/production/switchyard-api.yaml
spec:
  template:
    spec:
      initContainers:
      - name: migrate
        image: ghcr.io/madfam-org/enclii-api:latest
        command: ["/app/switchyard-api", "migrate", "up"]
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: enclii-secrets
              key: database-url
```

**Verification:**
```sql
-- After fix, this should return 10+ tables:
SELECT table_name FROM information_schema.tables WHERE table_schema = 'public';
-- Must include: projects, environments, services, releases, deployments, etc.
```

**Files to Modify:**
- `apps/switchyard-api/cmd/server/main.go`
- `apps/switchyard-api/internal/db/db.go` (add RunMigrations function)
- `infra/k8s/production/switchyard-api.yaml` (add init container)

---

### 0.2 JWT Nil Pointer Panic
**Severity:** 🔴 P0 CRITICAL - Auth crashes on token validation

**Error Log:**
```
panic recovered: runtime error: invalid memory address or nil pointer dereference
/app/apps/switchyard-api/internal/auth/jwt.go:301
```

**Problem Location:**
- `apps/switchyard-api/internal/auth/jwt.go:301` - Nil pointer dereference in key lookup
- `apps/switchyard-api/internal/auth/jwt.go:261` - ValidateAccessToken function

**Root Cause Analysis:**
The JWKS keyset is likely nil when validating external (Janua) tokens. The code doesn't handle the case where:
1. JWKS fetch fails silently
2. Key ID not found in keyset
3. Keyset parsing returns nil

**Required Fix:**
```go
// jwt.go - Add nil checks
func (v *JWTValidator) getKey(token *jwt.Token) (interface{}, error) {
    if v.keySet == nil {
        return nil, fmt.Errorf("JWKS keyset not initialized")
    }

    kid, ok := token.Header["kid"].(string)
    if !ok {
        return nil, fmt.Errorf("missing kid header")
    }

    key, found := v.keySet.LookupKeyID(kid)
    if !found {
        // Try refreshing JWKS
        if err := v.refreshJWKS(); err != nil {
            return nil, fmt.Errorf("key %s not found and refresh failed: %w", kid, err)
        }
        key, found = v.keySet.LookupKeyID(kid)
        if !found {
            return nil, fmt.Errorf("key %s not found after refresh", kid)
        }
    }

    var rawKey interface{}
    if err := key.Raw(&rawKey); err != nil {
        return nil, fmt.Errorf("failed to get raw key: %w", err)
    }
    return rawKey, nil
}
```

**Files to Modify:**
- `apps/switchyard-api/internal/auth/jwt.go`

---

### 0.3 GitHub Webhook Not Configured
**Severity:** 🔴 P0 CRITICAL - Auto-deploy cannot work

**Current State:**
- Webhook endpoint exists: `POST /v1/webhooks/github`
- `ENCLII_GITHUB_WEBHOOK_SECRET` is configured in pod
- But NO webhook exists on `madfam-org/enclii` GitHub repo

**Required Action:**
```bash
# Create GitHub webhook via gh CLI or API
gh api repos/madfam-org/enclii/hooks -X POST \
  -f name=web \
  -f active=true \
  -f events[]="push" \
  -f events[]="pull_request" \
  -f config[url]="https://api.enclii.dev/v1/webhooks/github" \
  -f config[secret]="0a619aa7b0bf6b1bf75e252dacfc02a2afac33e4ccbe19a9ff0077bdc9d33508" \
  -f config[content_type]="json"
```

**Note:** Secret value from: `kubectl exec -n enclii deploy/switchyard-api -- env | grep GITHUB_WEBHOOK`

---

### 0.4 Register Dogfooding Services in Database
**Severity:** 🔴 P0 CRITICAL - Services must exist for webhooks to find them

**After migrations run, insert:**
```sql
-- Create Enclii project
INSERT INTO projects (id, name, slug) VALUES
  ('550e8400-e29b-41d4-a716-446655440000', 'Enclii', 'enclii');

-- Create prod environment
INSERT INTO environments (id, project_id, name, kube_namespace) VALUES
  ('550e8400-e29b-41d4-a716-446655440001', '550e8400-e29b-41d4-a716-446655440000', 'prod', 'enclii');

-- Register services with git_repo for webhook matching
INSERT INTO services (id, project_id, name, git_repo, auto_deploy, auto_deploy_env, build_config) VALUES
  ('550e8400-e29b-41d4-a716-446655440010', '550e8400-e29b-41d4-a716-446655440000',
   'switchyard-api', 'https://github.com/madfam-org/enclii', true, 'prod',
   '{"type":"dockerfile","dockerfile":"apps/switchyard-api/Dockerfile","context":"."}'),

  ('550e8400-e29b-41d4-a716-446655440011', '550e8400-e29b-41d4-a716-446655440000',
   'switchyard-ui', 'https://github.com/madfam-org/enclii', true, 'prod',
   '{"type":"dockerfile","dockerfile":"apps/switchyard-ui/Dockerfile","context":"."}'),

  ('550e8400-e29b-41d4-a716-446655440012', '550e8400-e29b-41d4-a716-446655440000',
   'docs-site', 'https://github.com/madfam-org/enclii', true, 'prod',
   '{"type":"dockerfile","dockerfile":"apps/docs-site/Dockerfile","context":"."}');
```

---

## PHASE 1: CORE FUNCTIONALITY (Estimated: 2-3 Weeks)

After Phase 0, these features make the platform minimally viable.

### 1.1 Complete Service CRUD API
**Priority:** P1 | **Effort:** 3-5 days

**Current State:**
- `service_handlers.go` exists but incomplete
- Missing: Create, Update, Delete operations with proper validation

**Required Endpoints:**
```
POST   /v1/projects/:project/services     - Create service
GET    /v1/projects/:project/services     - List services
GET    /v1/projects/:project/services/:id - Get service
PUT    /v1/projects/:project/services/:id - Update service
DELETE /v1/projects/:project/services/:id - Delete service
POST   /v1/projects/:project/services/:id/deploy - Trigger manual deploy
```

**Implementation Requirements:**
1. Validate git_repo URL format (must be valid Git URL)
2. Validate build_config schema matches supported types
3. On create, optionally trigger initial build
4. On delete, clean up all releases, deployments, K8s resources

**Files:**
- `apps/switchyard-api/internal/api/service_handlers.go`
- `apps/switchyard-api/internal/db/services.go`

---

### 1.2 Build Pipeline Execution
**Priority:** P1 | **Effort:** 5-7 days

**Current State:**
- `internal/builder/buildpacks.go` exists with Paketo Buildpacks support
- `internal/builder/docker.go` exists for Dockerfile builds
- Build triggers exist but may not execute properly

**Required Implementation:**
1. **Build Queue:** Async build processing with status tracking
2. **Build Logs:** Real-time log streaming during build
3. **Build Artifacts:** Push to ghcr.io/madfam-org with proper tags
4. **SBOM Generation:** CycloneDX SBOM on successful build
5. **Image Signing:** Cosign signature on push

**Build Flow:**
```
Webhook/Manual Trigger
    ↓
Create Release (status: building)
    ↓
Clone repo at git_sha
    ↓
Detect build type (Dockerfile vs Buildpacks)
    ↓
Execute build → Stream logs
    ↓
Push image to registry
    ↓
Generate SBOM → Store in R2
    ↓
Update Release (status: ready, image_uri)
    ↓
If auto_deploy → Create Deployment
```

**Files:**
- `apps/switchyard-api/internal/builder/buildpacks.go`
- `apps/switchyard-api/internal/builder/docker.go`
- `apps/switchyard-api/internal/api/build_handlers.go`
- `apps/switchyard-api/internal/services/build.go` (create if missing)

---

### 1.3 Kubernetes Reconciler Enhancement
**Priority:** P1 | **Effort:** 3-5 days

**Current State:**
- `internal/reconciler/controller.go` has basic worker pool
- `internal/reconciler/service.go` generates K8s manifests
- May not handle all deployment scenarios

**Required Enhancements:**
1. **Rolling Updates:** Proper RollingUpdate strategy with maxSurge/maxUnavailable
2. **Health Checks:** Generate readiness/liveness probes from service config
3. **Resource Limits:** Apply CPU/memory limits from service spec
4. **Service Discovery:** Create K8s Service for each deployment
5. **Ingress Rules:** Generate Ingress resources for exposed services
6. **Status Sync:** Watch deployment status and update DB

**Reconciliation Loop:**
```go
func (r *Reconciler) reconcileDeployment(ctx context.Context, d *Deployment) error {
    // 1. Generate Deployment manifest
    // 2. Generate Service manifest
    // 3. Generate Ingress if service.expose = true
    // 4. Apply via kubectl/client-go
    // 5. Watch rollout status
    // 6. Update deployment.status in DB
    // 7. Update deployment.health based on pod status
}
```

**Files:**
- `apps/switchyard-api/internal/reconciler/controller.go`
- `apps/switchyard-api/internal/reconciler/service.go`
- `apps/switchyard-api/internal/reconciler/ingress.go` (create)

---

### 1.4 Environment Variables Management
**Priority:** P1 | **Effort:** 2-3 days

**Current State:**
- `envvar_handlers.go` exists (14KB)
- Migration `011_environment_variables.up.sql` exists

**Required Features:**
1. **CRUD for env vars** per service/environment
2. **Secret detection:** Mark vars as sensitive, encrypt at rest
3. **Injection:** Pass env vars to deployments via K8s Secrets
4. **Bulk operations:** Import/export env vars
5. **Preview env inheritance:** Preview envs inherit from parent

**Endpoints:**
```
GET    /v1/services/:id/env                - List env vars
POST   /v1/services/:id/env                - Set env var(s)
PUT    /v1/services/:id/env/:key           - Update single var
DELETE /v1/services/:id/env/:key           - Delete var
POST   /v1/services/:id/env/bulk           - Bulk import
```

**Files:**
- `apps/switchyard-api/internal/api/envvar_handlers.go`
- `apps/switchyard-api/internal/db/envvars.go`

---

## PHASE 2: DEVELOPER EXPERIENCE (Estimated: 3-4 Weeks)

### 2.1 Preview Environments (PR-based)
**Priority:** P2 | **Effort:** 5-7 days

**Current State:**
- `preview_handlers.go` exists (17KB)
- Migration `012_preview_environments.up.sql` exists
- Webhook handles PR events

**Required Implementation:**
1. **On PR Open:** Create preview environment `preview-{pr-number}`
2. **Auto-deploy:** Deploy PR branch to preview env
3. **Preview URL:** Generate `pr-{number}.preview.{project}.enclii.dev`
4. **Comment on PR:** Post preview URL as GitHub comment
5. **On PR Update:** Rebuild and redeploy
6. **On PR Close/Merge:** Delete preview environment and resources

**Webhook Flow:**
```go
func (h *Handler) handlePullRequest(ctx context.Context, event github.PullRequestEvent) {
    switch event.Action {
    case "opened", "reopened", "synchronize":
        // Create/update preview environment
        env := h.createPreviewEnv(event.PullRequest.Number)
        release := h.triggerBuild(service, event.PullRequest.Head.SHA)
        h.deploy(release, env)
        h.postGitHubComment(event.PullRequest, env.URL)
    case "closed":
        h.deletePreviewEnv(event.PullRequest.Number)
    }
}
```

**Files:**
- `apps/switchyard-api/internal/api/preview_handlers.go`
- `apps/switchyard-api/internal/api/webhook_handlers.go`

---

### 2.2 Real-time Log Streaming
**Priority:** P2 | **Effort:** 3-4 days

**Current State:**
- `logs_handlers.go` exists (14KB)
- CLI has `enclii logs -f` command

**Required Features:**
1. **Build logs:** Stream during build execution
2. **Runtime logs:** Stream from running pods via K8s API
3. **WebSocket support:** Real-time streaming to UI
4. **Log persistence:** Store last N lines in DB or object storage
5. **Multi-pod aggregation:** Merge logs from all replicas

**Implementation:**
```go
// WebSocket endpoint for log streaming
func (h *Handler) StreamLogs(c *gin.Context) {
    conn, _ := upgrader.Upgrade(c.Writer, c.Request, nil)
    defer conn.Close()

    podLogs := h.k8s.GetPodLogs(namespace, labelSelector, follow=true)
    for line := range podLogs {
        conn.WriteMessage(websocket.TextMessage, line)
    }
}
```

**Files:**
- `apps/switchyard-api/internal/api/logs_handlers.go`
- `apps/switchyard-ui/app/(protected)/services/[slug]/logs/page.tsx`

---

### 2.3 Deployment Rollback
**Priority:** P2 | **Effort:** 2-3 days

**Required Features:**
1. **One-click rollback:** Redeploy previous release
2. **Release history:** List all releases for a service
3. **Rollback API:** `POST /v1/deployments/:id/rollback`
4. **CLI command:** `enclii rollback <service> [--to-release <id>]`

**Implementation:**
```go
func (h *Handler) Rollback(c *gin.Context) {
    deployment := h.db.GetDeployment(c.Param("id"))
    previousRelease := h.db.GetPreviousRelease(deployment.ReleaseID)

    newDeployment := &Deployment{
        ReleaseID:     previousRelease.ID,
        EnvironmentID: deployment.EnvironmentID,
        Status:        "pending",
    }
    h.db.CreateDeployment(newDeployment)
    h.reconciler.Schedule(newDeployment)
}
```

**Files:**
- `apps/switchyard-api/internal/api/deployment_handlers.go`
- `packages/cli/internal/cmd/rollback.go`

---

### 2.4 Custom Domains
**Priority:** P2 | **Effort:** 5-7 days

**Current State:**
- `domain_handlers.go` exists (12KB)
- Migration `004_custom_domains_routes.up.sql` exists
- cert-manager NOT deployed

**Required Implementation:**
1. **Deploy cert-manager** with Let's Encrypt ClusterIssuer
2. **Domain API:** CRUD for custom domains per service
3. **DNS validation:** Verify domain ownership via CNAME
4. **Certificate provisioning:** Auto-generate TLS cert
5. **Ingress update:** Add domain to service Ingress

**Domain Flow:**
```
User adds domain → DNS validation CNAME → Verify ownership
    → Create Certificate resource → cert-manager provisions
    → Update Ingress with TLS → Domain active
```

**Infrastructure:**
```yaml
# Deploy cert-manager
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Create ClusterIssuer
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: admin@enclii.dev
    privateKeySecretRef:
      name: letsencrypt-prod-key
    solvers:
    - http01:
        ingress:
          class: nginx
```

**Files:**
- `apps/switchyard-api/internal/api/domain_handlers.go`
- `apps/switchyard-api/internal/reconciler/ingress.go`
- `infra/k8s/base/cert-manager.yaml` (create)

---

## PHASE 3: RAILWAY PARITY FEATURES (Estimated: 4-6 Weeks)

### 3.1 Database Addon Provisioning
**Priority:** P3 | **Effort:** 2-3 weeks

**Goal:** One-click PostgreSQL/Redis/MongoDB provisioning within projects

**Implementation Options:**
1. **Helm-based:** Deploy database Helm charts per addon
2. **Operator-based:** Use CloudNativePG, Redis Operator
3. **Cloud-managed:** Provision RDS/CloudSQL via Terraform

**Recommended: CloudNativePG Operator**
```yaml
# Per-project database
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: myproject-postgres
  namespace: myproject-prod
spec:
  instances: 1
  storage:
    size: 10Gi
```

**API Endpoints:**
```
POST   /v1/projects/:id/addons          - Create addon
GET    /v1/projects/:id/addons          - List addons
DELETE /v1/projects/:id/addons/:addon   - Delete addon
GET    /v1/projects/:id/addons/:addon/credentials - Get connection string
```

**Files:**
- `apps/switchyard-api/internal/api/addon_handlers.go` (create)
- `apps/switchyard-api/internal/addons/postgres.go` (create)
- `apps/switchyard-api/internal/addons/redis.go` (create)

---

### 3.2 Persistent Volumes for Services
**Priority:** P3 | **Effort:** 1-2 weeks

**Current State:**
- Spec supports volumes but reconciler doesn't generate PVCs

**Required:**
```yaml
# Service spec with volumes
volumes:
  - name: data
    path: /app/data
    size: 10Gi
```

**Reconciler generates:**
```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: myservice-data
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 10Gi
---
# In Deployment
volumeMounts:
  - name: data
    mountPath: /app/data
volumes:
  - name: data
    persistentVolumeClaim:
      claimName: myservice-data
```

**Files:**
- `apps/switchyard-api/internal/reconciler/service.go`
- `apps/switchyard-api/internal/reconciler/pvc.go` (create)

---

### 3.3 Scheduled Jobs (Cron)
**Priority:** P3 | **Effort:** 1 week

**Required:**
```yaml
# Service spec with cron jobs
jobs:
  - name: daily-backup
    schedule: "0 2 * * *"
    command: ["./backup.sh"]
```

**Reconciler generates:**
```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: myservice-daily-backup
spec:
  schedule: "0 2 * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: job
            image: <service-image>
            command: ["./backup.sh"]
          restartPolicy: OnFailure
```

**Files:**
- `apps/switchyard-api/internal/reconciler/cronjob.go` (create)

---

### 3.4 Autoscaling (HPA)
**Priority:** P3 | **Effort:** 1 week

**Required:**
```yaml
# Service spec with autoscaling
autoscaling:
  enabled: true
  minReplicas: 1
  maxReplicas: 10
  targetCPUUtilization: 70
```

**Reconciler generates:**
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: myservice
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: myservice
  minReplicas: 1
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

**Files:**
- `apps/switchyard-api/internal/reconciler/hpa.go` (create)

---

## PHASE 4: UI COMPLETION (Estimated: 2-3 Weeks)

### 4.1 Service Import Flow
**Priority:** P2 | **Effort:** 3-4 days

**Current:** UI shows GitHub repo browser but import doesn't complete

**Required:**
1. List user's GitHub repos (via Janua OAuth)
2. Select repo → Create service with auto-detected build config
3. Show build type detection result (Dockerfile vs Buildpacks)
4. Trigger initial build on import

**Files:**
- `apps/switchyard-ui/app/(protected)/services/import/page.tsx`
- `apps/switchyard-ui/lib/api.ts`

---

### 4.2 Deployment Dashboard
**Priority:** P2 | **Effort:** 3-4 days

**Required Features:**
1. List all deployments with status indicators
2. Show deployment history per service
3. Real-time status updates (pending → building → deploying → running)
4. One-click rollback button
5. Resource usage graphs (CPU/memory)

**Files:**
- `apps/switchyard-ui/app/(protected)/services/[slug]/deployments/page.tsx`
- `apps/switchyard-ui/components/deployment-card.tsx`

---

### 4.3 Environment Variable Editor
**Priority:** P2 | **Effort:** 2-3 days

**Required:**
1. Key-value editor with add/edit/delete
2. Secret masking (show ••••••••)
3. Bulk import from .env file
4. Copy to clipboard
5. Sync indicator with deploy button

**Files:**
- `apps/switchyard-ui/app/(protected)/services/[slug]/env/page.tsx`
- `apps/switchyard-ui/components/env-editor.tsx`

---

### 4.4 Log Viewer
**Priority:** P2 | **Effort:** 2-3 days

**Required:**
1. Real-time log stream via WebSocket
2. Search/filter functionality
3. Timestamp toggle
4. Download logs button
5. Pod selector for multi-replica services

**Files:**
- `apps/switchyard-ui/app/(protected)/services/[slug]/logs/page.tsx`
- `apps/switchyard-ui/components/log-viewer.tsx`

---

## PHASE 5: VERCEL PARITY (OPTIONAL - Estimated: 3-6 Months)

These are nice-to-have features that achieve Vercel parity but are not required for Railway parity.

### 5.1 CDN Integration
- Cloudflare caching rules for static assets
- Cache invalidation on deploy

### 5.2 Image Optimization
- Next.js Image Optimization API
- Cloudflare Polish integration

### 5.3 Edge Middleware
- Cloudflare Workers for edge logic
- A/B testing at edge

### 5.4 Multi-Region Deployments
- K8s cluster federation
- GeoDNS routing

---

## IMPLEMENTATION ORDER SUMMARY

```
WEEK 1 (Critical Fixes):
├── 0.1 Database migrations (1 day)
├── 0.2 JWT nil pointer fix (0.5 day)
├── 0.3 GitHub webhook setup (0.5 day)
├── 0.4 Register dogfooding services (0.5 day)
└── 1.1 Service CRUD completion (2-3 days)

WEEK 2-3 (Core Build/Deploy):
├── 1.2 Build pipeline execution (5-7 days)
├── 1.3 Reconciler enhancements (3-5 days)
└── 1.4 Environment variables (2-3 days)

WEEK 4-5 (DX Features):
├── 2.1 Preview environments (5-7 days)
├── 2.2 Log streaming (3-4 days)
└── 2.3 Rollback (2-3 days)

WEEK 6-7 (Custom Domains + UI):
├── 2.4 Custom domains (5-7 days)
├── 4.1 Service import UI (3-4 days)
└── 4.2-4.4 Dashboard/Logs UI (5-7 days)

WEEK 8-12 (Railway Parity):
├── 3.1 Database addons (2-3 weeks)
├── 3.2 Persistent volumes (1-2 weeks)
├── 3.3 Cron jobs (1 week)
└── 3.4 Autoscaling (1 week)
```

---

## KEY FILE REFERENCES

### Control Plane API
```
apps/switchyard-api/
├── cmd/server/main.go              # Entry point - ADD MIGRATION RUNNER
├── internal/
│   ├── api/
│   │   ├── handlers.go             # Route registration
│   │   ├── webhook_handlers.go     # GitHub webhooks
│   │   ├── build_handlers.go       # Build triggers
│   │   ├── deployment_handlers.go  # Deployment CRUD
│   │   ├── service_handlers.go     # Service CRUD
│   │   └── preview_handlers.go     # PR previews
│   ├── auth/
│   │   └── jwt.go                  # FIX NIL POINTER
│   ├── builder/
│   │   ├── buildpacks.go           # Paketo builds
│   │   └── docker.go               # Dockerfile builds
│   ├── reconciler/
│   │   ├── controller.go           # Worker pool
│   │   └── service.go              # K8s manifest generation
│   └── db/
│       ├── migrations/             # SQL migrations
│       └── db.go                   # ADD RunMigrations()
```

### Web UI
```
apps/switchyard-ui/
├── app/(protected)/
│   ├── services/
│   │   ├── import/page.tsx         # GitHub import flow
│   │   └── [slug]/
│   │       ├── page.tsx            # Service overview
│   │       ├── deployments/        # Deployment history
│   │       ├── env/                # Env var editor
│   │       └── logs/               # Log viewer
│   └── dashboard/page.tsx          # Main dashboard
├── components/
│   ├── deployment-card.tsx
│   ├── env-editor.tsx
│   └── log-viewer.tsx
└── lib/api.ts                      # API client
```

### Infrastructure
```
infra/
├── k8s/
│   ├── base/
│   │   ├── postgres.yaml           # Control plane DB
│   │   └── redis.yaml              # Session store
│   └── production/
│       ├── switchyard-api.yaml     # ADD INIT CONTAINER
│       └── switchyard-ui.yaml
└── terraform/
    └── modules/                    # Hetzner/Cloudflare
```

---

## SUCCESS CRITERIA

**Phase 0 Complete When:**
- [ ] `SELECT COUNT(*) FROM services` returns rows
- [ ] No panic logs in API pod
- [ ] GitHub webhook delivers to API (check webhook deliveries)
- [ ] `curl https://api.enclii.dev/v1/projects` returns data

**Phase 1 Complete When:**
- [ ] Can create service via API
- [ ] Webhook triggers build
- [ ] Build pushes image to ghcr.io
- [ ] Deployment creates running pods

**Phase 2 Complete When:**
- [ ] PR creates preview environment automatically
- [ ] Preview URL works
- [ ] Logs stream in real-time
- [ ] Rollback works

**Full Parity When:**
- [ ] Can deploy any Dockerfile/Buildpacks project
- [ ] Custom domains with auto-SSL
- [ ] One-click database provisioning
- [ ] Complete UI for all operations
- [ ] CLI fully functional

---

## ENVIRONMENT VARIABLES REQUIRED

```bash
# Database
DATABASE_URL=postgresql://enclii:xxx@postgres.data.svc.cluster.local:5432/enclii

# Auth (Janua)
OIDC_ISSUER=https://auth.madfam.io
OIDC_CLIENT_ID=jnc_l_Q6z3Q07H2jEOdwrV9OxbGOWFjZojIq
OIDC_CLIENT_SECRET=<secret>
OIDC_REDIRECT_URI=https://app.enclii.dev/api/auth/callback

# GitHub
GITHUB_WEBHOOK_SECRET=0a619aa7b0bf6b1bf75e252dacfc02a2afac33e4ccbe19a9ff0077bdc9d33508

# Container Registry
REGISTRY_URL=ghcr.io/madfam-org
REGISTRY_USERNAME=<github-username>
REGISTRY_PASSWORD=<github-pat>

# Object Storage (R2)
R2_ENDPOINT=https://<account>.r2.cloudflarestorage.com
R2_ACCESS_KEY_ID=<key>
R2_SECRET_ACCESS_KEY=<secret>
R2_BUCKET=enclii-artifacts
```

---

**END OF PROMPT**

This prompt provides a complete, prioritized implementation plan. Execute phases in order. Phase 0 must be completed first as everything else depends on a working database and authentication.
