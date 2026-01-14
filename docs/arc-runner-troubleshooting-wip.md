# ARC Runner Troubleshooting - Work In Progress

**Date**: 2026-01-14
**Status**: IN PROGRESS - Root cause identified, solution pending

## Problem Summary

ARC (Actions Runner Controller) ephemeral runners crash immediately after starting (~1-2 seconds), creating a restart loop that fails after 5 attempts.

## Root Cause Analysis

### Confirmed Findings

1. **Runner registers successfully with GitHub** - The controller logs show:
   ```
   "Runner exists in GitHub service" runnerId: 31
   ```

2. **Runner pod exits immediately after registration** - Controller detects:
   ```
   "Ephemeral runner pod has finished, but the runner still exists in the service. Deleting the pod to restart it."
   ```

3. **Issue is NOT DinD-specific** - Tested with minimal config (no DinD) and same crash occurs

4. **Issue is NOT volume-related** - Tested without custom PVC mounts, same result

5. **JIT config is being provided** - Base64-encoded `runnerJITConfig` is present in EphemeralRunner status

### Key Observation

The runner container starts, receives JIT config, registers with GitHub, but then **exits with code 0** almost immediately instead of staying alive to wait for jobs. The controller interprets this as a failure since the runner is still registered but the pod is gone.

## Infrastructure State

### Working Components
- ARC Controller: Running in `arc-system` namespace (v0.10.1)
- Listeners: Running for both blue/green scale sets
- GitHub App Secret: Present with correct keys (github_app_id, github_app_installation_id, github_app_private_key)
- PVCs: Bound (arc-go-cache, arc-npm-cache, arc-docker-cache-blue)

### Kubeconfig
```bash
export KUBECONFIG=~/.kube/config-hetzner
# Cluster: 95.217.198.239:6443
```

## What We've Tried

### 1. TCP/TLS Docker Configuration Fix
- Changed DOCKER_HOST from `tcp://localhost:2376` to `unix:///var/run/docker.sock`
- Removed DOCKER_TLS_VERIFY and DOCKER_CERT_PATH
- **Result**: No change, runners still crash

### 2. Clean Configuration (No Conflicting Volumes)
- Removed manual `work` volume definition (let ARC inject it)
- Removed duplicate github config from blue/green values (inherited from base)
- **Result**: No change, runners still crash

### 3. Minimal Configuration Test
- Deployed `enclii-test-runner` with absolute minimal config (no DinD, no custom volumes)
- **Result**: Same crash pattern - runner registers then exits immediately

## Current Configuration Files

### values-runner-set.yaml (Base)
```yaml
githubConfigUrl: "https://github.com/madfam-org"
githubConfigSecret: github-app-secret
runnerGroup: "default"

containerMode:
  type: "dind"

template:
  spec:
    serviceAccountName: arc-runner
    containers:
      - name: runner
        image: ghcr.io/actions/actions-runner:2.321.0
        resources:
          limits:
            cpu: "2"
            memory: 4Gi
          requests:
            cpu: "500m"
            memory: 1Gi
        env:
          - name: GOPATH
            value: /home/runner/go
          - name: GOCACHE
            value: /home/runner/go/cache
          - name: npm_config_cache
            value: /home/runner/.npm
        volumeMounts:
          - name: go-cache
            mountPath: /home/runner/go
          - name: npm-cache
            mountPath: /home/runner/.npm
    terminationGracePeriodSeconds: 300

listenerTemplate:
  spec:
    containers:
      - name: listener
        resources:
          limits:
            cpu: 100m
            memory: 128Mi
          requests:
            cpu: 50m
            memory: 64Mi
```

### values-runner-set-blue.yaml
```yaml
runnerScaleSetName: "enclii-runners-blue"
minRunners: 1
maxRunners: 6

template:
  metadata:
    labels:
      arc.enclii.dev/color: blue
      arc.enclii.dev/active: "true"
  spec:
    volumes:
      - name: go-cache
        persistentVolumeClaim:
          claimName: arc-go-cache
      - name: npm-cache
        persistentVolumeClaim:
          claimName: arc-npm-cache
      - name: docker-cache
        persistentVolumeClaim:
          claimName: arc-docker-cache-blue
```

## Next Steps to Try

### 1. Check Runner Image Entrypoint
The runner container has no explicit `command` - it relies on the image's default. The ARC runner image (ghcr.io/actions/actions-runner:2.321.0) should have a proper entrypoint that:
- Reads JIT config from environment/secret
- Configures the runner
- Starts listening for jobs

**Action**: Inspect the actual runner image entrypoint:
```bash
docker pull ghcr.io/actions/actions-runner:2.321.0
docker inspect ghcr.io/actions/actions-runner:2.321.0 --format='{{.Config.Entrypoint}} {{.Config.Cmd}}'
```

### 2. Check JIT Config Mount
ARC injects JIT config as a secret mounted to the runner. Verify:
```bash
KUBECONFIG=~/.kube/config-hetzner kubectl get secrets -n arc-runners | grep jitconfig
```

### 3. Capture Runner Logs Before Exit
The runner exits so fast we can't capture logs. Try:
```bash
# Watch for new pods and immediately grab logs
KUBECONFIG=~/.kube/config-hetzner kubectl logs -n arc-runners -l app.kubernetes.io/component=runner -f --timestamps
```

### 4. Try Explicit Command Override
If the image entrypoint is broken, try specifying the command explicitly:
```yaml
containers:
  - name: runner
    image: ghcr.io/actions/actions-runner:2.321.0
    command: ["/home/runner/run.sh"]
```

### 5. Check GitHub Runner Application Logs
Inside the runner container at `/home/runner/_diag/` there should be diagnostic logs.

### 6. Upgrade/Downgrade ARC Controller
Current: v0.10.1 - Try v0.10.0 or v0.9.x to see if it's a regression.

### 7. Check Listener Logs
The listener communicates with GitHub and tells the controller when to create runners:
```bash
KUBECONFIG=~/.kube/config-hetzner kubectl logs -n arc-runners -l app.kubernetes.io/component=autoscaling-listener -f
```

## Useful Commands

```bash
# Set kubeconfig
export KUBECONFIG=~/.kube/config-hetzner

# Check all ARC resources
kubectl get autoscalingrunnerset,ephemeralrunnerset,ephemeralrunner -n arc-runners

# Check controller logs
kubectl logs -n arc-system -l app.kubernetes.io/name=gha-rs-controller --tail=100

# Check listener logs
kubectl logs -n arc-runners -l app.kubernetes.io/component=autoscaling-listener --tail=100

# Watch runner pods
kubectl get pods -n arc-runners -w

# Get ephemeral runner status
kubectl get ephemeralrunner -n arc-runners -o yaml

# Helm releases
helm list -n arc-runners
helm list -n arc-system

# Upgrade blue runners
helm upgrade --install enclii-runners-blue \
  oci://ghcr.io/actions/actions-runner-controller-charts/gha-runner-scale-set \
  --namespace arc-runners \
  --version 0.10.1 \
  --values infra/helm/arc/values-runner-set.yaml \
  --values infra/helm/arc/values-runner-set-blue.yaml
```

## Cleanup Before Next Session

```bash
# Delete test runner
helm uninstall enclii-test-runner -n arc-runners

# Set blue minRunners to 0 to stop crash loop
# Edit infra/helm/arc/values-runner-set-blue.yaml: minRunners: 0
# Then upgrade
```

## Files Modified This Session

1. `infra/helm/arc/values-runner-set.yaml` - Clean DinD configuration
2. `infra/helm/arc/values-runner-set-blue.yaml` - Removed work volume, set minRunners: 1
3. `infra/helm/arc/values-runner-set-green.yaml` - Removed work volume

## Related GitHub Issues to Check

- https://github.com/actions/actions-runner-controller/issues (search "ephemeral runner exits immediately")
- https://github.com/actions/runner/issues (search "JIT config")
