#!/bin/bash
# porkbun-tenant-credentials.sh — load ONE tenant's Porkbun registrar API key
# pair into Vault through Enclii, then verify it against the live Porkbun API.
#
# WHY THIS EXISTS
#
# Porkbun API keys are scoped to one Porkbun ACCOUNT. MADFAM's global
# ENCLII_PORKBUN_* pair operates the estate's own domains and is structurally
# unable to touch a domain a client holds in the client's own Porkbun login —
# Porkbun answers INVALID_DOMAIN, which reads exactly like a typo. Loading the
# client's own key pair is therefore not a convenience; it is the only way
# Enclii can operate that registrar at all.
#
# creatumundo.mx is the live case: transferred into CTM's own Porkbun account
# on 2026-09-05.
#
# WHAT THIS SCRIPT DOES NOT DO
#
# It never echoes, logs, or writes a credential value to disk. The two values
# are read with a silent prompt, handed to `enclii secrets intake submit` over
# stdin, and never appear in your terminal, your shell history, or this script's
# output. The assistant that told you to run this never sees them either.
#
# It also performs no production mutation. Writing a credential the platform
# will use is not a registrar change — nothing about the domain moves until you
# separately run a `--apply` operation.
#
# USAGE
#
#   scripts/operator/porkbun-tenant-credentials.sh                  # tenant: crea
#   ENCLII_TENANT=crea scripts/operator/porkbun-tenant-credentials.sh
#   ENCLII_TENANT=crea VERIFY_DOMAIN=creatumundo.mx \
#     scripts/operator/porkbun-tenant-credentials.sh
#
# PREREQUISITES
#
#   1. `enclii` on PATH and logged in (`enclii auth login`) — this is the ONE
#      token you supply. Switchyard writes Vault on your behalf; you never
#      handle a Vault token.
#   2. The switchyard-secret-writer Vault policy must have been RE-APPLIED
#      since this tenant's path was added to
#      scripts/provision-switchyard-vault-writer.sh. CI proves the block is in
#      git; it cannot prove the running Vault has it. Symptom if it was not:
#      step 3 or 4 fails with a permission error or "credentials missing".
#        VAULT_TOKEN=<admin> POLICY_ONLY=1 \
#          bash scripts/provision-switchyard-vault-writer.sh
#   3. HUMAN STEP, in the CLIENT's Porkbun dashboard, once per domain:
#      Domain Management → the domain → Details → enable "API Access".
#      Porkbun refuses every API call for a domain that has not been opted in,
#      and reports it identically to a bad key. Step 4 below tells the two
#      apart for you.

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

TENANT="${ENCLII_TENANT:-crea}"
INTAKE_TARGET="${TENANT}/porkbun-registrar"
VERIFY_DOMAIN="${VERIFY_DOMAIN:-}"
DEFAULT_REASON="load ${TENANT} Porkbun registrar credentials so Enclii can operate the tenant's own registrar account"
REASON="${REASON:-$DEFAULT_REASON}"

echo "================================================"
echo "  Porkbun registrar credentials — tenant: ${TENANT}"
echo "================================================"
echo ""

for cmd in enclii; do
    if ! command -v "$cmd" &> /dev/null; then
        echo -e "${RED}ERROR: required command '$cmd' not found on PATH${NC}"
        exit 1
    fi
done

# ---------------------------------------------------------------------------
# 1. Confirm the intake target exists before asking for anything secret.
#
# Asking for a credential and only then discovering the target is unregistered
# would leave the operator holding a live secret with nowhere to put it.
# ---------------------------------------------------------------------------
echo -e "${BLUE}[1/4]${NC} Checking intake target ${INTAKE_TARGET}..."
if ! enclii secrets intake targets --json 2>/dev/null | grep -q "\"${INTAKE_TARGET}\""; then
    echo -e "${RED}ERROR: intake target '${INTAKE_TARGET}' is not registered.${NC}"
    echo ""
    echo "Add it to apps/switchyard-api/internal/secretsintake/registry.yaml and"
    echo "deploy switchyard-api, then rerun. Registered targets:"
    enclii secrets intake targets || true
    exit 1
fi
echo -e "  ${GREEN}found${NC}"
echo ""

# ---------------------------------------------------------------------------
# 2. Read the pair. Silent prompts; values never touch the screen or a file.
# ---------------------------------------------------------------------------
echo -e "${BLUE}[2/4]${NC} Enter ${TENANT}'s Porkbun API credentials."
echo "  Get them from the CLIENT's Porkbun account: Account → API Access."
echo "  Input is hidden and is never printed, logged, or saved to disk."
echo ""

read -rs -p "  Porkbun API key (pk1_...): " PORKBUN_API_KEY
echo ""
read -rs -p "  Porkbun secret key (sk1_...): " PORKBUN_SECRET_KEY
echo ""
echo ""

if [[ -z "${PORKBUN_API_KEY}" || -z "${PORKBUN_SECRET_KEY}" ]]; then
    echo -e "${RED}ERROR: both values are required — Porkbun authenticates with the pair.${NC}"
    unset PORKBUN_API_KEY PORKBUN_SECRET_KEY
    exit 1
fi

# ---------------------------------------------------------------------------
# 3. Hand them to Switchyard, which merges them into Vault.
#
# Piped on stdin so neither value ever becomes a process argument (visible in
# `ps`) or a shell-history entry.
# ---------------------------------------------------------------------------
echo -e "${BLUE}[3/4]${NC} Submitting to Enclii (values are written straight to Vault)..."
INTAKE_OUTPUT=$(
    printf 'porkbun_api_key=%s\nporkbun_secret_key=%s\n' "${PORKBUN_API_KEY}" "${PORKBUN_SECRET_KEY}" |
        enclii secrets intake submit "${INTAKE_TARGET}" --stdin --reason "${REASON}"
) || {
    echo -e "${RED}ERROR: intake submission failed.${NC}"
    unset PORKBUN_API_KEY PORKBUN_SECRET_KEY
    exit 1
}

# Drop the values from this shell the moment they are no longer needed.
unset PORKBUN_API_KEY PORKBUN_SECRET_KEY

echo "${INTAKE_OUTPUT}"
echo -e "  ${GREEN}written${NC}"
echo ""

# ---------------------------------------------------------------------------
# 4. Verify — read-only, through Enclii, against the live Porkbun API.
#
# `ping` proves the key pair itself works without naming a domain. `domains`
# then shows what the account holds and, per domain, whether the dashboard's
# per-domain API Access toggle is on. Those are the two failures Porkbun
# otherwise reports identically.
# ---------------------------------------------------------------------------
echo -e "${BLUE}[4/4]${NC} Verifying the credentials through Enclii (read-only)..."
if ! enclii providers porkbun ping --tenant "${TENANT}"; then
    echo ""
    echo -e "${YELLOW}The credential pair did not validate.${NC}"
    echo "Most likely the key or secret was mistyped, or the key was created in a"
    echo "different Porkbun account than the one holding the tenant's domains."
    echo "Rerun this script to replace them."
    exit 1
fi
echo ""

echo "  Domains visible to this credential scope:"
enclii providers porkbun domains --tenant "${TENANT}" || true
echo ""

if [[ -n "${VERIFY_DOMAIN}" ]]; then
    echo "  Registrar state for ${VERIFY_DOMAIN}:"
    enclii providers porkbun nameservers "${VERIFY_DOMAIN}" --tenant "${TENANT}" || true
    echo ""
fi

echo "================================================"
echo -e "  ${GREEN}Done.${NC}"
echo "================================================"
echo ""
echo "If a domain you expect is missing above, or an operation later fails with"
echo "INVALID_DOMAIN, the cause is almost always the per-domain toggle:"
echo ""
echo "  Porkbun dashboard → Domain Management → <domain> → Details → API Access"
echo ""
echo "That toggle is a human step in the CLIENT's account; nothing in Enclii can"
echo "flip it. 'apiAccess: 0' in 'enclii providers porkbun renewals --tenant"
echo "${TENANT}' is the same finding."
echo ""
echo "Next, all dry-run first:"
echo "  enclii providers porkbun renewals --tenant ${TENANT}"
echo "  enclii providers porkbun nameservers-apply <domain> --tenant ${TENANT} \\"
echo "      --nameservers <ns1>,<ns2>"
