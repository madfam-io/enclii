# ArgoCD Image Updater

Automated container image updates for GitOps deployments.

## Overview

ArgoCD Image Updater watches container registries for new image versions and automatically updates Git repositories when new images are available. This enables:

- **Automated Deployments**: New images trigger GitOps deployments automatically
- **Version Control**: All image updates are committed to Git
- **Audit Trail**: Full history of image changes
- **Rollback**: Easy rollback via Git revert

## Installation

### 1. Install ArgoCD Image Updater

```bash
kubectl create namespace argocd-image-updater

kubectl apply -n argocd-image-updater \
  -f https://raw.githubusercontent.com/argoproj-labs/argocd-image-updater/stable/manifests/install.yaml
```

Or via Helm:

```bash
helm repo add argo https://argoproj.github.io/argo-helm
helm install argocd-image-updater argo/argocd-image-updater \
  -n argocd \
  --set config.registries[0].name=ghcr \
  --set config.registries[0].api_url=https://ghcr.io \
  --set config.registries[0].prefix=ghcr.io
```

### 2. Configure Registry Credentials

```bash
kubectl create secret generic ghcr-credentials \
  -n argocd \
  --from-literal=username=<github-username> \
  --from-literal=password=<github-token>
```

### 3. Configure Git Write-Back

```bash
# Option 1: SSH key (recommended)
kubectl create secret generic git-creds \
  -n argocd \
  --from-file=sshPrivateKey=/path/to/id_rsa

# Option 2: HTTPS token
kubectl create secret generic git-creds \
  -n argocd \
  --from-literal=username=<github-username> \
  --from-literal=password=<github-token>
```

### 4. Apply Configuration

```bash
kubectl apply -f registries-config.yaml
```

## Application Configuration

Add annotations to ArgoCD Applications to enable image updates:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: switchyard-api
  annotations:
    # Enable image updates
    argocd-image-updater.argoproj.io/image-list: |
      switchyard-api=ghcr.io/madfam-org/switchyard-api

    # Update strategy (semver, latest, digest)
    argocd-image-updater.argoproj.io/switchyard-api.update-strategy: semver

    # Git write-back method
    argocd-image-updater.argoproj.io/write-back-method: git

    # Branch for updates
    argocd-image-updater.argoproj.io/write-back-target: kustomization
```

## Update Strategies

| Strategy | Description | Use Case |
|----------|-------------|----------|
| `semver` | Latest semantic version | Production releases |
| `latest` | Latest by date | Development/staging |
| `digest` | Track specific digest | Immutable deployments |
| `name` | Lexicographic sort | Custom naming |

## Write-Back Methods

| Method | Description |
|--------|-------------|
| `git` | Commit changes to Git repository |
| `argocd` | Update ArgoCD Application directly |

## Monitoring

### View Logs

```bash
kubectl logs -n argocd-image-updater -l app.kubernetes.io/name=argocd-image-updater -f
```

### Check Update Status

```bash
# Application annotations show last update
kubectl get application <name> -n argocd -o yaml | grep argocd-image-updater
```

## Troubleshooting

### Image Not Updating

1. Check Image Updater logs for errors
2. Verify registry credentials are correct
3. Ensure image exists in registry
4. Check Application annotations are correct

### Git Write-Back Failing

1. Verify Git credentials secret exists
2. Check repository permissions
3. Ensure target branch exists

### Rate Limiting

GitHub Container Registry has rate limits. Configure credentials to avoid limits:

```yaml
registries:
  - name: ghcr
    api_url: https://ghcr.io
    prefix: ghcr.io
    credentials: pullsecret:argocd/ghcr-credentials
```
