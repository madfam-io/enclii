#!/usr/bin/env bash
#
# Apply the switchyard-secret-writer Vault policy to the LIVE Vault, over the
# authorized SSH bastion + on-node k3s kubectl (the sandbox has no cluster-wired
# kubectl). POLICY-ONLY: writes the policy, rotates nothing. Existing
# switchyard-secret-writer tokens inherit the updated paths.
#
# WHY THIS EXISTS AS A COMMITTED SCRIPT
# ====================================
# CI's intake/policy parity gate proves a path block exists IN GIT. It cannot
# prove the policy was APPLIED to the running Vault, and merging does not apply
# it. Until an operator runs this, the first call that needs a newly added path
# 403s and surfaces as "credentials missing" — indistinguishable from never
# having run the intake at all. Two live cases so far:
#   - `enclii secrets provision oidc --platform lexidrop` (secret/data/lexidrop,
#     added in enclii #471);
#   - `enclii providers porkbun ... --tenant crea` (secret/data/crea, added in
#     enclii #527), which Switchyard READS back on every registrar operation.
#
# ASSERTED PATH
# =============
# After writing, the script re-reads the live policy and proves ONE path landed.
# Which one is `ASSERT_PATH` (default `crea`, the current live case):
#   ASSERT_PATH=lexidrop bash scripts/apply-switchyard-vault-policy-remote.sh
#
# THE ADMIN TOKEN IS HANDLED WITHOUT EVER TOUCHING argv, history, disk, or a log:
#   - read with `read -rs` (silent; not a command argument, so not in history);
#   - carried to the vault pod as the FIRST LINE OF STDIN, consumed by a
#     `read` INSIDE the pod that exports VAULT_TOKEN there. It is never a
#     kubectl/env/vault argv token on the host or in the pod, so it cannot
#     appear in `ps`, the SSH command line, or Vault's own audit of argv.
#   - scrubbed from this shell on exit.
#
# You just run this and paste the token when prompted:
#   bash scripts/apply-switchyard-vault-policy-remote.sh
#
# THE TOKEN IS TAKEN ON STDIN, NEVER IN argv. Do not add a --token flag or a
# VAULT_TOKEN=... prefix: both put the credential where `ps` and the shell
# history can read it, which is the whole thing this script is shaped to avoid.
#
# Last Updated: 2026-09-07
set -euo pipefail

SSH_HOST="${SSH_HOST:-ssh.madfam.io}"
KCTL="${KCTL:-sudo /usr/local/bin/k3s kubectl}"
VAULT_NS="${VAULT_NS:-vault}"
VAULT_POD="${VAULT_POD:-vault-0}"
# Target the pod's OWN listener from inside the pod. The ClusterIP service
# (vault.vault.svc.cluster.local) refuses connections looped back from vault-0
# itself; the pod listens on http://127.0.0.1:8200 (tls_disable=1) and that is
# what `vault status` uses by default in-pod.
VAULT_ADDR="${VAULT_ADDR:-http://127.0.0.1:8200}"
POLICY_NAME="${POLICY_NAME:-switchyard-secret-writer}"
# Which secret/data/<path> to prove landed in the live policy after the write.
ASSERT_PATH="${ASSERT_PATH:-crea}"

# Locate the canonical policy source (this same repo). We reuse the HCL block
# embedded in provision-switchyard-vault-writer.sh so the policy never drifts
# from the reviewed source of truth.
#
# Resolve THIS script's REAL path first, following symlinks, so it works when
# invoked by absolute path, through a PATH symlink, or via an alias/function —
# from any working directory. (A bare `dirname "$BASH_SOURCE"` points at the
# symlink's dir, not the repo, and then the sibling source is not found.)
_resolve_self() {
  local src="${BASH_SOURCE[0]}" dir
  while [ -h "$src" ]; do
    dir="$(cd -P "$(dirname "$src")" >/dev/null 2>&1 && pwd)"
    src="$(readlink "$src")"
    case "$src" in /*) : ;; *) src="$dir/$src" ;; esac
  done
  cd -P "$(dirname "$src")" >/dev/null 2>&1 && pwd
}
SCRIPT_DIR="$(_resolve_self)"
SRC="${SRC:-$SCRIPT_DIR/provision-switchyard-vault-writer.sh}"
[ -r "$SRC" ] || { echo "[FAIL] cannot read policy source: $SRC" >&2; exit 1; }

# Extract just the HCL heredoc body (between `cat >"$POLICY_HCL" <<'EOF'` and EOF).
POLICY_HCL="$(awk '/^cat >"\$POLICY_HCL" <<.EOF.$/{f=1;next} f&&/^EOF$/{f=0} f' "$SRC")"
[ -n "$POLICY_HCL" ] || { echo "[FAIL] could not extract policy HCL from $SRC" >&2; exit 1; }
if ! printf '%s\n' "$POLICY_HCL" | grep -q "path \"secret/data/${ASSERT_PATH}\""; then
  echo "[FAIL] extracted policy is missing secret/data/${ASSERT_PATH} — is this checkout current with main?" >&2
  exit 1
fi
echo "[INFO] policy source: $SRC"
echo "[INFO] data paths in policy: $(printf '%s\n' "$POLICY_HCL" | grep -cE 'path "secret/data/')  (incl. secret/data/${ASSERT_PATH})"

# --- Read the admin token silently. Not an argument => not in shell history. ---
printf '>> Paste the Vault admin token (input hidden), then press Enter: '
read -rs VAULT_ADMIN_TOKEN
printf '\n'
[ -n "$VAULT_ADMIN_TOKEN" ] || { echo "[FAIL] empty token" >&2; exit 1; }
# Guarantee the token leaves this shell no matter how we exit.
trap 'VAULT_ADMIN_TOKEN=; unset VAULT_ADMIN_TOKEN' EXIT

echo "[INFO] applying policy '${POLICY_NAME}' to ${VAULT_ADDR} via ${SSH_HOST} (pod ${VAULT_NS}/${VAULT_POD})..."

# Feed TWO things to the remote shell over ONE stdin stream:
#   line 1 : the admin token   (consumed by `read` inside the vault pod)
#   rest   : the policy HCL     (written to the pod's /tmp, then `vault policy write`)
# The remote pipeline does the token read INSIDE the pod, so VAULT_TOKEN is set
# in the pod's process env from its own stdin — never a host/pod argv token.
{
  printf '%s\n' "$VAULT_ADMIN_TOKEN"
  printf '%s\n' "$POLICY_HCL"
} | ssh "$SSH_HOST" "$KCTL exec -i -n '$VAULT_NS' '$VAULT_POD' -- sh -c '
    set -e
    export VAULT_ADDR=\"$VAULT_ADDR\"
    IFS= read -r VT            # first stdin line = admin token
    export VAULT_TOKEN=\"\$VT\"
    VT=
    cat > /tmp/${POLICY_NAME}.hcl   # remaining stdin = policy HCL
    vault policy write ${POLICY_NAME} /tmp/${POLICY_NAME}.hcl
    # Read-proof the applied policy WITHOUT printing the token: confirm the
    # asserted path is now present in the live policy.
    if vault policy read ${POLICY_NAME} | grep -q \"secret/data/${ASSERT_PATH}\"; then
      echo APPLIED_OK_asserted_path_present
    else
      echo APPLIED_BUT_asserted_path_MISSING
    fi
    rm -f /tmp/${POLICY_NAME}.hcl
    unset VAULT_TOKEN
  '"

echo "[INFO] done. If you saw APPLIED_OK_asserted_path_present above, the live policy now permits secret/data/${ASSERT_PATH}."
