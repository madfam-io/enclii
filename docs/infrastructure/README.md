# Infrastructure Documentation

**Last Updated:** January 2026

This section documents Enclii's production infrastructure components deployed in January 2026.

> **Current State:** Running on a single Hetzner AX41-NVME dedicated server. Infrastructure (Longhorn, ArgoCD) is prepared for multi-node scaling when additional nodes are added.

## Contents

| Document | Description |
|----------|-------------|
| [GitOps with ArgoCD](./GITOPS.md) | GitOps deployment management using App-of-Apps pattern |
| [Storage with Longhorn](./STORAGE.md) | Block storage (single-node; prepared for multi-node scaling) |
| [Cloudflare Integration](./CLOUDFLARE.md) | Zero-trust ingress, tunnel route automation, DNS |
| [External Secrets](./EXTERNAL_SECRETS.md) | Secret synchronization from external providers |

## Quick Reference

### Check Infrastructure Health

```bash
# ArgoCD sync status
kubectl get applications -n argocd

# Longhorn volumes
kubectl get volumes.longhorn.io -n longhorn-system

# Cloudflare tunnel
kubectl get pods -n cloudflare-tunnel

# External secrets sync
kubectl get externalsecrets -n enclii
```

### Access UIs

```bash
# ArgoCD (https://localhost:8080)
kubectl port-forward svc/argocd-server -n argocd 8080:443

# Longhorn (http://localhost:8081)
kubectl port-forward svc/longhorn-frontend -n longhorn-system 8081:80
```

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                      Cloudflare Edge                             │
│              (TLS, DDoS, WAF, Global Load Balancing)             │
└─────────────────────────────────────────────────────────────────┘
                              │
                     Cloudflare Tunnel
                              │
┌─────────────────────────────────────────────────────────────────┐
│                   Kubernetes Cluster (k3s)                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │
│  │   ArgoCD    │  │  Longhorn   │  │  External   │              │
│  │   GitOps    │  │   Storage   │  │   Secrets   │              │
│  └─────────────┘  └─────────────┘  └─────────────┘              │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                    Enclii Services                          ││
│  │  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌──────────┐ ││
│  │  │    API    │  │    UI     │  │   Docs    │  │  Janua   │ ││
│  │  │  :4200    │  │  :4201    │  │  :4203    │  │  (SSO)   │ ││
│  │  └───────────┘  └───────────┘  └───────────┘  └──────────┘ ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                              │
                    ┌─────────┴─────────┐
                    ▼                   ▼
           ┌─────────────┐      ┌─────────────┐
           │ PostgreSQL  │      │    Redis    │
           │ (In-cluster)│      │ (In-cluster)│
           └─────────────┘      └─────────────┘
```

## Related Documentation

- [Deployment Guide](../../infra/DEPLOYMENT.md)
- [Production Checklist](../production/PRODUCTION_CHECKLIST.md)
- [Production Roadmap](../production/PRODUCTION_DEPLOYMENT_ROADMAP.md)
- [Architecture Overview](../architecture/ARCHITECTURE.md)
