# Enclii - System Context

**Version:** 1.0.0
**Last Updated:** 2026-01-21

## Overview

Enclii is MADFAM's Platform-as-a-Service (PaaS) for deploying containerized applications. It provides Railway-style deployment experience on cost-effective bare metal infrastructure.

## Architecture

| Component | Port | Domain | Description |
|-----------|------|--------|-------------|
| Switchyard API | 4200 (container) / 80 (K8s service) | api.enclii.dev | Control plane API |
| Switchyard UI | 4201 (container) / 80 (K8s service) | app.enclii.dev | User dashboard |
| Dispatch | 4203 (container) / 80 (K8s service) | admin.enclii.dev | Infrastructure control tower |
| Landing Page | 80 (container) / 80 (K8s service) | enclii.dev, www.enclii.dev | Marketing site |
| Docs | 80 (container) / 80 (K8s service) | docs.enclii.dev | Documentation |

## Key Components

### Dispatch (Admin UI)
**Purpose:** Superuser interface for managing infrastructure - domains, tunnels, and ecosystem resources.

**Authorization Model:**
- Email domain must be in `ALLOWED_ADMIN_DOMAINS` (default: `@madfam.io`)
- User role must be in `ALLOWED_ADMIN_ROLES` (default: `superadmin,admin,operator`)

**Key Files:**
| Purpose | Location |
|---------|----------|
| Middleware | `apps/dispatch/middleware.ts` |
| Auth Context | `apps/dispatch/contexts/AuthContext.tsx` |
| K8s Deployment | `apps/dispatch/k8s/deployment.yaml` |

### Cloudflare Tunnel
**Purpose:** Zero-trust ingress routing all external traffic through Cloudflare.

**Configuration:** `infra/k8s/production/cloudflared-unified.yaml`

**Architecture:**
```
Internet → Cloudflare Edge → cloudflared pods → K8s Service:80 → Container:4xxx
           (TLS, DDoS)        (2 replicas)       (ClusterIP)      (targetPort)
```

## Kubernetes Resources

| Resource | Namespace | Purpose |
|----------|-----------|---------|
| `switchyard-api` | enclii | Control plane API |
| `switchyard-ui` | enclii | User dashboard |
| `dispatch` | enclii | Admin interface |
| `cloudflared` | cloudflare-tunnel | Ingress tunnel |

## Authentication

Enclii uses **Janua** for all authentication:
- `NEXT_PUBLIC_JANUA_URL`: `https://auth.madfam.io`
- OIDC flow with RS256 JWT validation

## Critical Configuration

### Dispatch Environment Variables
```yaml
ALLOWED_ADMIN_DOMAINS: "@madfam.io"
ALLOWED_ADMIN_ROLES: "superadmin,admin,operator"
NEXT_PUBLIC_JANUA_URL: "https://auth.madfam.io"
```

### Cloudflare Tunnel Routing
Key routes in `cloudflared-unified.yaml`:
- `admin.enclii.dev` → `dispatch.enclii.svc.cluster.local:80`
- `app.enclii.dev` → `switchyard-ui.enclii.svc.cluster.local:80`
- `api.enclii.dev` → `switchyard-api.enclii.svc.cluster.local:80`

## Troubleshooting

### Check Dispatch health
```bash
curl -s https://admin.enclii.dev/api/health
```

### View Dispatch logs
```bash
kubectl logs -n enclii -l app=dispatch -f
```

### Check Cloudflare tunnel
```bash
kubectl logs -n cloudflare-tunnel -l app=cloudflared -f
```

### Restart Cloudflare tunnel (after config changes)
```bash
kubectl rollout restart deployment/cloudflared -n cloudflare-tunnel
```

## Related Documentation
- [CLAUDE.md](./CLAUDE.md) - Full development guide
- [Janua System Context](/Users/aldoruizluna/labspace/janua/SYSTEM_CONTEXT.md)
- [Dhanam System Context](/Users/aldoruizluna/labspace/dhanam/SYSTEM_CONTEXT.md)
