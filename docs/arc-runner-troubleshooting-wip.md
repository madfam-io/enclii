# ARC Runner Troubleshooting - RESOLVED

**Date**: 2026-01-14
**Status**: ✅ RESOLVED - Runners now working

## Problem Summary

ARC (Actions Runner Controller) ephemeral runners crashed immediately after starting (~1-2 seconds), creating a restart loop that failed after 5 attempts.

## Root Cause

**Two separate issues were identified:**

### Issue 1: Missing Command (Fixed)
The `ghcr.io/actions/actions-runner:2.321.0` image has NO entrypoint:
```
Entrypoint: [] | Cmd: [/bin/bash]
```

When defining custom `template.spec.containers`, you override ARC's default injection. Without explicit `command: ["/home/runner/run.sh"]`, the container runs `/bin/bash` and exits immediately.

### Issue 2: Custom Container Override (Root Cause)
**When using `containerMode.type=dind` with custom `template.spec.containers`, ARC's Helm chart merge behavior breaks.** Even with command specified, the runner would connect to GitHub, register, then immediately exit.

The problem was that custom containers:
- Replace (not merge with) ARC's injected configuration
- Miss critical environment variables or settings that ARC normally provides
- Break the init container image reference when partially specified

## Solution

**Use the simplest possible configuration - let ARC handle everything:**

```yaml
# values-runner-set.yaml
githubConfigUrl: "https://github.com/madfam-org"
githubConfigSecret: github-app-secret
runnerGroup: "default"

# NOTE: DinD disabled for now - runners work without it
# containerMode:
#   type: "dind"

template:
  spec:
    serviceAccountName: arc-runner
    terminationGracePeriodSeconds: 300
    # Do NOT define containers - let ARC inject them
```

## What We Tried (and failed)

1. **Added `command: ["/home/runner/run.sh"]`** - Runner still exited immediately
2. **Added required DinD volumeMounts and env vars** - Runner connected to GitHub then exited
3. **Partial container spec with just volumeMounts** - Helm merge broke init container image
4. **Full container spec with all ARC defaults** - Missing something ARC normally injects

## Working Configuration

### Base Config (values-runner-set.yaml)
- No `containerMode` (DinD disabled for now)
- No custom containers
- Only pod-level settings (serviceAccountName, terminationGracePeriodSeconds)

### Color-specific Config (values-runner-set-blue.yaml)
- `runnerScaleSetName`
- `minRunners` / `maxRunners`
- Labels and annotations only

## Current State

- ✅ Blue runner scale set: Working (1/1 pods running)
- ✅ Runners register with GitHub
- ✅ Runners stay running waiting for jobs
- ⚠️ DinD disabled - Docker builds will need alternative approach

## Future Improvements

### Docker Support Options
1. **Use `setup-docker` GitHub Action** - Installs Docker at job runtime
2. **Use Kaniko for builds** - Rootless container builds (already planned)
3. **Wait for ARC chart fix** - Monitor Helm chart updates for proper merge behavior

### Caching
1. **Use GitHub Actions cache** - Built-in caching for dependencies
2. **Future: PVC-based caching** - When ARC chart supports proper container merge

## Useful Commands

```bash
# Set kubeconfig
export KUBECONFIG=~/.kube/config-hetzner

# Check runner status
kubectl get ephemeralrunner,pods -n arc-runners

# Check controller logs
kubectl logs -n arc-system -l app.kubernetes.io/name=gha-rs-controller --tail=50

# Check listener logs
kubectl logs -n arc-runners -l app.kubernetes.io/component=autoscaling-listener --tail=50

# Upgrade runners
helm upgrade --install enclii-runners-blue \
  oci://ghcr.io/actions/actions-runner-controller-charts/gha-runner-scale-set \
  --namespace arc-runners \
  --version 0.10.1 \
  --values infra/helm/arc/values-runner-set.yaml \
  --values infra/helm/arc/values-runner-set-blue.yaml
```

## Files Modified

- `infra/helm/arc/values-runner-set.yaml` - Simplified, no custom containers
- `infra/helm/arc/values-runner-set-blue.yaml` - Removed PVC volumes
- `infra/helm/arc/values-runner-set-green.yaml` - Removed PVC volumes
