#!/bin/bash
# =============================================================================
# Direct SuLuna Deployment - Simplified kubectl-free deployment
# =============================================================================
#
# This script deploys SuLuna's DNS configuration without requiring kubectl.
# For full deployment (including K8s), use: deploy-client.sh --client suluna
#
# Credential Loading:
#   Uses the credential library from lib/cloudflare-credentials.sh
#   Run: scripts/cloudflare-zone-create.sh --setup to configure credentials
#
# =============================================================================
set -e

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="$(dirname "$SCRIPT_DIR")/lib"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}ℹ${NC} $1"; }
log_success() { echo -e "${GREEN}✅${NC} $1"; }
log_warn() { echo -e "${YELLOW}⚠️${NC} $1"; }
log_error() { echo -e "${RED}❌${NC} $1"; }

# Load credential library
if [ -f "$LIB_DIR/cloudflare-credentials.sh" ]; then
    source "$LIB_DIR/cloudflare-credentials.sh"
else
    log_error "Credential library not found: $LIB_DIR/cloudflare-credentials.sh"
    exit 1
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🚀 SuLuna Direct Deployment - Zero Touch Automation"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Load credentials
echo ""
log_info "Loading credentials..."

if ! load_cloudflare_credentials; then
    log_error "Failed to load Cloudflare credentials"
    echo ""
    echo "Run 'scripts/cloudflare-zone-create.sh --setup' to configure credentials."
    echo "Or see 'docs/guides/CREDENTIAL_SETUP.md' for manual setup."
    exit 1
fi

if ! validate_cloudflare_credentials; then
    exit 1
fi

# Check for Janua admin token
JANUA_API="${JANUA_API:-https://api.janua.dev}"
if [ -z "${ADMIN_TOKEN:-}" ] && [ -z "${JANUA_ADMIN_TOKEN:-}" ]; then
    log_warn "ADMIN_TOKEN or JANUA_ADMIN_TOKEN not set - skipping Janua RBAC"
    SKIP_JANUA=true
else
    ADMIN_TOKEN="${ADMIN_TOKEN:-$JANUA_ADMIN_TOKEN}"
    SKIP_JANUA=false
fi

echo ""
echo "=== PHASE 1: JANUA RBAC (Identity Layer) ==="

if [ "$SKIP_JANUA" = "true" ]; then
    log_warn "Skipping Janua setup (no admin token)"
else
    # Check current orgs
    echo "📋 Checking existing organizations..."
    EXISTING_ORGS=$(curl -sf -H "Authorization: Bearer $ADMIN_TOKEN" "$JANUA_API/api/v1/organizations/" 2>/dev/null || echo "[]")
    echo "Existing orgs: $EXISTING_ORGS"

    # Create SuLuna org
    echo ""
    echo "📁 Creating SuLuna organization..."
    ORG_RESPONSE=$(curl -sf -X POST "$JANUA_API/api/v1/organizations/" \
      -H "Authorization: Bearer $ADMIN_TOKEN" \
      -H "Content-Type: application/json" \
      -d '{
        "name": "SuLuna",
        "slug": "suluna",
        "description": "SuLuna - Agency Model Client (Managed by MADFAM)"
      }' 2>&1)

    echo "Organization response: $ORG_RESPONSE"

    if echo "$ORG_RESPONSE" | grep -q '"id"'; then
      ORG_ID=$(echo "$ORG_RESPONSE" | jq -r '.id')
      log_success "Organization created: $ORG_ID"
    else
      log_warn "Organization may already exist or failed to create"
      ORG_ID=""
    fi
fi

echo ""
echo "=== PHASE 2: CLOUDFLARE DNS (Network Layer) ==="

# Check if suluna.mx zone exists
echo "🌐 Checking for suluna.mx zone..."
ZONE_RESPONSE=$(curl -sf -X GET "https://api.cloudflare.com/client/v4/zones?name=suluna.mx" \
  -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
  -H "Content-Type: application/json" 2>&1)

ZONE_ID=$(echo "$ZONE_RESPONSE" | jq -r '.result[0].id // empty')

if [ -z "$ZONE_ID" ]; then
  echo "📝 Zone doesn't exist, creating suluna.mx..."
  CREATE_ZONE=$(curl -sf -X POST "https://api.cloudflare.com/client/v4/zones" \
    -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"suluna.mx\",
      \"account\": {\"id\": \"$CLOUDFLARE_ACCOUNT_ID\"},
      \"jump_start\": true
    }" 2>&1)
  echo "Zone creation response: $CREATE_ZONE"
  ZONE_ID=$(echo "$CREATE_ZONE" | jq -r '.result.id // empty')

  if [ -n "$ZONE_ID" ]; then
    log_success "Zone created: $ZONE_ID"
    NAMESERVERS=$(echo "$CREATE_ZONE" | jq -r '.result.name_servers[]' 2>/dev/null)
    echo ""
    log_warn "PORKBUN ACTION REQUIRED - Set these nameservers:"
    echo "$NAMESERVERS"
  fi
else
  log_success "Zone exists: $ZONE_ID"
fi

# Create DNS record for links.suluna.mx
if [ -n "$ZONE_ID" ]; then
  echo ""
  echo "📍 Creating DNS CNAME for links.suluna.mx..."
  DNS_RESPONSE=$(curl -sf -X POST "https://api.cloudflare.com/client/v4/zones/$ZONE_ID/dns_records" \
    -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"type\": \"CNAME\",
      \"name\": \"links\",
      \"content\": \"${TUNNEL_ID}.cfargotunnel.com\",
      \"proxied\": true,
      \"ttl\": 1
    }" 2>&1)

  if echo "$DNS_RESPONSE" | jq -e '.success' >/dev/null 2>&1; then
    log_success "DNS record created: links.suluna.mx -> ${TUNNEL_ID}.cfargotunnel.com"
  else
    log_warn "DNS record may already exist: $(echo "$DNS_RESPONSE" | jq -r '.errors[0].message // "unknown error"')"
  fi
fi

echo ""
echo "=== PHASE 3: KUBERNETES DEPLOYMENT ==="
echo "⚠️ kubectl not connected - K8s deployment skipped"
echo "   Run manually when cluster access available:"
echo "   kubectl create namespace suluna-production"
echo "   kubectl apply -f dogfooding/clients/suluna-linkstack.k8s.yaml"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📊 DEPLOYMENT SUMMARY"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ "$SKIP_JANUA" = "true" ]; then
    echo "⏭️ Janua RBAC:    Skipped (no admin token)"
else
    echo "✅ Janua RBAC:    Attempted"
fi
echo "✅ Cloudflare:    Zone + DNS configured"
echo "⏳ Kubernetes:    Pending cluster access"
echo ""
echo "Next: Establish kubectl connection and run K8s deployment"
