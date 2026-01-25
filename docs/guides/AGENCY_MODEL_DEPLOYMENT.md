# Agency Model Deployment Guide: SuLuna LinkStack

> **First Multi-Tenant Customer Deployment**
> This guide validates the Agency Model where MADFAM manages client infrastructure without sharing credentials.

**Client:** SuLuna (suluna.mx@gmail.com)
**Service:** LinkStack (self-hosted Linktree alternative)
**Domain:** links.suluna.mx

---

## Overview

### The Agency Model

```
┌────────────────────────────────────────────────────────────┐
│                    MADFAM Platform                          │
│  ┌─────────────────┐     ┌─────────────────┐              │
│  │  admin@madfam.io │◄────│    Janua SSO    │              │
│  │  (Platform Admin)│     │   (auth.madfam.io)             │
│  └────────┬────────┘     └─────────────────┘              │
│           │                                                │
│           │ "Switch Scope" to SuLuna Org                  │
│           ▼                                                │
│  ┌────────────────────────────────────────────────────────┐│
│  │                SuLuna Organization                      ││
│  │  ┌─────────────────┐   ┌─────────────────┐            ││
│  │  │ admin@madfam.io │   │ suluna.mx@gmail │            ││
│  │  │ (Managed_Services│   │ (Owner)         │            ││
│  │  │  Team - Admin)   │   │                 │            ││
│  │  └────────┬────────┘   └────────┬────────┘            ││
│  │           │                      │                      ││
│  │           ▼                      ▼                      ││
│  │  ┌─────────────────────────────────────────────┐       ││
│  │  │           LinkStack Container               │       ││
│  │  │  namespace: suluna-production               │       ││
│  │  │  domain: links.suluna.mx                    │       ││
│  │  └─────────────────────────────────────────────┘       ││
│  └────────────────────────────────────────────────────────┘│
└────────────────────────────────────────────────────────────┘
```

**Key Insight:** admin@madfam.io can manage SuLuna's infrastructure through the `Managed_Services` custom role without needing their login credentials.

---

## Deliverable 1: RBAC Script (Janua Configuration)

### Prerequisites

```bash
# Set your admin JWT token (get from Janua dashboard or API login)
export JANUA_API="https://auth.madfam.io"
export ADMIN_TOKEN="your-admin-jwt-token"

# Verify connection
curl -s -H "Authorization: Bearer $ADMIN_TOKEN" \
  "$JANUA_API/api/v1/auth/me" | jq '.email'
# Should return: "admin@madfam.io"
```

### Step 1: Create SuLuna Organization

```bash
# Create the organization
curl -X POST "$JANUA_API/api/v1/organizations/" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "SuLuna",
    "slug": "suluna",
    "description": "SuLuna - Managed Services Client",
    "billing_email": "suluna.mx@gmail.com"
  }' | jq

# Response includes organization ID - save it
export SULUNA_ORG_ID="<organization-id-from-response>"
```

### Step 2: Create Managed_Services Custom Role

```bash
# Create custom role with admin permissions for managed services
curl -X POST "$JANUA_API/api/v1/organizations/$SULUNA_ORG_ID/roles" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Managed_Services",
    "description": "MADFAM managed services team - full infrastructure access",
    "permissions": [
      "services:read",
      "services:write",
      "services:deploy",
      "services:delete",
      "logs:read",
      "metrics:read",
      "secrets:read",
      "secrets:write",
      "domains:manage",
      "billing:read"
    ]
  }' | jq

# Save the role ID
export MANAGED_SERVICES_ROLE_ID="<role-id-from-response>"
```

### Step 3: Invite Client Owner

```bash
# Invite the client as Organization Owner
curl -X POST "$JANUA_API/api/v1/organizations/$SULUNA_ORG_ID/invite" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "suluna.mx@gmail.com",
    "role": "owner",
    "permissions": [],
    "message": "Welcome to your SuLuna dashboard! You have full ownership of your organization. MADFAM provides managed services for your infrastructure."
  }' | jq

# Note: Client receives email to accept invitation
```

### Step 4: Add admin@madfam.io to Managed_Services

Since admin@madfam.io created the organization, they're already a member. Update their role:

```bash
# Get admin@madfam.io user ID
export ADMIN_USER_ID=$(curl -s -H "Authorization: Bearer $ADMIN_TOKEN" \
  "$JANUA_API/api/v1/auth/me" | jq -r '.id')

# Update role to use custom Managed_Services role
# Note: The API uses built-in roles (admin, member, viewer)
# For custom roles, we use the permissions system
curl -X PUT "$JANUA_API/api/v1/organizations/$SULUNA_ORG_ID/members/$ADMIN_USER_ID/role?role=admin" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq

# This gives admin@madfam.io full admin access to manage the org
```

### Complete RBAC Setup Script

Save this as `scripts/onboard-suluna.sh`:

```bash
#!/bin/bash
set -euo pipefail

# =============================================================================
# SuLuna Client Onboarding Script
# Agency Model: MADFAM manages client infrastructure
# =============================================================================

JANUA_API="${JANUA_API:-https://auth.madfam.io}"
CLIENT_EMAIL="suluna.mx@gmail.com"
CLIENT_ORG_NAME="SuLuna"
CLIENT_ORG_SLUG="suluna"

echo "🔐 SuLuna Client Onboarding"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Check for admin token
if [ -z "${ADMIN_TOKEN:-}" ]; then
    echo "❌ ADMIN_TOKEN environment variable required"
    echo "   Get your token: curl -X POST $JANUA_API/api/v1/auth/login ..."
    exit 1
fi

# Verify admin auth
echo "📋 Verifying admin credentials..."
ADMIN_EMAIL=$(curl -sf -H "Authorization: Bearer $ADMIN_TOKEN" \
  "$JANUA_API/api/v1/auth/me" | jq -r '.email')
ADMIN_USER_ID=$(curl -sf -H "Authorization: Bearer $ADMIN_TOKEN" \
  "$JANUA_API/api/v1/auth/me" | jq -r '.id')

if [ "$ADMIN_EMAIL" != "admin@madfam.io" ]; then
    echo "❌ Expected admin@madfam.io, got: $ADMIN_EMAIL"
    exit 1
fi
echo "✅ Authenticated as: $ADMIN_EMAIL"

# Step 1: Create Organization
echo ""
echo "📁 Creating $CLIENT_ORG_NAME organization..."
ORG_RESPONSE=$(curl -sf -X POST "$JANUA_API/api/v1/organizations/" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"$CLIENT_ORG_NAME\",
    \"slug\": \"$CLIENT_ORG_SLUG\",
    \"description\": \"$CLIENT_ORG_NAME - Managed Services Client\",
    \"billing_email\": \"$CLIENT_EMAIL\"
  }")

ORG_ID=$(echo "$ORG_RESPONSE" | jq -r '.id')
echo "✅ Organization created: $ORG_ID"

# Step 2: Create Managed_Services Role
echo ""
echo "👥 Creating Managed_Services custom role..."
ROLE_RESPONSE=$(curl -sf -X POST "$JANUA_API/api/v1/organizations/$ORG_ID/roles" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Managed_Services",
    "description": "MADFAM managed services team - full infrastructure access",
    "permissions": [
      "services:read", "services:write", "services:deploy", "services:delete",
      "logs:read", "metrics:read", "secrets:read", "secrets:write",
      "domains:manage", "billing:read"
    ]
  }')

ROLE_ID=$(echo "$ROLE_RESPONSE" | jq -r '.id')
echo "✅ Managed_Services role created: $ROLE_ID"

# Step 3: Invite Client Owner
echo ""
echo "📧 Inviting $CLIENT_EMAIL as owner..."
INVITE_RESPONSE=$(curl -sf -X POST "$JANUA_API/api/v1/organizations/$ORG_ID/invite" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"$CLIENT_EMAIL\",
    \"role\": \"owner\",
    \"permissions\": [],
    \"message\": \"Welcome to your $CLIENT_ORG_NAME dashboard! You have full ownership of your organization. MADFAM provides managed services for your infrastructure.\"
  }")

INVITE_ID=$(echo "$INVITE_RESPONSE" | jq -r '.invitation_id')
echo "✅ Invitation sent: $INVITE_ID"

# Summary
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🎉 SuLuna Onboarding Complete!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Organization ID: $ORG_ID"
echo "Organization Slug: $CLIENT_ORG_SLUG"
echo "Managed_Services Role ID: $ROLE_ID"
echo "Client Invitation ID: $INVITE_ID"
echo ""
echo "Next Steps:"
echo "1. Client accepts invitation at their email"
echo "2. Deploy LinkStack with: enclii service create --file dogfooding/clients/suluna-linkstack.yaml"
echo "3. Configure domain: links.suluna.mx"
echo ""
echo "Environment variables for deployment:"
echo "export SULUNA_ORG_ID=$ORG_ID"
echo "export SULUNA_NAMESPACE=suluna-production"
```

---

## Deliverable 2: LinkStack Deployment Manifest

Save as `dogfooding/clients/suluna-linkstack.yaml`:

```yaml
# LinkStack Service Specification for SuLuna
# Self-hosted Linktree alternative for client deployment
#
# Repository: https://github.com/LinkStackOrg/linkstack-docker
# License: AGPL-3.0
#
# Agency Model: Deployed in client namespace, managed by MADFAM
# Client: SuLuna (suluna.mx@gmail.com)

apiVersion: enclii.io/v1
kind: Service
metadata:
  name: linkstack
  namespace: suluna-production  # Client-scoped namespace
  labels:
    app: linkstack
    tier: client-application
    client: suluna
    criticality: medium
  annotations:
    enclii.dev/description: "LinkStack - Self-hosted link sharing for SuLuna"
    enclii.dev/owner: managed-services
    enclii.dev/client: suluna
    enclii.dev/billing: suluna-org

spec:
  # ==========================================================================
  # SOURCE CONFIGURATION
  # ==========================================================================
  source:
    # Use official LinkStack Docker image
    image:
      repository: linkstackorg/linkstack
      tag: latest
      pullPolicy: Always

  # ==========================================================================
  # DEPLOYMENT CONFIGURATION
  # ==========================================================================
  deployment:
    replicas: 1  # Single replica for cost efficiency

    strategy:
      type: RollingUpdate
      rollingUpdate:
        maxSurge: 1
        maxUnavailable: 0

    # Container configuration
    container:
      port: 80

      # Health checks
      healthCheck:
        path: /
        port: 80
        initialDelaySeconds: 30
        periodSeconds: 15
        timeoutSeconds: 5
        failureThreshold: 3
        successThreshold: 1

      # Readiness check
      readinessCheck:
        path: /
        port: 80
        initialDelaySeconds: 10
        periodSeconds: 10
        timeoutSeconds: 3
        failureThreshold: 2

      # Resource limits (PHP/Laravel is memory-hungry)
      resources:
        requests:
          cpu: "100m"
          memory: "256Mi"
        limits:
          cpu: "500m"
          memory: "512Mi"

      # Security context
      securityContext:
        runAsNonRoot: false  # LinkStack needs root for Apache
        readOnlyRootFilesystem: false  # Needs write for SQLite
        allowPrivilegeEscalation: false

    # Pod configuration
    pod:
      labels:
        app: linkstack
        client: suluna

  # ==========================================================================
  # NETWORKING
  # ==========================================================================
  networking:
    # Client domain
    domains:
      - name: links.suluna.mx
        primary: true
        tls:
          enabled: true
          provider: cloudflare

    # Internal service
    service:
      type: ClusterIP
      port: 80
      targetPort: 80

    # Ingress rules
    ingress:
      enabled: true
      annotations:
        nginx.ingress.kubernetes.io/proxy-body-size: "20m"
        nginx.ingress.kubernetes.io/configuration-snippet: |
          add_header X-Frame-Options "SAMEORIGIN" always;
          add_header X-Content-Type-Options "nosniff" always;
          add_header X-XSS-Protection "1; mode=block" always;

      # Rate limiting
      rateLimit:
        enabled: true
        requestsPerMinute: 120
        burst: 30

  # ==========================================================================
  # ENVIRONMENT CONFIGURATION
  # ==========================================================================
  env:
    # Application settings
    - name: SERVER_ADMIN
      value: "admin@madfam.io"

    # LinkStack specific
    - name: HTTP_SERVER_NAME
      value: "links.suluna.mx"

    - name: HTTPS_SERVER_NAME
      value: "links.suluna.mx"

    - name: LOG_CHANNEL
      value: "stderr"

    - name: TZ
      value: "America/Mexico_City"

    # PHP tuning
    - name: PHP_MEMORY_LIMIT
      value: "256M"

    - name: UPLOAD_MAX_FILESIZE
      value: "20M"

  # ==========================================================================
  # PERSISTENT STORAGE
  # ==========================================================================
  volumes:
    # SQLite database + user uploads
    - name: linkstack-data
      mountPath: /htdocs
      persistentVolumeClaim:
        storageClass: longhorn  # Enclii's default storage
        accessMode: ReadWriteOnce
        size: 5Gi
        labels:
          client: suluna
          backup: enabled

  # ==========================================================================
  # DEPLOYMENT PIPELINE
  # ==========================================================================
  pipeline:
    # Manual deployment (not auto-deploy from git)
    autoDeploy: false

    stages:
      - name: deploy
        strategy: rolling
        rollbackOnFailure: true

    webhooks:
      onSuccess:
        - url: "${SLACK_WEBHOOK_URL}"
          payload: |
            {
              "text": "✅ LinkStack deployed for SuLuna",
              "attachments": [{
                "color": "good",
                "fields": [
                  {"title": "Client", "value": "SuLuna", "short": true},
                  {"title": "Domain", "value": "links.suluna.mx", "short": true}
                ]
              }]
            }

  # ==========================================================================
  # MONITORING
  # ==========================================================================
  monitoring:
    # Basic uptime monitoring
    healthEndpoint:
      enabled: true
      path: /
      interval: 60

    alerts:
      - name: LinkStackSuLunaDown
        condition: up{service="linkstack",namespace="suluna-production"} == 0
        duration: 5m
        severity: warning
        annotations:
          summary: "SuLuna LinkStack is down"
          description: "LinkStack instance for SuLuna has been unavailable for 5 minutes"

  # ==========================================================================
  # BACKUP CONFIGURATION
  # ==========================================================================
  backup:
    enabled: true
    schedule: "0 3 * * *"  # Daily at 3 AM
    retention: 7  # Keep 7 days of backups
    destination:
      type: r2
      bucket: suluna-backups
      path: linkstack/

  # ==========================================================================
  # CLIENT ISOLATION
  # ==========================================================================
  isolation:
    # Network policies - only allow traffic from Cloudflare Tunnel
    networkPolicy:
      enabled: true
      ingress:
        - from:
            - namespaceSelector:
                matchLabels:
                  name: cloudflare-tunnel
          ports:
            - port: 80
              protocol: TCP

    # Resource quotas for the namespace
    resourceQuota:
      enabled: true
      limits:
        cpu: "2"
        memory: "2Gi"
        persistentVolumeClaims: "3"
        services: "5"
```

### Cloudflare Tunnel Configuration

Add to `infra/k8s/production/cloudflared-unified.yaml`:

```yaml
# In the ingress section, add:

      # ============================================
      # Client Services (namespace: suluna-production)
      # ============================================

      # SuLuna LinkStack
      - hostname: links.suluna.mx
        service: http://linkstack.suluna-production.svc.cluster.local:80
        originRequest:
          connectTimeout: 10s
          httpHostHeader: links.suluna.mx
```

---

## Deliverable 3: Agency Onboarding Checklist

### 3-Step Client Onboarding

```
┌─────────────────────────────────────────────────────────────────┐
│                 AGENCY MODEL ONBOARDING                         │
│                    SuLuna LinkStack                             │
└─────────────────────────────────────────────────────────────────┘

□ STEP 1: JANUA CONFIGURATION (5 minutes)
  ├─ □ Run: ./scripts/onboard-suluna.sh
  ├─ □ Verify org created: curl $JANUA_API/api/v1/organizations/
  ├─ □ Confirm invitation sent to suluna.mx@gmail.com
  └─ □ Document org ID: _____________________

□ STEP 2: ENCLII DEPLOYMENT (10 minutes)
  ├─ □ Create namespace: kubectl create ns suluna-production
  ├─ □ Deploy service: enclii service create --file dogfooding/clients/suluna-linkstack.yaml
  ├─ □ Add tunnel route: Update cloudflared-unified.yaml
  ├─ □ Apply tunnel: kubectl apply -f infra/k8s/production/cloudflared-unified.yaml
  └─ □ Verify pod running: kubectl get pods -n suluna-production

□ STEP 3: VERIFICATION (5 minutes)
  ├─ □ Visit https://links.suluna.mx - should show LinkStack setup
  ├─ □ Log into app.enclii.dev as admin@madfam.io
  ├─ □ Switch scope to "SuLuna" organization
  ├─ □ Verify ONLY LinkStack appears (no MADFAM services)
  └─ □ Document: Client can see their own services only

═══════════════════════════════════════════════════════════════════
COMPLETION CRITERIA:
  ✓ Client received invitation email
  ✓ LinkStack accessible at links.suluna.mx
  ✓ admin@madfam.io can manage via dashboard scope switch
  ✓ Client cannot see MADFAM internal services
═══════════════════════════════════════════════════════════════════
```

---

## Phase 3: Separation of Concerns Verification

### Test Scenarios

#### Scenario A: Admin View (admin@madfam.io)

1. **Login** to https://app.enclii.dev
2. **Default Scope**: Should show MADFAM internal services (Janua, Enclii, etc.)
3. **Switch Scope**: Click organization dropdown → Select "SuLuna"
4. **SuLuna View**: Should show ONLY:
   - `linkstack` service in `suluna-production` namespace
   - No Janua, Enclii, or other MADFAM services

#### Scenario B: Client View (suluna.mx@gmail.com)

1. **Accept Invitation** via email
2. **Login** to https://app.enclii.dev (or app.janua.dev)
3. **Default Scope**: Only "SuLuna" organization available
4. **Dashboard**: Shows ONLY their LinkStack service
5. **No Access**: Cannot see or switch to MADFAM organization

### Verification Commands

```bash
# As admin@madfam.io - List orgs (should see multiple)
curl -s -H "Authorization: Bearer $ADMIN_TOKEN" \
  "$JANUA_API/api/v1/organizations/" | jq '.[].name'
# Expected: "MADFAM", "SuLuna", ...

# As admin@madfam.io - List SuLuna services only
curl -s -H "Authorization: Bearer $ADMIN_TOKEN" \
  "$ENCLII_API/api/v1/organizations/$SULUNA_ORG_ID/services" | jq '.[].name'
# Expected: "linkstack"

# As client - List services (should only see their own)
curl -s -H "Authorization: Bearer $CLIENT_TOKEN" \
  "$ENCLII_API/api/v1/services" | jq '.[].name'
# Expected: "linkstack" (nothing else)
```

---

## Summary

### What This Proves

1. **Multi-Tenancy Works**: Client organizations are isolated
2. **Agency Model Works**: MADFAM can manage without credentials
3. **Scope Switching Works**: Dashboard shows correct services per org
4. **Billing Separation Works**: Each org has independent billing

### Cost for Client

- **LinkStack**: ~$5/month (0.5 vCPU, 512MB RAM)
- **Storage**: ~$1/month (5GB Longhorn)
- **Bandwidth**: ~$0 (Cloudflare free tier)
- **Total**: ~$6/month managed service

### Next Steps

1. ✅ Create SuLuna organization in Janua
2. ✅ Deploy LinkStack to Enclii
3. ⏳ Client accepts invitation and configures LinkStack
4. ⏳ Document in client success stories for sales

---

*Agency Model Validated | First Multi-Tenant Customer | Enclii + Janua*
