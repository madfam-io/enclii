# External Secrets Operator Configuration

Centralized secret management for Enclii using External Secrets Operator (ESO).

## Architecture

```
Doppler (or Vault)
       │
       ▼
ClusterSecretStore
       │
       ▼
ExternalSecret (per namespace)
       │
       ▼
Kubernetes Secret (auto-synced)
```

## Quick Start

### 1. Get Doppler Service Token

1. Create account at [doppler.com](https://doppler.com)
2. Create a project for Enclii
3. Go to Access → Service Tokens → Generate
4. Copy the token (starts with `dp.st.`)

### 2. Create Auth Secret

```bash
kubectl create secret generic doppler-token-auth \
  -n external-secrets \
  --from-literal=dopplerToken=dp.st.YOUR_TOKEN_HERE
```

### 3. Apply ClusterSecretStore

```bash
kubectl apply -f cluster-secret-store.yaml
```

### 4. Verify Store is Ready

```bash
kubectl get clustersecretstore doppler-store
# Should show STATUS: Valid
```

### 5. Create ExternalSecrets

```bash
kubectl apply -f example-external-secret.yaml
```

## Migrating Existing Secrets

To migrate from Kubernetes secrets to Doppler:

1. Export existing secrets:
```bash
kubectl get secret enclii-secrets -n enclii -o json | jq -r '.data | to_entries[] | "\(.key)=\(.value | @base64d)"'
```

2. Add each secret to Doppler via CLI or dashboard:
```bash
doppler secrets set DATABASE_URL="postgres://..."
doppler secrets set REDIS_URL="redis://..."
```

3. Create ExternalSecret pointing to Doppler
4. Update deployments to use the new secret name
5. Delete old Kubernetes secret

## Secret Rotation

ESO automatically syncs secrets based on `refreshInterval`:
- Default: 1 hour
- For sensitive secrets, reduce to 5-15 minutes
- For static config, increase to 24 hours

## Providers

### Doppler (Recommended)
- Simple setup, great UI
- Free tier: 5 users, unlimited secrets
- Auto-sync with webhooks available

### HashiCorp Vault
- Self-hosted or HCP
- More complex, better for enterprise
- Dynamic secrets, PKI, transit encryption

### Kubernetes (Migration Helper)
- Copy secrets between namespaces
- Useful during migration to external provider

## Troubleshooting

### ExternalSecret not syncing
```bash
kubectl describe externalsecret <name> -n <namespace>
# Check events for errors
```

### ClusterSecretStore invalid
```bash
kubectl describe clustersecretstore doppler-store
# Verify auth secret exists and token is valid
```

### Secret not created
```bash
kubectl get events -n <namespace> --field-selector reason=SyncFailed
```

## Security Best Practices

1. **Least Privilege**: Create separate Doppler configs per environment
2. **Rotation**: Enable automatic rotation in Doppler
3. **Audit**: Review Doppler access logs regularly
4. **RBAC**: Limit who can view ExternalSecrets in Kubernetes
5. **Namespacing**: Use ExternalSecret (not ClusterExternalSecret) for namespace isolation
