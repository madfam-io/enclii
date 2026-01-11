# enclii rollback

Rollback a service to a previous deployment.

## Synopsis

```bash
enclii rollback <service> [flags]
```

## Description

The `rollback` command reverts a service to a previous release. By default, it rolls back to the immediately previous deployment. You can specify a target revision or release ID.

## Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `service` | Yes | Service name to rollback |

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--env`, `-e` | string | `production` | Target environment |
| `--to-revision` | int | | Rollback to specific revision number |
| `--to-release` | string | | Rollback to specific release ID |
| `--dry-run` | bool | `false` | Preview rollback without executing |
| `--wait`, `-w` | bool | `true` | Wait for rollback to complete |
| `--timeout` | duration | `5m` | Rollback timeout |

## Examples

### Rollback to Previous Version
```bash
enclii rollback api
```

**Output:**
```
Rolling back api...
  Current:  rel_abc123 (v1.2.3)
  Target:   rel_xyz789 (v1.2.2)
  Strategy: rolling

Progress: ████████████████████ 100%

Rollback successful!
  Release: rel_xyz789
  Status:  healthy
```

### Rollback to Specific Revision
```bash
enclii rollback api --to-revision 5
```

### Rollback to Specific Release
```bash
enclii rollback api --to-release rel_xyz789
```

### Preview Rollback (Dry Run)
```bash
enclii rollback api --dry-run
```

**Output:**
```
DRY RUN - No changes will be made

Would rollback:
  Service:  api
  From:     rel_abc123 (v1.2.3) - deployed 2h ago
  To:       rel_xyz789 (v1.2.2) - deployed 1d ago

Changes:
  - Revert: feat: add user endpoint
  - Revert: fix: rate limiting bug
```

### Rollback in Staging
```bash
enclii rollback api --env staging --to-revision 3
```

## Rollback Behavior

1. **Identifies target release** from history
2. **Validates release** is still available
3. **Executes deployment** with same strategy as original
4. **Updates traffic routing** to rolled-back version
5. **Verifies health** of rolled-back instances

## Automatic Rollback

Enclii can automatically rollback failed deployments when:
- Health checks fail for new instances
- Error rate exceeds 2% for 2 minutes
- P95 latency exceeds defined SLO

Configure in `enclii.yaml`:
```yaml
spec:
  deployment:
    autoRollback:
      enabled: true
      errorThreshold: 2%
      latencyThreshold: 500ms
```

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Rollback successful |
| `10` | Invalid revision/release |
| `30` | Rollback failed |
| `40` | Timeout |

## See Also

- [`enclii deploy`](./deploy.md) - Deploy a service
- [`enclii ps`](./ps.md) - Check deployment status
- [`enclii logs`](./logs.md) - Debug deployment issues
