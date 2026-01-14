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

## Current Issue (In Progress)

**Runner is stable but jobs not being assigned!**

### Verified Working
- Runner pod is Running (1/1) and stable for 5+ minutes
- Runner registered with GitHub (runnerId: 43, ready: true)
- Listener is running in arc-system namespace and polling for messages
- Jobs are queued with correct label `enclii-runners-blue`
- `ARC_BOOTSTRAP_COMPLETE=true` is set in GitHub repo variables

### Still Investigating
- Jobs remain "queued" with no runner assigned
- Listener shows `"assigned job": 0` in logs
- GitHub doesn't seem to be routing jobs to the runner

### Next Steps to Try
1. Check GitHub organization runner settings
   - Verify runner group permissions
   - Check if runner appears in org/repo settings
2. Check if runner needs to be in a specific runner group
   - Current: `runnerGroup: "default"`
   - May need to match org-level config
3. Try removing `runnerGroup` setting entirely
4. Check GitHub Actions runner debug logs
   ```bash
   # Enable debug logging in workflow
   ACTIONS_RUNNER_DEBUG: true
   ```
5. Check if there's a webhook issue between GitHub and ARC

### Useful Debug Commands
```bash
# Check listener for job assignments
kubectl logs -n arc-system enclii-runners-blue-754b578d-listener --tail=50

# Check if runner appears in GitHub
gh api repos/madfam-org/enclii/actions/runners

# Check runner at org level
gh api orgs/madfam-org/actions/runners
```
