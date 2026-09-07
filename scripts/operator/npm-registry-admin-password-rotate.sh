#!/usr/bin/env bash
# npm-registry-admin-password-rotate.sh — rotate the npm.madfam.io htpasswd
# admin credential end to end: new password → bcrypt locally → Vault → ESO →
# pod restart → live verification.
#
# WHY THIS EXISTS
#
# Until 2026-09-07 the registry's htpasswd file was a plain `kind: Secret`
# committed to madfam-org/enclii, which is PUBLIC. The committed line was the
# bcrypt hash of admin@madfam.io at COST 5 — 32 rounds, roughly 1/32 the work
# of the cost-10 default — so the hash was both published and cheap to attack
# offline. `git rm` does not remove it from history, so the only real
# remediation is to rotate the password, which is what this script does. The
# published hash then verifies nothing.
#
# It also fixes the operational problem that motivated the audit: nobody holds
# the current plaintext, so `npm login --registry https://npm.madfam.io` fails
# for everyone. After this run, the operator holds a password they chose.
#
# WHAT THIS SCRIPT DOES NOT DO
#
# It never echoes, logs, or writes the password or the Vault token to disk.
# Both are read with a silent prompt and travel to the cluster over stdin —
# never in argv (where `ps` would show them), never in a file, never in your
# shell history. The bcrypt hash is computed LOCALLY, so the plaintext never
# leaves this machine; only the hash is sent to Vault.
#
# It performs no `kubectl apply`. The only cluster mutation is the Vault KV
# write; the Secret, and then the pod, update themselves (see FLOW).
#
# FLOW
#
#   you → bcrypt (local) → Vault secret/npm-registry #htpasswd
#                            ↓ ESO ClusterSecretStore vault-store (≤15m,
#                              forced here to seconds by annotating the
#                              ExternalSecret)
#                          Secret npm-registry/verdaccio-auth #htpasswd
#                            ↓ Reloader (deployment carries
#                              reloader.stakater.com/auto: "true")
#                          Deployment verdaccio, strategy: Recreate (~30s)
#                            ↓
#                          https://npm.madfam.io/-/whoami → 200
#
# USAGE
#
#   bash scripts/operator/npm-registry-admin-password-rotate.sh
#   REGISTRY_USER=someone@madfam.io bash scripts/operator/…
#
# Runs from any directory. macOS and Linux.
#
# PREREQUISITES
#
#   1. SSH to the bastion: `ssh ssh.madfam.io` must work. That cloudflared
#      tunnel is the only authorized path to the cluster.
#   2. A Vault token with WRITE on `secret/npm-registry` (admin, or the
#      switchyard-secret-writer token). Read access for ESO is already granted
#      by the `eso-reader` policy (`read` on `secret/data/*`) — this rotation
#      needs NO Vault policy change.
#   3. `htpasswd` (macOS ships it at /usr/sbin/htpasswd; Debian:
#      `apt install apache2-utils`). If it is missing this script falls back to
#      python3 + bcrypt, and tells you how to install one of them if neither is
#      present.

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

REGISTRY_USER="${REGISTRY_USER:-admin@madfam.io}"
REGISTRY_URL="${REGISTRY_URL:-https://npm.madfam.io}"
BASTION="${BASTION:-ssh.madfam.io}"
KX="${KX:-sudo /usr/local/bin/k3s kubectl}"
VAULT_PATH="${VAULT_PATH:-secret/npm-registry}"
VAULT_NS="${VAULT_NS:-vault}"
NS="${NS:-npm-registry}"
ES_NAME="${ES_NAME:-verdaccio-auth}"
SECRET_NAME="${SECRET_NAME:-verdaccio-auth}"
BCRYPT_COST="${BCRYPT_COST:-10}"

step() { printf '\n%b== %s%b\n' "$BLUE" "$1" "$NC"; }
ok()   { printf '%b   OK%b %s\n' "$GREEN" "$NC" "$1"; }
warn() { printf '%b   !!%b %s\n' "$YELLOW" "$NC" "$1"; }
die()  { printf '\n%bFATAL%b %s\n' "$RED" "$NC" "$1" >&2; exit 1; }

# --- 0. preflight -----------------------------------------------------------
step "0/6 Preflight"

command -v ssh >/dev/null || die "ssh is required"
command -v curl >/dev/null || die "curl is required"

HASHER=""
if command -v htpasswd >/dev/null 2>&1; then
  HASHER="htpasswd"
elif [[ -x /usr/sbin/htpasswd ]]; then
  # macOS ships htpasswd in /usr/sbin, which is on PATH for most shells but not
  # all (some minimal PATHs drop it). Name it explicitly rather than failing.
  HASHER="/usr/sbin/htpasswd"
elif python3 -c 'import bcrypt' >/dev/null 2>&1; then
  HASHER="python3-bcrypt"
else
  die "no bcrypt tool found. Install one:
       macOS   — htpasswd ships at /usr/sbin/htpasswd (check your PATH)
       Debian  — sudo apt install apache2-utils
       either  — pip3 install bcrypt"
fi
ok "bcrypt via ${HASHER}"

ssh -o BatchMode=yes -o ConnectTimeout=15 "$BASTION" true 2>/dev/null \
  || die "cannot reach the bastion over 'ssh ${BASTION}'. That tunnel is the only
       authorized path to the cluster — fix it before rotating (a half-done
       rotation leaves Vault ahead of the running pod)."
ok "bastion reachable: ssh ${BASTION}"

VPOD="$(ssh "$BASTION" "$KX -n $VAULT_NS get pod --no-headers" 2>/dev/null \
  | awk '/^vault-0/{print $1; exit}')"
[[ -n "$VPOD" ]] || die "no vault-0 pod found in namespace ${VAULT_NS}"
ok "vault pod: ${VPOD}"

ssh "$BASTION" "$KX -n $NS get externalsecret $ES_NAME" >/dev/null 2>&1 \
  || die "ExternalSecret ${NS}/${ES_NAME} does not exist yet. ArgoCD must sync
       infra/k8s/base/verdaccio first — otherwise this script writes Vault and
       nothing consumes it."
ok "ExternalSecret ${NS}/${ES_NAME} exists"

# --- 1. collect the new password and the Vault token ------------------------
step "1/6 New password and Vault token (both silent, neither is stored)"

printf 'New password for %s: ' "$REGISTRY_USER"
read -rs PW; echo
printf 'Confirm: '
read -rs PW2; echo
[[ "$PW" == "$PW2" ]] || die "passwords do not match"
unset PW2
[[ -n "$PW" ]] || die "empty password"
# Verdaccio's htpasswd backend hands the password to bcrypt, which silently
# truncates at 72 bytes. A longer password would appear to work while only its
# first 72 bytes were ever checked — refuse rather than create that illusion.
[[ "${#PW}" -le 72 ]] || die "password longer than 72 bytes; bcrypt truncates there"
[[ "${#PW}" -ge 16 ]] || warn "password is under 16 characters — this credential is internet-reachable"
ok "password accepted (${#PW} characters, not echoed)"

printf 'Vault token (write on %s): ' "$VAULT_PATH"
read -rs VT; echo
[[ -n "$VT" ]] || die "empty Vault token"
ok "Vault token read"

# --- 2. bcrypt locally ------------------------------------------------------
step "2/6 Generate the htpasswd line locally (plaintext never leaves this host)"

if [[ "$HASHER" == "python3-bcrypt" ]]; then
  # Held in a variable, not a heredoc: a heredoc delimiter nested inside a
  # $( ) command substitution is mis-parsed by bash. The program text therefore
  # contains no single quotes.
  # prefix=b"2y" matches what Apache htpasswd emits and what Verdaccio expects;
  # 2a/2b hash identically but the literal prefix differs.
  BCRYPT_PY='
import bcrypt, os
user = os.environ["REG_USER"]
pw = os.environ["REG_PW"].encode()
cost = int(os.environ["REG_COST"])
h = bcrypt.hashpw(pw, bcrypt.gensalt(rounds=cost, prefix=b"2y")).decode()
print(user + ":" + h)
'
  HTLINE="$(REG_USER="$REGISTRY_USER" REG_PW="$PW" REG_COST="$BCRYPT_COST" \
    python3 -c "$BCRYPT_PY")"
else
  # -n prints to stdout instead of editing a file; -b takes the password as an
  # argument; -B is bcrypt; -C sets the cost. The password appears in this
  # process argv only, for the lifetime of one local invocation, and is never
  # sent anywhere.
  HTLINE="$("$HASHER" -nbB -C "$BCRYPT_COST" "$REGISTRY_USER" "$PW")"
fi

HTLINE="${HTLINE%$'\n'}"
if [[ ! "$HTLINE" =~ ^"$REGISTRY_USER":\$2[aby]\$[0-9]{2}\$ ]]; then
  die "unexpected htpasswd output shape; refusing to write it to Vault"
fi
# Assert the cost actually landed. A cost-5 hash is exactly the weakness this
# rotation exists to retire, so shipping one here would be silent regression.
GOT_COST="$(printf '%s' "$HTLINE" | sed -E 's/^.*\$2[aby]\$([0-9]{2})\$.*$/\1/')"
[[ "$GOT_COST" == "$(printf '%02d' "$BCRYPT_COST")" ]] \
  || die "expected bcrypt cost ${BCRYPT_COST}, got ${GOT_COST}"
ok "bcrypt cost ${GOT_COST} line generated (hash not printed)"

# --- 3. write to Vault over the bastion, value on stdin ---------------------
step "3/6 Write ${VAULT_PATH} #htpasswd (token and value both over stdin)"

# Two lines on stdin: the token, then the hash line. The remote shell reads the
# token with `read -r`, exports it for one command, and feeds the REMAINDER of
# stdin to `vault kv patch … htpasswd=-`. Neither value is ever an argument, so
# neither appears in `ps` output or in any shell history, here or there.
# `kv patch` (not `put`) preserves any other keys already at this path — but
# KV v2 answers 404 to a patch on a path that does not exist yet (the first
# rotation after #529 hit exactly that: the path was never seeded). So: patch
# when the path exists, `put` (create) when it does not. The value still
# travels on stdin either way.
if ! printf '%s\n%s\n' "$VT" "$HTLINE" \
  | ssh "$BASTION" "$KX -n $VAULT_NS exec -i $VPOD -- sh -c 'read -r T; export VAULT_TOKEN=\"\$T\"; if vault kv get $VAULT_PATH >/dev/null 2>&1; then vault kv patch $VAULT_PATH htpasswd=- >/dev/null; else vault kv put $VAULT_PATH htpasswd=- >/dev/null; fi'"
then
  die "vault kv write failed (token lacks write on ${VAULT_PATH}?). Nothing changed:
       the registry still accepts the OLD password. Safe to re-run."
fi
ok "Vault updated"

# Read back the cost only — never the hash — to prove the write landed.
VERIFY_COST="$(printf '%s\n' "$VT" \
  | ssh "$BASTION" "$KX -n $VAULT_NS exec -i $VPOD -- sh -c 'read -r T; VAULT_TOKEN=\"\$T\" vault kv get -field=htpasswd $VAULT_PATH'" \
  | sed -E 's/^.*\$2[aby]\$([0-9]{2})\$.*$/\1/')"
[[ "$VERIFY_COST" == "$GOT_COST" ]] \
  || die "read-back cost (${VERIFY_COST}) does not match what was written (${GOT_COST})"
ok "read-back confirms the new hash is at ${VAULT_PATH} #htpasswd"
unset VT

# --- 4. force ESO to resync, wait for the Secret to change ------------------
step "4/6 Wait for ESO to sync the Secret"

BEFORE_RV="$(ssh "$BASTION" "$KX -n $NS get secret $SECRET_NAME -o jsonpath='{.metadata.resourceVersion}'" 2>/dev/null || echo "")"

# refreshInterval is 15m; this annotation makes ESO reconcile now instead.
ssh "$BASTION" "$KX -n $NS annotate externalsecret $ES_NAME force-sync=\"\$(date +%s)\" --overwrite" >/dev/null \
  || warn "could not annotate the ExternalSecret; falling back to the 15m refreshInterval"

SYNCED=false
for _ in $(seq 1 60); do
  sleep 5
  COND="$(ssh "$BASTION" "$KX -n $NS get externalsecret $ES_NAME -o jsonpath='{.status.conditions}'" 2>/dev/null || echo "")"
  NOW_RV="$(ssh "$BASTION" "$KX -n $NS get secret $SECRET_NAME -o jsonpath='{.metadata.resourceVersion}'" 2>/dev/null || echo "")"
  # ESO reports one Ready condition; SecretSynced + True is the healthy pair.
  # The resourceVersion check is what proves this rotation's value landed
  # rather than an older sync that was already True before we started.
  if [[ "$COND" == *SecretSynced* && "$COND" == *'"status":"True"'* \
        && -n "$NOW_RV" && "$NOW_RV" != "$BEFORE_RV" ]]; then
    SYNCED=true
    break
  fi
done
if [[ "$SYNCED" == true ]]; then
  ok "Secret ${NS}/${SECRET_NAME} re-materialized from Vault"
else
  warn "ESO did not report a changed Secret within 5 minutes. Check:
       ssh ${BASTION} '${KX} -n ${NS} get externalsecret ${ES_NAME} -o jsonpath={.status.conditions}'
       Continuing — the pod check below is the real gate."
fi

# --- 5. wait for the pod (Recreate + Reloader) ------------------------------
step "5/6 Wait for the verdaccio pod to come back"

# strategy: Recreate on an RWO Longhorn PVC — the old pod terminates fully
# before the new one attaches. There is a window with NO pod, so poll for a
# Running+Ready one rather than assuming a rollout is in flight.
# Reloader restarts the pod when the Secret changes — but only if the
# Deployment carries the annotation (it did not until 2026-09-07, and a pod
# that predates the Secret keeps serving the kubelet's stale projection while
# reporting Ready). So: compare the pod's creation time with the Secret's and
# restart explicitly when the pod is older. Recreate ⇒ ~30 s without a pod.
POD_TS="$(ssh "$BASTION" "$KX -n $NS get pod -l app.kubernetes.io/name=verdaccio -o jsonpath='{.items[0].metadata.creationTimestamp}'" 2>/dev/null || echo "")"
SEC_TS="$(ssh "$BASTION" "$KX -n $NS get secret verdaccio-auth -o jsonpath='{.metadata.creationTimestamp}'" 2>/dev/null || echo "")"
if [[ -n "$POD_TS" && -n "$SEC_TS" && "$POD_TS" < "$SEC_TS" ]]; then
  warn "pod (${POD_TS}) predates the Secret (${SEC_TS}) — Reloader did not fire; restarting explicitly"
  ssh "$BASTION" "$KX -n $NS rollout restart deploy/verdaccio" >/dev/null \
    || die "rollout restart failed — run it by hand:
       ssh ${BASTION} '${KX} -n ${NS} rollout restart deploy/verdaccio'"
  sleep 10
fi
READY=false
for _ in $(seq 1 60); do
  sleep 5
  PHASE="$(ssh "$BASTION" "$KX -n $NS get pod -l app.kubernetes.io/name=verdaccio -o jsonpath='{.items[0].status.phase}'" 2>/dev/null || echo "")"
  RDY="$(ssh "$BASTION" "$KX -n $NS get pod -l app.kubernetes.io/name=verdaccio -o jsonpath='{.items[0].status.containerStatuses[0].ready}'" 2>/dev/null || echo "")"
  if [[ "$PHASE" == "Running" && "$RDY" == "true" ]]; then READY=true; break; fi
done
[[ "$READY" == true ]] || warn "verdaccio pod not Ready after 5 minutes — check:
       ssh ${BASTION} '${KX} -n ${NS} get pod -l app.kubernetes.io/name=verdaccio'
       A Multi-Attach error here means the PVC did not detach; see
       docs/runbooks/LONGHORN_VOLUME_RECOVERY.md."
[[ "$READY" == true ]] && ok "verdaccio pod Running and Ready"

# --- 6. verify against the live registry ------------------------------------
step "6/6 Verify the new credential against ${REGISTRY_URL}"

WHOAMI_OK=false
for _ in $(seq 1 12); do
  BODY="$(curl -fsS --max-time 15 -u "${REGISTRY_USER}:${PW}" "${REGISTRY_URL}/-/whoami" 2>/dev/null || echo "")"
  # -f makes curl exit non-zero on 401/403, so a non-empty BODY already means
  # 200; naming the user as well guards against a cached anonymous response.
  if [[ "$BODY" == *"$REGISTRY_USER"* ]]; then
    WHOAMI_OK=true
    break
  fi
  sleep 10
done
unset PW

if [[ "$WHOAMI_OK" == true ]]; then
  ok "/-/whoami returned 200 and named ${REGISTRY_USER}"
else
  die "/-/whoami did not accept the new credential.
       Vault holds the new hash, so re-running is safe and idempotent. If it
       keeps failing, the pod may still be serving the old htpasswd file:
         ssh ${BASTION} '${KX} -n ${NS} rollout restart deploy/verdaccio'"
fi

cat <<EOF

${GREEN}Rotation complete.${NC} The bcrypt hash published in git before 2026-09-07
no longer verifies anything.

Log in from your workstation with the password you just chose:

  npm login --registry ${REGISTRY_URL} --auth-type=legacy

(\`--auth-type=legacy\` is required: it sends username/password to the htpasswd
backend. Without it npm attempts the web login flow, which this registry does
not serve.)
EOF
