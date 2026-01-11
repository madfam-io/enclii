# enclii services sync

Synchronize service configuration with the control plane.

## Synopsis

```bash
enclii services sync [flags]
```

## Description

The `services sync` command uploads your local `enclii.yaml` configuration to the Enclii control plane. This updates the service definition without triggering a deployment. Use `enclii deploy` to apply the changes.

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--file`, `-f` | string | `enclii.yaml` | Path to service configuration file |
| `--dry-run` | bool | `false` | Validate and show diff without applying |
| `--force` | bool | `false` | Skip confirmation prompt |
| `--output`, `-o` | string | `table` | Output format: `table`, `json`, `yaml`, `diff` |

## Examples

### Sync Configuration
```bash
enclii services sync
```

**Output:**
```
Syncing service configuration...

Changes detected:
  spec.runtime.instances: 1 → 2
  spec.runtime.resources.memory: 256Mi → 512Mi
  spec.env[+]: DATABASE_POOL_SIZE=10

Apply changes? [y/N]: y

Configuration synced successfully!
  Service: api
  Version: 3

To deploy these changes:
  enclii deploy --env staging
```

### Dry Run (Preview Changes)
```bash
enclii services sync --dry-run
```

**Output:**
```
DRY RUN - No changes will be applied

Current → New:
  spec:
    runtime:
-     instances: 1
+     instances: 2
      resources:
-       memory: "256Mi"
+       memory: "512Mi"
    env:
+     - name: DATABASE_POOL_SIZE
+       value: "10"
```

### Show Diff Output
```bash
enclii services sync --dry-run -o diff
```

### Sync from Custom File
```bash
enclii services sync -f ./config/production.yaml
```

### Force Sync (Skip Confirmation)
```bash
enclii services sync --force
```

## Validation

Before syncing, the CLI validates:
- YAML syntax
- Required fields (`apiVersion`, `kind`, `metadata.name`)
- Resource limits within quotas
- Valid environment variable names
- Port range (1-65535)

## Configuration Versioning

Each sync creates a new configuration version:
- Versions are immutable
- Previous versions can be restored
- Changes tracked with author and timestamp

View version history:
```bash
enclii services history api
```

## Relationship to Deploy

| Command | Effect |
|---------|--------|
| `services sync` | Updates config definition only |
| `deploy` | Builds, creates release, and deploys |

Typical workflow:
```bash
# 1. Update enclii.yaml locally
vim enclii.yaml

# 2. Validate and sync config
enclii services sync --dry-run
enclii services sync

# 3. Deploy when ready
enclii deploy --env staging
```

## See Also

- [`enclii init`](./init.md) - Create initial configuration
- [`enclii deploy`](./deploy.md) - Deploy the service
- [Service Specification Reference](../../reference/service-spec.md)
