# GitOps with ArgoCD

**Last Updated:** January 2026
**Status:** Operational

---

## Overview

Enclii uses ArgoCD for GitOps-based deployment management. All infrastructure and application deployments are declaratively defined in Git and automatically synchronized to the cluster.

## Architecture

```
Git Repository (infra/argocd/)
         │
         ▼
    ArgoCD Server
    (argocd namespace)
         │
    ┌────┴────┐
    ▼         ▼
Root App   Self-heal
    │         │
    ▼         ▼
Child Apps   Drift
(per-service) correction
```

### App-of-Apps Pattern

ArgoCD follows the App-of-Apps pattern:

1. **Root Application** (`infra/argocd/root-application.yaml`)
   - Manages all child applications
   - Points to `infra/argocd/apps/` directory

2. **Child Applications** (`infra/argocd/apps/*.yaml`)
   - One Application per service/component
   - Each defines source, destination, sync policy

## Configuration

### Directory Structure

```
infra/argocd/
├── root-application.yaml    # Root app definition
├── apps/
│   ├── switchyard-api.yaml  # API application
│   ├── switchyard-ui.yaml   # Web UI application
│   ├── docs-site.yaml       # Documentation site
│   ├── longhorn.yaml        # Storage system
│   └── cloudflared.yaml     # Tunnel ingress
└── README.md
```

### Root Application

```yaml
# infra/argocd/root-application.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: root
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/madfam-org/enclii
    targetRevision: main
    path: infra/argocd/apps
  destination:
    server: https://kubernetes.default.svc
    namespace: argocd
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

### Child Application Template

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: switchyard-api
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/madfam-org/enclii
    targetRevision: main
    path: infra/k8s/production
  destination:
    server: https://kubernetes.default.svc
    namespace: enclii
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
```

## Operations

### Accessing ArgoCD UI

```bash
# Port forward to ArgoCD server
kubectl port-forward svc/argocd-server -n argocd 8080:443

# Get admin password
kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d

# Access UI at https://localhost:8080
# Username: admin
# Password: (from command above)
```

### Checking Sync Status

```bash
# List all applications
kubectl get applications -n argocd

# Describe specific application
kubectl describe application switchyard-api -n argocd

# Check application health
kubectl get applications -n argocd -o wide
```

### Manual Sync Operations

```bash
# Force sync an application
kubectl patch application switchyard-api -n argocd \
  --type merge -p '{"operation":{"sync":{}}}'

# Refresh application (fetch latest manifests)
kubectl patch application switchyard-api -n argocd \
  --type merge -p '{"metadata":{"annotations":{"argocd.argoproj.io/refresh":"hard"}}}'
```

### Rollback

```bash
# View application history
argocd app history switchyard-api

# Rollback to specific revision
argocd app rollback switchyard-api <revision>
```

## Sync Policies

### Automated Sync

All applications are configured with automated sync:

| Setting | Value | Description |
|---------|-------|-------------|
| `prune` | `true` | Delete resources removed from Git |
| `selfHeal` | `true` | Revert manual cluster changes |
| `allowEmpty` | `false` | Prevent sync of empty directories |

### Self-Healing

When `selfHeal: true`, ArgoCD automatically corrects drift:

1. Manual `kubectl edit` changes are reverted
2. Resource deletions are restored
3. Configuration drift is corrected

**Important:** Disable self-heal temporarily during debugging:

```bash
kubectl patch application switchyard-api -n argocd \
  --type merge -p '{"spec":{"syncPolicy":{"automated":{"selfHeal":false}}}}'
```

## Troubleshooting

### Application Stuck in "Progressing"

```bash
# Check application events
kubectl describe application <app-name> -n argocd

# Check pod status in target namespace
kubectl get pods -n enclii

# View ArgoCD controller logs
kubectl logs -n argocd \
  -l app.kubernetes.io/name=argocd-application-controller -f
```

### Sync Failed

```bash
# View sync operation details
kubectl get application <app-name> -n argocd -o yaml | \
  yq '.status.operationState'

# Check for resource conflicts
kubectl get application <app-name> -n argocd -o yaml | \
  yq '.status.sync.comparedTo'
```

### Out of Sync

```bash
# View what's different
kubectl get application <app-name> -n argocd -o yaml | \
  yq '.status.resources[] | select(.status != "Synced")'
```

## Best Practices

1. **Never edit resources directly** - Always modify Git and let ArgoCD sync
2. **Use Kustomize overlays** - Environment-specific configs in `infra/k8s/{env}/`
3. **Review sync status** - Check ArgoCD UI before deployments
4. **Monitor drift** - Self-heal ensures consistency but review causes

## Related Documentation

- [Deployment Guide](../../infra/DEPLOYMENT.md)
- [Storage with Longhorn](./STORAGE.md)
- [Cloudflare Integration](./CLOUDFLARE.md)
- [Production Checklist](../production/PRODUCTION_CHECKLIST.md)

## Verification

```bash
# Verify ArgoCD is healthy
kubectl get applications -n argocd

# Expected output:
NAME             SYNC STATUS   HEALTH STATUS
root             Synced        Healthy
switchyard-api   Synced        Healthy
switchyard-ui    Synced        Healthy
...
```
