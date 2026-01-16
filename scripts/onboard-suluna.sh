#!/bin/bash
set -euo pipefail

# =============================================================================
# SuLuna Client Onboarding Script
# Agency Model: MADFAM manages client infrastructure
# =============================================================================
# Usage:
#   export ADMIN_TOKEN="your-jwt-token"
#   ./scripts/onboard-suluna.sh
#
# Prerequisites:
#   - Admin JWT token from Janua (admin@madfam.io)
#   - curl and jq installed
# =============================================================================

JANUA_API="${JANUA_API:-https://api.janua.dev}"
CLIENT_EMAIL="suluna.mx@gmail.com"
CLIENT_ORG_NAME="SuLuna"
CLIENT_ORG_SLUG="suluna"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🔐 SuLuna Client Onboarding - Agency Model Deployment"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Check for admin token
if [ -z "${ADMIN_TOKEN:-}" ]; then
    echo -e "${RED}❌ ADMIN_TOKEN environment variable required${NC}"
    echo ""
    echo "Get your token by logging in:"
    echo "  curl -X POST $JANUA_API/api/v1/auth/login \\"
    echo "    -H 'Content-Type: application/json' \\"
    echo "    -d '{\"email\": \"admin@madfam.io\", \"password\": \"your-password\"}'"
    echo ""
    echo "Then export it:"
    echo "  export ADMIN_TOKEN=\"<access_token from response>\""
    exit 1
fi

# Check for required tools
command -v curl >/dev/null 2>&1 || { echo -e "${RED}❌ curl is required${NC}"; exit 1; }
command -v jq >/dev/null 2>&1 || { echo -e "${RED}❌ jq is required${NC}"; exit 1; }

# =============================================================================
# Step 0: Verify admin credentials
# =============================================================================
echo "📋 Verifying admin credentials..."

ADMIN_RESPONSE=$(curl -sf -H "Authorization: Bearer $ADMIN_TOKEN" \
  "$JANUA_API/api/v1/auth/me" 2>/dev/null || echo '{"error": "auth failed"}')

if echo "$ADMIN_RESPONSE" | jq -e '.error' >/dev/null 2>&1; then
    echo -e "${RED}❌ Authentication failed. Check your ADMIN_TOKEN.${NC}"
    exit 1
fi

ADMIN_EMAIL=$(echo "$ADMIN_RESPONSE" | jq -r '.email')
ADMIN_USER_ID=$(echo "$ADMIN_RESPONSE" | jq -r '.id')

if [ "$ADMIN_EMAIL" != "admin@madfam.io" ]; then
    echo -e "${YELLOW}⚠️  Expected admin@madfam.io, got: $ADMIN_EMAIL${NC}"
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi
echo -e "${GREEN}✅ Authenticated as: $ADMIN_EMAIL${NC}"
echo ""

# =============================================================================
# Step 1: Check if organization already exists
# =============================================================================
echo "📁 Checking if $CLIENT_ORG_NAME organization exists..."

EXISTING_ORGS=$(curl -sf -H "Authorization: Bearer $ADMIN_TOKEN" \
  "$JANUA_API/api/v1/organizations/" 2>/dev/null || echo '[]')

EXISTING_ORG_ID=$(echo "$EXISTING_ORGS" | jq -r ".[] | select(.slug == \"$CLIENT_ORG_SLUG\") | .id")

if [ -n "$EXISTING_ORG_ID" ]; then
    echo -e "${YELLOW}⚠️  Organization '$CLIENT_ORG_NAME' already exists (ID: $EXISTING_ORG_ID)${NC}"
    read -p "Skip organization creation and continue? (Y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Nn]$ ]]; then
        exit 1
    fi
    ORG_ID="$EXISTING_ORG_ID"
else
    # Create organization
    echo "Creating $CLIENT_ORG_NAME organization..."
    
    ORG_RESPONSE=$(curl -sf -X POST "$JANUA_API/api/v1/organizations/" \
      -H "Authorization: Bearer $ADMIN_TOKEN" \
      -H "Content-Type: application/json" \
      -d "{
        \"name\": \"$CLIENT_ORG_NAME\",
        \"slug\": \"$CLIENT_ORG_SLUG\",
        \"description\": \"$CLIENT_ORG_NAME - Managed Services Client\",
        \"billing_email\": \"$CLIENT_EMAIL\"
      }" 2>/dev/null)
    
    if echo "$ORG_RESPONSE" | jq -e '.detail' >/dev/null 2>&1; then
        echo -e "${RED}❌ Failed to create organization:${NC}"
        echo "$ORG_RESPONSE" | jq -r '.detail'
        exit 1
    fi
    
    ORG_ID=$(echo "$ORG_RESPONSE" | jq -r '.id')
    echo -e "${GREEN}✅ Organization created: $ORG_ID${NC}"
fi
echo ""

# =============================================================================
# Step 2: Create Managed_Services custom role
# =============================================================================
echo "👥 Creating Managed_Services custom role..."

# Check if role already exists
EXISTING_ROLES=$(curl -sf -H "Authorization: Bearer $ADMIN_TOKEN" \
  "$JANUA_API/api/v1/organizations/$ORG_ID/roles" 2>/dev/null || echo '[]')

EXISTING_ROLE_ID=$(echo "$EXISTING_ROLES" | jq -r '.[] | select(.name == "Managed_Services") | .id')

if [ -n "$EXISTING_ROLE_ID" ] && [ "$EXISTING_ROLE_ID" != "null" ]; then
    echo -e "${YELLOW}⚠️  Managed_Services role already exists (ID: $EXISTING_ROLE_ID)${NC}"
    ROLE_ID="$EXISTING_ROLE_ID"
else
    ROLE_RESPONSE=$(curl -sf -X POST "$JANUA_API/api/v1/organizations/$ORG_ID/roles" \
      -H "Authorization: Bearer $ADMIN_TOKEN" \
      -H "Content-Type: application/json" \
      -d '{
        "name": "Managed_Services",
        "description": "MADFAM managed services team - full infrastructure access",
        "permissions": [
          "services:read", "services:write", "services:deploy", "services:delete",
          "logs:read", "metrics:read", "secrets:read", "secrets:write",
          "domains:manage", "billing:read"
        ]
      }' 2>/dev/null)
    
    if echo "$ROLE_RESPONSE" | jq -e '.detail' >/dev/null 2>&1; then
        echo -e "${RED}❌ Failed to create role:${NC}"
        echo "$ROLE_RESPONSE" | jq -r '.detail'
        exit 1
    fi
    
    ROLE_ID=$(echo "$ROLE_RESPONSE" | jq -r '.id')
    echo -e "${GREEN}✅ Managed_Services role created: $ROLE_ID${NC}"
fi
echo ""

# =============================================================================
# Step 3: Invite client as owner
# =============================================================================
echo "📧 Inviting $CLIENT_EMAIL as organization owner..."

# Check for existing invitation
EXISTING_INVITES=$(curl -sf -H "Authorization: Bearer $ADMIN_TOKEN" \
  "$JANUA_API/api/v1/organizations/$ORG_ID/invitations?status=pending" 2>/dev/null || echo '[]')

EXISTING_INVITE=$(echo "$EXISTING_INVITES" | jq -r ".[] | select(.email == \"$CLIENT_EMAIL\") | .id")

if [ -n "$EXISTING_INVITE" ] && [ "$EXISTING_INVITE" != "null" ]; then
    echo -e "${YELLOW}⚠️  Pending invitation already exists for $CLIENT_EMAIL (ID: $EXISTING_INVITE)${NC}"
    INVITE_ID="$EXISTING_INVITE"
else
    INVITE_RESPONSE=$(curl -sf -X POST "$JANUA_API/api/v1/organizations/$ORG_ID/invite" \
      -H "Authorization: Bearer $ADMIN_TOKEN" \
      -H "Content-Type: application/json" \
      -d "{
        \"email\": \"$CLIENT_EMAIL\",
        \"role\": \"owner\",
        \"permissions\": [],
        \"message\": \"Welcome to your $CLIENT_ORG_NAME dashboard! You have full ownership of your organization. MADFAM provides managed services for your infrastructure.\"
      }" 2>/dev/null)
    
    if echo "$INVITE_RESPONSE" | jq -e '.detail' >/dev/null 2>&1; then
        echo -e "${RED}❌ Failed to send invitation:${NC}"
        echo "$INVITE_RESPONSE" | jq -r '.detail'
        exit 1
    fi
    
    INVITE_ID=$(echo "$INVITE_RESPONSE" | jq -r '.invitation_id')
    echo -e "${GREEN}✅ Invitation sent: $INVITE_ID${NC}"
fi
echo ""

# =============================================================================
# Summary
# =============================================================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${GREEN}🎉 SuLuna Onboarding Complete!${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Organization Details:"
echo "  ID:    $ORG_ID"
echo "  Name:  $CLIENT_ORG_NAME"
echo "  Slug:  $CLIENT_ORG_SLUG"
echo ""
echo "Role Details:"
echo "  ID:    $ROLE_ID"
echo "  Name:  Managed_Services"
echo ""
echo "Invitation:"
echo "  ID:    $INVITE_ID"
echo "  Email: $CLIENT_EMAIL"
echo "  Role:  owner"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Next Steps:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "1. Wait for client to accept invitation email"
echo ""
echo "2. Create Kubernetes namespace:"
echo "   kubectl create namespace suluna-production"
echo "   kubectl label namespace suluna-production client=suluna"
echo ""
echo "3. Deploy LinkStack:"
echo "   enclii service create --file dogfooding/clients/suluna-linkstack.yaml"
echo ""
echo "4. Add Cloudflare Tunnel route (links.suluna.mx)"
echo ""
echo "5. Verify deployment:"
echo "   kubectl get pods -n suluna-production"
echo "   curl -I https://links.suluna.mx"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Environment variables for subsequent scripts:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "export SULUNA_ORG_ID=$ORG_ID"
echo "export SULUNA_NAMESPACE=suluna-production"
echo "export SULUNA_ROLE_ID=$ROLE_ID"
echo ""
