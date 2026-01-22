# Golden Configuration Snapshots

This directory contains reference snapshots of critical Kubernetes manifests.
These snapshots are used by CI to detect unintentional configuration drift.

## Purpose

Golden configs serve as a **stability enforcement mechanism** (Operation Ratchet).
They lock in known-good configurations and prevent accidental regressions.

## Directory Structure

```
tests/golden/
├── README.md                    # This file
└── k8s/
    ├── production/
    │   ├── environment-patch.yaml.golden    # OIDC/SSO configuration
    │   ├── cloudflared-unified.yaml.golden  # Cloudflare tunnel routes
    │   └── security-patch.yaml.golden       # Security context settings
    ├── base/
    │   └── roundhouse.yaml.golden           # Build pipeline config
    └── apps/
        └── dispatch-deployment.yaml.golden  # Admin UI deployment
```

## Protected Configuration Keys

These keys are monitored by `scripts/validate.sh` and CI:

| Key | File | Purpose |
|-----|------|---------|
| `imagePullSecrets:` | roundhouse.yaml, dispatch/deployment.yaml | Registry auth |
| `ENCLII_OIDC_ISSUER` | environment-patch.yaml | SSO provider |
| `ENCLII_OIDC_CLIENT_ID` | environment-patch.yaml | OAuth client |
| `ENCLII_OIDC_CLIENT_SECRET` | environment-patch.yaml | OAuth secret ref |
| `ENCLII_EXTERNAL_JWKS_URL` | environment-patch.yaml | JWT validation |
| `ALLOWED_ADMIN_DOMAINS` | dispatch/k8s/deployment.yaml | Admin access |
| `ALLOWED_ADMIN_ROLES` | dispatch/k8s/deployment.yaml | Role auth |
| `NEXT_PUBLIC_JANUA_URL` | Dockerfiles | SSO endpoint |
| `hostname: api.enclii.dev` | cloudflared-unified.yaml | Critical route |
| `hostname: app.enclii.dev` | cloudflared-unified.yaml | Critical route |
| `hostname: admin.enclii.dev` | cloudflared-unified.yaml | Critical route |
| `hostname: auth.madfam.io` | cloudflared-unified.yaml | Critical route |

## Usage

### Update golden configs (after intentional changes)

```bash
./scripts/update-golden.sh
```

Run this after you've intentionally modified a critical manifest and verified
the changes work in production.

### Check for drift

```bash
./scripts/check-golden.sh
```

This is run automatically by CI. It fails if any critical manifest differs
from its golden snapshot.

### Local validation

```bash
./scripts/validate.sh --golden
```

Runs all validation checks including golden config comparison.

## When to Update Golden Configs

1. **After intentional manifest changes**: Once changes are deployed and verified
2. **During planned migrations**: Update as part of the migration process
3. **After security patches**: Lock in new security configurations

## When NOT to Update Golden Configs

1. **To make CI pass**: Fix the actual issue instead
2. **Without understanding the diff**: Always review what changed
3. **Without testing**: Changes must be verified in production first

## CI Integration

The `golden-config` job in `.github/workflows/ci.yml` runs `check-golden.sh`
on every PR and push to main. If it fails:

1. Review the diff output
2. If the change is intentional and tested: run `./scripts/update-golden.sh`
3. If the change is unintentional: revert the manifest changes

## Troubleshooting

### "Golden config not initialized"

Run `./scripts/update-golden.sh` to create initial snapshots.

### "Unexpected diff in X.yaml"

1. Run `diff infra/k8s/path/to/file.yaml tests/golden/k8s/path/to/file.yaml.golden`
2. Determine if the change is intentional
3. Either fix the manifest or update the golden config
