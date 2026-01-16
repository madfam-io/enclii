# Zero-Touch Deployment Execution Guide

> **Client**: SuLuna (suluna.mx@gmail.com)
> **Workload**: LinkStack (links.suluna.mx)
> **Model**: Agency (MADFAM manages client infrastructure)

---

## Pre-Flight Checklist

### 1. Porkbun Nameserver Configuration ✅

The user must point `suluna.mx` nameservers to Cloudflare:

```
Nameserver 1: adam.ns.cloudflare.com
Nameserver 2: debbie.ns.cloudflare.com
```

**Porkbun Steps:**
1. Login to [porkbun.com](https://porkbun.com)
2. Navigate to Domain Management → `suluna.mx`
3. Click "Edit" next to Nameservers
4. Select "Custom nameservers"
5. Enter the Cloudflare nameservers above
6. Save changes

**Propagation**: Allow 15-60 minutes for DNS propagation.

---

## Required Environment Variables

Export these before running the deployment:

```bash
# ============================================================================
# Cloudflare API Configuration
# ============================================================================
# Get from: https://dash.cloudflare.com/profile/api-tokens
# Required permissions: Zone:Read, Zone:Edit, DNS:Edit
export CLOUDFLARE_API_TOKEN="your-cloudflare-api-token"

# Get from: Cloudflare Dashboard → Account Home → Account ID (right sidebar)
export CLOUDFLARE_ACCOUNT_ID="your-cloudflare-account-id"

# Get from: Cloudflare Dashboard → Zero Trust → Networks → Tunnels
# The UUID of your existing tunnel (e.g., "a1b2c3d4-e5f6-7890-abcd-ef1234567890")
export TUNNEL_ID="your-tunnel-uuid"

# ============================================================================
# Janua API Configuration
# ============================================================================
# Janua API endpoint
export JANUA_API="http://localhost:4100"  # Or production URL

# Admin JWT token for admin@madfam.io
# Get by: POST /api/v1/auth/login with admin credentials
export JANUA_ADMIN_TOKEN="your-janua-admin-jwt-token"

# ============================================================================
# Kubernetes Configuration
# ============================================================================
# Ensure kubectl is configured to the correct cluster
# Verify with: kubectl cluster-info
export KUBECONFIG="${KUBECONFIG:-$HOME/.kube/config}"

# ============================================================================
# Application Secrets
# ============================================================================
# Generate a Laravel APP_KEY for LinkStack
# Generate with: openssl rand -base64 32
export LINKSTACK_APP_KEY="$(openssl rand -base64 32)"
```

---

## Quick Export Template

Copy-paste this block and fill in your values:

```bash
# Cloudflare
export CLOUDFLARE_API_TOKEN=""
export CLOUDFLARE_ACCOUNT_ID=""
export TUNNEL_ID=""

# Janua
export JANUA_API="http://localhost:4100"
export JANUA_ADMIN_TOKEN=""

# App Secrets
export LINKSTACK_APP_KEY="$(openssl rand -base64 32)"
```

---

## Execution Command

### Full Zero-Touch Deployment

```bash
cd ~/labspace/enclii

# Make scripts executable (first time only)
chmod +x scripts/deploy-client.sh
chmod +x scripts/provision-domain.sh
chmod +x scripts/onboard-suluna.sh

# Execute the full deployment chain
./scripts/deploy-client.sh suluna
```

### What It Does (4 Phases)

| Phase | Action | Script |
|-------|--------|--------|
| 1. Identity | Create SuLuna org, roles, invites in Janua | `onboard-suluna.sh` |
| 2. Namespace | Create `suluna-production` K8s namespace | `kubectl create ns` |
| 3. Network | Cloudflare Zone, DNS, Tunnel ConfigMap | `provision-domain.sh` |
| 4. Application | Deploy LinkStack pods, service, PVC | `kubectl apply -f` |

---

## Manual Phase Execution (If Needed)

### Phase 1: Identity Only
```bash
./scripts/onboard-suluna.sh
```

### Phase 2: Namespace Only
```bash
kubectl create namespace suluna-production --dry-run=client -o yaml | kubectl apply -f -
kubectl label namespace suluna-production client=suluna managed-by=madfam
```

### Phase 3: Network Only
```bash
./scripts/provision-domain.sh \
  --domain "suluna.mx" \
  --subdomain "links" \
  --service "linkstack" \
  --namespace "suluna-production"
```

### Phase 4: Application Only
```bash
# First, update the APP_KEY secret
kubectl create secret generic linkstack-secrets \
  --namespace suluna-production \
  --from-literal=APP_KEY="base64:${LINKSTACK_APP_KEY}" \
  --dry-run=client -o yaml | kubectl apply -f -

# Deploy the application
kubectl apply -f dogfooding/clients/suluna-linkstack.k8s.yaml
```

---

## Verification Commands

### Check Deployment Status
```bash
# All resources in namespace
kubectl get all -n suluna-production

# Pod logs
kubectl logs -n suluna-production -l app=linkstack -f

# Detailed pod status
kubectl describe pod -n suluna-production -l app=linkstack
```

### Check Cloudflare Tunnel
```bash
# Verify ConfigMap has new ingress
kubectl get configmap cloudflared-config -n foundry -o yaml | grep -A5 "links.suluna.mx"

# Check cloudflared logs
kubectl logs -n foundry -l app=cloudflared --tail=50
```

### Check DNS Propagation
```bash
# DNS lookup
dig links.suluna.mx CNAME +short

# Expected output: ${TUNNEL_ID}.cfargotunnel.com.

# HTTP test (after propagation)
curl -I https://links.suluna.mx
```

### Check Janua RBAC
```bash
# List organizations
curl -H "Authorization: Bearer $JANUA_ADMIN_TOKEN" \
  "$JANUA_API/api/v1/organizations/" | jq '.[] | select(.slug=="suluna")'

# List org members
ORG_ID=$(curl -s -H "Authorization: Bearer $JANUA_ADMIN_TOKEN" \
  "$JANUA_API/api/v1/organizations/" | jq -r '.[] | select(.slug=="suluna") | .id')

curl -H "Authorization: Bearer $JANUA_ADMIN_TOKEN" \
  "$JANUA_API/api/v1/organizations/$ORG_ID/members" | jq
```

---

## Troubleshooting

### Pod Not Starting
```bash
# Check events
kubectl get events -n suluna-production --sort-by='.lastTimestamp'

# Check PVC status (Longhorn must be available)
kubectl get pvc -n suluna-production
```

### DNS Not Resolving
```bash
# Check zone status in Cloudflare
curl -s -X GET "https://api.cloudflare.com/client/v4/zones?name=suluna.mx" \
  -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" | jq '.result[0].status'

# Should return: "active"
# If "pending", nameservers haven't propagated yet
```

### Tunnel Not Routing
```bash
# Restart cloudflared to pick up ConfigMap changes
kubectl rollout restart deployment/cloudflared -n foundry

# Check tunnel health
kubectl exec -n foundry -it deploy/cloudflared -- cloudflared tunnel info
```

### Janua API Errors
```bash
# Check if Janua is running
curl -s "$JANUA_API/health" | jq

# Test auth
curl -s -H "Authorization: Bearer $JANUA_ADMIN_TOKEN" \
  "$JANUA_API/api/v1/users/me" | jq
```

---

## Post-Deployment Steps

### 1. Client Onboarding Email

Send to `suluna.mx@gmail.com`:

```
Subject: Your LinkStack Instance is Ready!

Hi SuLuna,

Your self-hosted LinkStack is now live at:
https://links.suluna.mx

You've been invited to the SuLuna organization in our management portal.
Check your email for the invitation link.

Login to manage your account:
- Profile settings
- Custom themes
- Link analytics

Support: admin@madfam.io

- MADFAM Team
```

### 2. Agency Model Verification

Login as `admin@madfam.io` and verify:
- [ ] Can access SuLuna organization
- [ ] Has Admin role via `Managed_Services`
- [ ] Can view infrastructure without client credentials

### 3. Monitoring Setup (Optional)

```bash
# Add Prometheus ServiceMonitor if monitoring is enabled
kubectl apply -f - <<EOF
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: linkstack
  namespace: suluna-production
  labels:
    app: linkstack
spec:
  selector:
    matchLabels:
      app: linkstack
  endpoints:
    - port: http
      path: /metrics
      interval: 30s
EOF
```

---

## Files Created

| File | Purpose |
|------|---------|
| `scripts/provision-domain.sh` | Cloudflare Zone/DNS/Tunnel automation |
| `scripts/deploy-client.sh` | Master deployment orchestrator |
| `scripts/onboard-suluna.sh` | Janua RBAC setup |
| `dogfooding/clients/suluna-linkstack.yaml` | Enclii service spec |
| `dogfooding/clients/suluna-linkstack.k8s.yaml` | Raw K8s manifest |
| `docs/guides/AGENCY_MODEL_DEPLOYMENT.md` | Full deployment guide |
| `docs/guides/ZERO_TOUCH_EXECUTION.md` | This file |

---

*Zero-Touch Deployment | Agency Model Validation | MADFAM Platform*
