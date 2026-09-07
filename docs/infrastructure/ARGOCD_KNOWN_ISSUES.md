---
title: ArgoCD Known Issues
description: Known bugs, workarounds, and proposed upstream fixes for ArgoCD
sidebar_position: 6
tags: [argocd, oci, helm, bugs]
---

> [!IMPORTANT]
> MADFAM-ENCLII-FIRST-LEGACY-RAW v1: This document contains legacy raw infrastructure command examples.
> Routine production operations must use Enclii web, API, or CLI. Treat raw
> `kubectl`, `helm`, SSH, provider CLI/API, `docker exec`, and direct container
> access as platform bootstrap or documented break-glass only, and record any
> missing Enclii adapter gap.


# ArgoCD Known Issues

## Multi-Source OCI Helm Revision Resolution Bug

**Status:** Open (ArgoCD v3.2.5)
**Affected Apps:** `arc-runners`, `arc-runners-blue`
**Impact:** Sync status shows `Unknown`, auto-sync disabled. Pods are Healthy and functional.

### Symptom

Two ARC (Actions Runner Controller) applications show `Unknown` sync status with `ComparisonError`:

```
ComparisonError: failed to load target state: failed to generate manifest for source 1 of 2:
rpc error: code = Unknown desc = OCI Helm: failed to get chart version for
oci://ghcr.io/actions/actions-runner-controller-charts/gha-runner-scale-set-controller:
403 Forbidden
```

The apps remain `Healthy` — the deployed resources work correctly. Only the sync comparison fails.

### Root Cause

ArgoCD v3.2.5 has a bug in multi-source OCI Helm revision resolution. When an Application uses `sources` (multi-source) with an OCI Helm chart, the revision resolver sends a HEAD request to the **base repo URL** instead of the **full chart path**.

**What happens:**

1. Application spec defines:
   ```yaml
   sources:
     - repoURL: oci://ghcr.io/actions/actions-runner-controller-charts
       chart: gha-runner-scale-set-controller
       targetRevision: 0.10.1
   ```

2. ArgoCD's revision resolver constructs the HEAD request URL as:
   ```
   HEAD https://ghcr.io/v2/actions/actions-runner-controller-charts/manifests/0.10.1
   ```

3. The **correct** URL should be:
   ```
   HEAD https://ghcr.io/v2/actions/actions-runner-controller-charts/gha-runner-scale-set-controller/manifests/0.10.1
   ```

4. The base URL (`actions-runner-controller-charts`) is a namespace, not a chart — GHCR returns 403.

**Key detail:** This only affects multi-source (`sources[]`) applications. Single-source OCI Helm apps work correctly because the code path that concatenates `repoURL + chart` is different.

### Reproduction Steps

1. Create an ArgoCD Application with multi-source OCI Helm:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-oci-multisource
  namespace: argocd
spec:
  project: default
  sources:
    - repoURL: oci://ghcr.io/actions/actions-runner-controller-charts
      chart: gha-runner-scale-set-controller
      targetRevision: 0.10.1
      helm:
        releaseName: test
        valueFiles:
          - $values/values.yaml
    - repoURL: https://github.com/your-org/your-repo.git
      targetRevision: main
      ref: values
  destination:
    server: https://kubernetes.default.svc
    namespace: test
```

2. Apply and observe sync status:
```bash
kubectl apply -f test-oci-multisource.yaml
kubectl get application test-oci-multisource -n argocd -o jsonpath='{.status.conditions}'
```

3. Expected: `ComparisonError` with 403 from GHCR.

4. Verify the single-source equivalent works:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-oci-singlesource
  namespace: argocd
spec:
  project: default
  source:
    repoURL: oci://ghcr.io/actions/actions-runner-controller-charts
    chart: gha-runner-scale-set-controller
    targetRevision: 0.10.1
    helm:
      releaseName: test
  destination:
    server: https://kubernetes.default.svc
    namespace: test
```

### What We Tried

| Attempt | Result |
|---------|--------|
| Embed chart name in `repoURL` (`oci://ghcr.io/.../gha-runner-scale-set-controller`) | ArgoCD appends chart name again, double path |
| GHCR credentials (`ghcr-oci-creds` secret in argocd namespace) | Auth succeeds but URL is still wrong — 404 instead of 403 |
| Anonymous access (no credentials) | Same 403, confirms it's a URL construction issue |
| Different `repoURL` formats (with/without `oci://` prefix) | No change in behavior |

### Workaround

Manage ARC charts directly via Helm CLI, bypassing ArgoCD sync:

```bash
# Controller
helm upgrade arc-controller \
  oci://ghcr.io/actions/actions-runner-controller-charts/gha-runner-scale-set-controller \
  --namespace arc-system \
  --version 0.10.1 \
  -f infra/helm/arc/values-controller.yaml

# Runner Scale Set
helm upgrade arc-runner-blue \
  oci://ghcr.io/actions/actions-runner-controller-charts/gha-runner-scale-set \
  --namespace arc-runners \
  --version 0.10.1 \
  -f infra/helm/arc/values-runner-set.yaml \
  -f infra/helm/arc/values-runner-set-blue.yaml
```

Auto-sync is disabled on both ARC apps to prevent ArgoCD from interfering with Helm-managed releases. See `infra/argocd/apps/arc-runners.yaml`.

### Proposed Upstream Fix

The bug is in ArgoCD's OCI revision resolver, likely in `util/oci/` or the Helm source generator's multi-source path.

**Expected behavior:** When resolving a chart revision for a multi-source OCI Helm entry, the resolver should concatenate `repoURL` + `chart` before issuing the HEAD/GET request — the same way it does for single-source applications.

**Pseudocode of the fix:**

```go
// Current (broken) behavior in multi-source path:
// Uses repoURL directly for version resolution
func resolveRevision(repoURL string, chart string, version string) {
    // BUG: HEAD https://ghcr.io/v2/<repoURL-path>/manifests/<version>
    ref := repoURL + "/manifests/" + version
    head(ref)
}

// Fixed behavior:
func resolveRevision(repoURL string, chart string, version string) {
    // Concatenate chart name into the OCI reference path
    fullRef := repoURL + "/" + chart
    ref := fullRef + "/manifests/" + version
    head(ref)
}
```

The single-source code path already does this concatenation correctly. The multi-source path skips it.

### GitHub Issue Template

Use the following to file at https://github.com/argoproj/argo-cd/issues:

---

**Title:** Multi-source OCI Helm: revision resolver uses base repoURL without chart name, causing ComparisonError

**Labels:** `bug`, `component:helm`, `component:oci`

**Body:**

#### Summary

When using multi-source (`sources[]`) with an OCI Helm chart, ArgoCD's revision resolver sends HEAD requests to the base `repoURL` without appending the `chart` name. This causes 403/404 errors from OCI registries (GHCR) and results in `ComparisonError` / `Unknown` sync status.

Single-source OCI Helm applications work correctly — the chart name is properly concatenated in that code path.

#### Version

- ArgoCD: v3.2.5
- Kubernetes: v1.33.7+k3s3

#### Application Spec (minimal reproduction)

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
spec:
  sources:
    - repoURL: oci://ghcr.io/actions/actions-runner-controller-charts
      chart: gha-runner-scale-set-controller
      targetRevision: 0.10.1
      helm:
        releaseName: test
        valueFiles:
          - $values/values.yaml
    - repoURL: https://github.com/example/repo.git
      targetRevision: main
      ref: values
  destination:
    server: https://kubernetes.default.svc
    namespace: test
```

#### Expected Behavior

ArgoCD resolves the chart version by issuing:
```
HEAD https://ghcr.io/v2/actions/actions-runner-controller-charts/gha-runner-scale-set-controller/manifests/0.10.1
```

#### Actual Behavior

ArgoCD resolves the chart version by issuing:
```
HEAD https://ghcr.io/v2/actions/actions-runner-controller-charts/manifests/0.10.1
```

The `chart` field (`gha-runner-scale-set-controller`) is not appended to the URL path, causing GHCR to return 403 (the base path is a namespace, not a chart).

#### Error

```
ComparisonError: failed to load target state: failed to generate manifest for source 1 of 2:
rpc error: code = Unknown desc = OCI Helm: failed to get chart version for
oci://ghcr.io/actions/actions-runner-controller-charts/gha-runner-scale-set-controller:
403 Forbidden
```

#### Workaround

Manage affected charts via `helm upgrade` directly. Disable auto-sync on the ArgoCD Application.

#### Suggested Fix

In the multi-source revision resolution path, concatenate `repoURL + "/" + chart` before constructing the OCI reference URL — matching the behavior of the single-source code path.

---

## Selva Services "Progressing" Health Status

**Status:** Fixed (resource customizations added)
**Affected Apps:** `selva-services`
**Impact:** ArgoCD shows `Progressing` or `Unknown` health. All 8 pods are Running and healthy.

### Symptom

The `selva-services` application shows `Progressing` health status with 29 resources at `Unknown` health. The affected resource types are:
- `ScaledObject` (KEDA) — no built-in health check
- `ServiceMonitor` (Prometheus) — declarative resource, no status
- `PodDisruptionBudget` — ArgoCD can't assess `currentHealthy` vs `desiredHealthy`

### Root Cause

ArgoCD has no built-in health assessment for these CRD types. Without custom health checks, it defaults to `Unknown` or `Progressing`.

### Fix

Custom Lua health checks added in `infra/argocd/resource-customizations.yaml`. Apply to the cluster:

```bash
KUBECONFIG=~/.kube/config-hetzner kubectl patch configmap argocd-cm \
  -n argocd --type merge -p "$(cat infra/argocd/resource-customizations.yaml)"
```

This adds health checks for:
- `keda.sh/ScaledObject` — checks `Ready` condition
- `monitoring.coreos.com/ServiceMonitor` — always Healthy (declarative)
- `policy/PodDisruptionBudget` — checks `currentHealthy >= desiredHealthy`

---

## Network-Policies OutOfSync (PostHog Namespace)

**Status:** Fixed (sync options added)
**Affected Apps:** `network-policies`
**Impact:** ArgoCD shows `OutOfSync` for 11 posthog namespace policies + 2 monitoring policies. No security impact — policies for non-existent namespaces are harmless.

### Symptom

The `network-policies` app shows OutOfSync because:
1. PostHog namespace doesn't exist (PostHog not deployed — Helm chart broken, using Cloud proxy)
2. Policies targeting the `posthog` namespace can't be created without the namespace
3. Monitoring policies have annotation drift from manual `kubectl` application

### Fix

Added sync options to `infra/argocd/apps/network-policies.yaml`:
- `CreateNamespace=true` — auto-creates namespaces so policies can be applied
- `SkipDryRunOnMissingResource=true` — prevents sync failures on missing namespace resources

For monitoring policy drift, force sync from ArgoCD to reconcile annotations.

---

## ExternalSecret `spec.data` Changes Never Applied (Runtime-Registered Apps)

**Status:** Fixed and **verified in production 2026-09-07** — ArgoCD's own auto-sync wrote the object with no
operator step (reconciler emits `ServerSideDiff=true`, ExternalSecret ignore rule removed). See
[Outcome](#outcome-verified-in-production-2026-09-07) for what was measured, and
[SSA co-ownership](#the-real-second-order-cause-ssa-co-ownership-delays-the-write-by-one-self-heal-cycle) for
the residual gotcha that makes a landed write look dropped.
**Affected Apps:** every Application registered at runtime by Enclii
(`app.kubernetes.io/managed-by: enclii-platform`, `enclii.dev/registration-mode: runtime`) — observed on `nauta-services`
**Impact:** Additions to an ExternalSecret's `spec.data` were silently discarded. The app stayed
`OutOfSync` forever while auto-sync kept reporting `Succeeded`. Operators worked around it by hand-patching
the live object, which is exactly the raw-`kubectl` drift this platform exists to remove.

### Symptom

After a repo PR added two `spec.data` entries to an ExternalSecret:

- `nauta-services` showed `OutOfSync`, with the ExternalSecret as the only out-of-sync resource.
- The auto-sync operation reported `Succeeded`, message
  `externalsecret.external-secrets.io/nauta-web-secrets serverside-applied`.
- The live object still held its original 13 keys, and its `managedFields` entry for manager
  `argocd-controller` / operation `Apply` had not moved for weeks — a **no-op apply**.

### Root Cause

The generated Application set `RespectIgnoreDifferences=true` **and** an `ignoreDifferences` rule whose
`jqPathExpressions` reach *inside a list*:

```yaml
- group: external-secrets.io
  kind: ExternalSecret
  jqPathExpressions:
    - .spec.data[]?.remoteRef.conversionStrategy
    - .spec.data[]?.remoteRef.decodingStrategy
    - .spec.data[]?.remoteRef.metadataPolicy
```

With `RespectIgnoreDifferences=true`, ArgoCD copies the live values at ignored paths into the desired
object *before* applying (`controller/sync.go` → `normalizeTargetResources`). How it copies them depends on
the resource kind:

```go
versionedObject, err := scheme.Scheme.New(normalizedTarget.GroupVersionKind())
if err == nil { /* strategic merge: lists merged by patch key */ }
// CRDs land here instead:
return jsonpatch.CreateMergePatch(originalJSON, modifiedJSON)  // RFC 7386
```

`scheme.Scheme` only knows built-in Kubernetes types. `ExternalSecret` is a CRD, so both patch creation and
application fall back to **RFC 7386 JSON merge patch, in which arrays are atomic** — the whole live
`spec.data` list replaced the desired one, discarding the new entries. The diff is computed on a separate
path that does not do this substitution, so ArgoCD kept reporting `OutOfSync` while applying nothing.

The three fields the rule hid are apiserver-applied CRD schema defaults (ESO is pinned to chart `0.9.11`,
whose `v1beta1` types carry `+kubebuilder:default=` on `conversionStrategy`, `decodingStrategy` and
`metadataPolicy`), so the rule existed only to silence defaulting noise.

> [!WARNING]
> Never add an `ignoreDifferences` rule with `jqPathExpressions` (or JSON pointers) that select fields
> inside a **list** on a **CRD** while `RespectIgnoreDifferences=true` is set. It will silently drop writes
> to that list. Built-in kinds (`apps`, `batch`, `policy`, core) are safe: they resolve in the scheme and
> take the strategic-merge path. A unit test in `application_reconciler_test.go` enforces this.

### Fix

In `apps/switchyard-api/internal/argocd/application_reconciler.go`:

1. The `external-secrets.io/ExternalSecret` `ignoreDifferences` rule is **removed**.
2. `ServerSideDiff=true` is added to the `argocd.argoproj.io/compare-options` annotation, so the diff comes
   from a server-side-apply dry run. The apiserver returns the object with CRD defaults already applied, so
   the defaulted fields are identical on both sides and cancel out of the diff with no ignore rule.

`ServerSideDiff` must go in the **`argocd.argoproj.io/compare-options` annotation**, not in
`spec.syncPolicy.syncOptions`. In ArgoCD v3.2.5 the controller reads it only from that annotation or the
`ARGOCD_APPLICATION_CONTROLLER_SERVER_SIDE_DIFF` env var (`controller/state.go`); placing it in
`syncOptions` is silently ignored. All other ignore rules (Deployment/StatefulSet replicas, Secret data,
`volumeClaimTemplates`, etc.) are unchanged.

### How the change reaches already-registered Applications

`ReconcileApplication` is **idempotent and re-applies the full desired spec on every call** — `mergeApplication`
compares the whole `spec` plus labels and annotations against the live Application and writes back on any
difference. It is not first-registration-only. So an existing Application picks the fix up as soon as the
reconciler runs against it, with no manual ArgoCD edit:

```bash
# Re-run reconciliation for an already-onboarded repo (idempotent; safe to repeat).
curl -sS -X POST "$ENCLII_API/v1/admin/onboard/ensure" \
  -H "Authorization: Bearer $ENCLII_ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"repo_full_name":"madfam-org/nauta","project_name":"nauta"}'
```

Deploying the new switchyard-api alone does **not** rewrite existing Applications; the reconcile above (or the
next onboard/ensure for that repo) is what propagates it.

### Smoke Test

After rollout and the `onboard/ensure` call:

```bash
# 1. Annotation carries both compare options.
kubectl -n argocd get application nauta-services \
  -o jsonpath='{.metadata.annotations.argocd\.argoproj\.io/compare-options}{"\n"}'
# want: IgnoreExtraneous=true,ServerSideDiff=true

# 2. No ExternalSecret ignore rule remains.
kubectl -n argocd get application nauta-services \
  -o jsonpath='{range .spec.ignoreDifferences[*]}{.kind}{"\n"}{end}' | grep -c ExternalSecret
# want: 0

# 3. Sync status settles at Synced.
kubectl -n argocd get application nauta-services -o jsonpath='{.status.sync.status}{"\n"}'
# want: Synced

# 4. On the NEXT change to the ExternalSecret spec, the argocd-controller Apply
#    managedFields timestamp must ADVANCE (proof the apply is no longer a no-op).
kubectl -n <namespace> get externalsecret nauta-web-secrets -o json \
  | jq -r '.metadata.managedFields[] | select(.manager=="argocd-controller" and .operation=="Apply") | .time'
# want: a timestamp newer than the previous change, and the new keys present in spec.data
```

A stale timestamp in step 4 alongside a `Succeeded` sync means the no-op apply has returned — re-check that no
list-selecting ignore rule was reintroduced for a CRD.

### Outcome (verified in production, 2026-09-07)

The fix works. On `nauta-services`, ArgoCD's own automated self-heal wrote the object at **04:18:07 UTC** with
no operator step:

```
"msg":"Applying resource ExternalSecret/nauta-web-secrets ...","dry-run":"none",
"manager":"argocd-controller","serverSideApply":true,"serverSideDiff":true
```

Live `spec.data` went 13 -> 17 (both `JANUA_DELEGATE_*` and both `JANUA_PORTAL_*` present), the
`argocd-controller` Apply `managedFields` timestamp advanced from `2026-08-13T05:24:28Z` to
`2026-09-07T04:18:07Z`, ESO re-synced the derived Secret, and Reloader restarted `nauta-web`. App settled
`Synced` / `Healthy`.

Two hypotheses from the first pass were tested and **disproved**; record them so nobody re-investigates:

- **The repo-server manifest cache was NOT stale.** The cache entry for the synced revision was read directly
  out of Redis (`mfst|app.kubernetes.io/instance|nauta-services|<sha>|nauta|...`) and already contained the
  correct 17-entry render. A `argocd.argoproj.io/refresh: hard` is **not** part of this fix.
- **`removeWebhookMutation` did NOT collapse predicted-live to live.** It strips fields not owned by the
  configured manager, but `argocd-controller` owned `.spec.data` outright, so nothing was stripped. There is
  also no mutating webhook registered for `external-secrets.io` on this cluster.

### The real second-order cause: SSA co-ownership delays the write by one self-heal cycle

`spec.data` carries **no `x-kubernetes-list-type` marker** in the ESO CRD (verified against the live CRD on
chart 0.9.11), so server-side apply treats the whole list as a single **atomic** ownership unit. It cannot be
co-owned field-by-field: whoever last applied it owns all of it.

A hand `kubectl patch` on 2026-08-16 made `kubectl-patch` a second owner of `.spec.data`. While that lasted, a
**non-forced** SSA from `argocd-controller` was rejected:

```
error: Apply failed with 1 conflict: conflict with "kubectl-patch" using
external-secrets.io/v1beta1: .spec.data
```

gitops-engine sets `ForceConflicts = true` whenever server-side apply is on
(`pkg/utils/kube/resource_ops.go:462-463` at the SHA argo-cd v3.2.5 pins,
`gitops-engine v0.7.1-0.20251217140045-5baed5604d2d`), so ArgoCD does eventually win — but only on a sync that
actually reaches the apply. The earlier `Succeeded` syncs that left the object untouched were self-heal
attempts already deep into backoff (`SelfHealAttemptsCount:18`); the write landed on the run where the counter
had reset to `1`. **Net effect: a legitimate change can look "applied but ignored" for one or more self-heal
cycles before it lands.** It is not lost, and no operator step is required — but it is easy to misread as a
regression.

Reading a live object once is not enough to conclude a write was dropped. Re-read it, and check whether the
`argocd-controller` Apply timestamp advanced, before opening an investigation.

#### Detecting the hazard elsewhere

Any ExternalSecret whose `.spec.data` is owned by a manager other than `argocd-controller` will show the same
delay the next time git changes it:

One `kubectl` call, all namespaces, no per-object shell quoting:

```bash
kubectl get externalsecrets -A -o json --show-managed-fields | python3 -c '
import json, sys
for it in json.load(sys.stdin)["items"]:
    m = it["metadata"]
    owners = [f["manager"] for f in m.get("managedFields", [])
              if "f:data" in json.dumps(f.get("fieldsV1", {}).get("f:spec", {}))]
    foreign = [o for o in owners if o != "argocd-controller"]
    if foreign:
        print(f'"'"'{m["namespace"]}/{m["name"]}: {owners}'"'"')'
```

As of 2026-09-07 this reports `fortuna/fortuna-secrets` and `fortuna/fortuna-acca-secrets` (co-owned by
`kubectl`), plus several ExternalSecrets that were only ever created by hand
(`kubectl-client-side-apply`, in `dhanam`, `enclii`, `janua`, `madfam-site`) and are not ArgoCD-managed.

**Do not "fix" these by hand-patching again** — that is what created the condition. If a specific object must
land immediately rather than on the next self-heal, hand ownership back to ArgoCD once, from the platform, with
the manifest that is already in git:

```bash
kubectl -n <ns> apply --server-side --force-conflicts \
  --field-manager=argocd-controller -f <path-to-git-manifest>.yaml
```

Verify with `--dry-run=server` first; a clean run prints `serverside-applied (server dry run)` and a conflicting
one names the competing manager.

### Why `Replace=true` was considered and rejected

Emitting `argocd.argoproj.io/sync-options: Replace=true` for ExternalSecret kinds would force the write on the
first sync. It is the wrong trade: gitops-engine routes CRDs and Namespaces away from `kubectl replace` and
through `UpdateResource` (`pkg/sync/sync_context.go:1185-1199`), which drops SSA field ownership entirely and
sends a full-object update. That reintroduces last-writer-wins on every field of the object and gives up the
conflict detection that surfaced this problem in the first place. Force-conflicts SSA already converges; it just
converges one cycle later.

---

## Other Known Issues

_No other known issues at this time._
