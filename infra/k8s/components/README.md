# Kubernetes Components

Reusable Kustomize components for standardized deployment patterns.

## Components

### deployment-template

Generic deployment template with ServiceAccount, Deployment, and Service.

**Usage:**
```yaml
# apps/<service>/k8s/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

components:
  - ../../../../infra/k8s/components/deployment-template

patches:
  - target:
      kind: Deployment
      name: APP_NAME
    patch: |
      - op: replace
        path: /metadata/name
        value: my-service
      # ... customize other fields
```

**Replacements Required:**
- `APP_NAME`: Service name
- `NAMESPACE`: Target namespace
- `IMAGE`: Container image
- `CONTAINER_PORT`: Port the app listens on

### environment

Consolidated environment variables for all environments.

**Files:**
- `common.env`: Shared across all environments
- `production.env`: Production overrides
- `staging.env`: Staging overrides

**Usage:**
```yaml
# infra/k8s/production/kustomization.yaml
configMapGenerator:
  - name: env-config
    envs:
      - ../components/environment/common.env
      - ../components/environment/production.env
```

## Benefits

1. **Reduced Duplication**: ~200 lines of YAML per service → 1 template
2. **Consistent Patterns**: All services follow same security, probe, resource patterns
3. **Easier Maintenance**: Update template once, all services benefit
4. **Environment Parity**: Same structure across dev/staging/prod
