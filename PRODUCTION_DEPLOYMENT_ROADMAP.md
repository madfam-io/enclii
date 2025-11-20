# Enclii Production Deployment Roadmap
**Date:** November 20, 2025
**Current Production Readiness:** 70%
**Target Production Readiness:** 95%+
**Estimated Timeline:** 6-8 weeks
**Estimated Monthly Cost:** $450-650 (vs $2,000+ with Auth0/Clerk)

---

## Executive Summary

This roadmap outlines the path to deploying Enclii to production with:
- ✅ **Best value infrastructure** using managed Kubernetes + managed databases
- ✅ **High availability** with multi-AZ deployment and automated failover
- ✅ **Plinto integration** for authentication (saving $24K-58K/year vs Auth0)
- ✅ **Full observability** with Prometheus, Grafana, and Loki
- ✅ **SOC 2 compliance** readiness with audit logging and secrets management

**Recommended Infrastructure:** DigitalOcean Kubernetes (DOKS) + Managed PostgreSQL + Managed Redis

**Why DigitalOcean?**
- 50% cheaper than AWS/GCP for similar workloads
- Fully managed Kubernetes with automatic updates
- Built-in load balancers and block storage
- Excellent documentation and developer experience
- Predictable pricing (no surprise bills)
- 99.95% uptime SLA

---

## Part 1: Current State Assessment

### What We Have ✅

**Platform Features (70% Production Ready):**
- JWT authentication infrastructure with RBAC
- CSRF protection middleware
- Security headers (CSP, HSTS, X-Frame-Options)
- Pagination to prevent DoS attacks
- Rate limiting with memory-bounded LRU cache
- Audit logging infrastructure (async with fallback)
- 11 security tests (100% middleware coverage)

**Infrastructure (60% Production Ready):**
- Complete Kubernetes manifests for all services
- PostgreSQL deployment with health checks
- Redis deployment with persistence (AOF + RDB)
- Nginx ingress controller configured
- cert-manager with Let's Encrypt integration
- NetworkPolicies for service isolation
- Pod security contexts (non-root, read-only FS)
- RBAC with ClusterRole/ServiceAccount
- Jaeger tracing deployment
- Prometheus ServiceMonitor definitions
- Environment-specific overlays (dev/staging/production)

**Documentation:**
- Comprehensive audit reports (15,000+ lines)
- Deployment guides (1,726 lines)
- Secrets management strategy (315 lines)
- Migration guides (Railway, Vercel)
- Architecture documentation

### Critical Gaps ❌

| Gap | Impact | Priority |
|-----|--------|----------|
| **No Database HA** | Single point of failure | 🔴 Critical |
| **Secrets in plaintext YAML** | SOC 2 violation | 🔴 Critical |
| **No cloud infrastructure** | Cannot deploy to production | 🔴 Critical |
| **No automated backups** | Data loss risk | 🔴 Critical |
| **Prometheus not deployed** | Cannot monitor SLOs | 🔴 Critical |
| **No authentication service** | JWT infra exists but no login UI | 🔴 Critical |
| **No Grafana dashboards** | Cannot visualize metrics | 🟠 High |
| **No log aggregation** | Difficult debugging | 🟠 High |
| **No auto-scaling** | Cannot handle traffic spikes | 🟠 High |
| **TLS not configured** | HTTP-only traffic | 🟠 High |

---

## Part 2: Recommended Infrastructure (Best Value + Stability)

### Option A: DigitalOcean (RECOMMENDED) 💰

**Monthly Cost Estimate: $450-600**

| Component | Specification | Monthly Cost |
|-----------|---------------|--------------|
| **DOKS Cluster** | 3 nodes × $48 (4 vCPU, 8GB RAM) | $144 |
| **Managed PostgreSQL** | HA cluster (Primary + Standby, 4GB RAM) | $120 |
| **Managed Redis** | 2GB RAM, persistence enabled | $30 |
| **Load Balancer** | Included with DOKS | $12 |
| **Block Storage** | 100GB SSD for PVCs | $10 |
| **Backups** | Daily automated database backups | $20 |
| **Spaces (S3)** | 250GB object storage for logs/backups | $5 |
| **Monitoring** | Self-hosted Prometheus + Grafana | $0 |
| **Plinto** | Self-hosted auth platform | $0 |
| **Bandwidth** | 6TB included (typical usage ~500GB) | $0 |
| **Snapshots** | Weekly cluster snapshots | $15 |
| **Total** | | **$356/month** |
| **Buffer (20%)** | For scaling and overages | **+$71** |
| **Grand Total** | | **~$427/month** |

**Pros:**
- ✅ Simple, predictable pricing
- ✅ Excellent documentation and support
- ✅ Fully managed control plane (free)
- ✅ One-click HA PostgreSQL with automated failover
- ✅ Built-in monitoring dashboard
- ✅ 99.95% uptime SLA
- ✅ Easy migration path to more powerful nodes

**Cons:**
- ⚠️ Fewer regions than AWS/GCP (15 vs 30+)
- ⚠️ Limited advanced features (no managed Kafka, etc.)

### Option B: AWS EKS (Enterprise-grade)

**Monthly Cost Estimate: $650-850**

| Component | Specification | Monthly Cost |
|-----------|---------------|--------------|
| **EKS Cluster** | Control plane (per cluster) | $72 |
| **EC2 Instances** | 3 × t3.large (2 vCPU, 8GB RAM) | $152 |
| **RDS PostgreSQL** | db.t3.medium Multi-AZ | $134 |
| **ElastiCache Redis** | cache.t3.micro | $24 |
| **Application Load Balancer** | | $23 |
| **EBS Storage** | 100GB gp3 | $10 |
| **Data Transfer** | ~500GB/month | $45 |
| **Backups (RDS)** | 100GB snapshots | $10 |
| **S3** | 250GB for logs/backups | $6 |
| **CloudWatch** | Basic metrics and logs | $30 |
| **NAT Gateway** | High availability (2 AZs) | $73 |
| **Total** | | **$579/month** |
| **Buffer (20%)** | | **+$116** |
| **Grand Total** | | **~$695/month** |

**Pros:**
- ✅ Battle-tested at massive scale
- ✅ Deep integration with AWS services (IAM, VPC, etc.)
- ✅ Best-in-class security features
- ✅ Global presence (30+ regions)
- ✅ Advanced networking (VPC peering, PrivateLink)

**Cons:**
- ⚠️ 60% more expensive than DigitalOcean
- ⚠️ Complexity (steep learning curve)
- ⚠️ NAT Gateway costs add up quickly
- ⚠️ Control plane costs $72/month (free on DO)

### Option C: Google Cloud GKE (Developer-friendly)

**Monthly Cost Estimate: $600-800**

| Component | Specification | Monthly Cost |
|-----------|---------------|--------------|
| **GKE Autopilot** | Control plane + managed nodes | $300 |
| **Cloud SQL PostgreSQL** | db-n1-standard-1 HA | $165 |
| **Memorystore Redis** | 2GB Basic | $48 |
| **Load Balancer** | | $18 |
| **Persistent Disk** | 100GB SSD | $17 |
| **Cloud Logging** | 50GB ingestion | $25 |
| **Cloud Storage** | 250GB for backups | $5 |
| **Total** | | **$578/month** |
| **Buffer (20%)** | | **+$116** |
| **Grand Total** | | **~$694/month** |

**Pros:**
- ✅ GKE Autopilot eliminates node management
- ✅ Excellent container-native experience
- ✅ Built-in security scanning (Binary Authorization)
- ✅ Generous free tier (still applies to some services)

**Cons:**
- ⚠️ 60% more expensive than DigitalOcean
- ⚠️ Autopilot can be opinionated (less control)
- ⚠️ Logging costs add up for high-traffic apps

### Option D: Hetzner Cloud (Ultra Budget)

**Monthly Cost Estimate: $150-250**

| Component | Specification | Monthly Cost |
|-----------|---------------|--------------|
| **Managed K8s (K3s)** | 3 × CPX31 (4 vCPU, 8GB RAM) | $84 |
| **Managed PostgreSQL** | Not available - self-host | $0 |
| **Managed Redis** | Not available - self-host | $0 |
| **Load Balancer** | Hetzner LB | $5 |
| **Block Storage** | 100GB | $5 |
| **Backups** | Volume snapshots | $10 |
| **S3-compatible (Wasabi)** | 250GB | $6 |
| **Total** | | **$110/month** |
| **Buffer** | | **+$40** |
| **Grand Total** | | **~$150/month** |

**Pros:**
- ✅ Extremely cheap (70% less than DigitalOcean)
- ✅ Excellent hardware for the price
- ✅ EU-based (GDPR-friendly)

**Cons:**
- ⚠️ No managed databases (must self-host with Patroni)
- ⚠️ Limited regions (Germany, Finland, USA)
- ⚠️ Smaller community and ecosystem
- ⚠️ More operational overhead

---

## Part 3: Recommended Choice - DigitalOcean DOKS

**Why DigitalOcean wins for Enclii:**

1. **Best value/complexity ratio** - 50% cheaper than AWS while remaining fully managed
2. **Managed databases out-of-the-box** - PostgreSQL HA cluster with one click
3. **No surprise bills** - Predictable pricing with bandwidth included
4. **Production-ready quickly** - Less configuration than AWS/GCP
5. **Scales when needed** - Easy to upgrade nodes or add clusters

**Cost Savings vs SaaS Auth:**
- **Auth0/Clerk:** $2,000-5,000/month
- **Plinto (self-hosted):** $0 (included in infrastructure cost)
- **Annual savings:** $24,000-58,000

**Infrastructure Architecture on DigitalOcean:**

```
┌──────────────────────────────────────────────────────────────┐
│                    DigitalOcean Cloud                        │
├──────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │              Load Balancer (TLS Termination)           │ │
│  │          enclii.dev → 443 (HTTPS)                      │ │
│  └───────────────────────┬────────────────────────────────┘ │
│                          │                                   │
│  ┌───────────────────────▼────────────────────────────────┐ │
│  │         DOKS Kubernetes Cluster (3 nodes)              │ │
│  │                                                          │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │ │
│  │  │ Switchyard   │  │   Plinto     │  │ Conductor    │ │ │
│  │  │  API (5x)    │  │  Auth (3x)   │  │  CLI (2x)    │ │ │
│  │  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘ │ │
│  │         │                 │                 │           │ │
│  │  ┌──────▼─────────────────▼─────────────────▼───────┐ │ │
│  │  │          Observability Stack                      │ │ │
│  │  │  Prometheus │ Grafana │ Loki │ Jaeger            │ │ │
│  │  └──────────────────────────────────────────────────┘ │ │
│  │                                                          │ │
│  └──────────────────────────────────────────────────────┘ │
│                          │                                   │
│  ┌───────────────────────▼────────────────────────────────┐ │
│  │  Managed PostgreSQL HA Cluster (Primary + Standby)    │ │
│  │  - Automatic failover (30s)                            │ │
│  │  - Daily backups (7-day retention)                     │ │
│  │  - Point-in-time recovery (PITR)                       │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  Managed Redis (2GB, persistence enabled)             │ │
│  │  - AOF + RDB backups                                   │ │
│  │  - Automatic failover                                  │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  Spaces (S3-compatible)                                │ │
│  │  - Log archival                                        │ │
│  │  - Database backup storage (offsite)                   │ │
│  │  - Static assets CDN                                   │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                               │
└──────────────────────────────────────────────────────────────┘
```

---

## Part 4: Plinto Integration Strategy

### Phase 3A: Deploy Plinto Authentication Platform

**Timeline:** Week 1-2 (12-16 hours)

Plinto will replace the Phase 2 placeholder authentication with a full-featured auth platform:

**What Plinto Provides:**
- ✅ Complete login/signup UI (15 pre-built components)
- ✅ OAuth 2.0 + SAML 2.0 SSO support
- ✅ Multi-factor authentication (TOTP/SMS/WebAuthn)
- ✅ Organization multi-tenancy with RBAC
- ✅ JWT tokens with RS256 signing (compatible with current Enclii middleware)
- ✅ 202 REST API endpoints
- ✅ Audit logging for compliance
- ✅ Python/Go/React/Next.js SDKs

**Deployment Steps:**

#### 1. Add Plinto to Kubernetes Manifests

Create `/home/user/enclii/infra/k8s/base/plinto.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: plinto
  labels:
    app: plinto
spec:
  replicas: 3
  selector:
    matchLabels:
      app: plinto
  template:
    metadata:
      labels:
        app: plinto
    spec:
      containers:
      - name: plinto
        image: ghcr.io/madfam-io/plinto:latest
        ports:
        - containerPort: 8000
          name: http
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: plinto-secret
              key: database-url
        - name: REDIS_URL
          valueFrom:
            secretKeyRef:
              name: plinto-secret
              key: redis-url
        - name: JWT_PRIVATE_KEY
          valueFrom:
            secretKeyRef:
              name: plinto-secret
              key: jwt-private-key
        - name: JWT_PUBLIC_KEY
          valueFrom:
            secretKeyRef:
              name: plinto-secret
              key: jwt-public-key
        - name: PLINTO_BASE_URL
          value: "https://auth.enclii.dev"
        - name: PLINTO_ALLOWED_ORIGINS
          value: "https://app.enclii.dev,https://enclii.dev"
        resources:
          requests:
            cpu: 200m
            memory: 256Mi
          limits:
            cpu: 1000m
            memory: 1Gi
        livenessProbe:
          httpGet:
            path: /health
            port: 8000
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8000
          initialDelaySeconds: 10
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: plinto
spec:
  selector:
    app: plinto
  ports:
  - port: 8000
    targetPort: 8000
    name: http
  type: ClusterIP
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: plinto-ingress
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - auth.enclii.dev
    secretName: plinto-tls
  rules:
  - host: auth.enclii.dev
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: plinto
            port:
              number: 8000
```

#### 2. Integrate Plinto with Existing Auth Middleware

**Update `apps/switchyard-api/internal/middleware/auth.go`:**

```go
// Plinto uses RS256 JWT tokens - update validation
func (a *AuthMiddleware) Middleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // ... existing code ...

        // Update JWT parsing to support RS256 from Plinto
        token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
            // Check both HS256 (internal) and RS256 (Plinto)
            switch token.Method.(type) {
            case *jwt.SigningMethodRSA:
                // Fetch Plinto's public key from /.well-known/jwks.json
                return getPlintoPublicKey()
            case *jwt.SigningMethodHMAC:
                // Internal tokens
                return a.jwtSecret, nil
            default:
                return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
            }
        })

        // ... rest of middleware ...
    }
}
```

#### 3. Add Plinto SDK to Frontend

**Install Plinto React SDK:**

```bash
cd apps/switchyard-ui
npm install @plinto/react
```

**Update `apps/switchyard-ui/app/layout.tsx`:**

```typescript
import { PlintoProvider } from '@plinto/react';

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        <PlintoProvider
          domain="auth.enclii.dev"
          clientId={process.env.NEXT_PUBLIC_PLINTO_CLIENT_ID!}
          audience="https://api.enclii.dev"
        >
          {children}
        </PlintoProvider>
      </body>
    </html>
  );
}
```

**Replace `apps/switchyard-ui/contexts/AuthContext.tsx` with Plinto hooks:**

```typescript
'use client';

import { usePlinto } from '@plinto/react';
import { useRouter } from 'next/navigation';
import { useEffect } from 'react';

export function useRequireAuth() {
  const { isAuthenticated, isLoading } = usePlinto();
  const router = useRouter();

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      router.push('/login');
    }
  }, [isAuthenticated, isLoading]);

  return { isAuthenticated, isLoading };
}
```

#### 4. Create Login/Signup Pages

**Create `apps/switchyard-ui/app/login/page.tsx`:**

```typescript
'use client';

import { SignIn } from '@plinto/react';
import { useRouter } from 'next/navigation';

export default function LoginPage() {
  const router = useRouter();

  return (
    <div className="flex min-h-screen items-center justify-center">
      <SignIn
        onSuccess={() => router.push('/dashboard')}
        providers={['google', 'github', 'email']}
      />
    </div>
  );
}
```

#### 5. Migration Path from Current Auth

**Step 1:** Deploy Plinto alongside existing auth
**Step 2:** Configure Plinto to share JWT signing key (temporary)
**Step 3:** Migrate frontend to use Plinto UI components
**Step 4:** Migrate API to validate both old and new tokens
**Step 5:** Deprecate old auth endpoints after 30-day grace period
**Step 6:** Remove old AuthContext and custom login pages

**Database Migration:**

```sql
-- Export existing users from switchyard database
SELECT id, email, name, created_at
FROM users
ORDER BY created_at;

-- Import into Plinto via API
POST https://auth.enclii.dev/api/v1/users/bulk-import
{
  "users": [...],
  "send_welcome_email": true
}
```

### Plinto Cost Analysis

**Self-hosted Plinto infrastructure:**
- Compute: Already included in DOKS cluster (no additional cost)
- Database: Shares PostgreSQL with Switchyard (no additional cost)
- Redis: Shares Redis with Switchyard (no additional cost)
- Total additional cost: **$0/month**

**Savings vs Auth0:**
- Auth0 Professional: $228/month + $0.35/user/month (breaks even at ~650 users = $228 + $227 = $455)
- Auth0 Enterprise: $2,000-5,000/month base
- **Annual savings with Plinto: $24,000-58,000**

---

## Part 5: Production Deployment Phases

### Phase 3: Infrastructure & Observability (Weeks 1-3)

**Goal:** Deploy to DigitalOcean with full observability and HA databases

| Task | Effort | Cost |
|------|--------|------|
| **3.1: Set up DigitalOcean account** | 1h | $0 |
| **3.2: Create DOKS cluster (3 nodes)** | 2h | $144/mo |
| **3.3: Provision managed PostgreSQL HA** | 1h | $120/mo |
| **3.4: Provision managed Redis** | 1h | $30/mo |
| **3.5: Configure VPC and firewall rules** | 2h | $0 |
| **3.6: Set up Spaces (S3) for backups** | 1h | $5/mo |
| **3.7: Install Sealed Secrets** | 2h | $0 |
| **3.8: Migrate secrets from dev to sealed** | 3h | $0 |
| **3.9: Deploy Prometheus Operator** | 4h | $0 |
| **3.10: Deploy Grafana with dashboards** | 6h | $0 |
| **3.11: Deploy Loki for log aggregation** | 4h | $0 |
| **3.12: Configure Jaeger integration** | 2h | $0 |
| **3.13: Set up alert rules (Slack/PagerDuty)** | 3h | $10/mo |
| **3.14: Configure automated backups** | 3h | $20/mo |
| **3.15: Deploy Plinto auth platform** | 8h | $0 |
| **3.16: Integrate Plinto with Switchyard** | 6h | $0 |
| **Total** | **48h (1.5 weeks)** | **$329/mo** |

**Deliverables:**
- ✅ Production Kubernetes cluster on DigitalOcean
- ✅ HA PostgreSQL with automated backups (RPO <1h, RTO <4h)
- ✅ HA Redis with persistence
- ✅ Complete observability stack (Prometheus, Grafana, Loki, Jaeger)
- ✅ Encrypted secrets management (Sealed Secrets)
- ✅ Plinto authentication platform deployed
- ✅ TLS certificates with Let's Encrypt
- ✅ Alert rules for SLO violations

### Phase 4: Security Hardening & Compliance (Weeks 3-4)

**Goal:** Achieve SOC 2 baseline readiness and harden security

| Task | Effort | Cost |
|------|--------|------|
| **4.1: Implement namespace-scoped RBAC** | 3h | $0 |
| **4.2: Separate service accounts per component** | 2h | $0 |
| **4.3: Add Pod Disruption Budgets** | 2h | $0 |
| **4.4: Enhance NetworkPolicies (zero-trust)** | 4h | $0 |
| **4.5: Enable Kubernetes audit logging** | 3h | $0 |
| **4.6: Implement immutable audit log table** | 4h | $0 |
| **4.7: Configure TLS between services (mTLS)** | 6h | $0 |
| **4.8: Set up secret rotation (90-day policy)** | 4h | $0 |
| **4.9: Implement resource quotas per namespace** | 2h | $0 |
| **4.10: Add admission controllers (Kyverno/OPA)** | 6h | $0 |
| **4.11: Container image scanning (Trivy)** | 3h | $0 |
| **4.12: Vulnerability scanning automation** | 3h | $0 |
| **4.13: Configure egress filtering** | 3h | $0 |
| **4.14: Add rate limiting at ingress level** | 2h | $0 |
| **Total** | **47h (1.5 weeks)** | **$0** |

**Deliverables:**
- ✅ SOC 2 CC6.1-6.8 controls implemented
- ✅ Zero-trust network architecture
- ✅ Immutable audit trails
- ✅ Automated vulnerability scanning
- ✅ Policy enforcement with admission controllers
- ✅ Secrets rotation automation

### Phase 5: Operational Excellence (Weeks 4-6)

**Goal:** Implement auto-scaling, CI/CD, and disaster recovery

| Task | Effort | Cost |
|------|--------|------|
| **5.1: Configure Horizontal Pod Autoscaler (HPA)** | 3h | $0 |
| **5.2: Configure Vertical Pod Autoscaler (VPA)** | 2h | $0 |
| **5.3: Set up GitHub Actions CI/CD pipeline** | 8h | $0 |
| **5.4: Implement blue-green deployment** | 6h | $0 |
| **5.5: Configure canary deployments with Flagger** | 6h | $0 |
| **5.6: Create DR runbooks and automation** | 8h | $0 |
| **5.7: Implement backup restoration tests (weekly)** | 4h | $0 |
| **5.8: Set up staging environment** | 4h | $50/mo |
| **5.9: Configure load testing (k6)** | 6h | $0 |
| **5.10: Implement chaos engineering tests (Chaos Mesh)** | 6h | $0 |
| **5.11: Create operational dashboards** | 4h | $0 |
| **5.12: Set up on-call rotation (PagerDuty)** | 2h | $29/mo |
| **5.13: Document runbooks (incident response)** | 8h | $0 |
| **Total** | **67h (2 weeks)** | **$79/mo** |

**Deliverables:**
- ✅ Auto-scaling based on CPU/memory/custom metrics
- ✅ Automated CI/CD with GitOps
- ✅ Zero-downtime deployments (blue-green + canary)
- ✅ Tested disaster recovery procedures
- ✅ Chaos engineering validation
- ✅ On-call rotation and incident response

### Phase 6: Testing & Validation (Weeks 6-8)

**Goal:** Comprehensive testing and production readiness validation

| Task | Effort | Cost |
|------|--------|------|
| **6.1: Expand unit test coverage to 80%+** | 16h | $0 |
| **6.2: Write integration tests (API + DB)** | 12h | $0 |
| **6.3: Implement E2E tests (Playwright)** | 12h | $0 |
| **6.4: Load testing (1000 RPS sustained)** | 6h | $0 |
| **6.5: Penetration testing (OWASP Top 10)** | 8h | $0 |
| **6.6: Security audit (third-party recommended)** | 16h | $2,000 |
| **6.7: Performance benchmarking** | 4h | $0 |
| **6.8: SLO validation (99.95% target)** | 4h | $0 |
| **6.9: Compliance documentation (SOC 2)** | 12h | $0 |
| **6.10: User acceptance testing** | 8h | $0 |
| **Total** | **98h (3 weeks)** | **$2,000** |

**Deliverables:**
- ✅ 80%+ code coverage
- ✅ Comprehensive integration and E2E tests
- ✅ Load tested to 1000 RPS
- ✅ Security validated (penetration test)
- ✅ SOC 2 documentation complete
- ✅ Production readiness: 95%+

---

## Part 6: Total Cost Summary

### One-Time Costs

| Item | Cost |
|------|------|
| **Infrastructure setup** | $0 (DIY) |
| **Third-party security audit** | $2,000 (optional but recommended) |
| **Domain registration** | $12/year |
| **SSL certificates** | $0 (Let's Encrypt) |
| **Total One-Time** | **$2,012** |

### Monthly Recurring Costs

| Item | Cost |
|------|------|
| **DigitalOcean DOKS (3 nodes)** | $144 |
| **Managed PostgreSQL HA** | $120 |
| **Managed Redis** | $30 |
| **Load Balancer** | $12 |
| **Block Storage (100GB)** | $10 |
| **Spaces (S3, 250GB)** | $5 |
| **Backups** | $20 |
| **Snapshots** | $15 |
| **Staging environment (50% of prod)** | $50 |
| **Alerting (Opsgenie/PagerDuty)** | $29 |
| **Total Monthly** | **$435/month** |

**Annual cost:** $5,220 + $2,012 setup = **$7,232 first year**

### Cost Comparison vs Alternatives

| Solution | Monthly | Annual | 5-Year Total |
|----------|---------|--------|--------------|
| **Enclii (self-hosted) + Plinto** | $435 | $5,220 | $26,100 |
| **Railway + Auth0** | $2,220 | $26,640 | $133,200 |
| **Vercel + Clerk** | $2,500 | $30,000 | $150,000 |
| **AWS EKS + Cognito** | $695 | $8,340 | $41,700 |
| **GCP GKE + Firebase Auth** | $694 | $8,328 | $41,640 |

**5-Year Savings with Enclii + Plinto:**
- vs Railway + Auth0: **$107,100 saved**
- vs Vercel + Clerk: **$123,900 saved**
- vs AWS EKS: **$15,600 saved**

---

## Part 7: Production Readiness Checklist

### Infrastructure ✅

- [ ] **Cloud provider account** (DigitalOcean) configured
- [ ] **DOKS cluster** deployed with 3 nodes
- [ ] **Managed PostgreSQL HA** provisioned (Primary + Standby)
- [ ] **Managed Redis** provisioned with persistence
- [ ] **VPC and firewall rules** configured
- [ ] **Load balancer** with TLS termination
- [ ] **DNS records** pointing to load balancer
- [ ] **Spaces (S3)** configured for backups
- [ ] **Automated daily backups** enabled
- [ ] **Disaster recovery tested** (restore from backup)

### Security ✅

- [ ] **Sealed Secrets** installed and all secrets encrypted
- [ ] **TLS certificates** auto-provisioned via cert-manager
- [ ] **mTLS between services** configured
- [ ] **NetworkPolicies** enforcing zero-trust
- [ ] **RBAC** with least-privilege per service
- [ ] **Pod Security Standards** enforced (restricted)
- [ ] **Admission controllers** (Kyverno/OPA) validating deployments
- [ ] **Container image scanning** (Trivy) in CI pipeline
- [ ] **Secrets rotation** automated (90-day policy)
- [ ] **Audit logging** enabled (Kubernetes + application)

### Authentication ✅

- [ ] **Plinto deployed** with 3 replicas
- [ ] **Plinto database** schema migrated
- [ ] **Existing users migrated** to Plinto
- [ ] **JWT validation** supports both RS256 (Plinto) and HS256 (internal)
- [ ] **Frontend** integrated with Plinto React SDK
- [ ] **Login/signup pages** using Plinto components
- [ ] **OAuth providers** configured (Google, GitHub)
- [ ] **MFA enabled** for admin accounts
- [ ] **Session management** configured
- [ ] **SSO tested** (if applicable)

### Observability ✅

- [ ] **Prometheus** deployed and scraping metrics
- [ ] **Grafana** deployed with dashboards:
  - [ ] Request rate/latency/errors (RED metrics)
  - [ ] Database connection pool utilization
  - [ ] Cache hit rates
  - [ ] Pod resource usage
  - [ ] Node resource usage
  - [ ] SLO compliance (99.95% uptime)
- [ ] **Loki** deployed for log aggregation
- [ ] **Jaeger** integrated for distributed tracing
- [ ] **Alert rules** configured:
  - [ ] Error rate > 2% for 2 minutes
  - [ ] P95 latency > 500ms
  - [ ] Database connections > 80%
  - [ ] Pod restarts > 5/hour
  - [ ] Node CPU/memory > 90%
- [ ] **PagerDuty/Opsgenie** integration for on-call

### Operations ✅

- [ ] **CI/CD pipeline** (GitHub Actions) deploying automatically
- [ ] **Blue-green deployments** configured
- [ ] **Canary deployments** configured with Flagger
- [ ] **Horizontal Pod Autoscaler (HPA)** configured
- [ ] **Vertical Pod Autoscaler (VPA)** configured
- [ ] **Pod Disruption Budgets (PDB)** defined
- [ ] **Resource quotas** per namespace
- [ ] **Load testing** validated (1000 RPS sustained)
- [ ] **Chaos engineering** tests passing
- [ ] **DR runbooks** documented and tested

### Testing ✅

- [ ] **Unit tests** at 80%+ coverage
- [ ] **Integration tests** (API + database)
- [ ] **E2E tests** (Playwright) for critical flows
- [ ] **Load tests** (k6) for capacity planning
- [ ] **Security tests** (OWASP ZAP) for vulnerabilities
- [ ] **Penetration test** by third party (recommended)

### Compliance ✅

- [ ] **SOC 2 controls** implemented (CC6.1-6.8)
- [ ] **Audit logs** immutable and retained for 1 year
- [ ] **Data retention policies** documented
- [ ] **Privacy policy** and terms of service
- [ ] **Incident response plan** documented
- [ ] **Access control matrix** documented
- [ ] **Vendor management** (subprocessors listed)

---

## Part 8: Timeline and Milestones

```
Week 1-2: Infrastructure Setup
├─ Day 1-2:   Set up DigitalOcean account, create DOKS cluster
├─ Day 3-4:   Provision managed databases (PostgreSQL, Redis)
├─ Day 5-7:   Deploy Sealed Secrets, migrate all secrets
├─ Day 8-10:  Deploy observability stack (Prometheus, Grafana, Loki)
└─ Day 11-14: Deploy Plinto, integrate with Switchyard

Week 3-4: Security Hardening
├─ Day 15-17: Implement zero-trust networking (NetworkPolicies, mTLS)
├─ Day 18-20: Configure RBAC and admission controllers
├─ Day 21-23: Enable audit logging and immutable logs
├─ Day 24-26: Container scanning and vulnerability management
└─ Day 27-28: Secrets rotation automation

Week 5-6: Operational Excellence
├─ Day 29-31: Configure auto-scaling (HPA, VPA)
├─ Day 32-34: Set up CI/CD pipeline (GitHub Actions)
├─ Day 35-37: Implement blue-green and canary deployments
├─ Day 38-40: DR runbooks and backup restoration tests
└─ Day 41-42: Chaos engineering validation

Week 7-8: Testing and Validation
├─ Day 43-46: Expand test coverage (unit, integration, E2E)
├─ Day 47-49: Load testing and performance benchmarking
├─ Day 50-52: Security audit and penetration testing
├─ Day 53-55: SLO validation and compliance documentation
└─ Day 56:    Go-live decision (production readiness review)
```

**Total Timeline:** 8 weeks (can be compressed to 6 weeks with 2 engineers)

---

## Part 9: Migration from Dev to Production

### Step-by-Step Migration

#### Step 1: Set up DigitalOcean Infrastructure (Day 1-3)

```bash
# Install doctl CLI
brew install doctl  # or snap install doctl

# Authenticate
doctl auth init

# Create Kubernetes cluster
doctl kubernetes cluster create enclii-production \
  --region nyc3 \
  --version 1.28.2-do.0 \
  --size s-4vcpu-8gb \
  --count 3 \
  --auto-upgrade=true \
  --surge-upgrade=true \
  --maintenance-window "saturday=02:00" \
  --tag production

# Save kubeconfig
doctl kubernetes cluster kubeconfig save enclii-production

# Create managed PostgreSQL database
doctl databases create enclii-postgres \
  --engine pg \
  --version 15 \
  --size db-s-2vcpu-4gb \
  --region nyc3 \
  --num-nodes 2

# Create managed Redis
doctl databases create enclii-redis \
  --engine redis \
  --version 7 \
  --size db-s-1vcpu-2gb \
  --region nyc3

# Create Spaces bucket
doctl compute space create enclii-backups --region nyc3
```

#### Step 2: Deploy Core Infrastructure (Day 3-5)

```bash
# Clone repository
git clone https://github.com/madfam-io/enclii.git
cd enclii

# Install Sealed Secrets
kubectl apply -f https://github.com/bitnami-labs/sealed-secrets/releases/download/v0.24.0/controller.yaml

# Create production namespace
kubectl create namespace enclii-production

# Seal all secrets
./scripts/seal-secrets.sh production

# Deploy base infrastructure
kubectl apply -k infra/k8s/production

# Verify deployments
kubectl get pods -n enclii-production
```

#### Step 3: Configure DNS and TLS (Day 5-6)

```bash
# Point DNS to DigitalOcean load balancer
doctl compute load-balancer list

# Update DNS records (Cloudflare/Route53)
api.enclii.dev    → A    <load-balancer-ip>
auth.enclii.dev   → A    <load-balancer-ip>
app.enclii.dev    → A    <load-balancer-ip>

# cert-manager will auto-provision Let's Encrypt certs
kubectl get certificate -n enclii-production
```

#### Step 4: Deploy Plinto (Day 7-9)

```bash
# Apply Plinto manifests
kubectl apply -f infra/k8s/base/plinto.yaml -n enclii-production

# Initialize Plinto database
kubectl exec -it deployment/plinto -n enclii-production -- \
  python manage.py migrate

# Create first admin user
kubectl exec -it deployment/plinto -n enclii-production -- \
  python manage.py createsuperuser

# Verify Plinto is running
curl https://auth.enclii.dev/health
```

#### Step 5: Deploy Observability Stack (Day 10-14)

```bash
# Add Prometheus Operator Helm repo
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# Install kube-prometheus-stack (Prometheus + Grafana + Alertmanager)
helm install prometheus prometheus-community/kube-prometheus-stack \
  -n monitoring --create-namespace \
  -f infra/k8s/monitoring/prometheus-values.yaml

# Install Loki for logs
helm install loki grafana/loki-stack \
  -n monitoring \
  -f infra/k8s/monitoring/loki-values.yaml

# Import Grafana dashboards
kubectl apply -f infra/k8s/monitoring/dashboards/ -n monitoring

# Verify Grafana access
kubectl port-forward -n monitoring svc/prometheus-grafana 3000:80
# Open http://localhost:3000 (admin/prom-operator)
```

#### Step 6: Migrate Data from Dev to Production (Day 15-16)

```bash
# Dump development database
pg_dump enclii_dev > enclii_dev_dump.sql

# Restore to production (DigitalOcean managed PostgreSQL)
psql $PRODUCTION_DATABASE_URL < enclii_dev_dump.sql

# Migrate users to Plinto
python scripts/migrate-users-to-plinto.py \
  --source $PRODUCTION_DATABASE_URL \
  --plinto-api https://auth.enclii.dev
```

#### Step 7: Deploy Applications (Day 17-18)

```bash
# Build and push production images
make build-all
docker tag switchyard-api:latest ghcr.io/madfam/switchyard-api:v1.0.0
docker push ghcr.io/madfam/switchyard-api:v1.0.0

# Update image tags in production overlay
cd infra/k8s/production
kustomize edit set image switchyard-api=ghcr.io/madfam/switchyard-api:v1.0.0

# Deploy
kubectl apply -k infra/k8s/production

# Verify health
kubectl get pods -n enclii-production
curl https://api.enclii.dev/health
```

#### Step 8: Configure Monitoring and Alerts (Day 19-21)

```bash
# Apply PrometheusRule for alerts
kubectl apply -f infra/k8s/monitoring/alerts.yaml -n monitoring

# Configure PagerDuty integration
kubectl create secret generic pagerduty-key \
  --from-literal=key=$PAGERDUTY_INTEGRATION_KEY \
  -n monitoring

# Test alerts
kubectl apply -f infra/k8s/monitoring/test-alert.yaml
```

#### Step 9: Run Production Validation Tests (Day 22-25)

```bash
# Load testing
k6 run tests/load/production-load-test.js

# Security scanning
trivy image ghcr.io/madfam/switchyard-api:v1.0.0

# Penetration testing
docker run -t owasp/zap2docker-stable zap-baseline.py \
  -t https://api.enclii.dev

# Backup and restore test
./scripts/test-dr.sh production
```

#### Step 10: Go-Live Checklist (Day 26-28)

- [ ] All services healthy (0 CrashLoopBackOff)
- [ ] TLS certificates issued and valid
- [ ] Monitoring dashboards showing data
- [ ] Alerts routing to PagerDuty
- [ ] Backups running daily
- [ ] DR procedure tested successfully
- [ ] Load test passed (1000 RPS sustained)
- [ ] Security scan passed (no critical vulnerabilities)
- [ ] SOC 2 documentation complete
- [ ] On-call rotation configured

**🚀 Production Go-Live!**

---

## Part 10: Post-Deployment Operations

### Daily Operations

**Monitoring Dashboards:**
- Check Grafana dashboards daily for anomalies
- Review error budgets (SLO compliance)
- Verify backup completion

**Alert Response:**
- On-call engineer responds to PagerDuty alerts
- Follow incident response runbooks
- Post-mortems for all production incidents

### Weekly Operations

**Security:**
- Review audit logs for suspicious activity
- Scan for new CVEs in container images
- Rotate database credentials (automated)

**Capacity Planning:**
- Review resource utilization trends
- Adjust auto-scaling thresholds if needed
- Forecast infrastructure costs

**Disaster Recovery:**
- Test database restore from backup
- Verify snapshot integrity
- Update DR runbooks

### Monthly Operations

**Compliance:**
- Generate SOC 2 compliance reports
- Review access control matrix
- Update security policies

**Cost Optimization:**
- Review DigitalOcean billing
- Right-size resources (VPA recommendations)
- Identify unused resources

**Platform Updates:**
- Upgrade Kubernetes version (automated)
- Update Helm charts
- Patch security vulnerabilities

---

## Part 11: Risk Mitigation

### High-Risk Scenarios

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| **Database failure** | Low | Critical | Multi-AZ HA cluster, automated failover, daily backups |
| **Kubernetes cluster failure** | Low | Critical | Multi-node cluster, Pod Disruption Budgets, node auto-repair |
| **DDoS attack** | Medium | High | Rate limiting, CloudFlare proxy, WAF rules |
| **Data breach** | Low | Critical | Zero-trust networking, mTLS, audit logging, encryption at rest |
| **Secrets leak** | Medium | Critical | Sealed Secrets, secret scanning in CI, rotation automation |
| **Cost overrun** | Low | Medium | Resource quotas, budget alerts at 80%, staging = 50% of prod |
| **Third-party API outage (Plinto deps)** | Low | Medium | Caching, graceful degradation, fallback to email/password |
| **Deployment failure** | Medium | Medium | Blue-green deployments, automated rollback on errors |
| **Human error** | Medium | Medium | Admission controllers, peer review, staging environment |

### Disaster Recovery Plan

**RTO (Recovery Time Objective):** 4 hours
**RPO (Recovery Point Objective):** 1 hour

**Disaster Scenarios:**

1. **Database corruption:**
   - Restore from latest backup (automated daily)
   - Apply WAL logs for point-in-time recovery
   - Estimated recovery time: 30 minutes

2. **Entire cluster failure:**
   - Provision new DOKS cluster (10 minutes)
   - Deploy infrastructure from git (20 minutes)
   - Restore database from backup (30 minutes)
   - Verify health and switch DNS (10 minutes)
   - Total: ~70 minutes

3. **Region outage (DigitalOcean NYC3 down):**
   - Provision cluster in different region (SFO3)
   - Deploy from git with region-specific configs
   - Restore database from Spaces backup (cross-region)
   - Update DNS to new load balancer
   - Total: ~2 hours

---

## Part 12: Success Metrics

### Platform Metrics

| Metric | Current | Target | Measurement |
|--------|---------|--------|-------------|
| **Production Readiness** | 70% | 95% | Checklist completion |
| **Security Score** | 8.5/10 | 9.5/10 | Audit findings |
| **SLO Compliance (Uptime)** | N/A | 99.95% | Prometheus |
| **P95 API Latency** | N/A | <200ms | Prometheus |
| **Error Rate** | N/A | <0.1% | Prometheus |
| **Test Coverage** | 25% | 80%+ | CI pipeline |
| **Deploy Frequency** | Manual | Daily | GitHub Actions |
| **MTTR (Mean Time to Recover)** | N/A | <1 hour | Incident logs |
| **Database Backup Success Rate** | N/A | 100% | Monitoring |

### Business Metrics

| Metric | Value |
|--------|-------|
| **Infrastructure Cost Savings** | $107,100 over 5 years (vs Railway + Auth0) |
| **Time to Production** | 8 weeks (vs 6+ months with AWS) |
| **Operational Overhead** | 2-4 hours/week (after stabilization) |
| **Security Posture** | SOC 2 Type II ready |
| **Scalability** | 1000+ RPS sustained, auto-scales to 10,000+ |

---

## Part 13: Next Steps (Action Items)

### Immediate Actions (This Week)

1. **Create DigitalOcean account** - Use this link for $200 credit: https://m.do.co/c/creditcode
2. **Review and approve this roadmap** - Confirm timeline and budget
3. **Set up GitHub Actions secrets** - DIGITALOCEAN_ACCESS_TOKEN, GHCR_TOKEN
4. **Create Terraform workspace** (optional) - For infrastructure as code

### Week 1 Actions

1. **Provision DigitalOcean infrastructure:**
   - DOKS cluster (3 nodes)
   - Managed PostgreSQL HA
   - Managed Redis
   - Spaces bucket

2. **Deploy base infrastructure:**
   - Sealed Secrets controller
   - cert-manager
   - nginx-ingress
   - NetworkPolicies

3. **Configure DNS:**
   - Point `api.enclii.dev`, `auth.enclii.dev`, `app.enclii.dev` to load balancer

### Decision Points

**Before proceeding, confirm:**

- [ ] **Infrastructure choice:** DigitalOcean vs AWS vs GCP vs Hetzner?
- [ ] **Budget approval:** $435/month recurring + $2,012 one-time?
- [ ] **Timeline approval:** 8 weeks to production?
- [ ] **Plinto integration:** Self-hosted auth vs Auth0/Clerk?
- [ ] **Staffing:** 1 engineer full-time or 2 engineers part-time?
- [ ] **Security audit:** Third-party penetration test ($2,000)?

---

## Conclusion

This roadmap provides a complete path from **70% to 95%+ production readiness** in **8 weeks** with a **monthly infrastructure cost of ~$435** (vs $2,000-5,000 for managed auth platforms).

**Key advantages of this approach:**

1. ✅ **Cost-effective:** $107,100 saved over 5 years vs Railway + Auth0
2. ✅ **Fast time-to-production:** 8 weeks (vs 6+ months with AWS from scratch)
3. ✅ **Fully managed databases:** DigitalOcean handles HA, backups, failover
4. ✅ **Plinto integration:** Self-hosted auth with full control and zero per-user fees
5. ✅ **Production-grade observability:** Prometheus, Grafana, Loki, Jaeger
6. ✅ **SOC 2 compliance readiness:** Audit logging, RBAC, secrets encryption
7. ✅ **Disaster recovery tested:** Automated backups with 1-hour RPO, 4-hour RTO
8. ✅ **Scalable:** Auto-scaling to handle 1000+ RPS (10,000+ with node scaling)

**Recommended next step:** Create DigitalOcean account and start Week 1 infrastructure provisioning.

**Questions or need help?** Review the detailed implementation guides in `/home/user/enclii/infra/DEPLOYMENT.md` or reach out to the team.

---

**Document Version:** 1.0
**Last Updated:** November 20, 2025
**Author:** Claude (Enclii Platform Team)
**Status:** Ready for Review and Approval
