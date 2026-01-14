# Cloudflare Integration

**Last Updated:** January 2026
**Status:** Operational

---

## Overview

Enclii uses Cloudflare for zero-trust ingress via Cloudflare Tunnel, DNS management, and multi-tenant SSL via Cloudflare for SaaS. This provides enterprise-grade security without exposing cluster nodes to the public internet.

## Architecture

```
Internet
    │
    ▼
┌─────────────────────────────────────────┐
│       Cloudflare Edge Network            │
│  ┌─────────────────────────────────────┐ │
│  │ • TLS Termination                   │ │
│  │ • DDoS Protection                   │ │
│  │ • WAF Rules                         │ │
│  │ • Geographic Load Balancing         │ │
│  └─────────────────────────────────────┘ │
└─────────────────────────────────────────┘
                    │
            Encrypted Tunnel
                    │
                    ▼
┌─────────────────────────────────────────┐
│         cloudflared pods (2 replicas)    │
│         (cloudflare-tunnel namespace)    │
└─────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────┐
│     Kubernetes Services (ClusterIP)      │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  │
│  │api:80   │  │app:80   │  │docs:80  │  │
│  └─────────┘  └─────────┘  └─────────┘  │
└─────────────────────────────────────────┘
```

## Components

### 1. Cloudflare Tunnel

Zero-trust ingress that replaces traditional load balancers.

**Benefits:**
- No public IPs needed on nodes
- No firewall port configuration
- Built-in DDoS protection
- Automatic failover

**Configuration:** `infra/k8s/production/cloudflared-unified.yaml`

### 2. Tunnel Route Automation

Routes are managed via Cloudflare API, not ConfigMap.

**Source Code:**
- Client: `apps/switchyard-api/internal/cloudflare/`
- Service: `apps/switchyard-api/internal/services/tunnel_routes_cloudflare.go`
- Types: `apps/switchyard-api/internal/cloudflare/types.go`

### 3. Cloudflare for SaaS

Multi-tenant SSL for custom domains.

- First 100 custom domains: **FREE**
- Additional: $0.10/domain/month
- Automatic SSL provisioning (~30 seconds)

## Credentials

Stored in Kubernetes secret: `enclii-cloudflare-credentials`

| Key | Description |
|-----|-------------|
| `api-token` | Cloudflare API token with Zone/Tunnel permissions |
| `account-id` | Cloudflare account identifier |
| `zone-id` | Zone identifier for enclii.dev |
| `tunnel-id` | Tunnel identifier |

### Environment Variables

```yaml
env:
  - name: ENCLII_CLOUDFLARE_API_TOKEN
    valueFrom:
      secretKeyRef:
        name: enclii-cloudflare-credentials
        key: api-token
  - name: ENCLII_CLOUDFLARE_ACCOUNT_ID
    valueFrom:
      secretKeyRef:
        name: enclii-cloudflare-credentials
        key: account-id
  - name: ENCLII_CLOUDFLARE_ZONE_ID
    valueFrom:
      secretKeyRef:
        name: enclii-cloudflare-credentials
        key: zone-id
  - name: ENCLII_CLOUDFLARE_TUNNEL_ID
    valueFrom:
      secretKeyRef:
        name: enclii-cloudflare-credentials
        key: tunnel-id
```

## Route Automation

### How It Works

When `enclii domains add` is called:

1. **Domain record created** in database
2. **TunnelRoutesServiceCloudflare.AddRoute()** called
3. **Cloudflare API** updates tunnel configuration
4. **No pod restart needed** (API-based, not ConfigMap)

### API Flow

```go
// apps/switchyard-api/internal/services/tunnel_routes_cloudflare.go
func (s *TunnelRoutesServiceCloudflare) AddRoute(ctx context.Context, spec *RouteSpec) error {
    // 1. Get current configuration from Cloudflare API
    config, err := s.cfClient.GetTunnelConfiguration(ctx, s.tunnelID)

    // 2. Check if route exists, update or insert
    // 3. Call Cloudflare API to update configuration
    err = s.cfClient.UpdateTunnelConfiguration(ctx, s.tunnelID, config)

    // No restart needed - changes are immediate
}
```

### Route Structure

```yaml
# Cloudflare Tunnel ingress configuration
ingress:
  - hostname: api.enclii.dev
    service: http://switchyard-api.enclii.svc.cluster.local:80
  - hostname: app.enclii.dev
    service: http://switchyard-ui.enclii.svc.cluster.local:80
  - hostname: docs.enclii.dev
    service: http://docs-site.enclii.svc.cluster.local:80
  - service: http_status:404  # Catch-all (must be last)
```

## Tunnel Configuration

### Production Tunnels

| Tunnel | ID | Purpose |
|--------|-----|---------|
| enclii-prod | c9fac286-497b-4aac-9288-f784a1ea561c | Enclii services |
| janua-prod | 803de96d-30f4-4d2b-8283-22fe939d4ee7 | Auth services |

### Port Mapping

**Critical:** Cloudflare routes to K8s Service port (80), NOT container port.

```
Cloudflare Route → K8s Service:80 → Container:4xxx
                    (ClusterIP)     (targetPort)
```

| Service | Container Port | Service Port | Route Target |
|---------|---------------|--------------|--------------|
| switchyard-api | 4200 | 80 | http://svc:80 |
| switchyard-ui | 4201 | 80 | http://svc:80 |
| docs-site | 4203 | 80 | http://svc:80 |

## Operations

### Check Tunnel Status

```bash
# Check cloudflared pods
kubectl get pods -n cloudflare-tunnel

# View tunnel connections
kubectl logs -n cloudflare-tunnel -l app=cloudflared -f

# Check tunnel health via API
curl -s https://api.cloudflare.com/client/v4/accounts/{account_id}/cfd_tunnel/{tunnel_id} \
  -H "Authorization: Bearer $CF_TOKEN" | jq '.result.status'
```

### List Routes

```bash
# Via Cloudflare API
curl -s https://api.cloudflare.com/client/v4/accounts/{account_id}/cfd_tunnel/{tunnel_id}/configurations \
  -H "Authorization: Bearer $CF_TOKEN" | jq '.result.config.ingress'
```

### Add Route Manually

```bash
# Via CLI (recommended)
enclii domains add myapp.enclii.dev --service myapp --namespace default --port 80

# Via API directly
curl -X PUT "https://api.cloudflare.com/client/v4/accounts/{account_id}/cfd_tunnel/{tunnel_id}/configurations" \
  -H "Authorization: Bearer $CF_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{...updated config...}'
```

### Remove Route

```bash
# Via CLI
enclii domains remove myapp.enclii.dev
```

## Troubleshooting

### Tunnel Not Connected

```bash
# Check pod status
kubectl get pods -n cloudflare-tunnel

# Check pod logs for errors
kubectl logs -n cloudflare-tunnel -l app=cloudflared --tail=50

# Verify credentials
kubectl get secret enclii-cloudflare-credentials -n enclii -o yaml
```

### Route Not Working

```bash
# Verify route exists in Cloudflare
curl -s "https://api.cloudflare.com/client/v4/accounts/{account_id}/cfd_tunnel/{tunnel_id}/configurations" \
  -H "Authorization: Bearer $CF_TOKEN" | jq '.result.config.ingress[] | select(.hostname=="<hostname>")'

# Check if service exists
kubectl get svc -n enclii

# Test service connectivity from cloudflared pod
kubectl exec -n cloudflare-tunnel <cloudflared-pod> -- \
  curl -s http://switchyard-api.enclii.svc.cluster.local:80/health
```

### SSL Certificate Issues

For Cloudflare for SaaS custom domains:

```bash
# Check custom hostname status
curl -s "https://api.cloudflare.com/client/v4/zones/{zone_id}/custom_hostnames?hostname=<hostname>" \
  -H "Authorization: Bearer $CF_TOKEN" | jq '.result[0].ssl.status'

# Expected: "active"
# If "pending_validation", customer needs to add CNAME record
```

## DNS Configuration

### Zone Records

| Record | Type | Target | Proxy |
|--------|------|--------|-------|
| api.enclii.dev | CNAME | <tunnel-id>.cfargotunnel.com | Proxied |
| app.enclii.dev | CNAME | <tunnel-id>.cfargotunnel.com | Proxied |
| docs.enclii.dev | CNAME | <tunnel-id>.cfargotunnel.com | Proxied |

### Customer Custom Domains

Customers add CNAME record pointing to fallback origin:

```
customer-app.customer-domain.com → proxy.enclii.dev
```

SSL auto-provisions via Cloudflare for SaaS.

## Security

### Zero-Trust Architecture

- No public IPs on cluster nodes
- All traffic through encrypted tunnel
- DDoS protection at Cloudflare edge
- WAF rules applied before reaching cluster

### API Token Permissions

Required scopes:
- Zone:Read
- Zone:Edit (for custom hostnames)
- Account:Cloudflare Tunnel:Read
- Account:Cloudflare Tunnel:Edit

## Related Documentation

- [GitOps with ArgoCD](./GITOPS.md)
- [Storage with Longhorn](./STORAGE.md)
- [Deployment Guide](../../infra/DEPLOYMENT.md)
- [Production Deployment Roadmap](../production/PRODUCTION_DEPLOYMENT_ROADMAP.md)

## Verification

```bash
# Verify tunnel is healthy
kubectl get pods -n cloudflare-tunnel

# Expected:
NAME                           READY   STATUS    RESTARTS
cloudflared-xxxxxxxxxx-xxxxx   1/1     Running   0
cloudflared-xxxxxxxxxx-xxxxx   1/1     Running   0

# Verify API integration
curl https://api.enclii.dev/health

# Expected:
{"service":"switchyard-api","status":"healthy","version":"0.1.0"}

# Verify Cloudflare client in API logs
kubectl logs -n enclii -l app=switchyard-api --tail=20 | grep -i cloudflare

# Expected:
level=info msg="Cloudflare API client initialized"
```
