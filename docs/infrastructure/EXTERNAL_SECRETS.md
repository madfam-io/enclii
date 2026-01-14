# External Secrets Operator

**Last Updated:** January 2026
**Status:** Operational

---

## Overview

Enclii uses the External Secrets Operator (ESO) to synchronize secrets from external secret management systems into Kubernetes. This provides a secure, GitOps-friendly approach to secret management.

## Architecture

```
┌─────────────────────────────────────────┐
│         External Secret Store            │
│  (e.g., Vault, AWS Secrets Manager)      │
└─────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────┐
│     External Secrets Operator            │
│     (external-secrets namespace)         │
│  ┌─────────────────────────────────────┐ │
│  │ • SecretStore/ClusterSecretStore    │ │
│  │ • ExternalSecret resources          │ │
│  │ • Sync controller                   │ │
│  └─────────────────────────────────────┘ │
└─────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────┐
│       Kubernetes Secrets                 │
│  ┌─────────────────────────────────────┐ │
│  │ • enclii-cloudflare-credentials     │ │
│  │ • enclii-oidc-credentials           │ │
│  │ • enclii-registry-credentials       │ │
│  └─────────────────────────────────────┘ │
└─────────────────────────────────────────┘
```

## Configuration

### ClusterSecretStore

Defines the connection to external secret provider:

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ClusterSecretStore
metadata:
  name: vault-backend
spec:
  provider:
    vault:
      server: "https://vault.internal:8200"
      path: "secret"
      version: "v2"
      auth:
        kubernetes:
          mountPath: "kubernetes"
          role: "external-secrets"
```

### ExternalSecret

Defines which secrets to sync:

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: cloudflare-credentials
  namespace: enclii
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: vault-backend
    kind: ClusterSecretStore
  target:
    name: enclii-cloudflare-credentials
    creationPolicy: Owner
  data:
    - secretKey: api-token
      remoteRef:
        key: enclii/cloudflare
        property: api-token
    - secretKey: account-id
      remoteRef:
        key: enclii/cloudflare
        property: account-id
    - secretKey: zone-id
      remoteRef:
        key: enclii/cloudflare
        property: zone-id
    - secretKey: tunnel-id
      remoteRef:
        key: enclii/cloudflare
        property: tunnel-id
```

## Secret Types

### Cloudflare Credentials

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: cloudflare-credentials
  namespace: enclii
spec:
  secretStoreRef:
    name: vault-backend
    kind: ClusterSecretStore
  target:
    name: enclii-cloudflare-credentials
  data:
    - secretKey: api-token
      remoteRef:
        key: enclii/cloudflare
        property: api-token
    - secretKey: account-id
      remoteRef:
        key: enclii/cloudflare
        property: account-id
    - secretKey: zone-id
      remoteRef:
        key: enclii/cloudflare
        property: zone-id
    - secretKey: tunnel-id
      remoteRef:
        key: enclii/cloudflare
        property: tunnel-id
```

### OIDC Credentials

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: oidc-credentials
  namespace: enclii
spec:
  secretStoreRef:
    name: vault-backend
    kind: ClusterSecretStore
  target:
    name: enclii-oidc-credentials
  data:
    - secretKey: client-id
      remoteRef:
        key: enclii/oidc
        property: client-id
    - secretKey: client-secret
      remoteRef:
        key: enclii/oidc
        property: client-secret
```

### Registry Credentials

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: registry-credentials
  namespace: enclii
spec:
  secretStoreRef:
    name: vault-backend
    kind: ClusterSecretStore
  target:
    name: enclii-registry-credentials
  data:
    - secretKey: username
      remoteRef:
        key: enclii/registry
        property: username
    - secretKey: password
      remoteRef:
        key: enclii/registry
        property: password
```

## Operations

### Check Sync Status

```bash
# List all ExternalSecrets
kubectl get externalsecrets -A

# Check sync status
kubectl get externalsecrets -n enclii -o wide

# Describe specific ExternalSecret
kubectl describe externalsecret cloudflare-credentials -n enclii
```

### Force Refresh

```bash
# Annotate to force immediate refresh
kubectl annotate externalsecret cloudflare-credentials \
  -n enclii \
  force-sync=$(date +%s) --overwrite
```

### Verify Synced Secret

```bash
# Check if target secret exists
kubectl get secret enclii-cloudflare-credentials -n enclii

# Verify secret keys (don't print values!)
kubectl get secret enclii-cloudflare-credentials -n enclii \
  -o jsonpath='{.data}' | jq 'keys'
```

## Troubleshooting

### ExternalSecret Not Syncing

```bash
# Check ExternalSecret status
kubectl get externalsecret <name> -n <namespace> -o yaml | \
  yq '.status'

# Check operator logs
kubectl logs -n external-secrets \
  -l app.kubernetes.io/name=external-secrets -f

# Verify SecretStore connectivity
kubectl get clustersecretstores -o wide
```

### Authentication Errors

```bash
# Check SecretStore status
kubectl get clustersecretstores vault-backend -o yaml | \
  yq '.status'

# Verify Kubernetes service account
kubectl get sa external-secrets -n external-secrets

# Check RBAC
kubectl auth can-i get secrets --as=system:serviceaccount:external-secrets:external-secrets
```

### Secret Not Updating

```bash
# Check refresh interval
kubectl get externalsecret <name> -n <namespace> \
  -o jsonpath='{.spec.refreshInterval}'

# Force refresh
kubectl annotate externalsecret <name> -n <namespace> \
  force-sync=$(date +%s) --overwrite

# Check last sync time
kubectl get externalsecret <name> -n <namespace> \
  -o jsonpath='{.status.refreshTime}'
```

## Security Best Practices

1. **Use ClusterSecretStore** for shared secrets across namespaces
2. **Set appropriate refreshInterval** - 1h for most secrets, shorter for sensitive ones
3. **Monitor sync status** - Alert on failed syncs
4. **Rotate secrets in source** - ESO will automatically sync changes
5. **Audit access** - Review who can read ExternalSecret resources

## Comparison with Sealed Secrets

| Feature | External Secrets | Sealed Secrets |
|---------|-----------------|----------------|
| Secret source | External provider | Git (encrypted) |
| Rotation | Automatic | Manual re-encrypt |
| Multi-cluster | Easy | Complex |
| GitOps friendly | Yes | Yes |
| Learning curve | Medium | Low |

Enclii uses External Secrets Operator for its automatic rotation and multi-cluster capabilities.

## Related Documentation

- [GitOps with ArgoCD](./GITOPS.md)
- [Cloudflare Integration](./CLOUDFLARE.md)
- [Production Checklist](../production/PRODUCTION_CHECKLIST.md)

## Verification

```bash
# Verify operator is healthy
kubectl get pods -n external-secrets

# Expected:
NAME                                       READY   STATUS
external-secrets-xxxxxxxxx-xxxxx           1/1     Running
external-secrets-cert-controller-xxxxx     1/1     Running
external-secrets-webhook-xxxxx             1/1     Running

# Verify ClusterSecretStore
kubectl get clustersecretstores

# Expected:
NAME            AGE     STATUS
vault-backend   30d     Valid

# Verify secrets are synced
kubectl get externalsecrets -n enclii

# Expected:
NAME                      STORE           REFRESH INTERVAL   STATUS
cloudflare-credentials    vault-backend   1h                 SecretSynced
oidc-credentials          vault-backend   1h                 SecretSynced
registry-credentials      vault-backend   1h                 SecretSynced
```
